package io.github.xaviiromf.cliampx.ui

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
    fun shownThroughTheScrim() {
        val cases = listOf(
            Triple(Background.Solid, 100, 0f),
            Triple(Background.Plasma, 0, 0f),
            Triple(Background.Plasma, 50, 0.35f),
            Triple(Background.Plasma, 100, 0.7f),
            Triple(Background.Plasma, 250, 0.7f),
            Triple(Background.Plasma, -5, 0f),
        )
        for ((b, level, want) in cases) assertEquals("$b $level", want, backdropShown(b, level), 1e-6f)
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
