package io.github.xaviiromf.cliampx

import android.os.Bundle
import android.util.Log
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import java.io.File

/**
 * Debug builds only. Runs the download, SHA-256 check and installer of the
 * updater on an APK pushed next to the app, since a test cannot publish a
 * GitHub release:
 *
 *     adb push app-debug.apk /sdcard/Android/data/io.github.xaviiromf.cliampx/files/update.apk
 *     adb shell am start -n io.github.xaviiromf.cliampx/.DebugUpdateActivity --es sha256 <sha256sum of the APK>
 */
class DebugUpdateActivity : AppCompatActivity() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val source = File(getExternalFilesDir(null), "update.apk")
        val apk = Updates.Apk("cliamp-x-debug.apk", source.toURI().toString(), intent.getStringExtra("sha256").orEmpty())
        scope.launch {
            runCatching { Updates.download(File(cacheDir, "updates"), apk) }
                .onSuccess { toast(if (Updates.install(this@DebugUpdateActivity, it)) "installer opened for $it" else getString(R.string.update_allow)) }
                .onFailure { toast(Updates.message(this@DebugUpdateActivity, it)) }
            finish()
        }
    }

    // Also in logcat, as screenshots miss toasts: adb logcat -s DebugUpdate
    private fun toast(text: String) {
        Log.i("DebugUpdate", text)
        Toast.makeText(applicationContext, text, Toast.LENGTH_LONG).show()
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }
}
