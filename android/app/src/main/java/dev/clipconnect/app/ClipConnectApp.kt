package dev.clipconnect.app

import android.app.Application
import android.util.Log

class ClipConnectApp : Application() {
    companion object {
        private const val TAG = "ClipConnect"
        lateinit var instance: ClipConnectApp

        fun log(msg: String) {
            Log.d(TAG, msg)
        }
    }

    override fun onCreate() {
        super.onCreate()
        instance = this
    }
}
