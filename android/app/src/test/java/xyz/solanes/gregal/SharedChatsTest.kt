package xyz.solanes.gregal

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class SharedChatsTest {
    private val listJson = """
        {"me":"carla",
         "chats":[{"id":"excursio-1","name":"Excursió","owner":"aleix","mine":false,"updated":1700000000,
                   "winds":[{"id":"mestral","name":"Mestral","color":"#3366FF","style":"pso","role":"muntanya"}],
                   "people":[{"name":"carla","status":"accepted"},{"name":"pere","status":"invited"}],
                   "last":{"author_name":"Mestral","text":"Fa sol","ts":1700000000}}],
         "invites":[{"id":"sopar-2","name":"Sopar","owner":"pere","mine":false,"winds":[],"people":[{"name":"carla","status":"invited"}]}]}
    """.trimIndent()

    @Test
    fun parsesChatsAndInvites() {
        val (me, chats, invites) = SharedChatsJson.list(JSONObject(listJson))
        assertEquals("carla", me)
        assertEquals(1, chats.size)
        val c = chats[0]
        assertEquals("aleix", c.owner)
        assertEquals(listOf("aleix", "carla"), c.humans) // el Pere encara no ha acceptat
        assertEquals("Mestral", c.winds.single().name)
        assertEquals(1700000000_000L, c.updated)
        assertEquals("Fa sol", c.lastText)
        assertEquals("sopar-2", invites.single().id)
    }

    @Test
    fun historyKnowsWhoWroteEachMessage() {
        val h = SharedChatsJson.history(JSONObject("""
            {"history":[
              {"from":"user","text":"Hola!","ts":10,"speaker_id":"user:aleix","speaker_name":"aleix","author":"aleix","mine":false},
              {"from":"user","text":"Ei","ts":11,"speaker_id":"user:carla","speaker_name":"carla","author":"carla","mine":true},
              {"from":"pet","text":"Bon dia a tots dos","ts":12,"speaker_id":"mestral","speaker_name":"Mestral"}]}
        """.trimIndent()))
        // Un missatge d'una altra persona va a l'esquerra amb el seu nom; el meu, a la dreta.
        assertEquals("pet", h[0].asPetMsg().from)
        assertEquals("aleix", h[0].asPetMsg().speakerName)
        assertEquals("user", h[1].asPetMsg().from)
        assertEquals(12_000L, h[2].asPetMsg().ts)
        assertEquals("mestral", h[2].asPetMsg().speakerId)
    }

    @Test
    fun saveBodyCarriesWindsAndPeople() {
        val o = SharedChatsJson.save(" Excursió ", listOf("mestral"), listOf("carla", "pere"))
        assertEquals("Excursió", o.getString("name"))
        assertEquals(2, o.getJSONArray("people").length())
        assertFalse(o.has("id"))
        assertTrue(SharedChatsJson.save("X", listOf("a"), listOf("b"), id = "x-1").has("id"))
    }

    @Test
    fun inviteNotifiesOnlyOnce() {
        val (_, _, invites) = SharedChatsJson.list(JSONObject(listJson))
        assertEquals(1, SharedInviteWatch.fresh(invites, emptySet()).size)
        assertTrue(SharedInviteWatch.fresh(invites, setOf("sopar-2")).isEmpty())
    }

    @Test
    fun sharedStringsHaveBothLanguages() {
        for (k in STR_APP_CA.keys.filter { it.startsWith("app.shared.") }) assertTrue(k, STR_APP_EN.containsKey(k))
        assertEquals("aleix et convida a «Excursió».", tr("ca", "app.shared.inviteText", "aleix", "Excursió"))
    }
}
