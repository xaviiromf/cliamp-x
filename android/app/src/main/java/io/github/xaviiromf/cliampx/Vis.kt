package io.github.xaviiromf.cliampx

import kotlin.math.PI
import kotlin.math.cos
import kotlin.math.exp
import kotlin.math.max
import kotlin.math.min
import kotlin.math.sin
import kotlin.random.Random

/**
 * The core modes the native UI draws (UI-SPEC §0), in picker and strip
 * order, None last. Any other mode draws as Bars.
 */
val NATIVE_VIS = listOf(
    "Bars", "ClassicPeak", "ClassicLED", "Bricks", "Columns", "BarsDot", "BarsOutline", "Mirror", "Butterfly",
    "Ascii", "Mosaic", "Logo", "Spectrogram", "Terrain", "Skyline", "Radial", "Pulse", "Ripple", "Tunnel", "Spiral",
    "Helix", "Plasma", "Aurora", "Retro", "Flame", "Rain", "Matrix", "Scatter", "Geyser", "Firework", "Bubbles",
    "Sakura", "Firefly", "Starfield", "None",
)

/** The native modes that move on their own (scroll, spin, particles): reduced motion draws them as Bars. */
private val MOVING_VIS = setOf(
    "Mosaic", "Spectrogram", "Terrain", "Radial", "Ripple", "Tunnel", "Spiral", "Helix", "Plasma", "Aurora", "Retro",
    "Flame", "Rain", "Matrix", "Scatter", "Geyser", "Firework", "Bubbles", "Sakura", "Firefly", "Starfield",
)

/** The renderer that draws the core's mode: its own when native and allowed, else Bars. */
fun nativeVis(mode: String, reduced: Boolean): String = when {
    mode == "None" -> "None"
    mode !in NATIVE_VIS -> "Bars"
    reduced && mode in MOVING_VIS -> "Bars"
    else -> mode
}

/** The next native visualizer of the strip's cycle (§2.3), None left out. */
fun nextVisualizer(current: String): String {
    val i = NATIVE_VIS.indexOf(current)
    val next = if (i < 0 || i >= NATIVE_VIS.size - 2) 0 else i + 1 // the last before None, None and unknown modes go to Bars
    return NATIVE_VIS[next]
}

/** src linearly resampled into dst, with no allocation; an empty src clears dst. */
fun resampleInto(src: FloatArray, dst: FloatArray) {
    if (src.isEmpty()) return dst.fill(0f)
    if (src.size == 1 || dst.size == 1) return dst.fill(src[0])
    for (i in dst.indices) {
        val x = i * (src.size - 1f) / (dst.size - 1)
        val lo = x.toInt().coerceAtMost(src.size - 2)
        val f = x - lo
        dst[i] = (src[lo] * (1 - f) + src[lo + 1] * f).coerceIn(0f, 1f)
    }
}

/** The level a at x in 0..1, linearly interpolated. */
fun sampleAt(a: FloatArray, x: Float): Float {
    val p = x.coerceIn(0f, 1f) * (a.size - 1)
    val lo = p.toInt().coerceAtMost(a.size - 2)
    val f = p - lo
    return a[lo] * (1 - f) + a[lo + 1] * f
}

/** One level eased toward its target over dt seconds: fast up, slower down, as the core's smoothing. */
fun ease(level: Float, target: Float, dt: Float): Float {
    val rate = if (target > level) ATTACK else DECAY
    return level + (target - level) * (1 - exp(-rate * dt))
}

/** A hash of three ints to 0..1, for flicker and glyphs that need no stored state. */
fun hash01(a: Int, b: Int, c: Int): Float {
    var h = a * 374761393 + b * 668265263 + c * 2147483647
    h = (h xor (h ushr 13)) * 1274126177
    return ((h xor (h ushr 16)) and 0xFFFFFF) / 16777215f
}

private const val ATTACK = 30f
private const val DECAY = 10f
private const val PEAK_HOLD = 0.3f
private const val PEAK_GRAVITY = 3f
private const val HIST_RATE = 24f

/**
 * The animation state of one native visualizer: the eased bands, the peak
 * caps, the bass and beat, the scrolling history and the particles. Every
 * buffer is made once, so a frame allocates nothing. Positions are 0..1 of
 * the canvas.
 */
class VisState(private val rnd: Random = Random(7)) {
    val target = FloatArray(BINS)
    val level = FloatArray(BINS)
    val peak = FloatArray(BINS)
    private val hold = FloatArray(BINS)
    private val fall = FloatArray(BINS)
    var time = 0f; private set
    var spin = 0f; private set
    var bass = 0f; private set
    var energy = 0f; private set
    var beats = 0; private set
    private var bassSlow = 0f
    private var sinceBeat = 10f
    private var mode = ""

