package xyz.solanes.gregal

import androidx.compose.ui.graphics.Color
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Ancoratge de la paleta: els hex han de ser els mateixos que
 * `internal/tema/tema.go` (`Fosc()`/`Clar()`), d'on surten la web i el TUI, i
 * que `docs/identitat.md` ("Paleta" i "Paleta clara").
 *
 * Si un d'aquests tests peta, el que està malament és el Kotlin: tema.go mana.
 */
class PaletaTest {

    // Compose desa el color en un ULong: els 32 bits alts són l'ARGB.
    private fun argb(c: Color): Long = c.value.toLong() shr 32
    private fun hex(c: Color): String = String.format("#%06X", argb(c) and 0xFFFFFF)
    private fun alfa(c: Color): Int = ((argb(c) shr 24) and 0xFF).toInt()

    @Test
    fun foscTeElsHexosDeLaWeb() {
        assertEquals("#13151B", hex(DarkPal.deepTop))     // --bg (fons)
        assertEquals("#0E1015", hex(DarkPal.deepBottom))  // --sidebar/--panel (abisme)
        assertEquals("#1A1D25", hex(DarkPal.surface))     // --surface (night)
        assertEquals("#242833", hex(DarkPal.surfaceHi))   // --surface-hi
        assertEquals("#F3F5FA", hex(DarkPal.ink))         // --ink
        assertEquals("#E4E7EF", hex(DarkPal.text))        // --text
        assertEquals("#A3AABA", hex(DarkPal.muted))       // --muted (slate)
        assertEquals("#8890A2", hex(DarkPal.faint))       // --faint
        assertEquals("#8193FF", hex(DarkPal.sea))         // --accent (escuma)
        assertEquals("#A3B0FF", hex(DarkPal.seaClara))    // --accent-hi
        assertEquals("#F5C451", hex(DarkPal.sand))        // --amber (arena)
        assertEquals("#5CE1D6", hex(DarkPal.onada))       // --onada
        assertEquals("#6EDB95", hex(DarkPal.green))       // --green
        assertEquals("#FF8A85", hex(DarkPal.red))         // --red
        assertEquals("#C4A8FF", hex(DarkPal.lila))        // --lila
        assertEquals("#2A2245", hex(DarkPal.lilaFons))    // --lila-fons
        assertEquals("#2C3240", hex(DarkPal.algua))       // docs: línies/algua
        assertEquals("#13151B", hex(DarkPal.onSea))       // text sobre l'accent: fons
    }

    @Test
    fun clarTeElsHexosDeLaWeb() {
        assertEquals("#F6F7FA", hex(LightPal.deepTop))    // --bg
        assertEquals("#EEF0F5", hex(LightPal.deepBottom)) // --sidebar/--panel
        assertEquals("#FFFFFF", hex(LightPal.surface))    // --surface
        assertEquals("#E5E8F0", hex(LightPal.surfaceHi))  // --surface-hi
        assertEquals("#0E1220", hex(LightPal.ink))        // --ink
        assertEquals("#1F2433", hex(LightPal.text))       // --text
        assertEquals("#545D71", hex(LightPal.muted))      // --muted
        assertEquals("#5F6879", hex(LightPal.faint))      // --faint
        assertEquals("#4F5BE8", hex(LightPal.sea))        // --accent
        assertEquals("#3D47C7", hex(LightPal.seaClara))   // --accent-hi
        assertEquals("#9A5B00", hex(LightPal.sand))       // --amber
        assertEquals("#0A776F", hex(LightPal.onada))      // --onada
        assertEquals("#187A41", hex(LightPal.green))      // --green
        assertEquals("#C92A35", hex(LightPal.red))        // --red
        assertEquals("#7444D6", hex(LightPal.lila))       // --lila
        assertEquals("#EEE8FD", hex(LightPal.lilaFons))   // --lila-fons
        assertEquals("#D5DAE4", hex(LightPal.algua))      // docs: alguaClar
        assertEquals("#FFFFFF", hex(LightPal.onSea))      // text sobre l'accent: blanc
    }

    @Test
    fun liniesSonGrisFredINoAccent() {
        // --line: rgba(201,209,230,.10) / .20 al fosc
        assertEquals("#C9D1E6", hex(DarkPal.line))
        assertEquals(0x1A, alfa(DarkPal.line))
        assertEquals(0x33, alfa(DarkPal.lineHi))
        // --line: rgba(30,42,74,.14) / .28 al clar
        assertEquals("#1E2A4A", hex(LightPal.line))
        assertEquals(0x24, alfa(LightPal.line))
        assertEquals(0x47, alfa(LightPal.lineHi))
    }

    @Test
    fun elsAliasHistoricosNoDivergueixen() {
        // `BubbleDark` era un verd fosc inventat; ara és la mateixa superfície
        // que la bombolla de la web. Si algú el tornés a separar, es veuria.
        assertEquals(DarkPal.surface, DarkPal.bubbleDark)
        assertEquals(DarkPal.surface, DarkPal.pillDark)
        assertEquals(DarkPal.surfaceHi, DarkPal.codeDark)
    }

    @Test
    fun triaDePaletaSegonsTema() {
        assertEquals(DarkPal, paletteFor("fosc", systemDark = false))
        assertEquals(LightPal, paletteFor("clar", systemDark = true))
        assertEquals(DarkPal, paletteFor("sistema", systemDark = true))
        assertEquals(LightPal, paletteFor("sistema", systemDark = false))
    }
}
