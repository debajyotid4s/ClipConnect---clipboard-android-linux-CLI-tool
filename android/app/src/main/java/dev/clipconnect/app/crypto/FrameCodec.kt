package dev.clipconnect.app.crypto

import java.io.DataInputStream
import java.io.DataOutputStream
import java.io.IOException
import java.nio.ByteBuffer
import java.nio.ByteOrder
import javax.crypto.Cipher

/**
 * Handles encryption/decryption of frames using AES-256-GCM.
 * Matches laptop's frame.Codec.
 *
 * Frame format: [4-byte big-endian length ‖ ciphertext‖tag]
 * No AAD used (simpler debugging).
 */
class FrameCodec(private val key: ByteArray) {

    private var counter: Long = 0

    companion object {
        private const val HEADER_SIZE = 4
        const val MAX_PLAINTEXT = 1024 * 1024 // 1 MiB
        private const val TAG_SIZE = 16
    }

    /**
     * Encrypt and encode a plaintext message into a wire frame.
     * Format: [4-byte big-endian length][ciphertext‖tag]
     */
    fun encode(plaintext: ByteArray): ByteArray {
        if (plaintext.size > MAX_PLAINTEXT) {
            throw IOException("plaintext too large: ${plaintext.size} bytes")
        }
        val cipherLen = plaintext.size + TAG_SIZE
        val result = ByteArray(HEADER_SIZE + cipherLen)
        ByteBuffer.wrap(result).order(ByteOrder.BIG_ENDIAN).putInt(cipherLen)

        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        val nonce = buildNonce(counter)
        counter++
        val spec = javax.crypto.spec.GCMParameterSpec(128, nonce)
        cipher.init(Cipher.ENCRYPT_MODE, javax.crypto.spec.SecretKeySpec(key, "AES"), spec)
        // NO AAD
        val ciphertext = cipher.doFinal(plaintext)
        System.arraycopy(ciphertext, 0, result, HEADER_SIZE, ciphertext.size)
        return result
    }

    /**
     * Decrypt and decode a received frame.
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

        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        val nonce = buildNonce(counter)
        counter++
        val spec = javax.crypto.spec.GCMParameterSpec(128, nonce)
        cipher.init(Cipher.DECRYPT_MODE, javax.crypto.spec.SecretKeySpec(key, "AES"), spec)
        // NO AAD
        return cipher.doFinal(ciphertext)
    }

    /**
     * Read one frame from input stream.
     */
    fun readFrame(input: DataInputStream): ByteArray {
        val header = ByteArray(HEADER_SIZE)
        input.readFully(header)

        val length = ByteBuffer.wrap(header).order(ByteOrder.BIG_ENDIAN).int
        if (length > MAX_PLAINTEXT + TAG_SIZE) {
            throw IOException("Frame too large: $length")
        }

        val body = ByteArray(length)
        input.readFully(body)

        val fullFrame = header + body
        return decode(fullFrame)
    }

    /**
     * Write one frame to output stream.
     */
    fun writeFrame(output: DataOutputStream, plaintext: ByteArray) {
        val frame = encode(plaintext)
        output.write(frame)
        output.flush()
    }

    private fun buildNonce(counter: Long): ByteArray {
        return ByteBuffer.allocate(12).apply {
            order(ByteOrder.BIG_ENDIAN)
            position(4)
            putLong(counter)
        }.array()
    }
}
