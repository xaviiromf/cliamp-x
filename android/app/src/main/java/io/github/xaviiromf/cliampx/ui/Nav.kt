package io.github.xaviiromf.cliampx.ui

import androidx.annotation.DrawableRes
import androidx.annotation.StringRes
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshots.SnapshotStateList
import io.github.xaviiromf.cliampx.Item
import io.github.xaviiromf.cliampx.Kind
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Track
import kotlinx.coroutines.flow.MutableSharedFlow

enum class Tab(@StringRes val label: Int, @DrawableRes val icon: Int, val tag: String) {
    Home(R.string.tab_home, R.drawable.ic_home, "tab_home"),
    Search(R.string.tab_search, R.drawable.ic_search, "tab_search"),
    Library(R.string.tab_library, R.drawable.ic_library, "tab_library"),
}

/** The screens of a tab's stack (§2, §7.3). */
@Immutable
sealed interface Screen {
    data object Home : Screen
    data object Search : Screen
    data object Library : Screen

    /** An album: `MPREb_…`, a release `al:MPREb_…`, or `local:al:…`. */
    data class Album(val id: String, val title: String, val art: String = "", val subtitle: String = "") : Screen

    /** An artist: `UC…` or `local:ar:…`. */
    data class Artist(val id: String, val name: String, val art: String = "") : Screen
    data class Playlist(val provider: String, val id: String, val title: String, val art: String = "") : Screen
    data class Genre(val id: String, val title: String) : Screen

    /** The endless "For you" grid of albums, or of artists. */
    data class ParaTi(val artists: Boolean) : Screen
    data class Remixes(val track: Track) : Screen

    /** Ajustes; install checks for an app update and installs it (the Inicio banner's [ Instalar ]). */
    data class Settings(val install: Boolean = false) : Screen
    data object Themes : Screen
    data object ThemeEditor : Screen
    data object Skins : Screen
    data object Fonts : Screen
    data object Visualizers : Screen
    data object Backgrounds : Screen

    /** The equalizer, pushed from Ajustes. */
    data object Eq : Screen
    data object Stats : Screen

    /** Tu año en cliamp-x, pushed from Estadísticas. */
    data object Wrapped : Screen
    data object Ratings : Screen

    /** Acerca de, pushed from Ajustes, and a license text it opens (file in assets/licenses). */
    data object About : Screen
    data class License(val title: String, val file: String) : Screen
}

/** The page an item opens, or null for a song (which plays). */
fun Item.screen(): Screen? = when (kind) {
    Kind.ALBUM -> Screen.Album(id, title, art, subtitle)
    Kind.ARTIST -> Screen.Artist(id, title, art)
    Kind.PLAYLIST -> Screen.Playlist(provider, id, title, art)
    Kind.GENRE -> Screen.Genre(id, title)
    Kind.SONG -> null
}

/** A screen on a stack. key is unique in the process, so the same page twice keeps two saved states. */
@Immutable
data class Entry(val screen: Screen, val key: String)

/** The pages of the player sheet (§2.3, §2.7, §2.8), and the full-screen lyrics. */
enum class PlayerPage { Player, Queue, Eq, Lyrics }

/** The small bottom sheets (§2.6, §3.12, §3.15). */
sealed interface Sheet {
    /**
     * A song's actions. index is its live-playlist index in the queue's
     * Siguiente rows, read at playlist revision; next its play-next
     * position; list the local list it is row of.
     */
    data class Song(
        val track: Track, val index: Int? = null, val next: Int? = null, val revision: Long = 0,
        val list: String? = null, val row: Int = 0,
    ) : Sheet

    /** The actions of an album, artist, playlist or genre. */
    data class Of(val item: Item) : Sheet

    /** Añadir a una lista… for tracks, or for the songs of the album or playlist item. */
    data class AddTo(val tracks: List<Track>, val item: Item? = null) : Sheet

    /** Nueva lista: a name, then the tracks go in, or with queue the whole queue. */
    data class NewList(val tracks: List<Track>, val item: Item? = null, val queue: Boolean = false) : Sheet

