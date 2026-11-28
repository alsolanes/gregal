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

/** The setup's welcome: the winds tell what Gregal is, then they ask for the server. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
class FolkTourTest {
    @get:Rule val ui = createComposeRule()

    @Test fun `the tour tells four things and Next ends on Let's go`() {
        var done = 0
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = DarkPal.scheme()) { FolkTour("ca", onDone = { done++ }) }
            }
        }
        ui.onNodeWithText(tr("ca", "app.tour.work.title")).assertExists()
        assertEquals("a little app window shows it live", true, ui.onAllNodesWithTag("folk-window").fetchSemanticsNodes().isNotEmpty())
        repeat(3) { ui.onNodeWithTag("folk-tour-next").performClick(); ui.waitForIdle() }
        ui.onNodeWithText(tr("ca", "app.tour.home.title")).assertExists()
        ui.onNodeWithText(tr("ca", "app.tour.start")).performClick()
        assertEquals(1, done)
    }

    @Test fun `without motion the setup skips the film, and after the tour the winds ask for the server`() {
        val app = ApplicationProvider.getApplicationContext<Application>()
        val vm = GregalViewModel(app)
        val prefs = Prefs(app)
        vm.attachPrefs(prefs)
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = DarkPal.scheme()) { PairingScreen(vm, prefs) }
            }
        }
        ui.onNodeWithTag("folk-intro").assertExists()
        ui.onNodeWithTag("folk-tour").assertExists()
        ui.onNodeWithTag("folk-tour-skip").performClick()
        ui.waitForIdle()
        ui.onNodeWithTag("folk-tour").assertDoesNotExist()
        ui.onNodeWithText(tr(vm.idioma.value, "app.pair.say.hello")).assertExists()
    }
}
