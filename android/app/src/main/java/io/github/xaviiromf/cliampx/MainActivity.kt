package io.github.xaviiromf.cliampx

import android.Manifest
import android.annotation.SuppressLint
import android.app.SearchManager
import android.content.Intent
import android.content.pm.ActivityInfo
import android.content.pm.PackageManager
import android.content.res.Configuration
import android.graphics.Point
import android.media.AudioManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.MediaStore
import androidx.appcompat.app.AppCompatActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.testTagsAsResourceId
import androidx.core.content.ContextCompat
import androidx.core.content.edit
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import io.github.xaviiromf.cliampx.ui.App
import io.github.xaviiromf.cliampx.ui.LiveTheme
import io.github.xaviiromf.cliampx.ui.Look
import io.github.xaviiromf.cliampx.ui.Looks
import io.github.xaviiromf.cliampx.ui.Nav
import io.github.xaviiromf.cliampx.ui.Onboarding
import io.github.xaviiromf.cliampx.ui.Screen
import io.github.xaviiromf.cliampx.ui.Skin
import io.github.xaviiromf.cliampx.ui.Skins
import io.github.xaviiromf.cliampx.ui.Tab
import com.yausername.youtubedl_android.YoutubeDL
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.drop
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File

/** The native touch UI (UI-SPEC §7.1). The service owns the core; this only talks to it over IPC. */
class MainActivity : AppCompatActivity() {
    private val prefs by lazy { getSharedPreferences("ui", MODE_PRIVATE) }
    private var onboarding by mutableStateOf(false)

    private val askPermissions = registerForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { startCore(linkOf(intent)) }

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        volumeControlStream = AudioManager.STREAM_MUSIC
        lockOrientation()
        // Python is unpacked by the first start of the core.
        Look.load(this)
        val firstRun = !File(noBackupFilesDir, "${YoutubeDL.baseName}/packages/python").exists()
        Looks.load(this)
        Skins.load(this)
        Stations.load(this)
        // A fresh install (no Python yet) shows the onboarding until it ends, also after a restart; an update never does.
        if (firstRun && !prefs.contains(ONBOARDING)) prefs.edit { putBoolean(ONBOARDING, true) }
        onboarding = prefs.getBoolean(ONBOARDING, false)
        setContent {
            LiveTheme {
                Box(Modifier.semantics { testTagsAsResourceId = true }) { if (onboarding) Onboarding(::onboarded) else App(firstRun) }
            }
        }

