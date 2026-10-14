package io.github.xaviiromf.cliampx.ui

import android.content.Context
import android.content.Intent
import android.os.SystemClock
import android.provider.Settings
import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.MarqueeSpacing
import androidx.compose.foundation.background
import androidx.compose.foundation.basicMarquee
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.Orientation
import androidx.compose.foundation.gestures.draggable
import androidx.compose.foundation.gestures.rememberDraggableState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.WindowInsetsSides
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.isImeVisible
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.only
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveableStateHolder
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.core.content.edit
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.CliampService
import io.github.xaviiromf.cliampx.ErrorMap
import io.github.xaviiromf.cliampx.PlaybackClock
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Snapshot
import io.github.xaviiromf.cliampx.Updates
import io.github.xaviiromf.cliampx.Ytdlp
import io.github.xaviiromf.cliampx.likes
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.merge
import kotlinx.coroutines.launch

/** The app shell (UI-SPEC §2): the open tab, the status line, the mini-player and the tab bar. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun App(firstRun: Boolean) {
    val p = LocalPalette.current
    val connected by Remote.connected.collectAsStateWithLifecycle()
    val stopped by Remote.stopped.collectAsStateWithLifecycle()
    val snap by Remote.state.collectAsStateWithLifecycle()
    if (!connected && snap == null) return Splash(firstRun, stopped)
    BackHandler(enabled = Nav.canBack) { Nav.back() }
    val holder = rememberSaveableStateHolder()
    // A popped screen's lists and scroll go with it.
    LaunchedEffect(Nav.dropped.size) {
        Nav.dropped.forEach(holder::removeState)
        Nav.dropped.clear()
    }
    // The bars' backgrounds run to the screen edges; their content keeps clear of a side nav bar and the cutout.
    val sides = WindowInsets.safeDrawing.only(WindowInsetsSides.Horizontal)
    Column(Modifier.fillMaxSize().background(p.bg).imePadding()) {
        Column(Modifier.weight(1f).windowInsetsPadding(WindowInsets.safeDrawing.only(WindowInsetsSides.Top + WindowInsetsSides.Horizontal))) {
            AnimatedContent(Nav.top, Modifier.weight(1f), transitionSpec = { fadeIn(tween(150)) togetherWith fadeOut(tween(150)) }, label = "screen") { entry ->
                holder.SaveableStateProvider(entry.key) {
                    when (val screen = entry.screen) {
                        Screen.Home -> HomeScreen()
                        Screen.Search -> SearchScreen()
                        Screen.Library -> LibraryScreen()
                        is Screen.Album -> AlbumScreen(screen)
                        is Screen.Artist -> ArtistScreen(screen)
                        is Screen.Playlist -> PlaylistScreen(screen)
                        is Screen.Genre -> GenreScreen(screen)
                        is Screen.ParaTi -> ParaTiScreen(screen)
                        is Screen.Remixes -> RemixesScreen(screen)
                        is Screen.Settings -> SettingsScreen(screen)
                        Screen.Themes -> ThemesScreen()
                        Screen.Visualizers -> VisualizersScreen()
                        Screen.Eq -> snap?.let { EqPage(it) { Nav.pop() } }
                        Screen.Stats -> StatsScreen()
                        Screen.Ratings -> RatingsScreen()
                    }
                }
            }
            if (stopped) CoreStopped()
        }
        StatusLine(snap, sides)
        MiniPlayer(snap, sides)
        TabBar(sides)
    }
    // A sheet takes the focus from the search field, so closing it never brings the keyboard back (§1.2).
    val focus = LocalFocusManager.current
    LaunchedEffect(Nav.player || Nav.sheet != null) { if (Nav.player || Nav.sheet != null) focus.clearFocus() }
    // A keyboard hidden with back leaves no focused field, which a recreated window would open again.
    val ime = WindowInsets.isImeVisible
    LaunchedEffect(ime) { if (!ime) focus.clearFocus() }
    // No song or no core: the sheet closes (an open request too), so it does not open by itself later and the shell's banner shows.
    val s = snap
    val gone = s?.track == null || stopped
    LaunchedEffect(gone, Nav.player) {
        if (gone && Nav.player) {
            Nav.player = false
            Nav.page = PlayerPage.Player
        }
    }
    if (Nav.player && !stopped && s?.track != null) PlayerSheet(s)
    // The battery card waits for the first song (§2.10).
    val context = LocalContext.current
    val playing = s?.playing == true
    LaunchedEffect(playing) { if (playing) context.getSharedPreferences("ui", Context.MODE_PRIVATE).edit { putBoolean(PLAYED, true) } }
    SheetHost(snap)
}

@Composable
private fun Splash(firstRun: Boolean, stopped: Boolean) {
    val p = LocalPalette.current
    val waiting by Remote.gateWaiting.collectAsStateWithLifecycle()
    Column(
        Modifier.fillMaxSize().background(p.bg).padding(24.dp),
        verticalArrangement = Arrangement.Center, horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text("C L I A M P - X", color = p.accent, style = Type.logo)
        Spacer(Modifier.height(8.dp))
        if (stopped) {
            CoreStopped()
        } else {
            SpinnerLine(stringResource(R.string.starting))
            if (firstRun) Text(stringResource(R.string.first_run), color = p.textSecondary, style = Type.caption, textAlign = TextAlign.Center)
            if (waiting > 0) Text("◌ " + stringResource(R.string.ytdlp_preparing), color = p.textSecondary, style = Type.caption)
        }
    }
}

/** The core ended with an error: Reintentar starts it again. */
@Composable
private fun CoreStopped() {
    val p = LocalPalette.current
    val context = LocalContext.current
    Banner("banner_stopped") {
        Text("ERR: " + stringResource(R.string.core_stopped), Modifier.weight(1f), color = p.error, style = Type.caption)
        BracketButton(stringResource(R.string.retry)) {
            ContextCompat.startForegroundService(context, Intent(context, CliampService::class.java))
        }
    }
}

