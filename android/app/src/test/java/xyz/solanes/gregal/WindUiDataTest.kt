package xyz.solanes.gregal

import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class WindUiDataTest {
    private val xaloc = Wind("xaloc", "Xaloc", "#EFA68F", "música", persona = "Curiós i juganer")

    @Test fun speakerMetadataKeepsOrdinaryColonsAndUrls() {
        val message = GregalViewModel.PetMsg("pet", "https://example.org: una descoberta", "xaloc", "Xaloc")
        val resolved = resolveWindMessage(message, listOf(xaloc))
        assertEquals(xaloc, resolved.wind)
        assertEquals(message.text, resolved.message.text)
        assertEquals("Xaloc", resolved.name)
    }
    @Test fun onlyKnownLegacyPrefixIsRemoved() {
        assertEquals("Hola: bon dia", resolveWindMessage(GregalViewModel.PetMsg("pet", "Xaloc: Hola: bon dia"), listOf(xaloc)).message.text)
        assertEquals("Nota: bon dia", resolveWindMessage(GregalViewModel.PetMsg("pet", "Nota: bon dia"), listOf(xaloc)).message.text)
    }
    @Test fun userMessageNeverGetsMisidentifiedAsWind() {
        val message = GregalViewModel.PetMsg("user", "Xaloc: em sents?")
        val result = resolveWindMessage(message, listOf(xaloc))
        assertNull(result.wind)
        assertEquals(message, result.message)
    }
    @Test fun namedSingletonIsAChatAndGroupsNeedTwoDistinctMembers() {
        assertTrue(canCreateConversation("Racó", listOf("x"), false))
        assertFalse(canCreateConversation("Racó", listOf("x"), true))
        assertFalse(canCreateConversation("Racó", listOf("x", "x"), true))
        assertTrue(canCreateConversation("Racó", listOf("x", "y"), true))
        assertFalse(canCreateConversation(" ", listOf("x"), false))
        assertFalse(canCreateConversation("Racó", emptyList(), false))
    }
    @Test fun editingPreservesIdAndPersona() {
        val body = JSONObject(WindRequests.save("Nom nou", xaloc.color, xaloc.role, xaloc.style, xaloc.persona, xaloc.id))
        assertEquals("xaloc", body.getString("id"))
        assertEquals(xaloc.persona, body.getString("persona"))
        val parsed = Wind.parse("""{"winds":[{"id":"xaloc","name":"Xaloc","color":"#EFA68F","role":"música","persona":"Curiós i juganer"}]}""").single()
        assertEquals(xaloc.persona, parsed.persona)
        assertFalse(JSONObject(WindRequests.save("Nou", "#EFA68F", "música")).has("id"))
    }
    @Test fun everyPresetHasSupportedStyleAndBothLanguages() {
        assertEquals(8, WindSuggestions.all.size)
        WindSuggestions.all.forEach { preset ->
            assertEquals(preset.style, windStyleOf(preset.style).encode())
            listOf("role", "persona").forEach { suffix ->
                val key = "app.vents.preset.${preset.name}.$suffix"
                assertTrue(STR_APP_CA.containsKey(key))
                assertTrue(STR_APP_EN.containsKey(key))
            }
        }
        assertEquals(STR_APP_CA.keys, STR_APP_EN.keys)
    }
    @Test fun malformedColorDoesNotCrashUi() {
        assertEquals(0xFFFF9F00L, Wind("x", "X", "#nope!!", "").argb)
    }
}
