package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
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

    private fun show() {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    Column(Modifier.fillMaxSize().background(DarkPal.abyss)) {
                        GregalFloatingHeader("ca", Color(0xFF8193FF), WindStyle(), WindMood.IDLE, false, {}, {}, {})
                        Spacer(Modifier.height(16.dp))
                        // El calaix fa 300 dp; el selector hi va amb 6 dp de marge.
                        Box(Modifier.width(300.dp).background(DarkPal.surface).padding(vertical = 12.dp)) {
                            ModeSwitcher("ca", AppArea.Vents, {}, Modifier.fillMaxWidth().padding(horizontal = 6.dp).testTag("area-switcher"))
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
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        File("build/mode-look/header-and-drawer.png").also { it.parentFile?.mkdirs() }.outputStream().use {
            Bitmap.createBitmap(bmp, 0, 0, bmp.width, (bmp.height * 0.35f).toInt()).compress(Bitmap.CompressFormat.PNG, 100, it)
        }
    }
}
