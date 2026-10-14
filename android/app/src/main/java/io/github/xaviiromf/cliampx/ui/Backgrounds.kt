package io.github.xaviiromf.cliampx.ui

import android.content.Context
import android.content.SharedPreferences
import android.graphics.RuntimeShader
import android.os.Build
import android.os.SystemClock
import android.util.Log
import androidx.annotation.RequiresApi
import androidx.annotation.StringRes
import androidx.compose.animation.ContentTransform
import androidx.compose.animation.EnterTransition
import androidx.compose.animation.ExitTransition
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.FiniteAnimationSpec
import androidx.compose.animation.core.snap
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.FloatState
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.CacheDrawScope
import androidx.compose.ui.draw.DrawResult
import androidx.compose.ui.draw.drawWithCache
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.CompositingStrategy
import androidx.compose.ui.graphics.ShaderBrush
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.layout.positionInWindow
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.unit.IntSize
import androidx.core.content.edit
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.repeatOnLifecycle
import io.github.xaviiromf.cliampx.R
import kotlinx.coroutines.delay

/** The backgrounds drawn behind the whole app (Ajustes → Fondo). Solid is the theme's bg alone. */
enum class Background(@StringRes val label: Int) {
    Solid(R.string.bg_solid), Crt(R.string.bg_crt), Waves(R.string.bg_waves), Cyberpunk(R.string.bg_cyberpunk),
    Retro(R.string.bg_retro), Glitch(R.string.bg_glitch), Matrix(R.string.bg_matrix), Stars(R.string.bg_stars),
    Plasma(R.string.bg_plasma), Aurora(R.string.bg_aurora), Vhs(R.string.bg_vhs), Dither(R.string.bg_dither),
    Tron(R.string.bg_tron), Bokeh(R.string.bg_bokeh), Circuit(R.string.bg_circuit), Topo(R.string.bg_topo),
    Grain(R.string.bg_grain),
}

/** How fast the app moves: Slow stretches every transition, Off snaps them and holds the background still. */
enum class MotionSpeed(@StringRes val label: Int, val factor: Float) {
    Slow(R.string.motion_slow, 1.6f), Normal(R.string.motion_normal, 1f), Off(R.string.motion_off, 0f),
}

/** The background and motion choices, kept in the "ui" preferences and read live by the whole UI. */
object Looks {
    var background by mutableStateOf(Background.Solid)
        private set
    var listLevel by mutableIntStateOf(40)
        private set
    var playerLevel by mutableIntStateOf(70)
        private set
    var motion by mutableStateOf(MotionSpeed.Normal)
        private set

    /** The visualizer fills the player behind the artwork. */
    var playerVis by mutableStateOf(false)
        private set
    private var prefs: SharedPreferences? = null

    fun load(context: Context) {
        val p = context.applicationContext.getSharedPreferences("ui", Context.MODE_PRIVATE).also { prefs = it }
        background = enumOr(p.getString(BACKGROUND, null), Background.Solid)
        listLevel = p.getInt(LIST_LEVEL, 40).coerceIn(0, 100)
        playerLevel = p.getInt(PLAYER_LEVEL, 70).coerceIn(0, 100)
        motion = enumOr(p.getString(MOTION, null), MotionSpeed.Normal)
        playerVis = p.getBoolean(PLAYER_VIS, false)
    }

    fun putBackground(b: Background) {
        background = b
        prefs?.edit { putString(BACKGROUND, b.name) }
    }

    fun putListLevel(v: Int) {
        listLevel = v.coerceIn(0, 100)
        prefs?.edit { putInt(LIST_LEVEL, listLevel) }
    }

    fun putPlayerLevel(v: Int) {
        playerLevel = v.coerceIn(0, 100)
        prefs?.edit { putInt(PLAYER_LEVEL, playerLevel) }
    }

    fun putMotion(m: MotionSpeed) {
        motion = m
        prefs?.edit { putString(MOTION, m.name) }
    }

    fun putPlayerVis(on: Boolean) {
        playerVis = on
        prefs?.edit { putBoolean(PLAYER_VIS, on) }
    }

