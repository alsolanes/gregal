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
import org.robolectric.annotation.LooperMode

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class PrefsPagesUiTest {
    @get:Rule val ui = createComposeRule()

    private fun host(simple: Boolean = false): GregalViewModel {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        vm.setSimpleVents(simple)
        vm.openPreferences()
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) { PrefsScreen(vm) {} }
            }
        }
        return vm
    }

    @Test fun settingsIndexRoutesToConnectionsWithoutShowingCredentialForms() {
        val vm = host(simple = true)
        ui.onNodeWithText(PrefsPage.Permissions.title(vm.idioma.value)).assertDoesNotExist()
        // This host uses the local server, whose owner is an administrator.
        ui.onNodeWithText(PrefsPage.Server.title(vm.idioma.value)).assertExists()
        ui.onAllNodes(hasSetTextAction()).assertCountEquals(0)
        ui.onNodeWithText(PrefsPage.Connections.title(vm.idioma.value)).performClick()
        ui.onNodeWithText(PrefsPage.CalendarMail.title(vm.idioma.value)).performClick()
        ui.onNodeWithText("Google Calendar").assertIsDisplayed()
        ui.onNodeWithText("Gmail").assertIsDisplayed()
        ui.onAllNodes(hasSetTextAction()).assertCountEquals(0)
        ui.runOnIdle { vm.back() }
        ui.onNodeWithText(PrefsPage.Accounts.title(vm.idioma.value)).assertIsDisplayed()
    }

    @Test fun themeChoiceStillUpdatesTheExistingPreferenceAndReturnsToAppearance() {
        val vm = host()
        ui.onNodeWithText(PrefsPage.Appearance.title(vm.idioma.value)).performClick()
        ui.onNodeWithText(PrefsPage.Theme.title(vm.idioma.value)).performClick()
        ui.onNodeWithText(tr(vm.idioma.value, "prefs.theme.light")).performClick()
        ui.runOnIdle { assertEquals("clar", vm.tema.value); vm.back() }
        ui.onNodeWithText(PrefsPage.Names.title(vm.idioma.value)).assertIsDisplayed()
        ui.onNodeWithText(PrefsPage.Palette.title(vm.idioma.value)).assertIsDisplayed()
    }
}
