package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.ui.Modifier
import androidx.compose.runtime.*
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Assert.*
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
class ModelPickerHostUiTest {
    @get:Rule val ui = createComposeRule()
    @Composable private fun HostTheme(content: @Composable () -> Unit) {
        CompositionLocalProvider(LocalPal provides LightPal) {
            MaterialTheme(colorScheme = LightPal.scheme()) {
                androidx.compose.material3.Surface(Modifier.fillMaxSize(), content = content)
            }
        }
    }
    private fun snapshot(name: String) {
        ui.waitForIdle()
        ui.runOnIdle {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).single()
            val view = activity.window.decorView
            val bitmap = android.graphics.Bitmap.createBitmap(view.width, view.height, android.graphics.Bitmap.Config.ARGB_8888)
            view.draw(android.graphics.Canvas(bitmap))
            java.io.File("build/picker-ui", name).also { it.parentFile!!.mkdirs() }.outputStream().use {
                bitmap.compress(android.graphics.Bitmap.CompressFormat.PNG, 100, it)
            }
        }
    }
    @Test fun searchableGroupedModelsRouteToTheirOwnProviderWithoutCredentialFields() {
        var selected = ""
        ui.setContent {
            HostTheme {
                ModelPickerContent("en", listOf(
                    ProviderInfo(name = "alpha", key = "sí"), ProviderInfo(name = "beta", key = "sí"),
                    ProviderInfo(name = "unconfigured", key = "${'$'}{DUMMY_TEST_ENV}"),
                    ProviderInfo(name = "local", url = "http://localhost:8080")),
                    mapOf("alpha" to (1..25).map { "model-$it" }, "beta" to listOf("other/model"),
                        "unconfigured" to listOf("public-model"), "local" to listOf("local-model")),
                    ServerState(provider = "alpha", model = "model-1"),
                    onSelect = { selected = it }, onSettings = {})
            }
        }
        snapshot("configured-models-en.png")
        ui.onNodeWithText("public-model").assertDoesNotExist()
        ui.onNodeWithText("local-model").assertDoesNotExist()
        ui.onNodeWithText(tr("en", "app.menu.keyPlaceholder")).assertDoesNotExist()
        ui.onNodeWithTag("model-search").performTextInput("model-25")
        ui.onNode(hasText("model-25") and !hasSetTextAction()).assertExists().performClick()
        assertEquals("alpha/model-25", selected)
        ui.onNodeWithTag("model-search").performTextReplacement("beta")
        ui.onNodeWithText("other/model").performClick()
        assertEquals("beta/other/model", selected)
    }
    @Test fun emptyConfigurationOffersSettingsNotKeyEntry() {
        var settings = false
        ui.setContent { HostTheme { ModelPickerContent("en", emptyList(), emptyMap(), ServerState(), {}, { settings = true }) } }
        snapshot("empty-config-settings-en.png")
        ui.onNodeWithTag("provider-settings-cta").performClick()
        assertTrue(settings)
        ui.onAllNodes(hasSetTextAction()).assertCountEquals(0)
    }
    @Test fun pickerFitsTheRealDropdownMenuContainer() {
        ui.setContent { HostTheme {
            androidx.compose.material3.DropdownMenu(true, {}) {
                ModelPickerContent("ca", listOf(ProviderInfo(name = "configured", key = "sí")),
                    mapOf("configured" to listOf("model")), ServerState(), {}, {})
            }
        } }
        ui.onNodeWithText("model").assertExists()
    }
    @Test fun catalanDeviceShowsActualSettingsInCatalanWithSystemChoice() {
        val app = androidx.test.core.app.ApplicationProvider.getApplicationContext<android.app.Application>()
        app.getSharedPreferences("gregal", 0).edit().clear().commit()
        val config = android.content.res.Configuration(app.resources.configuration)
        config.setLocales(android.os.LocaleList.forLanguageTags("ca-AD"))
        val vm = GregalViewModel(app)
        vm.attachPrefs(Prefs(app.createConfigurationContext(config)))
        vm.setConn(Conn("127.0.0.1", "1", ""))
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) { PrefsScreen(vm) {} }
            }
        }
        ui.onNodeWithText(tr("ca", "prefs.title")).assertExists()
        snapshot("settings-device-ca.png")
        ui.onAllNodesWithText("Sistema")[1].performScrollTo().assertIsSelected()
        ui.onNodeWithText("English").performScrollTo().performClick()
        ui.onNodeWithText(tr("en", "prefs.title")).assertExists()
        snapshot("settings-manual-en.png")
        assertEquals("en", Prefs(app).idiomaChoice)
    }
    @Suppress("UNCHECKED_CAST")
    @Test fun settingsOffersExistingCustomProviderForKeyEditingAndTesting() {
        val app = androidx.test.core.app.ApplicationProvider.getApplicationContext<android.app.Application>()
        val vm = GregalViewModel(app)
        vm.attachPrefs(Prefs(app))
        vm.setConn(Conn("127.0.0.1", "1", ""))
        val field = vm.javaClass.getDeclaredField("_providers").apply { isAccessible = true }
        (field.get(vm) as kotlinx.coroutines.flow.MutableStateFlow<List<ProviderInfo>>).value = listOf(
            ProviderInfo(name = "cloud", url = "https://example.invalid/v1", key = "sí"),
            ProviderInfo(name = "local", url = "http://localhost:8080"))
        ui.setContent { HostTheme { ProviderSetup(vm, current = "cloud", change = true) } }
        ui.onNodeWithText("cloud").assertExists().performClick()
        ui.onNodeWithText("local").assertExists().performClick()
        ui.onNodeWithText(tr(vm.idioma.value, "app.menu.test")).assertIsEnabled()
        ui.onNodeWithText(tr(vm.idioma.value, "app.menu.keyPlaceholder")).assertExists()
        ui.onNodeWithText(tr(vm.idioma.value, "app.menu.keyMemory")).assertExists()
    }
    @Test fun settingsRetainsCredentialEditor() {
        ui.setContent { HostTheme { ProviderKeyField("en", "", {}) } }
        ui.onNodeWithText(tr("en", "app.menu.keyPlaceholder")).assertExists()
        ui.onAllNodes(hasSetTextAction()).assertCountEquals(1)
    }
}
