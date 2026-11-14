package dev.clipconnect.app.bridge

import android.util.Log
import java.io.BufferedInputStream
import java.io.BufferedOutputStream
import java.io.ByteArrayOutputStream
import java.io.DataInputStream
import java.io.DataOutputStream
import java.io.IOException
import java.net.InetSocketAddress
import java.net.ServerSocket
import java.net.Socket
import java.util.concurrent.atomic.AtomicBoolean

/**
 * In-process IPC server that serves READ_CLIPBOARD requests over a local TCP
 * socket bound to 127.0.0.1. It is bound by the app itself (no ADB, no Shell
 * UID 2000, no `app_process`), so it works on any device where the app can read
 * its own clipboard through the public `ClipboardManager` API — valid for the
 * owning package on Android 10+ (API 29+).
 *
 * Using a loopback TCP socket avoids the Android quirk where AF_UNIX socket
 * files are not reliably persisted on disk; the client simply connects to the
 * same 127.0.0.1 port. Nothing clipboard-sensitive is logged; only lengths and
 * event names are logged.
 */
class ClipboardIpcServer(
    private val port: Int,
    private val reader: () -> String?
) {
    companion object {
        const val TAG = "ClipboardIpcServer"
        const val OP_READ_CLIPBOARD = "READ_CLIPBOARD"
        const val MAX_LINE_BYTES = 1_000_000
        const val READ_TIMEOUT_MS = 5_000
        const val HOST = "127.0.0.1"
    }

    private val running = AtomicBoolean(false)
    private var serverSocket: ServerSocket? = null
    private var acceptThread: Thread? = null

    /** Bind and start accepting connections. Idempotent. @return the bound port or -1 on failure. */
    fun start(): Int {
        if (!running.getAndSet(true)) {
            try {
                val server = ServerSocket().apply {
                    reuseAddress = true
                    bind(InetSocketAddress(HOST, port))
                }
                serverSocket = server
                Thread(this::acceptLoop).apply {
                    name = "clipboard-ipc"
                    start()
                }
                Log.d(TAG, "IPC server listening on $HOST:$port")
                return server.localPort
            } catch (e: Throwable) {
                Log.e(TAG, "Failed to start IPC server: ${e.message}")
                running.set(false)
                return -1
            }
        }
        return port
    }

    private fun acceptLoop() {
        val server = serverSocket ?: return
        while (running.get()) {
            try {
                val client = server.accept()
                handleClient(client)
            } catch (e: IOException) {
                if (running.get()) Log.e(TAG, "accept error: ${e.message}")
                break
            }
        }
    }

    private fun handleClient(client: Socket) {
        try {
            client.soTimeout = READ_TIMEOUT_MS
            val input = DataInputStream(BufferedInputStream(client.getInputStream()))
            val output = DataOutputStream(BufferedOutputStream(client.getOutputStream()))
            val line = readLine(input)
            if (line == null || line.isBlank()) {
                closeQuietly(client)
                return
            }
            val op = line.substringBefore(' ').trim()
            val response = if (op == OP_READ_CLIPBOARD) readResponse() else "ERR unknown-op"
            writeResponse(output, response)
        } catch (e: Throwable) {
            Log.e(TAG, "client handling error: ${e.message}")
        } finally {
            closeQuietly(client)
        }
    }

    private fun readResponse(): String {
        val text = try {
            reader()
        } catch (e: Throwable) {
            Log.e(TAG, "clipboard read failed: ${e.message}")
            null
        }
        val payload = text ?: ""
        // "OK <len> <base64>" — length-prefixed so the client never over-reads.
        val b64 = android.util.Base64.encodeToString(payload.toByteArray(Charsets.UTF_8), android.util.Base64.NO_WRAP)
        return "OK ${b64.length} $b64"
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
            if (buf.size() > MAX_LINE_BYTES) return null
            c = input.read()
        }
        val bytes = buf.toByteArray()
        if (bytes.isEmpty() && c == -1) return null
        return String(bytes, Charsets.UTF_8)
    }

    private fun writeResponse(output: DataOutputStream, response: String) {
        try {
            val data = response.toByteArray(Charsets.UTF_8)
            output.write(data)
            output.write('\n'.code)
            output.flush()
        } catch (e: IOException) {
            Log.d(TAG, "write response: ${e.message}")
        }
    }

    fun stop() {
        if (!running.getAndSet(false)) return
        closeQuietly(serverSocket)
        acceptThread?.interrupt()
        serverSocket = null
        Log.d(TAG, "IPC server stopped")
    }

    private fun closeQuietly(closeable: AutoCloseable?) {
        try {
            closeable?.close()
        } catch (e: Throwable) {
            Log.d(TAG, "close: ${e.message}")
        }
    }
}
