package xyz.solanes.gregal

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.time.ZoneOffset

/** B13: conversió de coordenades, esvaïment i textos de la reproducció (funcions pures; Robolectric només per a org.json). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class OrdinadorAccionsTest {
    private val eps = 1e-3f

    @Test fun fitRectLetterboxesAWideImage() {
        // Captura 960x540 en una caixa 360x400: ample ple, franges a dalt i a baix.
        val b = fitRect(360f, 400f, 960f, 540f)
        assertEquals(0f, b.x, eps)
        assertEquals(360f, b.w, eps)
        assertEquals(202.5f, b.h, eps)
        assertEquals((400f - 202.5f) / 2f, b.y, eps)
        // El centre del viewport cau al centre de la caixa.
        val c = b.map(.5f, .5f)
        assertEquals(180f, c.x, eps)
        assertEquals(200f, c.y, eps)
    }

    @Test fun fitRectPillarboxesATallImage() {
        val b = fitRect(800f, 400f, 400f, 800f)
        assertEquals(200f, b.w, eps)
        assertEquals(400f, b.h, eps)
        assertEquals(300f, b.x, eps)
        assertEquals(0f, b.y, eps)
        val corner = b.map(1f, 1f)
        assertEquals(500f, corner.x, eps)
        assertEquals(400f, corner.y, eps)
    }

    @Test fun fitRectWithoutSizesFillsTheBox() {
        val b = fitRect(300f, 200f, 0f, 0f)
        assertEquals(FitBox(0f, 0f, 300f, 200f), b)
    }

    @Test fun toScreenPlacesTheViewportUnderChromesToolbar() {
        // Pantalla 1280x800, viewport 1280x715 a 85 px de dalt (pestanyes i adreça).
        val a = ComputerAction(x = .5f, y = .5f, viewW = 1280f, viewH = 715f, screenW = 1280f, screenH = 800f, top = 85f)
        val p = toScreen(a, a.x, a.y)
        assertEquals(.5f, p.x, eps)
        assertEquals((85f + 357.5f) / 800f, p.y, eps)
        // Sense pantalla coneguda, el viewport és la pantalla.
        val q = toScreen(ComputerAction(viewW = 1000f, viewH = 500f), .25f, .75f)
        assertEquals(.25f, q.x, eps)
        assertEquals(.75f, q.y, eps)
    }

    @Test fun fadeStartsFullAndEndsAtZero() {
        assertEquals(1f, fade(0f, 900f), eps)
        assertEquals(1f, fade(-50f, 900f), eps)
        assertEquals(.75f, fade(450f, 900f), eps)
        assertEquals(0f, fade(900f, 900f), eps)
        assertEquals(0f, fade(5000f, 900f), eps)
        assertEquals(0f, fade(10f, 0f), eps)
        // Sempre decreix.
        var prev = 1f
        for (t in 0..900 step 50) {
            val f = fade(t.toFloat(), 900f)
            assertTrue("fade($t)=$f > $prev", f <= prev)
            prev = f
        }
        assertTrue(actionDurationMs("type") > actionDurationMs("click"))
    }

    @Test fun parsesTheReplayAnswerAndKeepsPasswordsMasked() {
        val o = JSONObject("""{"run":42,"actions":[
            {"id":7,"at":"2026-11-11T16:20:31Z","run":42,"step":"2/4 · Omple les dades","type":"type","owner":"local","x":0.3,"y":0.31,
             "rect":[0.02,0.29,0.47,0.05],"target":"Contrasenya del compte","text":"••••••••","masked":true,
             "view":{"w":1258,"h":702},"shot":"abc","shotUrl":"/api/ordinador/accions/captura?id=abc"},
            {"id":8,"type":"drag","x":0.1,"y":0.2,"toX":0.6,"toY":0.7,"target":"A → B","view":{"w":800,"h":600,"screenW":1280,"screenH":800,"left":10,"top":90}}]}""")
        val list = parseComputerActions(o)
        assertEquals(2, list.size)
        val t = list[0]
        assertTrue(t.masked)
        assertEquals("••••••••", t.text)
        assertEquals(listOf(0.02f, 0.29f, 0.47f, 0.05f), t.rect)
        assertEquals("/api/ordinador/accions/captura?id=abc", t.shotUrl)
        assertNull(t.toX)
        assertEquals("Escriu «••••••••» a «Contrasenya del compte»", describeComputerAction("ca", t))
        assertEquals("Types «••••••••» in «Contrasenya del compte»", describeComputerAction("en", t))
        assertEquals("2 de 5 · 16:20:31 · Pas 2/4 · Omple les dades", replayCaption("ca", t, 1, 5, ZoneOffset.UTC))
        val d = list[1]
        assertEquals(.6f, d.toX!!, eps)
        assertEquals(1280f, d.screenW, eps)
        assertEquals("Arrossega A → B", describeComputerAction("ca", d))
        assertFalse(d.masked)
    }

    @Test fun plainSentences() {
        assertEquals("Clica «Reserva»", describeComputerAction("ca", ComputerAction(type = "click", target = "Reserva")))
        assertEquals("Clicks on the screen", describeComputerAction("en", ComputerAction(type = "click")))
        assertEquals("Desplaça amunt", describeComputerAction("ca", ComputerAction(type = "scroll", dir = "up")))
        assertEquals("Scrolls down", describeComputerAction("en", ComputerAction(type = "scroll")))
        assertEquals("Tria «21:00» a «Hora»", describeComputerAction("ca", ComputerAction(type = "select", target = "Hora", text = "21:00")))
    }
}
