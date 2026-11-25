package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.Row
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.junit4.v2.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
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

/** 114: les cares grans són els vents de plastilina (112), tenyits del color del vent; les petites, la cara de Canvas. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindClayTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()
    private val orange = Color(0xFFFF9F00)

    private fun show(motion: Boolean = false, dir: String = "characters", content: @androidx.compose.runtime.Composable () -> Unit) = ui.setContent {
        CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides motion, LocalWindClay provides dir) {
            MaterialTheme(colorScheme = LightPal.scheme()) { content() }
        }
    }

    private fun clay(sheet: String) = ui.onNode(SemanticsMatcher.expectValue(WindClayKey, sheet), useUnmergedTree = true)
    private fun anyClay() = ui.onAllNodes(SemanticsMatcher.keyIsDefined(WindClayKey), useUnmergedTree = true)
    private fun anyFace() = ui.onAllNodes(SemanticsMatcher.keyIsDefined(WindFaceKey), useUnmergedTree = true)
    // captureToImage waits for a redraw Robolectric does not give: the window is painted by hand and cut to the node.
    private fun pixels(tag: String): Bitmap {
        val r = ui.onNodeWithTag(tag, useUnmergedTree = true).fetchSemanticsNode().boundsInWindow
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        return Bitmap.createBitmap(bmp, r.left.toInt(), r.top.toInt(), r.width.toInt(), r.height.toInt())
    }

    @Test fun aLargeFacePlaysItsSilhouetteAndSwitchesMood() {
        var mood by mutableStateOf(WindMood.IDLE)
        show {
            Row {
                WindFace(size = 96, mood = mood, accent = orange, clay = true)
                WindFace(size = 96, mood = mood, accent = orange, style = windStyleOf("pss"), clay = true)
                WindFace(size = 96, mood = WindMood.HAPPY, accent = orange, style = windStyleOf("psn"), clay = true)
            }
        }
        clay("pebble/idle").assertExists()
        clay("curl/idle").assertExists()
        clay("cloud/idle").assertExists()
        mood = WindMood.THINKING
        ui.waitForIdle()
        clay("pebble/thinking").assertExists()
        clay("curl/thinking").assertExists()
        clay("pebble/idle").assertDoesNotExist()
        // The clay face is still the wind's face: one per wind for 109, with its colour.
        anyFace().assertCountEquals(3)
    }

    @Test fun smallAvatarsAndMissingSheetsKeepTheCanvasFace() {
        show {
            Row {
                WindFace(size = 40, accent = orange)
                WindHalo(orange, WindStyle(), size = 40)
            }
        }
        anyFace().assertCountEquals(2)
        anyClay().assertCountEquals(0)
    }

    @Test fun aMissingSheetFallsBackToTheCanvasFace() {
        show(dir = "no-such-folder") { WindFace(size = 96, accent = orange, clay = true) }
        anyFace().assertCountEquals(1)
        anyClay().assertCountEquals(0)
    }

    @Test fun aMinairoWithoutSheetsKeepsItsDrawing() {
        show(dir = "no-such-folder") {
            CompositionLocalProvider(LocalAvatarTheme provides "minairo") { WindFace(size = 96, accent = orange, clay = true) }
        }
        anyFace().assertCountEquals(1)
        anyClay().assertCountEquals(0)
    }

    /** The same face with motion on and off, now and half a second later. */
    private fun frames(): Map<String, Bitmap> {
        ui.mainClock.autoAdvance = false
        show {
            Row {
                WindFace(size = 96, accent = orange, clay = true, modifier = Modifier.testTag("still"))
                CompositionLocalProvider(LocalWindMotion provides true) {
                    WindFace(size = 96, accent = orange, clay = true, modifier = Modifier.testTag("playing"))
                }
            }
        }
        ui.mainClock.advanceTimeByFrame()
        val now = mapOf("still" to pixels("still"), "playing" to pixels("playing"))
        ui.mainClock.advanceTimeBy(500)
        return now + mapOf("still later" to pixels("still"), "playing later" to pixels("playing"))
    }

    @Test fun withMotionTheSheetPlays() {
        val f = frames()
        assertFalse("half a second later the idle sheet is on another frame", f.getValue("playing").sameAs(f.getValue("playing later")))
    }

    @Test fun motionOffShowsTheFirstFrame() {
        val f = frames()
        assertTrue("motion off does not move", f.getValue("still").sameAs(f.getValue("still later")))
        assertTrue("and it is the sheet's first frame", f.getValue("still").sameAs(f.getValue("playing")))
    }

    private fun moodsOfManifest() = org.json.JSONObject(ui.activity.assets.open("characters/manifest.json").bufferedReader().use { it.readText() })
        .getJSONObject("moods")

    /** 142: while a turn works, the big wind handles the prop of its work kind, then goes back to talk and idle. */
    @Test fun eachWorkKindPlaysTheSheetWithItsProp() {
        val moods = moodsOfManifest()
        var mood by mutableStateOf(WindMood.IDLE)
        show { WindFace(size = 96, mood = mood, accent = orange, clay = true) }
        for ((kind, prop) in listOf(
            windMoodOf(state = "working", activity = "grep") to "magnifier", windMoodOf(phase = "searching") to "magnifier",
            windMoodOf(state = "working", activity = "read") to "book", windMoodOf(state = "working", activity = "edit") to "pencil",
            windMoodOf(writing = true) to "pencil", windMoodOf(state = "working", activity = "bash") to "laptop",
            windMoodOf(state = "working", activity = "plan") to "gear", windMoodOf(phase = "thinking") to "gear",
            windMoodOf(state = "completed") to "confetti",
        )) {
            mood = kind
            ui.waitForIdle()
            val name = windClayMood(kind)
            clay("pebble/$name").assertExists()
            assertEquals("$kind holds its prop", prop, moods.getJSONObject(name).optString("prop"))
        }
        for ((back, sheet) in listOf(WindMood.TALKING to "pebble/discussing", WindMood.IDLE to "pebble/idle")) {
            mood = back
            ui.waitForIdle()
            clay(sheet).assertExists()
            assertEquals("", moods.getJSONObject(sheet.substringAfter('/')).optString("prop"))
        }
    }

    @Test fun motionOffHoldsTheStillFrameWithTheProp() {
        val still = moodsOfManifest().getJSONObject("searching").getInt("still")
        assertTrue("the still frame is not the first", still > 0)
        ui.mainClock.autoAdvance = false
        show {
            Row {
                WindFace(size = 96, mood = WindMood.SEARCHING, accent = orange, clay = true, modifier = Modifier.testTag("still"))
                CompositionLocalProvider(LocalWindMotion provides true) {
                    WindFace(size = 96, mood = WindMood.SEARCHING, accent = orange, clay = true, modifier = Modifier.testTag("playing"))
                }
            }
        }
        ui.mainClock.advanceTimeByFrame()
        val first = pixels("playing")
        assertFalse("motion off is not on the first frame", pixels("still").sameAs(first))
        ui.mainClock.advanceTimeBy(still * 1000L / 12 + 40)
        assertTrue("it is the still frame the playing face reaches", pixels("still").sameAs(pixels("playing")))
    }

    /** Average colour of the body: the opaque pixels of the face. */
    private fun body(accent: Color): FloatArray {
        show { WindFace(size = 96, accent = accent, clay = true, modifier = Modifier.testTag("f")) }
        val bmp = pixels("f")
        val sum = FloatArray(3)
        var n = 0
        for (y in 0 until bmp.height) for (x in 0 until bmp.width) {
            val p = bmp.getPixel(x, y)
            if (android.graphics.Color.alpha(p) < 250) continue
            sum[0] += android.graphics.Color.red(p).toFloat(); sum[1] += android.graphics.Color.green(p).toFloat(); sum[2] += android.graphics.Color.blue(p).toFloat()
            n++
        }
        assertTrue("the face has a body", n > 500)
        return FloatArray(3) { sum[it] / n }
    }

    @Test fun theBodyTakesTheWindColourThroughTheMask() {
        val red = body(Color(0xFFE0453A))
        assertTrue("a red wind is red: ${red.toList()}", red[0] > red[2] + 40f)
    }

    @Test fun aBlueWindIsBlue() {
        val blue = body(Color(0xFF3A6FE0))
        assertTrue("a blue wind is blue: ${blue.toList()}", blue[2] > blue[0] + 40f)
    }

    /** Opaque pixels of a clay minairó in an orange wind wearing the barretina of [style]'s fourth letter that are far bluer or greener than red. */
    private fun capPixels(style: String): Int {
        show {
            CompositionLocalProvider(LocalAvatarTheme provides "minairo") {
                WindFace(size = 96, accent = orange, style = windStyleOf(style), clay = true, modifier = Modifier.testTag("f"))
            }
        }
        clay("minairo/idle").assertExists()
        val bmp = pixels("f")
        var n = 0
        for (y in 0 until bmp.height) for (x in 0 until bmp.width) {
            val p = bmp.getPixel(x, y)
            val (r, g, b) = Triple(android.graphics.Color.red(p), android.graphics.Color.green(p), android.graphics.Color.blue(p))
            if (android.graphics.Color.alpha(p) > 250 && maxOf(g, b) > r + 25) n++
        }
        return n
    }

    @Test fun aClayMinairoWearsItsOwnBarretinaColour() {
        assertTrue("a navy barretina is blue on the clay minairo", capPixels("psob") > 150)
    }

    @Test fun aClayMinairoInARedBarretinaHasNoBlueCap() {
        assertTrue("a red barretina and an orange body have nothing blue or green", capPixels("psov") < 30)
    }

    /** Painted pixels around a 72 dp clay face, outside its own box: where only the 100 burst reaches. */
    private fun burst(motion: Boolean): Int {
        var mood by mutableStateOf(WindMood.WORKING)
        ui.mainClock.autoAdvance = false
        show(motion) {
            Box(Modifier.testTag("stage").background(Color.White).padding(48.dp)) { WindFace(size = 72, mood = mood, accent = orange, clay = true) }
        }
        ui.mainClock.advanceTimeBy(400)
        mood = WindMood.DONE
        androidx.compose.runtime.snapshots.Snapshot.sendApplyNotifications()
        ui.mainClock.advanceTimeBy(320)
        clay("pebble/done").assertExists()
        val bmp = pixels("stage")
        val px = ui.activity.resources.displayMetrics.density
        val inner = (46 * px).toInt()..(bmp.width - 46 * px).toInt()
        var painted = 0
        for (x in 0 until bmp.width) for (y in 0 until bmp.height) {
            if (x in inner && y in inner) continue
            if (bmp.getPixel(x, y) != android.graphics.Color.WHITE) painted++
        }
        return painted
    }

    @Test fun aClayFaceStillCelebrates() {
        assertTrue(burst(motion = true) > 40)
    }

    @Test fun noClayBurstWithoutMotion() {
        assertEquals(0, burst(motion = false))
    }

    /** A few frames to look at (output/ui-review/114/): every silhouette in two colours, some moods, a mood change. */
    @Test fun framesToLookAt() {
        val dir = java.io.File("../../output/ui-review/114").also { it.mkdirs() }
        fun save(name: String) = java.io.File(dir, "$name.png").outputStream().use { pixels("strip").compress(Bitmap.CompressFormat.PNG, 100, it) }
        var mood by mutableStateOf(WindMood.IDLE)
        ui.mainClock.autoAdvance = false
        show(motion = true) {
            Column(Modifier.testTag("strip")) {
                for (c in listOf(Color(0xFF4FA3D9), Color(0xFFEFA68F))) Row {
                    for (shape in listOf("pso", "pss", "psn")) WindFace(size = 96, mood = mood, accent = c, style = windStyleOf(shape), clay = true)
                    // A small avatar next to them keeps the Canvas face.
                    WindFace(size = 40, mood = mood, accent = c)
                }
            }
        }
        for (m in listOf(WindMood.IDLE, WindMood.WORKING, WindMood.WAITING, WindMood.DONE, WindMood.FAILED)) {
            mood = m
            ui.mainClock.advanceTimeBy(400)
            for (i in 0 until 4) {
                save("${m.name.lowercase()}-$i")
                ui.mainClock.advanceTimeBy(1000L / 12)
            }
        }
        mood = WindMood.THINKING
        for (i in 0 until 4) { ui.mainClock.advanceTimeBy(60); save("crossfade-$i") }
        assertEquals(true, dir.listFiles()!!.isNotEmpty())
    }
}
