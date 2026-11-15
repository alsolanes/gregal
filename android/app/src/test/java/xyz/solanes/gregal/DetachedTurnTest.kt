package xyz.solanes.gregal

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** Torn que ha perdut el flux (has sortit de l'app): quan hi ha la resposta al fil del servidor. */
class DetachedTurnTest {
    private fun u(t: String) = GregalViewModel.PetMsg("user", t)
    private fun p(t: String, id: String = "") = GregalViewModel.PetMsg("pet", t, id = id)

    @Test fun laRespostaNovaDespresDelMissatge() {
        val known = listOf(u("hola"), p("bon dia", "m1"), u("quin temps fa?"))
        assertFalse("encara no ha respost", DetachedTurn.answered(listOf(u("hola"), p("bon dia", "m1")), known, "quin temps fa?"))
        assertTrue(DetachedTurn.answered(listOf(u("hola"), p("bon dia", "m1"), u("quin temps fa?"), p("Sol, 18 °C", "m2")), known, "quin temps fa?"))
    }

    @Test fun elMateixTextDuesVegadesNoEnganya() {
        val known = listOf(u("i ara?"), p("primera", "m1"), u("i ara?"))
        // Al servidor només hi ha la primera vegada: la resposta vella no compta.
        assertFalse(DetachedTurn.answered(listOf(u("i ara?"), p("primera", "m1")), known, "i ara?"))
    }

    @Test fun continuaAllargaElMateixMissatge() {
        val known = listOf(u("explica-ho"), p("Primer punt", "m1"))
        assertFalse(DetachedTurn.answered(listOf(u("explica-ho"), p("Primer punt", "m1")), known, ""))
        assertTrue(DetachedTurn.answered(listOf(u("explica-ho"), p("Primer punt. I el segon.", "m1")), known, ""))
    }
}
