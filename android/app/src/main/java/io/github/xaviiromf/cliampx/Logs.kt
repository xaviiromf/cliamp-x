package io.github.xaviiromf.cliampx

import android.content.ClipData
import android.content.Context
import android.content.Intent
import android.os.Build
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
 * Enviar registro (Ajustes → Sistema): the end of cliamp.log, this app's
 * logcat and its crash buffer in one text file, handed to the share sheet.
 * There is no telemetry: nothing leaves the phone unless the user shares it.
 */
object Logs {
    /** At most the last 512 KB of cliamp.log. */
    private const val MAX_CORE_LOG = 512L shl 10

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

    /** logcat -d with args; Android shows an app only its own processes' lines (the core's included). */
    private fun logcat(vararg args: String): String = try {
        val p = ProcessBuilder(listOf("logcat", "-d", "-v", "threadtime") + args).redirectErrorStream(true).start()
        p.inputStream.bufferedReader().use { it.readText() }.also { p.waitFor() }
    } catch (e: IOException) {
        "(logcat: ${e.message})\n"
    }

    /** Writes the report to cache/shares/ (served by the FileProvider), replacing an older one. */
    suspend fun write(ctx: Context): File = withContext(Dispatchers.IO) {
        val dir = File(ctx.cacheDir, "shares").apply { mkdirs() }
        dir.listFiles { f -> f.name.startsWith("cliamp-x-log-") }?.forEach { it.delete() }
        val now = Date()
        val file = File(dir, "cliamp-x-log-" + SimpleDateFormat("yyyyMMdd-HHmmss", Locale.ROOT).format(now) + ".txt")
        file.bufferedWriter().use { w ->
            w.write("cliamp-x ${BuildConfig.VERSION_NAME} (${BuildConfig.VERSION_CODE}) ${BuildConfig.ABI}\n")
            w.write("Android ${Build.VERSION.RELEASE} (API ${Build.VERSION.SDK_INT}) · ${Build.MANUFACTURER} ${Build.MODEL}\n")
            w.write("$now\n\n== cliamp.log ==\n")
            w.write(tail(File(Core.configDir(ctx), "cliamp.log"), MAX_CORE_LOG))
            w.write("\n== logcat ==\n")
            w.write(logcat())
            w.write("\n== logcat crash ==\n")
            w.write(logcat("-b", "crash"))
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
