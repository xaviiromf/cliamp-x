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
import androidx.compose.foundation.layout.heightIn
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
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.withStyle
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.Item
import io.github.xaviiromf.cliampx.Kind
import io.github.xaviiromf.cliampx.Library
import io.github.xaviiromf.cliampx.likes
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Snapshot
import io.github.xaviiromf.cliampx.TerminalActivity
import io.github.xaviiromf.cliampx.Track
import io.github.xaviiromf.cliampx.speedLabel
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
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
        SheetBars()
        Column(
            Modifier.fillMaxWidth().navigationBarsPadding().imePadding().verticalScroll(rememberScrollState())
                .padding(vertical = 8.dp).semantics { testTagsAsResourceId = true },
        ) {
            when (sheet) {
                is Sheet.Song -> SongActions(sheet, close)
                is Sheet.Of -> ItemActions(sheet.item, close)
                is Sheet.AddTo -> AddTo(sheet)
                is Sheet.NewList -> NewList(sheet, close)
                is Sheet.Rename -> Rename(sheet.item, close)
                is Sheet.DeleteList -> Confirm(stringResource(R.string.delete_list_confirm, sheet.item.title), stringResource(R.string.delete), close) {
                    Remote.editList("playlist.delete", JSONObject().put("playlist", sheet.item.id)) {
                        // The list's page goes with it.
                        if ((Nav.top.screen as? Screen.Playlist)?.let { it.provider == "local" && it.id == sheet.item.id } == true) Nav.pop()
                    }
                }
                Sheet.PlayerMore -> PlayerMore(s, close)
                Sheet.QueueMore -> QueueMore()
                Sheet.Speed -> SpeedSheet(s, close)
                Sheet.Sleep -> SleepSheet(s, close)
                Sheet.Normalize -> NormalizeSheet(s, close)
                Sheet.Crossfade -> CrossfadeSheet(s, close)
                Sheet.ClearQueue -> Confirm(stringResource(R.string.clear_queue_confirm), stringResource(R.string.clear), close) { Remote.fire("queue.clear") }
            }
        }
    }
}

/** An action row, 56 dp or taller as the label wraps: a one-cell glyph (text or vector) and the label. */
@Composable
private fun MenuRow(glyph: String, label: String, tag: String, glyphColor: Color = LocalPalette.current.accent, @DrawableRes icon: Int? = null, onClick: () -> Unit) {
    val p = LocalPalette.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = 56.dp).clickable(onClick = onClick).padding(horizontal = 16.dp).testTag(tag),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.width(32.dp)) {
            if (icon != null) Icon(painterResource(icon), null, Modifier.size(20.dp), tint = glyphColor) else Text(glyph, color = glyphColor, style = Type.body)
        }
        Text(label, color = p.textPrimary, style = Type.body)
    }
}

