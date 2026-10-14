package io.github.xaviiromf.cliampx.ui

import android.app.WallpaperManager
import android.content.Context
import android.widget.Toast
import androidx.annotation.StringRes
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Slider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.listSaver
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.toMutableStateList
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.Core
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.io.File

/** The seven keys of a theme file (theme/theme.go), in CliampPalette order. */
val THEME_KEYS = listOf("bg", "accent", "bright_fg", "fg", "green", "yellow", "red")

/** The editor's labels of THEME_KEYS. */
private val KEY_LABELS = listOf(
    R.string.color_bg, R.string.color_accent, R.string.color_bright_fg, R.string.color_fg,
    R.string.color_green, R.string.color_yellow, R.string.color_red,
)

internal val THEME_NAME = Regex("[A-Za-z0-9][A-Za-z0-9 ._-]{0,39}")
private val HEX6 = Regex("#[0-9a-fA-F]{6}")

/** Names the core keeps: the terminal colors, the cover theme, and `theme list`. */
private val RESERVED = setOf("default", "default - terminal colors", "cover", "list")

/** Why name cannot be a theme file's name, or null when it can. */
@StringRes
fun themeNameError(name: String): Int? = when {
    !THEME_NAME.matches(name) -> R.string.theme_bad_name
    name.lowercase() in RESERVED -> R.string.theme_reserved
    else -> null
}

fun hexOf(c: Color) = "#%06x".format(c.toArgb() and 0xFFFFFF)

/** A theme of the user: its file name and its seven colors as #rrggbb, in THEME_KEYS order. */
data class UserTheme(val name: String, val colors: List<String>) {
    /** The file as the core and the desktop read it. */
    fun toml() = "# Made with the cliamp-x app.\n" + THEME_KEYS.zip(colors).joinToString("") { (k, v) -> "$k = \"$v\"\n" }

    companion object {
        /** A theme with name and colors checked (they may come from an imported file), or null. */
        fun of(name: String, colors: List<String>): UserTheme? =
            if (themeNameError(name) == null && colors.size == THEME_KEYS.size && colors.all(HEX6::matches)) UserTheme(name, colors.map { it.lowercase() }) else null

        /** A theme file's seven colors as the core parses them; null when one is missing or not #rrggbb. */
        fun parse(name: String, toml: String): UserTheme? {
            val values = toml.lineSequence().map { it.trim() }.filter { it.isNotEmpty() && !it.startsWith("#") }.mapNotNull { line ->
                val (k, v) = line.split("=", limit = 2).takeIf { it.size == 2 } ?: return@mapNotNull null
                // A # after white space starts a comment; an unquoted #rrggbb stays whole.
                k.trim() to v.trim().replace(Regex("""\s+#.*$"""), "").trim('"', '\'', ' ')
            }.toMap()
            return of(name, THEME_KEYS.map { values[it] ?: return null })
        }
    }
}

/** The user themes in the core's themes directory, the one the desktop uses under ~/.config/cliamp-x. */
object UserThemes {
    fun dir(ctx: Context) = File(Core.configDir(ctx), "themes")

    fun exists(ctx: Context, name: String) = File(dir(ctx), "$name.toml").isFile

    /** Writes t whole or not at all: the core reads the directory at each theme change. */
    fun write(dir: File, t: UserTheme) {
        dir.mkdirs()
        val tmp = File(dir, ".${t.name}.tmp")
        tmp.writeText(t.toml())
        if (!tmp.renameTo(File(dir, "${t.name}.toml"))) {
            tmp.delete()
            throw java.io.IOException("cannot write ${t.name}.toml")
        }
    }

    /** The complete themes of dir; a file the core would skip, or one without bg, is left out. */
    fun readAll(dir: File): List<UserTheme> =
        dir.listFiles { f -> f.isFile && f.name.endsWith(".toml") }.orEmpty().sortedBy { it.name }
            .mapNotNull { runCatching { UserTheme.parse(it.name.removeSuffix(".toml"), it.readText()) }.getOrNull() }

