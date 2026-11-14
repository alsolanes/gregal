package xyz.solanes.gregal

import androidx.compose.ui.test.assertTextEquals
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** La resposta d'un vent convidat diu de qui és, no sembla del vent del fil. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class GuestJoinsTest {
    @get:Rule val rule = createComposeRule()

    @Test fun theGuestSaysWhoItIs() {
        assertEquals("→ Lluerna s'hi afegeix", guestJoinsText("ca", "Lluerna"))
        assertEquals("→ Lluerna joins in", guestJoinsText("en", "Lluerna"))
        rule.setContent { GuestJoinsLine("ca", "Lluerna") }
        rule.onNodeWithTag("guest-joins").assertTextEquals("→ Lluerna s'hi afegeix")
    }
}
