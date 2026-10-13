package io.github.xaviiromf.cliampx.ui

import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.Snapshot
import io.github.xaviiromf.cliampx.eqDrag
import io.github.xaviiromf.cliampx.eqGain
import io.github.xaviiromf.cliampx.eqPending
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.channels.Channel
import org.json.JSONObject
import kotlin.math.abs
import kotlin.math.roundToInt

/** The center frequencies of the core's 10 bands (player/eq.go). */
private val FREQS = listOf("70", "180", "320", "600", "1k", "3k", "6k", "12k", "14k", "16k")

/**
 * The equalizer page (§2.8, §3.15): ten block columns dragged in 1 dB steps
 * between -12 and +12, the gains, and the presets. A drag sends its value
 * through a conflated sender: one eq call in flight, the latest wins.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun EqPage(s: Snapshot) {
    val p = LocalPalette.current
    val ops by Remote.ops.collectAsStateWithLifecycle()
    var presets by remember { mutableStateOf(emptyList<String>()) }
    LaunchedEffect(Unit) {
        try {
            presets = Remote.eqPresets()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            Remote.showError(e)
        }
    }
    // The gains the finger set that the snapshot does not show yet, by band.
    val local = remember { mutableStateMapOf<Int, Int>() }
    var selected by remember { mutableIntStateOf(-1) }
    val wake = remember { Channel<Unit>(Channel.CONFLATED) }
    LaunchedEffect(Unit) {
        // One eq call in flight. Each touch starts a round that sends, newest value first, every band the core lacks.
        for (touch in wake) {
            val sent = HashMap<Int, Int>() // a value the core did not take is not sent again until the next touch
            while (true) {
                val (band, db) = eqPending(local, Remote.state.value?.eqBands.orEmpty(), sent) ?: break
                sent[band] = db
                try {
                    Remote.call("eq", JSONObject().put("band", band).put("value", db))
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    Remote.showError(e)
                    local.remove(band) // the column shows the core's gain again
                }
            }
        }
    }
    // Read through State, so the gesture handlers, started once, see the newest gains.
    val bands by rememberUpdatedState(List(10) { s.eqBands.getOrNull(it)?.roundToInt() ?: 0 })
    LaunchedEffect(s.eqBands) { local.keys.removeAll { it != selected && local[it] == bands[it] } }
    fun gain(band: Int) = local[band] ?: bands[band]
    fun set(band: Int, db: Int) {
        local[band] = db
        wake.trySend(Unit)
    }

    val preset = if (s.eqPreset == "Custom" || s.eqPreset.isEmpty()) stringResource(R.string.eq_custom) else s.eqPreset
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
        Row(Modifier.fillMaxWidth().height(56.dp), verticalAlignment = Alignment.CenterVertically) {
            IconBtn(R.drawable.ic_back, stringResource(R.string.back), Modifier.testTag("eq_back")) { Nav.page = PlayerPage.Player }
            Text(
                "▸─ " + stringResource(R.string.eq) + " · " + preset + " " + "─".repeat(80), Modifier.weight(1f),
                color = p.accent, style = Type.headline, maxLines = 1, softWrap = false, overflow = TextOverflow.Clip,
            )
        }
        Row(Modifier.fillMaxWidth().height(240.dp).padding(start = 8.dp, end = 16.dp, top = 8.dp)) {
            Box(Modifier.width(36.dp).fillMaxHeight()) {
                for ((label, align) in listOf("+12" to Alignment.TopEnd, "0" to Alignment.CenterEnd, "-12" to Alignment.BottomEnd)) {
                    Text(label, Modifier.align(align).padding(end = 4.dp), color = p.textSecondary, style = Type.label)
                }
            }
            for (band in 0 until 10) {
                val description = stringResource(R.string.eq_band, FREQS[band])
                Box(
                    Modifier.weight(1f).fillMaxHeight().testTag("eq_band_$band").semantics { contentDescription = description }
                        .pointerInput(band) {
                            // A tap sets the gain where it lands; a drag moves it from where it was, the slop included.
                            awaitEachGesture {
                                val down = awaitFirstDown()
                                val base = gain(band)
                                var moved = false
                                selected = band
                                while (true) {
                                    val change = awaitPointerEvent().changes.first()
                                    val dy = change.position.y - down.position.y
                                    if (abs(dy) > viewConfiguration.touchSlop) moved = true
                                    if (moved) {
                                        change.consume()
                                        val db = eqDrag(base, dy, size.height / 24f)
                                        if (db != gain(band)) set(band, db)
                                    }
                                    if (!change.pressed) break
                                }
                                if (!moved) set(band, eqGain(down.position.y, size.height.toFloat()))
                                selected = -1
                            }
                        }
                        .drawBehind { eqColumn(gain(band), if (band == selected) p.accent else p.textPrimary, p.divider) },
                )
            }
        }
        LabelRow(FREQS, p.textSecondary, Type.label)
        LabelRow(List(10) { gain(it).let { db -> if (db > 0) "+$db" else "$db" } }, p.textPrimary, Type.numeric)
        Row(Modifier.fillMaxWidth().padding(start = 44.dp, end = 16.dp, top = 4.dp), horizontalArrangement = Arrangement.SpaceBetween) {
            for (r in listOf(R.string.eq_low, R.string.eq_mid, R.string.eq_high)) Text(stringResource(r), color = p.textSecondary, style = Type.label)
        }
        if ("settings.set" in ops) {
            BracketChip(
                stringResource(R.string.eq_auto, stringResource(if (s.eqAuto) R.string.on else R.string.off)), s.eqAuto,
                Modifier.padding(horizontal = 12.dp).testTag("eq_auto"),
            ) { Remote.fire("settings.set", JSONObject().put("name", "eq_auto").put("value", if (s.eqAuto) 0 else 1)) }
        }
        SectionRule(stringResource(R.string.eq_presets))
        FlowRow(Modifier.padding(horizontal = 12.dp)) {
            for (name in presets) {
                val on = name.equals(s.eqPreset, ignoreCase = true)
                val label = if (name == "Custom") stringResource(R.string.eq_custom) else name
                BracketChip(if (on) "●$label" else label, on, Modifier.testTag("eq_preset_$name")) {
                    local.clear()
                    Remote.fire("eq", JSONObject().put("name", name))
                }
            }
        }
    }
}

/** A row of ten labels under the columns. */
@Composable
private fun LabelRow(labels: List<String>, color: Color, style: TextStyle) {
    Row(Modifier.fillMaxWidth().padding(start = 44.dp, end = 16.dp, top = 4.dp)) {
        for (label in labels) Text(label, Modifier.weight(1f), color = color, style = style, textAlign = TextAlign.Center, maxLines = 1)
    }
}

/**
 * One band as blocks: a cell per dB from the 0 row up or down to db, the
 * 0 row as ─, or ━ where the band sits at 0.
 */
private fun DrawScope.eqColumn(db: Int, color: Color, zero: Color) {
    val cell = size.height / 24
    val mid = size.height / 2
    val w = size.width * 0.6f
    val x = (size.width - w) / 2
    val gap = 1.dp.toPx()
    drawLine(zero, Offset(0f, mid), Offset(size.width, mid), 1.dp.toPx())
    if (db == 0) drawLine(color, Offset(x, mid), Offset(x + w, mid), 3.dp.toPx())
    for (k in 0 until abs(db)) {
        val top = if (db > 0) mid - (k + 1) * cell else mid + k * cell
        drawRect(color, Offset(x, top + gap / 2), Size(w, cell - gap))
    }
}
