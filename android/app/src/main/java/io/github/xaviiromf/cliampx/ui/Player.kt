package io.github.xaviiromf.cliampx.ui

import android.content.Intent
import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedContent
import androidx.compose.foundation.MarqueeSpacing
import androidx.compose.foundation.background
import androidx.compose.foundation.basicMarquee
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.layout.wrapContentHeight
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.LongState
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.isTraversalGroup
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.testTagsAsResourceId
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.ErrorMap
import io.github.xaviiromf.cliampx.LoginActivity
import io.github.xaviiromf.cliampx.Lyric
import io.github.xaviiromf.cliampx.Lyrics
import io.github.xaviiromf.cliampx.PlaybackClock
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Snapshot
import io.github.xaviiromf.cliampx.Ytdlp
import io.github.xaviiromf.cliampx.audioFormat
import io.github.xaviiromf.cliampx.likes
import io.github.xaviiromf.cliampx.clock
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch
import org.json.JSONObject
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

/** Behind every sheet (§4.1). */
val SCRIM = Color(0x99000000)

/** What the art square shows (§2.3). */
enum class ArtView { Cover, Visualizer, Lyrics }

/**
 * The player sheet (§7.3): a full-height modal sheet with the Player, Queue
 * and Eq pages. Only the Player page lets the sheet be dragged, so the
 * queue's scroll and drags and the EQ columns never fight it.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PlayerSheet(s: Snapshot) {
    val p = LocalPalette.current
    val state = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val scope = rememberCoroutineScope()
    val close: () -> Unit = {
        if (Looks.motion == MotionSpeed.Off) Nav.player = false else scope.launch { state.hide() }.invokeOnCompletion { Nav.player = false }
    }
    ModalBottomSheet(
        onDismissRequest = { Nav.player = false },
        sheetState = state,
        sheetGesturesEnabled = Nav.page == PlayerPage.Player,
        shape = RectangleShape,
        containerColor = Color.Transparent,
        contentColor = p.textPrimary,
        tonalElevation = 0.dp,
        scrimColor = SCRIM,
        dragHandle = null,
        contentWindowInsets = { WindowInsets(0) },
        // The whole window on a tablet too, where the sheet would stop at 640 dp.
        sheetMaxWidth = Dp.Unspecified,
    ) {
        SheetBars()
        BackHandler(Nav.page != PlayerPage.Player) { Nav.page = PlayerPage.Player }
        BoxWithConstraints(Modifier.fillMaxSize().nestedScroll(Scrolls).calmRefreshRate(calmPlayerPage())) {
            val wide = twoPanes(maxWidth)
            Backdrop(Looks.playerLevel, Modifier.matchParentSize())
            // The visualizer behind the artwork (Ajustes or ⋮), dimmed under the text; only one vis.frame grid runs at a time.
            if (backgroundVis(s) && (wide || Nav.page == PlayerPage.Player || Nav.page == PlayerPage.Lyrics)) {
                Visualizer(s, Modifier.matchParentSize().graphicsLayer { alpha = BACKGROUND_VIS_ALPHA }.testTag("vis_background"))
            }
            Column(
                Modifier.fillMaxSize().windowInsetsPadding(WindowInsets.safeDrawing)
                    .semantics { testTagsAsResourceId = true }.testTag("player"),
            ) {
                if (wide) {
                    // Two panes: the player, and beside it the queue, or the EQ or lyrics page it opens.
                    Row(Modifier.weight(1f)) {
                        Box(Modifier.weight(1f).fillMaxHeight().semantics { isTraversalGroup = true }) { PlayerPage(s, close) }
                        val side = if (Nav.page == PlayerPage.Player) PlayerPage.Queue else Nav.page
                        AnimatedContent(
                            side, Modifier.weight(1f).fillMaxHeight().semantics { isTraversalGroup = true }.testTag("player_side"),
                            transitionSpec = { Motion.screen(250) }, label = "side",
                        ) { page ->
                            when (page) {
                                PlayerPage.Eq -> EqPage(s)
                                PlayerPage.Lyrics -> KaraokePage(s)
                                else -> QueuePage(s, back = false)
                            }
                        }
                    }
                } else {
                    AnimatedContent(Nav.page, Modifier.weight(1f), transitionSpec = { Motion.screen(250) }, label = "page") { page ->
                        when (page) {
                            PlayerPage.Player -> PlayerPage(s, close)
                            PlayerPage.Queue -> QueuePage(s)
                            PlayerPage.Eq -> EqPage(s)
                            PlayerPage.Lyrics -> KaraokePage(s)
                        }
                    }
                }
                // The shell's status line sits behind the sheet: the sheet's actions report here.
                StatusLine(s, WindowInsets(0))
            }
        }
    }
}

/** From this width the player sheet shows two panes (the window size class "expanded"). */
fun twoPanes(width: Dp) = width >= 840.dp

