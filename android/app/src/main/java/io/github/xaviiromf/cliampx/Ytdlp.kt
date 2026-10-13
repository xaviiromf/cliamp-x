package io.github.xaviiromf.cliampx

import android.content.Context
import android.os.SystemClock
import androidx.core.content.edit
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.TimeoutCancellationException
import kotlinx.coroutines.async
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withTimeout
import java.io.File
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL

/**
 * Keeps yt-dlp new (UI-SPEC §7.8): the bundled copy stops working when
 * YouTube changes. It replaces the file that CLIAMP_YTDLP runs with the
 * newest GitHub release, checked against its SHA2-256SUMS, by an atomic
 * rename. The core runs yt-dlp fresh for each song, so the next play uses it.
 * youtubedl-android's own updater has no timeouts and deletes the old copy
 * first, so it is not used.
 */
object Ytdlp {
    sealed interface Status {
        data object Idle : Status
        data object Updating : Status
        data class Updated(val tag: String) : Status
        data class UpToDate(val tag: String) : Status
        data class Failed(val message: String) : Status
    }

    private const val TAG = "ytdlp_tag"
    private const val CHECKED = "ytdlp_checked"
    private const val MAX_SIZE = 64L shl 20
    private val SHA256 = Regex("[0-9a-f]{64}")
    private val TAG_NAME = Regex("[0-9A-Za-z._-]+")

    private val _status = MutableStateFlow<Status>(Status.Idle)
    val status: StateFlow<Status> = _status.asStateFlow()

    /** The elapsedRealtime of the last Updated or UpToDate, or 0. */
    @Volatile var finishedAt = 0L
        private set

    private val lock = Mutex()
    private var running: Deferred<Status>? = null

    /** The release tag in use, or null for the bundled copy. */
    fun tag(ctx: Context): String? = Updates.prefs(ctx).getString(TAG, null)

    fun checked(ctx: Context) = Updates.prefs(ctx).getLong(CHECKED, 0)

    /**
     * Updates yt-dlp when the daily check is due, or always with force. A call
     * while an update runs waits for it and returns its result.
     */
    suspend fun update(ctx: Context, force: Boolean): Status {
        val app = ctx.applicationContext
        val job = lock.withLock {
            running?.takeIf { it.isActive } ?: Remote.scope.async(Dispatchers.IO) { run(app, force) }.also { running = it }
        }
        return job.await()
    }

    private suspend fun run(ctx: Context, force: Boolean): Status {
        val prefs = Updates.prefs(ctx)
        val now = System.currentTimeMillis()
        if (!force && !Updates.due(prefs.getLong(CHECKED, 0), now)) return _status.value
        prefs.edit { putLong(CHECKED, now) }
        _status.value = Status.Updating
        val target = Core.ytdlp(ctx)
        val part = File(target.path + ".part")
        val result = try {
            withTimeout(120_000) { fetch(ctx, target, part) }
        } catch (e: TimeoutCancellationException) {
            Status.Failed(message(ctx, IOException("timeout", e)))
        } catch (e: CancellationException) {
            _status.value = Status.Idle
            throw e
        } catch (e: Exception) {
            Status.Failed(message(ctx, e))
        } finally {
            part.delete()
        }
        if (result is Status.Updated || result is Status.UpToDate) finishedAt = SystemClock.elapsedRealtime()
        _status.value = result
        return result
    }

