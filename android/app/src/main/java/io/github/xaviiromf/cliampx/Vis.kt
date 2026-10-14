package io.github.xaviiromf.cliampx

/**
 * One visualizer frame of the core (vis.frame, UI-SPEC §5.4): a code point
 * and a foreground and background color per cell, row by row. A color is
 * 0 for the default, 1..16 for terminal color 0..15 (drawn in the theme's
 * palette), or an opaque ARGB value, which is negative.
 */
class VisFrame(val mode: String, val cols: Int, val rows: Int) {
    val glyph = IntArray(cols * rows) { ' '.code }
    val fg = IntArray(cols * rows)
    val bg = IntArray(cols * rows)

    companion object {
        /**
         * The rows of vis.frame on a cols x rows grid. Each code point takes
         * one cell; SGR sets the colors and any other escape is skipped.
         * shortcut: a double-width glyph takes one cell; no mode draws one, a Lua mode could.
         */
        fun parse(mode: String, lines: List<String>, cols: Int, rows: Int): VisFrame {
            val f = VisFrame(mode, cols, rows)
            val pen = IntArray(2) // fg, bg
            for ((r, line) in lines.withIndex()) {
                if (r >= rows) break
                var c = 0
                var i = 0
                while (i < line.length) {
                    val cp = line.codePointAt(i)
                    if (cp == 0x1B) {
                        i = escape(line, i + 1, pen)
                        continue
                    }
                    i += Character.charCount(cp)
                    if (c < cols) {
                        val k = r * cols + c
                        f.glyph[k] = cp; f.fg[k] = pen[0]; f.bg[k] = pen[1]
                    }
                    c++
                }
            }
            return f
        }

        /** Skips the escape after ESC at i, applying an SGR to pen; returns the index after it. */
        private fun escape(s: String, i: Int, pen: IntArray): Int {
            if (i >= s.length) return i
            if (s[i] != '[') return i + 1
            var j = i + 1
            while (j < s.length && s[j].code !in 0x40..0x7E) j++
            if (j < s.length && s[j] == 'm') sgr(s.substring(i + 1, j), pen)
            return j + 1
        }

        /** Applies the SGR parameters p to pen: resets, the 16 colors, 256 colors and truecolor. */
        fun sgr(p: String, pen: IntArray) {
            val n = if (p.isEmpty()) listOf(0) else p.split(';').map { it.toIntOrNull() ?: 0 }
            var i = 0
            while (i < n.size) {
                when (val c = n[i]) {
                    0 -> pen.fill(0)
                    39 -> pen[0] = 0
                    49 -> pen[1] = 0
                    in 30..37 -> pen[0] = c - 29
                    in 90..97 -> pen[0] = c - 81
                    in 40..47 -> pen[1] = c - 39
                    in 100..107 -> pen[1] = c - 91
                    38, 48 -> {
                        val k = if (c == 38) 0 else 1
                        when (n.getOrNull(i + 1)) {
                            5 -> { pen[k] = color256(n.getOrElse(i + 2) { 0 }); i += 2 }
                            2 -> { pen[k] = rgb(n.getOrElse(i + 2) { 0 }, n.getOrElse(i + 3) { 0 }, n.getOrElse(i + 4) { 0 }); i += 4 }
                        }
                    }
                }
                i++
            }
        }

        private fun rgb(r: Int, g: Int, b: Int) =
            (0xFF shl 24) or (r.coerceIn(0, 255) shl 16) or (g.coerceIn(0, 255) shl 8) or b.coerceIn(0, 255)

        /** xterm's 256 colors: the 16 terminal colors, the 6x6x6 cube, then 24 grays. */
        fun color256(n: Int): Int = when {
            n < 16 -> n.coerceAtLeast(0) + 1
            n < 232 -> {
                val v = intArrayOf(0, 95, 135, 175, 215, 255)
                val i = n - 16
                rgb(v[i / 36], v[i / 6 % 6], v[i % 6])
            }
            else -> (8 + (n.coerceAtMost(255) - 232) * 10).let { rgb(it, it, it) }
        }
    }
}

/**
 * The block glyphs the visualizer draws as rectangles, so bars meet with
 * no gap in any font: block elements, quadrants and sextants. Writes left,
 * top, right, bottom in 0..1 of the cell into out and returns how many
 * rectangles, or 0 for any other glyph. Shades fill the cell: see shade.
 */
fun blockRects(cp: Int, out: FloatArray): Int {
    fun put(k: Int, l: Float, t: Float, r: Float, b: Float): Int {
        out[k * 4] = l; out[k * 4 + 1] = t; out[k * 4 + 2] = r; out[k * 4 + 3] = b
        return k + 1
    }
    fun cells(bits: Int, cols: Int, rows: Int): Int {
        var k = 0
        for (i in 0 until cols * rows) if (bits shr i and 1 == 1) {
            val x = i % cols; val y = i / cols
            k = put(k, x / cols.toFloat(), y / rows.toFloat(), (x + 1) / cols.toFloat(), (y + 1) / rows.toFloat())
        }
        return k
    }
    return when (cp) {
        0x2580 -> put(0, 0f, 0f, 1f, 0.5f)
        in 0x2581..0x2588 -> put(0, 0f, 1 - (cp - 0x2580) / 8f, 1f, 1f)
        in 0x2589..0x258F -> put(0, 0f, 0f, (0x2590 - cp) / 8f, 1f)
        0x2590 -> put(0, 0.5f, 0f, 1f, 1f)
        in 0x2591..0x2593 -> put(0, 0f, 0f, 1f, 1f)
        0x2594 -> put(0, 0f, 0f, 1f, 0.125f)
        0x2595 -> put(0, 0.875f, 0f, 1f, 1f)
        in 0x2596..0x259F -> cells(QUADRANTS[cp - 0x2596], 2, 2)
        in 0x1FB00..0x1FB3B -> {
            // The 60 sextants leave out the empty, full, left and right patterns.
            var bits = cp - 0x1FB00 + 1
            if (bits >= 21) bits++
            if (bits >= 42) bits++
            cells(bits, 2, 3)
        }
        else -> 0
    }
}

/** The alpha of a shade glyph ░▒▓, or 1. */
fun shade(cp: Int) = if (cp in 0x2591..0x2593) (cp - 0x2590) / 4f else 1f

/** Quadrants ▖▗▘▙▚▛▜▝▞▟ as bits: upper left 1, upper right 2, lower left 4, lower right 8. */
private val QUADRANTS = intArrayOf(4, 8, 1, 13, 9, 7, 11, 2, 6, 14)

/** The next mode of the strip's cycle (§2.3) in the core's order, None left out. */
fun nextVisualizer(current: String, all: List<String>): String {
    val modes = all.filter { it != "None" }
    if (modes.isEmpty()) return current
    return modes[(modes.indexOf(current) + 1) % modes.size] // an unknown mode goes to the first
}
