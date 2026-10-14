package io.github.xaviiromf.cliampx

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class IntegrationsTest {
    @Test
    fun snapshotQualityAndFormat() {
        val s = Fixtures.snapshot(
            """{"revision":1,"quality_wifi":"alta","quality_mobile":"ahorro","premium_quality":true,"scrobble_lastfm":true,""" +
                """"audio_codec":"opus","audio_kbps":160,"audio_rate":48000}""",
        )
        assertEquals("alta", s.qualityWifi)
        assertEquals("ahorro", s.qualityMobile)
        assertTrue(s.premiumQuality)
        assertTrue(s.scrobbleLastfm)
        assertFalse(s.scrobbleListenbrainz)
        assertEquals("OPUS · 160 kbps · 48 kHz", audioFormat(s.audioCodec, s.audioKbps, s.audioRate))
    }

    @Test
    fun audioFormatParts() {
        for ((args, want) in listOf(
            Triple("", 0, 0) to "",
            Triple("aac", 0, 0) to "AAC",
            Triple("mp3", 320, 44100) to "MP3 · 320 kbps · 44.1 kHz",
            Triple("", 128, 0) to "128 kbps",
            Triple("flac", 0, 96000) to "FLAC · 96 kHz",
        )) {
            assertEquals(want, audioFormat(args.first, args.second, args.third))
        }
    }

    @Test
    fun downloadRules() {
        assertTrue(Offline.allowed(false, null))
        assertTrue(Offline.allowed(false, "mobile"))
        assertTrue(Offline.allowed(true, "wifi"))
        assertFalse(Offline.allowed(true, "mobile"))
        assertFalse(Offline.allowed(true, null))
        assertEquals("wifi", Offline.networkKind(true))
        assertEquals("mobile", Offline.networkKind(false))
    }

    @Test
    fun cacheDue() {
        val h = Offline.INTERVAL_MS
        data class Case(val on: Boolean, val wifiOnly: Boolean, val net: String?, val last: Long, val now: Long, val due: Boolean)
        for (c in listOf(
            Case(false, false, "wifi", 0, h, false), // off
            Case(true, true, "mobile", 0, h, false), // Wi-Fi only on mobile data
            Case(true, true, "wifi", 0, h, true),
            Case(true, false, "mobile", 0, h, true),
            Case(true, true, "wifi", h, h + h - 1, false), // filled less than 12 h ago
            Case(true, true, "wifi", h, h + h, true),
            Case(true, true, "wifi", 5 * h, h, true), // the clock went back
        )) {
            assertEquals(c.toString(), c.due, Offline.cacheDue(c.on, c.wifiOnly, c.net, c.last, c.now))
        }
    }
}
