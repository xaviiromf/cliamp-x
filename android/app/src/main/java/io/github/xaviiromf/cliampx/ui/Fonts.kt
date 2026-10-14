package io.github.xaviiromf.cliampx.ui

import android.content.Context
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CornerBasedShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalFontFamilyResolver
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontVariation
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.core.content.edit
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Updates
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import java.io.File
import java.io.IOException
import java.io.InputStream
import java.io.OutputStream
import java.net.URL
import java.util.zip.ZipFile
import kotlin.coroutines.coroutineContext

/** One file of a downloadable font: its pinned URL and SHA-256, and for an archive the font's entry in it. */
class FontFile(val url: String, val sha256: String, val entry: String? = null)

/**
 * A downloadable font of assets/fonts.json. bold is null when the font has
 * one weight, or, variable, when bold comes from its wght axis.
 */
class FontSpec(val id: String, val name: String, val license: String, val regular: FontFile, val bold: FontFile?, val variable: Boolean) {
    val files get() = listOfNotNull("regular" to regular, bold?.let { "bold" to it })
}

private val SHA256 = Regex("[0-9a-f]{64}")
private val ID = Regex("[a-z0-9_]+")
private const val MAX_FONT = 20L shl 20

/** Reads fonts.json; every file must be https with a SHA-256. */
fun parseFonts(json: String): List<FontSpec> {
    val all = JSONArray(json)
    return (0 until all.length()).map { i ->
        val o = all.getJSONObject(i)
        val id = o.getString("id")
        require(ID.matches(id)) { "bad font id $id" }
        fun file(key: String) = o.optJSONObject(key)?.let { f ->
            FontFile(f.getString("url"), f.getString("sha256"), f.optString("entry").ifEmpty { null }).also {
                require(it.url.startsWith("https://")) { "$id: ${it.url} is not https" }
                require(SHA256.matches(it.sha256)) { "$id: bad SHA-256" }
            }
        }
        FontSpec(id, o.getString("name"), o.getString("license"), file("regular") ?: error("$id has no regular"), file("bold"), o.optBoolean("variable"))
    }
}

/** Copies at most max bytes, reporting the running total. */
private suspend fun copyLimited(input: InputStream, out: OutputStream, max: Long, onBytes: (Long) -> Unit = {}) {
    val buffer = ByteArray(64 shl 10)
    var total = 0L
    while (true) {
        coroutineContext.ensureActive()
        val n = input.read(buffer)
        if (n < 0) break
        total += n
        if (total > max) throw IOException("larger than ${max shr 20} MB")
        out.write(buffer, 0, n)
        onBytes(total)
    }
}

/**
 * Downloads spec into dir as <id>-regular and <id>-bold. Each download is
 * checked against its SHA-256 before any byte of it is used (a mismatch
 * throws Updates.HashMismatch and keeps nothing); an archive gives up only
 * its named entry. progress runs from 0 to 1.
 */
suspend fun installFont(dir: File, spec: FontSpec, progress: (Float) -> Unit = {}) = withContext(Dispatchers.IO) {
    dir.mkdirs()
    val sources = spec.files.map { it.second.url to it.second.sha256 }.distinct()
    val parts = sources.indices.map { File(dir, "${spec.id}.$it.part") }
    val tmps = spec.files.map { File(dir, "${spec.id}-${it.first}.tmp") }
    try {
        sources.forEachIndexed { i, (url, sha) ->
            val conn = URL(url).openConnection().apply {
                connectTimeout = 30_000
                readTimeout = 30_000
                useCaches = false
            }
            val length = conn.contentLengthLong
            conn.getInputStream().use { input ->
                parts[i].outputStream().use { out ->
                    copyLimited(input, out, MAX_FONT) { if (length > 0) progress((i + it.toFloat() / length) / sources.size) }
                }
            }
            if (Updates.sha256(parts[i]) != sha) throw Updates.HashMismatch()
        }
        spec.files.forEachIndexed { i, (weight, f) ->
            val part = parts[sources.indexOf(f.url to f.sha256)]
            if (f.entry == null) {
                part.copyTo(tmps[i], overwrite = true)
            } else {
                ZipFile(part).use { zip ->
                    val entry = zip.getEntry(f.entry) ?: throw IOException("${f.entry} is not in ${f.url}")
                    zip.getInputStream(entry).use { input -> tmps[i].outputStream().use { copyLimited(input, it, MAX_FONT) } }
                }
            }
            if (!tmps[i].renameTo(Fonts.file(dir, spec.id, weight))) throw IOException("cannot write ${spec.id}-$weight")
        }
        progress(1f)
    } finally {
        (parts + tmps).forEach(File::delete)
    }
}

