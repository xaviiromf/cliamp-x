package io.github.xaviiromf.cliampx.ui

import android.annotation.SuppressLint
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Environment
import android.os.PowerManager
import android.provider.Settings
import android.widget.Toast
import androidx.activity.compose.LocalActivity
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.core.content.edit
import androidx.core.net.toUri
import androidx.lifecycle.compose.LifecycleResumeEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.BuildConfig
import io.github.xaviiromf.cliampx.CliampService
import io.github.xaviiromf.cliampx.Core
import io.github.xaviiromf.cliampx.LoginActivity
import io.github.xaviiromf.cliampx.MainActivity
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Snapshot
import io.github.xaviiromf.cliampx.TerminalActivity
import io.github.xaviiromf.cliampx.Updates
import io.github.xaviiromf.cliampx.Ytdlp
import io.github.xaviiromf.cliampx.normalizeValue
import io.github.xaviiromf.cliampx.speedLabel
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.io.File

/** The name the core gives its terminal-colors theme, which Android draws as winamp (§4.1). */
private const val DEFAULT_THEME = "Default - Terminal colors"

/** The four native visualizers (§0) and their labels; any other mode draws only in Modo terminal. */
private val NATIVE_VIS = listOf("Bars" to R.string.vis_bars, "ClassicPeak" to R.string.vis_classic_peak, "Mirror" to R.string.vis_mirror, "None" to R.string.vis_none)

@SuppressLint("BatteryLife") // a music player that One UI must not suspend; no Play Store
fun ignoreBatteryOptimizations(context: Context) {
    context.startActivity(Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, "package:${context.packageName}".toUri()))
}

