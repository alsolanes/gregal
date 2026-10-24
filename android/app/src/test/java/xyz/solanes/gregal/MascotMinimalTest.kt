package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.unit.dp
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class MascotMinimalTest {
    @get:Rule val ui = createComposeRule()
    private val accent = Color(0xFFFF7900)

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

    @Test fun stackedAvatarCountsOnlyMembersNotAlreadyDrawn() {
        val members = listOf(Wind("a", "Xaloc", "#29BCA3", "música"), Wind("b", "Gregal", "#4F5BE8", "idees"))
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides LightPal) {
                WindHalo(accent, WindStyle(), stacked = members)
            }
        }
        ui.onNodeWithText("+1").assertExists()
        ui.onNodeWithText("+2").assertDoesNotExist()
    }

    @Test fun styleCodesAndAllPersonalizedPartsStillRenderInBothThemes() {
        var palette by androidx.compose.runtime.mutableStateOf(LightPal)
        val styles = WindEyes.entries.flatMap { eyes -> WindMouth.entries.flatMap { mouth ->
            WindShape.entries.map { shape -> WindStyle(eyes, mouth, shape) }
        } }
        assertEquals(48, styles.map { it.encode() }.toSet().size)
        styles.forEach { assertEquals(it, windStyleOf(it.encode())) }
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides palette) {
                MaterialTheme(colorScheme = palette.scheme()) {
                    Column(Modifier.fillMaxWidth().background(palette.deepTop).padding(16.dp).testTag("gallery")) {
                        androidx.compose.material3.Text("Bola · Núvol · Espiral", color = palette.ink)
                        Row {
                            WindShape.entries.forEach { shape -> WindFace(size = 112, accent = accent, style = WindStyle(shape = shape)) }
                        }
                        androidx.compose.material3.Text("Ulls personals", color = palette.ink)
                        Row { WindEyes.entries.forEach { eyes -> WindFace(size = 80, accent = Color(0xFF29BCA3), style = WindStyle(eyes = eyes)) } }
                        androidx.compose.material3.Text("Boques personals", color = palette.ink)
                        Row { WindMouth.entries.forEach { mouth -> WindFace(size = 80, accent = Color(0xFFF5DDA6), style = WindStyle(mouth = mouth)) } }
                        androidx.compose.material3.Text("Petit · perfil · colla", color = palette.ink)
                        Row {
                            WindHalo(accent, WindStyle(kit = "shopping"), size = 36)
                            WindHalo(accent, WindStyle(kit = "shopping"), size = 92)
                            WindHalo(Color(0xFF4F5BE8), WindStyle(shape = WindShape.CLOUD), size = 64,
                                stacked = listOf(Wind("x", "Xaloc", "#29BCA3", "música")))
                        }
                        androidx.compose.material3.Text("48 combinacions preservades", color = palette.ink)
                        styles.chunked(8).forEach { row -> Row {
                            row.forEach { style -> WindFace(size = 40, accent = Color(0xFF4F5BE8), style = style) }
                        } }
                    }
                }
            }
        }
        listOf("light" to LightPal, "dark" to DarkPal).forEach { (name, colors) ->
            ui.runOnIdle { palette = colors }
            val gallery = image("gallery")
            java.io.File("build/mascot-ui/minimal-$name.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
                gallery.compress(Bitmap.CompressFormat.PNG, 100, it)
            }
            assertTrue(gallery.width > 300)
        }
    }

    @Test fun ambientMoodsNeverBounceContinuously() {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides true) {
                Row(Modifier.background(Color.Black)) {
                    listOf(WindMood.IDLE, WindMood.HAPPY, WindMood.TALKING).forEach { mood ->
                        Box(Modifier.testTag(mood.name)) { WindFace(size = 128, accent = accent, mood = mood) }
                    }
                }
            }
        }
        val centers = mutableMapOf<String, MutableList<Float>>()
        repeat(7) {
            ui.mainClock.advanceTimeBy(400)
            listOf("IDLE", "HAPPY", "TALKING").forEach { tag ->
                val frame = image(tag)
                val ys = (0 until frame.height).filter { y -> frame.getPixel(frame.width / 2, y) == 0xFFFF7900.toInt() }
                centers.getOrPut(tag) { mutableListOf() }.add((ys.first() + ys.last()) / 2f)
            }
        }
        centers.forEach { (mood, ys) -> assertTrue("$mood should breathe, not bounce: $ys", ys.max() - ys.min() <= 1f) }
    }

    @Test fun idleSleepyFaceDoesNotEmitFloatingDecoration() {
        ui.mainClock.autoAdvance = false
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides true) {
                Box(Modifier.background(Color.Black).testTag("idle")) {
                    WindFace(size = 128, accent = accent, style = WindStyle(eyes = WindEyes.SLEEPY))
                }
            }
        }
        repeat(4) {
            ui.mainClock.advanceTimeBy(1000)
            val frame = image("idle")
            for (y in 0 until (frame.height * .10f).toInt()) for (x in 0 until frame.width) {
                assertEquals("Rest is quiet: no floating letters or particles", android.graphics.Color.BLACK, frame.getPixel(x, y))
            }
        }
    }

    @Test fun systemAnimatorScaleDisablesAllSharedDecorativeMotion() {
        val app = androidx.test.core.app.ApplicationProvider.getApplicationContext<android.app.Application>()
        val resolver = app.contentResolver
        val key = android.provider.Settings.Global.ANIMATOR_DURATION_SCALE
        val previous = android.provider.Settings.Global.getFloat(resolver, key, 1f)
        android.provider.Settings.Global.putFloat(resolver, key, 0f)
        try {
            ui.mainClock.autoAdvance = false
            ui.setContent {
                CompositionLocalProvider(LocalWindMotion provides true) {
                    Column(Modifier.background(Color.Black).testTag("reduced")) {
                        WindFace(size = 96, accent = accent, mood = WindMood.TALKING)
                        TypingDots(accent)
                    }
                }
            }
            ui.mainClock.advanceTimeBy(100)
            val before = image("reduced")
            ui.mainClock.advanceTimeBy(1100)
            assertTrue("Animator scale 0 must also stop typing transitions", identical(before, image("reduced")))
        } finally {
            android.provider.Settings.Global.putFloat(resolver, key, previous)
        }
    }

    @Test fun profileToolDoesNotChangeTheFaceOrCoverItsEyes() {
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false) {
                Row(Modifier.background(Color.Black)) {
                    Box(Modifier.testTag("plain")) { WindFace(size = 96, accent = accent) }
                    Box(Modifier.testTag("specialist")) { WindFace(size = 96, accent = accent, style = WindStyle(kit = "shopping")) }
                }
            }
        }
        val plain = image("plain")
        val specialist = image("specialist")
        for (y in 0 until (plain.height * .60f).toInt()) for (x in 0 until plain.width) {
            assertEquals("The optional profile tool must leave the identity untouched", plain.getPixel(x, y), specialist.getPixel(x, y))
        }
        assertFalse("The specialist still has a discreet tool on the large profile", identical(plain, specialist))
    }

    @Test fun spiralIsADistinctSilhouetteNotABallWithAnEtchedLine() {
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false) {
                Row(Modifier.background(Color.Black)) {
                    WindShape.entries.forEach { shape ->
                        Box(Modifier.testTag(shape.name)) { WindFace(size = 96, accent = accent, style = WindStyle(shape = shape)) }
                    }
                }
            }
        }
        val ball = image("BALL")
        val spiral = image("SPIRAL")
        var different = 0
        for (y in 0 until ball.height) for (x in 0 until ball.width) {
            if ((ball.getPixel(x, y) != android.graphics.Color.BLACK) !=
                (spiral.getPixel(x, y) != android.graphics.Color.BLACK)) different++
        }
        assertTrue("The outline must identify an espiral without inner decoration", different > ball.width * ball.height * .09f)
    }

    @Test fun idleWrapperDoesNotInventPresenceOrPaintAHalo() {
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides LightPal) {
                Row(Modifier.background(Color.White)) {
                    Box(Modifier.testTag("default")) { WindHalo(accent, WindStyle(), online = true) }
                    Box(Modifier.testTag("offline")) { WindHalo(accent, WindStyle(), online = false) }
                }
            }
        }
        val avatar = image("default")
        assertTrue("online is a compatibility hint, not proof of presence", identical(avatar, image("offline")))
        assertEquals("Empty space around the silhouette stays clean", android.graphics.Color.WHITE,
            avatar.getPixel((avatar.width * .08f).toInt(), avatar.height / 2))
    }

    @Test fun defaultFaceIsMatteWithTwoLargeSimpleEyes() {
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false) {
                Box(Modifier.background(Color.Black).testTag("face")) { WindFace(size = 96, accent = accent) }
            }
        }
        val face = image("face")
        val mattePixel = face.getPixel((face.width * .35f).toInt(), (face.height * .25f).toInt())
        assertEquals("The body is one flat accent, without a glossy highlight", 0xFFFF7900.toInt(), mattePixel)
        var eyePixels = 0
        val expectedInk = if (contrastWith(accent, Color(0xFF202127)) >= contrastWith(accent, Color.White))
            0xFF202127.toInt() else android.graphics.Color.WHITE
        for (y in (face.height * .35f).toInt() until (face.height * .60f).toInt()) {
            for (x in (face.width * .25f).toInt() until (face.width * .75f).toInt()) {
                if (face.getPixel(x, y) == expectedInk) eyePixels++
            }
        }
        assertTrue("Two bold, unadorned eyes must survive at avatar size", eyePixels > face.width * face.height * .018f)
    }

    @Test fun smallSpecialistKeepsItsUnclutteredSilhouette() {
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false) {
                Row(Modifier.background(Color.White)) {
                    Box(Modifier.testTag("plain")) { WindFace(size = 48, accent = accent) }
                    Box(Modifier.testTag("specialist")) {
                        WindFace(size = 48, accent = accent, style = WindStyle(kit = "shopping"))
                    }
                }
            }
        }
        assertTrue("A 48dp avatar must prioritize face and shape, not feet or tools",
            identical(image("plain"), image("specialist")))
    }
}
