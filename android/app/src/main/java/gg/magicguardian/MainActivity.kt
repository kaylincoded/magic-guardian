package gg.magicguardian

import android.app.DownloadManager
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.Environment
import android.webkit.JavascriptInterface
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.FileProvider
import java.io.File

/**
 * Main activity that displays the magic-guardian web UI in a WebView.
 * Starts the foreground service if not already running.
 */
class MainActivity : AppCompatActivity() {

    private lateinit var webView: WebView
    private var downloadId: Long = -1

    private val downloadReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            val id = intent?.getLongExtra(DownloadManager.EXTRA_DOWNLOAD_ID, -1) ?: -1
            if (id == downloadId) {
                installApk()
            }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // Start the foreground service (which starts the Go binary)
        startGuardianService()

        // Register download complete receiver
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            registerReceiver(downloadReceiver, IntentFilter(DownloadManager.ACTION_DOWNLOAD_COMPLETE), RECEIVER_EXPORTED)
        } else {
            registerReceiver(downloadReceiver, IntentFilter(DownloadManager.ACTION_DOWNLOAD_COMPLETE))
        }

        // Set up WebView
        webView = WebView(this).apply {
            settings.javaScriptEnabled = true
            settings.domStorageEnabled = true
            settings.cacheMode = WebSettings.LOAD_NO_CACHE
            settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW

            // Add JavaScript interface for APK download
            addJavascriptInterface(UpdateInterface(), "AndroidUpdate")

            webViewClient = object : WebViewClient() {
                override fun shouldOverrideUrlLoading(
                    view: WebView?,
                    request: WebResourceRequest?
                ): Boolean {
                    // Keep all navigation within the WebView for localhost
                    val url = request?.url?.toString() ?: return false
                    if (url.startsWith("http://127.0.0.1:${GuardianService.PORT}")) {
                        return false
                    }
                    // Open external links in browser
                    val intent = Intent(Intent.ACTION_VIEW, request.url)
                    startActivity(intent)
                    return true
                }
            }
        }

        setContentView(webView)

        // Load the web UI (with a small delay to let the binary start)
        webView.postDelayed({
            webView.loadUrl("http://127.0.0.1:${GuardianService.PORT}")
        }, 1500)
    }

    override fun onBackPressed() {
        if (webView.canGoBack()) {
            webView.goBack()
        } else {
            // Move to background instead of closing (service keeps running)
            moveTaskToBack(true)
        }
    }

    override fun onDestroy() {
        try {
            unregisterReceiver(downloadReceiver)
        } catch (_: Exception) {}
        webView.destroy()
        super.onDestroy()
        // Note: we do NOT stop the service here — it keeps running in background
    }

    private fun startGuardianService() {
        val intent = Intent(this, GuardianService::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }

    private fun downloadApk(url: String) {
        Toast.makeText(this, "Downloading update...", Toast.LENGTH_SHORT).show()
        
        val request = DownloadManager.Request(Uri.parse(url))
            .setTitle("Magic Guardian Update")
            .setDescription("Downloading new version...")
            .setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
            .setDestinationInExternalPublicDir(Environment.DIRECTORY_DOWNLOADS, "magic-guardian.apk")
            .setAllowedOverMetered(true)
            .setAllowedOverRoaming(true)

        val dm = getSystemService(DOWNLOAD_SERVICE) as DownloadManager
        downloadId = dm.enqueue(request)
    }

    private fun installApk() {
        val file = File(Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_DOWNLOADS), "magic-guardian.apk")
        if (!file.exists()) {
            Toast.makeText(this, "Download failed", Toast.LENGTH_SHORT).show()
            return
        }

        val uri = FileProvider.getUriForFile(this, "$packageName.fileprovider", file)
        val intent = Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(uri, "application/vnd.android.package-archive")
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        startActivity(intent)
    }

    inner class UpdateInterface {
        @JavascriptInterface
        fun downloadUpdate(url: String) {
            runOnUiThread { downloadApk(url) }
        }
    }
}
