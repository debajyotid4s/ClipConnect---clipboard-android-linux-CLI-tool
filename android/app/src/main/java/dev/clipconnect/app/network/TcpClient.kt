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

internal const val TAG = "TcpClient"
internal const val MAGIC = "CCN1"
internal const val NONCE_LEN = 16
internal const val KEY_LEN = 32
internal const val INFO_C2S = "clipconnect v1 c2s"
internal const val INFO_S2C = "clipconnect v1 s2c"
internal const val ACK_TIMEOUT_MS = 3000L

/**
 * TCP client with protocol handshake (CCN1 + HKDF) and auto-reconnect.
 * Uses raw TCP (no TLS) with application-level AES-256-GCM encryption.
 * Supports seq/ack for reliable clip delivery.
 */
class TcpClient {
    internal var host: String = ""
    internal var port: Int = 0
    internal var secret: ByteArray? = null

    internal var c2sCodec: FrameCodec? = null
    internal var s2cCodec: FrameCodec? = null

    internal var listener: Listener? = null

    // Stable per-install identity sent in the hello frame ("pid"). The laptop
    // uses it only as a legacy hint; the authoritative outbox key is the
    // per-connection id ("cid") the server mints. Empty for clients that predate
    // this field.
    internal var phoneId: String = ""

    // Per-connection id minted by the server and echoed back in our hello. The
    // laptop keys its durable outbox on this, so a fresh id per connection
    // prevents a reconnected phone from claiming another phone's queued clips.
    @Volatile
    internal var connectionId: String = ""

    /** The per-connection id the server assigned to this connection. */
    val connId: String get() = connectionId

    internal var socket: Socket? = null
    internal var socketIn: DataInputStream? = null
    internal var socketOut: DataOutputStream? = null

    internal val connected = java.util.concurrent.atomic.AtomicBoolean(false)
    internal val shuttingDown = java.util.concurrent.atomic.AtomicBoolean(false)

    internal var readThread: Thread? = null
    internal var writeThread: Thread? = null
    internal var supervisorThread: Thread? = null

    internal val lastPongTime = java.util.concurrent.atomic.AtomicLong(System.currentTimeMillis())
    internal val writerQueue = java.util.concurrent.ConcurrentLinkedQueue<Any>()

    // Seq/ack state
    internal val nextSeq = AtomicLong(1)
    internal var unackedClip: UnackedClip? = null
    internal val unackedLock = Any()

    internal data class UnackedClip(val seq: Long, val text: String, val sentAt: Long)

    fun setConfig(host: String, port: Int, secret: ByteArray) {
        this.host = host
        this.port = port
        this.secret = secret
    }

    /** Set the stable per-install identity sent in the hello frame. */
    fun setPhoneId(id: String) {
        this.phoneId = id
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
        pendingClip?.let { pending ->
            synchronized(unackedLock) {
                unackedClip = pending
            }
            Log.d(TAG, "Restored pending clip seq=${pending.seq} for resend")
            val json = JSONObject().apply {
                put("t", "clip")
                put("text", pending.text)
                put("seq", pending.seq)
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
