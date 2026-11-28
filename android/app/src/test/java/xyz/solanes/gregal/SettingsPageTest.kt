package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** 182: al calaix de Vents les converses van a dalt i «Configuració» és una pàgina, no un desplegable. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class SettingsPageTest {
    @get:Rule val ui = createComposeRule()

    private val app get() = ApplicationProvider.getApplicationContext<Application>()

    private fun vm(): GregalViewModel {
        val vm = GregalViewModel(app)
        vm.attachPrefs(Prefs(app))
        vm.setSimpleVents(true)
        vm.go(Screen.Company)
        vm.resetBackStackForTest()
        return vm
    }

    private fun drawer(lang: String): GregalViewModel {
        val vm = vm()
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    val screen by vm.screen.collectAsState()
                    GregalDrawerContent(vm, lang, screen, true, "", "", "", nav = { vm.go(it) }, close = {})
                }
            }
        }
        return vm
    }

    private fun settings(lang: String): GregalViewModel {
        val vm = vm()
        vm.setIdioma(lang)
        vm.go(Screen.Settings)
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) { SettingsScreen(vm) {} }
            }
        }
        return vm
    }

    // positionInRoot: boundsInRoot is clipped to 0 for rows scrolled out of the drawer.
    private fun top(text: String) = ui.onNodeWithText(text).fetchSemanticsNode().positionInRoot.y

    @Test fun `the drawer puts the conversations first, then the winds, the other spaces and Settings`() {
        drawer("ca")
        val order = listOf("Nova conversa", "Avui", "Converses", "Totes les converses",
            "Gregal Docs", "Automatitzacions", "Desat")
        val tops = order.map(::top)
        for (i in 1 until order.size) assertTrue("${order[i - 1]} above ${order[i]}: $tops", tops[i - 1] < tops[i])
        // Configuració is pinned at the foot: scrolling to the last space leaves it below, where it was.
        val settings = top("Configuració")
        ui.onNodeWithText("Desat").performScrollTo()
        assertEquals(settings, top("Configuració"))
        assertTrue(top("Desat") < settings)
    }

    @Test fun `Settings is one row that opens the settings page instead of unfolding`() {
        val vm = drawer("ca")
        ui.onNodeWithText("Configuració i més").assertDoesNotExist()
        ui.onAllNodesWithText("Configuració").assertCountEquals(1)
        ui.onNodeWithText("Configuració").performClick()
        ui.runOnIdle { assertEquals(Screen.Settings, vm.screen.value) }
        ui.onNodeWithText(tr("ca", "prefs.title")).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.computer.name")).assertDoesNotExist()
        ui.onNodeWithText("Configuració").assertIsSelected()
    }

    @Test fun `in English the drawer row says Settings`() {
        drawer("en")
        ui.onNodeWithText("Settings & more").assertDoesNotExist()
        ui.onNodeWithText("Settings").assertExists()
    }

    @Test fun `each section of the page has a name and a line and opens what the drawer opened`() {
        val vm = settings("ca")
        fun open(title: String, about: String) {
            ui.onNodeWithText(about).assertExists()
            ui.onNodeWithText(title).performScrollTo().performClick()
        }
        open(tr("ca", "prefs.title"), tr("ca", "app.settings.prefsAbout"))
        ui.runOnIdle { assertEquals(Screen.Prefs, vm.screen.value); vm.back() }
        for ((title, about, panel) in listOf(
            Triple(tr("ca", "app.computer.name"), tr("ca", "app.settings.computerAbout"), "computer"),
            Triple(guideIntro("ca").first, tr("ca", "app.settings.guideAbout"), "guide"),
            Triple(tr("ca", "app.vents.myWinds"), tr("ca", "app.settings.myWindsAbout"), "directory"),
        )) {
            open(title, about)
            ui.runOnIdle {
                assertEquals(Screen.Company, vm.screen.value)
                assertEquals(panel, vm.ventsPanel.value)
                vm.back()
            }
        }
        open(tr("ca", "app.local.title"), tr("ca", "app.settings.localAbout"))
        ui.runOnIdle {
            assertEquals(Screen.Automatitzacions, vm.screen.value)
            assertEquals("local", vm.ventsPanel.value)
            vm.back()
        }
        ui.onNodeWithText(tr("ca", "app.nav.codeChat")).assertDoesNotExist()
        open(tr("ca", "app.nav.sectionCode"), tr("ca", "app.settings.codeAbout"))
        ui.onNodeWithText(tr("ca", "app.nav.codeChat")).performScrollTo().performClick()
        ui.runOnIdle { assertEquals(Screen.Chat, vm.screen.value) }
    }

    @Test fun `back from every section returns to the settings page`() {
        val vm = vm()
        vm.go(Screen.Settings)
        vm.openPreferences()
        assertTrue(vm.back())
        assertEquals(Screen.Settings, vm.screen.value)
        for (panel in listOf("computer", "guide", "directory")) {
            vm.openVentsPanel(panel)
            assertTrue(vm.back())
            assertEquals(panel, Screen.Settings, vm.screen.value)
            assertEquals(null, vm.ventsPanel.value)
        }
        // La portada tanca el panell sola quan el full o el directori es tanquen amb Enrere.
        vm.openVentsPanel("directory")
        vm.closeVentsPanel()
        assertEquals(Screen.Settings, vm.screen.value)
        vm.openVentsPanel("local")
        vm.go(Screen.Automatitzacions)
        assertTrue(vm.back())
        assertEquals(Screen.Settings, vm.screen.value)
        for (s in listOf(Screen.Chat, Screen.Sessions, Screen.Carpeta, Screen.Work)) {
            vm.go(s)
            assertTrue(vm.back())
            assertEquals(Screen.Settings, vm.screen.value)
        }
    }

    // Revisió: triar un vent a «Els meus vents» tanca el directori; això no és Enrere.
    @Test fun `opening a conversation from a section of the page stays in the conversation`() {
        val vm = vm()
        vm.go(Screen.Settings)
        vm.openVentsPanel("directory")
        vm.openVentsChat("wind:teca")
        vm.closeVentsPanel()
        assertEquals(Screen.Company, vm.screen.value)
        assertEquals("wind:teca", vm.ventsOpen.value)
    }

    @Test fun `a panel opened from the winds home still closes onto the winds home`() {
        val vm = vm()
        vm.openVentsPanel("directory")
        vm.closeVentsPanel()
        assertEquals(Screen.Company, vm.screen.value)
        assertEquals(null, vm.ventsPanel.value)
    }

    @Test fun `the page texts exist in Catalan and English`() {
        for (key in listOf("app.settings.title", "app.settings.prefsAbout", "app.settings.computerAbout", "app.settings.localAbout",
            "app.settings.guideAbout", "app.settings.myWindsAbout", "app.settings.codeAbout", "app.settings.codeWork")) {
            assertTrue(key, STR_APP_CA.containsKey(key) && STR_APP_EN.containsKey(key))
        }
        assertEquals("Configuració", tr("ca", "app.settings.title"))
        assertEquals("Settings", tr("en", "app.settings.title"))
    }
}
