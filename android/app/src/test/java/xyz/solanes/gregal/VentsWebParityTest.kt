package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
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

/** 152: Android names the computer, greets and titles Desat as the web does (127), and the routine rows draw each wind's own pack (141). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class VentsWebParityTest {
    @get:Rule val ui = createComposeRule()

    private val app get() = ApplicationProvider.getApplicationContext<Application>()

    @Test fun `the computer has one name, as computerName in winds-workspace js`() {
        for ((lang, name) in listOf("ca" to "L’ordinador de Gregal", "en" to "Gregal’s computer")) {
            assertEquals(name, tr(lang, "app.computer.name"))
            // The pane it opens and Gregal's header button say the same.
            assertEquals(name, ordinadorCopy(lang, "local"))
            assertEquals(name, ordinadorCopy(lang, "title"))
            assertEquals(name, threadCopy(lang, "computer"))
        }
    }

    @Test fun `the greeting has the words and hours of salutacio in i18n js`() {
        for ((hour, ca, en) in listOf(Triple(0, "Bona nit", "Good night"), Triple(5, "Bona nit", "Good night"),
            Triple(6, "Bon dia", "Good morning"), Triple(12, "Bon dia", "Good morning"),
            Triple(13, "Bona tarda", "Good afternoon"), Triple(19, "Bona tarda", "Good afternoon"),
            Triple(20, "Bon vespre", "Good evening"), Triple(23, "Bon vespre", "Good evening"))) {
            assertEquals("ca $hour h", "$ca.", askGreeting("ca", hour, ""))
            assertEquals("en $hour h", "$en.", askGreeting("en", hour, ""))
        }
    }

    private fun drawer(lang: String) {
        val vm = GregalViewModel(app)
        vm.attachPrefs(Prefs(app))
        vm.setSimpleVents(true)
        vm.setIdioma(lang)
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    val screen by vm.screen.collectAsState()
                    // 182: Configuració is a page now, and the computer row lives there.
                    if (screen == Screen.Settings) SettingsScreen(vm) {}
                    else GregalDrawerContent(vm, lang, screen, true, "", "", "", nav = { vm.go(it) }, close = {})
                }
            }
        }
    }

    @Test fun `the drawer names the computer and Desat's first tab as the web`() {
        drawer("ca")
        ui.onNodeWithText("Desat").performClick()
        ui.onNodeWithText("Respostes").assertExists()
        ui.onNodeWithText("Configuració").performClick()
        ui.onNodeWithText("L’ordinador de Gregal").assertExists()
        ui.onNodeWithText("Ordinador").assertDoesNotExist()
    }

    @Test fun `in English the first Desat tab is Replies`() {
        drawer("en")
        ui.onNodeWithText("Saved").performClick()
        ui.onNodeWithText("Replies").assertExists()
        ui.onNodeWithText("Settings").performClick()
        ui.onNodeWithText("Gregal’s computer").assertExists()
    }

    private fun image(tag: String): Bitmap {
        val bounds = ui.onNodeWithTag(tag, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
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

    /** The same picture, give or take one or two levels of blending (the row's surface is composited apart). */
    private fun alike(a: Bitmap, b: Bitmap): Boolean {
        if (a.width != b.width || a.height != b.height) return false
        for (y in 0 until a.height) for (x in 0 until a.width) {
            val p = a.getPixel(x, y)
            val q = b.getPixel(x, y)
            if ((0..24 step 8).any { kotlin.math.abs((p shr it and 0xFF) - (q shr it and 0xFF)) > 2 }) return false
        }
        return true
    }

    @Test fun `a routine row draws an Esperit wind in Lliure with its own pack`() {
        val teca = Wind("teca", "Teca", "#29ADFF", "r", avatar = "esperit")
        // GET /api/winds/automations sends no avatar: only the style code.
        val routine = WindAutomation("a1", "routine", "teca", "Teca", "#29ADFF", "pso", "News at 8", "", "daily", "08:00",
            0.0, true, "", "", true)
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false, LocalAvatarTheme provides "free") {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    Column(Modifier.background(Color.Black)) {
                        WindAutomationRow("en", routine, showWind = true, winds = listOf(teca), onAction = { _, _ -> })
                        // On the row's own background, so only the face can differ.
                        Surface(color = Ink.copy(alpha = 0.04f)) {
                            Column {
                                Box(Modifier.testTag("own")) { WindFace(size = 36, accent = Color(teca.argb), style = teca.faceStyle) }
                                Box(Modifier.testTag("cares")) { WindFace(size = 36, accent = Color(teca.argb), style = windStyleOf("pso")) }
                            }
                        }
                    }
                }
            }
        }
        val row = image("automation-face")
        assertFalse("not the Cares face", alike(row, image("cares")))
        assertTrue("the wind's own Esperit, as in the list", alike(row, image("own")))
    }
}
