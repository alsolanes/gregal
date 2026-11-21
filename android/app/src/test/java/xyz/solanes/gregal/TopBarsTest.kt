package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.assertTextEquals
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/**
 * Totes les pantalles dels dos calaixos fan servir la mateixa capçalera
 * (`GregalTopBar`): el mateix títol, la mateixa mida. Excepcions: els xats
 * (Codi i la conversa amb un vent), amb capçalera pròpia.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h1600dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class TopBarsTest {
    @get:Rule val ui = createComposeRule()

    private val app get() = ApplicationProvider.getApplicationContext<Application>()

    // Un servidor que no respon: les pantalles carreguen i no hi arriba res de real.
    private fun vm(): GregalViewModel = GregalViewModel(app).also {
        it.attachPrefs(Prefs(app))
        it.setSimpleVents(false)
        it.setIdioma("ca")
        it.setConn(Conn("127.0.0.1", "9", "", local = true))
    }

    private fun show(content: @Composable () -> Unit) = ui.setContent {
        CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { GregalBg { content() } }
        }
    }

    @Test fun everyDrawerScreenUsesTheSharedTopBar() {
        val vm = vm()
        val screens: List<Pair<String, @Composable () -> Unit>> = listOf(
            tr("ca", "app.work.title") to { WorkScreen(vm) {} },
            tr("ca", "nav.convs") to { SessionsScreen(vm) {} },
            "Backlog" to { BacklogScreen(vm) {} },
            tr("ca", "act.title") to { AutomatitzacionsScreen(vm) {} },
            tr("ca", "nav.activity") to { JobsScreen(vm) {} },
            tr("ca", "graphs.title") to { FlowsScreen(vm) {} },
            tr("ca", "app.carpeta.title") to { CarpetaScreen(vm) {} },
            tr("ca", "notes.title") to { NotesScreen(vm) {} },
            webCopy("ca", "title") to { WebsScreen(vm) {} },
            tr("ca", "saved.title") to { DesatScreen(vm) {} },
            tr("ca", "app.off.title") to { OfficeScreen(vm, {}) {} },
            tr("ca", "app.retro.title") to { RetroScreen(vm, {}) {} },
            tr("ca", "prefs.title") to { PrefsScreen(vm) {} },
            // Els vents sense servidor: la llista, no el fil de Gregal.
            tr("ca", "app.vents.title") to { CompanyScreen(vm) {} },
        )
        var current by mutableStateOf(0)
        show { screens[current].second() }
        screens.forEachIndexed { i, (title, _) ->
            ui.runOnIdle { current = i }
            ui.onNodeWithTag(GregalTopBarTitle).assertTextEquals(title)
        }
    }

    // Dins de Notes: la llista, la nota àgil, una nota oberta i l'editor.
    @Test fun everyNotesPageUsesTheSharedTopBar() {
        val vm = vm()
        show { NotesScreen(vm) {} }
        ui.onNodeWithTag(GregalTopBarTitle).assertTextEquals(tr("ca", "notes.title"))
        ui.onNodeWithText(tr("ca", "notes.new"), useUnmergedTree = true).performClick()
        ui.onNodeWithTag("agile-body").assertExists()
        ui.onNodeWithTag(GregalTopBarTitle).assertTextEquals(tr("ca", "notes.title"))
        val open = GregalViewModel::class.java.getDeclaredField("_openNote").apply { isAccessible = true }.get(vm)
            as MutableStateFlow<NoteDetail?>
        ui.runOnIdle { open.value = NoteDetail(Note("n1", "Llista de la compra"), emptyList()) }
        ui.onNodeWithTag(GregalTopBarTitle).assertTextEquals("Llista de la compra")
        ui.onNodeWithContentDescription(tr("ca", "notes.edit")).performClick()
        ui.onNodeWithTag(GregalTopBarTitle).assertTextEquals(tr("ca", "notes.edit"))
    }
}