    /** Renombrar… a local list. */
    data class Rename(val item: Item) : Sheet

    /** Eliminar a local list, after a confirmation. */
    data class DeleteList(val item: Item) : Sheet
    /** A scrobbling credential for settings.set name (Ajustes → Scrobbling). */
    data class Credential(val name: String, @StringRes val title: Int) : Sheet
    data object PlayerMore : Sheet
    data object QueueMore : Sheet
    data object Speed : Sheet
    data object Sleep : Sheet
    data object Normalize : Sheet
    data object Crossfade : Sheet
    data object ClearQueue : Sheet
    /** Cerrar sesión y borrar datos, after a confirmation. */
    data object EraseData : Sheet

    /** Importar copia, after Backup.check accepted the file: the confirmation to replace the data. */
    data object RestoreBackup : Sheet

    /** Guardar el actual como skin… (old null), or Renombrar… the user skin old. */
    data class SkinName(val old: String?) : Sheet
    data class SkinActions(val name: String) : Sheet

    /** The theme editor's Guardar como…: its seven colors, and the name it offers. */
    data class ThemeName(val colors: List<String>, val initial: String) : Sheet
}

/**
 * The tabs and their screen stacks (UI-SPEC §7.3). It lives in the process,
 * so a recreated activity keeps every stack.
 */
object Nav {
    var tab by mutableStateOf(Tab.Home)
        private set

    private var serial = 0
    private val stacks = Tab.entries.associateWith { tab -> mutableStateListOf(Entry(root(tab), tab.name)) }

    /** The keys of popped screens: the host drops their saved state. */
    val dropped = mutableStateListOf<String>()

    /** The player sheet is open, on page. */
    var player by mutableStateOf(false)
    var page by mutableStateOf(PlayerPage.Player)

    /** The bottom sheet on screen, if any. */
    var sheet by mutableStateOf<Sheet?>(null)

    /** A tap on the tab that is already open at its root: its screen scrolls to the top. */
    val reselected = MutableSharedFlow<Tab>(extraBufferCapacity = 1)

    private fun root(tab: Tab) = when (tab) {
        Tab.Home -> Screen.Home
        Tab.Search -> Screen.Search
        Tab.Library -> Screen.Library
    }

    private val stack: SnapshotStateList<Entry> get() = stacks.getValue(tab)

    /** The screen on show. */
    val top: Entry get() = stack.last()

    fun openPlayer(at: PlayerPage = PlayerPage.Player) {
        page = at
        player = true
    }

    /** Pushes screen on the open tab, over the sheets (an action sheet or the player opened it). */
    fun open(screen: Screen) {
        sheet = null
        player = false
        page = PlayerPage.Player
        stack += Entry(screen, "s${serial++}")
    }

    /** Opens Ajustes on Inicio's stack, from anywhere (§2.5). */
    fun settings(install: Boolean = false) {
        tab = Tab.Home
        if ((top.screen as? Screen.Settings)?.install == install) return
        open(Screen.Settings(install))
    }

    /** Replaces the open tab's top screen, as a renamed list's page. */
    fun replace(screen: Screen) {
        pop()
        open(screen)
    }

    /** Pops the open tab's top screen. */
    fun pop() {
        if (stack.size > 1) dropped += stack.removeAt(stack.lastIndex).key
    }

    /** A tap on a tab: another tab opens; the open one pops to its root, or scrolls to the top there (§2.1). */
    fun select(t: Tab) {
        when {
            t != tab -> tab = t
            stack.size > 1 -> while (stack.size > 1) pop()
            else -> reselected.tryEmit(t)
        }
    }

    /** Whether system back has a screen to leave in the shell (§2.9); false at Inicio's root. */
    val canBack get() = stack.size > 1 || tab != Tab.Home

    /** System back (§2.9): pop a screen; Buscar and Biblioteca at their root go to Inicio. */
    fun back() {
        if (stack.size > 1) pop() else tab = Tab.Home
    }
}
