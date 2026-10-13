package io.github.xaviiromf.cliampx.ui

import android.content.Intent
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.SettingsActivity

/** Inicio: the logo, Ajustes and the banner slot. shortcut: the carousels come in M3. */
@Composable
fun HomeScreen() {
    val p = LocalPalette.current
    val context = LocalContext.current
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
        Row(Modifier.fillMaxWidth().height(56.dp).padding(start = 16.dp, end = 4.dp), verticalAlignment = Alignment.CenterVertically) {
            Text("C L I A M P - X", color = p.accent, style = Type.logo)
            Spacer(Modifier.weight(1f))
            IconBtn(R.drawable.ic_settings, stringResource(R.string.settings), Modifier.testTag("settings")) {
                context.startActivity(Intent(context, SettingsActivity::class.java))
            }
        }
        Banners()
        EmptyState(stringResource(R.string.home_empty), stringResource(R.string.tab_search)) { Nav.select(Tab.Search) }
    }
}
