package io.github.xaviiromf.cliampx.ui

import android.content.ClipData
import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.graphics.layer.drawLayer
import androidx.compose.ui.graphics.rememberGraphicsLayer
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.content.FileProvider
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote
import io.github.xaviiromf.cliampx.StatCount
import io.github.xaviiromf.cliampx.Wrap
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File
import java.time.OffsetDateTime
import java.time.Year
import java.time.format.DateTimeFormatter

/**
 * Tu año en cliamp-x (FINAL-PLAN T8): the year's `stats wrapped` as
 * swipeable cards in the theme and font, each one shared as a PNG. This
 * year or the last one; read again on each visit, never cached.
 */
@Composable
fun WrappedScreen() {
    val now = remember { Year.now().value }
    var year by rememberSaveable { mutableIntStateOf(now) }
    val (load, retry) = rememberLoad("wrapped:$year") { Remote.wrapped(year) }
    val w = (load as? Load.Done)?.value
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.wrapped))
        Row(Modifier.padding(horizontal = 12.dp)) {
            for (y in listOf(now, now - 1)) BracketChip("$y", y == year, Modifier.testTag("wrapped_$y")) { year = y }
        }
        when {
            w == null -> LoadState(load, retry)
            w.cards.isEmpty() -> EmptyState(stringResource(R.string.wrapped_none, year))
            else -> key(w.year) { Story(w) }
        }
    }
}

/** The cards in a pager, and `● · ·` under them. */
@Composable
private fun ColumnScope.Story(w: Wrap) {
    val p = LocalPalette.current
    val cards = w.cards
    val pager = rememberPagerState { cards.size }
    HorizontalPager(
        pager, Modifier.weight(1f).testTag("wrapped_pager"), contentPadding = PaddingValues(horizontal = 16.dp), pageSpacing = 12.dp,
        key = { cards[it] },
    ) { StoryPage(w, cards[it]) }
    val page = stringResource(R.string.wrapped_page, pager.currentPage + 1, cards.size)
    Text(
        cards.indices.joinToString(" ") { if (it == pager.currentPage) "●" else "·" },
        Modifier.align(Alignment.CenterHorizontally).padding(vertical = 8.dp).semantics { contentDescription = page },
        color = p.accent, style = Type.body,
    )
}

/** One card, recorded as it draws so [ Compartir imagen ] shares exactly what is on screen. */
@Composable
private fun StoryPage(w: Wrap, card: Wrap.Card) {
    val layer = rememberGraphicsLayer()
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
        // 4:5, the portrait size social apps keep whole; large text grows it instead of clipping.
        BoxWithConstraints(Modifier.fillMaxWidth()) {
            StoryCard(
                w, card,
                Modifier.fillMaxWidth().heightIn(min = maxWidth * 1.25f).testTag("wrapped_card_${card.name.lowercase()}")
                    .drawWithContent {
                        layer.record { this@drawWithContent.drawContent() }
                        drawLayer(layer)
                    },
            )
        }
        BracketButton(stringResource(R.string.wrapped_share), Modifier.align(Alignment.CenterHorizontally).testTag("wrapped_share")) {
            scope.launch {
                try {
                    share(context, layer.toImageBitmap(), "cliamp-x-${w.year}-${card.name.lowercase()}")
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    Remote.showError(e)
                }
            }
        }
    }
}

