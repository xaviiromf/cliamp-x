package io.github.xaviiromf.cliampx.ui

import androidx.annotation.DrawableRes
import androidx.annotation.StringRes
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Track
import kotlinx.coroutines.flow.MutableSharedFlow

enum class Tab(@StringRes val label: Int, @DrawableRes val icon: Int, val tag: String) {
    Home(R.string.tab_home, R.drawable.ic_home, "tab_home"),
    Search(R.string.tab_search, R.drawable.ic_search, "tab_search"),
    Library(R.string.tab_library, R.drawable.ic_library, "tab_library"),
}

/** The pages of the player sheet (§2.3, §2.7, §2.8). */
enum class PlayerPage { Player, Queue, Eq }

/** The small bottom sheets (§2.6, §3.15). */
sealed interface Sheet {
    /**
     * A song's actions. index is its live-playlist index in the queue's
     * Siguiente rows, read at playlist revision; next its play-next position.
     */
    data class Song(val track: Track, val index: Int? = null, val next: Int? = null, val revision: Long = 0) : Sheet
    data object PlayerMore : Sheet
    data object Speed : Sheet
    data object Sleep : Sheet
    data object ClearQueue : Sheet
}

/**
 * The tabs (UI-SPEC §7.3). It lives in the process, so a recreated activity
 * keeps its tab. shortcut: no per-tab screen stacks yet; M3 adds them with
 * the detail pages.
 */
object Nav {
    var tab by mutableStateOf(Tab.Home)
        private set

    /** The player sheet is open, on page. */
    var player by mutableStateOf(false)
    var page by mutableStateOf(PlayerPage.Player)

    /** The bottom sheet on screen, if any. */
    var sheet by mutableStateOf<Sheet?>(null)

    fun openPlayer(at: PlayerPage = PlayerPage.Player) {
        page = at
        player = true
    }

    /** A tap on the tab that is already open: its screen scrolls to the top. */
    val reselected = MutableSharedFlow<Tab>(extraBufferCapacity = 1)

    fun select(t: Tab) {
        if (t == tab) reselected.tryEmit(t) else tab = t
    }

    /** System back (§2.9): Buscar and Biblioteca go to Inicio; false at Inicio. */
    fun back(): Boolean {
        if (tab == Tab.Home) return false
        tab = Tab.Home
        return true
    }
}
