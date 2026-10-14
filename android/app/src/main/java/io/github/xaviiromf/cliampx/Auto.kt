package io.github.xaviiromf.cliampx

import android.content.Context
import android.os.Process
import android.support.v4.media.MediaBrowserCompat.MediaItem
import android.support.v4.media.MediaDescriptionCompat
import java.util.concurrent.ConcurrentHashMap

/**
 * The Android Auto browse tree of CliampService: Inicio (one folder per
 * shelf), Biblioteca (the saved lists), Me gusta and Recientes, plus voice
 * search. A media ID is its folder and row, home/2/5; play() finds the row
 * again from the folder, so an ID still plays after the process restarted.
 */
object Auto {
    const val ROOT = "root"
    private const val HOME = "home"
    private const val LIBRARY = "library"
    private const val LIKED = "fav"
    private const val RECENT = "hist"
    private const val SEARCH = "search"

    /** The callers that may browse: Android Auto, Android Automotive, the Assistant, the system media controls and this app. */
    private val TRUSTED = setOf(
        "com.google.android.projection.gearhead", "com.google.android.carassistant", "com.android.car.media",
        "com.google.android.googlequicksearchbox", "com.google.android.apps.automotive.templates.host",
        "com.android.systemui", "com.google.android.autosimulator",
    )

    /** Whether a caller (its package, which MediaBrowserServiceCompat checked against its uid) may read the library. */
    fun trusted(pkg: String, uid: Int, self: String) =
        uid == Process.SYSTEM_UID || uid == Process.myUid() || pkg == self || pkg in TRUSTED

    /** What each listed row does when played, by media ID. */
    private val plays = ConcurrentHashMap<String, () -> Unit>()

    private fun folder(id: String, title: String) =
        MediaItem(MediaDescriptionCompat.Builder().setMediaId(id).setTitle(title).build(), MediaItem.FLAG_BROWSABLE)

    private fun playable(id: String, title: String, subtitle: String, play: () -> Unit): MediaItem {
        plays[id] = play
        return MediaItem(MediaDescriptionCompat.Builder().setMediaId(id).setTitle(title).setSubtitle(subtitle).build(), MediaItem.FLAG_PLAYABLE)
    }

    private fun songs(parent: String, tracks: List<Track>, play: (Int) -> Unit) =
        tracks.mapIndexed { i, t -> playable("$parent/$i", t.name, t.artist, { play(i) }) }

    private fun items(parent: String, items: List<Item>) =
        items.mapIndexed { i, item -> playable("$parent/$i", item.title, item.subtitle, { Remote.playItem(item) }) }

    fun root(ctx: Context) = listOf(
        folder(HOME, ctx.getString(R.string.tab_home)), folder(LIBRARY, ctx.getString(R.string.tab_library)),
        folder(LIKED, ctx.getString(R.string.lib_liked)), folder(RECENT, ctx.getString(R.string.home_recent)),
    )

    /** The rows of a folder; throws IpcError while the core is down, which the caller sends as an empty folder. */
    suspend fun children(ctx: Context, parent: String): List<MediaItem> = when {
        parent == ROOT -> root(ctx)
        parent == HOME -> Remote.home().mapIndexed { i, shelf -> folder("$HOME/$i", shelf.title) }
        parent.startsWith("$HOME/") -> Remote.home().getOrNull(parent.substringAfter('/').toIntOrNull() ?: -1)?.let { items(parent, it.items) }.orEmpty()
        parent == LIBRARY -> items(parent, Remote.playlists("ytmusic"))
        parent == LIKED -> Remote.tracks("ytmusic", LIKED, 0).rows.let { rows -> songs(parent, rows) { Remote.load("ytmusic", LIKED, it, rows[it]) } }
        parent == RECENT -> Remote.history().let { rows -> songs(parent, rows) { Remote.play(rows, it) } }
        else -> emptyList()
    }

    /** Voice search and the search box of the car: the songs found, playable from any row. */
    suspend fun search(query: String): List<MediaItem> {
        val rows = searchSongs(query)
        return songs(SEARCH, rows) { Remote.play(rows, it) }
    }

    // The search's artists, albums and lists are tiles in the app, not songs to play.
    private suspend fun searchSongs(query: String) = Remote.search(query).flatMap { it.rows }.filter { it.kind == Kind.SONG }

    /** Plays the songs a voice search finds, from the first; false when it finds none. */
    suspend fun playSearch(query: String): Boolean {
        val rows = searchSongs(query)
        if (rows.isNotEmpty()) Remote.play(rows, 0)
        return rows.isNotEmpty()
    }

    /** Plays a media ID; an ID from before a restart loads its folder again first. Returns false for an unknown ID. */
    suspend fun play(ctx: Context, id: String): Boolean {
        if (!plays.containsKey(id)) children(ctx, parent(id))
        val play = plays[id] ?: return false
        play()
        return true
    }

    /** The folder of a media ID: home/2/5 is in home/2. */
    fun parent(id: String) = id.substringBeforeLast('/', ROOT)
}
