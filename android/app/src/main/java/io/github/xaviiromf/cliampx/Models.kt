package io.github.xaviiromf.cliampx

import androidx.annotation.StringRes
import androidx.compose.runtime.Immutable
import org.json.JSONObject
import kotlin.math.roundToInt

// The core's JSON (docs/remote-control.md) as immutable models, parsed off the
// main thread. Go leaves out empty fields (omitempty): a missing number is 0, a
// missing bool false and a missing string "".

/** A track as the core sends it. raw is its JSON, sent back unchanged so provider_meta keeps its identity. */
@Immutable
data class Track(
    val raw: String,
    val path: String,
    val title: String,
    val artist: String,
    val album: String,
    val artUrl: String,
    val durationSecs: Int,
    val meta: Map<String, String>,
    /** The ♥ favorite (the core's bookmark field). */
    val favorite: Boolean = false,
    /** The row's index in the live playlist, in queue.list and playnext.list rows. */
    val index: Int = 0,
) {
    /** The title, or the file name of a track that has none. */
    val name get() = title.ifEmpty { path.substringAfterLast('/') }

    /** What the art loader reads: the cover URL, or the file of a local track. */
    val art get() = artUrl.ifEmpty { path.takeIf { it.startsWith("/") }.orEmpty() }

    fun json() = JSONObject(raw)

    companion object {
        fun parse(o: JSONObject): Track {
            val meta = o.optJSONObject("provider_meta")
            return Track(
                raw = o.toString(),
                path = o.optString("path"),
                title = o.optString("title"),
                artist = o.optString("artist"),
                album = o.optString("album"),
                artUrl = o.optString("album_art_url"),
                durationSecs = o.optInt("duration_secs"),
                meta = meta?.keys()?.asSequence()?.associateWith { meta.optString(it) }.orEmpty(),
                favorite = o.optBoolean("bookmark"),
                index = o.optInt("index"),
            )
        }

        /** The tracks array of a result, or none. */
        fun list(result: JSONObject): List<Track> {
            val a = result.optJSONArray("tracks") ?: return emptyList()
            return (0 until a.length()).map { parse(a.getJSONObject(it)) }
        }
    }
}

/** The colors of a cliamp-x theme as hex strings. Default sends only its name. */
@Immutable
data class ThemeInfo(
    val name: String, val bg: String, val accent: String, val brightFg: String,
    val fg: String, val green: String, val yellow: String, val red: String,
) {
    companion object {
        fun parse(o: JSONObject) = ThemeInfo(
            o.optString("name"), o.optString("bg"), o.optString("accent"), o.optString("bright_fg"),
            o.optString("fg"), o.optString("green"), o.optString("yellow"), o.optString("red"),
        )

        /** The themes of `theme {name:"list"}` (G17), Default first. */
        fun list(result: JSONObject): List<ThemeInfo> {
            val a = result.optJSONArray("themes") ?: return emptyList()
            return List(a.length()) { parse(a.getJSONObject(it)) }
        }
    }
}

/** The core's status message: kind is activity, success, warning or error. */
@Immutable
data class Notice(val text: String, val kind: String)

