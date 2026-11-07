package dev.clipconnect.app.crypto

import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.security.SecureRandom
import java.security.MessageDigest
import javax.crypto.Cipher
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * Simplified crypto: SHA-256 key derivation + AES-256-GCM.
 */
object Crypto {

    private const val KEY_LEN = 32
    private const val NONCE_LEN = 12

    /**
     * Derive a 32-byte AES-GCM key from the pairing secret using SHA-256.
     * Matches laptop's crypto.DeriveKey().
     */
    fun deriveKey(secret: ByteArray): ByteArray {
        val digest = MessageDigest.getInstance("SHA-256")
        return digest.digest(secret)
    }

    /**
     * Create an AES-256-GCM cipher with a zero-initialized counter nonce.
     */
    fun newAesGcm(key: ByteArray, decrypt: Boolean): Cipher {
        val iv = ByteArray(NONCE_LEN) // all zeros
        val spec = GCMParameterSpec(128, iv)
        return Cipher.getInstance("AES/GCM/NoPadding").apply {
            init(
                if (decrypt) Cipher.DECRYPT_MODE else Cipher.ENCRYPT_MODE,
                SecretKeySpec(key, "AES"),
                spec
            )
        }
    }

    /**
     * Encrypt data with AES-GCM and counter-based nonce.
     */
    fun encryptWithCounter(plaintext: ByteArray, key: ByteArray, counter: Long): ByteArray {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        val iv = buildNonce(counter)
        val spec = GCMParameterSpec(128, iv)
        cipher.init(Cipher.ENCRYPT_MODE, SecretKeySpec(key, "AES"), spec)
        return cipher.doFinal(plaintext)
    }

    /**
     * Decrypt data with AES-GCM and counter-based nonce.
     */
    fun decryptWithCounter(ciphertext: ByteArray, key: ByteArray, counter: Long): ByteArray {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        val iv = buildNonce(counter)
        val spec = GCMParameterSpec(128, iv)
        cipher.init(Cipher.DECRYPT_MODE, SecretKeySpec(key, "AES"), spec)
        return cipher.doFinal(ciphertext)
    }

    /**
     * Build 12-byte nonce: 4 zero bytes + 8-byte big-endian counter.
     * Must match laptop's nonce layout: [0x00, 0x00, 0x00, 0x00, counter(8 bytes)]
     */
    private fun buildNonce(counter: Long): ByteArray {
        return ByteBuffer.allocate(NONCE_LEN).apply {
            order(ByteOrder.BIG_ENDIAN)
            position(4)
            putLong(counter)
        }.array()
    }

    /**
     * Generate random bytes.
     */
    fun randomBytes(size: Int): ByteArray {
        val bytes = ByteArray(size)
        SecureRandom.getInstanceStrong().nextBytes(bytes)
        return bytes
    }
}
