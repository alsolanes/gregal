package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.junit4.StateRestorationTester
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode
import java.io.File

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class PlotTypesHostUiTest {
    @get:Rule val ui = createComposeRule()
    private fun mount(type: String): StateRestorationTester {
        val values = if (type == "area" || type == "horizontal-bar") "[-2,0,3]" else "[1,2,3]"
        val extra = if (type == "horizontal-bar") ",{\"label\":\"T\",\"values\":[1,-3,2]}" else ""
        val spec = """{"title":"Native $type","type":"$type","labels":["A","B","C"],"series":[{"label":"S","values":$values}$extra]}"""
        return StateRestorationTester(ui).also { restoration ->
            restoration.setContent {
                CompositionLocalProvider(LocalPal provides LightPal) {
                    MaterialTheme(colorScheme = LightPal.scheme()) {
                        Surface(Modifier.fillMaxSize()) {
                            Column(Modifier.verticalScroll(rememberScrollState())) {
                                CodeBlock("gregal-plot", spec, "ca")
                            }
                        }
                    }
                }
            }
        }
    }
    private fun capture(type: String) {
        ui.waitForIdle()
        val bounds = ui.onNodeWithContentDescription("Native $type · $type · 1–3 / 3").fetchSemanticsNode().boundsInRoot
        ui.runOnIdle {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).single()
            val view = activity.window.decorView
            val image = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
            view.draw(android.graphics.Canvas(image))
            if (type == "area" || type == "horizontal-bar") {
                var colored = 0
                for (y in bounds.top.toInt().coerceAtLeast(0) until bounds.bottom.toInt().coerceAtMost(image.height)) {
                    for (x in bounds.left.toInt().coerceAtLeast(0) until bounds.right.toInt().coerceAtMost(image.width)) {
                        val pixel = image.getPixel(x, y)
                        // Count the saturated blue accent/fill, not neutral grid/background.
                        if (android.graphics.Color.blue(pixel) - android.graphics.Color.red(pixel) > 30 &&
                            android.graphics.Color.blue(pixel) - android.graphics.Color.green(pixel) > 25) colored++
                    }
                }
                org.junit.Assert.assertTrue("$type must render filled geometry, got $colored pixels", colored > 1500)
            }
            File("build/plots-ui/plots-$type.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
                image.compress(Bitmap.CompressFormat.PNG, 100, it)
            }
        }
    }
    private fun circular(type: String) {
        val restoration = mount(type)
        ui.onNodeWithText("+").assertIsNotEnabled()
        ui.onNodeWithText("−").assertIsNotEnabled()
        val chart = ui.onNodeWithContentDescription("Native $type · $type · 1–3 / 3").performScrollTo()
        chart.performTouchInput { click(Offset(width * 0.62f, height * 0.2f)) }
        ui.onNodeWithText("S: 1.0").assertExists()
        ui.onNodeWithText("1 · A").performScrollTo().performClick().assertIsNotSelected()
        ui.onNodeWithText("S: 1.0").assertDoesNotExist()
        restoration.emulateSavedInstanceStateRestore()
        ui.onNodeWithText("1 · A").assertIsNotSelected()
        ui.onNodeWithText("Següent").performScrollTo().performClick()
        ui.onNodeWithText("S: 2.0").assertExists()
        capture(type)
        ui.onNodeWithText("Amplia").performScrollTo().performClick()
        ui.onNodeWithText("Tanca").assertExists().performClick()
        ui.onNodeWithText("Amplia").performScrollTo().performClick()
        ui.runOnIdle {
            val dialog = org.robolectric.shadows.ShadowDialog.getLatestDialog()
            dialog.dispatchKeyEvent(android.view.KeyEvent(android.view.KeyEvent.ACTION_DOWN, android.view.KeyEvent.KEYCODE_BACK))
            dialog.dispatchKeyEvent(android.view.KeyEvent(android.view.KeyEvent.ACTION_UP, android.view.KeyEvent.KEYCODE_BACK))
        }
        ui.waitForIdle()
        ui.onNodeWithText("Tanca").assertDoesNotExist()
        ui.onNodeWithText("S: 2.0").assertExists()
    }
    @Test fun pieTogglesCategoriesAndRestoresSelection() = circular("pie")
    @Test fun doughnutTogglesCategoriesAndRestoresSelection() = circular("doughnut")
    @Test fun radarKeepsFullDomainAndSeriesToggle() {
        mount("radar")
        ui.onNodeWithText("+").assertIsNotEnabled()
        ui.onNodeWithText("Següent").performScrollTo().performClick()
        ui.onNodeWithText("S: 1.0").assertExists()
        capture("radar")
        ui.onNodeWithText("1 · S").performScrollTo().performClick().assertIsNotSelected()
        ui.onNodeWithText("S: 1.0").assertDoesNotExist()
    }
    private fun cartesian(type: String) {
        mount(type)
        ui.onNodeWithText("Següent").performScrollTo().performClick()
        ui.onNodeWithText("S: -2.0").assertExists()
        capture(type)
        ui.onNodeWithText("+").performScrollTo().performClick()
        ui.onNodeWithContentDescription("Native $type · $type · 1–2 / 3").assertExists()
    }
    @Test fun areaSupportsSelectionAndZoom() = cartesian("area")
    @Test fun horizontalBarSupportsSelectionAndZoom() = cartesian("horizontal-bar")
}
