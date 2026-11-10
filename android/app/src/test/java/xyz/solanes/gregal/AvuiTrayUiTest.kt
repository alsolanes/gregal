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

/** La safata «Avui» pintada amb dades d'exemple, en fosc i en clar (build/avui/avui-*.png). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h1500dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class AvuiTrayUiTest {
    @get:Rule val ui = createComposeRule()

    private val winds = listOf(
        Wind("xaloc", "Xaloc", "#EFA68F", "agenda"),
        Wind("gregal", "Gregal", "#4F5BE8", "notícies"),
        Wind("garbi", "Garbí", "#29BCA3", "casa"),
    )

    private fun sample() = AvuiView(
        date = "2026-11-10",
        now = listOf(
            AvuiCard("c1", "xaloc", "Xaloc", "brief", "now", "Truca al dentista abans de les 12",
                why = "Ho vas apuntar dilluns i encara és obert", days = 2,
                actions = listOf(AvuiAction("Més detalls", "Explica'm més"), AvuiAction("Recorda-m'ho demà", "Recorda-m'ho demà"))),
            AvuiCard("c2", "garbi", "Garbí", "feina:r1", "now", "La comparativa de neveres és a punt",
                why = "Tres models per sota de 900 €", actions = listOf(AvuiAction("Explica'm el resultat", "Explica'm"))),
        ),
        fyi = listOf(
            AvuiCard("c3", "gregal", "Gregal", "heart", "fyi", "Plourà a partir de les 17 h",
                why = "Agafa paraigua si surts tard", held = true, feedback = "useful"),
        ),
        ahead = listOf(
            AvuiCard("c4", "xaloc", "Xaloc", "brief", "ahead", "Divendres sopar a casa de la Marta",
                why = "Compra el vi dijous"),
        ),
        done = listOf(AvuiCard("c0", "xaloc", "Xaloc", "remind", "now", "Paga la quota", state = "done")),
        held = 2,
        settings = AvuiSettings("22:00", "08:00", "calm", ""),
    )

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

    @Test fun pintaLaSafataEnFoscIEnClar() {
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
        ui.onNodeWithText("Dimarts 10 de novembre").assertExists()
        ui.onNodeWithText("ARA TOCA").assertExists()
        ui.onNodeWithText("PER SI DE CAS").assertExists()
        ui.onNodeWithText("MÉS ENDAVANT").assertExists()
        ui.onNodeWithText("des de fa 2 dies").assertExists()
        ui.onNodeWithText("Això és tot per avui").assertExists()
        ui.onNodeWithText("1 feta avui").assertExists()
        ui.onNodeWithText("Ajusta el de demà").assertExists()
        listOf("dark" to DarkPal, "light" to LightPal).forEach { (name, colors) ->
            ui.runOnIdle { palette = colors }
            val shot = capture("shot")
            java.io.File("build/avui/avui-$name.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
                shot.compress(Bitmap.CompressFormat.PNG, 100, it)
            }
            assertTrue(shot.width > 300)
        }
    }

    @Test fun fetTreuLaTargetaIElsBotonsDecideixen() {
        var view by mutableStateOf(sample())
        val acts = mutableListOf<Pair<String, String>>()
        val runs = mutableListOf<String>()
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                AvuiTray("ca", view, winds, false,
                    onAct = { id, a -> acts += id to a; view = AvuiLogic.apply(view, id, a) },
                    onRun = { c, a -> runs += c.wind + ":" + a.prompt }, onSettings = { _, _ -> }, onRetry = {})
            }
        }
        ui.onAllNodesWithTag("avui-done", useUnmergedTree = true)[0].performClick()
        ui.waitForIdle()
        assertEquals("c1" to "done", acts.single())
        ui.onNodeWithText("Truca al dentista abans de les 12").assertDoesNotExist()
        ui.onNodeWithText("2 fetes avui").assertExists()
        ui.onNodeWithText("Explica'm el resultat").performClick()
        assertEquals(listOf("garbi:Explica'm"), runs)
    }

    @Test fun safataBuidaDiuQueTotEsEnCalma() {
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides LightPal) {
                AvuiTray("ca", AvuiView("2026-11-10", emptyList(), emptyList(), emptyList(), emptyList(), 0, AvuiSettings()),
                    winds, false, { _, _ -> }, { _, _ -> }, { _, _ -> }, {})
            }
        }
        ui.onNodeWithTag("avui-empty").assertExists()
        ui.onNodeWithText("ARA TOCA").assertDoesNotExist()
        ui.onNodeWithText("Això és tot per avui").assertDoesNotExist()
    }
}
