package io.github.xaviiromf.cliampx.ui

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.PowerManager
import android.widget.Toast
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LifecycleResumeEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.xaviiromf.cliampx.CliampService
import io.github.xaviiromf.cliampx.LoginActivity
import io.github.xaviiromf.cliampx.MainActivity
import io.github.xaviiromf.cliampx.R
import io.github.xaviiromf.cliampx.Remote

private const val PAGES = 3

/**
 * The first-run onboarding (FINAL-PLAN T9): the permissions, the YouTube
 * Music sign-in or Omitir, and a skin. The core starts behind it, so a
 * picked skin shows at once when the core is up; done gets the skin to
 * apply once it is.
 */
@Composable
fun Onboarding(done: (Skin?) -> Unit) {
    val p = LocalPalette.current
    val context = LocalContext.current
    val ops by Remote.ops.collectAsStateWithLifecycle()
    val signedIn by Remote.signedIn.collectAsStateWithLifecycle()
    var page by rememberSaveable { mutableIntStateOf(0) }
    var chosen by rememberSaveable { mutableStateOf<String?>(null) }
    BackHandler(page > 0) { page-- }
    Box(Modifier.fillMaxSize().testTag("onboarding")) {
        Backdrop(Looks.listLevel, Modifier.matchParentSize(), paused = false)
        Column(Modifier.fillMaxSize().windowInsetsPadding(WindowInsets.safeDrawing)) {
            Text(stringResource(R.string.onboard_welcome), Modifier.padding(start = 16.dp, top = 16.dp, end = 16.dp), color = p.accent, style = Type.logo)
            Text(stringResource(R.string.onboard_step, page + 1, PAGES), Modifier.padding(horizontal = 16.dp), color = p.textSecondary, style = Type.caption)
            Column(Modifier.weight(1f).verticalScroll(rememberScrollState())) {
                when (page) {
                    0 -> PermissionsPage()
                    1 -> SignInPage(signedIn == true)
                    else -> {
                        ScreenRule(stringResource(R.string.onboard_skin))
                        Help(stringResource(R.string.onboard_skin_help))
                        for (skin in Skins.BUILT_IN) {
                            SkinRow(skin, stringResource(skin.label), null, chosen == skin.name, "onboard_skin_" + skin.name) {
                                chosen = skin.name
                                if ("theme" in ops) Skins.apply(context, skin) // a live preview once the core is up
                            }
                        }
                    }
                }
            }
            Row(Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                if (page > 0) BracketButton(stringResource(R.string.back), Modifier.testTag("onboard_back")) { page-- }
                Spacer(Modifier.weight(1f))
                when {
                    page < PAGES - 1 -> {
                        val label = if (page == 1 && signedIn != true) R.string.skip else R.string.next
                        KeyPill(stringResource(label), Modifier.testTag("onboard_next")) { page++ }
                    }
                    else -> KeyPill(stringResource(R.string.onboard_start), Modifier.testTag("onboard_done")) { done(Skins.BUILT_IN.find { it.name == chosen }) }
                }
            }
        }
    }
}

/** Notifications (Android 13+), local music and the battery optimization, each with its state. */
@Composable
private fun PermissionsPage() {
    val context = LocalContext.current
    fun granted(permission: String) = ContextCompat.checkSelfPermission(context, permission) == PackageManager.PERMISSION_GRANTED
    var notify by remember { mutableStateOf(true) }
    var music by remember { mutableStateOf(true) }
    var battery by remember { mutableStateOf(true) }
    LifecycleResumeEffect(Unit) {
        // Back from a permission dialog or the battery setting: read them again.
        notify = Build.VERSION.SDK_INT < 33 || granted(Manifest.permission.POST_NOTIFICATIONS)
        music = granted(MainActivity.musicPermission())
        battery = context.getSystemService(PowerManager::class.java).isIgnoringBatteryOptimizations(context.packageName)
        onPauseOrDispose {}
    }
    val askNotify = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { notify = it }
    // The core reads the music folder when it starts.
    val askMusic = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) {
        music = it
        if (it) CliampService.send(context, CliampService.ACTION_RESTART)
    }
    ScreenRule(stringResource(R.string.onboard_permissions))
    Help(stringResource(R.string.onboard_permissions_help))
    if (Build.VERSION.SDK_INT >= 33) {
        PermissionRow(stringResource(R.string.notifications), stringResource(R.string.notifications_help), notify, stringResource(R.string.allow), "onboard_notify") {
            askNotify.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
    }
    PermissionRow(stringResource(R.string.lib_local), stringResource(R.string.music_help), music, stringResource(R.string.allow), "onboard_music") {
        askMusic.launch(MainActivity.musicPermission())
    }
    PermissionRow(stringResource(R.string.battery), stringResource(R.string.battery_card), battery, stringResource(R.string.battery_short), "onboard_battery") {
        ignoreBatteryOptimizations(context)
    }
}

/** A permission: its name, ✓ when given or the button that asks, and what it is for. */
@Composable
private fun PermissionRow(title: String, help: String, granted: Boolean, action: String, tag: String, ask: () -> Unit) {
    val p = LocalPalette.current
    Row(Modifier.fillMaxWidth().heightIn(min = 48.dp).padding(start = 16.dp, end = 12.dp, top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(title, Modifier.weight(1f), color = p.textPrimary, style = Type.bodyStrong)
        if (granted) Text("✓ " + stringResource(R.string.allowed), Modifier.padding(horizontal = 4.dp).testTag(tag + "_ok"), color = p.playing, style = Type.body)
        else BracketButton(action, Modifier.testTag(tag), onClick = ask)
    }
    Help(help)
}

/** YouTube Music: Iniciar sesión, or Importar cookies.txt; Omitir goes on without it. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun SignInPage(signedIn: Boolean) {
    val p = LocalPalette.current
    val context = LocalContext.current
    val importCookies = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) Toast.makeText(context, LoginActivity.importCookies(context, uri), Toast.LENGTH_LONG).show()
    }
    ScreenRule(stringResource(R.string.youtube_music))
    Text(
        (if (signedIn) "✓ " else "") + stringResource(if (signedIn) R.string.signed_in_state else R.string.signed_out_state),
        Modifier.padding(horizontal = 16.dp, vertical = 8.dp).testTag("onboard_signin_state"),
        color = if (signedIn) p.playing else p.textSecondary, style = Type.bodyStrong,
    )
    Help(stringResource(R.string.sign_in_card))
    FlowRow(Modifier.padding(horizontal = 12.dp)) {
        BracketButton(stringResource(R.string.sign_in_short), Modifier.testTag("onboard_signin")) {
            context.startActivity(Intent(context, LoginActivity::class.java))
        }
        BracketButton(stringResource(R.string.import_cookies), Modifier.testTag("onboard_import")) {
            importCookies.launch(arrayOf("text/*", "application/octet-stream"))
        }
    }
    Help(stringResource(R.string.youtube_music_help))
    Help(stringResource(R.string.onboard_signin_help))
}
