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

        // Get the binary path - prefer extracted binary (for updates), fallback to APK native lib
        val binaryFile = getExecutableBinary()

        if (!binaryFile.exists()) {
            Log.e(TAG, "Binary not found at ${binaryFile.absolutePath}")
            return
        }

        val dbPath = File(applicationContext.filesDir, "magic-guardian.db").absolutePath
        val cacheDir = applicationContext.cacheDir.absolutePath

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
            
            // Set environment variables for the updater
            val env = pb.environment()
            env["GUARDIAN_BINARY_PATH"] = getUpdatedBinaryPath().absolutePath
            env["GUARDIAN_CACHE_DIR"] = cacheDir
            
            process = pb.start()

            // Monitor process and restart if it exits
            Thread {
                try {
                    process?.inputStream?.bufferedReader()?.forEachLine { line ->
                        Log.i(TAG, line)
                    }
                } catch (e: Exception) {
                    Log.e(TAG, "Log reader error", e)
                }
                
                // Process ended - check if we should restart
                val exitCode = try { process?.waitFor() ?: -1 } catch (e: Exception) { -1 }
                Log.i(TAG, "Binary exited with code $exitCode")
                process = null
                
                // Restart after a short delay (for updates)
                if (exitCode == 0) {
                    Log.i(TAG, "Restarting binary in 1 second...")
                    Thread.sleep(1000)
                    startBinary()
                }
            }.start()

            Log.i(TAG, "Binary started on port $PORT")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start binary", e)
        }
    }

    /**
     * Returns the binary to execute.
     * Uses APK native lib by default, but prefers updated binary if it exists.
     */
    private fun getExecutableBinary(): File {
        // Check for updated binary first (downloaded via self-update)
        val updatedBinary = getUpdatedBinaryPath()
        if (updatedBinary.exists() && updatedBinary.canExecute()) {
            Log.i(TAG, "Using updated binary: ${updatedBinary.absolutePath}")
            return updatedBinary
        }
        
        // Default: use the binary from APK's native lib directory (always executable)
        val nativeLibDir = applicationContext.applicationInfo.nativeLibraryDir
        val apkBinary = File(nativeLibDir, "libguardian.so")
        Log.i(TAG, "Using APK binary: ${apkBinary.absolutePath}")
        return apkBinary
    }

    private fun getUpdatedBinaryPath(): File {
        return File(applicationContext.filesDir, "bin/magic-guardian")
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
