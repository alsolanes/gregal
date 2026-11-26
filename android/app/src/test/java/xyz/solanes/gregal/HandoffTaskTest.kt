package xyz.solanes.gregal

import androidx.compose.ui.test.assertTextEquals
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** 161: un encàrrec que un vent passa a un altre diu de qui és, amb la seva cara, no sembla teu. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class HandoffTaskTest {
    @get:Rule val rule = createComposeRule()

    private fun decode(json: String) = GregalJson.decodeFromString(HistoryDto.serializer(), json).history.map { it.toPetMsg() }

    @Test fun theChatKeepsWhoAskedIt() {
        val msgs = decode("""{"history":[
            {"id":"m-1","from":"user","text":"Encàrrec de Lluerna: mira el preu","by":{"id":"lluerna","name":"Lluerna"}},
            {"id":"m-2","from":"user","text":"hola"}
        ]}""")
        assertEquals(MsgBy("lluerna", "Lluerna"), msgs[0].by)
        assertNull("sense by és un missatge teu, com sempre", msgs[1].by)
        assertEquals("hola", msgs[1].text)
    }

    @Test fun theStreamDoneHistoryKeepsItToo() {
        val done = GregalJson.decodeFromString(RotllanaDoneDto.serializer(),
            """{"history":[{"from":"user","text":"Task from Gregal: x","by":{"id":"gregal","name":"Gregal"}}]}""")
        assertEquals(MsgBy("gregal", "Gregal"), done.history.single().toPetMsg().by)
    }

    @Test fun theRowSaysTaskFromWithoutRepeatingTheName() {
        assertEquals("Encàrrec de Lluerna", tr("ca", "app.vents.handoffFrom", "Lluerna"))
        assertEquals("Task from Lluerna", tr("en", "app.vents.handoffFrom", "Lluerna"))
        assertEquals("mira el preu", handoffBody("Encàrrec de Lluerna: mira el preu", "Lluerna"))
        assertEquals("check the price", handoffBody("Task from Lluerna: check the price", "Lluerna"))
        assertEquals("sense prefix", handoffBody("sense prefix", "Lluerna"))
        val by = MsgBy("lluerna", "Lluerna")
        val m = GregalViewModel.PetMsg("user", "Encàrrec de Lluerna: mira el preu", by = by)
        rule.setContent { HandoffRow(m, by, null, "ca") }
        rule.onNodeWithTag("handoff-from").assertTextEquals("Encàrrec de Lluerna")
        rule.onNodeWithText("mira el preu", substring = true).assertExists()
        rule.onNodeWithText("Encàrrec de Lluerna: mira el preu", substring = true).assertDoesNotExist()
    }
}
