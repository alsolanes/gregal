package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.unit.dp
import kotlin.math.cos
import kotlin.math.hypot
import kotlin.math.sin
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** Cares que bufen: a working Cares face blows downwind, as `blow` in winds.js. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp-xxhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindBreathTest {
    @get:Rule val ui = createComposeRule()

    private val accent = Color(0xFF29ADFF)
    private fun style(name: String) = Wind(name.lowercase(), name, "#29ADFF", "r").faceStyle

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

    /** Pixels that are not the black ground, inside the rectangle [x0, x1) × [y0, y1). */
    private fun ink(b: Bitmap, x0: Int, y0: Int, x1: Int, y1: Int): Int {
        var n = 0
        for (y in y0.coerceAtLeast(0) until y1.coerceAtMost(b.height)) for (x in x0.coerceAtLeast(0) until x1.coerceAtMost(b.width)) {
            if (b.getPixel(x, y) != android.graphics.Color.BLACK) n++
        }
        return n
    }

    /** A 40 dp face on black with 20 dp around it; the margin is 10 % of the face (4 dp). */
    @Composable private fun framed(tag: String, name: String, mood: WindMood, tappable: Boolean = false) {
        Box(Modifier.testTag(tag).background(Color.Black).padding(20.dp)) {
            WindFace(size = 40, mood = mood, accent = accent, style = style(name), tappable = tappable)
        }
    }

    @Test fun `three long lines, or two short ones under 28 px, that leave the face and stay within 10 percent of its box`() {
        assertEquals(3, windBreathLines(40).size)
        assertEquals(2, windBreathLines(24).size)
        assertEquals(3, windBreathLines(28).size)
        for (deg in 0 until 360 step 45) for (size in listOf(24, 40)) for ((d, _) in windBreathLines(size)) {
            val n = Regex("-?[\\d.]+").findAll(d).map { it.value.toFloat() }.toList()
            val a = Math.toRadians(deg + 90.0)
            for (i in n.indices step 2) {
                val x = 40 + n[i] * cos(a) - n[i + 1] * sin(a)
                val y = 40 + n[i] * sin(a) + n[i + 1] * cos(a)
                assertTrue("$deg $d: $x,$y within the face box plus 8 of 80", x in -8.0..88.0 && y in -8.0..88.0)
                assertTrue("$deg $d leaves the face", hypot(x - 40, y - 40) >= 33)
            }
        }
    }

    @Test fun `a working Cares face blows toward where its wind goes, as on the web`() {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false) {
                Row {
                    framed("tramuntana", "Tramuntana", WindMood.WORKING)
                    framed("ponent", "Ponent", WindMood.WORKING)
                    framed("idle", "Tramuntana", WindMood.IDLE)
                    framed("reading", "Tramuntana", WindMood.READING)
                    framed("discussing", "Tramuntana", WindMood.DISCUSSING)
                }
            }
        }
        ui.mainClock.advanceTimeBy(100)
        val north = image("tramuntana")
        val px = north.width / 80f
        fun dp(v: Int) = (v * px).toInt()
        // Face box: 20..60 dp; the breath may use 4 dp around it, plus a pixel of antialiasing.
        assertTrue("Tramuntana blows south", ink(north, dp(20), dp(60) + 1, dp(60), dp(64)) > 0)
        assertEquals("not north", 0, ink(north, 0, 0, north.width, dp(20) - 1))
        assertEquals("nothing beyond the margin", 0, ink(north, 0, dp(64) + 1, north.width, north.height))
        val west = image("ponent")
        assertTrue("Ponent blows east", ink(west, dp(60) + 1, dp(20), dp(64), dp(60)) > 0)
        assertEquals("not west", 0, ink(west, 0, 0, dp(20) - 1, west.height))
        assertEquals("nothing beyond the margin", 0, ink(west, dp(64) + 1, 0, west.width, west.height))
        assertTrue("reading blows too", ink(image("reading"), dp(20), dp(60) + 1, dp(60), dp(64)) > 0)
        for (still in listOf("idle", "discussing")) assertEquals("$still does not blow", 0, ink(image(still), dp(20), dp(60) + 1, dp(60), dp(64)))
        // In Cares the breath replaces the working cog (top right); reading keeps its book.
        assertEquals("no cog", ink(image("idle"), dp(50), dp(20), dp(60), dp(30)), ink(north, dp(50), dp(20), dp(60), dp(30)))
        assertNotEquals("the book stays", ink(image("idle"), dp(50), dp(20), dp(60), dp(30)), ink(image("reading"), dp(50), dp(20), dp(60), dp(30)))
    }

    @Test fun `the breath streams while the wind works, still in a list face, and goes away when it is idle`() {
        ui.mainClock.autoAdvance = false
        val mood = mutableStateOf(WindMood.WORKING)
        ui.setContent {
            Row {
                framed("face", "Tramuntana", mood.value, tappable = true)
                framed("list", "Tramuntana", WindMood.WORKING)
                CompositionLocalProvider(LocalWindMotion provides false) { framed("still", "Tramuntana", WindMood.WORKING) }
            }
        }
        ui.mainClock.advanceTimeBy(100)
        val first = image("face")
        val still = image("still")
        val list = image("list")
        val px = first.width / 80f
        fun below(b: Bitmap) = Bitmap.createBitmap(b, (20 * px).toInt(), (60 * px).toInt() + 1, (40 * px).toInt(), (4 * px).toInt())
        // Brightness along Tramuntana's middle line, which runs south below the
        // face (face units 80..85, a column around x 40; one unit is half a dp).
        fun unit(u: Float) = ((20 + u / 2) * px).toInt()
        fun line(b: Bitmap) = (unit(80f) until unit(85f)).map { y ->
            (unit(38f)..unit(42f)).maxOf { x -> b.getPixel(x, y).let { maxOf(android.graphics.Color.red(it), android.graphics.Color.green(it), android.graphics.Color.blue(it)) } }
        }
        val before = line(first)
        ui.mainClock.advanceTimeBy(300)
        val later = line(image("face"))
        // Dashes with visible gaps travel outward, not a solid stroke whose tip flickers.
        assertTrue("gaps between dashes: $before", before.zipWithNext().any { (a, b) -> b > a + 30 })
        assertNotEquals("the dashes move", before, later)
        assertTrue("with motion off it is shown but still, like the rest of Cares", ink(below(still), 0, 0, Int.MAX_VALUE, Int.MAX_VALUE) > 0)
        assertTrue(below(still).sameAs(below(image("still"))))
        assertTrue("a list face shows it but keeps still, like the rest of Cares in lists", ink(below(list), 0, 0, Int.MAX_VALUE, Int.MAX_VALUE) > 0 && below(list).sameAs(below(image("list"))))
        mood.value = WindMood.IDLE
        ui.mainClock.advanceTimeBy(100)
        assertEquals("back to idle, no breath", 0, ink(below(image("face")), 0, 0, Int.MAX_VALUE, Int.MAX_VALUE))
    }
}
