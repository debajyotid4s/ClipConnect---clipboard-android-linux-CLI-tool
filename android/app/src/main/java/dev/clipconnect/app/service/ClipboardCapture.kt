package dev.clipconnect.app.service

import android.content.ClipboardManager
import android.content.Context
import android.util.Log
import dev.clipconnect.app.network.TcpClient

/**
 * Event-driven outgoing clipboard capture (phone -> laptop).
 *
 * Android 10+ restricts ClipboardManager.getPrimaryClip() to the app that currently
 * owns window focus (the default IME is the only exception, and we do not use it).
 * Therefore capture is only possible while the Activity is foregrounded, and this
 * class is deliberately tied to Activity lifecycle callbacks rather than running a
 * permanent background poll.
 *
 * Design:
 *  - A single [ClipboardManager.OnPrimaryClipChangedListener] is the capture source.
 *    It fires on genuine clipboard changes and is far faster than polling.
 *  - The listener is registered only while the Activity reports window focus and is
 *    unregistered the moment focus is lost. This is the honest boundary: we do not
 *    pretend to monitor the clipboard while backgrounded.
 *  - A short debounce + sha256 dedupe guards against multiple callbacks for one
 *    logical copy and against echo (a clip we just wrote from the laptop).
 *  - All reads go through the public [ClipboardManager] API; no ADB, no shell, no
 *    hidden APIs. Clipboard plaintext is only handed to the network layer.
 *
 * This class holds a weak-ish reference to the Activity via [focusOwner] so a
 * destroyed Activity can never leak through a registered listener.
 */
class ClipboardCapture(
    private val context: Context,
    internal val tcpClient: () -> TcpClient?,
    internal val onSent: (Int) -> Unit = {},
) {
    companion object {
        internal const val TAG = "ClipboardCapture"

        // Debounce: ignore callbacks that arrive faster than this after a real one,
        // so a single copy that triggers multiple onPrimaryClipChanged callbacks is
        // not sent several times.
        internal const val DEBOUNCE_MS = 400L

        // Callback invoked when the laptop writes a clip to this phone, so the
        // outgoing guard can mark it as already-sent. Set once by MainActivity
        // (which owns the single ClipboardCapture instance) so IncomingClipApplier
        // — created with an arbitrary Context — can reach it without a leak.
        @Volatile
        private var appliedListener: ((String) -> Unit)? = null

        /** Register the applied-clip notifier. Called once from MainActivity. */
        fun registerAppliedListener(fn: (String) -> Unit) {
            appliedListener = fn
        }

        /** Invoke the registered applied-clip listener from IncomingClipApplier.
         * Never throws. */
        fun fireApplied(text: String) {
            try {
                appliedListener?.invoke(text)
            } catch (e: Throwable) {
                Log.e(TAG, "applied listener error: ${e.message}")
            }
        }
    }

    /** Notify the applied-clip listener, if any. Never throws. */
    internal fun notifyApplied(text: String) {
        try {
            appliedListener?.invoke(text)
        } catch (e: Throwable) {
            Log.e(TAG, "applied listener error: ${e.message}")
        }
    }

    interface FocusOwner {
        /** True only when this Activity actually holds window focus. */
        fun hasWindowFocus(): Boolean
    }

    internal val clipboardManager: ClipboardManager? =
        context.getSystemService(Context.CLIPBOARD_SERVICE) as? ClipboardManager

    internal var listener: ClipboardManager.OnPrimaryClipChangedListener? = null

    // Focus-gated registration state. Guarded by [lock]; never touch from the main
    // thread while a focus callback is in flight.
    internal val lock = Any()
    internal var registered = false
    internal var lastSentHash: String? = null
    internal var lastSentAt = 0L

    /**
     * Register the listener if (and only if) the focus owner currently has window
     * focus. Safe to call from both the focus callback thread and the main thread.
     */
    fun onFocusGained(focus: FocusOwner) {
        synchronized(lock) {
            if (registered) return
            if (!focus.hasWindowFocus()) return
            val mgr = clipboardManager ?: return
            val l = object : ClipboardManager.OnPrimaryClipChangedListener {
                override fun onPrimaryClipChanged() {
                    onClipboardChanged()
                }
            }
            try {
                mgr.addPrimaryClipChangedListener(l)
                listener = l
                registered = true
                Log.d(TAG, "capture listener registered (focus gained)")
            } catch (e: Throwable) {
                Log.e(TAG, "failed to register capture listener: ${e.message}")
            }
        }
    }

    /**
     * Unregister the listener if present. Called on focus loss / pause / destroy so
     * we stop pretending to monitor the clipboard while backgrounded.
     */
    fun onFocusLost() {
        synchronized(lock) {
            unregisterLocked()
        }
    }

    private fun unregisterLocked() {
        if (!registered) return
        val mgr = clipboardManager
        val l = listener
        if (mgr != null && l != null) {
            try {
                mgr.removePrimaryClipChangedListener(l)
            } catch (e: Throwable) {
                Log.d(TAG, "remove listener: ${e.message}")
            }
        }
        listener = null
        registered = false
        Log.d(TAG, "capture listener unregistered (focus lost)")
    }

    /** Tear everything down. Call from Activity onDestroy. */
    fun destroy() {
        synchronized(lock) {
            unregisterLocked()
        }
    }
}
