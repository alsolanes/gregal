package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isSelectable
import androidx.compose.ui.test.junit4.v2.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.text.TextLayoutResult
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode
import kotlin.math.abs

/** The avatar pickers lay their choices out evenly: whole names, one card width, swatches in a grid. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindPickerLayoutTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private fun show(theme: String, content: @Composable () -> Unit) = ui.setContent {
        CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false, LocalAvatarTheme provides theme) {
            MaterialTheme(colorScheme = LightPal.scheme()) { content() }
        }
    }

    private fun cutOff(text: String): Boolean {
        val node = ui.onNode(hasText(text), useUnmergedTree = true).fetchSemanticsNode()
        val layouts = mutableListOf<TextLayoutResult>()
        node.config[SemanticsActions.GetTextLayoutResult].action!!.invoke(layouts)
        return layouts.single().let { l -> l.multiParagraph.didExceedMaxLines || (0 until l.lineCount).any { l.isLineEllipsized(it) } }
    }

    // Catalan has the longest names («Rosa de rellotge», «Esperit de vent»).
    @Test fun `Preferences shows every pack name whole`() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        vm.setConn(Conn("127.0.0.1", "9", "t"))
        vm.attachPrefs(Prefs(ApplicationProvider.getApplicationContext<Application>()))
        vm.setIdioma("ca")
        vm.openPreferences(PrefsPage.Appearance)
        show("cares") { PrefsScreen(vm) {} }
        assertEquals("ca", vm.idioma.value)
        for (pack in windPacks + "free") {
            val name = tr("ca", avatarLabelKey(pack))
            ui.onNode(hasText(name) and isSelectable()).performScrollTo()
            assertFalse("«$name» is cut off", cutOff(name))
        }
        val width = { pack: String -> ui.onAllNodes(hasTestTag("avatarSample-$pack"), useUnmergedTree = true)[0].fetchSemanticsNode().boundsInRoot.width }
        assertTrue("a dial, which fills its square, is drawn smaller than a figure", width("rosa") < width("pelut"))
    }

    @Test fun `the editor's avatar choices are cards of one width with whole names`() {
        show("free") { WindEditor("ca", Wind("bit", "Bit", "#29ADFF", "lectura"), {}, { _, _, _, _ -> }, { _, _ -> }) }
        val widths = windPacks.map { pack ->
            val name = tr("ca", avatarLabelKey(pack))
            ui.onNode(hasText(name) and isSelectable()).performScrollTo()
            assertFalse("«$name» is cut off", cutOff(name))
            ui.onNode(hasText(name) and isSelectable()).fetchSemanticsNode().boundsInRoot.width
        }
        assertEquals("one width for every card: $widths", 1, widths.distinct().size)
    }

    @Test fun `a wind with its own colour gets two even rows of swatches`() {
        show("cares") { WindEditor("ca", Wind("bit", "Bit", "#29ADFF", "lectura"), {}, { _, _, _, _ -> }, { _, _ -> }) }
        val swatches = (1..9).map { n ->
            val node = ui.onNode(hasContentDescription(tr("ca", "app.vents.colorChoice", n.toString()))).performScrollTo().fetchSemanticsNode()
            node.boundsInRoot
        }
        val rows = swatches.groupBy { it.top.toInt() }.values.toList()
        assertEquals("nine swatches in two rows", 2, rows.size)
        val columns = rows[0].map { it.center.x }
        for (s in rows[1]) assertTrue("the second row sits in the first row's columns: ${s.center.x} vs $columns",
            columns.any { abs(it - s.center.x) < 1f })
        assertTrue("the wind's own colour is the ninth and chosen", ui.onNode(hasContentDescription(tr("ca", "app.vents.colorChoice", "9"))).fetchSemanticsNode().config[SemanticsProperties.Selected])
    }
}
