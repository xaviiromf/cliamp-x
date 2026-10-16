@file:Suppress("DEPRECATION") // androidx.media points to Media3; PLAN.md picks it as less code

package io.github.xaviiromf.cliampx

import android.app.ActivityManager
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
import android.media.AudioDeviceCallback
import android.media.AudioDeviceInfo
import android.media.AudioManager
import android.os.Binder
import android.os.Bundle
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.os.PowerManager
import android.os.SystemClock
import android.support.v4.media.MediaBrowserCompat
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
import androidx.core.content.edit
import androidx.core.graphics.createBitmap
import androidx.media.AudioAttributesCompat
import androidx.media.AudioFocusRequestCompat
import androidx.media.AudioManagerCompat
import androidx.media.MediaBrowserServiceCompat
import androidx.compose.ui.graphics.toArgb
import io.github.xaviiromf.cliampx.ui.Art
import io.github.xaviiromf.cliampx.ui.paletteOf
import com.termux.terminal.TerminalSession
import com.termux.terminal.TerminalSessionClient
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filterNotNull
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import org.json.JSONObject
import kotlin.math.abs

/**
 * Owns the Go core: its terminal session, the media session and notification
 * that stand in for MPRIS, audio focus, and Remote, the IPC link of the native
 * UI. Modo terminal only binds and shows the session, so closing it never
 * signals the core. AutoService lends its session and browse tree (Auto) to
 * Android Auto, and it follows the audio output for the EQ per output and the
 * resume on connect. Not exported: its start commands quit, restart and open links.
 */
class CliampService : Service(), TerminalSessionClient {
    inner class LocalBinder : Binder() {
        val service get() = this@CliampService
    }

    private val binder = LocalBinder()
    private val main = Handler(Looper.getMainLooper())
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    private val ipc by lazy { Ipc(Core.socket(this)) }
    private lateinit var media: MediaSessionCompat
    private lateinit var audio: AudioManager
    private lateinit var wakeLock: PowerManager.WakeLock

    var session: TerminalSession? = null
        private set

    /** Modo terminal, while it is bound. */
    var terminal: TerminalActivity? = null

    private var preparing = false
    private var afterExit: (() -> Unit)? = null

    private var playing = false
    private var hasFocus = false
    private var resumeOnFocus = false
    private var artUrl = ""
    // A cover that fails to load leaves art null; the theme art then stands in for it.
    private var artLoading = false
    private var art: Bitmap? = null
    private var snap: Snapshot? = null

    // A seek from the notification or the lock screen: the session shows the target until the core reports it (§5.4).
    private var seekTarget = 0.0
    private var seekPath: String? = null
    private var seekAt = 0L

    // Modo terminal binds the terminal session, AutoService the media session.
    override fun onBind(intent: Intent): IBinder = binder

    val sessionToken: MediaSessionCompat.Token get() = media.sessionToken

    override fun onCreate() {
        super.onCreate()
        Art.install(this)
        Remote.start(this)
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
        // A new notification and art only when something the system shows changed, not on every state.get.
        scope.launch {
            Remote.state.filterNotNull().distinctUntilChanged { a, b ->
                val now = SystemClock.elapsedRealtime()
                a.track?.path == b.track?.path && a.track?.title == b.track?.title && a.state == b.state &&
                    a.buffering == b.buffering && a.seeking == b.seeking && a.speed == b.speed && a.duration == b.duration && a.seekable == b.seekable &&
                    a.theme == b.theme &&
                    abs(PlaybackClock.positionAt(a, now) - PlaybackClock.positionAt(b, now)) < 2
            }.collect(::onState)
        }
        watchOutputs()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        // The widget, the tile and the shortcuts start the service in the foreground: it must show its notification.
        if (intent?.action != ACTION_QUIT && intent?.action != ACTION_ERASE) ServiceCompat.startForeground(this, NOTIFICATION_ID, notification(), ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK)
        val control = intent?.action in setOf(ACTION_PREV, ACTION_TOGGLE, ACTION_NEXT, ACTION_RESUME)
        // With no core, a control from the widget, the tile or a shortcut resumes: start the core and play what it restored.
        if (control && session == null) return START_NOT_STICKY.also { resume() }
        when (intent?.action) {
            ACTION_PREV -> op("prev")
            ACTION_TOGGLE -> op("toggle")
            ACTION_RESUME -> resume()
            ACTION_NEXT -> op("next")
            ACTION_QUIT -> quit()
            ACTION_ERASE -> erase()
            // The rest come from the app on screen, which may start the foreground service.
            else -> {
                if (intent?.action == ACTION_RESTART) restart() else start()
                intent?.getStringExtra(EXTRA_LINK)?.let(::open)
            }
        }
        return START_NOT_STICKY
    }

