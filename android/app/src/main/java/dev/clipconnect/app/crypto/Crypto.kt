package dev.clipconnect.app.crypto

import java.nio.ByteBuffer
import java.nio.ByteOrder
import java.security.MessageDigest
import java.security.SecureRandom
import java.security.Key as SecurityKey
import javax.crypto.Cipher
import javax.crypto.Mac
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * ClipboardConnect crypto: SHA-256 key derivation, HKDF, AES-256-GCM.
 */
object Crypto {

    private const val KEY_LEN = 32
    private const val NONCE_LEN = 12
    private const val HKDF_BLOCK_LEN = 32 // SHA-256 block size in bytes

    /**
     * Derive a 32-byte AES-GCM key from the pairing secret using SHA-256.
     * Matches laptop's crypto.DeriveKey().
     */
    fun deriveKey(secret: ByteArray): ByteArray {
        val digest = MessageDigest.getInstance("SHA-256")
        return digest.digest(secret)
    }

    /**
     * HKDF using HMAC-SHA256. Matches Go's hkdf.Key(sha256.New, ikm, salt, infoString, keyLen).
     */
    fun hkdfSha256(ikm: ByteArray, salt: ByteArray, info: String, keyLen: Int = KEY_LEN): ByteArray {
        // HKDF-Extract: PRK = HMAC-Hash(salt, ikm)
        val prk = hmacSha256(salt, ikm)
        // HKDF-Expand
        val n = (keyLen + 31) / 32 // number of blocks
        val okm = ByteArray(keyLen)
        var block = ByteArray(0)
        for (i in 0 until n) {
            val input = block + info.toByteArray(Charsets.UTF_8) + byteArrayOf((i + 1).toByte())
            block = hmacSha256(prk, input)
            System.arraycopy(block, 0, okm, i * 32, minOf(32, keyLen - i * 32))
        }
        return okm
    }

    private fun hmacSha256(key: ByteArray, data: ByteArray): ByteArray {
        val mac = Mac.getInstance("HmacSHA256")
        val spec = SecretKeySpec(key, "HmacSHA256")
        mac.init(spec)
        return mac.doFinal(data)
    }

    /**
     * Compute the SHA-256 hex digest of a string.
     */
    fun sha256(text: String): String {
        val digest = MessageDigest.getInstance("SHA-256")
        val bytes = digest.digest(text.toByteArray(Charsets.UTF_8))
        return bytes.joinToString("") { "%02x".format(it) }
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
