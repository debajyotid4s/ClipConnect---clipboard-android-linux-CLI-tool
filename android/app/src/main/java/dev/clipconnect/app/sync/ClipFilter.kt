package dev.clipconnect.app.sync

import java.security.MessageDigest

/**
 * Clipboard content policy shared by both sync directions.
 *
 * Mirrors the laptop's clip package:
 *  - skip empty text and text over the wire limit (256 KiB)
 *  - skip clips flagged sensitive (password-manager hints)
 *  - sha256 hash for the echo guard / dedupe
 *
 * Clipboard plaintext is only ever passed to the network layer; it is never
 * written to a log.
 */
object ClipFilter {

    const val MAX_CLIP_BYTES = 256 * 1024 // 256 KiB, matches laptop wire limit

    private val sensitivePatterns = listOf("x-kde-passwordManagerHint")

    /** @return true if the clip must NOT be synced (empty, too large, or sensitive). */
    fun shouldSkip(text: String?): Boolean {
        if (text.isNullOrEmpty()) return true
        if (text.toByteArray(Charsets.UTF_8).size > MAX_CLIP_BYTES) return true
        return isSensitive(text)
    }

    private fun isSensitive(text: String): Boolean {
        for (p in sensitivePatterns) {
            if (text.contains(p)) return true
        }
        return false
    }

    fun hash(text: String): String {
        val digest = MessageDigest.getInstance("SHA-256")
        val bytes = digest.digest(text.toByteArray(Charsets.UTF_8))
        return bytes.joinToString("") { "%02x".format(it) }
    }
}
