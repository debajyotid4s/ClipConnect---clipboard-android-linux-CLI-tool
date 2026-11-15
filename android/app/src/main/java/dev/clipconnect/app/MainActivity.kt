package dev.clipconnect.app

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import android.util.Log
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.platform.LocalContext
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import dev.clipconnect.app.network.TcpClient
import dev.clipconnect.app.service.ConnectionService
import dev.clipconnect.app.service.IncomingClipApplier
import dev.clipconnect.app.setup.MiUiSetup
import dev.clipconnect.app.discovery.MdnsDiscoverer
private const val TAG = "MainActivity"

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            MaterialTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    MainScreen(this@MainActivity)
                }
            }
        }

        // Handle deep link (QR scan result via URI)
        intent?.data?.let { uri ->
            if (uri.scheme == "clipconnect" && uri.host == "pair") {
                handlePairingUri(uri.toString())
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        intent.data?.let { uri ->
            if (uri.scheme == "clipconnect" && uri.host == "pair") {
                handlePairingUri(uri.toString())
            }
        }
    }

    private fun handlePairingUri(uriStr: String) {
        parsePairingUri(uriStr) { host, port, secret ->
            savePairing(host, port, secret)
            startConnectionService(host, port, secret)
        }
    }

    private fun parsePairingUri(rawValue: String, onPair: (host: String, port: Int, secret: ByteArray) -> Unit) {
        try {
            val uri = Uri.parse(rawValue)
            if (uri.scheme != "clipconnect" || uri.host != "pair") {
                Log.e(TAG, "Bad URI: $rawValue")
                return
            }
            val secretB64 = (uri.getQueryParameter("k") ?: "").trim()
            val host = (uri.getQueryParameter("h")?.split(",")?.firstOrNull() ?: "").trim()
            val port = uri.getQueryParameter("p")?.toIntOrNull() ?: 48123
            if (secretB64.isBlank() || host.isBlank()) return
            val b64 = secretB64.replace('-', '+').replace('_', '/')
            val padded = when (b64.length % 4) { 2 -> b64 + "=="; 3 -> b64 + "="; else -> b64 }
            val secret = android.util.Base64.decode(padded, android.util.Base64.DEFAULT)
            Log.d(TAG, "Parsed: host=$host port=$port")
            onPair(host, port, secret)
        } catch (e: Exception) {
            Log.e(TAG, "Parse error: ${e.message}")
        }
    }

    private fun savePairing(host: String, port: Int, secret: ByteArray) {
        val prefs = getSharedPreferences("clipconnect", Context.MODE_PRIVATE)
        prefs.edit().apply {
            putString("host", host)
            putInt("port", port)
            putString("secret", android.util.Base64.encodeToString(secret, android.util.Base64.DEFAULT))
            apply()
        }
    }

    private fun startConnectionService(host: String, port: Int, secret: ByteArray) {
        val intent = Intent(this, ConnectionService::class.java).apply {
            putExtra(ConnectionService.EXTRA_HOST, host)
            putExtra(ConnectionService.EXTRA_PORT, port)
            putExtra(ConnectionService.EXTRA_SECRET, secret)
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }

    private fun stopConnectionService() {
        val intent = Intent(this, ConnectionService::class.java)
        stopService(intent)
    }

    private fun clearPairing() {
        getSharedPreferences("clipconnect", Context.MODE_PRIVATE).edit().clear().apply()
    }

    private fun loadPairing(): Pair<String, Int>? {
        val prefs = getSharedPreferences("clipconnect", Context.MODE_PRIVATE)
        val host = prefs.getString("host", null) ?: return null
        val port = prefs.getInt("port", 0)
        return Pair(host, port)
    }

    private fun loadSecret(): ByteArray? {
        val prefs = getSharedPreferences("clipconnect", Context.MODE_PRIVATE)
        val secretB64 = prefs.getString("secret", null) ?: return null
        return android.util.Base64.decode(secretB64, android.util.Base64.DEFAULT)
    }

    private val notificationPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) {}

    private val localNetworkPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) {}

    @OptIn(ExperimentalMaterial3Api::class)
    @Composable
    fun MainScreen(activity: MainActivity) {
        var connectionStatus by remember { mutableStateOf("Disconnected") }
        var pairedHost by remember { mutableStateOf("") }
        var pairedPort by remember { mutableIntStateOf(0) }
        var showPairingDialog by remember { mutableStateOf(false) }
        var showScanner by remember { mutableStateOf(false) }
        var checkNotificationPermission by remember { mutableStateOf(false) }
        var checkLocalNetwork by remember { mutableStateOf(false) }
        var batteryPromptShown by remember { mutableStateOf(false) }
        var tcpClient by remember { mutableStateOf<TcpClient?>(null) }
        var syncing by remember { mutableStateOf(false) }

        // Single coroutine scope for off-main clipboard work. Cancelled on stop.
        val uiScope = remember { CoroutineScope(Dispatchers.Main) }

        // Load saved pairing on first composition
        LaunchedEffect(Unit) {
            val pairing = activity.loadPairing()
            if (pairing != null) {
                pairedHost = pairing.first
                pairedPort = pairing.second
            }

            // Start connection service if paired
            if (pairing != null) {
                val secret = activity.loadSecret()
                if (secret != null) {
                    activity.startConnectionService(pairing.first, pairing.second, secret)
                }
            }
        }

        // Auto-discover the laptop over mDNS once paired, and switch the
        // connection to the resolved address. The QR IPs remain the fallback.
        LaunchedEffect(Unit) {
            val pairing = activity.loadPairing()
            if (pairing == null) return@LaunchedEffect
            val secret = activity.loadSecret() ?: return@LaunchedEffect
            val id = dev.clipconnect.app.crypto.Crypto.mdnsId(secret)
            val discoverer = MdnsDiscoverer(this@MainActivity, id).apply {
                onService = { host, port ->
                    if (host != pairedHost) {
                        Log.d(TAG, "mDNS discovered laptop $host:$port; reconnecting")
                        activity.startConnectionService(host, port, secret)
                    }
                }
            }
            discoverer.start()
            // Discovery is best-effort; keep the QR fallback alive if it times out.
            delay(20000)
            discoverer.stop()
        }

        // Outgoing sync (phone -> laptop) is fully automated: the ConnectionService
        // polls the clipboard via an in-process IPC server every ~800ms and sends
        // changed clips to the laptop. No manual launch or tap is required.
        // Bind to ConnectionService to get the real TcpClient
        LaunchedEffect(tcpClient) {
            tcpClient?.let { client ->
                client.setListener(object : TcpClient.Listener {
                    override fun onConnected() {
                        connectionStatus = "Connected"
                        IncomingClipApplier.attach(IncomingClipApplier())
                        Log.d(TAG, "UI: Connected")
                    }
                    override fun onDisconnected(reason: String) {
                        connectionStatus = "Disconnected: $reason"
                        Log.d(TAG, "UI: Disconnected: $reason")
                    }
                    override fun onMessage(text: String) {
                        // Apply the laptop clip to the system clipboard (echo-guarded).
                        // Run off the main thread so clipboard IPC never stalls UI.
                        uiScope.launch(Dispatchers.IO) {
                            IncomingClipApplier.apply(this@MainActivity, text)
                        }
                        Log.d(TAG, "Received clip from laptop: ${text.length} chars")
                    }
                    override fun onError(error: String) {
                        connectionStatus = "Error: $error"
                        Log.e(TAG, "UI: Error: $error")
                    }
                })
            }
        }

        // Request notification permission (Android 13+)
        if (checkNotificationPermission && Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            checkNotificationPermission = false
            notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
        }

        // Request local network permission (Android 17+, API 37+)
        if (checkLocalNetwork && Build.VERSION.SDK_INT >= Build.VERSION_CODES.VANILLA_ICE_CREAM) {
            checkLocalNetwork = false
            localNetworkPermissionLauncher.launch("android.permission.ACCESS_LOCAL_NETWORK")
        }

        // Show battery optimization prompt once
        if (batteryPromptShown && Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            batteryPromptShown = false
            try {
                val pm = activity.getSystemService(android.content.Context.POWER_SERVICE) as android.os.PowerManager
                if (!pm.isIgnoringBatteryOptimizations(activity.packageName)) {
                    val intent = Intent().apply {
                        action = Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS
                        data = Uri.parse("package:${activity.packageName}")
                    }
                    intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                    activity.startActivity(intent)
                }
            } catch (e: Exception) {
                Log.d(TAG, "Battery optimization dialog failed: ${e.message}")
            }
        }

        // Observe connection state from the service via StateFlow (no main-thread
        // polling). The service flips this on connect/disconnect on its own threads.
        val context = LocalContext.current
        var clientAcquired by remember { mutableStateOf(false) }

        LaunchedEffect(clientAcquired) {
            val client = ServiceHolder.currentClient
            if (client != null) {
                tcpClient = client
                Log.d(TAG, "Acquired TcpClient from ConnectionService holder (connected=${client.isConnected})")
                clientAcquired = true
            }
        }

        // Collect the service's connection StateFlow off the main thread, then
        // flip UI state on the main dispatcher. Never blocks on IO.
        LaunchedEffect(Unit) {
            ServiceHolder.connectionState.collect { connected ->
                if (connected) {
                    connectionStatus = "Connected"
                    Log.d(TAG, "UI: Connected (via StateFlow)")
                }
            }
        }

        fun doPair(host: String, port: Int, secret: ByteArray) {
            pairedHost = host
            pairedPort = port
            connectionStatus = "Connecting..."
            activity.savePairing(host, port, secret)
            activity.startConnectionService(host, port, secret)
        }

        fun doUnpair() {
            activity.clearPairing()
            activity.stopConnectionService()
            pairedHost = ""
            pairedPort = 0
            connectionStatus = "Disconnected"
            tcpClient = null
        }

        Scaffold(topBar = { TopAppBar(title = { Text("ClipConnect") }) }) { padding ->
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding)
                    .padding(16.dp)
                    .verticalScroll(rememberScrollState()),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.Top
            ) {
                // Connection status card
                Card(modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp)) {
                    Column(modifier = Modifier.padding(16.dp)) {
                        Text("Connection", style = MaterialTheme.typography.titleMedium)
                        Spacer(modifier = Modifier.height(8.dp))
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            CircularProgressIndicator(
                                progress = if (connectionStatus == "Connected") 1f else 0f,
                                modifier = Modifier.size(24.dp)
                            )
                            Spacer(modifier = Modifier.width(12.dp))
                            Text(connectionStatus, style = MaterialTheme.typography.bodyLarge)
                        }
                    }
                }

                // Pair card
                Card(modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp)) {
                    Column(modifier = Modifier.padding(16.dp)) {
                        Text("Pair", style = MaterialTheme.typography.titleMedium)
                        Spacer(modifier = Modifier.height(8.dp))
                        if (pairedHost.isEmpty()) {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.spacedBy(8.dp)
                            ) {
                                Button(
                                    onClick = { showPairingDialog = true },
                                    modifier = Modifier.weight(1f)
                                ) { Text("Manual") }
                                Button(
                                    onClick = { showScanner = true },
                                    modifier = Modifier.weight(1f)
                                ) { Text("QR") }
                            }
                        } else {
                            Text(
                                "Paired: $pairedHost:$pairedPort",
                                style = MaterialTheme.typography.bodyMedium
                            )
                            Spacer(modifier = Modifier.height(8.dp))
                            OutlinedButton(onClick = {
                                showPairingDialog = true
                            }, modifier = Modifier.fillMaxWidth()) {
                                Text("Update")
                            }
                            Spacer(modifier = Modifier.height(4.dp))
                            OutlinedButton(onClick = { doUnpair() }, modifier = Modifier.fillMaxWidth()) {
                                Text("Unpair")
                            }
                        }
                    }
                }

                // Sync to Laptop (phone -> laptop) card — fully automated.
                Card(modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp)) {
                    Column(modifier = Modifier.padding(16.dp)) {
                        Text("Sync to Laptop", style = MaterialTheme.typography.titleMedium)
                        Spacer(modifier = Modifier.height(8.dp))
                        Text(
                            "Your phone clipboard is read automatically and sent to the laptop over the LAN. Changes appear within ~1 second.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        Spacer(modifier = Modifier.height(4.dp))
                        Text(
                            "Mode: automatic (continuous)",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.primary
                        )
                    }
                }

                // Setup checklist — state + deep-link + App-info fallback.
                // Re-checked every app start via MiUiSetup.rows().
                val setupRows = MiUiSetup.rows(activity)
                Card(modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp)) {
                    Column(modifier = Modifier.padding(16.dp)) {
                        Text("Survival Checklist (MIUI/HyperOS)", style = MaterialTheme.typography.titleMedium)
                        Spacer(modifier = Modifier.height(8.dp))
                        Text(
                            "Keep these on so the connection survives background and reboot.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        Spacer(modifier = Modifier.height(8.dp))
                        for (row in setupRows) {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Text(
                                    if (row.ok) "✓ " else "✗ ",
                                    color = if (row.ok) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error,
                                    style = MaterialTheme.typography.bodyMedium
                                )
                                Text(row.label, style = MaterialTheme.typography.bodyMedium, modifier = Modifier.weight(1f))
                                if (!row.ok && row.intent != null) {
                                    TextButton(onClick = {
                                        try {
                                            activity.startActivity(row.intent)
                                        } catch (e: Throwable) {
                                            Log.d(TAG, "deep-link failed: ${e.message}")
                                        }
                                    }) { Text("Open") }
                                } else if (!row.ok) {
                                    Text("manual", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                            }
                            Spacer(modifier = Modifier.height(4.dp))
                        }
                    }
                }
            }
        }

        if (showScanner) {
            ScannerOverlay(
                onDismiss = { showScanner = false },
                onResult = { uriStr ->
                    parsePairingUri(uriStr) { host, port, secret ->
                        doPair(host, port, secret)
                    }
                }
            )
        }

        if (showPairingDialog) {
            PairingDialog(
                onDismiss = { showPairingDialog = false },
                onPair = { h, p, s -> doPair(h, p, s) },
                currentHost = pairedHost,
                currentPort = pairedPort
            )
        }
    }

    @Composable
    fun PairingDialog(
        onDismiss: () -> Unit,
        onPair: (host: String, port: Int, secret: ByteArray) -> Unit,
        currentHost: String,
        currentPort: Int
    ) {
        var host by remember { mutableStateOf(currentHost) }
        var port by remember { mutableStateOf(if (currentPort > 0) currentPort.toString() else "48123") }
        var secret by remember { mutableStateOf("") }

        AlertDialog(
            onDismissRequest = { onDismiss() },
            title = { Text("Manual Pair") },
            text = {
                Column {
                    OutlinedTextField(
                        value = host,
                        onValueChange = { host = it },
                        label = { Text("IP") },
                        modifier = Modifier.fillMaxWidth()
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    OutlinedTextField(
                        value = port,
                        onValueChange = { port = it },
                        label = { Text("Port") },
                        modifier = Modifier.fillMaxWidth()
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    OutlinedTextField(
                        value = secret,
                        onValueChange = { secret = it },
                        label = { Text("Secret (base64url)") },
                        modifier = Modifier.fillMaxWidth()
                    )
                }
            },
            confirmButton = {
                Button(onClick = {
                    if (host.isNotBlank() && secret.isNotBlank()) {
                        val b64 = secret.replace('-', '+').replace('_', '/')
                        val padded = when (b64.length % 4) {
                            2 -> b64 + "=="
                            3 -> b64 + "="
                            else -> b64
                        }
                        val decoded = android.util.Base64.decode(padded.trim(), android.util.Base64.DEFAULT)
                        if (decoded.isNotEmpty()) {
                            onPair(host, port.toIntOrNull() ?: 48123, decoded)
                        }
                    }
                }) { Text("Pair") }
            },
            dismissButton = {
                TextButton(onClick = onDismiss) { Text("Cancel") }
            }
        )
    }
}

/**
 * Thin accessor so composables can read the service's client and connection
 * StateFlow without importing the Service class (avoids a circular import and
 * keeps the UI layer decoupled from the Service singleton).
 */
object ServiceHolder {
    val currentClient: TcpClient? get() = ConnectionService.currentClient
    val connectionState get() = ConnectionService.connectionState
}
