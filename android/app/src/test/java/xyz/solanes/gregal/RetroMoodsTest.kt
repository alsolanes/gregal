package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * Moods del #retro: 6 tons reals (no decoració). Contracte:
 *  - El canal porta `persona`; POST /api/retro/rewrite la canvia I
 *    regenera els missatges d'agent (mateixos IDs i ordre).
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class RetroMoodsTest {

    @Test
    fun moodsAreSixDistinctIds() {
        assertEquals(6, RETRO_MOODS.size)
        assertEquals(6, RETRO_MOODS.map { it.id }.toSet().size)
        assertEquals(
            listOf("neutral", "sarcastic", "playful", "sharp", "flirty", "serious"),
            RETRO_MOODS.map { it.id },
        )
    }

    @Test
    fun channelParsesPersona() {
        val ch = parseRetroChannelResponse(
            """{"channel":{"key":"run-5","title":"t","persona":"sharp","degraded":false,"agents":[],"messages":[]}}"""
        )
        assertEquals("sharp", ch?.persona)
    }

    @Test
    fun channelWithoutPersonaDefaultsNeutral() {
        val ch = parseRetroChannelResponse(
            """{"channel":{"key":"run-5","title":"t","degraded":false,"agents":[],"messages":[]}}"""
        )
        assertEquals("neutral", ch?.persona)
    }

    @Test
    fun rewriteRequestCarriesKeyAndPersona() {
        val body = RetroRequests.rewrite("run-5", "flirty")
        assertTrue(body.contains("\"key\""))
        assertTrue(body.contains("run-5"))
        assertTrue(body.contains("\"persona\""))
        assertTrue(body.contains("flirty"))
    }
}
