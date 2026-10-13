package io.github.xaviiromf.cliampx.ui

import android.content.Context
import android.content.Intent
import android.os.PowerManager
import android.widget.Toast
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.core.content.edit
import androidx.lifecycle.compose.LifecycleResumeEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.Item
import io.github.xaviiromf.cliampx.Kind
import io.github.xaviiromf.cliampx.LoginActivity
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.SettingsActivity

/**
 * Inicio (§2.1, §3.2): the logo and Ajustes, the banner slot, then the
 * carousels, each shown once it has items. Each loads when the list
 * composes it. A pull asks the provider for new lists.
 */
@Composable
fun HomeScreen() {
    val p = LocalPalette.current
    val context = LocalContext.current
    val list = rememberLazyListState()
    LaunchedEffect(Unit) { Nav.reselected.collect { if (it == Tab.Home) list.animateScrollToItem(0) } }
    Pull(::refreshAll) {
        LazyColumn(Modifier.fillMaxSize(), state = list) {
            item(key = "top") {
                Row(Modifier.fillMaxWidth().height(56.dp).padding(start = 16.dp, end = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text("C L I A M P - X", color = p.accent, style = Type.logo)
                    Spacer(Modifier.weight(1f))
                    IconBtn(R.drawable.ic_settings, stringResource(R.string.settings), Modifier.testTag("settings")) {
                        context.startActivity(Intent(context, SettingsActivity::class.java))
                    }
                }
            }
            item(key = "banners") {
                Column {
                    Banners()
                    SignInCard()
                    BatteryCard()
                }
            }
            item(key = "history") { HistoryShelf() }
            item(key = "home") { HomeShelves() }
            item(key = "foryou_albums") { ForYouShelf(artists = false) }
            item(key = "foryou_artists") { ForYouShelf(artists = true) }
            item(key = "library") { LibraryShelves() }
        }
    }
}

/** Escuchado recientemente: a tap plays the history from that song. */
@Composable
private fun HistoryShelf() {
    val (load, _) = rememberLoad("history") { Remote.history() }
    val tracks = (load as? Load.Done)?.value.orEmpty()
    if (tracks.isEmpty()) return
    val items = tracks.map { Item(Kind.SONG, it.path, it.name, it.artist, it.art, it) }
    Carousel(stringResource(R.string.home_recent), items, "history") { i, _ -> Remote.play(tracks, i) }
}

/** The provider's home shelves (G2); the one place Inicio shows a load error. */
@Composable
private fun HomeShelves() {
    val (load, retry) = rememberLoad("home") { Remote.home() }
    val hidden by Remote.hidden.collectAsStateWithLifecycle()
    LoadState(load, retry)
    val shelves = (load as? Load.Done)?.value ?: return
    Column {
        shelves.forEachIndexed { k, shelf ->
            val items = shelf.items.filter { (it.track?.path ?: it.id) !in hidden }
            if (items.isNotEmpty()) Carousel(shelf.title, items, "shelf$k") { _, item -> openItem(item, items) }
        }
    }
}

/** Para ti: álbumes or artistas, the first 20 of the endless list, with [Ver todo]. */
@Composable
private fun ForYouShelf(artists: Boolean) {
    val (load, _) = rememberLoad("foryou:$artists:0:20") { Remote.forYou(artists, 0, 20) }
    val hidden by Remote.hidden.collectAsStateWithLifecycle()
    val items = (load as? Load.Done)?.value.orEmpty().filter { it.id !in hidden }
    if (items.isEmpty()) return
    Carousel(
        stringResource(if (artists) R.string.foryou_artists else R.string.foryou_albums), items, if (artists) "foryou_artists" else "foryou_albums",
        action = { BracketButton(stringResource(R.string.see_all), Modifier.testTag(if (artists) "see_all_artists" else "see_all_albums")) { Nav.open(Screen.ParaTi(artists)) } },
    ) { _, item -> openItem(item, items) }
}

/** Novedades de tus artistas (al: rows, ● unseen) and Tus géneros (genre: rows) from the account's lists. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun LibraryShelves() {
    val (load, _) = rememberLoad("playlists:ytmusic") { Remote.playlists("ytmusic") }
    val rows = (load as? Load.Done)?.value.orEmpty()
    val releases = rows.filter { it.kind == Kind.ALBUM }
    val genres = rows.filter { it.kind == Kind.GENRE }
    Column {
        if (releases.isNotEmpty()) Carousel(stringResource(R.string.home_new), releases, "new") { _, item -> openItem(item, releases) }
        if (genres.isNotEmpty()) {
            SectionRule(stringResource(R.string.home_genres))
            FlowRow(Modifier.padding(horizontal = 12.dp)) {
                genres.forEachIndexed { i, g ->
                    BracketChip(g.title, false, Modifier.testTag("genre_chip_$i"), icon = R.drawable.ic_star) { Nav.open(Screen.Genre(g.id, g.title)) }
                }
            }
        }
    }
}

/** Signed out: the public home, and a card that offers the sign-in (§2.10). Dismissed for good. */
@Composable
private fun SignInCard() {
    val p = LocalPalette.current
    val context = LocalContext.current
    val signedIn by Remote.signedIn.collectAsStateWithLifecycle()
    val prefs = remember { context.getSharedPreferences("ui", Context.MODE_PRIVATE) }
    var dismissed by remember { mutableStateOf(prefs.getBoolean(SIGN_IN_DISMISSED, false)) }
    val import = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) Toast.makeText(context, LoginActivity.importCookies(context, uri), Toast.LENGTH_LONG).show()
    }
    if (signedIn != false || dismissed) return
    Card("banner_signin", onClose = {
        dismissed = true
        prefs.edit { putBoolean(SIGN_IN_DISMISSED, true) }
    }) {
        Text(stringResource(R.string.sign_in_card), color = p.textPrimary, style = Type.caption)
        FlowRow {
            BracketButton(stringResource(R.string.sign_in_short), Modifier.testTag("signin_login")) {
                context.startActivity(Intent(context, LoginActivity::class.java))
            }
            BracketButton(stringResource(R.string.import_cookies)) { import.launch(arrayOf("text/*", "application/octet-stream")) }
        }
    }
}

