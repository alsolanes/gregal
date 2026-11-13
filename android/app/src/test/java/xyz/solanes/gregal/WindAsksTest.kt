package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** Preguntes i aprovacions dels vents: el torn espera i la targeta el desbloqueja. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindAsksTest {
    @get:Rule val ui = createComposeRule()
    private fun vm() = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
    private val api = GregalApi("http://x", "t")

    @Test fun choicesBlockBecomesQuickRepliesAndLeavesTheText() {
        val md = "Ho tinc a punt.\n\n```choices\n- Publica-la\n2. Canvia el títol\nDeixa-ho\n```"
        assertEquals(listOf("Publica-la", "Canvia el títol", "Deixa-ho"), extractChoices(md))
        assertEquals("Ho tinc a punt.", withoutChoices(md))
        // En directe, el bloc encara no està tancat: tampoc es veu.
        assertEquals("Ho tinc a punt.", withoutChoices("Ho tinc a punt.\n```choices\n- Pub"))
        // Sense bloc, el text és el mateix; mai més de 4 opcions.
        assertEquals("Hola", withoutChoices("Hola"))
        assertTrue(extractChoices("Hola").isEmpty())
        assertEquals(4, extractChoices("```choices\na\nb\nc\nd\ne\nf\n```").size)
    }

    @Test fun streamEventsCarryApprovalsAndQuestions() {
        val ap = api.parseWindStreamEvent("approve_request",
            """{"key":"wa1","type":"approve","name":"bash","kind":"install","what":"sdkmanager --licenses","reason":"x"}""")
        val a = (ap as GregalApi.WindStreamEvent.Ask).ask
        assertEquals("wa1", a.key); assertEquals("install", a.kind); assertEquals("sdkmanager --licenses", a.what)
        val q = api.parseWindStreamEvent("question_request",
            """{"key":"wa2","type":"question","query":"Quin color?","options":[{"label":"Blau"},{"label":"Verd","description":"més fosc"}]}""")
        val qa = (q as GregalApi.WindStreamEvent.Ask).ask
        assertEquals("question", qa.type); assertEquals(listOf("Blau" to "", "Verd" to "més fosc"), qa.options)
        assertEquals("wa1", (api.parseWindStreamEvent("ask_resolved", """{"key":"wa1","answered":true}""") as GregalApi.WindStreamEvent.AskResolved).key)
        assertNull(api.parseWindStreamEvent("approve_request", """{"type":"approve"}""")?.takeIf { (it as? GregalApi.WindStreamEvent.Ask)?.ask?.key.isNullOrBlank().not() })
    }

    @Test fun anApprovalCardAsksAndAnsweringClearsIt() {
        val vm = vm()
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.Ask(GregalApi.WindAsk("wa1", "approve", name = "bash", kind = "install", what = "sdkmanager --licenses")))
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { WindAskCards(vm, "en", androidx.compose.ui.graphics.Color(0xFF8899AA), busy = false) }
        } }
        ui.onNodeWithTag("approve-card").assertExists()
        ui.onNodeWithText("Shall I install software?").assertExists()
        // L'ordre va plegada sota «See the command» i s'obre amb un toc.
        ui.onNodeWithText("sdkmanager --licenses").assertDoesNotExist()
        ui.onNodeWithTag("ask-command-toggle").performClick()
        ui.onNodeWithText("sdkmanager --licenses").assertExists()
        ui.onNodeWithTag("ask-allow").performClick()
        ui.waitForIdle()
        ui.onNodeWithTag("approve-card").assertDoesNotExist()
    }

    @Test fun aQuestionCardOffersTheOptionsAndFreeText() {
        val vm = vm()
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.Ask(GregalApi.WindAsk("wa2", "question", query = "Quin color?",
            options = listOf("Blau" to "", "Verd" to "més fosc"))))
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { WindAskCards(vm, "ca", androidx.compose.ui.graphics.Color(0xFF8899AA), busy = false) }
        } }
        ui.onNodeWithText("Quin color?").assertExists()
        ui.onAllNodesWithTag("ask-option").assertCountEquals(2)
        ui.onNodeWithText("més fosc").assertExists()
        ui.onAllNodesWithTag("ask-option")[1].performClick()
        ui.waitForIdle()
        ui.onNodeWithTag("question-card").assertDoesNotExist()
    }

    @Test fun resolvedRequestsDisappearWithoutAnswering() {
        val vm = vm()
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.Ask(GregalApi.WindAsk("wa3", "approve")))
        assertEquals(1, vm.windAsks.value.size)
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.AskResolved("wa3"))
        assertTrue(vm.windAsks.value.isEmpty())
    }
}
