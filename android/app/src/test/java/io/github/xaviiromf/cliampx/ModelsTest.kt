package io.github.xaviiromf.cliampx

import kotlinx.coroutines.runBlocking
import org.json.JSONObject
import org.junit.Assert.assertArrayEquals
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
        assertEquals(R.string.err_session, ErrorMap.text("internal_error", "Sign in to confirm you're not a bot"))
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

class PlayerModelsTest {
    @Test
    fun playerFieldsFromRealOutput() {
        val s = Fixtures.snapshot()
        assertEquals("Custom", s.eqPreset)
        assertEquals(List(10) { 0.0 }, s.eqBands)
        assertEquals("Bars", s.visualizer)
        assertFalse(s.track!!.favorite)
        // queue.list and playnext.list rows, as the core sends them (emulator).
        val row = Track.parse(JSONObject("""{"path":"/sdcard/Music/test_dos.mp3","index":7,"queue_position":1,"bookmark":true}"""))
        assertEquals(7, row.index)
        assertTrue(row.favorite)
        // A finished queue.list job (emulator, trimmed): the rows and the playlist_revision they were read at.
        val page = Page.parse(
            JSONObject(
                """{"id":"fe34935ae5ab7b0a32feb982831285e7","operation":"queue.list","state":"succeeded",""" +
                    """"result":{"ok":true,"index":3,"total":23,"tracks":[${row.raw}]},""" +
                    """"snapshot":{"revision":40,"playlist_revision":28,"state":"paused","index":3,"total":27}}""",
            ),
        )
        assertEquals(23, page.total)
        assertEquals(28, page.revision)
        assertEquals("/sdcard/Music/test_dos.mp3", page.rows.single().path)
        assertEquals("a job without a snapshot is not checked", 0, Page.parse(JSONObject("""{"result":{"total":0}}""")).revision)
    }

    @Test
    fun lyrics() {
        val synced = Lyrics.parse(JSONObject("""{"ok":true,"lyrics":[{"start":0,"text":""},{"start":12.5,"text":"Uno"},{"start":20,"text":"Dos"}]}"""))
        assertTrue(Lyrics.synced(synced))
        for ((pos, want) in listOf(-1.0 to -1, 0.0 to 0, 12.4 to 0, 12.5 to 1, 19.9 to 1, 300.0 to 2)) {
            assertEquals("at $pos", want, Lyrics.active(synced, pos))
        }
        val plain = Lyrics.parse(JSONObject("""{"ok":true,"lyrics":[{"start":0,"text":"Uno"},{"start":0,"text":"Dos"}]}"""))
        assertFalse(Lyrics.synced(plain))
        assertEquals(emptyList<Lyric>(), Lyrics.parse(JSONObject("""{"ok":true}""")))
    }

    @Test
    fun swapsAreAdjacent() {
        assertEquals(listOf(2 to 3, 3 to 4, 4 to 5), swaps(2, 5))
        assertEquals(listOf(5 to 4, 4 to 3), swaps(5, 3))
        assertEquals(emptyList<Pair<Int, Int>>(), swaps(4, 4))
    }

    @Test
    fun swapsChainTheRevision() = runBlocking {
        val sent = mutableListOf<Triple<Int, Int, Long?>>()
        fun record(params: JSONObject) {
            sent += Triple(params.getInt("index"), params.getInt("to"), if (params.has("if_revision")) params.getLong("if_revision") else null)
        }
        // Each queue.move job commits a new revision, not always one higher (another edit can land in between).
        val revisions = ArrayDeque(listOf(8L, 10L, 11L))
        sendSwaps(2, 5, 7) { params ->
            record(params)
            JSONObject().put("state", "succeeded").put("snapshot", JSONObject().put("playlist_revision", revisions.removeFirst()))
        }
        assertEquals(listOf(Triple(2, 3, 7L), Triple(3, 4, 8L), Triple(4, 5, 10L)), sent)
        sent.clear()
        sendSwaps(1, 0, null) { params -> JSONObject().also { record(params) } }
        assertEquals("playnext.move sends no revision", listOf(Triple(1, 0, null)), sent)
    }

    @Test
    fun aFailedSwapStopsTheMove() = runBlocking {
        var calls = 0
        val e = runCatching { sendSwaps(0, 3, 4) { calls++; throw IpcError("queue.move", "conflict", "stale") } }.exceptionOrNull()
        assertEquals("conflict", (e as IpcError).code)
        assertEquals(1, calls)
    }

