package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

class GroupInboxTest {
    private val winds = listOf(
        Wind("a", "Marinada", "#EFC69A", "calma"),
        Wind("b", "Xaloc", "#EFA68F", "música"),
        Wind("c", "Ponent", "#C9B8FF", "llibres"),
    )
    private val chats = mapOf(
        "a" to listOf(GregalViewModel.PetMsg("pet", "Coses", ts = 3000)),
        "b" to listOf(GregalViewModel.PetMsg("pet", "Música", ts = 2000)),
        "c" to emptyList(),
    )

    private fun rows() = ventsInbox(winds, emptyList(), chats, emptyMap())

    @Test fun noMetaYieldsSingleGroupWithEverything() {
        val meta = GregalApi.WindsMeta(emptyList(), emptyList())
        val groups = groupInbox(rows(), meta)
        assertEquals(1, groups.size)
        assertEquals(listOf("wind:a", "wind:b", "wind:c"), groups[0].rows.map { it.key })
        assertFalse(groups[0].isFavorites)
    }

    @Test fun favoritesComeFirstAndRestFollows() {
        val meta = GregalApi.WindsMeta(listOf("wind:b"), emptyList())
        val groups = groupInbox(rows(), meta)
        assertEquals(2, groups.size)
        assertTrue(groups[0].isFavorites)
        assertEquals(listOf("wind:b"), groups[0].rows.map { it.key })
        assertEquals(listOf("wind:a", "wind:c"), groups[1].rows.map { it.key })
    }

    @Test fun sectionsSitBetweenFavoritesAndTheRestWithoutDuplicates() {
        val sec = GregalApi.WindSection("mus", "Música i llibres", listOf("wind:b", "wind:c", "wind:zz"))
        val meta = GregalApi.WindsMeta(listOf("wind:b"), listOf(sec))
        val groups = groupInbox(rows(), meta)
        // Favorits → secció → resta; la fila favorita no es repeteix a la resta,
        // i les claus que no existeixen (wind:zz) s'ignoren.
        assertEquals(3, groups.size)
        assertEquals(listOf("wind:b"), groups[0].rows.map { it.key })
        assertEquals("Música i llibres", groups[1].header)
        assertEquals(listOf("wind:c"), groups[1].rows.map { it.key })
        assertEquals(listOf("wind:a"), groups[2].rows.map { it.key })
    }

    @Test fun emptySectionDoesNotShowUp() {
        val sec = GregalApi.WindSection("buits", "Buits", listOf("wind:nope"))
        val groups = groupInbox(rows(), GregalApi.WindsMeta(emptyList(), listOf(sec)))
        assertEquals(1, groups.size)
        assertEquals(listOf("wind:a", "wind:b", "wind:c"), groups[0].rows.map { it.key })
    }
}
