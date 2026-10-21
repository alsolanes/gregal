package xyz.solanes.gregal

import androidx.compose.ui.graphics.Color
import org.junit.Assert.*
import org.junit.Test

/** L'accent clar (groc pàl·lid) sobre bombolla blanca no es llegeix:
 *  el helper l'ha d'enfosquir; el fosc (Sea) ha de quedar intacte. */
class LinkColorTest {

    private fun lum(c: Color) = 0.2126f * c.red + 0.7152f * c.green + 0.0722f * c.blue

    @Test
    fun accentClarSenfosqueix() {
        val clar = Color(0xFFC6E86B) // verd-groc pàl·lid d'un vent
        val fosc = readableLinkOnLight(clar)
        assertTrue("l'enllaç ha de perdre lluminositat", lum(fosc) < lum(clar) * 0.6f)
        assertTrue("l'enllaç ha de quedar llegible sobre blanc", lum(fosc) < 0.45f)
    }

    @Test
    fun accentFoscEsQuedaComEsta() {
        val sea = Color(0xFF0E7C86)
        assertEquals(sea, readableLinkOnLight(sea))
    }
}
