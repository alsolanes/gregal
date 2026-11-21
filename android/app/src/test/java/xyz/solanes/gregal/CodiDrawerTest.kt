package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
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

/**
 * El calaix de Codi té un sol lloc per a cada cosa: sense la secció dels vents
 * (el selector Codi | Vents ja hi porta) i una sola fila d'automatització.
 * El que només hi havia a Activitat i a Grafs s'obre des d'Automatitzacions.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h1600dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class CodiDrawerTest {
    @get:Rule val ui = createComposeRule()

    private val app get() = ApplicationProvider.getApplicationContext<Application>()

    // Un servidor que no respon: les pantalles carreguen i no hi arriba res de real.
    private fun vm(local: Boolean = false): GregalViewModel = GregalViewModel(app).also {
        it.attachPrefs(Prefs(app))
        it.setSimpleVents(false)
        it.setIdioma("ca")
        it.setConn(Conn("127.0.0.1", "9", "", local = local))
    }

    private fun drawer(vm: GregalViewModel) {
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    val screen by vm.screen.collectAsState()
                    GregalDrawerContent(vm, "ca", screen, true, "", "", "", nav = { vm.go(it) }, close = {})
                }
            }
        }
    }

    private fun rowLabels(): List<String> =
        ui.onAllNodes(hasClickAction() and SemanticsMatcher.keyIsDefined(SemanticsProperties.Selected))
            .fetchSemanticsNodes().map { n ->
                n.config.getOrNull(SemanticsProperties.Text)?.joinToString("") { it.text }.orEmpty()
            }

    @Test fun theCodiDrawerHasOneRowPerPlaceAndNoWindsSection() {
        drawer(vm())
        // Les files i les dues opcions del selector: «Vents» només hi ha de ser una vegada.
        val labels = rowLabels()
        assertEquals("cap fila repetida: $labels", labels.distinct(), labels)
        ui.onNodeWithText("Els teus vents").assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.nav.ventet")).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "nav.activity")).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "graphs.title")).assertDoesNotExist()
        assertEquals(1, labels.count { it == tr("ca", "nav.automations") })
    }

    @Test fun notesStaysReachableFromCodiUnderTools() {
        val vm = vm()
        drawer(vm)
        ui.onNodeWithText(tr("ca", "nav.notes")).performScrollTo().performClick()
        ui.runOnIdle { assertEquals(Screen.Notes, vm.screen.value) }
    }

    // Una notificació d'un job obre Activitat: el calaix hi marca Automatitzacions.
    @Test fun automationsIsSelectedOnActivityAndGraphs() {
        val vm = vm()
        drawer(vm)
        for (s in listOf(Screen.Jobs, Screen.Flows, Screen.Automatitzacions)) {
            ui.runOnIdle { vm.go(s) }
            ui.onNodeWithText(tr("ca", "nav.automations")).assertIsSelected()
        }
    }

    private fun automations(vm: GregalViewModel) {
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    val screen by vm.screen.collectAsState()
                    when (screen) {
                        Screen.Jobs -> JobsScreen(vm) {}
                        Screen.Flows -> FlowsScreen(vm) {}
                        else -> AutomatitzacionsScreen(vm) {}
                    }
                }
            }
        }
        ui.runOnIdle { vm.go(Screen.Automatitzacions) }
    }

    @Test fun theRunResultsAreReachableFromAutomations() {
        val vm = vm(local = true)
        automations(vm)
        ui.onNodeWithContentDescription(tr("ca", "nav.activity")).performClick()
        ui.runOnIdle { assertEquals(Screen.Jobs, vm.screen.value) }
        ui.onNodeWithText(tr("ca", "app.jobs.tabResults")).assertExists()
    }

    @Test fun theGraphsAreReachableFromAutomations() {
        val vm = vm()
        automations(vm)
        ui.onNodeWithContentDescription(tr("ca", "graphs.title")).performClick()
        ui.runOnIdle { assertEquals(Screen.Flows, vm.screen.value) }
    }
}
