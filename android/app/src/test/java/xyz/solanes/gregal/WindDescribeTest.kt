package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.ProgressBarRangeInfo
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** 166: one sentence and the AI proposes the whole wind; the creator shows it and nothing is created until you tap create. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindDescribeTest {
    @get:Rule val ui = createComposeRule()

    private val reply = SuggestDto(name = "Fogó", role = "Cuina de temporada", persona = "Cuino amb calma.", style = "etn", color = "#e8a040", kit = "cooking")

    @Test fun describingAWindFillsTheCreatorAndItsPreviewAndSavesNothingUntilCreate() {
        var asked: String? = null
        var done: ((SuggestDto) -> Unit)? = null
        var saved: WindDraft? = null
        render(null, onDescribe = { d, _, ok -> asked = d; done = ok }, onSave = { draft, _ -> saved = draft })
        ui.onNodeWithText(tr("ca", "app.vents.describe")).performTextInput("un vent que m’ajudi amb receptes")
        ui.onNodeWithTag("wind-describe-go").performClick()
        ui.runOnIdle { assertEquals("un vent que m’ajudi amb receptes", asked); done!!(reply) }
        val preview = hasAnyAncestor(hasTestTag("wind-editor-preview"))
        ui.onNode(preview and hasText("Fogó")).assertExists()
        ui.onNode(preview and hasText("Cuina de temporada")).assertExists()
        ui.onNode(preview and SemanticsMatcher.expectValue(WindFaceKey, Color(Wind("", "", "#E8A040", "").argb))).assertExists()
        ui.onNode(hasText("Fogó") and hasSetTextAction()).assertExists()
        ui.onNodeWithText(tr("ca", "app.company.windShape_n")).assertIsSelected()
        ui.runOnIdle { assertNull(saved) }
        ui.onNodeWithText(tr("ca", "app.company.windSave")).performClick()
        ui.runOnIdle { assertEquals(WindDraft("Fogó", "#E8A040", "Cuina de temporada", "etn", "Cuino amb calma.", ""), saved) }
    }

    @Test fun whileItRunsTheButtonWaitsAndAFailureKeepsTheFields() {
        var fail: (() -> Unit)? = null
        render(null, onDescribe = { _, err, _ -> fail = err })
        ui.onNodeWithText(tr("ca", "app.company.windName")).performTextInput("Lupa")
        ui.onNodeWithText(tr("ca", "app.company.windRole")).performTextInput("Revisió")
        ui.onNodeWithContentDescription(tr("ca", "app.vents.colorChoice", "2")).performClick()
        ui.onNodeWithText(tr("ca", "app.vents.describe")).performTextInput("un vent que revisi textos")
        ui.onNodeWithTag("wind-describe-go").performClick().assertIsNotEnabled()
        ui.onNode(hasAnyAncestor(hasTestTag("wind-describe-go")) and hasProgressBarRangeInfo(ProgressBarRangeInfo.Indeterminate), useUnmergedTree = true).assertExists()
        ui.runOnIdle { fail!!() }
        ui.onNodeWithText(tr("ca", "app.vents.suggestError")).assertIsDisplayed()
        ui.onNode(hasText("Lupa") and hasSetTextAction()).assertExists()
        ui.onNode(hasText("Revisió") and hasSetTextAction()).assertExists()
        ui.onNodeWithContentDescription(tr("ca", "app.vents.colorChoice", "2")).assertIsSelected()
        ui.onNodeWithTag("wind-describe-go").assertIsEnabled()
    }

    @Test fun editingAWindHasNoDescriptionAndTheFieldSuggestStillWorks() {
        val original = Wind("brisa", "Brisa", "#123456", "lectura", "bto", "Persona original.")
        var asked: Pair<String, String>? = null
        var done: ((String, String) -> Unit)? = null
        render(original, onSuggest = { name, role, _, ok -> asked = name to role; done = ok })
        ui.onNodeWithText(tr("ca", "app.vents.describe")).assertDoesNotExist()
        ui.onNodeWithTag("wind-describe-go").assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.vents.personality")).performScrollTo().performClick()
        ui.onNodeWithText(tr("ca", "app.company.windSuggest")).performScrollTo().performClick()
        ui.runOnIdle { assertEquals("Brisa" to "lectura", asked); done!!("Persona de la IA.", "esn") }
        ui.onNodeWithText("Persona de la IA.").assertExists()
    }

    @Test fun theDescriptionStringsExistInCatalanAndEnglish() {
        for (key in listOf("app.vents.describe", "app.vents.describeHint", "app.vents.describeGo")) {
            assertNotNull(key, STR_APP_CA[key])
            assertNotNull(key, STR_APP_EN[key])
            assertNotEquals(key, STR_APP_CA[key], STR_APP_EN[key])
        }
    }

    @Test fun theApiSendsTheDescriptionAndReadsTheWholeWind() {
        FakeGregal { """{"name":"Fogó","role":"Cuina de temporada","persona":"Cuino amb calma.","style":"etn","color":"#e8a040","kit":"cooking"}""" }.use { server ->
            val got = GregalApi("http://127.0.0.1:${server.port}", "token").describeWind("receptes de temporada")
            val req = server.requests.single()
            assertEquals("POST /api/winds/suggest", req.substringBefore(" {"))
            assertEquals("receptes de temporada", JSONObject(req.substringAfter("suggest ")).getString("description"))
            assertEquals(listOf("Fogó", "Cuina de temporada", "Cuino amb calma.", "etn", "#e8a040", "cooking"),
                listOf(got.name, got.role, got.persona, got.style, got.color, got.kit))
        }
    }

    private fun render(
        wind: Wind?,
        onSuggest: (String, String, () -> Unit, (String, String) -> Unit) -> Unit = { _, _, _, _ -> error("Unexpected suggestion") },
        onDescribe: (String, () -> Unit, (SuggestDto) -> Unit) -> Unit = { _, _, _ -> error("Unexpected description") },
        onSave: (WindDraft, (Boolean) -> Unit) -> Unit = { _, _ -> error("Unexpected save") },
    ) {
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { WindEditor("ca", wind, {}, onSuggest, onSave, onDescribe = onDescribe) }
            }
        }
    }
}
