package io.github.xaviiromf.cliampx

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.provider.Settings
import androidx.core.content.FileProvider
import androidx.core.content.edit
import androidx.core.net.toUri
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.io.File
import java.io.IOException
import java.net.ConnectException
import java.net.SocketTimeoutException
import java.net.URL
import java.net.UnknownHostException
import java.security.MessageDigest

/**
 * App updates from the GitHub releases (PLAN.md phase 4). The core finds the
 * newest release and its checksums with `upgrade --check --json`; the app
 * downloads the APK, checks its SHA-256 and opens the system installer.
 */
object Updates {
    /** The newest release, whether it is newer than this app, and its APK for this ABI. */
    class Release(val latest: String, val newer: Boolean, val apk: Apk?)

    class Apk(val name: String, val url: String, val sha256: String)

    class HashMismatch : IOException("SHA-256 mismatch")

    const val PREFS = "updates"
    const val NIGHTLY = "nightly"
    private const val LAST_CHECK = "last_check"
    private const val DAY = 24 * 60 * 60 * 1000L

    // Only release files of this repository. The SHA-256 from the release guards the bytes.
    private const val DOWNLOADS = "https://github.com/xaviiromf/cliamp-x/releases/download/"
    private const val MAX_APK = 200L shl 20
    private val SHA256 = Regex("[0-9a-f]{64}")

    fun prefs(ctx: Context) = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    /** The newer release the daily check found, which the Inicio banner offers. */
    val offer = MutableStateFlow<String?>(null)

    /** Asks the core for the newest release of the chosen channel. Blocks; throws with the core's error. */
    fun check(ctx: Context): Release {
        val args = mutableListOf("upgrade", "--check", "--json")
        if (prefs(ctx).getBoolean(NIGHTLY, false)) args += "--prerelease"
        val process = Core.command(ctx, *args.toTypedArray()).redirectErrorStream(true).start()
        // The core's HTTP timeouts end it: 30 s per request.
        val out = process.inputStream.bufferedReader().readText().trim()
        if (process.waitFor() != 0) throw IOException(out.lines().last())
        return parse(out.lines().last(), BuildConfig.ABI)
    }

    /** Reads the report and picks cliamp-x-<version>-<abi>.apk, when it is a release file with a SHA-256. */
    fun parse(json: String, abi: String): Release {
        val report = JSONObject(json)
        val latest = report.getString("latest")
        val name = "cliamp-x-${latest.removePrefix("v").substringBefore('+')}-$abi.apk"
        val assets = report.getJSONArray("assets")
        val apk = (0 until assets.length()).map(assets::getJSONObject)
            .firstOrNull { it.getString("name") == name }
            ?.let { Apk(name, it.getString("url"), it.getString("sha256")) }
            ?.takeIf { it.url.startsWith(DOWNLOADS) && SHA256.matches(it.sha256) }
        return Release(latest, report.getBoolean("newer"), apk)
    }

    /**
     * Downloads apk into dir and checks its SHA-256, which deletes a file that
     * does not match. A verified file from an earlier try is reused.
     */
    suspend fun download(dir: File, apk: Apk): File = withContext(Dispatchers.IO) {
        val file = File(dir, apk.name)
        if (file.isFile && sha256(file) == apk.sha256) return@withContext file
        dir.deleteRecursively() // older updates and broken downloads
        dir.mkdirs()
        val part = File(dir, "${apk.name}.part")
        try {
            val conn = URL(apk.url).openConnection().apply {
                connectTimeout = 30_000
                readTimeout = 30_000
                useCaches = false // not in the image cache
            }
            conn.getInputStream().use { input ->
                part.outputStream().use { out ->
                    val buffer = ByteArray(64 shl 10)
                    var total = 0L
                    while (true) {
                        ensureActive()
                        val n = input.read(buffer)
                        if (n < 0) break
                        total += n
                        if (total > MAX_APK) throw IOException("the APK is larger than ${MAX_APK shr 20} MB")
                        out.write(buffer, 0, n)
                    }
                }
            }
            if (sha256(part) != apk.sha256) throw HashMismatch()
            if (!part.renameTo(file)) throw IOException("cannot write $file")
        } finally {
            part.delete()
        }
        file
    }

    fun sha256(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(64 shl 10)
            while (true) digest.update(buffer, 0, input.read(buffer).takeIf { it >= 0 } ?: break)
        }
        return digest.digest().joinToString("") { "%02x".format(it) }
    }

    /**
     * Opens the system installer for file. When Android does not let this app
     * install apps yet, it opens that setting instead and returns false.
     */
    fun install(activity: Activity, file: File): Boolean {
        if (!activity.packageManager.canRequestPackageInstalls()) {
            activity.startActivity(Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, "package:${activity.packageName}".toUri()))
            return false
        }
        val uri = FileProvider.getUriForFile(activity, "${activity.packageName}.updates", file)
        activity.startActivity(
            Intent(Intent.ACTION_VIEW).setDataAndType(uri, "application/vnd.android.package-archive")
                .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        )
        return true
    }

    /** Whether the automatic check is due: at most one a day. Records this one. */
    fun due(ctx: Context, now: Long = System.currentTimeMillis()): Boolean {
        val prefs = prefs(ctx)
        if (!due(prefs.getLong(LAST_CHECK, 0), now)) return false
        prefs.edit { putLong(LAST_CHECK, now) }
        return true
    }

    // A clock set back counts as due, so it cannot stop the checks.
    fun due(last: Long, now: Long) = now - last !in 0 until DAY

    /** The message for a failed check or download, in the words of the app. */
    fun message(ctx: Context, e: Throwable): String {
        val text = e.message.orEmpty()
        return when {
            e is HashMismatch -> ctx.getString(R.string.update_bad_hash)
            "rate limit" in text -> ctx.getString(R.string.update_rate_limit)
            offline(e) -> ctx.getString(R.string.update_offline)
            else -> ctx.getString(R.string.update_failed, text)
        }
    }

    /** No network: Java's errors anywhere in the causes, or the Go dial and timeout errors that the core prints. */
    fun offline(e: Throwable) = generateSequence(e) { it.cause }.any { cause ->
        cause is UnknownHostException || cause is ConnectException || cause is SocketTimeoutException ||
            listOf("dial tcp", "no such host", "i/o timeout", "Client.Timeout", "network is unreachable")
                .any { it in cause.message.orEmpty() }
    }
}
