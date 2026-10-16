package io.github.xaviiromf.cliampx

import android.content.ClipData
import android.content.Context
import android.app.ActivityManager
import android.content.Intent
import android.media.AudioManager
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.Build
import android.os.PowerManager
import androidx.core.content.FileProvider
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import java.io.IOException
import java.io.RandomAccessFile
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * Enviar registro (Ajustes → Sistema): a header with the phone and the
 * settings that matter for playback, the last lines of cliamp.log, this app's
 * logcat and its crash buffer in one text file (about 300 KB at most),
 * handed to the share sheet. There is no telemetry: nothing leaves the phone
 * unless the user shares it.
 */
object Logs {
    /** The report keeps this many lines of cliamp.log and of logcat. */
    private const val CORE_LINES = 1500
    private const val LOGCAT_LINES = 1000

    /** Bytes read from the end of cliamp.log before the line cut: a few long lines cannot swell the report. */
    private const val CORE_READ = 160L shl 10

    /** logcat lines read before the noise filter drops some. */
    private const val LOGCAT_READ = 4000

    /** Tags of Android's own UI and graphics code: they log on every frame and say nothing about the player. */
    val NOISY_TAGS = setOf(
        "Choreographer", "HWUI", "OpenGLRenderer", "ViewRootImpl", "InsetsController", "ImeTracker",
        "InputMethodManager", "InputTransport", "SurfaceView", "BufferQueueProducer", "BLASTBufferQueue",
        "SurfaceControl", "ViewRootImplExtImpl", "Gralloc4", "RenderThread", "ScrollView", "WindowOnBackDispatcher",
        "ProfileInstaller", "AssistStructure", "AutofillManager", "ContentCatcher", "CompatChangeReporter",
        "DisplayManager", "VRI", "ActivityThread", "chatty", "libc", "EGL_emulation", "ConfigStore", "HostConnection",
        "GoogleInputMethodService", "InteractionJankMonitor", "FrameTracker",
    )

    /** The settings of config.toml that change how playback sounds or behaves; the file has more (accounts, paths). */
    val CONFIG_KEYS = listOf(
        "theme", "visualizer", "low_power", "quality_wifi", "quality_mobile", "premium_quality", "buffer_ms",
        "sample_rate", "resample_quality", "normalize", "crossfade", "gapless", "mono", "speed",
    )

    /** The "ui" preferences of the look: Fondo, its levels, the motion (animations) and the player visualizer. */
    val UI_KEYS = listOf("background", "background_lists", "background_player", "motion", "player_vis")

    /** What the header says. The strings are already formatted: the function that fills it reads the system. */
    data class Info(
        val app: String,
        val android: String,
        val device: String,
        val abi: String,
        val ramMb: Long,
        val lowRam: Boolean,
        val cores: Int,
        val network: String,
        val batterySaver: Boolean,
        val deviceSampleRate: String,
        val settings: List<Pair<String, String>>,
    )

    /** The header of the report, one fact per line. */
    fun header(i: Info, now: String): String = buildString {
        appendLine(i.app)
        appendLine(i.android)
        appendLine("device: ${i.device}")
        appendLine("abi: ${i.abi}")
        appendLine("ram: ${i.ramMb} MB, low-ram device: ${if (i.lowRam) "yes" else "no"}")
        appendLine("cpu cores: ${i.cores}")
        appendLine("network: ${i.network}")
        appendLine("battery saver: ${if (i.batterySaver) "on" else "off"}")
        appendLine("device sample rate: ${i.deviceSampleRate}")
        appendLine("settings:")
        if (i.settings.isEmpty()) appendLine("  (none set)")
        for ((k, v) in i.settings) appendLine("  $k = $v")
        appendLine(now)
    }

    /** The values of the wanted keys in a config.toml text, in the order of keys; a key not set is left out. */
    fun configValues(toml: String, keys: List<String>): List<Pair<String, String>> {
        val set = toml.lineSequence().map { it.trim() }.filter { !it.startsWith("#") && '=' in it }
            .associate { it.substringBefore('=').trim() to it.substringAfter('=').trim() }
        return keys.mapNotNull { k -> set[k]?.let { k to it } }
    }

    /** The last n lines of text, or all of it. Keeps the final newline when text has one. */
    fun lastLines(text: String, n: Int): String {
        if (n <= 0 || text.isEmpty()) return ""
        var from = text.length
        // A final newline ends the last line; it is not the start of one more.
        var seen = if (text.endsWith('\n')) -1 else 0
        while (from > 0 && seen < n) {
            from = text.lastIndexOf('\n', from - 1)
            if (from < 0) return text
            seen++
        }
        return text.substring(from + 1)
    }

    private val THREADTIME = Regex("""^\S+ \S+\s+\d+\s+\d+ ([VDIWEFA]) ([^:]*?)\s*: """)

    /** logcat -v threadtime text without the lines of noisy tags. Warnings and errors always stay. */
    fun dropNoise(logcat: String, noisy: Set<String>): String = logcat.lineSequence().filter { line ->
        val m = THREADTIME.find(line) ?: return@filter true // a header line or a continuation
        m.groupValues[1] in "WEFA" || m.groupValues[2] !in noisy
    }.joinToString("\n")

