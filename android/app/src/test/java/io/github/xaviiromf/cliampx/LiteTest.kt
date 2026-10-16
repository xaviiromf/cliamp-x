package io.github.xaviiromf.cliampx

import io.github.xaviiromf.cliampx.ui.Background
import io.github.xaviiromf.cliampx.ui.Looks
import io.github.xaviiromf.cliampx.ui.MotionSpeed
import org.junit.Assert.assertEquals
import org.junit.Test

class LiteTest {
    private val arm = listOf("arm64-v8a", "armeabi-v7a")
    private val gib = 1L shl 30

    @Test
    fun weakPhoneRule() {
        // lowRam, totalMem, abis, max kHz of each core -> weak
        data class Case(val name: String, val lowRam: Boolean, val mem: Long, val abis: List<String>, val khz: List<Long>, val weak: Boolean)
        for (c in listOf(
            Case("flagship", false, 8 * gib, arm, listOf(3_000_000, 2_000_000), false),
            Case("exactly 2 GHz is fast enough", false, 6 * gib, arm, listOf(2_000_000, 1_800_000), false),
            Case("Moto G20 (T700, 4 GB reports 3.7)", false, 3_700_000_000L, arm, listOf(1_800_000, 1_800_000), true),
            Case("flagged low RAM", true, 8 * gib, arm, listOf(3_000_000), true),
            Case("under 4 GiB", false, 3 * gib, arm, listOf(2_800_000), true),
            Case("slow clock only", false, 6 * gib, arm, listOf(1_800_000, 1_500_000), true),
            Case("32-bit only", false, 6 * gib, listOf("armeabi-v7a"), listOf(2_800_000), true),
            Case("x86_64 emulator", false, 8 * gib, listOf("x86_64"), emptyList(), false),
            Case("unreadable clock and RAM are ignored", false, 0, arm, listOf(0, 0), false),
        )) {
            assertEquals(c.name, c.weak, Lite.isWeak(c.lowRam, c.mem, c.abis, c.khz))
        }
    }

    @Test
    fun modeDecidesWithDetection() {
        assertEquals(true, Lite.isOn(LiteMode.On, false))
        assertEquals(false, Lite.isOn(LiteMode.Off, true))
        assertEquals(true, Lite.isOn(LiteMode.Auto, true))
        assertEquals(false, Lite.isOn(LiteMode.Auto, false))
    }

    @Test
    fun overridesKeepAndRestoreTheUsersChoices() {
        Looks.putBackground(Background.Matrix)
        Looks.putMotion(MotionSpeed.Slow)
        Looks.putPlayerVis(true)
        Lite.mode = LiteMode.Off
        assertEquals(Background.Matrix, Looks.shownBackground)
        assertEquals(MotionSpeed.Slow, Looks.activeMotion)
        assertEquals(true, Looks.showPlayerVis)
        Lite.mode = LiteMode.On
        assertEquals(Background.Solid, Looks.shownBackground)
        assertEquals(MotionSpeed.Off, Looks.activeMotion)
        assertEquals(false, Looks.showPlayerVis)
        // The saved choices are untouched while it is on.
        assertEquals(Background.Matrix, Looks.background)
        assertEquals(MotionSpeed.Slow, Looks.motion)
        assertEquals(true, Looks.playerVis)
        Lite.mode = LiteMode.Off
        assertEquals(Background.Matrix, Looks.shownBackground)
        assertEquals(MotionSpeed.Slow, Looks.activeMotion)
        Lite.mode = LiteMode.Auto
    }

    @Test
    fun coverDecodeSize() {
        assertEquals(900, Lite.artPx(900, false))
        assertEquals(450, Lite.artPx(900, true))
        assertEquals(96, Lite.artPx(96, true))
        assertEquals(96, Lite.artPx(144, true))
        assertEquals(60, Lite.artPx(60, true))
    }
}
