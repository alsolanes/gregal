package xyz.solanes.gregal

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.unit.dp
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class VentsChatChromeTest {
    @get:Rule val ui = createComposeRule()

    @Test fun composerRetainsDraftAndOnlySendsNonBlankText() {
        var sent = ""
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    var field by remember { mutableStateOf(TextFieldValue("")) }
                    VentsComposer("ca", field, { field = it }, "Escriu…", field.text.isNotBlank()) { sent = field.text }
                }
            }
        }
        ui.onNodeWithContentDescription(tr("ca", "app.vents.send")).assertIsNotEnabled()
        ui.onNodeWithText("Escriu…").performTextInput("@Xaloc una idea")
        ui.onNodeWithContentDescription(tr("ca", "app.vents.send")).assertIsEnabled().performClick()
        ui.runOnIdle { assertEquals("@Xaloc una idea", sent) }
    }

    @Test fun compactHeaderKeepsBackAndProfileAccessible() {
        var back = false
        var profile = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    VentsChatHeader("ca", "Xaloc", "Música", Color(0xFFEFA68F), WindStyle(), WindMood.IDLE,
                        onBack = { back = true }, onProfile = { profile = true })
                }
            }
        }
        ui.onNodeWithText("Xaloc").assertIsDisplayed()
        ui.onNodeWithContentDescription(tr("ca", "app.vents.profile")).performClick()
        ui.onNodeWithContentDescription(tr("ca", "app.vents.back")).performClick()
        ui.runOnIdle { assertTrue(profile); assertTrue(back) }
    }

    @Test fun composerWithToolsExposesModeWorkspaceAndPlusMenu() {
        var pickedMode = ""
        var openedPerm = false
        var openedWs = false
        var attached = false
        val st = ServerState(mode = "code", access = "edits", project = "gregal-live", branch = "main")
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    var field by remember { mutableStateOf(TextFieldValue("")) }
                    VentsComposer(
                        "ca", field, { field = it }, "Escriu…", false,
                        busy = true, accent = Color(0xFFEFA68F), st = st,
                        onAttachDoc = { attached = true },
                        onMode = { pickedMode = it },
                        onPerm = { openedPerm = true },
                        onWorkspace = { openedWs = true },
                    ) {}
                }
            }
        }
        ui.onNodeWithText("⚡ Actua").assertIsDisplayed().performClick()
        ui.runOnIdle { assertEquals("chat", pickedMode) }
        ui.onNodeWithText("gregal-live · main").assertIsDisplayed().performClick()
        ui.runOnIdle { assertTrue(openedWs) }
        ui.onNodeWithContentDescription(tr("ca", "app.mode.more")).performClick()
        ui.onNodeWithText(tr("ca", "app.composer.attachDoc")).assertIsDisplayed().performClick()
        ui.runOnIdle { assertTrue(attached) }
        ui.onNodeWithContentDescription(tr("ca", "app.mode.more")).performClick()
        ui.onNodeWithText(tr("ca", "app.perm.title") + " · " + tr("ca", "app.perm.edits")).performClick()
        ui.runOnIdle { assertTrue(openedPerm) }
    }

    @Test fun tapsOnLinkChipCopyTheRealUrlToTheClipboard() {
        val url = "https://ca.wikipedia.org/wiki/Roure"
        var copied = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    androidx.compose.material3.Surface(color = androidx.compose.ui.graphics.Color(0xFF13151B)) {
                        androidx.compose.foundation.layout.Column(
                            androidx.compose.ui.Modifier.fillMaxSize().padding(12.dp),
                        ) {
                            MessageLinks("ca", "Mira [el roure]($url) i també https://example.org/informe.pdf. Prou.", Color(0xFFEFA68F)) { copied = true }
                        }
                    }
                }
            }
        }
        // Captura real del render (píxels), no només semàntica.
        ui.waitForIdle()
        ui.runOnIdle {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).single()
            val view = activity.window.decorView
            val bmp = android.graphics.Bitmap.createBitmap(view.width, view.height, android.graphics.Bitmap.Config.ARGB_8888)
            view.draw(android.graphics.Canvas(bmp))
            java.io.File("build/link-ui", "chips-dark.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
                bmp.compress(android.graphics.Bitmap.CompressFormat.PNG, 100, it)
            }
        }
        val chip = ui.onNodeWithContentDescription(tr("ca", "app.vents.copyLink") + ": el roure")
        chip.assertIsDisplayed().performClick()
        ui.runOnIdle {
            assertTrue(copied)
            val cm = androidx.test.core.app.ApplicationProvider.getApplicationContext<android.app.Application>()
                .getSystemService(android.content.ClipboardManager::class.java)
            assertEquals(url, cm.primaryClip?.getItemAt(0)?.text?.toString())
        }
    }

    @Test fun aReplyWithoutLinksHasNoChipRow() {
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    MessageLinks("ca", "Cap enllaç aquí, només text.", Color(0xFFEFA68F))
                }
            }
        }
        ui.onAllNodesWithContentDescription(tr("ca", "app.vents.copyLink"), substring = true)
            .assertCountEquals(0)
    }

    @Test fun thePlusMenuOffersTheWebPreview() {
        var previewed = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    var field by remember { mutableStateOf(TextFieldValue("")) }
                    VentsComposer("ca", field, { field = it }, "Escriu…", false,
                        accent = Color(0xFFEFA68F), onPreview = { previewed = true }) { }
                }
            }
        }
        ui.onNodeWithContentDescription(tr("ca", "app.mode.more")).performClick()
        ui.onNodeWithText(tr("ca", "app.preview.open")).assertIsDisplayed().performClick()
        ui.runOnIdle { assertTrue("el menú + ha d'obrir la previsualització", previewed) }
    }

    @Test fun whileTheWindWorksTheSendButtonBecomesStop() {
        var stopped = false
        var sent = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    var field by remember { mutableStateOf(TextFieldValue("text")) }
                    VentsComposer("ca", field, { field = it }, "Escriu…", true,
                        busy = true, accent = Color(0xFFEFA68F), onStop = { stopped = true }) { sent = true }
                }
            }
        }
        ui.onNodeWithContentDescription(tr("ca", "app.vents.stop")).assertIsDisplayed().performClick()
        ui.runOnIdle { assertTrue("un toc mentre treballa ha d'aturar, no enviar", stopped && !sent) }
        ui.onAllNodesWithContentDescription(tr("ca", "app.vents.send")).assertCountEquals(0)
    }

    @Test fun withoutABusyTurnTheButtonSends() {
        var stopped = false
        var sent = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    var field by remember { mutableStateOf(TextFieldValue("text")) }
                    VentsComposer("ca", field, { field = it }, "Escriu…", true,
                        busy = false, accent = Color(0xFFEFA68F), onStop = { stopped = true }) { sent = true }
                }
            }
        }
        ui.onNodeWithContentDescription(tr("ca", "app.vents.send")).assertIsDisplayed().performClick()
        ui.runOnIdle { assertTrue("en repòs el botó envia", sent && !stopped) }
    }
}
