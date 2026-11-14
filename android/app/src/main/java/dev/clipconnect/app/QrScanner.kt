package dev.clipconnect.app

import android.Manifest
import android.content.pm.PackageManager
import android.util.Log
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat

private const val TAG = "QrScanner"

/**
 * QR Code Scanner overlay.
 * Uses quickie library (CameraX + ML Kit) for QR scanning.
 */
@Composable
@OptIn(ExperimentalMaterial3Api::class)
fun ScannerOverlay(
    onDismiss: () -> Unit,
    onResult: (String) -> Unit
) {
    val context = LocalContext.current

    val qrScannerLauncher = rememberLauncherForActivityResult(
        contract = io.github.g00fy2.quickie.ScanQRCode()
    ) { result ->
        when (result) {
            is io.github.g00fy2.quickie.QRResult.QRSuccess -> {
                val rawValue = result.content.rawValue
                if (rawValue != null && rawValue.startsWith("clipconnect://")) {
                    Log.d(TAG, "QR scanned: ${rawValue.length} chars")
                    onResult(rawValue)
                }
            }
            is io.github.g00fy2.quickie.QRResult.QRUserCanceled -> {}
            is io.github.g00fy2.quickie.QRResult.QRMissingPermission -> {}
            is io.github.g00fy2.quickie.QRResult.QRError -> {
                Log.e(TAG, "QR error: ${result.exception.message}")
            }
        }
    }

    var cameraPermissionGranted by remember {
        mutableStateOf(
            ContextCompat.checkSelfPermission(context, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED
        )
    }

    val cameraPermissionLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { isGranted ->
        cameraPermissionGranted = isGranted
    }

    LaunchedEffect(Unit) {
        if (!cameraPermissionGranted) {
            cameraPermissionLauncher.launch(Manifest.permission.CAMERA)
        }
    }

    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(
            modifier = Modifier
                .padding(16.dp)
                .fillMaxWidth(),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Text("Scan QR Code", style = MaterialTheme.typography.titleMedium)
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                "Point camera at laptop's pairing QR code",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(16.dp))

            if (cameraPermissionGranted) {
                Button(
                    onClick = { qrScannerLauncher.launch(null) },
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Text("Open Camera Scanner")
                }
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    "Or use the manual paste option below",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            } else {
                Card(
                    modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp),
                    colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.errorContainer)
                ) {
                    Column(modifier = Modifier.padding(16.dp)) {
                        Text("Camera permission denied", style = MaterialTheme.typography.bodyMedium)
                        Spacer(modifier = Modifier.height(8.dp))
                        Button(onClick = { cameraPermissionLauncher.launch(Manifest.permission.CAMERA) }) {
                            Text("Retry Permission")
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.height(16.dp))

            var pasteText by remember { mutableStateOf("") }
            OutlinedTextField(
                value = pasteText,
                onValueChange = { pasteText = it },
                label = { Text("Or paste pairing code here") },
                modifier = Modifier.fillMaxWidth()
            )
            Spacer(modifier = Modifier.height(8.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Button(
                    onClick = { if (pasteText.isNotBlank()) onResult(pasteText) },
                    modifier = Modifier.weight(1f)
                ) { Text("Submit") }
                OutlinedButton(
                    onClick = onDismiss,
                    modifier = Modifier.weight(1f)
                ) { Text("Cancel") }
            }
        }
    }
}
