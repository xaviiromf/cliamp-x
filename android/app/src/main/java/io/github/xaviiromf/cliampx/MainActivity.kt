package io.github.xaviiromf.cliampx

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.media.AudioManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.testTagsAsResourceId
import androidx.core.content.ContextCompat
import androidx.core.content.edit
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import io.github.xaviiromf.cliampx.ui.App
import io.github.xaviiromf.cliampx.ui.CliampTheme
import io.github.xaviiromf.cliampx.ui.Nav
import io.github.xaviiromf.cliampx.ui.livePalette
import com.yausername.youtubedl_android.YoutubeDL
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.drop
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File

/** The native touch UI (UI-SPEC §7.1). The service owns the core; this only talks to it over IPC. */
class MainActivity : ComponentActivity() {
    private val prefs by lazy { getSharedPreferences("ui", MODE_PRIVATE) }

    private val askPermissions = registerForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { startCore(linkOf(intent)) }

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        volumeControlStream = AudioManager.STREAM_MUSIC
        // Python is unpacked by the first start of the core.
        val firstRun = !File(noBackupFilesDir, "${YoutubeDL.baseName}/packages/python").exists()
        setContent {
            val theme by remember { Remote.state.map { it?.theme }.distinctUntilChanged() }.collectAsStateWithLifecycle(null)
            CliampTheme(livePalette(theme)) {
                Box(Modifier.semantics { testTagsAsResourceId = true }) { App(firstRun) }
            }
        }

        if (savedInstanceState == null && intent.getBooleanExtra(EXTRA_OPEN_PLAYER, false)) Nav.openPlayer()
        // The first start asks for notifications and local music before the core reads the music folder.
        val link = if (savedInstanceState == null) linkOf(intent) else null
        val missing = permissions().filter { ContextCompat.checkSelfPermission(this, it) != PackageManager.PERMISSION_GRANTED }
        if (missing.isNotEmpty() && !prefs.getBoolean(ASKED, false)) {
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

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        if (intent.getBooleanExtra(EXTRA_OPEN_PLAYER, false)) Nav.openPlayer()
        startCore(linkOf(intent)) // also starts a core that ended with an error
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

        /** The notification opens the player sheet. */
        const val EXTRA_OPEN_PLAYER = "open_player"

        fun permissions() = buildList {
            if (Build.VERSION.SDK_INT >= 33) add(Manifest.permission.POST_NOTIFICATIONS)
            add(musicPermission())
        }

        fun musicPermission() =
            if (Build.VERSION.SDK_INT >= 33) Manifest.permission.READ_MEDIA_AUDIO else Manifest.permission.READ_EXTERNAL_STORAGE
    }
}
