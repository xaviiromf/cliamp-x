package io.github.xaviiromf.cliampx.ui

import io.github.xaviiromf.cliampx.LauncherIcon
import android.content.pm.PackageManager
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class StereoTest {
    @Test
    fun nightHours() {
        val dim = (0..23).filter(Stereo::night)
        assertEquals(listOf(0, 1, 2, 3, 4, 5, 6, 22, 23), dim)
    }

    @Test
    fun shiftStaysSmallAndMoves() {
        val places = (0L until 120L).map { Stereo.shift(29_000_000L + it) }
        assertTrue(places.all { (x, y) -> x in -12..12 && y in -12..12 })
        // A new place every minute, and the screen's whole range used over two hours.
        assertTrue(places.zipWithNext().all { (a, b) -> a != b })
        assertEquals(25, places.map { it.first }.toSet().size)
        assertEquals(25, places.map { it.second }.toSet().size)
    }

    @Test
    fun splashTypesLetterByLetter() {
        val cases = listOf(0 to "█" + " ".repeat(14), 3 to "C L█" + " ".repeat(11), 15 to "C L I A M P - X", 99 to "C L I A M P - X", -1 to "█" + " ".repeat(14))
        for ((n, want) in cases) assertEquals("n=$n", want, splashLogo(n))
        assertTrue((0..15).all { splashLogo(it).length == 15 })
    }

    @Test
    fun launcherIconStates() {
        val cases = listOf(
            Triple(PackageManager.COMPONENT_ENABLED_STATE_DEFAULT, LauncherIcon.Default, true),
            Triple(PackageManager.COMPONENT_ENABLED_STATE_DEFAULT, LauncherIcon.Green, false),
            Triple(PackageManager.COMPONENT_ENABLED_STATE_ENABLED, LauncherIcon.GameBoy, true),
            Triple(PackageManager.COMPONENT_ENABLED_STATE_DISABLED, LauncherIcon.Default, false),
        )
        for ((state, icon, want) in cases) assertEquals("$state $icon", want, LauncherIcon.enabled(state, icon))
    }
}
