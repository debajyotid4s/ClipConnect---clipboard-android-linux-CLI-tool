package dev.clipconnect.app.crypto

import java.io.DataInputStream
import java.io.DataOutputStream
import java.io.IOException
import java.nio.ByteBuffer
import java.nio.ByteOrder

/**
 * Handles encryption/decryption of frames using AES-256-GCM.
 * Matches laptop's frame.Codec (no AAD, counter-based nonce).
 */
class FrameCodec(private val key: ByteArray) {

    private var counter: Long = 0

    companion object {
        private const val HEADER_SIZE = 4
        private const val MAX_PLAINTEXT = 1024 * 1024 // 1 MiB
        private const val TAG_SIZE = 16
    }

    /**
     * Encrypt and encode a plaintext message into a wire frame.
     * Format: [4-byte big-endian length][ciphertext‖tag]
     */
    fun encode(plaintext: ByteArray): ByteArray {
        val ciphertext = Crypto.encryptWithCounter(plaintext, key, counter)
        counter++

        val length = ciphertext.size
        val result = ByteArray(HEADER_SIZE + length)
        ByteBuffer.wrap(result).order(ByteOrder.BIG_ENDIAN).putInt(length)
        System.arraycopy(ciphertext, 0, result, HEADER_SIZE, length)
        return result
    }

    /**
     * Decrypt and decode a received frame.
     * buf must contain [4-byte length ‖ ciphertext‖tag].
     */
    fun decode(buf: ByteArray): ByteArray {
        if (buf.size < HEADER_SIZE) {
            throw IOException("Insufficient data for frame header")
        }

        val length = ByteBuffer.wrap(buf, 0, HEADER_SIZE).order(ByteOrder.BIG_ENDIAN).int
        if (length > MAX_PLAINTEXT + TAG_SIZE) {
            throw IOException("Frame too large: $length bytes")
        }

        if (buf.size < HEADER_SIZE + length) {
            throw IOException("Incomplete frame")
        }

        val ciphertext = buf.copyOfRange(HEADER_SIZE, HEADER_SIZE + length)
        val plaintext = Crypto.decryptWithCounter(ciphertext, key, counter)
        counter++
        return plaintext
    }

    /**
     * Read one frame from input stream.
     */
    fun readFrame(`in`: DataInputStream): ByteArray {
        val header = ByteArray(HEADER_SIZE)
        `in`.readFully(header)

        val length = ByteBuffer.wrap(header).order(ByteOrder.BIG_ENDIAN).int
        if (length > MAX_PLAINTEXT + TAG_SIZE) {
            throw IOException("Frame too large: $length")
        }

        val body = ByteArray(length)
        `in`.readFully(body)

        val fullFrame = header + body
        return decode(fullFrame)
    }

    /**
     * Write one frame to output stream.
     */
    fun writeFrame(`out`: DataOutputStream, plaintext: ByteArray) {
        val frame = encode(plaintext)
        `out`.write(frame)
        `out`.flush()
    }
}
