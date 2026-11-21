package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode
import java.io.File

/**
 * Les pantalles que tenien capçalera pròpia, ara amb `GregalTopBar` sobre el
 * fons comú, fosc i clar. Captures a `build/top-bars-look/`.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h640dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class TopBarsLookTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val app get() = ApplicationProvider.getApplicationContext<Application>()

    // Un servidor que no respon: res de dades reals a les captures.
    private fun vm(): GregalViewModel = GregalViewModel(app).also {
        it.attachPrefs(Prefs(app))
        it.setSimpleVents(false)
        it.setIdioma("ca")
        it.setConn(Conn("127.0.0.1", "9", "", local = true))
    }

    private fun capture(name: String, pal: GregalPalette, content: @Composable () -> Unit) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) { GregalBg { content() } }
            }
        }
        ui.mainClock.advanceTimeBy(800)
        ui.onNodeWithTag(GregalTopBarTitle).assertExists()
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        File("build/top-bars-look/$name.png").also { it.parentFile?.mkdirs() }.outputStream().use { bmp.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private fun note(name: String, pal: GregalPalette) {
        val vm = vm()
        (GregalViewModel::class.java.getDeclaredField("_openNote").apply { isAccessible = true }.get(vm)
            as MutableStateFlow<NoteDetail?>).value =
            NoteDetail(Note("n1", "Llista de la compra", body = "- llet\n- pa\n- tomàquets", tags = listOf("casa")), emptyList())
        capture(name, pal) { NotesScreen(vm) {} }
    }

    @Test fun websDark() = capture("webs-dark", DarkPal) { WebsScreen(vm()) {} }
    @Test fun websLight() = capture("webs-light", LightPal) { WebsScreen(vm()) {} }
    @Test fun notesDark() = capture("notes-dark", DarkPal) { NotesScreen(vm()) {} }
    @Test fun notesLight() = capture("notes-light", LightPal) { NotesScreen(vm()) {} }
    @Test fun noteDark() = note("note-dark", DarkPal)
    @Test fun noteLight() = note("note-light", LightPal)
    @Test fun savedDark() = capture("saved-dark", DarkPal) { DesatScreen(vm()) {} }
    @Test fun savedLight() = capture("saved-light", LightPal) { DesatScreen(vm()) {} }
    @Test fun automationsDark() = capture("automations-dark", DarkPal) { AutomatitzacionsScreen(vm()) {} }
    @Test fun automationsLight() = capture("automations-light", LightPal) { AutomatitzacionsScreen(vm()) {} }
    @Test fun retroDark() = capture("retro-dark", DarkPal) { RetroScreen(vm(), {}) {} }
    @Test fun retroLight() = capture("retro-light", LightPal) { RetroScreen(vm(), {}) {} }
    @Test fun windsDark() = capture("winds-dark", DarkPal) { CompanyScreen(vm()) {} }
    @Test fun windsLight() = capture("winds-light", LightPal) { CompanyScreen(vm()) {} }
}
