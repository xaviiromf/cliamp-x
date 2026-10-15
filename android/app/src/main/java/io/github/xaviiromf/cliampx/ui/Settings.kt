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
import androidx.compose.material3.Slider
import androidx.compose.material3.Text
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import kotlin.math.roundToInt
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
import io.github.xaviiromf.cliampx.LauncherIcon
import io.github.xaviiromf.cliampx.Language
import io.github.xaviiromf.cliampx.LoginActivity
import io.github.xaviiromf.cliampx.Logs
import io.github.xaviiromf.cliampx.IpcError
import io.github.xaviiromf.cliampx.MainActivity
import io.github.xaviiromf.cliampx.Offline
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Routes
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
internal const val DEFAULT_THEME = "Default - Terminal colors"

/** The label of each built-in visualizer mode (§0); a Lua mode shows its name only. */
private val VIS_LABELS = mapOf(
    "Bars" to R.string.vis_bars, "ClassicPeak" to R.string.vis_classic_peak, "ClassicLED" to R.string.vis_classic_led,
    "Bricks" to R.string.vis_bricks, "Columns" to R.string.vis_columns, "BarsDot" to R.string.vis_bars_dot,
    "BarsOutline" to R.string.vis_bars_outline, "Mirror" to R.string.vis_mirror, "Butterfly" to R.string.vis_butterfly,
    "Ascii" to R.string.vis_ascii, "Mosaic" to R.string.vis_mosaic, "Logo" to R.string.vis_logo,
    "Spectrogram" to R.string.vis_spectrogram, "Terrain" to R.string.vis_terrain, "Skyline" to R.string.vis_skyline,
    "Radial" to R.string.vis_radial, "Pulse" to R.string.vis_pulse, "Ripple" to R.string.vis_ripple,
    "Tunnel" to R.string.vis_tunnel, "Spiral" to R.string.vis_spiral, "Helix" to R.string.vis_helix,
    "Plasma" to R.string.vis_plasma, "Aurora" to R.string.vis_aurora, "Retro" to R.string.vis_retro,
    "Flame" to R.string.vis_flame, "Rain" to R.string.vis_rain, "Matrix" to R.string.vis_matrix,
    "Scatter" to R.string.vis_scatter, "Geyser" to R.string.vis_geyser, "Firework" to R.string.vis_firework,
    "Bubbles" to R.string.vis_bubbles, "Sakura" to R.string.vis_sakura, "Firefly" to R.string.vis_firefly,
    "Starfield" to R.string.vis_starfield, "Wave" to R.string.vis_wave, "Scope" to R.string.vis_scope,
    "Heartbeat" to R.string.vis_heartbeat, "Binary" to R.string.vis_binary, "Sand" to R.string.vis_sand,
    "Stereo" to R.string.vis_stereo, "Omarchy" to R.string.vis_omarchy, "RedSector" to R.string.vis_red_sector,
    "YinYang" to R.string.vis_yin_yang, "CoverPulse" to R.string.vis_cover_pulse, "CoverBars" to R.string.vis_cover_bars,
    "CoverShatter" to R.string.vis_cover_shatter, "None" to R.string.vis_none,
)

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
    var resumeOnConnect by remember { mutableStateOf(Routes.prefs(context).getBoolean(Routes.RESUME, false)) }
    var optimized by remember { mutableStateOf(false) }
    var music by remember { mutableStateOf(true) }
    val update = remember { UpdateFlow() }
    var icon by remember { mutableStateOf(LauncherIcon.current(context)) }
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
                    SettingRow(stringResource(R.string.skins), Skins.BUILT_IN.find { Skins.isOn(it, s) }?.let { stringResource(it.label) } ?: Skins.user.find { Skins.isOn(it, s) }?.name.orEmpty(), "set_skins") {
                        Nav.open(Screen.Skins)
                    }
                    if ("theme" in ops) SettingRow(stringResource(R.string.theme), themeLabel(s?.theme?.name.orEmpty()), "set_theme") { Nav.open(Screen.Themes) }
                    if ("vis" in ops) SettingRow(stringResource(R.string.visualizer), s?.visualizer.orEmpty(), "set_vis") { Nav.open(Screen.Visualizers) }
                    SettingRow(stringResource(R.string.font), Fonts.name(context, Look.font), "set_font") { Nav.open(Screen.Fonts) }
                    Choice(stringResource(R.string.text_size), TEXT_SIZES, Look.textSize, "set_text_size") { Look.update(context) { textSize = it } }
                    Choice(stringResource(R.string.density), DENSITIES, Look.density, "set_density") { Look.update(context) { density = it } }
                    Choice(stringResource(R.string.corners), CORNERS, if (Look.rounded) 1 else 0, "set_corners") { Look.update(context) { rounded = it == 1 } }
                    Toggle(stringResource(R.string.haptics), Look.haptics, "set_haptics") { Look.update(context) { haptics = it == 1 } }
                    SettingRow(stringResource(R.string.background), stringResource(Looks.background.label), "set_background") { Nav.open(Screen.Backgrounds) }
                    if (Looks.background != Background.Solid) {
                        LevelSlider(stringResource(R.string.bg_level_lists), Looks.listLevel, "set_bg_lists", Looks::putListLevel)
                        LevelSlider(stringResource(R.string.bg_level_player), Looks.playerLevel, "set_bg_player", Looks::putPlayerLevel)
                    }
                    FlowRow(Modifier.padding(start = 16.dp, end = 12.dp), itemVerticalAlignment = Alignment.CenterVertically) {
                        Text(stringResource(R.string.motion), Modifier.padding(end = 8.dp), color = p.textPrimary, style = Type.body)
                        for (m in MotionSpeed.entries) {
                            BracketChip(stringResource(m.label), Looks.motion == m, Modifier.testTag("set_motion_" + m.name.lowercase())) { Looks.putMotion(m) }
                        }
                    }
                    Toggle(stringResource(R.string.player_vis), Looks.playerVis, "set_player_vis") { Looks.putPlayerVis(it == 1) }
                    Choice(stringResource(R.string.launcher_icon), LauncherIcon.entries.map { it.label }, icon.ordinal, "set_icon") {
                        if (it != icon.ordinal) {
                            icon = LauncherIcon.entries[it]
                            LauncherIcon.set(context, icon)
                        }
                    }
                    Help(stringResource(R.string.launcher_icon_help))
                    SettingRow(stringResource(R.string.stereo_charging), "", "set_stereo") { openDreamSettings(context) }
                    Help(stringResource(R.string.stereo_help))
                }
            }
            item(key = "playback") {
                Column {
                    SectionRule(stringResource(R.string.set_playback))
                    if ("eq" in ops) SettingRow(stringResource(R.string.eq), eqLabel(s), "set_eq") { Nav.open(Screen.Eq) }
                    Toggle(stringResource(R.string.resume_on_connect), resumeOnConnect, "set_resume_connect") {
                        resumeOnConnect = it == 1
                        Routes.prefs(context).edit { putBoolean(Routes.RESUME, resumeOnConnect) }
                    }
                    Help(stringResource(R.string.resume_on_connect_help))
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
            if (settable) item(key = "network") { NetworkSection(s) }
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
            if (settable) item(key = "scrobble") { ScrobbleSection(s) }
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
                    TextChoice(
                        stringResource(R.string.language),
                        listOf(stringResource(R.string.language_system), stringResource(R.string.language_en), stringResource(R.string.language_es)),
                        Language.TAGS.indexOf(Language.current(context)).coerceAtLeast(0), "set_language",
                    ) { Language.set(context, Language.TAGS[it]) }
                    Help(stringResource(R.string.language_help))
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
                    SettingRow(stringResource(R.string.about), "", "set_about") { Nav.open(Screen.About) }
                    var sending by remember { mutableStateOf(false) }
                    if (sending) {
                        Row(Modifier.heightIn(min = 48.dp).padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) { SpinnerLine(stringResource(R.string.send_log_writing), Type.body) }
                    } else {
                        BracketButton(stringResource(R.string.send_log), Modifier.padding(horizontal = 12.dp).testTag("set_send_log")) {
                            sending = true
                            scope.launch {
                                try {
                                    Logs.share(context, Logs.write(context))
                                } catch (e: CancellationException) {
                                    throw e
                                } catch (e: Exception) {
                                    Toast.makeText(context, "ERR: " + (e.message ?: e.javaClass.simpleName), Toast.LENGTH_LONG).show()
                                } finally {
                                    sending = false
                                }
                            }
                        }
                    }
                    Help(stringResource(R.string.send_log_help))
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

/** The system's screen saver settings, where cliamp-x is picked for Modo estéreo; the display settings where a system hides them. */
private fun openDreamSettings(context: Context) {
    try {
        context.startActivity(Intent(android.provider.Settings.ACTION_DREAM_SETTINGS))
    } catch (e: android.content.ActivityNotFoundException) {
        context.startActivity(Intent(android.provider.Settings.ACTION_DISPLAY_SETTINGS))
    }
}

private val QUALITIES = listOf("alta" to R.string.quality_high, "normal" to R.string.quality_normal, "ahorro" to R.string.quality_saver)

/**
 * Red y descargas: the YouTube quality on Wi-Fi and on mobile data, Calidad
 * Premium (core settings), then the app's download rules and the automatic
 * cache of liked and recent songs (Offline).
 */
@Composable
private fun NetworkSection(s: Snapshot?) {
    val context = LocalContext.current
    val ops by Remote.ops.collectAsStateWithLifecycle()
    val network by Remote.network.collectAsStateWithLifecycle()
    val prefs = remember { Offline.prefs(context) }
    var wifiOnly by remember { mutableStateOf(prefs.getBoolean(Offline.WIFI_ONLY, false)) }
    var cache by remember { mutableStateOf(prefs.getBoolean(Offline.CACHE, false)) }
    var cacheWifi by remember { mutableStateOf(prefs.getBoolean(Offline.CACHE_WIFI_ONLY, true)) }
    var songs by remember { mutableStateOf(prefs.getInt(Offline.CACHE_SONGS, 50)) }
    var mb by remember { mutableStateOf(prefs.getInt(Offline.CACHE_MB, 500)) }
    fun text(name: String, value: String) = Remote.fire("settings.set", JSONObject().put("name", name).put("text", value))
    Column {
        SectionRule(stringResource(R.string.set_network))
        network?.let { Help(stringResource(if (it == "wifi") R.string.network_now_wifi else R.string.network_now_mobile)) }
        val labels = QUALITIES.map { stringResource(it.second) }
        TextChoice(stringResource(R.string.quality_wifi), labels, QUALITIES.indexOfFirst { it.first == s?.qualityWifi.orEmpty().ifEmpty { "alta" } }, "set_quality_wifi") {
            text("quality_wifi", QUALITIES[it].first)
        }
        TextChoice(stringResource(R.string.quality_mobile), labels, QUALITIES.indexOfFirst { it.first == s?.qualityMobile.orEmpty().ifEmpty { "normal" } }, "set_quality_mobile") {
            text("quality_mobile", QUALITIES[it].first)
        }
        Help(stringResource(R.string.quality_help))
        Toggle(stringResource(R.string.premium_quality), s?.premiumQuality == true, "set_premium") { text("premium_quality", if (it == 1) "on" else "off") }
        Help(stringResource(R.string.premium_quality_help))
        if ("download" in ops) {
            Toggle(stringResource(R.string.downloads_wifi_only), wifiOnly, "set_dl_wifi") {
                wifiOnly = it == 1
                prefs.edit { putBoolean(Offline.WIFI_ONLY, wifiOnly) }
            }
        }
        if ("download.auto" in ops) {
            Toggle(stringResource(R.string.auto_cache), cache, "set_cache") {
                cache = it == 1
                prefs.edit { putBoolean(Offline.CACHE, cache) }
                if (cache) Remote.scope.launch { Remote.fillCache(force = false) }
            }
            Help(stringResource(R.string.auto_cache_help))
            if (cache) {
                Toggle(stringResource(R.string.auto_cache_wifi_only), cacheWifi, "set_cache_wifi") {
                    cacheWifi = it == 1
                    prefs.edit { putBoolean(Offline.CACHE_WIFI_ONLY, cacheWifi) }
                }
                TextChoice(stringResource(R.string.auto_cache_songs), Offline.SONGS.map { "$it" }, Offline.SONGS.indexOf(songs), "set_cache_songs") {
                    songs = Offline.SONGS[it]
                    prefs.edit { putInt(Offline.CACHE_SONGS, songs) }
                }
                TextChoice(
                    stringResource(R.string.auto_cache_size), Offline.MB.map { if (it == 0) stringResource(R.string.no_limit) else "$it MB" },
                    Offline.MB.indexOf(mb), "set_cache_mb",
                ) {
                    mb = Offline.MB[it]
                    prefs.edit { putInt(Offline.CACHE_MB, mb) }
                }
                BracketButton(stringResource(R.string.auto_cache_now), Modifier.padding(horizontal = 12.dp).testTag("set_cache_now")) {
                    Remote.scope.launch { Remote.fillCache(force = true) }
                }
            }
        }
    }
}

/**
 * Scrobbling: ListenBrainz with the user's token; Last.fm with the user's
 * API key and secret, then the browser sign-in (lastfm.auth start, the user
 * allows cliamp-x, then finish). Desconectar clears the credential.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun ScrobbleSection(s: Snapshot?) {
    val p = LocalPalette.current
    val context = LocalContext.current
    val ops by Remote.ops.collectAsStateWithLifecycle()
    val scope = rememberCoroutineScope()
    var token by rememberSaveable { mutableStateOf("") }
    val needKeys = stringResource(R.string.lastfm_need_keys)
    val on = stringResource(R.string.connected)
    val off = stringResource(R.string.disconnected)
    fun state(connected: Boolean) = if (connected) on else off
    Column {
        SectionRule(stringResource(R.string.set_scrobble))
        Line("ListenBrainz · " + state(s?.scrobbleListenbrainz == true), if (s?.scrobbleListenbrainz == true) p.playing else p.textSecondary)
        FlowRow(Modifier.padding(horizontal = 12.dp)) {
            BracketButton(stringResource(R.string.lb_token), Modifier.testTag("set_lb_token")) {
                Nav.sheet = Sheet.Credential("listenbrainz_token", R.string.lb_token)
            }
            if (s?.scrobbleListenbrainz == true) BracketButton(stringResource(R.string.disconnect), Modifier.testTag("set_lb_off"), color = p.error) { Remote.setText("listenbrainz_token", "") }
        }
        Help(stringResource(R.string.lb_help))
        Line("Last.fm · " + state(s?.scrobbleLastfm == true), if (s?.scrobbleLastfm == true) p.playing else p.textSecondary)
        FlowRow(Modifier.padding(horizontal = 12.dp)) {
            BracketButton(stringResource(R.string.lastfm_key), Modifier.testTag("set_lastfm_key")) {
                Nav.sheet = Sheet.Credential("lastfm_api_key", R.string.lastfm_key)
            }
            BracketButton(stringResource(R.string.lastfm_secret), Modifier.testTag("set_lastfm_secret")) {
                Nav.sheet = Sheet.Credential("lastfm_api_secret", R.string.lastfm_secret)
            }
            if ("lastfm.auth" in ops) {
                BracketButton(stringResource(R.string.lastfm_connect), Modifier.testTag("set_lastfm_connect")) {
                    scope.launch {
                        try {
                            val r = Remote.call("lastfm.auth", JSONObject().put("name", "start"))
                            token = r.optString("token")
                            context.startActivity(Intent(Intent.ACTION_VIEW, r.optString("url").toUri()))
                        } catch (e: CancellationException) {
                            throw e
                        } catch (e: Exception) {
                            if (e is IpcError && e.code == "invalid_params") Toast.makeText(context, needKeys, Toast.LENGTH_LONG).show() else Remote.showError(e)
                        }
                    }
                }
            }
            if (token.isNotEmpty()) {
                BracketButton(stringResource(R.string.lastfm_finish), Modifier.testTag("set_lastfm_finish")) {
                    scope.launch {
                        try {
                            Remote.call("lastfm.auth", JSONObject().put("name", "finish").put("text", token))
                            token = ""
                        } catch (e: CancellationException) {
                            throw e
                        } catch (e: Exception) {
                            Remote.showError(e)
                        }
                    }
                }
            }
            if (s?.scrobbleLastfm == true) BracketButton(stringResource(R.string.disconnect), Modifier.testTag("set_lastfm_off"), color = p.error) { Remote.setText("lastfm_session_key", "") }
        }
        Help(stringResource(if (token.isNotEmpty()) R.string.lastfm_finish_help else R.string.lastfm_help))
    }
}

/** A 56 dp row: the label, `[value]` in accent, and `›` when it opens something. */
@Composable
private fun SettingRow(label: String, value: String, tag: String, onClick: () -> Unit) {
    val p = LocalPalette.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = Look.row(56.dp)).clickable(onClick = onClick).padding(horizontal = 16.dp).testTag(tag),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text(label, Modifier.weight(1f), color = p.textPrimary, style = Type.body)
        if (value.isNotEmpty()) Bracketed(value, false)
        Text("›", color = p.textSecondary, style = Type.body)
    }
}