    private suspend fun fetch(ctx: Context, target: File, part: File): Status {
        val prefs = Updates.prefs(ctx)
        val repo = if (prefs.getBoolean(Updates.NIGHTLY, false)) "yt-dlp/yt-dlp-nightly-builds" else "yt-dlp/yt-dlp"
        val latest = open("https://github.com/$repo/releases/latest").apply { instanceFollowRedirects = false }
        val tag = try {
            tagOf(latest.getHeaderField("Location")) ?: throw IOException("no release tag (HTTP ${latest.responseCode})")
        } finally {
            latest.disconnect()
        }
        if (tag == prefs.getString(TAG, null) && target.exists()) return Status.UpToDate(tag)
        val base = "https://github.com/$repo/releases/download/$tag/"
        val sums = open(base + "SHA2-256SUMS").let { c ->
            try {
                c.inputStream.bufferedReader().readText()
            } finally {
                c.disconnect()
            }
        }
        val hash = hashOf(sums) ?: throw IOException("no yt-dlp in SHA2-256SUMS")
        val c = open(base + "yt-dlp")
        try {
            c.inputStream.use { input ->
                part.outputStream().use { out ->
                    val buffer = ByteArray(64 shl 10)
                    var total = 0L
                    while (true) {
                        currentCoroutineContext().ensureActive()
                        val n = input.read(buffer)
                        if (n < 0) break
                        total += n
                        if (total > MAX_SIZE) throw IOException("yt-dlp is larger than ${MAX_SIZE shr 20} MB")
                        out.write(buffer, 0, n)
                    }
                }
            }
        } finally {
            c.disconnect()
        }
        val head = part.inputStream().use { input -> ByteArray(2).also { input.read(it) } }
        if (String(head) != "#!") throw IOException("the download is not yt-dlp")
        if (Updates.sha256(part) != hash) throw Updates.HashMismatch()
        // Atomic: a yt-dlp that runs now keeps reading the old file.
        if (!part.renameTo(target)) throw IOException("cannot write $target")
        prefs.edit { putString(TAG, tag) }
        return Status.Updated(tag)
    }

    /** Updates' words for no network, a bad hash or the rate limit; otherwise says what failed. */
    private fun message(ctx: Context, e: Exception) =
        if (Updates.offline(e) || e is Updates.HashMismatch || "rate limit" in e.message.orEmpty()) Updates.message(ctx, e)
        else ctx.getString(R.string.ytdlp_failed, e.message ?: e.toString())

    private fun open(url: String) = (URL(url).openConnection() as HttpURLConnection).apply {
        connectTimeout = 15_000
        readTimeout = 15_000
        useCaches = false
    }

    /** The tag of a releases/latest redirect: https://github.com/<repo>/releases/tag/<tag>. */
    fun tagOf(location: String?): String? =
        location?.substringAfter("/releases/tag/", "")?.takeIf { TAG_NAME.matches(it) }

    /** The SHA-256 of the yt-dlp zipapp in a SHA2-256SUMS file. */
    fun hashOf(sums: String): String? = sums.lineSequence()
        .map { it.trim() }
        .firstOrNull { it.endsWith(" yt-dlp") }
        ?.substringBefore(' ')?.lowercase()?.takeIf { SHA256.matches(it) }

    /** Whether a play request waits for the update: only the first one, while the bundled copy is the only one. */
    fun gate(tag: String?, status: Status) = tag == null && status is Status.Updating

    /** What the error watcher does about a failed YouTube play (§7.8). */
    enum class Watch {
        /** Wait for the running update; retry when it ends Updated or UpToDate. */
        WAIT,

        /** Retry now: an update finished less than a minute ago. */
        RETRY,

        /** Force an update; retry when it ends Updated or UpToDate. */
        UPDATE,

        /** Force an update; retry only when it ends Updated. */
        UPDATE_IF_NEW,
        NONE,
    }

    /**
     * The watcher's step. now, finishedAt and lastForced are elapsedRealtime
     * (lastForced 0: never); wallNow and checked are wall-clock millis.
     */
    fun watch(status: Status, now: Long, finishedAt: Long, tag: String?, lastForced: Long, kind: String, checked: Long, wallNow: Long) = when {
        status is Status.Updating -> Watch.WAIT
        finishedAt > 0 && now - finishedAt < 60_000 -> Watch.RETRY
        tag == null -> if (lastForced == 0L || now - lastForced >= 60_000) Watch.UPDATE else Watch.NONE
        kind == "ytdlp" && wallNow - checked > 3_600_000 -> Watch.UPDATE_IF_NEW
        else -> Watch.NONE
    }

    /** Whether the watcher looks at a snapshot: a YouTube play error of the track on screen. */
    fun watches(s: Snapshot): Boolean {
        val p = s.playError?.let { s.errorPath } ?: return false
        val host = p.substringAfter("://", "").substringBefore('/')
        return listOf("youtube.com", "youtu.be").any { host == it || host.endsWith(".$it") }
    }
}
