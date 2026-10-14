package io.github.xaviiromf.cliampx.ui

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.wrapContentHeight
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.itemsIndexed
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.SwipeToDismissBox
import androidx.compose.material3.SwipeToDismissBoxState
import androidx.compose.material3.SwipeToDismissBoxValue
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.Item
import io.github.xaviiromf.cliampx.Kind
import io.github.xaviiromf.cliampx.Library
import io.github.xaviiromf.cliampx.Page
import io.github.xaviiromf.cliampx.Paging
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Track
import io.github.xaviiromf.cliampx.clock
import kotlinx.coroutines.CancellationException
import org.json.JSONObject

/**
 * Album (§2.4, §3.6): the cover, title, artist and preview lines, play,
 * shuffle, save and download, then the numbered songs. A song plays the
 * album from it.
 */
@Composable
fun AlbumScreen(s: Screen.Album) {
    val ops by Remote.ops.collectAsStateWithLifecycle()
    val snap by Remote.state.collectAsStateWithLifecycle()
    val signedIn by Remote.signedIn.collectAsStateWithLifecycle()
    val pid = s.id.removePrefix("al:")
    val (tracks, retry) = rememberLoad("album:${s.id}") { Remote.albumTracks(s.id) }
    val (preview, _) = rememberLoad("preview:$pid") { Remote.preview(pid) }
    val rows = (tracks as? Load.Done)?.value.orEmpty()
    val pv = (preview as? Load.Done)?.value
    val favorites by Remote.favorites.collectAsStateWithLifecycle()
    val saved = favorites[pid] ?: pv?.favorite
    val artist = rows.firstOrNull()?.artist?.substringBefore(", ")?.ifEmpty { null } ?: s.subtitle
    val artistId = rows.firstNotNullOfOrNull { it.meta["ytmusic.artist"] }
    val art = s.art.ifEmpty { pv?.art.orEmpty() }.ifEmpty { rows.firstOrNull()?.art.orEmpty() }
    val item = Item(Kind.ALBUM, s.id, s.title, artist, art, artistId = artistId.orEmpty())
    Pull(Remote::reload) {
        LazyColumn(Modifier.fillMaxSize()) {
            item(key = "top") { TopBar(more = { Nav.sheet = Sheet.Of(item) }) }
            item(key = "head") {
                Header(art, s.title, artist, pv?.lines.orEmpty().filter { it != artist }, artistId?.let { { Nav.open(Screen.Artist(it, artist)) } })
            }
            item(key = "buttons") {
                Buttons(rows, play = { i -> Remote.loadAlbum(s.id, i, rows[i]) }) {
                    // Save: YouTube albums (MPREb_, a release al:MPREb_ too) of a signed-in account, once the state is known.
                    if (saved != null && signedIn == true && pid.startsWith("MPREb_") && "provider.favorite" in ops) {
                        BracketChip("", saved, Modifier.testTag("detail_save"), icon = if (saved) R.drawable.ic_heart else R.drawable.ic_heart_outline) {
                            Remote.toggle("provider.favorite", pid)
                        }
                    }
                    if ("download" in ops && !item.local) DownloadChip(s.id, s.title)
                }
            }
            item(key = "rule") { SectionRule(stringResource(R.string.songs)) }
            if (tracks !is Load.Done) item(key = "state") { LoadState(tracks, retry) }
            if (tracks is Load.Done && rows.isEmpty()) item(key = "empty") { EmptyState(stringResource(R.string.no_songs)) }
            numbered(rows, snap?.track?.path) { i -> Remote.loadAlbum(s.id, i, rows[i]) }
        }
    }
}

/**
 * Artist (§2.4, §3.7): the picture, name and preview lines, follow and
 * radio, the five popular songs, then albums, singles and local albums.
 */
