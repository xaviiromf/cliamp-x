package io.github.xaviiromf.cliampx

import io.github.xaviiromf.cliampx.Ytdlp.Status
import io.github.xaviiromf.cliampx.Ytdlp.Watch
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import kotlinx.coroutines.yield
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class YtdlpPolicyTest {
    private val hour = 3_600_000L

    @Test
    fun watcherTable() {
        val now = 1_000_000L
        val wall = 10 * hour
        data class Case(val name: String, val status: Status, val finishedAt: Long, val tag: String?, val lastForced: Long, val kind: String, val checked: Long, val want: Watch)
        for (c in listOf(
            Case("updating", Status.Updating, 0, null, 0, "", 0, Watch.WAIT),
            Case("finished 30 s ago", Status.UpToDate("t"), now - 30_000, "t", 0, "ytdlp", wall, Watch.RETRY),
            Case("finished 2 min ago, tag set", Status.UpToDate("t"), now - 120_000, "t", 0, "", wall, Watch.NONE),
            Case("bundled, never forced", Status.Idle, 0, null, 0, "", wall, Watch.UPDATE),
            Case("bundled, forced 30 s ago", Status.Failed("x"), 0, null, now - 30_000, "", wall, Watch.NONE),
            Case("bundled, forced 2 min ago", Status.Failed("x"), 0, null, now - 120_000, "", wall, Watch.UPDATE),
            Case("ytdlp kind, checked 2 h ago", Status.Idle, 0, "t", 0, "ytdlp", wall - 2 * hour, Watch.UPDATE_IF_NEW),
            Case("ytdlp kind, checked 10 min ago", Status.Idle, 0, "t", 0, "ytdlp", wall - 600_000, Watch.NONE),
            Case("other kind", Status.Idle, 0, "t", 0, "network", 0, Watch.NONE),
        )) {
            assertEquals(c.name, c.want, Ytdlp.watch(c.status, now, c.finishedAt, c.tag, c.lastForced, c.kind, c.checked, wall))
        }
    }

    @Test
    fun gateOnlyBeforeTheFirstUpdate() {
        assertTrue(Ytdlp.gate(null, Status.Updating))
        assertFalse(Ytdlp.gate("2026.08.19", Status.Updating))
        assertFalse(Ytdlp.gate(null, Status.Failed("offline")))
        assertFalse(Ytdlp.gate(null, Status.Idle))
    }

    @Test
    fun watchesOnlyYouTubeErrorsOfTheTrackOnScreen() {
        fun snap(path: String, errorPath: String = path, error: String = "boom") = Fixtures.snapshot(
            """{"revision":1,"track":{"path":"$path"},"error":"$error","error_path":"$errorPath"}""",
        )
        assertTrue(Ytdlp.watches(snap("https://music.youtube.com/watch?v=1")))
        assertTrue(Ytdlp.watches(snap("https://youtu.be/1")))
        assertTrue(Ytdlp.watches(snap("https://www.youtube.com/watch?v=1")))
        assertFalse(Ytdlp.watches(snap("https://notyoutube.com/watch?v=1")))
        assertFalse(Ytdlp.watches(snap("/storage/emulated/0/Music/a.mp3")))
        assertFalse(Ytdlp.watches(snap("https://youtu.be/1", errorPath = "https://youtu.be/2")))
        assertFalse(Ytdlp.watches(snap("https://youtu.be/1", error = "")))
    }

    @Test
    fun tagFromTheRedirect() {
        assertEquals("2026.08.19", Ytdlp.tagOf("https://github.com/yt-dlp/yt-dlp/releases/tag/2026.08.19"))
        assertEquals("2026.10.10.233141", Ytdlp.tagOf("https://github.com/yt-dlp/yt-dlp-nightly-builds/releases/tag/2026.10.10.233141"))
        assertNull(Ytdlp.tagOf(null))
        assertNull(Ytdlp.tagOf("https://github.com/yt-dlp/yt-dlp/releases"))
        assertNull(Ytdlp.tagOf("https://github.com/yt-dlp/yt-dlp/releases/tag/../../evil"))
    }

    @Test
    fun hashFromTheSums() {
        val h = "1fa6733c37ea6fb51c99ad8fe785e7b7e5f3246c9b980230329d4fb72ed8d4d6"
        val sums = "${"a".repeat(64)}  yt-dlp.exe\n${"b".repeat(64)}  yt-dlp_linux\n$h  yt-dlp\n${"c".repeat(64)}  yt-dlp.tar.gz\n"
        assertEquals(h, Ytdlp.hashOf(sums))
        assertNull(Ytdlp.hashOf("${"b".repeat(64)}  yt-dlp_linux\n"))
        assertNull(Ytdlp.hashOf("nothex  yt-dlp\n"))
    }
}

class RenderSenderTest {
    /** Waits for cond without assuming how fast the test JVM runs. */
    private suspend fun until(cond: () -> Boolean) = withTimeout(5_000) { while (!cond()) delay(5) }

    @Test
    fun oneSendAtATimeEndingInTheLastMode() = runBlocking {
        val sent = mutableListOf<String>()
        var inFlight = 0
        var most = 0
        val sender = RenderSender {
            inFlight++
            most = maxOf(most, inFlight)
            delay(50)
            sent += it
            inFlight--
        }
        val job = launch { sender.run() }
        until { inFlight == 1 } // the first "off" is in flight
        for (on in listOf(true, false, true, false, true)) {
            sender.terminal.value = on
            yield()
        }
        until { sent.lastOrNull() == "on" && inFlight == 0 }
        delay(100)
        assertEquals(1, most)
        assertEquals(listOf("off", "on"), sent)

        // A reconnect sends the mode again, once.
        sender.reset()
        until { sent.size == 3 }
        delay(100)
        assertEquals(listOf("off", "on", "on"), sent)
        job.cancel()
    }

    @Test
    fun aFailedSendIsRetriedAfterAReconnect() = runBlocking {
        var attempts = 0
        val sent = mutableListOf<String>()
        val sender = RenderSender {
            if (attempts++ == 0) throw IpcError("ui.render", "unavailable", "down")
            sent += it
        }
        val job = launch { sender.run() }
        until { attempts == 1 }
        delay(50)
        assertTrue(sent.isEmpty()) // nothing retries until the reconnect
        sender.reset()
        until { sent.isNotEmpty() }
        delay(50)
        assertEquals(listOf("off"), sent)
        job.cancel()
    }
}
