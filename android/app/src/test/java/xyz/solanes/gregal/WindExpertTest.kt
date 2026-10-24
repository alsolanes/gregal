package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

/** L'estudi «fes-te expert»: fases i progrés. */
class WindExpertTest {
    @Test fun fases() {
        val inactiu = GregalApi.WindExpert("a", "", "", 0, 8, 0, 0)
        assertFalse(inactiu.actiu); assertFalse(inactiu.exit)
        val cercant = GregalApi.WindExpert("a", "bàsquet", "cerca", 3, 8, 0, 0)
        assertTrue(cercant.actiu); assertFalse(cercant.exit)
        val aprenent = GregalApi.WindExpert("a", "bàsquet", "apren", 8, 8, 0, 0)
        assertTrue(aprenent.actiu)
        val fet = GregalApi.WindExpert("a", "bàsquet", "fet", 8, 8, 0, 0)
        assertFalse(fet.actiu); assertTrue(fet.exit)
        val error = GregalApi.WindExpert("a", "bàsquet", "error", 8, 8, 0, 0, "x")
        assertFalse(error.actiu); assertFalse(error.exit)
    }

    @Test fun progresCreixIAcabaAl100() {
        val zero = GregalApi.WindExpert("a", "t", "cerca", 0, 8, 0, 0).progres
        val mig = GregalApi.WindExpert("a", "t", "cerca", 4, 8, 0, 0).progres
        val fet = GregalApi.WindExpert("a", "t", "fet", 8, 8, 0, 0).progres
        assertTrue(zero >= 0f && zero < mig && mig < fet)
        assertEquals(1f, fet, 0.001f)
        // passos 0 → sense divisió per zero
        assertEquals(0f, GregalApi.WindExpert("a", "t", "cerca", 0, 0, 0, 0).progres, 0.001f)
    }
}
