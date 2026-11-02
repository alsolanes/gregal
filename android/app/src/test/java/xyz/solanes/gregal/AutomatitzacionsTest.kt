package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class AutomatitzacionsTest {

    private fun job(id: String, kind: String, name: String = id) = JobItem(
        id = id, name = name, prompt = "fes una cosa", kind = kind, enabled = true, nextDue = "08:00",
    )

    @Test fun agrupaCadaCosaAlSeuBlocIPlegaTotsElsOrigens() {
        val jobs = listOf(
            job("r1", "remind", "Avisa'm del reg"),
            job("d1", "daily", "Resum del dia"),
            job("i1", "interval", "Mira el correu"),
            job("m1", "manual", "El que premo jo"),
        )
        val rutines = listOf(GregalApi.WindRutina(id = "p1", wind = "xaloc", windName = "Xaloc", name = "Pla",
            kind = "daily", time = "07:00", intervalH = 24.0, prompt = "revisa", enabled = true,
            nextDue = "", nextDueAt = "dilluns", lastStatus = ""))
        val fluxos = listOf(FlowSummary(name = "Flux", slug = "flux", desc = "tres passos", steps = 3))
        val recordatoris = listOf(ProgramatReminder(id = "q1", windName = "Marinada", at = "18:00", text = "truca"))

        val grups = agrupaAutomatitzacions("ca", jobs, rutines, fluxos, recordatoris)

        // Ordre fix: recordatoris, programades, plans, fluxos.
        assertEquals(listOf("recordatoris", "programades", "plans", "fluxos"), grups.map { it.clau })
        // El recordatori del job i el programat del servidor hi són tots dos.
        assertEquals(2, grups[0].items.size)
        // Les programades són daily + interval.
        assertEquals(2, grups[1].items.size)
        assertEquals(1, grups[2].items.size)
        assertEquals(1, grups[3].items.size)
        // Un treball manual no és cap automatització.
        assertTrue(grups.none { g -> g.items.any { it.titol == "El que premo jo" } })
    }

    @Test fun senseResLaPantallaHoPotDir() {
        val grups = agrupaAutomatitzacions("ca", emptyList(), emptyList(), emptyList(), emptyList())
        assertEquals(0, grups.sumOf { it.items.size })
        assertEquals(4, grups.size)
    }

    @Test fun elMotDelVentEsPotCanviarISurtATotsElsTextos() {
        val abans = tr("ca", "nav.company")
        assertEquals("Vents", abans)
        try {
            VentNom.singular = "follet"
            VentNom.plural = "follets"
            // El títol comença el text: el mot hi va amb majúscula.
            assertEquals("Follets", tr("ca", "nav.company"))
            // On el text ja porta majúscula, el mot també en porta.
            assertEquals("follets", tr("ca", "app.library.title").substringAfter("de "))
        } finally {
            VentNom.singular = ""
            VentNom.plural = ""
        }
        assertEquals("Vents", tr("ca", "nav.company"))
    }
}
