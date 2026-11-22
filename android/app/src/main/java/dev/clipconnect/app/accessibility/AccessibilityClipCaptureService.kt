package dev.clipconnect.app.accessibility

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.AccessibilityServiceInfo
import android.view.accessibility.AccessibilityEvent
import android.view.accessibility.AccessibilityNodeInfo
import android.util.Log
import java.util.TreeSet

/**
 * Proof-of-concept accessibility clipboard-preview capture.
 *
 * This is a SEPARATE experiment from the working foreground ClipboardManager
 * reader. Its only job is to answer one question: while the app is
 * backgrounded, does the SystemUI clipboard overlay expose the actual copied
 * text to a third-party accessibility service?
 *
 * It logs the full accessibility window tree on clipboard-related events. It
 * never sends anything to the laptop and never modifies the clipboard.
 *
 * Privacy: only accessibility node text is logged (event text + content
 * description). No clipboard plaintext is read via ClipboardManager here.
 */
class AccessibilityClipCaptureService : AccessibilityService() {

    companion object {
        private const val TAG = "AccClipPoC"

        // Windows that may carry the SystemUI clipboard preview.
        internal val systemUiPackages = setOf(
            "com.android.systemui",
            "com.miui.systemui",
            "com.miui.notification",
            "com.android.statusbar",
        )

        // Keywords that suggest a clipboard-preview node rather than a random one.
        internal val clipboardHints = setOf(
            "copied", "clipboard", "clip", "text copied to clipboard",
            "paste", "paste from clipboard",
        )

        // Distinct events we care about for logging the window tree.
        internal val trackedTypes = setOf(
            AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED,
            AccessibilityEvent.TYPE_WINDOW_CONTENT_CHANGED,
        )
    }

    override fun onServiceConnected() {
        super.onServiceConnected()
        Log.d(TAG, "PoC accessibility service connected")
        // Retrieve interactive windows + nodes, and clipboard/window events.
        val info = AccessibilityServiceInfo().apply {
            eventTypes = AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED or
                    AccessibilityEvent.TYPE_WINDOW_CONTENT_CHANGED
            feedbackType = AccessibilityServiceInfo.FEEDBACK_GENERIC
            flags = AccessibilityServiceInfo.FLAG_RETRIEVE_INTERACTIVE_WINDOWS
        }
        setServiceInfo(info)
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {
        if (event == null) return

        // Only act on the tracked event types.
        if (!trackedTypes.contains(event.eventType)) return

        val pkg = event.packageName?.toString() ?: "null"
        val cls = event.className?.toString() ?: "null"
        Log.d(TAG, "EVENT type=${event.eventType} pkg=$pkg cls=$cls text=${eventTextPreview(event)}")

        // Capture the current window tree (all windows) so we can inspect the
        // SystemUI overlay even though this event came from another package.
        val windows = getWindows()
        if (windows.isEmpty()) {
            Log.d(TAG, "  windows: none reported")
            return
        }

        Log.d(TAG, "=== WINDOW TREE (${windows.size} windows) ===")
        for (widx in windows.indices) {
            val root = windows[widx].root
            if (root == null) continue
            logWindow(widx, root)
        }
    }

    private fun logWindow(widx: Int, root: AccessibilityNodeInfo) {
        val win = root.windowId
        Log.d(TAG, "--- window[$widx] id=$win package=${root.packageName} ---")
        val nodes = TreeSet<AccessibilityNodeInfo>()
        nodes.add(root)
        while (nodes.isNotEmpty()) {
            val node = nodes.pollFirst()
            val text = node.text?.toString()
            val desc = node.contentDescription?.toString()
            val pkg = node.packageName?.toString() ?: "null"
            val cls = node.className?.toString() ?: "null"
            val isSystemUi = systemUiPackages.contains(pkg)
            val hint = clipboardHint(text, desc)
            // Log every node but mark SystemUI windows and clipboard hints so the
            // log is scannable.
            if (isSystemUi || hint != null || !text.isNullOrEmpty() || !desc.isNullOrEmpty()) {
                Log.d(TAG, "  node pkg=$pkg cls=$cls text=${trunc(text)} desc=${trunc(desc)} " +
                    "sysui=$isSystemUi hint=$hint")
            }
            // Collect children (bounded to avoid pathological trees).
            val childCount = node.childCount
            for (i in 0 until minOf(childCount, 40)) {
                val child = node.getChild(i) ?: continue
                nodes.add(child)
            }
        }
    }

    override fun onInterrupt() {
        // no-op
    }

    private fun logAllWindows() {
        Log.d(TAG, "=== MANUAL DUMP ===")
        getWindows().forEachIndexed { i, w ->
            w.root?.let { logWindow(i, it) }
        }
    }
}
