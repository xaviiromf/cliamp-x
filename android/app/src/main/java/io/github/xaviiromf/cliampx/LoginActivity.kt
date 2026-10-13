package io.github.xaviiromf.cliampx

import android.annotation.SuppressLint
import android.content.Context
import android.os.Bundle
import android.webkit.CookieManager
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.core.net.toUri
import androidx.core.view.WindowCompat
import java.io.File

/**
 * Signs in to YouTube Music in a WebView and saves its cookies where the core
 * reads them (ytmusic_cookies.txt). Plan B, when Google refuses the WebView,
 * is "Importar cookies.txt" in the settings.
 */
class LoginActivity : AppCompatActivity() {
    @SuppressLint("SetJavaScriptEnabled") // Google sign-in needs JavaScript
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        WindowCompat.setDecorFitsSystemWindows(window, false)
        val web = WebView(this)
        CookieManager.getInstance().setAcceptThirdPartyCookies(web, true)
        web.settings.javaScriptEnabled = true
        web.settings.domStorageEnabled = true
        web.settings.userAgentString = CHROME_UA
        web.webViewClient = object : WebViewClient() {
            override fun onPageFinished(view: WebView, url: String) {
                val cookies = CookieManager.getInstance()
                if (url.toUri().host != "music.youtube.com" || cookies.getCookie(MUSIC)?.contains("SAPISID=") != true) return
                cookies.flush()
                saveSignIn(this@LoginActivity, netscape(cookies))
                Toast.makeText(this@LoginActivity, R.string.signed_in, Toast.LENGTH_LONG).show()
                finish()
            }
        }
        // A WebView draws under its own padding, so a frame takes the insets.
        val root = FrameLayout(this).apply { addView(web) }
        padForSystemBars(root, ime = true)
        setContentView(root)
        if (savedInstanceState == null) web.loadUrl(LOGIN_URL)
    }

    companion object {
        private const val MUSIC = "https://music.youtube.com"
        private const val LOGIN_URL = "https://accounts.google.com/ServiceLogin?continue=https://music.youtube.com/"
        // A recent Chrome on Android, without the "wv" WebView token that Google refuses.
        private const val CHROME_UA = "Mozilla/5.0 (Linux; Android 16; SM-A556B) AppleWebKit/537.36 " +
            "(KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36"

        /**
         * The WebView cookies in the Netscape cookies.txt format that the core and
         * yt-dlp read. CookieManager gives names and values only, so the domain
         * comes from the URL and the expiry is 0 (a session cookie, which both keep).
         */
        fun netscape(cookies: CookieManager): String {
            val seen = HashSet<String>()
            val out = StringBuilder("# Netscape HTTP Cookie File\n# Written by cliamp-x. Do not share: it holds your sign-in.\n\n")
            for ((url, domain) in listOf(
                MUSIC to ".youtube.com",
                "https://www.youtube.com" to ".youtube.com",
                "https://www.google.com" to ".google.com",
                "https://accounts.google.com" to "accounts.google.com",
            )) {
                val subdomains = if (domain.startsWith(".")) "TRUE" else "FALSE"
                for (pair in cookies.getCookie(url)?.split("; ").orEmpty()) {
                    val (name, value) = pair.split("=", limit = 2).takeIf { it.size == 2 } ?: continue
                    // accounts.google.com also returns the .google.com cookies: keep the first.
                    val group = if (domain.endsWith("google.com")) "google" else "youtube"
                    if (!seen.add("$group\t$name")) continue
                    out.append("$domain\t$subdomains\t/\tTRUE\t0\t$name\t$value\n")
                }
            }
            return out.toString()
        }

        /** Saves a cookies.txt as the YouTube Music sign-in and restarts the core to read it. */
        fun saveSignIn(context: Context, cookies: String) {
            val config = Core.configDir(context).apply { mkdirs() }
            val tmp = File(config, ".cookies.tmp")
            tmp.writeText(cookies)
            tmp.renameTo(Core.cookies(context))
            // A "cliamp-x logout" record would make the core ignore the new cookies.
            File(config, "ytmusic_auth.json").delete()
            CliampService.send(context, CliampService.ACTION_RESTART)
        }
    }
}
