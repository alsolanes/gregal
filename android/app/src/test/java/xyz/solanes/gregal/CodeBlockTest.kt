package xyz.solanes.gregal

import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.text.TextLayoutResult
import androidx.compose.ui.unit.sp
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** Un bloc de codi conserva les línies senceres i es desplaça de costat. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class CodeBlockTest {
    @get:Rule val ui = createComposeRule()

    private val code = "fun main() {\n    val s = \"" + "x".repeat(400) + "\"\n}"

    private fun layout(): TextLayoutResult {
        val results = mutableListOf<TextLayoutResult>()
        ui.onNodeWithText(code).fetchSemanticsNode().config[SemanticsActions.GetTextLayoutResult].action!!(results)
        return results.single()
    }

    @Test fun `una linia mes llarga que la pantalla no es parteix`() {
        ui.setContent { CodeBlock("kotlin", code, "en") }
        assertEquals("tres línies de codi, tres línies pintades", 3, layout().lineCount)
    }

    @Test fun `el codi es desplaca de costat i la capcalera es queda a lloc`() {
        ui.setContent { CodeBlock("kotlin", code, "en") }
        val scroller = ui.onNode(hasScrollAction())
        val range = scroller.fetchSemanticsNode().config.getOrNull(SemanticsProperties.HorizontalScrollAxisRange)
        assertTrue("el bloc ha de tenir recorregut horitzontal", range != null && range.maxValue() > 0f)
        scroller.performSemanticsAction(SemanticsActions.ScrollBy) { it(range!!.maxValue(), 0f) }
        ui.waitForIdle()
        assertEquals("el codi ha arribat al final", range!!.maxValue(), range.value())
        ui.onNodeWithText(tr("en", "app.code.copy")).assertIsDisplayed()
        ui.onNodeWithText("kotlin").assertIsDisplayed()
    }

    @Test fun `el codi te interlineat propi`() {
        ui.setContent { CodeBlock("kotlin", code, "en") }
        val style = layout().layoutInput.style
        assertEquals(13.sp, style.fontSize)
        assertEquals(19.sp, style.lineHeight)
    }
}
