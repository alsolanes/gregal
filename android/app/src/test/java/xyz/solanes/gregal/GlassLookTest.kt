package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.junit4.createAndroidComposeRule
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
import kotlin.math.abs

/**
 * El Vidre s'ha de notar: la llum del fons es veu clarament damunt del color
 * de base (amb el Vidre d'abans, amb prou feines). Captures a `build/glass-look/`.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h640dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class GlassLookTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val reply = "## Tres excursions\n\nLa millor tria depèn de quant vulguis caminar.\n\n- **Montserrat**: cremallera des de Monistrol.\n- **Cadaqués**: el poble blanc i el Cap de Creus."

    private fun capture(name: String, pal: GregalPalette): Bitmap {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    GregalBg {
                        Column(Modifier.fillMaxSize().padding(12.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                            WindBubble(GregalViewModel.PetMsg("user", "Què puc fer dissabte?", ts = 1, id = "u-1"), Color(0xFF8193FF), tail = true, showMeta = false)
                            WindBubble(GregalViewModel.PetMsg("pet", reply, ts = 2, id = "m-1"), Color(0xFF8193FF), tail = true, showMeta = false)
                            GlassCard(Modifier.fillMaxWidth().testTag("card")) { Text("Itinerari Girona · PDF", color = pal.ink, modifier = Modifier.padding(16.dp)) }
                        }
                    }
                }
            }
        }
        ui.mainClock.advanceTimeBy(600)
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        // La mesura de llum mira el terç de baix: hi ha d'estar buit.
        val card = ui.onNodeWithTag("card").fetchSemanticsNode().boundsInRoot
        assertTrue("el contingut no ha d'arribar al terç de baix", card.bottom < bmp.height * 2 / 3)
        File("build/glass-look/$name.png").also { it.parentFile?.mkdirs() }.outputStream().use { bmp.compress(Bitmap.CompressFormat.PNG, 100, it) }
        return bmp
    }

    /** La diferència més gran, en un canal de color, entre el fons buit (meitat de baix) i el color de base. */
    private fun maxLight(bmp: Bitmap, base: Color): Int {
        val b = intArrayOf((base.red * 255).toInt(), (base.green * 255).toInt(), (base.blue * 255).toInt())
        var best = 0
        for (y in bmp.height * 2 / 3 until bmp.height step 8) for (x in 0 until bmp.width step 8) {
            val p = bmp.getPixel(x, y)
            val d = maxOf(abs(((p shr 16) and 0xFF) - b[0]), abs(((p shr 8) and 0xFF) - b[1]), abs((p and 0xFF) - b[2]))
            best = maxOf(best, d)
        }
        return best
    }

    @Test fun `en fosc la llum del Vidre es veu`() {
        val light = maxLight(capture("dark", DarkPal.glassy()), DarkPal.deepTop)
        assertTrue("llum del fons: $light", light >= 60) // amb el Vidre d'abans, 43 com a molt
    }

    /** El cantell de dalt d'una targeta, més clar que el del costat. */
    @Test fun `en fosc la targeta de vidre té el cantell de dalt il·luminat`() {
        val bmp = capture("dark", DarkPal.glassy())
        val card = ui.onNodeWithTag("card").fetchSemanticsNode().boundsInRoot
        fun lum(x: Float, y: Float): Int { val p = bmp.getPixel(x.toInt(), y.toInt()); return ((p shr 16) and 0xFF) + ((p shr 8) and 0xFF) + (p and 0xFF) }
        val top = lum(card.center.x, card.top + 1)
        val side = lum(card.left + 1, card.center.y)
        assertTrue("dalt $top, costat $side", top - side >= 30)
    }

    @Test fun `en clar la llum del Vidre es veu`() {
        val light = maxLight(capture("light", LightPal.glassy()), LightPal.deepTop)
        assertTrue("llum del fons: $light", light >= 50) // amb el Vidre d'abans, 43 com a molt
    }
}
