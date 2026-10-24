package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Test

/** La llista de converses mostra noms llegibles, no títols interns. */
class ConvLookTest {
    @Test fun `xat d'un vent`() {
        assertEquals(ConvLook(ConvKind.WIND, "Garbí"), SessionInfo("s", 2, "x", "Company — Garbí").look())
    }

    @Test fun `rotllana sense identificador`() {
        assertEquals(ConvLook(ConvKind.GROUP, "Debat de viatge"), SessionInfo("s", 2, "x", "Rotllana — debat-de-viatge-a220a724a2c01b94").look())
        assertEquals(ConvLook(ConvKind.GROUP, "Sopar de dijous"), SessionInfo("s", 2, "x", "Rotllana — Sopar de dijous").look())
    }

    @Test fun `conversa de l'agent`() {
        assertEquals(ConvLook(ConvKind.AGENT, "Arregla el login"), SessionInfo("s", 2, "x", "Arregla el login").look())
        assertEquals(ConvLook(ConvKind.AGENT, "sessio-1"), SessionInfo("sessio-1", 2, "x").look())
    }
}
