package io.github.xaviiromf.cliampx

import android.content.Context
import android.content.SharedPreferences
import android.content.pm.PackageManager
import android.provider.MediaStore
import android.util.Log
import androidx.core.content.ContextCompat
import androidx.core.content.edit
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.io.File

/**
 * The local music folders the core indexes (settings.get and settings.set
 * music_dirs), and Todo el teléfono: while it is on, the folders of every
 * song MediaStore knows go to the core at each start and pull to refresh.
 */
object MusicFolders {
    const val PHONE = "phone_wide"
    private const val OWN = "own_dirs"

    fun prefs(ctx: Context): SharedPreferences = ctx.getSharedPreferences("music", Context.MODE_PRIVATE)

    private val _list = MutableStateFlow<List<String>>(emptyList())
    val list: StateFlow<List<String>> = _list.asStateFlow()

    /** The folder of a MediaStore file path, or null for a path with no folder. */
    fun folderOf(path: String): String? = path.substringBeforeLast('/', "").takeIf { it.startsWith("/") }

    /** Each folder once and sorted, without the folders inside another one: the core reads those through it. */
    fun distinct(folders: List<String>): List<String> {
        val all = folders.map { it.trimEnd('/') }.filter { it.isNotEmpty() }.toSortedSet()
        return all.filter { f -> all.none { f.startsWith("$it/") } }
    }

    /** The folders of every song MediaStore lists on the phone's storage and SD cards. */
    private fun phoneFolders(ctx: Context): List<String> {
        @Suppress("DEPRECATION") // DATA is still the file path, and the core reads files by path
        val data = MediaStore.Audio.Media.DATA
        val paths = mutableListOf<String>()
        ctx.contentResolver.query(MediaStore.Audio.Media.EXTERNAL_CONTENT_URI, arrayOf(data), "${MediaStore.Audio.Media.IS_MUSIC} != 0", null, null)?.use { c ->
            while (c.moveToNext()) c.getString(0)?.let { paths += it }
        }
        return distinct(paths.mapNotNull(::folderOf)).filter { File(it).isDirectory }
    }

    private fun items(result: JSONObject): List<String> =
        result.optJSONArray("items")?.let { a -> (0 until a.length()).map(a::getString) }.orEmpty()

    /** Reads the core's folders. An older core without settings.get keeps the list empty. */
    suspend fun load() {
        try {
            _list.value = items(Remote.call("settings.get", JSONObject().put("name", "music_dirs")))
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            Log.w(TAG, "music folders: $e")
        }
    }

    /** Changes the folders with one settings.set name (music_dir_add, music_dir_remove, music_dirs), keeps the list the core returns and loads the screens again. Throws IpcError. */
    suspend fun set(name: String, text: String? = null, args: List<String>? = null) {
        val params = JSONObject().put("name", name)
        text?.let { params.put("text", it) }
        args?.let { params.put("args", JSONArray(it)) }
        _list.value = items(Remote.call("settings.set", params))
        Remote.reload()
    }

    /** Turns Todo el teléfono on or off; off gives back the folders the user had before it was on. */
    suspend fun setPhone(ctx: Context, on: Boolean) {
        val p = prefs(ctx)
        if (on && !p.getBoolean(PHONE, false)) p.edit { putString(OWN, JSONArray(_list.value).toString()) }
        p.edit { putBoolean(PHONE, on) }
        if (on) return syncPhone(ctx)
        val own = p.getString(OWN, null) ?: return
        try {
            set("music_dirs", args = JSONArray(own).let { a -> (0 until a.length()).map(a::getString) })
            p.edit { remove(OWN) }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            Log.w(TAG, "own music folders: $e")
        }
    }

    /** Todo el teléfono: while it is on and the app may read audio, the core indexes the folders of every song on the phone. */
    suspend fun syncPhone(ctx: Context) {
        if (!prefs(ctx).getBoolean(PHONE, false)) return
        if (ContextCompat.checkSelfPermission(ctx, MainActivity.musicPermission()) != PackageManager.PERMISSION_GRANTED) return
        try {
            set("music_dirs", args = withContext(Dispatchers.IO) { phoneFolders(ctx) })
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            Log.w(TAG, "phone music folders: $e") // an older core without music_dirs
        }
    }

    private const val TAG = "MusicFolders"
}
