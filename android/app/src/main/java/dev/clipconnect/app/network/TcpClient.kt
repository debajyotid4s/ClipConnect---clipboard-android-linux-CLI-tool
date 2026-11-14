package dev.clipconnect.app.network

import dev.clipconnect.app.crypto.Crypto
import dev.clipconnect.app.crypto.FrameCodec
import android.util.Log
import org.json.JSONObject
import java.io.DataInputStream
import java.io.DataOutputStream
import java.io.IOException
import java.net.InetSocketAddress
import java.net.Socket
import java.security.SecureRandom
import java.util.concurrent.atomic.AtomicLong

private const val TAG = "TcpClient"
private const val MAGIC = "CCN1"
private const val NONCE_LEN = 16
private const val KEY_LEN = 32
private const val INFO_C2S = "clipconnect v1 c2s"
private const val INFO_S2C = "clipconnect v1 s2c"
private const val ACK_TIMEOUT_MS = 3000L

/**
 * TCP client with protocol handshake (CCN1 + HKDF) and auto-reconnect.
 * Uses raw TCP (no TLS) with application-level AES-256-GCM encryption.
 * Supports seq/ack for reliable clip delivery.
 */
class TcpClient {
    private var host: String = ""
    private var port: Int = 0
    private var secret: ByteArray? = null

    private var c2sCodec: FrameCodec? = null
    private var s2cCodec: FrameCodec? = null

    private var listener: Listener? = null

    private var socket: Socket? = null
    private var socketIn: DataInputStream? = null
    private var socketOut: DataOutputStream? = null

    private val connected = java.util.concurrent.atomic.AtomicBoolean(false)
    private val shuttingDown = java.util.concurrent.atomic.AtomicBoolean(false)

    private var readThread: Thread? = null
    private var writeThread: Thread? = null
    private var supervisorThread: Thread? = null

    private val lastPongTime = java.util.concurrent.atomic.AtomicLong(System.currentTimeMillis())
    private val writerQueue = java.util.concurrent.ConcurrentLinkedQueue<Any>()

    // Seq/ack state
    private val nextSeq = AtomicLong(1)
    private var unackedClip: UnackedClip? = null
    private val unackedLock = Any()

    data class UnackedClip(val seq: Long, val text: String, val sentAt: Long)

    fun setConfig(host: String, port: Int, secret: ByteArray) {
        this.host = host
        this.port = port
        this.secret = secret
    }

    interface Listener {
        fun onConnected()
        fun onDisconnected(reason: String)
        fun onMessage(text: String)
        fun onError(error: String)
    }

    fun setListener(listener: Listener) {
        this.listener = listener
    }

    fun start() {
        if (shuttingDown.get()) return
        Log.d(TAG, "Starting supervisor loop")
        supervisorThread = Thread({
            val backoff = ReconnectBackoff()
            while (!shuttingDown.get()) {
                try {
                    connectAndRun()
                    backoff.reset()
                } catch (e: Exception) {
                    Log.e(TAG, "Supervisor error: ${e.message}", e)
                }
                if (!shuttingDown.get() && !connected.get()) {
                    val delay = backoff.fail()
                    Log.d(TAG, "Reconnecting in ${delay}ms")
                    Thread.sleep(delay)
                }
            }
        }).apply { start() }
    }

    private fun connectAndRun() {
        val targetHost = host
        val targetPort = port
        val targetSecret = secret ?: run {
            Log.w(TAG, "No secret configured")
            Thread.sleep(1000)
            return
        }

        // Save unacked clip before reconnecting (will be retried)
        val pendingClip: UnackedClip? = synchronized(unackedLock) {
            unackedClip?.takeIf { System.currentTimeMillis() - it.sentAt < 10000 }
        }

        Log.d(TAG, "Connecting to $targetHost:$targetPort")
        closeSocket()
        if (!NetValidation.isValidAddress(targetHost) || !NetValidation.isValidPort(targetPort)) {
            Log.w(TAG, "Rejected invalid connect target $targetHost:$targetPort")
            Thread.sleep(1000)
            return
        }
        writerQueue.clear()
        socketIn = null
        socketOut = null

        // Raw TCP connection (no TLS)
        socket = Socket().apply {
            soTimeout = 30000
            connect(InetSocketAddress(targetHost, targetPort), 10000)
        }
        socketIn = DataInputStream(socket!!.getInputStream())
        socketOut = DataOutputStream(socket!!.getOutputStream())

        // Protocol handshake: CCN1 + nonces + HKDF
        performHandshake(targetSecret)

        connected.set(true)
        lastPongTime.set(System.currentTimeMillis())
        Log.d(TAG, "Handshake complete, connected to $targetHost:$targetPort")

        // Restore pending clip if it survived the reconnect
        if (pendingClip != null) {
            synchronized(unackedLock) {
                unackedClip = pendingClip
            }
            Log.d(TAG, "Restored pending clip seq=${pendingClip.seq} for resend")
            // Queue it for immediate send
            val json = org.json.JSONObject().apply {
                put("t", "clip")
                put("text", pendingClip.text)
                put("seq", pendingClip.seq)
            }
            writerQueue.add(json.toString().toByteArray(Charsets.UTF_8))
        }

        listener?.onConnected()

        startReadLoop()
        startWriteLoop()

        // Main loop - wait for disconnect or heartbeat timeout
        while (connected.get() && !shuttingDown.get()) {
            Thread.sleep(1000)
            if (System.currentTimeMillis() - lastPongTime.get() > 40000) {
                Log.d(TAG, "Heartbeat timeout, reconnecting")
                break
            }
            // Check for unacked clips that need resending
            checkAndResendUnacked()
        }

        connected.set(false)
        Log.d(TAG, "Disconnected")
        closeSocket()
        readThread?.interrupt()
        writeThread?.interrupt()
        listener?.onDisconnected("Connection lost")
    }

