package dev.clipconnect.app.discovery

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import android.util.Log
import dev.clipconnect.app.crypto.Crypto
import java.util.concurrent.atomic.AtomicBoolean

/**
 * mDNS discovery of the laptop's `_clipconnect._tcp` service via `NsdManager`.
 *
 * The laptop advertises TXT `id=<hex of first 4 bytes of
 * HMAC-SHA256(secret, "mdns-id")>`. We match on the service type and, when a
 * registered secret is available, on the id. The secret is never transmitted.
 *
 * NsdManager is a platform API; discovery is inherently device-dependent, so
 * this stays thin and surfaces results through [onService].
 */
class MdnsDiscoverer(
    private val context: Context,
    private val expectedId: String? = null
) {
    companion object {
        private const val TAG = "MdnsDiscoverer"
        private const val SERVICE_TYPE = "_clipconnect._tcp"
    }

    private val active = AtomicBoolean(false)
    private val nsdManager = context.getSystemService(Context.NSD_SERVICE) as NsdManager

    /** Invoked for each matching discovered service. */
    var onService: ((host: String, port: Int) -> Unit)? = null
        set(value) {
            field = value
        }

    private val listener = object : NsdManager.DiscoveryListener {
        override fun onStartDiscoveryFailed(regType: String, errorCode: Int) {
            Log.e(TAG, "Start discovery failed: $errorCode")
        }

        override fun onDiscoveryStarted(regType: String) {
            Log.d(TAG, "Discovery started")
        }

        override fun onServiceFound(service: NsdServiceInfo) {
            nsdManager.resolveService(service, resolveCallback)
        }

        override fun onServiceLost(service: NsdServiceInfo) {
            Log.d(TAG, "Service lost: ${service.serviceName}")
        }

        override fun onDiscoveryStopped(regType: String) {
            Log.d(TAG, "Discovery stopped")
        }

        override fun onStopDiscoveryFailed(regType: String, errorCode: Int) {
            Log.e(TAG, "Stop discovery failed: $errorCode")
        }
    }

    private val resolveCallback = object : NsdManager.ResolveListener {
        override fun onResolveFailed(service: NsdServiceInfo, errorCode: Int) {
            Log.d(TAG, "Resolve failed: $errorCode")
        }

        override fun onServiceResolved(service: NsdServiceInfo) {
            if (!active.get()) return
            val host = service.host?.hostName ?: return
            val port = service.port
            if (port == 0) return

            // Match on advertised id when a secret is registered.
            val attrs = service.attributes
            val id = attrs["id"]?.let { String(it, Charsets.UTF_8) }
            if (!expectedId.isNullOrEmpty() && id != expectedId) {
                Log.d(TAG, "mDNS id mismatch: got=$id expected=$expectedId")
                return
            }
            Log.d(TAG, "mDNS service resolved: $host:$port")
            onService?.invoke(host, port)
        }
    }

    fun start() {
        if (active.getAndSet(true)) return
        try {
            nsdManager.discoverServices(SERVICE_TYPE, 0, listener)
        } catch (t: Throwable) {
            Log.e(TAG, "mDNS discovery failed", t)
            active.set(false)
        }
    }

    fun stop() {
        if (!active.getAndSet(false)) return
        try {
            nsdManager.stopServiceDiscovery(listener)
        } catch (_: Exception) {
        }
    }
}
