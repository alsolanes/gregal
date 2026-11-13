package xyz.solanes.gregal

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** Fes: plantilles que s'envien d'un toc, amb un buit o cada dia. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp")
class FesTest {
    @get:Rule val ui = createComposeRule()

    @Test fun messages() {
        val all = fesTemplates("ca").flatMap { it.second }
        assertEquals(12, all.size)
        val prices = all.first { it.title == "Compara preus" }
        assertEquals("Compara'm preus de auriculars: tres opcions amb el preu, on comprar-les i quina em recomanes.", fesMessage("ca", prices, " auriculars "))
        val today = all.first { it.title == "Llista del dia" }
        assertTrue(fesMessage("ca", today, everyDay = true).startsWith("Cada dia a les 08:00: Fes-me la llista d'avui"))
        assertEquals(fesTemplates("en").flatMap { it.second }.size, all.size)
        assertTrue(all.filter { it.blank.isNotEmpty() }.all { "{x}" in it.prompt })
    }

    @Test fun tapSendsOrOpensTheBlank() {
        val sent = mutableListOf<String>()
        ui.setContent { MaterialTheme { Column(Modifier.verticalScroll(rememberScrollState())) { FesContent("ca") { sent += it } } } }
        ui.onNodeWithText("Llista del dia").performScrollTo().performClick()
        assertTrue(sent.single().startsWith("Fes-me la llista d'avui"))
        ui.onNodeWithText("Compara preus").performScrollTo().performClick()
        ui.onNodeWithTag("fes-input").performTextInput("una bici plegable")
        ui.onNodeWithTag("fes-go").performClick()
        assertTrue(sent.last().contains("una bici plegable"))
        ui.onNodeWithTag("fes-daily-Triatge del correu").performScrollTo().performClick()
        assertTrue(sent.last().startsWith("Cada dia a les 09:00"))
    }
}