/** The core's notice, or an app error for 4 s, as one caption row (§2.10). The player sheet shows its own. */
@Composable
fun StatusLine(snap: Snapshot?, sides: WindowInsets) {
    val p = LocalPalette.current
    // An app message for 4 s: a failed call (true), or a done action the core does not report.
    val app by produceState<Pair<String, Boolean>?>(null) {
        merge(Remote.errors.map { it to true }, Remote.info.map { it to false }).collectLatest {
            value = it
            delay(4000)
            value = null
        }
    }
    val notice = snap?.notice?.takeUnless { it.kind in setOf("warning", "error") && snap.playError != null }
    val (text, color) = when {
        app?.second == true -> "ERR: ${app?.first}" to p.error
        app != null -> app?.first.orEmpty() to p.playing
        notice == null -> return
        notice.kind == "success" -> notice.text to p.playing
        notice.kind == "warning" -> "WARN: ${notice.text}" to p.warning
        notice.kind == "error" -> "ERR: ${notice.text}" to p.error
        else -> notice.text to p.textSecondary
    }
    Text(
        text, Modifier.fillMaxWidth().windowInsetsPadding(sides).height(24.dp).padding(horizontal = 16.dp, vertical = 4.dp).testTag("status_line"),
        color = color, style = Type.caption.bold(color != p.textSecondary && color != p.error), maxLines = 1, overflow = TextOverflow.Ellipsis,
    )
}

/**
 * The mini-player (§2.2): art, title, a state line, ♥, play or pause, and
 * the progress along its bottom. A tap or a drag up opens the player; a
 * horizontal swipe skips.
 */
