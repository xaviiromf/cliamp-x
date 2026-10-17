package io.github.xaviiromf.cliampx

import android.app.ActivityManager
import android.content.Context
import android.os.Build
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.core.content.edit
import io.github.xaviiromf.cliampx.ui.enumOr
import java.io.File

/** The choices of Ajustes → Modo ligero. */
enum class LiteMode(val label: Int) {
    Auto(R.string.lite_auto), On(R.string.lite_on), Off(R.string.lite_off),
}

/**
 * Modo ligero for slow phones. While on it overrides, without touching the
 * user's own choices (Looks keeps them and reads through its active* getters):
 * no visualizer behind the player and at most 10 fps elsewhere, a still
 * background, no animation (so no cover-color blend), covers decoded at half
 * size, and, in the core, a 1 s speaker buffer and a delayed next-track
 * prefetch (CLIAMP_LIGHT, player/device_rate.go). The core reads the variable
 * at start, so a change restarts it (Settings does).
 */
object Lite {
    var mode by mutableStateOf(LiteMode.Auto)
    private var weak by mutableStateOf(false)
    private var probed = false

    /** Whether the mode is in force now. */
    val on get() = isOn(mode, weak)

    /** Reads the saved mode and, once, probes the phone. */
    fun load(ctx: Context) {
        mode = enumOr(ctx.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(MODE, null), LiteMode.Auto)
        if (!probed) {
            weak = probe(ctx)
            probed = true
        }
    }

    /** Saves m; true when that changes whether the mode is in force, which needs a core restart. */
    fun set(ctx: Context, m: LiteMode): Boolean {
        val before = on
        mode = m
        ctx.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit { putString(MODE, m.name) }
        return on != before
    }

    /** The core's environment entry: CLIAMP_LIGHT=1 when the mode is on. */
    fun env(ctx: Context): List<String> {
        load(ctx)
        return if (on) listOf("CLIAMP_LIGHT=1") else emptyList()
    }

    fun isOn(mode: LiteMode, weak: Boolean) = when (mode) {
        LiteMode.On -> true
        LiteMode.Off -> false
        LiteMode.Auto -> weak
    }

    /**
     * The rule of Automático: a slow phone is one that Android flags as low
     * RAM, reports under 4 GiB of RAM (a phone sold with 4 GB reports about
     * 3.7 GB), has no 64-bit ABI (arm64-v8a or x86_64), or whose fastest core
     * tops out under 2 GHz (cpuinfo_max_freq in kHz; 0 for a core that cannot
     * be read, which is ignored). The Moto G20 (Unisoc T700, 1.8 GHz, 4 GB)
     * matches the RAM and the clock.
     */
    fun isWeak(lowRam: Boolean, totalMem: Long, abis: List<String>, maxFreqKhz: List<Long>): Boolean {
        val top = maxFreqKhz.maxOrNull() ?: 0L
        return lowRam || totalMem in 1 until FOUR_GIB || abis.none { it == "arm64-v8a" || it == "x86_64" } || top in 1 until 2_000_000L
    }

    private fun probe(ctx: Context): Boolean {
        val am = ctx.getSystemService(ActivityManager::class.java) ?: return false
        val mem = ActivityManager.MemoryInfo().also(am::getMemoryInfo)
        val freqs = File("/sys/devices/system/cpu").listFiles { f -> f.name.matches(Regex("cpu\\d+")) }.orEmpty()
            .map { runCatching { File(it, "cpufreq/cpuinfo_max_freq").readText().trim().toLong() }.getOrDefault(0L) }
        return isWeak(am.isLowRamDevice, mem.totalMem, Build.SUPPORTED_ABIS.toList(), freqs)
    }

    /** px for a cover drawn px wide: half of it when on, but never under what a small thumbnail needs. */
    fun artPx(px: Int, on: Boolean) = if (on) maxOf(px / 2, minOf(px, 96)) else px

    private const val FOUR_GIB = 4L shl 30
    private const val PREFS = "lite"
    private const val MODE = "mode"
}
