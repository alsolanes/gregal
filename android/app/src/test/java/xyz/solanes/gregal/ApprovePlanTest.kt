package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Test

/** La targeta de permís pregunta en pla què farà, on i si es pot desfer, amb el verb al botó. */
class ApprovePlanTest {
    @Test fun deleteNamesTheFileTheFolderAndThatItCannotBeUndone() {
        val p = approvePlan("ca", "delete", "rm -f \"C:/Users/alsol/AppData/Local/Temp/gsbx/fora/esborrar.txt\"")
        assertEquals("Vols que esborri esborrar.txt?", p.question)
        assertEquals("És a gsbx/fora, fora del projecte. No es podrà desfer.", p.sub)
        assertEquals("Esborra'l", p.yes)
    }

    @Test fun writeInsideTheWindProjectsSaysProjects() {
        val p = approvePlan("ca", "", "write C:\\Users\\x\\gregal\\projectes\\zefir\\js\\game.js")
        assertEquals("Vols que escrigui game.js?", p.question)
        assertEquals("És a Projectes › zefir/js.", p.sub)
        assertEquals("Escriu-lo", p.yes)
    }

    @Test fun englishAndOtherKinds() {
        assertEquals("Shall I install software?", approvePlan("en", "install", "winget install foo").question)
        assertEquals("Publica-la", approvePlan("ca", "publish_public", "publish public x").yes)
        val other = approvePlan("ca", "other", "bash something")
        assertEquals("Et demana permís", other.question)
        assertEquals("Fes-ho", other.yes)
    }
}
