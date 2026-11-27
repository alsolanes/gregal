package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import android.os.Looper
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import java.time.Duration
import kotlin.math.abs
import kotlin.math.cos
import kotlin.math.sin
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

/** Cap de vent and Rosa de rellotge, as `cap()` and `rosa()` in winds.js; with Esperit, the volume of the packs. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp-xxhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindCapRosaTest {
    @get:Rule val ui = createComposeRule()

    private val accent = Color(0xFF29ADFF)
    private fun style(name: String, pack: String) = Wind(name.lowercase(), name, "#29ADFF", "r", avatar = pack).faceStyle

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

    /** The pixel at (x, y) of the web's 100 box, drawn at 0.8 in the face. */
    private fun at(b: Bitmap, x: Float, y: Float) = b.getPixel((x * .8f / 80f * b.width).toInt(), (y * .8f / 80f * b.height).toInt())

    /**
     * Pixels drawn over the black ground in the 100 box rectangle [x0, x1) × [y0, y1).
     * The contact shadow under a head is ground, not ink: at most half of a dark tone, under 48 on every channel.
     */
    private fun ink(b: Bitmap, x0: Int, y0: Int, x1: Int, y1: Int): Int {
        val k = b.width * .8f / 80f
        var n = 0
        for (y in (y0 * k).toInt() until minOf((y1 * k).toInt(), b.height)) for (x in (x0 * k).toInt() until minOf((x1 * k).toInt(), b.width)) {
            val c = b.getPixel(x, y)
            if (maxOf(android.graphics.Color.red(c), android.graphics.Color.green(c), android.graphics.Color.blue(c)) >= 48) n++
        }
        return n
    }

    private fun faces(motion: Boolean, vararg faces: Pair<String, @Composable () -> Unit>) {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides motion, LocalAvatarTheme provides "free") {
                Row {
                    for ((tag, face) in faces) Box(Modifier.testTag(tag).background(Color.Black)) { face() }
                }
            }
        }
        ui.mainClock.advanceTimeBy(100)
    }

    private fun face(tag: String, name: String, pack: String, size: Int, mood: WindMood = WindMood.IDLE) =
        Pair<String, @Composable () -> Unit>(tag) { WindFace(size = size, mood = mood, accent = accent, style = style(name, pack)) }

    @Test fun `Preferences offers Cap de vent and Rosa de rellotge and saves the theme`() {
        FakeGregal { """{"theme":"rosa"}""" }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(Conn("127.0.0.1", server.port, "token", "alice"))
            vm.openPreferences(PrefsPage.Appearance)
            ui.setContent {
                CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                    MaterialTheme(colorScheme = LightPal.scheme()) { PrefsScreen(vm) {} }
                }
            }
            val lang = vm.idioma.value
            ui.onNodeWithText(tr(lang, "winds.avatarCap")).performScrollTo().assertIsDisplayed()
            ui.onNodeWithText(tr(lang, "winds.avatarRosa")).performScrollTo().performClick()
            repeat(150) { if (vm.avatarTheme.value != "rosa") { shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(20)); Thread.sleep(5) } }
            assertEquals("rosa", vm.avatarTheme.value)
            val post = server.requests.single { it.startsWith("POST /api/winds/avatar-theme ") }
            assertEquals("rosa", JSONObject(post.substringAfter("avatar-theme ")).getString("theme"))
        }
        assertEquals("cap", avatarThemeOf("""{"avatarTheme":"cap"}"""))
        assertEquals("rosa", windPack("free", style("Bit", "rosa")))
    }

    @Test fun `the editor offers both packs in Free mode and saves the one picked`() {
        var saved: WindDraft? = null
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false, LocalAvatarTheme provides "free") {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    WindEditor("ca", Wind("bit", "Bit", "#29ADFF", "lectura"), {}, { _, _, _, _ -> }, { draft, _ -> saved = draft })
                }
            }
        }
        ui.onNodeWithText(tr("ca", "winds.avatarRosa")).performScrollTo().assertIsDisplayed()
        ui.onNodeWithText(tr("ca", "winds.avatarCap")).performScrollTo().performClick()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle { assertEquals("cap", saved?.avatar) }
    }

    @Test fun `WindFace draws Cap de vent and Rosa de rellotge when chosen`() {
        faces(false, face("cares", "Bit", "cares", 40), face("cap", "Bit", "cap", 40), face("rosa", "Bit", "rosa", 40))
        val cares = image("cares")
        val cap = image("cap")
        val rosa = image("rosa")
        assertFalse(cares.sameAs(cap))
        assertFalse(cares.sameAs(rosa))
        assertFalse(cap.sameAs(rosa))
        // Rosa is a dial that fills its box; Cap's head leaves the corners empty.
        assertNotEquals(android.graphics.Color.BLACK, at(rosa, 50f, 2f))
        assertEquals(android.graphics.Color.BLACK, at(cap, 3f, 3f))
    }

    @Test fun `Cap de vent blows to the opposite of the wind's direction`() {
        faces(false, face("tramuntana", "Tramuntana", "cap", 40), face("migjorn", "Migjorn", "cap", 40), face("ponent", "Ponent", "cap", 40))
        // The head (R 25) sits 11 toward where the wind comes from; the breath leaves it on the other side.
        val north = image("tramuntana")
        assertTrue("Tramuntana blows south", ink(north, 48, 70, 52, 98) > 0)
        assertEquals("its head is north, nothing above it", 0, ink(north, 0, 0, 100, 12))
        val south = image("migjorn")
        assertTrue("Migjorn blows north", ink(south, 48, 2, 52, 30) > 0)
        assertEquals("nothing below its head", 0, ink(south, 0, 88, 100, 100))
        val west = image("ponent")
        assertTrue("Ponent blows east", ink(west, 70, 48, 98, 52) > 0)
        assertEquals("nothing west of its head", 0, ink(west, 0, 0, 10, 100))
    }

    @Test fun `Rosa de rellotge points its long tip in the wind's direction`() {
        val winds = listOf("Tramuntana" to 0, "Llevant" to 90, "Migjorn" to 180, "Ponent" to 270)
        faces(false, *winds.map { (name, _) -> face(name, name, "rosa", 96) }.toTypedArray())
        for ((name, deg) in winds) {
            val b = image(name)
            // Inside the right half of the long point (wind colour), 25 from the centre, turned by the bearing.
            val a = Math.toRadians(deg.toDouble())
            fun spot(sign: Float) = at(b, 50f + sign * (1.5f * cos(a) + 25f * sin(a)).toFloat(), 50f + sign * (1.5f * sin(a) - 25f * cos(a)).toFloat())
            fun isAccent(c: Int) = abs(android.graphics.Color.red(c) - 0x29) < 12 && abs(android.graphics.Color.green(c) - 0xAD) < 12 && abs(android.graphics.Color.blue(c) - 0xFF) < 12
            assertTrue("$name points to $deg", isAccent(spot(1f)))
            assertFalse("$name does not point away", isAccent(spot(-1f)))
        }
    }

    @Test fun `below 28 dp both drop the details`() {
        faces(false, face("cap24", "Tramuntana", "cap", 24), face("cap28", "Tramuntana", "cap", 28),
            face("rosa24", "Tramuntana", "rosa", 24), face("rosa28", "Tramuntana", "rosa", 28))
        // Cap: the large breath has a middle line; the small one only two thick lines either side of it.
        assertTrue("28 dp: three lines", ink(image("cap28"), 49, 74, 51, 86) > 0)
        assertEquals("24 dp: no middle line", 0, ink(image("cap24"), 49, 74, 51, 86))
        assertTrue("24 dp: the side lines", ink(image("cap24"), 40, 74, 47, 86) > 0)
        // Rosa: the east cardinal point (cream) is drawn at 28 dp, not at 24 where only the wind's point stays.
        val big = android.graphics.Color.red(at(image("rosa28"), 62f, 47.5f))
        val small = android.graphics.Color.red(at(image("rosa24"), 62f, 47.5f))
        assertTrue("cardinal point $big over bare dial $small", big > small + 40)
    }

    // Amb el moviment apagat la cara és un fotograma quiet, també mentre treballa (com el web i el mapa).
    @Test fun `motion off keeps both still, idle or working`() {
        // Bottom halves: Cap Tramuntana's breath and Rosa Migjorn's point, away from the mood badge.
        fun bottom(b: Bitmap) = Bitmap.createBitmap(b, 0, b.height / 2, b.width, b.height / 2)
        faces(false, face("cap", "Tramuntana", "cap", 96), face("rosa", "Migjorn", "rosa", 96),
            face("capWork", "Tramuntana", "cap", 96, WindMood.WORKING), face("rosaWork", "Migjorn", "rosa", 96, WindMood.WORKING))
        val before = listOf("cap", "rosa", "capWork", "rosaWork").associateWith { bottom(image(it)) }
        ui.mainClock.advanceTimeBy(300)
        assertTrue("idle Cap is still", before.getValue("cap").sameAs(bottom(image("cap"))))
        assertTrue("idle Rosa is still", before.getValue("rosa").sameAs(bottom(image("rosa"))))
        assertTrue("working Cap is still too", before.getValue("capWork").sameAs(bottom(image("capWork"))))
        assertTrue("working Rosa is still too", before.getValue("rosaWork").sameAs(bottom(image("rosaWork"))))
    }

    // Amb el moviment encès, la feina mou fins i tot les cares petites de les llistes.
    @Test fun `with motion on small working Cap and Rosa move`() {
        faces(true, face("capWork", "Tramuntana", "cap", 40, WindMood.WORKING), face("rosaWork", "Migjorn", "rosa", 40, WindMood.WORKING))
        val cap = image("capWork")
        val rosa = image("rosaWork")
        ui.mainClock.advanceTimeBy(300)
        assertFalse("working Cap blows", cap.sameAs(image("capWork")))
        assertFalse("working Rosa swings", rosa.sameAs(image("rosaWork")))
    }

    @Test fun `with motion on an idle Cap and Rosa move`() {
        fun bottom(b: Bitmap) = Bitmap.createBitmap(b, 0, b.height / 2, b.width, b.height / 2)
        faces(true, face("cap", "Tramuntana", "cap", 96), face("rosa", "Migjorn", "rosa", 96))
        assertEquals("Cap's head, not a Cares ball", 0, ink(image("cap"), 0, 0, 100, 12))
        val cap = bottom(image("cap"))
        val rosa = bottom(image("rosa"))
        ui.mainClock.advanceTimeBy(500)
        assertFalse(cap.sameAs(bottom(image("cap"))))
        assertFalse(rosa.sameAs(bottom(image("rosa"))))
    }

    private fun light(c: Int) = android.graphics.Color.red(c) + android.graphics.Color.green(c) + android.graphics.Color.blue(c)

    @Test fun `Esperit, Cap and Rosa have the volume of Cares, a rim light and a coloured contact shadow`() {
        // Head centre and radius in the 100 box: Esperit Ponent (62, 50) R 28, Cap Migjorn (50, 61) R 25, Rosa (50, 50) R 50.
        val heads = listOf(Triple("esperit", "Ponent", floatArrayOf(62f, 50f, 28f)), Triple("cap", "Migjorn", floatArrayOf(50f, 61f, 25f)),
            Triple("rosa", "Migjorn", floatArrayOf(50f, 50f, 50f)))
        faces(false, *heads.map { (pack, name, _) -> face(pack, name, pack, 96) }.toTypedArray())
        for ((pack, _, h) in heads) {
            val b = image(pack)
            // Down and right, away from the features: the plain gradient only darkens toward the edge, the rim lights it back.
            fun ray(f: Float) = light(at(b, h[0] + .5f * h[2] * f, h[1] + .866f * h[2] * f))
            assertTrue("$pack: the rim lights the lower edge (${ray(.97f)} over ${ray(.5f)})", ray(.97f) > ray(.5f) + 20)
        }
        for ((pack, _, h) in heads.take(2)) {
            // The ground shadow under the head is a dark tone of the wind's colour, not grey.
            val c = at(image(pack), h[0], h[1] + 1.13f * h[2])
            assertNotEquals("$pack: a contact shadow", android.graphics.Color.BLACK, c)
            assertTrue("$pack: tinted like the wind", android.graphics.Color.blue(c) > android.graphics.Color.red(c) + 8)
        }
    }

    @Test fun `the gravity tilt moves the light of every pack with motion on, and nothing without it`() {
        ui.mainClock.autoAdvance = false
        val packs = listOf("esperit", "cap", "rosa")
        ui.setContent {
            CompositionLocalProvider(LocalAvatarTheme provides "free") {
                Column {
                    for (pack in packs) Row {
                        CompositionLocalProvider(LocalWindMotion provides true) {
                            Box(Modifier.testTag(pack).background(Color.Black)) { WindFace(size = 96, accent = accent, style = style("Migjorn", pack)) }
                            Box(Modifier.testTag("$pack-list").background(Color.Black)) { WindFace(size = 40, accent = accent, style = style("Migjorn", pack)) }
                        }
                        CompositionLocalProvider(LocalWindMotion provides false) {
                            Box(Modifier.testTag("$pack-still").background(Color.Black)) { WindFace(size = 96, accent = accent, style = style("Migjorn", pack)) }
                        }
                    }
                }
            }
        }
        ui.mainClock.advanceTimeBy(100)
        val tags = packs.flatMap { listOf(it, "$it-list", "$it-still") }
        val before = tags.associateWith { image(it) }
        try {
            WindLean.value.value = Offset(1f, .6f)
            ui.waitForIdle()
            for (pack in packs) {
                assertFalse("$pack: the light follows the tilt", before.getValue(pack).sameAs(image(pack)))
                assertTrue("$pack: list faces do not tilt", before.getValue("$pack-list").sameAs(image("$pack-list")))
                assertTrue("$pack: without motion nothing moves", before.getValue("$pack-still").sameAs(image("$pack-still")))
            }
        } finally {
            WindLean.value.value = Offset.Zero
        }
    }
}
