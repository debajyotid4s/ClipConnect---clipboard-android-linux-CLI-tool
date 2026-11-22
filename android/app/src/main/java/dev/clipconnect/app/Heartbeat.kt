package dev.clipconnect.app

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow

/**
 * Heartbeat is a process-wide trigger the UI observes to fire the red-wave
 * animation. Any component can [pulse] it (connection established, clip
 * acknowledged) and the UI shows one animation per pulse.
 *
 * It is a plain Int counter so the value survives configuration changes and the
 * composable only reacts to increases.
 */
object Heartbeat {
    private val _pulse = MutableStateFlow(0)

    /** Observable pulse counter. The UI observes this to drive animations. */
    val pulse = _pulse.asStateFlow()

    /** Fire the heartbeat once. Returns the new pulse value. */
    fun pulse(): Int {
        _pulse.value += 1
        return _pulse.value
    }
}
