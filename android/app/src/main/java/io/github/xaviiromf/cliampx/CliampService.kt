@file:Suppress("DEPRECATION") // androidx.media points to Media3; PLAN.md picks it as less code

package io.github.xaviiromf.cliampx

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.BroadcastReceiver
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.ServiceInfo
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.media.AudioManager
import android.os.Binder
import android.os.Bundle
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.os.PowerManager
import android.support.v4.media.MediaMetadataCompat
import android.support.v4.media.session.MediaSessionCompat
import android.support.v4.media.session.PlaybackStateCompat
import android.system.Os
import android.system.OsConstants
import android.util.Log
import android.widget.Toast
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import androidx.core.content.ContextCompat
import androidx.media.AudioAttributesCompat
import androidx.media.AudioFocusRequestCompat
import androidx.media.AudioManagerCompat
import com.termux.terminal.TerminalSession
import com.termux.terminal.TerminalSessionClient
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

/**
 * Owns the Go core: its terminal session, the media session and notification
 * that stand in for MPRIS, audio focus and the IPC link. The activity only
 * binds and shows the session, so closing it never signals the core.
 */
class CliampService : Service(), TerminalSessionClient {
    inner class LocalBinder : Binder() {
        val service get() = this@CliampService
    }

    private val binder = LocalBinder()
    private val main = Handler(Looper.getMainLooper())
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    private lateinit var ipc: Ipc
    private lateinit var media: MediaSessionCompat
    private lateinit var audio: AudioManager
    private lateinit var wakeLock: PowerManager.WakeLock

    var session: TerminalSession? = null
        private set

    /** The activity showing the session, while it is bound. */
    var activity: MainActivity? = null

    private var preparing = false
    private var afterExit: (() -> Unit)? = null
    private var events: Job? = null

    private var playing = false
    private var hasFocus = false
    private var resumeOnFocus = false
    private var title = ""
    private var artist = ""
    private var artUrl = ""
    private var art: Bitmap? = null
    private var lastState = JSONObject()

    override fun onBind(intent: Intent): IBinder = binder

