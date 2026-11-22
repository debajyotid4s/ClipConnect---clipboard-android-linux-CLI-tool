package dev.clipconnect.app.ui

/**
 * The observable connection state of the phone-to-laptop link.
 *
 * The UI renders one screen per variant so there is a single source of truth for
 * what the user sees. The [ConnectionService] advances these states as the TCP
 * connection is established, lost, or retried; the UI only reads them.
 *
 * Keep this a plain data hierarchy (no Android imports) so it can be unit tested
 * and so the service and the ViewModel share one vocabulary.
 */
sealed interface LinkState {

    /** No pairing is stored. Home shows a single "Pair with laptop" action. */
    data object Unpaired : LinkState

    /** A pairing exists and the socket is opening for the first time. */
    data object Connecting : LinkState

    /** A pairing exists and the socket is reconnecting after a loss. */
    data class Reconnecting(val attempt: Int) : LinkState

    /** The socket is up. [since] is the epoch millis the connection was accepted. */
    data class Connected(val since: Long) : LinkState

    /** The last connect attempt failed with a human-readable reason. */
    data class Failed(val reason: String) : LinkState
}

/**
 * A one-way event the UI reacts to rather than renders as state.
 *
 * Sent/received describe a clip that crossed the wire (length + time only — never
 * content). [Acked] fires when the laptop acknowledges a clip this phone sent,
 * which is what drives the heartbeat pulse.
 */
sealed interface SyncEvent {
    data class Sent(val chars: Int) : SyncEvent
    data class Received(val chars: Int) : SyncEvent
    data object Acked : SyncEvent
}

/**
 * One line of the diagnostics log. Immutable and cheap so it can live in a ring
 * buffer observed as a stable list. [atMillis] is epoch millis for display.
 */
data class LogLine(
    val kind: Kind,
    val message: String,
    val atMillis: Long = System.currentTimeMillis(),
) {
    enum class Kind { Info, Ok, Error, Warn }
}