    override fun onDestroy() {
        audio.unregisterAudioDeviceCallback(outputs)
        setPlaying(false)
        if (hasFocus) AudioManagerCompat.abandonAudioFocusRequest(audio, focusRequest)
        media.release()
        scope.cancel()
        Remote.stop()
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
            val shown = terminal
            // The pty and the process start with the first size, which the view gives.
            if (shown != null) shown.attach(s) else s.updateSize(80, 24, 1, 1)
            // A state that came while no core ran was not published, and the new core's same state is no change: show it now.
            publish()
            // The bundled yt-dlp is too old for YouTube: replace it before the first play (§7.8).
            val app = applicationContext
            Remote.scope.launch { Ytdlp.update(app, force = Ytdlp.tag(app) == null) }
        }
    }

    /** Starts the core for a caller that is not on screen (Android Auto), in the foreground when Android allows it. */
    private fun ensureCore() {
        if (session != null || preparing) return
        try {
            ContextCompat.startForegroundService(this, Intent(this, CliampService::class.java))
        } catch (e: IllegalStateException) {
            Log.w(TAG, "foreground start: $e") // ForegroundServiceStartNotAllowedException: run the core while bound
            start()
        }
    }

    /** Whether the core answers with a state within 90 s (the first start unpacks Python and ffmpeg). */
    private suspend fun coreUp() = withTimeoutOrNull(90_000) {
        Remote.connected.first { it }
        Remote.state.first { it != null }
    } != null

    /** Starts the core unless it runs, then plays the queue it restored (Reanudar, the widget, the tile). */
    private fun resume() {
        ensureCore()
        scope.launch {
            if (coreUp() && Remote.state.value?.track != null && Remote.state.value?.playing != true) op("play")
        }
    }

    /** Runs block on the core for Android Auto once it is up; a failed call leaves the car's list empty. */
    private fun <T> withCore(fallback: T, done: (T) -> Unit, block: suspend () -> T) {
        ensureCore()
        scope.launch {
            val value = try {
                if (coreUp()) block() else fallback
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                Log.w(TAG, "auto: $e")
                fallback
            }
            done(value)
        }
    }

    /** AutoService's browse tree. */
    fun browse(parentId: String, result: MediaBrowserServiceCompat.Result<MutableList<MediaBrowserCompat.MediaItem>>) {
        result.detach()
        if (parentId == Auto.ROOT) return result.sendResult(Auto.root(this).toMutableList()) // no core needed
        withCore(emptyList(), { result.sendResult(it.toMutableList()) }) { Auto.children(this, parentId) }
    }

    /** AutoService's voice search. */
    fun search(query: String, result: MediaBrowserServiceCompat.Result<MutableList<MediaBrowserCompat.MediaItem>>) {
        result.detach()
        withCore(emptyList(), { result.sendResult(it.toMutableList()) }) { Auto.search(query) }
    }

    /** Stops the core and starts it again. */
    fun restart() = stopCore { start() }

    /** Stops the core and the service. */
    fun quit() {
        Remote.quitting.value = true
        stopCore { finish(true) }
    }

    /** Stops the core, then wipes the app's data: sign-in, settings, lists, history and downloads. */
    fun erase() {
        Remote.quitting.value = true
        stopCore { getSystemService(ActivityManager::class.java).clearApplicationUserData() }
    }

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
        if (closeActivity) terminal?.finishAndRemoveTask()
        setPlaying(false)
        PlayerWidget.update(this, null, null)
        ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    /** Runs `libcliamp.so open <link>` once the core listens, so it acts on this core. */
    private fun open(link: String) {
        scope.launch(Dispatchers.IO) {
            // The first start unpacks Python and ffmpeg before the core runs: allow a minute.
            val up = (1..600).any { runCatching { ipc.state() }.isSuccess || run { Thread.sleep(100); false } }
            val error = if (!up) getString(R.string.core_not_started) else runCatching {
                val process = Core.command(this@CliampService, "open", link).redirectErrorStream(true).start()
                val out = process.inputStream.bufferedReader().readText().trim()
                if (process.waitFor() == 0) null else out.lines().last()
            }.getOrElse { it.message }
            if (error != null) withContext(Dispatchers.Main) {
                Toast.makeText(this@CliampService, error, Toast.LENGTH_LONG).show()
            }
        }
    }

    private fun op(name: String, params: JSONObject = JSONObject()) = Remote.fire(name, params)

    private fun onState(s: Snapshot) {
        snap = s
        val url = s.track?.art.orEmpty()
        if (url != artUrl) {
            artUrl = url
            art = null
            artLoading = url.isNotEmpty()
            if (artLoading) scope.launch {
                val bitmap = Art.software(url, 512)
                if (url == artUrl) {
                    art = bitmap
                    artLoading = false
                    publish()
                }
            }
        }
        setPlaying(s.playing)
        publish()
    }

    /** Copies the last state to the media session and the notification. */
    private fun publish() {
        if (session == null) return // stopped: keep the notification gone
        val s = snap ?: return
        val now = SystemClock.elapsedRealtime()
        // A YouTube seek reopens the stream for about 12 s, and the snapshots meanwhile carry the old position.
        val held = seekPath != null && seekHolds(seekTarget, seekPath, seekAt, s, now, SEEK_HOLD_MS)
        media.setMetadata(
            MediaMetadataCompat.Builder()
                .putString(MediaMetadataCompat.METADATA_KEY_TITLE, s.track?.name.orEmpty())
                .putString(MediaMetadataCompat.METADATA_KEY_ARTIST, s.track?.artist.orEmpty())
                .putString(MediaMetadataCompat.METADATA_KEY_ALBUM, s.track?.album.orEmpty())
                .putLong(MediaMetadataCompat.METADATA_KEY_DURATION, (s.duration * 1000).toLong())
                .putBitmap(MediaMetadataCompat.METADATA_KEY_ALBUM_ART, shownArt())
                .build()
        )
        val state = when {
            // A seek stands still until it lands (G7 reports its target meanwhile): the system must not move it on.
            s.playing && (s.buffering || s.seeking) -> PlaybackStateCompat.STATE_BUFFERING
            s.playing -> PlaybackStateCompat.STATE_PLAYING
            s.state == "paused" -> PlaybackStateCompat.STATE_PAUSED
            else -> PlaybackStateCompat.STATE_STOPPED
        }
        media.setPlaybackState(
            PlaybackStateCompat.Builder()
                .setActions(
                    PlaybackStateCompat.ACTION_PLAY or PlaybackStateCompat.ACTION_PAUSE or
                        PlaybackStateCompat.ACTION_PLAY_PAUSE or PlaybackStateCompat.ACTION_SKIP_TO_NEXT or
                        PlaybackStateCompat.ACTION_SKIP_TO_PREVIOUS or PlaybackStateCompat.ACTION_STOP or
                        PlaybackStateCompat.ACTION_PLAY_FROM_MEDIA_ID or PlaybackStateCompat.ACTION_PLAY_FROM_SEARCH or
                        (if (s.seekable) PlaybackStateCompat.ACTION_SEEK_TO else 0L)
                )
                // Android 13+ builds the media controls from the session, so Quit is a custom action there.
                .addCustomAction(ACTION_QUIT, getString(R.string.quit), R.drawable.ic_close)
                // The system moves the position on from receivedAt at the core's speed; a held seek stands still.
                .setState(
                    state, ((if (held) seekTarget else s.position) * 1000).toLong(),
                    if (held) 0f else s.speed.toFloat(), if (held) now else s.receivedAt,
                )
                .build()
        )
        getSystemService(NotificationManager::class.java).notify(NOTIFICATION_ID, notification())
        PlayerWidget.update(this, s, art)
    }

    private fun notification(): Notification {
        fun action(icon: Int, label: Int, action: String) = NotificationCompat.Action(
            icon, getString(label),
            PendingIntent.getService(this, action.hashCode(), Intent(this, CliampService::class.java).setAction(action), PendingIntent.FLAG_IMMUTABLE)
        )
        val open = PendingIntent.getActivity(
            this, 0, Intent(this, MainActivity::class.java).putExtra(MainActivity.EXTRA_OPEN_PLAYER, true),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        return NotificationCompat.Builder(this, CHANNEL)
            .setSmallIcon(R.drawable.ic_glyph)
            .setContentTitle(snap?.track?.name.orEmpty().ifEmpty { getString(R.string.app_name) })
            .setContentText(snap?.track?.artist.orEmpty())
            .setLargeIcon(shownArt())
            // The theme's colors: the accent tints the notification, and the art carries them to the media controls.
            .setColor(themeColors().second)
            .setColorized(true)
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

    /** The theme's bg and accent as ARGB. */
    private fun themeColors(): Pair<Int, Int> = paletteOf(snap?.theme).let { it.bg.toArgb() to it.accent.toArgb() }

    private var themeArt: Triple<Bitmap?, Pair<Int, Int>, Bitmap>? = null

    /**
     * The notification and lock screen art: the cover as a duotone from the theme's bg to its accent, or for a
     * song with none, the logo in the accent on the bg. The system takes the media controls' colors from it.
     */
    private fun shownArt(): Bitmap? {
        val cover = art
        if (snap?.track == null || artLoading) return null // nothing, or the cover is loading
        val colors = themeColors()
        themeArt?.takeIf { it.first === cover && it.second == colors }?.let { return it.third }
        val bitmap = createBitmap(THEME_ART_PX, THEME_ART_PX)
        val canvas = android.graphics.Canvas(bitmap)
        canvas.drawColor(colors.first)
        if (cover != null) {
            val side = minOf(cover.width, cover.height)
            val x = (cover.width - side) / 2
            val y = (cover.height - side) / 2
            val paint = android.graphics.Paint(android.graphics.Paint.FILTER_BITMAP_FLAG)
            paint.colorFilter = android.graphics.ColorMatrixColorFilter(duotone(colors.first, colors.second))
            canvas.drawBitmap(cover, android.graphics.Rect(x, y, x + side, y + side), android.graphics.Rect(0, 0, THEME_ART_PX, THEME_ART_PX), paint)
        } else ContextCompat.getDrawable(this, R.drawable.ic_glyph)?.mutate()?.apply {
            setTint(colors.second)
            setBounds(THEME_ART_PX / 4, THEME_ART_PX / 4, THEME_ART_PX * 3 / 4, THEME_ART_PX * 3 / 4)
            draw(canvas)
        }
        themeArt = Triple(cover, colors, bitmap)
        return bitmap
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

    private val routePrefs by lazy { Routes.prefs(this) }
    private var outputsListed = false

    // Plugging headphones or connecting a Bluetooth device may resume (opt-in); every change may switch the EQ.
    private val outputs = object : AudioDeviceCallback() {
        override fun onAudioDevicesAdded(added: Array<out AudioDeviceInfo>) {
            followOutput()
            // The first call lists the outputs that were already there.
            if (!outputsListed) {
                outputsListed = true
                return
            }
            if (routePrefs.getBoolean(Routes.RESUME, false) && session != null && Remote.state.value?.state == "paused" &&
                added.any { it.isSink && Routes.headphones(it.type) }
            ) op("play")
        }

        override fun onAudioDevicesRemoved(removed: Array<out AudioDeviceInfo>) = followOutput()
    }

    private fun followOutput() {
        Routes.current.value = Routes.pick(audio.getDevices(AudioManager.GET_DEVICES_OUTPUTS).map { it.type to it.productName.toString() })
    }

    /** Follows the output: the EQ preset the user picks is kept for it, and another output brings back its own. */
    private fun watchOutputs() {
        audio.registerAudioDeviceCallback(outputs, main)
        var synced: String? = null
        scope.launch {
            combine(Routes.current, Remote.state.filterNotNull()) { route, s -> Triple(route, s.eqPreset, s.eqAuto) }
                .distinctUntilChanged().collect { (route, preset, auto) ->
                    val key = Routes.eqKey(route)
                    val step = Routes.eqStep(route, synced, preset, auto, routePrefs.getString(key, null))
                    synced = route
                    step.apply?.let { op("eq", JSONObject().put("name", it)) }
                    if (step.save) routePrefs.edit { putString(key, preset) }
                }
        }
    }

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
        // The system's media controls and Android Auto may play after the core ended.
        override fun onPlay() = if (session == null) resume() else op("play")
        override fun onPlayFromMediaId(mediaId: String, extras: Bundle?) =
            withCore(false, { if (!it) Log.w(TAG, "auto: no row $mediaId") }) { Auto.play(this@CliampService, mediaId) }
        // "Ok Google, pon …": an empty query plays what was playing.
        override fun onPlayFromSearch(query: String?, extras: Bundle?) {
            if (query.isNullOrBlank()) return resume()
            withCore(false, {}) { Auto.playSearch(query) }
        }
        override fun onPause() = op("pause")
        override fun onStop() = op("stop")
        override fun onSkipToNext() = op("next")
        override fun onSkipToPrevious() = op("prev")
        override fun onSeekTo(pos: Long) {
            seekTarget = pos / 1000.0
            seekPath = snap?.track?.path
            seekAt = SystemClock.elapsedRealtime()
            publish()
            // Nothing may arrive while the stream reopens: show the core's position again when the hold ends.
            main.postDelayed(::publish, SEEK_HOLD_MS + 100)
            op("seek.absolute", JSONObject().put("value", seekTarget))
        }
        override fun onCustomAction(action: String, extras: Bundle?) {
            if (action == ACTION_QUIT) quit()
        }
    }

    // TerminalSessionClient: the session reports to the view of the bound Modo terminal.
    override fun onTextChanged(changedSession: TerminalSession) {
        terminal?.onScreenUpdated(changedSession)
    }

    override fun onSessionFinished(finishedSession: TerminalSession) {
        if (finishedSession !== session) return
        session = null
        val next = afterExit
        afterExit = null
        if (next != null) return next()
        // The core quit by itself: q in Modo terminal closes the app; a crash stays on screen with Reintentar.
        val clean = finishedSession.exitStatus == 0
        if (clean) Remote.quitting.value = true else Remote.stopped.value = true
        finish(clean)
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
        private const val SEEK_HOLD_MS = 15_000L
        private const val THEME_ART_PX = 512
        const val EXTRA_LINK = "link"
        const val ACTION_PREV = "io.github.xaviiromf.cliampx.PREV"
        const val ACTION_TOGGLE = "io.github.xaviiromf.cliampx.TOGGLE"
        const val ACTION_NEXT = "io.github.xaviiromf.cliampx.NEXT"
        const val ACTION_QUIT = "io.github.xaviiromf.cliampx.QUIT"
        const val ACTION_ERASE = "io.github.xaviiromf.cliampx.ERASE"
        const val ACTION_RESTART = "io.github.xaviiromf.cliampx.RESTART"
        const val ACTION_RESUME = "io.github.xaviiromf.cliampx.RESUME"

        fun send(context: Context, action: String) {
            context.startService(Intent(context, CliampService::class.java).setAction(action))
        }
    }
}
