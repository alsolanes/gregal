package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
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
    private fun tool(id: String, name: String, args: String, state: StepState = StepState.Ok, out: String = "sortida"): WorkStep {
        val sum = summarizeTool(name, args, "ca")
        return WorkStep(id, StepKind.Tool, sum.label, sum.target, sum.meta, out, state, name, exploreTool(name, args), args)
    }

    private fun show(vararg steps: WorkStep) {
        ui.setContent { MaterialTheme { WorkGroupCard(WorkGroup(steps.toList(), started = 1L, ended = 9_000L), null, "", "ca") } }
        // Obre la targeta del torn (la capçalera és el primer element tocable).
        ui.onAllNodes(hasClickAction())[0].performClick()
    }

    // Les crides van dins de frases (tasca 009): la fila «Explorat» és una pastilla i s'obre en tocar-la.
    @Test fun consecutiveExplorationFoldsIntoOneChipThatOpensAndCloses() {
        show(
            tool("1", "read", """{"path":"a.go"}"""),
            tool("2", "bash", """{"command":"ls src"}"""),
            tool("3", "edit", """{"path":"a.go","old_string":"x","new_string":"y"}"""),
        )
        // Sense narració, la frase la posa Gregal; el detall de les crides no es veu.
        ui.onNodeWithText("Ha editat fitxers").assertIsDisplayed()
        ui.onNodeWithText("1 fitxer · 1 llistat").assertIsDisplayed()
        ui.onNodeWithText("a.go +1 −1").assertIsDisplayed()
        ui.onNodeWithText("Explorat").assertDoesNotExist()
        ui.onNodeWithText("Edita").assertDoesNotExist()
        ui.onNodeWithText("1 fitxer · 1 llistat").performClick()
        ui.onNodeWithText("Explorat").assertIsDisplayed()
        ui.onNodeWithText("Llista").assertIsDisplayed()
        ui.onNodeWithText("src").assertIsDisplayed()
        ui.onAllNodesWithText("1 fitxer · 1 llistat")[0].performClick()
        ui.onNodeWithText("Explorat").assertDoesNotExist()
        ui.onNodeWithText("a.go +1 −1").performClick()
        ui.onNodeWithText("Edita").assertIsDisplayed()
        ui.onNodeWithText("sortida").assertIsDisplayed()
        ui.onNodeWithText("a.go +1 −1").performClick()
        ui.onNodeWithText("Edita").assertDoesNotExist()
    }

    @Test fun narrationOwnsTheChipsAfterIt() {
        show(
            WorkStep("n", StepKind.Note, "Busco on es valida", detail = "Busco on es valida"),
            tool("1", "read", """{"path":"src/login.go"}"""),
        )
        ui.onNodeWithText("Busco on es valida").assertIsDisplayed()
        ui.onNodeWithText("login.go").assertIsDisplayed()
        ui.onNodeWithText("Ha mirat els fitxers").assertDoesNotExist()
    }

    @Test fun aFailedChipYouClosedStaysClosedWhenAnotherFails() {
        var work by mutableStateOf(WorkGroup(listOf(
            tool("1", "bash", """{"command":"npm run build"}""", StepState.Failed, "ERROR: exit: exit status 2\nTS2304: x"),
        ), started = 1L, ended = 9_000L))
        ui.setContent { MaterialTheme { WorkGroupCard(work, null, "", "ca") } }
        ui.onAllNodes(hasClickAction())[0].performClick()
        ui.onNodeWithText("TS2304: x").assertIsDisplayed()
        ui.onNodeWithText("npm run build · codi 2").performClick()
        ui.onNodeWithText("TS2304: x").assertDoesNotExist()
        work = work.add(tool("2", "bash", """{"command":"go vet ./..."}""", StepState.Failed, "ERROR: exit: exit status 1\nvet: y"))
        ui.onNodeWithText("vet: y").assertIsDisplayed()
        ui.onNodeWithText("TS2304: x").assertDoesNotExist()
    }

    // Sovint és l'única còpia del que ha dit el model: es llegeix sencera, no retallada.
    @Test fun longNarrationIsNotCut() {
        val long = (1..12).joinToString("\n") { "Línia $it de la narració" }
        show(
            WorkStep("n", StepKind.Note, "Línia 1 de la narració", detail = long),
            tool("1", "read", """{"path":"src/login.go"}"""),
        )
        val layout = mutableListOf<androidx.compose.ui.text.TextLayoutResult>()
        ui.onNodeWithText(long).fetchSemanticsNode()
            .config[androidx.compose.ui.semantics.SemanticsActions.GetTextLayoutResult].action!!.invoke(layout)
        org.junit.Assert.assertEquals(12, layout[0].lineCount)
        org.junit.Assert.assertFalse(layout[0].didOverflowHeight)
    }

    @Test fun aFailedCallOpensRedAndAQuietOneStaysClosed() {
        show(
            tool("1", "bash", """{"command":"grep -rn foo src"}""", out = "ERROR: exit: exit status 1\n"),
            tool("2", "bash", """{"command":"npm run build"}""", StepState.Failed, "ERROR: exit: exit status 2\nTS2304: x"),
        )
        ui.onNodeWithText("\"foo\" · sense resultats").assertIsDisplayed()
        ui.onNodeWithText("Cerca").assertDoesNotExist()
        ui.onNodeWithText("npm run build · codi 2").assertIsDisplayed()
        // Obert sol, i la línia del codi de sortida no es repeteix a la sortida.
        ui.onNodeWithText("TS2304: x").assertIsDisplayed()
        ui.onNodeWithText("ERROR: exit", substring = true).assertDoesNotExist()
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
