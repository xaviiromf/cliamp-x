package io.github.xaviiromf.cliampx.ui

import androidx.annotation.DrawableRes
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.repeatOnLifecycle
import io.github.xaviiromf.cliampx.Downloads
import io.github.xaviiromf.cliampx.IpcError
import io.github.xaviiromf.cliampx.Item
import io.github.xaviiromf.cliampx.Library
import io.github.xaviiromf.cliampx.Paging
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Sort
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import org.json.JSONObject

/** Biblioteca's chips (§2.1). */
private enum class Chip(val label: Int, val tag: String) {
    Lists(R.string.lib_lists, "lib_chip_lists"),
    Albums(R.string.albums, "lib_chip_albums"),
    Artists(R.string.lib_artists, "lib_chip_artists"),
    Downloads(R.string.downloads, "lib_chip_downloads"),
}

/**
 * Biblioteca (§2.1, §3.5): [+] for a new list, the pinned rows (♥,
 * Historial, Música local, Tus valoraciones, Estadísticas), then the
 * chips: the account's and the local lists, the albums, the artists and
 * the downloads.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun LibraryScreen() {
    val p = LocalPalette.current
    val ops by Remote.ops.collectAsStateWithLifecycle()
    var chip by rememberSaveable { mutableStateOf(Chip.Lists) }
    val list = rememberLazyListState()
    LaunchedEffect(Unit) { Nav.reselected.collect { if (it == Tab.Library) list.animateScrollToItem(0) } }
    val (yt, retryYt) = rememberLoad("playlists:ytmusic") { Remote.playlists("ytmusic") }
    val count = { id: String -> (yt as? Load.Done)?.value?.firstOrNull { it.id == id }?.count ?: 0 }
    // Álbumes' play order, "top" (your favorites first) until the user picks one.
    var sort by rememberSaveable { mutableStateOf("top") }
    val (albumPages, sorts, retryAlbums) = rememberAlbums(sort, list, chip == Chip.Albums)
    Pull({ refreshAll() }) {
        BoxWithConstraints {
            // Álbumes' columns, as Para ti's Adaptive(160.dp) grid (§3.10).
            val cols = Library.columns(maxWidth.value)
            LazyColumn(Modifier.fillMaxSize(), state = list) {
                item(key = "top") {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Box(Modifier.weight(1f)) { ScreenRule(stringResource(R.string.tab_library)) }
                        if ("playlist.create" in ops) {
                            IconBtn(R.drawable.ic_add, stringResource(R.string.new_list), Modifier.padding(top = 8.dp).testTag("lib_new")) {
                                Nav.sheet = Sheet.NewList(emptyList())
                            }
                        }
                    }
                }
                item(key = "pinned") {
                    Column {
                        Pinned("", R.drawable.ic_heart, stringResource(R.string.lib_liked), count("fav"), "lib_liked", p.favorite) {
                            Nav.open(Screen.Playlist("ytmusic", "fav", it))
                        }
                        Pinned("", R.drawable.ic_history, stringResource(R.string.lib_history), 0, "lib_history") { Nav.open(Screen.Playlist("ytmusic", "hist", it)) }
                        Pinned("", R.drawable.ic_note, stringResource(R.string.lib_local), count("local:all"), "lib_local") {
                            Nav.open(Screen.Playlist("ytmusic", "local:all", it))
                        }
                        if ("taste.list" in ops) Pinned("✓", null, stringResource(R.string.ratings), 0, "lib_ratings") { Nav.open(Screen.Ratings) }
                        if ("stats" in ops) Pinned("", R.drawable.ic_stats, stringResource(R.string.stats), 0, "lib_stats") { Nav.open(Screen.Stats) }
                    }
                }
                item(key = "chips") {
                    FlowRow(Modifier.padding(horizontal = 12.dp)) {
                        for (c in Chip.entries) {
                            if (c == Chip.Downloads && "download.status" !in ops) continue
                            BracketChip(stringResource(c.label), chip == c, Modifier.testTag(c.tag)) { chip = c }
                        }
                    }
                }
                when (chip) {
                    Chip.Lists -> lists(yt, retryYt)
                    Chip.Albums -> albums(albumPages, sorts, sort, cols, { sort = it }, retryAlbums)
                    Chip.Artists -> artists()
                    Chip.Downloads -> item(key = "downloads") { DownloadsPanel() }
                }
                item(key = "end") { Spacer(Modifier.height(16.dp)) }
            }
        }
    }
}

/** A pinned row (56 dp): a one-cell glyph, text or vector, the label and a count. A tap gets the label. */
@Composable
private fun Pinned(glyph: String, @DrawableRes icon: Int?, label: String, count: Int, tag: String, color: androidx.compose.ui.graphics.Color = LocalPalette.current.accent, open: (String) -> Unit) {
    val p = LocalPalette.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = 56.dp).clickable { open(label) }.padding(horizontal = 16.dp).testTag(tag),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.width(32.dp)) {
            if (icon != null) Icon(painterResource(icon), null, Modifier.size(20.dp), tint = color) else Text(glyph, color = color, style = Type.body)
        }
        Text(label, Modifier.weight(1f), color = p.textPrimary, style = Type.body, maxLines = 1, overflow = TextOverflow.Ellipsis)
        if (count > 0) Text(pluralStringResource(R.plurals.songs_count, count, count), color = p.textSecondary, style = Type.label)
    }
}

