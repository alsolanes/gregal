package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.junit4.v2.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.unit.dp
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** 139: mentre treballa el vent es mou i porta una espurna per feina; sense moviment, quiet. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp-xxhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindBusyMotionTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()
    private val accent = Color(0xFFEFA68F)

    private fun show(motion: Boolean, pal: GregalPalette = LightPal, content: @Composable () -> Unit) = ui.setContent {
        CompositionLocalProvider(LocalPal provides pal, LocalWindMotion provides motion) { content() }
    }

    @Composable private fun face(mood: WindMood, pal: GregalPalette) {
        Box(Modifier.testTag("shot").background(pal.surface).padding(16.dp)) {
            WindFace(size = 96, mood = mood, accent = accent)
        }
    }

    private fun capture(): Bitmap {
        val bounds = ui.onNodeWithTag("shot").fetchSemanticsNode().boundsInRoot
        val view = ui.activity.window.decorView
        val root = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(root)) }
        return Bitmap.createBitmap(root, bounds.left.toInt(), bounds.top.toInt(), bounds.width.toInt(), bounds.height.toInt())
    }

    private fun save(shot: Bitmap, name: String) {
        java.io.File("../../output/ui-review/139/$name.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
            shot.compress(Bitmap.CompressFormat.PNG, 100, it)
        }
    }

    private val work = listOf(WindMood.THINKING, WindMood.SEARCHING, WindMood.READING, WindMood.WRITING, WindMood.WORKING, WindMood.DISCUSSING)

    /** Quatre fotogrames per feina, en clar i en fosc, a output/ui-review/139/. */
    @Test fun everyKindOfWorkMovesAndIsCaptured() {
        var shown by mutableStateOf(WindMood.IDLE to LightPal)
        ui.mainClock.autoAdvance = false
        show(motion = true) { face(shown.first, shown.second) }
        for (mood in work) for ((pal, tone) in listOf(LightPal to "light", DarkPal to "dark")) {
            shown = mood to pal
            // Passat el canvi d'estat (300 ms), els fotogrames són de la feina.
            ui.mainClock.advanceTimeBy(400L)
            val frames = (0 until 4).map { i ->
                if (i > 0) ui.mainClock.advanceTimeBy(230L)
                capture().also { save(it, "${mood.name.lowercase()}-$tone-$i") }
            }
            assertTrue("$mood $tone moves", frames.zipWithNext().all { (a, b) -> !a.sameAs(b) })
        }
    }

    @Test fun withoutMotionItKeepsStill() {
        ui.mainClock.autoAdvance = false
        show(motion = false) { face(WindMood.WRITING, LightPal) }
        ui.mainClock.advanceTimeBy(16L)
        val a = capture()
        ui.mainClock.advanceTimeBy(700L)
        assertTrue(a.sameAs(capture()))
    }

    @Test fun aRealListFlipsOnlyPastTheMargins() {
        lateinit var list: LazyListState
        var up = false
        show(motion = false) {
            list = rememberLazyListState()
            up = rememberWindUp(list)
            LazyColumn(Modifier.height(400.dp), state = list, contentPadding = PaddingValues(bottom = 82.dp)) {
                items(60) { Box(Modifier.height(50.dp).fillMaxWidth()) }
            }
        }
        val dp = ui.density.density
        fun scroll(byDp: Float) { ui.runOnIdle { list.dispatchRawDelta(byDp * dp) }; ui.waitForIdle() }
        scroll(100_000f)
        assertFalse("at the end", up)
        scroll(-150f)
        assertFalse("150 dp up", up)
        scroll(-100f)
        assertTrue("250 dp up", up)
        scroll(180f)
        assertTrue("70 dp from the end", up)
        scroll(50f)
        assertFalse("20 dp from the end", up)
    }
}