/**
 * The player page: portrait (§3.9), or with the art beside the controls
 * when the page is wider than tall (§3.10).
 */
@Composable
private fun PlayerPage(s: Snapshot, close: () -> Unit) {
    if (s.track == null) return
    var view by rememberSaveable { mutableStateOf(ArtView.Cover) }
    val now = rememberNow(s)
    val seek = remember { SeekState() }
    val setView: (ArtView) -> Unit = { view = it }
    BoxWithConstraints(Modifier.fillMaxSize()) {
        if (maxWidth > maxHeight) {
            Column(Modifier.fillMaxSize()) {
                PlayerHeader(s, close)
                BoxWithConstraints(Modifier.weight(1f).fillMaxWidth().padding(horizontal = 24.dp, vertical = 8.dp)) {
                    val side = minOf(maxHeight, maxWidth / 2 - 12.dp).coerceAtLeast(0.dp)
                    val height = maxHeight
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Box(Modifier.size(side)) { ArtArea(s, view, side, setView) }
                        // Scrolls rather than clips at a large font size.
                        Column(Modifier.weight(1f).padding(start = 24.dp).verticalScroll(rememberScrollState())) {
                            Column(Modifier.heightIn(min = height), verticalArrangement = Arrangement.Center) {
                                TitleBlock(s)
                                Strip(s, view, 56.dp)
                                SeekRows(s, seek, now)
                                Transport(s)
                                Chips(s, view, setView)
                            }
                        }
                    }
                }
            }
        } else {
            PortraitPlayer(s, close, view, setView, seek, now, maxWidth, maxHeight)
        }
    }
}

@Composable
private fun PortraitPlayer(
    s: Snapshot, close: () -> Unit, view: ArtView, setView: (ArtView) -> Unit, seek: SeekState, now: LongState, maxWidth: Dp, maxHeight: Dp,
) {
    // The rows around the art as measured: a large font makes them taller than the 420 dp of §3.9.
    var top by remember { mutableIntStateOf(0) }
    var bottom by remember { mutableIntStateOf(0) }
    val rows = if (top == 0 || bottom == 0) PLAYER_ROWS else with(LocalDensity.current) { (top + bottom).toDp() } + 80.dp
    val (art, strip) = playerArt(maxWidth, maxHeight, rows)
    // Scrolls rather than clips when the rows outgrow the page at a large font; unbounded, they also measure their full height.
    Column(
        Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).heightIn(min = maxHeight),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Column(Modifier.onSizeChanged { top = it.height }) { PlayerHeader(s, close) }
        // The art fills the width on a tall screen; the height left splits above and below the stack, never inside it (§3.9).
        Spacer(Modifier.weight(1f))
        Box(Modifier.padding(top = 8.dp).size(art)) { ArtArea(s, view, art, setView) }
        Box(Modifier.fillMaxWidth().height(if (strip) 72.dp else 0.dp)) { if (strip) Strip(s, view, 72.dp) }
        Column(Modifier.fillMaxWidth().onSizeChanged { bottom = it.height }) {
            Column(Modifier.padding(horizontal = 24.dp)) {
                TitleBlock(s)
                SeekRows(s, seek, now)
            }
            Transport(s)
            Chips(s, view, setView)
        }
        Spacer(Modifier.weight(1f))
    }
}

