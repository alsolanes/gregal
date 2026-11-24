package xyz.solanes.gregal

import androidx.compose.ui.graphics.Color
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * La conversa d'un vent es tenyeix del seu color (fons i bombolles) i la
 * capçalera en diu l'estat. Colors: els de les colles (winds_squads.go) i el
 * blau de Gregal.
 */
class WindChatColorStateTest {
    private val colors = listOf(
        0xFFF4B6C2, 0xFFEACF87, 0xFF9ED3C5, 0xFFEFA68F, 0xFFF6C7A1, 0xFFA9D3F0, 0xFFC1B4DE,
        0xFFA8CDE2, 0xFFBFCFA4, 0xFFB9C6F2, 0xFFF5C99B, 0xFFD7B8E6,
        0xFFF2B8A2, 0xFFA8D8EA, 0xFFC7C2F0, 0xFFCFE3A6, 0xFF9CC4E8, 0xFFE8D3A3,
        0xFF3D7BFF,
    ).map { Color(it) }

    @Test fun `el text de les bombolles del vent passa AA en clar i en fosc`() {
        for (t in GregalTemes) for ((mode, p) in listOf("clar" to t.clar.solid(), "fosc" to t.fosc.solid())) {
            for (c in colors) {
                val bubble = windBubbleColor(c, windWash(c, p.surface))
                for (text in listOf(p.ink, p.text)) {
                    val ratio = contrastWith(text, bubble)
                    assertTrue("${t.id}/$mode $c: contrast %.2f < 4.5".format(ratio), ratio >= 4.5)
                }
            }
        }
    }

    @Test fun `el fons i la bombolla agafen el color del vent`() {
        val accent = Color(0xFFEFA68F)
        for (p in listOf(DarkPal, LightPal)) {
            val wash = windWash(accent, p.surface)
            val bubble = windBubbleColor(accent, wash)
            assertNotEquals(p.surface, wash)
            assertNotEquals(wash, bubble)
            // Més rosat que la superfície: el vermell puja respecte del blau.
            assertTrue(wash.red - wash.blue > p.surface.red - p.surface.blue)
            assertTrue(bubble.red - bubble.blue > wash.red - wash.blue)
            assertEquals(1f, bubble.alpha)
        }
    }

    private fun ask(wind: String) = GregalApi.WindAsk(key = "k-$wind", type = "approve", wind = wind)

    @Test fun `treballant quan el vent te un torn en marxa`() =
        assertEquals("winds.state.working", windChatState("xaloc", setOf("xaloc"), emptyList(), done = false))

    @Test fun `t'espera quan el vent te una pregunta per a tu`() {
        assertEquals("winds.state.waiting", windChatState("xaloc", emptySet(), listOf(ask("garbi"), ask("xaloc")), done = false))
        // Una pregunta en un torn en marxa: el que importa és que t'espera.
        assertEquals("winds.state.waiting", windChatState("xaloc", setOf("xaloc"), listOf(ask("xaloc")), done = true))
    }

    @Test fun `fet quan s'ha acabat el torn`() =
        assertEquals("winds.state.done", windChatState("xaloc", setOf("garbi"), listOf(ask("garbi")), done = true))

    @Test fun `res altrament`() =
        assertNull(windChatState("xaloc", setOf("garbi"), listOf(ask("garbi")), done = false))

    @Test fun `les etiquetes hi son en catala i en angles`() {
        for (key in listOf("winds.state.working", "winds.state.waiting", "winds.state.done")) {
            assertTrue(key, STR_CA.containsKey(key) && STR_EN.containsKey(key))
            assertNotEquals(STR_CA[key], STR_EN[key])
        }
    }
}
