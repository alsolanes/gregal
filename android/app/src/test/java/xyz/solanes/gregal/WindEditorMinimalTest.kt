package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
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
class WindEditorMinimalTest {
    @get:Rule val ui = createComposeRule()

    @Test fun unsavedIdentityAndAppearanceSurviveThemeRecreation() {
        val restore = androidx.compose.ui.test.junit4.StateRestorationTester(ui)
        restore.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    WindEditor("ca", null, {}, { _, _, _, _ -> }, { _, _ -> })
                }
            }
        }
        ui.onNodeWithText(tr("ca", "app.company.windName")).performTextInput("Brisa")
        ui.onNodeWithText(tr("ca", "app.company.windShape_s")).performClick()
        ui.onNodeWithContentDescription(tr("ca", "app.vents.colorChoice", "2")).performClick()
        restore.emulateSavedInstanceStateRestore()
        ui.onNodeWithText("Brisa").assertExists()
        ui.onNodeWithText(tr("ca", "app.company.windShape_s")).assertIsSelected()
        ui.onNodeWithContentDescription(tr("ca", "app.vents.colorChoice", "2")).assertIsSelected()
    }

    @Test fun solidReferenceColorsAreSelectableWithoutChangingHiddenPersona() {
        val original = Wind("solid", "Brisa", "#123456", "lectura", "pso", "Persona conservada")
        var saved: WindDraft? = null
        render(original, onSave = { draft, _ -> saved = draft })
        ui.onNodeWithContentDescription(tr("ca", "app.vents.colorChoice", "2")).performClick()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle {
            assertEquals("#1A9BEE", saved?.color)
            assertEquals(original.persona, saved?.persona)
        }
    }

    @Test fun choosingATemplateNeverDiscardsAnExistingHiddenPersona() {
        val original = Wind("same-id", "Brisa", "#123456", "lectura", "bto", "Persona pròpia que cal conservar.")
        var saved: WindDraft? = null
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    WindEditor("ca", original, {}, { _, _, _, _ -> error("Unexpected suggestion") },
                        { draft, _ -> saved = draft })
                }
            }
        }
        ui.onNodeWithText(original.persona).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.vents.changePreset")).performScrollTo().performClick()
        ui.onNodeWithText(tr("ca", "app.vents.galleryTitle")).assertIsDisplayed()
        val template = WindTemplates.all.first()
        ui.onNodeWithText(tr("ca", "app.vents.tpl.${template.id}.title")).performClick()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle {
            assertEquals(original.id, saved?.id)
            assertEquals(original.name, saved?.name)
            assertEquals(original.persona, saved?.persona)
            assertEquals(template.style, saved?.style)
            assertEquals(template.color, saved?.color)
            assertEquals(tr("ca", "app.vents.tpl.${template.id}.role"), saved?.role)
        }
    }

    @Test fun savingPreservesTheHiddenDraftAndWaitsForConfirmation() {
        val original = Wind("stable-id", "Brisa", "#123456", "lectura", "bto", "Persona que no es pot perdre.")
        var saved: WindDraft? = null
        var complete: ((Boolean) -> Unit)? = null
        var closed = false
        render(original, onBack = { closed = true }, onSave = { draft, done -> saved = draft; complete = done })
        ui.onNodeWithText(original.persona).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.onNodeWithText(tr("ca", "app.vents.saving")).assertIsNotEnabled()
        ui.onNodeWithText(tr("ca", "app.company.windShape_n")).assertIsNotEnabled()
        ui.onNodeWithContentDescription(tr("ca", "app.vents.colorChoice", "2")).assertIsNotEnabled()
        ui.onNodeWithContentDescription(tr("ca", "app.vents.back")).assertIsNotEnabled()
        ui.runOnIdle {
            assertFalse(closed)
            assertEquals(WindDraft(original.name, original.color, original.role, original.style, original.persona, original.id), saved)
            complete!!(false)
        }
        ui.onNodeWithText(tr("ca", "app.vents.saveError")).assertIsDisplayed()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle { assertFalse(closed); complete!!(true) }
        ui.runOnIdle { assertTrue(closed) }
    }

    @Test fun expressionAndPersonalityStayEditableBehindTheirDisclosures() {
        val original = Wind("details", "Brisa", "#123456", "lectura", "bto", "Persona original.")
        var saved: WindDraft? = null
        render(original, onSave = { draft, _ -> saved = draft })
        ui.onNodeWithText(tr("ca", "app.vents.customize")).performScrollTo().performClick()
        ui.onNodeWithText(tr("ca", "app.company.windFaceEyes")).performScrollTo()
        ui.onNodeWithText(tr("ca", "app.company.windEyes_m")).performSemanticsAction(androidx.compose.ui.semantics.SemanticsActions.OnClick) { it() }.assertIsSelected()
        // Scroll the outer column via its disclosure; performScrollTo on a chip only
        // reaches the nested horizontal row and cannot reveal a vertically hidden mouth.
        ui.onNodeWithText(tr("ca", "app.vents.personality")).performScrollTo()
        ui.onNodeWithText(tr("ca", "app.company.windMouth_n")).assertIsDisplayed().performClick().assertIsSelected()
        ui.onNodeWithText(tr("ca", "app.vents.personality")).performClick()
        ui.onNode(hasSetTextAction() and hasText(tr("ca", "app.vents.personality")))
            .performScrollTo().performTextReplacement("Persona editada.")
        ui.onNode(hasText(tr("ca", "app.vents.personality")) and !hasSetTextAction())
            .performScrollTo().performClick()
        ui.onNodeWithText("Persona editada.").assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle {
            assertEquals("Persona editada.", saved?.persona)
            assertEquals("mno", saved?.style)
            assertEquals(original.color, saved?.color)
            assertEquals(original.id, saved?.id)
        }
    }

    @Test fun aiSuggestionKeepsTheRealCallbackErrorAndResultPaths() {
        val original = Wind("suggest", "Brisa", "#123456", "lectura", "bto", "Persona original.")
        var failure: (() -> Unit)? = null
        var suggestion: ((String, String) -> Unit)? = null
        var saved: WindDraft? = null
        render(original, onSuggest = { name, role, error, done ->
            assertEquals(original.name, name)
            assertEquals(original.role, role)
            failure = error; suggestion = done
        }, onSave = { draft, _ -> saved = draft })
        ui.onNodeWithText(tr("ca", "app.company.windSuggest")).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.vents.personality")).performScrollTo().performClick()
        ui.onNodeWithText(tr("ca", "app.company.windSuggest")).performScrollTo().performClick()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).assertIsNotEnabled()
        ui.runOnIdle { failure!!() }
        ui.onNodeWithText(tr("ca", "app.vents.suggestError")).assertIsDisplayed()
        ui.onNodeWithText(original.persona).assertExists()
        ui.onNodeWithText(tr("ca", "app.company.windSuggest")).performScrollTo().performClick()
        ui.runOnIdle { suggestion!!("Persona de la IA.", "esn") }
        ui.onNodeWithText("Persona de la IA.").assertExists()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle {
            assertEquals("Persona de la IA.", saved?.persona)
            assertEquals("esn", saved?.style)
            assertEquals(original.id, saved?.id)
            assertEquals(original.color, saved?.color)
        }
    }

    @Test fun optionalPresetGalleryCanBeCancelledAndStillInitializesANewPersona() {
        var closed = false
        var saved: WindDraft? = null
        render(null, onBack = { closed = true }, onSave = { draft, _ -> saved = draft })
        ui.onNodeWithText(tr("ca", "app.company.windName")).performTextInput("Brisa")
        ui.onNodeWithText(tr("ca", "app.vents.changePreset")).performScrollTo().performClick()
        ui.onNodeWithContentDescription(tr("ca", "app.vents.back")).performClick()
        ui.runOnIdle { assertFalse(closed) }
        ui.onNodeWithText("Brisa").assertExists()
        ui.onNodeWithText(tr("ca", "app.vents.changePreset")).performScrollTo().performClick()
        ui.onNodeWithText("Marinada").performScrollTo().performClick()
        ui.onNode(hasSetTextAction() and hasText(tr("ca", "app.vents.personality"))).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.company.windSave")).performClick()
        ui.runOnIdle {
            assertEquals("Marinada", saved?.name)
            assertEquals(tr("ca", "app.vents.preset.Marinada.persona"), saved?.persona)
            assertEquals(WindSuggestions.all.first().style, saved?.style)
        }
    }

    @Test fun backFromTheDirectCreatorLeavesInsteadOfOpeningTheGallery() {
        var closed = false
        render(null, onBack = { closed = true })
        ui.onNodeWithContentDescription(tr("ca", "app.vents.back")).performClick()
        ui.runOnIdle { assertTrue(closed) }
        ui.onNodeWithText(tr("ca", "app.vents.galleryTitle")).assertDoesNotExist()
    }

    private fun render(
        wind: Wind?,
        onBack: () -> Unit = {},
        onSuggest: (String, String, () -> Unit, (String, String) -> Unit) -> Unit = { _, _, _, _ -> error("Unexpected suggestion") },
        onSave: (WindDraft, (Boolean) -> Unit) -> Unit = { _, _ -> error("Unexpected save") },
    ) {
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { WindEditor("ca", wind, onBack, onSuggest, onSave) }
            }
        }
    }

    @Test fun opensDirectlyWithVisualControlsAndCreatesWithoutChoosingATemplate() {
        var saved: WindDraft? = null
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    WindEditor("ca", null, {}, { _, _, _, _ -> error("Unexpected suggestion") },
                        { draft, _ -> saved = draft })
                }
            }
        }
        ui.onNodeWithText(tr("ca", "app.vents.galleryTitle")).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.company.windName")).assertIsDisplayed()
        ui.onNodeWithText(tr("ca", "app.company.windRole")).assertIsDisplayed()
        ui.onNodeWithText(tr("ca", "app.company.windFaceShape")).assertIsDisplayed()
        ui.onNodeWithText(tr("ca", "app.company.windColor")).assertIsDisplayed()
        ui.onNodeWithText(tr("ca", "app.company.windFaceEyes")).assertDoesNotExist()
        ui.onNode(hasSetTextAction() and hasText(tr("ca", "app.vents.personality"))).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.company.windSave")).assertIsNotEnabled()
        ui.onNodeWithText(tr("ca", "app.company.windName")).performTextInput("  Brisa  ")
        ui.onNodeWithText(tr("ca", "app.company.windRole")).performTextInput("  llibres  ")
        ui.onNodeWithText(tr("ca", "app.company.windShape_n")).performClick()
        ui.onNodeWithContentDescription(tr("ca", "app.vents.colorChoice", "2")).performClick()
        ui.onNodeWithText(tr("ca", "app.company.windSave")).assertIsEnabled().performClick()
        ui.runOnIdle {
            assertEquals("Brisa", saved?.name)
            assertEquals("llibres", saved?.role)
            assertEquals("#1A9BEE", saved?.color)
            assertEquals("n", windStyleOf(saved?.style).shape.key)
            assertEquals("", saved?.id)
        }
    }
}
