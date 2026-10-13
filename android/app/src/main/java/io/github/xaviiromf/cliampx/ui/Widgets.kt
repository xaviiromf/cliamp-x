package io.github.xaviiromf.cliampx.ui

import android.os.SystemClock
import androidx.annotation.DrawableRes
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.spring
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.LongState
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.input.pointer.util.VelocityTracker
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.repeatOnLifecycle
import io.github.xaviiromf.cliampx.PlaybackClock
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Snapshot
import io.github.xaviiromf.cliampx.Track
import io.github.xaviiromf.cliampx.clock
import io.github.xaviiromf.cliampx.seekHolds
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import org.json.JSONObject
import kotlin.math.abs

object Glyphs {
    /** Text glyphs drawn in JetBrains Mono; GlyphTest checks each one is in the font. */
    const val ALL = "─▸▶◌✓✕✗░·…>_[]●‖■×"

    /** JetBrains Mono 2.304 has no braille: Android draws these with its fallback font. */
    const val SPINNER = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
}

/** The braille spinner frame, one every 100 ms. */
@Composable
fun spinner(): String {
    val frame by produceState(0) {
        while (true) {
            delay(100)
            value = (value + 1) % Glyphs.SPINNER.length
        }
    }
    return Glyphs.SPINNER[frame].toString()
}

/** `── Label ───…` across the width, with an optional action at its end. */
@Composable
fun SectionRule(label: String, action: @Composable (() -> Unit)? = null) {
    val p = LocalPalette.current
    Row(Modifier.fillMaxWidth().height(40.dp).padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) {
        Text("── $label " + "─".repeat(120), Modifier.weight(1f), color = p.textSecondary, style = Type.label, maxLines = 1, softWrap = false, overflow = TextOverflow.Clip)
        action?.invoke()
    }
}

/** The current screen's rule: `▸─ Label ──…` in headline accent. */
@Composable
fun ScreenRule(label: String) {
    val p = LocalPalette.current
    Text(
        "▸─ $label " + "─".repeat(80), Modifier.fillMaxWidth().height(56.dp).padding(horizontal = 16.dp).padding(top = 16.dp),
        color = p.accent, style = Type.headline, maxLines = 1, softWrap = false, overflow = TextOverflow.Clip,
    )
}

/** A secondary button: `[ Texto ]`, 48 dp tall. */
@Composable
fun BracketButton(text: String, modifier: Modifier = Modifier, color: Color = LocalPalette.current.accent, onClick: () -> Unit) {
    Box(modifier.heightIn(min = 48.dp).clickable(onClick = onClick).padding(horizontal = 4.dp), contentAlignment = Alignment.Center) {
        Text("[ $text ]", color = color, style = Type.body)
    }
}

/** A 48 dp icon button. */
@Composable
fun IconBtn(
    @DrawableRes icon: Int, description: String, modifier: Modifier = Modifier, tint: Color = LocalPalette.current.textSecondary,
    size: Dp = 48.dp, onClick: () -> Unit,
) {
    Box(modifier.size(size).clickable(onClick = onClick), contentAlignment = Alignment.Center) {
        Icon(painterResource(icon), description, Modifier.size(24.dp), tint = tint)
    }
}