        if (savedInstanceState == null && intent.getBooleanExtra(EXTRA_OPEN_PLAYER, false)) Nav.openPlayer()
        if (savedInstanceState == null) shortcut(intent)
        // The first start asks for notifications and local music before the core reads the music folder.
        // Onboarding asks for them itself while the core starts behind it.
        val link = if (savedInstanceState == null) linkOf(intent) else null
        val missing = permissions().filter { ContextCompat.checkSelfPermission(this, it) != PackageManager.PERMISSION_GRANTED }
        if (!onboarding && missing.isNotEmpty() && !prefs.getBoolean(ASKED, false)) {
            prefs.edit { putBoolean(ASKED, true) }
            askPermissions.launch(missing.toTypedArray())
        } else {
            startCore(link)
        }
        if (savedInstanceState == null) offerUpdate()
        // A quit from earlier in this process left quitting set: only a new one closes the window.
        lifecycleScope.launch {
            Remote.quitting.drop(1).first { it }
            finish()
        }
        // Events skip position ticks: re-read the state while a song plays on screen.
        lifecycleScope.launch {
            repeatOnLifecycle(Lifecycle.State.STARTED) {
                while (true) {
                    delay(5000)
                    if (Remote.state.value?.playing == true) Remote.refresh()
                }
            }
        }
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        lockOrientation() // a foldable that opens or closes
    }

    /** Phones stay in portrait (UI-SPEC §3.9); a tablet or an open foldable turns, with the landscape player (§3.10). */
    @SuppressLint("SourceLockedOrientationActivity")
    private fun lockOrientation() {
        // The display's size, not the window's: a letterboxed portrait window would keep a tablet locked.
        val (w, h) = if (Build.VERSION.SDK_INT >= 30) {
            windowManager.maximumWindowMetrics.bounds.let { it.width() to it.height() }
        } else {
            @Suppress("DEPRECATION")
            Point().also { windowManager.defaultDisplay.getRealSize(it) }.let { it.x to it.y }
        }
        val want = if (portraitOnly((minOf(w, h) / resources.displayMetrics.density).toInt())) {
            ActivityInfo.SCREEN_ORIENTATION_PORTRAIT
        } else {
            ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
        }
        if (requestedOrientation != want) requestedOrientation = want
    }

    /** The end of onboarding: it does not show again, and the picked skin applies once the core takes it. */
    private fun onboarded(skin: Skin?) {
        prefs.edit { putBoolean(ONBOARDING, false).putBoolean(ASKED, true) }
        onboarding = false
        val app = applicationContext
        if (skin != null) Remote.scope.launch(Dispatchers.Main) {
            Remote.ops.first { "theme" in it }
            Skins.apply(app, skin)
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        if (intent.getBooleanExtra(EXTRA_OPEN_PLAYER, false)) Nav.openPlayer()
        startCore(linkOf(intent)) // also starts a core that ended with an error
        shortcut(intent)
    }

    /** The launcher shortcuts (res/xml/shortcuts.xml) and the tile's fallback. */
    private fun shortcut(intent: Intent) {
        when (intent.action) {
            ACTION_SEARCH -> Nav.select(Tab.Search)
            ACTION_LIKED -> {
                Nav.select(Tab.Library)
                Nav.open(Screen.Playlist("ytmusic", "fav", getString(R.string.lib_liked)))
            }
            ACTION_RESUME -> ContextCompat.startForegroundService(this, Intent(this, CliampService::class.java).setAction(CliampService.ACTION_RESUME))
            MediaStore.INTENT_ACTION_MEDIA_PLAY_FROM_SEARCH -> {
                val query = intent.getStringExtra(SearchManager.QUERY)
                if (query.isNullOrBlank()) return shortcut(Intent(ACTION_RESUME))
                Remote.scope.launch {
                    Remote.connected.first { it }
                    try {
                        Auto.playSearch(query)
                    } catch (e: CancellationException) {
                        throw e
                    } catch (e: Exception) {
                        Remote.showError(e)
                    }
                }
            }
        }
    }

    /** At most once a day, looks for a newer release for the Inicio banner. Installing stays the user's choice. */
    private fun offerUpdate() {
        File(cacheDir, "updates").deleteRecursively() // the APK of an update that is installed by now
        if (!Updates.due(this)) return
        lifecycleScope.launch {
            val release = withContext(Dispatchers.IO) { runCatching { Updates.check(this@MainActivity) }.getOrNull() }
            if (release != null && release.newer && release.apk != null) Updates.offer.value = release.latest
        }
    }

    private fun startCore(link: String?) {
        ContextCompat.startForegroundService(this, Intent(this, CliampService::class.java).putExtra(CliampService.EXTRA_LINK, link))
    }

    /** A cliamp-x:// link, or a shared http(s) URL turned into one, for `cliamp-x open`. */
    private fun linkOf(intent: Intent): String? = when (intent.action) {
        Intent.ACTION_VIEW -> intent.dataString?.takeIf { it.startsWith("cliamp-x:") }
        Intent.ACTION_SEND -> intent.getStringExtra(Intent.EXTRA_TEXT)
            ?.let { Regex("https?://\\S+").find(it)?.value }
            ?.let { "cliamp-x://play?url=" + Uri.encode(it) }
        else -> null
    }

    companion object {
        private const val ASKED = "asked_permissions"
        private const val ONBOARDING = "onboarding"

        /** The notification opens the player sheet. */
        const val EXTRA_OPEN_PLAYER = "open_player"

        const val ACTION_RESUME = "io.github.xaviiromf.cliampx.RESUME"
        const val ACTION_SEARCH = "io.github.xaviiromf.cliampx.SEARCH"
        const val ACTION_LIKED = "io.github.xaviiromf.cliampx.LIKED"

        /** A screen under 600 dp on its short side is a phone's (the window size class "compact"). */
        fun portraitOnly(smallestWidthDp: Int) = smallestWidthDp < 600

        fun permissions() = buildList {
            if (Build.VERSION.SDK_INT >= 33) add(Manifest.permission.POST_NOTIFICATIONS)
            add(musicPermission())
        }

        fun musicPermission() =
            if (Build.VERSION.SDK_INT >= 33) Manifest.permission.READ_MEDIA_AUDIO else Manifest.permission.READ_EXTERNAL_STORAGE
    }
}
