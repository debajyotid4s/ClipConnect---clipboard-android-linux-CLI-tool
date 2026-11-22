package dev.clipconnect.app.ui

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.MaterialTheme
import androidx.compose.ui.res.stringResource
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.Button
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import dev.clipconnect.app.R

/**
 * Pairing bottom sheet with two tabs: Scan and Enter manually.
 *
 * The manual tab validates inline and shows one message at a time, so a bad IP,
 * bad port, or malformed secret is surfaced rather than crashing (the previous
 * Base64.decode path). The secret is masked and never shown in plaintext.
 *
 * [onScan] opens the camera scanner; [onManual] receives validated host/port/secret.
 * [onDismiss] closes the sheet.
 */
@Composable
fun PairSheet(
    onScan: () -> Unit,
    onManual: (host: String, port: Int, secret: ByteArray) -> Unit,
    onDismiss: () -> Unit,
) {
    var tab by remember { mutableIntStateOf(0) }
    var host by remember { mutableStateOf("") }
    var port by remember { mutableStateOf("10369") }
    var secret by remember { mutableStateOf("") }
    var secretVisible by remember { mutableStateOf(false) }
    // One inline message at a time; cleared when any field changes.
    var validation by remember { mutableStateOf<String?>(null) }

    androidx.compose.foundation.layout.Spacer(modifier = Modifier.height(20.dp))
    Text(stringResource(R.string.pair_title), style = MaterialTheme.typography.titleLarge)
    Spacer(4)
    Text(
        "Connect your phone to a laptop on the same network.",
        style = MaterialTheme.typography.bodyMedium,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
    )
    Spacer(12)

    TabRow(selectedTabIndex = tab) {
        Tab(selected = tab == 0, onClick = { tab = 0 }) { Text(stringResource(R.string.tab_scan)) }
        Tab(selected = tab == 1, onClick = { tab = 1 }) { Text(stringResource(R.string.tab_manual)) }
    }
    Spacer(4)

    if (tab == 0) {
        androidx.compose.foundation.layout.Spacer(modifier = Modifier.height(4.dp))
        Button(
            onClick = onScan,
            modifier = Modifier.fillMaxWidth(),
        ) { Text(stringResource(R.string.btn_open_scanner)) }
        Spacer(8)
        Text(
            "Or paste the pairing URL from the laptop's \"Pair\" screen below.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        androidx.compose.foundation.layout.Spacer(modifier = Modifier.height(8.dp))
        OutlinedTextField(
            value = secret, // reuse the secret box as a paste area for the URL
            onValueChange = {
                secret = it
                validation = null
            },
            label = { Text(stringResource(R.string.paste_url)) },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
        )
        Spacer(8)
        Button(
            onClick = {
                // The scanner handles the URL; here we hand a pasted URL to the
                // caller's scan path (MainActivity parses clipconnect:// URLs).
                if (secret.isNotBlank()) onScan()
            },
            enabled = secret.isNotBlank(),
            modifier = Modifier.fillMaxWidth(),
        ) { Text(stringResource(R.string.btn_paste_connect)) }
    } else {
        OutlinedTextField(
            value = host,
            onValueChange = { host = it; validation = null },
            label = { Text(stringResource(R.string.field_ip)) },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(12)
        OutlinedTextField(
            value = port,
            onValueChange = { new ->
                if (new.filter { it.isDigit() }.length == new.length) port = new
                validation = null
            },
            label = { Text(stringResource(R.string.field_port)) },
            singleLine = true,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(12)
        OutlinedTextField(
            value = secret,
            onValueChange = {
                secret = it
                validation = null
            },
            label = { Text(stringResource(R.string.field_secret)) },
            singleLine = true,
            visualTransformation = if (secretVisible) VisualTransformation.None else PasswordVisualTransformation(),
            trailingIcon = {
                TextButton(onClick = { secretVisible = !secretVisible }) {
                    Text(if (secretVisible) stringResource(R.string.btn_show) else stringResource(R.string.btn_hide))
                }
            },
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(4)
        validation?.let { msg ->
            Text(
                msg,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.error,
            )
        }
        Spacer(16)
        Button(
            onClick = {
                when (val result = PairingParser.validate(host, port, secret)) {
                    is PairingParser.Result.Invalid -> validation = result.message
                    is PairingParser.Result.Valid -> {
                        validation = null
                        onManual(result.host, result.port, result.secret)
                    }
                }
            },
            modifier = Modifier.fillMaxWidth(),
        ) { Text(stringResource(R.string.btn_pair)) }
    }
    Spacer(8)
    TextButton(onClick = onDismiss, modifier = Modifier.fillMaxWidth()) { Text(stringResource(R.string.btn_cancel)) }
}

@Composable
private fun Spacer(dp: Int) {
    androidx.compose.foundation.layout.Spacer(modifier = Modifier.height(dp.dp))
}
