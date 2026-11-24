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

    /** A minairó as winds.js sees a saved wind: its id, the bearing it blows from and its style code. */
    private fun styleOf(id: String, code: String = "pso") = windStyleOf(code).copy(windId = id, bearing = windDirection(id, id))

    @Test fun `each wind gets its own minairo, the same one every time`() {
        val names = listOf("tramuntana", "gregal", "llevant", "xaloc", "migjorn", "garbi", "ponent", "mestral", "bit", "lupa", "boira", "pessic")
        val mouths = listOf("s", "o", "n", "t")
        val styles = names.mapIndexed { i, id -> styleOf(id, "p" + mouths[i % 4] + "o") }
        val body = Color(0xFFA8CDE2)
        val looks = styles.map { minairoLook(it, body) }
        assertEquals(looks, styles.map { minairoLook(it, body) })
        assertEquals("both fold sides", setOf(-1, 1), looks.map { it.fold }.toSet())
        assertEquals("a beard length per mouth", 4, looks.map { it.beard }.toSet().size)
        assertEquals("needle case, St John's wort and pebble", 3, looks.map { it.gear }.toSet().size)
    }

    private data class Pick(val id: String, val seed: Int, val onBlue: Color, val onPink: Color, val brows: String, val moustache: String, val glasses: Boolean)

    // 120's notes: the barretina from the id hash and the body hue, the traits from the face seed.
    @Test fun `each minairo wears the barretina and traits the web gives the same wind`() {
        val v = Color(0xFFC9342C); val m = Color(0xFF6E2F6B); val f = Color(0xFF2E6B3F)
        val o = Color(0xFFC8962E); val b = Color(0xFF26406E); val g = Color(0xFF7D1F33)
        val picks = listOf(
            Pick("llevant", 293, v, m, "straight", "walrus", false), Pick("xaloc", 61, m, m, "bushy", "lobes", false),
            Pick("migjorn", 251, v, m, "straight", "lobes", false), Pick("garbi", 347, m, m, "arched", "lobes", false),
            Pick("ponent", 300, v, m, "straight", "lobes", false), Pick("mestral", 828, f, f, "bushy", "lobes", false),
            Pick("tramuntana", 83, o, b, "straight", "lobes", true), Pick("gregal", 118, f, f, "straight", "handlebar", false),
            Pick("minairo-1", 302, g, m, "arched", "handlebar", false), Pick("minairo-2", 303, v, m, "bushy", "lobes", false))
        for (p in picks) {
            val look = minairoLook(styleOf(p.id), Color(0xFFA8CDE2))
            assertEquals("${p.id} on blue", p.onBlue, look.cap)
            assertEquals("${p.id} on pink", p.onPink, minairoLook(styleOf(p.id), Color(0xFFEFA68F)).cap)
            assertEquals(p.id, listOf(p.brows, p.moustache, p.glasses, p.seed % 3), listOf(look.brows, look.moustache, look.glasses, look.gear))
        }
        // East winds fold to the left, west ones to the right; north and south by the seed (both odd here).
        assertEquals(listOf(-1, -1, -1, 1, 1, 1, 1, 1), listOf("gregal", "llevant", "xaloc", "garbi", "ponent", "mestral", "tramuntana", "migjorn")
            .map { minairoLook(styleOf(it), Color(0xFFA8CDE2)).fold })
        assertEquals(listOf(63.5f, 67.5f, 71f, 74.5f), listOf("n", "s", "o", "t").map { minairoLook(styleOf("xaloc", "p${it}o"), Color(0xFFA8CDE2)).beard })
    }

    @Test fun `the fourth letter of the style is the barretina the person picked`() {
        val palette = listOf("v" to 0xFFC9342C, "m" to 0xFF6E2F6B, "f" to 0xFF2E6B3F, "o" to 0xFFC8962E, "b" to 0xFF26406E, "g" to 0xFF7D1F33)
        // No hue check on a choice: a red barretina over a pink body is the person's call.
        for ((letter, hex) in palette) {
            val style = styleOf("llevant", "pso$letter")
            assertEquals(letter, style.barretina)
            assertEquals("pso$letter", style.encode())
            assertEquals(Color(hex), minairoLook(style, Color(0xFFEFA68F)).cap)
        }
        assertEquals("an unknown letter is automatic", "pso", windStyleOf("psox").encode())
        assertEquals(minairoLook(styleOf("llevant"), Color(0xFFA8CDE2)), minairoLook(styleOf("llevant", "psox"), Color(0xFFA8CDE2)))
        assertEquals(listOf("Automàtica", "Vermella", "Morada", "Verd fosc", "Mostassa", "Blau marí", "Granat"),
            (listOf("") + palette.map { it.first }).map { tr("ca", barretinaLabelKey(it)) })
        assertEquals(listOf("Automatic", "Red", "Purple", "Dark green", "Mustard", "Navy blue", "Garnet"),
            (listOf("") + palette.map { it.first }).map { tr("en", barretinaLabelKey(it)) })
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

    /** A pixel in the barretina's own colour: its hue, and neither the dark ground nor a washed-out tint. */
    private fun capLike(cap: Color): (Int) -> Boolean {
        val hsv = FloatArray(3).also { android.graphics.Color.colorToHSV(android.graphics.Color.rgb((cap.red * 255).toInt(), (cap.green * 255).toInt(), (cap.blue * 255).toInt()), it) }
        return { p ->
            val q = FloatArray(3).also { android.graphics.Color.colorToHSV(p, it) }
            val d = Math.abs(q[0] - hsv[0]).let { minOf(it, 360f - it) }
            d < 15f && q[1] > .35f && q[2] > .25f
        }
    }
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
            // A 24 dp face is 72 px square here: the cap is a mass of its colour on top and the beard a white one below.
            val inCap = capLike(minairoLook(w.faceStyle, Color(w.argb)).cap)
            val cap = count(face, 0f, .42f, inCap)
            val beard = count(face, .55f, 1f, ::white)
            assertTrue("${w.name}: cap on top ($cap of $area)", cap > area * 6 / 100)
            assertTrue("${w.name}: no cap colour below", count(face, .6f, 1f, inCap) < cap / 10)
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

    // The eight winds of the web review sheet (120), to set side by side with it.
    private val rose = listOf(
        "tramuntana" to "#A8CDE2" to "pso", "xaloc" to "#EFA68F" to "eto", "llevant" to "#E8AEC2" to "pto", "mestral" to "#BFCFA4" to "esn",
        "gregal" to "#C1B4DE" to "mso", "ponent" to "#EACF87" to "mon", "migjorn" to "#9ED3C5" to "bss", "garbi" to "#F5B45D" to "pso",
    ).map { (ic, code) -> Wind(ic.first, ic.first.replaceFirstChar { it.uppercase() }, ic.second, "r", style = code, avatar = "minairo") }

    @Composable private fun sheet(bg: Color, tag: String) {
        Column(Modifier.testTag(tag).background(bg).padding(8.dp)) {
            // Four to a row at 120 dp, so the sheet fits the test screen.
            for (size in listOf(24, 48, 120)) for (row in rose.chunked(if (size > 100) 4 else 8)) Row(verticalAlignment = Alignment.CenterVertically) {
                for (w in row) Box(Modifier.size((size + 16).dp), contentAlignment = Alignment.Center) {
                    WindFace(size = size, accent = Color(w.argb), style = w.faceStyle)
                }
            }
        }
    }

    @Test fun `the eight rose minairons small and large on dark and light`() {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalAvatarTheme provides "minairo") {
                Column { sheet(Color(0xFF17161C), "dark"); sheet(Color(0xFFFAF8F4), "light") }
            }
        }
        ui.mainClock.advanceTimeBy(100)
        keep("sizes-dark", shot("dark"))
        keep("sizes-light", shot("light"))
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
