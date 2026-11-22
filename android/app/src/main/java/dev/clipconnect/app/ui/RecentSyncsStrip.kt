package dev.clipconnect.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.height
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import dev.clipconnect.app.R
import androidx.compose.ui.res.stringResource
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

/**
 * One entry in the recent-syncs strip. Direction is sent (up) or received (down);
 * only time-since and length are shown — never clipboard content.
 */
data class SyncItem(
    val direction: Direction,
    val chars: Int,
    val atMillis: Long,
) {
    enum class Direction { Sent, Received }

    /** Human "Ns ago" style, relative to [nowMillis]. */
    @Composable
    fun timeSince(nowMillis: Long): String {
        val diff = (nowMillis - atMillis).coerceAtLeast(0)
        return when {
            diff < 1_000 -> stringResource(R.string.just_now)
            diff < 60_000 -> "${diff / 1000}s ago"
            diff < 3_600_000 -> "${diff / 60_000}m ago"
            else -> "${diff / 3_600_000}h ago"
        }
    }
}

/**
 * A compact horizontal strip showing the last few synced clips. It gives users
 * the feedback they actually want — whether a copy went through — without
 * surfacing clipboard plaintext. [items] is trimmed to the newest [max].
 */
@Composable
fun RecentSyncsStrip(
    items: List<SyncItem>,
    max: Int = 5,
    nowMillis: Long = System.currentTimeMillis(),
    modifier: Modifier = Modifier,
) {
    val recent = items.takeLast(max)
    if (recent.isEmpty()) return

    Card(
        modifier = modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
    ) {
        Column(modifier = Modifier.padding(12.dp)) {
            Text(
                stringResource(R.string.recent_syncs),
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(4)
            if (recent.isEmpty()) {
                Text(
                    stringResource(R.string.nothing_synced),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            } else {
                Column(modifier = Modifier.fillMaxWidth()) {
                    for (item in recent) {
                        SyncRow(item, nowMillis)
                        Spacer(2)
                    }
                }
            }
        }
    }
}

@Composable
private fun SyncRow(item: SyncItem, nowMillis: Long) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        val sent = item.direction == SyncItem.Direction.Sent
        Text(
            if (sent) "↑" else "↓",
            style = MaterialTheme.typography.titleMedium,
            color = if (sent) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.size(18.dp).width(18.dp),
        )
        Text(
            "${item.chars} chars",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurface,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f),
        )
        Text(
            item.timeSince(nowMillis),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun Spacer(dp: Int) {
    androidx.compose.foundation.layout.Spacer(modifier = Modifier.height(dp.dp))
}

/** Format an epoch millis as HH:MM for an absolute timestamp. */
internal fun formatClock(atMillis: Long): String {
    val formatter = DateTimeFormatter.ofPattern("HH:mm").withZone(ZoneId.systemDefault())
    return formatter.format(Instant.ofEpochMilli(atMillis))
}
