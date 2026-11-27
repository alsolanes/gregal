package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.foundation.layout.Column
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.text.TextLayoutResult
import androidx.compose.ui.unit.Density
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
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
 * La pastilla de l'ofici (tasca 044): el `role` de cada vent, amb el seu color
 * i la icona del kit, al costat del nom a la portada, a la capçalera del xat i
 * al directori. Captures a `build/role-pill/`.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindRolePillTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val app get() = ApplicationProvider.getApplicationContext<Application>()
    private val xaloc = Wind("xaloc", "Xaloc", "#EFA68F", "Música i concerts")

    private fun show(pal: GregalPalette = LightPal, fontScale: Float = 1f, content: @Composable () -> Unit) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            val density = LocalDensity.current
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false,
                LocalDensity provides Density(density.density, fontScale)) {
                MaterialTheme(colorScheme = pal.scheme()) { GregalBg { content() } }
            }
        }
        ui.mainClock.advanceTimeBy(800)
    }

    private fun save(name: String) {
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        File("build/role-pill/$name.png").also { it.parentFile?.mkdirs() }.outputStream().use { bmp.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private fun conversation(pal: GregalPalette, wind: Wind, fontScale: Float = 1f) {
        val vm = GregalViewModel(app).also {
            it.attachPrefs(Prefs(app))
            it.setIdioma("ca")
            // Un servidor que no respon: res de dades reals a les captures.
            it.setConn(Conn("127.0.0.1", "9", "", local = true))
        }
        show(pal, fontScale) { WindConversation(vm, "ca", wind, onBack = {}) }
    }

    private fun pill(place: String) = ui.onNode(hasTestTag("role-pill") and hasAnyAncestor(hasTestTag(place)), useUnmergedTree = true)
    private fun pillText(place: String, role: String) =
        ui.onNode(hasText(role) and hasAnyAncestor(hasTestTag("role-pill") and hasAnyAncestor(hasTestTag(place))), useUnmergedTree = true)
    private fun pillIcon(place: String) = ui.onNode(hasTestTag("role-pill-icon") and hasAnyAncestor(hasTestTag(place)), useUnmergedTree = true)
    private fun name(place: String, name: String) =
        ui.onNode(hasText(name) and hasAnyAncestor(hasTestTag(place)) and !hasAnyAncestor(hasTestTag("role-pill")), useUnmergedTree = true)

    // hasVisualOverflow no serveix: amb el·lipsi el paràgraf ocupa tot l'ample i el marca sempre.
    private fun SemanticsNodeInteraction.ellipsized(): Boolean {
        val layout = mutableListOf<TextLayoutResult>()
        fetchSemanticsNode().config[SemanticsActions.GetTextLayoutResult].action?.invoke(layout)
        return layout.first().isLineEllipsized(0)
    }

    @Test fun `la portada posa l'ofici sota el nom de cada cara`() {
        val ponent = Wind("ponent", "Ponent", "#EACF87", "Llibres", kit = "books")
        show(DarkPal) {
            VentsStartPage("ca", "Anna", listOf(xaloc, ponent), emptyList(), "", {}, busy = false, onSend = {}, onWind = { _, _ -> }, onThread = {})
        }
        save("start-dark")
        name("start-wind-xaloc", "Xaloc").assertExists()
        pillText("start-wind-xaloc", "Música i concerts").assertExists()
        // Un ofici d'una paraula s'hi llegeix sencer, icona inclosa: no «Ll…».
        pillIcon("start-wind-ponent").assertExists()
        assertFalse(pillText("start-wind-ponent", "Llibres").ellipsized())
    }

    @Test fun `la capçalera del xat d'un vent porta l'ofici al costat del nom`() {
        conversation(LightPal, xaloc)
        save("header-light")
        name("chat-header-pill", "Xaloc").assertIsDisplayed()
        pillText("chat-header-pill", "Música i concerts").assertExists()
    }

    @Test fun `el directori porta l'ofici al costat del nom`() {
        show { DirectoryRow("ca", xaloc, onProfile = {}, onTalk = {}) }
        name("dir-xaloc", "Xaloc").assertIsDisplayed()
        pillText("dir-xaloc", "Música i concerts").assertExists()
    }

    @Test fun `amb kit la pastilla duu la icona, sense kit no, i sense rol no hi ha pastilla`() {
        show {
            Column {
                DirectoryRow("ca", Wind("kit", "Xaloc", "#EFA68F", "Concerts", kit = "music"), onProfile = {}, onTalk = {})
                DirectoryRow("ca", Wind("bare", "Marinada", "#EFC69A", "calma"), onProfile = {}, onTalk = {})
                DirectoryRow("ca", Wind("none", "Ponent", "#EACF87", ""), onProfile = {}, onTalk = {})
            }
        }
        save("kits-light")
        pillIcon("dir-kit").assertExists()
        pill("dir-bare").assertExists()
        pillIcon("dir-bare").assertDoesNotExist()
        pill("dir-none").assertDoesNotExist()
        name("dir-none", "Ponent").assertIsDisplayed()
    }

    @Test fun `cada kit de les mascotes té una icona per a la pastilla`() {
        assertEquals(emptyList<String>(), windKits.keys.filter { windKitIcon(it) == null })
        assertEquals(null, windKitIcon(""))
    }

    private val long = xaloc.copy(role = "Música, concerts, festivals i entrades a bon preu a tot Catalunya")

    @Test fun `amb lletra gran la pastilla de la capçalera s'escurça i el nom es continua veient`() {
        conversation(DarkPal, long, fontScale = 1.5f)
        save("header-dark-large")
        name("chat-header-pill", "Xaloc").assertIsDisplayed()
        assertFalse(name("chat-header-pill", "Xaloc").ellipsized())
        assertTrue(pillText("chat-header-pill", long.role).ellipsized())
    }

    @Test fun `amb lletra gran la pastilla del directori s'escurça i el nom es continua veient`() {
        show(LightPal, fontScale = 1.5f) { DirectoryRow("ca", long, onProfile = {}, onTalk = {}) }
        save("dir-light-large")
        name("dir-xaloc", "Xaloc").assertIsDisplayed()
        assertFalse(name("dir-xaloc", "Xaloc").ellipsized())
        assertTrue(pillText("dir-xaloc", long.role).ellipsized())
    }
}
