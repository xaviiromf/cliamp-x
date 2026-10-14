package io.github.xaviiromf.cliampx.ui

import android.content.Context
import android.content.SharedPreferences
import android.net.Uri
import android.widget.Toast
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.annotation.StringRes
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.content.edit
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Snapshot
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.io.IOException

/**
 * A skin (FINAL-PLAN T5): a theme, a font, a background with its two
 * intensities, and a visualizer mode, applied together. A built-in skin
 * shows its label; a user skin its name.
 */
data class Skin(
    val name: String, val theme: String, val font: String, val background: Background,
    val listLevel: Int, val playerLevel: Int, val vis: String, @StringRes val label: Int = 0,
) {
    fun json(): JSONObject = JSONObject().put("name", name).put("theme", theme).put("font", font).put("background", background.name)
        .put("background_lists", listLevel).put("background_player", playerLevel).put("visualizer", vis)

    companion object {
        const val MAX_NAME = 40
        private val FONT = Regex("[a-z0-9_]{1,40}")
        private val VIS = Regex("[A-Za-z0-9 ._-]{0,40}")

        /** A skin of an imported or stored object, or null when a field is missing or not valid. */
        fun parse(o: JSONObject): Skin? {
            val name = o.optString("name").trim()
            val theme = o.optString("theme")
            val font = o.optString("font")
            val vis = o.optString("visualizer")
            val bg = Background.entries.find { it.name == o.optString("background") }
            if (name.isEmpty() || name.length > MAX_NAME || name.any(Char::isISOControl)) return null
            if (!THEME_NAME.matches(theme) || !FONT.matches(font) || !VIS.matches(vis) || bg == null) return null
            return Skin(name, theme, font, bg, o.optInt("background_lists", 40).coerceIn(0, 100), o.optInt("background_player", 70).coerceIn(0, 100), vis)
        }

        fun parseAll(a: JSONArray?): List<Skin> =
            (0 until (a?.length() ?: 0)).mapNotNull { a!!.optJSONObject(it)?.let(::parse) }.distinctBy { it.name }.take(Skins.MAX)
    }
}

/** The built-in skins and the user's, kept in the "skins" preferences. */
object Skins {
    const val MAX = 200

    val BUILT_IN = listOf(
        Skin("crt_amber", "amber", "vt323", Background.Crt, 50, 80, "ClassicLED", R.string.skin_crt_amber),
        Skin("synthwave", "synthwave-84", Fonts.DEFAULT, Background.Retro, 45, 80, "Retro", R.string.skin_synthwave),
        Skin("matrix", "matrix", Fonts.DEFAULT, Background.Matrix, 45, 80, "Matrix", R.string.skin_matrix),
        Skin("gameboy", "gameboy", "departure_mono", Background.Dither, 35, 60, "Bricks", R.string.skin_gameboy),
        Skin("winamp", "winamp", Fonts.DEFAULT, Background.Solid, 40, 70, "ClassicPeak", R.string.skin_winamp),
        Skin("nord", "nord", "adwaita", Background.Aurora, 30, 50, "Wave", R.string.skin_nord),
        Skin("paper", "papercolor-light", "adwaita", Background.Grain, 30, 40, "Bars", R.string.skin_paper),
        Skin("vaporwave", "vaporwave", Fonts.DEFAULT, Background.Plasma, 45, 75, "Mirror", R.string.skin_vaporwave),
    )

    val user = mutableStateListOf<Skin>()
    private var prefs: SharedPreferences? = null

    fun load(ctx: Context) {
        val p = ctx.applicationContext.getSharedPreferences("skins", Context.MODE_PRIVATE).also { prefs = it }
        user.clear()
        user += Skin.parseAll(runCatching { JSONArray(p.getString(USER, "[]")) }.getOrNull())
    }

    private fun store() = prefs?.edit { putString(USER, JSONArray(user.map(Skin::json)).toString()) }

    /** The look on screen as a skin called name. */
    fun current(name: String, s: Snapshot?) = Skin(
        name, s?.theme?.name.orEmpty().ifEmpty { DEFAULT_THEME }, Look.font, Looks.background, Looks.listLevel, Looks.playerLevel, s?.visualizer.orEmpty(),
    )