/** The bundled fonts and the downloadable ones of assets/fonts.json (FINAL-PLAN T2). */
object Fonts {
    const val DEFAULT = "jetbrains"

    /** The bundled fonts, OFL 1.1, with their names as the picker shows them. */
    val BUNDLED = listOf("jetbrains" to "JetBrains Mono Nerd Font", "adwaita" to "Adwaita Mono")
    private val bundled = mapOf(
        "jetbrains" to FontFamily(
            Font(R.font.jetbrains_mono_nerd_regular, FontWeight.Normal),
            Font(R.font.jetbrains_mono_nerd_bold, FontWeight.Bold),
        ),
        "adwaita" to FontFamily(
            Font(R.font.adwaita_mono_regular, FontWeight.Normal),
            Font(R.font.adwaita_mono_bold, FontWeight.Bold),
        ),
    )
    val default get() = bundled.getValue(DEFAULT)

    sealed interface Download {
        class Running(val progress: Float) : Download
        class Failed(val message: String) : Download
    }

    /** The downloads in progress or failed, by font id. */
    val downloads = mutableStateMapOf<String, Download>()

    /** Grows with each finished download, so the picker reads the installed fonts again. */
    var installs by mutableIntStateOf(0)
        private set

    private var specs: List<FontSpec>? = null
    private val families = mutableMapOf<String, FontFamily>()

    fun specs(ctx: Context): List<FontSpec> =
        specs ?: parseFonts(ctx.assets.open("fonts.json").bufferedReader().use { it.readText() }).also { specs = it }

    fun dir(ctx: Context) = File(ctx.filesDir, "fonts")

    fun name(ctx: Context, id: String) = BUNDLED.find { it.first == id }?.second ?: specs(ctx).find { it.id == id }?.name ?: id

    fun file(dir: File, id: String, weight: String) = File(dir, "$id-$weight")

    fun installed(dir: File, spec: FontSpec) = spec.files.all { file(dir, spec.id, it.first).isFile }

    /** The family of id, or JetBrains Mono when id is neither bundled nor downloaded. */
    fun family(ctx: Context, id: String): FontFamily {
        bundled[id]?.let { return it }
        families[id]?.let { return it }
        val dir = dir(ctx)
        val spec = specs(ctx).find { it.id == id }?.takeIf { installed(dir, it) } ?: return default
        return familyOf(dir, spec).also { families[id] = it }
    }

    /** A downloaded font's family; a variable font gets its regular and bold from the wght axis. */
    fun familyOf(dir: File, spec: FontSpec): FontFamily {
        val regular = file(dir, spec.id, "regular")
        fun axis(w: FontWeight) = if (spec.variable) FontVariation.Settings(w, FontStyle.Normal) else FontVariation.Settings()
        val fonts = mutableListOf(Font(regular, FontWeight.Normal, variationSettings = axis(FontWeight.Normal)))
        when {
            spec.bold != null -> fonts += Font(file(dir, spec.id, "bold"), FontWeight.Bold)
            spec.variable -> fonts += Font(regular, FontWeight.Bold, variationSettings = axis(FontWeight.Bold))
        }
        return FontFamily(fonts)
    }