/** The runtime snapshot. receivedAt is the elapsedRealtime of its position. */
@Immutable
data class Snapshot(
    val revision: Long,
    val playlistRevision: Long,
    val state: String,
    val track: Track?,
    val position: Double,
    val duration: Double,
    val seekable: Boolean,
    val volume: Double,
    val index: Int,
    val total: Int,
    val playNextTotal: Int,
    val shuffle: Boolean,
    val repeat: String,
    val speed: Double,
    val theme: ThemeInfo?,
    val buffering: Boolean,
    val seeking: Boolean,
    val error: String,
    val errorKind: String,
    val errorPath: String,
    val notice: Notice?,
    val sleepMinutes: Int,
    val sleepAt: Long,
    val autoRadio: Boolean,
    val normalize: String,
    val crossfade: Int,
    val eqAuto: Boolean,
    val eqPreset: String,
    val eqBands: List<Double>,
    val visualizer: String,
    val receivedAt: Long,
    val mono: Boolean = false,
    val qualityWifi: String = "",
    val qualityMobile: String = "",
    val premiumQuality: Boolean = false,
    val scrobbleListenbrainz: Boolean = false,
    val scrobbleLastfm: Boolean = false,
    val audioCodec: String = "",
    val audioKbps: Int = 0,
    val audioRate: Int = 0,
) {
    val playing get() = state == "playing"

    /** The play error of the track on screen, which the error card shows (§2.10), or null. */
    val playError get() = error.takeIf { it.isNotEmpty() && track != null && errorPath == track.path }

    companion object {
        fun parse(o: JSONObject, receivedAt: Long) = Snapshot(
            revision = o.optLong("revision"),
            playlistRevision = o.optLong("playlist_revision"),
            state = o.optString("state"),
            track = o.optJSONObject("track")?.let(Track::parse),
            position = o.optDouble("position", 0.0),
            duration = o.optDouble("duration", 0.0),
            seekable = o.optBoolean("seekable"),
            volume = o.optDouble("volume", 0.0),
            index = o.optInt("index"),
            total = o.optInt("total"),
            playNextTotal = o.optInt("play_next_total"),
            shuffle = o.optBoolean("shuffle"),
            repeat = o.optString("repeat"),
            speed = o.optDouble("speed", 0.0).takeIf { it > 0 } ?: 1.0,
            theme = o.optJSONObject("theme")?.let(ThemeInfo::parse),
            buffering = o.optBoolean("buffering"),
            seeking = o.optBoolean("seeking"),
            error = o.optString("error"),
            errorKind = o.optString("error_kind"),
            errorPath = o.optString("error_path"),
            notice = o.optJSONObject("notice")?.let { Notice(it.optString("text"), it.optString("kind")) }?.takeIf { it.text.isNotEmpty() },
            sleepMinutes = o.optInt("sleep_minutes"),
            sleepAt = o.optLong("sleep_at"),
            autoRadio = o.optBoolean("auto_radio"),
            normalize = o.optString("normalize"),
            crossfade = o.optInt("crossfade"),
            eqAuto = o.optBoolean("eq_auto"),
            eqPreset = o.optString("eq_preset"),
            eqBands = o.optJSONArray("eq_bands")?.let { a -> List(a.length()) { a.optDouble(it, 0.0) } }.orEmpty(),
            visualizer = o.optString("visualizer"),
            receivedAt = receivedAt,
            mono = o.optBoolean("mono"),
            qualityWifi = o.optString("quality_wifi"),
            qualityMobile = o.optString("quality_mobile"),
            premiumQuality = o.optBoolean("premium_quality"),
            scrobbleListenbrainz = o.optBoolean("scrobble_listenbrainz"),
            scrobbleLastfm = o.optBoolean("scrobble_lastfm"),
            audioCodec = o.optString("audio_codec"),
            audioKbps = o.optInt("audio_kbps"),
            audioRate = o.optInt("audio_rate"),
        )
    }
}

/** Where a snapshot came from (§5.4). */
enum class Source { EVENT, RETAINED, STATE_GET, PLAYBACK_JOB, OTHER_JOB }

object Snapshots {
    /** The operations whose job snapshot carries a fresh position. */
    val PLAYBACK_OPS = setOf(
        "play", "pause", "toggle", "stop", "next", "prev", "seek", "seek.absolute", "queue.play", "queue.replace",
        "track.play", "provider.load", "provider.load_album", "radio.start", "url.load", "speed",
    )

    /**
     * Merges new into cur, or returns null to ignore it as older than floor.
     * A retained event or an unrelated job snapshot can carry a minutes-old
     * position, so those keep the current one unless the track changed.
     */
    fun merge(cur: Snapshot?, new: Snapshot, source: Source, floor: Long): Snapshot? {
        if (new.revision < floor) return null
        val fresh = cur == null || source == Source.EVENT || source == Source.STATE_GET ||
            source == Source.PLAYBACK_JOB || cur.track?.path != new.track?.path
        return if (fresh) new else new.copy(position = cur.position, receivedAt = cur.receivedAt)
    }
}

