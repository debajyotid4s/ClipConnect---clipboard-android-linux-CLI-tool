package dev.clipconnect.app.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.ClipboardManager
import android.os.IBinder
import android.os.Binder
import android.os.Handler
import android.os.Looper
import android.util.Log
import dev.clipconnect.app.R
import dev.clipconnect.app.bridge.ClipboardIpcClient
import dev.clipconnect.app.bridge.ClipboardIpcServer
import dev.clipconnect.app.crypto.Crypto
import dev.clipconnect.app.network.TcpClient
import dev.clipconnect.app.sync.ClipboardSync

/**
 * Foreground service that maintains the TCP connection.
 * Keeps the app alive in the background even when the UI is not visible.
 * Incoming clips (laptop->phone) are applied by IncomingClipApplier.
 */
class ConnectionService : Service() {

    companion object {
        const val CHANNEL_ID = "clipconnect_connection"
        const val NOTIFICATION_ID = 1001
        const val EXTRA_HOST = "host"
        const val EXTRA_PORT = "port"
        const val EXTRA_SECRET = "secret"

        // Outgoing poll interval. Short enough to feel live, light enough to
        // avoid waking the bridge on every frame.
        const val POLL_MS = 800L

        // Fixed loopback port for the in-process clipboard IPC server. Picked from
        // the ephemeral range; the app both binds and connects to it.
        const val PORT = 39876

        private const val TAG = "ConnectionService"

        // Emits connection state so the UI can observe it via a StateFlow instead
        // of polling a static holder on the main thread.
        private val _connectionState = MutableStateFlow(false)
        val connectionState = _connectionState.asStateFlow()

        fun markConnected(value: Boolean) {
            _connectionState.value = value
        }
    }

    // Binder exposed to bound clients (the Activity) so it can read the TcpClient.
    // The service is the single owner of the network connection.
    inner class LocalBinder : Binder() {
        fun getService(): ConnectionService = this@ConnectionService
    }
    private val binder = LocalBinder()

    private val tcpClient = TcpClient()
    private var isRunning = false
    private var sync: ClipboardSync? = null

    // Continuous outgoing poll (phone -> laptop). The app binds its own in-process
    // IPC server that reads the clipboard via the public ClipboardManager API, and
    // the poll loop connects to it. No ADB / Shell process required.
    private var pollThread: Thread? = null
    private var ipcServer: ClipboardIpcServer? = null
    private var ipcClient: ClipboardIpcClient? = null
    private var lastClipHash: String? = null
    // Hash of the clipboard content we last OBSERVED (read), tracked separately
    // from [lastClipHash] (the last thing we SENT). This lets us tell a genuine
    // clipboard change apart from spurious onPrimaryClipChanged callbacks that
    // fire on app focus/switch and re-read an unchanged clipboard. Without this,
    // switching into the app would set the send slot to a stale value and drop
    // the next real clip.
    private var lastReadHash: String? = null
    private val pollLock = Any()

    // Event-driven outgoing path. When the user copies something, the system
    // fires this callback almost immediately (far faster than the fallback poll
    // and immune to MIUI's "stale for a cycle" read). The poll below remains as
    // a safety net in case the listener is suppressed while the app is
    // backgrounded (MIUI/HyperOS behaviour varies by vendor).
    private var clipboardManager: ClipboardManager? = null
    private var primaryClipListener: ClipboardManager.OnPrimaryClipChangedListener? = null

    // This service manages both directions:
    //  - incoming (laptop -> phone): applied by IncomingClipApplier
    //  - outgoing  (phone -> laptop): polled from the clipboard via in-process IPC

    override fun onCreate() {
        super.onCreate()
        Log.d(TAG, "ConnectionService onCreate")
        createNotificationChannel()
        startForeground(NOTIFICATION_ID, buildNotification())
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent == null) return START_NOT_STICKY

        val host = intent.getStringExtra(EXTRA_HOST) ?: ""
        val port = intent.getIntExtra(EXTRA_PORT, 0)
        val secret = intent.getByteArrayExtra(EXTRA_SECRET) ?: ByteArray(0)

