package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** Les taques del fons Vidre es mouen amb moviment i queden quietes sense. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w200dp-h300dp-mdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class AuroraTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private fun frame(): IntArray {
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        return IntArray(bmp.width * bmp.height).also { bmp.getPixels(it, 0, bmp.width, 0, 0, bmp.width, bmp.height) }
    }

    /** Dos fotogrames separats 3 s. */
    private fun twoFrames(motion: Boolean): Pair<IntArray, IntArray> {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides DarkPal.glassy(), LocalWindMotion provides motion) {
                SeaBackdrop(Modifier.fillMaxSize()) { Box(Modifier.fillMaxSize()) }
            }
        }
        ui.mainClock.advanceTimeBy(500)
        val a = frame()
        repeat(60) { ui.mainClock.advanceTimeBy(50); ui.waitForIdle() }
        return a to frame()
    }

    @Test fun ambMovimentLesTaquesEsMouen() {
        val (a, b) = twoFrames(motion = true)
        assertNotEquals(a.toList(), b.toList())
    }

    @Test fun senseMovimentLesTaquesQuedenQuietes() {
        val (a, b) = twoFrames(motion = false)
        assertEquals(a.toList(), b.toList())
    }
}
