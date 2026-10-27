package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Test

class FeinesTest {
    private fun t(id: String, status: String) = WindTask(id, "t", "/w", "", 0, 0, status, emptyList(), emptyList(), "", "")

    @Test fun `nomes avisa de les que anaven en marxa i ja no`() {
        val abans = listOf(t("a", "running"), t("b", "done"), t("c", "running"))
        val ara = listOf(t("a", "done"), t("b", "done"), t("c", "running"), t("d", "done"))
        assertEquals(listOf("a"), feinesAcabades(abans, ara).map { it.id })
    }

    @Test fun `els fitxers relatius es busquen a la carpeta de la feina`() {
        assertEquals("/srv/w/index.html", t("a", "done").copy(workspace = "/srv/w/").pathOf("index.html"))
        assertEquals("C:/x/a.html", t("a", "done").pathOf("C:/x/a.html"))
    }
}
