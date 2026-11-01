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

    // --- Enllaços copiables: extracció del text del missatge ---

    @Test
    fun treuElQueLaProsaEnganxaAlFinalDeLAdreca() {
        assertEquals("https://example.org/x", trimUrlTail("https://example.org/x)."))
        assertEquals("https://example.org/x?a=1", trimUrlTail("https://example.org/x?a=1,"))
        assertEquals("https://ca.wikipedia.org/wiki/Roure", trimUrlTail("https://ca.wikipedia.org/wiki/Roure"))
    }

    @Test
    fun elNomCurtEsElDominiSenseWww() {
        assertEquals("example.org", linkHost("https://www.example.org/a/b?c=1"))
        assertEquals("ca.wikipedia.org", linkHost("https://ca.wikipedia.org/wiki/Roure#Història"))
        assertEquals("127.0.0.1:8097", linkHost("http://user:pw@127.0.0.1:8097/x"))
    }

    @Test
    fun recullEnllacosMarkdownIAdrecesCruesSenseRepetir() {
        val text = """
            Mira [aquesta font](https://example.org/a) i també https://example.org/b.
            Repeteixo https://example.org/a i tanco amb [una altra](https://example.org/c).
        """.trimIndent()
        val links = messageLinks(text)
        assertEquals(listOf("https://example.org/a", "https://example.org/b", "https://example.org/c"), links.map { it.url })
        assertEquals("aquesta font", links[0].label)
        assertEquals("example.org", links[1].label)
    }

    @Test
    fun lesImatgesNoSonenEnllacosIMaiMésDeSis() {
        val imatge = "![portada](https://cdn.example.org/p.png)"
        assertTrue(messageLinks(imatge).isEmpty())
        val molts = (1..9).joinToString(" ") { "https://example.org/$it" }
        assertEquals(6, messageLinks(molts).size)
    }

    @Test
    fun elCodiNoComptaComAEnllac() {
        val text = "```\ncurl https://example.org/api\n```\nAra sí: https://example.org/real"
        assertEquals(listOf("https://example.org/real"), messageLinks(text).map { it.url })
    }
}
