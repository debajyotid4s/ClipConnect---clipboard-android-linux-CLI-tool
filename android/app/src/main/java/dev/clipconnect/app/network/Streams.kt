package dev.clipconnect.app.network

import android.util.Log
import org.json.JSONObject

/** Start the background thread that reads and dispatches inbound frames. */
internal fun TcpClient.startReadLoop() {
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

/** Start the background thread that serializes outbound frames to the socket. */
internal fun TcpClient.startWriteLoop() {
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
