package io.github.xaviiromf.cliampx.ui

import android.content.Context
import android.content.Intent
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.core.net.toUri
import io.github.xaviiromf.cliampx.BuildConfig
import io.github.xaviiromf.cliampx.R
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.IOException

/** A component the APK carries: its name, license and the license text in assets/licenses. */
class Notice(val name: String, val license: String, val file: String)

const val SOURCE_URL = "https://github.com/xaviiromf/cliamp-x"
private const val UPSTREAM_URL = "https://github.com/bjarneo/cliamp"

/** go-modules.txt is written by make android-core, so it is not in git. */
const val GO_NOTICES = "go-modules.txt"

/** Everything the APK carries besides the downloadable fonts, which come from fonts.json. */
val NOTICES = listOf(
    Notice("cliamp-x, cliamp", "MIT", "cliamp-x-MIT.txt"),
    Notice("youtubedl-android", "GPL-3.0", "GPL-3.0.txt"),
    Notice("yt-dlp", "Unlicense", "Unlicense.txt"),
    Notice("FFmpeg", "GPL-3.0", "GPL-3.0.txt"),
    Notice("Python", "PSF-2.0", "PSF-2.0.txt"),
    Notice("QuickJS", "MIT", "QuickJS-MIT.txt"),
    Notice("Termux terminal-view, terminal-emulator", "Apache-2.0", "Apache-2.0.txt"),
    Notice("AndroidX, Jetpack Compose, Kotlin, kotlinx.coroutines", "Apache-2.0", "Apache-2.0.txt"),
    Notice("Material Symbols", "Apache-2.0", "Apache-2.0.txt"),
    Notice("JetBrains Mono Nerd Font", "OFL 1.1", "JetBrainsMono-OFL.txt"),
    Notice("Adwaita Mono", "OFL 1.1", "AdwaitaMono-OFL.txt"),
    Notice("Go, Go modules", "BSD, MIT, Apache-2.0", GO_NOTICES),
)

private fun open(context: Context, url: String) = context.startActivity(Intent(Intent.ACTION_VIEW, url.toUri()))

/**
 * Acerca de (Ajustes → Sistema): the version, the source and that cliamp-x
 * is not the official cliamp, the licenses of the APK and the source, the
 * privacy policy, and every third-party license, each opened in full.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun AboutScreen() {
    val p = LocalPalette.current
    val context = LocalContext.current
    val fonts = remember { Fonts.specs(context).map { Notice(it.name, licenseName(it.license), it.license) } }
    Column(Modifier.fillMaxSize()) {
        TopBar(stringResource(R.string.about))
        LazyColumn(Modifier.fillMaxSize().testTag("about_list")) {
            item(key = "head") {
                Column(Modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
                    Text("cliamp-x", color = p.accent, style = Type.display)
                    Text(
                        stringResource(R.string.version, BuildConfig.VERSION_NAME) + " (${BuildConfig.VERSION_CODE}) · ${BuildConfig.ABI}",
                        Modifier.testTag("about_version"), color = p.textPrimary, style = Type.body,
                    )
                    Text(stringResource(R.string.about_independent), Modifier.padding(top = 8.dp), color = p.textSecondary, style = Type.label)
                }
                FlowRow(Modifier.padding(horizontal = 12.dp)) {
                    BracketButton(stringResource(R.string.about_source), Modifier.testTag("about_source")) { open(context, SOURCE_URL) }
                    BracketButton(stringResource(R.string.about_upstream), Modifier.testTag("about_upstream")) { open(context, UPSTREAM_URL) }
                }
            }
            item(key = "license") {
                Column {
                    SectionRule(stringResource(R.string.about_license))
                    Help(stringResource(R.string.about_license_help))
                    NoticeRow(Notice(stringResource(R.string.about_apk), "GPL-3.0", "GPL-3.0.txt"), "about_gpl")
                    NoticeRow(Notice(stringResource(R.string.about_code), "MIT", "cliamp-x-MIT.txt"), "about_mit")
                }
            }
            item(key = "privacy") {
                Column {
                    SectionRule(stringResource(R.string.about_privacy))
                    Text(
                        stringResource(R.string.about_privacy_text), Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp).testTag("about_privacy"),
                        color = p.textPrimary, style = Type.body,
                    )
                }
            }
            item(key = "third") { SectionRule(stringResource(R.string.about_third_party)) }
            itemsIndexed(NOTICES + fonts, key = { i, _ -> "n$i" }) { i, n -> NoticeRow(n, "notice_$i") }
            item(key = "end") { Spacer(Modifier.height(16.dp)) }
        }
    }
}

/** A component and its license; a tap opens the license text. */
@Composable
private fun NoticeRow(n: Notice, tag: String) {
    val p = LocalPalette.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = Look.row(48.dp)).clickable { Nav.open(Screen.License(n.name, n.file)) }
            .padding(horizontal = 16.dp).testTag(tag),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text(n.name, Modifier.weight(1f), color = p.textPrimary, style = Type.body)
        Text(n.license, color = p.textSecondary, style = Type.caption)
        Text("›", color = p.textSecondary, style = Type.body)
    }
}

/** One license text from assets/licenses, a paragraph per row so a long one scrolls smoothly. */
@Composable
fun LicenseScreen(screen: Screen.License) {
    val p = LocalPalette.current
    val context = LocalContext.current
    val paragraphs by produceState<List<String>?>(null, screen.file) {
        value = withContext(Dispatchers.IO) {
            try {
                context.assets.open("licenses/${screen.file}").bufferedReader().use { it.readText() }.split(Regex("\n\\s*\n"))
            } catch (e: IOException) {
                emptyList()
            }
        }
    }
    Column(Modifier.fillMaxSize()) {
        TopBar(screen.title)
        val text = paragraphs
        when {
            text == null -> Loading()
            text.isEmpty() -> EmptyState(stringResource(R.string.about_no_license, screen.file))
            else -> LazyColumn(Modifier.fillMaxSize().testTag("license_text")) {
                items(text) { Text(it.trim(), Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp), color = p.textPrimary, style = Type.label) }
            }
        }
    }
}
