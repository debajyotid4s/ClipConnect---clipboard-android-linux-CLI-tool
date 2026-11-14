package dev.clipconnect.app.service

import android.content.ClipboardManager
import android.content.Context
import android.content.ClipData
import android.os.Handler
import android.os.Looper
import android.util.Log
import dev.clipconnect.app.capture.ClipboardDedupe

/**
 * Applies incoming clips from the laptop to the system clipboard.
 *
 * The old accessibility service owned this path; with clipboard reading removed,
 * this lightweight holder keeps the laptop->phone write path alive and records
 * applied clips into the echo guard so they are never re-sent.
 */
class IncomingClipApplier {
    companion object {
        private const val TAG = "IncomingClip"

        @Volatile
        private var instance: IncomingClipApplier? = null

        fun attach(applier: IncomingClipApplier) {
            instance = applier
        }

        fun apply(context: Context, text: String) {
            instance?.apply(context, text)
        }
    }

    private val dedupe = ClipboardDedupe()

    fun apply(context: Context, text: String) {
        Handler(Looper.getMainLooper()).post {
            try {
                val clipboard = context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
                val clip = ClipData.newPlainText("ClipConnect", text)
                clipboard.setPrimaryClip(clip)
                Log.d(TAG, "Applied clip from laptop: ${text.length} chars")

                val hash = dedupe.hash(text)
                dedupe.recordApplied(hash)
            } catch (e: Exception) {
                Log.e(TAG, "Error writing received clip: ${e.message}")
            }
        }
    }
}