    /** Saves t and makes it the theme on screen; Tema lists it on its next visit. */
    suspend fun saveAndUse(ctx: Context, t: UserTheme) {
        withContext(Dispatchers.IO) { write(dir(ctx), t) }
        Remote.call("theme", JSONObject().put("name", t.name))
        Remote.forget("themes")
    }
}

/** A color as hue (0..360), saturation and value (0..1). */
data class Hsv(val h: Float, val s: Float, val v: Float) {
    fun color() = Color.hsv(h.coerceIn(0f, 360f), s.coerceIn(0f, 1f), v.coerceIn(0f, 1f))
}

fun hsvOf(c: Color): Hsv {
    val max = maxOf(c.red, c.green, c.blue)
    val d = max - minOf(c.red, c.green, c.blue)
    val h = when {
        d == 0f -> 0f
        max == c.red -> 60 * (((c.green - c.blue) / d) % 6)
        max == c.green -> 60 * ((c.blue - c.red) / d + 2)
        else -> 60 * ((c.red - c.green) / d + 4)
    }
    return Hsv((h + 360) % 360, if (max == 0f) 0f else d / max, max)
}

/**
 * A dark theme from a wallpaper's colors (WallpaperColors' primary first):
 * bg and the texts take the primary's hue, the accent the most colorful
 * seed; every color but bg keeps 4.5:1 on bg, giving up saturation first.
 */
fun seedTheme(seeds: List<Color>): List<Color> {
    val hsv = seeds.ifEmpty { listOf(Color.Gray) }.map(::hsvOf)
    val base = hsv.first()
    val bg = Hsv(base.h, minOf(base.s, 0.6f), 0.09f).color()
    fun readable(h: Float, s: Float, v: Float): Color {
        var c = Hsv(h, s, v)
        while (contrast(c.color(), bg) < 4.5f && (c.s > 0f || c.v < 1f)) c = if (c.s > 0f) c.copy(s = maxOf(0f, c.s - 0.05f)) else c.copy(v = minOf(1f, c.v + 0.05f))
        return c.color()
    }
    val main = hsv.maxBy { it.s * it.v }
    return listOf(
        bg, readable(main.h, main.s.coerceIn(0.5f, 0.9f), 1f), readable(base.h, 0.06f, 0.96f), readable(base.h, 0.15f, 0.72f),
        readable(130f, 0.55f, 0.85f), readable(48f, 0.6f, 0.9f), readable(4f, 0.6f, 0.95f),
    )
}

