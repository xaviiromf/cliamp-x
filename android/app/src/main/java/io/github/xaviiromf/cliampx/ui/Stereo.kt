package io.github.xaviiromf.cliampx.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Snapshot
import kotlinx.coroutines.delay
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

/** The rules of Modo estéreo: when it dims and where the burn-in shift puts it. */
object Stereo {
    /** Night dimming from 22:00 to 06:59. */
    fun night(hour: Int) = hour >= 22 || hour < 7

    /** The burn-in shift for a minute since the epoch: a step in -12..12 dp on each axis, a new place every minute. */
    fun shift(minute: Long): Pair<Int, Int> = (Math.floorMod(minute * 7, 25L) - 12).toInt() to (Math.floorMod(minute * 11, 25L) - 12).toInt()
}

/**
 * Modo estéreo: an always-on screen with the clock, the artwork, the song
 * and the visualizer. It dims at night (dim asks the host for a low
 * brightness), moves a little every minute against burn-in, and a tap leaves.
 */
@Composable
fun StereoScreen(exit: () -> Unit, dim: (Boolean) -> Unit) {
    val p = LocalPalette.current
    val s by Remote.state.collectAsStateWithLifecycle()
    val minute by produceState(System.currentTimeMillis() / 60_000) {
        while (true) {
            delay(60_000 - System.currentTimeMillis() % 60_000)
            value = System.currentTimeMillis() / 60_000
        }
    }
    val time = Instant.ofEpochSecond(minute * 60).atZone(ZoneId.systemDefault())
    val night = Stereo.night(time.hour)
    LaunchedEffect(night) { dim(night) }
    val (dx, dy) = Stereo.shift(minute)
    val close = stringResource(R.string.close)
    BoxWithConstraints(
        Modifier.fillMaxSize().calmRefreshRate(true).background(if (night) Color.Black else p.bg).clickable(onClickLabel = close, onClick = exit).testTag("stereo"),
        contentAlignment = Alignment.Center,
    ) {
        val wide = maxWidth > maxHeight
        val content = Modifier.offset(dx.dp, dy.dp).alpha(if (night) 0.55f else 1f).padding(24.dp)
        val clock = DateTimeFormatter.ofPattern("HH:mm").format(time)
        val track = s?.track
        if (track == null) {
            Column(content, horizontalAlignment = Alignment.CenterHorizontally) {
                Clock(clock)
                Text("C L I A M P - X", color = p.textSecondary, style = Type.logo)
            }
        } else if (wide) {
            val art = minOf(maxHeight * 0.7f, maxWidth * 0.4f)
            Row(content, verticalAlignment = Alignment.CenterVertically) {
                ArtBox(track.art, art, fade = true)
                Spacer(Modifier.width(32.dp))
                Column(Modifier.weight(1f)) { Details(s!!, clock, TextAlign.Start) }
            }
        } else {
            val art = minOf(maxWidth * 0.75f, maxHeight * 0.38f)
            Column(content, horizontalAlignment = Alignment.CenterHorizontally) {
                Clock(clock)
                Spacer(Modifier.height(24.dp))
                ArtBox(track.art, art, fade = true)
                Spacer(Modifier.height(16.dp))
                Details(s!!, null, TextAlign.Center)
            }
        }
    }
}

@Composable
private fun Clock(text: String) {
    Text(text, color = LocalPalette.current.accent, style = Type.display.copy(fontSize = 72.sp, lineHeight = 80.sp), maxLines = 1)
}

/** The clock when given, the title, the artist and the visualizer. */
@Composable
private fun ColumnScope.Details(s: Snapshot, clock: String?, align: TextAlign) {
    val p = LocalPalette.current
    val track = s.track ?: return
    clock?.let { Clock(it) }
    Text(track.name, Modifier.fillMaxWidth(), color = p.accent, style = Type.display, textAlign = align, maxLines = 2, overflow = TextOverflow.Ellipsis)
    if (track.artist.isNotEmpty()) {
        Text(track.artist, Modifier.fillMaxWidth(), color = p.textSecondary, style = Type.body, textAlign = align, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
    if (s.visualizer.isNotEmpty() && s.visualizer != "None") {
        Spacer(Modifier.height(16.dp))
        Box(Modifier.fillMaxWidth().height(VIS_HEIGHT)) { Visualizer(s, Modifier.fillMaxSize().testTag("stereo_vis")) }
    }
}

private val VIS_HEIGHT: Dp = 96.dp