/** A 0..100 % setting in steps of 5: the label and value above a square slider. */
@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
private fun LevelSlider(label: String, value: Int, tag: String, set: (Int) -> Unit) {
    val p = LocalPalette.current
    Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp).testTag(tag)) {
        Row(Modifier.fillMaxWidth().padding(top = 8.dp)) {
            Text(label, Modifier.weight(1f), color = p.textPrimary, style = Type.body)
            Text("$value %", color = p.accent, style = Type.numeric)
        }
        Slider(
            value.toFloat(), { set(it.roundToInt()) }, Modifier.fillMaxWidth().height(48.dp).semantics { contentDescription = label },
            valueRange = 0f..100f, steps = 19,
            thumb = { Box(Modifier.size(4.dp, 24.dp).background(p.accent)) },
            track = { state ->
                Box(
                    Modifier.fillMaxWidth().height(24.dp).drawBehind {
                        val x = size.width * (state.value - state.valueRange.start) / (state.valueRange.endInclusive - state.valueRange.start)
                        val y = size.height / 2
                        drawLine(p.textSecondary, Offset(x, y), Offset(size.width, y), 1.dp.toPx())
                        drawLine(p.accent, Offset(0f, y), Offset(x, y), 3.dp.toPx())
                    },
                )
            },
        )
    }
}

