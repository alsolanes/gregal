package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** B6: llista de responsabilitats, pausa i «per què ho has fet?» (dades com les de l'API). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class ResponsabilitatsTest {
    @get:Rule val ui = createComposeRule()

    private val listJson = """
    {"responsabilitats":[
      {"id":"resp-1","wind":"xaloc","windName":"Xaloc","kind":"watch","objective":"Horaris del bus de l'aeroport",
       "sources":[{"kind":"web","url":"https://example.org/bus","label":"https://example.org/bus","checkedAt":"2026-11-11T09:00:00Z"}],
       "allowed":["explore","prepare"],"askFirst":[],"budget":{"prepsPerDay":6,"messagesPerDay":3,"maxSteps":8},
       "spent":{"day":"2026-11-11","preps":1,"messages":1},"state":"active","everyMinutes":60,"origin":"conversation",
       "pending":0,"log":[{"at":"2026-11-11T09:00:00Z","kind":"prepared","sources":["https://example.org/bus"],
       "changes":["New or changed:\nEl bus A1 deixa de parar a la T2"],"runId":412,"messageId":"m-1","resultId":"res-1",
       "result":"Caldrà l'A2.","proposal":"Reservar el bus A2"}]},
      {"id":"cor-garbi","wind":"garbi","windName":"Garbí","kind":"heartbeat","objective":"música",
       "sources":[{"kind":"topic","label":"música"}],"allowed":["explore","prepare"],"askFirst":[],
       "budget":{"prepsPerDay":48,"messagesPerDay":6},"spent":{"preps":0,"messages":0},"state":"paused","origin":"heartbeat",
       "pending":0,"log":[]}
    ]}""".trimIndent()

    @Test fun parsesTheList() {
        val list = parseResponsabilitats(JSONObject(listJson))
        assertEquals(2, list.size)
        val r = list[0]
        assertEquals("resp-1", r.id)
        assertEquals("Xaloc", r.windName)
        assertEquals(1, r.prepsToday)
        assertEquals(6, r.prepsBudget)
        assertEquals("prepared", r.log[0].kind)
        assertEquals(412L, r.log[0].runId)
        assertEquals("Reservar el bus A2", r.log[0].proposal)
        assertEquals("heartbeat", list[1].kind)
        assertTrue(parseResponsabilitats(JSONObject("{}")).isEmpty())
    }

    @Test fun stateAndActions() {
        val (watch, heart) = parseResponsabilitats(JSONObject(listJson))
        assertEquals("active", respStateKey(watch))
        assertEquals(listOf("pause", "finish"), respActions(watch))
        // El cor es pausa però no s'acaba ni s'esborra.
        assertEquals(listOf("resume"), respActions(heart))
        assertEquals("waitingComputer", respStateKey(watch.copy(waitingComputer = true)))
        assertEquals(listOf("delete"), respActions(watch.copy(state = "done")))
        assertEquals("Avui: 1/6 preparacions · 1/3 missatges", respBudgetLine("ca", watch))
        assertEquals("consultar, preparar (comparar, resumir, esborrany)", respCanDo("ca", watch))
    }

    @Test fun whyParsesAndRejectsGarbage() {
        val w = parseRespWhy(JSONObject("""{"responsabilitat":${JSONObject(listJson).getJSONArray("responsabilitats").getJSONObject(0)},
            "act":{"kind":"prepared","sources":["https://example.org/bus"],"changes":["El bus A1 deixa de parar a la T2"],"messageId":"m-1"}}"""))
        assertEquals("Horaris del bus de l'aeroport", w!!.item.objective)
        assertEquals("m-1", w.act.messageId)
        assertNull(parseRespWhy(JSONObject("""{"error":"x"}""")))
    }

    @Test fun cardPausesAndAsksWhy() {
        val r = parseResponsabilitats(JSONObject(listJson))[0]
        var action = ""
        var why: RespAct? = null
        ui.setContent { MaterialTheme { RespCard(r, "ca", Color(0xFF3FA7B5), onAction = { action = it }, onWhy = { why = it }) } }
        ui.onNodeWithTag("resp-state").assertTextEquals("Activa")
        ui.onNodeWithText("Horaris del bus de l'aeroport").assertExists()
        ui.onNodeWithText("Enviar, comprar o publicar: sempre et pregunta", substring = true).assertExists()
        ui.onNodeWithTag("resp-pause").performClick()
        assertEquals("pause", action)
        ui.onNodeWithTag("resp-why").performClick()
        assertEquals("res-1", why?.resultId)
    }

    @Test fun whyShowsGoalSourcesAndChanges() {
        val r = parseResponsabilitats(JSONObject(listJson))[0]
        ui.setContent { MaterialTheme { RespWhyBody(RespWhy(r, r.log[0]), "en") } }
        ui.onNodeWithText("Why did you do this?").assertExists()
        ui.onNodeWithText("Horaris del bus de l'aeroport").assertExists()
        ui.onNodeWithText("· https://example.org/bus").assertExists()
        ui.onNodeWithText("El bus A1 deixa de parar a la T2", substring = true).assertExists()
        ui.onNodeWithText("Reservar el bus A2").assertExists()
    }

    @Test fun whyForANonResponsibilityMessage() {
        ui.setContent { MaterialTheme { RespWhyBody(null, "ca") } }
        ui.onNodeWithText("Aquest missatge no ve de cap responsabilitat.").assertExists()
    }

    @Test fun copyHasBothLanguages() {
        val keys = listOf("title", "pause", "resume", "finish", "why", "askAlways", "held", "act.prepared", "act.deferred", "waitingComputer")
        keys.forEach { k ->
            assertFalse(k, respCopy("ca", k) == k)
            assertFalse(k, respCopy("en", k) == respCopy("ca", k))
        }
    }
}
