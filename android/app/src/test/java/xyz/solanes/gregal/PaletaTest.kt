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
        assertEquals("#0F1B21", hex(DarkPal.deepTop))     // --bg (fons)
        assertEquals("#0B1519", hex(DarkPal.deepBottom))  // --sidebar/--panel (abisme)
        assertEquals("#16262E", hex(DarkPal.surface))     // --surface (night)
        assertEquals("#203640", hex(DarkPal.surfaceHi))   // --surface-hi
        assertEquals("#F2F5EF", hex(DarkPal.ink))         // --ink
        assertEquals("#DCE5DF", hex(DarkPal.text))        // --text
        assertEquals("#A3B5AF", hex(DarkPal.muted))       // --muted (slate)
        assertEquals("#8FA29C", hex(DarkPal.faint))       // --faint
        assertEquals("#D4F25A", hex(DarkPal.sea))         // --accent (escuma)
        assertEquals("#E4F98F", hex(DarkPal.seaClara))    // --accent-hi
        assertEquals("#F2B48E", hex(DarkPal.sand))        // --amber (arena)
        assertEquals("#7FD6C2", hex(DarkPal.onada))       // --onada
        assertEquals("#86E3A6", hex(DarkPal.green))       // --green
        assertEquals("#FF8F86", hex(DarkPal.red))         // --red
        assertEquals("#C9B2FF", hex(DarkPal.lila))        // --lila
        assertEquals("#2A2645", hex(DarkPal.lilaFons))    // --lila-fons
        assertEquals("#24383F", hex(DarkPal.algua))       // docs: línies/algua
        assertEquals("#0F1B21", hex(DarkPal.onSea))       // text sobre l'accent: fons
    }

    @Test
    fun clarTeElsHexosDeLaWeb() {
        assertEquals("#EEF1EC", hex(LightPal.deepTop))    // --bg
        assertEquals("#E4E9E3", hex(LightPal.deepBottom)) // --sidebar/--panel
        assertEquals("#FFFFFF", hex(LightPal.surface))    // --surface
        assertEquals("#DEE5DE", hex(LightPal.surfaceHi))  // --surface-hi
        assertEquals("#10202A", hex(LightPal.ink))        // --ink
        assertEquals("#1C2D35", hex(LightPal.text))       // --text
        assertEquals("#4C5F5C", hex(LightPal.muted))      // --muted
        assertEquals("#52645E", hex(LightPal.faint))      // --faint
        assertEquals("#17323D", hex(LightPal.sea))        // --accent
        assertEquals("#0D232C", hex(LightPal.seaClara))   // --accent-hi
        assertEquals("#9A4E1E", hex(LightPal.sand))       // --amber
        assertEquals("#0A6E62", hex(LightPal.onada))      // --onada
        assertEquals("#2F6B1F", hex(LightPal.green))      // --green
        assertEquals("#C2332E", hex(LightPal.red))        // --red
        assertEquals("#7444D6", hex(LightPal.lila))       // --lila
        assertEquals("#EEE8FD", hex(LightPal.lilaFons))   // --lila-fons
        assertEquals("#D2DBD3", hex(LightPal.algua))      // docs: alguaClar
        assertEquals("#FFFFFF", hex(LightPal.onSea))      // text sobre l'accent: blanc
    }

    @Test
    fun liniesSonGrisFredINoAccent() {
        // --line: rgba(207,227,218,.10) / .20 al fosc (sàlvia pàl·lida)
        assertEquals("#CFE3DA", hex(DarkPal.line))
        assertEquals(0x1A, alfa(DarkPal.line))
        assertEquals(0x33, alfa(DarkPal.lineHi))
        // --line: rgba(16,32,42,.14) / .28 al clar (la tinta de mar)
        assertEquals("#10202A", hex(LightPal.line))
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
