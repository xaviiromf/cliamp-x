package io.github.xaviiromf.cliampx

import android.content.Context
import android.net.Uri
import android.widget.Toast
import androidx.annotation.StringRes
import io.github.xaviiromf.cliampx.ui.Personalization
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.IOException
import java.io.InputStream
import java.io.OutputStream
import java.nio.ByteBuffer
import java.nio.charset.CodingErrorAction
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.util.zip.ZipEntry
import java.util.zip.ZipInputStream
import java.util.zip.ZipOutputStream

/**
 * Ajustes → Copia de seguridad: the user's data in one zip (docs/android.md).
 *
 * Layout, `manifest.json` first: `config/` is a whitelist of the core's config
 * directory (settings, favorites, history, ratings, local playlists, user
 * themes), `data/` of its data directory (play log, search history), `app/`
 * the app's own state (personalization and a few SharedPreferences). The
 * YouTube Music session (`config/ytmusic_*`) is in only when the manifest says
 * so. Credentials in config.toml are stripped, and downloads, caches, yt-dlp
 * and logs are never included.
 *
 * Importing is a trust boundary: [read] checks the whole zip against the
 * whitelist and the limits and writes nothing; [restore] runs on a stopped core.
 */
object Backup {
    const val FORMAT = "cliamp-x-backup"
    const val VERSION = 1
    const val MANIFEST = "manifest.json"
    const val MAX_ENTRY = 24 shl 20
    const val MAX_TOTAL = 64 shl 20
    const val MAX_ENTRIES = 1000
    private const val MAX_NAME = 120

    private val FIXED = setOf(
        "config/config.toml", "config/favorites.toml", "config/history.toml", "config/taste.json", "config/ytmusic_genres.json",
        "data/plays.jsonl", "data/search_history.json", "app/personalization.json",
    )
    private val SESSION = setOf("config/ytmusic_cookies.txt", "config/ytmusic_auth.json")
    private val PREFS = listOf("ui", "music", "routes", "offline", "stations")
    private val DIRS = setOf("config/", "config/playlists/", "config/themes/", "data/", "app/", "app/prefs/")
    private val NAMED = Regex("""^config/(playlists|themes)/[^/\\\p{Cntrl}.][^/\\\p{Cntrl}]*\.toml$""")
    private val SECRET = Regex("""(?i)(token|secret|passw|session|api_?key|cookie|credential|auth)""")

    /** A refusal with the message for the user. */
    class Invalid(@StringRes val res: Int, vararg val args: Any) : IOException("backup refused: $res ${args.toList()}")

    class Content(val files: Map<String, ByteArray>, val session: Boolean)

    /** A checked backup waiting for the core to stop. */
    @Volatile var pending: Content? = null

    fun allowed(name: String, session: Boolean): Boolean {
        if (name.length > MAX_NAME + 20 || ".." in name) return false
        if (name == MANIFEST || name in FIXED) return true
        if (name in SESSION) return session
        if (name.startsWith("app/prefs/") && PREFS.any { name == "app/prefs/$it.json" }) return true
        return NAMED.matches(name) && name.substringAfterLast('/').length <= MAX_NAME
    }

    fun manifest(session: Boolean): ByteArray =
        JSONObject().put("format", FORMAT).put("version", VERSION).put("session", session).toString().toByteArray()

    /** Reads the zip and refuses it whole at the first thing that is not on the whitelist or past a limit. */
    fun read(input: InputStream): Content {
        val files = LinkedHashMap<String, ByteArray>()
        var total = 0
        try {
            ZipInputStream(input).use { zip ->
                var entry: ZipEntry? = zip.nextEntry
                while (entry != null) {
                    val name = entry.name
                    if (entry.isDirectory) {
                        if (name !in DIRS) throw Invalid(R.string.backup_err_path, name)
                    } else {
                        // Whether the session may be in comes from the manifest, checked below.
                        if (!allowed(name, true)) throw Invalid(R.string.backup_err_path, name)
                        if (name in files || files.size >= MAX_ENTRIES) throw Invalid(R.string.backup_err_path, name)
                        // The sizes in the zip headers are not trusted: count what comes out.
                        val out = ByteArrayOutputStream()
                        val buf = ByteArray(16 * 1024)
                        while (true) {
                            val n = zip.read(buf)
                            if (n < 0) break
                            out.write(buf, 0, n)
                            total += n
                            if (out.size() > MAX_ENTRY) throw Invalid(R.string.backup_err_size, name)
                            if (total > MAX_TOTAL) throw Invalid(R.string.backup_err_total)
                        }
                        files[name] = out.toByteArray()
                    }
                    entry = zip.nextEntry
                }
            }
        } catch (e: Invalid) {
            throw e
        } catch (e: IOException) {
            throw Invalid(R.string.backup_err_zip, e.message.orEmpty())
        }
        val m = try {
            JSONObject(utf8(files[MANIFEST] ?: throw Invalid(R.string.backup_err_format)))
        } catch (e: org.json.JSONException) {
            throw Invalid(R.string.backup_err_format)
        }
        if (m.optString("format") != FORMAT) throw Invalid(R.string.backup_err_format)
        if (m.opt("version") != VERSION) throw Invalid(R.string.backup_err_version, "${m.opt("version")}")
        val session = m.optBoolean("session", false)
        files.remove(MANIFEST)
        files.keys.firstOrNull { !allowed(it, session) }?.let { throw Invalid(R.string.backup_err_path, it) }
        files.forEach { (name, bytes) -> check(name, bytes) }
        return Content(files, session)
    }