/** A track's art at size, or the `░` placeholder with a note while it loads or when there is none. */
@Composable
fun ArtBox(key: String, size: Dp, modifier: Modifier = Modifier) {
    val p = LocalPalette.current
    val px = with(LocalDensity.current) { size.roundToPx() }
    val art = rememberArt(key, px)
    Box(modifier.size(size), contentAlignment = Alignment.Center) {
        if (art != null) {
            Image(art, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
        } else {
            val cells = ((size.value / 7).toInt() + 1) * ((size.value / 14).toInt() + 1)
            Text("░".repeat(cells), Modifier.fillMaxSize(), color = p.fg.copy(alpha = 0.25f), style = Type.label.copy(lineHeight = 14.sp), overflow = TextOverflow.Clip)
            Icon(painterResource(R.drawable.ic_note), null, Modifier.size(size / 2), tint = p.fg.copy(alpha = 0.5f))
        }
    }
}

/**
 * A 64 dp song row: art, the marker and title (▶ in green when it plays),
 * artist · album and the duration, then trailing (⋮ or ≡). A long-press
 * opens the song's actions.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun TrackRow(
    track: Track, playing: Boolean, modifier: Modifier = Modifier, marker: String = "",
    onLongClick: (() -> Unit)? = null, trailing: (@Composable () -> Unit)? = null, onClick: () -> Unit,
) {
    val p = LocalPalette.current
    val haptic = LocalHapticFeedback.current
    Row(
        modifier.fillMaxWidth().height(64.dp).background(p.bg)
            .combinedClickable(
                onLongClick = onLongClick?.let { { haptic.performHapticFeedback(HapticFeedbackType.LongPress); it() } },
                onClick = onClick,
            )
            .padding(start = 16.dp, end = if (trailing != null) 4.dp else 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        ArtBox(track.art, 48.dp)
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text(
                buildAnnotatedString {
                    if (marker.isNotEmpty()) withStyle(SpanStyle(color = p.accent)) { append("$marker ") }
                    append((if (playing) "▶" else "") + track.name)
                },
                color = if (playing) p.playing else p.textPrimary,
                style = if (playing) Type.bodyStrong else Type.body, maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
            Row {
                Text(
                    listOf(track.artist, track.album).filter { it.isNotEmpty() }.joinToString(" · "), Modifier.weight(1f),
                    color = p.textSecondary, style = Type.label, maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
                if (track.durationSecs > 0) Text("  " + clock(track.durationSecs), color = p.textSecondary, style = Type.label)
            }
        }
        trailing?.invoke()
    }
}

/**
 * A toggle or choice `[Label]` (§4.4): off, the brackets dim and the label
 * accent; on, all accent bold. icon draws a vector glyph before the label.
 */
@Composable
fun BracketChip(label: String, on: Boolean, modifier: Modifier = Modifier, @DrawableRes icon: Int? = null, onClick: () -> Unit) {
    val p = LocalPalette.current
    val style = Type.body.bold(on)
    val bracket = if (on) p.accent else p.textSecondary
    Row(
        modifier.heightIn(min = 48.dp).clickable(onClick = onClick).padding(horizontal = 4.dp).semantics { selected = on },
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text("[", color = bracket, style = style)
        if (icon != null) Icon(painterResource(icon), null, Modifier.size(18.dp), tint = p.accent)
        if (icon != null && label.isNotEmpty()) Spacer(Modifier.width(4.dp))
        Text(label, color = p.accent, style = style)
        Text("]", color = bracket, style = style)
    }
}

/** The primary action (§4.4): accent fill, onAccent bold text, square corners. */
@Composable
fun KeyPill(text: String, modifier: Modifier = Modifier, enabled: Boolean = true, onClick: () -> Unit) {
    val p = LocalPalette.current
    Box(
        modifier.height(48.dp).background(if (enabled) p.accent else p.surfaceHigh).clickable(enabled, onClick = onClick).padding(horizontal = 16.dp),
        contentAlignment = Alignment.Center,
    ) { Text(text, color = if (enabled) p.onAccent else p.textSecondary, style = Type.bodyStrong) }
}

/**
 * elapsedRealtime, moved every 250 ms only while s plays on a started
 * screen (§5.4). Read it in a draw lambda or derivedStateOf, so a tick
 * redraws without recomposing.
 */
@Composable
fun rememberNow(s: Snapshot?): LongState {
    val now = remember { mutableLongStateOf(SystemClock.elapsedRealtime()) }
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    LaunchedEffect(s) {
        now.longValue = SystemClock.elapsedRealtime()
        if (s?.playing != true) return@LaunchedEffect
        lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) {
            while (true) {
                now.longValue = SystemClock.elapsedRealtime()
                delay(250)
            }
        }
    }
    return now
}

/** What the seek bar shows: the finger while it drags, then the target until the core reports it (§5.4). */
@Stable
class SeekState {
    var dragging by mutableStateOf<Double?>(null)
    private var target by mutableStateOf<Double?>(null)
    private var path: String? = null
    private var releasedAt = 0L

    fun position(s: Snapshot, now: Long): Double =
        dragging ?: target?.takeIf { seekHolds(it, path, releasedAt, s, now) } ?: PlaybackClock.positionAt(s, now)

    fun release(to: Double, path: String?) {
        dragging = null
        target = to
        this.path = path
        releasedAt = SystemClock.elapsedRealtime()
        Remote.fire("seek.absolute", JSONObject().put("value", to))
    }
}

/**
 * The seek bar `━━━●───` (§2.3): 48 dp tall to touch, the head the only
 * round shape. A drag or a tap seeks on release.
 */
