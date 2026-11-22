package dev.clipconnect.app.crypto

/**
 * ClipboardConnect crypto: SHA-256 key derivation, HKDF, AES-256-GCM.
 *
 * The public surface here matches the laptop's crypto package so both sides
 * derive identical keys and frames. Internals (HMAC, nonce layout) live in
 * CryptoPrimitives so each file stays focused.
 */
object Crypto {

    internal const val KEY_LEN = 32
    internal const val NONCE_LEN = 12

    /**
     * Derive a 32-byte AES-GCM key from the pairing secret using SHA-256.
     * Matches laptop's crypto.DeriveKey().
     */
    fun deriveKey(secret: ByteArray): ByteArray {
        val digest = java.security.MessageDigest.getInstance("SHA-256")
        return digest.digest(secret)
    }

    /**
     * HKDF using HMAC-SHA256. Matches Go's hkdf.Key(sha256.New, ikm, salt, infoString, keyLen).
     */
    fun hkdfSha256(ikm: ByteArray, salt: ByteArray, info: String, keyLen: Int = KEY_LEN): ByteArray {
        val prk = hmacSha256(salt, ikm)
        return expand(prk, info, keyLen)
    }

    /**
     * First 4 bytes of HMAC-SHA256(secret, "mdns-id"), hex-encoded.
     * Advertised in the mDNS TXT record; the secret itself is never advertised.
     * Matches the laptop mdns.mdnsId.
     */
    fun mdnsId(secret: ByteArray): String {
        val digest = hmacSha256(secret, "mdns-id".toByteArray(Charsets.UTF_8))
        return digest.take(4).joinToString("") { "%02x".format(it) }
    }

    /**
     * Compute the SHA-256 hex digest of a string.
     */
    fun sha256(text: String): String {
        val digest = java.security.MessageDigest.getInstance("SHA-256")
        val bytes = digest.digest(text.toByteArray(Charsets.UTF_8))
        return bytes.joinToString("") { "%02x".format(it) }
    }

    /**
     * Create an AES-256-GCM cipher with a zero-initialized counter nonce.
     */
    fun newAesGcm(key: ByteArray, decrypt: Boolean): javax.crypto.Cipher {
        val iv = ByteArray(NONCE_LEN) // all zeros
        val spec = javax.crypto.spec.GCMParameterSpec(128, iv)
        return javax.crypto.Cipher.getInstance("AES/GCM/NoPadding").apply {
            init(
                if (decrypt) javax.crypto.Cipher.DECRYPT_MODE else javax.crypto.Cipher.ENCRYPT_MODE,
                javax.crypto.spec.SecretKeySpec(key, "AES"),
                spec
            )
        }
    }

    /**
     * Encrypt data with AES-GCM and counter-based nonce.
     */
    fun encryptWithCounter(plaintext: ByteArray, key: ByteArray, counter: Long): ByteArray {
        val cipher = javax.crypto.Cipher.getInstance("AES/GCM/NoPadding")
        val spec = javax.crypto.spec.GCMParameterSpec(128, dev.clipconnect.app.crypto.buildNonce(counter))
        cipher.init(javax.crypto.Cipher.ENCRYPT_MODE, javax.crypto.spec.SecretKeySpec(key, "AES"), spec)
        return cipher.doFinal(plaintext)
    }

    /**
     * Decrypt data with AES-GCM and counter-based nonce.
     */
    fun decryptWithCounter(ciphertext: ByteArray, key: ByteArray, counter: Long): ByteArray {
        val cipher = javax.crypto.Cipher.getInstance("AES/GCM/NoPadding")
        val spec = javax.crypto.spec.GCMParameterSpec(128, dev.clipconnect.app.crypto.buildNonce(counter))
        cipher.init(javax.crypto.Cipher.DECRYPT_MODE, javax.crypto.spec.SecretKeySpec(key, "AES"), spec)
        return cipher.doFinal(ciphertext)
    }

    /**
     * Generate random bytes.
     */
    fun randomBytes(size: Int): ByteArray {
        val bytes = ByteArray(size)
        java.security.SecureRandom.getInstanceStrong().nextBytes(bytes)
        return bytes
    }
}
