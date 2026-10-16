package io.github.xaviiromf.cliampx

import org.json.JSONObject
import java.io.File

/** M3U playlist text, for Importar M3U and Exportar M3U in Biblioteca (FINAL-PLAN T9). */
object M3u {
    const val MAX_BYTES = 2 * 1024 * 1024
    const val MAX_ENTRIES = 5000

    /** One entry: a path or URL, the #EXTINF title ("" when there is none) and its seconds (-1 unknown). */
    data class Entry(val path: String, val title: String, val seconds: Int)

    private fun isUrl(s: String) = s.startsWith("http://", true) || s.startsWith("https://", true)

    /** The entries of text: #EXTINF titles kept, comments, blank lines and a BOM dropped. A backslash in a relative path is a slash. */
    fun parse(text: String): List<Entry> {
        val out = ArrayList<Entry>()
        var title = ""
        var seconds = -1
        for (raw in text.removePrefix("﻿").lineSequence()) {
            val line = raw.trim()
            when {
                line.isEmpty() || line.startsWith("#EXTM3U") -> {}
                line.startsWith("#EXTINF:") -> {
                    val (d, t) = line.removePrefix("#EXTINF:").split(',', limit = 2).let { it[0] to it.getOrElse(1) { "" } }
                    seconds = d.trim().substringBefore(' ').toIntOrNull() ?: -1
                    title = t.trim()
                }
                line.startsWith("#") -> {}
                else -> {
                    out += Entry(if (isUrl(line)) line else line.replace('\\', '/'), title, seconds)
                    title = ""
                    seconds = -1
                }
            }
        }
        return out
    }

    /** What an entry plays: a URL as it is, an absolute path or a relative one under the first of roots where exists says it is, else null. */
    fun resolve(path: String, roots: List<String>, exists: (String) -> Boolean): String? = when {
        isUrl(path) -> path
        path.startsWith("/") -> File(path).normalize().path.takeIf(exists)
        "://" in path -> null
        else -> roots.firstNotNullOfOrNull { File(it, path).normalize().path.takeIf(exists) }
    }

    /** The core's track for entry at path: "Artist - Title" splits in two, a title-less entry is named after its file. */
    fun track(e: Entry, path: String): JSONObject {
        val name = e.title.ifEmpty { if (isUrl(path)) "" else path.substringAfterLast('/').substringBeforeLast('.') }
        val o = JSONObject().put("path", path)
        val (artist, title) = name.split(" - ", limit = 2).let { if (it.size == 2 && it[0].isNotBlank() && it[1].isNotBlank()) it[0].trim() to it[1].trim() else "" to name }
        if (title.isNotEmpty()) o.put("title", title)
        if (artist.isNotEmpty()) o.put("artist", artist)
        if (e.seconds > 0) o.put("duration_secs", e.seconds)
        return o
    }

    /** An #EXTM3U file of tracks, local ones by path and YouTube ones by watch URL; a track with neither is left out. */
    fun write(tracks: List<Track>): String = buildString {
        append("#EXTM3U\n")
        for (t in tracks) {
            if (!t.path.startsWith("/") && !isUrl(t.path)) continue
            val label = (if (t.artist.isNotEmpty() && t.title.isNotEmpty()) "${t.artist} - ${t.title}" else t.name).replace(Regex("[\r\n]+"), " ")
            append("#EXTINF:").append(if (t.durationSecs > 0) t.durationSecs else -1).append(',').append(label).append('\n')
            append(t.path.replace(Regex("[\r\n]+"), "")).append('\n')
        }
    }
}
