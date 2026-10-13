package io.github.xaviiromf.cliampx.ui

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
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.WindowInsetsSides
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
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
import androidx.compose.runtime.mutableLongStateOf
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
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.repeatOnLifecycle
import io.github.xaviiromf.cliampx.CliampService
import io.github.xaviiromf.cliampx.ErrorMap
import io.github.xaviiromf.cliampx.PlaybackClock
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.SettingsActivity
import io.github.xaviiromf.cliampx.Snapshot
import io.github.xaviiromf.cliampx.Updates
import io.github.xaviiromf.cliampx.Ytdlp
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** The app shell (UI-SPEC §2): the open tab, the status line, the mini-player and the tab bar. */
@Composable
fun App(firstRun: Boolean) {
    val p = LocalPalette.current
    val connected by Remote.connected.collectAsStateWithLifecycle()
    val stopped by Remote.stopped.collectAsStateWithLifecycle()
    val snap by Remote.state.collectAsStateWithLifecycle()
    if (!connected && snap == null) return Splash(firstRun, stopped)
    BackHandler(enabled = Nav.tab != Tab.Home) { Nav.back() }
    val holder = rememberSaveableStateHolder()
    // The bars' backgrounds run to the screen edges; their content keeps clear of a side nav bar and the cutout.
    val sides = WindowInsets.safeDrawing.only(WindowInsetsSides.Horizontal)
    Column(Modifier.fillMaxSize().background(p.bg).imePadding()) {
        Column(Modifier.weight(1f).windowInsetsPadding(WindowInsets.safeDrawing.only(WindowInsetsSides.Top + WindowInsetsSides.Horizontal))) {
            AnimatedContent(Nav.tab, Modifier.weight(1f), transitionSpec = { fadeIn(tween(150)) togetherWith fadeOut(tween(150)) }, label = "tab") { tab ->
                holder.SaveableStateProvider(tab.name) {
                    when (tab) {
                        Tab.Home -> HomeScreen()
                        Tab.Search -> SearchScreen()
                        Tab.Library -> LibraryScreen()
                    }
                }
            }
            if (stopped) CoreStopped()
        }
        StatusLine(snap, sides)
        MiniPlayer(snap, sides)
        TabBar(sides)
    }
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
            if (waiting) Text("◌ " + stringResource(R.string.ytdlp_preparing), color = p.textSecondary, style = Type.caption)
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

/** The core's notice, or an app error for 4 s, as one caption row (§2.10). */
@Composable
private fun StatusLine(snap: Snapshot?, sides: WindowInsets) {
    val p = LocalPalette.current
    val appError by produceState<String?>(null) {
        Remote.errors.collect {
            value = it
            delay(4000)
            value = null
        }
    }
    val notice = snap?.notice?.takeUnless { it.kind in setOf("warning", "error") && snap.playError != null }
    val (text, color) = when {
        appError != null -> "ERR: $appError" to p.error
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

/** The mini-player (§2.2): art, title, a state line, play or pause, and the progress along its bottom. */
@Composable
private fun MiniPlayer(snap: Snapshot?, sides: WindowInsets) {
    val p = LocalPalette.current
    val pending by Remote.pending.collectAsStateWithLifecycle()
    val waiting by Remote.gateWaiting.collectAsStateWithLifecycle()
    val track = pending ?: snap?.track ?: return
    val error = snap?.playError?.takeIf { pending == null }
    val (second, color) = when {
        pending != null && waiting -> "◌ " + stringResource(R.string.ytdlp_preparing) to p.textSecondary
        pending != null || snap?.buffering == true -> "◌ " + stringResource(R.string.loading) to p.textSecondary
        error != null -> "ERR: " + (ErrorMap.card(snap.errorKind).first?.let { stringResource(it) } ?: error) to p.error
        else -> track.artist to p.textSecondary
    }
    val playing = snap?.playing == true && pending == null
    val context = LocalContext.current
    val marquee = remember { Settings.Global.getFloat(context.contentResolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f) != 0f }
    Column(Modifier.fillMaxWidth().background(p.surface).windowInsetsPadding(sides).testTag("mini_player")) {
        Row(Modifier.fillMaxWidth().height(62.dp).padding(start = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            ArtBox(track.art, 48.dp)
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text(
                    track.name, if (playing && marquee) Modifier.basicMarquee(initialDelayMillis = 1600, velocity = 42.dp, spacing = MarqueeSpacing(59.dp)) else Modifier,
                    color = p.textPrimary, style = Type.body, maxLines = 1, overflow = if (playing && marquee) TextOverflow.Clip else TextOverflow.Ellipsis,
                )
                Text(second, color = color, style = Type.label, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            IconBtn(
                if (playing) R.drawable.ic_pause else R.drawable.ic_play, stringResource(R.string.play_pause),
                Modifier.testTag("mini_toggle"), tint = p.textPrimary,
            ) { Remote.fire("toggle") }
        }
        Progress(snap)
    }
}

/** The 2 dp accent progress line; a 250 ms ticker moves it only while playing on screen. */
@Composable
private fun Progress(snap: Snapshot?) {
    val p = LocalPalette.current
    var now by remember { mutableLongStateOf(SystemClock.elapsedRealtime()) }
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    LaunchedEffect(snap) {
        now = SystemClock.elapsedRealtime()
        if (snap?.playing != true) return@LaunchedEffect
        lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) {
            while (true) {
                now = SystemClock.elapsedRealtime()
                delay(250)
            }
        }
    }
    Box(
        Modifier.fillMaxWidth().height(2.dp).drawBehind {
            val s = snap ?: return@drawBehind
            if (s.duration <= 0) return@drawBehind
            val f = (PlaybackClock.positionAt(s, now) / s.duration).toFloat().coerceIn(0f, 1f)
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
                context.startActivity(Intent(context, SettingsActivity::class.java).putExtra(SettingsActivity.EXTRA_UPDATE, true))
            }
            BracketButton(stringResource(R.string.later)) { Updates.offer.value = null }
        }
    }
}
