package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/** Llegendes: the same wind is the same felt character on Android and on the web (winds.js folkCharacter). */
class FolkFaceTest {
    @Test fun `each wind gets the character the web gives it`() {
        // What winds.js answers: idHash(w) % 3 over ['minairo', 'dona', 'tio'].
        val web = mapOf("gregal" to "tio", "xaloc" to "dona", "mestral" to "tio", "ponent" to "minairo",
            "marinada" to "tio", "tramuntana" to "minairo")
        for ((id, who) in web) assertEquals(id, who, folkCharacter(id))
    }

    @Test fun `the cast is the three folk characters and folk is a pack`() {
        assertEquals(listOf("minairo", "dona", "tio"), folkCast)
        assertTrue("folk" in windPacks)
    }
}
