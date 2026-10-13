package io.github.xaviiromf.cliampx

import android.annotation.SuppressLint
import android.content.Context
import android.os.SystemClock
import android.util.Log
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.awaitCancellation
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.conflate
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import java.io.IOException

/**
 * The native UI's link to the core (UI-SPEC §5, §7.2): the runtime.state
 * subscription, operation calls through V2 jobs, the yt-dlp play gate and
 * error watcher, and the ui.render mode. CliampService starts it.
 */
object Remote {
    private const val TAG = "cliamp-x"

    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    private val _state = MutableStateFlow<Snapshot?>(null)
    val state: StateFlow<Snapshot?> = _state.asStateFlow()
    private val _connected = MutableStateFlow(false)
    val connected: StateFlow<Boolean> = _connected.asStateFlow()
    private val _ops = MutableStateFlow<Set<String>>(emptySet())
    val ops: StateFlow<Set<String>> = _ops.asStateFlow()
    private val _signedIn = MutableStateFlow<Boolean?>(null)
    val signedIn: StateFlow<Boolean?> = _signedIn.asStateFlow()

    /** Salir: the activities finish. */
    val quitting = MutableStateFlow(false)

    /** The core ended by itself with an error: the shell offers Reintentar (§2.10). */
    val stopped = MutableStateFlow(false)
    private val _errors = MutableSharedFlow<String>(extraBufferCapacity = 8)
    val errors: SharedFlow<String> = _errors

    /** How many play requests wait for the first yt-dlp update. */
    private val _gateWaiting = MutableStateFlow(0)
    val gateWaiting: StateFlow<Int> = _gateWaiting.asStateFlow()

    /** The track of a play request in flight, shown until the core reports it. */
    private val plays = Latest<Track>(scope)
    val pending: StateFlow<Track?> = plays.current

    @SuppressLint("StaticFieldLeak") // the application context
    private lateinit var app: Context
    private lateinit var ipc: Ipc
    private var loop: Job? = null
    private val lock = Any()
    private var floor = 0L
    private val guard = RetryGuard()
    private var lastForced = 0L
    private val cache = HashMap<String, Pair<Long, Any?>>()
    private val render = RenderSender { call("ui.render", JSONObject().put("name", it)) }

    private val PLAY_OPS = setOf(
        "play", "toggle", "next", "prev", "queue.play", "queue.replace", "track.play",
        "provider.load", "provider.load_album", "radio.start",
    )
    private val POLL = longArrayOf(0, 30, 30, 60, 130)

    /** Starts the subscription for the life of the service. Idempotent. */
    fun start(ctx: Context) {
        if (loop?.isActive == true) return
        app = ctx.applicationContext
        ipc = Ipc(Core.socket(app))
        quitting.value = false
        stopped.value = false
        loop = scope.launch {
            launch { render.run() }
            subscribe()
        }
    }

    fun stop() {
        loop?.cancel()
        loop = null
        _connected.value = false
        // No core plays now: the last song shows stopped, so the tickers and the marquee stop.
        val now = SystemClock.elapsedRealtime()
        synchronized(lock) {
            _state.update { it?.copy(state = "stopped", buffering = false, position = PlaybackClock.positionAt(it, now), receivedAt = now) }
        }
    }

    /** Modo terminal is on screen: the core draws only then. */
    fun terminalVisible(on: Boolean) {
        render.terminal.value = on
    }

    private suspend fun subscribe() = withContext(Dispatchers.IO) {
        var wait = 200L
        while (isActive) {
            try {
                ipc.subscribe(listOf("runtime.state")).use { sub ->
                    coroutineScope {
                        // Cancelling the loop closes the socket, which ends the blocked read.
                        val closer = launch {
                            try {
                                awaitCancellation()
                            } finally {
                                sub.close()
                            }
                        }
                        synchronized(lock) { floor = 0 } // a new core counts revisions from 0
                        launch { connected() }
                        wait = 200L
                        while (true) {
                            val event = sub.next() ?: break
                            if (event.optString("event") == "system.overflow") break
                            val data = event.optJSONObject("data") ?: continue
                            apply(data, if (event.optBoolean("retained")) Source.RETAINED else Source.EVENT)
                        }
                        closer.cancel()
                    }
                }
            } catch (e: IOException) {
                // The core is not up yet, or it stopped: retry below.
            } catch (e: JSONException) {
                Log.w(TAG, "runtime.state: $e")
            }
            _connected.value = false
            delay(wait)
            wait = (wait * 2).coerceAtMost(2000)
        }
    }