    @Test
    fun eqSendsWhatTheCoreLacks() {
        val flat = List(10) { 0.0 }
        // Band 0 to +3, Flat, then a tap at +3 again: the core is at 0, so +3 goes out again.
        assertEquals(0 to 3, eqPending(mapOf(0 to 3), flat, emptyMap()))
        assertNull("the core has it", eqPending(mapOf(0 to 3), flat.toMutableList().also { it[0] = 3.0 }, emptyMap()))
        assertNull("sent this round, the core did not take it", eqPending(mapOf(0 to 3), flat, mapOf(0 to 3)))
        assertEquals("a newer value of a sent band", 0 to 4, eqPending(mapOf(0 to 4), flat, mapOf(0 to 3)))
        assertEquals("the next band", 2 to -1, eqPending(mapOf(0 to 3, 2 to -1), flat.toMutableList().also { it[0] = 3.0 }, emptyMap()))
        assertEquals("no bands known yet", 5 to 0, eqPending(mapOf(5 to 0), emptyList(), emptyMap()))
    }

    @Test
    fun seekHoldsTheTarget() {
        fun at(position: Double, receivedAt: Long, path: String = "a") =
            Fixtures.snapshot("""{"revision":1,"state":"playing","position":$position,"track":{"path":"$path"}}""", receivedAt)
        val released = 10_000L
        assertTrue("an older snapshot", seekHolds(60.0, "a", released, at(10.0, 9_000), 11_000))
        assertTrue("a newer one far from the target", seekHolds(60.0, "a", released, at(10.5, 10_500), 11_000))
        assertFalse("a newer one at the target", seekHolds(60.0, "a", released, at(59.0, 10_500), 11_000))
        assertFalse("5 s passed", seekHolds(60.0, "a", released, at(10.0, 9_000), 15_000))
        assertFalse("another song", seekHolds(60.0, "a", released, at(1.0, 10_500, "b"), 11_000))
    }

    @Test
    fun eqGains() {
        for ((y, want) in listOf(0f to 12, 240f to 0, 480f to -12, 250f to 0, 235f to 0, 230f to 1, -50f to 12, 900f to -12)) {
            assertEquals("tap at $y", want, eqGain(y, 480f))
        }
        assertEquals(0, eqGain(10f, 0f))
        // A drag moves from where the band was: up is negative dy.
        assertEquals(3, eqDrag(0, -61f, 20f))
        assertEquals(-2, eqDrag(1, 59f, 20f))
        assertEquals(12, eqDrag(10, -100f, 20f))
        assertEquals(-12, eqDrag(-12, 100f, 20f))
    }
}

class PagingTest {
    @Test
    fun wholeListNextOffset() {
        assertEquals(200, Paging.next(200, 450))
        assertEquals(400, Paging.next(400, 450))
        assertNull(Paging.next(450, 450))
        assertNull(Paging.next(0, 0))
    }
}

/** provider.* results from a real core (emulator, signed out, Batch B), trimmed to a few rows. */
object BrowseFixtures {
    const val HOME = """{"ok":true,"total":2,"shelves":[{"title":"Quick picks","items":[{"kind":"song","id":"xukbqwRuN5w","title":"Choosin' Texas","subtitle":"Ella Langley","image_url":"https://yt3.googleusercontent.com/pXFQCa7a4g8yaLQOuE2y0zWK4X0A9Kdjx1PTC_KICZc9OEgahsFl-F6VBihBNOLAiYX7XnuPqaC4Yio=w400-h400-l90-rj","track":{"title":"Choosin' Texas","artist":"Ella Langley","album":"Dandelion","path":"https://music.youtube.com/watch?v=xukbqwRuN5w","album_art_url":"https://yt3.googleusercontent.com/pXFQCa7a4g8yaLQOuE2y0zWK4X0A9Kdjx1PTC_KICZc9OEgahsFl-F6VBihBNOLAiYX7XnuPqaC4Yio=w400-h400-l90-rj","stream":true,"provider_meta":{"ytmusic.album":"MPREb_QC0L98k1tGe","ytmusic.artist":"UCecnyZYofHiBVDJpx1XNYOQ","ytmusic.id":"xukbqwRuN5w","ytmusic.plays":"329M plays"}}}]},{"title":"New releases","items":[{"kind":"album","id":"MPREb_QIX6ZSIenDm","title":"Amor Factura","subtitle":"Single • Ricardo Arjona & Grupo Firme","image_url":"https://yt3.googleusercontent.com/sw-vFj78Rl1nPwjok9-yrqxvGGix9dbEVn3XB_STg4X3N1S2i0Nk5xQWdLAD4YOKJ6_mYr-LofXVxBk=w400-h400-l90-rj"},{"kind":"playlist","id":"pl:VLRDTMAK5uy_lr0LWzGrq6FU9GIxWvFHTRPQD2LHMqlFA","title":"New Release Mix","subtitle":"Infraction Music, Social Repose, Medium Build","image_url":"https://music.youtube.com/image/mixart?r=ENgEGNgEMh4IBxoNL2cvMTFwYzFzazdmNhoLL20vMDEyZ245bHJiCAoGCgQKAmVu"},{"kind":"artist","id":"UCRr1xG_2WIDs18a6cIiCxeA","title":"Daft Punk"}]}]}"""

