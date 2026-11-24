package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** 090: la feina del vent (l'ordinador, el document) en una targeta a sobre del camp. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindWorkCardTest {
    @get:Rule val ui = createComposeRule()
    private val carta = DocRef("w1", "Carta de benvinguda", "doc", "mine")
    private val renfe = OrdinadorLive(live = true, url = "https://www.renfe.com/horaris", title = "Horaris")

    @Test fun theCardShowsTheComputerOnlyWhenTheWindUsesIt() {
        var shot by mutableStateOf<OrdinadorLive?>(renfe)
        var opened = 0
        ui.setContent { MaterialTheme { WindWorkCard("en", Color.Blue, shot, null, onComputer = { opened++ }, onDoc = {}) } }
        ui.onNodeWithText("On renfe.com").assertIsDisplayed()
        ui.onNodeWithText("Watch").performClick()
        assertEquals(1, opened)
        shot = OrdinadorLive(live = false)
        ui.waitForIdle()
        ui.onNodeWithTag("wind-work-card").assertDoesNotExist()
        shot = null
        ui.waitForIdle()
        ui.onNodeWithTag("wind-work-card").assertDoesNotExist()
    }

    @Test fun theCardShowsTheDocumentAndOpensIt() {
        var picked: DocRef? = null
        ui.setContent { MaterialTheme { WindWorkCard("ca", Color.Blue, null, carta, onComputer = {}, onDoc = { picked = it }) } }
        ui.onNodeWithText("Carta de benvinguda").assertIsDisplayed()
        ui.onNodeWithText("Obre").performClick()
        assertEquals(carta, picked)
    }

    @Test fun aDocChangedDuringTheTurnIsTheTurnsWork() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        vm.setConn(Conn("127.0.0.1", "9", "t"))
        val chats = vm.windChat as kotlinx.coroutines.flow.MutableStateFlow<Map<String, List<GregalViewModel.PetMsg>>>
        chats.value = mapOf("mestral" to listOf(GregalViewModel.PetMsg("user", "Escriu-me una carta")))
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.DocChanged(carta), "mestral")
        // La targeta del xat hi és, i també és la feina del torn.
        assertEquals(listOf("", "doc"), vm.windChat.value["mestral"].orEmpty().map { it.kind })
        assertEquals(carta, turnDoc(vm.windChat.value["mestral"].orEmpty()))
        // Un torn nou encara no ha escrit res.
        assertNull(turnDoc(vm.windChat.value["mestral"].orEmpty() + GregalViewModel.PetMsg("user", "Gràcies")))
    }

    @Test fun theCardHidesWhenTheTurnEndsUnlessItWasOpened() {
        var working by mutableStateOf(true)
        val conn = Conn("127.0.0.1", "9", "t")
        ui.setContent { MaterialTheme { GregalWorkStrip(conn, "en", Color.Blue, working, carta, onOpen = {}, onDoc = {}) } }
        ui.onNodeWithText("Carta de benvinguda").assertIsDisplayed()
        working = false
        ui.waitForIdle()
        ui.onNodeWithTag("wind-work-card").assertDoesNotExist()
        // Si l'has obert durant el torn, la targeta es queda quan acaba.
        working = true
        ui.waitForIdle()
        ui.onNodeWithText("Open").performClick()
        working = false
        ui.waitForIdle()
        ui.onNodeWithText("Carta de benvinguda").assertIsDisplayed()
    }
}
