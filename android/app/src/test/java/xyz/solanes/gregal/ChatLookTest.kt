package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.unit.Density
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
 * Captures del fil de l'assistent (B1) a `build/chat-look/`: el mateix fil en
 * clar i fosc a 360 dp, i a 320 dp amb la lletra al 200 %. També comprova que
 * a 320 dp/200 % els controls de la capçalera i del compositor tenen target
 * tàctil sencer i que res no surt de la pantalla.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w400dp-h3600dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class ChatLookTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val now = System.currentTimeMillis() / 1000
    private val thread = listOf(
        CapMsg("user", "Prepara'm el viatge de divendres a Girona", ts = now - 7200, id = "m-1"),
        CapMsg("cap", "Hi poso. Miro trens i l'hotel i et dic alguna cosa.", ts = now - 7190, id = "m-2"),
        CapMsg("cap", "Si vols, també puc reservar taula per sopar.", ts = now - 7185, id = "m-3"),
        CapMsg("user", "Sí, a prop de l'hotel", ts = now - 7100, id = "m-4"),
        CapMsg("cap", "Opcions que he trobat:\n\n| Tren | Surt | Arriba |\n|---|---|---|\n| R11 | 17:12 | 18:35 |\n| AVE | 17:40 | 18:18 |\n",
            ts = now - 7000, id = "m-5"),
        CapMsg("cap", "Sobre el viatge · He trobat el trasllat que faltava: el bus directe et deixa a 5 minuts de l'hotel.",
            ts = now - 600, id = "m-6"),
    )

    @Composable
    private fun Look(pal: GregalPalette, width: Int, fontScale: Float) {
        val d = LocalDensity.current
        CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false,
            LocalDensity provides Density(d.density, fontScale)) {
            MaterialTheme(colorScheme = pal.scheme()) {
                Column(Modifier.width(width.dp).background(pal.surface)) {
                    VentsChatHeader("ca", "Gregal", "Preparant l'itinerari", Color(0xFF8193FF), WindStyle(), WindMood.IDLE,
                        onBack = {}, onProfile = {}, centered = true, onComputer = {})
                    Column(Modifier.padding(horizontal = 12.dp, vertical = 10.dp)) {
                        val slots = threadSlots(thread.map { ThreadEntry(it.from, it.ts) })
                        thread.forEachIndexed { i, m ->
                            val s = slots[i]
                            Column(Modifier.padding(top = if (i == 0) 0.dp else if (s.firstOfRun) GregalChat.gapBetween else GregalChat.gapInRun)) {
                                if (i == 5) ThreadDivider(threadCopy("ca", "unread"), accent = true)
                                s.separatorAt?.let { ThreadDivider(threadSeparatorLabel("ca", it)) }
                                WindBubble(GregalViewModel.PetMsg(if (m.from == "user") "user" else "pet", m.text, ts = m.ts, id = m.id),
                                    Color(0xFF8193FF), tail = s.lastOfRun, showMeta = s.lastOfRun)
                            }
                        }
                        Spacer(Modifier.height(GregalChat.gapBetween))
                        ResultCard("Itinerari Girona v2", "PDF · 2 pàgines", kind = resultKindOf("itinerari.pdf"),
                            footer = threadCopy("ca", "ready"), footerAction = threadCopy("ca", "seeWork"), onFooter = {}, onOpen = {})
                    }
                    VentsComposer("ca", TextFieldValue(""), {}, "Escriu a Gregal…", canSend = false,
                        workCount = 2, onWork = {}) {}
                }
            }
        }
    }

    private fun capture(name: String, pal: GregalPalette, width: Int = 360, fontScale: Float = 1f) {
        // Les cares i els anells animen per sempre: el rellotge, a mà.
        ui.mainClock.autoAdvance = false
        ui.setContent { Look(pal, width, fontScale) }
        ui.mainClock.advanceTimeBy(600)
        // captureToImage espera un redibuix que Robolectric no dona: es pinta la vista a mà.
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        val out = File("build/chat-look/$name.png")
        out.parentFile?.mkdirs()
        // Retalla el blanc de sota: la finestra és alta perquè hi càpiga tot.
        val w = (width * bmp.width / 400f).toInt().coerceAtMost(bmp.width)
        val h = ui.onNodeWithTag("vents-composer-field").getBoundsInRoot().bottom.value
            .let { (it + 24) * bmp.width / 400f }.toInt().coerceIn(1, bmp.height)
        out.outputStream().use { Bitmap.createBitmap(bmp, 0, 0, w, h).compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    @Test fun clar360() = capture("after-light-360", LightPal)
    @Test fun fosc360() = capture("after-dark-360", DarkPal)
    @Test fun clar320Lletra200() = capture("after-light-320-font200", LightPal, width = 320, fontScale = 2f)

    @Test fun targetsTactilsA320i200() {
        ui.mainClock.autoAdvance = false
        ui.setContent { Look(LightPal, 320, 2f) }
        ui.mainClock.advanceTimeBy(600)
        for (label in listOf(tr("ca", "app.vents.back"), threadCopy("ca", "computer"))) {
            val b = ui.onNodeWithContentDescription(label).assertIsDisplayed().getBoundsInRoot()
            assertTrue("$label massa petit", b.right - b.left >= 44.dp && b.bottom - b.top >= 44.dp)
            assertTrue("$label surt de la pantalla", b.right <= 320.dp)
        }
        ui.onNodeWithTag("result-card").assertIsDisplayed().assertHasClickAction()
        ui.onNodeWithTag("work-strip").assertHasClickAction()
        val strip = ui.onNodeWithTag("work-strip").getBoundsInRoot()
        assertTrue(strip.bottom - strip.top >= GregalChat.workStrip)
        ui.onNodeWithTag("result-card-footer").getBoundsInRoot().let { assertTrue("peu: " + (it.bottom - it.top) + " " + it, it.bottom - it.top >= 48.dp) }
    }
}
