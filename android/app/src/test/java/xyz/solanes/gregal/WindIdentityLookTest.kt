package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.getBoundsInRoot
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.unit.Dp
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.MutableStateFlow
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
import kotlin.math.abs

/**
 * 131: el vent és qui pregunta i qui parla. La targeta de permís porta la seva
 * cara, la vora i el Permet del seu color (text a 4.5:1); al grup, la cara
 * petita només al primer missatge de cada ratxa i la mateixa regla per a una
 * resposta curta que per a una de llarga. Fotogrames clar i fosc a
 * output/ui-review/131/.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h1400dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindIdentityLookTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()
    private val xaloc = Wind("xaloc", "Xaloc", "#EFA68F", "Música i concerts")
    private val gregal = Wind("gregal", "Gregal", "#3D7BFF", "")

    private fun <T> flow(vm: GregalViewModel, name: String) = GregalViewModel::class.java.getDeclaredField(name)
        .apply { isAccessible = true }.get(vm) as MutableStateFlow<T>

    private fun vm() = GregalViewModel(ApplicationProvider.getApplicationContext<Application>()).apply { setConn(Conn("127.0.0.1", "9", "t")) }

    private fun frame(name: String): Bitmap {
        ui.mainClock.advanceTimeBy(600)
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        File("../../output/ui-review/131/$name.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
            bmp.compress(Bitmap.CompressFormat.PNG, 100, it)
        }
        return bmp
    }

    private fun px(d: Dp) = with(ui.density) { d.toPx() }
    private fun near(a: Int, b: Color, what: String) {
        val c = Color(a)
        assertTrue("$what: $c vs $b", abs(c.red - b.red) < .02f && abs(c.green - b.green) < .02f && abs(c.blue - b.blue) < .02f)
    }

    /** A Feina (sense xat del vent): la targeta és de qui pregunta, no de l'accent de la pantalla. */
    private fun ask(name: String, pal: GregalPalette, wind: Wind) {
        val vm = vm()
        flow<List<Wind>>(vm, "_winds").value = listOf(xaloc, gregal)
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.Ask(GregalApi.WindAsk("wa1", "approve", name = "mail", kind = "rule",
            what = "mail send", detail = "Per a: llista@example.org\nAssumpte: Concert de dissabte",
            reason = "Em vas dir que et pregunti abans d'enviar cap correu.", wind = wind.id)))
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    Box(Modifier.fillMaxSize().background(pal.deepTop)) { WindAskCards(vm, "ca", pal.sea, busy = false) }
                }
            }
        }
        val bmp = frame(name)
        val c = Color(wind.argb)
        ui.onNodeWithTag("ask-face", useUnmergedTree = true).assert(SemanticsMatcher.expectValue(WindFaceKey, c))
        // La vora esquerra de 3 dp, del color del vent.
        val card = ui.onNodeWithTag("approve-card").getBoundsInRoot()
        near(bmp.getPixel(px(card.left + Dp(2f)).toInt(), px((card.top + card.bottom) / 2).toInt()), c, "edge")
        // Permet: el color del vent (o tant a prop com permeti el 4.5:1) i la tinta que hi llegeix.
        val (fill, ink) = allowPaint(c, pal.ink, pal.deepTop)
        val allow = ui.onNodeWithTag("ask-allow").getBoundsInRoot()
        near(bmp.getPixel(px(allow.left + Dp(6f)).toInt(), px((allow.top + allow.bottom) / 2).toInt()), fill, "allow")
        assertTrue(contrastWith(fill, ink) >= 4.5)
    }

    @Test fun permisXalocClar() = ask("ask-xaloc-light", LightPal, xaloc)
    @Test fun permisXalocFosc() = ask("ask-xaloc-dark", DarkPal, xaloc)
    @Test fun permisGregalClar() = ask("ask-gregal-light", LightPal, gregal)
    @Test fun permisGregalFosc() = ask("ask-gregal-dark", DarkPal, gregal)

    private val long = "Dissabte hi ha tres concerts a prop. " + "El primer és a la sala petita del barri, amb entrades a deu euros i porta a les nou. ".repeat(11)

    private fun group(name: String, pal: GregalPalette) {
        val vm = vm()
        flow<Map<String, List<GregalViewModel.PetMsg>>>(vm, "_rotllanaChat").value = mapOf("colla" to listOf(
            GregalViewModel.PetMsg("user", "Què fem dissabte?"),
            GregalViewModel.PetMsg("pet", "Hi ha concerts.", speakerId = "xaloc", speakerName = "Xaloc"),
            GregalViewModel.PetMsg("pet", long, speakerId = "xaloc", speakerName = "Xaloc"),
            GregalViewModel.PetMsg("pet", "Jo miraria el temps abans.", speakerId = "gregal", speakerName = "Gregal"),
            GregalViewModel.PetMsg("pet", "Diu que farà sol.", speakerId = "xaloc", speakerName = "Xaloc"),
        ))
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    RotllanaConversation(vm, "ca", Rotllana("colla", "Colla", listOf("xaloc", "gregal")), listOf(xaloc, gregal), onBack = {})
                }
            }
        }
        frame(name)
        // Una cara petita al costat del nom a cada ratxa (xaloc, gregal, xaloc), no a cada missatge.
        val header = with(ui.density) { ChatHeaderInset.toPx() }
        fun below(w: Wind) = ui.onAllNodes(SemanticsMatcher.expectValue(WindFaceKey, Color(w.argb)), useUnmergedTree = true)
            .fetchSemanticsNodes().count { it.boundsInRoot.top >= header }
        assertEquals(3, ui.onAllNodesWithTag("wind-byline", useUnmergedTree = true).fetchSemanticsNodes().size)
        assertEquals(2, below(xaloc))
        assertEquals(1, below(gregal))
        // La mateixa regla per a una resposta curta i una de llarga: comencen al mateix lloc.
        val short = ui.onNodeWithText("Hi ha concerts.", useUnmergedTree = true).getBoundsInRoot()
        val longOne = ui.onNodeWithText(long.trim(), substring = true, useUnmergedTree = true).getBoundsInRoot()
        assertEquals(short.left.value, longOne.left.value, 0.5f)
    }

    @Test fun grupClar() = group("group-light", LightPal)
    @Test fun grupFosc() = group("group-dark", DarkPal)
}
