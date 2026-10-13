package io.github.xaviiromf.cliampx

import android.content.Context
import android.os.Environment
import com.yausername.ffmpeg.FFmpeg
import com.yausername.youtubedl_android.YoutubeDL
import java.io.File
import java.util.Locale
import java.util.TimeZone

/** Where the Go core lives and the environment it runs in. */
object Core {
    fun binary(ctx: Context) = File(ctx.applicationInfo.nativeLibraryDir, "libcliamp.so")
    fun configDir(ctx: Context) = File(ctx.filesDir, "config")
    fun configFile(ctx: Context) = File(configDir(ctx), "config.toml")
    fun socket(ctx: Context) = File(configDir(ctx), "cliamp-x.sock")
    fun cookies(ctx: Context) = File(configDir(ctx), "ytmusic_cookies.txt")

    /**
     * Unpacks Python, yt-dlp and ffmpeg (youtubedl-android does it once per
     * library version), makes the directories and writes the first config.
     * Slow on the first start: call it off the main thread.
     */
    fun prepare(ctx: Context) {
        YoutubeDL.getInstance().init(ctx)
        FFmpeg.getInstance().init(ctx)
        listOf(File(ctx.filesDir, "home"), configDir(ctx), File(ctx.filesDir, "data"), File(ctx.cacheDir, "tmp"))
            .forEach { it.mkdirs() }
        val config = configFile(ctx)
        if (!config.exists()) {
            @Suppress("DEPRECATION") // only the path is used, as locallib reads it by path
            val music = Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_MUSIC).path
            config.writeText(
                """
                |# Written by the cliamp-x app on its first start. config.toml.example lists every key.
                |mouse = true
                |low_power = true
                |music_dir = "$music"
                |""".trimMargin()
            )
        }
    }

    /** The environment of the core: PLAN.md phase 2 step 3 and the extbin variables. */
    fun env(ctx: Context): Array<String> {
        val lib = ctx.applicationInfo.nativeLibraryDir
        // These paths copy YoutubeDL.init and executeImpl of youtubedl-android 0.18.1.
        val base = File(ctx.noBackupFilesDir, YoutubeDL.baseName)
        val python = File(base, "packages/python")
        val pythonHome = "$python/usr"
        val libraryPath = "$python/usr/lib:$base/packages/ffmpeg/usr/lib:$base/packages/aria2c/usr/lib"
        val certs = "$python/usr/etc/tls/cert.pem"
        val ytdlp = File(base, "${YoutubeDL.ytdlpDirName}/${YoutubeDL.ytdlpBin}")
        val locale = Locale.getDefault()
        val sep = "\u001f"
        return arrayOf(
            "HOME=${ctx.filesDir}/home",
            "CLIAMP_CONFIG_DIR=${configDir(ctx)}",
            "XDG_DATA_HOME=${ctx.filesDir}/data",
            "XDG_CACHE_HOME=${ctx.cacheDir}",
            "TMPDIR=${ctx.cacheDir}/tmp",
            "TERM=xterm-256color",
            "COLORTERM=truecolor",
            "LANG=${listOf(locale.language, locale.country).filter { it.isNotEmpty() }.joinToString("_")}.UTF-8",
            "TZ=${TimeZone.getDefault().id}",
            "PATH=$lib:${System.getenv("PATH")}",
            // yt-dlp is libpython.so running the yt-dlp zipapp. QuickJS solves the
            // YouTube signatures, as the library passes it.
            "CLIAMP_YTDLP=$lib/libpython.so $ytdlp --js-runtimes quickjs:$lib/libqjs.so",
            "CLIAMP_YTDLP_ENV=LD_LIBRARY_PATH=$libraryPath${sep}SSL_CERT_FILE=$certs${sep}PYTHONHOME=$pythonHome${sep}HOME=$pythonHome",
            "CLIAMP_FFMPEG=$lib/libffmpeg.so",
            "CLIAMP_FFMPEG_ENV=LD_LIBRARY_PATH=$libraryPath${sep}SSL_CERT_FILE=$certs",
        )
    }
}
