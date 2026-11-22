package dev.clipconnect.app.accessibility

import android.view.accessibility.AccessibilityEvent
import android.util.Log

/**
 * Logging helpers for the accessibility PoC. Kept separate from the service so
 * the event-handling logic stays small and the pure text helpers are obvious to
 * review.
 */

internal fun AccessibilityClipCaptureService.eventTextPreview(event: AccessibilityEvent): String {
    val sb = StringBuilder()
    for (t in event.text) {
        if (t != null) {
            if (sb.isNotEmpty()) sb.append(" | ")
            sb.append(trunc(t.toString()))
        }
    }
    return sb.toString()
}

internal fun AccessibilityClipCaptureService.clipboardHint(text: String?, desc: String?): String? {
    val combined = ((text ?: "") + " " + (desc ?: "")).lowercase()
    for (h in AccessibilityClipCaptureService.clipboardHints) {
        if (combined.contains(h)) return h
    }
    return null
}

internal fun AccessibilityClipCaptureService.trunc(s: String?): String {
    if (s == null) return "null"
    val max = 120
    return if (s.length <= max) s else s.substring(0, max) + "…"
}
