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
import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import javax.net.ssl.SSLContext
import javax.net.ssl.SSLSocket
import javax.net.ssl.SSLSocketFactory
import javax.net.ssl.X509TrustManager
import java.util.concurrent.ConcurrentLinkedQueue
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference

class TcpClient(private var _host: String, private var _port: Int, private var _secret: ByteArray) {
    fun setConfig(host: String, port: Int, secret: ByteArray) {
        _host = host
        _port = port
        _secret = secret
    }

    interface Listener {
        fun onConnected()
        fun onDisconnected(reason: String)
        fun onMessage(text: String)
        fun onError(error: String)
    }

    private var socket: SSLSocket? = null
    private var c2sCodec: FrameCodec? = null
    private var s2cCodec: FrameCodec? = null
    private var listener: Listener? = null
    private var readThread: Thread? = null
    private var writeThread: Thread? = null
    private var supervisorThread: Thread? = null

    private val connected = AtomicBoolean(false)
    private val shuttingDown = AtomicBoolean(false)
    private val writerQueue = ConcurrentLinkedQueue<Any>()
    private val lastPongTime = AtomicReference(System.currentTimeMillis())
    private var socketIn: DataInputStream? = null
    private var socketOut: DataOutputStream? = null

    fun setListener(listener: Listener) {
        this.listener = listener
    }

    fun start() {
        if (shuttingDown.get()) return
        Log.d("TcpClient", "Starting supervisor loop")
        supervisorThread = Thread {
            var delay = 1000L
            val maxDelay = 30000L
            while (!shuttingDown.get()) {
                try {
                    connectAndRun()
                    delay = 1000L
                } catch (e: Exception) {
                    Log.e("TcpClient", "Supervisor error: ${e.message}", e)
                }
                if (!shuttingDown.get() && !connected.get()) {
                    Log.d("TcpClient", "Reconnecting in ${delay}ms")
                    Thread.sleep(delay)
                    delay = minOf(delay * 2, maxDelay)
                }
            }
        }
        supervisorThread?.start()
    }

    private fun connectAndRun() {
        val host = _host
        val port = _port
        val secret = _secret
        Log.d("TcpClient", "Connecting to $host:$port with TLS")

        closeSocket()
        writerQueue.clear()
        socketIn = null
        socketOut = null

        val sslFactory = insecureSslSocketFactory
        socket = sslFactory.createSocket(host, port) as SSLSocket
        socket!!.soTimeout = 30000 // 30s read timeout
        socket!!.startHandshake()
        Log.d("TcpClient", "TLS handshake complete")

        // Get streams BEFORE handshake (they may be refreshed)
        socketIn = DataInputStream(socket!!.getInputStream())
        socketOut = DataOutputStream(socket!!.getOutputStream())

        performAppHandshake(secret)

        connected.set(true)
        lastPongTime.set(System.currentTimeMillis())
        Log.d("TcpClient", "Connected to $host:$port")
        listener?.onConnected()

        startReadLoop()
        startWriteLoop()

        // Main loop - wait for disconnect
        while (connected.get() && !shuttingDown.get()) {
            Thread.sleep(1000)
            if (System.currentTimeMillis() - lastPongTime.get() > 25000) {
                Log.d("TcpClient", "Heartbeat timeout, reconnecting")
                break
            }
        }

        connected.set(false)
        Log.d("TcpClient", "Disconnected")
        closeSocket()
        readThread?.interrupt()
        writeThread?.interrupt()
        listener?.onDisconnected("Connection lost")
    }

    private fun performAppHandshake(secret: ByteArray) {
        val out = socketOut!!
        val `in` = socketIn!!

        val key = Crypto.deriveKey(secret)
        c2sCodec = FrameCodec(key)
        s2cCodec = FrameCodec(key)

        val hello = """{"t":"hello","v":1}""".toByteArray(Charsets.UTF_8)
        c2sCodec!!.writeFrame(out, hello)
        out.flush()
        Log.d("TcpClient", "Sent client hello")

        val s2cResponse = s2cCodec!!.readFrame(`in`)
        val response = String(s2cResponse, Charsets.UTF_8)
        Log.d("TcpClient", "Server hello: $response")
        if (response != """{"t":"hello","v":1}""") {
            throw IOException("Invalid hello response: $response")
        }
    }

    private fun startReadLoop() {
        readThread = Thread {
            val `in` = socketIn!!
            Log.d("TcpClient", "Read loop started")
            while (connected.get() && !socket!!.isClosed && !Thread.currentThread().isInterrupted) {
                try {
                    val frame = s2cCodec!!.readFrame(`in`)
                    val msg = String(frame, Charsets.UTF_8)
                    val json = JSONObject(msg)
                    Log.d("TcpClient", "Received: ${json.optString("t")}")
                    when (json.optString("t")) {
                        "clip" -> {
                            val text = json.optString("text", "")
                            listener?.onMessage(text)
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
                        Log.e("TcpClient", "Read error: ${e.message}", e)
                        break
                    }
                }
            }
            Log.d("TcpClient", "Read loop exited")
        }
        readThread?.start()
    }

    private fun startWriteLoop() {
        writeThread = Thread {
            val out = socketOut!!
            Log.d("TcpClient", "Write loop started")
            while (connected.get() && !socket!!.isClosed && !Thread.currentThread().isInterrupted) {
                try {
                    val item = writerQueue.poll()
                    if (item != null) {
                        val data = item as ByteArray
                        c2sCodec!!.writeFrame(out, data)
                        out.flush()
                        Log.d("TcpClient", "Sent frame: ${data.size} bytes")
                    } else {
                        Thread.sleep(50)
                    }
                } catch (e: Exception) {
                    if (connected.get() && !shuttingDown.get()) {
                        Log.e("TcpClient", "Write error: ${e.message}", e)
                        break
                    }
                }
            }
            Log.d("TcpClient", "Write loop exited")
        }
        writeThread?.start()
    }

    fun sendClip(text: String) {
        if (!connected.get()) return
        try {
            val escapedText = text.replace("\\", "\\\\")
                .replace("\"", "\\\"")
                .replace("\n", "\\n")
                .replace("\r", "\\r")
                .replace("\t", "\\t")
            val msg = """{"t":"clip","text":"${escapedText}"}""".toByteArray(Charsets.UTF_8)
            writerQueue.add(msg)
            Log.d("TcpClient", "Queued clip: ${text.length} chars")
        } catch (e: Exception) {
            Log.d("TcpClient", "Send error: ${e.message}")
            connected.set(false)
            listener?.onError("Send failed: ${e.message}")
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
            Log.d("TcpClient", "Close socket error: ${e.message}")
        }
        socket = null
        socketIn = null
        socketOut = null
    }

    val isConnected: Boolean get() = connected.get()

    companion object {
        private val insecureSslSocketFactory: SSLSocketFactory by lazy {
            val sslContext = SSLContext.getInstance("TLS").apply {
                val trustAll = object : X509TrustManager {
                    override fun checkClientTrusted(chain: Array<out X509Certificate>, authType: String) {}
                    override fun checkServerTrusted(chain: Array<out X509Certificate>, authType: String) {}
                    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
                }
                init(null, arrayOf(trustAll), SecureRandom())
            }
            sslContext.socketFactory
        }
    }
}
