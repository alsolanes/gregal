package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/** L'inici «pregunta primer»: salutació, suggeriments i les targetes d'«Avui». */
class VentsAskHomeTest {
    private val marinada = Wind("w-1", "Marinada", "#EFC69A", "calma")
    private val ponent = Wind("w-2", "Ponent", "#E8D58A", "calma")

    @Test fun greetingFollowsTheHourAndUsesTheFirstName() {
        assertEquals("Bon dia, Aleix.", askGreeting("ca", 9, "aleix solanes"))
        assertEquals("Good evening.", askGreeting("en", 21, ""))
        assertEquals("Bona tarda, Marta.", askGreeting("ca", 15, "Marta"))
        assertEquals("Good night.", askGreeting("en", 3, "  "))
    }

    @Test fun suggestionsExistInBothLanguages() {
        assertEquals(askSuggestions("ca").size, askSuggestions("en").size)
        assertTrue(askSuggestions("en").all { it.isNotBlank() })
    }

    @Test fun todayPutsNewsFirstThenWorkThenConversations() {
        val rows = listOf(
            VentsInboxRow("wind:w-1", "Marinada", "I found it cheaper", listOf(marinada), 2_000L),
            VentsInboxRow("wind:w-2", "Ponent", "A book for you", listOf(ponent), 1_000L),
            VentsInboxRow("wind:w-3", "Idle", "", emptyList(), 0L),
        )
        val task = WindTask("t1", "Compare flights", "", "", 0, 0, "running", listOf("Marinada"), emptyList(), "", "")
        val rem = ProgramatReminder("r1", "Gregal", "2026-11-04T09:00:00", "Call the doctor")
        val conn = GregalApi.Connections(emptyList(), "", false, true, "07:30", "Gregal")
        val cards = todayCards("en", rows, setOf("w-1"), listOf(task), listOf(rem), conn, {}, {}, {}, {})
        assertEquals(listOf("wind:w-1", "task:t1", "wind:w-2", "rem:r1", "briefing"), cards.map { it.key })
        assertEquals("New message", cards[0].kicker)
        assertEquals("Every day at 07:30", cards.last().title)
        // Sense missatges no és una conversa per reprendre.
        assertTrue(cards.none { it.key == "wind:w-3" })
    }

    @Test fun briefingOffInvitesToSetItUp() {
        val conn = GregalApi.Connections(emptyList(), "", false, false, "", "")
        val cards = todayCards("ca", emptyList(), emptySet(), emptyList(), emptyList(), conn, {}, {}, {}, {})
        assertEquals("Configura’l", cards.single().title)
    }
}