    /** Strict UTF-8 text. */
    private fun utf8(bytes: ByteArray): String = try {
        Charsets.UTF_8.newDecoder().onMalformedInput(CodingErrorAction.REPORT).onUnmappableCharacter(CodingErrorAction.REPORT)
            .decode(ByteBuffer.wrap(bytes)).toString()
    } catch (e: java.nio.charset.CharacterCodingException) {
        throw Invalid(R.string.backup_err_content, "UTF-8")
    }

    /** The entry is UTF-8 and, by extension, JSON, JSON lines or TOML without control characters (the core ignores what it cannot parse). */
    private fun check(name: String, bytes: ByteArray) {
        try {
            val text = utf8(bytes)
            when {
                name.endsWith(".jsonl") -> text.lineSequence().filter { it.isNotBlank() }.forEach { JSONObject(it) }
                name.endsWith(".json") -> when (JSONTokenerValue(text)) {
                    is JSONObject, is JSONArray -> {}
                    else -> throw Invalid(R.string.backup_err_content, name)
                }
                else -> if (text.any { it < ' ' && it != '\n' && it != '\r' && it != '\t' }) throw Invalid(R.string.backup_err_content, name)
            }
            if (name == "app/personalization.json") Personalization.parse(text)
            if (name.startsWith("app/prefs/")) prefsFromJson(JSONObject(text))
        } catch (e: Invalid) {
            throw e
        } catch (e: Exception) {
            throw Invalid(R.string.backup_err_content, name)
        }
    }

    private fun JSONTokenerValue(text: String): Any = org.json.JSONTokener(text).nextValue()

    /** One SharedPreferences as {"key": ["b"|"i"|"l"|"f"|"s"|"ss", value]}. */
    fun prefsToJson(all: Map<String, *>): JSONObject = JSONObject().also { o ->
        for ((k, v) in all) when (v) {
            is Boolean -> o.put(k, JSONArray().put("b").put(v))
            is Int -> o.put(k, JSONArray().put("i").put(v))
            is Long -> o.put(k, JSONArray().put("l").put(v))
            is Float -> o.put(k, JSONArray().put("f").put(v.toDouble()))
            is String -> o.put(k, JSONArray().put("s").put(v))
            is Set<*> -> o.put(k, JSONArray().put("ss").put(JSONArray(v.map { it.toString() })))
        }
    }

    /** The typed values of prefsToJson; anything else throws. */
    fun prefsFromJson(o: JSONObject): Map<String, Any> = o.keys().asSequence().associateWith { k ->
        require(k.length <= 100)
        val a = o.getJSONArray(k)
        when (a.getString(0)) {
            "b" -> a.getBoolean(1)
            "i" -> a.getInt(1)
            "l" -> a.getLong(1)
            "f" -> a.getDouble(1).toFloat()
            "s" -> a.getString(1).also { require(it.length <= 65536) }
            "ss" -> a.getJSONArray(1).let { s -> (0 until s.length()).map { i -> s.getString(i) }.toSet() }.also { require(it.size <= 1000) }
            else -> throw IllegalArgumentException("type")
        }
    }

    /** config.toml without the lines that look like credentials (scrobble tokens and keys, passwords), which a backup never carries. */
    fun stripSecrets(toml: String): String =
        toml.lineSequence().filterNot { it.substringBefore('=', "").let { k -> '=' in it && SECRET.containsMatchIn(k) } }.joinToString("\n")

    /** The files to export, by zip name. A file over the limit refuses the export. */
    fun collect(config: File, data: File, app: Map<String, ByteArray>, session: Boolean): Map<String, ByteArray> {
        val out = LinkedHashMap<String, ByteArray>()
        fun add(name: String, f: File) {
            if (!allowed(name, session) || !f.isFile || Files.isSymbolicLink(f.toPath())) return
            if (f.length() > MAX_ENTRY) throw Invalid(R.string.backup_err_size, name)
            val bytes = f.readBytes()
            out[name] = if (name == "config/config.toml") stripSecrets(String(bytes, Charsets.UTF_8)).toByteArray() else bytes
        }
        for (n in FIXED + SESSION) when {
            n.startsWith("config/") -> add(n, File(config, n.removePrefix("config/")))
            n.startsWith("data/") -> add(n, File(data, n.removePrefix("data/")))
        }
        for (d in listOf("playlists", "themes")) File(config, d).listFiles()?.sortedBy { it.name }?.forEach { add("config/$d/${it.name}", it) }
        app.forEach { (n, b) -> if (allowed(n, session)) out[n] = b }
        if (out.values.sumOf { it.size.toLong() } > MAX_TOTAL) throw Invalid(R.string.backup_err_total)
        return out
    }

