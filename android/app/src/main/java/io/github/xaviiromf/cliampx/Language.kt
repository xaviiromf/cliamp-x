package io.github.xaviiromf.cliampx

import android.content.Context
import androidx.appcompat.app.AppCompatDelegate
import androidx.core.app.LocaleManagerCompat
import androidx.core.os.LocaleListCompat
import java.util.Locale

/**
 * The app language (Ajustes → Sistema → Idioma): the system's, or English or
 * Spanish. Android 13+ keeps the choice for the whole app; below, AppCompat
 * stores it and applies it to the activities.
 */
object Language {
    /** The choices in Ajustes; "" follows the system. */
    val TAGS = listOf("", "en", "es")

    /** The chosen tag, "" when the app follows the system. */
    fun current(ctx: Context): String = LocaleManagerCompat.getApplicationLocales(ctx)[0]?.language.orEmpty()

    /** The locale the app shows, for the core's LANG. */
    fun locale(ctx: Context): Locale = withRegion(LocaleManagerCompat.getApplicationLocales(ctx)[0], LocaleManagerCompat.getSystemLocales(ctx)[0] ?: Locale.getDefault())

    /** The app language in the system's country: picking Spanish must not move YouTube Music's region (gl) to the US. */
    fun withRegion(app: Locale?, system: Locale): Locale = when {
        app == null -> system
        app.country.isNotEmpty() -> app
        else -> Locale.Builder().setLanguage(app.language).setRegion(system.country).build()
    }

    /** Sets the app language and restarts the core, whose own texts and YouTube Music's follow LANG. */
    fun set(ctx: Context, tag: String) {
        if (tag == current(ctx)) return
        AppCompatDelegate.setApplicationLocales(LocaleListCompat.forLanguageTags(tag))
        CliampService.send(ctx, CliampService.ACTION_RESTART)
    }
}