/** Makes a theme of the system wallpaper's colors, saves it as `wallpaper` and uses it. */
fun useWallpaperTheme(ctx: Context) {
    val app = ctx.applicationContext
    Remote.scope.launch(Dispatchers.Main) {
        val error = try {
            val seeds = withContext(Dispatchers.IO) {
                WallpaperManager.getInstance(app).getWallpaperColors(WallpaperManager.FLAG_SYSTEM)
                    ?.let { listOfNotNull(it.primaryColor, it.secondaryColor, it.tertiaryColor) }
            }
            if (seeds.isNullOrEmpty()) {
                app.getString(R.string.wallpaper_failed)
            } else {
                UserThemes.saveAndUse(app, UserTheme("wallpaper", seedTheme(seeds.map { Color(it.toArgb()) }).map(::hexOf)))
                null
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            Remote.message(e)
        }
        error?.let { Toast.makeText(app, it, Toast.LENGTH_LONG).show() }
    }
}

/**
 * Editor de temas: the seven colors of the theme on screen, a live preview
 * in them, and an HSV picker with a hex field for the chosen one. Guardar
 * como… writes a theme file to the core's themes directory and uses it.
 */
@Composable
fun ThemeEditorScreen() {
    val p = LocalPalette.current
    val context = LocalContext.current
    val s by Remote.state.collectAsStateWithLifecycle()
    val colors = rememberSaveable(saver = listSaver({ it.map(Color::toArgb) }, { it.map(::Color).toMutableStateList() })) {
        paletteOf(s?.theme).let { listOf(it.bg, it.accent, it.brightFg, it.fg, it.green, it.yellow, it.red) }.toMutableStateList()
    }
    var key by rememberSaveable { mutableIntStateOf(1) }
    // The picker's own hue and saturation, kept while the color is gray or black.
    var hsv by remember(key) { mutableStateOf(hsvOf(colors[key])) }
    fun pick(next: Hsv) {
        hsv = next
        colors[key] = next.color()
    }
    val edited = CliampPalette(colors[0], colors[1], colors[2], colors[3], colors[4], colors[5], colors[6])
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.theme_editor))
        LazyColumn(Modifier.fillMaxSize().testTag("editor_list")) {
            item(key = "preview") { CompositionLocalProvider(LocalPalette provides edited) { ThemePreview() } }
            item(key = "keys") {
                Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                    colors.forEachIndexed { i, c ->
                        val label = stringResource(KEY_LABELS[i])
                        Box(
                            Modifier.weight(1f).height(48.dp).clickable { key = i }.border(if (i == key) 3.dp else 1.dp, if (i == key) p.accent else p.divider)
                                .padding(4.dp).background(c).testTag("editor_key_" + THEME_KEYS[i])
                                .semantics {
                                    contentDescription = "$label ${hexOf(c)}"
                                    selected = i == key
                                },
                        )
                    }
                }
            }
            item(key = "picker") {
                val label = stringResource(KEY_LABELS[key])
                Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text("$label · ", color = p.textPrimary, style = Type.bodyStrong)
                        HexField(colors[key]) { pick(hsvOf(it)) }
                    }
                    if (key != 0) {
                        val ratio = contrast(colors[key], colors[0])
                        Text(
                            stringResource(R.string.editor_contrast, String.format(LocalConfiguration.current.locales[0], "%.1f", ratio)),
                            color = if (ratio < 4.5f) p.error else p.textSecondary, style = Type.label,
                        )
                    }
                    PickerSlider(stringResource(R.string.hue), hsv.h, 0f..360f, Brush.horizontalGradient((0..6).map { Hsv(it * 60f, 1f, 1f).color() }), "editor_hue") { pick(hsv.copy(h = it)) }
                    PickerSlider(stringResource(R.string.saturation), hsv.s, 0f..1f, Brush.horizontalGradient(listOf(hsv.copy(s = 0f).color(), hsv.copy(s = 1f).color())), "editor_sat") { pick(hsv.copy(s = it)) }
                    PickerSlider(stringResource(R.string.brightness), hsv.v, 0f..1f, Brush.horizontalGradient(listOf(Color.Black, hsv.copy(v = 1f).color())), "editor_val") { pick(hsv.copy(v = it)) }
                }
            }
            item(key = "save") {
                Row(Modifier.padding(horizontal = 16.dp, vertical = 8.dp), horizontalArrangement = Arrangement.spacedBy(16.dp), verticalAlignment = Alignment.CenterVertically) {
                    KeyPill(stringResource(R.string.theme_save_as), Modifier.testTag("editor_save")) {
                        // Editing a theme of the user offers its own name, to save over it.
                        val name = s?.theme?.name.orEmpty()
                        Nav.sheet = Sheet.ThemeName(colors.map(::hexOf), if (UserThemes.exists(context, name)) name else "")
                    }
                }
            }
        }
    }
}

