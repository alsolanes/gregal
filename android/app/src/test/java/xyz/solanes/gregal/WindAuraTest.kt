package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** L'aura del color del vent només marca feina en marxa, i només a les cares grans. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindAuraTest {
    @get:Rule val ui = createComposeRule()
    private val accent = Color(0xFFFF7900)
    private val working = setOf(
        WindMood.THINKING, WindMood.SEARCHING, WindMood.READING,
        WindMood.WRITING, WindMood.WORKING, WindMood.DISCUSSING,
    )

    private fun image(tag: String): Bitmap {
        val bounds = ui.onNodeWithTag(tag).fetchSemanticsNode().boundsInRoot
        lateinit var bitmap: Bitmap
        ui.runOnIdle {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).single()
            fun composeView(view: android.view.View): android.view.View? {
                if (view.javaClass.simpleName == "AndroidComposeView") return view
                if (view is android.view.ViewGroup) for (i in 0 until view.childCount) {
                    composeView(view.getChildAt(i))?.let { return it }
                }
                return null
            }
            val view = requireNotNull(composeView(activity.window.decorView))
            val root = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
            view.draw(android.graphics.Canvas(root))
            bitmap = Bitmap.createBitmap(root, bounds.left.toInt(), bounds.top.toInt(), bounds.width.toInt(), bounds.height.toInt())
        }
        return bitmap
    }

    private fun identical(a: Bitmap, b: Bitmap): Boolean {
        if (a.width != b.width || a.height != b.height) return false
        for (y in 0 until a.height) for (x in 0 until a.width) {
            if (a.getPixel(x, y) != b.getPixel(x, y)) return false
        }
        return true
    }

    /**
     * Llum taronja a la franja esquerra de l'anell, fora del cos (el còdol de 111 fa .39 de la caixa) i
     * lluny de la insígnia (a dalt a la dreta) i de l'ombra (a sota), sobre negre.
     */
    private fun auraLight(frame: Bitmap): Int {
        val side = minOf(frame.width, frame.height)
        val cy = frame.height / 2
        var light = 0
        for (y in cy - side / 5..cy + side / 5) for (x in 0 until frame.width / 2 - (side * .40f).toInt()) {
            val p = frame.getPixel(x, y)
            val red = android.graphics.Color.red(p)
            if (red > android.graphics.Color.blue(p) + 8) light += red
        }
        return light
    }

    @Test fun `una cara gran treballant té aura i en repòs no`() {
        for (mood in WindMood.entries) {
            assertEquals(mood.name, mood in working, windShowsAura(mood, size = 96))
            assertEquals(mood.name, mood in working, windShowsAura(mood, size = 64))
        }
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false) {
                Row(Modifier.background(Color.Black)) {
                    Box(Modifier.testTag("thinking")) { WindFace(size = 96, mood = WindMood.THINKING, accent = accent) }
                    Box(Modifier.testTag("idle")) { WindFace(size = 96, accent = accent) }
                }
            }
        }
        assertTrue("Pensant, la llum del vent envolta el cos", auraLight(image("thinking")) > 0)
        assertEquals("En repòs, al voltant del cos no hi ha res", 0, auraLight(image("idle")))
    }

    @Test fun `les cares petites de les llistes no tenen aura`() {
        for (mood in WindMood.entries) {
            assertFalse(mood.name, windShowsAura(mood, size = 63))
            assertFalse(mood.name, windShowsAura(mood, size = 40))
        }
    }

    @Test fun `l'aura apareix i desapareix amb un fos`() {
        ui.mainClock.autoAdvance = false
        var mood by mutableStateOf(WindMood.IDLE)
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false) {
                Box(Modifier.background(Color.Black).testTag("face")) { WindFace(size = 96, mood = mood, accent = accent) }
            }
        }
        ui.mainClock.advanceTimeByFrame()
        assertEquals(0, auraLight(image("face")))
        ui.runOnIdle { mood = WindMood.THINKING }
        // A passos, deixant que el bucle principal lliuri cada fotograma: d'un sol
        // salt de 200 ms el fos només n'engegaria el primer.
        fun play(ms: Int) = repeat(ms / 50) { ui.mainClock.advanceTimeBy(50); ui.waitForIdle() }
        play(200)
        val halfway = auraLight(image("face"))
        play(250)
        val full = auraLight(image("face"))
        assertTrue("A mig fos l'aura ja es veu ($halfway de $full)", halfway > 0 && halfway < full * .95f)
        play(500)
        assertEquals("Passat el fos, l'aura és sencera", full, auraLight(image("face")))
        ui.runOnIdle { mood = WindMood.IDLE }
        play(200)
        val leaving = auraLight(image("face"))
        assertTrue("En acabar la feina s'esvaeix a poc a poc ($leaving de $full)", leaving > 0 && leaving < full)
        play(600)
        assertEquals(0, auraLight(image("face")))
    }

    @Test fun `sense moviment l'aura queda quieta`() {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false) {
                Box(Modifier.background(Color.Black).testTag("face")) { WindFace(size = 96, mood = WindMood.THINKING, accent = accent) }
            }
        }
        ui.mainClock.advanceTimeBy(600)
        val before = image("face")
        assertTrue(auraLight(before) > 0)
        ui.mainClock.advanceTimeBy(1500)
        assertTrue("Sense moviment, l'aura no gira ni batega", identical(before, image("face")))
    }

    @Test fun `amb moviment l'aura gira i batega`() {
        val a = windAuraSway(0f)
        val b = windAuraSway(1.5f)
        assertEquals(0f to 1f, a)
        assertNotEquals(a.first, b.first)
        assertNotEquals(a.second, b.second)
    }
}