    const val ARTIST_ALBUMS = """{"ok":true,"total":3,"albums":[{"id":"ar:UCRr1xG_2WIDs18a6cIiCxeA","name":"Top songs","artist":"Daft Punk","artist_id":"UCRr1xG_2WIDs18a6cIiCxeA","image_url":"https://lh3.googleusercontent.com/qLhu6Py_4_xoBsoubKQsXlhOQGqU9YU1ZRAbFusF0LlrPkXbbpu7bEh-k_ZtE4JwLgubvucAQqcK1hRk=w400-h400-p-l90-rj"},{"id":"MPREb_K8qWMWVqXGi","name":"Random Access Memories","artist":"Daft Punk","artist_id":"UCRr1xG_2WIDs18a6cIiCxeA","year":2013,"image_url":"https://yt3.googleusercontent.com/N55arCGj69gtw6thXK8JUPisxoVYiwuIEQ7I6SGlkEyNcSJ7xIWPe76Vuu1SiUqRyx5w9qvR_zV8fV3CWQ=w400-h400-l90-rj"},{"id":"MPREb_X1DQ1j0PPrX","name":"GLBTM (Studio Outtakes) · Singles & EPs","artist":"Daft Punk","artist_id":"UCRr1xG_2WIDs18a6cIiCxeA","year":2023,"image_url":"https://yt3.googleusercontent.com/fgqj_3-DKrpBQNAoK9RmUAbW6-JaDbcPRF4ZsWjsLa1U-3m6X7pvg2QF5QrZLvs6HfTFS-d8DlKzYANK=w400-h400-l90-rj","single":true}]}"""

    const val ARTISTS = """{"ok":true,"total":2,"artists":[{"id":"UCgfk20rO0LzAQSrhkTfReEQ","name":"KHAOS","image_url":"https://yt3.googleusercontent.com/mt8WdzfdbPPAsPX6g0Of-LaAzNGkCmsCRs7jdZ43b9T-c6t1AtD5tHxzKZGQnxdcJz4jKmWFSg=w400-h400-l90-rj-dckXOK6j0F"},{"id":"UCXSYGVPdqu40oQtq8g580yA","name":"♥ MXZI","image_url":"https://yt3.googleusercontent.com/s-nX0_7xAY8Tg8vACYIwo0pA9GTMA02Lvwnfl657ylKAB-v56X6oMzS0JjMJOYX9kOSccm_SAQ=w400-h400-l90-rj","favorite":true}]}"""

    const val GENRES = """{"ok":true,"total":2,"sorts":[{"id":"mix","label":"Genre mix"},{"id":"songs","label":"Popular songs"}],"genres":[{"id":"ggMPOg1uX0UzWGxlRE5jMDVk","name":"African","group":"Genres"},{"id":"search:Shoegaze","name":"★ Shoegaze","group":"More genres","favorite":true}]}"""

