package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.test.core.app.ApplicationProvider
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/**
 * Els vents treballen sempre en Actua (tasca 052): els xats d'un vent i d'una
 * rotllana no ensenyen el mode de Codi ni el deixen canviar, ni amb Vents net
 * ni sense. El «+» hi continua, amb la resta de coses.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class VentsNoModeTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val app get() = ApplicationProvider.getApplicationContext<Application>()
    private val xaloc = Wind("xaloc", "Xaloc", "#EFA68F", "Música i concerts")
    private val ponent = Wind("ponent", "Ponent", "#29ADFF", "Webs")

    private fun vm(simple: Boolean) = GregalViewModel(app).also {
        it.attachPrefs(Prefs(app))
        it.setIdioma("ca")
        // Un servidor que no respon: el mode és el de per defecte, que es veia com «Actua».
        it.setConn(Conn("127.0.0.1", "9", "", local = true))
        it.setSimpleVents(simple)
    }

    private fun show(content: @Composable () -> Unit) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { GregalBg { content() } }
            }
        }
        ui.mainClock.advanceTimeBy(800)
    }

    private fun assertNoMode() {
        // Exacte: «Permís d'Actua» (el cadenat) hi continua.
        for (key in listOf("app.mode.act", "app.mode.plan", "app.mode.ask")) {
            val label = tr("ca", key)
            ui.onAllNodes(hasText(label) or hasText("✓ $label")).assertCountEquals(0)
        }
    }

    private fun assertNoModeInChatOrPlus() {
        assertNoMode()
        ui.onNode(hasContentDescription(tr("ca", "app.mode.more")) and !hasTestTag("chat-more")).performClick()
        ui.mainClock.autoAdvance = true
        ui.onNodeWithText(tr("ca", "app.composer.attachDoc")).assertIsDisplayed()
        assertNoMode()
    }

    private fun windChat(simple: Boolean) {
        val vm = vm(simple)
        show { WindConversation(vm, "ca", xaloc, onBack = {}) }
        assertNoModeInChatOrPlus()
    }

    private fun rotllanaChat(simple: Boolean) {
        val vm = vm(simple)
        show { RotllanaConversation(vm, "ca", Rotllana("r1", "Colla", listOf("xaloc", "ponent")), listOf(xaloc, ponent), onBack = {}) }
        assertNoModeInChatOrPlus()
    }

    @Test fun `el xat d'un vent no té mode`() = windChat(simple = false)
    @Test fun `el xat d'un vent amb Vents net no té mode`() = windChat(simple = true)
    @Test fun `el xat d'una rotllana no té mode`() = rotllanaChat(simple = false)
    @Test fun `el xat d'una rotllana amb Vents net no té mode`() = rotllanaChat(simple = true)
}
