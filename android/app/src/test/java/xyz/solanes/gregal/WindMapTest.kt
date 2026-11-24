package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import android.os.Looper
import java.time.Duration
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import kotlin.math.atan2
import kotlin.math.PI
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** Mapa de vent: the web's simulation, offered in both pickers, drawn by WindFace and still without motion. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindMapTest {
    @get:Rule val ui = createComposeRule()

    private fun style(id: String, name: String) = Wind(id, name, "#29ADFF", "r", avatar = "mapa").faceStyle
    private fun steps(st: MapState, n: Int, busy: (Int) -> Boolean = { false }): MapState {
        var s = st
        repeat(n) { s = mapStep(s, busy(it)) }
        return s
    }
    private fun near(expected: List<Double>, actual: List<Double>) =
        expected.zip(actual).forEach { (e, a) -> assertEquals(e, a, 1e-9) }

    @Test fun `the simulation gives the numbers the web gives for the same wind`() {
        // From mapStart/mapStep in internal/web/app/winds.js, run in node; 200 steps with every third one working.
        val ponent = mapStart(style("ponent", "Ponent"))
        assertEquals(7.853981633974483, ponent.phi, 1e-12)
        assertEquals(3.4, ponent.seed, 1e-12)
        assertEquals(3913883810L, ponent.r.toLong() and 0xffffffffL)
        assertEquals(90, ponent.ps.size)
        near(listOf(0.1282672598026693, 0.7484504841268063, 90.09374149609357), ponent.ps[0].let { listOf(it.x, it.y, it.life) })
        near(listOf(0.38970981258898973, 0.9049001671373844, 66.04525248520076), ponent.ps[89].let { listOf(it.x, it.y, it.life) })
        val one = mapStep(ponent, false)
        near(listOf(0.1282672598026693, 0.7484504841268063, 0.13314289947875174, 0.7509957027321544, 0.0), one.lines[0].let { listOf(it.x0, it.y0, it.x1, it.y1, it.alpha) })
        near(listOf(0.7107420680113137, 0.01094554434530437, 0.7155309888952979, 0.013650398642108524, 0.0), one.lines[5].let { listOf(it.x0, it.y0, it.x1, it.y1, it.alpha) })
        val later = steps(one, 199) { it % 3 == 0 }
        assertEquals(3.2000000000000024, later.t, 1e-12)
        assertEquals(1088649339L, later.r.toLong() and 0xffffffffL)
        assertEquals(50, later.ps[0].age)
        near(listOf(0.7710416411004306, 0.549851882519476, 57.84128388855606), later.ps[0].let { listOf(it.x, it.y, it.life) })
        near(listOf(0.9815646503883306, 0.6497874986497727, 0.9944000955246786, 0.6477256188880501, 0.4999813125236502),
            later.lines[7].let { listOf(it.x0, it.y0, it.x1, it.y1, it.alpha) })

        val bit = steps(mapStart(style("bit", "Bit")), 200) { it % 3 == 1 }
        assertEquals(4278997539L, bit.r.toLong() and 0xffffffffL)
        near(listOf(0.36811428326119605, -0.033111233035330145, 91.09828332439065), bit.ps[0].let { listOf(it.x, it.y, it.life) })

        // A wind not saved yet (the editor's preview) gets the picture its id will give it.
        val unsaved = mapStart(style("", "Lupa Ràpida"))
        assertEquals(3904214846L, unsaved.r.toLong() and 0xffffffffL)
        assertEquals(mapStart(style("lupa-rapida", "Lupa Ràpida")), unsaved)
    }

    @Test fun `the streaks blow away from where the wind comes from`() {
        // Ponent blows from the west, so its streaks run east: +x on the picture.
        var st = mapStart(style("ponent", "Ponent"))
        var dx = 0.0
        var dy = 0.0
        repeat(40) {
            st = mapStep(st, false)
            for (l in st.lines) {
                assertEquals("every streak within the swirl of its course", PI / 2, atan2(l.x1 - l.x0, l.y0 - l.y1), .61)
                dx += l.x1 - l.x0; dy += l.y1 - l.y0
            }
        }
        assertEquals("on average due east", PI / 2, atan2(dx, -dy), .1)
        // Tramuntana blows from the north: down the picture.
        val down = mapStep(mapStart(style("t", "Tramuntana")), false).lines[0]
        assertTrue(kotlin.math.abs(atan2(down.x1 - down.x0, down.y0 - down.y1)) > PI - .61)
    }

    @Test fun `Mapa de vent is offered in Preferences and saved as the theme`() {
        FakeGregal { """{"theme":"mapa"}""" }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(Conn("127.0.0.1", server.port, "token", "alice"))
            vm.openPreferences(PrefsPage.Appearance)
            ui.setContent {
                CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                    MaterialTheme(colorScheme = LightPal.scheme()) { PrefsScreen(vm) {} }
                }
            }
            val lang = vm.idioma.value
            ui.onNodeWithText(tr(lang, "winds.avatarMapa")).performScrollTo().performClick()
            repeat(150) { if (vm.avatarTheme.value != "mapa") { shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(20)); Thread.sleep(5) } }
            assertEquals("mapa", vm.avatarTheme.value)
            val post = server.requests.single { it.startsWith("POST /api/winds/avatar-theme ") }
            assertEquals("mapa", JSONObject(post.substringAfter("avatar-theme ")).getString("theme"))
        }
        assertEquals("mapa", avatarThemeOf("""{"avatarTheme":"mapa"}"""))
    }

    @Test fun `in Free mode the editor offers Mapa de vent and saves it with the wind`() {
        var saved: WindDraft? = null
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false, LocalAvatarTheme provides "free") {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    WindEditor("ca", Wind("bit", "Bit", "#29ADFF", "lectura"), {}, { _, _, _, _ -> }, { draft, _ -> saved = draft })
                }
            }
        }
        assertEquals("Mapa de vent", tr("ca", "winds.avatarMapa"))
        assertEquals("Wind map", tr("en", "winds.avatarMapa"))
        ui.onNodeWithText(tr("ca", "winds.avatarMapa")).performScrollTo().performClick()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle { assertEquals("mapa", saved?.avatar) }
    }

    @Test fun `WindFace draws the map when the theme or the wind's own avatar is mapa`() {
        val bit = style("bit", "Bit")
        assertEquals("mapa", windPack("free", bit))
        assertEquals("cares", windPack("free", bit.copy(pack = "")))
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false) {
                Row(Modifier.background(Color.Black)) {
                    for ((tag, theme, pack) in listOf(Triple("cares", "cares", ""), Triple("theme", "mapa", ""), Triple("own", "free", "mapa"))) {
                        CompositionLocalProvider(LocalAvatarTheme provides theme) {
                            Box(Modifier.testTag(tag)) { WindFace(size = 96, accent = Color(0xFF29ADFF), style = bit.copy(pack = pack)) }
                        }
                    }
                }
            }
        }
        assertFalse(identical(image("theme"), image("cares")))
        assertTrue("The same wind gets the same picture", identical(image("theme"), image("own")))
    }

    @Test fun `with motion off a map is one still frame, even while its wind works`() {
        ui.mainClock.autoAdvance = false
        val bit = style("bit", "Bit")
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalAvatarTheme provides "free") {
                Row(Modifier.background(Color.Black)) {
                    Box(Modifier.testTag("idle")) { WindFace(size = 96, accent = Color(0xFF29ADFF), style = bit) }
                    Box(Modifier.testTag("working")) { WindFace(size = 96, mood = WindMood.WORKING, accent = Color(0xFF29ADFF), style = bit) }
                }
            }
        }
        ui.mainClock.advanceTimeBy(100)
        val idle = image("idle")
        val working = image("working")
        ui.mainClock.advanceTimeBy(300)
        assertTrue("Without motion an idle map does not move", identical(idle, image("idle")))
        assertTrue("Without motion a working map does not move either", identical(working, image("working")))
    }

    @Test fun `with motion on a working map blows`() {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides true, LocalAvatarTheme provides "free") {
                Box(Modifier.testTag("working").background(Color.Black)) {
                    WindFace(size = 96, mood = WindMood.WORKING, accent = Color(0xFF29ADFF), style = style("bit", "Bit"))
                }
            }
        }
        ui.mainClock.advanceTimeBy(100)
        val working = image("working")
        ui.mainClock.advanceTimeBy(300)
        // The left half holds the streaks and not the mood badge (top right).
        fun streaks(b: Bitmap) = Bitmap.createBitmap(b, 0, 0, b.width / 2, b.height)
        assertFalse("Work moves the streaks", identical(streaks(working), streaks(image("working"))))
    }

    // The same capture as WindEsperitTest: draw the Compose view and cut the node out.
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
}
