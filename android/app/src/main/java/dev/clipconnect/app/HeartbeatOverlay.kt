package dev.clipconnect.app

import android.content.Context
import android.os.VibrationEffect
import android.os.Vibrator
import android.os.VibratorManager
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.delay
import kotlin.math.max
import kotlin.math.sqrt
import kotlinx.coroutines.launch

/**
 * HeartbeatOverlay draws a smooth red wave that expands from the center of the
 * screen and fades out. It is shown briefly whenever a confirmation event
 * happens — the phone connects, or a clip it sent is acknowledged by the
 * laptop.
 *
 * The effect is intentionally simple: one ring that grows from radius 0 to the
 * diagonal of the screen while fading from opaque to transparent. A single
 * light vibration accompanies it.
 */
@Composable
fun HeartbeatOverlay(
    trigger: Int,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current

    // Radius grows from 0 to the screen's half-diagonal so the ring clears the
    // corners on any device. Measured from the canvas size at draw time.
    val maxRadius = remember { 0f }

    // Alpha: invisible -> opaque -> invisible, so the wave appears then fades.
    val alpha = remember { Animatable(0f) }
    // Progress: 0 -> 1 across the animation duration.
    val progress = remember { Animatable(0f) }

    LaunchedEffect(trigger) {
        // Grow the ring across the full duration.
        launch {
            progress.animateTo(
                targetValue = 1f,
                animationSpec = tween(durationMillis = 900)
            )
        }
        // Fade in quickly, then fade out so the screen ends clean.
        launch {
            alpha.animateTo(1f, tween(durationMillis = 120))
            delay(120)
            alpha.animateTo(0f, tween(durationMillis = 780))
        }
        // Vibrate once, gently, on the heartbeat.
        vibrateOnce(context)
    }

    Canvas(modifier = modifier.alpha(max(alpha.value, 0f))) {
        val size = this.size
        // Half-diagonal of the canvas, minus a margin so the ring doesn't clip.
        val diag = sqrt(size.width * size.width + size.height * size.height) / 2f
        val center = Offset(size.width / 2f, size.height / 2f)
        val radius = (diag * 0.92f) * progress.value
        drawCircle(
            color = Color(0xFFF23B3B),
            radius = radius,
            center = center,
            style = androidx.compose.ui.graphics.drawscope.Stroke(
                width = 6f,
                cap = StrokeCap.Round
            )
        )
    }
}

/**
 * A single gentle vibration. Uses the API 31+ VibratorManager when available and
 * falls back to the legacy Vibrator otherwise. Guarded so devices without a
 * vibrator never crash.
 */
private fun vibrateOnce(context: Context) {
    try {
        val vibrator = if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.S) {
            val vm = context.getSystemService(Context.VIBRATOR_MANAGER_SERVICE) as? VibratorManager
            vm?.defaultVibrator
        } else {
            context.getSystemService(Context.VIBRATOR_SERVICE) as? Vibrator
        }
        if (vibrator == null) return
        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.Q) {
            vibrator.vibrate(
                VibrationEffect.createOneShot(60, VibrationEffect.DEFAULT_AMPLITUDE)
            )
        } else {
            @Suppress("DEPRECATION")
            vibrator.vibrate(60)
        }
    } catch (e: Throwable) {
        // Vibration is a nicety; never let it break the flow.
    }
}
