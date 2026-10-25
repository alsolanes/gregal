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
import org.robolectric.annotation.LooperMode

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class VentsMessengerTest {
    @get:Rule val ui = createComposeRule()

    @Test fun crewStaysInOneStripWithoutSquadHeadings() {
        val winds = listOf(
            Wind("a", "Marinada", "#EFC69A", "calma", squad = "daily"),
            Wind("b", "Xaloc", "#EFA68F", "música", squad = "data"),
            Wind("c", "Mestral", "#A8CDE2", "recerca", squad = "software"),
        )
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { VentsCrew("ca", winds, emptySet()) {} }
            }
        }
        ui.onNodeWithText(tr("ca", "app.vents.squad.daily").uppercase()).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.vents.squad.data").uppercase()).assertDoesNotExist()
        ui.onNodeWithText("Marinada").assertIsDisplayed()
        ui.onNodeWithText("Xaloc").assertIsDisplayed()
        ui.onNodeWithText("Mestral").assertIsDisplayed()
    }

    @Test fun groupDraftAndSelectedMembersSurviveThemeRecreation() {
        val restore = androidx.compose.ui.test.junit4.StateRestorationTester(ui)
        val winds = listOf(Wind("a", "Marinada", "#20BE92", "calma"), Wind("b", "Xaloc", "#1A9BEE", "música"))
        restore.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { ConversationPicker("ca", winds, true, {}, onCreate = { _, _ -> }) }
            }
        }
        ui.onNodeWithText(tr("ca", "app.vents.chatName")).performTextInput("Escapades")
        ui.onNodeWithText("Marinada").performClick()
        ui.onNodeWithText("Xaloc").performClick()
        restore.emulateSavedInstanceStateRestore()
        ui.onNodeWithText("Escapades").assertExists()
        ui.onNodeWithText(tr("ca", "app.vents.startChat")).assertIsEnabled()
    }

    @Test fun directChatDraftSurvivesThemeRecreation() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        val wind = Wind("draft", "Xaloc", "#1A9BEE", "música")
        val restore = androidx.compose.ui.test.junit4.StateRestorationTester(ui)
        restore.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { WindConversation(vm, "ca", wind, onBack = {}) }
            }
        }
        ui.onNodeWithText(tr("ca", "app.company.windChatHint", "Xaloc")).performTextInput("Un esborrany sense enviar")
        restore.emulateSavedInstanceStateRestore()
        ui.onNodeWithText("Un esborrany sense enviar").assertExists()
    }

    @Test fun homeHasSearchAndCreationWithoutTechnicalSubtitle() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { VentsHome(vm, "ca", {}) {} }
            }
        }
        ui.onNodeWithText(tr("ca", "app.vents.localMode")).assertDoesNotExist()
        ui.onNodeWithContentDescription("Cerca converses i vents").assertExists().performClick()
        ui.onNodeWithText("Cerca per nom o missatge").assertIsDisplayed()
        ui.onNodeWithContentDescription("Crea").assertExists()
    }
}