    const val PREVIEW_ALBUM = """{"ok":true,"preview":{"image_url":"https://yt3.googleusercontent.com/qrY7xjdjqL0FEneZKHk845JeeFjdWOgexUa_BhsUxskJ2iflhzpbofqGJZBcPHDGU9JjoZflE4yn3P74=w400-h400-l90-rj","lines":["Album · 2001","Daft Punk","14 songs • 1 hour, 1 minute"]}}"""
    const val PREVIEW_ARTIST = """{"ok":true,"preview":{"image_url":"https://lh3.googleusercontent.com/qLhu6Py_4_xoBsoubKQsXlhOQGqU9YU1ZRAbFusF0LlrPkXbbpu7bEh-k_ZtE4JwLgubvucAQqcK1hRk=w400-h400-p-l90-rj","lines":["75.1M monthly audience","7.19M subscribers","♫ Instant Crush (feat. Julian Casablancas) · Get Lucky (Radio Edit - feat. Pharrell Williams and Nile Rodgers) · One More Time (Radio Edit)"]}}"""
    const val PREVIEW_PLAYLIST = """{"ok":true,"preview":{"image_url":"https://yt3.googleusercontent.com/Cf5Fpy8zREpaiQ7sKWJvj2ug7d3MEA0Wy563x33D-otvTMHnbN2PrXcf_UjguKWRkKTyrIDEJjv_=s1200","lines":["","172K views • 8 tracks • 1 hour, 3 minutes"]}}"""

    // The real rows, plus a release and a starred genre as docs/remote-control.md names them (they need a signed-in account).
    const val PLAYLISTS = """{"ok":true,"total":5,"playlists":[{"id":"fav","name":"♥ Favorites","provider":"ytmusic","section":"Your music"},{"id":"pl:VLRDCLAK5uy_lheErTnEXzx41DFOdllzXH0qFOH6I6rbo","name":"Essential Reggae · Bob Marley, UB40, Peter Tosh, Jimmy Cliff","provider":"ytmusic","section":"Home · All-time essentials","image_url":"https://yt3.googleusercontent.com/8OBvGW7kG4G9XXDq554R26bCmXmr_iyjfuKHlPKoKMDO3vjuCvp4LncmgaXXmaACd52b7yTWJ62Mh6u-=w400-h400-l90-rj"},{"id":"al:MPREb_X1DQ1j0PPrX","name":"● GLBTM (Studio Outtakes) · Daft Punk · Single","provider":"ytmusic","section":"New releases","unseen":true},{"id":"genre:ggMPOg1uX0UzWGxlRE5jMDVk","name":"★ African","provider":"ytmusic","section":"Your genres"},{"id":"pl:VLmine","name":"Para correr","provider":"ytmusic","section":"Your music","track_count":45,"writable":true}]}"""

    const val HISTORY = """{"ok":true,"history":[{"track":{"title":"End of Line","artist":"Daft Punk","path":"https://music.youtube.com/watch?v=NOMa56y_Was","duration_secs":157},"played_at":"2026-10-13T22:47:44Z"},{"track":{"title":"Derezzed","artist":"Daft Punk","path":"https://music.youtube.com/watch?v=a7Wask21bew"},"played_at":"2026-10-13T22:40:00Z"},{"track":{"title":"End of Line","artist":"Daft Punk","path":"https://music.youtube.com/watch?v=NOMa56y_Was"},"played_at":"2026-10-13T22:30:00Z"}]}"""
}

class BrowseModelsTest {
    private fun json(s: String) = JSONObject(s)

    @Test
    fun shelves() {
        val shelves = Items.shelves(json(BrowseFixtures.HOME))
        assertEquals(listOf("Quick picks", "New releases"), shelves.map { it.title })
        val song = shelves[0].items.single()
        assertEquals(Kind.SONG, song.kind)
        assertEquals("https://music.youtube.com/watch?v=xukbqwRuN5w", song.track?.path)
        assertEquals("MPREb_QC0L98k1tGe", song.track?.meta?.get("ytmusic.album"))
        assertEquals(listOf(Kind.ALBUM, Kind.PLAYLIST, Kind.ARTIST), shelves[1].items.map { it.kind })
        assertEquals("pl:VLRDTMAK5uy_lr0LWzGrq6FU9GIxWvFHTRPQD2LHMqlFA", shelves[1].items[1].id)
        assertNull(shelves[1].items[0].track)
        assertEquals("", shelves[1].items[2].art)
    }

