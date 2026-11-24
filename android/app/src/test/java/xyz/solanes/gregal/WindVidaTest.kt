package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.getBoundsInRoot
import androidx.compose.ui.test.junit4.v2.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.dp
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** 100: els avatars parpellegen, miren, reaccionen al dit i celebren (o s'arronsen) en acabar un torn. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp-xxhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindVidaTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()
    private val accent = Color(0xFFEFA68F)

    @Test fun blinksComeAFewSecondsApartAndDifferPerWind() {
        for (seed in listOf(1, 7, 42, 1234, 99999)) {
            val blinks = windBlinkTimes(seed)
            assertTrue("at least two blinks in the 12 s loop: $blinks", blinks.size >= 2)
            assertTrue(blinks.all { it in 0f..12f })
            blinks.zipWithNext { a, b -> assertTrue("gap ${b - a} in $blinks", b - a in 2f..5f) }
        }
        assertNotEquals(windBlinkTimes(1), windBlinkTimes(2))
    }

    @Test fun glancesLookAwayNowAndThenAndDifferPerWind() {
        val glances = windGlances(7)
        assertTrue(glances.isNotEmpty())
        val looks = (0 until 1200).map { windGlanceAt(it / 100f, glances) }
        assertTrue("mostly looking ahead", looks.count { it.x == 0f && it.y == 0f } > 600)
        assertTrue("now and then aside", looks.any { it.x != 0f })
        assertNotEquals(windGlances(1), windGlances(2))
    }

    private fun show(motion: Boolean, content: @Composable () -> Unit) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides motion) {
                MaterialTheme(colorScheme = LightPal.scheme()) { content() }
            }
        }
    }

    private fun shot(tag: String): Bitmap {
        val r = ui.onNodeWithTag(tag).getBoundsInRoot()
        val px = ui.activity.resources.displayMetrics.density
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        return Bitmap.createBitmap(bmp, (r.left.value * px).toInt(), (r.top.value * px).toInt(),
            ((r.right - r.left).value * px).toInt(), ((r.bottom - r.top).value * px).toInt())
    }

    private fun idleFrames(motion: Boolean): List<Bitmap> {
        show(motion) { Box(Modifier.background(Color.White).testTag("face")) { WindFace(size = 96, accent = accent) } }
        return (0 until 6).map { ui.mainClock.advanceTimeBy(1300); shot("face") }
    }

    @Test fun anIdleFaceLivesWithMotion() {
        val frames = idleFrames(motion = true)
        assertTrue(frames.zipWithNext().any { (a, b) -> !a.sameAs(b) })
    }

    @Test fun anIdleFaceStaysStillWithoutMotion() {
        val frames = idleFrames(motion = false)
        assertTrue(frames.zipWithNext().all { (a, b) -> a.sameAs(b) })
    }

    @Test fun aTappedFaceWigglesAndSpringsBack() {
        // Two twins: same colour and style, so the same clock draws them alike until one is tapped.
        show(motion = true) {
            Row {
                Box(Modifier.testTag("a").background(Color.White).padding(12.dp)) { WindFace(size = 96, accent = accent, tappable = true) }
                Box(Modifier.testTag("b").background(Color.White).padding(12.dp)) { WindFace(size = 96, accent = accent, tappable = true) }
            }
        }
        ui.mainClock.advanceTimeBy(500)
        fun diff(): Int {
            val a = shot("a"); val b = shot("b")
            if (a.width != b.width || a.height != b.height) return -1
            return (0 until a.width).sumOf { x -> (0 until a.height).count { y -> a.getPixel(x, y) != b.getPixel(x, y) } }
        }
        val d0 = diff()
        ui.onNodeWithTag("a").performClick()
        ui.mainClock.advanceTimeBy(160)
        val d1 = diff()
        ui.mainClock.advanceTimeBy(3000)
        val d2 = diff()
        // A few edge pixels differ between two places on the screen; the wiggle moves thousands.
        assertTrue("twins $d0 / tapped $d1 / back $d2", d0 in 0..40 && d1 > 2000 && d2 in 0..40)
    }

    /** Painted pixels around the 72 dp face, outside its own box: where only the burst reaches. */
    private fun around(motion: Boolean, from: WindMood, to: WindMood, then: WindMood? = null): Int {
        var mood by mutableStateOf(from)
        show(motion) {
            Box(Modifier.testTag("stage").background(Color.White).padding(48.dp)) { WindFace(size = 72, mood = mood, accent = accent) }
        }
        ui.mainClock.advanceTimeBy(400)
        mood = to
        androidx.compose.runtime.snapshots.Snapshot.sendApplyNotifications()
        ui.mainClock.advanceTimeBy(320)
        if (then != null) {
            mood = then
            androidx.compose.runtime.snapshots.Snapshot.sendApplyNotifications()
            ui.mainClock.advanceTimeBy(1500)
        }
        val bmp = shot("stage")
        val px = ui.activity.resources.displayMetrics.density
        val inner = (46 * px).toInt()..(bmp.width - 46 * px).toInt()
        var painted = 0
        for (x in 0 until bmp.width) for (y in 0 until bmp.height) {
            if (x in inner && y in inner) continue
            if (bmp.getPixel(x, y) != android.graphics.Color.WHITE) painted++
        }
        return painted
    }

    @Test fun aTurnThatEndsWellBurstsOnce() {
        assertTrue(around(motion = true, from = WindMood.WORKING, to = WindMood.DONE) > 40)
    }

    @Test fun aFailedTurnShrugsWithoutConfetti() {
        // From IDLE: a working face's aura reaches the edge of its box, and the shrug tilts it a little past it.
        assertEquals(0, around(motion = true, from = WindMood.IDLE, to = WindMood.FAILED))
    }

    @Test fun noBurstWithoutMotion() {
        assertEquals(0, around(motion = false, from = WindMood.WORKING, to = WindMood.DONE))
    }

    @Test fun aBurstCutShortByANewTurnStillEnds() {
        assertEquals(0, around(motion = true, from = WindMood.IDLE, to = WindMood.DONE, then = WindMood.IDLE))
    }

    @Test fun aFaceShownDoneAlreadyDoesNotBurst() {
        assertEquals(0, around(motion = true, from = WindMood.DONE, to = WindMood.DONE))
    }

    /** Rightmost column of [tag] (from [top] to [bottom] dp, or all of it) that differs from its own right edge. */
    private fun rightEdge(tag: String, top: Float? = null, bottom: Float? = null): Int {
        val bmp = shot(tag)
        val px = ui.activity.resources.displayMetrics.density
        val rows = ((top ?: 0f) * px).toInt() until ((bottom?.times(px))?.toInt() ?: bmp.height)
        return rows.maxOf { y ->
            val bg = bmp.getPixel(bmp.width - 1, y)
            (bmp.width - 1 downTo 0).firstOrNull { bmp.getPixel(it, y) != bg } ?: -1
        }
    }

    private val xaloc = Wind("xaloc", "Xaloc", "#EFA68F", "Música i concerts")

    private fun typingWidths(motion: Boolean): Pair<Int, Int> {
        show(motion) {
            Box(Modifier.testTag("typing").background(Color.White).width(360.dp)) { WindTypingBubble("ca", xaloc, accent, WindMood.THINKING) }
        }
        ui.mainClock.advanceTimeBy(100)
        val rest = rightEdge("typing")
        ui.mainClock.advanceTimeBy(1200)
        return rest to rightEdge("typing")
    }

    @Test fun theTypingBubbleBreathes() {
        val (rest, full) = typingWidths(motion = true)
        assertTrue("$rest < $full", rest < full)
    }

    @Test fun theTypingBubbleIsStillWithoutMotion() {
        val (rest, full) = typingWidths(motion = false)
        assertEquals(rest, full)
    }

    private fun workCardFrames(motion: Boolean): Pair<Bitmap, Bitmap> {
        show(motion) {
            Box(Modifier.testTag("work").background(Color.White).width(360.dp).padding(vertical = 40.dp)) {
                WindWorkCard("ca", accent, null, DocRef("d1", "Carta al veí"), onComputer = {}, onDoc = {})
            }
        }
        ui.mainClock.advanceTimeBy(48)
        val early = shot("work")
        ui.mainClock.advanceTimeBy(1500)
        return early to shot("work")
    }

    @Test fun theWorkCardSlidesUp() {
        val (early, late) = workCardFrames(motion = true)
        assertFalse(early.sameAs(late))
    }

    @Test fun theWorkCardIsThereAtOnceWithoutMotion() {
        val (early, late) = workCardFrames(motion = false)
        assertTrue(early.sameAs(late))
    }

    /** The right edge of a new reply 48 ms after it arrives and once it has settled. */
    private fun replyEdges(motion: Boolean): Pair<Int, Int> {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        vm.setConn(Conn("127.0.0.1", "9", "t"))
        val chats = vm.windChat as MutableStateFlow<Map<String, List<GregalViewModel.PetMsg>>>
        val hola = GregalViewModel.PetMsg("user", "Hola")
        chats.value = mapOf(xaloc.id to listOf(hola))
        show(motion) { Box(Modifier.testTag("chat")) { WindConversation(vm, "ca", xaloc, onBack = {}) } }
        ui.mainClock.advanceTimeBy(500)
        chats.value = mapOf(xaloc.id to listOf(hola, GregalViewModel.PetMsg("pet", "Una resposta prou llarga per veure-la créixer")))
        ui.mainClock.advanceTimeBy(48)
        val r = ui.onNodeWithText("Una resposta prou llarga per veure-la créixer").getBoundsInRoot()
        val early = rightEdge("chat", r.top.value, r.bottom.value)
        ui.mainClock.advanceTimeBy(800)
        return early to rightEdge("chat", r.top.value, r.bottom.value)
    }

    @Test fun aNewReplySpringsInFromTheWindsSide() {
        val (early, late) = replyEdges(motion = true)
        assertTrue("$early < $late", early < late)
    }

    @Test fun withoutMotionANewReplyIsThereAtOnce() {
        val (early, late) = replyEdges(motion = false)
        assertEquals(early, late)
    }

    @Test fun aFailedTurnMovesTheFace() {
        var mood by mutableStateOf(WindMood.WORKING)
        // A list-sized face has no idle loop, so only the shrug can move it.
        show(motion = true) {
            Box(Modifier.testTag("stage").background(Color.White).padding(24.dp)) { WindFace(size = 40, mood = mood, accent = accent) }
        }
        ui.mainClock.advanceTimeBy(400)
        mood = WindMood.FAILED
        androidx.compose.runtime.snapshots.Snapshot.sendApplyNotifications()
        ui.mainClock.advanceTimeBy(400)
        val mid = shot("stage")
        ui.mainClock.advanceTimeBy(1200)
        val end = shot("stage")
        assertFalse("the face droops", mid.sameAs(end))
        ui.mainClock.advanceTimeBy(500)
        assertTrue("and settles", shot("stage").sameAs(end))
    }
}
