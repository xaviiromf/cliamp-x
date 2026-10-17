package io.github.xaviiromf.cliampx

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.io.File
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream

class BackupTest {
    @get:Rule val tmp = TemporaryFolder()

    private fun zip(vararg entries: Pair<String, ByteArray>, manifest: ByteArray? = Backup.manifest(false)): ByteArray {
        val out = ByteArrayOutputStream()
        ZipOutputStream(out).use { z ->
            for ((n, b) in listOfNotNull(manifest?.let { Backup.MANIFEST to it }) + entries) {
                z.putNextEntry(ZipEntry(n))
                z.write(b)
                z.closeEntry()
            }
        }
        return out.toByteArray()
    }

    private fun refused(res: Int, bytes: ByteArray) {
        try {
            Backup.read(ByteArrayInputStream(bytes))
            fail("accepted")
        } catch (e: Backup.Invalid) {
            assertEquals(res, e.res)
        }
    }

    @Test
    fun namesOutsideTheWhitelistAreRefused() {
        for (name in listOf(
            "../evil.toml", "/etc/passwd", "config/../../x", "config/playlists/../../x.toml", "config/playlists/..toml", "config/playlists/.hidden.toml",
            "config/playlists/a/b.toml", "config\\config.toml", "config/ytmusic_cookies.txt", "config/cliamp.log", "config/plugins/x.lua",
            "config/themes/x.json", "data/offline.json", "app/prefs/other.json", "config/playlists/a\u0000b.toml", "config.toml",
        )) refused(R.string.backup_err_path, zip(name to "x = 1".toByteArray()))
    }

    @Test
    fun theWhitelistTakesTheUserFiles() {
        for (name in listOf("config/config.toml", "config/playlists/Mi lista (1).toml", "config/playlists/Rock ñ.toml", "config/themes/neon.toml", "data/plays.jsonl", "app/prefs/ui.json", "app/prefs/stations.json")) {
            assertTrue(name, Backup.allowed(name, false))
        }
        assertFalse(Backup.allowed("config/ytmusic_cookies.txt", false))
        assertTrue(Backup.allowed("config/ytmusic_cookies.txt", true))
    }

    @Test
    fun formatAndVersion() {
        refused(R.string.backup_err_format, zip(manifest = null))
        refused(R.string.backup_err_format, zip(manifest = "{\"format\":\"other\",\"version\":1}".toByteArray()))
        refused(R.string.backup_err_format, zip(manifest = "nope".toByteArray()))
        refused(R.string.backup_err_version, zip(manifest = "{\"format\":\"cliamp-x-backup\",\"version\":2}".toByteArray()))
        refused(R.string.backup_err_version, zip(manifest = "{\"format\":\"cliamp-x-backup\"}".toByteArray()))
        refused(R.string.backup_err_format, "not a zip".toByteArray() + ByteArray(50))
        refused(R.string.backup_err_zip, zip("config/config.toml" to ByteArray(5000) { (it * 31).toByte() }).let { it.copyOf(it.size / 2) })
    }

    @Test
    fun sizeLimits() {
        refused(R.string.backup_err_size, zip("data/search_history.json" to ByteArray(Backup.MAX_ENTRY + 1) { ' '.code.toByte() }))
        val big = ByteArray(Backup.MAX_ENTRY - 1) { '\n'.code.toByte() }
        refused(R.string.backup_err_total, zip("config/config.toml" to big, "config/history.toml" to big, "data/plays.jsonl" to big))
    }

    @Test
    fun contentMustBeValid() {
        refused(R.string.backup_err_content, zip("config/config.toml" to byteArrayOf(0x61, 0xC3.toByte(), 0x28)))
        refused(R.string.backup_err_content, zip("config/config.toml" to "a\u0000b".toByteArray()))
        refused(R.string.backup_err_content, zip("config/taste.json" to "{not json".toByteArray()))
        refused(R.string.backup_err_content, zip("config/taste.json" to "42".toByteArray()))
        refused(R.string.backup_err_content, zip("data/plays.jsonl" to "{}\nnot json\n".toByteArray()))
        refused(R.string.backup_err_content, zip("app/prefs/ui.json" to "{\"a\":[\"x\",1]}".toByteArray()))
    }

    @Test
    fun cookiesNeedTheManifestToSayYes() {
        val cookies = "config/ytmusic_cookies.txt" to "# Netscape HTTP Cookie File\n".toByteArray()
        refused(R.string.backup_err_path, zip(cookies))
        assertEquals(setOf("config/ytmusic_cookies.txt"), Backup.read(ByteArrayInputStream(zip(cookies, manifest = Backup.manifest(true)))).files.keys)
    }

