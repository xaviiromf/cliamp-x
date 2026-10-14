package io.github.xaviiromf.cliampx

import android.content.ComponentName
import android.content.Context
import android.content.pm.PackageManager
import androidx.annotation.StringRes

/** The launcher icons (Ajustes → Icono de la app): each is an activity-alias of MainActivity in the manifest, one enabled at a time. */
enum class LauncherIcon(@StringRes val label: Int, val alias: String) {
    Default(R.string.icon_default, "Launcher"),
    Green(R.string.icon_green, "LauncherGreen"),
    Amber(R.string.icon_amber, "LauncherAmber"),
    Synthwave(R.string.icon_synthwave, "LauncherSynthwave"),
    GameBoy(R.string.icon_gameboy, "LauncherGameBoy"),
    ;

    fun component(context: Context) = ComponentName(context.packageName, "io.github.xaviiromf.cliampx.$alias")

    companion object {
        /** The enabled icon; the manifest enables Default only. */
        fun current(context: Context): LauncherIcon = entries.firstOrNull { enabled(context.packageManager.getComponentEnabledSetting(it.component(context)), it) } ?: Default

        /** Whether icon shows for a component state; DEFAULT is the manifest's choice. */
        fun enabled(state: Int, icon: LauncherIcon) = when (state) {
            PackageManager.COMPONENT_ENABLED_STATE_ENABLED -> true
            PackageManager.COMPONENT_ENABLED_STATE_DEFAULT -> icon == Default
            else -> false
        }

        /** Shows icon in the launcher. The new alias is enabled before the others go, so the app never leaves the launcher. */
        fun set(context: Context, icon: LauncherIcon) {
            val pm = context.packageManager
            fun state(e: LauncherIcon, on: Boolean) = pm.setComponentEnabledSetting(
                e.component(context),
                if (on) PackageManager.COMPONENT_ENABLED_STATE_ENABLED else PackageManager.COMPONENT_ENABLED_STATE_DISABLED,
                PackageManager.DONT_KILL_APP,
            )
            state(icon, true)
            entries.filter { it != icon }.forEach { state(it, false) }
        }
    }
}