/** Listas: the account's own lists (signed in) or the sign-in card, then the local lists. */
private fun LazyListScope.lists(yt: Load<List<Item>>, retry: () -> Unit) {
    item(key = "yt") {
        val signedIn by Remote.signedIn.collectAsStateWithLifecycle()
        Column {
            when {
                signedIn == false -> SignInCard(dismissible = false)
                yt is Load.Failed -> ErrorState(yt.message, retry)
                else -> {
                    val own = Library.own((yt as? Load.Done)?.value.orEmpty())
                    if (own.isNotEmpty()) {
                        SectionRule(stringResource(R.string.youtube_music))
                        own.forEachIndexed { i, row -> ItemRow(row, "list_yt_$i", row.count) }
                    }
                }
            }
        }
    }
    item(key = "local") {
        val (local, retryLocal) = rememberLoad("playlists:local") { Remote.playlists("local") }
        Column {
            SectionRule(stringResource(R.string.local))
            LoadState(local, retryLocal)
            val rows = (local as? Load.Done)?.value.orEmpty().filter { it.id !in Library.VIRTUAL }
            if (local is Load.Done && rows.isEmpty()) {
                EmptyState(stringResource(R.string.no_lists), stringResource(R.string.new_list_more)) { Nav.sheet = Sheet.NewList(emptyList()) }
            }
            rows.forEachIndexed { i, row -> ItemRow(row, "list_local_$i", row.count) }
        }
    }
}

/** Artistas: the followed (♥) and other library artists; a tap opens the artist. */
private fun LazyListScope.artists() {
    item(key = "artists") {
        val (load, retry) = rememberLoad("artists") { Remote.artists() }
        val favorites by Remote.favorites.collectAsStateWithLifecycle()
        Column {
            LoadState(load, retry)
            // The Artist page opens channel and local artist IDs only (§2.4).
            val rows = (load as? Load.Done)?.value.orEmpty().filter { it.id.startsWith("UC") || it.id.startsWith("local:ar:") }
            if (load is Load.Done && rows.isEmpty()) EmptyState(stringResource(R.string.no_results))
            rows.forEachIndexed { i, row -> ItemRow(row, "artist_$i", favorite = favorites[row.id] ?: row.favorite) }
        }
    }
}

/** A 64 dp row of a list or an artist: square art, the name and its song count, or ♥ when followed. A long-press opens its actions. */
@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun ItemRow(item: Item, tag: String, count: Int = 0, favorite: Boolean = false) {
    val p = LocalPalette.current
    val haptic = LocalHapticFeedback.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = 64.dp).combinedClickable(
            onLongClick = {
                haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                Nav.sheet = Sheet.Of(item)
            },
            onClick = { item.screen()?.let(Nav::open) },
        ).padding(horizontal = 16.dp).testTag(tag),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        ArtBox(item.art, 48.dp)
        Text(item.title, Modifier.weight(1f).padding(horizontal = 12.dp), color = p.textPrimary, style = Type.body, maxLines = 2, overflow = TextOverflow.Ellipsis)
        if (count > 0) Text("$count", color = p.textSecondary, style = Type.label)
        if (favorite) Icon(painterResource(R.drawable.ic_heart), null, Modifier.size(16.dp), tint = p.favorite)
    }
}

