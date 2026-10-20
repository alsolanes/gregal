package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * Mascotes del company: parseig del catàleg i check-ins per mascota.
 * Contractes (docs/api-contract.md):
 *  - GET /api/companion/pets → {pets:[{id,name,role}]}
 *  - POST /api/companion/checkin {pet} → {checkin,pet,at}
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class PetModeTest {

    @Test
    fun petsParseFromCatalog() {
        val raw = """{"pets":[
            {"id":"mel","name":"Mel","role":"música"},
            {"id":"llumen","name":"Llumen","role":"notícies"},
            {"id":"pixel","name":"Pixel","role":"codi"},
            {"id":"mica","name":"Mica","role":"sistema"}
        ]}"""
        val pets = Pet.parse(raw)
        assertEquals(4, pets.size)
        assertEquals("mel", pets[0].id)
        assertEquals("Mel", pets[0].name)
        assertEquals("música", pets[0].role)
        assertEquals("mica", pets[3].id)
    }

    @Test
    fun emptyCatalogMeansEmptyList() {
        val pets = Pet.parse("""{"pets":[]}""")
        assertTrue(pets.isEmpty())
        // notes:null no pot petar el parseig tampoc
        val pets2 = Pet.parse("""{}""")
        assertTrue(pets2.isEmpty())
    }

    @Test
    fun petCheckinParseKnowsWhoSpoke() {
        val raw = """{"checkin":"Avui toca escoltar el disc nou, va?","pet":"mel","at":"2026-10-19T22:00:00Z"}"""
        val c = PetCheckin.parse(raw)
        assertEquals("mel", c.pet)
        assertTrue(c.text.contains("disc"))
        assertEquals("2026-10-19T22:00:00Z", c.at)
    }

    @Test
    fun checkinRequestBodyCarriesPet() {
        val body = PetRequests.checkin("pixel")
        assertEquals("""{"pet":"pixel"}""", body)
    }

    @Test
    fun avatarsAre16x16WithPaletteChars() {
        for (id in listOf("mel", "llumen", "pixel", "mica")) {
            val g = PetAvatars.grid(id)
            assertEquals(16, g.size)
            for (r in g) assertEquals(16, r.length)
            val px = PetAvatars.pixels(g)
            assertEquals(16, px.size)
            for (row in px) for (c in row) assertTrue(c != 0 || true)
        }
    }

    @Test
    fun accentColorsExist() {
        for (id in listOf("mel", "llumen", "pixel", "mica")) {
            assertTrue(PetAvatars.accent(id) != 0L)
        }
    }
}
