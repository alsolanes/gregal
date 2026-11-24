package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** Les comparacions com a fitxes horitzontals (CompareCards.kt): fila amb imant o pàgines per lliscar. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h2000dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class CompareCardsUiTest {
    @get:Rule val ui = createComposeRule()
    private val fence = "```"

    private val two = """{"title":"Node o Deno","items":[
        {"name":"Node.js","facts":{"Preu":"Gratuït, de codi obert i amb una comunitat enorme que el manté des de fa anys","Tipus":"Runtime"}},
        {"name":"Deno","facts":{"Preu":"Gratuït","Tipus":"Runtime"},"pick":true}],"verdict":"Deno per començar de zero."}"""

    private val six = """{"title":"Llums","items":[${listOf("Aplic", "Bombeta", "Candela", "Disc", "Espelma", "Fanal")
        .joinToString(",") { """{"name":"$it","facts":{"Llum":"$it lm"}}""" }}],"verdict":"Totes fan llum."}"""

    private val parties = """{"title":"Programes electorals","items":[
        {"name":"Partit Blau","images":["https://e.invalid/blau-logo.png","https://e.invalid/blau-cap.jpg"],"url":"https://blau.example/programa","points":["Habitatge públic","Menys impostos a la feina","Més trens de rodalies"]},
        {"name":"Partit Verd","images":["https://e.invalid/verd.png"],"points":["Energia solar als terrats","Transport gratuït","Escola de 0 a 3 anys"]},
        {"name":"Partit Groc","points":["Sanitat sense llistes d'espera","Policia de proximitat","Turisme amb límits"]}],
        "verdict":"Resum de cada programa a partir de les seves fonts.","sources":[{"name":"Blau","url":"https://blau.example/programa"}]}"""

    private fun block(json: String) = "Aquí ho tens:\n\n${fence}gregal-compare\n$json\n$fence\n"

    private fun show(motion: Boolean = false, pal: GregalPalette = DarkPal, content: @Composable () -> Unit) {
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides motion, LocalPal provides pal) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    Box(Modifier.fillMaxWidth().background(pal.surface).testTag("shot")) { content() }
                }
            }
        }
    }

    private fun card(json: String) = (richBlocks(block(json)).filterIsInstance<Rich.Card>().single().card as RichCard.Compare)

    @Test fun twoItemsSitInARowWithTheirInitialPickAndVerdict() {
        show { RichCardBlock(card(two), Sea, "ca") }
        ui.onNodeWithTag("compare-row").assertExists()
        assertEquals(2, ui.onAllNodesWithTag("compare-item").fetchSemanticsNodes().size)
        ui.onNodeWithText("Deno per començar de zero.").assertIsDisplayed()
        ui.onNodeWithText("La tria").assertIsDisplayed()
        // Sense foto ni logo, la inicial.
        ui.onAllNodesWithText("N").assertCountEquals(1)
        ui.onAllNodesWithText("D").assertCountEquals(1)
    }

    @Test fun factsLineUpAcrossCards() {
        show { RichCardBlock(card(two), Sea, "ca") }
        // El preu de Node és molt més llarg que el de Deno, però «Tipus» comença a la mateixa altura.
        val tops = ui.onAllNodesWithText("Tipus").fetchSemanticsNodes().map { it.boundsInRoot.top }
        assertEquals(2, tops.size)
        assertEquals(tops[0], tops[1], 0.5f)
    }

    @Test fun sixItemsScrollWithAPeekOfTheNext() {
        show { RichCardBlock(card(six), Sea, "ca") }
        val row = ui.onNodeWithTag("compare-row").fetchSemanticsNode().boundsInRoot
        val first = ui.onAllNodesWithTag("compare-item")[0].fetchSemanticsNode().boundsInRoot
        assertTrue("the next card peeks in", first.width < row.width * 0.9f)
        ui.onNodeWithText("Bombeta").assertIsDisplayed()
        ui.onNodeWithText("Fanal").assertDoesNotExist()
        ui.onNodeWithTag("compare-row").performScrollToIndex(5)
        ui.onNodeWithText("Fanal").assertIsDisplayed()
        ui.onNodeWithText("Totes fan llum.").assertIsDisplayed()
    }

    @Test fun partiesSwipeOneCardPerPageWithDots() {
        show { RichCardBlock(card(parties), Sea, "ca") }
        ui.onNodeWithTag("compare-pager").assertExists()
        ui.onNodeWithTag("compare-row").assertDoesNotExist()
        val dots = ui.onAllNodesWithTag("compare-dot")
        dots.assertCountEquals(3)
        dots[0].assertIsSelected()
        ui.onNodeWithText("Habitatge públic").assertIsDisplayed()
        ui.onNodeWithTag("compare-pager").performTouchInput { swipeLeft() }
        ui.waitForIdle()
        dots[1].assertIsSelected()
        ui.onNodeWithText("Energia solar als terrats").assertIsDisplayed()
        ui.onNodeWithTag("compare-pager").performTouchInput { swipeLeft() }
        ui.waitForIdle()
        dots[2].assertIsSelected()
        ui.onNodeWithText("Turisme amb límits").assertIsDisplayed()
        ui.onNodeWithTag("compare-pager").performTouchInput { swipeRight() }
        ui.waitForIdle()
        dots[1].assertIsSelected()
        // Un toc a un punt hi porta.
        dots[0].performClick()
        ui.waitForIdle()
        dots[0].assertIsSelected()
        ui.onNodeWithText("Resum de cada programa a partir de les seves fonts.").assertIsDisplayed()
        ui.onNodeWithTag("compare-pick").assertDoesNotExist()
    }

    @Test fun aCodeChatAnswerShowsTheRow() {
        show { AssistantResponse(block(two), "ca") }
        ui.onNodeWithTag("compare-row").assertExists()
        ui.onNodeWithText("Deno").assertIsDisplayed()
    }

    @Test fun aWindChatAnswerShowsTheRow() {
        show { WindBubble(GregalViewModel.PetMsg("pet", block(two)), Sea) }
        // La bombolla del vent fusiona la semàntica dels fills.
        ui.onNodeWithTag("compare-row", useUnmergedTree = true).assertExists()
        ui.onNodeWithText("Deno", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test fun withoutMotionTheCardsStandStill() {
        ui.mainClock.autoAdvance = false
        show(motion = false) { RichCardBlock(card(six), Sea, "ca") }
        val start = capture()
        ui.mainClock.advanceTimeBy(1500)
        assertTrue("nothing moves", start.sameAs(capture()))
    }

    @Test fun withMotionTheCardsSpringIn() {
        ui.mainClock.autoAdvance = false
        show(motion = true) { RichCardBlock(card(six), Sea, "ca") }
        ui.mainClock.advanceTimeBy(16)
        val start = capture()
        ui.mainClock.advanceTimeBy(1500)
        assertFalse("the cards come in", start.sameAs(capture()))
    }

    /** Uns quants fotogrames a output/ui-review/116/. */
    @Test fun framesDark() = frames(DarkPal, "dark")
    @Test fun framesLight() = frames(LightPal, "light")

    private fun frames(pal: GregalPalette, pn: String) {
        ui.mainClock.autoAdvance = false
        val names = listOf("two", "six", "parties")
        val specs = listOf(card(two), card(six), card(parties))
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides true, LocalPal provides pal) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    Column(Modifier.fillMaxWidth().background(pal.surface)) {
                        specs.forEachIndexed { i, s -> Box(Modifier.testTag(names[i])) { RichCardBlock(s, Sea, "ca") } }
                    }
                }
            }
        }
        ui.mainClock.advanceTimeBy(100)
        names.forEach { save(capture(it), "$it-$pn-100ms") }
        ui.mainClock.advanceTimeBy(1500)
        names.forEach { save(capture(it), "$it-$pn") }
        ui.onNodeWithTag("compare-pager").performTouchInput { swipeLeft() }
        ui.mainClock.advanceTimeBy(1500)
        save(capture("parties"), "parties-$pn-page2")
    }

    private fun capture(tag: String = "shot"): Bitmap {
        val bounds = ui.onNodeWithTag(tag).fetchSemanticsNode().boundsInRoot
        lateinit var bitmap: Bitmap
        ui.runOnIdle {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).single()
            fun composeView(view: android.view.View): android.view.View? {
                if (view.javaClass.simpleName == "AndroidComposeView") return view
                if (view is android.view.ViewGroup) for (i in 0 until view.childCount) composeView(view.getChildAt(i))?.let { return it }
                return null
            }
            val view = requireNotNull(composeView(activity.window.decorView))
            val root = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
            view.draw(android.graphics.Canvas(root))
            bitmap = Bitmap.createBitmap(root, bounds.left.toInt(), bounds.top.toInt(), bounds.width.toInt(), bounds.height.toInt())
        }
        return bitmap
    }

    private fun save(shot: Bitmap, name: String) {
        java.io.File("../../output/ui-review/116/$name.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
            shot.compress(Bitmap.CompressFormat.PNG, 100, it)
        }
    }
}
