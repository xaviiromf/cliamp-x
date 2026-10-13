package io.github.xaviiromf.cliampx

import android.annotation.SuppressLint
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.provider.Settings
import android.util.TypedValue
import android.widget.Button
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.net.toUri
import androidx.core.view.WindowCompat
import java.io.ByteArrayOutputStream

/** Sign-in, local music, battery and quit. */
class SettingsActivity : AppCompatActivity() {
    private val importCookies = registerForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) importCookies(uri)
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
        button(R.string.battery) { ignoreBatteryOptimizations() }
        // Phase 4 adds "Buscar actualizaciones" and "Actualizar yt-dlp" here.
        text(R.string.quit_help)
        button(R.string.quit) {
            CliampService.send(this, CliampService.ACTION_QUIT)
            finishAffinity()
        }

        val scroll = ScrollView(this).apply { addView(list) }
        padForSystemBars(scroll)
        setContentView(scroll)
    }

    @SuppressLint("BatteryLife") // a music player that One UI must not suspend; no Play Store
    private fun ignoreBatteryOptimizations() {
        startActivity(Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, "package:$packageName".toUri()))
    }

    private fun importCookies(uri: Uri) {
        val text = runCatching {
            contentResolver.openInputStream(uri)?.use { input ->
                val out = ByteArrayOutputStream()
                val buffer = ByteArray(8192)
                while (out.size() <= MAX_COOKIES) out.write(buffer, 0, input.read(buffer).takeIf { it >= 0 } ?: break)
                out.toString(Charsets.UTF_8.name())
            }
        }.getOrNull()
        val message = when {
            text == null -> R.string.import_failed
            text.length > MAX_COOKIES || text.lineSequence().none { it.split('\t').size >= 7 } -> R.string.not_cookies
            !text.contains("\tSAPISID\t") -> R.string.no_sign_in
            else -> {
                LoginActivity.saveSignIn(this, text)
                R.string.signed_in
            }
        }
        Toast.makeText(this, message, Toast.LENGTH_LONG).show()
    }

    companion object {
        private const val MAX_COOKIES = 1 shl 20
    }
}
