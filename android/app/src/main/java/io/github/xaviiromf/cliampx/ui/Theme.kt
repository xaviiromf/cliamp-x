package io.github.xaviiromf.cliampx.ui

import android.app.Activity
import androidx.compose.animation.animateColorAsState
import androidx.compose.foundation.IndicationNodeFactory
import androidx.compose.foundation.LocalIndication
import androidx.compose.foundation.interaction.InteractionSource
import androidx.compose.foundation.interaction.PressInteraction
import androidx.compose.material3.LocalRippleConfiguration
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.MotionDurationScale
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.ContentDrawScope
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.node.DelegatableNode
import androidx.compose.ui.node.DrawModifierNode
import androidx.compose.ui.node.invalidateDraw
import androidx.compose.ui.hapticfeedback.HapticFeedback
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.DialogWindowProvider
import androidx.core.view.WindowCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.ThemeInfo
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map as mapFlow // aliased: lint takes the List.map calls above for Flow.map
import kotlinx.coroutines.launch
import androidx.compose.ui.Modifier
import androidx.compose.material3.Typography

/** The seven colors of a cliamp-x theme (UI-SPEC §4.1) and the roles made from them. */
@Immutable
data class CliampPalette(
    val bg: Color, val accent: Color, val brightFg: Color, val fg: Color,
    val green: Color, val yellow: Color, val red: Color,
    /** The surfaces' opacity: below 1 a background (Backgrounds.kt) shows through the bars and cards. */
    val glass: Float = 1f,
) {
    val textPrimary get() = brightFg
    val textSecondary get() = fg
    val onAccent get() = if (accent.luminance() > 0.179f) Color.Black else Color.White
    val playing get() = green
    val favorite get() = red
    val error get() = red
    val warning get() = yellow
    val surface get() = mixSrgb(bg, brightFg, 0.05f).copy(alpha = glass)
    val surfaceHigh get() = mixSrgb(bg, brightFg, 0.09f).copy(alpha = glass)
    val pressed get() = accent.copy(alpha = 0.12f)
    val divider get() = fg.copy(alpha = 0.35f)
}

/** a blended toward b by f in sRGB; Compose's lerp blends in Oklab, which leaves 5 % of white over black still black. */
internal fun mixSrgb(a: Color, b: Color, f: Float) =
    Color(a.red + (b.red - a.red) * f, a.green + (b.green - a.green) * f, a.blue + (b.blue - a.blue) * f)

/** The built-in winamp theme: Default (terminal colors) and any incomplete theme draw with it. */
val Winamp = CliampPalette(
    bg = Color(0xFF000000), accent = Color(0xFF00FF00), brightFg = Color(0xFFFFFFFF), fg = Color(0xFF969696),
    green = Color(0xFF29CE10), yellow = Color(0xFFD6B521), red = Color(0xFFEF3110),
)

private val HEX = Regex("#?([0-9a-fA-F]{6})")

fun hexColor(s: String): Color? = HEX.matchEntire(s.trim())?.let { Color(0xFF000000 or it.groupValues[1].toLong(16)) }

fun paletteOf(t: ThemeInfo?): CliampPalette {
    t ?: return Winamp
    val c = listOf(t.bg, t.accent, t.brightFg, t.fg, t.green, t.yellow, t.red).map { hexColor(it) ?: return Winamp }
    return CliampPalette(c[0], c[1], c[2], c[3], c[4], c[5], c[6])
}

/**
 * The palette of the core's theme (§4.6). Under the cover theme each new
 * cover's colors blend in over 400 ms, eased and scaled by Ajustes →
 * Animaciones; picking a theme is instant (§4.5).
 */
@Composable
fun livePalette(t: ThemeInfo?): CliampPalette {
    val target = paletteOf(t)
    // A new theme name starts its colors at their target: only a cover's next colors blend.
    return key(t?.name) {
        val c = listOf(target.bg, target.accent, target.brightFg, target.fg, target.green, target.yellow, target.red)
            .map { animateColorAsState(it, Motion.spec(400), label = "theme").value }
        CliampPalette(c[0], c[1], c[2], c[3], c[4], c[5], c[6])
    }
}

/**
 * Whether the system turned animations off (ANIMATOR_DURATION_SCALE 0,
 * §4.5). Compose already applies that scale to the crossfades and the
 * theme blend, which then end at once; the marquee ignores it, so it asks here.
 */
@Composable
fun reducedMotion(): Boolean {
    var reduced by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) {
        val scale = coroutineContext[MotionDurationScale] ?: return@LaunchedEffect
        // The first read starts Compose's observer, which writes state: do it
        // outside snapshotFlow's read-only snapshot, or it crashes with animations off.
        reduced = scale.scaleFactor == 0f
        snapshotFlow { scale.scaleFactor == 0f }.collect { reduced = it }
    }
    return reduced
}

val LocalPalette = staticCompositionLocalOf { Winamp }

/** The type roles of §4.2 in one font and text scale. */
@Immutable
class TypeRoles(private val family: FontFamily, private val scale: Float) {
    private fun style(size: Int, line: Int, bold: Boolean = false) = TextStyle(
        fontFamily = family, fontSize = (size * scale).sp, lineHeight = (line * scale).sp,
        fontWeight = if (bold) FontWeight.Bold else FontWeight.Normal,
    )

