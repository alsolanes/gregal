package xyz.solanes.gregal

import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
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
class ModeSwitcherTest {
    @get:Rule val ui = createComposeRule()

    @Test fun labelledAreasShowSelectionAndSwitchOnTap() {
        val selected = mutableStateOf(AppArea.Vents)
        ui.setContent {
            androidx.compose.runtime.CompositionLocalProvider(LocalPal provides LightPal) {
                ModeSwitcher("en", selected.value, { selected.value = it })
            }
        }
        ui.onNodeWithText("Winds").assertIsSelected()
        ui.onNodeWithText("Code").performClick()
        ui.runOnIdle { assertEquals(AppArea.Code, selected.value) }
        ui.onNodeWithText("Code").assertIsSelected()
    }
}
