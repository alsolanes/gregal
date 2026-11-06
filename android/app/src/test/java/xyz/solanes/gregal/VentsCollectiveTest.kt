package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class VentsCollectiveTest {
    @Test fun unApartirDeMolts() {
        assertEquals("pirata", CollectiveOptions.singularOf("pirates"))
        assertEquals("minairó", CollectiveOptions.singularOf("minairons"))
        assertEquals("follet", CollectiveOptions.singularOf("follets"))
        assertEquals("bruixa", CollectiveOptions.singularOf("bruixes"))
        assertEquals("drac", CollectiveOptions.singularOf("dracs"))
        assertEquals("fairy", CollectiveOptions.singularOf("fairies"))
        assertEquals("gent", CollectiveOptions.singularOf("gent"))
        assertEquals("pirate", CollectiveOptions.singularOf("pirates", "en"))
        assertEquals("witch", CollectiveOptions.singularOf("witches", "en"))
        assertEquals("elf", CollectiveOptions.singularOf("elves", "en"))
        assertEquals("gnome", CollectiveOptions.singularOf("gnomes", "en"))
    }

    @Test fun elPrimerEsElDeSempreILesCadenesHiSon() {
        assertEquals("Vents", CollectiveOptions.of("ca").first().plural)
        assertEquals("Winds", CollectiveOptions.of("en").first().plural)
        for (k in listOf("collective.title", "collective.help", "collective.other", "collective.otherHint", "collective.ok", "collective.laterButton", "collective.later")) {
            assertTrue(k, STR_APP_CA.containsKey(k) && STR_APP_EN.containsKey(k))
        }
        assertTrue(STR_APP_CA["collective.later"]!!.contains("Preferències"))
    }
}
