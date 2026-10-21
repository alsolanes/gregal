package xyz.solanes.gregal

import androidx.compose.ui.graphics.Color
import org.junit.Assert.*
import org.junit.Test

/** L'accent clar (groc pàl·lid) sobre bombolla CLARA no es llegeix:
 *  el helper l'ha d'enfosquir; el fosc (Sea) ha de quedar intacte.
 *  Sobre bombolla FOSCA és al revés: el clar ja es llegeix, el molt
 *  fosc s'ha d'aclarir. */
class LinkColorTest {

    private fun lum(c: Color) = 0.2126f * c.red + 0.7152f * c.green + 0.0722f * c.blue

    @Test
    fun accentClarSenfosqueixEnClar() {
        val clar = Color(0xFFC6E86B) // verd-groc pàl·lid d'un vent
        val fosc = readableLink(clar, dark = false)
        assertTrue("l'enllaç ha de perdre lluminositat", lum(fosc) < lum(clar) * 0.6f)
        assertTrue("l'enllaç ha de quedar llegible sobre blanc", lum(fosc) < 0.45f)
    }

    @Test
    fun accentFoscEsQuedaComEstaEnClar() {
        val sea = Color(0xFF0E7C86)
        assertEquals(sea, readableLink(sea, dark = false))
    }

    @Test
    fun accentClarEsQuedaComEstaEnFosc() {
        val clar = Color(0xFFC6E86B)
        assertEquals(clar, readableLink(clar, dark = true))
    }

    @Test
    fun accentMoltFoscSaclareixEnFosc() {
        val negreVerd = Color(0xFF14301F) // gairebé negre verdós
        val clar = readableLink(negreVerd, dark = true)
        assertTrue("l'enllaç ha de guanyar lluminositat", lum(clar) > 0.45f)
    }
}
