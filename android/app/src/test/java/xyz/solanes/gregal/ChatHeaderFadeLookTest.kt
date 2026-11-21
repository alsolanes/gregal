package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.MutableStateFlow
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
 * El fil d'un vent s'esvaeix sota la capçalera flotant (tornar, pastilla,
 * «…») en lloc de llegir-se entre els botons, amb tema pla i amb Vidre.
 * Captures a `build/chat-fade-look/`.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h640dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class ChatHeaderFadeLookTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val app get() = ApplicationProvider.getApplicationContext<Application>()

    private fun capture(name: String?, pal: GregalPalette, content: @Composable () -> Unit): Bitmap {
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
        if (name != null) File("build/chat-fade-look/$name.png").also { it.parentFile?.mkdirs() }.outputStream().use { bmp.compress(Bitmap.CompressFormat.PNG, 100, it) }
        return bmp
    }

    // Sota la franja de la capçalera hi ha el fons, no el fil; més avall, el fil sencer.
    @Test fun theThreadFadesOutUnderTheHeaderBand() {
        val bg = Color(0xFF102030)
        val thread = Color(0xFFFF0000)
        val bmp = capture(null, DarkPal) {
            Box(Modifier.fillMaxSize().background(bg)) {
                Box(Modifier.fillMaxSize().fadeUnderChatHeader().background(thread))
            }
        }
        val px = ui.activity.resources.displayMetrics.density
        val x = bmp.width / 2
        assertEquals(bg.toArgb(), bmp.getPixel(x, (20 * px).toInt()))
        assertEquals(thread.toArgb(), bmp.getPixel(x, ((ChatHeaderInset.value + 4) * px).toInt()))
    }

    private fun conversation(name: String, pal: GregalPalette) {
        val vm = GregalViewModel(app).also {
            it.attachPrefs(Prefs(app))
            it.setIdioma("ca")
            // Un servidor que no respon: res de dades reals a les captures.
            it.setConn(Conn("127.0.0.1", "9", "", local = true))
        }
        val wind = Wind("xaloc", "Xaloc", "#EFA68F", "Música i concerts")
        val msgs = (0 until 8).map { i ->
            GregalViewModel.PetMsg(if (i % 2 == 0) "user" else "pet",
                "Missatge $i. Fonts: el fil puja per sota de la capçalera i s'hi ha d'esvair, no llegir-se entre els botons.",
                ts = 1_000L + i, id = "m$i")
        }
        (GregalViewModel::class.java.getDeclaredField("_windChat").apply { isAccessible = true }.get(vm)
            as MutableStateFlow<Map<String, List<GregalViewModel.PetMsg>>>).value = mapOf(wind.id to msgs)
        capture(name, pal) { GregalBg { WindConversation(vm, "ca", wind, onBack = {}) } }
        ui.onNodeWithText("Missatge 7", substring = true).assertExists()
    }

    @Test fun conversationDark() = conversation("conversation-dark", DarkPal)
    @Test fun conversationLight() = conversation("conversation-light", LightPal)
    @Test fun conversationGlassDark() = conversation("conversation-glass-dark", DarkPal.glassy())
    @Test fun conversationGlassLight() = conversation("conversation-glass-light", LightPal.glassy())
}
