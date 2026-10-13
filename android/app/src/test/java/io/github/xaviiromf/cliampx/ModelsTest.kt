package io.github.xaviiromf.cliampx

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** Fixtures pasted from a real core (emulator, `libcliamp.so remote state`). */
object Fixtures {
    const val TRACK = """{"title":"Digital Love","artist":"Daft Punk","album":"Discovery","path":"https://music.youtube.com/watch?v=Smi8vAbm_8o","album_art_url":"https://yt3.googleusercontent.com/qrY7xjdjqL0FEneZKHk845JeeFjdWOgexUa_BhsUxskJ2iflhzpbofqGJZBcPHDGU9JjoZflE4yn3P74=w400-h400-l90-rj","duration_secs":302,"stream":true,"provider_meta":{"section":"Songs","ytmusic.album":"MPREb_7ltM34kr0mH","ytmusic.artist":"UCRr1xG_2WIDs18a6cIiCxeA","ytmusic.id":"Smi8vAbm_8o","ytmusic.plays":"53M plays"}}"""

    const val SNAPSHOT = """{"revision":3,"playlist_revision":1,"state":"playing","track":$TRACK,"logical_track":$TRACK,"position":3.7496598629999998,"duration":302,"seekable":true,"total":1,"shuffle":false,"repeat":"Off","mono":false,"speed":1,"eq_preset":"Custom","eq_bands":[0,0,0,0,0,0,0,0,0,0],"visualizer":"Bars","theme":{"name":"Default - Terminal colors"}}"""

    const val ARTIST_ROW = """{"title":"Daft Punk","album":"Daft Punk","path":"ytmusic:ar:UCRr1xG_2WIDs18a6cIiCxeA","album_art_url":"https://lh3.googleusercontent.com/qLhu6Py_4_xoBsoubKQsXlhOQGqU9YU1ZRAbFusF0LlrPkXbbpu7bEh-k_ZtE4JwLgubvucAQqcK1hRk=w400-h400-p-l90-rj","provider_meta":{"albumID":"ar:UCRr1xG_2WIDs18a6cIiCxeA","kind":"album","section":"Artists"}}"""

    fun snapshot(json: String = SNAPSHOT, at: Long = 0) = Snapshot.parse(JSONObject(json), at)
}

/** Deep JSON equality: android.jar's JSONObject, which the tests compile against, has no similar(). */
fun same(a: Any?, b: Any?): Boolean = when {
    a is JSONObject && b is JSONObject -> a.keys().asSequence().toSet() == b.keys().asSequence().toSet() &&
        a.keys().asSequence().all { same(a.get(it), b.get(it)) }
    else -> a == b
}

class ModelsTest {
    @Test
    fun snapshotFromRealOutput() {
        val s = Fixtures.snapshot(at = 42)
        assertEquals(3L, s.revision)
        assertEquals("playing", s.state)
        assertEquals("Digital Love", s.track?.name)
        assertEquals(302.0, s.duration, 0.0)
        assertEquals(1.0, s.speed, 0.0)
        assertEquals("Default - Terminal colors", s.theme?.name)
        assertEquals("", s.theme?.bg)
        assertEquals(42L, s.receivedAt)
    }

    @Test
    fun omittedFieldsAreZero() {
        val s = Fixtures.snapshot("""{"revision":1,"playlist_revision":0,"seekable":false}""")
        assertEquals(0.0, s.volume, 0.0) // omitempty drops volume 0
        assertEquals(0, s.index) // and index 0
        assertEquals(1.0, s.speed, 0.0) // speed 0 means 1.0
        assertFalse(s.shuffle) // a nil pointer
        assertNull(s.track)
        assertNull(s.notice)
        assertNull(s.playError)
        assertTrue(Fixtures.snapshot("""{"revision":1,"shuffle":true,"speed":1.5}""").let { it.shuffle && it.speed == 1.5 })
    }