object PlaybackClock {
    /** The position at now (elapsedRealtime), moving only while the song really plays. */
    fun positionAt(s: Snapshot, now: Long): Double {
        if (s.state != "playing" || s.buffering || s.seeking) return s.position
        val p = s.position + (now - s.receivedAt) / 1000.0 * s.speed
        return if (s.duration > 0) p.coerceAtMost(s.duration) else p
    }
}

/** What a row or a tile is: a search row's kind comes from its albumID (§5.3 Buscar). */
enum class Kind { SONG, ARTIST, ALBUM, PLAYLIST, GENRE }

val Track.kind: Kind
    get() {
        val id = meta["albumID"] ?: return Kind.SONG
        return when {
            id.startsWith("ar:") -> Kind.ARTIST
            id.startsWith("pl:") -> Kind.PLAYLIST
            else -> Kind.ALBUM
        }
    }

/** The ♥ of t: the state the core last reported for its path (Remote.liked), else the row's own. */
fun Map<String, Boolean>.likes(t: Track) = this[t.path] ?: t.favorite

/** A search placeholder row as the item its page and sheet open. */
fun Track.item() = Item(kind, meta["albumID"].orEmpty().removePrefix("ar:"), title, artist, artUrl)

@Immutable
data class Section(val title: String, val rows: List<Track>)

/** Search results grouped by provider_meta.section, in arrival order. */
fun searchSections(tracks: List<Track>): List<Section> =
    tracks.groupBy { it.meta["section"].orEmpty() }.map { (title, rows) -> Section(title, rows) }

/**
 * What a tile, a row or an action sheet shows (§2.6): a song (with its
 * track), an album, an artist, a playlist or a genre. id is what the
 * provider operations take. section is the row's section, or a genre's group.
 */
@Immutable
data class Item(
    val kind: Kind,
    val id: String,
    val title: String,
    val subtitle: String = "",
    val art: String = "",
    val track: Track? = null,
    val favorite: Boolean = false,
    val unseen: Boolean = false,
    val provider: String = "ytmusic",
    val artistId: String = "",
    val writable: Boolean = false,
    val count: Int = 0,
    val section: String = "",
) {
    val local get() = id.startsWith("local:") || provider == "local"
}

/** A home shelf: its title and cards. */
@Immutable
data class Shelf(val title: String, val items: List<Item>)

/** The picture and detail lines of an album, artist or playlist; favorite is null while unknown (signed out). */
@Immutable
data class Preview(val art: String, val lines: List<String>, val favorite: Boolean?)

/** A genre's play order. */
@Immutable
data class Sort(val id: String, val label: String)

/** The browse results of the core (docs/remote-control.md) as items. */
object Items {
    private fun objects(result: JSONObject, name: String): List<JSONObject> {
        val a = result.optJSONArray(name) ?: return emptyList()
        return List(a.length()) { a.getJSONObject(it) }
    }

    /** provider.albums and provider.artist_albums rows. */
    fun albums(result: JSONObject) = objects(result, "albums").map {
        val single = it.optBoolean("single")
        Item(
            Kind.ALBUM, it.optString("id"), RowName.of(it.optString("name"), single),
            listOf(it.optString("artist"), it.optInt("year").takeIf { y -> y > 0 }?.toString().orEmpty()).filter(String::isNotEmpty).joinToString(" · "),
            it.optString("image_url"), favorite = it.optBoolean("favorite"), artistId = it.optString("artist_id"),
            count = it.optInt("track_count"), section = if (single) "single" else "",
        )
    }

    /** provider.artists rows. */
    fun artists(result: JSONObject) = objects(result, "artists").map {
        Item(Kind.ARTIST, it.optString("id"), RowName.of(it.optString("name")), art = it.optString("image_url"), favorite = it.optBoolean("favorite"))
    }