    @Test
    fun exportLeavesOutTheSessionAndCredentials() {
        val config = tmp.newFolder("config")
        val data = tmp.newFolder("data")
        File(config, "config.toml").writeText("mouse = true\nmusic_dir = \"/m\"\n[scrobble]\nlistenbrainz_token = \"abc\"\nlastfm_session_key = \"k\"\nlastfm_api_secret = \"s\"\n[navidrome]\npassword = \"p\"\n")
        File(config, "ytmusic_cookies.txt").writeText("cookie")
        File(config, "ytmusic_auth.json").writeText("{}")
        File(config, "cliamp.log").writeText("log")
        File(config, "resume.json").writeText("{}")
        File(config, "cliamp-x.sock").writeText("")
        File(config, "playlists").mkdirs()
        File(config, "playlists/a.toml").writeText("[[track]]\n")
        File(data, "plays.jsonl").writeText("{\"a\":1}\n")
        File(data, "offline.json").writeText("{}")
        File(data, "lyrics").mkdirs()
        File(data, "lyrics/x.json").writeText("{}")

        val without = Backup.collect(config, data, emptyMap(), false)
        assertEquals(setOf("config/config.toml", "config/playlists/a.toml", "data/plays.jsonl"), without.keys)
        val toml = String(without.getValue("config/config.toml"))
        assertTrue(toml.contains("music_dir"))
        for (secret in listOf("abc", "lastfm", "password", "token")) assertFalse(secret, toml.contains(secret))

        val with = Backup.collect(config, data, emptyMap(), true)
        assertTrue("config/ytmusic_cookies.txt" in with.keys && "config/ytmusic_auth.json" in with.keys)
    }

    @Test
    fun roundTripRestoresEveryFile() {
        val config = tmp.newFolder("c1")
        val data = tmp.newFolder("d1")
        File(config, "favorites.toml").writeText("[[track]]\ntitle = \"Canción\"\n")
        File(config, "taste.json").writeText("{\"liked\":[]}")
        File(config, "playlists").mkdirs()
        File(config, "playlists/Mi lista ñ.toml").writeText("[[track]]\n")
        File(config, "themes").mkdirs()
        File(config, "themes/neon.toml").writeText("bg = \"#000000\"\n")
        File(data, "plays.jsonl").writeText("{\"a\":1}\n{\"a\":2}\n")
        val prefs = Backup.prefsToJson(mapOf("phone_wide" to true, "n" to 3, "l" to 5L, "f" to 1.5f, "s" to "x", "set" to setOf("a", "b")))
        val files = Backup.collect(config, data, mapOf("app/prefs/music.json" to prefs.toString().toByteArray()), true)

        val out = ByteArrayOutputStream()
        Backup.write(out, files, false)
        val back = Backup.read(ByteArrayInputStream(out.toByteArray()))
        assertFalse(back.session)
        assertEquals(files.keys, back.files.keys)

        val config2 = tmp.newFolder("c2")
        val data2 = tmp.newFolder("d2")
        File(config2, "favorites.toml").writeText("old")
        Backup.restore(config2, data2, back.files)
        for ((n, b) in files) if (!n.startsWith("app/")) assertArrayEquals(n, b, File(if (n.startsWith("data/")) data2 else config2, n.substringAfter('/')).readBytes())
        assertEquals(listOf("favorites.toml", "playlists", "taste.json", "themes"), config2.list()!!.sorted())
        val typed = Backup.prefsFromJson(org.json.JSONObject(String(back.files.getValue("app/prefs/music.json"))))
        assertEquals(true, typed["phone_wide"])
        assertEquals(3, typed["n"])
        assertEquals(5L, typed["l"])
        assertEquals(1.5f, typed["f"])
        assertEquals(setOf("a", "b"), typed["set"])
        assertNull(typed["none"])
    }

    @Test
    fun aFailedRestoreChangesNothing() {
        val config = tmp.newFolder("c")
        val data = tmp.newFolder("d")
        File(config, "favorites.toml").writeText("old")
        // data/ cannot be created under a file: the write of the second file fails before any rename.
        val blocked = File(tmp.root, "blocked").apply { writeText("") }
        try {
            Backup.restore(config, File(blocked, "sub"), linkedMapOf("config/favorites.toml" to "new".toByteArray(), "data/plays.jsonl" to "{}".toByteArray()))
            fail("restored")
        } catch (e: Exception) {
            // expected
        }
        assertEquals("old", File(config, "favorites.toml").readText())
        assertTrue(File(config, ".favorites.toml.restore").let { !it.exists() })
        assertEquals(0, data.list()!!.size)
    }
}
