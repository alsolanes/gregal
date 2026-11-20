package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** On va la ratlla «Nous» d'un vent: es fixa una vegada per obertura, amb el fil del servidor. */
class WindNewMarksTest {
    @Test fun `la ratlla espera el fil del servidor, no la copia`() {
        val marks = WindNewMarks()
        marks.opened("x", unread = 2)
        assertTrue("encara no se sap on", marks.pending("x"))
        // La còpia del mòbil (més curta, sense les novetats) no hi compta: només el servidor.
        marks.fromServer("x", serverCount = 10)
        assertEquals(8, marks.at.value["x"])
        assertFalse(marks.pending("x"))
    }

    @Test fun `una recarrega despres d'un torn no mou la ratlla`() {
        val marks = WindNewMarks()
        marks.opened("x", unread = 2)
        marks.fromServer("x", serverCount = 10)
        marks.fromServer("x", serverCount = 14)
        assertEquals(8, marks.at.value["x"])
    }

    @Test fun `sense novetats no hi ha ratlla ni espera, i tornar a obrir la treu`() {
        val marks = WindNewMarks()
        marks.opened("x", unread = 0)
        assertFalse(marks.pending("x"))
        marks.fromServer("x", serverCount = 10)
        assertEquals(null, marks.at.value["x"])
        marks.opened("y", unread = 3)
        marks.fromServer("y", serverCount = 5)
        marks.opened("y", unread = 0)
        assertEquals(null, marks.at.value["y"])
    }

    @Test fun `si el servidor no respon, no s'espera per sempre`() {
        val marks = WindNewMarks()
        marks.opened("x", unread = 2)
        marks.settle("x")
        assertFalse(marks.pending("x"))
        assertEquals(null, marks.at.value["x"])
    }
}