    val logo = style(15, 20, bold = true)
    val display = style(22, 28, bold = true)
    val headline = style(18, 24, bold = true)
    val body = style(14, 20)
    val bodyStrong = style(14, 20, bold = true)
    val tile = style(13, 18)
    val label = style(12, 16)
    val labelCaps = style(12, 16, bold = true)
    val numeric = style(13, 18)
    val caption = style(11, 14)
}

/** The type roles in the font and text size of Ajustes (Look); a change recomposes every reader. Colors are set where each one is used. */
object Type {
    private val roles by derivedStateOf { TypeRoles(Look.family, Look.TEXT_SCALES[Look.textSize]) }
    val logo get() = roles.logo
    val display get() = roles.display
    val headline get() = roles.headline
    val body get() = roles.body
    val bodyStrong get() = roles.bodyStrong
    val tile get() = roles.tile
    val label get() = roles.label
    val labelCaps get() = roles.labelCaps
    val numeric get() = roles.numeric
    val caption get() = roles.caption
}

/** Haptics off in Ajustes: every LocalHapticFeedback reader goes quiet. */
private object NoHaptics : HapticFeedback {
    override fun performHapticFeedback(hapticFeedbackType: HapticFeedbackType) {}
}

/** Press feedback: an instant accent fill, no ripple (§4.5). */
private class PressFill(private val color: Color) : IndicationNodeFactory {
    override fun create(interactionSource: InteractionSource): DelegatableNode = Node(interactionSource, color)
    override fun equals(other: Any?) = other is PressFill && other.color == color
    override fun hashCode() = color.hashCode()

    private class Node(private val source: InteractionSource, private val color: Color) : Modifier.Node(), DrawModifierNode {
        private var pressed = false

        override fun onAttach() {
            coroutineScope.launch {
                source.interactions.collect {
                    pressed = when (it) {
                        is PressInteraction.Press -> true
                        is PressInteraction.Release, is PressInteraction.Cancel -> false
                        else -> pressed
                    }
                    invalidateDraw()
                }
            }
        }

        override fun ContentDrawScope.draw() {
            if (pressed) drawRect(color)
            drawContent()
        }
    }
}

/** A sheet's dialog window copies the bar icons once, when it opens: follow the live palette instead (§4.1). */
@Composable
fun SheetBars() {
    val view = LocalView.current
    val light = LocalPalette.current.bg.luminance() > 0.5f
    SideEffect {
        val window = (view.parent as? DialogWindowProvider)?.window ?: return@SideEffect
        WindowCompat.getInsetsController(window, view).apply {
            isAppearanceLightStatusBars = light
            isAppearanceLightNavigationBars = light
        }
    }
}

/** The core's live theme, glassy over a background: every window of the app draws in it. */
@Composable
fun LiveTheme(content: @Composable () -> Unit) {
    val theme by remember { Remote.state.mapFlow { it?.theme }.distinctUntilChanged() }.collectAsStateWithLifecycle(null)
    // A background shows through the bars and cards.
    val palette = livePalette(theme).let { if (Looks.background == Background.Solid) it else it.copy(glass = 0.6f) }
    CliampTheme(palette, content)
}

/** Colors, type, corners (square unless Ajustes rounds them), haptics and press feedback from the cliamp-x theme; system bar icons follow it. */
@Composable
fun CliampTheme(p: CliampPalette, content: @Composable () -> Unit) {
    val view = LocalView.current
    val light = p.bg.luminance() > 0.5f
    // enableEdgeToEdge sets the system's own icon colors again on each configuration change the activity handles.
    val config = LocalConfiguration.current
    LaunchedEffect(light, config) {
        val window = (view.context as? Activity)?.window ?: return@LaunchedEffect
        WindowCompat.getInsetsController(window, view).apply {
            isAppearanceLightStatusBars = light
            isAppearanceLightNavigationBars = light
        }
    }
    val corner = Look.corner
    MaterialTheme(
        colorScheme = darkColorScheme(
            primary = p.accent, onPrimary = p.onAccent, secondary = p.green, tertiary = p.yellow, error = p.red,
            background = p.bg, onBackground = p.brightFg, surface = p.bg, onSurface = p.brightFg,
            surfaceVariant = p.surfaceHigh, onSurfaceVariant = p.fg, outline = p.divider,
            surfaceContainer = p.bg, surfaceContainerLow = p.bg, surfaceContainerLowest = p.bg,
            surfaceContainerHigh = p.bg, surfaceContainerHighest = p.bg, surfaceBright = p.bg, surfaceDim = p.bg,
        ),
        shapes = Shapes(corner, corner, corner, corner, corner),
        typography = Typography(bodyLarge = Type.body, bodyMedium = Type.body, labelLarge = Type.label),
    ) {
        CompositionLocalProvider(
            LocalPalette provides p,
            LocalIndication provides remember(p.pressed) { PressFill(p.pressed) },
            LocalRippleConfiguration provides null,
            LocalHapticFeedback provides if (Look.haptics) LocalHapticFeedback.current else NoHaptics,
        ) { content() }
    }
}
