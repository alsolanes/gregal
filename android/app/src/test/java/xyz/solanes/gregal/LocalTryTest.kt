package xyz.solanes.gregal

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.LooperMode

/** «Prova-ho ara» d'«A prop teu»: mateixa API i regles que vents-local.js. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h2400dp")
@LooperMode(LooperMode.Mode.PAUSED)
class LocalTryTest {
    @get:Rule val ui = createComposeRule()

    private val raw = """{"id":"t1","state":"done","place":"Vic","wind":"Ponent","seconds":41,"dropped":1,
        "searches":["festa major Vic 2026"],
        "items":[{"section":"ahead","topic":"festes","title":"Festa Major de Vic","detail":"del 6 al 10","url":"https://vic.cat/fm","host":"vic.cat"},
                 {"section":"fyi","topic":"news","title":"Sense font","url":"javascript:x","host":""},
                 {"section":"fyi","topic":"news","title":"Ja dit","url":"https://x.cat/a","host":"x.cat","repeated":true}]}"""

    @Test fun elCosPortaElFormulariPeroMaiConsentimentNiCoordenades() {
        val i = LocalInfo(enabled = true, consent = true, place = LocalPlace("Vic", "Osona"), topics = listOf("festes"), wind = "ponent")
        val o = LocalJson.tryBody(i)
        assertEquals("try", o.getString("action"))
        assertEquals("Vic", o.getJSONObject("place").getString("label"))
        assertEquals("ponent", o.getString("wind"))
        assertFalse(o.has("enabled") || o.has("consent") || o.has("lat"))
    }

    @Test fun llegeixElResultatITreuElQueNoTeFont() {
        val r = LocalJson.tryResult(raw)
        assertEquals(2, r.items.size)
        assertTrue(r.items[1].repeated)
        assertEquals(listOf("festa major Vic 2026"), r.searches)
        assertEquals("Ha trobat 2 coses a Vic en 41 s. Activat, t’arribarien així a «Avui»:", localTryHeadline("ca", r))
        assertTrue(localTryHeadline("ca", r.copy(items = emptyList(), nothing = true)).startsWith("No ha trobat res nou a Vic"))
        assertEquals("Ponent està mirant què hi ha a Vic… (12 s)", localTryHeadline("ca", LocalTryResult(place = "Vic", wind = "Ponent", seconds = 12)))
    }

    @Test fun esPotProvarSenseActivarIElResultatSurtALaTargeta() {
        val tried = mutableListOf<LocalInfo>()
        val off = LocalInfo(enabled = false, consent = false, place = LocalPlace("Vic"), topics = listOf("festes"),
            catalog = listOf(LocalTopic("festes", "Festes majors i populars")))
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    Box(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
                        LocalInfoCard("ca", off, emptyList(), null, null,
                            onSave = {}, onLocate = { _, _ -> }, onLocateFailed = {}, onRun = {}, onForget = {},
                            tryResult = LocalJson.tryResult(raw), onTry = { tried += it })
                    }
                }
            }
        }
        ui.onNodeWithTag("local-head").performClick()
        ui.onNodeWithTag("local-try").performScrollTo().performClick()
        assertEquals("Vic", tried.single().place.label)
        ui.onNodeWithTag("local-try-head").performScrollTo().assertTextContains("Ha trobat 2 coses a Vic", substring = true)
        ui.onNodeWithText("Festa Major de Vic").assertExists()
        ui.onNodeWithText("Font: vic.cat").assertExists()
        ui.onNodeWithText("Ja te l’havia dit: activat, no el repetiria.").assertExists()
    }

    @Test fun desDelMenuLaTargetaJaSurtOberta() {
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    Box(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
                        LocalInfoCard("ca", LocalInfo(), emptyList(), null, null,
                            onSave = {}, onLocate = { _, _ -> }, onLocateFailed = {}, onRun = {}, onForget = {},
                            startOpen = true)
                    }
                }
            }
        }
        ui.onNodeWithTag("local-try").performScrollTo().assertExists()
    }

    @Test fun unTallDeXarxaCurtNoFaFallarLaProva() {
        val blip = java.io.IOException("timeout")
        assertFalse(LocalLogic.tryGivesUp(blip, 1))
        assertFalse(LocalLogic.tryGivesUp(ApiException("HTTP 502: bad gateway"), 3))
        assertTrue(LocalLogic.tryGivesUp(blip, LocalLogic.TRY_MAX_MISSES))
        assertTrue(LocalLogic.tryGivesUp(ApiException("HTTP 404: aquesta prova ja no hi és"), 1))
    }
}
