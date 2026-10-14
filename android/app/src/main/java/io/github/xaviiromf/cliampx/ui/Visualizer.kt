package io.github.xaviiromf.cliampx.ui

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.tween
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawWithCache
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.repeatOnLifecycle
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Snapshot
import io.github.xaviiromf.cliampx.resample
import kotlin.math.max
import kotlin.math.min

/**
 * The Bars renderer (UI-SPEC §0, §5.4): the core's spectrum.get bands,
 * resampled to one bar per 12 dp, in the block spectrum's green, yellow and
 * red. Every band mode draws as Bars: the user deferred ClassicPeak and
 * Mirror (UI-SPEC §10).
 * Frames come only while this is composed, the screen started and the song
 * playing; then the bars fall to 0 in 300 ms and drawing stops.
 */
@Composable
fun Visualizer(s: Snapshot, modifier: Modifier = Modifier) {
    val p = LocalPalette.current
    // Read only in the draw lambda: a frame redraws without recomposing.
    val frame = remember { mutableStateOf(FloatArray(0)) }
    val gain = remember { Animatable(1f) }
    var terminalOnly by remember { mutableStateOf(false) }
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    val live = s.playing && !s.buffering
    LaunchedEffect(live) {
        if (!live) {
            gain.animateTo(0f, tween(300, easing = LinearEasing))
            return@LaunchedEffect
        }
        gain.snapTo(1f)
        lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) {
            Remote.spectrum().collect {
                frame.value = it
                terminalOnly = it.isEmpty() // a raw-sample mode set in Modo terminal sends no bands
            }
        }
    }
    Box(modifier, contentAlignment = Alignment.Center) {
        Spacer(
            Modifier.fillMaxSize().drawWithCache {
                // Hard stops measured from the bottom: green below 30 %, yellow to 60 %, red above.
                val brush = Brush.verticalGradient(
                    0f to p.red, 0.4f to p.red, 0.4f to p.yellow, 0.7f to p.yellow, 0.7f to p.green, 1f to p.green,
                    startY = 0f, endY = size.height,
                )
                val slot = 12.dp.toPx()
                val n = max(1, (size.width / slot).toInt())
                val width = size.width / n
                val gap = min(3.dp.toPx(), width / 3)
                onDrawBehind {
                    val g = gain.value
                    if (g <= 0f) return@onDrawBehind
                    val bars = resample(frame.value, n)
                    for (i in bars.indices) {
                        val h = bars[i].coerceIn(0f, 1f) * g * size.height
                        if (h > 0f) drawRect(brush, Offset(i * width, size.height - h), Size(width - gap, h))
                    }
                }
            },
        )
        if (terminalOnly && s.visualizer.isNotEmpty() && s.visualizer != "None") {
            Text(
                stringResource(R.string.vis_terminal_only, s.visualizer),
                color = p.textSecondary, style = Type.caption, textAlign = TextAlign.Center,
            )
        }
    }
}