/**
 * Ajustes (§2.5, §3.14): appearance, playback, the account, local music,
 * then the system: battery, app updates (phase 4), yt-dlp, Modo terminal
 * and Salir. Every setting goes to the core, which saves it and reports it
 * back in the snapshot.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun SettingsScreen(screen: Screen.Settings) {
    val p = LocalPalette.current
    val context = LocalContext.current
    val activity = LocalActivity.current
    val s by Remote.state.collectAsStateWithLifecycle()
    val ops by Remote.ops.collectAsStateWithLifecycle()
    val signedIn by Remote.signedIn.collectAsStateWithLifecycle()
    val ytdlp by Ytdlp.status.collectAsStateWithLifecycle()
    val prefs = remember { Updates.prefs(context) }
    var nightly by remember { mutableStateOf(prefs.getBoolean(Updates.NIGHTLY, false)) }
    var optimized by remember { mutableStateOf(false) }
    var music by remember { mutableStateOf(true) }
    val update = remember { UpdateFlow() }
    val scope = rememberCoroutineScope()
    LifecycleResumeEffect(Unit) {
        // Back from the battery, permission or install setting: read them again.
        optimized = !context.getSystemService(PowerManager::class.java).isIgnoringBatteryOptimizations(context.packageName)
        music = ContextCompat.checkSelfPermission(context, MainActivity.musicPermission()) == PackageManager.PERMISSION_GRANTED
        activity?.let { update.resumed(it) }
        onPauseOrDispose {}
    }
    // The banner's [ Instalar ]: check and install without asking again, once per visit.
    var installed by rememberSaveable { mutableStateOf(false) }
    LaunchedEffect(Unit) {
        if (screen.install && !installed && activity != null) {
            installed = true
            update.check(activity, scope, ask = false)
        }
    }
    val importCookies = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) Toast.makeText(context, LoginActivity.importCookies(context, uri), Toast.LENGTH_LONG).show()
    }
    // The core reads the music folder when it starts.
    val askMusic = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        music = granted
        if (granted) CliampService.send(context, CliampService.ACTION_RESTART)
    }
    val folder by produceState(Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_MUSIC).path) {
        withContext(Dispatchers.IO) { musicDir(Core.configFile(context)) }?.let { value = it }
    }
    fun set(name: String, value: Int) = Remote.fire("settings.set", JSONObject().put("name", name).put("value", value))
    val settable = "settings.set" in ops
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.settings))
        LazyColumn(Modifier.fillMaxSize().testTag("settings_list")) {
            item(key = "look") {
                Column {
                    SectionRule(stringResource(R.string.set_look))
                    if ("theme" in ops) SettingRow(stringResource(R.string.theme), themeLabel(s?.theme?.name.orEmpty()), "set_theme") { Nav.open(Screen.Themes) }
                    if ("vis" in ops) SettingRow(stringResource(R.string.visualizer), s?.visualizer.orEmpty(), "set_vis") { Nav.open(Screen.Visualizers) }
                }
            }
            item(key = "playback") {
                Column {
                    SectionRule(stringResource(R.string.set_playback))
                    if ("eq" in ops) SettingRow(stringResource(R.string.eq), eqLabel(s), "set_eq") { Nav.open(Screen.Eq) }
                    SettingRow(stringResource(R.string.speed), speedLabel(s?.speed ?: 1.0), "set_speed") { Nav.sheet = Sheet.Speed }
                    SettingRow(stringResource(R.string.sleep), sleepLabel(s), "set_sleep") { Nav.sheet = Sheet.Sleep }
                    if (settable) {
                        normalizeLabel(s)?.let { SettingRow(stringResource(R.string.normalize), it, "set_normalize") { Nav.sheet = Sheet.Normalize } }
                        SettingRow(stringResource(R.string.crossfade), crossfadeLabel(s?.crossfade ?: 0), "set_crossfade") { Nav.sheet = Sheet.Crossfade }
                        Toggle(stringResource(R.string.eq_auto_genre), s?.eqAuto == true, "set_eq_auto") { set("eq_auto", it) }
                        Toggle(stringResource(R.string.auto_radio), s?.autoRadio == true, "set_auto_radio") { set("auto_radio", it) }
                        Toggle(stringResource(R.string.mono), s?.mono == true, "set_mono") { set("mono", it) }
                    }
                }
            }
            item(key = "account") {
                Column {
                    SectionRule(stringResource(R.string.set_account))
                    Line(
                        stringResource(R.string.youtube_music) + " · " + stringResource(if (signedIn == true) R.string.signed_in_state else R.string.signed_out_state),
                        if (signedIn == true) p.playing else p.textSecondary,
                    )
                    Help(stringResource(R.string.youtube_music_help))
                    FlowRow(Modifier.padding(horizontal = 12.dp)) {
                        BracketButton(stringResource(R.string.sign_in_short), Modifier.testTag("set_signin")) {
                            context.startActivity(Intent(context, LoginActivity::class.java))
                        }
                        BracketButton(stringResource(R.string.import_cookies), Modifier.testTag("set_import")) {
                            importCookies.launch(arrayOf("text/*", "application/octet-stream"))
                        }
                        BracketButton(stringResource(R.string.sign_out), Modifier.testTag("set_signout")) { LoginActivity.signOut(context) }
                    }
                }
            }
            item(key = "local") {
                Column {
                    SectionRule(stringResource(R.string.lib_local))
                    Line(stringResource(R.string.music_folder, folder), p.textPrimary)
                    Help(stringResource(R.string.music_help))
                    if (!music) BracketButton(stringResource(R.string.allow_music), Modifier.padding(horizontal = 12.dp).testTag("set_music")) { askMusic.launch(MainActivity.musicPermission()) }
                }
            }
            item(key = "system") {
                Column {
                    SectionRule(stringResource(R.string.set_system))
                    Row(Modifier.fillMaxWidth().padding(start = 16.dp, end = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text(stringResource(if (optimized) R.string.battery_optimized else R.string.battery_free), Modifier.weight(1f), color = p.textPrimary, style = Type.body)
                        if (optimized) BracketButton(stringResource(R.string.battery_remove), Modifier.testTag("set_battery")) { ignoreBatteryOptimizations(context) }
                    }
                    Help(stringResource(R.string.battery_help))
                    Row(Modifier.fillMaxWidth().padding(start = 16.dp, end = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text(stringResource(R.string.updates), Modifier.weight(1f), color = p.textPrimary, style = Type.body)
                        for ((label, on) in listOf(R.string.channel_stable to false, R.string.channel_nightly to true)) {
                            BracketChip(stringResource(label), nightly == on, Modifier.testTag(if (on) "set_nightly" else "set_stable")) {
                                nightly = on
                                prefs.edit { putBoolean(Updates.NIGHTLY, on) }
                            }
                        }
                    }
                    FlowRow(Modifier.padding(start = 16.dp, end = 12.dp), itemVerticalAlignment = Alignment.CenterVertically) {
                        Text(stringResource(R.string.version, BuildConfig.VERSION_NAME) + " · ", color = p.textPrimary, style = Type.body)
                        BracketButton(stringResource(R.string.check_updates), Modifier.testTag("set_update")) { activity?.let { update.check(it, scope, ask = true) } }
                    }
                    UpdateStatus(update, scope)
                    Help(stringResource(R.string.updates_help))
                    FlowRow(Modifier.padding(start = 16.dp, end = 12.dp), itemVerticalAlignment = Alignment.CenterVertically) {
                        Text("yt-dlp " + (Ytdlp.tag(context) ?: stringResource(R.string.ytdlp_bundled)) + " · ", color = p.textPrimary, style = Type.body)
                        if (ytdlp == Ytdlp.Status.Updating) {
                            Row(Modifier.heightIn(min = 48.dp), verticalAlignment = Alignment.CenterVertically) { SpinnerLine(stringResource(R.string.ytdlp_updating), Type.body) }
                        } else {
                            BracketButton(stringResource(R.string.update_ytdlp), Modifier.testTag("set_ytdlp")) { Remote.scope.launch { Remote.updateYtdlp() } }
                        }
                    }
                    when (val y = ytdlp) {
                        is Ytdlp.Status.Updated -> Line(stringResource(R.string.ytdlp_done, y.tag), p.playing)
                        is Ytdlp.Status.UpToDate -> Line(stringResource(R.string.ytdlp_current, y.tag), p.textSecondary)
                        is Ytdlp.Status.Failed -> Line("ERR: " + y.message, p.error)
                        else -> {}
                    }
                    SettingRow(stringResource(R.string.terminal), "", "set_terminal") { context.startActivity(Intent(context, TerminalActivity::class.java)) }
                    Help(stringResource(R.string.quit_help))
                    BracketButton(stringResource(R.string.quit), Modifier.padding(horizontal = 12.dp).testTag("set_quit"), color = p.error) {
                        CliampService.send(context, CliampService.ACTION_QUIT)
                        activity?.finishAffinity()
                    }
                    Spacer(Modifier.height(16.dp))
                }
            }
        }
    }
}

/** A 56 dp row: the label, `[value]` in accent, and `›` when it opens something. */
@Composable
private fun SettingRow(label: String, value: String, tag: String, onClick: () -> Unit) {
    val p = LocalPalette.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = 56.dp).clickable(onClick = onClick).padding(horizontal = 16.dp).testTag(tag),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text(label, Modifier.weight(1f), color = p.textPrimary, style = Type.body)
        if (value.isNotEmpty()) Bracketed(value, false)
        Text("›", color = p.textSecondary, style = Type.body)
    }
}

