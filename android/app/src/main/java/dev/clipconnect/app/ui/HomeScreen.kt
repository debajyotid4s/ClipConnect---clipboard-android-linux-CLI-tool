package dev.clipconnect.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.height
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import dev.clipconnect.app.R

/**
 * Home screen: one screen, one state, one action.
 *
 * A hero status orb shows the link state at a glance. Below it, two lines say
 * where the link is and when it last synced. A single contextual primary action
 * matches the state (Pair / Connecting / Disconnect / Retry), with a secondary
 * route to Diagnostics and to forgetting the pairing.
 *
 * A recent-syncs strip gives copy-through feedback. A capture caveat chip appears
 * only when the app is unfocused (outgoing sync can't run). The MIUI checklist is
 * intentionally left to Diagnostics to keep the home screen calm.
 */
@Composable
fun HomeScreen(
    state: LinkState,
    recent: List<SyncItem>,
    captureActive: Boolean,
    onPair: () -> Unit,
    onForget: () -> Unit,
    onDiagnostics: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(modifier = modifier.fillMaxSize(), color = MaterialTheme.colorScheme.surface) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(20.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(18.dp),
        ) {
            SpacerTop()
            StatusOrb(state = state)
            StatusLabel(state)
            SyncSummary(state, recent)
            Spacer(8)
            PrimaryAction(state, onPair, onDiagnostics)
            Spacer(4)
            SecondaryActions(state, onForget, onDiagnostics)
            Spacer(12)
            if (!captureActive) {
                CaptureCaveatChip()
            }
        }
    }
}

@Composable
private fun SpacerTop() {
    androidx.compose.foundation.layout.Spacer(modifier = Modifier.height(4.dp))
}

@Composable
private fun Spacer(dp: Int) {
    androidx.compose.foundation.layout.Spacer(modifier = Modifier.height(dp.dp))
}

@Composable
private fun StatusLabel(state: LinkState) {
    val (title, sub) = when (state) {
        is LinkState.Unpaired -> stringResource(R.string.status_not_connected) to stringResource(R.string.action_pair)
        is LinkState.Connecting -> stringResource(R.string.status_connecting) to stringResource(R.string.connecting_subtitle)
        is LinkState.Reconnecting -> stringResource(R.string.status_reconnecting) to stringResource(R.string.reconnecting_subtitle)
        is LinkState.Connected -> stringResource(R.string.status_connected) to stringResource(R.string.connected_subtitle)
        is LinkState.Failed -> stringResource(R.string.status_failed) to state.reason
    }
    Text(title, style = MaterialTheme.typography.headlineMedium, fontWeight = FontWeight.SemiBold)
    Spacer(2)
    Text(sub, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
}

@Composable
private fun SyncSummary(state: LinkState, recent: List<SyncItem>) {
    if (state is LinkState.Connected && recent.isNotEmpty()) {
        val last = recent.last()
        Text(
            "Last sync ${last.timeSince(System.currentTimeMillis())} · ${last.chars} chars",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun PrimaryAction(
    state: LinkState,
    onPair: () -> Unit,
    onDiagnostics: () -> Unit,
) {
    when (state) {
        is LinkState.Unpaired -> Button(onClick = onPair) { Text(stringResource(R.string.action_pair)) }
        is LinkState.Connecting, is LinkState.Reconnecting -> {
            androidx.compose.foundation.layout.Row(
                horizontalArrangement = Arrangement.spacedBy(12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                CircularProgressIndicator(modifier = Modifier.size(20.dp))
                Text("Please wait…", style = MaterialTheme.typography.bodyMedium)
            }
        }
        is LinkState.Connected -> {
            // Connected: the primary affordance is reading logs; disconnecting is
            // a secondary overflow action handled by the caller.
            OutlinedButton(onClick = onDiagnostics) { Text(stringResource(R.string.action_view_logs)) }
        }
        is LinkState.Failed -> Button(onClick = onPair) { Text("Retry") }
    }
}

@Composable
private fun SecondaryActions(
    state: LinkState,
    onForget: () -> Unit,
    onDiagnostics: () -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.Center,
    ) {
        if (state is LinkState.Connected || state is LinkState.Failed || state is LinkState.Connecting || state is LinkState.Reconnecting) {
            androidx.compose.material3.TextButton(onClick = onDiagnostics) { Text(stringResource(R.string.action_diagnostics)) }
        }
        when (state) {
            is LinkState.Connected, is LinkState.Failed, is LinkState.Connecting, is LinkState.Reconnecting ->
                androidx.compose.material3.TextButton(onClick = onForget) { Text(stringResource(R.string.action_forget)) }
            else -> {}
        }
    }
}

@Composable
private fun CaptureCaveatChip() {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.small,
        color = MaterialTheme.colorScheme.primaryContainer,
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                stringResource(R.string.hint_open_app),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onPrimaryContainer,
            )
        }
    }
}
