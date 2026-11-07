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
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import dev.clipconnect.app.network.TcpClient
import dev.clipconnect.app.service.ClipboardAccessibilityService
import java.net.UnknownHostException

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
    }

    override fun onResume() {
        super.onResume()
        checkBatteryOptimization()
    }

    override fun onPause() {
        super.onPause()
    }

    private fun checkBatteryOptimization() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            val pm = getSystemService(android.content.Context.POWER_SERVICE) as android.os.PowerManager
            if (!pm.isIgnoringBatteryOptimizations(packageName)) {
                try {
                    val intent = Intent().apply {
                        action = Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS
                        data = Uri.parse("package:$packageName")
                    }
                    intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                    startActivity(intent)
                } catch (e: Exception) {
                    Log.d("MainActivity", "Battery optimization dialog failed: ${e.message}")
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MainScreen(activity: MainActivity) {
    var connectionStatus by remember { mutableStateOf("Disconnected") }
    var accessibilityEnabled by remember { mutableStateOf(false) }
    var pairedHost by remember { mutableStateOf("") }
    var pairedPort by remember { mutableStateOf(0) }
    var showPairingDialog by remember { mutableStateOf(false) }
    var showScanner by remember { mutableStateOf(false) }

    val tcpClient = remember { TcpClient("", 0, ByteArray(32)) }

    LaunchedEffect(Unit) {
        accessibilityEnabled = isAccessibilityEnabled(activity)
    }

    // Load saved pairing
    LaunchedEffect(Unit) {
        val prefs = activity.getSharedPreferences("clipconnect", android.content.Context.MODE_PRIVATE)
        val savedHost = prefs.getString("host", null)
        val savedPort = prefs.getInt("port", 0)
        val savedSecretBase64 = prefs.getString("secret", null)
        if (savedHost != null && savedSecretBase64 != null) {
            pairedHost = savedHost
            pairedPort = savedPort
            val secret = android.util.Base64.decode(savedSecretBase64, android.util.Base64.DEFAULT)
            tcpClient.setConfig(savedHost, savedPort, secret)
            tcpClient.start()
            
            // Wait for accessibility service to be ready (in background)
            Thread {
                for (i in 1..40) {
                    val svc = ClipboardAccessibilityService.instance
                    if (svc != null) {
                        svc.configure(tcpClient)
                        break
                    }
                    Thread.sleep(500)
                }
            }.start()
        }
    }

    LaunchedEffect(tcpClient) {
        tcpClient.setListener(object : TcpClient.Listener {
            override fun onConnected() { connectionStatus = "Connected" }
            override fun onDisconnected(reason: String) { connectionStatus = "Disconnected: $reason" }
            override fun onMessage(text: String) {
                val clipboard = activity.getSystemService(android.content.Context.CLIPBOARD_SERVICE) as android.content.ClipboardManager
                clipboard.setPrimaryClip(android.content.ClipData.newPlainText("ClipConnect", text))
            }
            override fun onError(error: String) { connectionStatus = "Error: $error" }
        })
    }

    fun doPair(host: String, port: Int, secret: ByteArray) {
        pairedHost = host
        pairedPort = port
        connectionStatus = "Connecting..."
        val prefs = activity.getSharedPreferences("clipconnect", android.content.Context.MODE_PRIVATE)
        prefs.edit().putString("host", host).putInt("port", port)
            .putString("secret", android.util.Base64.encodeToString(secret, android.util.Base64.DEFAULT)).apply()
        tcpClient.disconnect()
        tcpClient.setConfig(host, port, secret)
        tcpClient.start()
        ClipboardAccessibilityService.instance?.configure(tcpClient)
    }

    Scaffold(topBar = { TopAppBar(title = { Text("ClipConnect") }) }) { padding ->
        Column(
            modifier = Modifier.fillMaxSize().padding(padding).padding(16.dp).verticalScroll(rememberScrollState()),
            horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Top
        ) {
            Card(modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp)) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text("Connection", style = MaterialTheme.typography.titleMedium)
                    Spacer(modifier = Modifier.height(8.dp))
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        CircularProgressIndicator(progress = if (connectionStatus == "Connected") 1f else 0f, modifier = Modifier.size(24.dp))
                        Spacer(modifier = Modifier.width(12.dp))
                        Text(connectionStatus, style = MaterialTheme.typography.bodyLarge)
                    }
                }
            }

            Card(modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp)) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text("Pair", style = MaterialTheme.typography.titleMedium)
                    Spacer(modifier = Modifier.height(8.dp))
                    if (pairedHost.isEmpty()) {
                        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            Button(onClick = { showPairingDialog = true }, modifier = Modifier.weight(1f)) { Text("Manual") }
                            Button(onClick = { showScanner = true }, modifier = Modifier.weight(1f)) {
                                Text("QR")
                            }
                        }
                    } else {
                        Text("Paired: $pairedHost:$pairedPort", style = MaterialTheme.typography.bodyMedium)
                        Spacer(modifier = Modifier.height(8.dp))
                        OutlinedButton(onClick = {
                            pairedHost = ""; pairedPort = 0; tcpClient.disconnect()
                            connectionStatus = "Disconnected"
                            activity.getSharedPreferences("clipconnect", android.content.Context.MODE_PRIVATE).edit().clear().apply()
                        }) { Text("Unpair") }
                    }
                }
            }

            if (showScanner) {
                ScannerOverlay(onDismiss = { showScanner = false }, onResult = { uriStr ->
                    parseUri(activity, tcpClient, uriStr, ::doPair)
                })
            }

            if (showPairingDialog) {
                PairingDialog(onDismiss = { showPairingDialog = false }, onPair = { h, p, s ->
                    doPair(h, p, s); showPairingDialog = false
                })
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ScannerOverlay(onDismiss: () -> Unit, onResult: (String) -> Unit) {
    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(modifier = Modifier.padding(16.dp).fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
            Text("Scan QR Code", style = MaterialTheme.typography.titleMedium)
            Spacer(modifier = Modifier.height(8.dp))
            Text("Point camera at laptop's pairing QR", style = MaterialTheme.typography.bodySmall)
            Spacer(modifier = Modifier.height(16.dp))
            Button(onClick = onDismiss) { Text("Cancel") }
        }
    }
}

