package xyz.solanes.gregal

import android.app.Application
import android.content.Context
import android.content.ContextWrapper
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.SemanticsProperties
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
class DrawerModeSwitchTest {
    @get:Rule val ui = createComposeRule()

    private val app get() = ApplicationProvider.getApplicationContext<Application>()

    private fun packaged(name: String): Context = object : ContextWrapper(app) {
        override fun getPackageName() = name
    }

    // The drawer rows are selectable too; the switch options are tabs.
    private fun option(key: String) =
        hasText(tr("ca", key)) and SemanticsMatcher.expectValue(SemanticsProperties.Role, Role.Tab)

    private fun host(simple: Boolean): GregalViewModel {
        val vm = GregalViewModel(app)
        vm.attachPrefs(Prefs(app))
        vm.setSimpleVents(simple)
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    val screen by vm.screen.collectAsState()
                    GregalDrawerContent(vm, "ca", screen, true, "", "", "", nav = { vm.go(it) }, close = {})
                }
            }
        }
        return vm
    }

    @Test fun choosingVentsShowsTheWindsDrawerAndOpensTheWinds() {
        val vm = host(simple = false)
        ui.onNodeWithText("Gregal Vents").assertDoesNotExist()
        ui.onNode(option("app.mode.vents")).assertIsNotSelected().performClick()
        ui.onNodeWithText("Gregal Vents").assertExists()
        ui.onNode(option("app.mode.vents")).assertIsSelected()
        ui.runOnIdle {
            assertEquals(true, vm.simpleVents.value)
            assertEquals(Screen.Company, vm.screen.value)
        }
    }

    @Test fun choosingCodiShowsTheFullDrawerAndOpensTheCodeChat() {
        val vm = host(simple = true)
        ui.runOnIdle { vm.go(Screen.Company) }
        ui.onNodeWithText(tr("ca", "app.nav.sectionAgent")).assertDoesNotExist()
        ui.onNode(option("app.mode.code")).assertIsNotSelected().performClick()
        ui.onNodeWithText(tr("ca", "app.nav.sectionAgent")).assertExists()
        ui.runOnIdle {
            assertEquals(false, vm.simpleVents.value)
            assertEquals(Screen.Chat, vm.screen.value)
        }
    }

    // Enrere després de canviar de mode no ha de tornar a una pantalla de l'altre
    // mode amb el calaix del mode nou.
    @Test fun switchingModeStartsAFreshBackStack() {
        val vm = host(simple = false)
        ui.runOnIdle { vm.go(Screen.Sessions) }
        ui.onNode(option("app.mode.vents")).performClick()
        ui.runOnIdle {
            assertEquals(Screen.Company, vm.screen.value)
            assertEquals(emptyList<Screen>(), vm.backStack.value)
        }
    }

    // Vist a l'emulador: a Codi, Enrere des del xat (l'arrel) portava als vents.
    @Test fun backInCodiReturnsToTheCodeChatNotToTheWinds() {
        val vm = host(simple = true)
        ui.onNode(option("app.mode.code")).performClick()
        ui.runOnIdle {
            assertEquals(false, vm.back())
            assertEquals(Screen.Chat, vm.screen.value)
            vm.go(Screen.Sessions)
            vm.resetBackStackForTest()
            assertEquals(true, vm.back())
            assertEquals(Screen.Chat, vm.screen.value)
        }
    }

    @Test fun theChosenModeIsSavedAndLoadedAgain() {
        host(simple = false)
        ui.onNode(option("app.mode.vents")).performClick()
        ui.runOnIdle {
            val again = GregalViewModel(app)
            again.attachPrefs(Prefs(app))
            assertEquals(true, again.simpleVents.value)
        }
    }

    @Test fun withoutAChoiceEachAppStartsInItsOwnMode() {
        assertEquals(true, Prefs(packaged("xyz.solanes.gregal.vents")).simpleVents)
        assertEquals(false, Prefs(packaged("xyz.solanes.gregal")).simpleVents)
    }

    @Test fun theSwitchSaysCodeAndWindsInEnglish() {
        assertEquals("Codi", tr("ca", "app.mode.code"))
        assertEquals("Vents", tr("ca", "app.mode.vents"))
        assertEquals("Code", tr("en", "app.mode.code"))
        assertEquals("Winds", tr("en", "app.mode.vents"))
    }
}