    @Test
    fun batchAFields() {
        val path = "https://music.youtube.com/watch?v=Smi8vAbm_8o"
        val s = Fixtures.snapshot(
            """{"revision":9,"state":"playing","track":${Fixtures.TRACK},"buffering":true,"seeking":true,
            "error":"yt-dlp: Sign in to confirm","error_kind":"ytdlp","error_path":"$path",
            "notice":{"text":"Guardado","kind":"success"},"sleep_minutes":-1,"sleep_at":1791911200,
            "auto_radio":true,"normalize":"album","crossfade":4,"eq_auto":true}""",
        )
        assertTrue(s.buffering && s.seeking && s.autoRadio && s.eqAuto)
        assertEquals("ytdlp", s.errorKind)
        assertEquals("yt-dlp: Sign in to confirm", s.playError)
        assertEquals(Notice("Guardado", "success"), s.notice)
        assertEquals(-1, s.sleepMinutes)
        assertEquals(1791911200L, s.sleepAt)
        assertEquals("album", s.normalize)
        assertEquals(4, s.crossfade)
        // An error of another track is not the error of the one on screen.
        assertNull(s.copy(errorPath = "https://music.youtube.com/watch?v=other").playError)
    }

    @Test
    fun trackRawRoundTrips() {
        val original = JSONObject(Fixtures.TRACK)
        val track = Track.parse(original)
        assertTrue(same(track.json(), original))
        assertEquals("MPREb_7ltM34kr0mH", track.meta["ytmusic.album"])
        assertEquals(302, track.durationSecs)
    }

    @Test
    fun localTrackArtIsItsFile() {
        val t = Track.parse(JSONObject("""{"path":"/storage/emulated/0/Music/a.mp3"}"""))
        assertEquals("a.mp3", t.name)
        assertEquals("/storage/emulated/0/Music/a.mp3", t.art)
        assertEquals("", Track.parse(JSONObject("""{"path":"https://x/y"}""")).art)
    }

    @Test
    fun clockFormats() {
        for ((secs, want) in listOf(0 to "0:00", 302 to "5:02", 3725 to "1:02:05", -3 to "0:00")) {
            assertEquals(want, clock(secs))
        }
    }
}

class SnapshotMergeTest {
    private fun snap(revision: Long, position: Double, path: String = "a", at: Long = 0) =
        Fixtures.snapshot("""{"revision":$revision,"state":"playing","position":$position,"track":{"path":"$path"}}""", at)

    @Test
    fun olderRevisionsAreIgnored() {
        assertNull(Snapshots.merge(snap(5, 1.0), snap(4, 2.0), Source.EVENT, floor = 5))
        assertEquals(5L, Snapshots.merge(snap(5, 1.0), snap(5, 2.0), Source.STATE_GET, floor = 5)?.revision)
    }

    @Test
    fun aRestartedCoreIsAcceptedAfterTheFloorReset() {
        // The subscribe loop resets the floor to 0: a new core counts from 0.
        assertEquals(1L, Snapshots.merge(snap(80, 1.0), snap(1, 0.0), Source.RETAINED, floor = 0)?.revision)
    }

    @Test
    fun positionSources() {
        val cur = snap(5, 10.0, at = 100)
        for ((source, keeps) in listOf(
            Source.RETAINED to true, Source.OTHER_JOB to true,
            Source.EVENT to false, Source.STATE_GET to false, Source.PLAYBACK_JOB to false,
        )) {
            val merged = Snapshots.merge(cur, snap(6, 99.0, at = 200), source, floor = 5)!!
            assertEquals(source.name, if (keeps) 10.0 else 99.0, merged.position, 0.0)
            assertEquals(source.name, if (keeps) 100L else 200L, merged.receivedAt)
            assertEquals(6L, merged.revision) // every other field is taken
        }
    }

    @Test
    fun aNewTrackTakesThePositionFromAnySource() {
        val merged = Snapshots.merge(snap(5, 10.0), snap(6, 0.5, path = "b"), Source.RETAINED, floor = 5)!!
        assertEquals(0.5, merged.position, 0.0)
        assertEquals(0.5, Snapshots.merge(null, snap(6, 0.5), Source.OTHER_JOB, floor = 0)!!.position, 0.0)
    }
}

