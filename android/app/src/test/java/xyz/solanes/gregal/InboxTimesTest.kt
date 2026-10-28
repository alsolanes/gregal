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

    @Test fun `la llista fa servir el resum si el fil no s'ha carregat`() {
        val w = Wind("m1", "Marinada", "#EFC69A", "compres")
        val sum = mapOf("wind:m1" to ConvoSummary("wind:m1", "pet", "", "Bon dia!", 5_000L, 2))
        val row = ventsInbox(listOf(w), emptyList(), emptyMap(), emptyMap(), "", sum).single()
        assertEquals("Bon dia!", row.preview)
        assertEquals(5_000L, row.timestamp)
        assertEquals(true, row.hasMessages)
        // Sense resum ni fil: és un vent amb qui encara no has parlat.
        val idle = ventsInbox(listOf(w), emptyList(), emptyMap(), emptyMap()).single()
        assertEquals(false, idle.hasMessages)
    }
}
