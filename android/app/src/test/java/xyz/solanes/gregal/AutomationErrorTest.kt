package xyz.solanes.gregal

import android.app.Application
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.LooperMode

/** La targeta d'error de l'activitat diu què ha fallat, no un «%1$s» sense omplir. */
@RunWith(RobolectricTestRunner::class)
@LooperMode(LooperMode.Mode.PAUSED)
class AutomationErrorTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    @Test fun `la targeta d'error porta el motiu`() {
        val app = ApplicationProvider.getApplicationContext<Application>()
        val vm = GregalViewModel(app).also { it.setConn(Conn("127.0.0.1", "9", "", local = true)) }
        val state = GregalViewModel::class.java.getDeclaredField("_automations").apply { isAccessible = true }.get(vm)
            as MutableStateFlow<Map<String, WindAutomations>>
        ui.mainClock.autoAdvance = false
        ui.setContent { WindAutomationList(vm, "ca") }
        // Amb el rellotge aturat, la càrrega contra el port 9 no acaba: l'error es posa a mà.
        state.value = mapOf("" to WindAutomations(emptyList(), WindAutomationCounts(), "servidor apagat"))
        ui.mainClock.advanceTimeBy(500)
        ui.onNodeWithText("servidor apagat", substring = true).assertExists()
        ui.onAllNodesWithText("%1", substring = true).assertCountEquals(0)
    }
}
