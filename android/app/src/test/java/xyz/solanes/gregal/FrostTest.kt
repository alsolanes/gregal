package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.unit.dp
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.After
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/**
 * El vidre esmerilat: la barra pinta a sota, desenfocat, el contingut que té
 * darrere. Es comprova amb una vora nítida vermell/blau: just a tocar de la
 * vora, sota la barra, el vermell ha de quedar barrejat amb blau.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w200dp-h200dp-mdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class FrostTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    @Composable
    private fun Look(pal: GregalPalette) {
        CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
            val frost = rememberFrost()
            Box(Modifier.fillMaxSize()) {
                Row(Modifier.fillMaxSize().frostSource(frost)) {
                    Box(Modifier.weight(1f).fillMaxHeight().background(Color.Red))
                    Box(Modifier.weight(1f).fillMaxHeight().background(Color.Blue))
                }
                Box(Modifier.fillMaxWidth().height(60.dp).frostGlass(frost).testTag("bar"))
            }
        }
    }

    /** Blau del píxel 3 px a l'esquerra de la vora, al mig de la barra. */
    // Captura amb HardwareRenderer: el dibuix per programari no aplica RenderEffect.
    @Before fun hardware() { System.setProperty("robolectric.pixelCopyRenderMode", "hardware") }

    @After fun software() { System.clearProperty("robolectric.pixelCopyRenderMode") }

    /** El píxel 3 px a l'esquerra de la vora, al mig de la barra (r, g, b de 0 a 1). */
    private fun leftOfEdge(): FloatArray {
        ui.waitForIdle()
        val bar = ui.onNodeWithTag("bar").fetchSemanticsNode().boundsInRoot
        val view = ui.activity.window.decorView
        val bmp = android.graphics.Bitmap.createBitmap(view.width, view.height, android.graphics.Bitmap.Config.ARGB_8888)
        android.view.PixelCopy.request(ui.activity.window, bmp, {}, android.os.Handler(android.os.Looper.getMainLooper()))
        org.robolectric.Shadows.shadowOf(android.os.Looper.getMainLooper()).idle()
        val px = bmp.getPixel((bar.left + bar.right).toInt() / 2 - 3, (bar.top + bar.bottom).toInt() / 2)
        return floatArrayOf(android.graphics.Color.red(px) / 255f, android.graphics.Color.green(px) / 255f, android.graphics.Color.blue(px) / 255f)
    }

    @Test fun ambVidreLaBarraDesenfocaElQueTeDarrere() {
        ui.setContent { Look(DarkPal.glassy()) }
        val (_, _, blue) = leftOfEdge()
        assertTrue("la vora hauria de quedar difuminada (blau $blue)", blue > 0.15f)
    }

    @Test fun senseVidreNoEsDesenfocaRes() {
        ui.setContent { Look(DarkPal) }
        val (red, _, blue) = leftOfEdge()
        assertTrue("captura buida (vermell $red)", red > 0.8f)
        assertEquals(0f, blue, 0.01f)
    }

    @Test @Config(sdk = [30])
    fun abansDAndroid12NoEsDesenfocaRes() {
        ui.setContent { Look(DarkPal.glassy()) }
        val (red, _, blue) = leftOfEdge()
        assertTrue("captura buida (vermell $red)", red > 0.8f)
        assertEquals(0f, blue, 0.01f)
    }

    // El contingut real (bombolles damunt de transparent) no s'ha de veure nítid sota la barra.
    @Test fun elContingutNitidNoTraspassaLaBarra() {
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal.glassy(), LocalWindMotion provides false) {
                val frost = rememberFrost()
                Box(Modifier.fillMaxSize()) {
                    Box(Modifier.fillMaxSize().frostSource(frost)) {
                        Box(Modifier.align(Alignment.TopCenter).padding(top = 26.dp).size(8.dp).background(Color.Red))
                    }
                    Box(Modifier.fillMaxWidth().height(60.dp).frostGlass(frost).testTag("bar"))
                }
            }
        }
        val (red, _, _) = leftOfEdge()
        assertTrue("la bombolla es veu nítida sota la barra (vermell $red)", red < 0.6f)
    }

    @Test fun ambVidreLaLlistaPassaPerSotaDeLesBarres() {
        val bars = PaddingValues(top = 64.dp, bottom = 80.dp)
        val (outer, inner) = chatListPadding(bars, frosted = true)
        assertEquals(0.dp, outer.calculateTopPadding())
        assertEquals(0.dp, outer.calculateBottomPadding())
        assertEquals(64.dp, inner.calculateTopPadding())
        assertEquals(80.dp, inner.calculateBottomPadding())
        val (outer2, inner2) = chatListPadding(bars, frosted = false)
        assertEquals(64.dp, outer2.calculateTopPadding())
        assertEquals(80.dp, outer2.calculateBottomPadding())
        assertEquals(0.dp, inner2.calculateTopPadding())
    }
}
