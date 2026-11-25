package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.unit.dp
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

/**
 * El xat d'un vent com a Claude o ChatGPT: la resposta és text corrent amb el
 * nom del vent al principi de la ratxa, la teva és una bombolla discreta, i el
 * color del vent només és a la cara, al nom i als accents. Fotogrames clar i
 * fosc a output/ui-review/145/.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h1400dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindChatLookTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()
    private val wind = Wind("mestral", "Mestral", "#A8CDE2", "recerca")
    private val garbi = Wind("garbi", "Garbí", "#EFA68F", "cuina")

    private val reply = """
## Tres excursions per dissabte

La millor tria depèn de quant vulguis caminar. Totes surten de Barcelona en menys d'una hora i mitja.

- **Montserrat**: cremallera des de Monistrol i la ruta a Sant Jeroni.
- **Montseny**: fagedes i Santa Fe; [horaris del bus](https://example.org).

Per mirar l'hora del tren:

```
rodalies --from Barcelona --to Monistrol
```

Si plou, deixa-ho per diumenge.
""".trimIndent()

    private fun <T> flow(vm: GregalViewModel, name: String) = GregalViewModel::class.java.getDeclaredField(name)
        .apply { isAccessible = true }.get(vm) as MutableStateFlow<T>

    private fun save(name: String) {
        ui.mainClock.advanceTimeBy(600)
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        File("../../output/ui-review/145/$name.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
            bmp.compress(Bitmap.CompressFormat.PNG, 100, it)
        }
    }

    private fun direct(name: String, pal: GregalPalette) {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>()).apply { setConn(Conn("127.0.0.1", "9", "t")) }
        flow<Map<String, List<GregalViewModel.PetMsg>>>(vm, "_windChat").value = mapOf(wind.id to listOf(
            GregalViewModel.PetMsg("user", "Què puc fer dissabte a prop de Barcelona?", ts = 1_764_000_000L),
            GregalViewModel.PetMsg("pet", reply, wind.id, wind.name, ts = 1_764_000_060L),
            GregalViewModel.PetMsg("user", "I si vull anar en tren?", ts = 1_764_000_120L),
            GregalViewModel.PetMsg("pet", "Montserrat, sense dubte: el cremallera surt de la mateixa estació.", wind.id, wind.name, streaming = true),
        ))
        flow<Set<String>>(vm, "_windBusy").value = setOf(wind.id)
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) { WindConversation(vm, "ca", wind, onBack = {}) }
            }
        }
        save(name)
        // El nom al principi de cada ratxa de respostes, no a cada missatge.
        assertEquals(2, ui.onAllNodesWithTag("wind-byline", useUnmergedTree = true).fetchSemanticsNodes().size)
    }

    private fun group(name: String, pal: GregalPalette) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    Column(Modifier.fillMaxSize().background(pal.surface).padding(12.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                        WindBubble(GregalViewModel.PetMsg("user", "Què sopem avui?", ts = 1_764_000_000L), Color(wind.argb))
                        WindBubble(GregalViewModel.PetMsg("pet", "Una truita de patates amb amanida: en mitja hora és a taula.", garbi.id, garbi.name),
                            Color(garbi.argb), speaker = garbi, speakerName = garbi.name)
                        WindBubble(GregalViewModel.PetMsg("pet", "I si en sobra, demà fa un bon entrepà.", garbi.id, garbi.name),
                            Color(garbi.argb))
                        WindBubble(GregalViewModel.PetMsg("pet", "Diu que plourà: millor sopar a casa.", wind.id, wind.name),
                            Color(wind.argb), speaker = wind, speakerName = wind.name)
                    }
                }
            }
        }
        save(name)
        assertEquals(2, ui.onAllNodesWithTag("wind-byline", useUnmergedTree = true).fetchSemanticsNodes().size)
    }

    @Test fun ventClar() = direct("wind-light", LightPal)
    @Test fun ventFosc() = direct("wind-dark", DarkPal)
    @Test fun grupClar() = group("group-light", LightPal)
    @Test fun grupFosc() = group("group-dark", DarkPal)

    @Test fun `el nom del vent es llegeix sobre el fons del seu xat en tots els temes`() {
        val colors = listOf(0xFFF4B6C2, 0xFFEACF87, 0xFF9ED3C5, 0xFFA8CDE2, 0xFFC1B4DE, 0xFFEFA68F, 0xFF3D7BFF).map { Color(it) }
        for (t in GregalTemes) for ((mode, p) in listOf("clar" to t.clar.solid(), "fosc" to t.fosc.solid())) for (c in colors) {
            val ratio = contrastWith(windNameColor(c, p.ink), windWash(c, p.surface))
            assertTrue("${t.id}/$mode $c: contrast %.2f < 4.5".format(ratio), ratio >= 4.5)
        }
    }
}
