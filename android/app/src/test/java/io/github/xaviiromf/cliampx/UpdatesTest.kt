package io.github.xaviiromf.cliampx

import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.io.IOException
import java.net.UnknownHostException

class UpdatesTest {
    @get:Rule val tmp = TemporaryFolder()

    private val hash = "ab".repeat(32)
    private val base = "https://github.com/xaviiromf/cliamp-x/releases/download/nightly/"

    /** A report as `cliamp-x upgrade --check --json` prints it. */
    private fun report(latest: String, vararg assets: Triple<String, String, String>) =
        """{"current":"v2.2.1","latest":"$latest","prerelease":true,"newer":true,"assets":[""" +
            assets.joinToString(",") { (name, url, sha) -> """{"name":"$name","url":"$url","sha256":"$sha"}""" } + "]}"

    @Test
    fun parsePicksTheApkOfTheVersionAndAbi() {
        val v = "v2.2.2-nightly.20261013.812+abc1234"
        val apk = "cliamp-x-2.2.2-nightly.20261013.812-arm64-v8a.apk"
        val x86 = "cliamp-x-2.2.2-nightly.20261013.812-x86_64.apk"
        val json = report(v, Triple("checksums.txt", base + "checksums.txt", ""), Triple(apk, base + apk, hash), Triple(x86, base + x86, hash))
        val release = Updates.parse(json, "arm64-v8a")
        assertEquals(v, release.latest)
        assertTrue(release.newer)
        assertEquals(apk, release.apk?.name)
        assertEquals(base + apk, release.apk?.url)
        assertEquals(hash, release.apk?.sha256)
        assertEquals(x86, Updates.parse(json, "x86_64").apk?.name)
    }

    @Test
    fun parseRefusesApksItCannotTrust() {
        val apk = "cliamp-x-2.2.2-arm64-v8a.apk"
        for ((url, sha) in listOf(
            "https://evil.example/xaviiromf/cliamp-x/releases/download/v2.2.2/$apk" to hash,
            "http://github.com/xaviiromf/cliamp-x/releases/download/v2.2.2/$apk" to hash,
            "https://github.com/someone/cliamp-x/releases/download/v2.2.2/$apk" to hash,
            "https://github.com/xaviiromf/cliamp-x/releases/download/v2.2.2/$apk" to "",
            "https://github.com/xaviiromf/cliamp-x/releases/download/v2.2.2/$apk" to "AB".repeat(32),
        )) {
            assertNull("$url $sha", Updates.parse(report("v2.2.2", Triple(apk, url, sha)), "arm64-v8a").apk)
        }
        // v2.2.1 has no Android build: the release is found, without an APK.
        val release = Updates.parse(report("v2.2.1", Triple("cliamp-x-linux-amd64", base + "cliamp-x-linux-amd64", hash)), "arm64-v8a")
        assertEquals("v2.2.1", release.latest)
        assertNull(release.apk)
    }

    @Test
    fun downloadChecksTheHash() = runBlocking {
        val source = tmp.newFile("source.apk").apply { writeText("new cliamp-x") }
        val sha = Updates.sha256(source)
        val dir = File(tmp.root, "updates")
        val good = Updates.Apk("cliamp-x-2.2.2-arm64-v8a.apk", source.toURI().toString(), sha)

        val file = Updates.download(dir, good)
        assertEquals("new cliamp-x", file.readText())
        // A verified file is reused: the URL is not read again.
        assertEquals(file, Updates.download(dir, Updates.Apk(good.name, "file:///missing", sha)))

        try {
            Updates.download(dir, Updates.Apk(good.name, source.toURI().toString(), hash))
            fail("a wrong SHA-256 was accepted")
        } catch (_: Updates.HashMismatch) {
        }
        assertEquals("the mismatched download stays on disk", 0, dir.listFiles()?.size)
    }

    @Test
    fun dueOncePerDay() {
        val day = 24 * 60 * 60 * 1000L
        assertTrue(Updates.due(0, 1_000 * day))
        assertFalse(Updates.due(1_000 * day, 1_000 * day + day - 1))
        assertTrue(Updates.due(1_000 * day, 1_001 * day))
        assertTrue("a clock set back", Updates.due(1_000 * day, 999 * day))
    }

    @Test
    fun offlineErrors() {
        assertTrue(Updates.offline(IOException("wrapped", UnknownHostException("api.github.com"))))
        assertTrue(Updates.offline(IOException("checking latest version: Get \"https://api.github.com/\": dial tcp: lookup api.github.com: no such host")))
        assertFalse(Updates.offline(IOException("checking latest version: GitHub API rate limit exceeded: try again in an hour, or set GITHUB_TOKEN")))
    }
}