/** The card: logo and year, the card's own body, the footer; one node for TalkBack. */
@Composable
private fun StoryCard(w: Wrap, card: Wrap.Card, modifier: Modifier) {
    val p = LocalPalette.current
    Column(
        modifier.background(p.bg, Look.corner).border(1.dp, p.accent, Look.corner).padding(20.dp).semantics(mergeDescendants = true) {},
        verticalArrangement = Arrangement.SpaceBetween,
    ) {
        Column {
            Text("C L I A M P - X", color = p.accent, style = Type.logo)
            Text("── ${w.year} " + "─".repeat(80), color = p.textSecondary, style = Type.label, maxLines = 1, softWrap = false, overflow = TextOverflow.Clip)
        }
        Column(Modifier.padding(vertical = 16.dp)) {
            when (card) {
                Wrap.Card.Intro -> {
                    Title(stringResource(R.string.wrapped))
                    Big("${w.year}")
                    Line(pluralStringResource(R.plurals.plays_count, w.plays, w.plays))
                    Line(pluralStringResource(R.plurals.songs_count, w.songs, w.songs))
                    Line(pluralStringResource(R.plurals.artists_count, w.artists, w.artists))
                }
                Wrap.Card.Minutes -> {
                    Title(stringResource(R.string.wrapped_minutes))
                    Big("%,d".format(w.minutes))
                    Line(stringResource(R.string.wrapped_hours, w.minutes / 60))
                    Line(stringResource(R.string.wrapped_days, w.days))
                }
                Wrap.Card.Artists -> Ranking(stringResource(R.string.wrapped_artists), w.topArtists)
                Wrap.Card.Songs -> Ranking(stringResource(R.string.wrapped_songs), w.topSongs)
                Wrap.Card.Albums -> Ranking(stringResource(R.string.wrapped_albums), w.topAlbums)
                Wrap.Card.Genres -> Ranking(stringResource(R.string.wrapped_genres), w.topGenres)
                Wrap.Card.Streak -> {
                    Title(stringResource(R.string.wrapped_streak))
                    Big("${w.streak}")
                    Line(pluralStringResource(R.plurals.wrapped_streak_unit, w.streak))
                }
                Wrap.Card.Moments -> {
                    w.first?.let { first ->
                        Title(stringResource(R.string.wrapped_first))
                        Song(first)
                        val at = remember(w.firstAt) {
                            runCatching { OffsetDateTime.parse(w.firstAt).format(DateTimeFormatter.ofPattern("d MMM, HH:mm")) }.getOrDefault("")
                        }
                        if (at.isNotEmpty()) Line(at)
                        Spacer(Modifier.height(20.dp))
                    }
                    w.mostReplayed?.let { most ->
                        Title(stringResource(R.string.wrapped_most))
                        Song(most)
                        Line(pluralStringResource(R.plurals.wrapped_times, most.plays, most.plays))
                    }
                }
            }
        }
        Text("▸ cliamp-x", color = p.textSecondary, style = Type.caption)
    }
}

@Composable
private fun Title(text: String) = Text(text, Modifier.padding(bottom = 8.dp), color = LocalPalette.current.accent, style = Type.headline)

/** The card's number: the display role, larger. */
@Composable
private fun Big(text: String) = Text(
    text, color = LocalPalette.current.textPrimary, maxLines = 1,
    style = Type.display.let { it.copy(fontSize = it.fontSize * 2.4f, lineHeight = it.lineHeight * 2.4f) },
)

@Composable
private fun Line(text: String) = Text(text, color = LocalPalette.current.textSecondary, style = Type.body)

/** A song's title in bold and its artist dim. */
@Composable
private fun Song(s: StatCount) {
    Text(s.name, color = LocalPalette.current.textPrimary, style = Type.bodyStrong, maxLines = 2, overflow = TextOverflow.Ellipsis)
    if (s.detail.isNotEmpty()) Line(s.detail)
}

/** The top five: `1. Daft Punk` with the artist and the plays dim. */
@Composable
private fun Ranking(title: String, rows: List<StatCount>) {
    val p = LocalPalette.current
    Title(title)
    rows.take(5).forEachIndexed { i, r ->
        Row(Modifier.padding(vertical = 4.dp)) {
            Text("${i + 1}. ", color = p.accent, style = Type.bodyStrong)
            Column {
                Text(r.name, color = p.textPrimary, style = Type.body.bold(i == 0), maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(
                    listOfNotNull(r.detail.ifEmpty { null }, pluralStringResource(R.plurals.plays_count, r.plays, r.plays)).joinToString(" · "),
                    color = p.textSecondary, style = Type.label, maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
}

/** Writes image as a PNG in the cache's shares/ (served by the FileProvider) and opens the share sheet. */
private suspend fun share(context: Context, image: ImageBitmap, name: String) {
    val file = withContext(Dispatchers.IO) {
        var bitmap = image.asAndroidBitmap()
        if (bitmap.config == Bitmap.Config.HARDWARE) bitmap = bitmap.copy(Bitmap.Config.ARGB_8888, false)
        File(context.cacheDir, "shares").apply { mkdirs() }.resolve("$name.png").also { f ->
            f.outputStream().use { check(bitmap.compress(Bitmap.CompressFormat.PNG, 100, it)) { "PNG" } }
        }
    }
    val uri = FileProvider.getUriForFile(context, "${context.packageName}.updates", file)
    val send = Intent(Intent.ACTION_SEND).setType("image/png").putExtra(Intent.EXTRA_STREAM, uri)
        .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
    send.clipData = ClipData.newRawUri(null, uri) // the share sheet's preview reads it too
    context.startActivity(Intent.createChooser(send, context.getString(R.string.wrapped_share)))
}
