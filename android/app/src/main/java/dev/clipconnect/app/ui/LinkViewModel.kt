package dev.clipconnect.app.ui

import android.app.Application
import android.content.Intent
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import dev.clipconnect.app.service.ConnectionService
import dev.clipconnect.app.service.SecretStore
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch

/**
 * Owns the link's visible state and the intents that change it.
 *
 * The ViewModel is the single source of truth for [LinkState] and runs the
 * transition machine as the service publishes connection events. Pairing
 * (host/port/secret) lives in preferences + [SecretStore]; connecting and
 * forgetting are driven by the Activity, which alone can bind/start/stop the
 * service (a bare Context cannot bind).
 *
 * The UI reads [state] and [events] and calls the intent methods; it never binds
 * services or swaps listeners itself, which is what previously broke
 * incoming-clip application and leaked the Activity.
 */
class LinkViewModel(app: Application) : AndroidViewModel(app) {

    private val prefs = app.applicationContext
        .getSharedPreferences("clipconnect", android.content.Context.MODE_PRIVATE)

    private val _state = MutableStateFlow<LinkState>(LinkState.Unpaired)
    val state: StateFlow<LinkState> = _state.asStateFlow()

    private val _events = MutableStateFlow<List<SyncEvent>>(emptyList())
    val events: StateFlow<List<SyncEvent>> = _events.asStateFlow()

    private val logRepository = LogRepository()
    val log = logRepository.entries

    private var attached = false

    init {
        // Restore prior pairing on first creation.
        val host = prefs.getString("host", null)
        if (host != null) {
            _state.value = LinkState.Connecting
            reconnect(host, prefs.getInt("port", 0))
        }
    }

    /** Start observing the service's event stream (called once from the UI). */
    fun attach() {
        if (attached) return
        attached = true
        viewModelScope.launch {
            ConnectionService.events.collectLatest { event ->
                when (event) {
                    is ConnectionService.ConnectionEvent.Connected -> {
                        _state.value = LinkState.Connected(System.currentTimeMillis())
                        logRepository.ok("Connected to laptop")
                    }
                    is ConnectionService.ConnectionEvent.Disconnected -> {
                        _state.value = when (_state.value) {
                            is LinkState.Connected -> LinkState.Reconnecting(1)
                            is LinkState.Connecting -> LinkState.Failed(event.reason)
                            is LinkState.Reconnecting -> LinkState.Reconnecting(event.reason.length.coerceIn(1, 5))
                            else -> LinkState.Failed(event.reason)
                        }
                        logRepository.warn("Disconnected: ${event.reason}")
                    }
                    is ConnectionService.ConnectionEvent.Error -> {
                        _state.value = LinkState.Failed(event.reason)
                        logRepository.error(event.reason)
                    }
                    is ConnectionService.ConnectionEvent.Received -> {
                        _events.value = (_events.value + SyncEvent.Received(event.chars)).takeLast(8)
                    }
                }
            }
        }
        viewModelScope.launch {
            ConnectionService.log.collectLatest { line ->
                logRepository.info(line)
            }
        }
    }

    /** Start (or restart) the connection to a laptop. */
    fun connect(host: String, port: Int, secret: ByteArray) {
        savePairing(host, port, secret)
        _state.value = LinkState.Connecting
        reconnect(host, port)
        logRepository.info("Connecting to $host:$port")
        getApplication<android.app.Application>().startService(connectIntent(host, port))
    }

    /** Forget the pairing and stop the connection. */
    fun forget() {
        clearPairing()
        getApplication<android.app.Application>().stopService(Intent(getApplication<android.app.Application>(), ConnectionService::class.java))
        _state.value = LinkState.Unpaired
        _events.value = emptyList()
        logRepository.ok("Connection revoked and pairing cleared")
    }

    private fun reconnect(host: String, port: Int) {
        // Intent is built here; the Activity starts it so the socket survives
        // Activity recreation.
        _connectIntentHostPort = host to port
    }

    private var _connectIntentHostPort: Pair<String, Int> = "" to 0
    private fun connectIntent(host: String, port: Int): Intent =
        Intent(getApplication(), ConnectionService::class.java).apply {
            putExtra(ConnectionService.EXTRA_HOST, host)
            putExtra(ConnectionService.EXTRA_PORT, port)
            putExtra(ConnectionService.EXTRA_SECRET, loadSecret() ?: ByteArray(0))
        }

    // --- pairing storage ---

    private fun savePairing(host: String, port: Int, secret: ByteArray) {
        prefs.edit().apply {
            putString("host", host)
            putInt("port", port)
            if (prefs.getString("phoneId", null) == null) {
                putString("phoneId", generatePhoneId())
            }
            apply()
        }
        SecretStore.save(getApplication(), secret)
    }

    private fun clearPairing() {
        // Preserve the stable phoneId so the laptop can still key its outbox if
        // the phone reconnects; only drop the pairing-specific values.
        prefs.edit().apply {
            remove("host")
            remove("port")
            remove("secret_enc")
            apply()
        }
        SecretStore.clear(getApplication())
    }

    private fun loadSecret(): ByteArray? = SecretStore.load(getApplication())

    private fun generatePhoneId(): String {
        val bytes = ByteArray(12)
        java.security.SecureRandom().nextBytes(bytes)
        return android.util.Base64.encodeToString(bytes, android.util.Base64.NO_WRAP)
    }
}
