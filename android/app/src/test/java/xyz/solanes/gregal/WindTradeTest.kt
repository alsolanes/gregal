package xyz.solanes.gregal

import android.graphics.Bitmap
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.unit.dp
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** L'ofici al perfil d'un vent: lectura de «trade», cos de la rutina i el panell pintat. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindTradeTest {
    @get:Rule val ui = createComposeRule()

    private val profile = """{"wind":{"id":"garbi","name":"Garbí","color":"#29BCA3","role":"Esports"},
        "knowledge":"","personal_memory":"",
        "trade":{"kit":"sports","name_ca":"Esports","name_en":"Sports","sources":["ESPN","Mundo Deportivo",""],"sites":["espn.com"],
          "level":4,"knowledge":2,"lessons":7,
          "watches":[{"label_ca":"Resultats del Barça","label_en":"Barça results","prompt_ca":"Mira els resultats","prompt_en":"Check the results","time":"08:00","weekly":false,"interval_h":0},
                     {"label_ca":"Fitxatges","label_en":"Transfers","prompt_ca":"Mira fitxatges","prompt_en":"Check transfers","time":"","weekly":false,"interval_h":6},
                     {"label_ca":"Calendari","label_en":"Fixtures","prompt_ca":"Mira el calendari","prompt_en":"Check fixtures","time":"09:00","weekly":true,"interval_h":0},
                     {"label_ca":"","label_en":"","prompt_ca":"x","prompt_en":"x"}]}}"""

    @Test fun llegeixLOfici() {
        val t = requireNotNull(WindProfile.parse(profile).trade)
        assertEquals("sports", t.kit); assertEquals("Esports", t.name("ca")); assertEquals("Sports", t.name("en"))
        assertEquals(listOf("ESPN", "Mundo Deportivo"), t.sources)
        assertEquals(4, t.level); assertEquals(2, t.knowledge); assertEquals(7, t.lessons)
        assertEquals(3, t.watches.size)
        assertEquals("cada dia a les 08:00", t.watches[0].schedule("ca"))
        assertEquals("cada 6 h", t.watches[1].schedule("ca"))
        assertEquals("every 6 h", t.watches[1].schedule("en"))
        assertEquals("un cop per setmana", t.watches[2].schedule("ca"))
    }

    @Test fun senseOficiEsNull() {
        val base = """{"wind":{"id":"a","name":"A","color":"#29BCA3","role":"x"},"knowledge":"","personal_memory":""}"""
        assertNull(WindProfile.parse(base).trade)
        assertNull(WindProfile.parse(base.dropLast(1) + ""","trade":null}""").trade)
        assertEquals(10, WindTrade.parse(JSONObject("""{"level":42}"""))!!.level)
        assertEquals("1.5", WindTrade.hours(1.5))
    }

    @Test fun elCosDeLaRutinaComALEscriptori() {
        val t = WindProfile.parse(profile).trade!!
        val daily = t.watches[0].rutina("garbi", "ca")
        assertEquals("daily", daily.getString("kind")); assertEquals("08:00", daily.getString("time"))
        assertEquals("Mira els resultats", daily.getString("prompt")); assertEquals("Resultats del Barça", daily.getString("name"))
        assertFalse(daily.has("interval_h"))
        val every = t.watches[1].rutina("garbi", "en")
        assertEquals("interval", every.getString("kind")); assertEquals(6.0, every.getDouble("interval_h"), 0.0)
        assertEquals("Check transfers", every.getString("prompt")); assertEquals("Transfers", every.getString("name"))
        assertEquals("garbi", every.getString("wind"))
    }

    @Test fun pintaElPanellIActivaUnaVigilancia() {
        val p = WindProfile.parse(profile)
        val activated = mutableListOf<String>()
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    Box(Modifier.width(393.dp).background(DarkPal.surface).padding(16.dp).testTag("shot")) {
                        WindTradePanel(null, "ca", p.wind, p.trade!!,
                            onActivate = { w, done -> activated += w.label("ca"); done(true) }, onExpert = {})
                    }
                }
            }
        }
        ui.onNodeWithText("Nivell 4").assertExists()
        ui.onNodeWithText("2 coneixements estudiats · 7 lliçons apreses de les converses").assertExists()
        ui.onNodeWithText("EL QUE POT VIGILAR PER TU").assertExists()
        ui.onNodeWithText("Fes-lo més expert").assertExists()
        ui.onAllNodesWithText("Activa")[0].performClick()
        ui.waitForIdle()
        assertEquals(listOf("Resultats del Barça"), activated)
        ui.onNodeWithText("Activada").assertExists()
        val bounds = ui.onNodeWithTag("shot").fetchSemanticsNode().boundsInRoot
        ui.runOnIdle {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).single()
            val view = activity.window.decorView
            val root = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
            view.draw(android.graphics.Canvas(root))
            val shot = Bitmap.createBitmap(root, bounds.left.toInt(), bounds.top.toInt(), bounds.width.toInt(), bounds.height.toInt())
            java.io.File("build/trade/trade-dark.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
                shot.compress(Bitmap.CompressFormat.PNG, 100, it)
            }
        }
    }
}
