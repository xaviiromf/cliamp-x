package io.github.xaviiromf.cliampx

import android.annotation.SuppressLint
import android.content.Context
import android.net.ConnectivityManager
import android.net.Uri
import android.net.Network
import android.net.NetworkCapabilities
import android.os.Environment
import android.os.SystemClock
import android.provider.DocumentsContract
import android.provider.OpenableColumns
import android.util.Log
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.awaitCancellation
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.conflate
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filterNotNull
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
import java.io.File
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

    /** A done action the core does not report, for the status line (§2.10). */
    private val _info = MutableSharedFlow<String>(extraBufferCapacity = 8)
    val info: SharedFlow<String> = _info

    /** How many play requests wait for the first yt-dlp update. */
    private val _gateWaiting = MutableStateFlow(0)
    val gateWaiting: StateFlow<Int> = _gateWaiting.asStateFlow()

    /** The track of a play request in flight, shown until the core reports it. */
    private val plays = Latest<Track>(scope)

    /** The song of the last play request while it still loads (§2.2): the core reports no buffering during the yt-dlp extraction. */
    private val awaiting = MutableStateFlow<Track?>(null)
    private var awaitTimeout: Job? = null

    /** The song a play request loads, from the tap until it plays, fails, another song plays, or 45 s pass. */
    val pending: StateFlow<Track?> = combine(plays.current, awaiting) { a, b -> a ?: b }
        .stateIn(scope, SharingStarted.Eagerly, null)

    /** The ♥ state by path that the core reported after the list rows were read (§5.3: a bookmark leaves cached rows stale). */
    private val _liked = MutableStateFlow<Map<String, Boolean>>(emptyMap())
    val liked: StateFlow<Map<String, Boolean>> = _liked.asStateFlow()

    /** The save, follow and pin state by ID that the core returned for a toggle: cached rows and previews show the old one (§5.3). */
    private val _favorites = MutableStateFlow<Map<String, Boolean>>(emptyMap())
    val favorites: StateFlow<Map<String, Boolean>> = _favorites.asStateFlow()

    /** The IDs and paths rated "No me interesa" in this process: lists drop them at once (§2.6). */
    private val _hidden = MutableStateFlow<Set<String>>(emptySet())
    val hidden: StateFlow<Set<String>> = _hidden.asStateFlow()

    /** The network the core plays for, wifi or mobile (Offline.networkKind), null while unknown. */
    private val _network = MutableStateFlow<String?>(null)
    val network: StateFlow<String?> = _network.asStateFlow()
    private var netWatch: ConnectivityManager.NetworkCallback? = null

    /** Moves on each reconnect and pull to refresh: the screens on show load again. */
    private val _reloads = MutableStateFlow(0)
    val reloads: StateFlow<Int> = _reloads.asStateFlow()

    @SuppressLint("StaticFieldLeak") // the application context
    private lateinit var app: Context
    private lateinit var ipc: Ipc
    private var loop: Job? = null
    private val lock = Any()
    private var floor = 0L
    private val guard = RetryGuard()
    private var lastForced = 0L
    // Access-ordered and capped: the process lives as long as the service, and every query and page is a key.
    private val cache = object : LinkedHashMap<String, Pair<Long, Any?>>(16, 0.75f, true) {
        override fun removeEldestEntry(eldest: MutableMap.MutableEntry<String, Pair<Long, Any?>>) = size > 64
    }
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
        watchNetwork()
        loop = scope.launch {
            launch { render.run() }
            launch { reportNetwork() }
            subscribe()
        }
    }

    fun stop() {
        loop?.cancel()
        loop = null
        netWatch?.let { runCatching { app.getSystemService(ConnectivityManager::class.java).unregisterNetworkCallback(it) } }
        netWatch = null
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

    private fun watchNetwork() {
        if (netWatch != null) return
        val cb = object : ConnectivityManager.NetworkCallback() {
            override fun onCapabilitiesChanged(network: Network, caps: NetworkCapabilities) {
                _network.value = Offline.networkKind(caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_METERED))
            }
        }
        // A phone without the permission or a callback slot plays on the core's last network (wifi at start).
        if (runCatching { app.getSystemService(ConnectivityManager::class.java).registerDefaultNetworkCallback(cb) }.isSuccess) netWatch = cb
    }

    /**
     * Tells the core the network at every (re)connect and change, so mobile
     * data plays with quality_mobile (settings.set network, not saved by the
     * core), then fills the automatic cache when it is due.
     */
    private suspend fun reportNetwork() {
        combine(_network, _connected) { n, up -> n.takeIf { up } }.distinctUntilChanged().filterNotNull().collect { n ->
            try {
                call("settings.set", JSONObject().put("name", "network").put("text", n))
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                Log.w(TAG, "network: $e") // an older core without the setting
            }
            fillCache(force = false)
        }
    }

    /**
     * Fills the offline cache with download.auto (♥ favorites, then recently
     * played) within the limits of Ajustes, when it is due or with force (the
     * Llenar ahora button, which reports how many songs it queued).
     */
    suspend fun fillCache(force: Boolean) {
        val p = Offline.prefs(app)
        val now = System.currentTimeMillis()
        val on = force || p.getBoolean(Offline.CACHE, false)
        if (!Offline.cacheDue(on, p.getBoolean(Offline.CACHE_WIFI_ONLY, true), _network.value, if (force) 0 else Offline.lastFilled(p), now)) {
            if (force) _errors.tryEmit(app.getString(R.string.wifi_only_blocked))
            return
        }
        val (songs, mb) = Offline.cacheParams(p)
        try {
            val queued = call("download.auto", JSONObject().put("limit", songs).put("value", mb)).optInt("total")
            Offline.markFilled(p, now)
            if (force) _info.tryEmit(app.resources.getQuantityString(R.plurals.cache_queued, queued, queued))
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            if (force) showError(e) else Log.w(TAG, "download.auto: $e")
        }
    }

    /** Descargar from a sheet: download, unless Descargas solo con Wi-Fi holds it on mobile data. */
    fun download(params: JSONObject) {
        if (!Offline.allowed(Offline.prefs(app).getBoolean(Offline.WIFI_ONLY, false), _network.value)) {
            _errors.tryEmit(app.getString(R.string.wifi_only_blocked))
            return
        }
        fire("download", params)
    }

    /** settings.set with text, for the scrobbling credentials; done runs on Main, or failed gets the error's text. */
    fun setText(name: String, text: String, failed: (String) -> Unit = {}, done: () -> Unit = {}) {
        scope.launch {
            attempt(failed) {
                call("settings.set", JSONObject().put("name", name).put("text", text))
                withContext(Dispatchers.Main) { done() }
            }
        }
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
            // A new core (a restart, a sign-in) has other lists: the screens on show load again.
            _favorites.value = emptyMap()
            MusicFolders.syncPhone(app)
            reload()
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
        merged.track?.let { t -> if (_liked.value[t.path] != t.favorite) _liked.update { it + (t.path to t.favorite) } }
        awaiting.value?.let { if (!loads(it.path, merged)) awaiting.compareAndSet(it, null) }
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

    /**
     * Whether the phone has a working network. Offline, the core serves its
     * saved lists, so Inicio's pull says "Sin conexión a internet." itself (§2.10).
     */
    fun online(): Boolean {
        val cm = app.getSystemService(ConnectivityManager::class.java)
        return cm.getNetworkCapabilities(cm.activeNetwork)?.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED) == true
    }

    /** The text of a failed call (§5.7): invalid_params is a bug, so it names the operation. */
    fun message(e: Exception): String = when {
        e !is IpcError -> e.message ?: e.toString()
        e.code == "invalid_params" -> e.op
        e.code == "unavailable" && stopped.value -> app.getString(R.string.core_stopped)
        else -> ErrorMap.text(e.code, e.detail)?.let(app::getString) ?: e.detail.ifEmpty { e.message.orEmpty() }
    }

    /** Plays tracks from index with queue.replace (G6). */
    fun play(tracks: List<Track>, index: Int) =
        start(tracks[index], "queue.replace", JSONObject().put("tracks", JSONArray(tracks.map { it.json() })).put("index", index))

    /** Loads a provider list and plays its row index, the song track (G5). */
    fun load(provider: String, playlist: String, index: Int, track: Track) =
        start(track, "provider.load", JSONObject().put("provider", provider).put("playlist", playlist).put("index", index))

    /** Loads an album and plays its row index; a release (al:) loads as a list, which marks it seen (§5.3 Album). */
    fun loadAlbum(id: String, index: Int, track: Track) =
        if (id.startsWith("al:")) load("ytmusic", id, index, track) else start(track, "provider.load_album", yt().put("album", id).put("index", index))

    /** Runs a play request; the mini-player shows track loading until it plays (§2.2). */
    private fun start(track: Track, op: String, params: JSONObject) {
        plays.launch(track) {
            report {
                call(op, params)
                awaiting.value = track
                awaitTimeout?.cancel()
                awaitTimeout = scope.launch {
                    delay(45_000)
                    awaiting.compareAndSet(track, null)
                }
                state.value?.let { if (!loads(track.path, it)) awaiting.compareAndSet(track, null) }
            }
        }
    }

    private fun yt() = JSONObject().put("provider", "ytmusic")

    private suspend fun <T> io(block: suspend () -> T) = withContext(Dispatchers.IO) { block() }

    /** Escuchado recientemente (§5.3 Inicio). */
    suspend fun history(): List<Track> = cached("history", 60_000) { io { Items.history(call("history", JSONObject().put("limit", 30))) } }

    suspend fun home(): List<Shelf> = cached("home", 600_000) { io { Items.shelves(call("provider.home", yt())) } }

    /** A page of the endless "For you" albums or artists (G4, §5.5). */
    suspend fun forYou(artists: Boolean, offset: Int, limit: Int): List<Item> = cached("foryou:$artists:$offset:$limit", 600_000) {
        io {
            val params = yt().put("sort", "foryou").put("offset", offset).put("limit", limit)
            if (artists) Items.artists(call("provider.artists", params)) else Items.albums(call("provider.albums", params))
        }
    }

    /** Every row of a paged list (§5.5 "all pages"): page reads the rows from an offset and the full length. */
    private suspend fun <T> all(page: suspend (Int) -> Pair<List<T>, Int>): List<T> = io {
        val rows = ArrayList<T>()
        do {
            val (items, total) = page(rows.size)
            rows += items
        } while (items.isNotEmpty() && Paging.next(rows.size, total) != null)
        rows
    }

    /** Every row of a provider's lists. */
    suspend fun playlists(provider: String): List<Item> = cached("playlists:$provider", 600_000) {
        all { offset -> call("provider.playlists", JSONObject().put("provider", provider).put("offset", offset).put("limit", 200)).let { Items.playlists(it) to it.optInt("total") } }
    }

    /** The account's artists, ♥ the followed ones (§5.3 Biblioteca). */
    suspend fun artists(): List<Item> = cached("artists", 600_000) {
        all { offset -> call("provider.artists", yt().put("offset", offset).put("limit", 200)).let { Items.artists(it) to it.optInt("total") } }
    }

    /** A page of the account's albums in sort order, and the orders; total is the page length, as foryou (§5.5). */
    suspend fun albums(sort: String, offset: Int): Pair<List<Item>, List<Sort>> = cached("albums:$sort:$offset", 600_000) {
        io { call("provider.albums", yt().put("sort", sort).put("offset", offset).put("limit", ALBUMS)).let { Items.albums(it) to Items.sorts(it) } }
    }

    const val ALBUMS = 100

    /** The genres, or with moods the moods and moments, and the play orders (G3); kept for the session. */
    suspend fun genres(moods: Boolean): Pair<List<Item>, List<Sort>> = cached("genres:$moods", Long.MAX_VALUE) {
        io { Items.genres(call("provider.genres", yt().apply { if (moods) put("playlist", "browse:moods") })) }
    }

    suspend fun genreTracks(id: String, sort: String): List<Track> = cached("genre:$id:$sort", 600_000) {
        io { Track.list(call("provider.genre_tracks", yt().put("playlist", id).put("sort", sort).put("limit", 200))) }
    }

    suspend fun preview(id: String): Preview = cached("preview:$id", 1_800_000) { io { Items.preview(call("provider.preview", yt().put("playlist", id))) } }

    /** An album's songs, a release's (al:), or an artist's top songs (ar:, local:ar:). */
    suspend fun albumTracks(id: String): List<Track> = cached("album:$id", 1_800_000) {
        io {
            val params = if (id.startsWith("al:")) yt().put("playlist", id) else yt().put("album", id)
            Track.list(call(if (id.startsWith("al:")) "provider.tracks" else "provider.album_tracks", params.put("limit", 200)))
        }
    }

    suspend fun artistAlbums(id: String): List<Item> = cached("artist:$id", 1_800_000) { io { Items.albums(call("provider.artist_albums", yt().put("artist", id))) } }

    /** A page of a provider list's songs (§5.5); Favorites and History change often. */
    suspend fun tracks(provider: String, id: String, offset: Int): Page =
        cached("tracks:$provider:$id:$offset", if (id == "fav" || id == "hist") 60_000 else 600_000) {
            io {
                val result = call("provider.tracks", JSONObject().put("provider", provider).put("playlist", id).put("offset", offset).put("limit", 200))
                Page(Track.list(result), result.optInt("total"))
            }
        }

    /** Remixes and other versions of track (G13). */
    suspend fun remixes(track: Track): List<Section> = cached("remixes:${track.path}", 600_000) {
        io { searchSections(Track.list(call("discover.remixes", JSONObject().put("track", track.json())))) }
    }

    /** Toggles a save, follow or genre star (provider.favorite, provider.genre_favorite); favorites gets the new state (G20). */
    fun toggle(op: String, id: String) {
        scope.launch {
            report {
                val on = call(op, yt().put("playlist", id)).optBoolean("favorite")
                _favorites.update { it + (id to on) }
                invalidate() // cached lists show the old state (§5.3)
            }
        }
    }

    /** The songs of an item for its sheet's actions: an album's, a list's first page, a genre's mix, an artist's top songs. */
    suspend fun tracksOf(item: Item): List<Track> = when (item.kind) {
        Kind.SONG -> listOfNotNull(item.track)
        Kind.ALBUM -> albumTracks(item.id)
        Kind.ARTIST -> albumTracks(if (item.local) item.id else "ar:${item.id}")
        Kind.PLAYLIST -> tracks(item.provider, item.id, 0).rows
        Kind.GENRE -> genreTracks(item.id, "mix")
    }

    /** Reproducir or Aleatorio from an item's sheet: its list plays from the first row, or from a random one with shuffle on. */
    fun playItem(item: Item, shuffle: Boolean = false) {
        scope.launch {
            report {
                val tracks = tracksOf(item)
                if (tracks.isEmpty()) return@report
                if (shuffle) call("shuffle", JSONObject().put("name", "on"))
                val i = if (shuffle) tracks.indices.random() else 0
                when (item.kind) {
                    Kind.ALBUM -> loadAlbum(item.id, i, tracks[i])
                    Kind.PLAYLIST -> load(item.provider, item.id, i, tracks[i])
                    else -> play(tracks, i)
                }
            }
        }
    }

    /** Reproducir a continuación for every song of item, in order (the core queues one track a call). */
    fun queueItem(item: Item) {
        scope.launch { report { for (t in tracksOf(item)) call("track.queue", JSONObject().put("track", t.json())) } }
    }

    /** Adds tracks, or item's songs, to a saved list (playlist.add, playlist.add_many). */
    fun addTo(provider: String, playlist: String, tracks: List<Track>, item: Item?) {
        scope.launch { report { add(provider, playlist, tracks.ifEmpty { item?.let { tracksOf(it) }.orEmpty() }) } }
    }

    /**
     * Nueva lista: creates name on provider (G20 gives its ID), then adds the
     * songs, or with queue the whole queue (Guardar como lista); done runs
     * after, or failed with the error's text.
     */
    fun createList(provider: String, name: String, tracks: List<Track>, item: Item?, queue: Boolean, failed: (String) -> Unit, done: () -> Unit) {
        scope.launch {
            attempt(failed) {
                val id = call("playlist.create", JSONObject().put("provider", provider).put("playlist", name)).optString("playlist").ifEmpty { name }
                add(provider, id, if (queue) queueTracks() else tracks.ifEmpty { item?.let { tracksOf(it) }.orEmpty() })
                reload() // Biblioteca shows the new list
                done()
            }
        }
    }

    /**
     * Importar M3U: reads the file at uri (at most M3u.MAX_BYTES and M3u.MAX_ENTRIES entries), keeps the URLs and the files that exist (a relative
     * path is looked up in the file's folder and the music folders), and makes a local list named after the file of them. The status line says how many.
     */
    fun importM3u(uri: Uri) {
        scope.launch {
            report {
                val (name, text) = io { readM3u(uri) }
                val entries = M3u.parse(text)
                if (entries.size > M3u.MAX_ENTRIES) throw IllegalArgumentException(app.getString(R.string.m3u_too_big))
                val roots = listOfNotNull(m3uFolder(uri)) + MusicFolders.list.value
                val tracks = io { entries.mapNotNull { e -> M3u.resolve(e.path, roots) { File(it).isFile }?.let { M3u.track(e, it) } } }
                val skipped = entries.size - tracks.size
                if (tracks.isEmpty()) {
                    _info.tryEmit(app.getString(R.string.m3u_none, skipped))
                    return@report
                }
                val taken = playlists("local").map { it.id }
                val base = name.replace(Regex("[/\\:<>\"|?*]"), " ").trim().ifEmpty { "M3U" }
                val list = generateSequence(1) { it + 1 }.map { if (it == 1) base else "$base $it" }.first { Library.nameError(it, taken) == null }
                val id = call("playlist.create", JSONObject().put("provider", "local").put("playlist", list)).optString("playlist").ifEmpty { list }
                for (chunk in tracks.chunked(200)) call("playlist.add_many", JSONObject().put("provider", "local").put("playlist", id).put("tracks", JSONArray(chunk)))
                reload()
                _info.tryEmit(app.resources.getQuantityString(R.plurals.m3u_imported, tracks.size, tracks.size, list, skipped))
            }
        }
    }

    /** The display name without its extension and the text of the file at uri; throws when it is too big. */
    private fun readM3u(uri: Uri): Pair<String, String> {
        val name = app.contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { if (it.moveToFirst()) it.getString(0) else null }
        val bytes = app.contentResolver.openInputStream(uri)?.use { it.readNBytes(M3u.MAX_BYTES + 1) } ?: throw IOException(app.getString(R.string.m3u_unreadable))
        if (bytes.size > M3u.MAX_BYTES) throw IllegalArgumentException(app.getString(R.string.m3u_too_big))
        return (name ?: "M3U").substringBeforeLast('.') to String(bytes, Charsets.UTF_8)
    }

    /** The folder of the file at uri when it is a path on the phone's storage, else null. */
    private fun m3uFolder(uri: Uri): String? = runCatching {
        Core.treePath(uri.authority, DocumentsContract.getDocumentId(uri), Environment.getExternalStorageDirectory().path)?.substringBeforeLast('/')
    }.getOrNull()

    /** Exportar M3U: writes every song of the list item to the document at uri (G20: the status line says how many). */
    fun exportM3u(item: Item, uri: Uri) {
        scope.launch {
            report {
                val tracks = all { offset -> tracks(item.provider, item.id, offset).let { it.rows to it.total } }
                io {
                    app.contentResolver.openOutputStream(uri, "wt")?.use { it.write(M3u.write(tracks).toByteArray(Charsets.UTF_8)) } ?: throw IOException(app.getString(R.string.m3u_unreadable))
                }
                _info.tryEmit(app.getString(R.string.m3u_exported, item.title))
            }
        }
    }

    /** Runs block; a failure goes to the status line and, on Main, to failed with its text, for a sheet that covers the status line. */
    private suspend fun attempt(failed: (String) -> Unit, block: suspend () -> Unit) {
        try {
            block()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            showError(e)
            withContext(Dispatchers.Main) { failed(message(e)) }
        }
    }

    /** The whole queue in track order, every page of queue.list. */
    private suspend fun queueTracks(): List<Track> =
        all { offset -> call("queue.list", JSONObject().put("offset", offset).put("limit", 200)).let { Track.list(it) to it.optInt("total") } }

    private suspend fun add(provider: String, playlist: String, tracks: List<Track>) {
        val params = JSONObject().put("provider", provider).put("playlist", playlist)
        when (tracks.size) {
            0 -> {}
            1 -> call("playlist.add", params.put("track", tracks[0].json()))
            else -> call("playlist.add_many", params.put("tracks", JSONArray(tracks.map { it.json() })))
        }
        invalidate() // the list's cached rows and counts are old (§5.3)
        _info.tryEmit(app.getString(if (tracks.isEmpty()) R.string.list_created else R.string.added))
    }

    // One local list edit at a time, in the order given: a row's index counts the removals before it.
    private val edits = Channel<suspend () -> Unit>(Channel.UNLIMITED).also { ch -> scope.launch { for (edit in ch) edit() } }

    /**
     * Runs a local playlist edit (playlist.rename, .delete, .remove) after
     * the ones before it; then the screens on show load again and done runs
     * on Main, or, when it fails, the error shows and failed gets its text on Main.
     */
    fun editList(op: String, params: JSONObject, failed: (String) -> Unit = {}, done: () -> Unit = {}) {
        edits.trySend {
            attempt(failed) {
                call(op, params.put("provider", "local"))
                reload()
                withContext(Dispatchers.Main) { done() }
            }
        }
    }

    /** Deshacer: forgets a rating (taste.rate value 0, G11); a song or item rated "No me interesa" shows again. */
    suspend fun unrate(r: Rating) {
        call("taste.rate", JSONObject().put("kind", r.kind).put("playlist", r.id).put("value", 0))
        _hidden.update { it - r.id }
        invalidate()
    }

    suspend fun ratings(): List<Rating> = io { Rating.list(call("taste.list")) }

    suspend fun stats(period: String): Stats = io { Stats.parse(call("stats", JSONObject().put("name", period))) }

    /** The calendar year's story (T8), or the current year's with 0. */
    suspend fun wrapped(year: Int): Wrap = io { Wrap.parse(call("stats", JSONObject().put("name", "wrapped").put("value", year))) }

    suspend fun downloads(): Downloads = io { Downloads.parse(call("download.status")) }

    /** The themes with their colors (G17); a theme file added later shows after a reconnect. */
    suspend fun themes(): List<ThemeInfo> = cached("themes", Long.MAX_VALUE) { io { ThemeInfo.list(call("theme", JSONObject().put("name", "list"))) } }

    /** Every visualizer mode of the core, None last. */
    suspend fun visualizers(): List<String> = cached("vis:list", Long.MAX_VALUE) {
        io {
            val items = call("vis", JSONObject().put("name", "list")).optJSONArray("items") ?: JSONArray()
            List(items.length()) { items.getString(it) }
        }
    }

    /** Turns shuffle on, then plays: [⤨] on a page (§5.3 Album). */
    fun shuffled(play: () -> Unit) {
        scope.launch {
            report { call("shuffle", JSONObject().put("name", "on")) }
            play()
        }
    }

    /** Me interesa (1) or No me interesa (-1) for item (G11); -1 drops it from the lists on screen at once (§2.6). */
    fun rate(item: Item, value: Int) {
        val kind = item.kind.name.lowercase()
        val params = JSONObject().put("kind", kind).put("name", item.title).put("value", value)
        val track = item.track
        if (track != null) params.put("track", track.json()) else params.put("playlist", item.id)
        if (value < 0) _hidden.update { it + (track?.path ?: item.id) }
        scope.launch {
            report { call("taste.rate", params) }
            invalidate()
        }
    }

    /** The search rows by the core's sections, parsed off the main thread (§7.2). */
    suspend fun search(query: String): List<Section> = cached("search:$query", 120_000) {
        withContext(Dispatchers.IO) {
            searchSections(Track.list(call("provider.search", JSONObject().put("provider", "ytmusic").put("query", query).put("limit", 40))))
        }
    }

    /** Toggles the ♥ of track (G20). The result's state reaches every row of its path; cached lists are stale after it (§5.3). */
    fun favorite(track: Track) {
        scope.launch {
            report {
                val result = call("playlist.bookmark", yt().put("track", track.json()))
                val on = if (result.has("favorite")) result.getBoolean("favorite") else !(_liked.value[track.path] ?: track.favorite)
                _liked.update { it + (track.path to on) }
                invalidate()
            }
        }
    }

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
     * The core's visualizer frames on a cols x rows grid, one every periodMs
     * at most, over one connection while collected (§5.4); one request at a
     * time, so a slow core drops frames instead of queueing them. Reconnects
     * after 500 ms on an error.
     */
    fun visFrames(cols: Int, rows: Int, periodMs: Long): Flow<VisFrame> = flow {
        while (true) {
            try {
                FrameStream(Core.socket(app)).use { stream ->
                    while (true) {
                        val start = SystemClock.elapsedRealtime()
                        emit(stream.next(cols, rows))
                        delay(periodMs - (SystemClock.elapsedRealtime() - start))
                    }
                }
            } catch (e: IOException) {
                delay(500)
            } catch (e: JSONException) {
                delay(500)
            }
        }
    }.conflate().flowOn(Dispatchers.IO)

    /** The strip's tap (§2.3): the next mode of `vis list`, None left out. */
    fun nextVisualizer(current: String) {
        scope.launch { report { call("vis", JSONObject().put("name", io.github.xaviiromf.cliampx.nextVisualizer(current, visualizers()))) } }
    }

    /** The value cached under key however old, so a screen that comes back shows it at once (§7.3). */
    @Suppress("UNCHECKED_CAST")
    fun <T> peek(key: String): T? = synchronized(cache) { cache[key]?.second as T? }

    @Suppress("UNCHECKED_CAST")
    suspend fun <T> cached(key: String, ttlMs: Long, load: suspend () -> T): T {
        val now = SystemClock.elapsedRealtime()
        synchronized(cache) { cache[key]?.takeIf { now - it.first < ttlMs } }?.let { return it.second as T }
        return load().also { synchronized(cache) { cache[key] = now to it } }
    }

    fun invalidate() = synchronized(cache) { cache.clear() }

    /** Drops the value cached under key, so its next read loads it again. */
    fun forget(key: String) = synchronized(cache) { cache.remove(key) }

    /** Todo el teléfono's pull: the folders of the phone's music go to the core again. */
    suspend fun syncMusicFolders() = MusicFolders.syncPhone(app)

    /** Clears the cache and makes the screens on show load again. */
    fun reload() {
        invalidate()
        _reloads.update { it + 1 }
    }

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