@Composable
fun ArtistScreen(s: Screen.Artist) {
    val ops by Remote.ops.collectAsStateWithLifecycle()
    val signedIn by Remote.signedIn.collectAsStateWithLifecycle()
    val snap by Remote.state.collectAsStateWithLifecycle()
    val local = s.id.startsWith("local:")
    // An artist's top songs are ar:UC…; a local artist lists all its songs.
    val topId = if (local) s.id else "ar:${s.id}"
    val (preview, _) = rememberLoad("preview:${s.id}") { Remote.preview(s.id) }
    val (albums, retry) = rememberLoad("artist:${s.id}") { Remote.artistAlbums(s.id) }
    val (top, retryTop) = rememberLoad("album:$topId") { Remote.albumTracks(topId) }
    val pv = (preview as? Load.Done)?.value
    val favorites by Remote.favorites.collectAsStateWithLifecycle()
    val following = favorites[s.id] ?: pv?.favorite
    val releases = (albums as? Load.Done)?.value.orEmpty().filterNot { it.id.startsWith("ar:") }
    val songs = (top as? Load.Done)?.value.orEmpty().let { if (local) it else it.take(5) }
    val art = s.art.ifEmpty { pv?.art.orEmpty() }
    val item = Item(Kind.ARTIST, s.id, s.name, art = art)
    val play = { i: Int -> if (local) Remote.loadAlbum(s.id, i, songs[i]) else Remote.load("ytmusic", topId, i, songs[i]) }
    Pull(Remote::reload) {
        LazyColumn(Modifier.fillMaxSize()) {
            item(key = "top") { TopBar(more = { Nav.sheet = Sheet.Of(item) }) }
            item(key = "head") { Header(art, s.name, "", pv?.lines.orEmpty()) }
            item(key = "buttons") {
                Row(Modifier.padding(horizontal = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                    if (following != null && signedIn == true && !local && "provider.favorite" in ops) {
                        BracketButton(stringResource(if (following) R.string.following else R.string.follow), Modifier.testTag("detail_follow")) {
                            Remote.toggle("provider.favorite", s.id)
                        }
                    }
                    if (!local && "radio.start" in ops) {
                        BracketButton("◉ " + stringResource(R.string.radio), Modifier.testTag("detail_radio")) {
                            Remote.fire("radio.start", JSONObject().put("provider", "ytmusic").put("playlist", s.id).put("name", s.name))
                        }
                    }
                }
            }
            item(key = "popular") { SectionRule(stringResource(R.string.popular)) }
            if (top !is Load.Done) item(key = "top_state") { LoadState(top, retryTop) }
            if (top is Load.Done && songs.isEmpty()) item(key = "top_empty") { EmptyState(stringResource(R.string.no_songs)) }
            itemsIndexed(songs, key = { i, t -> "p:$i:${t.path}" }) { i, t ->
                TrackRow(
                    t, snap?.track?.path == t.path, Modifier.testTag("row_$i"), marker = "${i + 1}",
                    onLongClick = { Nav.sheet = Sheet.Song(t) },
                    trailing = { IconBtn(R.drawable.ic_more, stringResource(R.string.more), Modifier.testTag("row_more_$i")) { Nav.sheet = Sheet.Song(t) } },
                ) { play(i) }
            }
            if (!local && songs.isNotEmpty()) {
                item(key = "all") {
                    BracketButton(stringResource(R.string.see_all_songs), Modifier.padding(horizontal = 12.dp).testTag("detail_all")) {
                        Nav.open(Screen.Playlist("ytmusic", topId, s.name))
                    }
                }
            }
            if (albums !is Load.Done) item(key = "albums_state") { LoadState(albums, retry) }
            for ((title, rows, tag) in listOf(
                Triple(R.string.albums, releases.filter { it.section != "single" && !it.local }, "albums"),
                Triple(R.string.singles, releases.filter { it.section == "single" }, "singles"),
                Triple(R.string.in_local, releases.filter { it.local }, "local"),
            )) {
                if (rows.isNotEmpty()) item(key = tag) { Carousel(stringResource(title), rows, tag) { _, it -> openItem(it, rows) } }
            }
        }
    }
}

/**
 * Playlist (§2.4, §3.8): any provider list, `n canciones · h min`, play,
 * shuffle and download, then its songs in pages of 200 (§5.5).
 */
@Composable
fun PlaylistScreen(s: Screen.Playlist) {
    val ops by Remote.ops.collectAsStateWithLifecycle()
    val snap by Remote.state.collectAsStateWithLifecycle()
    val key = { offset: Int -> "tracks:${s.provider}:${s.id}:$offset" }
    val (first, retry) = rememberLoad(key(0)) { Remote.tracks(s.provider, s.id, 0) }
    val (preview, _) = if (s.id.startsWith("pl:")) rememberLoad("preview:${s.id}") { Remote.preview(s.id) } else Load.Loading to {}
    val page = (first as? Load.Done)?.value
    val total = page?.total ?: 0
    // The later pages: those still cached show at once, so the scroll position holds.
    var more by remember(page) {
        mutableStateOf(
            generateSequence(200) { it + 200 }.takeWhile { it < total }.map { Remote.peek<Page>(key(it)) }
                .takeWhile { it != null }.flatMap { it!!.rows }.toList(),
        )
    }
    val all = page?.rows.orEmpty() + more
    // Rows swiped away from a local list, by place, until the list is read again.
    var gone by remember(page) { mutableStateOf(emptySet<Int>()) }
    val shown = all.indices.filter { it !in gone }
    val rows = shown.map { all[it] }
    val list = rememberLazyListState()
    LaunchedEffect(page) {
        if (page == null) return@LaunchedEffect
        snapshotFlow { (list.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0) to list.layoutInfo.totalItemsCount }.collect { (last, count) ->
            val offset = Paging.next(page.rows.size + more.size, total) ?: return@collect
            if (last < count - 20) return@collect
            try {
                more = more + Remote.tracks(s.provider, s.id, offset).rows
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                Remote.showError(e)
            }
        }
    }
    val pv = (preview as? Load.Done)?.value
    val art = s.art.ifEmpty { pv?.art.orEmpty() }.ifEmpty { rows.firstOrNull()?.art.orEmpty() }
    val item = Item(Kind.PLAYLIST, s.id, s.title, art = art, provider = s.provider)
    val editable = Library.editable(item) && "playlist.remove" in ops
    // A row's place in the list on the core: its place when read, less the rows removed above it since.
    fun index(i: Int) = shown[i] - gone.count { it < shown[i] }
    Pull(Remote::reload) {
        LazyColumn(Modifier.fillMaxSize(), state = list) {
            item(key = "top") { TopBar(more = { Nav.sheet = Sheet.Of(item) }) }
            item(key = "head") {
                val p = LocalPalette.current
                Row(Modifier.padding(horizontal = 16.dp, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                    ArtBox(art, 160.dp)
                    Column(Modifier.padding(start = 16.dp)) {
                        Text(s.title, color = p.accent, style = Type.display, maxLines = 3, overflow = TextOverflow.Ellipsis)
                        if (page != null) Text(listLength(total - gone.size, rows), color = p.textSecondary, style = Type.label)
                    }
                }
            }
            item(key = "buttons") {
                Buttons(rows, play = { i -> Remote.load(s.provider, s.id, i, rows[i]) }) {
                    if ("download" in ops && !item.local) DownloadChip(s.id, s.title)
                }
            }
            item(key = "rule") { SectionRule(stringResource(R.string.songs)) }
            if (first !is Load.Done) item(key = "state") { LoadState(first, retry) }
            if (page != null && rows.isEmpty()) item(key = "empty") { EmptyState(stringResource(R.string.no_songs)) }
            itemsIndexed(rows, key = { i, t -> "${shown[i]}:${t.path}" }) { i, t ->
                val sheet = { Nav.sheet = if (editable) Sheet.Song(t, list = s.id, row = index(i)) else Sheet.Song(t) }
                val row = @Composable {
                    TrackRow(
                        t, snap?.track?.path == t.path, Modifier.testTag("row_$i"), onLongClick = sheet,
                        trailing = { IconBtn(R.drawable.ic_more, stringResource(R.string.more), Modifier.testTag("row_more_$i"), onClick = sheet) },
                    ) { Remote.load(s.provider, s.id, index(i), t) }
                }
                if (editable) {
                    SwipeRemove({
                        val at = shown[i]
                        Remote.editList("playlist.remove", JSONObject().put("playlist", s.id).put("index", index(i)), failed = { gone = gone - at })
                        gone = gone + at
                    }) { row() }
                } else {
                    row()
                }
            }
        }
    }
}

/** A row that a swipe to the start removes (§2.4): the red label shows behind it. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SwipeRemove(remove: () -> Unit, content: @Composable () -> Unit) {
    // Not saveable: a reloaded list reuses keys, and a restored "dismissed" state would remove the row that takes the key.
    val state = remember { SwipeToDismissBoxState(SwipeToDismissBoxValue.Settled, positionalThreshold = { it * 0.4f }) }
    SwipeToDismissBox(
        state,
        backgroundContent = { SwipeBehind(state, stringResource(R.string.remove_from_list)) },
        enableDismissFromStartToEnd = false,
        onDismiss = { if (it == SwipeToDismissBoxValue.EndToStart) remove() },
    ) { content() }
}

/** `n canciones · h min`; the length only once every row is loaded. */
@Composable
private fun listLength(total: Int, rows: List<Track>): String {
    val count = pluralStringResource(R.plurals.songs_count, total, total)
    if (rows.size < total || rows.isEmpty()) return count
    val min = rows.sumOf { it.durationSecs } / 60
    if (min == 0) return count // local files without a duration tag
    return count + " · " + if (min >= 60) stringResource(R.string.hours_minutes, min / 60, min % 60) else stringResource(R.string.minutes, min)
}

/**
 * Genre (§2.4, §3.8): the play orders as chips, play, the ★ that pins it
 * to Inicio, and its songs. Play and a song replace the queue with the
 * rows shown.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun GenreScreen(s: Screen.Genre) {
    val ops by Remote.ops.collectAsStateWithLifecycle()
    val snap by Remote.state.collectAsStateWithLifecycle()
    val (genres, _) = rememberLoad("genres:false") { Remote.genres(false) }
    val (moods, _) = rememberLoad("genres:true") { Remote.genres(true) }
    val sorts = (genres as? Load.Done)?.value?.second.orEmpty()
    var sort by rememberSaveable { mutableStateOf("") }
    val current = sort.ifEmpty { sorts.firstOrNull()?.id.orEmpty() }
    val known = listOf(genres, moods).flatMap { (it as? Load.Done)?.value?.first.orEmpty() }.firstOrNull { it.id == s.id }
    val favorites by Remote.favorites.collectAsStateWithLifecycle()
    val starred = favorites[s.id] ?: known?.favorite ?: false
    val (tracks, retry) = rememberLoad("genre:${s.id}:$current") { Remote.genreTracks(s.id, current) }
    val hidden by Remote.hidden.collectAsStateWithLifecycle()
    // Play sends the rows shown, so a song rated "No me interesa" can leave them at once (§2.6).
    val rows = (tracks as? Load.Done)?.value.orEmpty().filter { it.path !in hidden }
    val local = s.id.startsWith("local:")
    val item = Item(Kind.GENRE, s.id, s.title, favorite = starred)
    Pull(Remote::reload) {
        LazyColumn(Modifier.fillMaxSize()) {
            item(key = "top") { TopBar(more = { Nav.sheet = Sheet.Of(item) }) }
            item(key = "head") {
                val p = LocalPalette.current
                Column {
                    Text(s.title, Modifier.padding(horizontal = 16.dp, vertical = 8.dp), color = p.accent, style = Type.display)
                    if (!local && sorts.isNotEmpty()) {
                        FlowRow(Modifier.padding(horizontal = 12.dp)) {
                            sorts.forEachIndexed { i, o -> BracketChip(o.label, o.id == current, Modifier.testTag("sort_$i")) { sort = o.id } }
                        }
                    }
                }
            }
            item(key = "buttons") {
                Buttons(rows, shuffle = false, play = { i -> Remote.play(rows, i) }) {
                    if (!local && "provider.genre_favorite" in ops) {
                        BracketChip(stringResource(if (starred) R.string.unpin else R.string.pin), starred, Modifier.testTag("detail_pin"), icon = R.drawable.ic_star) {
                            Remote.toggle("provider.genre_favorite", s.id)
                        }
                    }
                }
            }
            item(key = "rule") { SectionRule(stringResource(R.string.songs)) }
            if (tracks !is Load.Done) item(key = "state") { LoadState(tracks, retry) }
            if (tracks is Load.Done && rows.isEmpty()) item(key = "empty") { EmptyState(stringResource(R.string.no_songs)) }
            itemsIndexed(rows, key = { i, t -> "$current:$i:${t.path}" }) { i, t ->
                TrackRow(
                    t, snap?.track?.path == t.path, Modifier.testTag("row_$i"),
                    onLongClick = { Nav.sheet = Sheet.Song(t) },
                    trailing = { IconBtn(R.drawable.ic_more, stringResource(R.string.more), Modifier.testTag("row_more_$i")) { Nav.sheet = Sheet.Song(t) } },
                ) { Remote.play(rows, i) }
            }
        }
    }
}

/**
 * Para ti (§2.4, §3.8): the endless grid of recommended albums or
 * artists, a page of 40 at a time as the end comes near (§5.5).
 */
@Composable
fun ParaTiScreen(s: Screen.ParaTi) {
    val reloads by Remote.reloads.collectAsStateWithLifecycle()
    val key = { offset: Int -> "foryou:${s.artists}:$offset:$PAGE" }
    // The pages still cached show at once, so a screen that comes back keeps its place.
    var pages by remember(reloads) {
        mutableStateOf(generateSequence(0) { it + PAGE }.map { Remote.peek<List<Item>>(key(it)) }.takeWhile { it != null }.map { it!! }.toList())
    }
    var end by remember(reloads) { mutableStateOf(false) }
    var failed by remember(reloads) { mutableStateOf<String?>(null) }
    var attempt by remember { mutableIntStateOf(0) }
    val grid = rememberLazyGridState()
    LaunchedEffect(reloads, attempt) {
        snapshotFlow { (grid.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0) to grid.layoutInfo.totalItemsCount }.collect { (last, count) ->
            if (end || failed != null || last < count - 12) return@collect
            val offset = pages.size * PAGE
            try {
                val page = Remote.forYou(s.artists, offset, PAGE)
                if (Paging.foryou(offset, PAGE, page.size) == null) end = true else pages = pages + listOf(page)
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                failed = Remote.message(e)
            }
        }
    }
    val hidden by Remote.hidden.collectAsStateWithLifecycle()
    val items = pages.flatten().filter { it.id !in hidden }
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(if (s.artists) R.string.foryou_artists else R.string.foryou_albums))
        Pull(Remote::reload) {
            LazyVerticalGrid(
                GridCells.Adaptive(160.dp), Modifier.fillMaxSize(), grid, PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp), horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                itemsIndexed(items, key = { i, it -> "$i:${it.id}" }) { i, item ->
                    BoxWithConstraints { Tile(item, Modifier.testTag("tile_foryou_$i"), width = maxWidth) { openItem(item, items) } }
                }
                when {
                    failed != null -> item(key = "failed", span = { GridItemSpan(maxLineSpan) }) {
                        ErrorState(failed!!) {
                            failed = null
                            attempt++
                        }
                    }
                    end && items.isEmpty() -> item(key = "empty", span = { GridItemSpan(maxLineSpan) }) { EmptyState(stringResource(R.string.no_results)) }
                    // No key: a keyed spinner would stay the scroll anchor as each page goes in above it.
                    !end -> item { SpinnerTile() }
                }
            }
        }
    }
}

