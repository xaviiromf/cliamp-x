package io.github.xaviiromf.cliampx

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
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

    @Test
    fun sampleRateEnvTakesOnlySaneRates() {
        assertEquals("CLIAMP_DEVICE_SAMPLE_RATE=48000", Core.sampleRateEnv("48000"))
        assertEquals("CLIAMP_DEVICE_SAMPLE_RATE=44100", Core.sampleRateEnv("44100"))
        for (bad in listOf(null, "", "abc", "0", "7999", "192001")) assertNull(bad, Core.sampleRateEnv(bad))
    }

    @Test
    fun treePathMapsPickedFoldersToPaths() {
        val ext = "com.android.externalstorage.documents"
        val dl = "com.android.providers.downloads.documents"
        val primary = "/storage/emulated/0"
        for ((authority, docId, want) in listOf(
            Triple(ext, "primary:Music/Rock", "/storage/emulated/0/Music/Rock"),
            Triple(ext, "primary:Download/MiMusica/", "/storage/emulated/0/Download/MiMusica"),
            Triple(ext, "primary:", "/storage/emulated/0"),
            Triple(ext, "home:Mix", "/storage/emulated/0/Documents/Mix"),
            Triple(ext, "1234-ABCD:Music", "/storage/1234-ABCD/Music"),
            Triple(ext, "1234-ABCD:", "/storage/1234-ABCD"),
            Triple(dl, "raw:/storage/emulated/0/Download/Mix", "/storage/emulated/0/Download/Mix"),
            Triple(dl, "msf:42", null),
            Triple(dl, "downloads", null),
            Triple("com.google.android.apps.docs.storage", "acc=1;doc=encoded=abc", null),
            Triple(ext, "primary", null),
            Triple(ext, "../x:Music", null),
            Triple(ext, "primary:Music/../../data", null),
            Triple(ext, null, null),
            Triple(null, "primary:Music", null),
        )) {
            assertEquals("$authority $docId", want, Core.treePath(authority, docId, primary))
        }
    }
}