    /**
     * provider.playlists rows: a release (al:, named `Title · Artist[ · Single]`)
     * is an album, a starred genre (genre:) a genre by the ID after the
     * prefix, anything else a playlist.
     */
    fun playlists(result: JSONObject) = objects(result, "playlists").map {
        val id = it.optString("id")
        val name = RowName.of(it.optString("name"))
        val base = Item(
            Kind.PLAYLIST, id, name, art = it.optString("image_url"), favorite = it.optBoolean("favorite"),
            unseen = it.optBoolean("unseen"), provider = it.optString("provider"), writable = it.optBoolean("writable"),
            count = it.optInt("track_count"), section = it.optString("section"),
        )
        when {
            id.startsWith("al:") -> base.copy(kind = Kind.ALBUM, title = name.substringBefore(" · "), subtitle = name.substringAfter(" · ", ""))
            id.startsWith("genre:") -> base.copy(kind = Kind.GENRE, id = id.removePrefix("genre:"))
            else -> base
        }
    }

    /** provider.genres: the genres (group in section) and the play orders. */
    fun genres(result: JSONObject): Pair<List<Item>, List<Sort>> =
        objects(result, "genres").map {
            Item(Kind.GENRE, it.optString("id"), RowName.of(it.optString("name")), favorite = it.optBoolean("favorite"), section = it.optString("group"))
        } to sorts(result)

    /** The play orders of provider.genres and provider.albums. */
    fun sorts(result: JSONObject) = objects(result, "sorts").map { Sort(it.optString("id"), it.optString("label")) }

    /** provider.home shelves. */
    fun shelves(result: JSONObject) = objects(result, "shelves").map { shelf ->
        Shelf(
            shelf.optString("title"),
            objects(shelf, "items").map {
                val kind = when (it.optString("kind")) {
                    "song" -> Kind.SONG
                    "artist" -> Kind.ARTIST
                    "playlist" -> Kind.PLAYLIST
                    else -> Kind.ALBUM
                }
                Item(kind, it.optString("id"), it.optString("title"), it.optString("subtitle"), it.optString("image_url"), it.optJSONObject("track")?.let(Track::parse))
            },
        )
    }

    /** provider.preview; blank lines and an artist's leading "♫ " dropped. */
    fun preview(result: JSONObject): Preview {
        val o = result.optJSONObject("preview") ?: JSONObject()
        val lines = o.optJSONArray("lines")?.let { a -> List(a.length()) { a.optString(it).removePrefix("♫ ") } }.orEmpty().filter(String::isNotBlank)
        return Preview(o.optString("image_url"), lines, if (o.has("favorite")) o.optBoolean("favorite") else null)
    }

    /** The history tracks, newest first, each song once, at most 20. */
    fun history(result: JSONObject) = objects(result, "history").mapNotNull { it.optJSONObject("track")?.let(Track::parse) }.distinctBy { it.path }.take(20)
}

/** Row names as shown (§5.6): the core's marks and tags stay out of the label. */
object RowName {
    private val MARKS = Regex("^((♥|●|★) )+")

    /** name without its ♥, ● and ★ marks and its ` · local` tag; a single also drops its ` · Singles & EPs` group. */
    fun of(name: String, single: Boolean = false): String {
        val n = name.replace(MARKS, "").removeSuffix(" · local")
        return if (single && " · " in n) n.substringBeforeLast(" · ") else n
    }
}

/** The last 10 search queries, newest first, each once (case aside). */
fun recents(list: List<String>, query: String): List<String> {
    val q = query.trim()
    if (q.isEmpty()) return list
    return (listOf(q) + list.filterNot { it.equals(q, ignoreCase = true) }).take(10)
}

/** The playing stream's format, `OPUS · 160 kbps · 48 kHz`, each part only when the core knows it; empty when none. */
fun audioFormat(codec: String, kbps: Int, rate: Int): String = listOfNotNull(
    codec.uppercase().ifEmpty { null },
    kbps.takeIf { it > 0 }?.let { "$it kbps" },
    rate.takeIf { it > 0 }?.let { (if (it % 1000 == 0) "${it / 1000}" else "%.1f".format(java.util.Locale.ROOT, it / 1000.0)) + " kHz" },
).joinToString(" · ")