    /** What every (re)subscribe refreshes (§5.2). */
    private suspend fun connected() {
        try {
            stateGet()
            _connected.value = true
            val caps = ipc.call(JSONObject().put("id", "capabilities").put("method", "capabilities")).optJSONArray("result") ?: JSONArray()
            _ops.value = (0 until caps.length()).map { caps.getJSONObject(it).optString("name") }.toSet()
            val providers = call("provider.list").optJSONArray("providers") ?: JSONArray()
            _signedIn.value = (0 until providers.length()).map { providers.getJSONObject(it) }
                .firstOrNull { it.optString("key") == "ytmusic" }?.takeIf { it.has("signed_in") }?.optBoolean("signed_in")
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            Log.w(TAG, "connect: $e")
        }
        invalidate()
        render.reset()
    }

    private fun stateGet() {
        ipc.state().optJSONObject("snapshot")?.let { apply(it, Source.STATE_GET) }
    }

    /** Reads state.get, for the position while playing (events skip position ticks). */
    fun refresh() {
        scope.launch(Dispatchers.IO) { runCatching { stateGet() } }
    }

    private fun apply(json: JSONObject, source: Source) {
        val snap = Snapshot.parse(json, SystemClock.elapsedRealtime())
        val merged = synchronized(lock) {
            Snapshots.merge(_state.value, snap, source, floor)?.also {
                floor = it.revision
                _state.value = it
            }
        } ?: return
        watchError(merged)
    }

    /**
     * Runs an operation as a V2 job and returns its result (§5.1). Play
     * operations first wait behind the yt-dlp gate. Cancelling the caller
     * cancels the job. Throws IpcError.
     */
    suspend fun call(op: String, params: JSONObject = JSONObject(), timeoutMs: Long = timeout(op)): JSONObject =
        job(op, params, timeoutMs).optJSONObject("result") ?: JSONObject()

    /** Runs an operation like call and returns the whole succeeded job: its result and the snapshot it committed. */
    suspend fun job(op: String, params: JSONObject = JSONObject(), timeoutMs: Long = timeout(op)): JSONObject {
        if (op in PLAY_OPS || (op == "url.load" && params.optBoolean("play"))) awaitGate()
        return withContext(Dispatchers.IO) {
            val submitted = checked(op) { ipc.submit(op, params) }.getJSONObject("job")
            val id = submitted.getString("id")
            var done = false
            try {
                val job = withTimeoutOrNull(timeoutMs) {
                    var j = submitted
                    var i = 0
                    while (j.optString("state") !in setOf("succeeded", "failed", "canceled")) {
                        delay(POLL.getOrElse(i++) { 250L })
                        j = checked(op) { ipc.job(id) }.getJSONObject("job")
                    }
                    j
                } ?: throw IpcError(op, "timeout", "timeout")
                done = true
                job.optJSONObject("snapshot")?.let { apply(it, if (op in Snapshots.PLAYBACK_OPS) Source.PLAYBACK_JOB else Source.OTHER_JOB) }
                if (job.optString("state") != "succeeded") throw IpcError.of(op, job.optJSONObject("error"))
                job
            } finally {
                if (!done) withContext(NonCancellable) { runCatching { ipc.cancel(id) } }
            }
        }
    }

    private inline fun checked(op: String, request: () -> JSONObject): JSONObject {
        val response = try {
            request()
        } catch (e: IOException) {
            throw IpcError(op, "unavailable", e.message.orEmpty())
        }
        if (!response.optBoolean("ok")) throw IpcError.of(op, response.optJSONObject("error"))
        return response
    }

    private fun timeout(op: String) = when {
        op.startsWith("provider.") -> 60_000L
        op == "radio.start" || op == "discover.remixes" -> 45_000L
        else -> 30_000L
    }

    /** Runs an operation in the background; a failure goes to the status line. */
    fun fire(op: String, params: JSONObject = JSONObject()) {
        scope.launch { report { call(op, params) } }
    }

    private suspend fun report(block: suspend () -> Unit) {
        try {
            block()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            showError(e)
        }
    }

    /** Puts a failed call on the status line; a stale queue edit only refetches, silently (§5.7). */
    fun showError(e: Exception) {
        Log.w(TAG, "call: $e")
        if (e is IpcError && e.code == "conflict" && (e.op.startsWith("queue.") || e.op.startsWith("playnext."))) return
        _errors.tryEmit(message(e))
    }

