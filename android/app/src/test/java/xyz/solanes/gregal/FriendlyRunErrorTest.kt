package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * friendlyRunError: errors de connexió del proveïdor traduïts a humà.
 * La mateixa taula existeix al backend Go (jobs_errors.go).
 */
class FriendlyRunErrorTest {

    @Test
    fun dnsRealDelMobil() {
        val cru = """provider https://opencode.ai/zen/go/v1: Post "https://opencode.ai/zen/go/v1/chat/completions": dial tcp: lookup opencode.ai: no such host (host not found: check the provider base_url)"""
        assertEquals(
            "No s'ha pogut arribar al proveïdor: el dispositiu no resol l'adreça opencode.ai. Comprova que hi ha internet i torna-ho a provar.",
            friendlyRunError(cru),
        )
    }

    @Test
    fun timeout() {
        assertEquals(
            "El proveïdor ha trigat massa a respondre. Torna-ho a provar: si continua així, pot estar saturat.",
            friendlyRunError("context deadline exceeded (Client.Timeout exceeded while awaiting headers)"),
        )
    }

    @Test
    fun clauRebutjada() {
        assertEquals(
            "El proveïdor ha rebutjat la clau (401). Revisa la clau API a la configuració.",
            friendlyRunError("provider http://x: 401 Unauthorized"),
        )
    }

    @Test
    fun limitDePeticions() {
        assertEquals(
            "El proveïdor ha limitat les peticions (429). Espera una mica i torna-ho a provar.",
            friendlyRunError("provider http://x: 429 Too Many Requests"),
        )
    }

    @Test
    fun desconegutEsQuedaTalQual() {
        assertEquals("el model ha dit que no", friendlyRunError("el model ha dit que no"))
    }

    @Test
    fun buit() {
        assertEquals("", friendlyRunError("   "))
    }
}
