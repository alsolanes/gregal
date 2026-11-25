package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** 139: el vent del xat no salta entre baix i dalt i canvia d'estat suau, sense reiniciar-se a cada eina. */
class WindSmoothTest {
    @Test fun theWindGoesUpOnlyClearlyAwayAndComesBackOnlyNearTheEnd() {
        var up = false
        // (dp des del final, en desplaçar-se) -> on queda
        val steps = listOf(
            0f to false, 120f to false, 199f to false, 150f to false,
            // Clarament lluny del final: puja.
            210f to true,
            // Tornant a baixar no torna fins que no és a prop del final.
            199f to true, 201f to true, 150f to true, 60f to true, 41f to true,
            39f to false,
            // Rondar un sol llindar no el fa saltar.
            45f to false, 35f to false, 190f to false, 100f to false,
        )
        for ((fromEnd, want) in steps) {
            up = windFliesUp(up, fromEnd, scrolled = true)
            assertEquals("at $fromEnd dp from the end", want, up)
        }
    }

    @Test fun contentGrowingWhileItStreamsDoesNotMoveIt() {
        // Sense desplaçar-se, la resposta creix i el final s'allunya: es queda a baix.
        var up = false
        for (fromEnd in listOf(0f, 80f, 260f, 600f)) up = windFliesUp(up, fromEnd, scrolled = false)
        assertFalse(up)
        // I a dalt, tampoc baixa perquè sí si continua lluny.
        assertTrue(windFliesUp(true, 600f, scrolled = false))
    }

    @Test fun aWorkMoodHoldsWhileToolsSwitchQuickly() {
        val hold = WindMoodHold(WindMood.IDLE, 0L)
        assertEquals(WindMood.SEARCHING, hold.at(0L, WindMood.SEARCHING))
        // Una altra eina abans de 1,2 s: continua l'animació que anava.
        assertEquals(WindMood.SEARCHING, hold.at(300L, WindMood.READING))
        assertEquals(900L, hold.waitFor(300L))
        // Torna a la de abans dins el marge: res pendent.
        assertEquals(WindMood.SEARCHING, hold.at(600L, WindMood.SEARCHING))
        assertEquals(0L, hold.waitFor(600L))
        assertEquals(WindMood.SEARCHING, hold.at(900L, WindMood.WRITING))
        assertEquals(WindMood.SEARCHING, hold.at(1199L, WindMood.WRITING))
        assertEquals(WindMood.WRITING, hold.at(1200L, WindMood.WRITING))
        // L'estat nou també es manté.
        assertEquals(WindMood.WRITING, hold.at(1500L, WindMood.DONE))
        assertEquals(WindMood.DONE, hold.at(2400L, WindMood.DONE))
        // Fora de la feina canvia de seguida.
        assertEquals(WindMood.WAITING, hold.at(2450L, WindMood.WAITING))
    }

    @Test fun aBurstOfToolsInTwoSecondsChangesTheFaceAtMostTwice() {
        val hold = WindMoodHold(WindMood.IDLE, 0L)
        val tools = listOf(WindMood.SEARCHING, WindMood.READING, WindMood.WORKING, WindMood.WRITING, WindMood.THINKING)
        var wanted = WindMood.IDLE
        var shown = WindMood.IDLE
        var changes = 0
        // Un esdeveniment cada 150 ms; el rellotge virtual avança de 50 en 50 (els temporitzadors de la cara).
        for (now in 0L..2000L step 50L) {
            if (now % 150L == 0L) wanted = tools[(now / 150L).toInt() % tools.size]
            val next = hold.at(now, wanted)
            if (next != shown) changes++
            shown = next
        }
        assertTrue("$changes visible changes", changes in 1..2)
    }

    @Test fun changingMoodHalfwayStartsFromWhereTheFaceIs() {
        val blend = WindMoodBlend(WindMood.IDLE)
        assertEquals(1f, blend.weight(WindMood.IDLE, 1f), 1e-6f)
        blend.retarget(WindMood.SEARCHING, 1f)
        val halfway = WindMood.entries.associateWith { blend.weight(it, .4f) }
        blend.retarget(WindMood.READING, .4f)
        // En el moment del canvi els pesos no salten.
        for (m in WindMood.entries) assertEquals("$m", halfway.getValue(m), blend.weight(m, 0f), 1e-6f)
        assertEquals(1f, WindMood.entries.sumOf { blend.weight(it, .7f).toDouble() }.toFloat(), 1e-5f)
        assertEquals(1f, blend.weight(WindMood.READING, 1f), 1e-6f)
        assertEquals(0f, blend.weight(WindMood.SEARCHING, 1f), 1e-6f)
    }
}
