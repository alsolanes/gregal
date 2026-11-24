package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode
import java.io.File

/**
 * La portada d'inici de Vents (tasca 027): en lloc del fil etern amb Gregal,
 * salutació, compositor, les cares dels vents i les converses recents.
 * Captures a `build/vents-start/`.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class VentsStartPageTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val gregal = Wind("gregal", "Gregal", "#8193FF", "Assistent")
    private val xaloc = Wind("xaloc", "Xaloc", "#EFA68F", "Música i concerts")
    private val now = java.time.OffsetDateTime.now()
    private val threads = listOf(
        WindThread("xaloc", "company-xaloc-s2", "Concerts de dissabte a Girona", now.truncatedTo(java.time.temporal.ChronoUnit.DAYS).toString(), 6),
        WindThread("gregal", "company-gregal", "Prepara'm el viatge de divendres", now.minusDays(1).toString(), 40),
    )

    private fun show(pal: GregalPalette, onWind: (Wind) -> Unit = {}, onThread: (WindThread) -> Unit = {}) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    GregalBg {
                        VentsStartPage("ca", "Anna", listOf(gregal, xaloc), threads, "", {}, busy = false,
                            onSend = {}, onWind = onWind, onThread = onThread)
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
        File("build/vents-start/$name.png").also { it.parentFile?.mkdirs() }.outputStream().use { bmp.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    @Test fun `la portada ensenya els vents i les converses recents, no un fil`() {
        show(DarkPal)
        ui.onNodeWithTag("start-wind-gregal").assertExists()
        ui.onNodeWithTag("start-wind-xaloc").assertExists()
        ui.onNodeWithText("Concerts de dissabte a Girona").assertExists()
        ui.onNodeWithText("Prepara'm el viatge de divendres").assertExists()
        // De qui és i quan: «Xaloc · Avui 14:32», «Gregal · Ahir».
        ui.onNode(hasText("Xaloc · Avui", substring = true)).assertExists()
        ui.onNodeWithText("Gregal · Ahir").assertExists()
        save("dark")
    }

    @Test fun `tocar una cara obre aquell vent i tocar una conversa la repren`() {
        var wind: Wind? = null
        var thread: WindThread? = null
        show(LightPal, onWind = { wind = it }, onThread = { thread = it })
        save("light")
        ui.onNodeWithTag("start-wind-xaloc").performClick()
        assertEquals("xaloc", wind?.id)
        ui.onNodeWithText("Prepara'm el viatge de divendres").performClick()
        assertEquals("company-gregal", thread?.name)
    }

    @Test fun `la marca de temps diu avui amb hora, ahir o el dia`() {
        val now = java.time.OffsetDateTime.of(2026, 11, 21, 18, 0, 0, 0, java.time.ZoneOffset.UTC)
        val ms = now.toInstant().toEpochMilli()
        org.junit.Assert.assertTrue(threadWhen(now.minusHours(2).toString(), "ca", ms).startsWith("Avui "))
        assertEquals("Ahir", threadWhen(now.minusDays(1).toString(), "ca", ms))
        assertEquals("15-11", threadWhen(now.minusDays(6).toString(), "ca", ms))
        assertEquals("", threadWhen("no és una data", "ca", ms))
    }
}