/** Álbumes' pages in one play order (§5.5, as foryou: the next offset is offset + limit, an empty page ends it). */
@Stable
private class AlbumPages {
    var pages by mutableStateOf(emptyList<List<Item>>())
    var end by mutableStateOf(false)
    var failed by mutableStateOf<String?>(null)
}

/**
 * Álbumes (§3.5): the play orders without foryou as chips, then a
 * grid of cols columns, ♥ on saved albums, a page of 100 at a time as the end
 * of the list comes near.
 */
@OptIn(ExperimentalLayoutApi::class)
private fun LazyListScope.albums(a: AlbumPages, sorts: List<Sort>, current: String, cols: Int, setSort: (String) -> Unit, retry: () -> Unit) {
    item(key = "album_sorts") {
        FlowRow(Modifier.padding(horizontal = 12.dp)) {
            sorts.filter { it.id != "foryou" }.forEachIndexed { i, o -> BracketChip(o.label, o.id == current, Modifier.testTag("album_sort_$i")) { setSort(o.id) } }
        }
    }
    val items = a.pages.flatten()
    itemsIndexed(items.chunked(cols), key = { r, _ -> "album_row_$current:$cols:$r" }) { r, row ->
        val favorites by Remote.favorites.collectAsStateWithLifecycle()
        BoxWithConstraints(Modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
            val w = (maxWidth - 12.dp * (cols - 1)) / cols
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                row.forEachIndexed { k, item ->
                    Tile(item.copy(favorite = favorites[item.id] ?: item.favorite), Modifier.testTag("album_${r * cols + k}"), width = w) { openItem(item, items) }
                }
            }
        }
    }
    item(key = "album_state") {
        val failed = a.failed
        when {
            failed != null -> ErrorState(failed, retry)
            a.end && items.isEmpty() -> EmptyState(stringResource(R.string.no_results))
            !a.end -> Loading()
        }
    }
}

/** Loads Álbumes' pages while the chip shows: the first at once, the next as the last rows come into view. */
@Composable
private fun rememberAlbums(sort: String, list: LazyListState, active: Boolean): Triple<AlbumPages, List<Sort>, () -> Unit> {
    val reloads by Remote.reloads.collectAsStateWithLifecycle()
    var sorts by remember { mutableStateOf(emptyList<Sort>()) }
    var attempt by remember { mutableIntStateOf(0) }
    val a = remember(sort, reloads) { AlbumPages() }
    LaunchedEffect(a, active, attempt) {
        if (!active) return@LaunchedEffect
        snapshotFlow { (list.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0) to list.layoutInfo.totalItemsCount }.collect { (last, count) ->
            if (a.end || a.failed != null || (a.pages.isNotEmpty() && last < count - 4)) return@collect
            val offset = a.pages.size * Remote.ALBUMS
            try {
                val (page, s) = Remote.albums(sort, offset)
                sorts = s
                if (Paging.foryou(offset, Remote.ALBUMS, page.size) == null) a.end = true else a.pages += listOf(page)
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                a.failed = Remote.message(e)
            }
        }
    }
    return Triple(a, sorts) {
        a.failed = null
        attempt++
    }
}

/**
 * Descargas (§3.13): the song that downloads with its progress, the queue,
 * the failures with Reintentar, then Reproducir descargas and Cancelar.
 * download.status is read every second while it runs and the chip shows.
 */