    /** The text of a failed call (§5.7): invalid_params is a bug, so it names the operation. */
    fun message(e: Exception): String = when {
        e !is IpcError -> e.message ?: e.toString()
        e.code == "invalid_params" -> e.op
        e.code == "unavailable" && stopped.value -> app.getString(R.string.core_stopped)
        else -> ErrorMap.text(e.code, e.detail)?.let(app::getString) ?: e.detail.ifEmpty { e.message.orEmpty() }
    }

    /** Plays tracks from index with queue.replace (G6). */
    fun play(tracks: List<Track>, index: Int) {
        plays.launch(tracks[index]) {
            report { call("queue.replace", JSONObject().put("tracks", JSONArray(tracks.map { it.json() })).put("index", index)) }
        }
    }

    /** The search rows by the core's sections, parsed off the main thread (§7.2). */
    suspend fun search(query: String): List<Section> = cached("search:$query", 120_000) {
        withContext(Dispatchers.IO) {
            searchSections(Track.list(call("provider.search", JSONObject().put("provider", "ytmusic").put("query", query).put("limit", 40))))
        }
    }

    /** Toggles the ♥ of track (G20); the job's snapshot carries the new state. */
    fun favorite(track: Track) = fire("playlist.bookmark", JSONObject().put("provider", "ytmusic").put("track", track.json()))

    /** The lyrics of the song that plays, path, kept for the session (§5.3 Player). */
    suspend fun lyrics(path: String): List<Lyric> = cached("lyrics:$path", Long.MAX_VALUE) {
        withContext(Dispatchers.IO) {
            try {
                Lyrics.parse(call("lyrics"))
            } catch (e: IpcError) {
                if ("no lyrics found" in e.detail) emptyList() else throw e
            }
        }
    }

    /**
     * Moves a queue row (queue.move, live indexes, from rows read at
     * revision) or a play-next row (playnext.move, revision null) as
     * adjacent swaps (§7.7). Throws on the first failure.
     */
    suspend fun move(op: String, from: Int, to: Int, revision: Long?) = sendSwaps(from, to, revision) { job(op, it) }

    /** A page of the tracks after the playing one, in play order (G22). */
    suspend fun upNext(offset: Int): Page = withContext(Dispatchers.IO) {
        Page.parse(job("queue.list", JSONObject().put("sort", "upnext").put("offset", offset).put("limit", 200)))
    }

    suspend fun playNext(): Page = withContext(Dispatchers.IO) { Page.parse(job("playnext.list", JSONObject().put("limit", 200))) }

    /** The EQ presets (G19), Custom last. */
    suspend fun eqPresets(): List<String> = cached("eq:list", Long.MAX_VALUE) {
        withContext(Dispatchers.IO) {
            val items = call("eq", JSONObject().put("name", "list")).optJSONArray("items") ?: JSONArray()
            List(items.length()) { items.getString(it) }
        }
    }

    /**
     * spectrum.get frames every 33 ms over one connection while collected
     * (§5.4), reconnecting after 500 ms on an error.
     */
    fun spectrum(): Flow<FloatArray> = flow {
        while (true) {
            try {
                SpectrumStream(Core.socket(app)).use { stream ->
                    while (true) {
                        emit(stream.next())
                        delay(33)
                    }
                }
            } catch (e: IOException) {
                delay(500)
            } catch (e: JSONException) {
                delay(500)
            }
        }
    }.flowOn(Dispatchers.IO)

    /** The last results of query however old, so a screen that comes back shows them at once (§7.3). */
    @Suppress("UNCHECKED_CAST")
    fun lastSearch(query: String) = synchronized(cache) { cache["search:$query"]?.second as List<Section>? }

    @Suppress("UNCHECKED_CAST")
    suspend fun <T> cached(key: String, ttlMs: Long, load: suspend () -> T): T {
        val now = SystemClock.elapsedRealtime()
        synchronized(cache) { cache[key]?.takeIf { now - it.first < ttlMs } }?.let { return it.second as T }
        return load().also { synchronized(cache) { cache[key] = now to it } }
    }

    fun invalidate() = synchronized(cache) { cache.clear() }

