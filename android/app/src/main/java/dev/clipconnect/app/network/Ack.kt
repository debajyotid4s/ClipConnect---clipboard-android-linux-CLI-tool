package dev.clipconnect.app.network

import android.util.Log
import org.json.JSONObject

/**
 * Send a clip with sequence number.
 * Returns the sequence number assigned to this clip, or 0 if send failed.
 */
internal fun TcpClient.sendClip(text: String): Long {
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
            unackedClip = TcpClient.UnackedClip(seq, text, System.currentTimeMillis())
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
internal fun TcpClient.sendAck(seq: Long) {
    if (seq <= 0L) return
    if (!connected.get() || s2cCodec == null) return
    try {
        val json = JSONObject().apply {
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
internal fun TcpClient.handleAck(ackSeq: Long) {
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
fun TcpClient.checkUnacked(): String? {
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
internal fun TcpClient.checkAndResendUnacked() {
    synchronized(unackedLock) {
        val clip = unackedClip ?: return
        if (System.currentTimeMillis() - clip.sentAt > ACK_TIMEOUT_MS) {
            Log.w(TAG, "Ack timeout for seq=${clip.seq}, resending")
            val json = JSONObject().apply {
                put("t", "clip")
                put("text", clip.text)
                put("seq", clip.seq)
            }
            val msg = json.toString().toByteArray(Charsets.UTF_8)
            try {
                writerQueue.add(msg)
                // Update the sent time for the retry
                unackedClip = TcpClient.UnackedClip(clip.seq, clip.text, System.currentTimeMillis())
            } catch (e: Exception) {
                Log.e(TAG, "Failed to resend clip: ${e.message}")
            }
        }
    }
}

/**
 * Clear the unacked clip on disconnect (will be retried on reconnect).
 */
fun TcpClient.clearUnacked() {
    synchronized(unackedLock) {
        unackedClip = null
    }
}