private const val PAGE = 40

/** Remixes y versiones (§2.4): the songs by the core's groups; a song plays its group from it. */
@Composable
fun RemixesScreen(s: Screen.Remixes) {
    val snap by Remote.state.collectAsStateWithLifecycle()
    val (load, retry) = rememberLoad("remixes:${s.track.path}") { Remote.remixes(s.track) }
    val sections = (load as? Load.Done)?.value.orEmpty()
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.remixes))
        LazyColumn(Modifier.fillMaxSize()) {
            if (load !is Load.Done) item(key = "state") { LoadState(load, retry) }
            if (load is Load.Done && sections.isEmpty()) item(key = "empty") { EmptyState(stringResource(R.string.no_results)) }
            var n = 0
            sections.forEachIndexed { k, section ->
                item(key = "rule:$k") { SectionRule(section.title) }
                val first = n
                itemsIndexed(section.rows, key = { i, t -> "$k:$i:${t.path}" }) { i, t ->
                    TrackRow(
                        t, snap?.track?.path == t.path, Modifier.testTag("row_${first + i}"),
                        onLongClick = { Nav.sheet = Sheet.Song(t) },
                        trailing = { IconBtn(R.drawable.ic_more, stringResource(R.string.more)) { Nav.sheet = Sheet.Song(t) } },
                    ) { Remote.play(section.rows, i) }
                }
                n += section.rows.size
            }
        }
    }
}

