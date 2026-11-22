package dev.clipconnect.app.service

import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.asSharedFlow
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.IBinder
import android.os.Binder
import android.util.Log
import dev.clipconnect.app.network.TcpClient
import dev.clipconnect.app.ui.SyncEvent

/**
 * Owns the encrypted TCP connection to the laptop for the life of the process.
 *
 * This is a plain background service — it is NOT a foreground service and runs no
 * clipboard polling loop. Reading the clipboard requires window focus (Android
 * 10+), so capture can only happen while the Activity is foregrounded; keeping a
 * foreground service alive in the background would not extend capture and would
 * only burn battery. The outgoing capture is owned by [ClipboardCapture], which the
 * Activity registers/unregisters against its own window-focus lifecycle.
 *
 * The service still exists so the socket survives Activity recreation (rotation,
 * process death) and so both directions share a single connection owner.
 *
 * State ownership: the service is the single source of truth for the link. It
 * holds the authoritative [TcpClient.Listener] (so incoming clips are always
 * applied) and publishes [connectionState] and [events] as flows. The UI observes
 * these and never replaces the listener — doing so would silently stop incoming
 * clips from being applied (see [onMessage]).
 */
class ConnectionService : Service() {

    companion object {
        private const val TAG = "ConnectionService"

        const val EXTRA_HOST = "host"
        const val EXTRA_PORT = "port"
        const val EXTRA_SECRET = "secret"

        private val _connectionState = MutableStateFlow(false)
        val connectionState = _connectionState.asStateFlow()

        /** One-way lifecycle events the UI observes (connect/disconnect/error). */
        private val _events = MutableSharedFlow<ConnectionEvent>(extraBufferCapacity = 8)
        val events = _events.asSharedFlow()

        /** Structured log lines emitted by the service, observed by Diagnostics. */
        private val _log = MutableSharedFlow<String>(extraBufferCapacity = 32)
        val log = _log.asSharedFlow()

        fun markConnected(value: Boolean) {
            _connectionState.value = value
        }
    }

    /** How the link transitioned; drives the ViewModel's [LinkState] machine. */
    sealed interface ConnectionEvent {
        data object Connected : ConnectionEvent
        data class Disconnected(val reason: String) : ConnectionEvent
        data class Error(val reason: String) : ConnectionEvent
        data class Received(val chars: Int) : ConnectionEvent
    }

    inner class LocalBinder : Binder() {
        fun getService(): ConnectionService = this@ConnectionService
    }
    private val binder = LocalBinder()

    private val tcpClient = TcpClient()
    private val applier = IncomingClipApplier(this)
    private var isRunning = false

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent == null) return START_NOT_STICKY

        val host = intent.getStringExtra(EXTRA_HOST) ?: ""
        val port = intent.getIntExtra(EXTRA_PORT, 0)
        val secret = intent.getByteArrayExtra(EXTRA_SECRET) ?: ByteArray(0)

        if (host.isNotEmpty() && secret.isNotEmpty()) {
            tcpClient.setConfig(host, port, secret)
            // Stable per-install identity for the laptop's durable outbox.
            val prefs = getSharedPreferences("clipconnect", Context.MODE_PRIVATE)
            val pid = prefs.getString("phoneId", null)
            if (!pid.isNullOrEmpty()) tcpClient.setPhoneId(pid)
            if (!isRunning) {
                isRunning = true
                // The service owns this listener for the life of the connection.
                // It applies incoming clips so laptop -> phone sync always works,
                // regardless of whether the UI is currently bound.
                tcpClient.setListener(object : TcpClient.Listener {
                    override fun onConnected() {
                        markConnected(true)
                        _events.tryEmit(ConnectionEvent.Connected)
                        Log.d(TAG, "connected to laptop")
                    }
                    override fun onDisconnected(reason: String) {
                        markConnected(false)
                        _events.tryEmit(ConnectionEvent.Disconnected(reason))
                        Log.d(TAG, "disconnected: $reason")
                    }
                    override fun onMessage(text: String) {
                        applier.apply(text)
                        _events.tryEmit(ConnectionEvent.Received(text.length))
                    }
                    override fun onError(error: String) {
                        _events.tryEmit(ConnectionEvent.Error(error))
                        Log.e(TAG, "session error: $error")
                    }
                })
                _log.tryEmit("Connecting to $host:$port")
                tcpClient.start()
            }
        }
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = binder

    /** Exposed to bound clients (the ViewModel) to read the shared TcpClient. */
    fun tcpClientForBinding(): TcpClient = tcpClient

    override fun onDestroy() {
        markConnected(false)
        isRunning = false
        tcpClient.disconnect()
        super.onDestroy()
    }
}
