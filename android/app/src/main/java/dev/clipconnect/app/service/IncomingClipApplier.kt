package dev.clipconnect.app.service

import android.content.ClipboardManager
import android.content.Context
import android.content.ClipData
import android.os.Handler
import android.os.Looper
import android.util.Log
import dev.clipconnect.app.capture.ClipboardDedupe

/**
 * Applies incoming clips from the laptop to the system clipboard (laptop -> phone).
 *
 * The laptop writes the clip; we record its hash in an echo guard so the outgoing
 * capture never re-sends it back. Writes are posted to the main looper because
 * ClipboardManager writes must happen on the UI thread.
 *
 * Clipboard plaintext is only ever handed to the system clipboard; it is never
 * logged. This class is stateless apart from a small echo cache and takes its
 * context explicitly, so it never holds a leaked Activity reference.
 */
class IncomingClipApplier(private val context: Context) {
    companion object {
        private const val TAG = "IncomingClip"
    }

    private val dedupe = ClipboardDedupe()

    fun apply(text: String) {
        Handler(Looper.getMainLooper()).post {
            try {
                val clipboard = context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
                val clip = ClipData.newPlainText("ClipConnect", text)
                clipboard.setPrimaryClip(clip)
                Log.d(TAG, "Applied clip from laptop: ${text.length} chars")
                dedupe.recordApplied(dedupe.hash(text))
                // Mark it as already-sent on the outgoing side so the phone never
                // echoes this clip back to the laptop (deterministic echo guard).
                ClipboardCapture.fireApplied(text)
            } catch (e: Exception) {
                Log.e(TAG, "Error writing received clip: ${e.message}")
            }
        }
    }
}
