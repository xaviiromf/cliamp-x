package io.github.xaviiromf.cliampx

import android.annotation.SuppressLint
import android.content.Context
import android.content.Intent
import android.os.Bundle
import android.provider.Settings
import android.util.TypedValue
import android.view.View
import android.widget.Button
import android.widget.LinearLayout
import android.widget.RadioButton
import android.widget.RadioGroup
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.edit
import androidx.core.net.toUri
import androidx.core.view.WindowCompat
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File

/** Sign-in, local music, battery, updates, Modo terminal and quit. */
class SettingsActivity : AppCompatActivity() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    private lateinit var status: TextView
    private var updating: Job? = null

    /** A verified update that waits for the permission to install apps. */
    private var pending: File? = null

    private val importCookies = registerForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) Toast.makeText(this, LoginActivity.importCookies(this, uri), Toast.LENGTH_LONG).show()
    }

    private val askMusic = registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        // The core reads the music folder when it starts.
        if (granted) CliampService.send(this, CliampService.ACTION_RESTART)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        WindowCompat.setDecorFitsSystemWindows(window, false)
        val pad = (16 * resources.displayMetrics.density).toInt()
        val list = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(pad, pad, pad, pad)
        }
        fun text(id: Int, size: Float = 14f) = list.addView(TextView(this).apply {
            setText(id)
            setTextSize(TypedValue.COMPLEX_UNIT_SP, size)
            setPadding(0, pad, 0, pad / 2)
        })
        fun button(id: Int, action: () -> Unit) = list.addView(Button(this).apply {
            setText(id)
            setOnClickListener { action() }
        })

        text(R.string.settings, 22f)
        text(R.string.youtube_music_help)
        button(R.string.sign_in) { startActivity(Intent(this, LoginActivity::class.java)) }
        button(R.string.import_cookies) { importCookies.launch(arrayOf("text/*", "application/octet-stream")) }
        text(R.string.music_help)
        button(R.string.allow_music) { askMusic.launch(MainActivity.musicPermission()) }
        text(R.string.battery_help)
        button(R.string.battery) { ignoreBatteryOptimizations(this) }
        text(R.string.updates_help)
        val prefs = Updates.prefs(this)
        list.addView(RadioGroup(this).apply {
            orientation = RadioGroup.HORIZONTAL
            for ((label, nightly) in listOf(R.string.channel_stable to false, R.string.channel_nightly to true)) {
                addView(RadioButton(context).apply {
                    id = View.generateViewId()
                    setText(label)
                    isChecked = prefs.getBoolean(Updates.NIGHTLY, false) == nightly
                    setOnClickListener { prefs.edit { putBoolean(Updates.NIGHTLY, nightly) } }
                })
            }
        })
        button(R.string.check_updates) { checkForUpdates(ask = true) }
        status = TextView(this).apply { text = getString(R.string.update_current, BuildConfig.VERSION_NAME) }
        list.addView(status)
        list.addView(Button(this).apply {
            setText(R.string.update_ytdlp)
            setOnClickListener { updateYtdlp(this) }
        })
        button(R.string.terminal) { startActivity(Intent(this, TerminalActivity::class.java)) }
        text(R.string.quit_help)
        button(R.string.quit) {
            CliampService.send(this, CliampService.ACTION_QUIT)
            finishAffinity()
        }

        val scroll = ScrollView(this).apply { addView(list) }
        padForSystemBars(scroll)
        setContentView(scroll)
        // The daily check of MainActivity found an update and the user chose to install it.
        if (savedInstanceState == null && intent.getBooleanExtra(EXTRA_UPDATE, false)) checkForUpdates(ask = false)
    }

    override fun onResume() {
        super.onResume()
        // Back from the setting that lets cliamp-x install apps.
        val file = pending ?: return
        if (packageManager.canRequestPackageInstalls()) {
            pending = null
            status.text = ""
            Updates.install(this, file)
        }
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }

    /** Asks the core for the newest release; ask confirms before the download. */
    private fun checkForUpdates(ask: Boolean) {
        if (updating?.isActive == true) return
        status.setText(R.string.update_checking)
        updating = scope.launch {
            val release = try {
                withContext(Dispatchers.IO) { Updates.check(this@SettingsActivity) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                status.text = Updates.message(this@SettingsActivity, e)
                return@launch
            }
            val apk = release.apk
            when {
                !release.newer -> status.text = getString(R.string.update_none, release.latest)
                apk == null -> status.text = getString(R.string.update_no_apk, release.latest)
                !ask -> install(apk)
                else -> AlertDialog.Builder(this@SettingsActivity)
                    .setMessage(getString(R.string.update_available, release.latest))
                    .setPositiveButton(R.string.update_install) { _, _ -> updating = scope.launch { install(apk) } }
                    .setNegativeButton(R.string.later) { _, _ -> status.text = "" }
                    .show()
            }
        }
    }

    /** Downloads apk, checks its SHA-256 and opens the system installer. */
    private suspend fun install(apk: Updates.Apk) {
        status.text = getString(R.string.update_downloading, apk.name)
        val file = try {
            Updates.download(File(cacheDir, "updates"), apk)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            status.text = Updates.message(this, e)
            return
        }
        status.text = ""
        if (!Updates.install(this, file)) {
            pending = file
            status.setText(R.string.update_allow)
        }
    }

    /**
     * Updates yt-dlp in place on the channel of the app, then plays a failed
     * song again: the core runs the new yt-dlp from its next song.
     */
    private fun updateYtdlp(button: Button) {
        Toast.makeText(this, R.string.ytdlp_updating, Toast.LENGTH_SHORT).show()
        button.isEnabled = false
        scope.launch {
            val message = when (val status = Remote.updateYtdlp()) {
                is Ytdlp.Status.Updated -> getString(R.string.ytdlp_done, status.tag)
                is Ytdlp.Status.UpToDate -> getString(R.string.ytdlp_current, status.tag)
                is Ytdlp.Status.Failed -> status.message
                else -> null
            }
            button.isEnabled = true
            if (message != null) Toast.makeText(this@SettingsActivity, message, Toast.LENGTH_LONG).show()
        }
    }

    companion object {
        const val EXTRA_UPDATE = "update"

        @SuppressLint("BatteryLife") // a music player that One UI must not suspend; no Play Store
        fun ignoreBatteryOptimizations(context: Context) {
            context.startActivity(Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, "package:${context.packageName}".toUri()))
        }
    }
}
