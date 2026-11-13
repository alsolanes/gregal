package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/** Targeta de resultat: el bloc ```gregal-result``` es llegeix i, si està mal fet, surt com a codi. */
@RunWith(RobolectricTestRunner::class)
class ResultBlocksTest {
    private val json = """{"title":"Barcelona → Girona · dv. 13","subtitle":"11 trens","icon":"🚆",
        "rows":[{"lead":"08:05","text":"Avant · 40 min","value":"17,40 €","tag":"Recomanat","pick":true},
                {"lead":"08:16","text":"R11 · 1 h 20","value":"8,95 €"}],
        "sources":[{"name":"Renfe","url":"https://www.renfe.com"},{"url":"https://www.trainline.es/x"}],
        "more":{"label":"Veure els 11","url":"https://www.renfe.com"}}"""

    @Test fun parsesCard() {
        val s = parseResultSpec(json)!!
        assertEquals("Barcelona → Girona · dv. 13", s.title)
        assertEquals(2, s.rows.size)
        assertTrue(s.rows[0].pick)
        assertEquals("www.trainline.es", s.sources[1].first)
        assertEquals("Veure els 11", s.moreLabel)
    }

    @Test fun invalidIsNull() {
        assertNull(parseResultSpec("no és json"))
        assertNull(parseResultSpec("""{"title":"Sense files","rows":[]}"""))
        assertNull(parseResultSpec("""{"rows":[{"text":"x"}]}"""))
    }

    @Test fun markdownGivesResultBlock() {
        val rich = groupRich(splitMd("Aquests són els millors:\n```gregal-result\n$json\n```\nVols que en reservi un?"))
        assertTrue(rich.any { it is Rich.Result })
        val broken = groupRich(splitMd("```gregal-result\n{trencat\n```"))
        assertTrue("un bloc mal fet surt com a codi", broken.any { it is Rich.Code })
    }

    @Test fun sourcesOncePerSite() {
        val names = resultSourceNames(listOf("idealo · QCY" to "https://www.idealo.es/a", "idealo · Sony" to "https://www.idealo.es/b",
            "Amazon" to "https://www.amazon.es/x", "sense enllaç" to ""))
        assertEquals(listOf("idealo", "amazon", "sense enllaç"), names)
    }

    @Test fun footTextSaysHowManySourcesAndHowLongAgo() {
        val now = 1_800_000_000_000L
        assertEquals("1 font", resultFootText("ca", 1, 0L, now))
        assertEquals("3 fonts · fa 2 min", resultFootText("ca", 3, now - 120_000L, now))
        assertEquals("2 sources · 1 h ago", resultFootText("en", 2, now - 3_600_000L, now))
        assertEquals("", resultFootText("ca", 0, now, now))
    }
}
