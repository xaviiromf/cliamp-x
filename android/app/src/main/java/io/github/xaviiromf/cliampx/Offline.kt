package io.github.xaviiromf.cliampx

import android.content.Context
import android.content.SharedPreferences
import androidx.core.content.edit

/**
 * The app's download rules (Ajustes → Red y descargas): downloads only on
 * Wi-Fi, and the automatic cache of liked and recent songs (download.auto).
 * Kept in the app: the core downloads whatever it is asked to.
 */
object Offline {
    const val WIFI_ONLY = "downloads_wifi_only"
    const val CACHE = "cache_on"
    const val CACHE_WIFI_ONLY = "cache_wifi_only"
    const val CACHE_SONGS = "cache_songs"
    const val CACHE_MB = "cache_mb"
    private const val CACHE_LAST = "cache_last"

    /** The song and MB limits Ajustes offers; 0 MB is no size cap. */
    val SONGS = listOf(25, 50, 100, 200)
    val MB = listOf(250, 500, 1000, 2000, 0)

    /** How often the cache fills itself at most. */
    const val INTERVAL_MS = 12 * 60 * 60 * 1000L

    fun prefs(ctx: Context): SharedPreferences = ctx.getSharedPreferences("offline", Context.MODE_PRIVATE)

    /** Whether a download may start on network (wifi, mobile, or null while unknown). */
    fun allowed(wifiOnly: Boolean, network: String?) = !wifiOnly || network == "wifi"

    /** Whether the automatic cache fills now: on, allowed on this network, and not filled in the last INTERVAL_MS (a clock set back counts as due). */
    fun cacheDue(on: Boolean, wifiOnly: Boolean, network: String?, last: Long, now: Long) =
        on && allowed(wifiOnly, network) && (now - last >= INTERVAL_MS || now < last)

    fun cacheParams(p: SharedPreferences): Pair<Int, Int> = p.getInt(CACHE_SONGS, 50) to p.getInt(CACHE_MB, 500)

    fun markFilled(p: SharedPreferences, now: Long) = p.edit { putLong(CACHE_LAST, now) }

    fun lastFilled(p: SharedPreferences) = p.getLong(CACHE_LAST, 0)

    /** The network kind the core plays for: an unmetered network counts as Wi-Fi, a metered one as mobile data. */
    fun networkKind(unmetered: Boolean) = if (unmetered) "wifi" else "mobile"
}