    /** The last max bytes of file from a line start, or a note when it is missing. */
    fun tail(file: File, max: Long): String {
        if (!file.isFile) return "(no ${file.name})\n"
        RandomAccessFile(file, "r").use { f ->
            val whole = f.length() <= max
            // One byte more than max tells whether the cut falls on a line start.
            val from = if (whole) 0L else f.length() - max - 1
            f.seek(from)
            val bytes = ByteArray((f.length() - from).toInt())
            f.readFully(bytes)
            val text = String(bytes, Charsets.UTF_8)
            // A cut starts mid-line, maybe mid-character: keep from the first line start.
            return if (whole) text else text.substringAfter('\n', "")
        }
    }

    /** The end of cliamp.log; when the core rotated it recently, the end of cliamp.log.1 comes first. */
    private fun coreLog(dir: File): String {
        val files = listOf("cliamp.log.1", "cliamp.log").map { File(dir, it) }.filter { it.isFile }
        if (files.isEmpty()) return "(no cliamp.log)\n"
        return lastLines(files.joinToString("") { tail(it, CORE_READ) }, CORE_LINES)
    }

    /** logcat -d with args; Android shows an app only its own processes' lines (the core's included). */
    private fun logcat(vararg args: String): String = try {
        val p = ProcessBuilder(listOf("logcat", "-d", "-v", "threadtime") + args).redirectErrorStream(true).start()
        p.inputStream.bufferedReader().use { it.readText() }.also { p.waitFor() }
    } catch (e: IOException) {
        "(logcat: ${e.message})\n"
    }

    private fun network(ctx: Context): String {
        val cm = ctx.getSystemService(ConnectivityManager::class.java)
        val caps = cm.getNetworkCapabilities(cm.activeNetwork ?: return "none") ?: return "none"
        return when {
            caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) -> "wifi"
            caps.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) -> "mobile data" + if (caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_METERED)) "" else " (metered)"
            caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) -> "ethernet"
            caps.hasTransport(NetworkCapabilities.TRANSPORT_VPN) -> "vpn"
            else -> "other"
        }
    }

    private fun info(ctx: Context): Info {
        val mem = ActivityManager.MemoryInfo().also { ctx.getSystemService(ActivityManager::class.java).getMemoryInfo(it) }
        val config = runCatching { Core.configFile(ctx).readText() }.getOrDefault("")
        val ui = ctx.getSharedPreferences("ui", Context.MODE_PRIVATE)
        val look = UI_KEYS.mapNotNull { k -> ui.all[k]?.let { "ui.$k" to it.toString() } }
        val rate = ctx.getSystemService(AudioManager::class.java).getProperty(AudioManager.PROPERTY_OUTPUT_SAMPLE_RATE)
        return Info(
            app = "cliamp-x ${BuildConfig.VERSION_NAME} (${BuildConfig.VERSION_CODE}) ${BuildConfig.BUILD_TYPE}",
            android = "Android ${Build.VERSION.RELEASE} (API ${Build.VERSION.SDK_INT})",
            device = "${Build.MANUFACTURER} ${Build.MODEL}",
            abi = "${BuildConfig.ABI} (phone: ${Build.SUPPORTED_ABIS.joinToString()})",
            ramMb = mem.totalMem shr 20,
            lowRam = ctx.getSystemService(ActivityManager::class.java).isLowRamDevice,
            cores = Runtime.getRuntime().availableProcessors(),
            network = network(ctx),
            batterySaver = ctx.getSystemService(PowerManager::class.java).isPowerSaveMode,
            deviceSampleRate = rate?.let { "$it Hz" } ?: "unknown",
            settings = configValues(config, CONFIG_KEYS) + look,
        )
    }

    /** Writes the report to cache/shares/ (served by the FileProvider), replacing an older one. */
    suspend fun write(ctx: Context): File = withContext(Dispatchers.IO) {
        val dir = File(ctx.cacheDir, "shares").apply { mkdirs() }
        dir.listFiles { f -> f.name.startsWith("cliamp-x-log-") }?.forEach { it.delete() }
        val now = Date()
        val file = File(dir, "cliamp-x-log-" + SimpleDateFormat("yyyyMMdd-HHmmss", Locale.ROOT).format(now) + ".txt")
        file.bufferedWriter().use { w ->
            w.write(header(info(ctx), "$now"))
            w.write("\n== cliamp.log (last $CORE_LINES lines) ==\n")
            w.write(coreLog(Core.configDir(ctx)))
            w.write("\n== logcat (this app, last $LOGCAT_LINES lines) ==\n")
            w.write(lastLines(dropNoise(logcat("-t", "$LOGCAT_READ"), NOISY_TAGS), LOGCAT_LINES))
            w.write("\n== logcat crash ==\n")
            w.write(logcat("-b", "crash", "-t", "200"))
        }
        file
    }

    /** Opens the share sheet for the report file. */
    fun share(ctx: Context, file: File) {
        val uri = FileProvider.getUriForFile(ctx, "${ctx.packageName}.updates", file)
        val send = Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_STREAM, uri)
            .putExtra(Intent.EXTRA_SUBJECT, file.name).addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        send.clipData = ClipData.newRawUri(null, uri)
        ctx.startActivity(Intent.createChooser(send, ctx.getString(R.string.send_log)))
    }
}
