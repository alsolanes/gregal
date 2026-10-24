package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

class VentsInboxTest {
    private val winds = listOf(
        Wind("a", "Marinada", "#EFC69A", "calma"),
        Wind("b", "Xaloc", "#EFA68F", "música"),
    )

    @Test fun combinesGroupsAndDirectChatsByRealRecencyWithoutInventingTimes() {
        val chats = mapOf("a" to listOf(GregalViewModel.PetMsg("pet", "Una idea", ts = 1000)))
        val groups = listOf(Rotllana("g", "Escapades", listOf("a", "b")))
        val groupChats = mapOf("g" to listOf(GregalViewModel.PetMsg("user", "Anem al bosc", ts = 2000)))
        val rows = ventsInbox(winds, groups, chats, groupChats)
        assertEquals(listOf("rot:g", "wind:a", "wind:b"), rows.map { it.key })
        assertEquals(0L, rows.last().timestamp)
        assertEquals("música", rows.last().preview)
    }

    @Test fun searchMatchesAccentedNamesRolesAndEarlierMessages() {
        val chats = mapOf("a" to listOf(
            GregalViewModel.PetMsg("pet", "Provem calçots a Valls"),
            GregalViewModel.PetMsg("user", "Perfecte"),
        ))
        assertEquals(listOf("wind:b"), ventsInbox(winds, emptyList(), chats, emptyMap(), "musica").map { it.key })
        val found = ventsInbox(winds, emptyList(), chats, emptyMap(), "CALÇOTS")
        assertEquals(listOf("wind:a"), found.map { it.key })
        assertEquals("Provem calçots a Valls", found.single().preview)
        assertTrue(ventsInbox(winds, emptyList(), chats, emptyMap(), "inexistent").isEmpty())
    }

    @Test fun groupSearchMatchesMembersWithoutDuplicatingConversations() {
        val groups = listOf(Rotllana("g", "Escapades", listOf("a", "b")))
        val rows = ventsInbox(winds, groups, emptyMap(), emptyMap(), "Xaloc")
        assertEquals(setOf("wind:b", "rot:g"), rows.map { it.key }.toSet())
        assertEquals(rows.size, rows.map { it.key }.distinct().size)
    }
}
