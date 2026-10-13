package io.github.xaviiromf.cliampx

import android.content.ComponentName
import android.content.Intent
import android.content.ServiceConnection
import android.content.res.Configuration
import android.graphics.Color
import android.graphics.Paint
import android.graphics.Typeface
import android.os.Bundle
import android.os.IBinder
import android.util.Log
import android.util.TypedValue
import android.view.Gravity
import android.view.KeyEvent
import android.view.MotionEvent
import android.view.View
import android.view.inputmethod.InputMethodManager
import android.widget.LinearLayout
import android.widget.TextView
import androidx.activity.addCallback
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import androidx.core.content.edit
import androidx.core.view.ViewCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import com.termux.terminal.KeyHandler
import com.termux.terminal.TerminalSession
import com.termux.view.TerminalView
import com.termux.view.TerminalViewClient

/**
 * Modo terminal: the core's TUI with two rows of extra keys. The service owns
 * the core; the core draws only while this is on screen.
 */
class TerminalActivity : AppCompatActivity(), ServiceConnection {
    private lateinit var terminal: TerminalView
    private lateinit var keys: LinearLayout
    private lateinit var ctrlKey: TextView
    private var service: CliampService? = null
    private var ctrl = false
    private var fontSize = 0
    private val prefs by lazy { getSharedPreferences("ui", MODE_PRIVATE) }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        WindowCompat.setDecorFitsSystemWindows(window, false)
        fontSize = prefs.getInt(FONT_SIZE, 0).takeIf { it > 0 } ?: defaultFontSize()
        terminal = TerminalView(this, null).apply {
            setTerminalViewClient(viewClient)
            setTextSize(fontSize)
            isFocusableInTouchMode = true
        }
        // Equal weights split the width in landscape; in portrait each row is full width.
        fun row() = LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
        keys = LinearLayout(this).apply {
            addView(keyRow("ESC" to { key(KeyEvent.KEYCODE_ESCAPE) }, "/" to { char('/') }, "F1" to { key(KeyEvent.KEYCODE_F1) },
                "F5" to { key(KeyEvent.KEYCODE_F5) }, "↑" to { key(KeyEvent.KEYCODE_DPAD_UP) }, "⌨" to ::toggleKeyboard,
                "⋮" to { startActivity(Intent(this@TerminalActivity, SettingsActivity::class.java)) }), row())
            addView(keyRow("TAB" to { key(KeyEvent.KEYCODE_TAB) }, "CTRL" to ::toggleCtrl, "SPC" to { char(' ') },
                "←" to { key(KeyEvent.KEYCODE_DPAD_LEFT) }, "↓" to { key(KeyEvent.KEYCODE_DPAD_DOWN) },
                "→" to { key(KeyEvent.KEYCODE_DPAD_RIGHT) }, "↵" to { key(KeyEvent.KEYCODE_ENTER) }), row())
        }
        ctrlKey = (keys.getChildAt(1) as LinearLayout).getChildAt(1) as TextView
        layoutKeys()
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setBackgroundColor(Color.BLACK)
            addView(terminal, LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, 0, 1f))
            addView(keys)
        }
        padForSystemBars(root, ime = true)
        setContentView(root)
        terminal.requestFocus()
        onBackPressedDispatcher.addCallback(this) { back() }
        bindService(Intent(this, CliampService::class.java), this, BIND_AUTO_CREATE)
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        layoutKeys()
    }

    /** Two rows of keys in portrait, one in landscape, where rows are scarce. */
    private fun layoutKeys() {
        keys.orientation = if (resources.configuration.orientation == Configuration.ORIENTATION_LANDSCAPE) LinearLayout.HORIZONTAL else LinearLayout.VERTICAL
    }

    override fun onStart() {
        super.onStart()
        Remote.terminalVisible(true)
        terminal.onScreenUpdated()
    }

    override fun onStop() {
        Remote.terminalVisible(false)
        super.onStop()
    }

    override fun onDestroy() {
        if (service?.terminal === this) service?.terminal = null
        unbindService(this)
        super.onDestroy()
    }

    override fun onServiceConnected(name: ComponentName, binder: IBinder) {
        val s = (binder as CliampService.LocalBinder).service
        service = s
        s.terminal = this
        s.session?.let(::attach)
    }

    override fun onServiceDisconnected(name: ComponentName) {
        service = null
    }

    fun attach(session: TerminalSession) {
        terminal.attachSession(session)
        terminal.onScreenUpdated()
    }

    fun onScreenUpdated(session: TerminalSession) {
        if (terminal.currentSession === session) terminal.onScreenUpdated()
    }

    private fun startCore() {
        ContextCompat.startForegroundService(this, Intent(this, CliampService::class.java))
    }

    /** Back is Esc, except on the queue with nothing open, where it returns to the native UI. */
    private fun back() {
        val session = terminal.currentSession
        val emulator = session?.emulator
        // shortcut: reads the screen name off the title row. The play-next overlay is also
        // titled Queue, so back there leaves the app; ask the core over IPC if that bothers.
        val title = emulator?.screen?.getSelectedText(0, 0, emulator.mColumns, emulator.mRows)
            ?.lineSequence()?.firstOrNull { "C L I A M P" in it }.orEmpty()
        if (emulator == null || !session.isRunning || "[Cola]" in title || "[Queue]" in title) finish()
        else key(KeyEvent.KEYCODE_ESCAPE)
    }

    private fun key(code: Int) {
        val session = terminal.currentSession ?: return
        if (!session.isRunning && code == KeyEvent.KEYCODE_ENTER) return startCore()
        if (session.emulator == null) return
        terminal.handleKeyCode(code, if (consumeCtrl()) KeyHandler.KEYMOD_CTRL else 0)
    }

    private fun char(c: Char) {
        if (terminal.currentSession?.emulator == null) return
        terminal.inputCodePoint(c.code, consumeCtrl(), false)
    }

    private fun toggleCtrl() {
        ctrl = !ctrl
        ctrlKey.setTextColor(if (ctrl) ACCENT else Color.WHITE)
    }

    private fun consumeCtrl(): Boolean = ctrl.also { if (it) toggleCtrl() }

    private fun toggleKeyboard() {
        val imm = getSystemService(InputMethodManager::class.java)
        if (ViewCompat.getRootWindowInsets(terminal)?.isVisible(WindowInsetsCompat.Type.ime()) == true) {
            imm.hideSoftInputFromWindow(terminal.windowToken, 0)
        } else {
            terminal.requestFocus()
            imm.showSoftInput(terminal, 0)
        }
    }

    private fun keyRow(vararg keys: Pair<String, () -> Unit>) = LinearLayout(this).apply {
        val ripple = TypedValue().also { theme.resolveAttribute(android.R.attr.selectableItemBackground, it, true) }.resourceId
        for ((label, action) in keys) {
            addView(TextView(context).apply {
                text = label
                gravity = Gravity.CENTER
                setTextColor(Color.WHITE)
                setTextSize(TypedValue.COMPLEX_UNIT_SP, 15f)
                minHeight = (44 * resources.displayMetrics.density).toInt()
                setBackgroundResource(ripple)
                setOnClickListener { action() }
            }, LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f))
        }
    }

    /** The largest font, in pixels, that still gives 60 columns across the short side of the screen. */
    private fun defaultFontSize(): Int {
        val width = resources.displayMetrics.run { minOf(widthPixels, heightPixels) }
        val paint = Paint().apply { typeface = Typeface.MONOSPACE }
        return (64 downTo MIN_FONT).firstOrNull { paint.textSize = it.toFloat(); width / paint.measureText("X") >= 60 } ?: MIN_FONT
    }

    private val viewClient = object : TerminalViewClient {
        override fun onScale(scale: Float): Float {
            if (scale in 0.9f..1.1f) return scale
            fontSize = (fontSize + if (scale > 1f) 2 else -2).coerceIn(MIN_FONT, 64)
            terminal.setTextSize(fontSize)
            prefs.edit { putInt(FONT_SIZE, fontSize) }
            return 1f
        }

        override fun onSingleTapUp(e: MotionEvent) = toggleKeyboard()

        override fun onKeyDown(keyCode: Int, e: KeyEvent, session: TerminalSession): Boolean {
            // "[Process completed - press Enter]": Enter starts the core again.
            if (keyCode == KeyEvent.KEYCODE_ENTER && !session.isRunning) {
                startCore()
                return true
            }
            return false
        }

        override fun readControlKey() = consumeCtrl()
        override fun shouldBackButtonBeMappedToEscape() = false
        // Samsung's keyboard keeps stale state with TYPE_NULL (termux-app#686).
        override fun shouldEnforceCharBasedInput() = true
        override fun shouldUseCtrlSpaceWorkaround() = false
        override fun isTerminalViewSelected() = true
        override fun copyModeChanged(copyMode: Boolean) {}
        override fun onKeyUp(keyCode: Int, e: KeyEvent) = false
        override fun onLongPress(event: MotionEvent) = false
        override fun readAltKey() = false
        override fun readShiftKey() = false
        override fun readFnKey() = false
        override fun onCodePoint(codePoint: Int, ctrlDown: Boolean, session: TerminalSession) = false
        override fun onEmulatorSet() {}
        override fun logError(tag: String?, message: String?) { Log.e(tag, message.orEmpty()) }
        override fun logWarn(tag: String?, message: String?) { Log.w(tag, message.orEmpty()) }
        override fun logInfo(tag: String?, message: String?) { Log.i(tag, message.orEmpty()) }
        override fun logDebug(tag: String?, message: String?) { Log.d(tag, message.orEmpty()) }
        override fun logVerbose(tag: String?, message: String?) { Log.v(tag, message.orEmpty()) }
        override fun logStackTraceWithMessage(tag: String?, message: String?, e: Exception?) { Log.e(tag, message, e) }
        override fun logStackTrace(tag: String?, e: Exception?) { Log.e(tag, "", e) }
    }

    companion object {
        private const val FONT_SIZE = "font_size"
        private const val MIN_FONT = 8
        private val ACCENT = Color.rgb(0x7a, 0xa2, 0xf7)
    }
}

/** Pads view by the system bars and the display cutout, and by the keyboard when ime is set (edge-to-edge, targetSdk 35). */
fun padForSystemBars(view: View, ime: Boolean = false) {
    val types = WindowInsetsCompat.Type.systemBars() or WindowInsetsCompat.Type.displayCutout() or
        (if (ime) WindowInsetsCompat.Type.ime() else 0)
    ViewCompat.setOnApplyWindowInsetsListener(view) { v, insets ->
        val i = insets.getInsets(types)
        v.setPadding(i.left, i.top, i.right, i.bottom)
        WindowInsetsCompat.CONSUMED
    }
}
