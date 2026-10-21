package xyz.solanes.gregal

import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/** El parser SSE del directe: pur, sense xarxa. */
@RunWith(RobolectricTestRunner::class)
class WindStreamTest {
    private val api = GregalApi("http://127.0.0.1:9", "")

    @Test
    fun parsePhaseTokenDoneWind() {
        val ph = api.parseWindStreamEvent("phase", """{"phase":"searching"}""")
        assertTrue(ph is GregalApi.WindStreamEvent.Phase)
        assertEquals("searching", (ph as GregalApi.WindStreamEvent.Phase).phase)
        val tok = api.parseWindStreamEvent("token", """{"token":"Hola"}""")
        assertEquals("Hola", (tok as GregalApi.WindStreamEvent.Token).token)
        val done = api.parseWindStreamEvent("done",
            """{"reply":"Hola!","wind":"xaloc","actions":[{"kind":"remind","at":"2026-10-21T20:00:00+02:00"}]}""")
        assertTrue(done is GregalApi.WindStreamEvent.DoneWind)
        val reply = (done as GregalApi.WindStreamEvent.DoneWind).reply
        assertEquals("Hola!", reply.text)
        assertEquals(listOf("2026-10-21T20:00:00+02:00"), reply.reminds)
    }

    @Test
    fun parseDoneRotllanaPortaVeus() {
        val hist = JSONArray().put(JSONObject().put("from", "pet").put("text", "Xaloc: Hola!")
            .put("speaker_id", "xaloc").put("speaker_name", "Xaloc"))
        val done = api.parseWindStreamEvent("done", JSONObject()
            .put("reply", JSONObject().put("wind", "xaloc").put("text", "Hola!"))
            .put("history", hist).put("actions", JSONArray()).toString())
        assertTrue(done is GregalApi.WindStreamEvent.DoneRotllana)
        val d = done as GregalApi.WindStreamEvent.DoneRotllana
        assertEquals("xaloc", d.wind)
        assertEquals(1, d.history.size)
        assertEquals("xaloc", d.history[0].speakerId)
    }

    @Test
    fun parseErrorIBuits() {
        val err = api.parseWindStreamEvent("error", """{"error":"boom"}""")
        assertEquals("boom", (err as GregalApi.WindStreamEvent.Error).message)
        assertNull(api.parseWindStreamEvent("token", ""))
        assertNull(api.parseWindStreamEvent("token", "no-json"))
        assertNull(api.parseWindStreamEvent("desconegut", """{"a":1}"""))
    }
}