/** art 48 dp, title and subtitle above an item's actions (§2.6). */
@Composable
private fun SheetHeader(art: String, title: String, subtitle: String) {
    val p = LocalPalette.current
    Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
        ArtBox(art, 48.dp)
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text(title, color = p.textPrimary, style = Type.body, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (subtitle.isNotEmpty()) Text(subtitle, color = p.textSecondary, style = Type.label, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
    Box(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp).height(1.dp).background(p.divider))
}

@Composable
private fun SongHeader(track: Track) = SheetHeader(track.art, track.name, listOf(track.artist, track.album).filter { it.isNotEmpty() }.joinToString(" · "))

/** Me gusta or Quitar de Me gusta, from the ♥ the core last reported for the song (§5.3). */
@Composable
private fun LikeRow(track: Track, close: () -> Unit) {
    val p = LocalPalette.current
    val liked by Remote.liked.collectAsStateWithLifecycle()
    val on = liked.likes(track)
    MenuRow(
        "", stringResource(if (on) R.string.unlike else R.string.like), "menu_like", p.favorite,
        icon = if (on) R.drawable.ic_heart_outline else R.drawable.ic_heart,
    ) {
        Remote.favorite(track)
        close()
    }
}

/** Me interesa and No me interesa (G11); the second drops the item from the lists at once. */
@Composable
private fun RateRows(item: Item, close: () -> Unit) {
    MenuRow("✓", stringResource(R.string.interested), "menu_interested") {
        Remote.rate(item, 1)
        close()
    }
    MenuRow("✗", stringResource(R.string.not_interested), "menu_not_interested", LocalPalette.current.error) {
        Remote.rate(item, -1)
        close()
    }
}

/** A song's actions (§2.6), each only when its operation exists; the queue's rows add Quitar de la cola. */
@Composable
private fun SongActions(sheet: Sheet.Song, close: () -> Unit) {
    val track = sheet.track
    val ops by Remote.ops.collectAsStateWithLifecycle()
    SongHeader(track)
    if ("track.queue" in ops) {
        MenuRow("▶", stringResource(R.string.play_next), "menu_play_next") {
            Remote.fire("track.queue", JSONObject().put("track", track.json()))
            close()
        }
    }
    if ("playlist.add" in ops) MenuRow("+", stringResource(R.string.add_to_list_more), "menu_add") { Nav.sheet = Sheet.AddTo(listOf(track)) }
    LikeRow(track, close)
    if ("radio.start" in ops) {
        MenuRow("◉", stringResource(R.string.start_radio), "menu_radio") {
            Remote.fire("radio.start", JSONObject().put("track", track.json()))
            close()
        }
    }
    if ("discover.remixes" in ops) MenuRow("≈", stringResource(R.string.remixes), "menu_remixes") { Nav.open(Screen.Remixes(track)) }
    track.meta["ytmusic.album"]?.let { id ->
        MenuRow("◎", stringResource(R.string.go_album), "menu_album") { Nav.open(Screen.Album(id, track.album, track.artUrl, track.artist)) }
    }
    track.meta["ytmusic.artist"]?.let { id ->
        MenuRow("→", stringResource(R.string.go_artist), "menu_artist") { Nav.open(Screen.Artist(id, track.artist.substringBefore(", "))) }
    }
    if ("download" in ops) {
        MenuRow("↓", stringResource(R.string.download), "menu_download") {
            Remote.fire("download", JSONObject().put("track", track.json()))
            close()
        }
    }
    if ("taste.rate" in ops) RateRows(Item(Kind.SONG, track.path, track.name, track.artist, track.art, track), close)
    if (sheet.list != null && "playlist.remove" in ops) {
        MenuRow("✗", stringResource(R.string.remove_from_list), "menu_remove_list", LocalPalette.current.error) {
            Remote.editList("playlist.remove", JSONObject().put("playlist", sheet.list).put("index", sheet.row))
            close()
        }
    }
    if (sheet.index != null || sheet.next != null) {
        MenuRow("✗", stringResource(R.string.remove_from_queue), "menu_remove", LocalPalette.current.error) {
            if (sheet.next != null) {
                Remote.fire("playnext.remove", JSONObject().put("index", sheet.next))
            } else {
                Remote.fire("queue.remove", JSONObject().put("index", sheet.index).put("if_revision", sheet.revision))
            }
            close()
        }
    }
}

/**
 * The actions of an album, artist, playlist or genre (§2.6), each only
 * when its operation exists, its IDs too, and, for the account's own,
 * while signed in. A save or follow shows once its state is known.
 */
@Composable
private fun ItemActions(item: Item, close: () -> Unit) {
    val ops by Remote.ops.collectAsStateWithLifecycle()
    val signedIn by Remote.signedIn.collectAsStateWithLifecycle()
    val favorites by Remote.favorites.collectAsStateWithLifecycle()
    val yt = !item.local
    val p = LocalPalette.current
    SheetHeader(item.art, item.title, item.subtitle)
    fun run(action: () -> Unit) {
        action()
        close()
    }
    val play = stringResource(R.string.play)
    val shuffle = stringResource(R.string.shuffle)
    val radio = stringResource(R.string.start_radio)
    val radioParams = JSONObject().put("provider", item.provider).put("playlist", item.id).put("name", item.title)
    when (item.kind) {
        Kind.ALBUM, Kind.PLAYLIST -> {
            MenuRow("▶", play, "menu_play") { run { Remote.playItem(item) } }
            MenuRow("", shuffle, "menu_shuffle", icon = R.drawable.ic_shuffle) { run { Remote.playItem(item, shuffle = true) } }
            if ("track.queue" in ops) MenuRow("▶", stringResource(R.string.play_next), "menu_play_next") { run { Remote.queueItem(item) } }
            if (item.kind == Kind.ALBUM && yt && signedIn == true && "provider.favorite" in ops) {
                val pid = item.id.removePrefix("al:")
                val (preview, _) = rememberLoad("preview:$pid") { Remote.preview(pid) }
                (favorites[pid] ?: (preview as? Load.Done)?.value?.favorite)?.let { saved ->
                    MenuRow("", stringResource(if (saved) R.string.unsave else R.string.save), "menu_save", p.favorite, icon = if (saved) R.drawable.ic_heart_outline else R.drawable.ic_heart) {
                        run { Remote.toggle("provider.favorite", pid) }
                    }
                }
            }
            if (item.kind == Kind.ALBUM) {
                MenuRow("→", stringResource(R.string.go_artist), "menu_artist") {
                    // The artist is known from the album's songs when the row did not carry it.
                    Remote.scope.launch {
                        val id = item.artistId.ifEmpty { runCatching { Remote.tracksOf(item) }.getOrNull()?.firstNotNullOfOrNull { it.meta["ytmusic.artist"] }.orEmpty() }
                        if (id.isNotEmpty()) withContext(Dispatchers.Main) { Nav.open(Screen.Artist(id, item.subtitle.substringBefore(" · ").substringBefore(", "))) }
                    }
                }
            }
            if (yt && "radio.start" in ops) MenuRow("◉", radio, "menu_radio") { run { Remote.fire("radio.start", radioParams) } }
            if (item.kind == Kind.ALBUM && "playlist.add" in ops) MenuRow("+", stringResource(R.string.add_to_list_more), "menu_add") { Nav.sheet = Sheet.AddTo(emptyList(), item) }
            if (yt && "download" in ops) {
                MenuRow("↓", stringResource(R.string.download), "menu_download") {
                    run { Remote.fire("download", JSONObject().put("provider", "ytmusic").put("playlist", item.id).put("name", item.title)) }
                }
            }
            if (item.kind == Kind.ALBUM && "taste.rate" in ops) RateRows(item, close)
            if (Library.editable(item)) {
                if ("playlist.rename" in ops) MenuRow(">", stringResource(R.string.rename_more), "menu_rename") { Nav.sheet = Sheet.Rename(item) }
                if ("playlist.delete" in ops) MenuRow("✗", stringResource(R.string.delete), "menu_delete", p.error) { Nav.sheet = Sheet.DeleteList(item) }
            }
        }
        Kind.ARTIST -> {
            if (yt && signedIn == true && "provider.favorite" in ops) {
                val (preview, _) = rememberLoad("preview:${item.id}") { Remote.preview(item.id) }
                (favorites[item.id] ?: (preview as? Load.Done)?.value?.favorite)?.let { following ->
                    MenuRow("", stringResource(if (following) R.string.unfollow else R.string.follow), "menu_follow", icon = R.drawable.ic_heart) {
                        run { Remote.toggle("provider.favorite", item.id) }
                    }
                }
            }
            if (yt && "radio.start" in ops) MenuRow("◉", radio, "menu_radio") { run { Remote.fire("radio.start", radioParams) } }
            if ("taste.rate" in ops) RateRows(item, close)
        }
        Kind.GENRE -> {
            MenuRow("▶", stringResource(R.string.play_mix), "menu_play") { run { Remote.playItem(item) } }
            if (yt && "provider.genre_favorite" in ops) {
                MenuRow("", stringResource(if (favorites[item.id] ?: item.favorite) R.string.unpin else R.string.pin), "menu_pin", icon = R.drawable.ic_star) {
                    run { Remote.toggle("provider.genre_favorite", item.id) }
                }
            }
            // Radio takes the category IDs only, not search: or local: genres.
            if (yt && !item.id.startsWith("search:") && "radio.start" in ops) {
                MenuRow("◉", radio, "menu_radio") { run { Remote.fire("radio.start", radioParams.put("playlist", "genre:" + item.id)) } }
            }
            if ("taste.rate" in ops) RateRows(item, close)
        }
        Kind.SONG -> {}
    }
}

/**
 * Añadir a una lista… (§2.6, §3.12): a new list, the account's lists that
 * take songs (signed in), and the local lists without the virtual ones.
 */
@Composable
private fun AddTo(sheet: Sheet.AddTo) {
    val p = LocalPalette.current
    val signedIn by Remote.signedIn.collectAsStateWithLifecycle()
    SectionRule(stringResource(R.string.add_to_list))
    MenuRow("+", stringResource(R.string.new_list_more), "menu_new_list") { Nav.sheet = Sheet.NewList(sheet.tracks, sheet.item) }
    @Composable
    fun lists(title: String, load: Load<List<Item>>, tag: String) {
        val rows = (load as? Load.Done)?.value.orEmpty()
        if (load is Load.Loading) Loading()
        if (rows.isEmpty()) return
        SectionRule(title)
        rows.forEachIndexed { i, row ->
            Row(
                Modifier.fillMaxWidth().heightIn(min = 56.dp).clickable {
                    Remote.addTo(row.provider, row.id, sheet.tracks, sheet.item)
                    Nav.sheet = null
                }.padding(horizontal = 16.dp).testTag("${tag}_$i"),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                ArtBox(row.art, 40.dp)
                Text(row.title, Modifier.weight(1f).padding(horizontal = 12.dp), color = p.textPrimary, style = Type.body, maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (row.count > 0) Text("${row.count}", color = p.textSecondary, style = Type.label)
            }
        }
    }
    if (signedIn == true) {
        val (yt, _) = rememberLoad("playlists:ytmusic") { Remote.playlists("ytmusic") }
        lists(stringResource(R.string.youtube_music), (yt as? Load.Done)?.let { Load.Done(it.value.filter(Item::writable)) } ?: yt, "list_yt")
    }
    val (local, _) = rememberLoad("playlists:local") { Remote.playlists("local") }
    lists(stringResource(R.string.local), (local as? Load.Done)?.let { Load.Done(it.value.filter { row -> row.id !in Library.VIRTUAL }) } ?: local, "list_local")
}

/** Nueva lista (§3.12): the name, the target, Crear and Cancelar. */
@Composable
private fun NewList(sheet: Sheet.NewList, close: () -> Unit) {
    val signedIn by Remote.signedIn.collectAsStateWithLifecycle()
    var target by remember { mutableStateOf(if (signedIn == true) "ytmusic" else "local") }
    NameSheet(stringResource(if (sheet.queue) R.string.save_queue else R.string.new_list), "", stringResource(R.string.create), close, target = {
        if (signedIn == true) {
            Row(Modifier.padding(horizontal = 12.dp)) {
                BracketChip(stringResource(R.string.youtube_music), target == "ytmusic", Modifier.testTag("target_ytmusic")) { target = "ytmusic" }
                BracketChip(stringResource(R.string.local), target == "local", Modifier.testTag("target_local")) { target = "local" }
            }
        }
    }) { name -> Remote.createList(target, name, sheet.tracks, sheet.item, sheet.queue) { Nav.sheet = null } }
}

/** Renombrar… a local list; its page follows the new name. */
@Composable
private fun Rename(item: Item, close: () -> Unit) {
    NameSheet(stringResource(R.string.rename), item.title, stringResource(R.string.rename), close) { name ->
        Remote.editList("playlist.rename", JSONObject().put("playlist", item.id).put("new_name", name)) {
            Nav.sheet = null
            val top = Nav.top.screen as? Screen.Playlist
            if (top != null && top.provider == "local" && top.id == item.id) Nav.replace(Screen.Playlist("local", name, name, top.art))
        }
    }
}

/**
 * The name sheet (§3.12): the only text field besides Buscar, focused at
 * once, then target's chips, the confirm key pill and Cancelar.
 */
@Composable
private fun NameSheet(title: String, initial: String, confirm: String, close: () -> Unit, target: @Composable () -> Unit = {}, done: (String) -> Unit) {
    val p = LocalPalette.current
    var name by remember { mutableStateOf(TextFieldValue(initial, TextRange(initial.length))) }
    val focus = remember { FocusRequester() }
    LaunchedEffect(Unit) { focus.requestFocus() }
    val ok = name.text.isNotBlank() && name.text.trim() != initial
    val submit = { if (ok) done(name.text.trim()) }
    SectionRule(title)
    BasicTextField(
        name, { name = it },
        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp).focusRequester(focus).testTag("name_field"),
        singleLine = true,
        textStyle = Type.body.copy(color = p.textPrimary),
        cursorBrush = SolidColor(p.accent),
        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
        keyboardActions = KeyboardActions(onDone = { submit() }),
        decorationBox = { field ->
            Row(Modifier.height(48.dp).background(p.surfaceHigh).padding(horizontal = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                Text("> ", color = p.accent, style = Type.body)
                Box(Modifier.weight(1f)) {
                    if (name.text.isEmpty()) {
                        Text(
                            buildAnnotatedString {
                                append(stringResource(R.string.name))
                                withStyle(SpanStyle(color = p.accent)) { append("_") }
                            },
                            color = p.textSecondary, style = Type.body,
                        )
                    }
                    field()
                }
            }
        },
    )
    target()
    Row(Modifier.padding(horizontal = 16.dp, vertical = 8.dp), horizontalArrangement = Arrangement.spacedBy(16.dp), verticalAlignment = Alignment.CenterVertically) {
        KeyPill(confirm, Modifier.testTag("name_create"), enabled = ok, onClick = submit)
        BracketButton(stringResource(R.string.cancel), onClick = close)
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
            val label = speedLabel(v)
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

/** The queue's ⋮ (§2.7): Guardar como lista… and Vaciar cola. */
@Composable
private fun QueueMore() {
    val ops by Remote.ops.collectAsStateWithLifecycle()
    if ("playlist.create" in ops) MenuRow("+", stringResource(R.string.save_queue_more), "menu_save_queue") { Nav.sheet = Sheet.NewList(emptyList(), queue = true) }
    MenuRow("✗", stringResource(R.string.clear_queue), "menu_clear_queue", LocalPalette.current.error) { Nav.sheet = Sheet.ClearQueue }
}

/** A confirmation (§3.12): the question, the action in the error color, and Cancelar. */
@Composable
private fun Confirm(question: String, action: String, close: () -> Unit, run: () -> Unit) {
    val p = LocalPalette.current
    Text(question, Modifier.padding(16.dp), color = p.textPrimary, style = Type.body)
    Row(Modifier.padding(horizontal = 12.dp), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
        BracketButton(action, Modifier.testTag("confirm"), color = p.error) {
            run()
            close()
        }
        BracketButton(stringResource(R.string.cancel), color = p.accent, onClick = close)
    }
}
