package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

/** L'aura del color del vent només marca feina en marxa, i només a les cares grans. */
class WindAuraTest {
    private val working = setOf(
        WindMood.THINKING, WindMood.SEARCHING, WindMood.READING,
        WindMood.WRITING, WindMood.WORKING, WindMood.DISCUSSING,
    )

    @Test fun `una cara gran treballant té aura i en repòs no`() {
        for (mood in WindMood.entries) {
            assertEquals(mood.name, mood in working, windShowsAura(mood, size = 96))
            assertEquals(mood.name, mood in working, windShowsAura(mood, size = 64))
        }
    }

    @Test fun `les cares petites de les llistes no tenen aura`() {
        for (mood in WindMood.entries) {
            assertFalse(mood.name, windShowsAura(mood, size = 63))
            assertFalse(mood.name, windShowsAura(mood, size = 40))
        }
    }

    @Test fun `sense moviment l'aura queda quieta`() {
        val still = windAuraSway(0f, motion = false)
        for (t in listOf(1f, 2.5f, 7f, 11.9f)) assertEquals(still, windAuraSway(t, motion = false))
        assertEquals(0f, still.first)
    }

    @Test fun `amb moviment l'aura gira i batega`() {
        val a = windAuraSway(0f, motion = true)
        val b = windAuraSway(1.5f, motion = true)
        assertNotEquals(a.first, b.first)
        assertNotEquals(a.second, b.second)
    }
}
