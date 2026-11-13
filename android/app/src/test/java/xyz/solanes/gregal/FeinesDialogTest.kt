package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
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
        ui.onNodeWithText("Esborra'l").assertExists()
        ui.onNodeWithText("Fet").performScrollTo().assertExists()
    }
}
