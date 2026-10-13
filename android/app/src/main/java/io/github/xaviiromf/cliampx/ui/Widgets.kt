package io.github.xaviiromf.cliampx.ui

import androidx.annotation.DrawableRes
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Track
import io.github.xaviiromf.cliampx.clock
import kotlinx.coroutines.delay

object Glyphs {
    /** Text glyphs drawn in JetBrains Mono; GlyphTest checks each one is in the font. */
    const val ALL = "─▸▶◌✓✕░·…>_[]"

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
fun IconBtn(@DrawableRes icon: Int, description: String, modifier: Modifier = Modifier, tint: Color = LocalPalette.current.textSecondary, onClick: () -> Unit) {
    Box(modifier.size(48.dp).clickable(onClick = onClick), contentAlignment = Alignment.Center) {
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

/** A 64 dp song row: art, title (▶ in green when it plays), artist · album and the duration. */
@Composable
fun TrackRow(track: Track, playing: Boolean, modifier: Modifier = Modifier, onClick: () -> Unit) {
    val p = LocalPalette.current
    Row(
        modifier.fillMaxWidth().height(64.dp).clickable(onClick = onClick).padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        ArtBox(track.art, 48.dp)
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text(
                (if (playing) "▶" else "") + track.name, color = if (playing) p.playing else p.textPrimary,
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
    }
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
