package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
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
class VentsExperienceTest {
    @get:Rule val ui = createComposeRule()
    private fun vm() = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())

    @Test fun companionScreenActuallyDisplaysRequestErrors() {
        val vm = vm()
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { CompanyScreen(vm) {} }
        } }
        ui.runOnIdle { vm.notify("No s’ha pogut enviar. Torna-ho a provar.") }
        ui.onNodeWithText("No s’ha pogut enviar. Torna-ho a provar.").assertExists()
    }

    @Test fun homeSeparatesConversationsFromWindManagementAndHasOneCreationMenu() {
        val vm = vm()
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { VentsHome(vm, "ca", {}) {} }
        } }
        ui.onNodeWithText("Converses").assertExists()
        ui.onNodeWithText("Els meus vents").assertExists()
        ui.onNodeWithContentDescription("Crea").performClick()
        ui.onNodeWithText("Nou xat").assertExists()
        ui.onNodeWithText("Nova rotllana").assertExists()
        ui.onNodeWithText("Nou vent").assertExists()
    }

    @Test fun editingStartsWithExistingIdentityAndWaitsForConfirmedSave() {
        val wind = Wind("same-id", "Marinada", "#EFC69A", "calma", persona = "Escolto amb calma.")
        var closed = false
        var completion: ((Boolean) -> Unit)? = null
        var saved: WindDraft? = null
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) {
                WindEditor("ca", wind, { closed = true }, { _, _, _, _ -> error("unexpected suggestion") }, { draft, done -> saved = draft; completion = done })
            }
        } }
        ui.onNodeWithText(tr("ca", "app.vents.galleryTitle")).assertDoesNotExist()
        ui.onNodeWithText("Escolto amb calma.").assertExists()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle { org.junit.Assert.assertFalse(closed); org.junit.Assert.assertEquals("same-id", saved?.id) }
        ui.runOnIdle { completion!!(false) }
        ui.onNodeWithText(tr("ca", "app.vents.saveError")).assertExists()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle { completion!!(true) }
        ui.runOnIdle { org.junit.Assert.assertTrue(closed) }
    }

    @Test fun choosingPresetAdvancesToEditablePersonalityNotEightCardWall() {
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) {
                WindEditor("ca", null, {}, { _, _, _, _ -> error("unexpected suggestion") }, { _, _ -> error("unexpected save") })
            }
        } }
        ui.onNodeWithText(tr("ca", "app.vents.galleryTitle")).assertIsDisplayed()
        ui.onNodeWithText("Marinada").performClick()
        ui.onNodeWithText(tr("ca", "app.vents.galleryTitle")).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.company.windName")).assertIsDisplayed()
        ui.onNodeWithText(tr("ca", "app.vents.personality")).assertIsDisplayed()
    }

    @Test fun creatorReplacesHomeAndHasExactlyOneOwner() {
        val vm = vm()
        vm.showWindCreator(true)
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { VentsHome(vm, "ca", {}) {} }
        } }
        ui.onAllNodesWithText(tr("ca", "app.vents.galleryTitle")).assertCountEquals(1)
        ui.onNodeWithText(tr("ca", "app.vents.conversations")).assertDoesNotExist()
    }

    @Test fun newChatAllowsOneMemberButGroupRequiresTwo() {
        // Compose excludes infinite transitions from idling; keep finite dialog setup advancing.
        val winds = listOf(Wind("a", "Marinada", "#EFC69A", "calma"), Wind("b", "Xaloc", "#EFA68F", "música"))
        var saved: List<String>? = null
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { ConversationPicker("ca", winds, false, {}, onCreate = { _, members -> saved = members }) }
        } }
        ui.onNodeWithText(tr("ca", "app.vents.chatName")).performTextInput("El meu racó")
        ui.mainClock.advanceTimeBy(100)
        ui.onNodeWithText(tr("ca", "app.vents.startChat")).assertIsNotEnabled()
        ui.onNodeWithText("Marinada").performClick()
        ui.mainClock.advanceTimeBy(100)
        ui.onNodeWithText(tr("ca", "app.vents.startChat")).assertIsEnabled().performClick()
        ui.runOnIdle { org.junit.Assert.assertEquals(listOf("a"), saved) }
    }

    @Test fun memberRowsShowRoleAndExpertise() {
        val winds = listOf(Wind("a", "Marinada", "#EFC69A", "calma"), Wind("b", "Xaloc", "#EFA68F", "música"))
        val resum = mapOf("a" to GregalApi.WindResum("a", "Marinada", "calma",
            listOf(GregalApi.Coneixement("calcots", "Calçots", "Temporada i ritual."))))
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { ConversationPicker("ca", winds, true, {}, resum = resum, onCreate = { _, _ -> }) }
        } }
        ui.onNodeWithText("calma").assertIsDisplayed()
        ui.onNodeWithText("música").assertIsDisplayed()
        ui.onNodeWithText("Sap de: Calçots").assertIsDisplayed()
    }

    @Test fun ventsGridShowsBigMascotsWithStatus() {
        val winds = listOf(Wind("a", "Marinada", "#EFC69A", "calma"), Wind("b", "Xaloc", "#EFA68F", "música"))
        var opened: String? = null
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { VentsGrid("ca", winds, setOf("b")) { opened = it.id } }
        } }
        ui.onNodeWithText("Marinada").assertIsDisplayed()
        ui.onNodeWithText("calma").assertIsDisplayed()
        ui.onNodeWithText("Xaloc").assertIsDisplayed()
        ui.onNodeWithText(tr("ca", "app.vents.gridThinking")).assertIsDisplayed()
        ui.onNodeWithContentDescription("Marinada").performClick()
        ui.runOnIdle { org.junit.Assert.assertEquals("a", opened) }
    }

    @Test fun profileShowsActualKnowledgeAndSeparatePersonalMemory() {
        val wind = Wind("a", "Marinada", "#EFC69A", "calma", persona = "Escolto amb calma.")
        val profile = WindProfile(wind, "Coneixement real del servidor", "Memòria personal real")
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) {
                WindProfileContent(vm(), "ca", wind, profile, false, null, emptyList(), {}, {}, {}, { _, _ -> error("unexpected write") })
            }
        } }
        ui.onNodeWithText(tr("ca", "app.vents.memory")).performClick()
        ui.onNodeWithText("Coneixement real del servidor").assertExists()
        ui.onNodeWithText("Memòria personal real").assertExists()
        ui.onNodeWithText(tr("ca", "app.vents.activity")).performClick()
        ui.onNodeWithText(tr("ca", "app.vents.noActivity")).assertExists()
    }

    @Test fun groupBubbleLabelsItsActualSpeakerAndOpensTheirProfile() {
        val wind = Wind("x", "Xaloc", "#EFA68F", "música")
        var opened = false
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) {
                WindBubble(GregalViewModel.PetMsg("pet", "https://example.org: una descoberta", "x", "Xaloc"),
                    androidx.compose.ui.graphics.Color(wind.argb), speaker = wind, speakerName = wind.name, onSpeakerClick = { opened = true })
            }
        } }
        ui.onNodeWithText("Xaloc").assertExists().performClick()
        ui.runOnIdle { org.junit.Assert.assertTrue(opened) }
    }

    @Test fun manageTabShowsWindsDefaultModelAndOpensPicker() {
        val vm = vm()
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { VentsHome(vm, "ca", {}) {} }
        } }
        ui.onNodeWithText(tr("ca", "app.vents.myWinds")).performClick()
        ui.onNodeWithText(tr("ca", "app.vents.defaultModel")).assertExists()
        ui.onNodeWithText(tr("ca", "app.vents.defaultModelSameAsWork")).assertExists()
        ui.onNodeWithText(tr("ca", "app.vents.defaultModel")).performClick()
        ui.onNodeWithText(tr("ca", "app.vents.pickWindsModel")).assertExists()
    }
}
