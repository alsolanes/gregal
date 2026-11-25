package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
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
 * La fila de cares de la portada de Vents (tasca 133): hi són tots els vents,
 * llisca amb imant, respiren desacompassades, diuen qui treballa i la cara
 * tocada vola fins a la capçalera del xat. Captures a `build/vents-start-faces/`.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class VentsStartFacesTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val winds = listOf(
        Wind("gregal", "Gregal", "#8193FF", "Assistent"), Wind("xaloc", "Xaloc", "#EFA68F", "Música"),
        Wind("ponent", "Ponent", "#7FC8A9", "Cuina"), Wind("garbi", "Garbí", "#F2C14E", "Viatges"),
        Wind("mestral", "Mestral", "#6EA8FE", "Esport"), Wind("tramuntana", "Tramuntana", "#C792EA", "Casa"),
        Wind("llevant", "Llevant", "#F78C6C", "Feina"), Wind("migjorn", "Migjorn", "#89DDFF", "Salut"),
    )

    private fun show(pal: GregalPalette, motion: Boolean = false, onWind: (Wind, Rect) -> Unit = { _, _ -> }) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides motion) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    GregalBg {
                        VentsStartPage("ca", "Anna", winds, emptyList(), "", {}, busy = false,
                            onSend = {}, onWind = onWind, onThread = {}, working = setOf("xaloc"))
                    }
                }
            }
        }
        ui.mainClock.advanceTimeBy(600)
    }

    private fun save(name: String) {
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        File("build/vents-start-faces/$name.png").also { it.parentFile?.mkdirs() }.outputStream().use { bmp.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    @Test fun `hi son tots els vents, tambe els que no hi caben`() {
        show(DarkPal)
        save("dark")
        ui.onNodeWithTag("start-wind-gregal").assertIsDisplayed()
        // A 360 dp no hi caben vuit cares: l'última s'hi arriba lliscant.
        // Lliscar la fila necessita fotogrames: sense moviment no hi ha cap animació infinita que ho impedeixi.
        ui.mainClock.autoAdvance = true
        ui.onNodeWithTag("start-faces").performScrollToNode(hasTestTag("start-wind-migjorn"))
        ui.onNodeWithTag("start-wind-migjorn").assertIsDisplayed()
        save("dark-scrolled")
    }

    @Test fun `en deixar anar la fila una cara queda alineada a l'esquerra`() {
        show(LightPal)
        save("light")
        val row = ui.onNodeWithTag("start-faces").fetchSemanticsNode().boundsInRoot
        ui.onNodeWithTag("start-faces").performTouchInput { swipeLeft(startX = right - 10f, endX = right - 10f - width * .37f, durationMillis = 400) }
        ui.mainClock.autoAdvance = true
        ui.mainClock.advanceTimeBy(3000)
        ui.waitForIdle()
        val lefts = winds.mapNotNull { w ->
            // La posició sense retallar: la cara que surt per l'esquerra també compta.
            ui.onAllNodesWithTag("start-wind-" + w.id).fetchSemanticsNodes().firstOrNull()?.positionInRoot?.x
        }
        val edge = row.left + 12f * ui.density.density
        assertTrue("ha lliscat: $lefts", lefts.first() < edge - 1f)
        val first = lefts.filter { it >= edge - 4f }.min()
        assertEquals("la primera cara sencera queda a la vora (imant): $lefts", edge, first, 2f)
    }

    @Test fun `un vent que treballa porta un punt i ho diu`() {
        show(DarkPal)
        ui.onNodeWithTag("start-wind-busy-xaloc", useUnmergedTree = true).assertExists()
        ui.onNodeWithTag("start-wind-busy-gregal", useUnmergedTree = true).assertDoesNotExist()
        ui.onNodeWithTag("start-wind-xaloc").assert(SemanticsMatcher.expectValue(SemanticsProperties.StateDescription, "Treballant…"))
    }

    @Test fun `tocar una cara diu quin vent i d'on surt la cara`() {
        var got: Pair<Wind, Rect>? = null
        show(LightPal) { w, r -> got = w to r }
        ui.onNodeWithTag("start-wind-xaloc").performClick()
        val (wind, from) = requireNotNull(got)
        assertEquals("xaloc", wind.id)
        val face = ui.onNodeWithTag("start-wind-xaloc").fetchSemanticsNode().boundsInRoot
        assertTrue("la cara és dins de la columna tocada: $from a $face", face.contains(from.center) && from.width > 0f)
    }

    @Test fun `les cares respiren desacompassades`() {
        // Al mateix instant, dues cares veïnes no són a la mateixa alçada de l'alè.
        for (t in listOf(0f, .7f, 1.9f)) assertNotEquals(startFaceBreath(t, 0), startFaceBreath(t, 1), 1e-4f)
        // I cada una respira: torna on era al cap d'un cicle, i no es queda quieta.
        assertEquals(startFaceBreath(.3f, 2), startFaceBreath(.3f + StartFaceBreathPeriod, 2), 1e-4f)
        val samples = (0 until 20).map { startFaceBreath(it * StartFaceBreathPeriod / 20f, 0) }
        assertTrue("s'infla i es desinfla: $samples", samples.max() - samples.min() > .02f)
        assertTrue("però poc: $samples", samples.all { it in .97f..1.03f })
    }

    private fun land(motion: Boolean, onLanded: () -> Unit) {
        val perch = WindPerch().apply { root = Offset.Zero; header = Offset(300f, 40f) }
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides motion) {
                Box(Modifier.fillMaxSize()) {
                    FaceLanding(perch, Rect(40f, 600f, 148f, 708f), Color(0xFFEFA68F), WindStyle(), onLanded)
                }
            }
        }
    }

    @Test fun `la cara tocada vola des de la fila fins a la capcalera del xat`() {
        var landed = false
        land(motion = true) { landed = true }
        ui.mainClock.advanceTimeBy(16)
        val from = Offset(94f, 654f)
        // El centre del forat de la capçalera: 22 dp més enllà de la cantonada.
        val to = Offset(300f, 40f) + Offset(22f, 22f) * ui.density.density
        val start = ui.onNodeWithTag("face-landing").fetchSemanticsNode().boundsInRoot.center
        assertTrue("surt d'on era la cara: $start", (start - from).getDistance() < (start - to).getDistance() / 4f)
        ui.mainClock.advanceTimeBy(150)
        val mid = ui.onNodeWithTag("face-landing").fetchSemanticsNode().boundsInRoot.center
        assertTrue("va cap a la capçalera: $start, $mid", (mid - to).getDistance() < (start - to).getDistance() - 50f)
        assertTrue(!landed)
        ui.mainClock.advanceTimeBy(120)
        save("landing")
        ui.mainClock.advanceTimeBy(1500)
        assertTrue("arriba i s'acaba", landed)
    }

    @Test fun `sense moviment la cara no vola`() {
        var landed = false
        land(motion = false) { landed = true }
        ui.mainClock.advanceTimeBy(16)
        assertTrue(landed)
        ui.onNodeWithTag("face-landing").assertDoesNotExist()
    }
}
