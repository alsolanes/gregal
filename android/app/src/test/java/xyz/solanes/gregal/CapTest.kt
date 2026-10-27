package xyz.solanes.gregal

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class CapTest {
    @Test fun `la decisio de delegar porta qui i el fil`() {
        val d = CapDecision.parse(JSONObject("""{"action":"wind","reply":"Això és cosa de la Marinada","winds":["m1"],
            "history":[{"from":"user","text":"sopar?"},{"from":"cap","text":"Això és cosa de la Marinada","action":"wind","winds":["m1"]}]}"""))
        assertTrue(d.delegates)
        assertEquals(listOf("m1"), d.winds)
        assertEquals(2, d.history.size)
        assertEquals(listOf("m1"), d.history[1].winds)
    }

    @Test fun `si respon ella no hi ha res a fer`() {
        assertFalse(CapDecision.parse(JSONObject("""{"action":"self","reply":"Bon dia"}""")).delegates)
        assertFalse(CapDecision.parse(JSONObject("""{"reply":"?"}""")).delegates)
    }

    @Test fun `el resum compta el que hi ha de nou`() {
        val st = CapState.parse(JSONObject("""{"name":"Bufi","brief":{"done":1,"failed":1,"running":3,"news":2,"text":"Ja està llesta"}}"""))
        assertEquals("Bufi", st.name)
        assertEquals(4, st.briefCount)
        assertEquals("organisation", st.faceStyle.kit)
    }

    @Test fun `nom de la rotllana nova`() {
        assertEquals("Marinada i Garbí", capRotllanaName(listOf("Marinada", "Garbí"), "ca"))
        assertEquals("A, B & C", capRotllanaName(listOf("A", "B", "C"), "en"))
    }
}
