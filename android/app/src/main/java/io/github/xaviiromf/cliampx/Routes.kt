package io.github.xaviiromf.cliampx

import android.content.Context
import android.content.SharedPreferences
import android.media.AudioDeviceInfo
import kotlinx.coroutines.flow.MutableStateFlow

/**
 * The audio output the phone plays on (speaker, wired, USB or a Bluetooth
 * device), for the EQ preset remembered per output and for Reanudar al
 * conectar auriculares. CliampService watches the devices.
 */
object Routes {
    const val RESUME = "resume_on_connect"

    /** The output now, as a key: speaker, wired, usb or bt:<device name>. */
    val current = MutableStateFlow(SPEAKER)

    fun prefs(ctx: Context): SharedPreferences = ctx.getSharedPreferences("routes", Context.MODE_PRIVATE)

    /** Whether an output of type is headphones or a Bluetooth speaker: connecting one may resume playback. */
    fun headphones(type: Int) = kind(type) != null

    /** Whether removing outputs of these types while playing must pause: headphones, headsets and Bluetooth, not the speaker. */
    fun pauseOnRemoved(types: List<Int>, playing: Boolean) = playing && types.any {
        headphones(it) || it == AudioDeviceInfo.TYPE_BLUETOOTH_SCO || it == AudioDeviceInfo.TYPE_HEARING_AID
    }

    private fun kind(type: Int): String? = when (type) {
        AudioDeviceInfo.TYPE_BLUETOOTH_A2DP, AudioDeviceInfo.TYPE_BLE_HEADSET, AudioDeviceInfo.TYPE_BLE_SPEAKER -> "bt"
        AudioDeviceInfo.TYPE_USB_HEADSET, AudioDeviceInfo.TYPE_USB_DEVICE -> "usb"
        AudioDeviceInfo.TYPE_WIRED_HEADSET, AudioDeviceInfo.TYPE_WIRED_HEADPHONES -> "wired"
        else -> null
    }

    /**
     * The key of the output media plays on among the outputs (type to
     * product name): Android sends media to the last connected kind first,
     * Bluetooth before USB before wired before the speaker.
     */
    fun pick(outputs: List<Pair<Int, String>>): String {
        for (k in listOf("bt", "usb", "wired")) {
            val o = outputs.firstOrNull { kind(it.first) == k } ?: continue
            return if (k == "bt") "bt:" + o.second.trim() else k
        }
        return SPEAKER
    }

    /**
     * What the EQ does on a state: the preset to apply when the output
     * changed and one is saved for it, or whether to save the preset now
     * playing for the output. The automatic EQ by genre and Personalizado
     * are never saved: they are not a preset the user picked.
     */
    fun eqStep(route: String, synced: String?, preset: String, auto: Boolean, saved: String?): EqStep = when {
        route != synced -> EqStep(apply = saved?.takeIf { it != preset && !auto })
        auto || preset.isEmpty() || preset == "Custom" || preset == saved -> EqStep()
        else -> EqStep(save = true)
    }

    data class EqStep(val apply: String? = null, val save: Boolean = false)

    fun eqKey(route: String) = "eq:$route"

    const val SPEAKER = "speaker"
}

/** The YouTube Music link of a song for Compartir, or null for a local file or a stream. */
fun shareUrl(track: Track): String? =
    Regex("^https://(music|www)\\.youtube\\.com/watch\\?v=([A-Za-z0-9_-]{11})").find(track.path)
        ?.let { "https://music.youtube.com/watch?v=" + it.groupValues[2] }