@Composable
private fun MiniPlayer(snap: Snapshot?, sides: WindowInsets) {
    val p = LocalPalette.current
    val pending by Remote.pending.collectAsStateWithLifecycle()
    val waiting by Remote.gateWaiting.collectAsStateWithLifecycle()
    val liked by Remote.liked.collectAsStateWithLifecycle()
    val track = pending ?: snap?.track ?: return
    val error = snap?.playError?.takeIf { pending == null }
    val (second, color) = when {
        pending != null && waiting > 0 -> "◌ " + stringResource(R.string.ytdlp_preparing) to p.textSecondary
        pending != null || snap?.buffering == true -> "◌ " + stringResource(R.string.loading) to p.textSecondary
        error != null -> "ERR: " + (ErrorMap.card(snap.errorKind).first?.let { stringResource(it) } ?: error) to p.error
        else -> track.artist to p.textSecondary
    }
    val playing = snap?.playing == true && pending == null
    val context = LocalContext.current
    val marquee = remember { Settings.Global.getFloat(context.contentResolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f) != 0f }
    val density = LocalDensity.current
    var lift by remember { mutableFloatStateOf(0f) }
    val open = { if (snap?.track != null) Nav.openPlayer() }
    Column(
        Modifier.fillMaxWidth().background(p.surface).windowInsetsPadding(sides).testTag("mini_player")
            .draggable(
                rememberDraggableState { lift += it }, Orientation.Vertical,
                onDragStarted = { lift = 0f },
                onDragStopped = { velocity ->
                    if (with(density) { lift < -48.dp.toPx() || velocity < -1000.dp.toPx() }) open()
                },
            )
            .clickable { open() },
    ) {
        Row(Modifier.fillMaxWidth().heightIn(min = 62.dp).skipSwipe().padding(start = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            ArtBox(track.art, 48.dp)
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text(
                    track.name, if (playing && marquee) Modifier.basicMarquee(initialDelayMillis = 1600, velocity = 42.dp, spacing = MarqueeSpacing(59.dp)) else Modifier,
                    color = p.textPrimary, style = Type.body, maxLines = 1, overflow = if (playing && marquee) TextOverflow.Clip else TextOverflow.Ellipsis,
                )
                Text(second, color = color, style = Type.label, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            if (pending == null) {
                FavoriteButton(liked.likes(track), Modifier.testTag("mini_fav")) { Remote.favorite(track) }
                IconBtn(
                    if (playing) R.drawable.ic_pause else R.drawable.ic_play, stringResource(R.string.play_pause),
                    Modifier.testTag("mini_toggle"), tint = p.textPrimary,
                ) { Remote.fire("toggle") }
            } else {
                // Loading: neither ▶ nor ‖ until the song plays.
                Box(Modifier.size(48.dp).testTag("mini_loading"), contentAlignment = Alignment.Center) { Text(spinner(), color = p.accent, style = Type.headline) }
            }
        }
        Progress(snap)
    }
}

/** The 2 dp accent progress line; it moves only while playing on screen. */
@Composable
private fun Progress(snap: Snapshot?) {
    val p = LocalPalette.current
    val now = rememberNow(snap)
    Box(
        Modifier.fillMaxWidth().height(2.dp).drawBehind {
            val s = snap ?: return@drawBehind
            if (s.duration <= 0) return@drawBehind
            val f = (PlaybackClock.positionAt(s, now.longValue) / s.duration).toFloat().coerceIn(0f, 1f)
            drawRect(p.accent, size = Size(size.width * f, size.height))
        },
    )
}

@Composable
private fun TabBar(sides: WindowInsets) {
    val p = LocalPalette.current
    Row(Modifier.fillMaxWidth().background(p.surface).navigationBarsPadding().windowInsetsPadding(sides).height(56.dp)) {
        for (tab in Tab.entries) {
            val active = tab == Nav.tab
            val color = if (active) p.accent else p.textSecondary
            val label = stringResource(tab.label)
            Column(
                Modifier.weight(1f).fillMaxHeight().clickable { Nav.select(tab) }.testTag(tab.tag)
                    .semantics { role = Role.Tab; selected = active },
                verticalArrangement = Arrangement.Center, horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Icon(painterResource(tab.icon), null, Modifier.size(24.dp), tint = color)
                Text(if (active) "[$label]" else label, color = color, style = Type.caption.bold(active))
            }
        }
    }
}

/** The Inicio banner slot (§2.10): reconnecting, yt-dlp, then the app update. */
@Composable
fun Banners() {
    val p = LocalPalette.current
    val context = LocalContext.current
    val connected by Remote.connected.collectAsStateWithLifecycle()
    val stopped by Remote.stopped.collectAsStateWithLifecycle()
    val ytdlp by Ytdlp.status.collectAsStateWithLifecycle()
    val offer by Updates.offer.collectAsStateWithLifecycle()
    if (!connected && !stopped) Banner("banner_reconnect") { SpinnerLine(stringResource(R.string.reconnecting), Type.caption) }
    var ready by remember { mutableStateOf(false) }
    LaunchedEffect(ytdlp) {
        ready = ytdlp is Ytdlp.Status.Updated || ytdlp is Ytdlp.Status.UpToDate
        if (ready) {
            delay((4000 - (SystemClock.elapsedRealtime() - Ytdlp.finishedAt)).coerceAtLeast(0))
            ready = false
        }
    }
    when (val s = ytdlp) {
        Ytdlp.Status.Updating -> Banner("banner_ytdlp") { SpinnerLine(stringResource(R.string.ytdlp_updating), Type.caption) }
        is Ytdlp.Status.Failed -> Banner("banner_ytdlp") {
            Text("ERR: ${s.message}", Modifier.weight(1f), color = p.error, style = Type.caption)
            BracketButton(stringResource(R.string.retry)) { Remote.scope.launch { Remote.updateYtdlp() } }
        }
        is Ytdlp.Status.Updated, is Ytdlp.Status.UpToDate -> if (ready) Banner("banner_ytdlp") {
            val tag = (s as? Ytdlp.Status.Updated)?.tag ?: (s as Ytdlp.Status.UpToDate).tag
            Text("✓ " + stringResource(R.string.ytdlp_ready, tag), color = p.playing, style = Type.caption.bold(true))
        }
        Ytdlp.Status.Idle -> {}
    }
    offer?.let { version ->
        Banner("banner_update") {
            Text(stringResource(R.string.update_banner, version), Modifier.weight(1f), color = p.textPrimary, style = Type.caption)
            BracketButton(stringResource(R.string.update_install)) {
                Updates.offer.value = null
                Nav.settings(install = true)
            }
            BracketButton(stringResource(R.string.later)) { Updates.offer.value = null }
        }
    }
}