    fun write(out: OutputStream, files: Map<String, ByteArray>, session: Boolean) = ZipOutputStream(out).use { zip ->
        for ((name, bytes) in linkedMapOf(MANIFEST to manifest(session)) + files) {
            zip.putNextEntry(ZipEntry(name))
            zip.write(bytes)
            zip.closeEntry()
        }
    }

    /** Writes every config/ and data/ file next to its place first, then renames them all over it: a failed write changes nothing. */
    fun restore(config: File, data: File, files: Map<String, ByteArray>) {
        val roots = mapOf("config/" to config, "data/" to data)
        val placed = mutableListOf<Pair<File, File>>()
        try {
            for ((name, bytes) in files) {
                val (prefix, root) = roots.entries.firstOrNull { name.startsWith(it.key) }?.let { it.key to it.value } ?: continue
                val dest = File(root, name.removePrefix(prefix))
                dest.parentFile!!.mkdirs()
                // The whitelist has no "..", and a parent that is a link out of the directory is refused too.
                if (!dest.parentFile!!.canonicalPath.startsWith(root.canonicalPath)) throw Invalid(R.string.backup_err_path, name)
                val tmp = File(dest.parentFile, ".${dest.name}.restore")
                tmp.writeBytes(bytes)
                placed += tmp to dest
            }
            placed.forEach { (tmp, dest) -> Files.move(tmp.toPath(), dest.toPath(), StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING) }
        } finally {
            placed.forEach { it.first.delete() }
        }
    }

    /** The app's part of the export: personalization and the SharedPreferences, by zip name. */
    private fun appState(ctx: Context, s: Snapshot?): Map<String, ByteArray> =
        linkedMapOf("app/personalization.json" to Personalization.now(ctx, s).json().toString().toByteArray()) +
            PREFS.associate { "app/prefs/$it.json" to prefsToJson(ctx.getSharedPreferences(it, Context.MODE_PRIVATE).all).toString().toByteArray() }

    /** Writes the backup to uri; the result shows as a toast. */
    fun export(ctx: Context, uri: Uri, s: Snapshot?, session: Boolean) {
        val app = ctx.applicationContext
        Remote.scope.launch(Dispatchers.Main) {
            val msg = try {
                withContext(Dispatchers.IO) {
                    val files = collect(Core.configDir(app), Core.dataDir(app), appState(app, s), session)
                    (app.contentResolver.openOutputStream(uri, "wt") ?: throw IOException("cannot open the file")).use { write(it, files, session) }
                }
                app.getString(R.string.backup_export_done)
            } catch (e: CancellationException) {
                throw e
            } catch (e: Invalid) {
                app.getString(R.string.export_failed, app.getString(e.res, *e.args))
            } catch (e: Exception) {
                app.getString(R.string.export_failed, e.message.orEmpty())
            }
            Toast.makeText(app, msg, Toast.LENGTH_LONG).show()
        }
    }

    /** Checks uri; a good backup becomes pending and onChecked runs with it, a bad one shows its reason and changes nothing. */
    fun check(ctx: Context, uri: Uri, onChecked: (Content) -> Unit) {
        val app = ctx.applicationContext
        Remote.scope.launch(Dispatchers.Main) {
            try {
                val c = withContext(Dispatchers.IO) { (app.contentResolver.openInputStream(uri) ?: throw IOException("cannot open the file")).use(::read) }
                pending = c
                onChecked(c)
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                val why = if (e is Invalid) app.getString(e.res, *e.args) else e.message.orEmpty()
                Toast.makeText(app, app.getString(R.string.backup_import_failed, why), Toast.LENGTH_LONG).show()
            }
        }
    }

    /** With the core stopped: the files, then the app state, which the screens read from here on. */
    fun apply(ctx: Context, c: Content) {
        restore(Core.configDir(ctx), Core.dataDir(ctx), c.files)
        for (n in PREFS) c.files["app/prefs/$n.json"]?.let { b ->
            ctx.getSharedPreferences(n, Context.MODE_PRIVATE).edit().also { e ->
                e.clear()
                for ((k, v) in prefsFromJson(JSONObject(String(b, Charsets.UTF_8)))) when (v) {
                    is Boolean -> e.putBoolean(k, v)
                    is Int -> e.putInt(k, v)
                    is Long -> e.putLong(k, v)
                    is Float -> e.putFloat(k, v)
                    is String -> e.putString(k, v)
                    is Set<*> -> e.putStringSet(k, v.map { it.toString() }.toSet())
                }
            }.commit()
        }
        c.files["app/personalization.json"]?.let { Personalization.apply(ctx, Personalization.parse(String(it, Charsets.UTF_8))) }
    }
}
