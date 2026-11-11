package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import java.time.LocalDate

/** La safata «Avui»: lectura del JSON del servidor (org.json necessita Robolectric). */
@RunWith(RobolectricTestRunner::class)
class AvuiJsonTest {
    private val raw = """{"date":"2026-11-10",
        "now":[{"id":"c1","wind":"xaloc","wind_name":"Xaloc","source":"brief","section":"now","title":"Truca al dentista",
            "why":"Ho vas apuntar dilluns","actions":[{"label":"Més detalls","prompt":"Explica'm més: Truca al dentista"},{"label":""}],
            "state":"open","created":1762761600,"days":2,"held":true}],
        "fyi":[{"id":"c2","wind":"gregal","wind_name":"Gregal","source":"heart","section":"fyi","title":"Plou a la tarda",
            "state":"open","feedback":"useful","saved":true,"created":1762765200}],
        "ahead":[],"done":[{"id":"c0","wind":"xaloc","wind_name":"Xaloc","section":"now","title":"Fet ja","state":"done"}],
        "held":3,"settings":{"quiet_from":"23:00","quiet_to":"07:30","level":"calm","tune":"menys notícies"}}"""

    @Test fun llegeixLaSafata() {
        val v = AvuiJson.view(raw)
        assertEquals("2026-11-10", v.date)
        assertEquals(1, v.now.size)
        val c = v.now[0]
        assertEquals("c1", c.id); assertEquals("xaloc", c.wind); assertEquals("Xaloc", c.windName)
        assertEquals("Ho vas apuntar dilluns", c.why); assertEquals(2, c.days); assertTrue(c.held)
        // L'acció sense etiqueta no hi entra.
        assertEquals(listOf(AvuiAction("Més detalls", "Explica'm més: Truca al dentista")), c.actions)
        assertEquals("useful", v.fyi[0].feedback); assertTrue(v.fyi[0].saved)
        assertTrue(v.ahead.isEmpty())
        assertEquals(1, v.done.size)
        assertEquals(3, v.held)
        assertEquals(AvuiSettings("23:00", "07:30", "calm", "menys notícies"), v.settings)
        assertTrue(v.settings.calm)
        assertEquals(2, v.openCount)
    }

    @Test fun respostaBuidaDonaValorsPerDefecte() {
        val v = AvuiJson.view("{}")
        assertTrue(v.all.isEmpty())
        assertEquals(AvuiSettings(), v.settings)
        assertEquals("normal", AvuiJson.settings("""{"level":"rar"}""").level)
    }

    @Test fun novetatsPortenTargetaISilenci() {
        val list = AvuiJson.novetats("""{"novetats":[
            {"ts":10,"wind":"xaloc","name":"Xaloc","source":"heart","text":"Hola","card":"c9","quiet":true},
            {"ts":11,"wind":"xaloc","name":"Xaloc","source":"remind","text":"Avís","job":"r1"},
            {"ts":12,"wind":"xaloc","text":"  "}]}""")
        assertEquals(2, list.size)
        assertEquals("c9", list[0].card); assertTrue(list[0].quiet)
        assertEquals("", list[1].card); assertFalse(list[1].quiet); assertEquals("r1", list[1].job)
    }
}

/** Lògica pura de la safata i del vigilant. */
class AvuiLogicTest {
    private fun card(id: String, section: String = "now") = AvuiCard(id, "xaloc", "Xaloc", "brief", section, "T $id")
    private val view = AvuiView("2026-11-10", listOf(card("a"), card("b")), listOf(card("f", "fyi")), emptyList(), emptyList(), 0, AvuiSettings())

    @Test fun fetLaTreuIComptaComAFeta() {
        val v = AvuiLogic.apply(view, "a", "done")
        assertEquals("done", v.now[0].state)
        assertEquals(listOf("a"), v.done.map { it.id })
        assertEquals(2, v.openCount)
        // Dues vegades no la compta dues vegades.
        assertEquals(1, AvuiLogic.apply(v, "a", "done").done.size)
    }

    @Test fun noMInteressaLAmagaIToggleDeFeedback() {
        val v = AvuiLogic.apply(view, "f", "notuseful")
        assertEquals("notuseful", v.fyi[0].feedback); assertEquals("dismissed", v.fyi[0].state)
        val u = AvuiLogic.apply(view, "b", "useful")
        assertEquals("useful", u.now[1].feedback); assertEquals("open", u.now[1].state)
        assertEquals("", AvuiLogic.apply(u, "b", "useful").now[1].feedback)
    }

    @Test fun mesTardIAraNoSurtenDeLaVista() {
        assertEquals("later", AvuiLogic.apply(view, "a", "later").now[0].state)
        assertEquals("dismissed", AvuiLogic.apply(view, "a", "dismiss").now[0].state)
    }

    @Test fun siElServidorFallaTornaComEra() {
        val after = AvuiLogic.apply(view, "a", "done")
        val back = AvuiLogic.revert(after, view, "a")
        assertEquals("open", back.now[0].state)
        assertTrue(back.done.isEmpty())
    }

    @Test fun dataEnCatalaIAnglès() {
        assertEquals("Dimarts 10 de novembre", AvuiLogic.dateLabel("ca", "2026-11-10"))
        assertEquals("Dijous 1 d’octubre", AvuiLogic.dateLabel("ca", "2026-10-01"))
        assertEquals("Tuesday, November 10", AvuiLogic.dateLabel("en", "2026-11-10"))
        assertEquals("Dimarts 10 de novembre", AvuiLogic.dateLabel("ca", "", LocalDate.of(2026, 11, 10)))
    }

    @Test fun edatIComptador() {
        assertEquals("", AvuiLogic.ageLabel("ca", 0))
        assertEquals("des d’ahir", AvuiLogic.ageLabel("ca", 1))
        assertEquals("des de fa 2 dies", AvuiLogic.ageLabel("ca", 2))
        assertEquals("open for 3 days", AvuiLogic.ageLabel("en", 3))
        assertEquals("1 feta avui", AvuiLogic.doneLabel("ca", 1))
        assertEquals("4 done today", AvuiLogic.doneLabel("en", 4))
    }

    @Test fun horesValides() {
        assertTrue(AvuiJson.validTime("22:00")); assertTrue(AvuiJson.validTime("07:30"))
        assertFalse(AvuiJson.validTime("24:00")); assertFalse(AvuiJson.validTime("7:3")); assertFalse(AvuiJson.validTime(""))
    }

    @Test fun lesTranquillesNoFanSonarPeroElCursorAvanca() {
        val noves = listOf(
            WindNovetat(10, "xaloc", "Xaloc", "heart", "fort", card = "c1"),
            WindNovetat(20, "xaloc", "Xaloc", "heart", "fluix", card = "c2", quiet = true),
            WindNovetat(30, "gregal", "Gregal", "heart", "nit", quiet = true),
        )
        val ts = WindWatchLogic.noves(noves, 0).second
        val ring = WindWatchLogic.noves(WindWatchLogic.toRing(noves), 0).first
        assertEquals(30L, ts)
        assertEquals(setOf("xaloc"), ring.keys)
        // La que sona és la que no era tranquil·la, encara que n'hi hagi una de més nova.
        assertEquals("fort", ring["xaloc"]?.text)
    }
}