/** A switch row: `[On]` in accent bold, `[Off]` dim brackets; a tap flips it (settings.set 0 or 1). */
@Composable
private fun Toggle(label: String, on: Boolean, tag: String, set: (Int) -> Unit) {
    val p = LocalPalette.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = 56.dp).clickable { set(if (on) 0 else 1) }.padding(horizontal = 16.dp).testTag(tag),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, Modifier.weight(1f), color = p.textPrimary, style = Type.body)
        Bracketed(stringResource(if (on) R.string.on else R.string.off), on)
    }
}

/** `[value]` as a bracket chip draws it (§4.4), without its own touch target. */
@Composable
private fun Bracketed(value: String, on: Boolean) {
    val p = LocalPalette.current
    val bracket = if (on) p.accent else p.textSecondary
    Text(
        buildAnnotatedString {
            withStyle(SpanStyle(color = bracket)) { append("[") }
            withStyle(SpanStyle(color = p.accent)) { append(value) }
            withStyle(SpanStyle(color = bracket)) { append("]") }
        },
        style = Type.body.bold(on), maxLines = 1, overflow = TextOverflow.Ellipsis,
    )
}

/** A dim explanation under a setting. */
@Composable
private fun Help(text: String) {
    Text(text, Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp), color = LocalPalette.current.textSecondary, style = Type.label)
}

/** A dim or colored text line of a section. */
@Composable
private fun Line(text: String, color: Color) {
    Text(text, Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp), color = color, style = Type.body)
}

@Composable
private fun themeLabel(name: String) = if (name == DEFAULT_THEME || name.isEmpty()) stringResource(R.string.theme_default) else name

@Composable
private fun eqLabel(s: Snapshot?) = s?.eqPreset?.takeUnless { it.isEmpty() || it == "Custom" } ?: stringResource(R.string.eq_custom)

@Composable
private fun sleepLabel(s: Snapshot?): String = when {
    s == null || s.sleepMinutes == 0 -> stringResource(R.string.sleep_off_state)
    s.sleepMinutes == -1 -> stringResource(R.string.sleep_chip_end)
    else -> sleepClock(s).orEmpty()
}

