package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
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
 * La portada de Codi (tasca 039): salutació, compositor i, a sota, les
 * converses de codi recents com a la portada de Vents. Captures a `build/code-start/`.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class CodeStartPageTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    // Migdia fix: a la matinada, «fa 3 hores» seria ahir i el test canviaria.
    private val now = java.time.ZonedDateTime.of(2026, 11, 21, 12, 0, 0, 0, java.time.ZoneId.systemDefault()).toOffsetDateTime()
    private val project = "/home/anna/botiga"
    // Com /api/sessions: la més nova primer, de tots els projectes.
    private val sessions = listOf(
        SessionInfo("w1", 3, "", "Logo de la web personal", now.minusMinutes(5).toString(), workspace = "/home/anna/web"),
        SessionInfo("s6", 12, "", "Login rebutja tokens caducats", now.minusMinutes(20).toString(), workspace = project),
        SessionInfo("s5", 8, "", "Paginació de la llista de comandes", now.minusHours(3).toString(), workspace = project),
        SessionInfo("s4", 30, "", "Migració de la base de dades", now.minusDays(1).toString(), workspace = project),
        SessionInfo("s3", 5, "", "Tests del parser de CSV", now.minusDays(3).toString(), workspace = project),
        SessionInfo("s2", 2, "", "README del projecte", now.minusDays(5).toString(), workspace = project),
        SessionInfo("s1", 4, "", "Primera prova", now.minusDays(9).toString(), workspace = project),
    )

    private fun show(pal: GregalPalette, list: List<SessionInfo> = sessions, onResume: (SessionInfo) -> Unit = {}) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    GregalBg {
                        CodeOpening("ca", "Bona tarda, Anna", list, onResume, Modifier.fillMaxSize(),
                            project = project, nowMs = now.toInstant().toEpochMilli()) { m ->
                            // Un compositor sintètic de l'alçada del de debò.
                            Surface(m.padding(horizontal = 12.dp).height(56.dp), shape = GregalShape.xl, color = Superficie) {
                                Text("Pregunta o demana", color = Faint, modifier = Modifier.padding(16.dp))
                            }
                        }
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
        File("build/code-start/$name.png").also { it.parentFile?.mkdirs() }.outputStream().use { bmp.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private fun top(text: String) = ui.onNodeWithText(text).fetchSemanticsNode().boundsInRoot.top

    @Test fun `sota el compositor hi ha les cinc converses mes noves, la mes nova primer`() {
        show(DarkPal)
        save("dark")
        ui.onNodeWithTag("start-recents").assertExists()
        assertTrue(top("Login rebutja tokens caducats") < top("Paginació de la llista de comandes"))
        assertTrue(top("Paginació de la llista de comandes") < top("Migració de la base de dades"))
        assertTrue(top("Tests del parser de CSV") < top("README del projecte"))
        ui.onNodeWithText("Primera prova").assertDoesNotExist()
        // Quan: «Avui 11:40», «Ahir».
        ui.onNodeWithText("Avui 11:40").assertExists()
        ui.onNodeWithText("Ahir").assertExists()
        assertTrue(top("Pregunta o demana") < top("Login rebutja tokens caducats"))
    }

    @Test fun `les converses d'un altre projecte no hi surten`() {
        show(DarkPal)
        ui.onNodeWithText("Logo de la web personal").assertDoesNotExist()
    }

    @Test fun `una conversa sense carpeta o un projecte desconegut hi compten`() {
        assertTrue(sameProject("/home/anna/botiga/", "/home/anna/botiga"))
        assertTrue(sameProject("", "/home/anna/botiga"))
        assertTrue(sameProject("/home/anna/web", ""))
        org.junit.Assert.assertFalse(sameProject("/home/anna/web", "/home/anna/botiga"))
        // A Windows les carpetes no distingeixen majúscules (com runs.WorkspaceKey al servidor).
        assertTrue(sameProject("C:\\Users\\anna\\Botiga", "c:\\users\\anna\\botiga\\"))
        org.junit.Assert.assertFalse(sameProject("/home/anna/Botiga", "/home/anna/botiga"))
    }

    @Config(qualifiers = "w780dp-h330dp-xhdpi")
    @Test fun `en horitzontal les recents es poden desplacar`() {
        show(DarkPal)
        save("dark-landscape")
        // No hi cap: la llista hi és sencera i la portada es desplaça.
        ui.onNodeWithText("README del projecte").assertExists()
        ui.onNode(hasScrollAction()).assertExists()
    }

    @Test fun `tocar una conversa recent la repren`() {
        var resumed: SessionInfo? = null
        show(LightPal, onResume = { resumed = it })
        save("light")
        ui.onNodeWithText("Migració de la base de dades").performClick()
        assertEquals("s4", resumed?.name)
    }

    @Test fun `sense converses la portada queda com sempre`() {
        show(DarkPal, list = emptyList())
        ui.onNodeWithTag("chat-opening").assertExists()
        ui.onNodeWithTag("start-recents").assertDoesNotExist()
    }

    // Un mòbil de 640 dp sense la barra d'estat ni la capçalera.
    @Config(qualifiers = "w360dp-h560dp-xhdpi")
    @Test fun `en un mobil petit hi cap tot`() {
        show(DarkPal)
        save("dark-small")
        ui.onNodeWithText("README del projecte").assertIsDisplayed()
    }
}
