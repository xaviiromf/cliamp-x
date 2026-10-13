package io.github.xaviiromf.cliampx.ui

import android.content.Intent
import androidx.annotation.DrawableRes
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.testTagsAsResourceId
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Snapshot
import io.github.xaviiromf.cliampx.TerminalActivity
import io.github.xaviiromf.cliampx.Track
import kotlinx.coroutines.launch
import org.json.JSONObject
import kotlin.math.abs

/** The bottom sheet of Nav.sheet over everything, the player sheet included. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SheetHost(s: Snapshot?) {
    val sheet = Nav.sheet ?: return
    val p = LocalPalette.current
    val state = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val scope = rememberCoroutineScope()
    val close: () -> Unit = { scope.launch { state.hide() }.invokeOnCompletion { Nav.sheet = null } }
    ModalBottomSheet(
        onDismissRequest = { Nav.sheet = null },
        sheetState = state,
        shape = RectangleShape,
        containerColor = p.surface,
        contentColor = p.textPrimary,
        tonalElevation = 0.dp,
        scrimColor = SCRIM,
        dragHandle = null,
        contentWindowInsets = { WindowInsets(0) },
    ) {
        Column(Modifier.fillMaxWidth().navigationBarsPadding().padding(vertical = 8.dp).semantics { testTagsAsResourceId = true }) {
            when (sheet) {
                is Sheet.Song -> SongActions(sheet, close)
                Sheet.PlayerMore -> PlayerMore(s, close)
                Sheet.Speed -> SpeedSheet(s, close)
                Sheet.Sleep -> SleepSheet(s, close)
                Sheet.ClearQueue -> ClearQueue(close)
            }
        }
    }
}

/** A 56 dp action row: a one-cell glyph (text or vector) and the label. */
@Composable
private fun MenuRow(glyph: String, label: String, tag: String, glyphColor: Color = LocalPalette.current.accent, @DrawableRes icon: Int? = null, onClick: () -> Unit) {
    val p = LocalPalette.current
    Row(
        Modifier.fillMaxWidth().height(56.dp).clickable(onClick = onClick).padding(horizontal = 16.dp).testTag(tag),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.width(32.dp)) {
            if (icon != null) Icon(painterResource(icon), null, Modifier.size(20.dp), tint = glyphColor) else Text(glyph, color = glyphColor, style = Type.body)
        }
        Text(label, color = p.textPrimary, style = Type.body)
    }
}

/** art, title and subtitle above a song's actions. */
@Composable
private fun SongHeader(track: Track) {
    val p = LocalPalette.current
    Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
        ArtBox(track.art, 48.dp)
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text(track.name, color = p.textPrimary, style = Type.body, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(
                listOf(track.artist, track.album).filter { it.isNotEmpty() }.joinToString(" · "),
                color = p.textSecondary, style = Type.label, maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
        }
    }
    Box(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp).height(1.dp).background(p.divider))
}

@Composable
private fun LikeRow(track: Track, close: () -> Unit) {
    val p = LocalPalette.current
    MenuRow(
        "", stringResource(if (track.favorite) R.string.unlike else R.string.like), "menu_like", p.favorite,
        icon = if (track.favorite) R.drawable.ic_heart_outline else R.drawable.ic_heart,
    ) {
        Remote.favorite(track)
        close()
    }
}

/**
 * A song's actions (§2.6). shortcut: only the ones whose screens exist in
 * M2 (play next, ♥ and the queue's remove); M3 adds the rest.
 */
@Composable
private fun SongActions(sheet: Sheet.Song, close: () -> Unit) {
    val track = sheet.track
    SongHeader(track)
    MenuRow("▶", stringResource(R.string.play_next), "menu_play_next") {
        Remote.fire("track.queue", JSONObject().put("track", track.json()))
        close()
    }
    LikeRow(track, close)
    if (sheet.index != null || sheet.next != null) {
        MenuRow("✗", stringResource(R.string.remove_from_queue), "menu_remove", LocalPalette.current.error) {
            if (sheet.next != null) {
                Remote.fire("playnext.remove", JSONObject().put("index", sheet.next))
            } else {
                Remote.fire("queue.remove", JSONObject().put("index", sheet.index).put("if_revision", Remote.state.value?.playlistRevision ?: 0))
            }
            close()
        }
    }
}

/** The player's ⋮: the song's ♥, then Velocidad…, Temporizador… and Modo terminal. */
@Composable
private fun PlayerMore(s: Snapshot?, close: () -> Unit) {
    val context = LocalContext.current
    s?.track?.let {
        SongHeader(it)
        LikeRow(it, close)
    }
    MenuRow("×", stringResource(R.string.speed_more), "menu_speed") { Nav.sheet = Sheet.Speed }
    MenuRow("", stringResource(R.string.sleep_more), "menu_sleep", icon = R.drawable.ic_timer) { Nav.sheet = Sheet.Sleep }
    MenuRow("", stringResource(R.string.terminal), "menu_terminal", icon = R.drawable.ic_terminal) {
        close()
        context.startActivity(Intent(context, TerminalActivity::class.java))
    }
}

/** The speed choices (§3.15). */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun SpeedSheet(s: Snapshot?, close: () -> Unit) {
    SectionRule(stringResource(R.string.speed))
    FlowRow(Modifier.padding(horizontal = 12.dp)) {
        for (v in listOf(0.5, 0.75, 1.0, 1.25, 1.5, 2.0)) {
            val label = (if (v % 1 == 0.0) v.toInt().toString() else v.toString()) + "x"
            BracketChip(label, s != null && abs(s.speed - v) < 0.01, Modifier.testTag("speed_$label")) {
                Remote.fire("speed", JSONObject().put("value", v))
                close()
            }
        }
    }
}

/** The sleep timer choices (§3.15) and when it pauses. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun SleepSheet(s: Snapshot?, close: () -> Unit) {
    val p = LocalPalette.current
    val minutes = s?.sleepMinutes ?: 0
    SectionRule(stringResource(R.string.sleep))
    FlowRow(Modifier.padding(horizontal = 12.dp)) {
        fun set(value: Int) {
            Remote.fire("sleep", JSONObject().put("value", value))
            close()
        }
        for (m in listOf(15, 30, 45, 60, 90)) {
            BracketChip(stringResource(R.string.sleep_minutes, m), minutes == m, Modifier.testTag("sleep_$m")) { set(m) }
        }
        BracketChip(stringResource(R.string.sleep_end_song), minutes == -1, Modifier.testTag("sleep_end")) { set(-1) }
        BracketChip(stringResource(R.string.sleep_off), false, Modifier.testTag("sleep_off")) { set(0) }
    }
    val active = when {
        minutes == -1 -> stringResource(R.string.sleep_at_end)
        else -> s?.let(::sleepClock)?.let { stringResource(R.string.sleep_at, it) }
    }
    if (active != null) Text(active, Modifier.padding(horizontal = 16.dp, vertical = 8.dp), color = p.textSecondary, style = Type.label)
}

/** ¿Vaciar la cola? with Vaciar in the error color. */
@Composable
private fun ClearQueue(close: () -> Unit) {
    val p = LocalPalette.current
    Text(stringResource(R.string.clear_queue_confirm), Modifier.padding(16.dp), color = p.textPrimary, style = Type.body)
    Row(Modifier.padding(horizontal = 12.dp), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
        BracketButton(stringResource(R.string.clear), Modifier.testTag("confirm"), color = p.error) {
            Remote.fire("queue.clear")
            close()
        }
        BracketButton(stringResource(R.string.cancel), color = p.accent, onClick = close)
    }
}