    /** Downloads spec in the app's scope, so leaving the picker does not stop it, then uses it. */
    fun download(ctx: Context, spec: FontSpec) {
        if (downloads[spec.id] is Download.Running) return
        downloads[spec.id] = Download.Running(0f)
        val app = ctx.applicationContext
        Remote.scope.launch {
            try {
                var shown = 0
                installFont(dir(app), spec) { f ->
                    // Whole percents only: each change recomposes the row.
                    val pct = (f * 100).toInt()
                    if (pct != shown) {
                        shown = pct
                        downloads[spec.id] = Download.Running(f)
                    }
                }
                downloads.remove(spec.id)
                installs++
                Look.update(app) { pickFont(app, spec.id) }
            } catch (e: CancellationException) {
                downloads.remove(spec.id)
                throw e
            } catch (e: Exception) {
                downloads[spec.id] = Download.Failed(message(app, e))
            }
        }
    }

    private fun message(ctx: Context, e: Exception) = when {
        e is Updates.HashMismatch -> ctx.getString(R.string.update_bad_hash)
        Updates.offline(e) -> ctx.getString(R.string.update_offline)
        else -> ctx.getString(R.string.font_failed, e.message.orEmpty())
    }
}

/**
 * The app's own look (FINAL-PLAN T2): font, text size, list density,
 * corners and haptics. Kept in the "look" preferences; the core does not
 * know them.
 */
object Look {
    private const val PREFS = "look"
    val TEXT_SCALES = floatArrayOf(0.9f, 1f, 1.15f, 1.3f)
    private val DENSITY_STEPS = listOf((-8).dp, 0.dp, 8.dp)

    var font by mutableStateOf(Fonts.DEFAULT)
        private set
    var family by mutableStateOf(Fonts.default)
        private set

    /** An index of TEXT_SCALES. */
    var textSize by mutableIntStateOf(1)

    /** 0 compacta, 1 normal, 2 amplia. */
    var density by mutableIntStateOf(1)
    var rounded by mutableStateOf(false)
    var haptics by mutableStateOf(true)

    fun prefs(ctx: Context) = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    fun load(ctx: Context) {
        val p = prefs(ctx)
        pickFont(ctx, p.getString("font", null) ?: Fonts.DEFAULT)
        textSize = p.getInt("text_size", 1).coerceIn(TEXT_SCALES.indices)
        density = p.getInt("density", 1).coerceIn(DENSITY_STEPS.indices)
        rounded = p.getBoolean("rounded", false)
        haptics = p.getBoolean("haptics", true)
    }

    /** Uses font id, or JetBrains Mono when it is gone (app data cleared). */
    fun pickFont(ctx: Context, id: String) {
        family = Fonts.family(ctx, id)
        font = if (family === Fonts.default) Fonts.DEFAULT else id
    }

    /** Applies change and saves the look. */
    fun update(ctx: Context, change: Look.() -> Unit) {
        change()
        prefs(ctx).edit {
            putString("font", font)
            putInt("text_size", textSize)
            putInt("density", density)
            putBoolean("rounded", rounded)
            putBoolean("haptics", haptics)
        }
    }

    /** A list row's minimum height in this density; never under the 48 dp touch target. */
    fun row(base: Dp): Dp = (base + DENSITY_STEPS[density]).coerceAtLeast(48.dp)

    private val SQUARE = RoundedCornerShape(0)
    private val ROUNDED = RoundedCornerShape(8.dp)

    /** Square corners (UI-SPEC) or rounded ones. */
    val corner: CornerBasedShape get() = if (rounded) ROUNDED else SQUARE
}

/** The chosen font as an Android Typeface, for text drawn on a Canvas (the visualizer grid). */
@Composable
fun rememberTypeface(bold: Boolean = false): android.graphics.Typeface {
    val resolver = LocalFontFamilyResolver.current
    val family = Look.family
    return remember(family, bold) {
        resolver.resolve(family, if (bold) FontWeight.Bold else FontWeight.Normal).value as android.graphics.Typeface
    }
}

