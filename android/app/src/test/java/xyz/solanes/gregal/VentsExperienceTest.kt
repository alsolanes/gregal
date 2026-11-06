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

    @Test fun notificationTargetSurvivesACatalogueThatArrivesLater() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext())
        vm.openVentsChat("wind:late")
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) { CompanyScreen(vm, {}) }
            }
        }
        ui.waitForIdle()
        org.junit.Assert.assertEquals("wind:late", vm.ventsOpen.value)
        ui.runOnIdle {
            @Suppress("UNCHECKED_CAST")
            (GregalViewModel::class.java.getDeclaredField("_winds").apply { isAccessible = true }.get(vm)
                as kotlinx.coroutines.flow.MutableStateFlow<List<Wind>>).value = listOf(Wind("late", "Brisa tardana", "#A8CDE2", "Prova"))
        }
        ui.onNodeWithText("Brisa tardana").assertExists()
    }

    @Test fun libraryEntryOpensTheRealRecoveryDialog() {
        val vm = vm()
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { CompanyScreen(vm) {} }
        } }
        val lang = vm.idioma.value
        ui.onNodeWithContentDescription(ventsCopy(lang, "explore")).performClick()
        // L'acció de clic directa: l'avís d'error (sense servidor a la prova) pot tapar el final del menú.
        ui.onNodeWithText(ventsCopy(lang, "library")).performSemanticsAction(androidx.compose.ui.semantics.SemanticsActions.OnClick)
        ui.onNodeWithText(tr(lang, "app.library.about")).assertExists()
    }

    @Test fun homeSeparatesConversationsFromWindManagementAndHasOneCreationMenu() {
        val vm = vm()
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { VentsHome(vm, "ca", {}) {} }
        } }
        // Les converses ocupen l'inici; gestionar els vents és una acció a part.
        // Buit o amb converses (si a la màquina hi ha un Gregal servint al
        // 8097, el model en carrega les reals), però mai el directori a dins.
        ui.waitForIdle()
        val buit = ui.onAllNodesWithText(tr("ca", "app.vents.emptyTitle")).fetchSemanticsNodes().isNotEmpty()
        val converses = ui.onAllNodesWithText(tr("ca", "app.vents.conversations"), ignoreCase = true).fetchSemanticsNodes().isNotEmpty()
        // Sense cap vent (compte nou) surt la benvinguda amb la colla del dia a dia.
        val benvinguda = ui.onAllNodesWithText(tr("ca", "app.vents.welcomeOwn")).fetchSemanticsNodes().isNotEmpty()
        // Amb vents (p. ex. un Gregal servint al 8097), l'inici és el de «pregunta primer».
        val pregunta = ui.onAllNodesWithText(askCopy("ca", "sub")).fetchSemanticsNodes().isNotEmpty()
        org.junit.Assert.assertTrue("la portada no ensenya ni les converses, ni el buit, ni la benvinguda, ni l'inici de preguntar", buit || converses || benvinguda || pregunta)
        ui.onNodeWithContentDescription(tr("ca", "app.vents.directory")).assertExists()
        ui.onNodeWithContentDescription("Crea").performClick()
        ui.onNodeWithText("Nou xat").assertExists()
        ui.onNodeWithText("Nova rotllana").assertExists()
        ui.onNodeWithText("Nou vent").assertExists()
    }

    @Test fun homeStartsWithConversationsAndCompactConnectionByDefault() {
        val vm = vm()
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { VentsHome(vm, "ca", {}) {} }
        } }
        // Les converses són el primer pla per defecte; el Ventet continua al menú.
        ui.onNodeWithTag("ventet-row").assertDoesNotExist()
        ui.onNodeWithTag("where-chip").assertExists()
        ui.onNodeWithText(tr("ca", "app.where.phoneHint")).assertDoesNotExist()
        ui.onNodeWithTag("vents-strip").assertDoesNotExist()
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
        // La personalitat està plegada, però l'edició visual no l'ha d'esborrar.
        ui.onNodeWithText("Escolto amb calma.").assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle {
            org.junit.Assert.assertFalse(closed)
            org.junit.Assert.assertEquals("same-id", saved?.id)
            org.junit.Assert.assertEquals("Escolto amb calma.", saved?.persona)
        }
        ui.runOnIdle { completion!!(false) }
        ui.onNodeWithText(tr("ca", "app.vents.saveError")).assertExists()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle { completion!!(true) }
        ui.runOnIdle { org.junit.Assert.assertTrue(closed) }
    }

    @Test fun optionalGalleryReturnsToVisualEditorWithPersonalityPreserved() {
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) {
                WindEditor("ca", null, {}, { _, _, _, _ -> error("unexpected suggestion") }, { _, _ -> error("unexpected save") })
            }
        } }
        ui.onNodeWithText(tr("ca", "app.vents.galleryTitle")).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.vents.changePreset")).performScrollTo().performClick()
        ui.onNodeWithText(tr("ca", "app.vents.galleryTitle")).assertIsDisplayed()
        ui.onNodeWithText("Marinada").performScrollTo().performClick()
        ui.onNodeWithText(tr("ca", "app.vents.galleryTitle")).assertDoesNotExist()
        ui.onNodeWithText(tr("ca", "app.company.windName")).performScrollTo().assertIsDisplayed()
        ui.onNodeWithText(tr("ca", "app.vents.personality")).performScrollTo().performClick()
        ui.onNodeWithText(tr("ca", "app.vents.preset.Marinada.persona")).assertExists()
    }

    @Test fun creatorReplacesHomeAndHasExactlyOneOwner() {
        val vm = vm()
        vm.showWindCreator(true)
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { VentsHome(vm, "ca", {}) {} }
        } }
        ui.onAllNodesWithTag("wind-editor-preview").assertCountEquals(1)
        ui.onAllNodesWithText(tr("ca", "app.company.windName")).assertCountEquals(1)
        ui.onNodeWithText(tr("ca", "app.vents.galleryTitle")).assertDoesNotExist()
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

    @Test fun ventsCrewShowsMascotsWithStatus() {
        val winds = listOf(Wind("a", "Marinada", "#EFC69A", "calma"), Wind("b", "Xaloc", "#EFA68F", "música"))
        var opened: String? = null
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
            MaterialTheme(colorScheme = LightPal.scheme()) { VentsCrew("ca", winds, setOf("b")) { opened = it.id } }
        } }
        ui.onNodeWithText("Marinada").assertIsDisplayed()
        ui.onNodeWithText("calma").assertDoesNotExist()
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
                WindProfileContent(vm(), "ca", wind, profile, false, null, emptyList(), emptyList(), {}, {}, {}, { _, _ -> error("unexpected write") })
            }
        } }
        ui.onNodeWithText(tr("ca", "app.vents.memory")).performClick()
        ui.onNodeWithText("Coneixement real del servidor").assertExists()
        ui.onNodeWithText("Memòria personal real").assertExists()
        // La pestanya Programat: el que farà sol i l'interruptor dels seguiments.
        ui.onNodeWithText(tr("ca", "app.vents.scheduled")).performClick()
        ui.onNodeWithText(tr("ca", "act.followups")).assertExists()
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
        ui.onNodeWithContentDescription(tr("ca", "app.vents.directory")).performClick()
        ui.onNodeWithText(tr("ca", "app.vents.myWinds")).assertExists()
        ui.onNodeWithText(tr("ca", "app.vents.defaultModel")).assertExists()
        ui.onNodeWithText(tr("ca", "app.vents.defaultModelSameAsWork")).assertExists()
        ui.onNodeWithText(tr("ca", "app.vents.defaultModel")).performClick()
        ui.onNodeWithText(tr("ca", "app.vents.pickWindsModel")).assertExists()
    }
}
