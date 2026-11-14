package xyz.solanes.gregal

import androidx.compose.foundation.layout.Column
import androidx.compose.material3.MaterialTheme
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

/** «Ho recordaré» al fil i «Què sap de tu». */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp")
class ApresTest {
    @get:Rule val ui = createComposeRule()

    @Test fun theHistoryKeepsTheMemoryKind() {
        val dto = Json { ignoreUnknownKeys = true }.decodeFromString<HistoryDto>("""{"history":[{"from":"pet","text":"Prefereix respostes curtes","kind":"memory"}]}""")
        assertEquals("memory", dto.history.single().toPetMsg().kind)
    }

    @Test fun noticeForgets() {
        var forgot = 0
        ui.setContent { MaterialTheme { MemoryNotice("ca", listOf("Prefereix respostes curtes", "Viu a Girona")) { forgot++ } } }
        ui.onNodeWithText("Ho recordaré").assertIsDisplayed()
        ui.onNodeWithText("Prefereix respostes curtes · Viu a Girona").assertIsDisplayed()
        ui.onNodeWithTag("memory-forget").performClick()
        assertEquals(1, forgot)
        ui.onNodeWithText("Oblidat.").assertIsDisplayed()
        ui.onNodeWithTag("memory-forget").assertDoesNotExist()
    }

    @Test fun listAndForget() {
        val gone = mutableListOf<Pair<String, String>>()
        ui.setContent { MaterialTheme { Column { ApresContent("ca", listOf(ApresGroup("", "", listOf("Es diu Aleix")), ApresGroup("llevantada", "Llevantada", listOf("No menja carn")))) { w, f -> gone += w to f } } } }
        ui.onNodeWithText("Sobre tu").assertIsDisplayed()
        ui.onNodeWithText("El que en sap Llevantada.").assertIsDisplayed()
        ui.onNodeWithTag("apres-x-llevantada-0").performClick()
        assertEquals(listOf("llevantada" to "No menja carn"), gone)
        for (k in listOf("title", "remember", "forget")) assertTrue(k, apresCopy("ca", k) != apresCopy("en", k))
    }
}
