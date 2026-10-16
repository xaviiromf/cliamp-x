package io.github.xaviiromf.cliampx

import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class StationsTest {
    private fun track(title: String, published: String? = null, artist: String = "", station: String = "", feed: Boolean = false) = Track.parse(
        JSONObject().put("path", "https://x/$title.mp3").put("title", title).put("artist", artist).put("station", station).apply {
            if (feed || published != null) put("provider_meta", JSONObject().apply {
                if (feed) put("podcast.feed", "https://x/feed")
                if (published != null) put("podcast.published", published)
            })
        },
    )

    @Test
    fun validAcceptsOnlyAbsoluteHttpUrls() {
        for ((input, want) in listOf(
            "https://ice1.somafm.com/groovesalad-128-mp3" to "https://ice1.somafm.com/groovesalad-128-mp3",
            "  http://a.b/x.pls \n" to "http://a.b/x.pls",
            "HTTPS://a.b" to "HTTPS://a.b",
            "ftp://a.b/x" to null,
            "a.b/x" to null,
            "https://" to null,
            "https://a b/x" to null,
            "javascript:alert(1)" to null,
            "" to null,
        )) assertEquals(input, want, Stations.valid(input))
    }

    @Test
    fun parseAllDropsBadUrlsAndRepeatsAndNamesByHost() {
        val a = JSONArray(
            """[{"name":" Groove ","url":"https://www.somafm.com/x"},{"name":"dup","url":"https://www.somafm.com/x"},
            {"name":"bad","url":"ftp://x"},{"url":"https://radio.example/s"},"junk"]""",
        )
        assertEquals(listOf(Station("Groove", "https://www.somafm.com/x"), Station("radio.example", "https://radio.example/s")), Stations.parseAll(a))
        assertEquals(emptyList<Station>(), Stations.parseAll(null))
    }

    @Test
    fun newestFirstSortsByDateAndKeepsTiesInFeedOrder() {
        val got = Stations.newestFirst(listOf(track("a", "2024-01-02"), track("b", "2024-03-01"), track("c"), track("d", "2024-01-02")))
        assertEquals(listOf("b", "a", "d", "c"), got.map { it.title })
    }

    @Test
    fun feedsAreEpisodesWithAFeedMark() {
        assertTrue(Stations.isFeed(listOf(track("a", feed = true), track("b", feed = true))))
        assertFalse(Stations.isFeed(listOf(track("a", feed = true), track("b"))))
        assertFalse(Stations.isFeed(emptyList()))
    }

    @Test
    fun nameOfUsesThePodcastTheStationOrTheHost() {
        val url = "https://www.show.example/feed"
        assertEquals("The Show", Stations.nameOf(url, listOf(track("e1", artist = "The Show", feed = true))))
        assertEquals("Groove Salad", Stations.nameOf(url, listOf(track("Song", station = "Groove Salad"))))
        assertEquals("Stream", Stations.nameOf(url, listOf(track("Stream"))))
        assertEquals("show.example", Stations.nameOf(url, emptyList()))
    }
}