    /** Whether skin is the look on screen; the intensities of Sólido do not count. */
    fun isOn(skin: Skin, s: Snapshot?): Boolean {
        val now = current("", s)
        return s != null && skin.theme.equals(now.theme, ignoreCase = true) && skin.font == now.font && skin.vis == now.vis &&
            skin.background == now.background && (skin.background == Background.Solid || (skin.listLevel == now.listLevel && skin.playerLevel == now.playerLevel))
    }

    /** Applies every part of skin: a font not downloaded yet downloads and applies when it is here. */
    fun apply(ctx: Context, skin: Skin) {
        val ops = Remote.ops.value
        if ("theme" in ops) Remote.fire("theme", JSONObject().put("name", skin.theme))
        if ("vis" in ops && skin.vis.isNotEmpty()) Remote.fire("vis", JSONObject().put("name", skin.vis))
        val spec = Fonts.specs(ctx).find { it.id == skin.font }
        if (spec != null && !Fonts.installed(Fonts.dir(ctx), spec)) Fonts.download(ctx, spec) else Look.update(ctx) { pickFont(ctx, skin.font) }
        Looks.putBackground(skin.background)
        Looks.putListLevel(skin.listLevel)
        Looks.putPlayerLevel(skin.playerLevel)
    }

    /** Why name cannot name a user skin (old is the one being renamed), or null. */
    @StringRes
    fun nameError(name: String, old: String?): Int? = when {
        name.length > Skin.MAX_NAME || name.any(Char::isISOControl) -> R.string.skin_bad_name
        user.any { it.name == name && it.name != old } -> R.string.skin_exists
        else -> null
    }

    /** Adds skin, or replaces the user skin of the same name. */
    fun put(skin: Skin) {
        val i = user.indexOfFirst { it.name == skin.name }
        if (i >= 0) user[i] = skin else if (user.size < MAX) user += skin
        store()
    }

    fun rename(old: String, name: String) {
        val i = user.indexOfFirst { it.name == old }
        if (i < 0) return
        user[i] = user[i].copy(name = name)
        store()
    }

    fun delete(name: String) {
        user.removeAll { it.name == name }
        store()
    }

    private const val USER = "user"
}

/**
 * All the personalization in one JSON file: the look on screen (as a skin),
 * the user skins and themes, text size, density, corners, haptics and
 * motion. A field missing on import keeps its value.
 */