/** The big art, the title, a tappable subtitle and the preview lines joined by ` · ` (§3.6). */
@Composable
private fun Header(art: String, title: String, subtitle: String, lines: List<String>, onSubtitle: (() -> Unit)? = null, size: Dp = 240.dp) {
    val p = LocalPalette.current
    Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        ArtBox(art, size)
        Text(title, Modifier.fillMaxWidth().padding(top = 12.dp), color = p.accent, style = Type.display, maxLines = 3, overflow = TextOverflow.Ellipsis)
        if (subtitle.isNotEmpty()) {
            Text(
                subtitle,
                Modifier.fillMaxWidth().testTag("detail_artist").then(
                    // A link gets a 48 dp target (§1.2), the text centered in it.
                    if (onSubtitle != null) Modifier.clickable(onClick = onSubtitle).heightIn(min = 48.dp).wrapContentHeight() else Modifier.padding(vertical = 4.dp),
                ),
                color = p.textPrimary, style = Type.bodyStrong, maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
        }
        if (lines.isNotEmpty()) {
            Text(lines.joinToString(" · "), Modifier.fillMaxWidth(), color = p.textSecondary, style = Type.label, maxLines = 3, overflow = TextOverflow.Ellipsis, textAlign = TextAlign.Start)
        }
    }
}

