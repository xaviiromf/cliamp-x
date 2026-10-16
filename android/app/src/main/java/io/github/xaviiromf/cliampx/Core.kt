package io.github.xaviiromf.cliampx

import android.content.Context
import android.media.AudioManager
import android.os.Environment
import com.yausername.ffmpeg.FFmpeg
import com.yausername.youtubedl_android.YoutubeDL
import java.io.File
import java.util.TimeZone
import java.util.concurrent.atomic.AtomicBoolean

/** Where the Go core lives and the environment it runs in. */
object Core {
    fun binary(ctx: Context) = File(ctx.applicationInfo.nativeLibraryDir, "libcliamp.so")
    fun configDir(ctx: Context) = File(ctx.filesDir, "config")
    fun configFile(ctx: Context) = File(configDir(ctx), "config.toml")
    /** The core's data directory: HOME/.local/share/cliamp-x (play log, search history), which its HOME decides. */
    fun dataDir(ctx: Context) = File(ctx.filesDir, "home/.local/share/cliamp-x")
    fun socket(ctx: Context) = File(configDir(ctx), "cliamp-x.sock")
    fun cookies(ctx: Context) = File(configDir(ctx), "ytmusic_cookies.txt")

    /** The yt-dlp zipapp that CLIAMP_YTDLP runs and Ytdlp replaces. */
    fun ytdlp(ctx: Context) = File(File(ctx.noBackupFilesDir, YoutubeDL.baseName), "${YoutubeDL.ytdlpDirName}/${YoutubeDL.ytdlpBin}")

    /** The yt-dlp plugin directory that CLIAMP_YTDLP passes with --plugin-dirs. */
    private fun ytdlpPlugins(ctx: Context) = File(ctx.filesDir, "yt-dlp-plugins")

    /**
     * yt-dlp solves YouTube's JS challenge from the whole player on each run:
     * about 11 s of QuickJS on the A55, for every song. Its cache of the
     * preprocessed player is off by default (big files, never pruned), and
     * this plugin turns it on, so a later song solves from the cache (UI-SPEC
     * §12.2). The section carries the yt-dlp version, as a new solver may
     * preprocess differently; pruneSolverCache removes old files.
     */
    private val SOLVER_CACHE = """
        |# Written by the cliamp-x app at each core start.
        |from yt_dlp.extractor.youtube.jsc._builtin.ejs import EJSBaseJCP
        |from yt_dlp.version import __version__
        |
        |EJSBaseJCP._ENABLE_PREPROCESSED_PLAYER_CACHE = True
        |EJSBaseJCP._CACHE_SECTION = f'challenge-solver-{__version__}'
        |""".trimMargin()

    /** Deletes the cached players (about 4 MB each) older than a week, and the sections they leave empty. */
    fun pruneSolverCache(cache: File, now: Long) {
        File(cache, "yt-dlp").listFiles { f -> f.name.startsWith("challenge-solver") }?.forEach { dir ->
            dir.listFiles()?.filter { now - it.lastModified() > 7 * 86_400_000L }?.forEach { it.delete() }
            dir.delete() // only when empty
        }
    }

    /**
     * The path of a folder from the system folder picker, as the core reads
     * music by path: external storage's `primary:Music/Rock` under primary
     * (the phone's storage), `home:` under its Documents, an SD card's
     * `1234-ABCD:Music` under /storage/1234-ABCD, and the Downloads
     * provider's `raw:/path`. Null for any other provider, such as a cloud.
     */
    fun treePath(authority: String?, docId: String?, primary: String): String? {
        if (docId == null || docId.split('/').any { it == ".." }) return null
        if (authority == "com.android.providers.downloads.documents") return docId.removePrefix("raw:").takeIf { docId.startsWith("raw:/") }
        if (authority != "com.android.externalstorage.documents") return null
        val parts = docId.split(':', limit = 2)
        if (parts.size != 2) return null
        val (volume, path) = parts
        val root = when {
            volume == "primary" -> primary
            volume == "home" -> "$primary/Documents"
            volume.matches(Regex("[0-9A-Za-z-]+")) -> "/storage/$volume"
            else -> return null
        }
        return if (path.isEmpty()) root else "$root/${path.trimEnd('/')}"
    }

    private val unpacked = AtomicBoolean(false)

    /**
     * Unpacks Python, yt-dlp and ffmpeg (youtubedl-android does it once per
     * library version), makes the directories and writes the first config.
     * Slow on the first start: call it off the main thread.
     */
    fun prepare(ctx: Context) {
        // Once per process: a core restart needs no second unpack check.
        if (!unpacked.get()) {
            YoutubeDL.getInstance().init(ctx)
            FFmpeg.getInstance().init(ctx)
            unpacked.set(true)
        }
        listOf(File(ctx.filesDir, "home"), configDir(ctx), File(ctx.filesDir, "data"), File(ctx.cacheDir, "tmp"))
            .forEach { it.mkdirs() }
        val plugin = File(ytdlpPlugins(ctx), "cliampx/yt_dlp_plugins/extractor/cliampx_solver_cache.py")
        if (!plugin.isFile || plugin.readText() != SOLVER_CACHE) {
            plugin.parentFile?.mkdirs()
            plugin.writeText(SOLVER_CACHE)
        }
        pruneSolverCache(ctx.cacheDir, System.currentTimeMillis())
        val config = configFile(ctx)
        if (!config.exists()) {
            @Suppress("DEPRECATION") // only the path is used, as locallib reads it by path
            val music = Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_MUSIC).path
            config.writeText(
                """
                |# Written by the cliamp-x app on its first start. config.toml.example lists every key.
                |mouse = true
                |music_dir = "$music"
                |""".trimMargin()
            )
        }
    }

    /** Runs the core once with args, as `libcliamp.so open` or `upgrade --check`, in the core's environment. */
    fun command(ctx: Context, vararg args: String) = ProcessBuilder(binary(ctx).path, *args).apply {
        environment().apply {
            clear()
            env(ctx).forEach { val (k, v) = it.split("=", limit = 2); put(k, v) }
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
        val ytdlp = ytdlp(ctx)
        val locale = Language.locale(ctx)
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
            // YouTube signatures, as the library passes it, with the solver cache plugin.
            "CLIAMP_YTDLP=$lib/libpython.so $ytdlp --js-runtimes quickjs:$lib/libqjs.so --plugin-dirs ${ytdlpPlugins(ctx)}",
            "CLIAMP_YTDLP_ENV=LD_LIBRARY_PATH=$libraryPath${sep}SSL_CERT_FILE=$certs${sep}PYTHONHOME=$pythonHome${sep}HOME=$pythonHome",
            "CLIAMP_FFMPEG=$lib/libffmpeg.so",
            "CLIAMP_FFMPEG_ENV=LD_LIBRARY_PATH=$libraryPath${sep}SSL_CERT_FILE=$certs",
        ) + listOfNotNull(sampleRateEnv(ctx.getSystemService(AudioManager::class.java)?.getProperty(AudioManager.PROPERTY_OUTPUT_SAMPLE_RATE)))
    }

    /**
     * The phone's native output rate for the core (player/device_android.go), so
     * it skips a second resample and Android can grant a FAST track; null when
     * the property is missing or not a sane rate.
     */
    fun sampleRateEnv(property: String?) =
        property?.toIntOrNull()?.takeIf { it in 8000..192000 }?.let { "CLIAMP_DEVICE_SAMPLE_RATE=$it" }
}
