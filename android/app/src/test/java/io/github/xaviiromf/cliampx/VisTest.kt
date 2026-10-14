package io.github.xaviiromf.cliampx

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class VisTest {
    @Test
    fun modeToRenderer() {
        for ((mode, reduced, want) in listOf(
            Triple("ClassicPeak", false, "ClassicPeak"),
            Triple("Starfield", false, "Starfield"),
            Triple("Starfield", true, "Bars"), // moves on its own
            Triple("Mirror", true, "Mirror"),
            Triple("Wave", false, "Bars"), // a mode only Modo terminal draws
            Triple("None", false, "None"),
            Triple("", false, "Bars"),
        )) assertEquals("$mode reduced=$reduced", want, nativeVis(mode, reduced))
    }

    @Test
    fun stripCycleSkipsNone() {
        assertEquals("ClassicPeak", nextVisualizer("Bars"))
        assertEquals("Bars", nextVisualizer("Starfield")) // the last before None wraps
        assertEquals("Bars", nextVisualizer("Matrix2"))
        assertEquals("Bars", nextVisualizer("None"))
        val seen = mutableSetOf<String>()
        var m = "Bars"
        repeat(NATIVE_VIS.size) { seen += m; m = nextVisualizer(m) }
        assertEquals(NATIVE_VIS.toSet() - "None", seen)
    }

    @Test
    fun resampleIntoIsLinear() {
        val dst = FloatArray(3)
        resampleInto(floatArrayOf(0f, 1f), dst)
        assertArrayEquals(floatArrayOf(0f, 0.5f, 1f), dst, 1e-6f)
        resampleInto(floatArrayOf(0.2f), dst)
        assertArrayEquals(floatArrayOf(0.2f, 0.2f, 0.2f), dst, 0f)
        resampleInto(FloatArray(0), dst)
        assertArrayEquals(FloatArray(3), dst, 0f)
        assertEquals(0.25f, sampleAt(floatArrayOf(0f, 0.5f, 1f), 0.25f), 1e-6f)
        assertEquals(1f, sampleAt(floatArrayOf(0f, 1f), 7f), 0f)
    }

    @Test
    fun easeRisesFasterThanItFalls() {
        val up = ease(0f, 1f, 0.033f)
        val down = 1f - ease(1f, 0f, 0.033f)
        assertTrue("up $up down $down", up > down && up > 0.5f && down > 0.2f)
        assertEquals(0.5f, ease(0.5f, 0.5f, 1f), 0f)
    }

    @Test
    fun peakHoldsThenFallsToTheBar() {
        val st = VisState()
        st.set(floatArrayOf(1f))
        st.step(0.1f, "ClassicPeak", snap = true)
        assertEquals(1f, st.peak[0], 0f)
        st.set(floatArrayOf(0f))
        st.step(0.1f, "ClassicPeak", snap = true)
        assertEquals("held", 1f, st.peak[0], 0f)
        repeat(5) { st.step(0.1f, "ClassicPeak", snap = true) }
        assertTrue("falling", st.peak[0] < 1f)
        repeat(30) { st.step(0.1f, "ClassicPeak", snap = true) }
        assertEquals("at the bar", 0f, st.peak[0], 0f)
    }

    @Test
    fun bassHitIsABeatAndHistoryScrolls() {
        val st = VisState()
        st.set(FloatArray(10) { 0.05f })
        repeat(20) { st.step(0.05f, "Ripple", snap = true) }
        val before = st.beats
        val head = st.histHead
        st.set(floatArrayOf(1f, 1f, 0.8f, 0.2f, 0.1f, 0.1f, 0.1f, 0.1f, 0.1f, 0.1f))
        st.step(0.05f, "Ripple", snap = true)
        assertEquals(before + 1, st.beats)
        assertTrue("a ring started", st.ringAge.any { it < 0.1f })
        assertTrue("history scrolled", st.histHead != head) // 24 columns a second
    }

    @Test
    fun particlesStayInThePool() {
        val st = VisState()
        st.set(FloatArray(10) { 1f })
        repeat(600) { st.step(0.016f, "Geyser") }
        assertTrue(st.count in 1..VisState.MAXP)
        st.step(0.016f, "Bars")
        assertEquals("a new mode clears them", 0, st.count)
    }
}
