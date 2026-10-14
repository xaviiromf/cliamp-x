package io.github.xaviiromf.cliampx

import android.app.PendingIntent
import android.appwidget.AppWidgetManager
import android.appwidget.AppWidgetProvider
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.widget.RemoteViews
import androidx.compose.ui.graphics.toArgb
import androidx.core.graphics.scale
import io.github.xaviiromf.cliampx.ui.paletteOf

/**
 * The home-screen mini player: the cover, the song and its artist, and
 * previous, play or pause and next, in the core theme's colors. The service
 * redraws it on each state it publishes; with no core, play resumes.
 */
class PlayerWidget : AppWidgetProvider() {
    override fun onUpdate(context: Context, manager: AppWidgetManager, ids: IntArray) {
        shown = null
        update(context, Remote.state.value?.takeIf { Remote.connected.value }, null)
    }

    companion object {
        /** What the widgets show now, so a state that changes only the position sends nothing. */
        private var shown: List<Any?>? = null
        private var cover: Pair<Bitmap, Bitmap>? = null

        fun update(context: Context, s: Snapshot?, art: Bitmap?) {
            val manager = AppWidgetManager.getInstance(context)
            val ids = manager.getAppWidgetIds(ComponentName(context, PlayerWidget::class.java))
            if (ids.isEmpty()) return
            val key = listOf(s?.track?.path, s?.track?.name, s?.track?.artist, s?.playing, s?.theme, art)
            if (key == shown) return
            shown = key
            manager.updateAppWidget(ids, views(context, s, art?.let(::small)))
        }

        // A widget's bitmaps go through binder: 160 px is sharp at 48 dp and small.
        private fun small(art: Bitmap): Bitmap {
            cover?.let { (from, to) -> if (from === art) return to }
            val side = 160
            return art.scale(side, side).also { cover = art to it }
        }

        private fun views(context: Context, s: Snapshot?, art: Bitmap?): RemoteViews {
            val p = paletteOf(s?.theme)
            fun service(action: String) = PendingIntent.getForegroundService(
                context, action.hashCode(), Intent(context, CliampService::class.java).setAction(action), PendingIntent.FLAG_IMMUTABLE,
            )
            val open = PendingIntent.getActivity(
                context, 1, Intent(context, MainActivity::class.java).putExtra(MainActivity.EXTRA_OPEN_PLAYER, true),
                PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
            )
            return RemoteViews(context.packageName, R.layout.widget_player).apply {
                setInt(R.id.widget_root, "setBackgroundColor", p.bg.toArgb())
                setTextViewText(R.id.widget_title, s?.track?.name?.ifEmpty { null } ?: context.getString(R.string.app_name))
                setTextColor(R.id.widget_title, (if (s?.playing == true) p.accent else p.brightFg).toArgb())
                setTextViewText(R.id.widget_artist, if (s?.track == null) context.getString(R.string.widget_idle) else s.track.artist)
                setTextColor(R.id.widget_artist, p.fg.toArgb())
                if (art != null) {
                    setImageViewBitmap(R.id.widget_art, art)
                    setInt(R.id.widget_art, "setColorFilter", 0)
                } else {
                    setImageViewResource(R.id.widget_art, R.drawable.ic_note)
                    setInt(R.id.widget_art, "setColorFilter", p.fg.toArgb())
                }
                setImageViewResource(R.id.widget_toggle, if (s?.playing == true) R.drawable.ic_pause else R.drawable.ic_play)
                for (id in listOf(R.id.widget_prev, R.id.widget_toggle, R.id.widget_next)) setInt(id, "setColorFilter", p.accent.toArgb())
                setOnClickPendingIntent(R.id.widget_prev, service(CliampService.ACTION_PREV))
                setOnClickPendingIntent(R.id.widget_toggle, service(CliampService.ACTION_TOGGLE))
                setOnClickPendingIntent(R.id.widget_next, service(CliampService.ACTION_NEXT))
                setOnClickPendingIntent(R.id.widget_body, open)
                setOnClickPendingIntent(R.id.widget_art, open)
            }
        }
    }
}
