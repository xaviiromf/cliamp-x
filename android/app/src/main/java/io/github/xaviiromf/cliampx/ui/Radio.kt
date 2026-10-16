package io.github.xaviiromf.cliampx.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Station
import io.github.xaviiromf.cliampx.Stations
import io.github.xaviiromf.cliampx.Track
import io.github.xaviiromf.cliampx.clock
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch

/**
 * Radio y podcasts: a URL field with Reproducir and Guardar, then the saved
 * stations and feeds. A stream or a playlist URL plays; a podcast feed opens
 * its episodes (Stations.isFeed).
 */
@Composable
fun RadioScreen() {
    val p = LocalPalette.current
    val scope = rememberCoroutineScope()
    val res = LocalResources.current
    var text by rememberSaveable { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    val url = Stations.valid(text)
    val invalid = text.isNotBlank() && url == null

    /** Resolves u, then plays it or opens its episodes; save keeps it in the list too. */
    fun go(u: String, save: Boolean) {
        busy = true
        error = null
        scope.launch {
            try {
                val tracks = Remote.urlTracks(u)
                val name = Stations.nameOf(u, tracks)
                when {
                    tracks.isEmpty() -> error = res.getString(R.string.no_results)
                    save -> Stations.put(Station(Stations.list.firstOrNull { it.url == u }?.name ?: name, u))
                    Stations.isFeed(tracks) -> Nav.open(Screen.Feed(u, name))
                    else -> Remote.play(tracks, 0)
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                // A saved URL that cannot be reached now (offline) is still kept under its host.
                if (save) Stations.put(Station(Stations.defaultName(u), u)) else error = Remote.message(e)
            } finally {
                busy = false
            }
        }
    }

    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.lib_radio))
        LazyColumn(Modifier.fillMaxSize()) {
            item(key = "field") {
                BasicTextField(
                    text, { text = it; error = null },
                    Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp).testTag("radio_url"),
                    singleLine = true,
                    textStyle = Type.body.copy(color = p.textPrimary),
                    cursorBrush = SolidColor(p.accent),
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go),
                    keyboardActions = KeyboardActions(onGo = { url?.let { go(it, false) } }),
                    decorationBox = { field ->
                        Row(Modifier.heightIn(min = 48.dp).background(p.surfaceHigh).padding(horizontal = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                            Text("> ", color = p.accent, style = Type.body)
                            Box(Modifier.weight(1f)) {
                                if (text.isEmpty()) Text(stringResource(R.string.radio_hint), color = p.textSecondary, style = Type.body, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                field()
                            }
                        }
                    },
                )
            }
            item(key = "buttons") {
                Row(Modifier.padding(horizontal = 12.dp), horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                    BracketButton(stringResource(R.string.play), Modifier.testTag("radio_play"), color = if (url != null && !busy) p.accent else p.textSecondary) { if (!busy) url?.let { go(it, false) } }
                    BracketButton(stringResource(R.string.store), Modifier.testTag("radio_save"), color = if (url != null && !busy) p.accent else p.textSecondary) { if (!busy) url?.let { go(it, true) } }
                    if (busy) SpinnerLine(stringResource(R.string.loading))
                }
            }
            item(key = "error") {
                val message = if (invalid) stringResource(R.string.radio_invalid) else error
                if (message != null) Text("ERR: $message", Modifier.padding(horizontal = 16.dp, vertical = 4.dp).testTag("radio_error"), color = p.error, style = Type.body)
            }
            item(key = "saved") { SectionRule(stringResource(R.string.radio_saved)) }
            if (Stations.list.isEmpty()) item(key = "none") { EmptyState(stringResource(R.string.radio_none)) }
            itemsIndexed(Stations.list.toList(), key = { _, s -> s.url }) { i, s ->
                Row(
                    Modifier.fillMaxWidth().heightIn(min = Look.row(64.dp)).clickable { if (!busy) go(s.url, false) }.padding(start = 16.dp).testTag("station_$i"),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(Modifier.weight(1f)) {
                        Text(s.name, color = p.textPrimary, style = Type.body, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        Text(s.url, color = p.textSecondary, style = Type.label, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                    IconBtn(R.drawable.ic_more, stringResource(R.string.more), Modifier.testTag("station_more_$i")) { Nav.sheet = Sheet.StationActions(s.url) }
                }
            }
            item(key = "end") { Spacer(Modifier.height(16.dp)) }
        }
    }
}

/** A podcast feed's episodes, newest first: title, then date · length. A tap plays from that episode on. */
@Composable
fun FeedScreen(s: Screen.Feed) {
    val p = LocalPalette.current
    val snap by Remote.state.collectAsStateWithLifecycle()
    val (load, retry) = rememberLoad("url:${s.url}") { Remote.urlTracks(s.url) }
    val episodes = (load as? Load.Done)?.value?.let(Stations::newestFirst).orEmpty()
    val saved = Stations.list.any { it.url == s.url }
    Column(Modifier.fillMaxSize()) {
        TopBar()
        LazyColumn(Modifier.fillMaxSize()) {
            item(key = "head") {
                Column(Modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
                    Text(s.name, color = p.accent, style = Type.display, maxLines = 3, overflow = TextOverflow.Ellipsis)
                    if (episodes.isNotEmpty()) Text(stringResource(R.string.radio_episodes, episodes.size), color = p.textSecondary, style = Type.label)
                }
            }
            item(key = "buttons") {
                Row(Modifier.padding(horizontal = 12.dp)) {
                    if (episodes.isNotEmpty()) BracketButton(stringResource(R.string.play), Modifier.testTag("feed_play")) { Remote.play(episodes, 0) }
                    if (!saved) BracketButton(stringResource(R.string.store), Modifier.testTag("feed_save")) { Stations.put(Station(s.name, s.url)) }
                }
            }
            item(key = "state") { LoadState(load, retry) }
            itemsIndexed(episodes, key = { i, t -> "${t.meta["podcast.guid"] ?: t.path}:$i" }) { i, t -> Episode(t, t.path == snap?.track?.path) { Remote.play(episodes, i) } }
            item(key = "end") { Spacer(Modifier.height(16.dp)) }
        }
    }
}

@Composable
private fun Episode(t: Track, playing: Boolean, onClick: () -> Unit) {
    val p = LocalPalette.current
    val sub = listOfNotNull(t.meta["podcast.published"], if (t.durationSecs > 0) clock(t.durationSecs) else null).joinToString(" · ")
    Column(Modifier.fillMaxWidth().heightIn(min = Look.row(64.dp)).clickable(onClick = onClick).padding(horizontal = 16.dp, vertical = 8.dp).testTag("feed_episode")) {
        Text((if (playing) "▶ " else "") + t.name, color = if (playing) p.playing else p.textPrimary, style = if (playing) Type.bodyStrong else Type.body, maxLines = 2, overflow = TextOverflow.Ellipsis)
        if (sub.isNotEmpty()) Text(sub, color = p.textSecondary, style = Type.label)
    }
}

/** A saved station's ⋮: Renombrar… and Eliminar, after a confirmation. */
@Composable
fun StationActionsSheet(sheet: Sheet.StationActions, close: () -> Unit) {
    val p = LocalPalette.current
    var confirm by remember { mutableStateOf(false) }
    val name = Stations.list.firstOrNull { it.url == sheet.url }?.name.orEmpty()
    SectionRule(name)
    if (confirm) {
        Confirm(stringResource(R.string.radio_delete_confirm, name), stringResource(R.string.delete), close) { Stations.remove(sheet.url) }
    } else {
        MenuRow(">", stringResource(R.string.rename_more), "menu_rename") { Nav.sheet = Sheet.StationName(sheet.url) }
        MenuRow("✗", stringResource(R.string.delete), "menu_delete", p.error) { confirm = true }
    }
}

@Composable
fun StationNameSheet(sheet: Sheet.StationName, close: () -> Unit) {
    val old = Stations.list.firstOrNull { it.url == sheet.url }?.name.orEmpty()
    NameSheet(stringResource(R.string.rename), old, stringResource(R.string.store), close, taken = null) { name, _ ->
        Stations.rename(sheet.url, name)
        Nav.sheet = null
    }
}
