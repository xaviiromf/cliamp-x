package io.github.xaviiromf.cliampx

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** Batch C results pasted from a real core (emulator, `libcliamp.so remote call … --wait`), shortened. */
object LibraryFixtures {
    const val STATS = """{"ok":true,"stats":{"plays":4,"secs":495.111927435,"songs":3,"artists":3,"top_artists":[{"name":"Daft Punk","plays":2,"secs":294.743219953},{"name":"Sebastian Martinez","plays":1,"secs":143.498866213}],"top_songs":[{"name":"Digital Love","detail":"Daft Punk","plays":2,"secs":294.743219953}],"top_genres":[{"name":"Blues","plays":1,"secs":143.498866213}],"daily":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,8.25186545725],"streak":1}}"""

    // The failed row is the documented shape (docs/remote-control.md); the rest is real.
    const val DOWNLOADS = """{"ok":true,"downloads":{"running":true,"current":{"title":"One More Time","artist":"Daft Punk","album":"Discovery","path":"https://music.youtube.com/watch?v=FGBhQbmPwH8","duration_secs":321,"stream":true,"provider_meta":{"ytmusic.album":"MPREb_7ltM34kr0mH","ytmusic.id":"FGBhQbmPwH8"}},"percent":42.5,"queued":[{"title":"Aerodynamic","artist":"Daft Punk","album":"Discovery","path":"https://music.youtube.com/watch?v=L93-7vRfxNs","duration_secs":213,"stream":true}],"done":2,"failed":[{"track":{"title":"Digital Love","path":"https://music.youtube.com/watch?v=4whD6uAryMs"},"error":"HTTP Error 403"}],"directory":"/data/user/0/io.github.xaviiromf.cliampx/files/home/Music/cliamp-x"}}"""

    const val DOWNLOADS_IDLE = """{"ok":true,"downloads":{"directory":"/data/user/0/io.github.xaviiromf.cliampx/files/home/Music/cliamp-x"}}"""

    // The artist row is the documented shape for a non-song kind; the song row is real.
    const val RATINGS = """{"ok":true,"ratings":[{"id":"https://music.youtube.com/watch?v=plnfIj7dkJE","kind":"song","name":"Starboy (feat. Daft Punk)","artist":"The Weeknd","interest":-1,"at":"2026-10-13T22:02:59-03:00"},{"id":"UCRr1xG_2WIDs18a6cIiCxeA","kind":"artist","name":"Daft Punk","interest":1,"at":"2026-10-13T22:00:00-03:00"}]}"""

    const val THEMES = """{"ok":true,"items":["Default - Terminal colors","alucard","cover"],"themes":[{"name":"Default - Terminal colors"},{"name":"alucard","bg":"#fffbeb","accent":"#644ac9","fg":"#6c664b","bright_fg":"#1f1f1f","green":"#14710a","yellow":"#846e15","red":"#cb3a2a"},{"name":"cover","bg":"#16141c","accent":"#c792ea","fg":"#a39bb5","bright_fg":"#e6e1f0","green":"#7fdbca","yellow":"#ffcb6b","red":"#f78c6c"}]}"""

    const val PLAYLISTS_YT = """{"ok":true,"total":5,"playlists":[{"id":"fav","name":"♥ Favorites","provider":"ytmusic","section":"Your music"},{"id":"hist","name":"History","provider":"ytmusic","section":"Your music"},{"id":"local:all","name":"Local music","provider":"ytmusic","section":"Your music","track_count":8},{"id":"pl:VLPLmine","name":"Para correr","provider":"ytmusic","section":"Your music","track_count":45,"writable":true},{"id":"shelf:0","name":"Quick picks","provider":"ytmusic","section":"Home","track_count":20}]}"""

    const val PLAYLISTS_LOCAL = """{"ok":true,"total":3,"playlists":[{"id":"Favorites","name":"Favorites","provider":"local","section":"Favorites"},{"id":"Recently Played","name":"Recently Played","provider":"local","track_count":10,"duration_secs":1597},{"id":"Mi lista","name":"Mi lista","provider":"local","track_count":14,"duration_secs":3676,"writable":true}]}"""
}

class LibraryTest {
    @Test
    fun stats() {
        val s = Stats.parse(JSONObject(LibraryFixtures.STATS))
        assertEquals(4, s.plays)
        assertEquals(495.1, s.secs, 0.1)
        assertEquals(3, s.songs)
        assertEquals(1, s.streak)
        assertEquals(StatCount("Daft Punk", "", 2), s.topArtists[0])
        assertEquals(StatCount("Digital Love", "Daft Punk", 2), s.topSongs[0])
        assertEquals("Blues", s.topGenres.single().name)
        assertEquals(30, s.daily.size)
        val empty = Stats.parse(JSONObject("""{"ok":true}"""))
        assertEquals(0, empty.plays)
        assertTrue(empty.topSongs.isEmpty() && empty.daily.isEmpty())
    }