/** A switch row: `[Encendido]` in accent bold, `[Apagado]` dim brackets; a tap flips it (settings.set 0 or 1). */
@Composable
private fun Toggle(label: String, on: Boolean, tag: String, set: (Int) -> Unit) {
    val p = LocalPalette.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = Look.row(56.dp)).clickable { set(if (on) 0 else 1) }.padding(horizontal = 16.dp).testTag(tag),
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

private val TEXT_SIZES = listOf(R.string.text_small, R.string.text_normal, R.string.text_large, R.string.text_huge)
private val DENSITIES = listOf(R.string.density_compact, R.string.density_normal, R.string.density_wide)
private val CORNERS = listOf(R.string.corners_square, R.string.corners_rounded)

/** A setting with a few choices as bracket chips under its label. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Choice(label: String, options: List<Int>, selected: Int, tag: String, pick: (Int) -> Unit) =
    TextChoice(label, options.map { stringResource(it) }, selected, tag, pick)

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun TextChoice(label: String, options: List<String>, selected: Int, tag: String, pick: (Int) -> Unit) {
    Column(Modifier.fillMaxWidth().padding(top = 8.dp)) {
        Text(label, Modifier.padding(horizontal = 16.dp), color = LocalPalette.current.textPrimary, style = Type.body)
        FlowRow(Modifier.padding(horizontal = 12.dp)) {
            options.forEachIndexed { i, option -> BracketChip(option, i == selected, Modifier.testTag("${tag}_$i")) { pick(i) } }
        }
    }
}

/** A dim explanation under a setting. */
@Composable
internal fun Help(text: String) {
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
    val context = LocalContext.current
    val s by Remote.state.collectAsStateWithLifecycle()
    val (load, retry) = rememberLoad("themes") { Remote.themes() }
    val current = s?.theme?.name.orEmpty().ifEmpty { DEFAULT_THEME }
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.theme))
        LazyColumn(Modifier.fillMaxSize()) {
            item(key = "dynamic") {
                Column {
                    SettingRow(stringResource(R.string.theme_editor), "", "theme_edit") { Nav.open(Screen.ThemeEditor) }
                    SectionRule(stringResource(R.string.theme_dynamic))
                    SettingRow(stringResource(R.string.theme_from_cover), "", "theme_dyn_cover") { Remote.fire("theme", JSONObject().put("name", "cover")) }
                    SettingRow(stringResource(R.string.theme_from_wallpaper), "", "theme_dyn_wallpaper") { useWallpaperTheme(context) }
                    SectionRule(stringResource(R.string.themes_all))
                }
            }
            if (load !is Load.Done) item(key = "state") { LoadState(load, retry) }
            // Prefixed so a user theme named "dynamic" or "state" cannot reuse a fixed item key (duplicate keys crash).
            itemsIndexed((load as? Load.Done)?.value.orEmpty(), key = { _, t -> "t:" + t.name }) { _, t ->
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
                            "wallpaper" -> stringResource(R.string.theme_wallpaper)
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
 * Visualizador (§3.14): a live 120 dp preview while music plays and every
 * mode of `vis list`, the core's order, None last.
 */
@Composable
fun VisualizersScreen() {
    val p = LocalPalette.current
    val s by Remote.state.collectAsStateWithLifecycle()
    val (all, _) = rememberLoad("vis:list") { Remote.visualizers() }
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.visualizer))
        LazyColumn(Modifier.fillMaxSize()) {
            s?.takeIf { it.playing }?.let { snap ->
                item(key = "preview") { Visualizer(snap, Modifier.fillMaxWidth().height(120.dp).padding(horizontal = 16.dp, vertical = 8.dp).testTag("vis_preview")) }
            }
            val names = (all as? Load.Done)?.value.orEmpty().sortedBy { it == "None" }
            itemsIndexed(names, key = { i, v -> "$i:$v" }) { _, name ->
                val on = s?.visualizer == name
                Row(
                    Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable { Remote.fire("vis", JSONObject().put("name", name)) }
                        .padding(horizontal = 16.dp).testTag("vis_$name"),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(if (on) "● " else "  ", color = p.accent, style = Type.body)
                    Text(name, Modifier.weight(1f), color = if (on) p.accent else p.textPrimary, style = Type.body.bold(on))
                    Text(VIS_LABELS[name]?.let { stringResource(it) }.orEmpty(), Modifier.weight(1f), color = p.textSecondary, style = Type.label)
                }
            }
        }
    }
}

/** Fondo: every background; a tap applies it at once behind this list too. */
@Composable
fun BackgroundsScreen() {
    val p = LocalPalette.current
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.background))
        LazyColumn(Modifier.fillMaxSize()) {
            item(key = "help") { Help(stringResource(R.string.background_help)) }
            itemsIndexed(Background.entries, key = { _, b -> b.name }) { _, b ->
                val on = b == Looks.background
                Row(
                    Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable { Looks.putBackground(b) }
                        .padding(horizontal = 16.dp).testTag("bg_" + b.name.lowercase()).semantics { selected = on },
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(if (on) "● " else "  ", color = p.accent, style = Type.body)
                    Text(stringResource(b.label), color = if (on) p.accent else p.textPrimary, style = Type.body.bold(on))
                }
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
