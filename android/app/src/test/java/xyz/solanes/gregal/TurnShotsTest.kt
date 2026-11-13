package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** «Ho he fet així»: les captures del torn sota la resposta del vent. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class TurnShotsTest {
    @get:Rule val ui = createComposeRule()

    private val shots = listOf(
        MsgShot("a", "Catedral de Girona - Viquipèdia", "https://ca.wikipedia.org/wiki/Catedral_de_Girona", "/api/ordinador/accions/captura?id=a"),
        MsgShot("b", "Història", "https://www.example.org/historia", "/api/ordinador/accions/captura?id=b"),
        MsgShot("c", "", "https://ca.wikipedia.org/wiki/Girona", "/api/ordinador/accions/captura?id=c"),
    )

    @Test fun theHistoryCarriesTheShots() {
        val dto = Json { ignoreUnknownKeys = true }.decodeFromString<HistoryDto>(
            """{"history":[{"from":"pet","text":"Fet","shots":[
                {"id":"a","title":"Girona","url":"https://ca.wikipedia.org/wiki/Girona","shotUrl":"/api/ordinador/accions/captura?id=a"},
                {"id":"x","shotUrl":"https://evil.example/x.jpg"}]}]}""")
        val m = dto.history.single().toPetMsg()
        // Només les del servidor (cap adreça de fora).
        assertEquals(listOf("a"), m.shots.map { it.id })
        assertEquals("Girona", m.shots.single().title)
        assertTrue(Json { ignoreUnknownKeys = true }.decodeFromString<HistoryDto>("""{"history":[{"from":"pet","text":"x"}]}""").history.single().toPetMsg().shots.isEmpty())
    }

    @Test fun copyAndHosts() {
        for (k in listOf("title", "prev", "next", "close")) assertTrue(k, turnShotsCopy("ca", k) != turnShotsCopy("en", k))
        assertEquals("3 captures", turnShotsCopy("ca", "count", 3))
        assertEquals("1 screenshot", turnShotsCopy("en", "count", 1))
        assertEquals("example.org", shotHost("https://www.example.org/historia"))
        assertEquals("", shotHost("no és cap adreça"))
    }

    @Test fun theStripOpensTheViewerAndItMoves() {
        ui.setContent { MaterialTheme { TurnShotsStrip(Conn("", "", ""), "ca", Color(0xFF8193FF), shots) } }
        ui.onNodeWithText("Ho he fet així").assertIsDisplayed()
        ui.onNodeWithText("3 captures").assertIsDisplayed()
        ui.onNodeWithTag("turn-shot-1").assertHasClickAction().performClick()
        ui.onNodeWithTag("turn-shots-viewer").assertIsDisplayed()
        ui.onNodeWithTag("turn-shots-pos").assertTextEquals("2 / 3")
        ui.onNodeWithContentDescription("Següent").performClick()
        ui.onNodeWithTag("turn-shots-pos").assertTextEquals("3 / 3")
        ui.onNodeWithContentDescription("Tanca").performClick()
        ui.onNodeWithTag("turn-shots-viewer").assertDoesNotExist()
    }
}
