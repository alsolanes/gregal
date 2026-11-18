package xyz.solanes.gregal

import androidx.compose.animation.EnterTransition
import androidx.compose.animation.ExitTransition
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test

/** El canvi de pantalla té direcció: la pila d'enrere diu si anem endavant o tornem. */
class ScreenTransitionTest {

    @Test
    fun laPilaQueCreixVaEndavant() {
        assertEquals(NavDirection.Forward, navDirection(0, 1))
        assertEquals(NavDirection.Forward, navDirection(2, 3))
    }

    @Test
    fun laPilaQueDecreixTornaEnrere() {
        assertEquals(NavDirection.Back, navDirection(1, 0))
        assertEquals(NavDirection.Back, navDirection(3, 2))
    }

    @Test
    fun laPilaIgualNomesFaFos() {
        assertEquals(NavDirection.None, navDirection(0, 0))
        assertEquals(NavDirection.None, navDirection(2, 2))
    }

    @Test
    fun ambMovimentHiHaTransicio() {
        for (d in NavDirection.entries) {
            val t = screenTransition(d, motion = true)
            assertNotEquals(EnterTransition.None, t.targetContentEnter)
            assertNotEquals(ExitTransition.None, t.initialContentExit)
        }
    }

    @Test
    fun senseMovimentElCanviEsImmediat() {
        for (d in NavDirection.entries) {
            val t = screenTransition(d, motion = false)
            assertEquals(EnterTransition.None, t.targetContentEnter)
            assertEquals(ExitTransition.None, t.initialContentExit)
        }
    }
}
