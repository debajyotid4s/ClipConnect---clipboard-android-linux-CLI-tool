package dev.clipconnect.app.service

import dev.clipconnect.app.crypto.Crypto

/**
 * Pure outgoing-clipboard dedupe decision.
 *
 * Kept free of Android so the core invariant — "a stale read must never pin the
 * dedupe slot and drop the next real clip" — can be unit tested on the JVM.
 */
object OutgoingDedupe {
    /** True when [text] differs (by sha256) from [seenHash], i.e. it is new. */
    fun isNew(text: String, seenHash: String?): Boolean {
        if (text.isBlank()) return false
        return Crypto.sha256(text) != seenHash
    }
}
