package io.github.xaviiromf.cliampx

import org.junit.Assert.assertEquals
import org.junit.Test

class MusicFoldersTest {
    @Test
    fun folderOfAMediaStorePath() {
        for ((path, want) in listOf(
            "/storage/emulated/0/Music/A/song.mp3" to "/storage/emulated/0/Music/A",
            "/storage/1234-ABCD/x.flac" to "/storage/1234-ABCD",
            "/song.mp3" to null,
            "song.mp3" to null,
            "" to null,
        )) assertEquals(path, want, MusicFolders.folderOf(path))
    }

    @Test
    fun distinctFoldersDropRepeatsAndNestedOnes() {
        for ((input, want) in listOf(
            emptyList<String>() to emptyList(),
            listOf("/s/Music/A", "/s/Download/B", "/s/Music/A") to listOf("/s/Download/B", "/s/Music/A"),
            listOf("/s/Music/A/Live", "/s/Music/A/", "/s/Music/AB") to listOf("/s/Music/A", "/s/Music/AB"),
            listOf("/s/Music/A/x", "/s/Music") to listOf("/s/Music"),
            listOf("", "/") to emptyList(),
        )) assertEquals(input.toString(), want, MusicFolders.distinct(input))
    }
}
