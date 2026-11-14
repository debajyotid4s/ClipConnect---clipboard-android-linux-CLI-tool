package dev.clipconnect.app.bridge

import android.util.Log
import java.io.BufferedInputStream
import java.io.BufferedOutputStream
import java.io.ByteArrayOutputStream
import java.io.DataInputStream
import java.io.DataOutputStream
import java.io.IOException
import java.net.InetSocketAddress
import java.net.Socket
import java.util.Base64

/**
 * App-side client for the in-process [ClipboardIpcServer]. Connects to the
 * loopback TCP socket, sends a READ_CLIPBOARD request, and returns the current
 * clipboard text (or null if the server is unreachable). Bounded reads protect
 * against oversized messages. Clipboard plaintext is returned to the caller
 * only and is never written to any log.
 */
class ClipboardIpcClient(private val port: Int) {
    private fun connectSocket(client: Socket) {
        client.connect(InetSocketAddress(ClipboardIpcServer.HOST, port), READ_TIMEOUT_MS)
    }

    /**
     * Read the current clipboard text.
     * @return the text, or null if the clipboard is empty or the server is unreachable.
     */
    fun readClipboard(): String? {
        val client = Socket()
        try {
            connectSocket(client)
            client.soTimeout = READ_TIMEOUT_MS
            val out = DataOutputStream(BufferedOutputStream(client.getOutputStream()))
            val `in` = DataInputStream(BufferedInputStream(client.getInputStream()))
            out.write((ClipboardIpcServer.OP_READ_CLIPBOARD + "\n").toByteArray(Charsets.UTF_8))
            out.flush()
            val responseLine = readLine(`in`) ?: return null
            return parseClipboard(responseLine)
        } catch (e: Throwable) {
            Log.d(TAG, "read failed: ${e.message}")
            return null
        } finally {
            closeQuietly(client)
        }
    }

    private fun parseClipboard(response: String): String? {
        val parts = response.split(' ', limit = 3)
        if (parts.size < 3 || parts[0] != "OK") return null
        val len = parts[1].toIntOrNull() ?: return null
        if (parts[2].length < len) return null
        return try {
            String(Base64.getDecoder().decode(parts[2].substring(0, len)), Charsets.UTF_8)
        } catch (e: IllegalArgumentException) {
            null
        }
    }

    private fun readLine(input: DataInputStream): String? {
        val buf = ByteArrayOutputStream()
        var c = try {
            input.read()
        } catch (e: IOException) {
            return null
        }
        while (c != -1 && c != '\n'.code) {
            buf.write(c)
            if (buf.size() > MAX_RESPONSE_BYTES) return null
            c = input.read()
        }
        val bytes = buf.toByteArray()
        if (bytes.isEmpty() && c == -1) return null
        return String(bytes, Charsets.UTF_8)
    }

    private fun closeQuietly(closeable: AutoCloseable?) {
        try {
            closeable?.close()
        } catch (e: Throwable) {
            Log.d(TAG, "close: ${e.message}")
        }
    }

    companion object {
        const val TAG = "ClipboardIpcClient"
        const val MAX_RESPONSE_BYTES = 1_000_000
        const val READ_TIMEOUT_MS = 5_000
    }
}
