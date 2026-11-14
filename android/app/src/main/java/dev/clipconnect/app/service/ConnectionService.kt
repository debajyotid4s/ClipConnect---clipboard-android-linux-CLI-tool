package dev.clipconnect.app.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.ClipboardManager
import android.os.IBinder
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

        @Volatile
        var currentClient: TcpClient? = null
            private set
    }

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
    private val pollLock = Any()

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
                sync = ClipboardSync(this, tcpClient).apply { attachIncoming() }
                tcpClient.start()
                startOutgoingPoll()
            }
            // Make client available to MainActivity without binding
            currentClient = tcpClient
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
        val server = ClipboardIpcServer(PORT) { readPhoneClipboard() }
        ipcServer = if (server.start() > 0) server else null
        ipcClient = ClipboardIpcClient(PORT)
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
        if (text.isBlank()) return
        val hash = Crypto.sha256(text)
        synchronized(pollLock) {
            if (hash == lastClipHash) return
            lastClipHash = hash
        }
        val client = currentClient
        if (client == null || !client.isConnected) return
        val seq = client.sendClip(text)
        if (seq == 0L) {
            // Session dropped: forget the hash so it re-sends on reconnect.
            synchronized(pollLock) { lastClipHash = null }
            Log.e(TAG, "poll: send failed, will retry on reconnect")
        }
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
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
        currentClient = null
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
            .setPriority(android.app.Notification.PRIORITY_LOW)
        return builder.build()
    }
}
