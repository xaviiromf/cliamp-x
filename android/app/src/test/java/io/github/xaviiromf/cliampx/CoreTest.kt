package io.github.xaviiromf.cliampx

import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class CoreTest {
    @get:Rule
    val tmp = TemporaryFolder()

    @Test
    fun pruneKeepsTheRecentSolverCache() {
        val day = 86_400_000L
        val now = 100 * day
        val yt = File(tmp.root, "yt-dlp")
        fun file(path: String, age: Long) = File(yt, path).apply {
            parentFile!!.mkdirs()
            writeText("x")
            setLastModified(now - age)
        }
        file("challenge-solver-2026.08.19/player-new.json", day)
        file("challenge-solver-2026.08.19/player-old.json", 8 * day)
        file("challenge-solver-2026.01.01/player.json", 30 * day)
        file("youtube-sigfuncs/old.json", 30 * day)
        Core.pruneSolverCache(tmp.root, now)
        val left = yt.walk().filter { it.isFile }.map { it.relativeTo(yt).path }.sorted().toList()
        assertEquals(listOf("challenge-solver-2026.08.19/player-new.json", "youtube-sigfuncs/old.json"), left)
        assertEquals(listOf("challenge-solver-2026.08.19", "youtube-sigfuncs"), yt.list()!!.sorted()) // the empty old section is gone
    }
}
