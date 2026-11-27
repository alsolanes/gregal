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

    // Targetes per aprovar (103): la cara del vent hi seu a sobre, amb el pas i una vista prèvia.
    private val zefir = Wind("zefir", "Zèfir", "#29BCA3", "web")
    private fun offlineVm() = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        .also { it.setConn(Conn("127.0.0.1", "9", "", local = true)) }
    private fun showAsk(vm: GregalViewModel, ask: GregalApi.WindAsk, lang: String = "en") {
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.Ask(ask))
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { WindAskCards(vm, lang, androidx.compose.ui.graphics.Color(0xFF29BCA3), busy = false, wind = zefir) }
        } }
    }

    @Test fun aUrlApprovalShowsThePageUnderThePerchedFace() {
        showAsk(offlineVm(), GregalApi.WindAsk("wa4", "approve", name = "browser", kind = "rule",
            what = "browser https://www.example.org/form?x=1", detail = "https://www.example.org/form?x=1", wind = "zefir"))
        ui.onNodeWithTag("ask-face", useUnmergedTree = true).assertExists()
        ui.onNodeWithTag("ask-step").assertTextContains("Browsing", substring = true)
        ui.onNodeWithTag("ask-preview-url").assertExists()
        ui.onNodeWithText("example.org").assertExists()
        ui.onNodeWithText("https://www.example.org/form?x=1").assertExists()
        ui.onNodeWithText("Allow").assertExists()
        ui.onNodeWithText("No").assertExists()
    }

    @Test fun aDocumentApprovalShowsTheFileAndWhereItIs() {
        showAsk(offlineVm(), GregalApi.WindAsk("wa5", "approve", name = "write", what = "write C:/u/gregal/projectes/zefir/index.html",
            detail = "C:/u/gregal/projectes/zefir/index.html"), lang = "ca")
        ui.onNodeWithTag("ask-preview-doc").assertExists()
        ui.onNodeWithText("index.html").assertExists()
        ui.onNodeWithText("Projectes › zefir").assertExists()
        ui.onNodeWithText("Permet").assertExists()
        ui.onNodeWithText("No").assertExists()
    }

    @Test fun aTextApprovalShowsTheTextItWillUse() {
        showAsk(offlineVm(), GregalApi.WindAsk("wa6", "approve", name = "notes", kind = "rule",
            what = "notes create", detail = "Compra pa i llet abans de les 8"))
        ui.onNodeWithTag("ask-preview-text").assertExists()
        ui.onNodeWithText("Compra pa i llet abans de les 8").assertExists()
        // Amb la vista prèvia, l'ordre plegada no hi és: ja es veu què farà.
        ui.onNodeWithTag("ask-command-toggle").assertDoesNotExist()
    }

    @Test fun answeringCollapsesTheCardAndTheFaceNodsOrShakes() {
        val vm = offlineVm()
        showAsk(vm, GregalApi.WindAsk("wa7", "approve", name = "browser", what = "browser https://example.org", detail = "https://example.org", wind = "zefir"))
        ui.onNodeWithText("Allow").performClick()
        ui.waitForIdle()
        ui.onNodeWithTag("approve-card").assertDoesNotExist()
        ui.onNodeWithTag("ask-answered").assertTextContains("You allowed: example.org", substring = true)
        ui.onNodeWithTag("ask-face-nod", useUnmergedTree = true).assertExists()
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.Ask(GregalApi.WindAsk("wa8", "approve", name = "write", what = "write a/b/c.txt", wind = "zefir")))
        ui.waitForIdle()
        ui.onNodeWithText("No").performClick()
        ui.waitForIdle()
        ui.onAllNodesWithTag("ask-answered")[1].assertTextContains("You declined: c.txt", substring = true)
        ui.onNodeWithTag("ask-face-shake", useUnmergedTree = true).assertExists()
    }

    // 131: la targeta és del vent que pregunta, també fora del seu xat (a Feina o en un grup).
    private val windColours = listOf(0xFFF4B6C2, 0xFFEACF87, 0xFF9ED3C5, 0xFFA8CDE2, 0xFFC1B4DE, 0xFFEFA68F, 0xFF3D7BFF, 0xFF29BCA3).map { androidx.compose.ui.graphics.Color(it) }

    @Test fun allowIsInTheWindColourWithTextAt45InEveryTheme() {
        for (t in GregalTemes) for ((mode, p) in listOf("clar" to t.clar, "fosc" to t.fosc)) for (c in windColours) {
            val (fill, ink) = allowPaint(c, p.ink, p.deepTop)
            assertTrue("${t.id}/$mode $c: ink is a theme ink", ink == p.ink || ink == p.deepTop)
            val ratio = contrastWith(fill, ink)
            assertTrue("${t.id}/$mode $c: contrast %.2f < 4.5".format(ratio), ratio >= 4.5)
        }
        // Un vent pastel en fosc i el blau de Gregal en fosc es queden tal com són.
        assertEquals(windColours[3], allowPaint(windColours[3], DarkPal.ink, DarkPal.deepTop).first)
        assertEquals(windColours[6], allowPaint(windColours[6], DarkPal.ink, DarkPal.deepTop).first)
    }

    private fun showFromFeina(vm: GregalViewModel, ask: GregalApi.WindAsk) {
        (GregalViewModel::class.java.getDeclaredField("_winds").apply { isAccessible = true }.get(vm) as kotlinx.coroutines.flow.MutableStateFlow<List<Wind>>).value = listOf(zefir)
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.Ask(ask))
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { WindAskCards(vm, "en", LightPal.sea, busy = false) }
        } }
    }

    @Test fun aPermissionCardWearsTheFaceOfTheWindThatAsksAndSaysWhyOnOneLine() {
        val why = "You asked me to check with you before sending any email, and this one goes to a whole mailing list of people outside the team."
        showFromFeina(offlineVm(), GregalApi.WindAsk("wa9", "approve", name = "mail", kind = "rule", what = "mail send", reason = why, wind = "zefir"))
        ui.onNodeWithTag("ask-face", useUnmergedTree = true).assertExists()
            .assert(SemanticsMatcher.expectValue(WindFaceKey, androidx.compose.ui.graphics.Color(zefir.argb)))
        ui.onNodeWithTag("ask-shield", useUnmergedTree = true).assertDoesNotExist()
        ui.onNodeWithText("Shall I send this email?").assertExists()
        // El perquè, un cop i en una línia: no es repeteix a sota de la pregunta.
        ui.onAllNodesWithText(why, substring = true).assertCountEquals(1)
        val line = ui.onNodeWithTag("ask-why", useUnmergedTree = true).assertTextEquals(why).getBoundsInRoot()
        assertTrue("one line: ${line.bottom - line.top}", (line.bottom - line.top).value < 24f)
    }

    @Test fun aCardFromAnUnknownWindKeepsTheShield() {
        showFromFeina(offlineVm(), GregalApi.WindAsk("wa10", "approve", name = "bash", kind = "install", what = "winget install foo"))
        ui.onNodeWithTag("ask-shield", useUnmergedTree = true).assertExists()
        ui.onNodeWithTag("ask-face", useUnmergedTree = true).assertDoesNotExist()
        ui.onNodeWithTag("ask-why", useUnmergedTree = true).assertDoesNotExist()
    }

    @Test fun resolvedRequestsDisappearWithoutAnswering() {
        val vm = vm()
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.Ask(GregalApi.WindAsk("wa3", "approve")))
        assertEquals(1, vm.windAsks.value.size)
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.AskResolved("wa3"))
        assertTrue(vm.windAsks.value.isEmpty())
    }
}
