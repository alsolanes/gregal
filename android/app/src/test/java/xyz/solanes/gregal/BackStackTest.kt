package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/** Enrere ha de tirar enrere: pila de pantalles al ViewModel. */
@RunWith(RobolectricTestRunner::class)
class BackStackTest {

    private fun vm(): GregalViewModel {
        val app = androidx.test.core.app.ApplicationProvider.getApplicationContext<android.app.Application>()
        val v = GregalViewModel(app)
        // Com si acabéssim de connectar: aterrem als Vents.
        v.go(Screen.Company)
        // La pila queda neta (go(Company) des de Pairing apila Pairing → netegem).
        v.resetBackStackForTest()
        return v
    }

    @Test
    fun goApilaIEnrereDesapila() {
        val v = vm()
        v.go(Screen.Chat)
        v.go(Screen.Work)
        assertEquals(Screen.Work, v.screen.value)
        assertTrue(v.back())            // Work → Chat
        assertEquals(Screen.Chat, v.screen.value)
        assertTrue(v.back())            // Chat → Company (arrel)
        assertEquals(Screen.Company, v.screen.value)
        assertFalse(v.back())           // ja a l'arrel sense pila → pot sortir
    }

    @Test
    fun goALaMatesPantallaNoApila() {
        val v = vm()
        v.go(Screen.Chat)
        v.go(Screen.Chat)
        // El primer go apila l'origen (Company); el segon, que no canvia
        // res, no ha d'apilar res més.
        assertEquals(listOf(Screen.Company), v.backStack.value)
    }

    @Test
    fun desDArrelVentsEnrereNoFaRes() {
        val v = vm()
        assertFalse(v.back())
        assertEquals(Screen.Company, v.screen.value)
    }

    @Test
    fun connectingNeverAddsPairingToNavigationHistory() {
        val app = androidx.test.core.app.ApplicationProvider.getApplicationContext<android.app.Application>()
        val v = GregalViewModel(app)
        v.go(Screen.Company)
        assertTrue(v.backStack.value.isEmpty())
        assertFalse(v.back())
    }

    @Test
    fun newChatPreservesWhereTheUserCameFrom() {
        val v = vm()
        v.go(Screen.Work)
        v.newChat()
        assertTrue(v.back())
        assertEquals(Screen.Work, v.screen.value)
    }

    @Test
    fun ventsXatObertEnrereElTanca() {
        val v = vm()
        v.openVentsChat("wind:tramuntana")
        assertEquals("wind:tramuntana", v.ventsOpen.value)
        v.openVentsChat(null)
        assertEquals(null, v.ventsOpen.value)
    }
}
