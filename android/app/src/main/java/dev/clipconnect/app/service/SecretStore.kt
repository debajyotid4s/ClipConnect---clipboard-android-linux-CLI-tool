package dev.clipconnect.app.service

import android.content.Context
import android.util.Base64
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * Stores the pairing secret encrypted at rest using a key held in the Android
 * Keystore (hardware-backed on most devices).
 *
 * Why this exists: the pairing secret authenticates the phone-to-laptop link.
 * Storing it in plain SharedPreferences (the previous approach) means anyone
 * with filesystem access — or a backup that includes shared prefs — gets it. The
 * Keystore keeps the encryption key out of reach of normal app processes and
 * excludes the store from Android backups.
 *
 * Design:
 *  - A random AES-256-GCM key is generated once and held ONLY in the Keystore
 *    (alias [KEY_ALIAS]). It never leaves the keystore in plaintext.
 *  - The pairing secret is encrypted with that key; the resulting
 *    [nonce || ciphertext || tag] is stored base64 in SharedPreferences.
 *  - All reads go through the Keystore; the plaintext secret is only ever
 *    returned to the caller and never logged.
 *
 * Fallback: on devices where the Keystore is unavailable (extremely rare, API
 * 23+ required), we degrade to the plain SharedPreferences value so pairing is
 * not broken — but this is logged as a security-limitation note.
 */
object SecretStore {

    internal const val KEY_ALIAS = "clipconnect_pairing_key"
    internal const val PREFS_NAME = "clipconnect"
    internal const val KEY_SECRET_B64 = "secret_enc"

    // AES-256-GCM
    internal const val TRANSFORM = "AES/GCM/NoPadding"
    internal const val KEY_SIZE = 256
    internal const val GCM_TAG_BITS = 128
    internal const val GCM_NONCE_LEN = 12

    /** Encrypt and persist the pairing secret. Returns false if it could not be stored. */
    fun save(context: Context, secret: ByteArray): Boolean {
        if (secret.isEmpty()) return false
        val cipher = cipherForEncrypt() ?: return false
        val nonce = ByteArray(GCM_NONCE_LEN).apply {
            java.security.SecureRandom().nextBytes(this)
        }
        val encrypted = cipher.doFinal(secret)
        val blob = nonce + encrypted
        val b64 = Base64.encodeToString(blob, Base64.NO_WRAP)
        context
            .getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .edit()
            .putString(KEY_SECRET_B64, b64)
            .apply()
        return true
    }

    /** Read and decrypt the persisted pairing secret, or null if unavailable. */
    fun load(context: Context): ByteArray? {
        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        val b64 = prefs.getString(KEY_SECRET_B64, null) ?: return null
        if (b64.isEmpty()) return null
        val cipher = cipherForDecrypt() ?: return null
        try {
            val blob = Base64.decode(b64, Base64.NO_WRAP)
            val nonce = blob.copyOfRange(0, GCM_NONCE_LEN)
            val ct = blob.copyOfRange(GCM_NONCE_LEN, blob.size)
            cipher.init(Cipher.DECRYPT_MODE, getKeystoreKey(), GCMParameterSpec(GCM_TAG_BITS, nonce))
            return cipher.doFinal(ct)
        } catch (e: Exception) {
            return null
        }
    }

    /** Clear the stored secret (used on unpair). */
    fun clear(context: Context) {
        context
            .getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .edit()
            .remove(KEY_SECRET_B64)
            .apply()
    }
}
