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

    @Test fun `proposa especialistes nous i curs per als que no en saben`() {
        val d = CapDecision.parse(JSONObject("""{"action":"rotllana","reply":"Faig rotllana","winds":["m1"],
            "create":[{"name":"Fogony","role":"cuina","persona":"Cuina.","kit":"","color":"#F7B2D9","style":"bon"}],
            "novices":["m1"],"pet":{"happy":80,"energy":40,"full":20,"level":3,"level_xp":20,"outfits":["llacet","gorra"],"outfit":"gorra"}}"""))
        assertEquals("Fogony", d.create.single().name)
        assertEquals(listOf("m1"), d.novices)
        assertEquals(3, d.pet?.level)
        assertTrue(d.pet!!.hungry)
        assertEquals("gorra", d.pet!!.outfit)
    }

    @Test fun `pedra paper tisora`() {
        assertEquals(1, rpsWinner(1, 0))   // paper guanya pedra
        assertEquals(1, rpsWinner(0, 2))   // pedra guanya tisora
        assertEquals(-1, rpsWinner(2, 0))  // tisora perd amb pedra
        assertEquals(0, rpsWinner(2, 2))
    }

    @Test fun `els nuvols pugen i marxen`() {
        val c = CapCloud(1, .5f, born = 1000, gold = false, life = 2000)
        assertEquals(1f, cloudY(c, 1000)!!, 0.001f)
        assertEquals(.5f, cloudY(c, 2000)!!, 0.001f)
        assertEquals(null, cloudY(c, 3500))
    }

    @Test fun `la roba del cataleg coincideix amb el servidor`() {
        assertEquals(listOf("llacet", "gorra", "bufanda", "ulleres", "barret", "auriculars", "corona", "flors", "carbassa", "gorro"),
            capOutfitCatalog.map { it.id })
    }
}