class Personalization(
    val current: Skin?, val skins: List<Skin>, val themes: List<UserTheme>,
    val textSize: Int?, val density: Int?, val rounded: Boolean?, val haptics: Boolean?, val motion: MotionSpeed?,
) {
    fun json(): JSONObject = JSONObject().put("format", FORMAT).put("version", 1)
        .put("current", current?.json())
        .put("skins", JSONArray(skins.map(Skin::json)))
        .put("themes", JSONArray(themes.map { t -> JSONObject().put("name", t.name).also { o -> THEME_KEYS.zip(t.colors).forEach { (k, v) -> o.put(k, v) } } }))
        .put("text_size", textSize).put("density", density).put("rounded", rounded).put("haptics", haptics).put("motion", motion?.name)

    companion object {
        const val FORMAT = "cliamp-x-personalization"
        const val MAX_BYTES = 1 shl 20

        /** Reads an exported file: another format throws, an entry that is not valid is left out. */
        fun parse(text: String): Personalization {
            val o = JSONObject(text)
            if (o.optString("format") != FORMAT) throw IOException("not a cliamp-x personalization file")
            val themes = o.optJSONArray("themes")
            return Personalization(
                current = o.optJSONObject("current")?.let(Skin::parse),
                skins = Skin.parseAll(o.optJSONArray("skins")),
                themes = (0 until (themes?.length() ?: 0)).mapNotNull { i ->
                    themes!!.optJSONObject(i)?.let { t -> UserTheme.of(t.optString("name"), THEME_KEYS.map { t.optString(it) }) }
                }.distinctBy { it.name.lowercase() }.take(Skins.MAX),
                textSize = o.intOr("text_size")?.takeIf { it in Look.TEXT_SCALES.indices },
                density = o.intOr("density")?.takeIf { it in 0..2 },
                rounded = o.boolOr("rounded"),
                haptics = o.boolOr("haptics"),
                motion = MotionSpeed.entries.find { it.name == o.optString("motion") },
            )
        }

        /** The stream as UTF-8 text, refused past MAX_BYTES. */
        fun readLimited(input: java.io.InputStream): String {
            val out = java.io.ByteArrayOutputStream()
            val buf = ByteArray(8192)
            while (true) {
                val n = input.read(buf)
                if (n < 0) break
                out.write(buf, 0, n)
                if (out.size() > MAX_BYTES) throw IOException("larger than 1 MB")
            }
            return out.toString(Charsets.UTF_8.name())
        }

        private fun JSONObject.intOr(key: String) = (opt(key) as? Int)
        private fun JSONObject.boolOr(key: String) = (opt(key) as? Boolean)

        /** The personalization in use now. */
        fun now(ctx: Context, s: Snapshot?) = Personalization(
            Skins.current("current", s), Skins.user.toList(), UserThemes.readAll(UserThemes.dir(ctx)),
            Look.textSize, Look.density, Look.rounded, Look.haptics, Looks.motion,
        )

        /** Writes the personalization to uri; the result shows as a toast. */
        fun export(ctx: Context, uri: Uri, s: Snapshot?) {
            val app = ctx.applicationContext
            Remote.scope.launch(Dispatchers.Main) {
                val msg = try {
                    withContext(Dispatchers.IO) {
                        val text = now(app, s).json().toString(2)
                        (app.contentResolver.openOutputStream(uri, "wt") ?: throw IOException("cannot open the file")).use { it.write(text.toByteArray()) }
                    }
                    app.getString(R.string.export_done)
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    app.getString(R.string.export_failed, e.message.orEmpty())
                }
                Toast.makeText(app, msg, Toast.LENGTH_LONG).show()
            }
        }

        /** Reads uri and applies it: themes first, so the skin on screen finds its theme. */
        fun import(ctx: Context, uri: Uri) {
            val app = ctx.applicationContext
            Remote.scope.launch(Dispatchers.Main) {
                val msg = try {
                    val d = withContext(Dispatchers.IO) {
                        val text = (app.contentResolver.openInputStream(uri) ?: throw IOException("cannot open the file")).use { readLimited(it) }
                        parse(text).also { d -> d.themes.forEach { UserThemes.write(UserThemes.dir(app), it) } }
                    }
                    Remote.forget("themes")
                    d.skins.forEach(Skins::put)
                    Look.update(app) {
                        d.textSize?.let { textSize = it }
                        d.density?.let { density = it }
                        d.rounded?.let { rounded = it }
                        d.haptics?.let { haptics = it }
                    }
                    d.motion?.let(Looks::putMotion)
                    d.current?.let { Skins.apply(app, it) }
                    app.getString(R.string.import_done, d.skins.size, d.themes.size)
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    app.getString(R.string.skins_import_failed, e.message.orEmpty())
                }
                Toast.makeText(app, msg, Toast.LENGTH_LONG).show()
            }
        }
    }
}

