package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** Conversa per veu: la franja diu què fa i «Atura» l'acaba. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp")
class VoiceConversationTest {
    @get:Rule val ui = createComposeRule()

    @Test fun stripShowsThePhaseAndStops() {
        val conv = VoiceConversation()
        var listened = 0
        conv.listen = { listened++; conv.phase = "listening" }
        conv.toggle()
        assertTrue(conv.on)
        assertEquals(1, listened)
        ui.setContent { MaterialTheme { VoiceConversationStrip(conv, "ca", Color(0xFF8193FF)) } }
        ui.onNodeWithText("🎙️ T'escolto…").assertIsDisplayed()
        conv.phase = "speaking"
        ui.onNodeWithText("🎙️ Et responc…").assertIsDisplayed()
        ui.onNodeWithText("Atura").performClick()
        assertFalse(conv.on)
        ui.onNodeWithTag("voice-strip").assertDoesNotExist()
    }

    @Test fun copy() {
        for (k in listOf("talk", "listening", "thinking", "speaking", "stop")) assertTrue(k, voiceTalkCopy("ca", k) != voiceTalkCopy("en", k))
    }
}
