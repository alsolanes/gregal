package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onChildAt
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.After
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** El xat amb vidre esmerilat: la llista corre per sota de les barres i el contingut comença sota la capçalera. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class ChatFrostUiTest {
    @get:Rule val ui = createComposeRule()

    // Enviar obre un torn que compta com a feina en marxa (estat global de
    // GregalNotifications): el tanquem perquè no arribi als altres tests.
    @After fun closeTurn() {
        GregalNotifications.workFinished(ApplicationProvider.getApplicationContext(), false)
        GregalNotifications.serviceGone()
    }

    private fun host(pal: GregalPalette) {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        vm.send("Hola, vent")
        ui.setContent {
            CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = pal.scheme()) { ChatScreen(vm) {} }
            }
        }
    }

    @Test fun ambVidreLaLlistaPassaPerSotaDeLesBarres() {
        host(DarkPal.glassy())
        val list = ui.onNodeWithTag("chat-list").fetchSemanticsNode().boundsInRoot
        val composer = ui.onNodeWithTag("chat-composer-area").fetchSemanticsNode().boundsInRoot
        assertEquals(0f, list.top, 0.5f)
        assertTrue("la llista s'atura al compositor", list.bottom > composer.top)
        val bar = ui.onNodeWithTag("chat-top-bar").fetchSemanticsNode().boundsInRoot
        val first = ui.onNodeWithTag("chat-list").onChildAt(0).fetchSemanticsNode().boundsInRoot
        assertTrue("el contingut comença sota la capçalera (${first.top} < ${bar.bottom})", first.top >= bar.bottom - 0.5f)
        val msg = ui.onNodeWithText("Hola, vent").fetchSemanticsNode().boundsInRoot
        assertTrue("el missatge queda per sobre del compositor", msg.bottom <= composer.top)
    }

    @Test fun senseVidreLaLlistaSAturaALesBarres() {
        host(DarkPal)
        val list = ui.onNodeWithTag("chat-list").fetchSemanticsNode().boundsInRoot
        val composer = ui.onNodeWithTag("chat-composer-area").fetchSemanticsNode().boundsInRoot
        val bar = ui.onNodeWithTag("chat-top-bar").fetchSemanticsNode().boundsInRoot
        assertTrue("la llista hauria de començar sota la capçalera (${list.top})", list.top >= bar.bottom - 0.5f)
        assertTrue(list.bottom <= composer.top + 0.5f)
    }
}