        if (host.isNotEmpty() && secret.isNotEmpty()) {
            tcpClient.setConfig(host, port, secret)
            if (!isRunning) {
                isRunning = true
                sync = ClipboardSync(this, tcpClient).apply {
                    attachIncoming()
                    onConnected = {
                        resetOutgoingDedupe()
                        markConnected(true)
                    }
                }
                tcpClient.start()
                startOutgoingPoll()
            }
        }
        return START_STICKY
    }

    /**
     * Continuous outgoing poll: every [POLL_MS] read the phone clipboard through
     * the in-process IPC server, and if the text changed, send it to the laptop.
     * Runs on a dedicated daemon thread so it keeps working while the UI is
     * backgrounded. The app both binds and connects to the local socket, so no
     * ADB or Shell process is required.
     */
    private fun startOutgoingPoll() {
        if (pollThread != null) return
        clipboardManager = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        val server = ClipboardIpcServer(PORT) { readPhoneClipboard() }
        ipcServer = if (server.start() > 0) server else null
        ipcClient = ClipboardIpcClient(PORT)
        registerClipboardListener()
        pollThread = Thread({
            while (!Thread.currentThread().isInterrupted) {
                try {
                    ipcClient?.readClipboard()?.let { sendIfChanged(it) }
                } catch (e: Throwable) {
                    Log.d(TAG, "poll read error: ${e.message}")
                }
                try {
                    Thread.sleep(POLL_MS)
                } catch (e: InterruptedException) {
                    break
                }
            }
        }).apply { name = "clip-poll"; isDaemon = true; start() }
        Log.d(TAG, "outgoing poll started (${POLL_MS}ms)")
    }

    /**
     * Listen for clipboard changes and push any new clip to the laptop the
     * instant it happens. Runs on the main thread, so it only queues work; the
     * actual read+send happens here directly (fast path) and is deduped by
     * [sendIfChanged]. We skip clips we just applied from the laptop to avoid
     * an echo loop.
     */
    private fun registerClipboardListener() {
        val mgr = clipboardManager ?: return
        val listener = object : ClipboardManager.OnPrimaryClipChangedListener {
            override fun onPrimaryClipChanged() {
                // readWithRetries updates lastReadHash and returns null when the
                // clipboard content is unchanged from what we last observed. This
                // ignores spurious callbacks that fire on app focus/switch (which
                // re-read an unchanged clipboard), so switching into the app can
                // no longer pollute the send slot and drop the next real clip.
                val text = readWithRetries()
                if (!text.isNullOrEmpty()) {
                    sendIfChanged(text)
                }
            }
        }
        try {
            mgr.addPrimaryClipChangedListener(listener)
            primaryClipListener = listener
            Log.d(TAG, "primary-clip listener registered")
        } catch (e: Throwable) {
            Log.e(TAG, "failed to register primary-clip listener: ${e.message}")
        }
    }

    /**
     * Read the clipboard, retrying briefly to survive MIUI's slow settle.
     *
     * Updates [lastReadHash] to the content we last observed. Callers use that to
     * tell a genuine clipboard change apart from spurious onPrimaryClipChanged
     * callbacks (which fire on app focus/switch and re-read an unchanged
     * clipboard). A stale re-read of already-observed content returns null here
     * so it never reaches the send path.
     */
    private fun readWithRetries(): String? {
        for (i in 0 until 6) {
            val text = try {
                val clip = clipboardManager?.getPrimaryClip()
                if (clip == null || clip.itemCount != 1) null else clip.getItemAt(0)?.text?.toString()
            } catch (e: Throwable) {
                null
            }
            if (text.isNullOrEmpty()) {
                yield150()
                continue
            }
            val hash = Crypto.sha256(text)
            val alreadyObserved = synchronized(pollLock) { hash == lastReadHash }
            if (alreadyObserved) {
                // Same content we already read/forwarded: a stale or focus-triggered
                // re-read. Keep retrying in case a newer copy is settling, but do
                // NOT advance lastReadHash past it.
                yield150()
                continue
            }
            synchronized(pollLock) { lastReadHash = hash }
            return text
        }
        return null
    }

    private fun yield150() {
        try {
            Thread.sleep(150)
        } catch (e: InterruptedException) {
            Thread.currentThread().interrupt()
        }
    }

    /**
     * Called on connect/disconnect so the outgoing dedupe does not carry a stale
     * hash across a reconnect (which would silently drop the next real clip).
     */
    private fun resetOutgoingDedupe() {
        synchronized(pollLock) {
            lastClipHash = null
            lastReadHash = null
        }
    }

    /** Read the current primary text clip via the public ClipboardManager API. */
    private fun readPhoneClipboard(): String? {
        return try {
            val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
            val clip = clipboard.getPrimaryClip() ?: return null
            if (clip.itemCount != 1) return null
            val text = clip.getItemAt(0).text ?: return null
            text.toString()
        } catch (e: Throwable) {
            Log.d(TAG, "clipboard read denied/failed: ${e.message}")
            null
        }
    }

    /** Send a clip only if its content differs from the last one we sent. */
    private fun sendIfChanged(text: String) {
        if (!OutgoingDedupe.isNew(text, lastClipHash)) return
        synchronized(pollLock) { lastClipHash = Crypto.sha256(text) }
        val client = tcpClient
        if (client == null || !client.isConnected) return
        val seq = client.sendClip(text)
        if (seq == 0L) {
            // Session dropped: forget the hash so it re-sends on reconnect.
            synchronized(pollLock) { lastClipHash = null }
            Log.e(TAG, "poll: send failed, will retry on reconnect")
        }
    }

    override fun onBind(intent: Intent?): IBinder? = binder

    /** Exposed to bound clients (the Activity) to read the shared TcpClient. */
    fun tcpClientForBinding(): TcpClient = tcpClient

    override fun onDestroy() {
        markConnected(false)
        isRunning = false
        sync?.stop()
        sync = null
        ipcServer?.stop()
        ipcServer = null
        ipcClient = null
        synchronized(pollLock) { lastClipHash = null }
        pollThread?.interrupt()
        pollThread = null
        tcpClient.disconnect()
        stopForeground(STOP_FOREGROUND_REMOVE)
        super.onDestroy()
    }

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            "ClipConnect Connection",
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "Maintains clipboard sync connection"
        }
        val manager = getSystemService(NOTIFICATION_SERVICE) as android.app.NotificationManager
        manager.createNotificationChannel(channel)
    }

    private fun buildNotification(): Notification {
        val builder = android.app.Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("ClipConnect")
            .setContentText("Clipboard sync active")
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setOngoing(true)
            .setPriority(android.app.Notification.PRIORITY_HIGH)
            .setShowWhen(false)
        return builder.build()
    }
}
