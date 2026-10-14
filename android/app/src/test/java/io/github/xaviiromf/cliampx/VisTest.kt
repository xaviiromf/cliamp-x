package io.github.xaviiromf.cliampx

import io.github.xaviiromf.cliampx.ui.VisGrid
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class VisTest {
    private val esc = "\u001b"

    @Test
    fun parseFrameColorsAndCells() {
        val f = VisFrame.parse(
            "Bars",
            listOf(
                "$esc[38;5;10m▁▂$esc[m x",
                "$esc[38;2;1;2;3;48;5;9m█$esc[39m⣿$esc[0m🬀",
                "too many rows",
            ),
            4, 2,
        )
        assertEquals("Bars", f.mode)
        assertArrayEquals(intArrayOf('▁'.code, '▂'.code, ' '.code, 'x'.code, '█'.code, '⣿'.code, 0x1FB00, ' '.code), f.glyph)
        val rgb = (0xFF shl 24) or 0x010203
        assertArrayEquals(intArrayOf(11, 11, 0, 0, rgb, 0, 0, 0), f.fg)
        assertArrayEquals(intArrayOf(0, 0, 0, 0, 10, 10, 0, 0), f.bg)
    }

    @Test
    fun parseClipsLongRowsAndSkipsOtherEscapes() {
        val f = VisFrame.parse("Wave", listOf("ab$esc[2Kcdef", "$esc]"), 3, 2)
        assertArrayEquals(intArrayOf('a'.code, 'b'.code, 'c'.code, ' '.code, ' '.code, ' '.code), f.glyph)
    }

    @Test
    fun sgrCodes() {
        for ((p, want) in listOf(
            "31" to intArrayOf(2, 0),
            "97;44" to intArrayOf(16, 5),
            "100" to intArrayOf(0, 9),
            "38;5;196" to intArrayOf((0xFF shl 24) or 0xFF0000, 0),
            "38;5;232" to intArrayOf((0xFF shl 24) or 0x080808, 0),
            "38;5;3" to intArrayOf(4, 0),
            "38;2;300;0;0" to intArrayOf((0xFF shl 24) or 0xFF0000, 0),
            "38;2" to intArrayOf((0xFF shl 24), 0), // a cut truecolor reads as black, not a crash
            "1" to intArrayOf(0, 0),
        )) {
            val pen = IntArray(2)
            VisFrame.sgr(p, pen)
            assertArrayEquals(p, want, pen)
        }
        val pen = intArrayOf(5, 6)
        VisFrame.sgr("", pen)
        assertArrayEquals(intArrayOf(0, 0), pen)
    }

    @Test
    fun blockGlyphsAsRects() {
        val out = FloatArray(24)
        for ((cp, want) in listOf(
            0x2588 to listOf(0f, 0f, 1f, 1f), // █
            0x2581 to listOf(0f, 0.875f, 1f, 1f), // ▁
            0x2580 to listOf(0f, 0f, 1f, 0.5f), // ▀
            0x258F to listOf(0f, 0f, 0.125f, 1f), // ▏
            0x2590 to listOf(0.5f, 0f, 1f, 1f), // ▐
            0x2596 to listOf(0f, 0.5f, 0.5f, 1f), // ▖
            0x1FB00 to listOf(0f, 0f, 0.5f, 1 / 3f), // 🬀 sextant-1
            0x1FB3B to listOf(0.5f, 0f, 1f, 1 / 3f, 0f, 1 / 3f, 0.5f, 2 / 3f, 0.5f, 1 / 3f, 1f, 2 / 3f, 0f, 2 / 3f, 0.5f, 1f, 0.5f, 2 / 3f, 1f, 1f), // sextant-23456
        )) {
            val n = blockRects(cp, out)
            assertArrayEquals(Integer.toHexString(cp), want.toFloatArray(), out.copyOf(n * 4), 1e-6f)
        }
        assertEquals(3, blockRects(0x2599, out)) // ▙
        assertEquals(0, blockRects('a'.code, out))
        assertEquals(0, blockRects(0x2801, out)) // braille draws as dots
        assertEquals(0.5f, shade(0x2592), 0f)
        assertEquals(1f, shade(0x2588), 0f)
    }

    @Test
    fun stripCycleSkipsNone() {
        val all = listOf("Bars", "Wave", "None", "MyLua")
        assertEquals("Wave", nextVisualizer("Bars", all))
        assertEquals("MyLua", nextVisualizer("Wave", all))
        assertEquals("Bars", nextVisualizer("MyLua", all))
        assertEquals("Bars", nextVisualizer("None", all))
        assertEquals("Bars", nextVisualizer("Gone", all))
        assertEquals("Bars", nextVisualizer("Bars", emptyList()))
    }

    @Test
    fun gridFillsTheView() {
        val g = VisGrid.of(1000, 168, 36f, 1.32f, 0.6f)!!
        assertEquals(5, g.rows)
        assertEquals(33.6f, g.cellH, 1e-4f)
        assertEquals(33.6f / 1.32f * 0.6f, g.cellW, 1e-4f)
        assertEquals((1000 / g.cellW).toInt(), g.cols)
        assertEquals((1000 - g.cols * g.cellW) / 2, g.x0, 1e-3f)
        assertEquals(1, VisGrid.of(3, 3, 36f, 1.32f, 0.6f)!!.rows)
        assertNull(VisGrid.of(0, 100, 36f, 1.32f, 0.6f))
    }
}
