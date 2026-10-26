package xyz.solanes.gregal

import java.time.Instant
import org.junit.Assert.*
import org.junit.Test

/** Programats: format d'hora dels avisos programats. */
class ProgramatsTest {
    @Test fun horaAvuiNomésHHPuntMM() {
        val now = Instant.parse("2026-10-24T10:00:00Z")
        val avui = "2026-10-24T15:30:00Z"
        val s = programatHora(avui, now)
        assertTrue("avui ha de ser HH:mm però és \"$s\"", s.matches(Regex("\\d\\d:\\d\\d")))
    }
    @Test fun horaAltreDiaPortaElDia() {
        val now = Instant.parse("2026-10-24T10:00:00Z")
        val dema = "2026-10-26T01:15:00Z"
        val s = programatHora(dema, now)
        assertTrue("un altre dia ha de portar la data: \"$s\"", s.contains("d") || s.length > 5)
    }
    @Test fun horaBuidaOInvàlidaÉsBuida() {
        assertEquals("", programatHora("", Instant.now()))
        assertEquals("", programatHora("no-una-data", Instant.now()))
    }
    @Test fun dataEnLaLlenguaDeLApp() {
        val now = Instant.parse("2026-10-24T10:00:00Z")
        val s = programatHora("2026-10-27T10:00:00Z", now, "en")
        assertFalse("en anglès no hi ha d'haver dies en català: \"$s\"", s.contains("dt.") || s.contains("d’oct"))
    }
    @Test fun miradaVencudaNoEsPresentaComFutura() {
        val now = Instant.parse("2026-10-26T10:00:00Z")
        assertTrue(programatProperaMirada("2026-10-24T21:07:00Z", "en", now).startsWith("Was due"))
        assertTrue(programatProperaMirada("2026-10-26T21:07:00Z", "en", now).startsWith("Next look"))
    }
}