    /**
     * Perform the CCN1 protocol handshake:
     * 1. Client sends CCN1 + 16-byte client nonce
     * 2. Server sends 16-byte server nonce
     * 3. Both sides derive c2s/s2c keys via HKDF-SHA256
     * 4. Exchange hello frames
     */
    private fun performHandshake(secret: ByteArray) {
        val out = socketOut!!
        val input = socketIn!!

        // Generate client nonce
        val clientNonce = ByteArray(NONCE_LEN)
        SecureRandom.getInstanceStrong().nextBytes(clientNonce)

        // Step 1: Send CCN1 + clientNonce
        out.write(MAGIC.toByteArray(Charsets.US_ASCII))
        out.write(clientNonce)
        out.flush()
        Log.d(TAG, "Sent CCN1 + client nonce")

        // Step 2: Read server nonce
        val serverNonce = ByteArray(NONCE_LEN)
        input.readFully(serverNonce)
        Log.d(TAG, "Received server nonce")

        // Step 3: Derive keys using HKDF-SHA256
        val salt = clientNonce + serverNonce
        val keyC2S = Crypto.hkdfSha256(secret, salt, INFO_C2S, KEY_LEN)
        val keyS2C = Crypto.hkdfSha256(secret, salt, INFO_S2C, KEY_LEN)

        c2sCodec = FrameCodec(keyC2S)
        s2cCodec = FrameCodec(keyS2C)

        // Step 4: Exchange hello frames
        val hello = """{"t":"hello","v":1}""".toByteArray(Charsets.UTF_8)
        c2sCodec!!.writeFrame(out, hello)
        out.flush()
        Log.d(TAG, "Sent client hello")

        val serverHello = s2cCodec!!.readFrame(input)
        val helloResponse = String(serverHello, Charsets.UTF_8)
        Log.d(TAG, "Server hello: $helloResponse")
        if (helloResponse != """{"t":"hello","v":1}""") {
            throw IOException("Invalid hello response: $helloResponse")
        }
    }

    private fun startReadLoop() {
        readThread = Thread({
            val input = socketIn!!
            Log.d(TAG, "Read loop started")
            while (connected.get() && !socket!!.isClosed && !Thread.currentThread().isInterrupted) {
                try {
                    val frame = s2cCodec!!.readFrame(input)
                    val msg = String(frame, Charsets.UTF_8)
                    val json = JSONObject(msg)
                    Log.d(TAG, "Received: ${json.optString("t")}")
                    when (json.optString("t")) {
                        "clip" -> {
                            val text = json.optString("text", "")
                            val seq = json.optLong("seq", 0L)
                            listener?.onMessage(text)
                            sendAck(seq)
                        }
                        "ack" -> {
                            val ackSeq = json.optLong("seq", 0)
                            handleAck(ackSeq)
                        }
                        "pong" -> {
                            lastPongTime.set(System.currentTimeMillis())
                        }
                        "ping" -> {
                            val pong = """{"t":"pong"}""".toByteArray(Charsets.UTF_8)
                            writerQueue.add(pong)
                            lastPongTime.set(System.currentTimeMillis())
                        }
                    }
                } catch (e: Exception) {
                    if (connected.get() && !shuttingDown.get()) {
                        Log.e(TAG, "Read error: ${e.message}", e)
                        break
                    }
                }
            }
            Log.d(TAG, "Read loop exited")
        }).apply { start() }
    }