    /** HIST columns of HROWS levels, low bands first; histHead is the oldest column. */
    val hist = Array(HIST) { FloatArray(HROWS) }
    var histHead = 0; private set
    /** How far the next column is toward its push, 0..1, for a smooth scroll. */
    var histPhase = 0f; private set

    val px = FloatArray(MAXP); val py = FloatArray(MAXP)
    val vx = FloatArray(MAXP); val vy = FloatArray(MAXP)
    val life = FloatArray(MAXP); val ttl = FloatArray(MAXP)
    val size = FloatArray(MAXP); val kind = IntArray(MAXP)
    var count = 0; private set

    val sx = FloatArray(STARS); val sy = FloatArray(STARS); val sz = FloatArray(STARS)
    var speed = 0f; private set
    val heads = FloatArray(COLS)
    val ringX = FloatArray(RINGS); val ringY = FloatArray(RINGS); val ringAge = FloatArray(RINGS) { 99f }
    private var ringNext = 0

    /** The core's bands as the new targets. */
    fun set(bands: FloatArray) = resampleInto(bands, target)

    fun at(x: Float) = sampleAt(level, x)
    fun peakAt(x: Float) = sampleAt(peak, x)

    /** Advances dt seconds in mode; snap jumps to the targets (reduced motion). */
    fun step(dt: Float, mode: String, snap: Boolean = false) {
        if (mode != this.mode) reset(mode)
        val d = min(dt, 0.1f)
        for (i in 0 until BINS) {
            level[i] = if (snap) target[i] else ease(level[i], target[i], d)
            if (level[i] >= peak[i]) {
                peak[i] = level[i]; hold[i] = PEAK_HOLD; fall[i] = 0f
            } else if (hold[i] > 0f) {
                hold[i] -= d
            } else {
                fall[i] += PEAK_GRAVITY * d
                peak[i] = max(level[i], peak[i] - fall[i] * d)
            }
        }
        var b = 0f
        for (i in 0 until BINS / 8) b += level[i]
        bass = b / (BINS / 8)
        energy = level.average().toFloat()
        bassSlow += (bass - bassSlow) * (1 - exp(-2f * d))
        sinceBeat += d
        if (bass > bassSlow * 1.25f + 0.05f && sinceBeat > 0.25f) {
            beats++; sinceBeat = 0f; onBeat(mode)
        }
        time += d
        spin += d * (0.3f + energy * 2f)
        speed = 0.15f + energy * 1.6f
        histPhase += d * HIST_RATE
        while (histPhase >= 1f) {
            histPhase -= 1f
            resampleInto(level, hist[histHead])
            histHead = (histHead + 1) % HIST
        }
        particles(d, mode)
    }

    private fun reset(mode: String) {
        this.mode = mode
        count = 0
        for (i in 0 until STARS) newStar(i, rnd.nextFloat())
        for (i in 0 until COLS) heads[i] = -rnd.nextFloat()
        ringAge.fill(99f)
    }

    private fun newStar(i: Int, z: Float) {
        sx[i] = rnd.nextFloat() * 2 - 1; sy[i] = rnd.nextFloat() * 2 - 1; sz[i] = max(0.05f, z)
    }

    private fun onBeat(mode: String) {
        when (mode) {
            "Firework" -> burst()
            "Ripple" -> ring()
        }
    }

    private fun burst() {
        val cx = 0.2f + rnd.nextFloat() * 0.6f
        val cy = 0.15f + rnd.nextFloat() * 0.35f
        val color = rnd.nextInt(4)
        val n = 64
        for (k in 0 until n) {
            val a = (2 * PI * k / n).toFloat()
            val v = (0.3f + bass * 0.4f) * (0.4f + 0.6f * rnd.nextFloat())
            spawn(cx, cy, cos(a) * v, sin(a) * v, 1.6f, 1f, color)
        }
    }

    private fun ring() {
        ringX[ringNext] = 0.2f + rnd.nextFloat() * 0.6f
        ringY[ringNext] = 0.2f + rnd.nextFloat() * 0.6f
        ringAge[ringNext] = 0f
        ringNext = (ringNext + 1) % RINGS
    }

