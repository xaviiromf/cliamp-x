package io.github.xaviiromf.cliampx

import android.content.Context
import android.content.SharedPreferences
import androidx.compose.runtime.mutableStateListOf
import androidx.core.content.edit
import org.json.JSONArray
import org.json.JSONObject
import java.net.URI

/** A saved radio station or podcast feed: what the user calls it and its URL. */
data class Station(val name: String, val url: String)

/** Radio y podcasts' saved list, kept as a JSON array in the "stations" preferences. */
object Stations {
    const val MAX = 200

    val list = mutableStateListOf<Station>()
    private var prefs: SharedPreferences? = null

    fun load(ctx: Context) {
        val p = ctx.applicationContext.getSharedPreferences("stations", Context.MODE_PRIVATE).also { prefs = it }
        list.clear()
        list += parseAll(runCatching { JSONArray(p.getString(KEY, "[]")) }.getOrNull())
    }

    private fun store() = prefs?.edit { putString(KEY, JSONArray(list.map { JSONObject().put("name", it.name).put("url", it.url) }).toString()) }

    fun parseAll(a: JSONArray?): List<Station> = (0 until (a?.length() ?: 0)).mapNotNull { i ->
        val o = a!!.optJSONObject(i) ?: return@mapNotNull null
        val url = valid(o.optString("url")) ?: return@mapNotNull null
        Station(o.optString("name").trim().ifEmpty { defaultName(url) }, url)
    }.distinctBy { it.url }.take(MAX)

    /** The trimmed URL when raw is an absolute http(s) URL with a host and no spaces, else null. */
    fun valid(raw: String): String? {
        val s = raw.trim()
        if (s.any(Char::isWhitespace)) return null
        val u = runCatching { URI(s) }.getOrNull() ?: return null
        return s.takeIf { u.scheme.equals("http", true) || u.scheme.equals("https", true) }?.takeIf { !u.host.isNullOrEmpty() }
    }

    /** A name for url until the user picks one: its host. */
    fun defaultName(url: String): String = runCatching { URI(url).host }.getOrNull()?.removePrefix("www.").orEmpty().ifEmpty { url }

    /** Adds the station, or renames the one with the same URL. A full list refuses. */
    fun put(s: Station) {
        val i = list.indexOfFirst { it.url == s.url }
        if (i >= 0) list[i] = s else if (list.size < MAX) list += s
        store()
    }

    fun rename(url: String, name: String) {
        val i = list.indexOfFirst { it.url == url }
        if (i < 0) return
        list[i] = list[i].copy(name = name.trim().ifEmpty { defaultName(url) })
        store()
    }

    fun remove(url: String) {
        list.removeAll { it.url == url }
        store()
    }

    /** Whether the tracks url.list returned are the episodes of a podcast feed. */
    fun isFeed(tracks: List<Track>) = tracks.isNotEmpty() && tracks.all { "podcast.feed" in it.meta }

    /** A feed's episodes newest first by their publication date (YYYY-MM-DD); ones without a date, and ties, keep the feed's order. */
    fun newestFirst(tracks: List<Track>): List<Track> = tracks.sortedByDescending { it.meta["podcast.published"].orEmpty() }

    /** What to call a resolved URL: the podcast's name, the station's, or the host. */
    fun nameOf(url: String, tracks: List<Track>): String {
        val t = tracks.firstOrNull() ?: return defaultName(url)
        return (if (isFeed(tracks)) t.artist else t.station.ifEmpty { t.title }).trim().ifEmpty { defaultName(url) }
    }

    private const val KEY = "list"
}
