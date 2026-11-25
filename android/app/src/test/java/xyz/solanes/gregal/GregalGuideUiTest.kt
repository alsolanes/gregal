package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** 153: com al web (136), la guia s'obre amb tres targetes i «Més» hi afegeix la resta al lloc. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
class GregalGuideUiTest {
    @get:Rule val ui = createComposeRule()

    private fun show(lang: String) = ui.setContent {
        CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { GregalGuide(lang, Color(0xFF5AA9E6), onTry = {}, onClose = {}) }
        }
    }

    @Test fun theGuideOpensWithThreeCardsAndMoreAddsTheRest() {
        show("ca")
        ui.onAllNodesWithTag("guide-card", useUnmergedTree = true).assertCountEquals(3)
        ui.onNodeWithText("Més").performScrollTo().performClick()
        ui.onAllNodesWithTag("guide-card", useUnmergedTree = true).assertCountEquals(guideTopics("ca").size)
        ui.onNodeWithTag("guide-more").assertDoesNotExist()
    }

    @Test fun moreSpeaksEnglishToo() {
        show("en")
        ui.onNodeWithText("More").assertExists()
    }
}
