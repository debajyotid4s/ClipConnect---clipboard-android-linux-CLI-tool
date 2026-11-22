package dev.clipconnect.app.service

import android.content.ClipboardManager
import android.util.Log
import dev.clipconnect.app.crypto.Crypto
import dev.clipconnect.app.network.TcpClient
import dev.clipconnect.app.network.sendClip
import dev.clipconnect.app.sync.ClipFilter

/**
 * Handle a clipboard-changed callback. Reads the current primary text clip with
 * a short settle retry, then sends it if it is genuinely new. Returns nothing;
 * failures are logged only.
 */
internal fun ClipboardCapture.onClipboardChanged() {
    val text = readWithSettle()
    if (text.isNullOrEmpty()) return

    val now = System.currentTimeMillis()
    if (now - lastSentAt < ClipboardCapture.DEBOUNCE_MS) return

    // Echo guard: skip a clip the laptop just wrote to this phone.
    if (lastSentHash != null && Crypto.sha256(text) == lastSentHash) return

    if (ClipFilter.shouldSkip(text)) {
        // Privacy: never log content. Log only that a clip was skipped and
        // roughly why (length + event name), so a skip is observable without
        // leaking clipboard plaintext.
        Log.d(ClipboardCapture.TAG, "skipped clip: ${text.length} chars")
        // Reset the skip sentinel so a later non-sensitive clip still sends.
        synchronized(lock) { lastSentHash = null }
        return
    }

    val client = tcpClient()
    if (client == null || !client.isConnected) return

    val seq = client.sendClip(text)
    synchronized(lock) {
        lastSentHash = Crypto.sha256(text)
        lastSentAt = now
    }
    if (seq == 0L) {
        synchronized(lock) { lastSentHash = null }
        Log.e(ClipboardCapture.TAG, "send failed; will retry on reconnect")
    } else {
        Log.d(ClipboardCapture.TAG, "sent clip seq=$seq: ${text.length} chars")
        onSent(text.length)
    }
}

/**
 * Read the primary text clip, retrying briefly to survive OEM settle lag. Returns
 * null for empty, multi-item, non-text, or focus-denied reads.
 */
internal fun ClipboardCapture.readWithSettle(): String? {
    val mgr = clipboardManager ?: return null
    for (attempt in 0 until 5) {
        val text = try {
            val clip = mgr.getPrimaryClip()
            if (clip == null || clip.itemCount != 1) null
            else clip.getItemAt(0)?.text?.toString()
        } catch (e: Throwable) {
            null
        }
        if (!text.isNullOrEmpty()) return text
        if (attempt < 4) {
            try { Thread.sleep(120) } catch (e: InterruptedException) {
                Thread.currentThread().interrupt()
                return null
            }
        }
    }
    return null
}

/**
 * On focus regain, re-read the current clipboard once so the latest available
 * content is captured immediately after reopening the app. Android provides no
 * clipboard history, so only the current value can be recovered.
 */
fun ClipboardCapture.readCurrentOnFocusGain() {
    val text = readWithSettle()
    if (text.isNullOrEmpty()) return
    val client = tcpClient()
    if (client == null || !client.isConnected) return
    // Use the same send path (dedupe + echo guard) so a stale re-read is dropped.
    if (ClipFilter.shouldSkip(text)) return
    val seq = client.sendClip(text)
    if (seq != 0L) {
        synchronized(lock) {
            lastSentHash = Crypto.sha256(text)
            lastSentAt = System.currentTimeMillis()
        }
        onSent(text.length)
        Log.d(ClipboardCapture.TAG, "re-read current clip on focus gain: ${text.length} chars")
    }
}

/**
 * Record a clip the laptop just wrote to this phone's clipboard, so the
 * outgoing capture treats it as already-sent and never echoes it back.
 *
 * This closes the phone→laptop echo bounce deterministically: without it, an
 * applied clip that the phone never sent would pass the outgoing
 * [lastSentHash] check, rely entirely on the laptop's own guard, and waste a
 * round-trip (or, under a network race, slip through). Recording it locally
 * makes the guard local and immediate.
 */
fun ClipboardCapture.onClipApplied(text: String) {
    if (text.isNullOrEmpty()) return
    notifyApplied(text)
    synchronized(lock) {
        lastSentHash = Crypto.sha256(text)
        lastSentAt = System.currentTimeMillis()
    }
}
