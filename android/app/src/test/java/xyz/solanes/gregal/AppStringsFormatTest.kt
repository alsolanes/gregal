package xyz.solanes.gregal

import org.junit.Test
import java.util.Locale

/**
 * Els strings amb arguments passen per String.format: cap patró de format
 * pot ser il·legal (un "%" solt petava l'app en obrir Programats).
 * Aquest test valida tots els strings de tots dos idiomes amb els arguments
 * que realment hi passen les pantalles.
 */
class AppStringsFormatTest {
    // (clau, nombre d'arguments que li passen les pantalles)
    private val usos = listOf(
        "app.programats.next" to 1,
        "app.programats.statLine" to 4,
    )

    @Test fun capStringAmbPercentIlLegal() {
        val errors = mutableListOf<String>()
        for ((key, argc) in usos) {
            for (lang in listOf("ca", "en")) {
                val raw = tr(lang, key, *Array(argc) { "0" })
                // tr ja fa el format; si peta, arriba aquí com a excepció.
                if (raw.isBlank()) errors.add("$lang:$key buit")
            }
        }
        if (errors.isNotEmpty()) throw AssertionError(errors.joinToString())
    }

    @Test fun elPercentatgeEsLiteral() {
        // "parla el 33%" amb %% escapat surt amb UN sol %.
        val ca = tr("ca", "app.programats.statLine", "3", "1", "2", "33")
        org.junit.Assert.assertTrue("cal acabat amb % però és: $ca", ca.endsWith("%"))
        org.junit.Assert.assertTrue("no hi ha d'haver %% doblat: $ca", !ca.contains("%%"))
    }
}
