package io.github.xaviiromf.cliampx

import androidx.annotation.StringRes
import androidx.compose.runtime.Immutable
import org.json.JSONObject

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
    val receivedAt: Long,
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
            receivedAt = receivedAt,
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

/** A search row's kind, from its albumID (§5.3 Buscar). */
enum class Kind { SONG, ARTIST, ALBUM, PLAYLIST }

val Track.kind: Kind
    get() {
        val id = meta["albumID"] ?: return Kind.SONG
        return when {
            id.startsWith("ar:") -> Kind.ARTIST
            id.startsWith("pl:") -> Kind.PLAYLIST
            else -> Kind.ALBUM
        }
    }

@Immutable
data class Section(val title: String, val rows: List<Track>)

/** Search results grouped by provider_meta.section, in arrival order. */
fun searchSections(tracks: List<Track>): List<Section> =
    tracks.groupBy { it.meta["section"].orEmpty() }.map { (title, rows) -> Section(title, rows) }

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
        code == "unavailable" -> R.string.reconnecting
        else -> null
    }
}