/**
 * Skins (Ajustes → Apariencia): the built-in skins, the user's with ⋮ to
 * rename or delete, Guardar el actual como skin…, and the export and
 * import of the whole personalization.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun SkinsScreen() {
    val context = LocalContext.current
    val s by Remote.state.collectAsStateWithLifecycle()
    val (themes, _) = rememberLoad("themes") { Remote.themes() }
    val palettes = (themes as? Load.Done)?.value.orEmpty().associate { it.name.lowercase() to paletteOf(it) }
    val export = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("application/json")) { uri ->
        if (uri != null) Personalization.export(context, uri, s)
    }
    val import = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) Personalization.import(context, uri)
    }
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.skins))
        LazyColumn(Modifier.fillMaxSize().testTag("skins_list")) {
            item(key = "help") { Help(stringResource(R.string.skins_help)) }
            item(key = "builtin") { SectionRule(stringResource(R.string.skins_builtin)) }
            items(Skins.BUILT_IN, key = { "b:" + it.name }) { skin ->
                SkinRow(skin, stringResource(skin.label), palettes[skin.theme.lowercase()], Skins.isOn(skin, s), "skin_" + skin.name) { Skins.apply(context, skin) }
            }
            item(key = "user") { SectionRule(stringResource(R.string.skins_user)) }
            items(Skins.user, key = { "u:" + it.name }) { skin ->
                SkinRow(skin, skin.name, palettes[skin.theme.lowercase()], Skins.isOn(skin, s), "skin_user_" + skin.name, more = { Nav.sheet = Sheet.SkinActions(skin.name) }) {
                    Skins.apply(context, skin)
                }
            }
            item(key = "save") {
                BracketButton(stringResource(R.string.skin_save), Modifier.padding(horizontal = 12.dp).testTag("skin_save")) { Nav.sheet = Sheet.SkinName(null) }
            }
            item(key = "backup") {
                Column {
                    SectionRule(stringResource(R.string.backup))
                    Help(stringResource(R.string.backup_help))
                    FlowRow(Modifier.padding(horizontal = 12.dp)) {
                        BracketButton(stringResource(R.string.export), Modifier.testTag("skin_export")) { export.launch("cliamp-x-personalizacion.json") }
                        BracketButton(stringResource(R.string.import_file), Modifier.testTag("skin_import")) {
                            import.launch(arrayOf("application/json", "text/plain", "application/octet-stream"))
                        }
                    }
                }
            }
        }
    }
}

/** A skin: ● when it is the look on screen, its name, theme · font · background · visualizer, and its theme's colors. */
@Composable
private fun SkinRow(skin: Skin, name: String, palette: CliampPalette?, on: Boolean, tag: String, more: (() -> Unit)? = null, onClick: () -> Unit) {
    val p = LocalPalette.current
    val context = LocalContext.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = Look.row(56.dp)).clickable(onClick = onClick).padding(start = 16.dp, end = if (more == null) 16.dp else 0.dp)
            .testTag(tag).semantics { selected = on },
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(if (on) "● " else "  ", color = p.accent, style = Type.body)
        Column(Modifier.weight(1f).padding(vertical = 4.dp)) {
            Text(name, color = if (on) p.accent else p.textPrimary, style = Type.body.bold(on), maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(
                listOf(skin.theme, Fonts.name(context, skin.font), stringResource(skin.background.label), skin.vis).filter { it.isNotEmpty() }.joinToString(" · "),
                color = p.textSecondary, style = Type.caption, maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
        }
        if (palette != null) {
            Row(Modifier.padding(start = 8.dp), horizontalArrangement = Arrangement.spacedBy(2.dp)) {
                listOf(palette.bg, palette.accent, palette.brightFg, palette.green).forEach { Box(Modifier.size(12.dp).background(it).border(1.dp, p.divider)) }
            }
        }
        if (more != null) IconBtn(R.drawable.ic_more, stringResource(R.string.more), Modifier.testTag(tag + "_more"), onClick = more)
    }
}

/** Guardar el actual como skin… (old null) or Renombrar… a user skin. */
@Composable
fun SkinNameSheet(sheet: Sheet.SkinName, close: () -> Unit) {
    val res = LocalResources.current
    val s by Remote.state.collectAsStateWithLifecycle()
    val title = stringResource(if (sheet.old == null) R.string.skin_save else R.string.rename)
    NameSheet(title, sheet.old.orEmpty(), stringResource(R.string.store), close, taken = null) { name, failed ->
        val bad = Skins.nameError(name, sheet.old)
        when {
            bad != null -> failed(res.getString(bad))
            sheet.old == null -> Skins.put(Skins.current(name, s))
            else -> Skins.rename(sheet.old, name)
        }
        if (bad == null) Nav.sheet = null
    }
}

/** A user skin's ⋮: Renombrar… and Eliminar, after a confirmation. */
@Composable
fun SkinActionsSheet(sheet: Sheet.SkinActions, close: () -> Unit) {
    val p = LocalPalette.current
    var confirm by remember { mutableStateOf(false) }
    SectionRule(sheet.name)
    if (confirm) {
        Confirm(stringResource(R.string.skin_delete_confirm, sheet.name), stringResource(R.string.delete), close) { Skins.delete(sheet.name) }
    } else {
        MenuRow(">", stringResource(R.string.rename_more), "menu_rename") { Nav.sheet = Sheet.SkinName(sheet.name) }
        MenuRow("✗", stringResource(R.string.delete), "menu_delete", p.error) { confirm = true }
    }
}
