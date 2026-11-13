package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.*
import androidx.compose.foundation.verticalScroll
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** Regles en llenguatge natural: les tres llistes, editables. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp")
class RulesTest {
    @get:Rule val ui = createComposeRule()

    @Test fun parseAndJson() {
        val r = WindRules.parse(JSONObject("""{"alone":["a"," "],"ask":["Enviar correus"],"never":[]}"""))
        assertEquals(listOf("a"), r.alone)
        assertEquals(listOf("Enviar correus"), r.ask)
        assertEquals("Enviar correus", WindRules.parse(r.toJson()).ask.single())
        for (k in listOf("title", "alone", "ask", "never", "add")) assertTrue(k, rulesCopy("ca", k) != rulesCopy("en", k))
    }

    @Test fun addAnExampleAndRemoveARule() {
        var rules by mutableStateOf(WindRules(never = listOf("Esborrar res de Documents")))
        ui.setContent { MaterialTheme { androidx.compose.foundation.layout.Column(androidx.compose.ui.Modifier.verticalScroll(androidx.compose.foundation.rememberScrollState())) { RulesContent("ca", rules) { rules = it } } } }
        ui.onNodeWithText("Pregunta'm abans").assertExists()
        ui.onNodeWithText("+ Enviar qualsevol correu o missatge").performScrollTo().performClick()
        assertEquals(listOf("Enviar qualsevol correu o missatge"), rules.ask)
        ui.onNodeWithTag("rule-x-never-0").performScrollTo().performClick()
        assertTrue(rules.never.isEmpty())
        ui.onNodeWithTag("rule-add-alone").performScrollTo().performTextInput("Reservar taula")
        ui.onNodeWithTag("rule-add-alone").performImeAction()
        assertEquals(listOf("Reservar taula"), rules.alone)
    }
}