class PlaybackClockTest {
    private fun snap(extra: String) = Fixtures.snapshot("""{"revision":1,"position":10,"duration":60$extra}""", at = 1_000)

    @Test
    fun positions() {
        for ((name, extra, want) in listOf(
            Triple("playing", ""","state":"playing"""", 12.0),
            Triple("paused", ""","state":"paused"""", 10.0),
            Triple("buffering", ""","state":"playing","buffering":true""", 10.0),
            Triple("seeking", ""","state":"playing","seeking":true""", 10.0),
            Triple("speed 1.5", ""","state":"playing","speed":1.5""", 13.0),
        )) {
            assertEquals(name, want, PlaybackClock.positionAt(snap(extra), 3_000), 1e-9)
        }
        assertEquals(60.0, PlaybackClock.positionAt(snap(""","state":"playing""""), 100_000), 0.0)
        val live = Fixtures.snapshot("""{"revision":1,"state":"playing","position":10}""", at = 0)
        assertEquals(110.0, PlaybackClock.positionAt(live, 100_000), 0.0) // no duration: no clamp
    }
}

class SearchSectionsTest {
    @Test
    fun groupsInArrivalOrderWithKinds() {
        fun row(section: String, albumID: String? = null) = Track.parse(
            JSONObject().put("path", "p$section$albumID").put(
                "provider_meta",
                JSONObject().put("section", section).apply { if (albumID != null) put("albumID", albumID) },
            ),
        )
        val rows = listOf(
            row("Local"), row("Artists", "ar:UC1"), row("Albums", "MPREb_1"), row("Artists", "ar:UC2"),
            row("Playlists", "pl:VL1"), row("Songs"),
        )
        val sections = searchSections(rows)
        assertEquals(listOf("Local", "Artists", "Albums", "Playlists", "Songs"), sections.map { it.title })
        assertEquals(2, sections[1].rows.size)
        assertEquals(
            listOf(Kind.SONG, Kind.ARTIST, Kind.ALBUM, Kind.ARTIST, Kind.PLAYLIST, Kind.SONG),
            rows.map { it.kind },
        )
        assertEquals(Kind.ARTIST, Track.parse(JSONObject(Fixtures.ARTIST_ROW)).kind)
        assertEquals(Kind.SONG, Track.parse(JSONObject(Fixtures.TRACK)).kind)
    }
}

class ErrorMapTest {
    @Test
    fun networkAndSignIn() {
        for (d in listOf("dial tcp: lookup music.youtube.com: no such host", "read: Connection reset by peer", "i/o timeout", "context deadline exceeded (Client.Timeout exceeded)")) {
            assertTrue(d, ErrorMap.network(d))
        }
        assertFalse(ErrorMap.network("HTTP Error 403: Forbidden"))
        assertTrue(ErrorMap.signIn("Sign in to confirm you're not a bot"))
        assertTrue(ErrorMap.signIn("needs sign-in"))
        assertFalse(ErrorMap.signIn("Video unavailable"))
        assertEquals(R.string.update_offline, ErrorMap.text("internal_error", "no such host"))
        assertEquals(R.string.reconnecting, ErrorMap.text("unavailable", ""))
        assertNull(ErrorMap.text("internal_error", "boom"))
    }

    @Test
    fun cards() {
        assertEquals(R.string.err_ytdlp to ErrorMap.Action.UPDATE_YTDLP, ErrorMap.card("ytdlp"))
        assertEquals(R.string.err_session to ErrorMap.Action.SIGN_IN, ErrorMap.card("session"))
        assertEquals(R.string.err_network to ErrorMap.Action.RETRY, ErrorMap.card("network"))
        assertEquals(R.string.err_unavailable to ErrorMap.Action.NEXT, ErrorMap.card("unavailable"))
        assertEquals(null to ErrorMap.Action.RETRY, ErrorMap.card(""))
    }
}