/**
 * ▶ Reproducir (key pill) and [⤨], which turns shuffle on and plays a
 * random row, then the page's own buttons, wrapping at a big font. Both
 * wait for the rows.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Buttons(rows: List<Track>, shuffle: Boolean = true, play: (Int) -> Unit, extra: @Composable () -> Unit = {}) {
    FlowRow(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp), itemVerticalAlignment = Alignment.CenterVertically) {
        KeyPill("▶ " + stringResource(R.string.play), Modifier.testTag("detail_play"), enabled = rows.isNotEmpty()) { play(0) }
        if (shuffle) {
            BracketChip("", false, Modifier.testTag("detail_shuffle"), icon = R.drawable.ic_shuffle) {
                if (rows.isNotEmpty()) Remote.shuffled { play(rows.indices.random()) }
            }
        }
        extra()
    }
}

/** [↓]: downloads a list or album for offline play (G15). */
@Composable
private fun DownloadChip(id: String, title: String) {
    BracketChip("↓", false, Modifier.testTag("detail_download")) {
        Remote.fire("download", JSONObject().put("provider", "ytmusic").put("playlist", id).put("name", title))
    }
}

/** An album's songs: number, title, duration and ⋮, 56 dp; the playing one ▶ in green (§3.6). */
@OptIn(ExperimentalFoundationApi::class)
private fun LazyListScope.numbered(rows: List<Track>, playingPath: String?, play: (Int) -> Unit) {
    itemsIndexed(rows, key = { i, t -> "$i:${t.path}" }) { i, t ->
        val p = LocalPalette.current
        val haptic = LocalHapticFeedback.current
        val playing = playingPath == t.path
        Row(
            Modifier.fillMaxWidth().heightIn(min = 56.dp).combinedClickable(
                onLongClick = {
                    haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                    Nav.sheet = Sheet.Song(t)
                },
                onClick = { play(i) },
            ).padding(start = 16.dp, end = 4.dp).testTag("row_$i"),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text("%2d".format(i + 1), Modifier.width(28.dp), color = p.textSecondary, style = Type.body)
            Text(
                (if (playing) "▶" else "") + t.name, Modifier.weight(1f), color = if (playing) p.playing else p.textPrimary,
                style = if (playing) Type.bodyStrong else Type.body, maxLines = 2, overflow = TextOverflow.Ellipsis,
            )
            if (t.durationSecs > 0) Text("  " + clock(t.durationSecs), color = p.textSecondary, style = Type.label)
            IconBtn(R.drawable.ic_more, stringResource(R.string.more), Modifier.testTag("row_more_$i")) { Nav.sheet = Sheet.Song(t) }
        }
    }
    item(key = "end") { Spacer(Modifier.heightIn(min = 16.dp)) }
}
