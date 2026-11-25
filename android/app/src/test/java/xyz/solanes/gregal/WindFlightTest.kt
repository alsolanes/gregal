package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.v2.createAndroidComposeRule
import androidx.compose.ui.unit.DpRect
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode
import kotlin.math.abs

/** 109: al xat d'un vent el vent és un de sol; a l'últim missatge seu a la cantonada i vola a la capçalera quan puges. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindFlightTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()
    private val xaloc = Wind("xaloc", "Xaloc", "#EFA68F", "Música i concerts")

    // On era cada xat viu a tot el procés: cada prova obre el seu pel final i no deixa
    // a les altres classes el xat de Xaloc pujat a dalt.
    @org.junit.Before @org.junit.After fun forgetPositions() = ChatPositions.saved.clear()

    private fun vmWith(msgs: Int): GregalViewModel {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        vm.setConn(Conn("127.0.0.1", "9", "t"))
        val chats = vm.windChat as MutableStateFlow<Map<String, List<GregalViewModel.PetMsg>>>
        chats.value = mapOf(xaloc.id to List(msgs) { i -> GregalViewModel.PetMsg(if (i % 2 == 0) "user" else "pet", "Missatge $i") })
        val rots = vm.rotllanaChat as MutableStateFlow<Map<String, List<GregalViewModel.PetMsg>>>
        rots.value = mapOf("colla" to List(4) { i -> GregalViewModel.PetMsg(if (i % 2 == 0) "user" else "pet", "Colla $i", speakerId = "xaloc", speakerName = "Xaloc") })
        return vm
    }

    private fun show(motion: Boolean = false, content: @androidx.compose.runtime.Composable () -> Unit) = ui.setContent {
        CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides motion) {
            MaterialTheme(colorScheme = LightPal.scheme()) { content() }
        }
    }

    private fun facesOf(w: Wind) = ui.onAllNodes(SemanticsMatcher.expectValue(WindFaceKey, Color(w.argb)), useUnmergedTree = true)
    private fun face() = ui.onNodeWithTag("wind-presence").getBoundsInRoot()
    private fun near(a: DpRect, b: DpRect, what: String) {
        assertTrue("$what: face $a, anchor $b", abs((a.left - b.left).value) < 2f && abs((a.top - b.top).value) < 2f &&
            abs((a.right - b.right).value) < 2f && abs((a.bottom - b.bottom).value) < 2f)
    }
    private fun corner() = ui.onNodeWithTag("wind-presence-corner", useUnmergedTree = true).getBoundsInRoot()
    // La cara de la capçalera fa 38 dp dins un forat de 44 (com WindHalo de 30).
    private fun headerSlot() = ui.onNodeWithTag("chat-header-face", useUnmergedTree = true).getBoundsInRoot()
        .let { DpRect(it.left + 3f.dpv, it.top + 3f.dpv, it.right - 3f.dpv, it.bottom - 3f.dpv) }
    private val Float.dpv get() = androidx.compose.ui.unit.Dp(this)

    private fun scrollUp() = ui.onNode(hasScrollToIndexAction()).performScrollToIndex(0)
    private fun scrollDown() = ui.onNode(hasScrollToIndexAction()).performTouchInput { repeat(20) { swipeUp() } }

    @Test fun aWindChatShowsTheWindOnce() {
        val vm = vmWith(6)
        (vm.windBusy as MutableStateFlow<Set<String>>).value = setOf(xaloc.id)
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.Ask(GregalApi.WindAsk("wa1", "approve", name = "write", what = "write a/b.txt", wind = "xaloc")))
        show { WindConversation(vm, "ca", xaloc, onBack = {}) }
        ui.waitForIdle()
        ui.onNodeWithTag("wind-typing").assertExists()
        ui.onNodeWithTag("approve-card").assertExists()
        facesOf(xaloc).assertCountEquals(1)
    }

    // 103: en contestar-li, és la cara del xat la que assenteix (amunt i avall) o fa que no (de costat).
    private fun answerMoves(allow: Boolean): Pair<Float, Float> {
        val vm = vmWith(6)
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.Ask(GregalApi.WindAsk("wa2", "approve", name = "write", what = "write a/b.txt", wind = "xaloc")))
        show(motion = true) { WindConversation(vm, "ca", xaloc, onBack = {}) }
        ui.waitForIdle()
        ui.mainClock.autoAdvance = false
        ui.onNodeWithTag(if (allow) "ask-allow" else "ask-decline").performClick()
        val during = List(20) { ui.mainClock.advanceTimeByFrame(); face() }
        // La targeta s'ha plegat i la cara pot haver canviat de lloc: el gest es mesura des d'on reposa després.
        ui.mainClock.advanceTimeBy(1500)
        val c = face()
        ui.onNodeWithTag("ask-face-nod", useUnmergedTree = true).assertDoesNotExist()
        ui.onNodeWithTag("ask-face-shake", useUnmergedTree = true).assertDoesNotExist()
        return during.maxOf { abs((it.left - c.left).value) } to during.maxOf { abs((it.top - c.top).value) }
    }

    @Test fun allowingMakesTheChatFaceNod() {
        val (dx, dy) = answerMoves(allow = true)
        assertTrue("nods: dx=$dx dy=$dy", dy > 3f && dx < 1f)
    }

    @Test fun decliningMakesTheChatFaceShake() {
        val (dx, dy) = answerMoves(allow = false)
        assertTrue("shakes: dx=$dx dy=$dy", dx > 3f && dy < 1f)
    }

    @Test fun aRotllanaKeepsTheFaceInItsHeader() {
        val vm = vmWith(2)
        show { RotllanaConversation(vm, "ca", Rotllana("colla", "Colla", listOf("xaloc")), listOf(xaloc), onBack = {}) }
        ui.onNodeWithText("Colla 3").assertExists()
        ui.onNodeWithTag("wind-presence").assertDoesNotExist()
        // Una a la capçalera i, en grup, la petita al costat del nom a cada ratxa de respostes (145).
        val faces = facesOf(xaloc).fetchSemanticsNodes()
        assertEquals(1, faces.count { it.boundsInRoot.top < with(ui.density) { ChatHeaderInset.toPx() } })
        assertEquals(ui.onAllNodesWithTag("wind-byline", useUnmergedTree = true).fetchSemanticsNodes().size, faces.size - 1)
    }

    @Test fun theListMovesTheFaceBetweenTheCornerAndTheHeader() {
        val vm = vmWith(40)
        show { WindConversation(vm, "ca", xaloc, onBack = {}) }
        scrollDown(); ui.waitForIdle()
        near(face(), corner(), "at the newest message")
        scrollUp(); ui.waitForIdle()
        near(face(), headerSlot(), "scrolled up")
        scrollDown(); ui.waitForIdle()
        near(face(), corner(), "back at the bottom")
    }

    // Amb moviment vola: un fotograma després de pujar encara és entremig, i amb la molla arriba.
    private fun flight(motion: Boolean): DpRect {
        val vm = vmWith(40)
        show(motion) { WindConversation(vm, "ca", xaloc, onBack = {}) }
        scrollDown(); ui.waitForIdle()
        ui.mainClock.autoAdvance = false
        scrollUp()
        ui.mainClock.advanceTimeByFrame(); ui.mainClock.advanceTimeByFrame()
        return face()
    }

    @Test fun withMotionItFlies() {
        val early = flight(motion = true)
        assertTrue("two frames in it is still flying: $early", abs((early.top - headerSlot().top).value) > 20f)
        // Fotogrames del vol per mirar-los (output/ui-review/109).
        val dir = java.io.File("build/ui-review/109").also { it.mkdirs() }
        for (i in 0 until 10) {
            java.io.File(dir, "flight-up-$i.png").outputStream().use { shot().compress(android.graphics.Bitmap.CompressFormat.PNG, 100, it) }
            ui.mainClock.advanceTimeBy(40)
        }
        ui.mainClock.advanceTimeBy(1500)
        near(face(), headerSlot(), "landed")
    }

    @Test fun withoutMotionItJumps() {
        near(flight(motion = false), headerSlot(), "without motion it jumps")
    }

    private fun tapOpensProfile(up: Boolean) {
        val vm = vmWith(40)
        show { WindConversation(vm, "ca", xaloc, onBack = {}) }
        scrollDown(); ui.waitForIdle()
        if (up) { scrollUp(); ui.waitForIdle() }
        ui.onNode(isDialog()).assertDoesNotExist()
        ui.onNodeWithTag("wind-presence").performClick()
        ui.waitForIdle()
        ui.onNode(isDialog()).assertExists()
    }

    @Test fun tappingTheFaceInTheCornerOpensTheProfile() = tapOpensProfile(up = false)

    @Test fun tappingTheFaceInTheHeaderOpensTheProfile() = tapOpensProfile(up = true)

    private fun shot(): android.graphics.Bitmap {
        val view = ui.activity.window.decorView
        val bmp = android.graphics.Bitmap.createBitmap(view.width, view.height, android.graphics.Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        return bmp
    }
}
