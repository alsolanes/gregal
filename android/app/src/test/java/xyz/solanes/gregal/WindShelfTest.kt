package xyz.solanes.gregal

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** El prestatge d'un vent (GET /api/winds/outputs): una targeta per peça, i tocar-la l'obre on toca. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class WindShelfTest {
    @get:Rule val ui = createComposeRule()

    private val raw = """{"items":[
        {"at":"2026-11-24T18:05:00+01:00","kind":"file","title":"Pressupost.pdf","url":"/api/winds/doc?name=Pressupost.pdf"},
        {"at":"2026-11-24T10:00:00+01:00","kind":"site","title":"Web del club","url":"/share/abc/"},
        {"at":"2026-11-20T10:00:00+01:00","kind":"deck","title":"Presentació","url":"/api/docs/d1a2/export?scope=mine&format=pptx"}]}"""

    @Test fun readsTheItemsAndKnowsWhichAreDocs() {
        val items = parseWindOutputs(raw)
        assertEquals(listOf("file", "site", "deck"), items.map { it.kind })
        assertEquals(DocRef("d1a2", "Presentació", "deck", "mine"), windOutputDoc(items[2]))
        assertNull(windOutputDoc(items[0]))
        assertNull(windOutputDoc(items[1]))
    }

    @Test fun eachPieceShowsItsIconTitleAndDateAndOpensOnTap() {
        val items = parseWindOutputs(raw)
        val opened = mutableListOf<String>()
        val shared = mutableListOf<String>()
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    WindShelfList("ca", items, onOpen = { opened += it.title }, onShare = { shared += it.url })
                }
            }
        }
        ui.onNodeWithText("📎").assertIsDisplayed()
        ui.onNodeWithText("📽️").assertIsDisplayed()
        ui.onNodeWithText(windActivityTime(items[0].at), substring = true).assertIsDisplayed()
        ui.onNodeWithText("Presentació").performClick()
        ui.onNodeWithText("Pressupost.pdf").performClick()
        assertEquals(listOf("Presentació", "Pressupost.pdf"), opened)
        // Només la web es comparteix: la resta vol la clau de l'app.
        ui.onNodeWithText("Comparteix").performClick()
        assertEquals(listOf("/share/abc/"), shared)
    }

    @Test fun anEmptyShelfSaysSo() {
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { WindShelfList("en", emptyList(), onOpen = {}, onShare = {}) }
            }
        }
        ui.onNodeWithText("hasn't made anything you can open yet", substring = true).assertIsDisplayed()
    }

    @Test fun theNewStringsExistInBothLanguages() {
        for (k in listOf("app.vents.shelf", "shelf.empty", "shelf.share", "shelf.loadError")) {
            assertTrue(k, STR_APP_CA.containsKey(k) && STR_APP_EN.containsKey(k))
        }
    }
}
