package io.github.xaviiromf.cliampx.ui

import android.content.Context
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.core.content.edit
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.Item
import io.github.xaviiromf.cliampx.Kind
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Section
import io.github.xaviiromf.cliampx.item
import io.github.xaviiromf.cliampx.kind
import io.github.xaviiromf.cliampx.recents
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay

@Immutable
private sealed interface Results {
    data object None : Results
    data object Loading : Results
    data class Failed(val message: String) : Results
    data class Done(val sections: List<Section>) : Results
}

/**
 * Buscar (UI-SPEC §3.3, §3.4). With no query: the recent queries and the
 * genre grids. With one: the results by the core's sections, artists,
 * albums and playlists as tiles and songs as rows. A song plays its section
 * from it; a tile opens its page.
 */
@Composable
fun SearchScreen() {
    val p = LocalPalette.current
    val context = LocalContext.current
    // Clearing the focus hides the keyboard, and a closing sheet then has no field to give it back to (§1.2).
    val focus = LocalFocusManager.current
    // A hardware Enter clears the focus but leaves the soft keyboard up: hide it too.
    val keyboard = LocalSoftwareKeyboardController.current
    val prefs = remember { context.getSharedPreferences("ui", Context.MODE_PRIVATE) }
    var recent by remember { mutableStateOf(prefs.getString(RECENT, "").orEmpty().lines().filter(String::isNotBlank)) }
    fun keep(list: List<String>) {
        recent = list
        prefs.edit { putString(RECENT, list.joinToString("\n")) }
    }
    var query by rememberSaveable { mutableStateOf("") }
    var submitted by rememberSaveable { mutableStateOf("") }
    fun submit() {
        submitted = query.trim()
        keep(recents(recent, submitted))
        focus.clearFocus()
        keyboard?.hide()
    }
    var attempt by remember { mutableIntStateOf(0) }
    // The query the list shows: only a new one scrolls back to the top.
    var shown by rememberSaveable { mutableStateOf("") }
    // Back from another tab, the last rows show at once, at their scroll position, while they refresh.
    var results by remember { mutableStateOf<Results>(Remote.peek<List<Section>>("search:$submitted")?.let(Results::Done) ?: Results.None) }
    val list = rememberLazyListState()
    val browse = rememberLazyListState()
    val snap by Remote.state.collectAsStateWithLifecycle()
    val hidden by Remote.hidden.collectAsStateWithLifecycle()

    LaunchedEffect(query) {
        val q = query.trim()
        if (q.length < 2) {
            submitted = ""
            return@LaunchedEffect
        }
        delay(450)
        submitted = q
    }
    LaunchedEffect(submitted, attempt) {
        if (submitted.isEmpty()) {
            results = Results.None
            shown = ""
            return@LaunchedEffect
        }
        if (submitted != shown || results !is Results.Done) results = Results.Loading
        results = try {
            Results.Done(Remote.search(submitted))
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            Results.Failed(Remote.message(e))
        }
        if (submitted != shown) {
            shown = submitted
            list.scrollToItem(0)
        }
    }
    LaunchedEffect(Unit) { Nav.reselected.collect { if (it == Tab.Search) (if (submitted.isEmpty()) browse else list).animateScrollToItem(0) } }
    // A result the user takes makes its query a recent one.
    val took = {
        focus.clearFocus()
        keep(recents(recent, submitted))
    }

    Column(Modifier.fillMaxSize()) {
        ScreenRule(stringResource(R.string.tab_search))
        val hint = stringResource(R.string.search_hint)
        BasicTextField(
            query, { query = it },
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp).testTag("search_field"),
            singleLine = true,
            textStyle = Type.body.copy(color = p.textPrimary),
            cursorBrush = SolidColor(p.accent),
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
            keyboardActions = KeyboardActions(onSearch = { submit() }),
            decorationBox = { field ->
                Row(Modifier.height(48.dp).background(p.surfaceHigh).padding(start = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text("> ", color = p.accent, style = Type.body)
                    Box(Modifier.weight(1f)) {
                        if (query.isEmpty()) {
                            Text(
                                buildAnnotatedString {
                                    append(hint)
                                    withStyle(SpanStyle(color = p.accent)) { append("_") }
                                },
                                color = p.textSecondary, style = Type.body, maxLines = 1,
                            )
                        }
                        field()
                    }
                    if (query.isNotEmpty()) {
                        val clear = stringResource(R.string.search_clear)
                        Box(
                            Modifier.size(48.dp).clickable { query = "" }.testTag("search_clear").semantics { contentDescription = clear },
                            contentAlignment = Alignment.Center,
                        ) { Text("✕", color = p.textSecondary, style = Type.body) }
                    }
                }
            },
        )
        if (submitted.isEmpty()) {
            Browse(browse, recent, onRecent = {
                query = it
                submitted = it
                keep(recents(recent, it))
            }, onClear = { keep(emptyList()) })
            return@Column
        }
        when (val r = results) {
            Results.None -> {}
            Results.Loading -> Loading()
            is Results.Failed -> ErrorState(r.message) { attempt++ }
            is Results.Done -> {
                val sections = r.sections.map { s -> s.copy(rows = s.rows.filter { it.path !in hidden && it.item().id !in hidden }) }.filter { it.rows.isNotEmpty() }
                if (sections.isEmpty()) EmptyState(stringResource(R.string.no_results)) else LazyColumn(Modifier.fillMaxSize(), state = list) {
                    var n = 0
                    sections.forEachIndexed { k, section ->
                        val places = section.rows.filter { it.kind != Kind.SONG }.map { it.item() }
                        val songs = section.rows.filter { it.kind == Kind.SONG }
                        if (places.isNotEmpty()) {
                            item(key = "c:$k:${section.title}") {
                                Carousel(section.title, places, places[0].kind.name.lowercase() + "s") { _, item ->
                                    took()
                                    openItem(item, emptyList())
                                }
                            }
                        }
                        if (songs.isEmpty()) return@forEachIndexed
                        item(key = "rule:$k:${section.title}") { SectionRule(section.title) }
                        val first = n
                        items(songs.withIndex().toList(), key = { (i, t) -> "$k:$i:${t.path}" }) { (i, track) ->
                            val n = first + i
                            TrackRow(
                                track, playing = snap?.track?.path == track.path, Modifier.testTag("row_$n"),
                                onLongClick = { Nav.sheet = Sheet.Song(track) },
                                trailing = { IconBtn(R.drawable.ic_more, stringResource(R.string.more), Modifier.testTag("row_more_$n")) { Nav.sheet = Sheet.Song(track) } },
                            ) {
                                took()
                                Remote.play(songs, i)
                            }
                        }
                        n += songs.size
                    }
                }
            }
        }
    }
}

