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
}