/** The normalization as the sheet names it, or null when the player has none. */
@Composable
private fun normalizeLabel(s: Snapshot?): String? =
    NORMALIZE.getOrNull(normalizeValue(s?.normalize.orEmpty()))?.let { stringResource(it) }

private val NORMALIZE = listOf(R.string.normalize_off, R.string.normalize_track, R.string.normalize_album)

@Composable
private fun crossfadeLabel(secs: Int) = if (secs <= 0) stringResource(R.string.off) else stringResource(R.string.seconds, secs)

/** music_dir from config.toml, as the core reads it, or null. */
private fun musicDir(config: File): String? =
    runCatching { config.readLines() }.getOrNull()?.firstNotNullOfOrNull { Regex("""^\s*music_dir\s*=\s*"(.*)"\s*$""").find(it)?.groupValues?.get(1) }

/**
 * The app update of phase 4 (Updates.kt): ask the core for the newest
 * release, offer it, download and check it, then open the installer. When
 * Android does not let the app install yet, the install waits for the
 * user to come back from that setting.
 */
private class UpdateFlow {
    var status by mutableStateOf("")
    var offer by mutableStateOf<Updates.Apk?>(null)
    var latest = ""
    private var pending: File? = null
    private var job: Job? = null

    fun check(activity: android.app.Activity, scope: kotlinx.coroutines.CoroutineScope, ask: Boolean) {
        if (job?.isActive == true) return
        offer = null
        status = activity.getString(R.string.update_checking)
        job = scope.launch {
            val release = try {
                withContext(Dispatchers.IO) { Updates.check(activity) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                status = Updates.message(activity, e)
                return@launch
            }
            val apk = release.apk
            latest = release.latest
            when {
                !release.newer -> status = activity.getString(R.string.update_none, release.latest)
                apk == null -> status = activity.getString(R.string.update_no_apk, release.latest)
                !ask -> install(activity, apk)
                else -> {
                    status = activity.getString(R.string.update_available, release.latest)
                    offer = apk
                }
            }
        }
    }

    fun accept(activity: android.app.Activity, scope: kotlinx.coroutines.CoroutineScope) {
        val apk = offer ?: return
        offer = null
        job = scope.launch { install(activity, apk) }
    }

    fun decline() {
        offer = null
        status = ""
    }

    /** Downloads apk, checks its SHA-256 and opens the system installer. */
    private suspend fun install(activity: android.app.Activity, apk: Updates.Apk) {
        status = activity.getString(R.string.update_downloading, apk.name)
        val file = try {
            Updates.download(File(activity.cacheDir, "updates"), apk)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            status = Updates.message(activity, e)
            return
        }
        status = ""
        if (!Updates.install(activity, file)) {
            pending = file
            status = activity.getString(R.string.update_allow)
        }
    }

    /** Back from the setting that lets cliamp-x install apps. */
    fun resumed(activity: android.app.Activity) {
        val file = pending ?: return
        if (activity.packageManager.canRequestPackageInstalls()) {
            pending = null
            status = ""
            Updates.install(activity, file)
        }
    }
}

/** The update check's line, with Instalar and Ahora no while a newer release is offered. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun UpdateStatus(update: UpdateFlow, scope: kotlinx.coroutines.CoroutineScope) {
    val p = LocalPalette.current
    val activity = LocalActivity.current
    if (update.status.isEmpty()) return
    Text(update.status, Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp).testTag("update_status"), color = p.textSecondary, style = Type.label)
    if (update.offer != null && activity != null) {
        FlowRow(Modifier.padding(horizontal = 12.dp)) {
            BracketButton(stringResource(R.string.update_install), Modifier.testTag("update_install")) { update.accept(activity, scope) }
            BracketButton(stringResource(R.string.later)) { update.decline() }
        }
    }
}

/**
 * Tema (§3.14): every theme with its seven colors; a tap applies it at once
 * and the whole UI, this list too, recolors from the snapshot (§4.6).
 */
@Composable
fun ThemesScreen() {
    val p = LocalPalette.current
    val s by Remote.state.collectAsStateWithLifecycle()
    val (load, retry) = rememberLoad("themes") { Remote.themes() }
    val current = s?.theme?.name.orEmpty().ifEmpty { DEFAULT_THEME }
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.theme))
        LazyColumn(Modifier.fillMaxSize()) {
            if (load !is Load.Done) item(key = "state") { LoadState(load, retry) }
            itemsIndexed((load as? Load.Done)?.value.orEmpty(), key = { _, t -> t.name }) { _, t ->
                val on = t.name == current
                // The active theme shows the colors on screen: for cover, the ones of this song's cover.
                val colors = paletteOf(if (on) s?.theme ?: t else t).let { listOf(it.bg, it.accent, it.brightFg, it.fg, it.green, it.yellow, it.red) }
                Row(
                    Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable { Remote.fire("theme", JSONObject().put("name", t.name)) }
                        .padding(horizontal = 16.dp).testTag("theme_" + if (t.name == DEFAULT_THEME) "default" else t.name),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(if (on) "● " else "  ", color = p.accent, style = Type.body)
                    Text(
                        when (t.name) {
                            DEFAULT_THEME -> stringResource(R.string.theme_default)
                            "cover" -> stringResource(R.string.theme_cover)
                            else -> t.name
                        },
                        Modifier.weight(1f), color = if (on) p.accent else p.textPrimary, style = Type.body.bold(on), maxLines = 1, overflow = TextOverflow.Ellipsis,
                    )
                    // The seven colors as squares, outlined so the theme's bg shows on any bg.
                    Row(horizontalArrangement = Arrangement.spacedBy(2.dp)) {
                        colors.forEach { Box(Modifier.size(14.dp).background(it).border(1.dp, p.divider)) }
                    }
                }
            }
        }
    }
}