/** mm:ss, or h:mm:ss. */
fun clock(secs: Int): String {
    val s = secs.coerceAtLeast(0)
    return if (s >= 3600) "%d:%02d:%02d".format(s / 3600, s / 60 % 60, s % 60) else "%d:%02d".format(s / 60, s % 60)
}

/** How the app words core errors (§2.10, §5.7). */
object ErrorMap {
    private val NETWORK = listOf(
        "no such host", "timeout", "timed out", "network is unreachable", "connection refused",
        "connection reset", "i/o timeout", "temporary failure in name resolution",
    )

    fun network(detail: String) = detail.lowercase().let { d -> NETWORK.any { it in d } }

    fun signIn(detail: String) = detail.lowercase().let { "sign in" in it || "sign-in" in it }

    enum class Action { UPDATE_YTDLP, SIGN_IN, RETRY, NEXT }

    /** The text (null: the error itself) and the action of a play error card, by error_kind. */
    fun card(kind: String): Pair<Int?, Action> = when (kind) {
        "ytdlp" -> R.string.err_ytdlp to Action.UPDATE_YTDLP
        "session" -> R.string.err_session to Action.SIGN_IN
        "network" -> R.string.err_network to Action.RETRY
        "unavailable" -> R.string.err_unavailable to Action.NEXT
        else -> null to Action.RETRY
    }

    /** The status-line text of a failed call, or null to use detail as is. */
    @StringRes
    fun text(code: String, detail: String): Int? = when {
        network(detail) -> R.string.update_offline
        signIn(detail) -> R.string.err_session
        code == "unavailable" -> R.string.reconnecting
        else -> null
    }
}

/** One lyrics line; start is 0 on every line of plain (unsynced) lyrics. */
@Immutable
data class Lyric(val start: Double, val text: String)

object Lyrics {
    fun parse(result: JSONObject): List<Lyric> {
        val a = result.optJSONArray("lyrics") ?: return emptyList()
        return List(a.length()) { a.getJSONObject(it).let { o -> Lyric(o.optDouble("start", 0.0), o.optString("text")) } }
    }

    /** Whether the lines carry timestamps, as the core's lyrics.Synced decides. */
    fun synced(lines: List<Lyric>) = lines.any { it.start > 0 }

    /** The line that sings at pos: the last one started, or -1 before the first. */
    fun active(lines: List<Lyric>, pos: Double): Int = lines.indexOfLast { it.start <= pos }
}

/**
 * A page of a whole list (§5.5): the rows from offset, the full length, and
 * the playlist_revision they were read at, which edits of these rows send as
 * if_revision (§5.3 Queue).
 */
@Immutable
data class Page(val rows: List<Track>, val total: Int, val revision: Long = 0) {
    companion object {
        /** From a finished queue.list or playnext.list job. */
        fun parse(job: JSONObject): Page {
            val result = job.optJSONObject("result") ?: JSONObject()
            return Page(Track.list(result), result.optInt("total"), playlistRevision(job))
        }
    }
}

/** The playlist_revision of the snapshot a job committed; 0, which the core does not check, when it has none. */
fun playlistRevision(job: JSONObject): Long = job.optJSONObject("snapshot")?.optLong("playlist_revision") ?: 0

object Paging {
    /** The offset of the next page of a whole list after loaded rows, or null at its end. */
    fun next(loaded: Int, total: Int): Int? = loaded.takeIf { it < total }

    /** The next offset of a foryou list (§5.5): offset + limit, whatever came back; an empty page ends it. */
    fun foryou(offset: Int, limit: Int, returned: Int): Int? = if (returned == 0) null else offset + limit
}

/**
 * A play request of the song at path still loads (§2.2): it is the song
 * on screen and neither plays nor failed. The yt-dlp extraction takes
 * seconds, and the core reports no buffering meanwhile.
 */
