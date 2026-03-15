package gg.magicguardian

import android.content.Intent
import android.os.Build
import android.os.Bundle
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.appcompat.app.AppCompatActivity

/**
 * Main activity that displays the magic-guardian web UI in a WebView.
 * Starts the foreground service if not already running.
 */
class MainActivity : AppCompatActivity() {

    private lateinit var webView: WebView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // Start the foreground service (which starts the Go binary)
        startGuardianService()

        // Set up WebView
        webView = WebView(this).apply {
            settings.javaScriptEnabled = true
            settings.domStorageEnabled = true
            settings.cacheMode = WebSettings.LOAD_NO_CACHE
            settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW

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
}
