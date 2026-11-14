package dev.clipconnect.app.sync

import android.content.Context
import android.util.Log
import dev.clipconnect.app.network.TcpClient
import dev.clipconnect.app.service.IncomingClipApplier
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Owns the incoming direction (laptop -> phone) on the phone.
 *
 *  - [TcpClient] delivers a clip, we apply it to the system clipboard
 *    (echo-guarded via [IncomingClipApplier]).
 *
 * The outgoing direction (phone -> laptop) is served by the ADB bridge in
 * MainActivity, which reads the clipboard as Shell and sends each new clip.
 *
 * Clipboard plaintext is only ever handed to the system clipboard; it is never
 * logged.
 */
class ClipboardSync(
    private val context: Context,
    private val tcpClient: TcpClient
) {
    companion object {
        private const val TAG = "ClipboardSync"
    }

    private val active = AtomicBoolean(false)

    /** Wire the TCP listener that applies incoming laptop clips. */
    fun attachIncoming() {
        tcpClient.setListener(object : TcpClient.Listener {
            override fun onConnected() {
                Log.d(TAG, "Connected")
            }

            override fun onDisconnected(reason: String) {
                Log.d(TAG, "Disconnected: $reason")
            }

            override fun onMessage(text: String) {
                IncomingClipApplier.apply(context, text)
                Log.d(TAG, "Applied laptop clip: ${text.length} chars")
            }

            override fun onError(error: String) {
                Log.e(TAG, "Session error: $error")
            }
        })
    }

    fun stop() {
        active.getAndSet(false)
    }
}
