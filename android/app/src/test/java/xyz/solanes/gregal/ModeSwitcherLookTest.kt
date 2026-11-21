package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshots.Snapshot
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.unit.dp
import org.junit.Assert.assertEquals
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
 * Codi | Vents es canvia només des del calaix: el selector hi ocupa l'amplada
 * en dues meitats iguals, cadascuna amb la icona i el nom centrats.
 * Captura a `build/mode-look/`.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h640dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class ModeSwitcherLookTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private var pal by mutableStateOf(DarkPal)
    private var ventsDot by mutableStateOf(false)

    private fun show() {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    Column(Modifier.fillMaxSize().background(pal.abyss)) {
                        GregalFloatingHeader("ca", Color(0xFF8193FF), WindStyle(), WindMood.IDLE, false, {}, {}, {})
                        Spacer(Modifier.height(16.dp))
                        // El calaix fa 300 dp; el selector hi va amb 6 dp de marge.
                        Box(Modifier.width(300.dp).background(pal.surface).padding(vertical = 12.dp)) {
                            ModeSwitcher("ca", AppArea.Code, ventsDot, {}, Modifier.fillMaxWidth().padding(horizontal = 6.dp).testTag("area-switcher"))
                        }
                    }
                }
            }
        }
        ui.mainClock.advanceTimeBy(600)
    }

    @Test fun `les dues opcions del calaix ocupen meitats iguals amb el text centrat`() {
        show()
        val bar = ui.onNodeWithTag("area-switcher").fetchSemanticsNode().boundsInRoot
        val tabs = ui.onAllNodes(hasClickAction() and hasAnyAncestor(hasTestTag("area-switcher"))).fetchSemanticsNodes().map { it.boundsInRoot }
        assertEquals(2, tabs.size)
        assertEquals("meitats iguals", tabs[0].width, tabs[1].width, 2f)
        assertTrue("entre les dues omplen el selector", tabs[0].width + tabs[1].width >= bar.width - 16f)
        for (label in listOf("Vents", "Codi")) {
            val text = ui.onNodeWithText(label, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val tab = tabs.first { it.left <= text.left && text.right <= it.right }
            // Icona (24 dp) + 6 dp + text, centrats: el mateix buit a cada banda.
            val iconAndGap = 30 * ui.density.density
            val leftGap = text.left - iconAndGap - tab.left
            val rightGap = tab.right - text.right
            assertEquals("$label: centrat dins la seva meitat", leftGap, rightGap, 4f)
        }
        capture("header-and-drawer")
    }

    // La cara flotant ja no hi és: l'únic avís que un vent treballa o té
    // novetats quan ets a Codi és aquest punt a la meitat «Vents».
    @Test fun `la meitat Vents porta un punt quan un vent treballa o te novetats`() {
        show()
        val desc = "Un vent treballa o té novetats"
        ui.onNodeWithContentDescription(desc, useUnmergedTree = true).assertDoesNotExist()
        for ((name, colors) in listOf("dark" to DarkPal, "light" to LightPal)) {
            set { ventsDot = false; pal = colors }
            capture("drawer-$name")
            set { ventsDot = true }
            val dot = ui.onNodeWithContentDescription(desc, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val vents = ui.onNodeWithText("Vents", useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            assertTrue("el punt va al costat de «Vents»", dot.left >= vents.right && dot.width < 12 * ui.density.density)
            capture("drawer-$name-dot")
        }
        set { ventsDot = false }
        ui.onNodeWithContentDescription(desc, useUnmergedTree = true).assertDoesNotExist()
    }

    // Amb el rellotge aturat, els canvis d'estat fets des del test s'han
    // d'anunciar a mà perquè el pròxim fotograma recompongui.
    private fun set(change: () -> Unit) {
        change()
        Snapshot.sendApplyNotifications()
        ui.mainClock.advanceTimeBy(100)
    }

    private fun capture(name: String) {
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        File("build/mode-look/$name.png").also { it.parentFile?.mkdirs() }.outputStream().use {
            Bitmap.createBitmap(bmp, 0, 0, bmp.width, (bmp.height * 0.35f).toInt()).compress(Bitmap.CompressFormat.PNG, 100, it)
        }
    }
}
