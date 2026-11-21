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

    private val now = java.time.OffsetDateTime.now()
    // Com /api/sessions: la més nova primer.
    private val sessions = listOf(
        SessionInfo("s6", 12, "", "Login rebutja tokens caducats", now.minusMinutes(20).toString()),
        SessionInfo("s5", 8, "", "Paginació de la llista de comandes", now.minusHours(3).toString()),
        SessionInfo("s4", 30, "", "Migració de la base de dades", now.minusDays(1).toString()),
        SessionInfo("s3", 5, "", "Tests del parser de CSV", now.minusDays(3).toString()),
        SessionInfo("s2", 2, "", "README del projecte", now.minusDays(5).toString()),
        SessionInfo("s1", 4, "", "Primera prova", now.minusDays(9).toString()),
    )

    private fun show(pal: GregalPalette, list: List<SessionInfo> = sessions, onResume: (SessionInfo) -> Unit = {}) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    GregalBg {
                        CodeOpening("ca", "Bona tarda, Anna", list, onResume, Modifier.fillMaxSize()) { m ->
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
        // Quan: «Avui 14:32», «Ahir».
        ui.onNode(hasText("Login rebutja tokens caducats") and hasText("Avui", substring = true)).assertExists()
        ui.onNodeWithText("Ahir").assertExists()
        assertTrue(top("Pregunta o demana") < top("Login rebutja tokens caducats"))
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
