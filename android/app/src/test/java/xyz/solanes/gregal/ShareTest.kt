package xyz.solanes.gregal

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.LooperMode

/** Compartir: el text per a WhatsApp/Telegram és el mateix que a share-msg.test.cjs. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h2400dp")
@LooperMode(LooperMode.Mode.PAUSED)
class ShareTest {
    @get:Rule val ui = createComposeRule()

    @Test fun lesTargetesPassenATextLlegible() {
        val plan = "Aquí ho tens:\n\n```gregal-plan\n{\"title\":\"Dissabte a Girona\",\"items\":[{\"time\":\"10:00\",\"title\":\"Tren\",\"detail\":\"Rodalies R11\"},{\"title\":\"Dinar\"}],\"total\":\"45 €\"}\n```\n"
        assertEquals("Aquí ho tens:\n\n*Dissabte a Girona*\n10:00 · Tren — Rodalies R11\n• Dinar\nTotal: 45 €", ShareText.plain(plan, "ca"))
        val res = "```gregal-result\n{\"title\":\"Mòbils\",\"rows\":[{\"lead\":\"Pixel 9a\",\"text\":\"càmera\",\"value\":\"399 €\",\"pick\":true,\"url\":\"https://x.cat/p\"}],\"sources\":[{\"url\":\"https://x.cat\"}]}\n```"
        assertEquals("*Mòbils*\n• Pixel 9a — càmera: 399 € ✓\n  https://x.cat/p\n\nhttps://x.cat", ShareText.plain(res, "ca"))
        val rec = "```gregal-recipe\n{\"title\":\"Crema\",\"servings\":2,\"ingredients\":[{\"name\":\"carbassa\",\"qty\":\"500 g\"}],\"steps\":[{\"text\":\"Bull\",\"minutes\":20}]}\n```"
        assertEquals("*Crema*\n2 servings\n\nIngredients:\n• 500 g carbassa\n\nSteps:\n1. Bull (20 min)", ShareText.plain(rec, "en"))
        val route = "```gregal-route\n{\"from\":\"Vic\",\"to\":\"Barcelona\",\"depart\":\"8:05\",\"arrive\":\"9:20\",\"service\":\"R3\",\"price\":\"4,6 €\",\"alternatives\":[{\"time\":\"8:35\",\"service\":\"R3\"}]}\n```"
        assertEquals("*Vic → Barcelona*\n8:05 → 9:20 · R3 · 4,6 €\n  · 8:35 · R3", ShareText.plain(route, "ca"))
        val chart = "```chart\n{\"title\":\"Vendes\",\"type\":\"bar\",\"labels\":[\"gen\",\"feb\"],\"series\":[{\"label\":\"2026\",\"values\":[3,5]}]}\n```"
        assertEquals("*Vendes*\n• gen: 3\n• feb: 5", ShareText.plain(chart, "ca"))
    }

    @Test fun desatLlegeixElQueTHanEnviat() {
        val got = WindActivityJson.saved("""{"items":[{"kind":"received","title":"Dissabte a Girona","id":"abc","text":"**hola**","from":"carla","wind_name":"Gregal","ts":1794600000,"unread":true}]}""")
        val r = got.single()
        assertEquals("received", r.kind)
        assertEquals("carla", r.from)
        assertEquals("**hola**", r.text)
        assertTrue(r.unread)
        assertEquals("De carla", ShareText.word("ca", "from").format(r.from))
    }

    @Test fun markdownNet() {
        val md = "## Títol\n\n**Molt** bé, mira [la web](https://a.cat) i [el doc](/api/winds/doc?name=a.xlsx).\n\n![foto](https://img.cat/a.jpg)\n![feta](/api/winds/image?id=0123456789abcdef01234567.jpg)\n- un\n- dos"
        assertEquals("*Títol*\n\n*Molt* bé, mira la web (https://a.cat) i el doc.\n\nhttps://img.cat/a.jpg\n\n• un\n• dos", ShareText.plain(md, "ca"))
        val broken = "```gregal-plan\n{trencat\n```"
        assertEquals(broken, ShareText.plain(broken, "ca"))
    }

    @Test fun elFullOfereixTextImatgePersonaIEnllac() {
        val vm = GregalViewModel(androidx.test.core.app.ApplicationProvider.getApplicationContext<android.app.Application>())
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    Box(Modifier.fillMaxSize()) {
                        ShareSheetBody(vm, "ca", ShareTarget("**Hola**", "Xaloc", capture = { null }), onDismiss = {})
                    }
                }
            }
        }
        ui.onNodeWithText("Comparteix com a text").assertExists()
        ui.onNodeWithText("Comparteix com a imatge").assertExists()
        ui.onNodeWithText("Envia a algú de Gregal").assertExists()
        ui.onNodeWithText("Crea un enllaç públic").assertExists()
        ui.mainClock.advanceTimeBy(2_000)
        ui.onNodeWithTag("share-copy").performClick()
        ui.mainClock.advanceTimeBy(500)
        ui.waitForIdle()
        assertTrue(ui.onAllNodesWithText("Copiat", useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty())
    }
}
