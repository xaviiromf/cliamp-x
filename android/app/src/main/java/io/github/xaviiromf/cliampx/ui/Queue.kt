package io.github.xaviiromf.cliampx.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material3.Icon
import androidx.compose.material3.SwipeToDismissBox
import androidx.compose.material3.SwipeToDismissBoxState
import androidx.compose.material3.SwipeToDismissBoxValue
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.CustomAccessibilityAction
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.customActions
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.zIndex
import io.github.xaviiromf.cliampx.Paging
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Snapshot
import io.github.xaviiromf.cliampx.Track
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import org.json.JSONObject

/** A queue row, keyed by its place when loaded, so a drag keeps it under the finger. */
@Immutable
private data class QRow(val key: Int, val track: Track)

/** One section's rows, the playlist_revision they were read at, and the row being dragged in it. */
@Stable
private class Rows {
    var list by mutableStateOf(emptyList<QRow>())
    var revision = 0L
    var dragging by mutableStateOf<Int?>(null)
    var offset by mutableFloatStateOf(0f)
    var from = 0

    fun at(key: Int) = list.indexOfFirst { it.key == key }

    fun swap(i: Int, j: Int) {
        list = list.toMutableList().also { it[i] = it[j].also { _ -> it[j] = it[i] } }
    }
}