/** The license's short name from its file name in assets/licenses. */
private fun licenseName(file: String) = when (file.substringAfterLast('-').substringBefore('.')) {
    "OFL" -> "OFL 1.1"
    "UFL" -> "UFL 1.0"
    else -> file.substringAfterLast('-').substringBefore('.')
}

/**
 * Fuente (Ajustes → Apariencia): a live preview in the font in use, the
 * bundled fonts, and the downloadable ones, each name drawn in its own font
 * once it is here. A tap uses an installed font, or downloads it first.
 */
@Composable
fun FontsScreen() {
    val p = LocalPalette.current
    val context = LocalContext.current
    val specs = remember { Fonts.specs(context) }
    val dir = remember { Fonts.dir(context) }
    val installed by produceState(emptyMap<String, FontFamily>(), Fonts.installs) {
        value = withContext(Dispatchers.IO) { specs.filter { Fonts.installed(dir, it) }.associate { it.id to Fonts.family(context, it.id) } }
    }
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.font))
        Column(Modifier.fillMaxWidth().background(p.surface).padding(horizontal = 16.dp, vertical = 8.dp).testTag("font_preview")) {
            Text("cliamp-x ▶ 03:14 ━━━━●──── 1.0x", color = p.accent, style = Type.headline, maxLines = 1, overflow = TextOverflow.Clip)
            Text(stringResource(R.string.font_sample), color = p.textPrimary, style = Type.body)
            Text("0O 1lI {}[]() => != ▁▂▃▄▅▆▇█ ░▒▓", color = p.textSecondary, style = Type.label)
        }
        LazyColumn(Modifier.fillMaxSize().testTag("font_list")) {
            item(key = "bundled") { SectionRule(stringResource(R.string.fonts_bundled)) }
            items(Fonts.BUNDLED, key = { it.first }) { (id, name) ->
                FontRow(id, name, "OFL 1.1", Fonts.family(context, id), null) { Look.update(context) { pickFont(context, id) } }
            }
            item(key = "download") {
                SectionRule(stringResource(R.string.fonts_download))
                Text(
                    stringResource(R.string.fonts_help), Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp),
                    color = p.textSecondary, style = Type.label,
                )
            }
            items(specs, key = { it.id }) { spec ->
                val family = installed[spec.id]
                FontRow(spec.id, spec.name, licenseName(spec.license), family, Fonts.downloads[spec.id]) {
                    if (family != null) Look.update(context) { pickFont(context, spec.id) } else Fonts.download(context, spec)
                }
            }
        }
    }
}

/**
 * A font: ● when in use, its name in its own font, the license, and when
 * it is not here (family null) `[Descargar]`, the download's bar, or its error.
 */
@Composable
private fun FontRow(id: String, name: String, license: String, family: FontFamily?, download: Fonts.Download?, onClick: () -> Unit) {
    val p = LocalPalette.current
    val on = Look.font == id
    val get = stringResource(R.string.font_get)
    Column(
        Modifier.fillMaxWidth().heightIn(min = Look.row(56.dp)).clickable(onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 4.dp).testTag("font_$id")
            .semantics { if (family == null) stateDescription = get },
        verticalArrangement = Arrangement.Center,
    ) {
        Row(Modifier.heightIn(min = 40.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(if (on) "● " else "  ", color = p.accent, style = Type.body)
            Text(
                name, Modifier.weight(1f), color = if (on) p.accent else p.textPrimary,
                style = Type.body.bold(on).let { if (family != null) it.copy(fontFamily = family) else it },
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
            Text(license, Modifier.padding(start = 8.dp), color = p.textSecondary, style = Type.caption)
            if (download is Fonts.Download.Running) {
                val cells = (download.progress * 8).toInt()
                Text(" ▕" + "█".repeat(cells) + "░".repeat(8 - cells) + "▏" + "%3d%%".format((download.progress * 100).toInt()), color = p.accent, style = Type.label)
            } else if (family == null) {
                Text(" [$get]", color = p.accent, style = Type.label)
            }
        }
        if (download is Fonts.Download.Failed) Text("ERR: " + download.message, color = p.error, style = Type.label)
    }
}
