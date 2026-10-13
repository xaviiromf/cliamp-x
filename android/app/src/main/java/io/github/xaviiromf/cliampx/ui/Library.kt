package io.github.xaviiromf.cliampx.ui

import android.content.Intent
import androidx.compose.foundation.layout.Column
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.TerminalActivity

/** Biblioteca. shortcut: only a way to Modo terminal until M4 brings the library. */
@Composable
fun LibraryScreen() {
    val context = LocalContext.current
    Column {
        ScreenRule(stringResource(R.string.tab_library))
        EmptyState(stringResource(R.string.library_empty), stringResource(R.string.terminal)) {
            context.startActivity(Intent(context, TerminalActivity::class.java))
        }
    }
}