/**
 * The queue page (§2.7, §3.11): Sonando, A continuación (play-next) and
 * Siguiente (the play order after the song, shuffle aware). A tap plays a
 * row, a swipe to the start removes it, the ≡ handle reorders.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun QueuePage(s: Snapshot, back: Boolean = true) {
    val p = LocalPalette.current
    val next = remember { Rows() }
    val up = remember { Rows() }
    var upTotal by remember { mutableIntStateOf(0) }
    var loaded by remember { mutableStateOf(false) }
    var failed by remember { mutableStateOf<String?>(null) }
    var reload by remember { mutableIntStateOf(0) }
    // A remove or a move in flight: the refetch waits for it, so the rows do not jump back and forth.
    var busy by remember { mutableIntStateOf(0) }
    val list = rememberLazyListState()
    val scope = rememberCoroutineScope()
    val rowPx = with(LocalDensity.current) { 64.dp.toPx() }

    LaunchedEffect(s.playlistRevision, s.playNextTotal, reload, busy) {
        if (busy > 0) return@LaunchedEffect
        delay(100)
        try {
            val n = Remote.playNext()
            val first = Remote.upNext(0)
            val rows = first.rows.toMutableList()
            // Reload as many rows as were shown, so the scroll position stays; a page of a newer queue waits for its refetch.
            while (rows.size < up.list.size) {
                val page = Remote.upNext(Paging.next(rows.size, first.total) ?: break)
                if (page.rows.isEmpty() || page.revision != first.revision) break
                rows += page.rows
            }
            next.list = n.rows.mapIndexed(::QRow)
            next.revision = n.revision
            up.list = rows.mapIndexed(::QRow)
            up.revision = first.revision
            upTotal = first.total
            failed = null
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            failed = Remote.message(e)
        }
        loaded = true
    }
    // The next page of Siguiente when the last visible row is within 20 of the loaded end (§5.5).
    LaunchedEffect(list, upTotal) {
        snapshotFlow { (list.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0) to list.layoutInfo.totalItemsCount }.collect { (last, count) ->
            val offset = Paging.next(up.list.size, upTotal) ?: return@collect
            if (last < count - 20 || busy > 0) return@collect
            runCatching { Remote.upNext(offset) }.onSuccess { page ->
                if (page.revision == up.revision) up.list += page.rows.mapIndexed { i, t -> QRow(offset + i, t) }
            }
        }
    }

    /**
     * Runs a queue edit and refetches when it fails. Edits pass the revision
     * their rows were read at, so a row that moved since gets a conflict,
     * which only refetches (§5.7), instead of editing another song.
     */
    fun edit(block: suspend () -> Unit) {
        busy++
        scope.launch {
            try {
                block()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                Remote.showError(e)
                reload++
            } finally {
                busy--
            }
        }
    }

    fun play(rows: Rows, r: QRow) = edit {
        Remote.call("queue.play", JSONObject().put("index", r.track.index).put("if_revision", rows.revision))
    }

    Column(Modifier.fillMaxSize()) {
        Row(Modifier.fillMaxWidth().heightIn(min = 56.dp), verticalAlignment = Alignment.CenterVertically) {
            // Beside the player (two panes) the queue is where back leads, so it has none.
            if (back) {
                IconBtn(R.drawable.ic_back, stringResource(R.string.back), Modifier.testTag("queue_back")) { Nav.page = PlayerPage.Player }
            } else {
                Spacer(Modifier.width(16.dp))
            }
            Text(
                "▸─ " + stringResource(R.string.queue) + " " + "─".repeat(80), Modifier.weight(1f).ruleSemantics(stringResource(R.string.queue)),
                color = p.accent, style = Type.headline, maxLines = 1, softWrap = false, overflow = TextOverflow.Clip,
            )
            IconBtn(R.drawable.ic_more, stringResource(R.string.more), Modifier.testTag("queue_more")) { Nav.sheet = Sheet.QueueMore }
        }
        FlowRow(Modifier.padding(horizontal = 12.dp)) {
            BracketChip(stringResource(R.string.shuffle), s.shuffle, Modifier.testTag("queue_shuffle")) {
                Remote.fire("shuffle", JSONObject().put("name", "toggle"))
            }
            BracketChip(stringResource(R.string.repeat, repeatLabel(s.repeat)), s.repeat == "All" || s.repeat == "One", Modifier.testTag("queue_repeat")) {
                Remote.fire("repeat", JSONObject().put("name", "cycle"))
            }
        }
        failed?.let { msg -> ErrorState(msg) { reload++ } }
        LazyColumn(Modifier.fillMaxSize(), state = list) {
            item(key = "rule_now") { SectionRule(stringResource(R.string.queue_now)) }
            s.track?.let { now -> item(key = "now") { TrackRow(now, playing = true, Modifier.testTag("row_now")) {} } }
            if (next.list.isNotEmpty()) {
                item(key = "rule_next") {
                    SectionRule(stringResource(R.string.queue_play_next, next.list.size)) {
                        BracketButton(stringResource(R.string.clear), Modifier.testTag("next_clear")) { edit { Remote.call("playnext.clear") } }
                    }
                }
                rows(next, "n", rowPx, handle = true, tag = { "next_$it" }, play = { play(next, it) },
                    remove = { i, _ -> edit { Remote.call("playnext.remove", JSONObject().put("index", i)) } },
                    move = { from, to -> edit { Remote.move("playnext.move", from, to, null) } },
                    sheet = { i, r -> Sheet.Song(r.track, next = i) },
                )
            }
            item(key = "rule_up") { SectionRule(stringResource(R.string.queue_up_next, upTotal)) }
            // The core refuses moves while shuffle is on.
            rows(up, "u", rowPx, handle = !s.shuffle, tag = { "row_$it" }, play = { play(up, it) },
                remove = { _, r ->
                    edit { Remote.call("queue.remove", JSONObject().put("index", r.track.index).put("if_revision", up.revision)) }
                },
                move = { from, to ->
                    // Siguiente is the live order after the song when shuffle is off: positions map to consecutive live indexes.
                    val base = up.list.minOf { it.track.index }
                    edit { Remote.move("queue.move", base + from, base + to, up.revision) }
                },
                sheet = { _, r -> Sheet.Song(r.track, index = r.track.index, revision = up.revision) },
            )
            if (!loaded) item(key = "loading") { Loading() }
            if (loaded && next.list.isEmpty() && up.list.isEmpty()) item(key = "empty") { EmptyState(stringResource(R.string.queue_empty)) }
        }
    }
}

