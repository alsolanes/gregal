package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.unit.dp
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode
import java.io.File

/** Captura (build/md-look/) d'una resposta típica d'un vent: títol, foto al mig, llistes i passos. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w400dp-h2400dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class MdLookTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val reply = """
## Tres excursions per dissabte

La millor tria depèn de quant vulguis caminar. Totes surten de Barcelona en menys d'una hora i mitja.

- **Montserrat**: cremallera des de Monistrol, vistes de tot el Bages i el monestir; la ruta a Sant Jeroni són 2 h.
- **Cadaqués**: el poble blanc, cales i el Cap de Creus; millor de bon matí, que a la tarda s'omple.
  - Aparca a l'entrada: el centre és de vianants.
  - Si fa tramuntana, deixa-ho per un altre dia.
- **Montseny**: fagedes i Santa Fe; [horaris del bus](https://www.sagales.com).

Com ho faria jo:

1. Surt a les 8 per evitar cues a la C-55.
2. Esmorza a Monistrol i puja amb el primer cremallera, que va més buit i deixa temps per a la ruta llarga.
3. Baixa a dinar a Collbató.
""".trimIndent()

    private fun capture(name: String, pal: GregalPalette) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    Column(Modifier.width(380.dp).background(pal.surface).padding(12.dp)) {
                        WindBubble(GregalViewModel.PetMsg("pet", reply, ts = System.currentTimeMillis() / 1000, id = "m-1"),
                            Color(0xFF8193FF), tail = true, showMeta = true)
                    }
                }
            }
        }
        ui.mainClock.advanceTimeBy(600)
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        val out = File("build/md-look/$name.png")
        out.parentFile?.mkdirs()
        val h = (bmp.height * 0.55f).toInt()
        out.outputStream().use { Bitmap.createBitmap(bmp, 0, 0, bmp.width, h).compress(Bitmap.CompressFormat.PNG, 100, it) }
        assertTrue(ui.onAllNodesWithTag("md-list", useUnmergedTree = true).fetchSemanticsNodes().size == 2)
    }

    @Test fun fosc() = capture("dark", DarkPal)
    @Test fun clar() = capture("light", LightPal)
}