/** A few rows of the app in the palette being edited: what each color paints. */
@Composable
private fun ThemePreview() {
    val p = LocalPalette.current
    Column(
        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp).border(1.dp, p.divider).background(p.bg)
            .padding(12.dp).testTag("editor_preview"),
        verticalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        Text("cliamp-x", color = p.accent, style = Type.logo)
        Text("▶ " + stringResource(R.string.preview_song), color = p.playing, style = Type.bodyStrong, maxLines = 1)
        Text("  " + stringResource(R.string.preview_artist), color = p.textSecondary, style = Type.label, maxLines = 1)
        Text("  " + stringResource(R.string.preview_next), color = p.textPrimary, style = Type.body, maxLines = 1)
        Text(
            buildAnnotatedString {
                withStyle(SpanStyle(color = p.accent)) { append("━━━━━━━●") }
                withStyle(SpanStyle(color = p.textSecondary)) { append("────── 01:23") }
            },
            style = Type.numeric, maxLines = 1,
        )
        Text(
            buildAnnotatedString {
                withStyle(SpanStyle(color = p.accent)) { append("[" + stringResource(R.string.on) + "] ") }
                withStyle(SpanStyle(color = p.favorite)) { append("♥ ") }
                withStyle(SpanStyle(color = p.warning)) { append("▲ ") }
                withStyle(SpanStyle(color = p.error)) { append("ERR") }
            },
            style = Type.body, maxLines = 1,
        )
    }
}

/** `#` and six hex digits; a complete value sets the color. */
@Composable
private fun HexField(color: Color, set: (Color) -> Unit) {
    val p = LocalPalette.current
    var text by remember(color) { mutableStateOf(hexOf(color).drop(1)) }
    val label = stringResource(R.string.hex)
    BasicTextField(
        text,
        { typed ->
            text = typed.filter { it in "0123456789abcdefABCDEF" }.take(6)
            if (text.length == 6) hexColor(text)?.let(set)
        },
        Modifier.heightIn(min = 48.dp).testTag("editor_hex").semantics { contentDescription = label },
        singleLine = true,
        textStyle = Type.body.copy(color = p.accent),
        cursorBrush = SolidColor(p.accent),
        keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.None, keyboardType = KeyboardType.Ascii),
        decorationBox = { field ->
            Row(Modifier.height(48.dp).background(p.surfaceHigh).padding(horizontal = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                Text("#", color = p.textSecondary, style = Type.body)
                Box(Modifier.size(width = 72.dp, height = 24.dp), contentAlignment = Alignment.CenterStart) { field() }
            }
        },
    )
}

/** A square slider over a gradient track, as LevelSlider draws Ajustes' 0..100 settings. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun PickerSlider(label: String, value: Float, range: ClosedFloatingPointRange<Float>, track: Brush, tag: String, set: (Float) -> Unit) {
    val p = LocalPalette.current
    Column(Modifier.fillMaxWidth()) {
        Text(label, Modifier.padding(top = 4.dp), color = p.textPrimary, style = Type.label)
        Slider(
            value, set, Modifier.fillMaxWidth().height(40.dp).testTag(tag).semantics { contentDescription = label },
            valueRange = range,
            thumb = { Box(Modifier.size(6.dp, 28.dp).background(p.brightFg).border(1.dp, p.bg)) },
            track = {
                Box(
                    Modifier.fillMaxWidth().height(16.dp).drawBehind {
                        drawRect(track)
                        drawLine(p.divider, Offset(0f, 0f), Offset(size.width, 0f), 1.dp.toPx())
                    },
                )
            },
        )
    }
}

/** Guardar como…: a theme name, then the file is written and used, and the editor closes. */
@Composable
fun ThemeNameSheet(sheet: Sheet.ThemeName, close: () -> Unit) {
    val context = LocalContext.current
    val res = LocalResources.current
    val scope = rememberCoroutineScope()
    NameSheet(stringResource(R.string.theme_name), sheet.initial, stringResource(R.string.store), close, taken = null, allowSame = true) { name, failed ->
        val bad = themeNameError(name)
        val theme = UserTheme.of(name, sheet.colors)
        if (bad != null || theme == null) {
            failed(res.getString(bad ?: R.string.theme_bad_name))
        } else {
            scope.launch {
                try {
                    UserThemes.saveAndUse(context, theme)
                    Nav.sheet = null
                    if (Nav.top.screen == Screen.ThemeEditor) Nav.pop()
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    failed(Remote.message(e))
                }
            }
        }
    }
}
