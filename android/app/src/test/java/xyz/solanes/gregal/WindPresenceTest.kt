package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.v2.createAndroidComposeRule
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** 088: el vent present a la cantonada del xat, amb l'humor del seu estat. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindPresenceTest {
    // v2: amb el vent ocupat, el GregalWorkStrip consulta el servidor a Dispatchers.IO; la v1 hi
    // continuava els efectes, i el scrollToItem del xat mesurava fora del fil principal mentre la prova dibuixava.
    @get:Rule val ui =createAndroidComposeRule<androidx.activity.ComponentActivity>()
    private val xaloc = Wind("xaloc", "Xaloc", "#EFA68F", "Música i concerts")

    @Test fun theMoodFollowsTheChatState() {
        assertEquals(WindMood.SEARCHING, windPresenceMood("winds.state.working", WindMood.SEARCHING))
        // Treballant sempre és un humor de feina, perquè l'aura hi surti.
        assertEquals(WindMood.WORKING, windPresenceMood("winds.state.working", WindMood.TALKING))
        assertEquals(WindMood.WORKING, windPresenceMood("winds.state.working", null))
        assertTrue(windShowsAura(windPresenceMood("winds.state.working", null), WindPresenceSize))
        assertEquals(WindMood.NEWS, windPresenceMood("winds.state.waiting", WindMood.SEARCHING))
        assertEquals(WindMood.DONE, windPresenceMood("winds.state.done", null))
        assertEquals(WindMood.IDLE, windPresenceMood(null, null))
        assertFalse(windShowsAura(windPresenceMood("winds.state.done", null), WindPresenceSize))
    }

    private fun vmWith(msgs: Int): GregalViewModel {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        vm.setConn(Conn("127.0.0.1", "9", "t"))
        val chats = vm.windChat as MutableStateFlow<Map<String, List<GregalViewModel.PetMsg>>>
        chats.value = mapOf(xaloc.id to List(msgs) { i -> GregalViewModel.PetMsg(if (i % 2 == 0) "user" else "pet", "Missatge $i") })
        val rots = vm.rotllanaChat as MutableStateFlow<Map<String, List<GregalViewModel.PetMsg>>>
        rots.value = mapOf("colla" to listOf(GregalViewModel.PetMsg("user", "Hola colla")))
        return vm
    }

    private fun show(motion: Boolean = false, content: @androidx.compose.runtime.Composable () -> Unit) = ui.setContent {
        CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides motion) {
            MaterialTheme(colorScheme = LightPal.scheme()) { content() }
        }
    }

    @Test fun theWindSitsInTheCornerOfItsChat() {
        val vm = vmWith(2)
        show { WindConversation(vm, "ca", xaloc, onBack = {}) }
        ui.onNodeWithTag("wind-presence").assertIsDisplayed()
    }

    @Test fun aRotllanaHasNoCornerWind() {
        val vm = vmWith(2)
        show { RotllanaConversation(vm, "ca", Rotllana("colla", "Colla", listOf("xaloc")), listOf(xaloc), onBack = {}) }
        ui.onNodeWithText("Hola colla").assertExists()
        ui.onNodeWithTag("wind-presence").assertDoesNotExist()
    }

    @Test fun theLastMessageStaysAboveTheWind() {
        val vm = vmWith(40)
        show { WindConversation(vm, "ca", xaloc, onBack = {}) }
        ui.onNode(hasScrollToIndexAction()).performTouchInput { repeat(20) { swipeUp() } }
        ui.waitForIdle()
        val last = ui.onNodeWithText("Missatge 39").assertIsDisplayed().getBoundsInRoot()
        val face = ui.onNodeWithTag("wind-presence").getBoundsInRoot()
        assertTrue("last message ends at ${last.bottom}, wind starts at ${face.top}", last.bottom <= face.top)
    }

    // Dues imatges del vent de la cantonada, separades 1,7 s de rellotge.
    private fun frames(motion: Boolean): Pair<android.graphics.Bitmap, android.graphics.Bitmap> {
        val vm = vmWith(2)
        (vm.windBusy as MutableStateFlow<Set<String>>).value = setOf(xaloc.id)
        ui.mainClock.autoAdvance = false
        show(motion) { WindConversation(vm, "ca", xaloc, onBack = {}) }
        ui.mainClock.advanceTimeBy(800)
        val r = ui.onNodeWithTag("wind-presence").getBoundsInRoot()
        val px = ui.activity.resources.displayMetrics.density
        fun shot(): android.graphics.Bitmap {
            val view = ui.activity.window.decorView
            val bmp = android.graphics.Bitmap.createBitmap(view.width, view.height, android.graphics.Bitmap.Config.ARGB_8888)
            ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
            return android.graphics.Bitmap.createBitmap(bmp, (r.left.value * px).toInt(), (r.top.value * px).toInt(),
                ((r.right - r.left).value * px).toInt(), ((r.bottom - r.top).value * px).toInt())
        }
        val a = shot()
        ui.mainClock.advanceTimeBy(1700)
        return a to shot()
    }

    @Test fun withoutMotionTheWindIsStill() {
        val (a, b) = frames(motion = false)
        assertTrue(a.sameAs(b))
    }

    @Test fun withMotionTheWindMoves() {
        val (a, b) = frames(motion = true)
        assertFalse(a.sameAs(b))
    }
}
