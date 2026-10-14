package io.github.xaviiromf.cliampx.ui

import io.github.xaviiromf.cliampx.ThemeInfo
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

class ArtUrlTest {
    private val song = "https://yt3.googleusercontent.com/qrY7xjd=w400-h400-l90-rj"

    @Test
    fun googleSizes() {
        assertEquals(listOf("https://yt3.googleusercontent.com/qrY7xjd=w1080-h1080-l90-rj"), artUrls(song, 1080))
        assertEquals(listOf("https://lh3.googleusercontent.com/x=w126-h126-l90-rj"), artUrls("https://lh3.googleusercontent.com/x=w400-h400-p-l90-rj", 126))
        assertEquals(listOf("https://yt3.ggpht.com/a=w1200-h1200-l90-rj"), artUrls("https://yt3.ggpht.com/a=s88-c-k-c0x00ffffff-no-rj", 4000))
        assertEquals(listOf("https://lh3.googleusercontent.com/x=w512-h512-l90-rj"), artUrls("https://lh3.googleusercontent.com/x", 512))
    }

    @Test
    fun ytimgVariants() {
        val hq = "https://i.ytimg.com/vi/Smi8vAbm_8o/hqdefault.jpg?sqp=-oaymwEa&rs=AOn4"
        fun vi(name: String) = "https://i.ytimg.com/vi/Smi8vAbm_8o/$name.jpg"
        assertEquals(listOf(vi("mqdefault")), artUrls(hq, 320))
        assertEquals(listOf(vi("maxresdefault"), vi("sddefault"), vi("hqdefault")), artUrls(hq, 321))
        assertEquals(listOf(vi("sddefault")), artUrls("https://i.ytimg.com/vi_webp/Smi8vAbm_8o/maxresdefault.webp", 1080, "sddefault"))
        assertEquals(listOf("https://example.com/a.png"), artUrls("https://example.com/a.png", 1080))
    }

    @Test
    fun squares() {
        assertArrayEquals(intArrayOf(280, 0, 720), artSquare(1280, 720, false)) // maxres 16:9
        assertArrayEquals(intArrayOf(105, 45, 270), artSquare(480, 360, true)) // hq: the 16:9 band, then its square
        assertArrayEquals(intArrayOf(140, 60, 360), artSquare(640, 480, true)) // sd
        assertArrayEquals(intArrayOf(0, 0, 544), artSquare(544, 544, false))
    }
}

class PaletteTest {
    private fun theme(bg: String = "#1a1b26", accent: String = "#7aa2f7") =
        ThemeInfo("tokyo-night", bg, accent, "#c0caf5", "#a9b1d6", "#9ece6a", "#e0af68", "#f7768e")

    @Test
    fun parses() {
        assertEquals(Color(0xFF1A1B26), hexColor("#1a1b26"))
        assertEquals(Color(0xFF00FF00), hexColor("00FF00"))
        assertNull(hexColor("#12345"))
        assertNull(hexColor(""))
        val p = paletteOf(theme())
        assertEquals(Color(0xFF7AA2F7), p.accent)
        assertEquals(Color(0xFFF7768E), p.red)
    }

    @Test
    fun defaultAndMissingColorsAreWinamp() {
        assertEquals(Winamp, paletteOf(null))
        assertEquals(Winamp, paletteOf(ThemeInfo("Default - Terminal colors", "", "", "", "", "", "", "")))
        assertEquals(Winamp, paletteOf(theme(bg = "")))
    }

    @Test
    fun derivedRoles() {
        assertEquals(Color.Black, Winamp.onAccent) // #00ff00 is light
        assertEquals(Color.White, paletteOf(theme(accent = "#3b4261")).onAccent)
        val s = Winamp.surface // 5 % of white over black
        assertEquals(0.05f, s.red, 0.01f)
        assertEquals(0.09f, Winamp.surfaceHigh.green, 0.01f)
        assertEquals(0.12f, Winamp.pressed.alpha, 0.005f) // 8-bit alpha
    }
}

class GlyphTest {
    @Test
    fun everyTextGlyphIsInJetBrainsMono() {
        // java.awt is not on the Android compile classpath, only on the test JVM.
        val awt = Class.forName("java.awt.Font")
        val font = awt.getMethod("createFont", Int::class.javaPrimitiveType, File::class.java)
            .invoke(null, 0, File("src/main/res/font/jetbrains_mono_regular.ttf")) // 0: TRUETYPE_FONT
        val canDisplay = awt.getMethod("canDisplay", Char::class.javaPrimitiveType)
        val missing = Glyphs.ALL.filterNot { canDisplay.invoke(font, it) as Boolean }
        assertTrue("missing: $missing", missing.isEmpty())
    }
}

class PlayerArtTest {
    @Test
    fun neverNegative() {
        // Portrait, the height left inside the insets: 343 dp art and the strip (§3.9).
        assertEquals(343.dp to true, playerArt(411.dp, 763.dp))
        for ((w, h) in listOf(411.dp to 891.dp, 891.dp to 411.dp, 891.dp to 300.dp, 20.dp to 20.dp, 0.dp to 0.dp)) {
            val (art, strip) = playerArt(w, h)
            assertTrue("$w × $h: $art", art >= 0.dp)
            assertTrue("$w × $h: $art", art <= w)
            if (strip) assertTrue(art >= 240.dp)
        }
        assertEquals(63.dp to false, playerArt(891.dp, 411.dp)) // a wide, short split-screen window: 411 − 420 + 72
    }
}