/** After the first song, while One UI may still suspend the app: the way to the battery setting (§2.10). */
@Composable
private fun BatteryCard() {
    val p = LocalPalette.current
    val context = LocalContext.current
    val prefs = remember { context.getSharedPreferences("ui", Context.MODE_PRIVATE) }
    var optimized by remember { mutableStateOf(false) }
    // Back from the system setting, the card goes once the optimization is off.
    LifecycleResumeEffect(Unit) {
        optimized = !context.getSystemService(PowerManager::class.java).isIgnoringBatteryOptimizations(context.packageName)
        onPauseOrDispose {}
    }
    if (!optimized || !prefs.getBoolean(PLAYED, false)) return
    Card("banner_battery") {
        Text(stringResource(R.string.battery_card), color = p.textPrimary, style = Type.caption)
        BracketButton(stringResource(R.string.battery_short)) { SettingsActivity.ignoreBatteryOptimizations(context) }
    }
}

/** A card of the banner slot: text and buttons stacked, with an optional ✕. */
@Composable
private fun Card(tag: String, onClose: (() -> Unit)? = null, content: @Composable () -> Unit) {
    Banner(tag) {
        Column(Modifier.weight(1f).padding(vertical = 8.dp)) { content() }
        if (onClose != null) IconBtn(R.drawable.ic_close, stringResource(R.string.close), Modifier.testTag("${tag}_close"), onClick = onClose)
    }
}

private const val SIGN_IN_DISMISSED = "sign_in_dismissed"

/** Set when a song first plays: the battery card waits for it (§2.10). */
const val PLAYED = "played"