/** ⌄, `── Sonando ahora ──`, ⋮, then the mode tags. */
@Composable
private fun PlayerHeader(s: Snapshot, close: () -> Unit) {
    val p = LocalPalette.current
    Row(Modifier.fillMaxWidth().heightIn(min = 48.dp), verticalAlignment = Alignment.CenterVertically) {
        IconBtn(R.drawable.ic_expand_down, stringResource(R.string.close_player), Modifier.testTag("player_close"), onClick = close)
        val title = stringResource(R.string.playing_now)
        Text(
            "── $title ──", Modifier.weight(1f).ruleSemantics(title),
            color = p.textSecondary, style = Type.label, textAlign = TextAlign.Center, maxLines = 1,
        )
        IconBtn(R.drawable.ic_more, stringResource(R.string.more), Modifier.testTag("player_more")) { Nav.sheet = Sheet.PlayerMore }
    }
    ModeTags(s)
}

/** The visualizer strip under the cover (tap: next mode), when no other visualizer shows. */
@Composable
private fun Strip(s: Snapshot, view: ArtView, height: Dp) {
    Box(Modifier.fillMaxWidth().height(height).padding(horizontal = 24.dp, vertical = 8.dp)) {
        if (view == ArtView.Cover && !backgroundVis(s) && s.visualizer.isNotEmpty() && s.visualizer != "None") {
            Visualizer(
                s,
                Modifier.fillMaxSize().clickable(onClickLabel = stringResource(R.string.vis_next)) {
                    Remote.nextVisualizer(s.visualizer)
                }.testTag("vis_strip"),
            )
        }
    }
}

/** The seek bar, the time and state, and the stream's format. */
@Composable
private fun SeekRows(s: Snapshot, seek: SeekState, now: LongState) {
    val p = LocalPalette.current
    val seekLabel = stringResource(R.string.seek)
    SeekBar(s, seek, now, Modifier.testTag("seek").semantics { contentDescription = seekLabel })
    TimeRow(s, seek, now)
    val format = audioFormat(s.audioCodec, s.audioKbps, s.audioRate)
    if (format.isNotEmpty()) {
        Text(format, Modifier.fillMaxWidth().testTag("audio_format"), color = p.textSecondary, style = Type.label, textAlign = TextAlign.End)
    }
}

/** The player and lyrics pages move at 30 fps at most; the queue and EQ pages scroll and drag at the display's full rate. */
fun calmPlayerPage() = Nav.page == PlayerPage.Player || Nav.page == PlayerPage.Lyrics

/** Whether the visualizer fills the player behind the artwork. */
fun backgroundVis(s: Snapshot) = Looks.playerVis && s.visualizer.isNotEmpty() && s.visualizer != "None"

/** How much of the background visualizer shows under the player's text. */
private const val BACKGROUND_VIS_ALPHA = 0.4f

/** The rows around the art in §3.9, the strip's 72 dp included, until the page has measured its own. */
val PLAYER_ROWS = 420.dp

/**
 * The art side and whether the visualizer strip shows (§3.9): the rows
 * around the art take rows and the art gets the rest, square; under
 * 240 dp the strip's 72 dp go to the art. Never below 0, as in a short split-screen window.
 */
fun playerArt(width: Dp, height: Dp, rows: Dp = PLAYER_ROWS): Pair<Dp, Boolean> {
    val art = minOf(width - 48.dp, height - rows)
    if (art >= 240.dp) return art to true
    return minOf(width - 48.dp, art + 72.dp).coerceAtLeast(0.dp) to false
}

/** RADIO · FUNDIDO · EQ AUTO · APAGADO 23:40, each only when it applies. */
@Composable
private fun ModeTags(s: Snapshot) {
    val p = LocalPalette.current
    val tags = buildList {
        if (s.autoRadio) add(stringResource(R.string.tag_radio))
        if (s.crossfade > 0) add(stringResource(R.string.tag_crossfade))
        if (s.eqAuto) add(stringResource(R.string.tag_eq_auto))
        if (s.sleepMinutes == -1) add(stringResource(R.string.tag_sleep_end))
        sleepClock(s)?.let { add(stringResource(R.string.tag_sleep, it)) }
    }
    if (tags.isNotEmpty()) {
        Text(
            tags.joinToString(" · "), Modifier.fillMaxWidth().padding(horizontal = 24.dp).heightIn(min = 16.dp),
            color = p.accent, style = Type.labelCaps.copy(fontSize = Type.caption.fontSize), maxLines = 1,
        )
    }
}

