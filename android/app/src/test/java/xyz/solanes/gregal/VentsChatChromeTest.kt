package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.*
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.text.input.TextFieldValue
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
class VentsChatChromeTest {
    @get:Rule val ui = createComposeRule()

    @Test fun composerRetainsDraftAndOnlySendsNonBlankText() {
        var sent = ""
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    var field by remember { mutableStateOf(TextFieldValue("")) }
                    VentsComposer("ca", field, { field = it }, "Escriu…", field.text.isNotBlank()) { sent = field.text }
                }
            }
        }
        ui.onNodeWithContentDescription(tr("ca", "app.vents.send")).assertIsNotEnabled()
        ui.onNodeWithText("Escriu…").performTextInput("@Xaloc una idea")
        ui.onNodeWithContentDescription(tr("ca", "app.vents.send")).assertIsEnabled().performClick()
        ui.runOnIdle { assertEquals("@Xaloc una idea", sent) }
    }

    @Test fun compactHeaderKeepsBackAndProfileAccessible() {
        var back = false
        var profile = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    VentsChatHeader("ca", "Xaloc", "Música", Color(0xFFEFA68F), WindStyle(), WindMood.IDLE,
                        onBack = { back = true }, onProfile = { profile = true })
                }
            }
        }
        ui.onNodeWithText("Xaloc").assertIsDisplayed()
        ui.onNodeWithContentDescription(tr("ca", "app.vents.profile")).performClick()
        ui.onNodeWithContentDescription(tr("ca", "app.vents.back")).performClick()
        ui.runOnIdle { assertTrue(profile); assertTrue(back) }
    }
}
