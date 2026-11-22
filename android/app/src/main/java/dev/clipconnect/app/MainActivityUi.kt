package dev.clipconnect.app

import android.content.Context
import android.content.Intent
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import dev.clipconnect.app.network.TcpClient
import dev.clipconnect.app.service.ConnectionService
import dev.clipconnect.app.setup.MiUiSetup

/**
 * MainScreen renders the connection, pair, sync, and OEM-checklist cards.
 * It observes the bound ConnectionService and wires the pairing actions back
 * to the Activity, which owns the network connection and pairing storage.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MainScreen(activity: MainActivity, captureActive: Boolean) {
    var connectionStatus by remember { mutableStateOf("Disconnected") }
    var pairedHost by remember { mutableStateOf("") }
    var pairedPort by remember { mutableIntStateOf(0) }
    var showPairingDialog by remember { mutableStateOf(false) }
    var showScanner by remember { mutableStateOf(false) }

    LaunchedEffect(Unit) {
        val pairing = activity.loadPairing()
        if (pairing != null) {
            pairedHost = pairing.first
            pairedPort = pairing.second
            connectionStatus = "Connecting…"
            val secret = activity.loadSecret()
            if (secret != null) {
                activity.startConnectionService(pairing.first, pairing.second, secret)
            }
        }
    }

    // Bind to the connection service once; observe connection state and wire the
    // incoming-clip applier. The service is the single owner of the network
    // connection; the Activity only observes and applies incoming clips.
    val context = LocalContext.current
    LaunchedEffect(Unit) {
        val svcConnection = object : android.content.ServiceConnection {
            override fun onServiceConnected(
                name: android.content.ComponentName,
                service: android.os.IBinder
            ) {
                val binder = service as ConnectionService.LocalBinder
                val svc = binder.getService()
                val client = svc.tcpClientForBinding()
                activity.boundTcpClient = client
                client.setListener(object : TcpClient.Listener {
                    override fun onConnected() {
                        connectionStatus = "Connected"
                        android.util.Log.d("MainActivity", "UI: Connected")
                    }
                    override fun onDisconnected(reason: String) {
                        connectionStatus = "Disconnected: $reason"
                        android.util.Log.d("MainActivity", "UI: Disconnected: $reason")
                    }
                    // Incoming clips are applied by the service's own listener
                    // (it owns the connection). We only observe status here.
                    override fun onMessage(text: String) {
                        android.util.Log.d("MainActivity", "Received clip from laptop: ${text.length} chars")
                    }
                    override fun onError(error: String) {
                        connectionStatus = "Error: $error"
                        android.util.Log.e("MainActivity", "UI: Error: $error")
                    }
                })
                if (client.isConnected) {
                    connectionStatus = "Connected"
                }
                android.util.Log.d("MainActivity", "Bound TcpClient (connected=${client.isConnected})")
            }
            override fun onServiceDisconnected(name: android.content.ComponentName) {
                connectionStatus = "Disconnected: service unbound"
            }
        }
        activity.bindService(Intent(activity, ConnectionService::class.java), svcConnection, Context.MODE_PRIVATE)
        activity.bindConnections += svcConnection
    }

    // Single source of truth for the connection label.
    LaunchedEffect(Unit) {
        ConnectionService.connectionState.collect { connected ->
            if (connected) connectionStatus = "Connected"
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
            // Connection status card — honest about capture state.
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
                    Spacer(modifier = Modifier.height(8.dp))
                    // Honest outgoing-capture status.
                    Text(
                        if (captureActive)
                            "Clipboard capture: active (app in foreground)"
                        else
                            "Clipboard capture: paused (open the app to send copies)",
                        style = MaterialTheme.typography.bodySmall,
                        color = if (captureActive) MaterialTheme.colorScheme.primary
                                else MaterialTheme.colorScheme.onSurfaceVariant
                    )
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
                        OutlinedButton(onClick = { showPairingDialog = true }, modifier = Modifier.fillMaxWidth()) {
                            Text("Update")
                        }
                        Spacer(modifier = Modifier.height(4.dp))
                        OutlinedButton(onClick = { doUnpair() }, modifier = Modifier.fillMaxWidth()) {
                            Text("Unpair")
                        }
                    }
                }
            }

            // Sync to Laptop (phone -> laptop) card — automated while foregrounded.
            Card(modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp)) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text("Sync to Laptop", style = MaterialTheme.typography.titleMedium)
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(
                        "While the app is open, copies on this phone are sent to the laptop automatically — no button. " +
                        "Because Android only lets apps read the clipboard when they are in focus, reopen the app to send a copy made while it was backgrounded.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }

            // Setup checklist — state + deep-link + App-info fallback.
            val setupRows = MiUiSetup.rows(activity)
            Card(modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp)) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text("Keep connection alive (OEM background rules)", style = MaterialTheme.typography.titleMedium)
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(
                        "These keep the background connection from being killed. They do not affect clipboard capture, " +
                        "which only works while the app is open.",
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
                                        android.util.Log.d("MainActivity", "deep-link failed: ${e.message}")
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
                activity.parsePairingUri(activity, uriStr) { host, port, secret ->
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

/**
 * PairingDialog collects a manual host/port/secret and hands the parsed values
 * back via [onPair]. Secret entry is length-only at the UI boundary.
 */
@Composable
fun PairingDialog(
    onDismiss: () -> Unit,
    onPair: (host: String, port: Int, secret: ByteArray) -> Unit,
    currentHost: String,
    currentPort: Int
) {
    var host by remember { mutableStateOf(currentHost) }
    var port by remember { mutableStateOf(if (currentPort > 0) currentPort.toString() else "10369") }
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
                        onPair(host, port.toIntOrNull() ?: 10369, decoded)
                    }
                }
            }) { Text("Pair") }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("Cancel") }
        }
    )
}
