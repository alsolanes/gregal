package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** Missatges escrits mentre l'agent treballa: a la cua, i «Ara» els hi diu ja. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class MessageQueueTest {
    @get:Rule val ui = createComposeRule()
    private fun vm() = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())

    @Test fun enqueueKeepsOrderIgnoresBlanksAndRemoves() {
        val vm = vm()
        vm.enqueue("  primer "); vm.enqueue("   "); vm.enqueue("segon")
        assertEquals(listOf("primer", "segon"), vm.queue.value)
        vm.removeQueued(0)
        assertEquals(listOf("segon"), vm.queue.value)
    }

    @Test fun withoutALiveTurnSteeringKeepsTheMessageQueued() {
        val vm = vm()
        vm.enqueue("digues-li això")
        vm.steerQueued(0)
        assertEquals(listOf("digues-li això"), vm.queue.value)
    }

    @Test fun queuedRowsOfferNowAndRemove() {
        var steered = -1; var removed = -1
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal) {
            MaterialTheme(colorScheme = LightPal.scheme()) {
                QueuedMessages(listOf("primer", "segon"), "ca", canSteer = true, onSteer = { steered = it }, onRemove = { removed = it })
            }
        } }
        ui.onAllNodesWithTag("queued-item").assertCountEquals(2)
        ui.onAllNodesWithTag("queued-now")[1].performClick()
        assertEquals(1, steered)
        ui.onAllNodesWithTag("queued-remove")[0].performClick()
        assertEquals(0, removed)
        ui.onNodeWithText("primer").performClick()
        assertEquals(0, steered)
    }

    @Test fun withoutALiveTurnThereIsNoNowButton() {
        ui.setContent { CompositionLocalProvider(LocalPal provides LightPal) {
            MaterialTheme(colorScheme = LightPal.scheme()) {
                QueuedMessages(listOf("primer"), "en", canSteer = false, onSteer = {}, onRemove = {})
            }
        } }
        ui.onAllNodesWithTag("queued-now").assertCountEquals(0)
        ui.onNodeWithTag("queued-remove").assertExists()
    }
}