/** HH:mm of the sleep timer's pause, or null when no timed pause is set. */
fun sleepClock(s: Snapshot): String? =
    if (s.sleepMinutes > 0 && s.sleepAt > 0) {
        DateTimeFormatter.ofPattern("HH:mm").format(Instant.ofEpochSecond(s.sleepAt).atZone(ZoneId.systemDefault()))
    } else {
        null
    }

/** The art square: the cover, the big visualizer, the lyrics, or the play error card. */
@Composable
private fun ArtArea(s: Snapshot, view: ArtView, side: Dp, setView: (ArtView) -> Unit) {
    val track = s.track ?: return
    if (s.playError != null) return ErrorCard(s)
    val name = stringResource(
        when (view) {
            ArtView.Cover -> R.string.cover
            ArtView.Visualizer -> R.string.visualizer
            ArtView.Lyrics -> R.string.lyrics
        },
    )
    val tap = stringResource(
        when {
            view == ArtView.Lyrics -> R.string.lyrics_full
            view == ArtView.Cover && !backgroundVis(s) -> R.string.show_visualizer
            else -> R.string.show_cover
        },
    )
    Box(
        Modifier.fillMaxSize()
            .then(if (view != ArtView.Lyrics) Modifier.skipSwipe().skipActions() else Modifier)
            .semantics { if (view != ArtView.Lyrics) contentDescription = name }
            .clickable(onClickLabel = tap) {
                when {
                    // A tap on the lyrics opens them full screen (karaoke).
                    view == ArtView.Lyrics -> Nav.page = PlayerPage.Lyrics
                    view == ArtView.Cover && !backgroundVis(s) -> setView(ArtView.Visualizer)
                    else -> setView(ArtView.Cover)
                }
            }
            .testTag("art"),
    ) {
        when (view) {
            ArtView.Cover -> ArtBox(track.art, side, fade = true)
            ArtView.Visualizer -> Visualizer(s, Modifier.fillMaxSize())
            ArtView.Lyrics -> LyricsView(s, side, karaoke = false)
        }
    }
}

/** The play error (§2.10) with its action and Siguiente. */
@Composable
private fun ErrorCard(s: Snapshot) {
    val p = LocalPalette.current
    val context = LocalContext.current
    val ytdlp by Ytdlp.status.collectAsStateWithLifecycle()
    val (text, action) = ErrorMap.card(s.errorKind)
    val next = stringResource(R.string.next)
    Column(
        Modifier.fillMaxSize().border(1.dp, p.divider, Look.corner).padding(16.dp).testTag("error_card"),
        verticalArrangement = Arrangement.Center,
    ) {
        Text("ERR: " + stringResource(R.string.err_play), color = p.error, style = Type.body)
        Text(text?.let { stringResource(it) } ?: s.error, color = p.textPrimary, style = Type.body)
        Spacer(Modifier.height(16.dp))
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            val tag = Modifier.testTag("error_action")
            when (action) {
                ErrorMap.Action.UPDATE_YTDLP -> KeyPill(stringResource(R.string.update_ytdlp), tag, enabled = ytdlp != Ytdlp.Status.Updating) {
                    Remote.scope.launch { Remote.updateYtdlp() }
                }
                ErrorMap.Action.SIGN_IN -> KeyPill(stringResource(R.string.sign_in_short), tag) {
                    context.startActivity(Intent(context, LoginActivity::class.java))
                }
                ErrorMap.Action.RETRY -> KeyPill(stringResource(R.string.retry), tag) {
                    Remote.fire("queue.play", JSONObject().put("index", s.index))
                }
                ErrorMap.Action.NEXT -> KeyPill(next, tag) { Remote.fire("next") }
            }
            if (action != ErrorMap.Action.NEXT) BracketButton(next) { Remote.fire("next") }
        }
    }
}