    private fun spawn(x: Float, y: Float, vx0: Float, vy0: Float, t: Float, s: Float, k: Int) {
        if (count == MAXP) return
        px[count] = x; py[count] = y; vx[count] = vx0; vy[count] = vy0
        life[count] = 0f; ttl[count] = t; size[count] = s; kind[count] = k
        count++
    }

    /** Spawns rate particles per second on average: whole ones, then the rest by chance. */
    private inline fun every(rate: Float, d: Float, make: () -> Unit) {
        val n = rate * d
        repeat(n.toInt()) { make() }
        if (rnd.nextFloat() < n - n.toInt()) make()
    }

    private fun particles(d: Float, mode: String) {
        var gravity = 0f
        when (mode) {
            "Rain" -> {
                gravity = 0.6f
                every(20f + 160f * energy, d) { spawn(rnd.nextFloat(), 0f, 0f, 0.8f + rnd.nextFloat() * 0.4f, 3f, 1f, 0) }
            }
            "Scatter" -> every(700f * energy, d) {
                val x = rnd.nextFloat()
                spawn(x, 1 - rnd.nextFloat() * at(x), 0f, 0f, 0.3f + rnd.nextFloat() * 0.3f, 1f, 0)
            }
            "Geyser" -> {
                gravity = 1.5f
                every(160f * (bass + 0.05f), d) {
                    spawn(0.5f + (rnd.nextFloat() - 0.5f) * 0.06f, 1f, (rnd.nextFloat() - 0.5f) * 0.9f, -(0.5f + bass * 1.8f + rnd.nextFloat() * 0.3f), 3f, 1f, 0)
                }
            }
            "Firework" -> {
                gravity = 0.35f
                if (sinceBeat > 0.9f && energy > 0.04f) { sinceBeat = 0f; burst() }
            }
            "Bubbles" -> every(2f + 30f * energy, d) {
                val x = rnd.nextFloat()
                spawn(x, 1.05f, 0f, -(0.12f + at(x) * 0.5f), 8f, 0.3f + at(x), rnd.nextInt(4))
            }
            "Sakura" -> every(10f + 60f * energy, d) {
                spawn(rnd.nextFloat() * 1.2f - 0.2f, -0.05f, 0.05f + energy * 0.1f, 0.08f + rnd.nextFloat() * 0.08f + energy * 0.25f, 12f, 0.6f + rnd.nextFloat() * 0.6f, rnd.nextInt(4))
            }
            "Firefly" -> {
                if (count < 36) spawn(rnd.nextFloat(), 0.2f + rnd.nextFloat() * 0.7f, 0f, 0f, 4f + rnd.nextFloat() * 4f, 1f, 0)
                for (i in 0 until count) {
                    vx[i] = (vx[i] + (rnd.nextFloat() - 0.5f) * d * 0.6f).coerceIn(-0.08f, 0.08f)
                    vy[i] = (vy[i] + (rnd.nextFloat() - 0.5f) * d * 0.6f).coerceIn(-0.06f, 0.06f)
                }
            }
            "Flame" -> every(50f * energy, d) {
                val x = rnd.nextFloat()
                spawn(x, 1 - at(x) * 0.9f, 0f, -(0.2f + rnd.nextFloat() * 0.3f), 0.6f, 1f, 0)
            }
            "Starfield" -> for (i in 0 until STARS) {
                sz[i] -= speed * d * 0.5f
                if (sz[i] < 0.05f) newStar(i, 1f)
            }
            "Matrix" -> for (c in 0 until COLS) {
                heads[c] += (0.25f + sampleAt(level, c / (COLS - 1f)) * 1.4f) * d
                if (heads[c] > 1.6f) heads[c] = -rnd.nextFloat() * 0.5f
            }
            "Ripple" -> {
                for (i in 0 until RINGS) ringAge[i] += d
                if (sinceBeat > 1.5f && energy > 0.04f) { sinceBeat = 0f; ring() }
            }
        }
        var i = 0
        while (i < count) {
            px[i] += vx[i] * d; py[i] += vy[i] * d; vy[i] += gravity * d; life[i] += d
            if (life[i] > ttl[i] || py[i] > 1.1f || py[i] < -0.3f) {
                count--
                px[i] = px[count]; py[i] = py[count]; vx[i] = vx[count]; vy[i] = vy[count]
                life[i] = life[count]; ttl[i] = ttl[count]; size[i] = size[count]; kind[i] = kind[count]
            } else {
                i++
            }
        }
    }

    companion object {
        const val BINS = 64
        const val HIST = 96
        const val HROWS = 24
        const val MAXP = 400
        const val STARS = 160
        const val COLS = 48
        const val RINGS = 8
    }
}
