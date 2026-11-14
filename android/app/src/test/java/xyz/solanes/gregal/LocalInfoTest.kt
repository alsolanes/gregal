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
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.LooperMode

/** «A prop teu» a l'Android: mateixes regles que vents-local.js i /api/winds/local. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h2400dp")
@LooperMode(LooperMode.Mode.PAUSED)
class LocalInfoTest {
    @get:Rule val ui = createComposeRule()

    private val raw = """{"settings":{"enabled":true,"consent":1794600000,"place":{"label":"Gràcia, Barcelona","region":"Catalunya","source":"device"},
        "topics":["concerts","news"],"interests":"jazz","avoid":"futbol","radius_km":5,"frequency":"weekly","time":"18:30","wind":"xaloc","voice":true},
        "seen":3,"topics":[{"id":"concerts","label":"Concerts i música"},{"id":"festes","label":"Festes majors i populars"},{"id":"news","label":"Notícies locals"}],
        "job":{"id":"local-me","next_due_at":"2026-11-15T18:30:00+01:00"}}"""

    @Test fun llegeixLaVistaIDiuElResum() {
        val i = LocalJson.view(raw)
        assertTrue(i.enabled && i.consent && i.voice)
        assertEquals("Gràcia, Barcelona", i.place.label)
        assertEquals("device", i.place.source)
        assertEquals(listOf("concerts", "news"), i.topics)
        assertEquals(5, i.radiusKm)
        assertEquals(3, i.catalog.size)
        assertEquals("2026-11-15T18:30:00+01:00", i.nextDueAt)
        assertEquals("Cada setmana a les 18:30 · Gràcia, Barcelona · a 5 km · Concerts i música, Notícies locals", LocalLogic.summary("ca", i))
        assertEquals("Off. Tap to set it up.", LocalLogic.summary("en", i.copy(enabled = false)))
        // Un servidor antic o una resposta buida no trenca res.
        val empty = LocalJson.view("{}")
        assertFalse(empty.enabled)
        assertEquals("daily", empty.frequency)
        assertEquals(10, empty.radiusKm)
    }

    @Test fun noSActivaSenseConsentimentLlocITemes() {
        val ok = LocalJson.view(raw)
        assertNull(LocalLogic.enableProblem(ok))
        assertEquals("app.local.needConsent", LocalLogic.enableProblem(ok.copy(consent = false)))
        assertEquals("app.local.needPlace", LocalLogic.enableProblem(ok.copy(place = LocalPlace())))
        assertEquals("app.local.needTopic", LocalLogic.enableProblem(ok.copy(topics = emptyList())))
        assertEquals(listOf("concerts", "news", "festes"), LocalLogic.toggleTopic(ok, "festes").topics)
        assertEquals(listOf("news"), LocalLogic.toggleTopic(ok, "concerts").topics)
        assertEquals("cal el teu consentiment", LocalLogic.serverMessage(ApiException("HTTP 400: cal el teu consentiment")))
    }

    @Test fun maiNoSurtenCoordenadesExactes() {
        val loc = LocalJson.locateBody(41.403712, 2.156789)
        assertEquals(41.4, loc.getDouble("lat"), 0.0)
        assertEquals(2.16, loc.getDouble("lon"), 0.0)
        assertEquals("device", loc.getString("source"))
        val body = LocalJson.body(LocalJson.view(raw))
        assertFalse(body.toString().contains("lat"))
        assertEquals("Gràcia, Barcelona", body.getJSONObject("place").getString("label"))
        assertEquals(5, body.getInt("radius_km"))
        assertFalse(LocalJson.body(LocalInfo()).has("place"))
        val p = LocalJson.located("""{"place":{"label":"Vic","region":"Catalunya","source":"device"}}""")
        assertEquals(LocalPlace("Vic", "Catalunya", "", "device"), p)
    }

    @Test fun laTargetaDemanaConsentimentIDesa() {
        val saved = mutableListOf<LocalInfo>()
        val off = LocalJson.view(raw).copy(enabled = false, consent = false, place = LocalPlace())
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    Box(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
                        LocalInfoCard("ca", off, emptyList(), null, null,
                            onSave = { saved += it }, onLocate = { _, _ -> }, onLocateFailed = {}, onRun = {}, onForget = {})
                    }
                }
            }
        }
        ui.onNodeWithText("Desactivat. Toca per configurar-ho.").assertExists()
        ui.onNodeWithTag("local-head").performClick()
        ui.onNodeWithText("Encara no has triat on ets").assertExists()
        // Sense consentiment ni lloc, no desa: diu què falta.
        ui.onNodeWithTag("local-save").performScrollTo().performClick()
        ui.onNodeWithText("Cal el teu consentiment per fer servir el teu lloc.").assertExists()
        assertTrue(saved.isEmpty())
        // Escriu el barri, afegeix festes, accepta i activa.
        ui.onNodeWithTag("local-type").performTextInput("Sants, Barcelona")
        ui.onNodeWithText("Fes-lo servir").performClick()
        ui.onNodeWithTag("local-place").assertTextEquals("Sants, Barcelona")
        ui.onNodeWithTag("local-topic-festes").performScrollTo().performClick()
        ui.onNodeWithTag("local-radius-25").performScrollTo().performClick()
        ui.onNodeWithTag("local-consent").performScrollTo().performClick()
        ui.onNodeWithTag("local-save").performScrollTo().performClick()
        val s = saved.single()
        assertTrue(s.enabled && s.consent)
        assertEquals(LocalPlace("Sants, Barcelona"), s.place)
        assertEquals(listOf("concerts", "news", "festes"), s.topics)
        assertEquals(25, s.radiusKm)
        assertFalse(JSONObject(LocalJson.body(s).toString()).toString().contains("lat"))
    }

    @Test fun elLlocTrobatPelMobilOmpleLaTargeta() {
        val on = LocalJson.view(raw)
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides LightPal) {
                Box(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
                    LocalInfoCard("en", on, emptyList(), "You are in Vic. Save to use it.", LocalPlace("Vic", source = "device"),
                        onSave = {}, onLocate = { _, _ -> }, onLocateFailed = {}, onRun = {}, onForget = {})
                }
            }
        }
        ui.onNodeWithTag("local-summary", useUnmergedTree = true).assertTextEquals("Every week at 18:30 · Gràcia, Barcelona · within 5 km · Concerts i música, Notícies locals")
        ui.onNodeWithTag("local-head").performClick()
        ui.onNodeWithTag("local-place").assertTextEquals("Vic")
        ui.onNodeWithText("phone location").assertExists()
        ui.onNodeWithTag("local-run").assertExists()
        ui.onNodeWithTag("local-forget").assertExists()
    }
}
