package dev.clipconnect.app

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

/**
 * Receives boot completed event and enables accessibility service.
 */
class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action == "android.intent.action.BOOT_COMPLETED") {
            // Accessibility service will be re-enabled if user had it enabled
            // This is controlled by Android's accessibility settings
        }
    }
}
