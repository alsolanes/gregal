package xyz.solanes.gregal

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * El fil que s'allarga més que la pantalla no ha d'arrencar la vista de cop.
 * Va passar a la conversa d'un vent: s'estava llegint el principi i, en créixer
 * el fil, la llista saltava a la cua («com si es tornés a pintar»).
 */
class ChatFollowGrowthTest {

    @Test
    fun `un fil que creix i no demanaves el final no t'arrenca`() {
        // Fins ara no hi havia scroll: el que es veia era el final. Creix i, si
        // no has demanat el final, no se't mou.
        assertFalse(shouldFollowOnGrowth(wasScrollable = false, scrollable = true, wantsEnd = false))
    }

    @Test
    fun `si has enviat o has baixat a ma, si que segueix`() {
        assertTrue(shouldFollowOnGrowth(wasScrollable = false, scrollable = true, wantsEnd = true))
    }

    @Test
    fun `un fil que ja feia scroll continua el final`() {
        // Ja hi havia scroll i estaves al final: acompanyar el text nou és el correcte.
        assertTrue(shouldFollowOnGrowth(wasScrollable = true, scrollable = true, wantsEnd = true))
        assertTrue(shouldFollowOnGrowth(wasScrollable = true, scrollable = true, wantsEnd = false))
    }

    @Test
    fun `si tot cap a la pantalla no hi ha res a seguir`() {
        assertTrue(shouldFollowOnGrowth(wasScrollable = false, scrollable = false, wantsEnd = false))
    }

    @Test
    fun `demanar el final es alhora seguir i voler-hi anar`() {
        val st = ChatFollowState()
        st.pause()
        assertFalse(st.following)
        st.requestFollow()
        assertTrue(st.following)
        assertTrue(st.wantsEnd)
        // Llegir a mà ho desactiva tot dues coses: no s'ha de tornar a saltar sol.
        st.startReading()
        assertFalse(st.wantsEnd)
        assertFalse(st.following)
    }
}