/**
 * Buscar with no query (§3.3): Recientes, then the genre grids by the
 * core's groups in arrival order, the moods after the first group.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Browse(state: LazyListState, recent: List<String>, onRecent: (String) -> Unit, onClear: () -> Unit) {
    val (genres, retry) = rememberLoad("genres:false") { Remote.genres(false) }
    val (moods, _) = rememberLoad("genres:true") { Remote.genres(true) }
    val g = (genres as? Load.Done)?.value?.first.orEmpty().groupBy { it.section }.entries.toList()
    val m = (moods as? Load.Done)?.value?.first.orEmpty().groupBy { it.section }.entries.toList()
    // The moods wait for the genres: a group inserted above the first one on show would leave the list scrolled down.
    val groups = if (g.isEmpty()) emptyList() else g.take(1) + m + g.drop(1)
    LazyColumn(Modifier.fillMaxSize(), state = state) {
        if (recent.isNotEmpty()) {
            item(key = "recent_rule") {
                SectionRule(stringResource(R.string.recent)) { BracketButton(stringResource(R.string.clear_recent), Modifier.testTag("recent_clear"), onClick = onClear) }
            }
            item(key = "recent") {
                FlowRow(Modifier.padding(horizontal = 12.dp)) {
                    recent.forEachIndexed { i, q -> BracketChip(q, false, Modifier.testTag("recent_$i")) { onRecent(q) } }
                }
            }
        }
        var n = 0
        for ((group, items) in groups) {
            item(key = "group:$group") { SectionRule(group) }
            val first = n
            items(items.chunked(2).withIndex().toList(), key = { (i, _) -> "$group:$i" }) { (i, pair) ->
                Row(Modifier.padding(horizontal = 16.dp, vertical = 4.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    pair.forEachIndexed { j, genre -> GenreTile(genre, i * 2 + j, Modifier.weight(1f).testTag("genre_${first + i * 2 + j}")) }
                    if (pair.size == 1) Spacer(Modifier.weight(1f))
                }
            }
            n += items.size
        }
        if (genres !is Load.Done) item(key = "genres_state") { LoadState(genres, retry) }
    }
}

/** A 64 dp genre tile: surface fill, a 4 dp bar cycling green, yellow, red and accent, ★ when pinned. */
@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun GenreTile(genre: Item, i: Int, modifier: Modifier = Modifier) {
    val p = LocalPalette.current
    val haptic = LocalHapticFeedback.current
    val bar = listOf(p.green, p.yellow, p.red, p.accent)[i % 4]
    val favorites by Remote.favorites.collectAsStateWithLifecycle()
    Row(
        modifier.height(IntrinsicSize.Min).heightIn(min = 64.dp).background(p.surface).combinedClickable(
            onLongClick = {
                haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                Nav.sheet = Sheet.Of(genre)
            },
            onClick = { Nav.open(Screen.Genre(genre.id, genre.title)) },
        ),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.width(4.dp).fillMaxHeight().background(bar))
        Text(genre.title, Modifier.weight(1f).padding(horizontal = 12.dp), color = p.textPrimary, style = Type.body, maxLines = 2, overflow = TextOverflow.Ellipsis)
        if (favorites[genre.id] ?: genre.favorite) Icon(painterResource(R.drawable.ic_star), null, Modifier.padding(end = 8.dp).size(18.dp), tint = p.accent)
    }
}

private const val RECENT = "recent_searches"
