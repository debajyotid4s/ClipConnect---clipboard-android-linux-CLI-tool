package dev.clipconnect.app.network

/**
 * Exponential reconnect backoff: 1s, 2s, 4s, ... capped at 30s, reset to 1s
 * on a successful connection. Matches the laptop reconnect policy.
 */
class ReconnectBackoff {
    companion object {
        const val INITIAL = 1000L
        const val MAX = 30000L
    }

    var delay = INITIAL
        private set

    /** Return the delay to wait after a failed attempt, then double it. */
    fun fail(): Long {
        val now = delay
        delay = minOf(delay * 2, MAX)
        return now
    }

    /** Reset after a successful connection. */
    fun reset() {
        delay = INITIAL
    }
}
