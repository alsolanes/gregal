package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.getBoundsInRoot
import androidx.compose.ui.test.junit4.v2.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.unit.dp
import java.io.File
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** 111: the Cares faces redesigned, a silhouette per wind, eyes that speak and a motion per mood. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w720dp-h1280dp-xxhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindCaresTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val six = listOf(
        "Tramuntana" to "#5B8DEF" to "pso", "Xaloc" to "#EFA68F" to "bss", "Bit" to "#29ADFF" to "eon",
        "Garbí" to "#F2C14E" to "mto", "Llevant" to "#3FAE73" to "pon", "Mestral" to "#8E6CD9" to "bts",
    ).map { (nc, code) -> Wind(nc.first.lowercase(), nc.first, nc.second, "r", style = code) }

    private val light = Color(0xFFF6F4EF)
    private val dark = Color(0xFF15131C)

    private fun shot(tag: String): Bitmap {
        val r = ui.onNodeWithTag(tag).getBoundsInRoot()
        val px = ui.activity.resources.displayMetrics.density
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        return Bitmap.createBitmap(bmp, (r.left.value * px).toInt(), (r.top.value * px).toInt(),
            ((r.right - r.left).value * px).toInt(), ((r.bottom - r.top).value * px).toInt())
    }

    /** Frames for design review go to $GREGAL_UI_REVIEW when it is set; the tests never need them. */
    private fun keep(name: String, bmp: Bitmap) {
        val dir = System.getenv("GREGAL_UI_REVIEW") ?: return
        File(dir).mkdirs()
        File(dir, "$name.png").outputStream().use { bmp.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private val glances = windGlances(7)
    private fun poses(m: WindMood) = (0 until 240).map { windPose(m, it * .05f, glances) }

    /** The moods with a motion of their own: idle, thinking, searching/reading, working/writing, waiting, failed, done, news. */
    private val signatures = listOf(WindMood.IDLE, WindMood.THINKING, WindMood.SEARCHING, WindMood.READING, WindMood.WRITING,
        WindMood.WORKING, WindMood.WAITING, WindMood.FAILED, WindMood.DONE, WindMood.NEWS)

    @Test fun everyMoodMovesItsOwnWay() {
        for (m in signatures) assertTrue("$m moves", poses(m).toSet().size > 10)
        for (a in signatures) for (b in signatures) if (a != b) assertNotEquals("$a vs $b", poses(a), poses(b))
    }

    @Test fun eachMoodHasItsSignature() {
        val idle = poses(WindMood.IDLE)
        val thinking = poses(WindMood.THINKING)
        assertTrue("thinking looks up", thinking.all { it.gy < -2f })
        assertTrue("thinking tilts slowly", thinking.maxOf { it.tilt } - thinking.minOf { it.tilt } > 3f)
        for (m in listOf(WindMood.SEARCHING, WindMood.READING)) {
            assertTrue("$m scans both ways", poses(m).let { p -> p.maxOf { it.gx } > 2f && p.minOf { it.gx } < -2f })
        }
        assertTrue("searching peers forward", poses(WindMood.SEARCHING).all { it.x > 1f })
        assertTrue("reading goes down the lines", poses(WindMood.READING).let { p -> p.maxOf { it.gy } - p.minOf { it.gy } > 1.5f })
        val work = poses(WindMood.WORKING)
        assertTrue("working squints with determination", work.all { it.lid > .3f && it.brow > 10f })
        // Count the bobs (y is down): a working wind bounces several times a second, an idle one breathes in place.
        fun bobs(p: List<WindPose>) = p.zipWithNext().count { (a, b) -> a.y >= -1f && b.y < -1f }
        assertTrue("working bobs fast (${bobs(work)} vs ${bobs(idle)})", bobs(work) >= 4 * maxOf(1, bobs(idle)))
        assertTrue("its feet step", (0 until 30).map { windStep(WindMood.WORKING, it * .05f, false) }.toSet().size > 5 &&
            windStep(WindMood.IDLE, .2f, false) == 0f)
        val waiting = poses(WindMood.WAITING)
        assertTrue("waiting leans toward you and looks out at you", waiting.all { it.x < 0f && it.tilt < -5f && it.gy > 0f && it.lid == 0f })
        assertTrue("and leans gently in and out", waiting.maxOf { it.tilt } - waiting.minOf { it.tilt } > 2f)
        val failed = poses(WindMood.FAILED)
        assertTrue("failed droops with sad lids", failed.all { it.y > 2f && it.sy < .95f && it.brow < -10f && it.gy > 2f && it.lid > .3f })
        assertTrue("and sighs", failed.maxOf { it.sy } - failed.minOf { it.sy } > .02f)
        assertTrue("done is content", poses(WindMood.DONE).all { it.smile == .5f })
        assertTrue("news hops hello", poses(WindMood.NEWS).minOf { it.y } < -3f)
    }

    @Test fun doneAnticipatesHopsAndSettles() {
        // y is down: a hop is negative.
        val hop = (0..100).map { windDoneHop(it / 100f) }
        assertEquals(Triple(0f, 1f, 1f), hop.first())
        val firstLift = hop.indexOfFirst { it.first < -.5f }
        assertTrue("it squashes before it leaves the ground", hop.take(firstLift).any { it.third < .9f && it.second > 1.05f })
        assertTrue("a real hop", hop.minOf { it.first } < -8f)
        val landed = hop.drop(firstLift).indexOfFirst { it.first == 0f } + firstLift
        val after = hop.drop(landed)
        assertTrue("lands with a squash", after.any { it.third < .95f })
        assertTrue("springs past rest before it settles", after.any { it.third > 1.01f })
        val last = hop.last()
        assertEquals(0f, last.first, .01f); assertEquals(1f, last.second, .01f); assertEquals(1f, last.third, .01f)
    }

    @Test fun aMoodBlendsIntoTheNext() {
        val a = windPose(WindMood.THINKING, 2f)
        val b = windPose(WindMood.WORKING, 2f)
        assertEquals(a, windPoseBetween(a, b, 0f))
        assertEquals(b, windPoseBetween(a, b, 1f))
        val mid = windPoseBetween(a, b, .5f)
        assertEquals((a.gy + b.gy) / 2f, mid.gy, 1e-4f)
        assertEquals((a.lid + b.lid) / 2f, mid.lid, 1e-4f)
        assertNotEquals(a, mid); assertNotEquals(b, mid)
    }

    private fun diff(a: Bitmap, b: Bitmap) =
        (0 until a.width).sumOf { x -> (0 until a.height).count { y -> a.getPixel(x, y) != b.getPixel(x, y) } }

    @Test fun aMoodChangeSpringsInsteadOfSnapping() {
        ui.mainClock.autoAdvance = false
        var mood by mutableStateOf(WindMood.THINKING)
        val w = six[0]
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides true) {
                Row(Modifier.background(light)) {
                    Box(Modifier.testTag("moved").size(120.dp), contentAlignment = Alignment.Center) {
                        WindFace(size = 96, mood = mood, accent = Color(w.argb), style = w.faceStyle, tappable = true)
                    }
                    Box(Modifier.testTag("there").size(120.dp), contentAlignment = Alignment.Center) {
                        WindFace(size = 96, mood = WindMood.WORKING, accent = Color(w.argb), style = w.faceStyle, tappable = true)
                    }
                }
            }
        }
        ui.mainClock.advanceTimeBy(600)
        mood = WindMood.WORKING
        androidx.compose.runtime.snapshots.Snapshot.sendApplyNotifications()
        ui.mainClock.advanceTimeBy(64)
        val early = diff(shot("moved"), shot("there"))
        ui.mainClock.advanceTimeBy(2000)
        val late = diff(shot("moved"), shot("there"))
        assertTrue("on its way ($early) and then there ($late)", early > 2000 && late < 60)
    }

    @Test fun withMotionOffEveryFaceIsStill() {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false) {
                Row(Modifier.background(light)) {
                    for (half in WindMood.entries.chunked(6)) Column {
                        for (m in half) Box(Modifier.testTag(m.name).size(100.dp), contentAlignment = Alignment.Center) {
                            WindFace(size = 96, mood = m, accent = Color(six[1].argb), style = six[1].faceStyle, tappable = true)
                        }
                    }
                }
            }
        }
        ui.mainClock.advanceTimeBy(300)
        val before = WindMood.entries.associateWith { shot(it.name) }
        ui.mainClock.advanceTimeBy(1700)
        for (m in WindMood.entries) assertTrue("$m is still", before.getValue(m).sameAs(shot(m.name)))
    }

    /** Pixels close to the face's ink in the left and right halves of its eye band. */
    private fun eyeInk(b: Bitmap, ink: Int): Pair<Int, Int> {
        fun near(p: Int) = kotlin.math.abs(android.graphics.Color.red(p) - android.graphics.Color.red(ink)) +
            kotlin.math.abs(android.graphics.Color.green(p) - android.graphics.Color.green(ink)) +
            kotlin.math.abs(android.graphics.Color.blue(p) - android.graphics.Color.blue(ink)) < 60
        var l = 0; var r = 0
        // Above the mouth, which starts just past the middle.
        for (y in b.height / 4 until b.height * 53 / 100) for (x in b.width / 5 until b.width * 4 / 5) {
            if (near(b.getPixel(x, y))) { if (x < b.width / 2) l++ else r++ }
        }
        return l to r
    }

    @Test fun smallFacesKeepTwoLargeCrispEyes() {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false) {
                Column {
                    for ((i, bg) in listOf(light, dark).withIndex()) Row(Modifier.background(bg)) {
                        for (w in six) Box(Modifier.testTag("${w.id}-$i")) {
                            WindFace(size = 24, accent = Color(w.argb), style = Wind(w.id, w.name, w.color, "r").faceStyle)
                        }
                    }
                }
            }
        }
        ui.mainClock.advanceTimeBy(100)
        val eyes = (0..1).flatMap { i -> six.map { w ->
            val face = shot("${w.id}-$i")
            val ink = windInk(Color(w.argb)).let { android.graphics.Color.rgb((it.red * 255).toInt(), (it.green * 255).toInt(), (it.blue * 255).toInt()) }
            Triple(w.name, face.width * face.height, eyeInk(face, ink))
        } }
        // A 24 dp face is 72 px square here: each eye is a solid dark mark (its shine aside) of at least 1.2 % of it, not a speck.
        assertTrue("$eyes", eyes.all { (_, area, lr) -> minOf(lr.first, lr.second) >= area * 12 / 1000 })
    }

    @Composable private fun sheet(bg: Color, tag: String) {
        Column(Modifier.testTag(tag).background(bg).padding(8.dp)) {
            for (size in listOf(24, 28, 40, 96)) Row(verticalAlignment = Alignment.CenterVertically) {
                for (w in six) Box(Modifier.size((size + 16).dp), contentAlignment = Alignment.Center) {
                    WindFace(size = size, accent = Color(w.argb), style = w.faceStyle)
                }
            }
        }
    }

    @Test fun sixWindsSmallAndLargeOnLightAndDark() {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false) {
                Column { sheet(light, "light"); sheet(dark, "dark") }
            }
        }
        ui.mainClock.advanceTimeBy(100)
        keep("sheet-light", shot("light"))
        keep("sheet-dark", shot("dark"))
    }

    private val strip = listOf(WindMood.IDLE, WindMood.HAPPY, WindMood.THINKING, WindMood.SEARCHING, WindMood.READING,
        WindMood.WRITING, WindMood.WORKING, WindMood.WAITING, WindMood.NEWS, WindMood.FAILED, WindMood.DONE, WindMood.TALKING)

    @Test fun everyMoodFilmstrip() {
        ui.mainClock.autoAdvance = false
        var switched by mutableStateOf(WindMood.THINKING)
        val w = six[0]
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides true) {
                Row(Modifier.background(light)) {
                    for (half in strip.chunked(6)) Column {
                        for (m in half) Box(Modifier.testTag(m.name).size(120.dp), contentAlignment = Alignment.Center) {
                            WindFace(size = 96, mood = m, accent = Color(w.argb), style = w.faceStyle, tappable = true)
                        }
                    }
                    Box(Modifier.testTag("switch").size(120.dp), contentAlignment = Alignment.Center) {
                        WindFace(size = 96, mood = switched, accent = Color(w.argb), style = w.faceStyle, tappable = true)
                    }
                }
            }
        }
        val frames = 12
        val rows = strip.map { it.name } + "switch"
        val cells = rows.associateWith { mutableListOf<Bitmap>() }
        for (f in 0 until frames) {
            ui.mainClock.advanceTimeBy(if (f == 0) 16 else 140)
            if (f == 3) { switched = WindMood.WORKING; androidx.compose.runtime.snapshots.Snapshot.sendApplyNotifications() }
            for (r in rows) cells.getValue(r) += shot(r)
        }
        val cw = cells.getValue(rows[0])[0].width
        val ch = cells.getValue(rows[0])[0].height
        val out = Bitmap.createBitmap(cw * frames, ch * rows.size, Bitmap.Config.ARGB_8888)
        val c = android.graphics.Canvas(out)
        rows.forEachIndexed { y, r -> cells.getValue(r).forEachIndexed { x, b -> c.drawBitmap(b, (x * cw).toFloat(), (y * ch).toFloat(), null) } }
        keep("moods", out)
    }
}
