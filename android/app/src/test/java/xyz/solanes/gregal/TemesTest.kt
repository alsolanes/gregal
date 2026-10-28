package xyz.solanes.gregal

import androidx.compose.ui.graphics.Color
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Els temes no són decoració: si un accent no es pot llegir damunt del fons, o
 * el text queda a mitges, el tema no existeix. Aquest test ho comprova amb el
 * contrast real (WCAG) de cada paleta, clara i fosca, i que el catàleg no
 * perdi el tema de sempre.
 */
class TemesTest {

    private fun canal(v: Float): Double =
        if (v <= 0.03928f) (v / 12.92).toDouble() else Math.pow(((v + 0.055) / 1.055).toDouble(), 2.4)

    private fun llum(c: Color): Double =
        0.2126 * canal(c.red) + 0.7152 * canal(c.green) + 0.0722 * canal(c.blue)

    private fun contrast(a: Color, b: Color): Double {
        val l1 = maxOf(llum(a), llum(b))
        val l2 = minOf(llum(a), llum(b))
        return (l1 + 0.05) / (l2 + 0.05)
    }

    @Test
    fun `el tema de sempre va primer i no canvia d'id`() {
        assertEquals("gregal", GregalTemes.first().id)
        assertEquals(5, GregalTemes.size)
        assertEquals("gregal", temaPerId(null).id)
        assertEquals("gregal", temaPerId("no-existeix").id)
        assertEquals(GregalTemes.size, GregalTemes.map { it.id }.toSet().size)
    }

    @Test
    fun `cada tema te les dues cares i la marca de fosca`() {
        for (t in GregalTemes) {
            assertTrue("${t.id}: la clara s'ha de saber clara", !t.clar.dark)
            assertTrue("${t.id}: la fosca s'ha de saber fosca", t.fosc.dark)
            assertTrue("${t.id}: la paleta fosca ha de ser mes fosca", llum(t.fosc.deepTop) < llum(t.clar.deepTop))
        }
    }

    @Test
    fun `el text i l'accent es llegeixen a tots els temes`() {
        for (t in GregalTemes) {
            for (p in listOf(t.clar, t.fosc)) {
                val nom = "${t.id} (${if (p.dark) "fosc" else "clar"})"
                assertTrue("$nom: el text principal no arriba a 7:1 (${contrast(p.text, p.deepTop)})",
                    contrast(p.text, p.deepTop) >= 7.0)
                assertTrue("$nom: el text secundari no arriba a 4:1 (${contrast(p.muted, p.deepTop)})",
                    contrast(p.muted, p.deepTop) >= 4.0)
                assertTrue("$nom: l'accent no arriba a 3:1 damunt el fons (${contrast(p.sea, p.deepTop)})",
                    contrast(p.sea, p.deepTop) >= 3.0)
                assertTrue("$nom: el text damunt l'accent no es llegeix (${contrast(p.onSea, p.sea)})",
                    contrast(p.onSea, p.sea) >= 4.5)
                assertTrue("$nom: la superficie no es distingeix del fons",
                    p.surface != p.deepTop && p.surfaceHi != p.surface)
            }
        }
    }

    @Test
    fun `el mode mana sobre el tema`() {
        // Amb mode clar, qualsevol tema ha de donar la seva paleta clara.
        for (t in GregalTemes) {
            assertTrue(!paletteFor(t.id, "clar", systemDark = true).dark)
            assertTrue(paletteFor(t.id, "fosc", systemDark = false).dark)
            // I en mode sistema, el que digui el telèfon.
            assertTrue(paletteFor(t.id, "sistema", systemDark = true).dark)
            assertTrue(!paletteFor(t.id, "sistema", systemDark = false).dark)
        }
        // El tema de sempre, en mode sistema fosc, ha de ser exactament DarkPal.
        assertEquals(DarkPal, paletteFor("gregal", "sistema", systemDark = true))
        assertEquals(LightPal, paletteFor("gregal", "sistema", systemDark = false))
    }
}
