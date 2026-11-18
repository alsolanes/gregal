package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.unit.dp
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode
import java.io.File

/**
 * El xat d'un vent amb la capçalera flotant (tornar, pastilla amb cara i nom,
 * «…») i «A fons» com a icona dins del compositor. Captures a `build/chat-look/`.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h640dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class FloatingChatHeaderLookTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    @Composable
    private fun Look(pal: GregalPalette) {
        val accent = Color(0xFFEFA68F)
        CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = pal.scheme()) {
                Box(Modifier.fillMaxSize().background(pal.surface)) {
                    Column(Modifier.fillMaxSize()) {
                        Column(Modifier.weight(1f).padding(start = 12.dp, end = 12.dp), verticalArrangement = Arrangement.Bottom) {
                            repeat(5) { i ->
                                WindBubble(GregalViewModel.PetMsg(if (i % 2 == 0) "pet" else "user",
                                    "Missatge $i: el fil passa per sota de la capçalera i es veu entre les pastilles.", ts = 0, id = "m$i"),
                                    accent)
                                Spacer(Modifier.height(8.dp))
                            }
                        }
                        VentsComposer("ca", TextFieldValue(""), {}, "Escriu a Xaloc…", canSend = false,
                            accent = accent, tempesta = true, onTempesta = {}) {}
                    }
                    VentsChatHeader("ca", "Xaloc", "Música i concerts", accent, WindStyle(), WindMood.IDLE,
                        onBack = {}, onProfile = {}, onSessions = {}, modifier = Modifier.align(Alignment.TopCenter))
                }
            }
        }
    }

    private fun capture(name: String, pal: GregalPalette) {
        ui.mainClock.autoAdvance = false
        ui.setContent { Look(pal) }
        ui.mainClock.advanceTimeBy(600)
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        val out = File("build/chat-look/$name.png")
        out.parentFile?.mkdirs()
        out.outputStream().use { bmp.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    @Test fun clar() = capture("floating-header-light", LightPal)
    @Test fun fosc() = capture("floating-header-dark", DarkPal)

    @Test fun headerIsShortAndDeepIsInsideTheComposer() {
        ui.mainClock.autoAdvance = false
        ui.setContent { Look(LightPal) }
        ui.mainClock.advanceTimeBy(600)
        val h = ui.onNodeWithTag("chat-header").getBoundsInRoot()
        assertTrue("capçalera massa alta: " + (h.bottom - h.top), h.bottom - h.top <= ChatHeaderInset)
        ui.onNodeWithTag("chat-header-pill").assertHasClickAction()
        val bar = ui.onNodeWithTag("vents-composer-bar").getBoundsInRoot()
        val deep = ui.onNodeWithTag("tempesta-toggle").assertIsDisplayed().getBoundsInRoot()
        assertTrue("«A fons» fora de la pastilla", deep.top >= bar.top && deep.bottom <= bar.bottom)
    }
}