    private suspend fun awaitGate() {
        if (!Ytdlp.gate(Ytdlp.tag(app), Ytdlp.status.value)) return
        _gateWaiting.update { it + 1 }
        try {
            withTimeoutOrNull(45_000) { Ytdlp.status.first { it !is Ytdlp.Status.Updating } }
        } finally {
            _gateWaiting.update { it - 1 }
        }
    }

    /** Retries a failed YouTube play once yt-dlp is new (§7.8). */
    private fun watchError(s: Snapshot) {
        if (!Ytdlp.watches(s)) return
        val path = s.errorPath
        if (!guard.start(path)) return
        scope.launch {
            try {
                val retry = when (
                    Ytdlp.watch(
                        Ytdlp.status.value, SystemClock.elapsedRealtime(), Ytdlp.finishedAt, Ytdlp.tag(app),
                        lastForced, s.errorKind, Ytdlp.checked(app), System.currentTimeMillis(),
                    )
                ) {
                    Ytdlp.Watch.WAIT -> Ytdlp.status.first { it !is Ytdlp.Status.Updating }.let { it is Ytdlp.Status.Updated || it is Ytdlp.Status.UpToDate }
                    Ytdlp.Watch.RETRY -> true
                    Ytdlp.Watch.UPDATE -> {
                        lastForced = SystemClock.elapsedRealtime()
                        Ytdlp.update(app, force = true).let { it is Ytdlp.Status.Updated || it is Ytdlp.Status.UpToDate }
                    }
                    Ytdlp.Watch.UPDATE_IF_NEW -> Ytdlp.update(app, force = true) is Ytdlp.Status.Updated
                    Ytdlp.Watch.NONE -> false
                }
                if (retry) retry(path)
            } finally {
                guard.done(path)
            }
        }
    }

    /** Plays the failed track again, when it is still the one on screen. */
    private suspend fun retry(path: String) {
        val s = state.value ?: return
        if (s.track?.path != path) return
        guard.markRetried(path)
        report { call("queue.play", JSONObject().put("index", s.index)) }
    }

    /** Actualizar yt-dlp: a forced update, then one retry of a failed song on screen. */
    suspend fun updateYtdlp(): Ytdlp.Status {
        val status = Ytdlp.update(app, force = true)
        val path = state.value?.takeIf { it.playError != null }?.errorPath
        if (path != null && status !is Ytdlp.Status.Failed) retry(path)
        return status
    }
}

/** The error watcher's bookkeeping (§7.8): one watcher per failed path at a time, one automatic retry per path. */
class RetryGuard {
    private val retried = HashSet<String>()
    private var watching: String? = null

    /** Whether an error on path starts a watcher, which then holds path until done. */
    @Synchronized
    fun start(path: String): Boolean {
        if (path in retried || watching == path) return false
        watching = path
        return true
    }

    @Synchronized
    fun done(path: String) {
        if (watching == path) watching = null
    }

    @Synchronized
    fun markRetried(path: String) {
        retried += path
    }
}

/**
 * Runs one play request at a time: a new one cancels the one before it, so
 * the last tap wins also when both wait behind the yt-dlp gate (a request
 * cancelled there sends nothing). current is the item of the running one.
 */
class Latest<T : Any>(private val scope: CoroutineScope) {
    private val _current = MutableStateFlow<T?>(null)
    val current: StateFlow<T?> = _current.asStateFlow()
    private var job: Job? = null

    fun launch(item: T, block: suspend () -> Unit) = synchronized(this) {
        job?.cancel()
        _current.value = item
        job = scope.launch {
            try {
                block()
            } finally {
                // The lock waits for the assignment above; a cancelled request leaves its successor's item.
                synchronized(this@Latest) { if (job === coroutineContext[Job]) _current.value = null }
            }
        }
    }
}

/**
 * Sends ui.render one request at a time, so a fast on, off, on always ends
 * in the last mode (§7.6). reset() makes it send again, after a reconnect.
 */
class RenderSender(private val send: suspend (String) -> Unit) {
    val terminal = MutableStateFlow(false)
    private val epoch = MutableStateFlow(0)

    fun reset() = epoch.update { it + 1 }

    suspend fun run() {
        var applied: String? = null
        var appliedEpoch = -1
        combine(terminal, epoch) { on, n -> (if (on) "on" else "off") to n }.conflate().collect { (want, n) ->
            if (want == applied && n == appliedEpoch) return@collect
            try {
                send(want)
                applied = want
                appliedEpoch = n
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                // The core is down: the reconnect resets and sends again.
            }
        }
    }
}
