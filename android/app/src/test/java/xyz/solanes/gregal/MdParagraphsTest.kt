package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Una resposta es llegeix amb un sol ritme: cada paràgraf és un bloc amb el
 * mateix aire que una llista, el codi o una taula, en lloc d'una línia en blanc
 * sencera (26 sp) entre paràgrafs i 12 dp entre blocs.
 */
class MdParagraphsTest {
    private fun texts(src: String) = mdParagraphs((groupRich(splitMd(src)).single() as Rich.Inline).segs)
        .map { p -> renderSegs(p, androidx.compose.ui.graphics.Color.Unspecified).text }

    @Test fun `una linia en blanc parteix els paragrafs i no en queda cap salt`() {
        assertEquals(listOf("Primer paràgraf.", "Segon, amb una\nlínia nova.", "Tercer."),
            texts("Primer paràgraf.\n\nSegon, amb una\nlínia nova.\n\n\nTercer."))
    }

    @Test fun `la negreta i els enllacos es queden al seu paragraf`() {
        assertEquals(listOf("Mira això.", "Un enllaç i negreta."),
            texts("Mira **això**.\n\nUn [enllaç](https://example.org) i **negreta**."))
    }

    @Test fun `un sol paragraf queda sencer`() {
        assertEquals(listOf("Hola, com va?"), texts("Hola, com va?"))
    }
}
