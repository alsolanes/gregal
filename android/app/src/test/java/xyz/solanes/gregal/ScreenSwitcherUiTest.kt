package xyz.solanes.gregal

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.snapshots.Snapshot
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** La pantalla que entra ve de la dreta endavant, de l'esquerra enrere, i sense moviment és al lloc de seguida. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class ScreenSwitcherUiTest {
    @get:Rule val ui = createComposeRule()

    private val nav = mutableStateOf(Screen.Company to 0)

    private fun host(start: Pair<Screen, Int>, motion: Boolean = true) {
        nav.value = start
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides motion) {
                ScreenSwitcher(nav.value.first, nav.value.second) { s ->
                    Box(Modifier.fillMaxSize().testTag(s.name))
                }
            }
        }
        ui.mainClock.advanceTimeBy(1000)
    }

    /** Canvia de pantalla i retorna on és la que entra uns 100 ms després. */
    private fun switchTo(to: Pair<Screen, Int>): Float {
        ui.runOnIdle { nav.value = to; Snapshot.sendApplyNotifications() }
        ui.mainClock.advanceTimeBy(100)
        return ui.onNodeWithTag(to.first.name).fetchSemanticsNode().positionInRoot.x
    }

    @Test fun endavantEntraPerLaDreta() {
        host(Screen.Company to 0)
        assertTrue(switchTo(Screen.Chat to 1) > 0f)
    }

    @Test fun enrereEntraPerLEsquerra() {
        host(Screen.Chat to 1)
        assertTrue(switchTo(Screen.Company to 0) < 0f)
    }

    @Test fun ambLaPilaIgualNomesFaFos() {
        host(Screen.Ventet to 1)
        assertEquals(0f, switchTo(Screen.Company to 1), 0.01f)
    }

    @Test fun senseMovimentLaVellaMarxaDeSeguida() {
        host(Screen.Company to 0, motion = false)
        ui.runOnIdle { nav.value = Screen.Chat to 1; Snapshot.sendApplyNotifications() }
        ui.mainClock.advanceTimeByFrame()
        ui.onNodeWithTag(Screen.Company.name).assertDoesNotExist()
        assertEquals(0f, ui.onNodeWithTag(Screen.Chat.name).fetchSemanticsNode().positionInRoot.x, 0.01f)
    }
}
