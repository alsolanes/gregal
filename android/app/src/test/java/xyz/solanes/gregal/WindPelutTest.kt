package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.foundation.layout.Row
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasAnyChild
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isSelectable
import androidx.compose.ui.test.junit4.v2.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** 156: the Pelut pack (155) on Android, a furry clay wind in its colour at every size. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindPelutTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()
    private val orange = Color(0xFFFF9F00)

    private fun show(theme: String = "pelut", dir: String = "characters", content: @Composable () -> Unit) = ui.setContent {
        CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false, LocalWindClay provides dir, LocalAvatarTheme provides theme) {
            MaterialTheme(colorScheme = LightPal.scheme()) { content() }
        }
    }

    private fun clay(sheet: String) = SemanticsMatcher.expectValue(WindClayKey, sheet)
    private fun anyClay() = ui.onAllNodes(SemanticsMatcher.keyIsDefined(WindClayKey), useUnmergedTree = true)
    private fun anyFace() = ui.onAllNodes(SemanticsMatcher.keyIsDefined(WindFaceKey), useUnmergedTree = true)

    private fun pixels(tag: String): Bitmap {
        val r = ui.onNodeWithTag(tag, useUnmergedTree = true).fetchSemanticsNode().boundsInWindow
        val view = ui.activity.window.decorView
        val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
        ui.runOnUiThread { view.draw(android.graphics.Canvas(bmp)) }
        return Bitmap.createBitmap(bmp, r.left.toInt(), r.top.toInt(), r.width.toInt(), r.height.toInt())
    }

    @Test fun `pelut is a pack for the theme and for a wind of its own`() {
        assertTrue("pelut" in windPacks)
        assertEquals("pelut", avatarThemeOf("""{"avatarTheme":"pelut"}"""))
        assertEquals("pelut", windPack("free", Wind("bit", "Bit", "#29ADFF", "r", avatar = "pelut").faceStyle))
        assertEquals("Pelut", tr("ca", avatarLabelKey("pelut")))
        assertEquals("Furry", tr("en", avatarLabelKey("pelut")))
    }

    @Test fun `a pelut is its clay sheet at large and small sizes, without asking for clay`() {
        var mood by mutableStateOf(WindMood.IDLE)
        show {
            Row {
                WindFace(size = 96, mood = mood, accent = orange)
                WindFace(size = 40, mood = mood, accent = orange)
                WindFace(size = 24, mood = mood, accent = orange, style = WindStyle(kit = "frontend"), showKit = true)
            }
        }
        ui.onAllNodes(clay("pelut/idle"), useUnmergedTree = true).assertCountEquals(3)
        mood = WindMood.THINKING
        ui.waitForIdle()
        ui.onAllNodes(clay("pelut/thinking"), useUnmergedTree = true).assertCountEquals(3)
        anyFace().assertCountEquals(3)
    }

    /** Average colour of the opaque pixels of the face tagged [tag]. */
    private fun body(tag: String): FloatArray {
        val bmp = pixels(tag)
        val sum = FloatArray(3)
        var n = 0
        for (y in 0 until bmp.height) for (x in 0 until bmp.width) {
            val p = bmp.getPixel(x, y)
            if (android.graphics.Color.alpha(p) < 250) continue
            sum[0] += android.graphics.Color.red(p).toFloat(); sum[1] += android.graphics.Color.green(p).toFloat(); sum[2] += android.graphics.Color.blue(p).toFloat()
            n++
        }
        assertTrue("the face has a body", n > 50)
        return FloatArray(3) { sum[it] / n }
    }

    @Test fun `a red pelut is red and a blue one is blue, large and small`() {
        val sizes = listOf(96, 40)
        show {
            Row {
                for (size in sizes) {
                    WindFace(size = size, accent = Color(0xFFE0453A), modifier = Modifier.testTag("red$size"))
                    WindFace(size = size, accent = Color(0xFF3A6FE0), modifier = Modifier.testTag("blue$size"))
                }
            }
        }
        for (size in sizes) {
            val red = body("red$size")
            assertTrue("a red pelut at $size is red: ${red.toList()}", red[0] > red[2] + 40f)
            val blue = body("blue$size")
            assertTrue("a blue pelut at $size is blue: ${blue.toList()}", blue[2] > blue[0] + 40f)
        }
    }

    @Test fun `without its sheets a pelut keeps a Canvas face`() {
        show(dir = "no-such-folder") {
            Row {
                WindFace(size = 96, accent = orange, clay = true)
                WindFace(size = 40, accent = orange)
            }
        }
        anyFace().assertCountEquals(2)
        anyClay().assertCountEquals(0)
    }

    @Test fun `Preferences lists Pelut and shows the clay packs as their 3D figure`() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        vm.setConn(Conn("127.0.0.1", "9", "t"))
        vm.openPreferences(PrefsPage.Appearance)
        show(theme = "cares") { PrefsScreen(vm) {} }
        ui.onNodeWithText(tr(vm.idioma.value, "winds.avatarPelut")).performScrollTo()
        for ((pack, clayFamily) in listOf("cares" to true, "minairo" to true, "pelut" to true, "esperit" to false)) {
            // Free's sample draws an Esperit too: the first one is the Esperit choice's own.
            val sample = ui.onAllNodesWithTag("avatarSample-$pack", useUnmergedTree = true)[0].fetchSemanticsNode()
            assertEquals("the $pack sample is clay", clayFamily, WindClayKey in sample.config)
        }
        assertEquals("pelut/idle", ui.onNodeWithTag("avatarSample-pelut", useUnmergedTree = true).fetchSemanticsNode().config[WindClayKey])
        assertEquals("minairo/idle", ui.onNodeWithTag("avatarSample-minairo", useUnmergedTree = true).fetchSemanticsNode().config[WindClayKey])
    }

    @Test fun `the editor in Free mode offers Pelut as a clay figure and saves it`() {
        var saved: WindDraft? = null
        show(theme = "free") { WindEditor("ca", Wind("bit", "Bit", "#29ADFF", "lectura"), {}, { _, _, _, _ -> }, { draft, _ -> saved = draft }) }
        val label = tr("ca", "winds.avatarPelut")
        ui.onNode(hasText(label) and isSelectable()).performScrollTo()
        ui.onNode(clay("pelut/idle") and hasAnyAncestor(isSelectable() and hasAnyChild(hasText(label))), useUnmergedTree = true).assertExists()
        val cares = tr("ca", "winds.avatarCares")
        ui.onNode(SemanticsMatcher.keyIsDefined(WindClayKey) and hasAnyAncestor(isSelectable() and hasAnyChild(hasText(cares))), useUnmergedTree = true)
            .assertExists("Cares is its clay figure too")
        ui.onNode(hasText(label) and isSelectable()).performClick()
        ui.onNodeWithText(tr("ca", "app.vents.saveChanges")).performClick()
        ui.runOnIdle { assertEquals("pelut", saved?.avatar) }
    }
}