fun loads(path: String, s: Snapshot): Boolean =
    s.track?.path == path && !(s.playing && !s.buffering) && s.playError == null

/**
 * A move from from to to as the adjacent swaps the core takes (§7.7), each
 * as the pair (index, to).
 */
fun swaps(from: Int, to: Int): List<Pair<Int, Int>> =
    if (from <= to) (from until to).map { it to it + 1 } else (from downTo to + 1).map { it to it - 1 }

/**
 * Sends a move as adjacent swaps through send, which returns the finished
 * job (§7.7). With a revision, the first swap passes it as if_revision and
 * each later one the revision of the previous job's snapshot; null sends
 * none. Throws on the first failure.
 */
suspend fun sendSwaps(from: Int, to: Int, revision: Long?, send: suspend (JSONObject) -> JSONObject) {
    var rev = revision
    for ((i, j) in swaps(from, to)) {
        val params = JSONObject().put("index", i).put("to", j)
        if (rev != null) params.put("if_revision", rev)
        val job = send(params)
        if (rev != null) rev = playlistRevision(job)
    }
}

/**
 * The seek bar after a release (§5.4) shows the target until a snapshot
 * reports a position within 2 s of it, 5 s pass, or another song plays.
 */
fun seekHolds(target: Double, path: String?, releasedAt: Long, s: Snapshot, now: Long, holdMs: Long = 5000): Boolean =
    s.track?.path == path && now - releasedAt < holdMs &&
        (s.receivedAt < releasedAt || kotlin.math.abs(s.position - target) >= 2)

/** The EQ gain of a tap at y in a column height tall: +12 dB at the top, -12 at the bottom, in 1 dB steps. */
fun eqGain(y: Float, height: Float): Int =
    if (height <= 0) 0 else Math.round(12 - y / height * 24).coerceIn(-12, 12)

/** The EQ gain after a drag of dy pixels (down is positive) from base, one step per cell pixels. */
fun eqDrag(base: Int, dy: Float, cell: Float): Int =
    if (cell <= 0) base else (base - Math.round(dy / cell)).coerceIn(-12, 12)

/**
 * The next EQ band to send, as (band, dB): one the finger set (local) that
 * the core (its eq_bands) does not have and this round did not send, or null.
 */
fun eqPending(local: Map<Int, Int>, core: List<Double>, sent: Map<Int, Int>): Pair<Int, Int>? =
    local.entries.firstOrNull { (band, db) -> sent[band] != db && core.getOrNull(band)?.roundToInt() != db }?.toPair()

/** The offline download queue of download.status (G15). */
@Immutable
data class Downloads(
    val running: Boolean,
    val current: Track?,
    val percent: Int,
    val queued: List<Track>,
    val done: Int,
    val failed: List<Pair<Track, String>>,
    val directory: String,
) {
    companion object {
        fun parse(result: JSONObject): Downloads {
            val o = result.optJSONObject("downloads") ?: JSONObject()
            val failed = o.optJSONArray("failed")
            return Downloads(
                o.optBoolean("running"), o.optJSONObject("current")?.let(Track::parse), o.optDouble("percent", 0.0).toInt(),
                Track.list(JSONObject().put("tracks", o.optJSONArray("queued"))), o.optInt("done"),
                List(failed?.length() ?: 0) { i -> failed!!.getJSONObject(i).let { Track.parse(it.optJSONObject("track") ?: JSONObject()) to it.optString("error") } },
                o.optString("directory"),
            )
        }
    }
}

/** One ranked artist, song or genre of the statistics; detail is a song's artist. */
@Immutable
data class StatCount(val name: String, val detail: String, val plays: Int)

