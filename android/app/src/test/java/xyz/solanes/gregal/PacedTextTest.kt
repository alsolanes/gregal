package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

class PacedTextTest {
    @Test fun cutNeverLeavesHalfAWord() {
        val t = "Bon dia, com estàs avui?"
        // Dins de «estàs»: recula fins al principi de la paraula.
        assertEquals("Bon dia, com ", t.take(wordSafeCut(t, 15)))
        // Just després d'una paraula sencera (ve un espai): es queda.
        assertEquals("Bon dia, com estàs", t.take(wordSafeCut(t, 18)))
        assertEquals(t.length, wordSafeCut(t + " ", t.length))
    }

    @Test fun chunkEndingMidWordWaitsForTheRest() {
        // El tros del servidor acaba a mitja paraula: no s'ensenya fins que arribi la resta.
        assertEquals("Bon dia, com ", "Bon dia, com est".take(wordSafeCut("Bon dia, com est", 16)))
        assertEquals(0, wordSafeCut("Bon", 3))
    }

    @Test fun longTokensStillStream() {
        val url = "Mira https://exemple.cat/una/ruta/molt/llarga/de/veritat"
        assertEquals(40, wordSafeCut(url, 40))
    }

    @Test fun cutGrowsMonotonicallyAsTextArrives() {
        val full = "El prefix del prompt es manté igual d'un torn a l'altre, així el proveïdor el reaprofita."
        // Amb el text que creix i el comptador que avança, el tall mai no recula.
        var shown = 0
        for (len in 1..full.length) {
            val c = wordSafeCut(full.take(len), len)
            assertTrue("no recula a len=$len", c >= shown)
            shown = c
        }
    }
}
