package io.github.xaviiromf.cliampx.ui

import androidx.compose.ui.graphics.Color
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class BackgroundsTest {
    @Test
    fun everyBackgroundButSolidHasAShader() {
        assertTrue(Background.entries.size >= 17)
        for (b in Background.entries) {
            val src = shaderSource(b)
            if (b == Background.Solid) {
                assertNull(src)
                continue
            }
            src!!
            assertTrue(b.name, "half3 paint(" in src && "half4 main(" in src)
            // AGSL rejects unbalanced code only on the device: catch the plain slips here.
            assertEquals(b.name, src.count { it == '{' }, src.count { it == '}' })
            assertEquals(b.name, src.count { it == '(' }, src.count { it == ')' })
        }
    }

    @Test
    fun storedNames() {
        val cases = listOf(
            "Matrix" to Background.Matrix,
            "Tron" to Background.Tron,
            null to Background.Solid,
            "matrix" to Background.Solid,
            "Gone" to Background.Solid,
        )
        for ((name, want) in cases) assertEquals(name.toString(), want, enumOr(name, Background.Solid))
        assertEquals(MotionSpeed.Off, enumOr("Off", MotionSpeed.Normal))
    }

    @Test
    fun contrastRatio() {
        assertEquals(21f, contrast(Color.Black, Color.White), 1e-3f)
        assertEquals(1f, contrast(Color.Red, Color.Red), 1e-6f)
        assertEquals(5.3f, contrast(hexColor("#a89984")!!, hexColor("#282828")!!), 0.05f)
    }

    @Test
    fun scrimKeepsTextContrast() {
        val palettes = mapOf(
            "gruvbox" to palette("#282828", "#7daea3", "#d4be98", "#a89984", "#a9b665", "#d8a657", "#ea6962"),
            "catppuccin-latte" to palette("#eff1f5", "#1a5cc8", "#4c4f69", "#686b80", "#287a16", "#956400", "#d20f39"),
            "amber" to palette("#0c0700", "#ffb000", "#ffd27a", "#c98a1c", "#d08a00", "#ffb000", "#ffcf5c"),
            "ayu-light" to palette("#fcfcfc", "#9c681f", "#5c6166", "#6e747a", "#5e7d00", "#946a2d", "#b45555"),
            "winamp" to Winamp,
        )
        for ((name, p) in palettes) {
            // Whatever a shader draws, from the palette or past it.
            val drawn = listOf(p.bg, p.accent, p.fg, p.green, p.yellow, p.red, Color.White, Color.Black)
            var last = 0f
            for (level in listOf(0, 40, 70, 100)) {
                val s = scrimOf(Background.Plasma, level, p)
                if (level == 0) assertEquals(name, 0f, s.shown) else assertTrue("$name $level shows more", s.shown > last)
                last = s.shown
                for (text in listOf(p.fg, p.brightFg)) {
                    val want = minOf(4.5f, contrast(text, p.bg))
                    for (x in drawn) {
                        val got = contrast(text, mixSrgb(s.color, x, s.shown))
                        assertTrue("$name $level $text over $x: $got < $want", got >= want - 1e-3f)
                    }
                }
            }
            assertEquals(name, Scrim(p.bg, 0f), scrimOf(Background.Solid, 100, p))
        }
    }

    private fun palette(vararg hex: String): CliampPalette {
        val c = hex.map { hexColor(it)!! }
        return CliampPalette(c[0], c[1], c[2], c[3], c[4], c[5], c[6])
    }

    @Test
    fun motionDurations() {
        val cases = listOf(
            Triple(300, MotionSpeed.Normal, 300),
            Triple(300, MotionSpeed.Slow, 480),
            Triple(250, MotionSpeed.Off, null),
        )
        for ((ms, m, want) in cases) assertEquals("$ms $m", want, Motion.durationOf(ms, m))
    }
}
