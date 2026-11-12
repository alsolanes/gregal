package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * Anar a casa per la LAN i fora pel domini, decidit per la xarxa on és el mòbil
 * i recordat mentre res no canviï. No es prova cap adreça a cada missatge:
 * això faria perdre segons cada vegada que ets fora.
 */
class ServerPickTest {

    private val lan = "192.168.1.84"
    private val ext = "gregal.solanes.xyz"
    private val home = listOf("192.168.1.30")
    private val away = listOf("10.20.30.40")

    @Before
    fun reset() = ServerPick.forget()

    @Test
    fun sameNetIsTheThirdOctet() {
        assertTrue(ServerPick.sameNet("192.168.1.84", "192.168.1.30"))
        assertFalse(ServerPick.sameNet("192.168.1.84", "192.168.2.30"))
        assertFalse(ServerPick.sameNet("192.168.1.84", ""))
        assertFalse(ServerPick.sameNet("192.168.1", "192.168.1.30"))
    }

    @Test
    fun atHomeGoesStraightToTheLan() {
        assertEquals("http://192.168.1.84:8097", ServerPick.base(lan, "8097", lan, ext, home))
    }

    @Test
    fun awayFromHomeGoesStraightToTheDomain() {
        assertEquals("https://gregal.solanes.xyz", ServerPick.base(lan, "8097", lan, ext, away))
    }

    /** Encara que hagis escrit el domini, a casa s'ha d'anar per la LAN. */
    @Test
    fun aDomainOnlySetupStillUsesTheLanAtHome() {
        assertEquals("http://192.168.1.84:8097", ServerPick.base(ext, "443", lan, ext, home))
        ServerPick.forget()
        assertEquals("https://gregal.solanes.xyz", ServerPick.base(ext, "443", lan, ext, away))
    }

    @Test
    fun withoutLearnedAddressesNothingChanges() {
        assertEquals("http://192.168.1.84:8097", ServerPick.base(lan, "8097", "", "", away))
    }

    @Test
    fun lanIpIsPulledOutOfTheWrittenAddress() {
        assertEquals("192.168.1.84", ServerPick.lanIpOf("http://192.168.1.84:8097"))
        assertEquals("192.168.1.84", ServerPick.lanIpOf("192.168.1.84:8097"))
        assertEquals("gregal.solanes.xyz", ServerPick.lanIpOf("gregal.solanes.xyz"))
    }

    /** Mentre no canvia res, la tria es recorda i no es recalcula. */
    @Test
    fun theChoiceIsStableWhileNothingChanges() {
        assertEquals("http://192.168.1.84:8097", ServerPick.base(lan, "8097", lan, ext, home, now = 1_000))
        assertEquals("http://192.168.1.84:8097", ServerPick.base(lan, "8097", lan, ext, home, now = 61_000))
    }

    /** I si surts de casa, es torna a decidir tot sol, sense esperar cap fallada. */
    @Test
    fun aNetworkChangeDecidesAgain() {
        assertEquals("http://192.168.1.84:8097", ServerPick.base(lan, "8097", lan, ext, home, now = 1_000))
        assertEquals("https://gregal.solanes.xyz", ServerPick.base(lan, "8097", lan, ext, away, now = 61_000))
    }

    /** Si una adreça no respon, la propera va per l'altra banda. */
    @Test
    fun aFailedAddressSwitchesToTheOther() {
        assertEquals("http://192.168.1.84:8097", ServerPick.base(lan, "8097", lan, ext, home, now = 1_000))
        assertEquals("https://gregal.solanes.xyz", ServerPick.failedToConnect("http://192.168.1.84:8097", now = 2_000))
        // La petició següent ja hi va, sense esperar res.
        assertEquals("https://gregal.solanes.xyz", ServerPick.base(lan, "8097", lan, ext, home, now = 3_000))
    }

    /** I si el que falla és el domini, es torna a la LAN. */
    @Test
    fun aFailedDomainGoesBackToTheLan() {
        ServerPick.base(lan, "8097", lan, ext, away, now = 1_000)
        assertEquals("http://192.168.1.84:8097", ServerPick.failedToConnect("https://gregal.solanes.xyz", now = 2_000))
    }

    /** Sense l'altra banda no hi ha res a canviar. */
    @Test
    fun withoutTheOtherSideThereIsNothingToSwitchTo() {
        ServerPick.base(lan, "8097", lan, "", home, now = 1_000)
        assertNull(ServerPick.failedToConnect("http://192.168.1.84:8097", now = 2_000))
    }

    /** El canvi forçat també caduca. */
    @Test
    fun theForcedSwitchExpires() {
        ServerPick.base(lan, "8097", lan, ext, home, now = 1_000)
        ServerPick.failedToConnect("http://192.168.1.84:8097", now = 2_000)
        assertEquals("http://192.168.1.84:8097", ServerPick.base(lan, "8097", lan, ext, home, now = 7_300_000))
    }
}
