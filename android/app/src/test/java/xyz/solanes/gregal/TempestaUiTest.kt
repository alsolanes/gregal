package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/** «A fons» a l'Android: què fa cada agent al xat i el selector de models a Preferències. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class TempestaUiTest {
    @get:Rule val ui = createComposeRule()

    private fun show(content: @androidx.compose.runtime.Composable () -> Unit) = ui.setContent {
        CompositionLocalProvider(LocalPal provides LightPal) { MaterialTheme(colorScheme = LightPal.scheme()) { content() } }
    }

    @Test fun elTaulerEnsenyaLaDescripcioNomesSiNHiHa() {
        val agents = listOf(
            TempestaAgent(0, "direct", "Directe", "Direct", "deepseek", "working", "Respon pas a pas.", "Answers step by step."),
            TempestaAgent(1, "gregal", "Gregal", "Gregal", "", "working"),
        )
        show { TempestaPanel("en", agents) }
        ui.onNodeWithText("Answers step by step.").assertIsDisplayed()
        ui.onNodeWithText("Respon pas a pas.").assertDoesNotExist()
        ui.onAllNodesWithTag("tempesta-desc").assertCountEquals(1)
    }

    @Test fun cadaAgentILaSintesiTenenSelectorAmbAutomatic() {
        val picked = mutableListOf<Pair<String, String>>()
        val chosen = mapOf("direct" to "", "sources" to "zen/glm-5.3-flash", "skeptic" to "", "synth" to "")
        show { TempestaModelsSection("ca", chosen, mapOf("zen" to listOf("glm-5.3-flash", "deepseek-v4.1-flash"))) { s, m -> picked += s to m } }
        listOf("Directe", "Fonts", "Advocat del diable", "Síntesi").forEach { ui.onNodeWithText(it).assertExists() }
        ui.onNodeWithText("Busca fonts oficials i actuals.").assertExists()
        ui.onAllNodesWithText("Automàtic").assertCountEquals(3)
        ui.onNodeWithTag("tempesta-model-direct").performClick()
        ui.onNodeWithText("zen/deepseek-v4.1-flash").performClick()
        ui.onNodeWithTag("tempesta-model-sources").performClick()
        ui.onNodeWithTag("tempesta-option-auto").performClick()
        assertEquals(listOf("direct" to "zen/deepseek-v4.1-flash", "sources" to ""), picked)
    }
}
