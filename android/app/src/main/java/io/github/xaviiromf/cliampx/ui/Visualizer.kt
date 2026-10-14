package io.github.xaviiromf.cliampx.ui

import android.graphics.Paint
import android.graphics.Typeface
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.tween
import androidx.compose.foundation.layout.Spacer
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.graphics.drawscope.drawIntoCanvas
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import androidx.core.content.res.ResourcesCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.repeatOnLifecycle
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Snapshot
import io.github.xaviiromf.cliampx.VisFrame
import io.github.xaviiromf.cliampx.blockRects
import io.github.xaviiromf.cliampx.shade
import kotlin.math.floor
import kotlin.math.min
import kotlin.math.roundToInt

/** A cell grid that fills a view: cols x rows cells of cellW x cellH px in a font of textSize px, offset x0 to center it. */
data class VisGrid(val cols: Int, val rows: Int, val cellW: Float, val cellH: Float, val textSize: Float, val x0: Float) {
    companion object {
        /**
         * The grid for a w x h px view and cells about target px tall, in a
         * font whose line is lineK and whose advance is advK of its size.
         * The rows fill the height exactly; the columns are whole cells.
         */
        fun of(w: Int, h: Int, target: Float, lineK: Float, advK: Float): VisGrid? {
            if (w <= 0 || h <= 0 || target <= 0f) return null
            val rows = (h / target).roundToInt().coerceAtLeast(1)
            val cellH = h / rows.toFloat()
            val size = cellH / lineK
            val cellW = size * advK
            val cols = floor(w / cellW).toInt().coerceAtLeast(1)
            return VisGrid(cols, rows, cellW, cellH, size, (w - cols * cellW) / 2)
        }
    }
}

/** The 16 terminal colors in the theme's palette, for the Default theme's frames. */
private fun terminalColors(p: CliampPalette) = listOf(
    p.bg, p.red, p.green, p.yellow, p.accent, p.accent, p.accent, p.fg,
    p.fg, p.red, p.green, p.yellow, p.accent, p.accent, p.accent, p.brightFg,
).map { it.toArgb() }.toIntArray()

/**
 * The core's own visualizer (UI-SPEC §0, §5.4): each vis.frame frame drawn
 * cell by cell on one Canvas in the monospace font, so every mode looks as
 * in Modo terminal. Frames come only while this is composed, the screen
 * started and the song playing, at most 30 per second (10 under reduced
 * motion); then the last frame fades out in 300 ms.
 */
@Composable
fun Visualizer(s: Snapshot, modifier: Modifier = Modifier) {
    val p = LocalPalette.current
    val context = LocalContext.current
    val target = with(LocalDensity.current) { 12.dp.toPx() }
    val paints = remember {
        val font = ResourcesCompat.getFont(context, R.font.jetbrains_mono_regular) ?: Typeface.MONOSPACE
        Paint(Paint.ANTI_ALIAS_FLAG).apply { typeface = font } to Paint() // text, and cells with no seams
    }
    var size by remember { mutableStateOf(IntSize.Zero) }
    val grid = remember(size) {
        val text = paints.first
        text.textSize = 100f
        val m = text.fontMetrics
        VisGrid.of(size.width, size.height, target, (m.descent - m.ascent) / 100f, text.measureText("M") / 100f)
    }
    // Read only in the draw lambda: a frame redraws without recomposing.
    var frame by remember { mutableStateOf<VisFrame?>(null) }
    val gain = remember { Animatable(1f) }
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    val live = s.playing && !s.buffering
    val reduced = reducedMotion()
    LaunchedEffect(live, grid, reduced) {
        if (!live || grid == null) {
            gain.animateTo(0f, tween(300, easing = LinearEasing))
            return@LaunchedEffect
        }
        gain.snapTo(1f)
        lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) {
            Remote.visFrames(grid.cols, grid.rows, if (reduced) 100L else 33L).collect { frame = it }
        }
    }
    val colors = remember(p) { terminalColors(p) }
    val rects = remember { FloatArray(6 * 4) }
    val chars = remember { CharArray(2) }
    Spacer(
        modifier.onSizeChanged { size = it }.drawBehind {
            val f = frame ?: return@drawBehind
            val g = grid ?: return@drawBehind
            val a = gain.value
            if (a <= 0f || f.cols != g.cols || f.rows != g.rows) return@drawBehind
            val (text, cell) = paints
            text.textSize = g.textSize
            val base = -text.fontMetrics.ascent
            val defaultFg = p.brightFg.toArgb()
            fun color(c: Int, default: Int) = when {
                c == 0 -> default
                c in 1..16 -> colors[c - 1]
                else -> c
            }
            val alpha = (a * 255).roundToInt()
            drawIntoCanvas { cv ->
                val nc = cv.nativeCanvas
                // Edges on whole pixels, so cells tile with no seam or overlap.
                fun rect(l: Float, t: Float, r: Float, b: Float) =
                    nc.drawRect(l.roundToInt().toFloat(), t.roundToInt().toFloat(), r.roundToInt().toFloat(), b.roundToInt().toFloat(), cell)
                for (r in 0 until f.rows) for (c in 0 until f.cols) {
                    val k = r * f.cols + c
                    val x = g.x0 + c * g.cellW
                    val y = r * g.cellH
                    if (f.bg[k] != 0) {
                        cell.color = color(f.bg[k], 0); cell.alpha = alpha
                        rect(x, y, x + g.cellW, y + g.cellH)
                    }
                    val cp = f.glyph[k]
                    if (cp == ' '.code || cp == 0x2800) continue
                    val fg = color(f.fg[k], defaultFg)
                    if (cp in 0x2801..0x28FF) {
                        // Braille: eight dots in two columns of four.
                        cell.color = fg; cell.alpha = alpha
                        val d = min(g.cellW / 2, g.cellH / 4) * 0.62f
                        val bits = cp - 0x2800
                        for (i in 0 until 8) if (bits shr i and 1 == 1) {
                            val col = if (i == 3 || i == 4 || i == 5 || i == 7) 1 else 0
                            val row = when (i) { 0, 3 -> 0; 1, 4 -> 1; 2, 5 -> 2; else -> 3 }
                            val cx = x + (col * 2 + 1) * g.cellW / 4
                            val cy = y + (row * 2 + 1) * g.cellH / 8
                            rect(cx - d / 2, cy - d / 2, cx + d / 2, cy + d / 2)
                        }
                        continue
                    }
                    val n = blockRects(cp, rects)
                    if (n > 0) {
                        cell.color = fg; cell.alpha = (alpha * shade(cp)).roundToInt()
                        for (i in 0 until n) {
                            rect(
                                x + rects[i * 4] * g.cellW, y + rects[i * 4 + 1] * g.cellH,
                                x + rects[i * 4 + 2] * g.cellW, y + rects[i * 4 + 3] * g.cellH,
                            )
                        }
                        continue
                    }
                    text.color = fg; text.alpha = alpha
                    nc.drawText(chars, 0, Character.toChars(cp, chars, 0), x, y + base, text)
                }
            }
        },
    )
}
