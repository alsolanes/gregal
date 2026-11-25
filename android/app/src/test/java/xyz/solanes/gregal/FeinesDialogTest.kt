package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/** Feina ensenya el que t'espera (els permisos pendents) i no només les feines de la colla. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class FeinesDialogTest {
    @get:Rule val ui = createComposeRule()

    @Test fun aPendingApprovalShowsUnderWaitingForYou() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.Ask(GregalApi.WindAsk("k1", "approve", name = "bash", kind = "delete",
            what = "rm \"C:/tmp/vell/pendent.txt\"", detail = "cd \"C:/tmp/vell\" && rm -f pendent.txt")))
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = DarkPal.scheme()) { FeinesDialog(vm, "ca", onOpenThread = {}) {} }
            }
        }
        ui.onNodeWithText("Feina").assertExists()
        ui.onNodeWithText("T'espera").assertExists()
        ui.onNodeWithText("Vols que esborri pendent.txt?").assertExists()
        ui.onNodeWithText("Permet").assertExists()
        ui.onNodeWithText("Fet").performScrollTo().assertExists()
    }

    // 153: com al web (136), la feina en marxa és una targeta amb la cara de qui la porta, que s'obre al lloc.
    @Test fun aRunningTaskIsACardWithItsOwnersFaceThatOpensInPlace() {
        val xaloc = Wind("xaloc", "Xaloc", "#EFA68F", "Música")
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>()).apply { setConn(Conn("127.0.0.1", "9", "t")) }
        fun <T> flow(name: String) = GregalViewModel::class.java.getDeclaredField(name).apply { isAccessible = true }.get(vm) as kotlinx.coroutines.flow.MutableStateFlow<T>
        flow<List<Wind>>("_winds").value = listOf(xaloc)
        flow<List<WindTask>?>("_windTasks").value = listOf(WindTask("t1", "Prepara la llista de concerts", "/w", "rotllana-1", 0, 0, "running",
            listOf("Xaloc"), listOf("concerts.md"), "", ""))
        var opened = ""
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { FeinesDialog(vm, "ca", onOpenThread = { opened = it }) {} }
            }
        }
        val card = hasAnyAncestor(hasTestTag("feina-card"))
        ui.onNode(hasText("Prepara la llista de concerts") and card, useUnmergedTree = true).assertExists()
        ui.onNode(hasText("En marxa") and card, useUnmergedTree = true).assertExists()
        ui.onNode(SemanticsMatcher.expectValue(WindFaceKey, androidx.compose.ui.graphics.Color(xaloc.argb)) and card, useUnmergedTree = true).assertExists()
        ui.onNodeWithText("concerts.md").assertDoesNotExist()
        ui.onNode(hasTestTag("feina-card-head")).performClick()
        ui.onNodeWithText("concerts.md").assertExists()
        ui.onNodeWithText("Obre el fil").performClick()
        assertEquals("rotllana-1", opened)
    }
}
