package dev.clipconnect.app.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

/**
 * Dark-first theme with a single accent (the heartbeat red) and neutral
 * surfaces. Everything else reads from [MaterialTheme.colorScheme] so there are
 * no hardcoded hex colors scattered across the screens.
 *
 * The accent [heartRed] is reused by the status orb and the heartbeat wave so
 * the app's identity is one consistent color.
 */
val heartRed = Color(0xFFF23B3B)

private val DarkColors = darkColorScheme(
    primary = heartRed,
    onPrimary = Color.White,
    primaryContainer = heartRed.copy(alpha = 0.28f),
    onPrimaryContainer = heartRed,
    secondary = Color(0xFF8E96A6),
    surface = Color(0xFF14151C),
    surfaceVariant = Color(0xFF1E1F29),
    onSurface = Color(0xFFE6E8EC),
    onSurfaceVariant = Color(0xFF9AA0AC),
    error = Color(0xFFF2757A),
    errorContainer = Color(0xFF5D1215),
)

private val LightColors = lightColorScheme(
    primary = heartRed,
    onPrimary = Color.White,
    primaryContainer = heartRed.copy(alpha = 0.18f),
    onPrimaryContainer = heartRed,
    secondary = Color(0xFF4C5361),
    surface = Color.White,
    surfaceVariant = Color(0xFFF1F2F4),
    onSurface = Color(0xFF14151C),
    onSurfaceVariant = Color(0xFF5B616D),
    error = Color(0xFFB3261E),
    errorContainer = Color(0xFFF5DBDA),
)

/**
 * Wraps the app content in the theme. The app ships dark-first, so we default to
 * the dark palette unless the system explicitly requests light.
 */
@Composable
fun ClipConnectTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit
) {
    MaterialTheme(
        colorScheme = if (darkTheme) DarkColors else LightColors,
        content = content
    )
}
