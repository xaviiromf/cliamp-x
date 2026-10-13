package io.github.xaviiromf.cliampx.ui

import android.app.Activity
import androidx.compose.foundation.IndicationNodeFactory
import androidx.compose.foundation.LocalIndication
import androidx.compose.foundation.interaction.InteractionSource
import androidx.compose.foundation.interaction.PressInteraction
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.LocalRippleConfiguration
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.ContentDrawScope
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.node.DelegatableNode
import androidx.compose.ui.node.DrawModifierNode
import androidx.compose.ui.node.invalidateDraw
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import androidx.core.view.WindowCompat
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.ThemeInfo
import kotlinx.coroutines.launch
import androidx.compose.ui.Modifier
import androidx.compose.material3.Typography

/** The seven colors of a cliamp-x theme (UI-SPEC §4.1) and the roles made from them. */
@Immutable
data class CliampPalette(
    val bg: Color, val accent: Color, val brightFg: Color, val fg: Color,
    val green: Color, val yellow: Color, val red: Color,
) {
    val textPrimary get() = brightFg
    val textSecondary get() = fg
    val onAccent get() = if (accent.luminance() > 0.179f) Color.Black else Color.White
    val playing get() = green
    val error get() = red
    val warning get() = yellow
    val surface get() = mix(bg, brightFg, 0.05f)
    val surfaceHigh get() = mix(bg, brightFg, 0.09f)
    val pressed get() = accent.copy(alpha = 0.12f)
    val divider get() = fg.copy(alpha = 0.35f)
}

/** a blended toward b by f in sRGB; Compose's lerp blends in Oklab, which leaves 5 % of white over black still black. */
private fun mix(a: Color, b: Color, f: Float) =
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

val LocalPalette = staticCompositionLocalOf { Winamp }

private val Mono = FontFamily(
    Font(R.font.jetbrains_mono_regular, FontWeight.Normal),
    Font(R.font.jetbrains_mono_bold, FontWeight.Bold),
)

private fun style(size: Int, line: Int, bold: Boolean = false) =
    TextStyle(fontFamily = Mono, fontSize = size.sp, lineHeight = line.sp, fontWeight = if (bold) FontWeight.Bold else FontWeight.Normal)

/** The type roles of §4.2. Colors are set where each one is used. */
object Type {
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

/** Colors, type, square shapes and press feedback from the cliamp-x theme; system bar icons follow it. */
@Composable
fun CliampTheme(p: CliampPalette, content: @Composable () -> Unit) {
    val view = LocalView.current
    SideEffect {
        val window = (view.context as? Activity)?.window ?: return@SideEffect
        WindowCompat.getInsetsController(window, view).apply {
            val light = p.bg.luminance() > 0.5f
            isAppearanceLightStatusBars = light
            isAppearanceLightNavigationBars = light
        }
    }
    val square = RoundedCornerShape(0)
    MaterialTheme(
        colorScheme = darkColorScheme(
            primary = p.accent, onPrimary = p.onAccent, secondary = p.green, tertiary = p.yellow, error = p.red,
            background = p.bg, onBackground = p.brightFg, surface = p.bg, onSurface = p.brightFg,
            surfaceVariant = p.surfaceHigh, onSurfaceVariant = p.fg, outline = p.divider,
            surfaceContainer = p.bg, surfaceContainerLow = p.bg, surfaceContainerLowest = p.bg,
            surfaceContainerHigh = p.bg, surfaceContainerHighest = p.bg, surfaceBright = p.bg, surfaceDim = p.bg,
        ),
        shapes = Shapes(square, square, square, square, square),
        typography = Typography(bodyLarge = Type.body, bodyMedium = Type.body, labelLarge = Type.label),
    ) {
        CompositionLocalProvider(
            LocalPalette provides p,
            LocalIndication provides remember(p.pressed) { PressFill(p.pressed) },
            LocalRippleConfiguration provides null,
        ) { content() }
    }
}
