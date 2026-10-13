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
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.conflate
import kotlinx.coroutines.flow.first
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
    private val _errors = MutableSharedFlow<String>(extraBufferCapacity = 8)
    val errors: SharedFlow<String> = _errors

    /** A play request waits for the first yt-dlp update. */
    private val _gateWaiting = MutableStateFlow(false)
    val gateWaiting: StateFlow<Boolean> = _gateWaiting.asStateFlow()

    /** The track of a play request in flight, shown until the core reports it. */
    private val _pending = MutableStateFlow<Track?>(null)
    val pending: StateFlow<Track?> = _pending.asStateFlow()

    @SuppressLint("StaticFieldLeak") // the application context
    private lateinit var app: Context
    private lateinit var ipc: Ipc
    private var loop: Job? = null
    private val lock = Any()
    private var floor = 0L
    private val retried = HashSet<String>()
    private var watching: String? = null
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
        loop = scope.launch {
            launch { render.run() }
            subscribe()
        }
    }

    fun stop() {
        loop?.cancel()
        loop = null
        _connected.value = false
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
    suspend fun call(op: String, params: JSONObject = JSONObject(), timeoutMs: Long = timeout(op)): JSONObject {
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
                job.optJSONObject("result") ?: JSONObject()
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
            Log.w(TAG, "call: $e")
            _errors.tryEmit(message(e))
        }
    }

    /** The text of a failed call (§5.7): invalid_params is a bug, so it names the operation. */
    fun message(e: Exception): String = when {
        e !is IpcError -> e.message ?: e.toString()
        e.code == "invalid_params" -> e.op
        else -> ErrorMap.text(e.code, e.detail)?.let(app::getString) ?: e.detail.ifEmpty { e.message.orEmpty() }
    }

    /** Plays tracks from index: queue.replace (G6), else track.play of that one track. */
    fun play(tracks: List<Track>, index: Int) {
        scope.launch {
            _pending.value = tracks[index]
            try {
                report {
                    if ("queue.replace" in ops.value) {
                        call("queue.replace", JSONObject().put("tracks", JSONArray(tracks.map { it.json() })).put("index", index))
                    } else {
                        // shortcut: the only fallback; remove once Batch A is merged.
                        call("track.play", JSONObject().put("track", tracks[index].json()))
                    }
                }
            } finally {
                _pending.value = null
            }
        }
    }

    suspend fun search(query: String): List<Track> = cached("search:$query", 120_000) {
        Track.list(call("provider.search", JSONObject().put("provider", "ytmusic").put("query", query).put("limit", 40)))
    }

    @Suppress("UNCHECKED_CAST")
    suspend fun <T> cached(key: String, ttlMs: Long, load: suspend () -> T): T {
        val now = SystemClock.elapsedRealtime()
        synchronized(cache) { cache[key]?.takeIf { now - it.first < ttlMs } }?.let { return it.second as T }
        return load().also { synchronized(cache) { cache[key] = now to it } }
    }

    fun invalidate() = synchronized(cache) { cache.clear() }

    private suspend fun awaitGate() {
        if (!Ytdlp.gate(Ytdlp.tag(app), Ytdlp.status.value)) return
        _gateWaiting.value = true
        try {
            withTimeoutOrNull(45_000) { Ytdlp.status.first { it !is Ytdlp.Status.Updating } }
        } finally {
            _gateWaiting.value = false
        }
    }

    /** Retries a failed YouTube play once yt-dlp is new (§7.8). */
    private fun watchError(s: Snapshot) {
        if (!Ytdlp.watches(s)) return
        val path = s.errorPath
        synchronized(lock) {
            if (path in retried || watching == path) return
            watching = path
        }
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
                synchronized(lock) { if (watching == path) watching = null }
            }
        }
    }

    /** Plays the failed track again, when it is still the one on screen. */
    private suspend fun retry(path: String) {
        val s = state.value ?: return
        if (s.track?.path != path) return
        synchronized(lock) { retried += path }
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
