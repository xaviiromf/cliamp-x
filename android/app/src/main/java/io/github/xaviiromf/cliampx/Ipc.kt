package io.github.xaviiromf.cliampx

import android.net.LocalSocket
import android.net.LocalSocketAddress
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.io.IOException

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

    /** Blocks, calling onEvent with each event line, until the stream closes. */
    fun subscribe(topics: List<String>, onEvent: (JSONObject) -> Unit) {
        connect().use { s ->
            s.soTimeout = 0 // events arrive only when the state changes
            s.outputStream.write((JSONObject().put("version", 2).put("id", "events").put("method", "subscribe").put("topics", JSONArray(topics)).toString() + "\n").toByteArray())
            val reader = s.inputStream.bufferedReader()
            val ack = JSONObject(reader.readLine() ?: throw IOException("ipc: subscribe closed"))
            if (!ack.optBoolean("ok")) throw IOException("ipc: subscribe: $ack")
            while (true) onEvent(JSONObject(reader.readLine() ?: return))
        }
    }
}