/** The listening summary of a period (G16); daily holds the minutes of the last 30 days, oldest first. */
@Immutable
data class Stats(
    val plays: Int, val secs: Double, val songs: Int, val artists: Int,
    val topArtists: List<StatCount>, val topSongs: List<StatCount>, val topGenres: List<StatCount>,
    val daily: List<Double>, val streak: Int,
) {
    companion object {
        fun parse(result: JSONObject): Stats {
            val o = result.optJSONObject("stats") ?: JSONObject()
            fun counts(name: String) = o.optJSONArray(name).let { a ->
                List(a?.length() ?: 0) { i -> a!!.getJSONObject(i).let { StatCount(it.optString("name"), it.optString("detail"), it.optInt("plays")) } }
            }
            val daily = o.optJSONArray("daily")
            return Stats(
                o.optInt("plays"), o.optDouble("secs", 0.0), o.optInt("songs"), o.optInt("artists"),
                counts("top_artists"), counts("top_songs"), counts("top_genres"),
                List(daily?.length() ?: 0) { daily!!.optDouble(it, 0.0) }, o.optInt("streak"),
            )
        }

        /**
         * The daily minutes as rows of block glyphs, top row first, rows tall:
         * each column fills to its share of the busiest day in eighths.
         */
        fun chart(daily: List<Double>, rows: Int = 4): List<String> {
            val max = daily.maxOrNull()?.takeIf { it > 0 } ?: return List(rows) { " ".repeat(daily.size) }
            val levels = daily.map { Math.round(it / max * rows * 8).toInt() }
            return (rows - 1 downTo 0).map { r -> levels.joinToString("") { " ▁▂▃▄▅▆▇█"[(it - r * 8).coerceIn(0, 8)].toString() } }
        }
    }
}

/** A "Me interesa" (interest 1) or "No me interesa" (-1) rating of taste.list (G11). id is a song's path, else the item's ID. */
@Immutable
data class Rating(val id: String, val kind: String, val name: String, val artist: String, val interest: Int) {
    companion object {
        fun list(result: JSONObject): List<Rating> {
            val a = result.optJSONArray("ratings") ?: return emptyList()
            return List(a.length()) { i ->
                a.getJSONObject(i).let { Rating(it.optString("id"), it.optString("kind"), it.optString("name"), it.optString("artist"), it.optInt("interest")) }
            }
        }
    }
}

/** Biblioteca's rows (§3.5) from the provider lists. */
object Library {
    /** The rows of YouTube Music's ♥, Historial and Música local, which Biblioteca pins. */
    val PINNED = listOf("fav", "hist", "local:all")

    /** The local provider's lists that are not playlist files: they take no songs and cannot be renamed. */
    val VIRTUAL = setOf("Favorites", "Recently Played")

    /** The account's own lists: the rows of the first section (§5.3 Biblioteca), without the pinned ones. */
    fun own(rows: List<Item>): List<Item> {
        val section = rows.firstOrNull()?.section ?: return emptyList()
        return rows.filter { it.section == section && it.id !in PINNED }
    }

    /** A local playlist file: one that can be renamed, deleted and edited. */
    fun editable(item: Item) = item.provider == "local" && item.kind == Kind.PLAYLIST && item.id !in VIRTUAL

    /** A grid's columns in width dp, as GridCells.Adaptive(160.dp) with 16 dp sides and 12 dp gaps; never fewer than two. */
    fun columns(width: Float) = maxOf(2, ((width - 32 + 12) / (160 + 12)).toInt())

    /**
     * Why the local provider would refuse name for a new list, or null: a
     * name taken (a list in taken, or a virtual list) or a character a file
     * name cannot hold (local/provider.go validateNewName). The core answers
     * both with a bare "operation failed".
     */
    @StringRes
    fun nameError(name: String, taken: Collection<String>): Int? = when {
        name in taken || name in VIRTUAL -> R.string.list_exists
        name == "." || name == ".." || name.any { it in "/\\:<>\"|?*" } -> R.string.list_bad_name
        else -> null
    }
}

/** 1x, 0.75x, 1.25x: a speed as the sheet and Ajustes show it. */
fun speedLabel(v: Double) = (if (v % 1 == 0.0) v.toInt().toString() else v.toString()) + "x"

/** The settings.set value of a snapshot normalize name: 0 off, 1 by song, 2 by album; -1 when the player has none. */
fun normalizeValue(name: String) = listOf("off", "track", "album").indexOf(name)
