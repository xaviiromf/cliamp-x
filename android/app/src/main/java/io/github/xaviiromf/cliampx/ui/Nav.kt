package io.github.xaviiromf.cliampx.ui

import androidx.annotation.DrawableRes
import androidx.annotation.StringRes
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import io.github.xaviiromf.cliampx.R
import kotlinx.coroutines.flow.MutableSharedFlow

enum class Tab(@StringRes val label: Int, @DrawableRes val icon: Int, val tag: String) {
    Home(R.string.tab_home, R.drawable.ic_home, "tab_home"),
    Search(R.string.tab_search, R.drawable.ic_search, "tab_search"),
    Library(R.string.tab_library, R.drawable.ic_library, "tab_library"),
}

/**
 * The tabs (UI-SPEC §7.3). It lives in the process, so a recreated activity
 * keeps its tab. shortcut: no per-tab screen stacks yet; M3 adds them with
 * the detail pages.
 */
object Nav {
    var tab by mutableStateOf(Tab.Home)
        private set

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
