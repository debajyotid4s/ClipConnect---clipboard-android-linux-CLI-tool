package dev.clipconnect.app

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import dev.clipconnect.app.service.ConnectionService

/**
 * Receives boot completed event and restarts the connection service.
 * The accessibility service is re-enabled by Android's accessibility settings
 * (user must enable it once after reboot).
 */
class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED) return

        // Restore pairing from shared preferences; secret from Keystore-backed store.
        val prefs = context.getSharedPreferences("clipconnect", Context.MODE_PRIVATE)
        val host = prefs.getString("host", null) ?: return
        val port = prefs.getInt("port", 0)
        val secret = dev.clipconnect.app.service.SecretStore.load(context) ?: return

        // Restart the connection service
        val serviceIntent = Intent(context, ConnectionService::class.java).apply {
            putExtra(ConnectionService.EXTRA_HOST, host)
            putExtra(ConnectionService.EXTRA_PORT, port)
            putExtra(ConnectionService.EXTRA_SECRET, secret)
        }

        android.os.Build.VERSION.SDK_INT.let {
            if (it >= android.os.Build.VERSION_CODES.O) {
                context.startForegroundService(serviceIntent)
            } else {
                context.startService(serviceIntent)
            }
        }
    }
}
