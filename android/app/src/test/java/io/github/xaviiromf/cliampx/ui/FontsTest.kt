package io.github.xaviiromf.cliampx.ui

import io.github.xaviiromf.cliampx.Updates
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.io.IOException
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream

class FontsTest {
    @get:Rule val tmp = TemporaryFolder()

    @Test
    fun theAssetPinsEveryFont() {
        val fonts = parseFonts(File("src/main/assets/fonts.json").readText())
        assertTrue("only ${fonts.size} fonts", fonts.size >= 15)
        assertEquals(fonts.size, fonts.map { it.id }.toSet().size)
        for (f in fonts) {
            assertTrue("${f.id}: no ${f.license}", File("src/main/assets/licenses/${f.license}").isFile)
            // A variable font draws bold from its axis, so it pins no bold file.
            if (f.variable) assertEquals(null, f.bold)
        }
        for ((id, _) in Fonts.BUNDLED) assertFalse(fonts.any { it.id == id })
    }

    @Test
    fun parseRejectsUnpinnedFiles() {
        val sha = "ab".repeat(32)
        for ((json, why) in listOf(
            """[{"id":"a","name":"A","license":"x","regular":{"url":"http://e.org/a.ttf","sha256":"$sha"}}]""" to "http",
            """[{"id":"a","name":"A","license":"x","regular":{"url":"https://e.org/a.ttf","sha256":"abc"}}]""" to "short hash",
            """[{"id":"a","name":"A","license":"x","regular":{"url":"https://e.org/a.ttf","sha256":"$sha"},"bold":{"url":"https://e.org/b.ttf","sha256":""}}]""" to "bold hash",
            """[{"id":"../a","name":"A","license":"x","regular":{"url":"https://e.org/a.ttf","sha256":"$sha"}}]""" to "id",
            """[{"id":"a","name":"A","license":"x"}]""" to "no regular",
        )) {
            try {
                parseFonts(json)
                fail("accepted: $why")
            } catch (_: Exception) {
            }
        }
    }

    private fun source(name: String, bytes: ByteArray) = File(tmp.root, name).apply { writeBytes(bytes) }

    private fun spec(regular: FontFile, bold: FontFile? = null) = FontSpec("t", "T", "x", regular, bold, false)

    @Test
    fun installsPlainFilesAndZipEntries() = runBlocking {
        val r = source("r.ttf", "regular".toByteArray())
        val zip = File(tmp.root, "f.zip")
        ZipOutputStream(zip.outputStream()).use { z ->
            z.putNextEntry(ZipEntry("TTF/B.ttf"))
            z.write("bold".toByteArray())
            z.closeEntry()
        }
        val dir = tmp.newFolder("fonts")
        var last = 0f
        installFont(dir, spec(FontFile(r.toURI().toString(), Updates.sha256(r)), FontFile(zip.toURI().toString(), Updates.sha256(zip), "TTF/B.ttf"))) { last = it }
        assertEquals("regular", File(dir, "t-regular").readText())
        assertEquals("bold", File(dir, "t-bold").readText())
        assertEquals(1f, last)
        assertEquals(setOf("t-regular", "t-bold"), dir.list()!!.toSet()) // no parts left
    }

    @Test
    fun aHashMismatchKeepsNothing() = runBlocking {
        val r = source("r.ttf", "regular".toByteArray())
        val dir = tmp.newFolder("fonts")
        try {
            installFont(dir, spec(FontFile(r.toURI().toString(), "00".repeat(32))))
            fail("installed a file with the wrong hash")
        } catch (_: Updates.HashMismatch) {
        }
        assertEquals(0, dir.list()!!.size)
    }

    @Test
    fun aMissingZipEntryFails() = runBlocking {
        val zip = File(tmp.root, "f.zip")
        ZipOutputStream(zip.outputStream()).use { z -> z.putNextEntry(ZipEntry("other.ttf")); z.closeEntry() }
        val dir = tmp.newFolder("fonts")
        try {
            installFont(dir, spec(FontFile(zip.toURI().toString(), Updates.sha256(zip), "TTF/R.ttf")))
            fail("installed a missing entry")
        } catch (e: IOException) {
            assertTrue(e.message!!.contains("TTF/R.ttf"))
        }
        assertEquals(0, dir.list()!!.size)
    }
}
