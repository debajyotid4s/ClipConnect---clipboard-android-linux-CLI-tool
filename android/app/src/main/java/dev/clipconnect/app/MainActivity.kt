package dev.clipconnect.app

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.util.Log
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import dev.clipconnect.app.network.TcpClient
import dev.clipconnect.app.service.ClipboardCapture
import dev.clipconnect.app.service.readCurrentOnFocusGain
import dev.clipconnect.app.service.onClipApplied
import dev.clipconnect.app.service.ConnectionService
import dev.clipconnect.app.service.SecretStore
import java.util.concurrent.atomic.AtomicBoolean

private const val TAG = "MainActivity"

/**
 * Hosts pairing UI, the connection service binding, and focus-gated outgoing
 * clipboard capture.
 *
 * Capture model (honest about Android 10+ focus restrictions):
 *  - Outgoing clipboard capture is event-driven and only runs while this Activity
 *    holds window focus. [ClipboardCapture] registers its listener on focus gain and
 *    unregisters on focus loss, so we never pretend to monitor the clipboard while
 *    backgrounded.
 *  - Incoming clips (laptop -> phone) keep working regardless of focus because the
 *    TCP connection lives in a background service.
 *  - [captureActive] reflects whether outgoing capture is currently enabled; the UI
 *    shows it so users understand why a backgrounded copy may not sync.
 */
class MainActivity : ComponentActivity(), ClipboardCapture.FocusOwner {

    internal val bindConnections = mutableListOf<android.content.ServiceConnection>()
    private val handler = Handler(Looper.getMainLooper())

    // captureActive tracks whether outgoing clipboard capture is currently enabled
    // (window focus held). Exposed to the UI for honest status.
    private val captureActive = AtomicBoolean(false)

    // The TcpClient from the bound service, set in onServiceConnected.
    internal var boundTcpClient: TcpClient? = null

    private lateinit var clipboardCapture: ClipboardCapture

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        clipboardCapture = ClipboardCapture(
            this,
            { tcpClientForBinding() },
        ) { chars ->
            // Transient confirmation that a copy was sent. Length-only: never
            // surfaces clipboard content.
            android.widget.Toast.makeText(
                applicationContext,
                applicationContext.getString(dev.clipconnect.app.R.string.clip_sent, chars),
                android.widget.Toast.LENGTH_SHORT,
            ).show()
        }
        // Let IncomingClipApplier mark laptop-applied clips as already-sent so
        // the outgoing capture never echoes them back.
        ClipboardCapture.registerAppliedListener { text -> clipboardCapture.onClipApplied(text) }

        // Handle deep link (QR scan result via URI)
        intent?.data?.let { uri ->
            if (uri.scheme == "clipconnect" && uri.host == "pair") {
                handlePairingUri(uri.toString())
            }
        }

        setContent {
            MaterialTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    MainScreen(this@MainActivity, captureActive.get())
                }
            }
        }
    }

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        if (hasFocus) {
            // Re-register listener and re-read current clipboard once so reopening the
            // app captures the latest available content. Android has no clipboard
            // history, so only the current value can be recovered.
            clipboardCapture.onFocusGained(this)
            clipboardCapture.readCurrentOnFocusGain()
            captureActive.set(true)
        } else {
            clipboardCapture.onFocusLost()
            captureActive.set(false)
        }
    }

    override fun hasWindowFocus(): Boolean = super.hasWindowFocus()

    override fun onDestroy() {
        boundTcpClient = null
        clipboardCapture.destroy()
        for (c in bindConnections) {
            try { unbindService(c) } catch (e: Throwable) { /* already unbound */ }
        }
        bindConnections.clear()
        handler.removeCallbacksAndMessages(null)
        super.onDestroy()
    }

    // --- pairing storage / service wiring ---

    private fun handlePairingUri(uriStr: String) {
        parsePairingUri(this, uriStr) { host, port, secret ->
            savePairing(host, port, secret)
            startConnectionService(host, port, secret)
        }
    }

    internal fun parsePairingUri(activity: MainActivity, rawValue: String, onPair: (host: String, port: Int, secret: ByteArray) -> Unit) {
        try {
            val uri = Uri.parse(rawValue)
            if (uri.scheme != "clipconnect" || uri.host != "pair") {
                Log.e(TAG, "Bad URI: $rawValue")
                return
            }
            val secretB64 = (uri.getQueryParameter("k") ?: "").trim()
            val host = (uri.getQueryParameter("h")?.split(",")?.firstOrNull() ?: "").trim()
            val port = uri.getQueryParameter("p")?.toIntOrNull() ?: 10369
            if (secretB64.isBlank() || host.isBlank()) return
            val b64 = secretB64.replace('-', '+').replace('_', '/')
            val padded = when (b64.length % 4) { 2 -> b64 + "=="; 3 -> b64 + "="; else -> b64 }
            val secret = android.util.Base64.decode(padded, android.util.Base64.DEFAULT)
            Log.d(TAG, "Parsed: host=$host port=$port")
            onPair(host, port, secret)
        } catch (e: Exception) {
            Log.e(TAG, "Parse error: ${e.message}")
        }
    }

    internal fun savePairing(host: String, port: Int, secret: ByteArray) {
        val prefs = getSharedPreferences("clipconnect", Context.MODE_PRIVATE)
        prefs.edit().apply {
            putString("host", host)
            putInt("port", port)
            // Stable per-install identity so the laptop can key its durable
            // outbox and replay unacknowledged clips across reconnects.
            if (prefs.getString("phoneId", null) == null) {
                putString("phoneId", generatePhoneId())
            }
            apply()
        }
        // Encrypt+persist the secret via Android Keystore (not plain storage).
        SecretStore.save(this, secret)
    }

    /** A stable, random per-install identity sent in the hello frame. */
    private fun generatePhoneId(): String {
        val bytes = ByteArray(12)
        java.security.SecureRandom().nextBytes(bytes)
        return android.util.Base64.encodeToString(bytes, android.util.Base64.NO_WRAP)
    }

    /** The stable per-install identity, or null if not yet paired. */
    private fun phoneId(): String? =
        getSharedPreferences("clipconnect", Context.MODE_PRIVATE).getString("phoneId", null)

    internal fun startConnectionService(host: String, port: Int, secret: ByteArray) {
        val intent = Intent(this, ConnectionService::class.java).apply {
            putExtra(ConnectionService.EXTRA_HOST, host)
            putExtra(ConnectionService.EXTRA_PORT, port)
            putExtra(ConnectionService.EXTRA_SECRET, secret)
        }
        startService(intent) // plain background service; capture is focus-gated
    }

    internal fun stopConnectionService() {
        stopService(Intent(this, ConnectionService::class.java))
    }

    /** Returns the bound TcpClient, or null if not yet bound. */
    private fun tcpClientForBinding(): TcpClient? = boundTcpClient

    internal fun clearPairing() {
        getSharedPreferences("clipconnect", Context.MODE_PRIVATE).edit().clear().apply()
        SecretStore.clear(this)
    }

    internal fun loadPairing(): Pair<String, Int>? {
        val prefs = getSharedPreferences("clipconnect", Context.MODE_PRIVATE)
        val host = prefs.getString("host", null) ?: return null
        val port = prefs.getInt("port", 0)
        return Pair(host, port)
    }

    internal fun loadSecret(): ByteArray? = SecretStore.load(this)

    private val qrPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) {}
}
