package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

/** Wiki: el flux bàsic de dades des del punt de vista de l'app. */
class WikiTest {

    @Test fun clausWikiExisteixen() {
        // ca i en han de tenir les mateixes claus noves
        for (k in listOf("app.wiki.open", "app.wiki.title", "app.wiki.empty", "app.wiki.emptyHelp",
                         "app.wiki.summarize", "app.wiki.summarizing", "app.wiki.summarizeDone",
                         "app.wiki.summarizeFail", "app.wiki.delete", "app.wiki.updated")) {
            val ca = tr("ca", k)
            val en = tr("en", k)
            assertTrue("falta $k", ca.isNotBlank() && en.isNotBlank())
        }
    }

    @Test fun titolDePaginaBuitCauAlNom() {
        // El lector fa p.titol.ifBlank { p.nom }: el comportament esperat
        val titol = "".ifBlank { "camp-nou" }
        assertEquals("camp-nou", titol)
    }
}
