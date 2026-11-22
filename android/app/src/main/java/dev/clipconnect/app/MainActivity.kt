package dev.clipconnect.app

import android.net.Uri
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.util.Log
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import dev.clipconnect.app.service.ClipboardCapture
import dev.clipconnect.app.service.readCurrentOnFocusGain
import dev.clipconnect.app.service.onClipApplied
import dev.clipconnect.app.ui.AppScreen
import dev.clipconnect.app.ui.LinkViewModel

private const val TAG = "MainActivity"

/**
 * Hosts the pairing UI, the connection service binding, and focus-gated outgoing
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
 *
 * The [LinkViewModel] owns link state; [AppScreen] owns sheet/dialog navigation and
 * consumes any pending deep-link pairing URL. This Activity only wires the service
 * lifecycle and focus capture to them.
 */
class MainActivity : ComponentActivity(), ClipboardCapture.FocusOwner {

    private val handler = Handler(Looper.getMainLooper())
    private var captureActive by mutableIntStateOf(0) // 0 = false, 1 = true

    // A clipconnect:// URL arriving before composition is buffered here and
    // consumed by AppScreen, which has access to the ViewModel.
    private var pendingPairingUri: String? = null

    private lateinit var clipboardCapture: ClipboardCapture

    // Created once and retained across recompositions. Config changes are
    // handled by retaining it in the Activity (no viewModel() dependency, which
    // is unavailable offline).
    private val viewModel: LinkViewModel by lazy {
        LinkViewModel(applicationContext as android.app.Application)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        clipboardCapture = ClipboardCapture(
            this,
            { null },
        ) { chars ->
            // Transient confirmation that a copy was sent. Length-only: never
            // surfaces clipboard content.
            android.widget.Toast.makeText(
                applicationContext,
                applicationContext.getString(R.string.clip_sent, chars),
                android.widget.Toast.LENGTH_SHORT,
            ).show()
        }
        ClipboardCapture.registerAppliedListener { text -> clipboardCapture.onClipApplied(text) }

        // Handle deep link (QR scan result via URI). Buffer it; AppScreen consumes
        // it once the ViewModel is available.
        intent?.data?.let { uri ->
            if (uri.scheme == "clipconnect" && uri.host == "pair") {
                pendingPairingUri = uri.toString()
            }
        }

        setContent {
            // Observe the service's event stream once the UI is composed.
            viewModel.attach()
            AppScreen(
                viewModel = viewModel,
                captureActive = captureActive == 1,
                pendingPairingUri = pendingPairingUri,
            )
        }
    }

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        captureActive = if (hasFocus) 1 else 0
        if (hasFocus) {
            clipboardCapture.onFocusGained(this)
            clipboardCapture.readCurrentOnFocusGain()
        } else {
            clipboardCapture.onFocusLost()
        }
    }

    override fun hasWindowFocus(): Boolean = super.hasWindowFocus()

    override fun onDestroy() {
        clipboardCapture.destroy()
        handler.removeCallbacksAndMessages(null)
        super.onDestroy()
    }
}
