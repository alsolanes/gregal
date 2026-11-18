package xyz.solanes.gregal

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.compositeOver
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * L'estil Vidre: les superfícies es tornen translúcides damunt d'un fons amb
 * taques de llum, però el text s'ha de continuar llegint al pitjor punt (el
 * centre d'una taca) de qualsevol tema i mode.
 */
class GlassTest {

    private fun canal(v: Float): Double =
        if (v <= 0.03928f) (v / 12.92).toDouble() else Math.pow(((v + 0.055) / 1.055).toDouble(), 2.4)

    private fun llum(c: Color): Double =
        0.2126 * canal(c.red) + 0.7152 * canal(c.green) + 0.0722 * canal(c.blue)

    private fun contrast(a: Color, b: Color): Double {
        val l1 = maxOf(llum(a), llum(b))
        val l2 = minOf(llum(a), llum(b))
        return (l1 + 0.05) / (l2 + 0.05)
    }

    private val totes = GregalTemes.flatMap { listOf(it.clar, it.fosc) }

    @Test
    fun `el vidre fa translucides les superficies i deixa el text i l'accent`() {
        for (p in totes) {
            val g = p.glassy()
            assertTrue(g.glass)
            assertTrue("superficie opaca", g.surface.alpha < 1f && g.surfaceHi.alpha < 1f)
            assertEquals(p.ink, g.ink)
            assertEquals(p.muted, g.muted)
            assertEquals(p.sea, g.sea)
            assertEquals(p.deepTop, g.deepTop)
            assertTrue(!p.glass)
        }
    }

    @Test
    fun `el text es llegeix damunt del vidre al centre de cada taca`() {
        for (t in GregalTemes) for (p in listOf(t.clar, t.fosc)) {
            val g = p.glassy()
            val nom = "${t.id} (${if (p.dark) "fosc" else "clar"})"
            val fons = listOf(g.deepTop) + auroraBlobs(g).map { it.color.compositeOver(g.deepTop) }
            for (f in fons) {
                val sup = g.surface.compositeOver(f)
                assertTrue("$nom: text ${contrast(g.ink, sup)}", contrast(g.ink, sup) >= 7.0)
                assertTrue("$nom: muted ${contrast(g.muted, sup)}", contrast(g.muted, sup) >= 4.0)
                assertTrue("$nom: text sense superfície ${contrast(g.text, f)}", contrast(g.text, f) >= 7.0)
            }
        }
    }

    @Test
    fun `sense vidre no hi ha taques i amb vidre surten de la paleta`() {
        for (p in totes) {
            assertTrue(auroraBlobs(p).isEmpty())
            val colors = auroraBlobs(p.glassy()).map { it.color.copy(alpha = 1f) }
            assertEquals(listOf(p.sea, p.onada, p.lila), colors)
        }
    }

    @Test
    fun `els components de Material deixen veure el fons`() {
        for (p in totes) {
            assertEquals(Color.Transparent, p.glassy().scheme().background)
            assertEquals(p.deepTop, p.scheme().background)
        }
    }
}
