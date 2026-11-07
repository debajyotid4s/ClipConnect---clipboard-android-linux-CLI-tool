package dev.clipconnect.app.service

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.AccessibilityServiceInfo
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.os.Handler
import android.os.Looper
import android.util.Log
import android.view.accessibility.AccessibilityEvent
import dev.clipconnect.app.network.TcpClient

class ClipboardAccessibilityService : AccessibilityService() {

    companion object {
        private const val TAG = "ClipboardSvc"
        @Volatile
        var instance: ClipboardAccessibilityService? = null
    }

    private var tcpClient: TcpClient? = null
    private var lastClipHash: String? = null
    private var lastSendTime = 0L
    private var isConfigured = false
    private val handler = Handler(Looper.getMainLooper())
    private var pollRunnable: Runnable? = null

    override fun onServiceConnected() {
        Log.i(TAG, "Accessibility service connected")
        instance = this

        val info = AccessibilityServiceInfo().apply {
            eventTypes = AccessibilityEvent.TYPE_WINDOW_CONTENT_CHANGED or
                         AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED
            feedbackType = AccessibilityServiceInfo.FEEDBACK_ALL_MASK
            flags = AccessibilityServiceInfo.FLAG_INCLUDE_NOT_IMPORTANT_VIEWS or
                    AccessibilityServiceInfo.FLAG_REPORT_VIEW_IDS
            notificationTimeout = 100
        }
        serviceInfo = info

        // Start polling for clipboard changes
        startClipboardPolling()
    }

    private fun startClipboardPolling() {
        pollRunnable = Runnable {
            handleClipboardChange()
            handler.postDelayed(pollRunnable!!, 500)
        }
        handler.post(pollRunnable!!)
        Log.d(TAG, "Clipboard polling started")
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent) {
        handleClipboardChange()
    }

    override fun onInterrupt() {
        Log.w(TAG, "Accessibility interrupted")
        handler.removeCallbacksAndMessages(null)
    }

    override fun onUnbind(intent: Intent?): Boolean {
        handler.removeCallbacksAndMessages(null)
        instance = null
        return super.onUnbind(intent)
    }

    private fun handleClipboardChange() {
        if (!isConfigured || tcpClient == null || !tcpClient!!.isConnected) {
            return
        }

        val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        val clip = clipboard.primaryClip
        if (clip == null || clip.itemCount == 0) return

        val text = clip.getItemAt(0).text?.toString() ?: return
        if (text.isEmpty() || text.length > 262144) return

        val now = System.currentTimeMillis()
        if (now - lastSendTime < 150) return

        val hash = text.hashCode().toString(16)
        if (hash == lastClipHash) return

        tcpClient!!.sendClip(text)
        lastClipHash = hash
        lastSendTime = now
        Log.d(TAG, "Clipboard change detected: ${text.length} chars")
    }

    fun configure(tcp: TcpClient) {
        isConfigured = true
        tcpClient = tcp
        Log.d(TAG, "Service configured with TcpClient")
        tcp.setListener(object : TcpClient.Listener {
            override fun onConnected() {
                Log.d(TAG, "TCP connected")
            }
            override fun onDisconnected(reason: String) {
                Log.d(TAG, "TCP disconnected: $reason")
                Handler(Looper.getMainLooper()).postDelayed({
                    if (isConfigured) tcp.start()
                }, 3000)
            }
            override fun onMessage(text: String) {
                Handler(Looper.getMainLooper()).post {
                    val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
                    val clip = android.content.ClipData.newPlainText("ClipConnect", text)
                    clipboard.setPrimaryClip(clip)
                    Log.d(TAG, "Received clip from laptop: ${text.length} chars")
                }
            }
            override fun onError(error: String) {
                Log.d(TAG, "TCP error: $error")
            }
        })
    }

    fun unconfigure() {
        isConfigured = false
        tcpClient?.disconnect()
        tcpClient = null
        handler.removeCallbacksAndMessages(null)
    }
}
