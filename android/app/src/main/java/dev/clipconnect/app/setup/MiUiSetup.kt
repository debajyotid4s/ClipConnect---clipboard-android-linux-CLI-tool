package dev.clipconnect.app.setup

import android.content.Context
import android.util.Log

/**
 * MIUI/HyperOS survival checklist.
 *
 * Each item reports its current state and, when not satisfied, offers a deep-link
 * to the right settings screen. Every call is wrapped in try/catch and falls back
 * to the generic App-info Page. Anything not exercised on a real device is marked
 * UNVERIFIED. Re-check every app start.
 *
 * Clipboard content is never touched here.
 */
object MiUiSetup {
    private const val TAG = "MiUiSetup"

    /** One checklist row: label, whether it currently passes, and where to deep-link. */
    data class Row(
        val label: String,
        val ok: Boolean,
        val intent: android.content.Intent?
    )

    /** All checklist rows for the current device/OS. */
    fun rows(context: Context): List<Row> {
        return listOf(
            notificationRow(context),
            autostartRow(context),
            batteryRow(context),
            pauseUnusedRow(context),
            lockInRecentsRow()
        )
    }

    /** Notification permission (Android 13+). */
    private fun notificationRow(context: Context): Row {
        if (android.os.Build.VERSION.SDK_INT < android.os.Build.VERSION_CODES.TIRAMISU) {
            return Row("Notification permission", true, null)
        }
        val granted = androidx.core.content.ContextCompat.checkSelfPermission(
            context, android.Manifest.permission.POST_NOTIFICATIONS
        ) == android.content.pm.PackageManager.PERMISSION_GRANTED
        return Row(
            "Notification permission",
            granted,
            if (granted) null else standardAppInfoIntent(context)
        )
    }

    /** Autostart (MIUI/HyperOS Security app). */
    private fun autostartRow(context: Context): Row {
        // Cannot read the autostart flag directly; the Security app is the source
        // of truth. Surface the deep-link and let the user flip it.
        val intent = miuiAutoStartIntent(context)
        return Row("Autostart (background launch)", false, intent)
    }

    /** Per-app battery saver = No restrictions. */
    private fun batteryRow(context: Context): Row {
        // AOSP battery optimization is a proxy; MIUI's own "No restrictions" lives
        // in the Security app and has no readable flag. Deep-link to PowerDetail.
        val pm = context.getSystemService(Context.POWER_SERVICE) as android.os.PowerManager
        val aospOk = pm.isIgnoringBatteryOptimizations(context.packageName)
        val intent = miuiBatteryNoRestrictIntent(context) ?: standardAppInfoIntent(context)
        return Row(
            "Battery saver = No restrictions",
            aospOk,
            intent
        )
    }

    /** "Pause app activity if unused" — toggle inside the per-app permissions screen. */
    private fun pauseUnusedRow(context: Context): Row {
        // No dedicated intent; it lives in the MIUI permissions editor. Deep-link
        // there and label the manual step.
        val intent = miuiPermissionsIntent(context) ?: standardAppInfoIntent(context)
        return Row("Pause app activity if unused", false, intent)
    }

    /** Lock the card in Recents — manual action, no intent. */
    private fun lockInRecentsRow(): Row =
        Row("Locked in Recents", false, null)
}
