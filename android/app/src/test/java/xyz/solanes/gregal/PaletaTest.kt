package xyz.solanes.gregal

import androidx.compose.ui.graphics.Color
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Ancoratge de la paleta: els hex han de ser els mateixos que `:root` de la
 * web (`internal/web/index.html`) i `internal/tui/theme.go`. Aquest fitxer és
 * el que hauria d'haver impedit que l'Android es quedés amb la paleta vella
 * (#101418/#0C2229) mentre la web i el TUI ja anaven amb la identitat.
 *
 * Si un d'aquests tests peta, el que està malament és el Kotlin: la web mana.
 */
class PaletaTest {

    // Compose desa el color en un ULong: els 32 bits alts són l'ARGB.
    private fun argb(c: Color): Long = c.value.toLong() shr 32
    private fun hex(c: Color): String = String.format("#%06X", argb(c) and 0xFFFFFF)
    private fun alfa(c: Color): Int = ((argb(c) shr 24) and 0xFF).toInt()

    @Test
    fun foscTeElsHexosDeLaWeb() {
        assertEquals("#0F172A", hex(DarkPal.deepTop))     // --bg
        assertEquals("#0B1220", hex(DarkPal.deepBottom))  // --sidebar/--panel
        assertEquals("#111C2F", hex(DarkPal.surface))     // --surface
        assertEquals("#1A2740", hex(DarkPal.surfaceHi))   // --surface-hi
        assertEquals("#F1F5F9", hex(DarkPal.ink))         // --ink
        assertEquals("#E2E8F0", hex(DarkPal.text))        // --text
        assertEquals("#94A3B8", hex(DarkPal.muted))       // --muted
        assertEquals("#475569", hex(DarkPal.faint))       // --faint
        assertEquals("#7FD4C1", hex(DarkPal.sea))         // --accent (escuma)
        assertEquals("#A3E4D6", hex(DarkPal.seaClara))    // --accent-hi
        assertEquals("#F2E7C9", hex(DarkPal.sand))        // --amber (arena)
        assertEquals("#6CB4C7", hex(DarkPal.onada))       // --onada
        assertEquals("#4ADE80", hex(DarkPal.green))       // --green
        assertEquals("#F97066", hex(DarkPal.red))         // --red
        assertEquals("#C4B5FD", hex(DarkPal.lila))        // --lila
        assertEquals("#2A2440", hex(DarkPal.lilaFons))    // --lila-fons
        assertEquals("#1E3A44", hex(DarkPal.algua))       // docs: línies/algua
    }

    @Test
    fun clarTeElsHexosDeLaWeb() {
        assertEquals("#F5F7F8", hex(LightPal.deepTop))    // --bg
        assertEquals("#EBEFF1", hex(LightPal.deepBottom)) // --sidebar/--panel
        assertEquals("#FFFFFF", hex(LightPal.surface))    // --surface
        assertEquals("#E2E9EC", hex(LightPal.surfaceHi))  // --surface-hi
        assertEquals("#0B1220", hex(LightPal.ink))        // --ink
        assertEquals("#1E293B", hex(LightPal.text))       // --text
        assertEquals("#52616B", hex(LightPal.muted))      // --muted
        assertEquals("#7C8B95", hex(LightPal.faint))      // --faint
        assertEquals("#0E8C74", hex(LightPal.sea))        // --accent
        assertEquals("#0A6F5C", hex(LightPal.seaClara))   // --accent-hi
        assertEquals("#9A6B12", hex(LightPal.sand))       // --amber
        assertEquals("#0E7490", hex(LightPal.onada))      // --onada
        assertEquals("#15803D", hex(LightPal.green))      // --green
        assertEquals("#DC2626", hex(LightPal.red))        // --red
        assertEquals("#6D28D9", hex(LightPal.lila))       // --lila
        assertEquals("#EDE9FE", hex(LightPal.lilaFons))   // --lila-fons
    }

    @Test
    fun liniesSonEscumaAlDeuIPerCent() {
        // --line: rgba(127,212,193,.10) / .20 al fosc
        assertEquals(0x1A, alfa(DarkPal.line))
        assertEquals(0x33, alfa(DarkPal.lineHi))
        // --line: rgba(14,140,116,.14) / .28 al clar
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
