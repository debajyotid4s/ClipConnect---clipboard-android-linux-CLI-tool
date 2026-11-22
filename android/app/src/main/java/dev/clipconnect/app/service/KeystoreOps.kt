package dev.clipconnect.app.service

import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey

/**
 * Keystore-backed AES-256-GCM operations for [SecretStore]. The encryption key
 * lives only in the Android Keystore and never leaves it in plaintext.
 */
internal fun SecretStore.getKeystoreKey(): SecretKey {
    val ks = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
    val existing = ks.getEntry(KEY_ALIAS, null) as? KeyStore.SecretKeyEntry
    if (existing != null) return existing.secretKey

    val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
    val spec = KeyGenParameterSpec.Builder(
        KEY_ALIAS,
        KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT
    )
        .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
        .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
        .setKeySize(KEY_SIZE)
        // User authentication not required: the app itself reads it, and we want
        // BootReceiver to work without a lockscreen round-trip.
        .build()
    generator.init(spec)
    return generator.generateKey() as SecretKey
}

internal fun SecretStore.cipherForEncrypt(): Cipher? = secureCipher(Cipher.ENCRYPT_MODE)
internal fun SecretStore.cipherForDecrypt(): Cipher? = secureCipher(Cipher.DECRYPT_MODE)

internal fun SecretStore.secureCipher(mode: Int): Cipher? {
    try {
        val cipher = Cipher.getInstance(TRANSFORM)
        cipher.init(mode, getKeystoreKey())
        return cipher
    } catch (e: Exception) {
        return null
    }
}
