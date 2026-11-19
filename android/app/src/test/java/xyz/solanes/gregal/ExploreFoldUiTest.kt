package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** Les crides d'exploració seguides del treball d'un torn es pleguen en una fila «Explorat». */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class ExploreFoldUiTest {
    @get:Rule val ui = createComposeRule()

    // Com el ViewModel en rebre un tool_call.
    private fun tool(id: String, name: String, args: String, state: StepState = StepState.Ok): WorkStep {
        val sum = summarizeTool(name, args, "ca")
        return WorkStep(id, StepKind.Tool, sum.label, sum.target, sum.meta, "sortida", state, name, exploreTool(name, args))
    }

    private fun show(vararg steps: WorkStep) {
        ui.setContent { MaterialTheme { WorkGroupCard(WorkGroup(steps.toList(), started = 1L, ended = 9_000L), null, "", "ca") } }
        // Obre la targeta del torn (la capçalera és el primer element tocable).
        ui.onAllNodes(hasClickAction())[0].performClick()
    }

    @Test fun consecutiveExplorationFoldsAndExpands() {
        show(
            tool("1", "read", """{"path":"a.go"}"""),
            tool("2", "bash", """{"command":"ls src"}"""),
            tool("3", "edit", """{"path":"a.go","old_string":"x","new_string":"y"}"""),
        )
        ui.onNodeWithText("Explorat").assertIsDisplayed()
        ui.onNodeWithText("1 fitxer · 1 llistat").assertIsDisplayed()
        ui.onNodeWithText("Edita").assertIsDisplayed()
        ui.onNodeWithText("Llista").assertDoesNotExist()
        ui.onNodeWithText("Explorat").performClick()
        ui.onNodeWithText("Llista").assertIsDisplayed()
        ui.onNodeWithText("src").assertIsDisplayed()
    }

    @Test fun aFailureInsideTheFoldStaysVisible() {
        show(
            tool("1", "read", """{"path":"a.go"}"""),
            tool("2", "bash", """{"command":"cat missing.go"}""", StepState.Failed),
        )
        ui.onNodeWithText("Explorat").assertIsDisplayed()
        ui.onNodeWithText("missing.go").assertIsDisplayed()
    }
}
