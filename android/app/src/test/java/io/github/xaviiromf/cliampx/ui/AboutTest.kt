package io.github.xaviiromf.cliampx.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

class AboutTest {
    @Test
    fun everyNoticeHasItsLicenseText() {
        // make android-core writes the Go modules' notices; a unit test runs without it.
        for (n in NOTICES.filter { it.file != GO_NOTICES }) {
            assertTrue("${n.name}: no ${n.file}", File("src/main/assets/licenses/${n.file}").isFile)
        }
    }

    @Test
    fun theSourceLicenseIsTheRepositoryLicense() {
        assertEquals(File("../../LICENSE").readText(), File("src/main/assets/licenses/cliamp-x-MIT.txt").readText())
    }
}
