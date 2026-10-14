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
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawWithCache
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.repeatOnLifecycle
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Snapshot
import io.github.xaviiromf.cliampx.VisState
import io.github.xaviiromf.cliampx.hash01
import io.github.xaviiromf.cliampx.nativeVis
import kotlinx.coroutines.launch
import kotlin.math.PI
import kotlin.math.abs
import kotlin.math.cos
import kotlin.math.max
import kotlin.math.min
import kotlin.math.sin
import kotlin.math.sqrt

/**
 * The native visualizers (UI-SPEC §0, §5.4): the core's spectrum.get bands
 * eased at the display's frame rate into one VisState, drawn on one canvas
 * by the renderer of the core's mode (NATIVE_VIS); any other mode draws as
 * Bars. Frames come only while this is composed, the screen started and the
 * song playing; then the levels fall to 0 in 300 ms and drawing stops.
 * Reduced motion steps once per spectrum frame with no easing, and draws
 * the modes that move on their own as Bars.
 */
@Composable
fun Visualizer(s: Snapshot, modifier: Modifier = Modifier) {
    val p = LocalPalette.current
    val st = remember { VisState() }
    // Read only in the draw lambda: a frame redraws without recomposing.
    val tick = remember { mutableLongStateOf(0L) }
    val gain = remember { Animatable(1f) }
    var terminalOnly by remember { mutableStateOf(false) }
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    val live = s.playing && !s.buffering
    val reduced = reducedMotion()
    val mode = nativeVis(s.visualizer, reduced)
    val stepMode by rememberUpdatedState(mode) // a new mode keeps the spectrum connection
    LaunchedEffect(live, reduced) {
        if (!live) {
            gain.animateTo(0f, tween(300, easing = LinearEasing))
            return@LaunchedEffect
        }
        gain.snapTo(1f)
        lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) {
            launch {
                Remote.spectrum().collect {
                    st.set(it)
                    terminalOnly = it.isEmpty() // a raw-sample mode set in Modo terminal sends no bands
                    if (reduced) {
                        st.step(0.033f, stepMode, snap = true)
                        tick.longValue++
                    }
                }
            }
            if (!reduced) {
                var last = 0L
                while (true) {
                    withFrameNanos { now ->
                        if (last != 0L) st.step((now - last) / 1e9f, stepMode)
                        last = now
                        tick.longValue = now
                    }
                }
            }
        }
    }
    Box(modifier, contentAlignment = Alignment.Center) {
        Spacer(
            Modifier.fillMaxSize().drawWithCache {
                val k = Kit(
                    p,
                    // Hard stops measured from the bottom: green below 30 %, yellow to 60 %, red above.
                    Brush.verticalGradient(
                        0f to p.red, 0.4f to p.red, 0.4f to p.yellow, 0.7f to p.yellow, 0.7f to p.green, 1f to p.green,
                        startY = 0f, endY = size.height,
                    ),
                    // The same stops from the middle out, for the mirrored modes.
                    Brush.verticalGradient(
                        0f to p.red, 0.2f to p.red, 0.2f to p.yellow, 0.35f to p.yellow, 0.35f to p.green,
                        0.65f to p.green, 0.65f to p.yellow, 0.8f to p.yellow, 0.8f to p.red, 1f to p.red,
                        startY = 0f, endY = size.height,
                    ),
                    Stroke(1.5.dp.toPx()),
                    Stroke(2.5.dp.toPx()),
                )
                onDrawBehind {
                    tick.longValue
                    val g = gain.value
                    if (g <= 0f) return@onDrawBehind
                    draw(mode, st, k, g)
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

/** What the renderers share, made once per size and palette. */
private class Kit(val p: CliampPalette, val bars: Brush, val mirror: Brush, val thin: Stroke, val thick: Stroke)

private fun DrawScope.draw(mode: String, st: VisState, k: Kit, g: Float) {
    when (mode) {
        "ClassicPeak" -> classicPeak(st, k, g)
        "ClassicLED" -> led(st, k, g, 6f, 1.5f, unlit = true, peaks = true)
        "Bricks" -> led(st, k, g, 10f, 3f, unlit = false, peaks = false)
        "Columns" -> bars(st, k, g, 5f, 1.5f)
        "BarsDot" -> barsDot(st, k, g)
        "BarsOutline" -> outline(st, k, g)
        "Mirror" -> mirror(st, k, g)
        "Butterfly" -> butterfly(st, k, g)
        "Ascii" -> ascii(st, k, g)
        "Mosaic" -> mosaic(st, k, g)
        "Logo" -> logo(st, k, g)
        "Spectrogram" -> spectrogram(st, k, g)
        "Terrain" -> terrain(st, k, g)
        "Skyline" -> skyline(st, k, g)
        "Radial" -> radial(st, k, g)
        "Pulse" -> pulse(st, k, g)
        "Ripple" -> ripple(st, k, g)
        "Tunnel" -> tunnel(st, k, g)
        "Spiral" -> spiral(st, k, g)
        "Helix" -> helix(st, k, g)
        "Plasma" -> plasma(st, k, g)
        "Aurora" -> aurora(st, k, g)
        "Retro" -> retro(st, k, g)
        "Flame" -> flame(st, k, g)
        "Rain" -> rain(st, k, g)
        "Matrix" -> matrix(st, k, g)
        "Scatter", "Geyser" -> sparks(st, k, g)
        "Firework" -> firework(st, k, g)
        "Bubbles" -> bubbles(st, k, g)
        "Sakura" -> sakura(st, k, g)
        "Firefly" -> firefly(st, k, g)
        "Starfield" -> starfield(st, k, g)
        else -> bars(st, k, g, 12f, 3f)
    }
}

// --- helpers ---------------------------------------------------------------

/** a blended toward b by f in sRGB, with no allocation. */
private fun mix(a: Color, b: Color, f: Float): Color {
    val t = f.coerceIn(0f, 1f)
    return Color(a.red + (b.red - a.red) * t, a.green + (b.green - a.green) * t, a.blue + (b.blue - a.blue) * t, a.alpha + (b.alpha - a.alpha) * t)
}

/** The block spectrum's color at height f: green, yellow from 30 %, red from 60 %. */
private fun heat(p: CliampPalette, f: Float) = when {
    f < 0.3f -> p.green
    f < 0.6f -> p.yellow
    else -> p.red
}

/** The same colors blended smoothly, for heat maps. */
private fun glow(p: CliampPalette, f: Float) = if (f < 0.5f) mix(p.green, p.yellow, f * 2) else mix(p.yellow, p.red, f * 2 - 1)

private fun frac(i: Int, n: Int) = if (n <= 1) 0f else i / (n - 1f)

private fun DrawScope.cell(c: Color, x: Float, y: Float, w: Float, h: Float, a: Float = 1f) =
    drawRect(c, Offset(x, y), Size(w, h), alpha = a.coerceIn(0f, 1f))

/** A square of side s centered on (x, y). */
private fun DrawScope.dot(c: Color, x: Float, y: Float, s: Float, a: Float = 1f) = cell(c, x - s / 2, y - s / 2, s, s, a)

/** The outline of a rectangle as four filled bars, so no Stroke is made per frame. */
private fun DrawScope.frame(c: Color, l: Float, t: Float, r: Float, b: Float, w: Float, a: Float) {
    cell(c, l, t, r - l, w, a); cell(c, l, b - w, r - l, w, a)
    cell(c, l, t, w, b - t, a); cell(c, r - w, t, w, b - t, a)
}

// --- bar modes ---------------------------------------------------------------

private fun DrawScope.bars(st: VisState, k: Kit, g: Float, slotDp: Float, gapDp: Float) {
    val n = max(1, (size.width / slotDp.dp.toPx()).toInt())
    val w = size.width / n
    val gap = min(gapDp.dp.toPx(), w / 3)
    for (i in 0 until n) {
        val h = st.at(frac(i, n)) * g * size.height
        if (h > 0.5f) drawRect(k.bars, Offset(i * w, size.height - h), Size(w - gap, h))
    }
}

/** Thin bars under peak caps that hold, then fall. */
private fun DrawScope.classicPeak(st: VisState, k: Kit, g: Float) {
    bars(st, k, g, 9f, 3f)
    val n = max(1, (size.width / 9.dp.toPx()).toInt())
    val w = size.width / n
    val cap = 2.dp.toPx()
    for (i in 0 until n) {
        val y = size.height - st.peakAt(frac(i, n)) * g * size.height - cap
        cell(k.p.brightFg, i * w, max(0f, y - cap * 0.5f), w - min(3.dp.toPx(), w / 3), cap)
    }
}

/** A grid of lit cells, colored by row; ClassicLED shows the unlit ones dim and gray peak cells. */
private fun DrawScope.led(st: VisState, k: Kit, g: Float, rowDp: Float, gapDp: Float, unlit: Boolean, peaks: Boolean) {
    val slot = if (unlit) 10.dp.toPx() else 16.dp.toPx()
    val n = max(1, (size.width / slot).toInt())
    val w = size.width / n
    val rows = max(3, (size.height / rowDp.dp.toPx()).toInt())
    val rh = size.height / rows
    val gap = gapDp.dp.toPx()
    for (i in 0 until n) {
        val f = frac(i, n)
        val lit = (st.at(f) * g * rows).toInt()
        val pk = (st.peakAt(f) * g * rows).toInt()
        for (r in 0 until rows) {
            val on = r < lit
            if (!on && !unlit && !(peaks && r == pk)) continue
            val c = if (peaks && r == pk && r >= lit) k.p.brightFg else heat(k.p, r / rows.toFloat())
            cell(c, i * w, size.height - (r + 1) * rh, w - gap, rh - min(gap, rh / 3), if (on || (peaks && r == pk)) 1f else 0.1f)
        }
    }
}

/** Bars made of square dots. */
private fun DrawScope.barsDot(st: VisState, k: Kit, g: Float) {
    val step = 6.dp.toPx()
    val s = 3.5.dp.toPx()
    val n = max(1, (size.width / step).toInt())
    val rows = (size.height / step).toInt()
    for (i in 0 until n) {
        val lit = (st.at(frac(i, n)) * g * rows).toInt()
        for (r in 0 until lit) cell(heat(k.p, r / rows.toFloat()), i * step, size.height - (r + 1) * step, s, s)
    }
}

/** Only the top edge of the bars, a stepped line over a faint fill. */
private fun DrawScope.outline(st: VisState, k: Kit, g: Float) {
    val n = max(1, (size.width / 12.dp.toPx()).toInt())
    val w = size.width / n
    val lw = 2.dp.toPx()
    var prev = -1f
    for (i in 0 until n) {
        val lv = st.at(frac(i, n)) * g
        val y = size.height - lv * size.height
        drawRect(k.bars, Offset(i * w, y), Size(w, size.height - y), alpha = 0.15f)
        val c = heat(k.p, lv)
        cell(c, i * w, y - lw / 2, w, lw)
        if (prev >= 0f) cell(c, i * w - lw / 2, min(prev, y), lw, abs(prev - y))
        prev = y
    }
}

/** Bars grown up and down from the middle. */
private fun DrawScope.mirror(st: VisState, k: Kit, g: Float) {
    val n = max(1, (size.width / 12.dp.toPx()).toInt())
    val w = size.width / n
    val gap = min(3.dp.toPx(), w / 3)
    val mid = size.height / 2
    for (i in 0 until n) {
        val h = st.at(frac(i, n)) * g * mid
        if (h > 0.5f) drawRect(k.mirror, Offset(i * w, mid - h), Size(w - gap, 2 * h))
    }
}

/** Mirror folded left and right too: the bass in the middle, the treble at the edges. */
private fun DrawScope.butterfly(st: VisState, k: Kit, g: Float) {
    val n = max(3, (size.width / 8.dp.toPx()).toInt())
    val w = size.width / n
    val c = (n - 1) / 2f
    val mid = size.height / 2
    for (i in 0 until n) {
        val h = st.at(abs(i - c) / c) * g * mid
        if (h > 0.5f) drawRect(k.mirror, Offset(i * w, mid - h), Size(w - 1.dp.toPx(), 2 * h))
    }
}

/** Columns of shade blocks: light at the top of each column, solid below (░▒▓█). */
private fun DrawScope.ascii(st: VisState, k: Kit, g: Float) {
    val cw = 8.dp.toPx()
    val ch = 7.dp.toPx()
    val n = max(1, (size.width / cw).toInt())
    val rows = (size.height / ch).toInt()
    for (i in 0 until n) {
        val lit = (st.at(frac(i, n)) * g * rows).toInt()
        for (r in 0 until lit) {
            val a = when (lit - 1 - r) { 0 -> 0.3f; 1 -> 0.55f; 2 -> 0.8f; else -> 1f }
            cell(heat(k.p, r / rows.toFloat()), i * cw, size.height - (r + 1) * ch, cw - 1.dp.toPx(), ch - 1.dp.toPx(), a)
        }
    }
}

/** A heat map of tiles that flicker with their band. */
private fun DrawScope.mosaic(st: VisState, k: Kit, g: Float) {
    val t = 14.dp.toPx()
    val cols = max(1, (size.width / t).toInt())
    val rows = max(1, (size.height / t).toInt())
    val tick = (st.time * 6).toInt()
    for (c in 0 until cols) {
        val lv = st.at(frac(c, cols)) * g
        for (r in 0 until rows) {
            val v = lv * (0.45f + 0.55f * hash01(c, r, tick + c)) * (0.6f + 0.4f * (rows - r) / rows)
            cell(glow(k.p, v), c * t, r * t, t - 2.dp.toPx(), t - 2.dp.toPx(), 0.12f + v)
        }
    }
}

private val LOGO = arrayOf(
    " ### ", "#    ", "#    ", "#    ", "#    ", "#    ", " ### ", // C
    "#    ", "#    ", "#    ", "#    ", "#    ", "#    ", "#####", // L
    "#####", "  #  ", "  #  ", "  #  ", "  #  ", "  #  ", "#####", // I
    " ### ", "#   #", "#   #", "#####", "#   #", "#   #", "#   #", // A
    "#   #", "## ##", "# # #", "#   #", "#   #", "#   #", "#   #", // M
    "#### ", "#   #", "#   #", "#### ", "#    ", "#    ", "#    ", // P
)

/** CLIAMP in pixel letters, each bouncing and lighting with its band. */
private fun DrawScope.logo(st: VisState, k: Kit, g: Float) {
    val px = min(size.width / 36f, size.height / 10f)
    val x0 = (size.width - px * 35) / 2
    val y0 = (size.height - px * 7) / 2
    for (l in 0 until 6) {
        val lv = st.at(l / 5f) * g
        val dy = -lv * px * 1.5f
        for (r in 0 until 7) {
            val row = LOGO[l * 7 + r]
            for (c in 0 until 5) {
                if (row[c] != '#') continue
                val c2 = heat(k.p, (6 - r) / 7f * 0.4f + lv * 0.6f)
                cell(c2, x0 + (l * 6 + c) * px, y0 + r * px + dy, px * 0.85f, px * 0.85f, 0.35f + lv)
            }
        }
    }
}

// --- scrolling modes -----------------------------------------------------------

/** A waterfall: time runs right to left, low bands at the bottom, color by level. */
private fun DrawScope.spectrogram(st: VisState, k: Kit, g: Float) {
    val n = VisState.HIST
    val cw = size.width / (n - 1)
    val rh = size.height / VisState.HROWS
    val shift = st.histPhase * cw
    for (j in 0 until n) {
        val col = st.hist[(st.histHead + j) % n]
        val x = j * cw - shift
        for (r in 0 until VisState.HROWS) {
            val v = col[r] * g
            if (v > 0.04f) cell(glow(k.p, v), x, size.height - (r + 1) * rh, cw + 0.5f, rh - 1.dp.toPx(), 0.05f + v * v * 1.6f)
        }
    }
}

/** Two mountain ranges scrolling past: the far one from the treble, the near one from the bass. */
private fun DrawScope.terrain(st: VisState, k: Kit, g: Float) {
    val n = VisState.HIST
    val cw = size.width / (n - 1)
    val shift = st.histPhase * cw
    val snow = 2.dp.toPx()
    for (j in 0 until n) {
        val col = st.hist[(st.histHead + j) % n]
        var lo = 0f; var hi = 0f
        for (r in 0 until 6) lo += col[r]
        for (r in 12 until 24) hi += col[r]
        val far = (0.3f + hi / 12 * 1.4f).coerceAtMost(1f) * g * size.height * 0.9f
        val near = (0.08f + lo / 6 * 0.4f) * g * size.height
        val x = j * cw - shift
        cell(k.p.accent, x, size.height - far, cw + 0.5f, far, 0.3f)
        cell(k.p.green, x, size.height - near, cw + 0.5f, near)
        cell(k.p.yellow, x, size.height - near, cw + 0.5f, snow)
    }
}

/** A city at night, a building per band, more windows lit the louder it plays. */
private fun DrawScope.skyline(st: VisState, k: Kit, g: Float) {
    val slot = 22.dp.toPx()
    val n = max(1, (size.width / slot).toInt())
    val w = size.width / n
    val win = 3.dp.toPx()
    val step = 6.dp.toPx()
    val body = mix(k.p.bg, k.p.fg, 0.3f)
    for (i in 0 until n) {
        val lv = st.at(frac(i, n)) * g
        val bw = w * (0.7f + 0.25f * hash01(i, 1, 0))
        val h = size.height * (0.2f + 0.75f * lv) * (0.8f + 0.2f * hash01(i, 2, 0))
        val x = i * w + (w - bw) / 2
        val top = size.height - h
        cell(body, x, top, bw, h)
        val cols = ((bw - win) / step).toInt()
        val rows = ((h - win) / step).toInt()
        for (c in 0 until cols) for (r in 0 until rows) {
            val on = hash01(i, c, r) < lv * 1.3f
            cell(if (on) k.p.yellow else k.p.bg, x + win + c * step, top + win + r * step, win, win, if (on) 1f else 0.6f)
        }
        if (lv > 0.6f) dot(k.p.red, x + bw / 2, top - win, win)
    }
}

// --- round modes -----------------------------------------------------------------

/** Spokes round a ring, turning slowly, the spectrum mirrored round the circle. */
private fun DrawScope.radial(st: VisState, k: Kit, g: Float) {
    val cx = size.width / 2; val cy = size.height / 2
    val m = min(cx, cy)
    val r0 = m * 0.3f
    val n = 48
    val sw = (2 * PI * r0 / n * 0.6).toFloat()
    for (i in 0 until n) {
        val a = (2 * PI * i / n).toFloat() + st.spin * 0.2f
        val lv = st.at(abs(i / (n / 2f) - 1f)) * g
        val len = lv * (m - r0)
        val c = cos(a); val s = sin(a)
        drawLine(heat(k.p, lv), Offset(cx + c * r0, cy + s * r0), Offset(cx + c * (r0 + len + 2), cy + s * (r0 + len + 2)), sw)
    }
    drawCircle(k.p.accent, r0 * 0.85f * (0.9f + st.bass * 0.3f), Offset(cx, cy), alpha = 0.3f + st.bass * 0.7f * g, style = k.thin)
}

/** A ring of squares breathing with the bass, its edge shaped by the spectrum. */
private fun DrawScope.pulse(st: VisState, k: Kit, g: Float) {
    val cx = size.width / 2; val cy = size.height / 2
    val m = min(cx, cy)
    val r = m * 0.45f * (1 + st.bass * g * 0.6f)
    drawCircle(k.p.accent, r * 0.55f, Offset(cx, cy), alpha = (0.15f + st.bass * 0.5f) * g)
    val s = max(2.dp.toPx(), m / 40)
    val n = 96
    for (i in 0 until n) {
        val a = (2 * PI * i / n).toFloat()
        val lv = st.at(abs(i / (n / 2f) - 1f)) * g
        val rr = min(m - s, r * (1 + lv * 0.5f))
        dot(heat(k.p, lv), cx + cos(a) * rr, cy + sin(a) * rr, s)
    }
}

/** Rings spreading on a pond, a new one on each beat. */
private fun DrawScope.ripple(st: VisState, k: Kit, g: Float) {
    val m = min(size.width, size.height)
    for (i in 0 until VisState.RINGS) {
        val age = st.ringAge[i]
        if (age > 2.5f) continue
        val c = when (i % 3) { 0 -> k.p.green; 1 -> k.p.yellow; else -> k.p.accent }
        val center = Offset(st.ringX[i] * size.width, st.ringY[i] * size.height)
        val a = (1 - age / 2.5f) * g
        drawCircle(c, age * m * 0.3f + 2, center, alpha = a, style = k.thick)
        if (age > 0.3f) drawCircle(c, (age - 0.3f) * m * 0.3f + 2, center, alpha = a * 0.5f, style = k.thin)
    }
    dot(k.p.brightFg, size.width / 2, size.height / 2, (3 + st.bass * 10 * g).dp.toPx(), 0.5f * g)
}

/** Flight down a tunnel of square frames, faster when loud. */
private fun DrawScope.tunnel(st: VisState, k: Kit, g: Float) {
    val n = 14
    val phase = (st.spin * 1.5f) % 1f
    for (i in n - 1 downTo 0) {
        val d = (i + 1 - phase) / n // 0 near, 1 far
        val half = min(size.width, size.height) * 0.04f / d
        val cx = size.width / 2 + sin(st.time * 0.7f + d * 4) * size.width * 0.12f * d
        val cy = size.height / 2 + cos(st.time * 0.5f + d * 3) * size.height * 0.1f * d
        val wx = half * size.width / min(size.width, size.height)
        val lv = st.at(d) * g
        val w = (1 + (1 - d) * 4 + lv * 4).dp.toPx()
        frame(glow(k.p, lv), cx - wx, cy - half, cx + wx, cy + half, w, (1 - d) * (0.4f + lv) * g)
    }
}

/** Three arms turning, each square sized by its band. */
private fun DrawScope.spiral(st: VisState, k: Kit, g: Float) {
    val cx = size.width / 2; val cy = size.height / 2
    val m = min(cx, cy)
    for (arm in 0 until 3) for (i in 0 until 40) {
        val t = i / 40f
        val a = st.spin + arm * 2.0944f + t * 4 * PI.toFloat()
        val lv = st.at(t) * g
        dot(heat(k.p, lv), cx + cos(a) * t * m, cy + sin(a) * t * m, (1.5f + lv * 7).dp.toPx(), 0.4f + lv)
    }
}

/** A turning double helix, its width set by the bands along it. */
private fun DrawScope.helix(st: VisState, k: Kit, g: Float) {
    val step = 6.dp.toPx()
    val n = (size.width / step).toInt()
    val mid = size.height / 2
    for (i in 0..n) {
        val x = i * step
        val f = x / size.width
        val ph = f * 4 * PI.toFloat() + st.spin * 2
        val amp = size.height * 0.4f * (0.3f + 0.7f * st.at(f) * g)
        val y1 = mid + sin(ph) * amp
        val y2 = mid - sin(ph) * amp
        val z = cos(ph)
        if (i % 3 == 0) drawLine(k.p.fg, Offset(x, y1), Offset(x, y2), 1.5.dp.toPx(), alpha = 0.35f * g)
        dot(k.p.green, x, y1, (4.5f + 2.5f * z).dp.toPx(), (0.65f + 0.35f * z) * g)
        dot(k.p.red, x, y2, (4.5f - 2.5f * z).dp.toPx(), (0.65f - 0.35f * z) * g)
    }
}

// --- field modes -------------------------------------------------------------------

/** Demoscene plasma in big pixels, its colors cycling with the music. */
private fun DrawScope.plasma(st: VisState, k: Kit, g: Float) {
    val t = 12.dp.toPx()
    val cols = (size.width / t).toInt() + 1
    val rows = (size.height / t).toInt() + 1
    val tt = st.spin * 2
    val a = (0.35f + st.energy * 1.6f).coerceAtMost(1f) * g
    for (c in 0 until cols) for (r in 0 until rows) {
        val dx = c - cols / 2f; val dy = r - rows / 2f
        val v = sin(c * 0.35f + tt) + sin(r * 0.45f - tt * 1.3f) + sin((c + r) * 0.25f + tt * 0.7f) + sin(sqrt(dx * dx + dy * dy) * 0.5f - tt * 2)
        val f = ((v + 4) / 8 + st.time * 0.05f) % 1f * 4
        val col = when (f.toInt()) {
            0 -> mix(k.p.green, k.p.yellow, f)
            1 -> mix(k.p.yellow, k.p.red, f - 1)
            2 -> mix(k.p.red, k.p.accent, f - 2)
            else -> mix(k.p.accent, k.p.green, f - 3)
        }
        cell(col, c * t, r * t, t, t, a)
    }
}

/** Curtains of northern lights drifting, their length set by the bands. */
private fun DrawScope.aurora(st: VisState, k: Kit, g: Float) {
    val w = 6.dp.toPx()
    val n = (size.width / w).toInt() + 1
    for (i in 0 until n) {
        val f = i / n.toFloat()
        val h = size.height * (0.25f + 0.6f * st.at(f) * g) * (0.75f + 0.25f * sin(f * 9 + st.time * 1.3f))
        val top = size.height * 0.08f + sin(f * 5 + st.time * 0.6f) * size.height * 0.08f
        val q = h / 4
        val tip = mix(k.p.green, k.p.accent, 0.5f + 0.5f * sin(f * 3 + st.time * 0.4f))
        cell(tip, i * w, top, w, q, 0.8f * g)
        cell(k.p.green, i * w, top + q, w, q, 0.55f * g)
        cell(k.p.green, i * w, top + 2 * q, w, q, 0.3f * g)
        cell(k.p.green, i * w, top + 3 * q, w, q, 0.12f * g)
    }
}

/** An 80s grid: a striped sun on the horizon, the spectrum as hills, the floor rushing by. */
private fun DrawScope.retro(st: VisState, k: Kit, g: Float) {
    val hz = size.height * 0.58f
    val cx = size.width / 2
    val r = size.height * 0.36f * (1 + st.bass * g * 0.12f)
    val band = 4.dp.toPx()
    var y = hz - r
    while (y < hz) {
        val dy = hz - y
        val half = sqrt(max(0f, r * r - dy * dy))
        val f = 1 - dy / r
        val th = band * (0.4f + 0.6f * (1 - f))
        cell(mix(k.p.yellow, k.p.red, f), cx - half, y, 2 * half, th, g)
        y += band
    }
    val n = 64
    val sw = size.width / n
    for (i in 0 until n) {
        val h = st.at(abs(i / (n / 2f) - 1f)) * g * size.height * 0.2f
        cell(mix(k.p.bg, k.p.accent, 0.6f), i * sw, hz - h, sw + 0.5f, h)
    }
    val lw = 1.5.dp.toPx()
    cell(k.p.accent, 0f, hz, size.width, lw, g)
    for (i in -10..10) {
        drawLine(k.p.red, Offset(cx + i * size.width * 0.04f, hz), Offset(cx + i * size.width * 0.25f, size.height), lw, alpha = 0.7f * g)
    }
    val phase = (st.spin * 1.2f) % 1f
    for (i in 0 until 8) {
        val z = (i + 1 - phase) / 8f
        cell(k.p.red, 0f, hz + (size.height - hz) * z * z, size.width, lw, 0.7f * g * z)
    }
}

// --- particle modes ------------------------------------------------------------------

/** Flames rising from the bottom, hot yellow under red tips, sparks flying off. */
private fun DrawScope.flame(st: VisState, k: Kit, g: Float) {
    val cw = 7.dp.toPx()
    val n = max(1, (size.width / cw).toInt())
    val rows = (size.height / cw).toInt()
    val tick = (st.time * 12).toInt()
    for (i in 0 until n) {
        val lv = st.at(frac(i, n)) * g * (0.7f + 0.3f * hash01(i, tick, 3))
        val lit = (lv * rows).toInt()
        for (r in 0 until lit) {
            val f = r / max(1f, lit.toFloat())
            val c = if (f < 0.35f) mix(k.p.brightFg, k.p.yellow, f / 0.35f) else mix(k.p.yellow, k.p.red, (f - 0.35f) / 0.65f)
            cell(c, i * cw, size.height - (r + 1) * cw, cw - 1, cw - 1, if (r == lit - 1) 0.5f else 1f)
        }
    }
    for (i in 0 until st.count) dot(k.p.yellow, st.px[i] * size.width, st.py[i] * size.height, 2.5.dp.toPx(), (1 - st.life[i] / st.ttl[i]) * g)
}

/** Drops falling in columns, lit where they fall through a bar. */
private fun DrawScope.rain(st: VisState, k: Kit, g: Float) {
    val n = max(1, (size.width / 12.dp.toPx()).toInt())
    val w = size.width / n
    for (i in 0 until n) {
        val h = st.at(frac(i, n)) * g * size.height
        if (h > 0.5f) drawRect(k.bars, Offset(i * w, size.height - h), Size(w - 3.dp.toPx(), h), alpha = 0.18f)
    }
    val dw = 2.dp.toPx(); val dh = 9.dp.toPx()
    for (j in 0 until st.count) {
        val c = (st.px[j] * n).toInt().coerceIn(0, n - 1)
        val lv = st.at(frac(c, n)) * g
        val inBar = st.py[j] > 1 - lv
        cell(if (inBar) heat(k.p, 1 - st.py[j]) else k.p.fg, c * w + w / 2 - 1.5.dp.toPx() - dw / 2, st.py[j] * size.height - dh, dw, dh, if (inBar) g else 0.4f * g)
    }
}

/** Green glyph trails falling down columns, faster where the band is loud. */
private fun DrawScope.matrix(st: VisState, k: Kit, g: Float) {
    val cs = 10.dp.toPx()
    val n = min(VisState.COLS, (size.width / cs).toInt())
    val x0 = (size.width - n * cs) / 2
    val rows = (size.height / cs).toInt()
    val sub = cs / 3
    val tick = (st.time * 5).toInt()
    for (c in 0 until n) {
        val head = (st.heads[c] * rows).toInt()
        val len = 5 + (st.at(frac(c, n)) * 14).toInt()
        for (j in 0..len) {
            val r = head - j
            if (r < 0 || r >= rows) continue
            val col = if (j == 0) k.p.brightFg else k.p.green
            val a = (1 - j / (len + 1f)) * g
            // A glyph: four of the nine sub-squares, picked by a hash that changes now and then.
            for (b in 0 until 9) {
                if (hash01(c * 31 + r, b, tick + c) < 0.45f) cell(col, x0 + c * cs + (b % 3) * sub, r * cs + (b / 3) * sub, sub - 1, sub - 1, a)
            }
        }
    }
}

/** Scatter's sparkles in the bars, or Geyser's fountain from the bass. */
private fun DrawScope.sparks(st: VisState, k: Kit, g: Float) {
    val s = 4.dp.toPx()
    for (i in 0 until st.count) {
        dot(heat(k.p, 1 - st.py[i]), st.px[i] * size.width, st.py[i] * size.height, s, (1 - st.life[i] / st.ttl[i]) * g)
    }
}

private fun pick(p: CliampPalette, i: Int) = when (i) { 0 -> p.green; 1 -> p.yellow; 2 -> p.red; else -> p.accent }

/** Bursts on the beat, falling and fading. */
private fun DrawScope.firework(st: VisState, k: Kit, g: Float) {
    val s = 4.dp.toPx()
    val aspect = size.height / size.width
    for (i in 0 until st.count) {
        val x = 0.5f + (st.px[i] - 0.5f) * aspect.coerceAtMost(1f)
        val f = st.life[i] / st.ttl[i]
        dot(pick(k.p, st.kind[i]), x * size.width, st.py[i] * size.height, s * (1.4f - f), (1 - f * f) * g)
    }
}

/** Rings rising and wobbling, bigger from louder bands. */
private fun DrawScope.bubbles(st: VisState, k: Kit, g: Float) {
    for (i in 0 until st.count) {
        val r = (3 + st.size[i] * 9).dp.toPx()
        val x = st.px[i] * size.width + sin(st.time * 2 + i) * 4.dp.toPx()
        val y = st.py[i] * size.height
        drawCircle(pick(k.p, st.kind[i]), r, Offset(x, y), alpha = 0.8f * g, style = k.thin)
        dot(k.p.brightFg, x - r * 0.35f, y - r * 0.35f, 2.dp.toPx(), 0.7f * g)
    }
}

/** Petals drifting down and swaying, more of them when loud. */
private fun DrawScope.sakura(st: VisState, k: Kit, g: Float) {
    val pink = mix(k.p.red, k.p.brightFg, 0.55f)
    for (i in 0 until st.count) {
        val s = (5 + st.size[i] * 4).dp.toPx()
        val x = (st.px[i] + sin(st.time * 1.5f + i) * 0.02f) * size.width
        val y = st.py[i] * size.height
        val c = if (st.kind[i] == 0) k.p.brightFg else pink
        cell(c, x - s / 2, y - s / 4, s, s / 2, 0.9f * g)
        cell(c, x - s / 4, y - s / 2, s / 2, s, 0.9f * g)
    }
}

/** Fireflies over a meadow that sways with the bands. */
private fun DrawScope.firefly(st: VisState, k: Kit, g: Float) {
    val n = max(1, (size.width / 4.dp.toPx()).toInt())
    val w = size.width / n
    for (i in 0 until n) {
        val h = size.height * (0.06f + 0.22f * st.at(frac(i, n)) * g) * (0.7f + 0.3f * hash01(i, 0, 9))
        cell(k.p.green, i * w, size.height - h, w - 1, h, 0.45f)
    }
    for (i in 0 until st.count) {
        val b = sin(PI.toFloat() * st.life[i] / st.ttl[i]) * (0.3f + st.at(st.px[i]) * 1.4f) * g
        val x = st.px[i] * size.width; val y = st.py[i] * size.height
        drawCircle(k.p.yellow, 7.dp.toPx(), Offset(x, y), alpha = (b * 0.25f).coerceIn(0f, 1f))
        dot(k.p.yellow, x, y, 3.dp.toPx(), b)
    }
}

/** Flight through stars: faster and streaking when loud. */
private fun DrawScope.starfield(st: VisState, k: Kit, g: Float) {
    val cx = size.width / 2; val cy = size.height / 2
    val m = max(cx, cy)
    val lw = 1.5.dp.toPx()
    for (i in 0 until VisState.STARS) {
        val z = st.sz[i]
        val x = cx + st.sx[i] / z * m * 0.5f
        val y = cy + st.sy[i] / z * m * 0.5f
        if (x < 0 || x > size.width || y < 0 || y > size.height) continue
        val c = if (i % 5 == 0) k.p.accent else k.p.brightFg
        val a = (1 - z) * g
        val z2 = z + st.speed * 0.06f
        drawLine(c, Offset(cx + st.sx[i] / z2 * m * 0.5f, cy + st.sy[i] / z2 * m * 0.5f), Offset(x, y), lw, alpha = a * 0.6f)
        dot(c, x, y, (1 + (1 - z) * 3).dp.toPx(), a)
    }
}
