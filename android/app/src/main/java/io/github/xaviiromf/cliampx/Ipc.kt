package io.github.xaviiromf.cliampx

import android.net.LocalSocket
import android.net.LocalSocketAddress
import org.json.JSONArray
import org.json.JSONObject
import java.io.BufferedReader
import java.io.Closeable
import java.io.File
import java.io.IOException

/** A failed V2 operation: the error code (docs/remote-control.md), its message and the diagnostic detail. */
class IpcError(val op: String, val code: String, message: String, val detail: String = "") : Exception("$op: $message") {
    companion object {
        fun of(op: String, error: JSONObject?) = IpcError(
            op,
            error?.optString("code").orEmpty().ifEmpty { "internal_error" },
            error?.optString("message").orEmpty(),
            error?.optString("detail").orEmpty(),
        )
    }
}

/** V2 IPC client of the core (docs/remote-control.md): one JSON object per line. */
class Ipc(private val socket: File) {
    private fun connect() = LocalSocket().apply {
        connect(LocalSocketAddress(socket.path, LocalSocketAddress.Namespace.FILESYSTEM))
        soTimeout = 5000
    }

    /** Sends one request and returns its response. Throws IOException when the core is not listening. */
    fun call(request: JSONObject): JSONObject = connect().use { s ->
        s.outputStream.write((request.put("version", 2).toString() + "\n").toByteArray())
        val line = s.inputStream.bufferedReader().readLine() ?: throw IOException("ipc: no response")
        JSONObject(line)
    }

    fun state(): JSONObject = call(JSONObject().put("id", "state").put("method", "state.get"))

    /** Starts an operation. The response holds the queued job, or the error. */
    fun submit(operation: String, params: JSONObject = JSONObject()): JSONObject =
        call(JSONObject().put("id", operation).put("method", "operation.submit").put("operation", operation).put("params", params))

    fun job(id: String): JSONObject = call(JSONObject().put("id", "job").put("method", "job.get").put("job_id", id))

    fun cancel(id: String): JSONObject = call(JSONObject().put("id", "cancel").put("method", "job.cancel").put("job_id", id))

    /** Opens an event stream. Closing it, from any thread, ends a blocked next(). */
    fun subscribe(topics: List<String>): Subscription {
        val s = connect()
        try {
            s.soTimeout = 0 // events arrive only when the state changes
            s.outputStream.write((JSONObject().put("version", 2).put("id", "events").put("method", "subscribe").put("topics", JSONArray(topics)).toString() + "\n").toByteArray())
            val reader = s.inputStream.bufferedReader()
            val ack = JSONObject(reader.readLine() ?: throw IOException("ipc: subscribe closed"))
            if (!ack.optBoolean("ok")) throw IOException("ipc: subscribe: $ack")
            return Subscription(s, reader)
        } catch (e: Exception) {
            s.close()
            throw e
        }
    }

    class Subscription(private val socket: LocalSocket, private val reader: BufferedReader) : Closeable {
        /** The next event, or null when the stream ended. */
        fun next(): JSONObject? = reader.readLine()?.let(::JSONObject)

        override fun close() {
            runCatching { socket.shutdownInput() } // wakes a read blocked in another thread
            socket.close()
        }
    }
}

/**
 * spectrum.get over one persistent connection (§5.1). Each next() asks for
 * one frame and returns its bands, 0..1. A read waits at most 2 s.
 */
class SpectrumStream(socket: File) : Closeable {
    private val s = LocalSocket().also {
        try {
            it.connect(LocalSocketAddress(socket.path, LocalSocketAddress.Namespace.FILESYSTEM))
            it.soTimeout = 2000
        } catch (e: IOException) {
            it.close()
            throw e
        }
    }
    private val reader = s.inputStream.bufferedReader()
    private val request = "{\"version\":2,\"id\":\"s\",\"method\":\"spectrum.get\"}\n".toByteArray()

    fun next(): FloatArray {
        s.outputStream.write(request)
        val line = reader.readLine() ?: throw IOException("ipc: spectrum closed")
        val bands = JSONObject(line).optJSONObject("result")?.optJSONArray("bands") ?: return FloatArray(0)
        return FloatArray(bands.length()) { bands.optDouble(it, 0.0).toFloat() }
    }

    override fun close() = s.close()
}
