package gg.magicguardian

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.os.PowerManager
import android.util.Log
import java.io.File

/**
 * Foreground service that runs the magic-guardian Go binary.
 * The binary starts an HTTP server on localhost:8090 serving the web UI.
 */
class GuardianService : Service() {

    companion object {
        const val TAG = "GuardianService"
        const val CHANNEL_ID = "magic_guardian_service"
        const val NOTIFICATION_ID = 1
        const val PORT = 8090

        @Volatile
        var process: Process? = null

        fun isRunning(): Boolean = process?.isAlive == true
    }

    private var wakeLock: PowerManager.WakeLock? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        startForeground(NOTIFICATION_ID, buildNotification())
        acquireWakeLock()
        startBinary()
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        stopBinary()
        releaseWakeLock()
        super.onDestroy()
    }

    private fun startBinary() {
        if (isRunning()) {
            Log.i(TAG, "Binary already running")
            return
        }

        // The binary is packaged as a native library (libguardian.so) in the APK's lib/ directory.
        // This is the only location where Android allows execution of binaries from an app.
        val nativeLibDir = applicationContext.applicationInfo.nativeLibraryDir
        val binaryFile = File(nativeLibDir, "libguardian.so")

        if (!binaryFile.exists()) {
            Log.e(TAG, "Binary not found at ${binaryFile.absolutePath}")
            return
        }

        val dbPath = File(applicationContext.filesDir, "magic-guardian.db").absolutePath

        try {
            val pb = ProcessBuilder(
                binaryFile.absolutePath,
                "-ui",
                "-listen", "127.0.0.1:$PORT",
                "-db", dbPath,
                "-auto-start"
            )
            pb.directory(applicationContext.filesDir)
            pb.redirectErrorStream(true)
            process = pb.start()

            // Log output in background thread
            Thread {
                try {
                    process?.inputStream?.bufferedReader()?.forEachLine { line ->
                        Log.i(TAG, line)
                    }
                } catch (e: Exception) {
                    Log.e(TAG, "Log reader error", e)
                }
            }.start()

            Log.i(TAG, "Binary started on port $PORT")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start binary", e)
        }
    }

    private fun stopBinary() {
        process?.let {
            it.destroy()
            it.waitFor()
            process = null
            Log.i(TAG, "Binary stopped")
        }
    }

    private fun acquireWakeLock() {
        val pm = getSystemService(POWER_SERVICE) as PowerManager
        wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "MagicGuardian::Service")
        wakeLock?.acquire()
    }

    private fun releaseWakeLock() {
        wakeLock?.let {
            if (it.isHeld) it.release()
        }
        wakeLock = null
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                getString(R.string.notification_channel_name),
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = getString(R.string.notification_channel_description)
            }
            val nm = getSystemService(NotificationManager::class.java)
            nm.createNotificationChannel(channel)
        }
    }

    private fun buildNotification(): Notification {
        val intent = Intent(this, MainActivity::class.java)
        val pendingIntent = PendingIntent.getActivity(
            this, 0, intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        return Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("Magic Guardian")
            .setContentText("Bot is running in the background")
            .setSmallIcon(android.R.drawable.ic_menu_manage)
            .setContentIntent(pendingIntent)
            .setOngoing(true)
            .build()
    }
}
