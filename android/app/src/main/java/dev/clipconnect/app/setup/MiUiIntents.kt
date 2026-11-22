package dev.clipconnect.app.setup

import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.Settings
import android.util.Log

/**
 * Deep links for the MIUI/HyperOS survival checklist. All are try/catch wrapped
 * at the call site and return null if unresolved, so a missing component never
 * crashes the UI.
 */
internal fun MiUiSetup.miuiAutoStartIntent(context: android.content.Context): Intent? {
    return try {
        Intent().apply {
            setComponent(
                android.content.ComponentName(
                    "com.miui.securitycenter",
                    "com.miui.permcenter.autostart.AutoStartManagementActivity"
                )
            )
        }
    } catch (e: Throwable) {
        Log.e("MiUiSetup", "autoStart intent failed: ${e.message}")
        null
    }
}

internal fun MiUiSetup.miuiPermissionsIntent(context: android.content.Context): Intent? {
    return try {
        Intent().apply {
            setComponent(
                android.content.ComponentName(
                    "com.miui.securitycenter",
                    "com.miui.permcenter.permissions.PermissionsEditorActivity"
                )
            )
            putExtra("extra_pkgname", context.packageName)
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
    } catch (e: Throwable) {
        Log.e("MiUiSetup", "permissions intent failed: ${e.message}")
        null
    }
}

internal fun MiUiSetup.miuiBatteryNoRestrictIntent(context: android.content.Context): Intent? {
    return try {
        Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS).apply {
            data = Uri.parse("package:${context.packageName}")
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
    } catch (e: Throwable) {
        Log.e("MiUiSetup", "battery intent failed: ${e.message}")
        null
    }
}

internal fun MiUiSetup.standardAppInfoIntent(context: android.content.Context): Intent? {
    return try {
        Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS).apply {
            data = Uri.parse("package:${context.packageName}")
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
    } catch (e: Throwable) {
        Log.e("MiUiSetup", "app info intent failed: ${e.message}")
        null
    }
}