    private fun startWriteLoop() {
        writeThread = Thread({
            val out = socketOut!!
            Log.d(TAG, "Write loop started")
            while (connected.get() && !socket!!.isClosed && !Thread.currentThread().isInterrupted) {
                try {
                    val item = writerQueue.poll()
                    if (item != null) {
                        val data = item as ByteArray
                        c2sCodec!!.writeFrame(out, data)
                        out.flush()
                        Log.d(TAG, "Sent frame: ${data.size} bytes")
                    } else {
                        Thread.sleep(50)
                    }
                } catch (e: Exception) {
                    if (connected.get() && !shuttingDown.get()) {
                        Log.e(TAG, "Write error: ${e.message}", e)
                        break
                    }
                }
            }
            Log.d(TAG, "Write loop exited")
        }).apply { start() }
    }

    /**
     * Send a clip with sequence number.
     * Returns the sequence number assigned to this clip, or 0 if send failed.
     */
    fun sendClip(text: String): Long {
        if (!connected.get() || c2sCodec == null) return 0L
        try {
            val seq = nextSeq.getAndIncrement()
            val json = JSONObject().apply {
                put("t", "clip")
                put("text", text)
                put("seq", seq)
            }
            val msg = json.toString().toByteArray(Charsets.UTF_8)
            writerQueue.add(msg)
            Log.d(TAG, "Queued clip seq=$seq: ${text.length} chars")

            // Track unacked clip
            synchronized(unackedLock) {
                unackedClip = UnackedClip(seq, text, System.currentTimeMillis())
            }

            return seq
        } catch (e: Exception) {
            Log.d(TAG, "Send error: ${e.message}")
            connected.set(false)
            listener?.onError("Send failed: ${e.message}")
            return 0L
        }
    }

    /**
     * Send an ack for a received clip so the sender's unacked state clears.
     * The laptop acks any seq > 0, including duplicates.
     */
    private fun sendAck(seq: Long) {
        if (seq <= 0L) return
        if (!connected.get() || s2cCodec == null) return
        try {
            val json = org.json.JSONObject().apply {
                put("t", "ack")
                put("seq", seq)
            }
            val msg = json.toString().toByteArray(Charsets.UTF_8)
            writerQueue.add(msg)
        } catch (e: Exception) {
            Log.e(TAG, "Ack send error: ${e.message}")
        }
    }

    /**
     * Handle incoming ack from the receiver.
     * Clears the unacked clip if the ack matches the expected sequence.
     */
    private fun handleAck(ackSeq: Long) {
        Log.d(TAG, "Received ack for seq=$ackSeq")
        synchronized(unackedLock) {
            if (ackSeq > 0 && unackedClip?.seq == ackSeq) {
                unackedClip = null
                Log.d(TAG, "Ack received for seq=$ackSeq, cleared unacked")
            }
        }
    }

    /**
     * Check for unacked clips that have timed out.
     * Called periodically (e.g., from supervisor loop) to resend stale clips.
     * Returns the latest unacked clip text, or null if none.
     */
    fun checkUnacked(): String? {
        synchronized(unackedLock) {
            val clip = unackedClip ?: return null
            if (System.currentTimeMillis() - clip.sentAt > ACK_TIMEOUT_MS) {
                Log.w(TAG, "Ack timeout for seq=${clip.seq}, marking for resend")
                return clip.text
            }
            return null
        }
    }

    /**
     * Check for unacked clips that have timed out and resend them.
     * Called periodically from the supervisor loop.
     */
    private fun checkAndResendUnacked() {
        synchronized(unackedLock) {
            val clip = unackedClip ?: return
            if (System.currentTimeMillis() - clip.sentAt > ACK_TIMEOUT_MS) {
                Log.w(TAG, "Ack timeout for seq=${clip.seq}, resending")
                val json = org.json.JSONObject().apply {
                    put("t", "clip")
                    put("text", clip.text)
                    put("seq", clip.seq)
                }
                val msg = json.toString().toByteArray(Charsets.UTF_8)
                try {
                    writerQueue.add(msg)
                    // Update the sent time for the retry
                    unackedClip = UnackedClip(clip.seq, clip.text, System.currentTimeMillis())
                } catch (e: Exception) {
                    Log.e(TAG, "Failed to resend clip: ${e.message}")
                }
            }
        }
    }

    /**
     * Clear the unacked clip on disconnect (will be retried on reconnect).
     */
    fun clearUnacked() {
        synchronized(unackedLock) {
            unackedClip = null
        }
    }

    fun disconnect() {
        shuttingDown.set(true)
        connected.set(false)
        readThread?.interrupt()
        writeThread?.interrupt()
        supervisorThread?.interrupt()
        closeSocket()
        c2sCodec = null
        s2cCodec = null
    }

    private fun closeSocket() {
        try {
            socket?.close()
        } catch (e: Exception) {
            Log.d(TAG, "Close socket error: ${e.message}")
        }
        socket = null
        socketIn = null
        socketOut = null
    }

    val isConnected: Boolean get() = connected.get()
}
