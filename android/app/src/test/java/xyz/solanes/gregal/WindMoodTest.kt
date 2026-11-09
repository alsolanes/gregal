package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

/** Les cares només mostren estats observats, amb la mateixa taula que el web. */
class WindMoodTest {
    @Test fun `fases del directe`() {
        assertEquals(WindMood.THINKING, windMoodOf(phase = "thinking"))
        assertEquals(WindMood.SEARCHING, windMoodOf(phase = "searching"))
        assertEquals(WindMood.WORKING, windMoodOf(phase = "scheduling"))
        assertEquals(WindMood.WRITING, windMoodOf(phase = "thinking", writing = true))
        assertEquals(WindMood.IDLE, windMoodOf())
    }

    @Test fun `la segona veu d'un torn discuteix`() {
        assertEquals(WindMood.DISCUSSING, WindLive(voices = 2).mood)
        assertEquals(WindMood.DISCUSSING, WindLive(writing = true, voices = 3).mood)
        assertEquals(WindMood.WRITING, WindLive(writing = true).mood)
        assertEquals(WindMood.THINKING, WindLive().mood)
        assertEquals(WindMood.READING, WindLive(phase = "tool", tool = "read_file").mood)
        assertEquals(WindMood.WRITING, WindLive(phase = "tool", tool = "read_file", writing = true).mood)
        assertEquals(WindMood.THINKING, WindLive(phase = "tool_done", tool = "read_file").mood)
    }

    @Test fun `passos en marxa segons l'activitat`() {
        assertEquals(WindMood.READING, windMoodOf(state = "working", activity = "read"))
        assertEquals(WindMood.SEARCHING, windMoodOf(state = "working", activity = "grep"))
        assertEquals(WindMood.WRITING, windMoodOf(state = "working", activity = "write"))
        assertEquals(WindMood.WORKING, windMoodOf(state = "working", activity = "bash"))
        assertEquals(WindMood.DISCUSSING, windMoodOf(state = "working", activity = "discussion"))
        assertEquals(WindMood.DONE, windMoodOf(state = "completed", activity = "write"))
        assertEquals(WindMood.FAILED, windMoodOf(state = "failed"))
        assertEquals(WindMood.IDLE, windMoodOf(state = "waiting"))
    }

    @Test fun `novetats sense obrir per vent`() {
        fun n(ts: Long, wind: String) = WindNovetat(ts, wind, wind, "remind", "hola")
        val list = listOf(n(100, "xaloc"), n(200, "marinada"), n(50, "gregal"), n(300, ""))
        assertEquals(setOf("xaloc", "marinada"), WindNewsLogic.unseen(list, emptyMap(), floor = 60))
        assertEquals(setOf("marinada"), WindNewsLogic.unseen(list, mapOf("xaloc" to 150L), floor = 60))
        assertEquals(emptySet<String>(), WindNewsLogic.unseen(list, mapOf("xaloc" to 150L, "marinada" to 200L), floor = 60))
    }

    /** La insígnia diu quants missatges esperen, no només que n'hi ha. */
    @Test fun `quants missatges pendents per vent`() {
        fun n(ts: Long, wind: String) = WindNovetat(ts, wind, wind, "remind", "hola")
        val list = listOf(
            n(100, "xaloc"), n(120, "xaloc"), n(140, "xaloc"),
            n(200, "marinada"),
            n(50, "gregal"),   // abans del terra: no compta
            n(300, ""),        // sense vent: no compta
        )
        assertEquals(mapOf("xaloc" to 3, "marinada" to 1), WindNewsLogic.unseenCounts(list, emptyMap(), floor = 60))
        // Amb xaloc vist fins al 100, en queden dos (120 i 140) — i marinada, un.
        assertEquals(mapOf("xaloc" to 2, "marinada" to 1), WindNewsLogic.unseenCounts(list, mapOf("xaloc" to 100L), floor = 60))
        assertEquals(emptyMap<String, Int>(), WindNewsLogic.unseenCounts(list, mapOf("xaloc" to 140L), floor = 200))
        // Les dues cares de la mateixa cosa: el conjunt i el compte no es poden separar.
        val seen = mapOf("marinada" to 200L)
        assertEquals(WindNewsLogic.unseen(list, seen, 60), WindNewsLogic.unseenCounts(list, seen, 60).keys)
    }

    @Test fun `el text de la insígnia`() {
        assertEquals("", unreadBadge(0))
        assertEquals("", unreadBadge(-2))
        assertEquals("1", unreadBadge(1))
        assertEquals("9", unreadBadge(9))
        assertEquals("9+", unreadBadge(10))
        assertEquals("9+", unreadBadge(87))
    }
}
