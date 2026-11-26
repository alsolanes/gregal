package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** 163: a quin vent pregunta el vent del xat i què li han respost (missatges «consult»). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindConsultTest {
    @get:Rule val ui = createComposeRule()
    private val mestral = Wind("mestral", "Mestral", "#A8CDE2", "recerca")
    private val llevant = Wind("llevant", "Llevant", "#E2B8A8", "codi")
    private val xaloc = Wind("xaloc", "Xaloc", "#B8E2A8", "dades")
    private val api = GregalApi("http://127.0.0.1:9", "")

    private fun <T> flow(vm: GregalViewModel, name: String) = GregalViewModel::class.java.getDeclaredField(name)
        .apply { isAccessible = true }.get(vm) as MutableStateFlow<T>

    private fun open(msgs: List<GregalViewModel.PetMsg>, live: WindLive? = null, newFrom: Int? = null): GregalViewModel {
        // Un port tancat: el fil que el test hi posa no el substitueix cap servidor real.
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>()).apply { setConn(Conn("127.0.0.1", "9", "t")) }
        flow<List<Wind>>(vm, "_winds").value = listOf(mestral, llevant, xaloc)
        flow<Map<String, List<GregalViewModel.PetMsg>>>(vm, "_windChat").value = mapOf(mestral.id to msgs)
        if (live != null) flow<Map<String, WindLive>>(vm, "_windLive").value = mapOf(mestral.id to live)
        if (newFrom != null) vm.windNew.at.value = mapOf(mestral.id to newFrom)
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { WindConversation(vm, "ca", mestral, onBack = {}) }
            }
        }
        return vm
    }

    private fun consult(w: Wind, text: String) = GregalViewModel.PetMsg("pet", text, w.id, w.name, kind = "consult")

    @Test fun theActivityCarriesTheConsultedWinds() {
        val e = api.parseWindStreamEvent("activity", """{"phase":"tool","tool":"wind","step":2,"winds":["llevant"]}""")
        assertEquals(GregalApi.WindStreamEvent.Activity("tool", "wind", 2, listOf("llevant")), e)
    }

    @Test fun theLabelNamesTheWindBeingAsked() {
        val asking = WindLive(phase = "tool", tool = "wind", asking = listOf(llevant))
        assertEquals("Preguntant a Llevant", windProgressLabel("ca", asking))
        assertEquals("Asking Llevant", windProgressLabel("en", asking))
        val many = asking.copy(asking = listOf(llevant, xaloc))
        assertEquals("Preguntant a 2 vents", windProgressLabel("ca", many))
        assertEquals("Asking 2 winds", windProgressLabel("en", many))
        assertEquals("Consultant un especialista", windProgressLabel("ca", asking.copy(asking = emptyList())))
        assertEquals("Asking a specialist", windProgressLabel("en", asking.copy(asking = emptyList())))
    }

    @Test fun theProgressRowShowsTheFaceOfTheWindBeingAsked() {
        open(listOf(GregalViewModel.PetMsg("user", "Revisa-ho"), GregalViewModel.PetMsg("pet", "", streaming = true)),
            live = WindLive(phase = "tool", tool = "wind", asking = listOf(llevant)))
        ui.onNodeWithText("Preguntant a Llevant").assertExists()
        ui.onNodeWithTag("wind-asking-face", useUnmergedTree = true).assertExists()
    }

    @Test fun noFaceWhenTheToolIsNotAskingAWind() {
        open(listOf(GregalViewModel.PetMsg("user", "Revisa-ho"), GregalViewModel.PetMsg("pet", "", streaming = true)),
            live = WindLive(phase = "tool", tool = "wind"))
        ui.onNodeWithText("Consultant un especialista").assertExists()
        ui.onNodeWithTag("wind-asking-face", useUnmergedTree = true).assertDoesNotExist()
    }

    @Test fun aRunOfAnswersIsFoldedUnderWhoWasAsked() {
        open(listOf(GregalViewModel.PetMsg("user", "Revisa-ho"),
            consult(llevant, "El codi és net"), consult(xaloc, "Les dades quadren"),
            GregalViewModel.PetMsg("pet", "Tot correcte")))
        ui.onNodeWithText("Ha preguntat a 2 vents").assertExists()
        ui.onNodeWithText("El codi és net", substring = true).assertDoesNotExist()
        ui.onNodeWithText("Tot correcte", substring = true).assertExists()
        ui.onNodeWithTag("crew-debate").performClick()
        ui.onNodeWithText("El codi és net", substring = true).assertExists()
        ui.onNodeWithText("Les dades quadren", substring = true).assertExists()
        ui.onAllNodesWithText("Xaloc", substring = true).onFirst().assertExists()
    }

    @Test fun oneWindAskedIsNamed() {
        open(listOf(GregalViewModel.PetMsg("user", "Revisa-ho"), consult(llevant, "El codi és net"),
            GregalViewModel.PetMsg("pet", "Tot correcte")))
        ui.onNodeWithText("Ha preguntat a Llevant").assertExists()
    }

    @Test fun theNewDividerStaysWhenItFallsInsideAFoldedRun() {
        open(listOf(GregalViewModel.PetMsg("user", "Revisa-ho"),
            consult(llevant, "El codi és net"), consult(xaloc, "Les dades quadren"),
            GregalViewModel.PetMsg("pet", "Tot correcte")), newFrom = 2)
        ui.onNodeWithText("Nous").assertExists()
    }
}
