package xyz.solanes.gregal

import org.json.JSONArray
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** Contracte B0: els missatges porten id estable i la novetat apunta al missatge del fil. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class MessageIdTest {
    @Test fun novetatPortaElMissatgeDelFil() {
        val list = AvuiJson.novetats("""{"novetats":[
            {"ts":10,"wind":"xaloc","name":"Xaloc","source":"heartbeat","text":"Juga el Barça","message_id":"m-abc"},
            {"ts":11,"wind":"xaloc","name":"Xaloc","source":"corner","text":"Racó nou"}]}""")
        assertEquals("m-abc", list[0].messageId)
        assertEquals("", list[1].messageId)
    }

    @Test fun elFilDelVentetPortaIds() {
        val h = CapState.parseHistory("""[{"id":"m-1","from":"user","text":"hola"},{"from":"cap","text":"servidor antic"}]""")
        assertEquals("m-1", h[0].id)
        assertEquals("", h[1].id)
    }

    @Test fun laCacheForaDeLiniaConservaLId() {
        val raw = OfflineLogic.encodeChat(listOf(GregalViewModel.PetMsg("pet", "hola", id = "m-9")))
        assertEquals("m-9", OfflineLogic.decodeChat(raw)!!.single().id)
    }
}
