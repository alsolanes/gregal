package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Test

class InboxTimesTest {
    @Test fun `les hores del servidor en segons passen a mil-lisegons`() {
        assertEquals(1_793_111_239_000L, msgMillis(1_793_111_239L))
        assertEquals(1_793_111_239_000L, msgMillis(1_793_111_239_000L))
        assertEquals(0L, msgMillis(0L))
    }

    @Test fun `hora de la fila segons el dia`() {
        val z = java.time.ZoneId.of("Europe/Madrid")
        val now = java.time.ZonedDateTime.of(2026, 10, 27, 15, 30, 0, 0, z).toInstant().toEpochMilli()
        fun at(d: Int, h: Int) = java.time.ZonedDateTime.of(2026, 10, d, h, 5, 0, 0, z).toInstant().toEpochMilli()
        assertEquals("09:05", inboxWhen(at(27, 9), "ca", now, z))
        assertEquals("ahir", inboxWhen(at(26, 18), "ca", now, z))
        assertEquals("yesterday", inboxWhen(at(26, 18), "en", now, z))
        assertEquals("1 Oct", inboxWhen(at(1, 18), "en", now, z))
    }
}