    @Test
    fun dailyChart() {
        // Four rows, top first; the busiest day fills every row, half of it two.
        assertEquals(listOf("  █", "  █", " ██", " ██"), Stats.chart(listOf(0.0, 4.0, 8.0)))
        assertEquals(listOf("▁▄█"), Stats.chart(listOf(1.0, 4.0, 8.0), rows = 1)) // eighths of one row
        assertEquals(listOf("   ", "   "), Stats.chart(listOf(0.0, 0.0, 0.0), rows = 2)) // no plays: blank, no division by 0
        val real = Stats.chart(Stats.parse(JSONObject(LibraryFixtures.STATS)).daily)
        assertEquals(4, real.size)
        assertTrue(real.all { it.length == 30 && it.endsWith("█") })
    }

    @Test
    fun downloads() {
        val d = Downloads.parse(JSONObject(LibraryFixtures.DOWNLOADS))
        assertTrue(d.running)
        assertEquals("One More Time", d.current?.name)
        assertEquals(42, d.percent)
        assertEquals(listOf("Aerodynamic"), d.queued.map { it.name })
        assertEquals(2, d.done)
        assertEquals("Digital Love" to "HTTP Error 403", d.failed.single().let { it.first.name to it.second })
        assertEquals("/data/user/0/io.github.xaviiromf.cliampx/files/home/Music/cliamp-x", d.directory)
        val idle = Downloads.parse(JSONObject(LibraryFixtures.DOWNLOADS_IDLE))
        assertFalse(idle.running)
        assertNull(idle.current)
        assertTrue(idle.queued.isEmpty() && idle.failed.isEmpty())
        assertEquals(d.directory, idle.directory)
    }

    @Test
    fun ratings() {
        val r = Rating.list(JSONObject(LibraryFixtures.RATINGS))
        assertEquals(Rating("https://music.youtube.com/watch?v=plnfIj7dkJE", "song", "Starboy (feat. Daft Punk)", "The Weeknd", -1), r[0])
        assertEquals(Rating("UCRr1xG_2WIDs18a6cIiCxeA", "artist", "Daft Punk", "", 1), r[1])
        // An empty store leaves the field out.
        assertTrue(Rating.list(JSONObject("""{"ok":true}""")).isEmpty())
    }

    @Test
    fun themes() {
        val t = ThemeInfo.list(JSONObject(LibraryFixtures.THEMES))
        assertEquals(listOf("Default - Terminal colors", "alucard", "cover"), t.map { it.name })
        assertEquals("", t[0].accent) // Default has its name only
        assertEquals("#644ac9", t[1].accent)
        assertEquals("#1f1f1f", t[1].brightFg)
    }

    @Test
    fun ownListsAndLocalLists() {
        val yt = Items.playlists(JSONObject(LibraryFixtures.PLAYLISTS_YT))
        // The first section without the pinned ♥, Historial and Música local.
        assertEquals(listOf("pl:VLPLmine"), Library.own(yt).map { it.id })
        assertEquals(8, yt.first { it.id == "local:all" }.count)
        assertTrue(Library.own(emptyList()).isEmpty())
        val local = Items.playlists(JSONObject(LibraryFixtures.PLAYLISTS_LOCAL))
        assertEquals(listOf(false, false, true), local.map(Library::editable))
        // A YouTube list, and YouTube's Música local row (a local: ID), are not local playlist files.
        assertFalse(Library.editable(yt.first { it.id == "pl:VLPLmine" }))
        assertFalse(Library.editable(yt.first { it.id == "local:all" }))
    }

    @Test
    fun newListNames() {
        val taken = listOf("Mi lista", "Gym")
        for ((name, want) in listOf(
            "Mi lista" to R.string.list_exists,
            "Favorites" to R.string.list_exists,
            "Recently Played" to R.string.list_exists,
            "mi lista" to null, // the playlist files sit on a case-sensitive file system
            "Rock/Pop" to R.string.list_bad_name,
            "a\\b" to R.string.list_bad_name,
            "Sí: no" to R.string.list_bad_name,
            "¿Qué?" to R.string.list_bad_name,
            "\"x\"" to R.string.list_bad_name,
            ".." to R.string.list_bad_name,
            "." to R.string.list_bad_name,
            "...para ti" to null,
            "Lista nueva" to null,
        )) assertEquals(name, want, Library.nameError(name, taken))
    }

    @Test
    fun gridColumns() {
        // Two on a phone held upright, more across, as GridCells.Adaptive(160.dp) inside 16 dp sides.
        for ((width, want) in listOf(200f to 2, 360f to 2, 411f to 2, 544f to 3, 600f to 3, 914f to 5)) {
            assertEquals("$width dp", want, Library.columns(width))
        }
    }

    @Test
    fun settingLabels() {
        assertEquals(listOf("0.5x", "0.75x", "1x", "1.25x", "2x"), listOf(0.5, 0.75, 1.0, 1.25, 2.0).map(::speedLabel))
        assertEquals(listOf(0, 1, 2, -1), listOf("off", "track", "album", "").map(::normalizeValue))
        assertTrue(Fixtures.snapshot("""{"revision":1,"mono":true}""").mono)
        assertFalse(Fixtures.snapshot("""{"revision":1}""").mono)
    }
}