    @Test
    fun albumsAndArtists() {
        val albums = Items.albums(json(BrowseFixtures.ARTIST_ALBUMS))
        assertEquals(listOf("Top songs", "Random Access Memories", "GLBTM (Studio Outtakes)"), albums.map { it.title })
        assertEquals("Daft Punk · 2013", albums[1].subtitle)
        assertEquals("UCRr1xG_2WIDs18a6cIiCxeA", albums[1].artistId)
        assertEquals(listOf("", "", "single"), albums.map { it.section })
        val artists = Items.artists(json(BrowseFixtures.ARTISTS))
        assertEquals(listOf("KHAOS", "MXZI"), artists.map { it.title })
        assertEquals(listOf(false, true), artists.map { it.favorite })
        assertEquals(Kind.ARTIST, artists[0].kind)
    }

    @Test
    fun genresAndSorts() {
        val (genres, sorts) = Items.genres(json(BrowseFixtures.GENRES))
        assertEquals(listOf("African", "Shoegaze"), genres.map { it.title })
        assertEquals(listOf("Genres", "More genres"), genres.map { it.section })
        assertEquals("search:Shoegaze", genres[1].id)
        assertTrue(genres[1].favorite)
        assertEquals(listOf(Sort("mix", "Genre mix"), Sort("songs", "Popular songs")), sorts)
    }

    @Test
    fun previews() {
        val album = Items.preview(json(BrowseFixtures.PREVIEW_ALBUM))
        assertEquals(listOf("Album · 2001", "Daft Punk", "14 songs • 1 hour, 1 minute"), album.lines)
        assertNull("signed out: unknown", album.favorite)
        assertTrue(Items.preview(json(BrowseFixtures.PREVIEW_ARTIST)).lines[2].startsWith("Instant Crush")) // "♫ " dropped
        assertEquals(listOf("172K views • 8 tracks • 1 hour, 3 minutes"), Items.preview(json(BrowseFixtures.PREVIEW_PLAYLIST)).lines)
        assertEquals(false, Items.preview(json("""{"ok":true,"preview":{"lines":["Band"],"favorite":false}}""")).favorite)
        assertEquals(true, Items.preview(json("""{"ok":true,"preview":{"favorite":true}}""")).favorite)
    }

    @Test
    fun playlistRows() {
        val rows = Items.playlists(json(BrowseFixtures.PLAYLISTS))
        assertEquals(listOf(Kind.PLAYLIST, Kind.PLAYLIST, Kind.ALBUM, Kind.GENRE, Kind.PLAYLIST), rows.map { it.kind })
        assertEquals("Favorites", rows[0].title)
        assertEquals("Essential Reggae · Bob Marley, UB40, Peter Tosh, Jimmy Cliff", rows[1].title)
        val release = rows[2]
        assertEquals("al:MPREb_X1DQ1j0PPrX", release.id)
        assertEquals("GLBTM (Studio Outtakes)", release.title)
        assertEquals("Daft Punk · Single", release.subtitle)
        assertTrue(release.unseen)
        assertEquals("ggMPOg1uX0UzWGxlRE5jMDVk", rows[3].id)
        assertEquals("African", rows[3].title)
        assertTrue(rows[4].writable)
        assertEquals(45, rows[4].count)
    }

    @Test
    fun historyIsDeduplicated() {
        val tracks = Items.history(json(BrowseFixtures.HISTORY))
        assertEquals(listOf("End of Line", "Derezzed"), tracks.map { it.title })
        assertEquals(157, tracks[0].durationSecs)
        assertEquals(20, Items.history(JSONObject().put("history", org.json.JSONArray((0 until 30).map { JSONObject().put("track", JSONObject().put("path", "p$it")) }))).size)
    }

    @Test
    fun searchPlaceholders() {
        val artist = Track.parse(JSONObject(Fixtures.ARTIST_ROW)).item()
        assertEquals(Kind.ARTIST, artist.kind)
        assertEquals("UCRr1xG_2WIDs18a6cIiCxeA", artist.id) // "ar:" stripped for the Artist page
        val album = Track.parse(JSONObject("""{"title":"Discovery","artist":"Daft Punk","path":"ytmusic:MPREb_7ltM34kr0mH","provider_meta":{"albumID":"MPREb_7ltM34kr0mH","kind":"album","section":"Albums"}}""")).item()
        assertEquals(Item(Kind.ALBUM, "MPREb_7ltM34kr0mH", "Discovery", "Daft Punk"), album)
    }

