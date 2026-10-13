package io.github.xaviiromf.cliampx.ui

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.ImageDecoder
import android.graphics.Rect
import android.media.MediaMetadataRetriever
import android.net.http.HttpResponseCache
import android.util.LruCache
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.async
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withPermit
import java.io.File
import java.net.HttpURLConnection
import java.net.URL
import java.nio.ByteBuffer
import java.util.concurrent.ConcurrentHashMap
import kotlin.math.min
import kotlin.math.roundToInt

/**
 * Cover art at the size it is drawn (UI-SPEC §7.4, §12.1): the URL asks
 * YouTube for that size, the image is cropped to its central square and
 * decoded at the target size, never upscaled later. No image library.
 */
object Art {
    private val cache = object : LruCache<String, Bitmap>((Runtime.getRuntime().maxMemory() / 8).toInt()) {
        override fun sizeOf(key: String, value: Bitmap) = value.allocationByteCount
    }
    private val permits = Semaphore(4)
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    /** The fetches running now and how many callers wait on each: the same art drawn twice loads once. */
    private val inFlight = HashMap<String, Pair<Deferred<Bitmap?>, Int>>()

    /** The ytimg file that exists for each video ID: not every video has maxresdefault. */
    private val variants = ConcurrentHashMap<String, String>()

    /** The disk cache of the image responses. The APK and yt-dlp downloads opt out. */
    fun install(ctx: Context) {
        if (HttpResponseCache.getInstalled() == null) HttpResponseCache.install(File(ctx.cacheDir, "http"), 32L shl 20)
    }

    /** The art of key (a URL, or a local file with embedded art), px pixels square, or null. */
    suspend fun load(key: String, px: Int, hardware: Boolean = true): Bitmap? {
        if (key.isEmpty() || px <= 0) return null
        val cacheKey = "$px:$hardware:$key"
        cache.get(cacheKey)?.let { return it }
        val fetch = synchronized(inFlight) {
            val (running, waiters) = inFlight[cacheKey] ?: (scope.async { permits.withPermit { runCatching { fetch(key, px, hardware) }.getOrNull() } } to 0)
            inFlight[cacheKey] = running to waiters + 1
            running
        }
        try {
            return fetch.await()?.also { cache.put(cacheKey, it) }
        } finally {
            // The last caller to leave (a row scrolled away) stops a fetch nobody waits for.
            synchronized(inFlight) {
                val (running, waiters) = inFlight[cacheKey] ?: return@synchronized
                if (running !== fetch) return@synchronized
                if (waiters > 1) {
                    inFlight[cacheKey] = running to waiters - 1
                } else {
                    inFlight.remove(cacheKey)
                    running.cancel()
                }
            }
        }
    }

    /** A bitmap the MediaSession can parcel. */
    suspend fun software(key: String, px: Int) = load(key, px, hardware = false)

    private fun fetch(key: String, px: Int, hardware: Boolean): Bitmap? {
        if (key.startsWith("/")) {
            val bytes = MediaMetadataRetriever().run {
                try {
                    setDataSource(key)
                    embeddedPicture
                } finally {
                    release()
                }
            } ?: return null
            return decode(bytes, px, false, hardware)
        }
        for (url in artUrls(key, px, YTIMG.matchEntire(key)?.groupValues?.get(1)?.let(variants::get))) {
            val bytes = get(url) ?: continue
            val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
            BitmapFactory.decodeByteArray(bytes, 0, bytes.size, bounds)
            // YouTube answers a missing maxresdefault with a 120x90 grey frame.
            if (bounds.outWidth <= 120 && "i.ytimg.com" in url) continue
            val name = url.substringAfterLast('/').substringBefore('.')
            if (name != "mqdefault") YTIMG.matchEntire(url)?.let { variants[it.groupValues[1]] = name }
            val ytimg4x3 = "i.ytimg.com" in url && bounds.outWidth * 3 == bounds.outHeight * 4
            return decode(bytes, px, ytimg4x3, hardware)
        }
        return null
    }

    private fun get(url: String): ByteArray? {
        val c = URL(url).openConnection() as HttpURLConnection
        c.connectTimeout = 15_000
        c.readTimeout = 15_000
        return try {
            if (c.responseCode != 200) null else c.inputStream.use { it.readBytes() }
        } finally {
            c.disconnect()
        }
    }

    private fun decode(bytes: ByteArray, px: Int, letterboxed: Boolean, hardware: Boolean): Bitmap =
        ImageDecoder.decodeBitmap(ImageDecoder.createSource(ByteBuffer.wrap(bytes))) { decoder, info, _ ->
            val (w, h) = info.size.width to info.size.height
            val (left, top, side) = artSquare(w, h, letterboxed)
            // The square comes out exactly px wide, so the view never scales a small decode up.
            val scale = px.toFloat() / side
            val tw = (w * scale).roundToInt().coerceAtLeast(1)
            val th = (h * scale).roundToInt().coerceAtLeast(1)
            decoder.setTargetSize(tw, th)
            val l = (left * scale).roundToInt()
            val t = (top * scale).roundToInt()
            decoder.crop = Rect(l, t, min(l + px, tw), min(t + px, th))
            decoder.allocator = if (hardware) ImageDecoder.ALLOCATOR_HARDWARE else ImageDecoder.ALLOCATOR_SOFTWARE
        }
}

private val SIZE_SPEC = Regex("=(w\\d+-h\\d+|s\\d+)[^/=]*$")
private val YTIMG = Regex("^https?://i\\.ytimg\\.com/vi(?:_webp)?/([^/]+)/[^/?]+\\.(?:jpg|webp)(?:\\?.*)?$")

/**
 * The URLs to try, best first, for an image drawn px pixels wide. variant is
 * the ytimg file that worked before for this video, if any.
 */
fun artUrls(url: String, px: Int, variant: String? = null): List<String> {
    val host = url.substringAfter("://", "").substringBefore('/')
    if (host.endsWith("googleusercontent.com") || host.endsWith("ggpht.com")) {
        val p = px.coerceIn(1, 1200)
        val spec = "=w$p-h$p-l90-rj"
        return listOf(if (SIZE_SPEC.containsMatchIn(url)) url.replace(SIZE_SPEC, spec) else url + spec)
    }
    val id = YTIMG.matchEntire(url)?.groupValues?.get(1) ?: return listOf(url)
    fun file(name: String) = "https://i.ytimg.com/vi/$id/$name.jpg"
    if (px <= 320) return listOf(file("mqdefault"))
    if (variant != null) return listOf(file(variant))
    return listOf("maxresdefault", "sddefault", "hqdefault").map(::file)
}

/**
 * The central square of a w×h image as left, top, side. A letterboxed 4:3
 * ytimg frame first drops its bars: the square comes from its 16:9 band.
 */
fun artSquare(w: Int, h: Int, letterboxed: Boolean): IntArray {
    val side = min(w, if (letterboxed) w * 9 / 16 else h)
    return intArrayOf((w - side) / 2, (h - side) / 2, side)
}

/** The art of key at px pixels; null while it loads or when there is none. Leaving cancels the load. */
@Composable
fun rememberArt(key: String, px: Int): ImageBitmap? {
    val art by produceState<ImageBitmap?>(null, key, px) { value = Art.load(key, px)?.asImageBitmap() }
    return art
}