/**
 * Visualizador (§3.14): a live 120 dp preview while music plays, the
 * native modes, and the way to the rest in Modo terminal.
 */
@Composable
fun VisualizersScreen() {
    val p = LocalPalette.current
    val context = LocalContext.current
    val s by Remote.state.collectAsStateWithLifecycle()
    val (all, _) = rememberLoad("vis:list") { Remote.visualizers() }
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.visualizer))
        LazyColumn(Modifier.fillMaxSize()) {
            s?.takeIf { it.playing }?.let { snap ->
                item(key = "preview") { Visualizer(snap, Modifier.fillMaxWidth().height(120.dp).padding(horizontal = 16.dp, vertical = 8.dp).testTag("vis_preview")) }
            }
            itemsIndexed(NATIVE_VIS, key = { _, v -> v.first }) { _, (name, label) ->
                val on = s?.visualizer == name
                Row(
                    Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable { Remote.fire("vis", JSONObject().put("name", name)) }
                        .padding(horizontal = 16.dp).testTag("vis_$name"),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(if (on) "● " else "  ", color = p.accent, style = Type.body)
                    Text(name, Modifier.weight(1f), color = if (on) p.accent else p.textPrimary, style = Type.body.bold(on))
                    Text(stringResource(label), Modifier.weight(1f), color = p.textSecondary, style = Type.label)
                }
            }
            item(key = "more") {
                val n = (all as? Load.Done)?.value?.size
                SettingRow(
                    if (n != null) stringResource(R.string.vis_more_count, n) else stringResource(R.string.vis_more), "", "vis_terminal",
                ) { context.startActivity(Intent(context, TerminalActivity::class.java)) }
            }
        }
    }
}

/** Normalización de volumen (§3.15): off, by song or by album. */
@Composable
fun NormalizeSheet(s: Snapshot?, close: () -> Unit) {
    val p = LocalPalette.current
    val current = normalizeValue(s?.normalize.orEmpty())
    SectionRule(stringResource(R.string.normalize))
    NORMALIZE.forEachIndexed { v, label ->
        Row(
            Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable {
                Remote.fire("settings.set", JSONObject().put("name", "normalize").put("value", v))
                close()
            }.padding(horizontal = 16.dp).testTag("normalize_$v"),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(if (v == current) "● " else "  ", color = p.accent, style = Type.body)
            Text(stringResource(label), color = if (v == current) p.accent else p.textPrimary, style = Type.body.bold(v == current))
        }
    }
}

/** Fundido entre canciones (§3.15): off, or 2 to 12 seconds. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun CrossfadeSheet(s: Snapshot?, close: () -> Unit) {
    SectionRule(stringResource(R.string.crossfade))
    FlowRow(Modifier.padding(horizontal = 12.dp)) {
        for (secs in 0..12 step 2) {
            BracketChip(crossfadeLabel(secs), (s?.crossfade ?: 0) == secs, Modifier.testTag("crossfade_$secs")) {
                Remote.fire("settings.set", JSONObject().put("name", "crossfade").put("value", secs))
                close()
            }
        }
    }
}
