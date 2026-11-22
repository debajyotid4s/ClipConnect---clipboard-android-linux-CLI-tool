package dev.clipconnect.app.ui

import android.content.ClipboardManager
import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import kotlinx.coroutines.flow.collectLatest

/**
 * Top-level app shell. Owns the navigation state for the pairing sheet and the
 * diagnostics screen, and composes [HomeScreen] as the default view.
 *
 * It consumes a pending deep-link pairing URL (from the Activity) by handing it
 * to the manual/scan path, and derives the recent-syncs strip from the
 * ViewModel's event stream.
 */
@Composable
fun AppScreen(
    viewModel: LinkViewModel,
    captureActive: Boolean,
    pendingPairingUri: String?,
) {
    var showPairSheet by remember { mutableStateOf(false) }
    var showDiagnostics by remember { mutableStateOf(false) }

    val state by viewModel.state.collectAsState()
    val events by viewModel.events.collectAsState()
    val logLines by viewModel.log.collectAsState()

    // Build the recent-syncs strip from the event stream (length-only).
    val recent = remember(events) {
        events.takeLast(5).map { event ->
            when (event) {
                is SyncEvent.Sent -> SyncItem(SyncItem.Direction.Sent, event.chars, System.currentTimeMillis())
                is SyncEvent.Received -> SyncItem(SyncItem.Direction.Received, event.chars, System.currentTimeMillis())
                is SyncEvent.Acked -> null
            }
        }.filterNotNull()
    }

    // Consume a pending deep-link pairing URL on first composition.
    LaunchedEffect(pendingPairingUri) {
        val uri = pendingPairingUri ?: return@LaunchedEffect
        when (val parsed = PairingParser.parseUri(uri)) {
            is PairingParser.Result.Valid -> viewModel.connect(parsed.host, parsed.port, parsed.secret)
            is PairingParser.Result.Invalid -> { /* silently ignored; user can use manual */ }
        }
    }

    if (showDiagnostics) {
        val context = androidx.compose.ui.platform.LocalContext.current
        DiagnosticsScreen(
            lines = logLines,
            onCopy = { copyLog(context, logLines) },
        )
    } else {
        HomeScreen(
            state = state,
            recent = recent,
            captureActive = captureActive,
            onPair = { showPairSheet = true },
            onForget = { viewModel.forget() },
            onDiagnostics = { showDiagnostics = true },
        )
    }

    if (showPairSheet) {
        PairSheet(
            onScan = { showPairSheet = false },
            onManual = { host, port, secret ->
                viewModel.connect(host, port, secret)
                showPairSheet = false
            },
            onDismiss = { showPairSheet = false },
        )
    }
}

/** Copy the diagnostics transcript to the system clipboard (length only). */
private fun copyLog(context: android.content.Context, lines: List<LogLine>) {
    val text = lines.joinToString("\n") { "${formatClock(it.atMillis)} [${it.kind}] ${it.message}" }
    val clipboard = context.getSystemService(Context.CLIPBOARD_SERVICE) as? ClipboardManager ?: return
    val clip = android.content.ClipData.newPlainText("ClipConnect diagnostics", text)
    clipboard.setPrimaryClip(clip)
}
