package io.github.xaviiromf.cliampx.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
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
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.Kind
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Section
import io.github.xaviiromf.cliampx.kind
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
 * Buscar (UI-SPEC §3.3, §3.4): the field, then the songs grouped by the
 * core's sections. A tap plays the section from that song. shortcut: the
 * artist, album and playlist results, the history and the genres come in M3.
 */
@Composable
fun SearchScreen() {
    val p = LocalPalette.current
    val keyboard = LocalSoftwareKeyboardController.current
    var query by rememberSaveable { mutableStateOf("") }
    var submitted by rememberSaveable { mutableStateOf("") }
    var attempt by remember { mutableIntStateOf(0) }
    // The query the list shows: only a new one scrolls back to the top.
    var shown by rememberSaveable { mutableStateOf("") }
    // Back from another tab, the last rows show at once, at their scroll position, while they refresh.
    var results by remember { mutableStateOf<Results>(Remote.lastSearch(submitted)?.let(::songs) ?: Results.None) }
    val list = rememberLazyListState()
    val snap by Remote.state.collectAsStateWithLifecycle()

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
            songs(Remote.search(submitted))
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
    LaunchedEffect(Unit) { Nav.reselected.collect { if (it == Tab.Search) list.animateScrollToItem(0) } }

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
            keyboardActions = KeyboardActions(onSearch = {
                submitted = query.trim()
                keyboard?.hide()
            }),
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
        when (val r = results) {
            Results.None -> {}
            Results.Loading -> Loading()
            is Results.Failed -> ErrorState(r.message) { attempt++ }
            is Results.Done -> if (r.sections.isEmpty()) {
                EmptyState(stringResource(R.string.no_results))
            } else {
                LazyColumn(Modifier.fillMaxSize(), state = list) {
                    var n = 0
                    for (section in r.sections) {
                        item(key = "rule:" + section.title) { SectionRule(section.title) }
                        val first = n
                        items(section.rows.withIndex().toList(), key = { (i, t) -> "${section.title}:$i:${t.path}" }) { (i, track) ->
                            TrackRow(track, playing = snap?.track?.path == track.path, Modifier.testTag("row_${first + i}")) {
                                Remote.play(section.rows, i)
                            }
                        }
                        n += section.rows.size
                    }
                }
            }
        }
    }
}

/** The song rows of each section. shortcut: songs only until M3 adds the other kinds. */
private fun songs(sections: List<Section>) =
    Results.Done(sections.map { s -> s.copy(rows = s.rows.filter { it.kind == Kind.SONG }) }.filter { it.rows.isNotEmpty() })
