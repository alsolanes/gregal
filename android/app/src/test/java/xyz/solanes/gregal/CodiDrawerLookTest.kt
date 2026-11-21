package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.unit.dp
import androidx.test.core.app.ApplicationProvider
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode
import java.io.File

/**
 * El calaix de Codi sense duplicats i Automatitzacions amb Activitat i Grafs a
 * la capçalera, fosc i clar. Captures a `build/codi-drawer-look/`.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h1000dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class CodiDrawerLookTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val app get() = ApplicationProvider.getApplicationContext<Application>()

    // Un servidor que no respon: res de dades reals a les captures.
    private fun vm(): GregalViewModel = GregalViewModel(app).also {
        it.attachPrefs(Prefs(app))
        it.setSimpleVents(false)
        it.setIdioma("ca")
        it.setConn(Conn("127.0.0.1", "9", "", local = true))
        it.go(Screen.Automatitzacions)
    }

    private fun capture(name: String, pal: GregalPalette, content: @androidx.compose.runtime.Composable () -> Unit) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) { content() }
            }
        }
        ui.mainClock.advanceTimeBy(800)
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        File("build/codi-drawer-look/$name.png").also { it.parentFile?.mkdirs() }.outputStream().use { bmp.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private fun drawer(name: String, pal: GregalPalette) {
        val vm = vm()
        capture(name, pal) {
            // El calaix fa 300 dp d'ample.
            Box(Modifier.fillMaxHeight().width(300.dp).background(pal.surface)) {
                GregalDrawerContent(vm, "ca", Screen.Automatitzacions, true, "", "gregal", "main", nav = {}, close = {})
            }
        }
        ui.onNodeWithText(tr("ca", "nav.automations")).assertExists()
    }

    private fun automations(name: String, pal: GregalPalette) {
        val vm = vm()
        capture(name, pal) { GregalBg { AutomatitzacionsScreen(vm) {} } }
        ui.onNodeWithContentDescription(tr("ca", "nav.activity")).assertExists()
        ui.onNodeWithContentDescription(tr("ca", "graphs.title")).assertExists()
    }

    @Test fun drawerDark() = drawer("drawer-dark", DarkPal)
    @Test fun drawerLight() = drawer("drawer-light", LightPal)
    @Test fun automationsDark() = automations("automations-dark", DarkPal)
    @Test fun automationsLight() = automations("automations-light", LightPal)
}
