package io.github.xaviiromf.cliampx

import android.media.AudioDeviceInfo
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

    @Test
    fun routePick() {
        val bt = AudioDeviceInfo.TYPE_BLUETOOTH_A2DP
        val wired = AudioDeviceInfo.TYPE_WIRED_HEADPHONES
        val speaker = AudioDeviceInfo.TYPE_BUILTIN_SPEAKER
        for ((outputs, want) in listOf(
            emptyList<Pair<Int, String>>() to "speaker",
            listOf(speaker to "SM-A556E") to "speaker",
            listOf(speaker to "", wired to "") to "wired",
            listOf(speaker to "", wired to "", bt to " Galaxy Buds2 ") to "bt:Galaxy Buds2",
            listOf(speaker to "", AudioDeviceInfo.TYPE_USB_HEADSET to "") to "usb",
            listOf(AudioDeviceInfo.TYPE_BLUETOOTH_SCO to "Buds", speaker to "") to "speaker", // calls only
        )) {
            assertEquals(outputs.toString(), want, Routes.pick(outputs))
        }
        assertTrue(Routes.headphones(bt))
        assertFalse(Routes.headphones(speaker))
    }

    @Test
    fun eqStep() {
        data class Case(val route: String, val synced: String?, val preset: String, val auto: Boolean, val saved: String?, val want: Routes.EqStep)
        for (c in listOf(
            Case("bt:Buds", "speaker", "Flat", false, "Bass", Routes.EqStep(apply = "Bass")), // new output with its own preset
            Case("bt:Buds", "speaker", "Flat", false, null, Routes.EqStep()), // new output, nothing saved: keep
            Case("bt:Buds", "speaker", "Bass", false, "Bass", Routes.EqStep()), // already on it
            Case("bt:Buds", null, "Flat", true, "Bass", Routes.EqStep()), // the EQ by genre picks
            Case("speaker", "speaker", "Rock", false, "Flat", Routes.EqStep(save = true)), // the user picked one
            Case("speaker", "speaker", "Rock", false, "Rock", Routes.EqStep()),
            Case("speaker", "speaker", "Custom", false, "Flat", Routes.EqStep()),
            Case("speaker", "speaker", "Rock", true, null, Routes.EqStep()),
        )) {
            assertEquals(c.toString(), c.want, Routes.eqStep(c.route, c.synced, c.preset, c.auto, c.saved))
        }
    }

    @Test
    fun shareLinks() {
        fun track(path: String) = Track("{}", path, "t", "a", "", "", 0, emptyMap())
        for ((path, want) in listOf(
            "https://music.youtube.com/watch?v=dQw4w9WgXcQ" to "https://music.youtube.com/watch?v=dQw4w9WgXcQ",
            "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PL1" to "https://music.youtube.com/watch?v=dQw4w9WgXcQ",
            "/storage/emulated/0/Music/a.mp3" to null,
            "https://example.com/watch?v=dQw4w9WgXcQ" to null,
            "https://music.youtube.com/watch?v=short" to null,
        )) {
            assertEquals(path, want, shareUrl(track(path)))
        }
    }

    @Test
    fun autoParent() {
        assertEquals("home/2", Auto.parent("home/2/5"))
        assertEquals("fav", Auto.parent("fav/3"))
        assertEquals(Auto.ROOT, Auto.parent("home"))
    }
}