    @Test
    fun likesFollowTheCore() {
        val row = Track.parse(JSONObject(Fixtures.TRACK)) // bookmark false
        assertFalse(emptyMap<String, Boolean>().likes(row))
        assertTrue(mapOf(row.path to true).likes(row))
        assertFalse(mapOf(row.path to false).likes(row.copy(favorite = true)))
    }
}

class RowNameTest {
    @Test
    fun marksAndTags() {
        for ((name, single, want) in listOf(
            Triple("♥ Discovery", false, "Discovery"),
            Triple("● New One · Band", false, "New One · Band"),
            Triple("★ Rock", false, "Rock"),
            Triple("♥ ● Both", false, "Both"),
            Triple("Home Album · local", false, "Home Album"),
            Triple("Digital Love · Singles & EPs", true, "Digital Love"),
            Triple("A · B · Singles & EPs", true, "A · B"),
            Triple("Plain · Name", false, "Plain · Name"),
            Triple("No group", true, "No group"),
        )) {
            assertEquals(name, want, RowName.of(name, single))
        }
    }
}

class BrowseHelpersTest {
    @Test
    fun foryouPaging() {
        assertEquals(40, Paging.foryou(0, 40, 40))
        assertEquals("a short page still moves by limit", 80, Paging.foryou(40, 40, 31))
        assertNull(Paging.foryou(80, 40, 0))
    }

    @Test
    fun recentQueries() {
        val ten = (1..10).map { "q$it" }
        assertEquals(listOf("daft punk"), recents(emptyList(), "  daft punk "))
        assertEquals(listOf("Q3", "q1", "q2", "q4"), recents(listOf("q1", "q2", "q3", "q4"), "Q3"))
        assertEquals(listOf("new") + ten.take(9), recents(ten, "new"))
        assertEquals(ten, recents(ten, "   "))
    }

    @Test
    fun loadingLastsUntilTheSongPlays() {
        val path = "https://music.youtube.com/watch?v=Smi8vAbm_8o"
        fun snap(extra: String, track: String = path) = Fixtures.snapshot("""{"revision":1,"track":{"path":"$track"}$extra}""")
        for ((name, s, want) in listOf(
            Triple("stopped while yt-dlp extracts", snap(""","state":"stopped""""), true),
            Triple("paused", snap(""","state":"paused""""), true),
            Triple("playing but buffering", snap(""","state":"playing","buffering":true"""), true),
            Triple("playing", snap(""","state":"playing""""), false),
            Triple("failed", snap(""","state":"stopped","error":"boom","error_path":"$path""""), false),
            Triple("another song", snap(""","state":"stopped"""", "https://youtu.be/x"), false),
        )) {
            assertEquals(name, want, loads(path, s))
        }
    }

    @Test
    fun aSessionSeekHoldsLongerThanAReopen() {
        val target = 272.0
        val stale = Fixtures.snapshot("""{"revision":1,"state":"playing","position":67,"track":{"path":"a"}}""", at = 12_000)
        assertTrue("12 s into a YouTube reopen", seekHolds(target, "a", 10_000, stale, 22_000, 15_000))
        assertFalse("the bar's 5 s hold", seekHolds(target, "a", 10_000, stale, 22_000))
        assertFalse("after 15 s", seekHolds(target, "a", 10_000, stale, 25_001, 15_000))
    }

    @Test
    fun duotoneMapsLuminanceToTheTheme() {
        val bg = 0xff102030.toInt()
        val accent = 0xff0000cc.toInt()
        fun apply(m: FloatArray, argb: Int): Int {
            val p = floatArrayOf((argb shr 16 and 0xff).toFloat(), (argb shr 8 and 0xff).toFloat(), (argb and 0xff).toFloat(), (argb ushr 24).toFloat(), 1f)
            val out = IntArray(4) { r -> Math.round((0..3).sumOf { p[it].toDouble() * m[r * 5 + it] } + m[r * 5 + 4]).toInt().coerceIn(0, 255) }
            return (out[3] shl 24) or (out[0] shl 16) or (out[1] shl 8) or out[2]
        }
        val m = duotone(bg, accent)
        for ((pixel, want) in listOf(
            0xff000000.toInt() to bg,
            0xffffffff.toInt() to accent,
            0x00ffffff to accent, // transparent pixels come out opaque
            0xff808080.toInt() to 0xff08107e.toInt(),
        )) assertEquals("%08x".format(pixel), "%08x".format(want), "%08x".format(apply(m, pixel)))
    }
}