/** Karaoke: the lyrics full screen over the background, a tap on a synced line seeks to it. */
@Composable
private fun KaraokePage(s: Snapshot) {
    val p = LocalPalette.current
    val track = s.track ?: return
    Column(Modifier.fillMaxSize().testTag("karaoke")) {
        Row(Modifier.fillMaxWidth().heightIn(min = 56.dp), verticalAlignment = Alignment.CenterVertically) {
            IconBtn(R.drawable.ic_back, stringResource(R.string.back), Modifier.testTag("karaoke_back")) { Nav.page = PlayerPage.Player }
            Text(
                "▸─ " + stringResource(R.string.lyrics) + " " + "─".repeat(80), Modifier.weight(1f).ruleSemantics(stringResource(R.string.lyrics)),
                color = p.accent, style = Type.headline, maxLines = 1, softWrap = false, overflow = TextOverflow.Clip,
            )
        }
        Text(
            track.name + if (track.artist.isNotEmpty()) " · " + track.artist else "", Modifier.fillMaxWidth().padding(horizontal = 24.dp),
            color = p.textSecondary, style = Type.label, textAlign = TextAlign.Center, maxLines = 1, overflow = TextOverflow.Ellipsis,
        )
        BoxWithConstraints(Modifier.weight(1f).fillMaxWidth().padding(horizontal = 16.dp)) { LyricsView(s, maxHeight, karaoke = true) }
    }
}

/**
 * The lyrics (§2.3): synced lines keep the one that sings centered in
 * accent bold and the others dim; plain lyrics just scroll. Loaded for each
 * song while the view is open, kept per path. karaoke draws them larger and
 * lets a tap on a synced line seek there.
 */
@Composable
private fun LyricsView(s: Snapshot, side: Dp, karaoke: Boolean) {
    val p = LocalPalette.current
    val path = s.track?.path ?: return
    var lines by remember(path) { mutableStateOf<List<Lyric>?>(null) }
    var failed by remember(path) { mutableStateOf<String?>(null) }
    var attempt by remember { mutableIntStateOf(0) }
    LaunchedEffect(path, attempt) {
        failed = null
        try {
            lines = Remote.lyrics(path)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            failed = Remote.message(e)
        }
    }
    val shown = lines
    when {
        failed != null -> ErrorState(failed!!) { attempt++ }
        shown == null -> Loading()
        shown.isEmpty() -> EmptyState(stringResource(R.string.no_lyrics))
        else -> {
            val synced = remember(shown) { Lyrics.synced(shown) }
            val now = rememberNow(s)
            val snap by rememberUpdatedState(s)
            val active by remember(shown) {
                derivedStateOf { if (synced) Lyrics.active(shown, PlaybackClock.positionAt(snap, now.longValue)) else -1 }
            }
            val list = rememberLazyListState()
            LaunchedEffect(active) { if (active >= 0) list.animateScrollToItem(active) }
            // The padding lets the first and last lines reach the middle, where the active one sits.
            val seekLabel = stringResource(R.string.lyrics_seek)
            val seeks = karaoke && synced && s.seekable
            LazyColumn(Modifier.fillMaxSize(), state = list, contentPadding = PaddingValues(vertical = (side / 2 - if (karaoke) 20.dp else 14.dp).coerceAtLeast(0.dp))) {
                itemsIndexed(shown) { i, line ->
                    val on = i == active
                    Text(
                        line.text.ifBlank { "…" },
                        Modifier.fillMaxWidth()
                            .then(
                                if (seeks) {
                                    Modifier.clickable(onClickLabel = seekLabel) {
                                        Remote.fire("seek.absolute", JSONObject().put("value", line.start))
                                    }.heightIn(min = 48.dp).wrapContentHeight()
                                } else {
                                    Modifier
                                },
                            )
                            .padding(horizontal = 8.dp, vertical = 4.dp),
                        color = when {
                            on -> p.accent
                            synced -> p.textSecondary
                            else -> p.textPrimary
                        },
                        style = (if (karaoke) Type.headline.copy(fontWeight = FontWeight.Normal) else Type.body).bold(on), textAlign = TextAlign.Center,
                    )
                }
            }
        }
    }
}

