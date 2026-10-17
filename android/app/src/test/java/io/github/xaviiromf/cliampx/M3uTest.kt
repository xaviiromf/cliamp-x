package io.github.xaviiromf.cliampx

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class M3uTest {
    @Test
    fun parsesTitlesCommentsBlankLinesBomAndLineEndings() {
        val text = "﻿#EXTM3U\r\n\r\n# a comment\r\n#EXTINF:215,Artist - Song\r\n/m/a.mp3\r\n#EXTINF:-1,Live\r\nhttps://youtu.be/x\r\nbare.mp3\r\n#EXTINF:abc,Odd\n#EXTGRP:x\nsub\\b.mp3"
        assertEquals(
            listOf(
                M3u.Entry("/m/a.mp3", "Artist - Song", 215),
                M3u.Entry("https://youtu.be/x", "Live", -1),
                M3u.Entry("bare.mp3", "", -1),
                M3u.Entry("sub/b.mp3", "Odd", -1),
            ),
            M3u.parse(text),
        )
        assertEquals(emptyList<M3u.Entry>(), M3u.parse(""))
    }

    @Test
    fun resolvesUrlsAbsoluteAndRelativePathsAndDropsTheMissing() {
        val have = setOf("/m/a.mp3", "/m/Rock/b.mp3", "/m/c.mp3")
        val roots = listOf("/dl", "/m")
        for ((path, want) in listOf(
            "https://youtu.be/x" to "https://youtu.be/x",
            "/m/a.mp3" to "/m/a.mp3",
            "/m/x/../c.mp3" to "/m/c.mp3",
            "/m/none.mp3" to null,
            "Rock/b.mp3" to "/m/Rock/b.mp3",
            "../m/a.mp3" to "/m/a.mp3",
            "none.mp3" to null,
            "ssh://h/a.mp3" to null,
            "rtmp://h/a" to null,
        )) {
            val got = M3u.resolve(path, roots, have::contains)
            if (want == null) assertNull(path, got) else assertEquals(path, want, got)
        }
        assertEquals("/m/a.mp3", M3u.resolve("../m/a.mp3", listOf("/dl"), have::contains))
    }

    @Test
    fun entriesBecomeTracks() {
        val a = M3u.track(M3u.Entry("/m/a.mp3", "Artist - Song - Live", 200), "/m/a.mp3")
        assertEquals("Artist", a.getString("artist"))
        assertEquals("Song - Live", a.getString("title"))
        assertEquals(200, a.getInt("duration_secs"))
        val b = M3u.track(M3u.Entry("x/My Song.v2.mp3", "", -1), "/m/My Song.v2.mp3")
        assertEquals("My Song.v2", b.getString("title"))
        assertEquals(false, b.has("artist"))
        assertEquals(false, b.has("duration_secs"))
        assertEquals(false, M3u.track(M3u.Entry("https://y/x", "", -1), "https://y/x").has("title"))
    }

    private fun track(path: String, title: String, artist: String, secs: Int) =
        Track("{}", path, title, artist, "", "", secs, emptyMap())

    @Test
    fun writeThenParseRoundTrips() {
        val tracks = listOf(
            track("/m/a.mp3", "Song", "Artist", 215),
            track("https://music.youtube.com/watch?v=jNQXAC9IVRw", "Zoo", "", 0),
            track("ytmusic:abc", "No path", "", 10),
            track("/m/c.mp3", "", "", 5),
        )
        val text = M3u.write(tracks)
        assertEquals(
            listOf(
                M3u.Entry("/m/a.mp3", "Artist - Song", 215),
                M3u.Entry("https://music.youtube.com/watch?v=jNQXAC9IVRw", "Zoo", -1),
                M3u.Entry("/m/c.mp3", "c.mp3", 5),
            ),
            M3u.parse(text),
        )
        assertEquals("#EXTM3U\n", M3u.write(emptyList()))
    }
}