/** A section's rows: tap plays, swipe to the start removes, ≡ drags. */
private fun LazyListScope.rows(
    rows: Rows, prefix: String, rowPx: Float, handle: Boolean, tag: (Int) -> String, play: (QRow) -> Unit,
    remove: (Int, QRow) -> Unit, move: (Int, Int) -> Unit, sheet: (Int, QRow) -> Sheet,
) {
    itemsIndexed(rows.list, key = { _, r -> prefix + r.key }) { i, r ->
        val p = LocalPalette.current
        // Not saveable: a refetch reuses keys, and a restored "dismissed" state would remove the row that takes the key.
        val dismiss = remember { SwipeToDismissBoxState(SwipeToDismissBoxValue.Settled, positionalThreshold = { it * 0.4f }) }
        val dragged = rows.dragging == r.key
        // TalkBack has no swipe or drag: remove and the moves are custom actions of the row.
        val actions = buildList {
            add(CustomAccessibilityAction(stringResource(R.string.remove_from_queue)) {
                val at = rows.at(r.key)
                rows.list = rows.list.filterNot { row -> row.key == r.key }
                remove(at, r)
                true
            })
            if (handle) {
                val at = rows.at(r.key)
                if (at > 0) add(CustomAccessibilityAction(stringResource(R.string.move_up)) { rows.swap(at, at - 1); move(at, at - 1); true })
                if (at in 0 until rows.list.lastIndex) {
                    add(CustomAccessibilityAction(stringResource(R.string.move_down)) { rows.swap(at, at + 1); move(at, at + 1); true })
                }
            }
        }
        SwipeToDismissBox(
            dismiss,
            backgroundContent = { SwipeBehind(dismiss, stringResource(R.string.remove_from_queue)) },
            modifier = Modifier.zIndex(if (dragged) 1f else 0f).graphicsLayer { translationY = if (dragged) rows.offset else 0f },
            enableDismissFromStartToEnd = false,
            gesturesEnabled = rows.dragging == null,
            onDismiss = {
                if (it == SwipeToDismissBoxValue.EndToStart) {
                    val at = rows.at(r.key)
                    rows.list = rows.list.filterNot { row -> row.key == r.key }
                    remove(at, r)
                }
            },
        ) {
            TrackRow(
                // The dragged row covers the ones it passes.
                r.track, playing = false,
                Modifier.then(if (dragged) Modifier.background(p.bg) else Modifier).testTag(tag(i)).semantics { customActions = actions },
                marker = if (prefix == "n") "Q${i + 1}" else "",
                onLongClick = { Nav.sheet = sheet(i, r) },
                trailing = if (!handle) null else {
                    { DragHandle(rows, r.key, rowPx, move) }
                },
                titleLines = 1,
            ) { play(r) }
        }
    }
}

/** The ≡ handle: the row follows the finger, swapping with a neighbor past half a row; the drop sends the move. */
@Composable
private fun DragHandle(rows: Rows, key: Int, rowPx: Float, move: (Int, Int) -> Unit) {
    val p = LocalPalette.current
    Box(
        // Hidden from TalkBack: the row's Move up and Move down actions do this.
        Modifier.size(48.dp).clearAndSetSemantics {}.pointerInput(key) {
            fun drop() {
                val to = rows.at(key)
                rows.dragging = null
                rows.offset = 0f
                if (to != rows.from && to >= 0) move(rows.from, to)
            }
            detectDragGestures(
                onDragStart = {
                    rows.dragging = key
                    rows.offset = 0f
                    rows.from = rows.at(key)
                },
                onDragEnd = ::drop,
                onDragCancel = ::drop,
            ) { change, amount ->
                change.consume()
                rows.offset += amount.y
                val i = rows.at(key)
                if (rows.offset > rowPx / 2 && i < rows.list.lastIndex) {
                    rows.swap(i, i + 1)
                    rows.offset -= rowPx
                } else if (rows.offset < -rowPx / 2 && i > 0) {
                    rows.swap(i, i - 1)
                    rows.offset += rowPx
                }
            }
        },
        contentAlignment = Alignment.Center,
    ) { Icon(painterResource(R.drawable.ic_drag), null, Modifier.size(24.dp), tint = p.textSecondary) }
}
