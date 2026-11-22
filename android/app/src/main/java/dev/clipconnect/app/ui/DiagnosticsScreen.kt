package dev.clipconnect.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.ui.res.stringResource
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import dev.clipconnect.app.R

/**
 * Diagnostics screen: a scrollable, read-only transcript of connection events.
 *
 * Moved out of the home screen so logs no longer dominate the one-screen,
 * one-state, one-action design. Supports copying the transcript to the clipboard
 * (length-only affordance) and uses theme-aware colors so it stays readable in
 * dark or light mode.
 */
@Composable
fun DiagnosticsScreen(
    lines: List<LogLine>,
    onCopy: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier.fillMaxSize()) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
        ) {
            Text(
                "Diagnostics",
                style = MaterialTheme.typography.titleLarge,
                color = MaterialTheme.colorScheme.onSurface,
            )
            Spacer(modifier = Modifier.weight(1f))
            OutlinedButton(onClick = onCopy) { Text(stringResource(R.string.diag_copy)) }
        }
        LazyColumn(
            modifier = Modifier.fillMaxWidth().fillMaxSize(),
            contentPadding = PaddingValues(16.dp),
        ) {
            if (lines.isEmpty()) {
                item {
                    Text(
                        "No events yet.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            items(lines, key = { it.atMillis }) { line ->
                LogRow(line)
                Spacer(modifier = Modifier.height(6.dp))
            }
        }
    }
}

@Composable
private fun LogRow(line: LogLine) {
    val color = when (line.kind) {
        LogLine.Kind.Error -> MaterialTheme.colorScheme.error
        LogLine.Kind.Warn -> MaterialTheme.colorScheme.secondary
        LogLine.Kind.Ok -> Color(0xFF4CC37A)
        LogLine.Kind.Info -> MaterialTheme.colorScheme.onSurfaceVariant
    }
    Row(modifier = Modifier.fillMaxWidth()) {
        Text(
            formatClock(line.atMillis),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.weight(0.3f),
        )
        Text(
            line.message,
            style = MaterialTheme.typography.bodySmall,
            color = color,
            fontFamily = FontFamily.Monospace,
            modifier = Modifier.weight(0.7f),
        )
    }
}
