package dev.clipconnect.app.ui

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.delay
import dev.clipconnect.app.Heartbeat

/**
 * The status orb is the screen's hero and the app's identity. It renders the
 * link state as a ring:
 *
 *  - Connected: a slow breathing ring (radius pulses gently, forever).
 *  - Connecting / Reconnecting: a faster pulse that grows and fades each cycle.
 *  - Unpaired / Failed: a dim, still ring.
 *
 * On each [Heartbeat] pulse (connect, or ack of a sent clip) the ring does one
 * sharp kick: it brightens and jumps radius, then settles back. The animation
 * sizes itself to the observed canvas so it looks right on any screen density.
 */
@Composable
fun StatusOrb(
    state: LinkState,
    modifier: Modifier = Modifier,
    sizeDp: Float = 120f,
) {
    val accent = heartRed
    val still = Color(0xFF3A3F4B)

    val (baseColor, animColor) = when (state) {
        is LinkState.Connected -> accent to accent
        is LinkState.Connecting -> accent to accent
        is LinkState.Reconnecting -> accent.copy(alpha = 0.6f) to accent
        is LinkState.Unpaired -> still to still
        is LinkState.Failed -> Color(0xFFF2757A) to Color(0xFFF2757A)
    }

    // Breathing base: radius oscillates smoothly between two fractions of the
    // orb radius. Only meaningful while alive (connected/connecting).
    val breathing = remember { Animatable(0f) }
    val isAlive = state is LinkState.Connected || state is LinkState.Connecting || state is LinkState.Reconnecting
    LaunchedEffect(isAlive) {
        if (isAlive) {
            val spec = when (state) {
                is LinkState.Reconnecting -> tween<Float>(900)
                else -> tween<Float>(1800)
            }
            // Loop 0 -> 1 -> 0 forever for a gentle breathing pulse.
            while (true) {
                breathing.animateTo(1f, spec)
                breathing.animateTo(0f, spec)
            }
        }
    }

    // Ack/connect kick: one sharp radius + brightness jump per pulse.
    val kickSize = remember { Animatable(0f) }
    val kickAlpha = remember { Animatable(0f) }
    LaunchedEffect(Heartbeat.pulse) {
        Heartbeat.pulse.collect { pulse ->
            if (pulse <= 0) return@collect
            val radiusKick = tween<Float>(180)
            val fadeKick = tween<Float>(900)
            kickSize.animateTo(1f, radiusKick)
            kickAlpha.animateTo(1f, radiusKick)
            delay(120)
            kickAlpha.animateTo(0f, fadeKick)
        }
    }

    Canvas(modifier = modifier) {
        val diameter = this.size.minDimension
        val radius = diameter / 2f
        // Breathing radius: base fraction plus a gentle oscillation.
        val breathe = if (isAlive) 0.94f + 0.06f * breathing.value else 0.97f
        val kick = if (isAlive) 0.10f * kickSize.value else 0f
        val ringRadius = radius * (breathe + kick)

        // Soft glow disc behind the ring, fading on the kick.
        drawCircle(
            color = accent.copy(alpha = 0.10f + 0.12f * kickAlpha.value),
            radius = ringRadius * 0.82f,
            center = Offset(size.width / 2f, size.height / 2f),
        )

        // The ring itself.
        drawCircle(
            color = if (isAlive) accent.copy(alpha = 0.35f + 0.65f * kickAlpha.value) else still.copy(alpha = 0.5f),
            radius = ringRadius,
            center = Offset(size.width / 2f, size.height / 2f),
            style = Stroke(
                width = if (isAlive) 6f else 4f,
                cap = StrokeCap.Round,
            ),
        )
    }
}
