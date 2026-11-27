package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** 182: les converses recents, a dalt del calaix de Vents, com `renderThreads` a la web. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class DrawerConversationsTest {
    @get:Rule val ui = createComposeRule()

    // Per ordre d'antiguitat: Aure és la conversa més antiga i Garbí la més nova dels vents.
    private val winds = listOf(
        Wind("aure", "Aure", "#29ADFF", "cuina"),
        Wind("bora", "Bora", "#EFA68F", "música"),
        Wind("cers", "Cers", "#8FD18F", "viatges"),
        Wind("dalt", "Dalt", "#C08FEF", "salut"),
        Wind("estiu", "Estiu", "#EFD98F", "llibres"),
        Wind("fosc", "Fosc", "#8FC0EF", "cinema"),
        Wind("garbi", "Garbí", "#EF8FB0", "jardí"),
    )

    /** [talked]: els vents amb conversa; [group]: el grup «Sopar de dissabte», el més nou de tots. */
    private fun drawer(talked: List<Wind> = winds, group: Boolean = true, open: String? = null): GregalViewModel {
        val app = ApplicationProvider.getApplicationContext<Application>()
        val vm = GregalViewModel(app)
        vm.attachPrefs(Prefs(app))
        vm.setSimpleVents(true)
        vm.go(Screen.Company)
        fun <T> flow(name: String) = GregalViewModel::class.java.getDeclaredField(name).apply { isAccessible = true }.get(vm) as MutableStateFlow<T>
        flow<List<Wind>>("_winds").value = winds
        flow<Map<String, List<GregalViewModel.PetMsg>>>("_windChat").value = talked.associate { w ->
            w.id to listOf(GregalViewModel.PetMsg("user", "hola " + w.name, ts = 1_000L + winds.indexOf(w)))
        }
        if (group) {
            flow<List<Rotllana>>("_rotllanes").value = listOf(Rotllana("sopar", "Sopar de dissabte", listOf("aure", "bora")))
            flow<Map<String, List<GregalViewModel.PetMsg>>>("_rotllanaChat").value =
                mapOf("sopar" to listOf(GregalViewModel.PetMsg("user", "qui porta el vi?", ts = 2_000L)))
        }
        open?.let { vm.openVentsChat(it) }
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    val screen by vm.screen.collectAsState()
                    GregalDrawerContent(vm, "ca", screen, true, "", "", "", nav = { vm.go(it) }, close = {})
                }
            }
        }
        return vm
    }

    private fun top(text: String) = ui.onNodeWithText(text).fetchSemanticsNode().positionInRoot.y

    private fun assertOrder(vararg order: String) {
        val tops = order.map(::top)
        for (i in 1 until order.size) assertTrue("${order[i - 1]} above ${order[i]}: $tops", tops[i - 1] < tops[i])
    }

    @Test fun `the five newest conversations come first, then Show more and All conversations`() {
        drawer()
        assertOrder("Converses", "Sopar de dissabte", "Garbí", "Fosc", "Estiu", "Dalt", "Mostra’n més (3)", "Totes les converses", "Nou especialista")
        listOf("Cers", "Bora", "Aure", "Xats").forEach { ui.onNodeWithText(it).assertDoesNotExist() }
        // La cara del vent, com a les files dels vents recents, dins la fila de la conversa.
        val row = ui.onNodeWithText("Estiu").fetchSemanticsNode().boundsInRoot
        val faces = ui.onAllNodes(SemanticsMatcher.expectValue(WindFaceKey, Color(winds[4].argb)), useUnmergedTree = true)
            .fetchSemanticsNodes().map { it.boundsInRoot }
        assertTrue("$row $faces", faces.any { it.top >= row.top && it.bottom <= row.bottom })
        // Un grup no porta la cara de cap dels seus vents.
        val group = ui.onNodeWithText("Sopar de dissabte").fetchSemanticsNode().boundsInRoot
        val anyFace = ui.onAllNodes(SemanticsMatcher.keyIsDefined(WindFaceKey), useUnmergedTree = true).fetchSemanticsNodes().map { it.boundsInRoot }
        assertTrue("$group $anyFace", anyFace.none { it.top >= group.top && it.bottom <= group.bottom })
    }

    @Test fun `Show more unfolds the rest in place and Show less folds them again`() {
        drawer()
        ui.onNodeWithText("Mostra’n més (3)").performClick()
        assertOrder("Dalt", "Cers", "Bora", "Aure", "Mostra’n menys", "Totes les converses")
        ui.onNodeWithText("Mostra’n menys").performClick()
        ui.onNodeWithText("Aure").assertDoesNotExist()
        ui.onNodeWithText("Mostra’n més (3)").assertExists()
    }

    @Test fun `the open conversation stays visible even when it is older than the five`() {
        drawer(open = "wind:aure")
        assertOrder("Dalt", "Aure", "Mostra’n més (2)", "Totes les converses")
        ui.onNodeWithText("Aure").assertIsSelected()
        ui.onNodeWithText("Cers").assertDoesNotExist()
        // Només una fila marcada: ni la de Gregal ni cap altra (el selector Vents | Codi és a part).
        ui.onAllNodes(isSelected() and !SemanticsMatcher.expectValue(SemanticsProperties.Role, Role.Tab)).assertCountEquals(1)
    }

    @Test fun `with five or fewer there is nothing to show more`() {
        drawer(talked = winds.takeLast(5), group = false)
        assertOrder("Garbí", "Fosc", "Estiu", "Dalt", "Cers", "Totes les converses")
        ui.onNodeWithText("Mostra’n", substring = true).assertDoesNotExist()
    }

    @Test fun `tapping a conversation opens that conversation`() {
        val vm = drawer()
        ui.onNodeWithText("Sopar de dissabte").performClick()
        ui.runOnIdle { assertEquals("rot:sopar", vm.ventsOpen.value) }
        ui.onNodeWithText("Dalt").performClick()
        ui.runOnIdle { assertEquals("wind:dalt", vm.ventsOpen.value) }
    }

    @Test fun `with no conversations only All conversations is left`() {
        val vm = drawer(talked = emptyList(), group = false)
        winds.forEach { ui.onNodeWithText(it.name).assertDoesNotExist() }
        assertOrder("Converses", "Totes les converses")
        ui.onNodeWithText("Mostra’n", substring = true).assertDoesNotExist()
        ui.onNodeWithText("Totes les converses").performClick()
        ui.runOnIdle { assertEquals("chats", vm.ventsPanel.value) }
    }

    @Test fun `the rows are named in both languages`() {
        assertEquals("Totes les converses", tr("ca", "app.drawer.allConversations"))
        assertEquals("All conversations", tr("en", "app.drawer.allConversations"))
        assertEquals("Mostra’n més (3)", tr("ca", "app.drawer.showMore", 3))
        assertEquals("Show more (3)", tr("en", "app.drawer.showMore", 3))
        assertEquals("Mostra’n menys", tr("ca", "app.drawer.showLess"))
        assertEquals("Show less", tr("en", "app.drawer.showLess"))
    }
}