@Composable
fun PairingDialog(onDismiss: () -> Unit, onPair: (host: String, port: Int, secret: ByteArray) -> Unit) {
    var host by remember { mutableStateOf("") }
    var port by remember { mutableStateOf("48123") }
    var secret by remember { mutableStateOf("") }
    AlertDialog(
        onDismissRequest = { onDismiss() },
        title = { Text("Manual Pair") },
        text = {
            Column {
                OutlinedTextField(value = host, onValueChange = { host = it }, label = { Text("IP") }, modifier = Modifier.fillMaxWidth())
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedTextField(value = port, onValueChange = { port = it }, label = { Text("Port") }, modifier = Modifier.fillMaxWidth())
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedTextField(value = secret, onValueChange = { secret = it }, label = { Text("Secret") }, modifier = Modifier.fillMaxWidth())
            }
        },
        confirmButton = {
            Button(onClick = {
                if (host.isNotBlank() && secret.isNotBlank()) {
                    val b64 = secret.replace('-', '+').replace('_', '/')
                    val padded = when (b64.length % 4) { 2 -> b64 + "=="; 3 -> b64 + "="; else -> b64 }
                    onPair(host, port.toIntOrNull() ?: 48123, android.util.Base64.decode(padded, android.util.Base64.DEFAULT))
                }
            }) { Text("Pair") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } }
    )
}

private fun parseUri(activity: android.content.Context, tcpClient: TcpClient, rawValue: String, onPair: (host: String, port: Int, secret: ByteArray) -> Unit) {
    try {
        val uri = Uri.parse(rawValue)
        if (uri.scheme != "clipconnect" || uri.host != "pair") { Log.e("MainActivity", "Bad URI: $rawValue"); return }
        val secretB64 = uri.getQueryParameter("k") ?: ""
        val host = uri.getQueryParameter("h")?.split(",")?.firstOrNull() ?: ""
        val port = uri.getQueryParameter("p")?.toIntOrNull() ?: 48123
        if (secretB64.isBlank() || host.isBlank()) return
        val b64 = secretB64.replace('-', '+').replace('_', '/')
        val padded = when (b64.length % 4) { 2 -> b64 + "=="; 3 -> b64 + "="; else -> b64 }
        val secret = android.util.Base64.decode(padded, android.util.Base64.DEFAULT)
        Log.d("MainActivity", "Parsed: host=$host port=$port")
        onPair(host, port, secret)
    } catch (e: Exception) { Log.e("MainActivity", "Parse error: ${e.message}") }
}

private fun isAccessibilityEnabled(activity: MainActivity): Boolean {
    val enabled = Settings.Secure.getString(activity.contentResolver, Settings.Secure.ENABLED_ACCESSIBILITY_SERVICES) ?: return false
    return enabled.contains("${activity.packageName}/dev.clipconnect.app.service.ClipboardAccessibilityService")
}
