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
}