    private const val BACKGROUND = "background"
    private const val LIST_LEVEL = "background_lists"
    private const val PLAYER_LEVEL = "background_player"
    private const val MOTION = "motion"
    private const val PLAYER_VIS = "player_vis"
}

/** The entry named name, or fallback for a missing or unknown name. */
inline fun <reified E : Enum<E>> enumOr(name: String?, fallback: E): E = enumValues<E>().firstOrNull { it.name == name } ?: fallback

/** The scrim drawn over a background: its color, and how much (0..1) of the background shows through it. */
data class Scrim(val color: Color, val shown: Float)

/**
 * The scrim of b at level 0..100 under palette p. Level 100 shows the most
 * of the background that keeps fg and brightFg at their contrast on bg (at
 * most 4.5:1) even where a shader draws white (black under a light theme),
 * with the scrim leaned fully to black (white); a lower level shows less
 * and leans the scrim off bg only as far as that contrast needs.
 */
fun scrimOf(b: Background, level: Int, p: CliampPalette): Scrim {
    if (b == Background.Solid || level <= 0) return Scrim(p.bg, 0f)
    val darkTheme = p.fg.luminance() > p.bg.luminance()
    val lean = if (darkTheme) Color.Black else Color.White
    val worst = if (darkTheme) Color.White else Color.Black
    val texts = listOf(p.fg, p.brightFg).map { it to minOf(4.5f, contrast(it, p.bg)) }
    fun fits(shown: Float, leaned: Float): Boolean {
        val under = mixSrgb(mixSrgb(p.bg, lean, leaned), worst, shown)
        return texts.all { (t, want) -> contrast(t, under) >= want - 1e-4f }
    }
    // Both searches are monotonic: more shown only nears the worst color, more leaned only leaves it.
    var lo = 0f
    var hi = 1f
    repeat(16) { val m = (lo + hi) / 2; if (fits(m, 1f)) lo = m else hi = m }
    val shown = lo * level.coerceAtMost(100) / 100f
    if (fits(shown, 0f)) return Scrim(p.bg, shown)
    lo = 0f
    hi = 1f
    repeat(16) { val m = (lo + hi) / 2; if (fits(shown, m)) hi = m else lo = m }
    return Scrim(mixSrgb(p.bg, lean, hi), shown)
}

/** The WCAG contrast ratio of a and b, 1..21. */
fun contrast(a: Color, b: Color): Float {
    val x = a.luminance()
    val y = b.luminance()
    return (maxOf(x, y) + 0.05f) / (minOf(x, y) + 0.05f)
}

/** Eased motion (FastOutSlowIn) scaled by Ajustes → Animaciones. */
object Motion {
    fun <T> spec(ms: Int): FiniteAnimationSpec<T> = durationOf(ms, Looks.motion)?.let { tween(it, easing = FastOutSlowInEasing) } ?: snap()

    /** ms stretched by m, or null when m turns motion off. */
    fun durationOf(ms: Int, m: MotionSpeed): Int? = if (m == MotionSpeed.Off) null else (ms * m.factor).toInt()

    /** A pushed or popped screen, or a player page: fades in rising slightly as the old one fades out. */
    fun screen(ms: Int = 300): ContentTransform =
        if (Looks.motion == MotionSpeed.Off) EnterTransition.None togetherWith ExitTransition.None
        else (fadeIn(spec(ms)) + slideInVertically(spec(ms)) { it / 40 }) togetherWith fadeOut(spec(ms * 2 / 3))
}

/** The compiled shader of b, or null for Solid, below Android 13 or when it fails to compile (then a gradient draws). */
private fun shaderOf(b: Background): RuntimeShader? {
    if (Build.VERSION.SDK_INT < 33) return null
    val source = shaderSource(b) ?: return null
    return try {
        RuntimeShader(source)
    } catch (e: IllegalArgumentException) {
        Log.e("cliamp", "background ${b.name}: ${e.message}")
        null
    }
}

/** Seconds for the shaders, the same in every window so a sheet lines up with the app; rate 0 holds a still frame. */
private fun clock(rate: Float): Float =
    if (rate == 0f) 30f else ((SystemClock.uptimeMillis() / 1000.0 * rate) % 3600.0).toFloat()

