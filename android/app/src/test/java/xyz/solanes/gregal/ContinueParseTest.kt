package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/** Flag parcial: el done l'arrossega fins a la bombolla (botó «continua»). */
@RunWith(RobolectricTestRunner::class)
class ContinueParseTest {
    private val api = GregalApi("http://127.0.0.1:9", "")
    @Test fun `done directe amb parcial`() {
        val e = api.parseWindStreamEvent("done",
            """{"reply":"primera meitat…","wind":"xaloc","parcial":true,"history":[]}""")
        val done = e as GregalApi.WindStreamEvent.DoneWind
        assertTrue(done.reply.parcial)
    }
    @Test fun `done directe sense parcial`() {
        val e = api.parseWindStreamEvent("done",
            """{"reply":"tot sencer","wind":"xaloc","history":[]}""")
        val done = e as GregalApi.WindStreamEvent.DoneWind
        assertFalse(done.reply.parcial)
    }
    @Test fun `done rotllana marca l'ultima veu`() {
        val e = api.parseWindStreamEvent("done",
            """{"reply":{"wind":"marinada","text":"meitat…"},"parcial":true,"history":[
                {"from":"user","text":"hola"},
                {"from":"pet","text":"meitat…","speaker_id":"marinada","speaker_name":"Marinada"}]}""")
        val done = e as GregalApi.WindStreamEvent.DoneRotllana
        assertTrue(done.parcial)
        assertTrue(done.history.last().parcial)
        assertFalse(done.history.first().parcial)
    }
    @Test fun `payloads continua`() {
        val dm = org.json.JSONObject(WindRequests.chatContinue("xaloc"))
        assertEquals("xaloc", dm.getString("wind"))
        assertTrue(dm.getBoolean("continue"))
        val rot = org.json.JSONObject(RotllanaRequests.chatContinue("passejades", "marinada"))
        assertEquals("marinada", rot.getString("wind"))
        assertTrue(rot.getBoolean("continue"))
    }
}
