package io.github.xaviiromf.cliampx

import android.annotation.SuppressLint
import android.app.PendingIntent
import android.content.Intent
import android.os.Build
import android.service.quicksettings.Tile
import android.service.quicksettings.TileService
import android.util.Log
import androidx.core.content.ContextCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.launch

/** The Quick Settings tile: play or pause, lit while a song plays; with no core it resumes. */
class PlayTile : TileService() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    private var watch: Job? = null

    override fun onStartListening() {
        watch = scope.launch {
            combine(Remote.state, Remote.connected) { s, up -> s.takeIf { up } }.collect { s ->
                val tile = qsTile ?: return@collect
                tile.state = if (s?.playing == true) Tile.STATE_ACTIVE else Tile.STATE_INACTIVE
                tile.subtitle = s?.track?.name.orEmpty()
                tile.updateTile()
            }
        }
    }

    override fun onStopListening() {
        watch?.cancel()
        watch = null
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }

    @SuppressLint("StartActivityAndCollapseDeprecated") // the Intent form is the only one below Android 14
    override fun onClick() {
        val toggle = Intent(this, CliampService::class.java).setAction(CliampService.ACTION_TOGGLE)
        try {
            ContextCompat.startForegroundService(this, toggle)
        } catch (e: IllegalStateException) {
            // Android refused a foreground start from the tile: open the app, which resumes.
            Log.w("cliamp-x", "tile: $e")
            val open = Intent(this, MainActivity::class.java).setAction(MainActivity.ACTION_RESUME).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            if (Build.VERSION.SDK_INT >= 34) {
                startActivityAndCollapse(PendingIntent.getActivity(this, 0, open, PendingIntent.FLAG_IMMUTABLE))
            } else {
                startActivityAndCollapse(open)
            }
        }
    }
}
