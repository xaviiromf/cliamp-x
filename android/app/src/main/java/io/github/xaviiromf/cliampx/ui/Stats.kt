package io.github.xaviiromf.cliampx.ui

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Rating
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.StatCount
import io.github.xaviiromf.cliampx.Stats
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch
import kotlin.math.roundToInt

/** The periods of `stats` (G16) and their chips. */
private val PERIODS = listOf("week" to R.string.stats_week, "month" to R.string.stats_month, "year" to R.string.stats_year, "all" to R.string.stats_all)

/**
 * Estadísticas (§3.16): the period chips, Tu año en cliamp-x, the summary, the minutes of the
 * last 30 days as a block chart, then the top artists, songs and genres.
 * Read again on each visit and period, never cached.
 */
@Composable
fun StatsScreen() {
    val p = LocalPalette.current
    var period by rememberSaveable { mutableStateOf("week") }
    val (load, retry) = rememberLoad("stats:$period") { Remote.stats(period) }
    val s = (load as? Load.Done)?.value
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.stats))
        LazyColumn(Modifier.fillMaxSize()) {
            item(key = "periods") {
                Row(Modifier.padding(horizontal = 12.dp)) {
                    for ((id, label) in PERIODS) BracketChip(stringResource(label), id == period, Modifier.testTag("stats_$id")) { period = id }
                }
            }
            item(key = "wrapped") {
                BracketButton("▸ " + stringResource(R.string.wrapped), Modifier.padding(horizontal = 12.dp).testTag("stats_wrapped")) { Nav.open(Screen.Wrapped) }
            }
            if (s == null) {
                item(key = "state") { LoadState(load, retry) }
                return@LazyColumn
            }
            item(key = "summary") {
                val min = (s.secs / 60).roundToInt()
                Text(
                    listOf(
                        if (min >= 60) stringResource(R.string.hours_minutes, min / 60, min % 60) else stringResource(R.string.minutes, min),
                        pluralStringResource(R.plurals.plays_count, s.plays, s.plays),
                        pluralStringResource(R.plurals.songs_count, s.songs, s.songs),
                        pluralStringResource(R.plurals.streak_days, s.streak, s.streak),
                    ).joinToString(" · "),
                    Modifier.padding(horizontal = 16.dp, vertical = 8.dp).testTag("stats_summary"), color = p.textPrimary, style = Type.body,
                )
            }
            item(key = "daily") {
                Column {
                    SectionRule(stringResource(R.string.stats_daily))
                    // One block column per day, oldest at the left; the line height packs the rows into one chart.
                    for (line in Stats.chart(s.daily)) {
                        Text(line, Modifier.padding(horizontal = 16.dp), color = p.accent, style = Type.body.copy(lineHeight = Type.body.fontSize), softWrap = false, maxLines = 1)
                    }
                }
            }
            ranking(R.string.lib_artists, s.topArtists, "artists")
            ranking(R.string.songs, s.topSongs, "songs")
            ranking(R.string.genres, s.topGenres, "genres")
            if (s.plays == 0) item(key = "empty") { EmptyState(stringResource(R.string.stats_none)) }
            item(key = "end") { Spacer(Modifier.height(16.dp)) }
        }
    }
}

/** ` 1. Daft Punk — 42  ██████████` per row: the name, the plays dim, a bar to the top count in accent. */
private fun LazyListScope.ranking(title: Int, rows: List<StatCount>, tag: String) {
    if (rows.isEmpty()) return
    item(key = "rule_$tag") { SectionRule(stringResource(title)) }
    val top = rows.maxOf { it.plays }.coerceAtLeast(1)
    itemsIndexed(rows, key = { i, _ -> "$tag:$i" }) { i, r ->
        val p = LocalPalette.current
        Row(Modifier.fillMaxWidth().heightIn(min = 32.dp).padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) {
            Text("%2d. ".format(i + 1), color = p.textSecondary, style = Type.body)
            Text(
                if (r.detail.isEmpty()) r.name else "${r.name} · ${r.detail}", Modifier.weight(1f),
                color = p.textPrimary, style = Type.body, maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
            Text(" — ${r.plays} ", color = p.textSecondary, style = Type.label)
            Text("█".repeat((r.plays * BAR / top).coerceAtLeast(1)), Modifier.width((BAR * 8).dp), color = p.accent, style = Type.label, maxLines = 1, softWrap = false)
        }
    }
}

/** The longest ranking bar, in cells. */
private const val BAR = 10

/**
 * Tus valoraciones (§3.16): the "No me interesa" and "Me interesa"
 * ratings, newest first, each with Deshacer, which forgets it. Read again
 * on each visit, never cached.
 */
@Composable
fun RatingsScreen() {
    val (load, retry) = rememberLoad("ratings") { Remote.ratings() }
    var undone by remember { mutableStateOf(emptySet<String>()) }
    val scope = rememberCoroutineScope()
    val rows = (load as? Load.Done)?.value.orEmpty().filter { it.id !in undone }
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.ratings))
        LazyColumn(Modifier.fillMaxSize()) {
            if (load !is Load.Done) item(key = "state") { LoadState(load, retry) }
            if (load is Load.Done && rows.isEmpty()) item(key = "empty") { EmptyState(stringResource(R.string.ratings_none)) }
            for ((interest, title) in listOf(-1 to R.string.not_interested, 1 to R.string.interested)) {
                val group = rows.filter { it.interest == interest }
                if (group.isEmpty()) continue
                item(key = "rule$interest") { SectionRule(stringResource(title)) }
                itemsIndexed(group, key = { _, r -> r.id }) { i, r ->
                    RatingRow(r, "rating_${if (interest < 0) "no" else "yes"}_$i") {
                        undone = undone + r.id // the row goes at once; a failure brings it back
                        scope.launch {
                            try {
                                Remote.unrate(r)
                            } catch (e: CancellationException) {
                                throw e
                            } catch (e: Exception) {
                                undone = undone - r.id
                                Remote.showError(e)
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun RatingRow(r: Rating, tag: String, undo: () -> Unit) {
    val p = LocalPalette.current
    val kind = when (r.kind) {
        "song" -> R.string.kind_song
        "album" -> R.string.kind_album
        "artist" -> R.string.kind_artist
        else -> R.string.kind_genre
    }
    Row(Modifier.fillMaxWidth().heightIn(min = 48.dp).padding(start = 16.dp, end = 4.dp).testTag(tag), verticalAlignment = Alignment.CenterVertically) {
        Text(stringResource(kind), Modifier.width(80.dp), color = p.textSecondary, style = Type.label, maxLines = 1)
        Text(r.name, Modifier.weight(1f), color = p.textPrimary, style = Type.body, maxLines = 2, overflow = TextOverflow.Ellipsis)
        BracketButton(stringResource(R.string.undo), Modifier.testTag("${tag}_undo"), onClick = undo)
    }
}
