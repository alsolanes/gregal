package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import java.time.Instant

@RunWith(RobolectricTestRunner::class)
class ProgramacionsTest {
    private val now = Instant.parse("2026-11-11T10:00:00Z")

    @Test fun relativeNearAndFar() {
        assertEquals("d'aquí a 25 min", automationRelative("ca", "2026-11-11T10:25:00Z", now))
        assertEquals("in 3 h", automationRelative("en", "2026-11-11T13:10:00+00:00", now))
        assertEquals("fa 5 min", automationRelative("ca", "2026-11-11T09:55:00Z", now))
        // Lluny o il·legible: només la data absoluta.
        assertEquals("", automationRelative("ca", "2026-11-13T10:00:00Z", now))
        assertEquals("", automationRelative("ca", "demà", now))
        assertEquals("", automationRelative("ca", "", now))
    }

    @Test fun parsesTaughtRoutineAndFullText() {
        val raw = """{"items":[{"id":"rutina-x-1","kind":"routine","wind":"x","wind_name":"Xaloc","title":"Vols",
            "detail":"Mira vols","repeat":"daily","time":"08:00","enabled":false,"can_pause":true,
            "text":"Mira vols barats","howto":"- Sense emojis","refs":2}],"counts":{"total":1,"paused":1}}"""
        val a = WindActivityJson.automations(raw).items.single()
        assertEquals("Mira vols barats", a.text)
        assertEquals("- Sense emojis", a.howto)
        assertEquals(2, a.refs)
    }

    @Test fun oldServerWithoutNewFieldsStillParses() {
        val raw = """{"items":[{"id":"remind-x-1","kind":"reminder","wind":"x","title":"ITV","repeat":"once","enabled":true}]}"""
        val a = WindActivityJson.automations(raw).items.single()
        assertEquals("", a.text)
        assertEquals("", a.howto)
        assertEquals(0, a.refs)
    }

    @Test fun b8StringsHaveParity() {
        assertEquals(STR_B8_CA.keys, STR_B8_EN.keys)
        for ((k, v) in STR_B8_CA) {
            assertEquals(k, v.count { it == '%' }, STR_B8_EN.getValue(k).count { it == '%' })
        }
        assertTrue(trB8("en", "b8.teachHelp").contains("videos"))
    }
}