/** The title (marquee while playing) with ♥, then artist · album. */
@Composable
private fun TitleBlock(s: Snapshot) {
    val p = LocalPalette.current
    val track = s.track ?: return
    val marquee = !reducedMotion()
    val moving = s.playing && marquee
    val liked by Remote.liked.collectAsStateWithLifecycle()
    Row(verticalAlignment = Alignment.CenterVertically) {
        Text(
            track.name,
            // TalkBack says the next song as it starts.
            Modifier.weight(1f).semantics { liveRegion = LiveRegionMode.Polite }.then(
                // In a layer: each step of the marquee re-records only the title.
                if (moving) Modifier.graphicsLayer().basicMarquee(initialDelayMillis = 1600, velocity = 42.dp, spacing = MarqueeSpacing(92.dp)) else Modifier,
            ),
            color = p.accent, style = Type.display, maxLines = 1, overflow = if (moving) TextOverflow.Clip else TextOverflow.Ellipsis,
        )
        FavoriteButton(liked.likes(track), Modifier.testTag("fav")) { Remote.favorite(track) }
    }
    // A tap on the artist or the album opens its page when the song names it (§2.3).
    Row(verticalAlignment = Alignment.CenterVertically) {
        val artist = track.meta["ytmusic.artist"]
        val album = track.meta["ytmusic.album"]
        // A link gets a 48 dp target (§1.2), the text centered in it.
        fun Modifier.opens(screen: Screen?) = if (screen == null) this else clickable { Nav.open(screen) }.heightIn(min = 48.dp).wrapContentHeight()
        if (track.artist.isNotEmpty()) {
            Text(
                track.artist, Modifier.weight(1f, fill = false).testTag("player_artist").opens(artist?.let { Screen.Artist(it, track.artist) }),
                color = p.textSecondary, style = Type.body, maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
        }
        if (track.album.isNotEmpty()) {
            Text(
                (if (track.artist.isNotEmpty()) " · " else "") + track.album,
                Modifier.weight(1f, fill = false).testTag("player_album").opens(album?.let { Screen.Album(it, track.album, track.artUrl, track.artist) }),
                color = p.textSecondary, style = Type.body, maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

/** ♥ in red when the song is a favorite, an outline in dim when not. */
@Composable
fun FavoriteButton(on: Boolean, modifier: Modifier = Modifier, onClick: () -> Unit) {
    val p = LocalPalette.current
    IconBtn(
        if (on) R.drawable.ic_heart else R.drawable.ic_heart_outline,
        stringResource(if (on) R.string.unlike else R.string.like), modifier,
        tint = if (on) p.favorite else p.textSecondary, onClick = onClick,
    )
}

/** `01:23 / 03:45` and the state: ▶ Sonando, ‖ En pausa, ◌ Cargando..., ■ Detenido or LIVE. */
@Composable
private fun TimeRow(s: Snapshot, seek: SeekState, now: LongState) {
    val p = LocalPalette.current
    val snap by rememberUpdatedState(s)
    val secs by remember { derivedStateOf { seek.position(snap, now.longValue).toInt() } }
    // Its own layer: the second ticking re-records only this row.
    Row(Modifier.fillMaxWidth().graphicsLayer(), verticalAlignment = Alignment.CenterVertically) {
        Text(
            if (s.duration > 0) clock(secs) + " / " + clock(s.duration.toInt()) else clock(secs),
            Modifier.weight(1f), color = p.textPrimary, style = Type.numeric,
        )
        val (status, color) = when {
            s.buffering -> "◌ " + stringResource(R.string.loading) to p.textSecondary
            s.playing && s.duration <= 0 -> stringResource(R.string.status_live) to p.playing
            s.playing -> "▶ " + stringResource(R.string.status_playing) to p.playing
            s.state == "paused" -> "‖ " + stringResource(R.string.status_paused) to p.textSecondary
            else -> "■ " + stringResource(R.string.status_stopped) to p.textSecondary
        }
        // TalkBack reads the state without its glyph.
        Text(status, Modifier.semantics { contentDescription = status.removePrefix("▶ ").removePrefix("‖ ").removePrefix("◌ ").removePrefix("■ ") }, color = color, style = Type.numeric.bold(color == p.playing))
    }
}

/** Shuffle, prev, the play/pause key pill, next and repeat. */
@Composable
private fun Transport(s: Snapshot) {
    val p = LocalPalette.current
    Row(
        Modifier.fillMaxWidth().height(88.dp).padding(horizontal = 8.dp),
        horizontalArrangement = Arrangement.SpaceEvenly, verticalAlignment = Alignment.CenterVertically,
    ) {
        val shuffleState = stringResource(if (s.shuffle) R.string.on else R.string.off)
        IconBtn(R.drawable.ic_shuffle, stringResource(R.string.shuffle), Modifier.testTag("shuffle").semantics { stateDescription = shuffleState }, tint = if (s.shuffle) p.accent else p.textSecondary, size = 56.dp) {
            Remote.fire("shuffle", JSONObject().put("name", "toggle"))
        }
        IconBtn(R.drawable.ic_prev, stringResource(R.string.prev), Modifier.testTag("prev"), tint = p.textPrimary, size = 56.dp) { Remote.fire("prev") }
        Box(
            Modifier.size(72.dp).background(p.accent).clickable(role = Role.Button) { Remote.fire("toggle") }.testTag("play_pause"),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                painterResource(if (s.playing) R.drawable.ic_pause else R.drawable.ic_play), stringResource(if (s.playing) R.string.pause else R.string.play),
                Modifier.size(32.dp), tint = p.onAccent,
            )
        }
        IconBtn(R.drawable.ic_next, stringResource(R.string.next), Modifier.testTag("next"), tint = p.textPrimary, size = 56.dp) { Remote.fire("next") }
        IconBtn(
            if (s.repeat == "One") R.drawable.ic_repeat_one else R.drawable.ic_repeat, stringResource(R.string.repeat, repeatLabel(s.repeat)),
            Modifier.testTag("repeat"), tint = if (s.repeat == "Off" || s.repeat.isEmpty()) p.textSecondary else p.accent, size = 56.dp,
        ) { Remote.fire("repeat", JSONObject().put("name", "cycle")) }
    }
}

@Composable
fun repeatLabel(mode: String) = stringResource(
    when (mode) {
        "All" -> R.string.repeat_all
        "One" -> R.string.repeat_one
        else -> R.string.repeat_off
    },
)

/** [Letras] [En cola: n] [EQ] [⏲ 23:40]; they wrap at a large font size. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Chips(s: Snapshot, view: ArtView, setView: (ArtView) -> Unit) {
    FlowRow(
        Modifier.fillMaxWidth().padding(horizontal = 8.dp),
        horizontalArrangement = Arrangement.SpaceEvenly, itemVerticalAlignment = Alignment.CenterVertically,
    ) {
        BracketChip(stringResource(R.string.lyrics), view == ArtView.Lyrics, Modifier.testTag("chip_lyrics")) {
            setView(if (view == ArtView.Lyrics) ArtView.Cover else ArtView.Lyrics)
        }
        BracketChip(stringResource(R.string.queue_count, s.total), false, Modifier.testTag("chip_queue")) { Nav.page = PlayerPage.Queue }
        BracketChip(stringResource(R.string.eq_short), false, Modifier.testTag("chip_eq")) { Nav.page = PlayerPage.Eq }
        val sleep = if (s.sleepMinutes == -1) stringResource(R.string.sleep_chip_end) else sleepClock(s)
        BracketChip(
            sleep.orEmpty(), sleep != null, Modifier.testTag("chip_sleep"), icon = R.drawable.ic_timer,
            description = stringResource(R.string.sleep) + (sleep?.let { ": $it" } ?: ""),
        ) { Nav.sheet = Sheet.Sleep }
    }
}
