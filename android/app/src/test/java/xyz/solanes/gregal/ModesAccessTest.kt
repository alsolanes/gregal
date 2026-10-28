package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test

/**
 * Els mateixos tres modes i tres permisos que la web i el TUI
 * (internal/agent/access.go), amb el text a tots dos idiomes.
 */
class ModesAccessTest {
    @Test
    fun elsModesInternsEsVeuenComPreguntaPlanificaOActua() {
        mapOf(
            "chat" to "chat", "inspect" to "chat",
            "plan" to "plan", "goal" to "plan",
            "code" to "act", "autonomous" to "act", "debug" to "act",
        ).forEach { (mode, ui) -> assertEquals(mode, ui, uiModeOf(mode)) }
    }

    @Test
    fun elsPermisosDActuaTenenNomIAjudaATotsDosIdiomes() {
        assertEquals(listOf("ask", "edits", "full"), ACCESSES)
        for (lang in listOf("ca", "en")) {
            for (a in ACCESSES) {
                for (key in listOf("app.perm.$a", "app.perm.${a}Help")) {
                    assertNotEquals(key, tr(lang, key))
                }
            }
            for (key in listOf("app.mode.ask", "app.mode.plan", "app.mode.act", "prefs.reviewer", "app.chat.composerPlan")) {
                assertNotEquals(key, tr(lang, key))
            }
        }
        assertEquals("Autonomia total", tr("ca", "app.perm.full"))
        assertEquals("Full autonomy", tr("en", "app.perm.full"))
    }
}
