package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class SessionListUiTest {
    @get:Rule val ui = createComposeRule()

    @Test fun searchFiltersRowsAndDeletionStillRequiresConfirmationFromTheOverflowMenu() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        // Keep refreshes away from any developer server running on the default port.
        (vm.conn as MutableStateFlow).value = Conn("127.0.0.1", "1", "")
        val lang = vm.idioma.value
        (vm.sessions as MutableStateFlow).value = listOf(
            SessionInfo("review-demo", 12, "2026-11-13T10:00:00Z", title = "Revisió visual de Gregal", current = true),
            SessionInfo("notes-demo", 4, "2026-11-12T10:00:00Z", title = "Les notes de la reunió del projecte"))
        ui.setContent {
            CompositionLocalProvider(LocalPal provides CalmaFosc) {
                MaterialTheme(colorScheme = CalmaFosc.scheme()) { SessionsScreen(vm) {} }
            }
        }
        ui.onNodeWithText("Revisió visual de Gregal").performScrollTo().assertIsDisplayed()
        ui.onNodeWithText(tr(lang, "app.sess.delete")).assertDoesNotExist()
        ui.onNode(hasSetTextAction()).performTextInput("visual")
        ui.onNodeWithText("Les notes de la reunió del projecte").assertDoesNotExist()
        ui.onNodeWithContentDescription(tr(lang, "app.goals.more")).performClick()
        ui.onNodeWithText(tr(lang, "app.sess.delete")).performClick()
        ui.onNodeWithText("review-demo").assertIsDisplayed()
        ui.onNodeWithText(tr(lang, "btn.close")).performClick()
        ui.onNodeWithText("Revisió visual de Gregal").assertIsDisplayed()
    }
}