@Composable
fun SeekBar(s: Snapshot, seek: SeekState, now: LongState, modifier: Modifier = Modifier) {
    val p = LocalPalette.current
    val d = s.duration
    val enabled = s.seekable && d > 0
    // The gesture blocks outlive a snapshot: they read the song they seek in here.
    val path by rememberUpdatedState(s.track?.path)
    fun at(x: Float, width: Int) = (x / width).coerceIn(0f, 1f) * d
    Box(
        modifier.fillMaxWidth().height(48.dp)
            .pointerInput(enabled, d) {
                if (enabled) detectTapGestures { seek.release(at(it.x, size.width), path) }
            }
            .pointerInput(enabled, d) {
                if (!enabled) return@pointerInput
                // Horizontal drags only: a vertical swipe that starts here still reaches the sheet (§7.5).
                var x = 0f
                detectHorizontalDragGestures(
                    onDragStart = {
                        x = it.x
                        seek.dragging = at(x, size.width)
                    },
                    onDragEnd = { seek.release(at(x, size.width), path) },
                    onDragCancel = { seek.dragging = null },
                ) { change, _ ->
                    change.consume()
                    x = change.position.x
                    seek.dragging = at(x, size.width)
                }
            }
            .drawBehind {
                val f = if (d > 0) (seek.position(s, now.longValue) / d).toFloat().coerceIn(0f, 1f) else 0f
                val y = size.height / 2
                val x = size.width * f
                drawLine(p.textSecondary, Offset(x, y), Offset(size.width, y), 1.dp.toPx())
                drawLine(p.accent, Offset(0f, y), Offset(x, y), 3.dp.toPx())
                if (enabled) drawCircle(p.accent, 6.dp.toPx(), Offset(x, y))
            },
    )
}

/**
 * A horizontal swipe that skips (§2.2, §7.5): past 30 % of the width, or a
 * fling over 1000 dp/s, left plays the next song and right the one before.
 * The content follows the finger and springs back.
 */
fun Modifier.skipSwipe(): Modifier = composed {
    val offset = remember { Animatable(0f) }
    val scope = rememberCoroutineScope()
    // The gesture reads the untranslated bounds, so the finger's speed stays true while the content moves.
    pointerInput(Unit) {
        val tracker = VelocityTracker()
        var dragX = 0f // the decision reads this, not the offset that animates behind it
        val back = { scope.launch { offset.animateTo(0f, spring(stiffness = 600f)) } }
        detectHorizontalDragGestures(
            onDragStart = {
                tracker.resetTracking()
                dragX = 0f
            },
            onDragEnd = {
                val v = tracker.calculateVelocity().x
                val dir = when {
                    abs(dragX) > size.width * 0.3f -> dragX
                    abs(v) > 1000.dp.toPx() -> v
                    else -> 0f
                }
                if (dir != 0f) Remote.fire(if (dir < 0) "next" else "prev")
                back()
            },
            onDragCancel = { back() },
        ) { change, dx ->
            change.consume()
            tracker.addPosition(change.uptimeMillis, change.position)
            dragX += dx
            scope.launch { offset.snapTo(dragX) }
        }
    }.graphicsLayer { translationX = offset.value }
}

/** The braille spinner (accent) and a dim text. */
@Composable
fun SpinnerLine(text: String, style: TextStyle = Type.caption) {
    val p = LocalPalette.current
    Row {
        Text(spinner(), color = p.accent, style = style)
        Text(" $text", color = p.textSecondary, style = style)
    }
}

@Composable
fun Loading() {
    Box(Modifier.fillMaxWidth().padding(vertical = 32.dp), contentAlignment = Alignment.Center) {
        SpinnerLine(stringResource(R.string.loading), Type.body)
    }
}

/** `  No hay canciones.`, dim and indented, with an optional action. */
@Composable
fun EmptyState(text: String, action: String? = null, onAction: () -> Unit = {}) {
    val p = LocalPalette.current
    Column(Modifier.fillMaxWidth().padding(16.dp)) {
        Text("  $text", color = p.textSecondary, style = Type.body)
        if (action != null) BracketButton(action, onClick = onAction)
    }
}

/** `ERR: <mensaje>` in red, then `[ Reintentar ]`. */
@Composable
fun ErrorState(message: String, onRetry: () -> Unit) {
    val p = LocalPalette.current
    Column(Modifier.fillMaxWidth().padding(16.dp)) {
        Text("ERR: $message", color = p.error, style = Type.body)
        BracketButton(stringResource(R.string.retry), onClick = onRetry)
    }
}

/** A banner of the Inicio slot: surface fill, text and its buttons. */
@Composable
fun Banner(tag: String, content: @Composable RowScope.() -> Unit) {
    val p = LocalPalette.current
    Row(
        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp).background(p.surface).heightIn(min = 48.dp)
            .padding(horizontal = 12.dp).testTag(tag),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) { content() }
}

/** Bold variant of a style, for the active tab and chips. */
fun TextStyle.bold(on: Boolean) = if (on) copy(fontWeight = FontWeight.Bold) else this
