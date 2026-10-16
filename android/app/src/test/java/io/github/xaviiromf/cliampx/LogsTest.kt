package io.github.xaviiromf.cliampx

import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class LogsTest {
    @get:Rule val tmp = TemporaryFolder()

    @Test
    fun tailKeepsWholeLinesFromTheEnd() {
        val log = tmp.newFile("cliamp.log").apply { writeText("first ñ line\nsecond\nthird\n") }
        for ((max, want) in listOf(
            1000L to "first ñ line\nsecond\nthird\n", // all of it
            log.length() to "first ñ line\nsecond\nthird\n", // exactly all
            26L to "second\nthird\n", // one byte short: the cut first line goes
            10L to "third\n", // cut mid "second": from the next line
            6L to "third\n", // cut on a line start
            5L to "", // cut inside the last line: no whole line
        )) assertEquals("max $max", want, Logs.tail(log, max))
        assertEquals("(no gone.log)\n", Logs.tail(File(tmp.root, "gone.log"), 10))
    }

    @Test
    fun lastLinesCutsByLines() {
        for ((text, n, want) in listOf(
            Triple("a\nb\nc\n", 2, "b\nc\n"),
            Triple("a\nb\nc", 2, "b\nc"),
            Triple("a\nb\nc\n", 3, "a\nb\nc\n"),
            Triple("a\nb\nc\n", 10, "a\nb\nc\n"),
            Triple("a\nb\nc\n", 1, "c\n"),
            Triple("a\nb\nc\n", 0, ""),
            Triple("", 5, ""),
            Triple("only", 1, "only"),
        )) assertEquals("$n of ${text.replace("\n", "|")}", want, Logs.lastLines(text, n))
    }

    @Test
    fun dropNoiseKeepsOurLinesAndEveryWarning() {
        val log = listOf(
            "10-16 12:00:00.000  100  100 D Choreographer: Skipped 30 frames",
            "10-16 12:00:00.001  100  120 I cliamp: playback: health",
            "10-16 12:00:00.002  100  100 E Choreographer: real error",
            "10-16 12:00:00.003  100  100 W HWUI    : warning",
            "--------- beginning of main",
            "10-16 12:00:00.004  100  100 I libc    : noise",
            "    continuation line",
        ).joinToString("\n")
        val want = listOf(
            "10-16 12:00:00.001  100  120 I cliamp: playback: health",
            "10-16 12:00:00.002  100  100 E Choreographer: real error",
            "10-16 12:00:00.003  100  100 W HWUI    : warning",
            "--------- beginning of main",
            "    continuation line",
        ).joinToString("\n")
        assertEquals(want, Logs.dropNoise(log, Logs.NOISY_TAGS))
    }

    @Test
    fun configValuesTakesTheWantedKeysInOrder() {
        val toml = "# theme = \"x\"\nmouse = true\n[ytmusic]\n quality_mobile = \"normal\"\ntheme = \"Tokyo Night\"\nbuffer_ms=2000\n"
        assertEquals(
            listOf("theme" to "\"Tokyo Night\"", "quality_mobile" to "\"normal\"", "buffer_ms" to "2000"),
            Logs.configValues(toml, listOf("theme", "visualizer", "quality_mobile", "buffer_ms")),
        )
        assertEquals(emptyList<Pair<String, String>>(), Logs.configValues("", Logs.CONFIG_KEYS))
    }

    @Test
    fun headerListsEveryFact() {
        val info = Logs.Info(
            app = "cliamp-x 1.0 (7) debug", android = "Android 16 (API 36)", device = "Google Pixel 9", abi = "arm64-v8a",
            ramMb = 7680, lowRam = false, cores = 8, network = "wifi", batterySaver = true, deviceSampleRate = "48000 Hz",
            settings = listOf("quality_wifi" to "\"alta\"", "ui.motion" to "Off"),
        )
        val want = """
            cliamp-x 1.0 (7) debug
            Android 16 (API 36)
            device: Google Pixel 9
            abi: arm64-v8a
            ram: 7680 MB, low-ram device: no
            cpu cores: 8
            network: wifi
            battery saver: on
            device sample rate: 48000 Hz
            settings:
              quality_wifi = "alta"
              ui.motion = Off
            NOW
        """.trimIndent() + "\n"
        assertEquals(want, Logs.header(info, "NOW"))
        assertEquals(true, Logs.header(info.copy(settings = emptyList()), "NOW").contains("  (none set)\n"))
    }
}
