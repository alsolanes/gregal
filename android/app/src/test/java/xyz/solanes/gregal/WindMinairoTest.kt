package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import android.os.Looper
import java.time.Duration
import androidx.compose.material3.MaterialTheme
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.core.app.ApplicationProvider
import org.json.JSONObject
import org.robolectric.Shadows.shadowOf
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
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** 118: the minairó pack, a Catalan minairó with a red barretina and a white beard, in every mood. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w720dp-h1280dp-xxhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindMinairoTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val six = listOf(
        "Tramuntana" to "#5B8DEF" to "pso", "Xaloc" to "#EFA68F" to "bss", "Bit" to "#29ADFF" to "eon",
        "Garbí" to "#F2C14E" to "mto", "Llevant" to "#3FAE73" to "pon", "Mestral" to "#8E6CD9" to "bts",
    ).map { (nc, code) -> Wind(nc.first.lowercase(), nc.first, nc.second, "r", style = code, avatar = "minairo") }

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

    @Test fun `a wind whose own avatar is minairo is drawn as one in Free mode`() {
        assertTrue("minairo" in windPacks)
        assertEquals("minairo", windPack("free", six[0].faceStyle))
        assertEquals("minairo", windPack("minairo", six[0].faceStyle.copy(pack = "")))
        assertEquals("minairo", avatarThemeOf("""{"avatarTheme":"minairo"}"""))
        assertEquals("Minairons", tr("ca", avatarLabelKey("minairo")))
        assertEquals("Minairons", tr("en", avatarLabelKey("minairo")))
    }

    @Test fun `the minairo pack is offered in Preferences with its sample and saved as the theme`() {
        FakeGregal { """{"theme":"minairo"}""" }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(Conn("127.0.0.1", server.port, "t"))
            vm.openPreferences(PrefsPage.Appearance)
            ui.setContent {
                CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                    MaterialTheme(colorScheme = LightPal.scheme()) { PrefsScreen(vm) {} }
                }
            }
            ui.onNodeWithTag("avatarSample-minairo", useUnmergedTree = true).performScrollTo().assertExists()
            ui.onNodeWithText(tr(vm.idioma.value, "winds.avatarMinairo")).performScrollTo().performClick()
            repeat(150) { if (vm.avatarTheme.value != "minairo") { shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(20)); Thread.sleep(5) } }
            assertEquals("minairo", vm.avatarTheme.value)
            val post = server.requests.single { it.startsWith("POST /api/winds/avatar-theme ") }
            assertEquals("minairo", JSONObject(post.substringAfter("avatar-theme ")).getString("theme"))
        }
    }

    @Test fun `each wind gets its own minairo, the same one every time`() {
        val names = listOf("tramuntana", "gregal", "llevant", "xaloc", "migjorn", "garbi", "ponent", "mestral", "bit", "lupa", "boira", "pessic")
        val looks = names.map { minairoLook(it) }
        assertEquals(looks, names.map { minairoLook(it) })
        assertEquals("both fold sides", setOf(-1, 1), looks.map { it.fold }.toSet())
        assertEquals("three beard lengths", 3, looks.map { it.beard }.toSet().size)
        assertEquals("needle case, St John's wort and pebble", 3, looks.map { it.gear }.toSet().size)
    }

    private fun diff(a: Bitmap, b: Bitmap) =
        (0 until a.width).sumOf { x -> (0 until a.height).count { y -> a.getPixel(x, y) != b.getPixel(x, y) } }

    @Test fun `every mood draws differently and motion off keeps it still`() {
        ui.mainClock.autoAdvance = false
        val w = six[0]
        var mood by mutableStateOf(WindMood.IDLE)
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalAvatarTheme provides "minairo") {
                // One spot for every mood: gradient dithering depends on where a face is drawn.
                Box(Modifier.testTag("face").size(100.dp).background(light), contentAlignment = Alignment.Center) {
                    WindFace(size = 96, mood = mood, accent = Color(w.argb), style = w.faceStyle, tappable = true)
                }
            }
        }
        val frames = WindMood.entries.associateWith { m ->
            mood = m
            androidx.compose.runtime.snapshots.Snapshot.sendApplyNotifications()
            // Past the aura's fade in, which is a change of state and not motion.
            ui.mainClock.advanceTimeBy(600)
            val before = shot("face")
            ui.mainClock.advanceTimeBy(1700)
            assertTrue("$m is still", before.sameAs(shot("face")))
            before
        }
        for (a in WindMood.entries) for (b in WindMood.entries) if (a < b) {
            assertTrue("$a and $b look different", diff(frames.getValue(a), frames.getValue(b)) > 40)
        }
    }

    @Test fun `with motion the barretina bobs and the beard sways`() {
        ui.mainClock.autoAdvance = false
        val w = six[0]
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides true, LocalAvatarTheme provides "minairo") {
                Box(Modifier.testTag("idle").size(120.dp).background(light), contentAlignment = Alignment.Center) {
                    WindFace(size = 96, mood = WindMood.IDLE, accent = Color(w.argb), style = w.faceStyle)
                }
            }
        }
        ui.mainClock.advanceTimeBy(100)
        val a = shot("idle")
        ui.mainClock.advanceTimeBy(900)
        val b = shot("idle")
        // The figure breathes too; the cap above it and the beard below it move on their own.
        fun band(bmp: Bitmap, from: Float, to: Float) = Bitmap.createBitmap(bmp, 0, (bmp.height * from).toInt(), bmp.width, (bmp.height * (to - from)).toInt())
        assertFalse("the barretina moves", band(a, .08f, .3f).sameAs(band(b, .08f, .3f)))
        assertFalse("the beard moves", band(a, .62f, .78f).sameAs(band(b, .62f, .78f)))
    }

    /** Pixels of [b] in the band [from]..[to] of its height that pass [test]. */
    private fun count(b: Bitmap, from: Float, to: Float, test: (Int) -> Boolean) =
        ((b.height * from).toInt() until (b.height * to).toInt()).sumOf { y -> (0 until b.width).count { x -> test(b.getPixel(x, y)) } }

    private fun red(p: Int) = android.graphics.Color.red(p) > 150 && android.graphics.Color.green(p) < 90 && android.graphics.Color.blue(p) < 90
    private fun white(p: Int) = android.graphics.Color.red(p) > 215 && android.graphics.Color.green(p) > 210 && android.graphics.Color.blue(p) > 200

    @Test fun `at 24 dp it still reads as a barretina over a beard`() {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalAvatarTheme provides "minairo") {
                Row(Modifier.background(dark)) {
                    for (w in six) Box(Modifier.testTag(w.id)) { WindFace(size = 24, accent = Color(w.argb), style = w.faceStyle) }
                }
            }
        }
        ui.mainClock.advanceTimeBy(100)
        for (w in six) {
            val face = shot(w.id)
            val area = face.width * face.height
            // A 24 dp face is 72 px square here: the cap is a red mass on top and the beard a white one below.
            val cap = count(face, 0f, .42f, ::red)
            val beard = count(face, .55f, 1f, ::white)
            assertTrue("${w.name}: red cap on top ($cap of $area)", cap > area * 6 / 100)
            assertTrue("${w.name}: no red below", count(face, .6f, 1f, ::red) < cap / 10)
            assertTrue("${w.name}: white beard below ($beard of $area)", beard > area * 6 / 100)
        }
    }

    @Test fun `the clay player plays the minairo sheets`() {
        ui.mainClock.autoAdvance = false
        val w = six[0]
        var clay by mutableStateOf(false)
        var theme by mutableStateOf("minairo")
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalAvatarTheme provides theme) {
                // The same spot each time: gradient dithering depends on where a face is drawn.
                Box(Modifier.testTag("face").size(140.dp).background(light), contentAlignment = Alignment.Center) {
                    WindFace(size = 128, accent = Color(w.argb), style = w.faceStyle.copy(shape = WindShape.BALL, kit = ""), clay = clay)
                }
            }
        }
        fun frame(): Bitmap { androidx.compose.runtime.snapshots.Snapshot.sendApplyNotifications(); ui.mainClock.advanceTimeBy(100); return shot("face") }
        val drawn = frame()
        clay = true
        val minairo = frame()
        assertFalse("the minairó turns to clay", drawn.sameAs(minairo))
        ui.onNode(SemanticsMatcher.expectValue(WindClayKey, "minairo/idle"), useUnmergedTree = true).assertExists()
        // Its own sheets, not the pebble of its shape that Cares plays.
        theme = "cares"
        assertFalse(minairo.sameAs(frame()))
        ui.onNode(SemanticsMatcher.expectValue(WindClayKey, "pebble/idle"), useUnmergedTree = true).assertExists()
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

    @Test fun `six minairons small and large on light and dark`() {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalAvatarTheme provides "minairo") {
                Column { sheet(light, "light"); sheet(dark, "dark") }
            }
        }
        ui.mainClock.advanceTimeBy(100)
        keep("sheet-light", shot("light"))
        keep("sheet-dark", shot("dark"))
    }

    @Test fun `every mood filmstrip`() {
        ui.mainClock.autoAdvance = false
        val strip = listOf(WindMood.IDLE, WindMood.HAPPY, WindMood.THINKING, WindMood.SEARCHING, WindMood.READING, WindMood.WRITING,
            WindMood.WORKING, WindMood.DISCUSSING, WindMood.WAITING, WindMood.NEWS, WindMood.FAILED, WindMood.DONE, WindMood.TALKING)
        val w = six[0]
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides true, LocalAvatarTheme provides "minairo") {
                Row(Modifier.background(light)) {
                    for (half in strip.chunked(7)) Column {
                        for (m in half) Box(Modifier.testTag(m.name).size(120.dp), contentAlignment = Alignment.Center) {
                            WindFace(size = 96, mood = m, accent = Color(w.argb), style = w.faceStyle, tappable = true)
                        }
                    }
                }
            }
        }
        val frames = 12
        val cells = strip.associateWith { mutableListOf<Bitmap>() }
        for (f in 0 until frames) {
            ui.mainClock.advanceTimeBy(if (f == 0) 16 else 110)
            for (m in strip) cells.getValue(m) += shot(m.name)
        }
        val cw = cells.getValue(strip[0])[0].width
        val ch = cells.getValue(strip[0])[0].height
        val out = Bitmap.createBitmap(cw * frames, ch * strip.size, Bitmap.Config.ARGB_8888)
        val c = android.graphics.Canvas(out)
        strip.forEachIndexed { y, m -> cells.getValue(m).forEachIndexed { x, b -> c.drawBitmap(b, (x * cw).toFloat(), (y * ch).toFloat(), null) } }
        keep("moods", out)
    }
}
