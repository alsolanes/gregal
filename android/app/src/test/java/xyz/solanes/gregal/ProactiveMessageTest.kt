package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** B6: un missatge proactiu del fil porta el run i els resultats; només aquest ofereix «per què?». */
class ProactiveMessageTest {
    private fun decode(json: String) = GregalJson.decodeFromString(HistoryDto.serializer(), json).history.map { it.toPetMsg() }

    @Test fun proactiveMessageKeepsRunAndRefs() {
        val msgs = decode("""{"history":[
            {"id":"m-1","from":"user","text":"vigila el preu"},
            {"id":"m-2","from":"pet","text":"Ha baixat","relatedRunId":42,"resultRefs":["res-1"]},
            {"id":"m-3","from":"pet","text":"Hola"},
            {"from":"pet","text":"antic","relatedRunId":7}
        ]}""")
        assertEquals(42L, msgs[1].relatedRunId)
        assertEquals(listOf("res-1"), msgs[1].resultRefs)
        assertTrue(msgs[1].proactive)
        assertFalse("la persona no rep «per què?»", msgs[0].proactive)
        assertFalse("un missatge normal tampoc", msgs[2].proactive)
        assertFalse("sense id no es pot preguntar al servidor", msgs[3].proactive)
    }

    @Test fun oldServerWithoutFields() {
        val m = decode("""{"history":[{"id":"h-ab","from":"pet","text":"x"}]}""").single()
        assertEquals(0L, m.relatedRunId)
        assertTrue(m.resultRefs.isEmpty())
        assertFalse(m.proactive)
    }
}
