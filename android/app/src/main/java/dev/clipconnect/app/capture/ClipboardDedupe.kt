package dev.clipconnect.app.capture

import android.util.Log
import dev.clipconnect.app.sync.ClipFilter

/**
 * Clipboard deduplication: sha256 hashing and echo guard.
 * Prevents applying a laptop clip from being re-sent back to the laptop.
 */
class ClipboardDedupe {
    companion object {
        private const val TAG = "ClipboardDedupe"
        const val ECHO_GUARD_WINDOW_MS = 2000L
        const val ECHO_CACHE_MAX_SIZE = 50
    }

    private var lastAppliedHash: String? = null
    private val echoCache = mutableListOf<Pair<String, Long>>()

    /**
     * Dedupe: check if the clip is a duplicate of the last applied clip
     * or was applied within the echo guard window.
     * Returns true if this is a duplicate (should be skipped).
     */
    fun isDuplicate(hash: String): Boolean {
        if (hash == lastAppliedHash) {
            Log.d(TAG, "Dedupe: duplicate hash ($hash)")
            return true
        }
        // Check echo cache
        val now = System.currentTimeMillis()
        if (echoCache.any { (h, t) -> h == hash && t > now - ECHO_GUARD_WINDOW_MS }) {
            Log.d(TAG, "Dedupe: echo cache hit for hash=$hash")
            return true
        }
        return false
    }

    /**
     * Record a clip as applied (received from laptop).
     * Triggers echo guard to prevent sending it back.
     */
    fun recordApplied(hash: String) {
        lastAppliedHash = hash
        addToEchoCache(hash, System.currentTimeMillis())
        Log.d(TAG, "Record applied: hash=${hash.take(16)}...")
    }

    fun hash(text: String): String = ClipFilter.hash(text)

    private fun addToEchoCache(hash: String, time: Long) {
        echoCache.add(Pair(hash, time))
        while (echoCache.size > ECHO_CACHE_MAX_SIZE) {
            echoCache.removeAt(0)
        }
    }
}