@Composable
private fun DownloadsPanel() {
    val p = LocalPalette.current
    val ops by Remote.ops.collectAsStateWithLifecycle()
    var state by remember { mutableStateOf<Load<Downloads>>(Load.Loading) }
    var poke by remember { mutableIntStateOf(0) }
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    LaunchedEffect(poke) {
        lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) {
            while (true) {
                state = try {
                    Load.Done(Remote.downloads())
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    // unavailable while connected: this core has no offline downloads.
                    Load.Failed(if (e is IpcError && e.code == "unavailable" && Remote.connected.value) "" else Remote.message(e))
                }
                // A queue that runs is followed closely; an idle one now and then, for a download started meanwhile.
                delay(if ((state as? Load.Done)?.value?.running == true) 1000 else 4000)
            }
        }
    }
    fun run(op: String, params: JSONObject = JSONObject()) {
        Remote.fire(op, params)
        poke++
    }
    Column {
        when (val s = state) {
            Load.Loading -> Loading()
            is Load.Failed -> if (s.message.isEmpty()) EmptyState(stringResource(R.string.downloads_off)) else ErrorState(s.message) { poke++ }
            is Load.Done -> {
                val d = s.value
                d.current?.let { t ->
                    SectionRule(stringResource(R.string.downloading))
                    Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp).heightIn(min = 48.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text("↓ " + t.name, Modifier.weight(1f), color = p.textPrimary, style = Type.body, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        Text("${d.percent} %", color = p.textPrimary, style = Type.numeric)
                    }
                    Box(
                        Modifier.fillMaxWidth().height(16.dp).padding(horizontal = 16.dp).testTag("download_progress").drawBehind {
                            val y = size.height / 2
                            val x = size.width * d.percent.coerceIn(0, 100) / 100f
                            drawLine(p.textSecondary, Offset(x, y), Offset(size.width, y), 1.dp.toPx())
                            drawLine(p.accent, Offset(0f, y), Offset(x, y), 3.dp.toPx())
                            drawCircle(p.accent, 5.dp.toPx(), Offset(x, y))
                        },
                    )
                }
                if (d.queued.isNotEmpty()) {
                    SectionRule(stringResource(R.string.downloads_queued, d.queued.size))
                    d.queued.forEach { Text("  " + it.name, Modifier.padding(horizontal = 16.dp, vertical = 6.dp), color = p.textPrimary, style = Type.body, maxLines = 1, overflow = TextOverflow.Ellipsis) }
                }
                if (d.failed.isNotEmpty()) {
                    SectionRule(stringResource(R.string.downloads_failed, d.failed.size)) {
                        BracketButton(stringResource(R.string.retry), Modifier.testTag("download_retry")) { run("download.retry") }
                    }
                    d.failed.forEach { (t, reason) ->
                        Column(Modifier.padding(horizontal = 16.dp, vertical = 6.dp)) {
                            Row {
                                Text("✗ ", color = p.error, style = Type.body)
                                Text(t.name, color = p.textPrimary, style = Type.body, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            }
                            Text("  $reason", color = p.textSecondary, style = Type.label, maxLines = 2, overflow = TextOverflow.Ellipsis)
                        }
                    }
                }
                if (d.current == null && d.done > 0) {
                    Text("✓ " + pluralStringResource(R.plurals.downloads_done, d.done, d.done), Modifier.padding(horizontal = 16.dp, vertical = 8.dp), color = p.playing, style = Type.bodyStrong)
                } else if (d.current == null && d.queued.isEmpty() && d.failed.isEmpty()) {
                    EmptyState(stringResource(R.string.downloads_none))
                }
                Row(Modifier.padding(horizontal = 16.dp, vertical = 8.dp), horizontalArrangement = Arrangement.spacedBy(16.dp), verticalAlignment = Alignment.CenterVertically) {
                    if (d.directory.isNotEmpty() && "url.load" in ops) {
                        KeyPill("▶ " + stringResource(R.string.play_downloads), Modifier.testTag("downloads_play")) {
                            Remote.fire("url.load", JSONObject().put("path", d.directory).put("play", true))
                        }
                    }
                    if (d.running || d.queued.isNotEmpty()) BracketButton(stringResource(R.string.cancel), Modifier.testTag("downloads_cancel")) { run("download.cancel") }
                }
            }
        }
    }
}
