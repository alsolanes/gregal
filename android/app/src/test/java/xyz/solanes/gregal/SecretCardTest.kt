package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** Formulari segur: el valor surt només per onSave, i el camp queda buit. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp")
class SecretCardTest {
    @get:Rule val ui = createComposeRule()

    @Test fun savesTheValueOnceAndClearsTheField() {
        var saved: String? = null
        val a = GregalApi.WindAsk(key = "k1", type = "secret", query = "Clau de l'API d'OpenWeather", name = "openweather_key", reason = "Per mirar el temps")
        ui.setContent { MaterialTheme { SecretCard("ca", a, Color(0xFF8193FF), onSave = { saved = it }, onDecline = {}) } }
        ui.onNodeWithText("Necessito: Clau de l'API d'OpenWeather").assertIsDisplayed()
        ui.onNodeWithTag("secret-save").assertIsNotEnabled()
        ui.onNodeWithTag("secret-input").performTextInput("sk-1234567890")
        // Amagada per defecte: el text no es veu.
        ui.onNodeWithTag("secret-input").assertTextContains("•••••••••••••", substring = true)
        ui.onNodeWithTag("secret-save").performClick()
        assertEquals("sk-1234567890", saved)
        ui.onNodeWithTag("secret-save").assertIsNotEnabled()
    }

    @Test fun declineSendsNothing() {
        var saved: String? = null
        var declined = false
        val a = GregalApi.WindAsk(key = "k2", type = "secret", query = "Contrasenya")
        ui.setContent { MaterialTheme { SecretCard("ca", a, Color(0xFF8193FF), onSave = { saved = it }, onDecline = { declined = true }) } }
        ui.onNodeWithText("Ara no").performClick()
        assertNull(saved)
        assertEquals(true, declined)
    }
}
