package xyz.solanes.gregal

import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Totes les paletes de [GregalTemes], en clar i en fosc, han de passar AA
 * (4,5:1) amb les dues bombolles del fil: el text de l'assistent i l'hora sobre
 * `surfaceHi`, el teu text sobre l'accent, i el text de les targetes de
 * resultat i dels separadors sobre la superfície.
 */
class BubbleContrastTest {
    private val aa = 4.5

    private fun each(check: (String, GregalPalette) -> Unit) {
        for (t in GregalTemes) {
            check(t.id + "/clar", t.clar)
            check(t.id + "/fosc", t.fosc)
        }
    }

    private fun assertAA(what: String, ratio: Double) =
        assertTrue("$what: contrast %.2f < 4.5".format(ratio), ratio >= aa)

    @Test fun `text de l'assistent sobre la seva bombolla`() = each { id, p ->
        assertAA("$id text/surfaceHi", contrastWith(p.text, p.surfaceHi))
        assertAA("$id ink/surfaceHi", contrastWith(p.ink, p.surfaceHi))
    }

    @Test fun `hora i rebut (12 sp) sobre la bombolla i sobre el fons`() = each { id, p ->
        assertAA("$id muted/surfaceHi", contrastWith(p.muted, p.surfaceHi))
        assertAA("$id muted/surface", contrastWith(p.muted, p.surface))
    }

    @Test fun `el teu text sobre la teva bombolla`() = each { id, p ->
        assertAA("$id onSea/sea", contrastWith(p.onSea, p.sea))
    }

    @Test fun `el codi en linia dins la bombolla es llegeix`() = each { id, p ->
        assertAA("$id text/surface", contrastWith(p.text, p.surface))
    }
}
