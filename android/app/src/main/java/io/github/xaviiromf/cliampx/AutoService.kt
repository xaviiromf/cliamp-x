@file:Suppress("DEPRECATION") // androidx.media points to Media3; PLAN.md picks it as less code

package io.github.xaviiromf.cliampx

import android.content.ComponentName
import android.content.Intent
import android.content.ServiceConnection
import android.os.Bundle
import android.os.IBinder
import android.support.v4.media.MediaBrowserCompat
import androidx.media.MediaBrowserServiceCompat

/**
 * The exported media browser for Android Auto and the system media controls
 * (onGetRoot lets only them in). It binds CliampService and lends its media
 * session and browse tree, so CliampService, whose start commands quit,
 * restart and open links, stays private to the app.
 */
class AutoService : MediaBrowserServiceCompat(), ServiceConnection {
    private var core: CliampService? = null

    override fun onCreate() {
        super.onCreate()
        bindService(Intent(this, CliampService::class.java), this, BIND_AUTO_CREATE)
    }

    // Any app may start it: a start does nothing, and the service lives only while a browser binds it.
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int) = START_NOT_STICKY.also { stopSelf(startId) }

    override fun onDestroy() {
        unbindService(this)
        super.onDestroy()
    }

    // A browser connects once the token is set, so the tree calls below find the core bound.
    override fun onServiceConnected(name: ComponentName, binder: IBinder) {
        val service = (binder as CliampService.LocalBinder).service
        core = service
        if (sessionToken == null) sessionToken = service.sessionToken
    }

    override fun onServiceDisconnected(name: ComponentName) {
        core = null
    }

    override fun onGetRoot(clientPackageName: String, clientUid: Int, rootHints: Bundle?): BrowserRoot? =
        if (Auto.trusted(clientPackageName, clientUid, packageName)) BrowserRoot(Auto.ROOT, null) else null

    override fun onLoadChildren(parentId: String, result: Result<MutableList<MediaBrowserCompat.MediaItem>>) {
        val service = core ?: return result.sendResult(mutableListOf())
        service.browse(parentId, result)
    }

    override fun onSearch(query: String, extras: Bundle?, result: Result<MutableList<MediaBrowserCompat.MediaItem>>) {
        val service = core ?: return result.sendResult(mutableListOf())
        service.search(query, result)
    }
}
