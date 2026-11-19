package xyz.solanes.gregal

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.compositeOver
import androidx.compose.ui.graphics.lerp
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

    /** El fons de SeaBackdrop a l'alçada `y` (0..1): el degradat cap a
     *  l'abisme des del 55 % i, a dalt, el vel d'escuma (pitjor cas: 7 %). */
    private fun fonsA(p: GregalPalette, y: Float): Color {
        val base = if (y <= 0.55f) p.deepTop else lerp(p.deepTop, p.deepBottom, (y - 0.55f) / 0.45f)
        return if (y < 0.25f) p.sea.copy(alpha = 0.07f).compositeOver(base) else base
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
            val fons = listOf(g.deepTop, g.deepBottom) + auroraBlobs(g).map { it.color.compositeOver(fonsA(g, it.y)) }
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
    fun `els diàlegs de Material queden opacs amb vidre`() {
        for (p in totes) {
            val s = p.glassy().scheme()
            assertEquals(p.deepTop, s.background)
            assertEquals(1f, s.surface.alpha)
            assertEquals(1f, s.surfaceContainerHigh.alpha)
        }
    }

    @Test
    fun `el calaix, els fulls i els diàlegs tenen superfície opaca amb vidre`() {
        for (p in totes) {
            assertEquals(p.surface, p.solidSurface)
            val g = p.glassy()
            assertEquals(1f, g.solidSurface.alpha)
            assertEquals(g.surface.compositeOver(g.deepTop), g.solidSurface)
        }
        // Cap full, diàleg o calaix amb la superfície translúcida: el text de
        // sota s'hi llegiria a través (vist a l'emulador amb el calaix).
        val src = java.io.File("src/main/java/xyz/solanes/gregal")
        val overlay = Regex("""(ModalBottomSheet|AlertDialog|ModalDrawerSheet)\(.*[cC]ontainerColor = (Superficie|PillDark|BubbleDark)\b""")
        val dolents = src.listFiles()!!.filter { it.extension == "kt" }.flatMap { f ->
            f.readLines().mapIndexedNotNull { i, l -> if (overlay.containsMatchIn(l)) "${f.name}:${i + 1}" else null }
        }
        assertTrue("translúcids: $dolents", dolents.isEmpty())
    }

    @Test
    fun `els diàlegs propis fan servir la paleta opaca`() {
        for (p in totes) {
            val d = p.glassy().solid()
            assertEquals(1f, d.surface.alpha)
            assertEquals(1f, d.surfaceHi.alpha)
            assertEquals(p.ink, d.ink)
            assertEquals(p, p.solid())
        }
        // Vist a l'emulador: «Què pot fer Gregal» deixava llegir el xat a través.
        // Tots els diàlegs passen per GregalDialog, que els dona la paleta opaca.
        val src = java.io.File("src/main/java/xyz/solanes/gregal")
        val cru = Regex("""(?<![A-Za-z])(androidx\.compose\.ui\.window\.)?Dialog\(""")
        val dolents = src.listFiles()!!.filter { it.extension == "kt" && it.name != "Theme.kt" }.flatMap { f ->
            f.readLines().mapIndexedNotNull { i, l -> if (cru.containsMatchIn(l)) "${f.name}:${i + 1}" else null }
        }
        assertTrue("diàlegs sense GregalDialog: $dolents", dolents.isEmpty())
    }
}
