package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

/** Mencions @Nom: desplegable mentre s'escriu i resolució en enviar. */
class MentionTest {
    private val xaloc = Wind("x", "Xaloc", "#EFA68F", "música")
    private val marinada = Wind("m", "Marinada", "#EFC69A", "calma")
    private val winds = listOf(xaloc, marinada)

    @Test fun `arroba sola ofereix tots`() {
        val q = activeMention("@", 1, winds)
        assertNotNull(q)
        assertEquals(2, q!!.cands.size)
    }
    @Test fun `fragment filtra`() {
        val q = activeMention("fes @xa", 7, winds)
        assertEquals(listOf(xaloc), q!!.cands)
    }
    @Test fun `fora de token no hi ha query`() {
        assertNull(activeMention("hola xaloc", 10, winds))
        assertNull(activeMention("fes @xa ves", 11, winds))
    }
    @Test fun `cal mid paraula no compta`() {
        assertNull(activeMention("correu@xa", 9, winds))
    }
    @Test fun `resolve troba el vent`() {
        assertEquals(xaloc, resolveMention("@Xaloc mira això", winds))
        assertEquals(marinada, resolveMention("pregunta a @marinada si ve", winds))
    }
    @Test fun `resolve sense coincidencia retorna null`() {
        assertNull(resolveMention("hola @Ningu que tal", winds))
        assertNull(resolveMention("sense mencions", winds))
    }
}