    override fun onCreate() {
        super.onCreate()
        ipc = Ipc(Core.socket(this))
        audio = getSystemService(AudioManager::class.java)
        wakeLock = getSystemService(PowerManager::class.java)
            .newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "cliampx:playback")
        media = MediaSessionCompat(this, "cliamp-x").apply {
            setCallback(mediaCallback)
            isActive = true
        }
        getSystemService(NotificationManager::class.java).createNotificationChannel(
            NotificationChannel(CHANNEL, getString(R.string.channel_playback), NotificationManager.IMPORTANCE_LOW)
        )
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_PREV -> op("prev")
            ACTION_TOGGLE -> op("toggle")
            ACTION_NEXT -> op("next")
            ACTION_QUIT -> quit()
            // The rest come from the app on screen, which may start the foreground service.
            else -> {
                ServiceCompat.startForeground(this, NOTIFICATION_ID, notification(), ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK)
                if (intent?.action == ACTION_RESTART) restart() else start()
                intent?.getStringExtra(EXTRA_LINK)?.let(::open)
            }
        }
        return START_NOT_STICKY
    }

    override fun onDestroy() {
        setPlaying(false)
        if (hasFocus) AudioManagerCompat.abandonAudioFocusRequest(audio, focusRequest)
        media.release()
        scope.cancel()
        super.onDestroy()
    }

    /** Starts the core unless it runs. The first start unpacks Python and ffmpeg first. */
    fun start() {
        if (session != null || preparing) return
        preparing = true
        scope.launch {
            withContext(Dispatchers.IO) {
                // Without yt-dlp the core still plays local files, so start it anyway.
                runCatching { Core.prepare(this@CliampService) }.onFailure { Log.e(TAG, "prepare", it) }
            }
            preparing = false
            val binary = Core.binary(this@CliampService).path
            val s = TerminalSession(binary, "$filesDir/home", arrayOf(binary), Core.env(this@CliampService), null, this@CliampService)
            session = s
            val shown = activity
            // The pty and the process start with the first size, which the view gives.
            if (shown != null) shown.attach(s) else s.updateSize(80, 24, 1, 1)
            events?.cancel()
            events = scope.launch(Dispatchers.IO) { followState() }
        }
    }

    /** Stops the core and starts it again. */
    fun restart() = stopCore { start() }

    /** Stops the core and the service. */
    fun quit() = stopCore { finish(true) }

    /** Stops the core with SIGTERM, so it saves the resume position, then runs then. */
    private fun stopCore(then: () -> Unit) {
        val s = session ?: return then()
        val stopping = afterExit != null
        afterExit = then
        // One SIGTERM only: the core takes a second one as "quit now" and skips the save.
        if (stopping) return
        if (s.pid > 0) runCatching { Os.kill(s.pid, OsConstants.SIGTERM) }
        main.postDelayed({ if (s.isRunning) s.finishIfRunning() }, 5000)
    }

    private fun finish(closeActivity: Boolean) {
        if (closeActivity) activity?.finishAndRemoveTask()
        setPlaying(false)
        ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    /** Runs `libcliamp.so open <link>` once the core listens, so it acts on this core. */
    private fun open(link: String) {
        scope.launch(Dispatchers.IO) {
            // The first start unpacks Python and ffmpeg before the core runs: allow a minute.
            val up = (1..600).any { runCatching { ipc.state() }.isSuccess || run { Thread.sleep(100); false } }
            val error = if (!up) "cliamp-x did not start" else runCatching {
                val process = Core.command(this@CliampService, "open", link).redirectErrorStream(true).start()
                val out = process.inputStream.bufferedReader().readText().trim()
                if (process.waitFor() == 0) null else out.lines().last()
            }.getOrElse { it.message }
            if (error != null) withContext(Dispatchers.Main) {
                Toast.makeText(this@CliampService, error, Toast.LENGTH_LONG).show()
            }
        }
    }

    /** ui.render: the core stops drawing while no one sees it. */
    fun render(on: Boolean) {
        scope.launch(Dispatchers.IO) {
            runCatching { ipc.submit("ui.render", JSONObject().put("name", if (on) "on" else "off")) }
                .onSuccess { if (!it.optBoolean("ok")) Log.i(TAG, "ui.render: ${it.opt("error")}") }
        }
    }

    private fun op(name: String, params: JSONObject = JSONObject()) {
        scope.launch(Dispatchers.IO) {
            runCatching { ipc.submit(name, params) }.onFailure { Log.w(TAG, "$name: $it") }
        }
    }

    /** Follows runtime.state, reconnecting while the core starts or restarts. */
    private suspend fun followState() {
        var wait = 200L
        while (currentCoroutineContext().isActive) {
            runCatching {
                ipc.subscribe(listOf("runtime.state")) { event ->
                    wait = 200L
                    event.optJSONObject("data")?.let { scope.launch { onState(it) } }
                }
            }
            delay(wait)
            wait = (wait * 2).coerceAtMost(2000L)
        }
    }

    private fun onState(snapshot: JSONObject) {
        lastState = snapshot
        val track = snapshot.optJSONObject("track") ?: JSONObject()
        title = track.optString("title").ifEmpty { track.optString("path").substringAfterLast('/') }
        artist = track.optString("artist")
        val url = track.optString("album_art_url")
        if (url != artUrl) {
            artUrl = url
            art = null
            if (url.startsWith("http")) scope.launch {
                val bitmap = withContext(Dispatchers.IO) { runCatching { fetchArt(url) }.getOrNull() }
                if (url == artUrl) {
                    art = bitmap
                    publish()
                }
            }
        }
        setPlaying(snapshot.optString("state") == "playing")
        publish()
    }

    private fun fetchArt(url: String): Bitmap? {
        val connection = URL(url).openConnection() as HttpURLConnection
        connection.connectTimeout = 10_000
        connection.readTimeout = 10_000
        return try {
            connection.inputStream.use { BitmapFactory.decodeStream(it) }
        } finally {
            connection.disconnect()
        }
    }

    /** Copies the last state to the media session and the notification. */
    private fun publish() {
        if (session == null) return // stopped: keep the notification gone
        val track = lastState.optJSONObject("track") ?: JSONObject()
        media.setMetadata(
            MediaMetadataCompat.Builder()
                .putString(MediaMetadataCompat.METADATA_KEY_TITLE, title)
                .putString(MediaMetadataCompat.METADATA_KEY_ARTIST, artist)
                .putString(MediaMetadataCompat.METADATA_KEY_ALBUM, track.optString("album"))
                .putLong(MediaMetadataCompat.METADATA_KEY_DURATION, (lastState.optDouble("duration", 0.0) * 1000).toLong())
                .putBitmap(MediaMetadataCompat.METADATA_KEY_ALBUM_ART, art)
                .build()
        )
        val state = when (lastState.optString("state")) {
            "playing" -> PlaybackStateCompat.STATE_PLAYING
            "paused" -> PlaybackStateCompat.STATE_PAUSED
            else -> PlaybackStateCompat.STATE_STOPPED
        }
        media.setPlaybackState(
            PlaybackStateCompat.Builder()
                .setActions(
                    PlaybackStateCompat.ACTION_PLAY or PlaybackStateCompat.ACTION_PAUSE or
                        PlaybackStateCompat.ACTION_PLAY_PAUSE or PlaybackStateCompat.ACTION_SKIP_TO_NEXT or
                        PlaybackStateCompat.ACTION_SKIP_TO_PREVIOUS or PlaybackStateCompat.ACTION_STOP or
                        (if (lastState.optBoolean("seekable")) PlaybackStateCompat.ACTION_SEEK_TO else 0L)
                )
                // Android 13+ builds the media controls from the session, so Quit is a custom action there.
                .addCustomAction(ACTION_QUIT, getString(R.string.quit), R.drawable.ic_close)
                .setState(state, (lastState.optDouble("position", 0.0) * 1000).toLong(), if (playing) 1f else 0f)
                .build()
        )
        getSystemService(NotificationManager::class.java).notify(NOTIFICATION_ID, notification())
    }

    private fun notification(): Notification {
        fun action(icon: Int, label: Int, action: String) = NotificationCompat.Action(
            icon, getString(label),
            PendingIntent.getService(this, action.hashCode(), Intent(this, CliampService::class.java).setAction(action), PendingIntent.FLAG_IMMUTABLE)
        )
        val open = PendingIntent.getActivity(this, 0, Intent(this, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE)
        return NotificationCompat.Builder(this, CHANNEL)
            .setSmallIcon(R.drawable.ic_glyph)
            .setContentTitle(title.ifEmpty { getString(R.string.app_name) })
            .setContentText(artist)
            .setLargeIcon(art)
            .setContentIntent(open)
            .setVisibility(NotificationCompat.VISIBILITY_PUBLIC)
            .setSilent(true)
            .addAction(action(R.drawable.ic_prev, R.string.prev, ACTION_PREV))
            .addAction(action(if (playing) R.drawable.ic_pause else R.drawable.ic_play, R.string.play_pause, ACTION_TOGGLE))
            .addAction(action(R.drawable.ic_next, R.string.next, ACTION_NEXT))
            .addAction(action(R.drawable.ic_close, R.string.quit, ACTION_QUIT))
            .setStyle(androidx.media.app.NotificationCompat.MediaStyle().setMediaSession(media.sessionToken).setShowActionsInCompactView(0, 1, 2))
            .build()
    }

    // Audio focus: a loss pauses, a transient loss pauses and resumes. Ducking is left to the system.
    private val focusListener = AudioManager.OnAudioFocusChangeListener { change ->
        when (change) {
            AudioManager.AUDIOFOCUS_LOSS -> {
                hasFocus = false
                resumeOnFocus = false
                if (playing) op("pause")
            }
            AudioManager.AUDIOFOCUS_LOSS_TRANSIENT -> if (playing) {
                resumeOnFocus = true
                op("pause")
            }
            AudioManager.AUDIOFOCUS_GAIN -> if (resumeOnFocus) {
                resumeOnFocus = false
                op("play")
            }
        }
    }

    private val focusRequest = AudioFocusRequestCompat.Builder(AudioManagerCompat.AUDIOFOCUS_GAIN)
        .setAudioAttributes(AudioAttributesCompat.Builder().setUsage(AudioAttributesCompat.USAGE_MEDIA).setContentType(AudioAttributesCompat.CONTENT_TYPE_MUSIC).build())
        .setOnAudioFocusChangeListener(focusListener)
        .build()

    private val noisy = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) = op("pause")
    }

    private fun setPlaying(now: Boolean) {
        if (now == playing) return
        playing = now
        if (now) {
            if (!hasFocus) {
                hasFocus = AudioManagerCompat.requestAudioFocus(audio, focusRequest) == AudioManager.AUDIOFOCUS_REQUEST_GRANTED
                if (!hasFocus) op("pause")
            }
            ContextCompat.registerReceiver(this, noisy, IntentFilter(AudioManager.ACTION_AUDIO_BECOMING_NOISY), ContextCompat.RECEIVER_NOT_EXPORTED)
            wakeLock.acquire(6 * 60 * 60 * 1000L)
        } else {
            unregisterReceiver(noisy)
            if (wakeLock.isHeld) wakeLock.release()
        }
    }

    private val mediaCallback = object : MediaSessionCompat.Callback() {
        override fun onPlay() = op("play")
        override fun onPause() = op("pause")
        override fun onStop() = op("stop")
        override fun onSkipToNext() = op("next")
        override fun onSkipToPrevious() = op("prev")
        override fun onSeekTo(pos: Long) = op("seek.absolute", JSONObject().put("value", pos / 1000.0))
        override fun onCustomAction(action: String, extras: Bundle?) {
            if (action == ACTION_QUIT) quit()
        }
    }

    // TerminalSessionClient: the session reports to the view of the bound activity.
    override fun onTextChanged(changedSession: TerminalSession) {
        activity?.onScreenUpdated(changedSession)
    }

    override fun onSessionFinished(finishedSession: TerminalSession) {
        if (finishedSession !== session) return
        session = null
        events?.cancel()
        val next = afterExit
        afterExit = null
        // The core quit by itself: close the app, but keep a failure on screen.
        next?.invoke() ?: finish(finishedSession.exitStatus == 0)
    }

    override fun onCopyTextToClipboard(session: TerminalSession, text: String) {
        getSystemService(ClipboardManager::class.java).setPrimaryClip(ClipData.newPlainText("cliamp-x", text))
    }

    override fun onPasteTextFromClipboard(session: TerminalSession?) {
        val text = getSystemService(ClipboardManager::class.java).primaryClip?.getItemAt(0)?.coerceToText(this)
        if (!text.isNullOrEmpty()) session?.emulator?.paste(text.toString())
    }

    override fun onColorsChanged(session: TerminalSession) = onTextChanged(session)
    override fun onTitleChanged(changedSession: TerminalSession) {}
    override fun onBell(session: TerminalSession) {}
    override fun onTerminalCursorStateChange(state: Boolean) {}
    override fun getTerminalCursorStyle(): Int? = null
    override fun logError(tag: String?, message: String?) { Log.e(tag, message.orEmpty()) }
    override fun logWarn(tag: String?, message: String?) { Log.w(tag, message.orEmpty()) }
    override fun logInfo(tag: String?, message: String?) { Log.i(tag, message.orEmpty()) }
    override fun logDebug(tag: String?, message: String?) { Log.d(tag, message.orEmpty()) }
    override fun logVerbose(tag: String?, message: String?) { Log.v(tag, message.orEmpty()) }
    override fun logStackTraceWithMessage(tag: String?, message: String?, e: Exception?) { Log.e(tag, message, e) }
    override fun logStackTrace(tag: String?, e: Exception?) { Log.e(tag, "", e) }

    companion object {
        private const val TAG = "cliamp-x"
        private const val CHANNEL = "playback"
        private const val NOTIFICATION_ID = 1
        const val EXTRA_LINK = "link"
        const val ACTION_PREV = "io.github.xaviiromf.cliampx.PREV"
        const val ACTION_TOGGLE = "io.github.xaviiromf.cliampx.TOGGLE"
        const val ACTION_NEXT = "io.github.xaviiromf.cliampx.NEXT"
        const val ACTION_QUIT = "io.github.xaviiromf.cliampx.QUIT"
        const val ACTION_RESTART = "io.github.xaviiromf.cliampx.RESTART"

        fun send(context: Context, action: String) {
            context.startService(Intent(context, CliampService::class.java).setAction(action))
        }
    }
}