/**
 * The time a background draws at: it steps at 20 fps while the screen is
 * started (never with the app hidden or the screen off), slower with the
 * system's reduced motion, and holds still when paused or motion is off.
 */
@Composable
private fun backdropTime(b: Background, paused: Boolean): FloatState {
    val reduced = reducedMotion()
    val rate = Looks.motion.factor * if (reduced) 0.25f else 1f
    val t = remember { mutableFloatStateOf(clock(rate)) }
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    LaunchedEffect(b, paused, rate) {
        t.floatValue = clock(rate)
        if (b == Background.Solid || paused || rate == 0f) return@LaunchedEffect
        lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) {
            // A plain write each step, no withFrameNanos: a frame request ran the Recomposer at every step.
            while (true) {
                t.floatValue = clock(rate)
                delay(STEP_MS)
            }
        }
    }
    return t
}

/**
 * The background at level (0..100) under its scrim (scrimOf),
 * filling modifier. It draws in its own layer, so a frame redraws only it.
 * paused holds it still while something opaque covers it.
 */
@Composable
fun Backdrop(level: Int, modifier: Modifier = Modifier, paused: Boolean = false) {
    val p = LocalPalette.current
    val b = Looks.background
    val scrim = remember(b, level, p) { scrimOf(b, level, p) }
    val shader = remember(b) { shaderOf(b) }
    val time = backdropTime(b, paused)
    val window = LocalWindowInfo.current.containerSize
    var origin by remember { mutableStateOf(Offset.Zero) }
    // The shader runs on a layer of 1/LOW_RES the size, scaled up: a full-resolution frame took 15-27 ms of GPU on the A55.
    Box(modifier.onGloballyPositioned { origin = it.positionInWindow() }) {
        Spacer(
            Modifier.fillMaxSize(1f / LOW_RES).graphicsLayer {
                scaleX = LOW_RES
                scaleY = LOW_RES
                transformOrigin = TransformOrigin(0f, 0f)
                compositingStrategy = CompositingStrategy.Offscreen
            }.drawWithCache {
                val cover = scrim.color.copy(alpha = 1f - scrim.shown)
                when {
                    scrim.shown == 0f -> onDrawBehind { drawRect(p.bg) }
                    shader == null || Build.VERSION.SDK_INT < 33 -> {
                        // Below Android 13: the palette as a still gradient.
                        val brush = Brush.verticalGradient(listOf(lerp(p.bg, p.accent, 0.35f), p.bg, lerp(p.bg, p.green, 0.25f)))
                        onDrawBehind {
                            drawRect(brush)
                            drawRect(cover)
                        }
                    }
                    else -> drawShader(shader, p, cover, window) { origin to time.floatValue }
                }
            },
        )
    }
}

/** shader with the palette, under the scrim; at, read at each draw, gives the view's origin and the time. */
@RequiresApi(33)
private fun CacheDrawScope.drawShader(shader: RuntimeShader, p: CliampPalette, scrim: Color, window: IntSize, at: () -> Pair<Offset, Float>): DrawResult {
    shader.setColorUniform("bg", p.bg.toArgb())
    shader.setColorUniform("accent", p.accent.toArgb())
    shader.setColorUniform("fg", p.fg.toArgb())
    shader.setColorUniform("green", p.green.toArgb())
    shader.setColorUniform("yellow", p.yellow.toArgb())
    shader.setColorUniform("red", p.red.toArgb())
    shader.setFloatUniform("dp", density / LOW_RES)
    shader.setFloatUniform("res", window.width.coerceAtLeast(1) / LOW_RES, window.height.coerceAtLeast(1) / LOW_RES)
    val brush = ShaderBrush(shader)
    return onDrawBehind {
        val (origin, t) = at()
        shader.setFloatUniform("origin", origin.x / LOW_RES, origin.y / LOW_RES)
        shader.setFloatUniform("t", t)
        drawRect(brush)
        drawRect(scrim)
    }
}

private const val LOW_RES = 2f

/** The background's step: 20 fps. Each step redraws the whole window, which costs the same CPU at any shader size. */
private const val STEP_MS = 50L
