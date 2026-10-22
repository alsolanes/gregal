package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

/** Lògica del vigilant: agrupa per vent, filtra velles i buides, avança el ts. */
class WindWatchTest {
    private fun n(ts: Long, wind: String, text: String) =
        WindNovetat(ts, wind, wind.replaceFirstChar { it.uppercase() }, "remind", text)

    @Test fun `agrupa per vent i avanca el ts`() {
        val (perVent, ts) = WindWatchLogic.noves(listOf(
            n(10, "xaloc", "primera"),
            n(20, "xaloc", "segona"),
            n(15, "marinada", "hola"),
        ), 0)
        assertEquals(2, perVent.size)
        assertEquals("segona", perVent["xaloc"]?.text)
        assertEquals("hola", perVent["marinada"]?.text)
        assertEquals(20L, ts)
    }

    @Test fun `ignora velles buides i sense vent`() {
        val (perVent, ts) = WindWatchLogic.noves(listOf(
            n(5, "xaloc", "vella"),
            n(30, "xaloc", "  "),
            n(30, "", "sense vent"),
            n(30, "xaloc", "nova"),
        ), 10)
        assertEquals(1, perVent.size)
        assertEquals("nova", perVent["xaloc"]?.text)
        assertEquals(30L, ts)
    }

    @Test fun `sense novetats no mou res`() {
        val (perVent, ts) = WindWatchLogic.noves(listOf(n(5, "xaloc", "vella")), 10)
        assertTrue(perVent.isEmpty())
        assertEquals(10L, ts)
    }
}
