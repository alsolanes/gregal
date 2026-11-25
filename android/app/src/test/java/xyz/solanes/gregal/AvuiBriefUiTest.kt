package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** «Avui» comença amb el resum del dia: una targeta per vent que ha fet o proposa alguna cosa. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h1500dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class AvuiBriefUiTest {
    @get:Rule val ui = createComposeRule()

    private val winds = listOf(
        Wind("garbi", "Garbí", "#29BCA3", "casa"),
        Wind("xaloc", "Xaloc", "#EFA68F", "futbol"),
        Wind("oratge", "Oratge", "#4F5BE8", "temps", kit = "weather"),
        Wind("mestral", "Mestral", "#B07CE8", "lectura"),
    )

    private fun sample() = AvuiView(
        date = "2026-11-10",
        now = listOf(
            AvuiCard("c1", "garbi", "Garbí", "feina:r1", "now", "La comparativa de neveres és a punt",
                actions = listOf(AvuiAction("Explica'm el resultat", "Explica'm"))),
            AvuiCard("c2", "garbi", "Garbí", "brief", "now", "Compra la nevera abans de divendres",
                actions = listOf(AvuiAction("Més detalls", "Més sobre la nevera"))),
        ),
        fyi = listOf(
            AvuiCard("c3", "oratge", "Oratge", "heart", "fyi", "Plourà a partir de les 17 h",
                actions = listOf(AvuiAction("Més detalls", "Més sobre la pluja"))),
            // Ja descartada: el Mestral no té res avui.
            AvuiCard("c5", "mestral", "Mestral", "heart", "fyi", "Un llibre nou", state = "dismissed"),
        ),
        ahead = emptyList(),
        done = listOf(AvuiCard("c4", "xaloc", "Xaloc", "feina:r2", "now", "La crònica del partit, feta", state = "done")),
        held = 0,
        settings = AvuiSettings(follows = listOf(AvuiFollow("barca", "team", "Barça", wind = "xaloc"))),
        glance = AvuiGlance(weather = AvuiWeather("Girona", 14.0, 9.0, 17.0, 3.0, listOf("17", "18"), 61, "07:40", "17:30")),
    )

    @Test fun elResumTriaPerVentISaltaElsQuiets() {
        val b = AvuiBriefLogic.briefs(sample(), winds)
        assertEquals(listOf("garbi", "oratge", "xaloc"), b.map { it.wind.id })
        assertEquals("c1", b[0].did?.id)
        assertEquals("c2", b[0].proposes?.id)
        assertNull(b[1].did); assertEquals("c3", b[1].proposes?.id)
        assertEquals("c4", b[2].did?.id); assertNull(b[2].proposes)
        assertTrue(AvuiBriefLogic.briefs(sample().copy(now = emptyList(), fyi = emptyList(), done = emptyList()), winds).isEmpty())
    }

    @Test fun avuiComencaAmbUnaTargetaPerVentQueEsLlisca() {
        val runs = mutableListOf<String>()
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                AvuiTray("ca", sample(), winds, false, { _, _ -> }, { c, a -> runs += c.id + ":" + a.prompt }, { _, _ -> }, {})
            }
        }
        ui.onNodeWithTag("avui-brief").assertExists()
        assertEquals(3, ui.onAllNodesWithTag("avui-brief-dot").fetchSemanticsNodes().size)
        ui.onNodeWithTag("avui-brief-garbi").assertExists()
        ui.onNode(hasText("La comparativa de neveres és a punt") and hasAnyAncestor(hasTestTag("avui-brief-garbi")), useUnmergedTree = true).assertExists()
        ui.onNodeWithTag("avui-brief-act", useUnmergedTree = true).performClick()
        assertEquals(listOf("c2:Més sobre la nevera"), runs)
        // El resum va primer, abans de la ullada.
        val brief = ui.onNodeWithTag("avui-brief").fetchSemanticsNode().boundsInRoot
        val weather = ui.onNodeWithTag("glance-weather").fetchSemanticsNode().boundsInRoot
        assertTrue(brief.bottom <= weather.top)

        ui.onNodeWithTag("avui-brief").performTouchInput { swipeLeft() }
        ui.waitForIdle()
        ui.onNodeWithTag("avui-brief-oratge").assertExists()
        ui.onNodeWithTag("brief-art-weather", useUnmergedTree = true).assertExists()
        ui.onNodeWithTag("avui-brief").performTouchInput { swipeLeft() }
        ui.waitForIdle()
        ui.onNodeWithTag("avui-brief-xaloc").assertExists()
        ui.onNodeWithTag("brief-art-match", useUnmergedTree = true).assertExists()
        ui.onNodeWithTag("avui-brief-mestral").assertDoesNotExist()
    }

    @Test fun senseResDeCapVentNoHiHaResum() {
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                AvuiTray("ca", sample().copy(now = emptyList(), fyi = emptyList(), done = emptyList()), winds, false,
                    { _, _ -> }, { _, _ -> }, { _, _ -> }, {})
            }
        }
        ui.onNodeWithTag("glance-weather").assertExists()
        ui.onNodeWithTag("avui-brief").assertDoesNotExist()
    }

    /** Fotogrames del resum (temps i partit) i de la targeta del partit, en fosc i en clar, a output/ui-review/135/. */
    @Test fun pintaElResumEnFoscIEnClar() {
        var palette by mutableStateOf(DarkPal)
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides palette) {
                MaterialTheme(colorScheme = palette.scheme()) {
                    Box(Modifier.fillMaxSize().background(palette.surface).testTag("shot")) {
                        AvuiTray("ca", sample(), winds, false, { _, _ -> }, { _, _ -> }, { _, _ -> }, {})
                    }
                }
            }
        }
        listOf("dark" to DarkPal, "light" to LightPal).forEach { (name, colors) ->
            ui.runOnIdle { palette = colors }
            listOf("garbi", "oratge", "xaloc").forEachIndexed { i, wind ->
                if (i > 0) { ui.onNodeWithTag("avui-brief").performTouchInput { swipeLeft() }; ui.waitForIdle() }
                ui.onNodeWithTag("avui-brief-$wind").assertExists()
                save(capture("shot"), "avui-$wind-$name")
            }
            repeat(2) { ui.onNodeWithTag("avui-brief").performTouchInput { swipeRight() }; ui.waitForIdle() }
        }
    }

    private fun save(shot: Bitmap, name: String) {
        assertTrue(shot.width > 300)
        java.io.File("../../output/ui-review/135/$name.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
            shot.compress(Bitmap.CompressFormat.PNG, 100, it)
        }
    }

    private fun capture(tag: String): Bitmap {
        val bounds = ui.onNodeWithTag(tag).fetchSemanticsNode().boundsInRoot
        lateinit var bitmap: Bitmap
        ui.runOnIdle {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).single()
            fun composeView(view: android.view.View): android.view.View? {
                if (view.javaClass.simpleName == "AndroidComposeView") return view
                if (view is android.view.ViewGroup) for (i in 0 until view.childCount) composeView(view.getChildAt(i))?.let { return it }
                return null
            }
            val view = requireNotNull(composeView(activity.window.decorView))
            val root = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
            view.draw(android.graphics.Canvas(root))
            bitmap = Bitmap.createBitmap(root, bounds.left.toInt(), bounds.top.toInt(), bounds.width.toInt(), bounds.height.toInt())
        }
        return bitmap
    }
}
