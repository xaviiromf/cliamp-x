package io.github.xaviiromf.cliampx

import android.os.Bundle
import android.service.dreams.DreamService
import android.view.WindowManager
import androidx.appcompat.app.AppCompatActivity
import androidx.activity.compose.setContent
import androidx.compose.ui.platform.ComposeView
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.LifecycleRegistry
import androidx.lifecycle.setViewTreeLifecycleOwner
import androidx.savedstate.SavedStateRegistryController
import androidx.savedstate.SavedStateRegistryOwner
import androidx.savedstate.setViewTreeSavedStateRegistryOwner
import io.github.xaviiromf.cliampx.ui.LiveTheme
import io.github.xaviiromf.cliampx.ui.Look
import io.github.xaviiromf.cliampx.ui.Looks
import io.github.xaviiromf.cliampx.ui.StereoScreen

/** Modo estéreo from the player's ⋮: full screen, kept on, over the lock screen. */
class StereoActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setShowWhenLocked(true)
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        WindowCompat.setDecorFitsSystemWindows(window, false)
        WindowCompat.getInsetsController(window, window.decorView).apply {
            hide(WindowInsetsCompat.Type.systemBars())
            systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        }
        Look.load(this)
        Looks.load(this)
        setContent {
            LiveTheme {
                StereoScreen(::finish) { night ->
                    window.attributes = window.attributes.apply {
                        screenBrightness = if (night) NIGHT_BRIGHTNESS else WindowManager.LayoutParams.BRIGHTNESS_OVERRIDE_NONE
                    }
                }
            }
        }
    }

    private companion object {
        const val NIGHT_BRIGHTNESS = 0.05f
    }
}

/**
 * Modo estéreo while charging: the system's screen saver (Ajustes →
 * Pantalla → Protector de pantalla → cliamp-x) starts it when a charging
 * screen would time out. A touch ends it.
 */
class StereoDream : DreamService(), LifecycleOwner, SavedStateRegistryOwner {
    private val registry = LifecycleRegistry(this)
    private val saved = SavedStateRegistryController.create(this)
    override val lifecycle: Lifecycle get() = registry
    override val savedStateRegistry get() = saved.savedStateRegistry

    override fun onCreate() {
        super.onCreate()
        saved.performRestore(null)
        registry.currentState = Lifecycle.State.CREATED
    }

    override fun onAttachedToWindow() {
        super.onAttachedToWindow()
        isFullscreen = true
        Look.load(this)
        Looks.load(this)
        val view = ComposeView(this).apply {
            setViewTreeLifecycleOwner(this@StereoDream)
            setViewTreeSavedStateRegistryOwner(this@StereoDream)
            setContent { LiveTheme { StereoScreen(::finish) { night -> isScreenBright = !night } } }
        }
        setContentView(view)
    }

    override fun onDreamingStarted() {
        super.onDreamingStarted()
        registry.currentState = Lifecycle.State.RESUMED
    }

    override fun onDreamingStopped() {
        registry.currentState = Lifecycle.State.CREATED
        super.onDreamingStopped()
    }

    override fun onDestroy() {
        registry.currentState = Lifecycle.State.DESTROYED
        super.onDestroy()
    }
}
