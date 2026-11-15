package xyz.solanes.gregal

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.LooperMode

/** Targetes riques del xat: mateix format i regles que md.js (md-cards.test.cjs). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h2400dp")
@LooperMode(LooperMode.Mode.PAUSED)
class RichCardsTest {
    @get:Rule val ui = createComposeRule()

    private val recipe = "Un plat ràpid.\n\n```gregal-recipe\n" +
        """{"title":"Cigrons amb espinacs","time":"20 min","level":"Fàcil","servings":2,""" +
        """"ingredients":[{"name":"Cigrons cuits","qty":"400 g"},{"name":"All i oli","qty":"Al gust"}],""" +
        """"steps":[{"text":"Daura l'all."},{"text":"Afegeix el tomàquet.","minutes":8}],"tip":"Pebre vermell."}""" + "\n```"

    @Test fun lesQuantitatsSEscalenComAlEscriptori() {
        assertEquals("600 g", scaleQty("400 g", 2, 3))
        assertEquals("3 l", scaleQty("1,5 l", 2, 4))
        assertEquals("2,25 l", scaleQty("1,5 l", 2, 3))
        assertEquals("1 ceba", scaleQty("½ ceba", 2, 4))
        assertEquals("Al gust", scaleQty("Al gust", 2, 6))
        assertEquals("aprox. 200 g", scaleQty("aprox. 400 g", 2, 1))
        assertEquals("1 c. cafè", scaleQty("1/2 c. cafè", 2, 4))
        // El JSON a la mateixa línia de l'obertura (els models ho fan).
        assertTrue(richBlocksLive("```gregal-recipe {\"title\":\"C\",\"steps\":[\"Bull\"]}\n```").single() is Rich.Card)
    }

    @Test fun receptaIPlaSonTargetesICheclistIGaleriaTambe() {
        val r = richBlocksLive(recipe)
        val card = (r.last() as Rich.Card).card as RichCard.Recipe
        assertEquals(2, card.spec.servings)
        assertEquals(8, card.spec.steps[1].minutes)
        val plan = richBlocksLive("```gregal-plan\n{\"title\":\"Girona\",\"image\":\"javascript:x\",\"items\":[{\"time\":\"08:05\",\"title\":\"Tren\"}],\"total\":{\"label\":\"Pressupost\",\"value\":\"42 €\"}}\n```")
        val p = ((plan.single() as Rich.Card).card as RichCard.Plan).spec
        assertEquals("", p.image)
        assertEquals("42 €", p.totalValue)
        val checks = richBlocksLive("Abans de marxar:\n- [ ] Passaport\n- [x] Bitllets\n\nI ja està.")
        assertEquals(2, ((checks[1] as Rich.Card).card as RichCard.Checks).items.size)
        assertTrue(checks.last() is Rich.Inline)
        val g = richBlocksLive("![a](https://e.com/a.jpg)\n![b](https://e.com/b.jpg)")
        assertEquals(2, ((g.single() as Rich.Card).card as RichCard.Gallery).images.size)
        assertNull(parseRecipeSpec("{\"title\":\"Res\"}"))
        assertTrue(richBlocksLive("```gregal-plan\nno és json\n```").single() is Rich.Code)
        assertTrue(richBlocksLive("Mira:\n```gregal-recipe\n{\"title\":\"Mig").none { it is Rich.Code })
    }

    @Test fun lesOpcionsPortenFotoINumeros() {
        val s = parseResultSpec("""{"title":"Tria","image":"https://e.com/a.jpg","numbered":true,"rows":[{"lead":"Model A","value":"699 €"}]}""")!!
        assertEquals("https://e.com/a.jpg", s.image)
        assertTrue(s.numbered)
    }

    @Test fun lesRacionsCanvienLesQuantitatsIElsIngredientsEsMarquen() {
        val card = (richBlocksLive(recipe).last() as Rich.Card).card
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    Column(Modifier.verticalScroll(rememberScrollState())) { RichCardBlock(card, DarkPal.sea, "ca") }
                }
            }
        }
        ui.onNodeWithText("400 g").assertExists()
        ui.onNodeWithContentDescription("Més racions").performClick()
        ui.onNodeWithText("600 g").assertExists()
        ui.onNodeWithText("3 racions", substring = true).assertExists()
        ui.onNodeWithText("Cigrons cuits").performClick()
        ui.onNodeWithText("8 min ⏱").assertExists()
    }

    private val plan = "```gregal-plan\n" +
        """{"title":"Girona, al teu ritme","items":[{"time":"08:05","title":"Tren","detail":"Avant"},{"time":"10:00","title":"Barri Vell"},""" +
        """{"time":"13:30","title":"Dinar"},{"time":"16:00","title":"Onyar"},{"time":"18:10","title":"Tornada"}],""" +
        """"actions":[{"label":"Una altra idea"},{"label":"Prepara el pla","prompt":"Prepara el pla amb reserves","primary":true}]}""" + "\n```"

    @Test fun elTrajecteIlesAccionsEsLlegeixenComAlEscriptori() {
        val p = ((richBlocksLive(plan).single() as Rich.Card).card as RichCard.Plan).spec
        assertEquals(listOf("Prepara el pla", "Una altra idea"), p.actions.map { it.label })
        assertEquals("Prepara el pla amb reserves", p.actions[0].prompt)
        assertEquals("Una altra idea", p.actions[1].prompt)
        val r = richBlocksLive("```gregal-route\n{\"from\":\"Barcelona\",\"to\":\"Girona\",\"depart\":\"08:05\",\"arrive\":\"08:45\",\"from_stop\":\"Sants\",\"mode\":\"Train\",\"price\":\"17,40 €\",\"alternatives\":[{\"time\":\"08:16\",\"service\":\"R11\",\"url\":\"javascript:x\"}]}\n```")
        val route = ((r.single() as Rich.Card).card as RichCard.Route).spec
        assertEquals("Sants", route.fromStop)
        assertEquals("Girona", route.toStop)
        assertEquals("train", route.mode)
        assertEquals("", route.alternatives.single().url)
        assertNull(parseRouteSpec("{\"from\":\"A\",\"depart\":\"08:00\"}"))
    }

    @Test fun elPlaEsCompacteIElsBotonsEnvienLaResposta() {
        val card = (richBlocksLive(plan).single() as Rich.Card).card
        val said = mutableListOf<String>()
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal, LocalCardSay provides { t: String -> said += t }) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    Column(Modifier.verticalScroll(rememberScrollState())) { RichCardBlock(card, DarkPal.sea, "ca") }
                }
            }
        }
        ui.onNodeWithText("Tornada").assertDoesNotExist()
        ui.onNodeWithText("Avant").assertDoesNotExist()
        ui.onNodeWithTag("plan-details").performClick()
        ui.onNodeWithText("Tornada").assertExists()
        ui.onNodeWithText("Avant").assertExists()
        ui.onNodeWithTag("card-cta").performClick()
        ui.onNodeWithTag("card-alt").performClick()
        assertEquals(listOf("Prepara el pla amb reserves", "Una altra idea"), said)
    }

    @Test fun senseXatQueEnviiNoHiHaBotons() {
        val card = (richBlocksLive(plan).single() as Rich.Card).card
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                MaterialTheme(colorScheme = DarkPal.scheme()) { Column { RichCardBlock(card, DarkPal.sea, "ca") } }
            }
        }
        ui.onNodeWithTag("card-cta").assertDoesNotExist()
    }
}
