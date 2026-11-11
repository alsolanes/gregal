package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** B3: el full «Ordinador» i la targeta de handoff (sense servidor ni VM: dades del contracte). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class OrdinadorSheetTest {
    @get:Rule val ui = createComposeRule()

    @Test fun parsesTheViewerAnswer() {
        val v = parseOrdinadorView(JSONObject("""{"id":"pc-carla","health":"starting","controlOwner":"user","leaseExpiry":"2026-11-11T10:00:00Z","viewer":true,"handoff":"wa-1","url":"/ordinador/visor/entra?t=abc","expiresAt":"x"}"""))
        assertEquals("starting", v.health)
        assertEquals("user", v.controlOwner)
        assertEquals("2026-11-11T10:00:00Z", v.leaseExpiry)
        assertEquals("wa-1", v.handoff)
        assertEquals("", parseOrdinadorView(JSONObject("""{"health":"running","leaseExpiry":null}""")).leaseExpiry)
        assertEquals("none", parseOrdinadorView(JSONObject("""{"health":"running"}""")).controlOwner)
    }

    @Test fun stateKeysAreHonest() {
        assertEquals("user", ordinadorStateKey(OrdinadorView("running", "user")))
        assertEquals("running", ordinadorStateKey(OrdinadorView("running", "agent")))
        assertEquals("idle", ordinadorStateKey(OrdinadorView("running", "none")))
        assertEquals("starting", ordinadorStateKey(OrdinadorView("starting")))
        assertEquals("stopped", ordinadorStateKey(OrdinadorView("stopped")))
        // Una salut desconeguda mai no es presenta com «funciona».
        assertEquals("unreachable", ordinadorStateKey(OrdinadorView("")))
    }

    @Test fun viewerUrlIsAbsoluteAndCarriesTheLanguage() {
        assertEquals("http://10.0.0.2:8097/ordinador/visor/entra?t=abc&lang=ca",
            viewerAbsoluteUrl("http://10.0.0.2:8097/", "/ordinador/visor/entra?t=abc", "ca"))
        assertEquals("https://g.example/ordinador/visor/entra?t=abc&lang=en",
            viewerAbsoluteUrl("https://g.example", "ordinador/visor/entra?t=abc", "en"))
        assertEquals("", viewerAbsoluteUrl("http://x", "", "ca"))
    }

    @Test fun copyExistsInBothLanguages() {
        val keys = listOf("title", "starting", "startingSub", "running", "runningSub", "idle", "idleSub", "user", "userSub",
            "stopped", "stoppedSub", "unreachable", "unreachableSub", "noViewer", "error", "take", "give", "done", "cancel",
            "close", "screen", "handoffTitle", "handoffHint", "open")
        for (k in keys) {
            val ca = ordinadorCopy("ca", k)
            val en = ordinadorCopy("en", k)
            assertTrue("falta $k", ca != k && en != k)
            if (k != "title") assertTrue("$k igual en ca i en", ca != en)
        }
    }

    @Test fun handoffCardAnswersDoneOrCancel() {
        var answered = ""
        ui.setContent {
            MaterialTheme {
                HandoffCard(Conn("", "", ""), "ca",
                    GregalApi.WindAsk(key = "wa-1", type = "handoff", name = "browser", kind = "login", what = "Inicia sessió a Exemple"),
                    Color(0xFF8193FF), onDone = { answered = "done" }, onCancel = { answered = "cancel" })
            }
        }
        ui.onNodeWithText("Et toca a tu a l'ordinador").assertIsDisplayed()
        ui.onNodeWithText("Inicia sessió a Exemple").assertIsDisplayed()
        ui.onNodeWithTag("handoff-open").assertHasClickAction()
        ui.onNodeWithTag("handoff-done").performClick()
        assertEquals("done", answered)
        ui.onNodeWithTag("handoff-cancel").performClick()
        assertEquals("cancel", answered)
    }
}
