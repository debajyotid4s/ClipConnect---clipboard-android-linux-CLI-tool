package dev.clipconnect.app.ui

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.StateFlow

/**
 * A bounded, observable log backed by a ring buffer.
 *
 * The UI observes [entries] as a stable list, so appending a line triggers a
 * recomposition without the view holding a mutable list that only updates on
 * other recompositions. Only the newest [capacity] lines are retained, which
 * bounds memory for long-running sessions. Kept free of Android so it is unit
 * testable on the JVM.
 */
class LogRepository(val capacity: Int = 200) {
    private val buffer = ArrayDeque<LogLine>(capacity)
    private val _entries = MutableStateFlow<List<LogLine>>(emptyList())
    val entries: StateFlow<List<LogLine>> = _entries.asStateFlow()

    init {
        require(capacity >= 1) { "capacity must be >= 1" }
    }

    /** Append a line, evicting the oldest when the buffer is full. */
    fun add(kind: LogLine.Kind, message: String) {
        buffer.addLast(LogLine(kind, message))
        if (buffer.size > capacity) buffer.removeFirst()
        // Copy to a new list so observers get a stable, immutable snapshot.
        _entries.value = buffer.toList()
    }

    fun info(message: String) = add(LogLine.Kind.Info, message)
    fun ok(message: String) = add(LogLine.Kind.Ok, message)
    fun warn(message: String) = add(LogLine.Kind.Warn, message)
    fun error(message: String) = add(LogLine.Kind.Error, message)

    /** Clear all lines (used when forgetting a pairing). */
    fun clear() {
        buffer.clear()
        _entries.value = emptyList()
    }
}
