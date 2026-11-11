package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/** La guia «Què pot fer Gregal» diu el mateix en català i anglès i té exemples per provar. */
class GregalGuideTest {
    @Test fun sameTopicsInBothLanguages() {
        val ca = guideTopics("ca"); val en = guideTopics("en")
        assertEquals(ca.size, en.size)
        ca.zip(en).forEach { (c, e) ->
            assertEquals(c.icon, e.icon)
            assertEquals(c.examples.size, e.examples.size)
            assertTrue(c.title.isNotBlank() && e.title.isNotBlank() && c.how.isNotBlank() && e.how.isNotBlank())
        }
        assertTrue("prou exemples per provar", ca.sumOf { it.examples.size } >= 8)
        assertTrue(guideIntro("ca").first.contains("Gregal") && guideIntro("en").first.contains("Gregal"))
    }

    @Test fun examplesAreReadyToSend() {
        // Cap exemple porta forats per omplir: s'envien tal qual al fil de Gregal.
        for (lang in listOf("ca", "en")) for (t in guideTopics(lang)) for (ex in t.examples)
            assertTrue(ex, !ex.contains("<") && !ex.contains("…"))
    }
}
