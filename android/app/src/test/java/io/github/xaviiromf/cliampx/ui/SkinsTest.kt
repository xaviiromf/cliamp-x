package io.github.xaviiromf.cliampx.ui

import androidx.compose.ui.graphics.Color
import io.github.xaviiromf.cliampx.R
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import java.io.IOException

class SkinsTest {
    private fun skinJson(name: String = "Mi skin", theme: String = "nord", font: String = "vt323", bg: String = "Crt", vis: String = "Bars") =
        JSONObject().put("name", name).put("theme", theme).put("font", font).put("background", bg)
            .put("background_lists", 30).put("background_player", 60).put("visualizer", vis)

    @Test
    fun builtInSkins() {
        assertTrue(Skins.BUILT_IN.size >= 8)
        assertEquals(Skins.BUILT_IN.size, Skins.BUILT_IN.map { it.name }.toSet().size)
        for (s in Skins.BUILT_IN) {
            assertTrue(s.name, s.label != 0)
            // A built-in skin survives its own export and import.
            assertEquals(s.name, s.copy(label = 0), Skin.parse(s.json()))
        }
    }

    @Test
    fun parseSkin() {
        val cases = listOf(
            skinJson() to true,
            skinJson(vis = "") to true,
            skinJson(name = "") to false,
            skinJson(name = "x".repeat(41)) to false,
            skinJson(name = "a\u0007b") to false,
            skinJson(theme = "../etc") to false,
            skinJson(font = "../font") to false,
            skinJson(bg = "Nope") to false,
            skinJson(vis = "a;b") to false,
            JSONObject() to false,
        )
        for ((o, ok) in cases) assertEquals(o.toString(), ok, Skin.parse(o) != null)
        val levels = Skin.parse(skinJson().put("background_lists", 500).put("background_player", -3))!!
        assertEquals(100, levels.listLevel)
        assertEquals(0, levels.playerLevel)
        // A repeated name keeps the first; a bad entry is left out.
        val all = Skin.parseAll(JSONArray().put(skinJson(theme = "nord")).put(skinJson(theme = "matrix")).put("x").put(skinJson(name = "Otro")))
        assertEquals(listOf("Mi skin" to "nord", "Otro" to "nord"), all.map { it.name to it.theme })
        assertEquals(emptyList<Skin>(), Skin.parseAll(null))
    }

    @Test
    fun themeNames() {
        val cases = listOf(
            "Mi tema" to null,
            "nord-2.1_b" to null,
            "" to R.string.theme_bad_name,
            " lead" to R.string.theme_bad_name,
            ".hidden" to R.string.theme_bad_name,
            "../up" to R.string.theme_bad_name,
            "a/b" to R.string.theme_bad_name,
            "x".repeat(41) to R.string.theme_bad_name,
            "Cover" to R.string.theme_reserved,
            "default" to R.string.theme_reserved,
            "list" to R.string.theme_reserved,
        )
        for ((name, want) in cases) assertEquals(name, want, themeNameError(name))
    }

    @Test
    fun themeFiles() {
        val colors = listOf("#101010", "#FF8800", "#eeeeee", "#aaaaaa", "#00ff00", "#ffff00", "#ff0000")
        val t = UserTheme.of("Mi tema", colors)!!
        assertEquals("#ff8800", t.colors[1])
        assertEquals(t, UserTheme.parse("Mi tema", t.toml()))
        // The core's forms: single quotes, unquoted with a comment, keys in any order.
        val toml = "red = '#ff0000'\nbg = #101010 # dark\naccent=\"#ff8800\"\nbright_fg = \"#eeeeee\"\nfg = \"#aaaaaa\"\ngreen = \"#00ff00\"\nyellow = \"#ffff00\"\n"
        assertEquals(t, UserTheme.parse("Mi tema", toml))
        assertNull(UserTheme.parse("Mi tema", toml.replace("red = '#ff0000'", "")))
        assertNull(UserTheme.of("Mi tema", colors.dropLast(1)))
        assertNull(UserTheme.of("Mi tema", colors.dropLast(1) + "red"))
        assertNull(UserTheme.of("../x", colors))
    }

    @Test
    fun hsvRoundTrip() {
        for (c in listOf(Color(0xFFFF8800), Color(0xFF123456), Color.Black, Color.White, Color(0xFF808080))) {
            assertEquals(hexOf(c), hexOf(hsvOf(c).color()))
        }
    }

    @Test
    fun wallpaperThemeIsReadable() {
        val seeds = listOf(
            listOf(Color(0xFF3355AA), Color(0xFFFFCC00)),
            listOf(Color.White),
            listOf(Color.Black, Color(0xFF00FF00)),
            emptyList(),
        )
        for (s in seeds) {
            val t = seedTheme(s)
            assertEquals(7, t.size)
            for (c in t.drop(1)) assertTrue("$s ${hexOf(c)}", contrast(c, t[0]) >= 4.5f)
        }
    }

    @Test
    fun personalizationFile() {
        val theme = UserTheme.of("Mi tema", listOf("#101010", "#ff8800", "#eeeeee", "#aaaaaa", "#00ff00", "#ffff00", "#ff0000"))!!
        val skin = Skin.parse(skinJson())!!
        val out = Personalization(skin, listOf(skin), listOf(theme), 2, 0, true, false, MotionSpeed.Slow).json().toString()
        val back = Personalization.parse(out)
        assertEquals(skin, back.current)
        assertEquals(listOf(skin), back.skins)
        assertEquals(listOf(theme), back.themes)
        assertEquals(2, back.textSize)
        assertEquals(0, back.density)
        assertEquals(true, back.rounded)
        assertEquals(false, back.haptics)
        assertEquals(MotionSpeed.Slow, back.motion)

        // Missing or bad fields keep the current value (null).
        val partial = Personalization.parse(JSONObject().put("format", Personalization.FORMAT).put("text_size", 9).put("density", "1").put("motion", "Fast").toString())
        assertNull(partial.current)
        assertNull(partial.textSize)
        assertNull(partial.density)
        assertNull(partial.motion)
        assertNotNull(partial.skins)

        for (bad in listOf("{}", "[]", "not json", JSONObject().put("format", "other").toString())) {
            try {
                Personalization.parse(bad)
                fail(bad)
            } catch (_: Exception) {
            }
        }
    }

    @Test
    fun readLimited() {
        assertEquals("añ", Personalization.readLimited("añ".toByteArray().inputStream()))
        try {
            Personalization.readLimited(ByteArray(Personalization.MAX_BYTES + 1).inputStream())
            fail()
        } catch (_: IOException) {
        }
    }
}
