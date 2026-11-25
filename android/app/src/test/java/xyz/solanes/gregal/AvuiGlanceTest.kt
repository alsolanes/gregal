package xyz.solanes.gregal

import android.app.Application
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import java.time.ZoneId
import java.time.ZonedDateTime
import java.util.Collections
import kotlin.concurrent.thread
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.LooperMode

/** La capçalera d'«Avui» (el temps, el calendari i el que segueixes): lectura, etiquetes i ajustos. */
@RunWith(RobolectricTestRunner::class)
@LooperMode(LooperMode.Mode.PAUSED)
class AvuiGlanceTest {
    private val raw = """{"date":"2026-11-24","now":[],"fyi":[],"ahead":[],"done":[],"held":0,
        "settings":{"quiet_from":"22:00","quiet_to":"08:00","level":"normal","place":"Girona",
            "follows":[{"id":"f1","kind":"team","name":"Barça"},{"id":"f2","kind":"other","name":"Fórmula 1","wind":"xaloc"},
                {"id":"f3","kind":"team","name":"Girona FC"}]},
        "glance":{"weather":{"place":"Girona","now_c":13.6,"min_c":8.2,"max_c":17,"rain_mm":3.4,"rain_hours":["16","17"],
                "code":61,"sunrise":"07:52","sunset":"17:21"},
            "events":[{"start":"2026-11-24T18:00:00+01:00","end":"2026-11-24T19:00:00+01:00","title":"Dentista","calendar":"Personal"},
                {"start":"2026-11-25T00:00:00+01:00","title":"Aniversari de la Marta","calendar":"Família"}],
            "follows":[{"id":"f1","kind":"team","name":"Barça","title":"Barça – Madrid","when":"2026-11-28T21:00:00+01:00",
                "detail":"Lliga, jornada 14","url":"https://example.com/partit","updated":1764000000},
                {"id":"f2","kind":"other","name":"Fórmula 1","wind":"xaloc","title":"GP d'Abu Dhabi","detail":"L'última del curs"}]}}"""

    @Test fun llegeixLaUllada() {
        val v = AvuiJson.view(raw)
        val w = v.glance.weather!!
        assertEquals("Girona", w.place)
        assertEquals(13.6, w.nowC, 0.001); assertEquals(17.0, w.maxC, 0.001)
        assertEquals(listOf("16", "17"), w.rainHours)
        assertEquals(61, w.code)
        assertEquals(2, v.glance.events.size)
        assertEquals("", v.glance.events[1].end)
        assertEquals("Barça – Madrid", v.glance.follows[0].title)
        assertEquals("2026-11-28T21:00:00+01:00", v.glance.follows[0].at)
        assertEquals("xaloc", v.glance.follows[1].wind)
        assertEquals("Girona", v.settings.place)
        assertEquals(listOf("f1", "f2", "f3"), v.settings.follows.map { it.id })
    }

    @Test fun llegeixPerQueNoHiHaRes() {
        val v = AvuiJson.view("""{"settings":{"place":"Xyz","follows":[{"id":"f1","kind":"team","name":"Barça"},{"id":"f2","kind":"team","name":"Girona FC"},{"id":"f3","kind":"team","name":"Nou"}]},
            "glance":{"weather_error":"no he trobat el temps de «Xyz»","follows":[{"id":"f1","kind":"team","name":"Barça","none":true},
            {"id":"f2","kind":"team","name":"Girona FC","error":"sense xarxa"}]}}""")
        assertEquals("no he trobat el temps de «Xyz»", v.glance.weatherError)
        assertTrue(v.glance.follows[0].none)
        assertEquals("sense xarxa", v.glance.follows[1].error)
        // Encara es busca el «Nou»; el temps ja ha dit que no.
        assertTrue(GlanceLogic.pending(v))
        assertFalse(GlanceLogic.pending(v.copy(settings = v.settings.copy(follows = v.settings.follows.take(2)))))
        assertTrue(GlanceLogic.pending(AvuiJson.view("""{"settings":{"place":"Girona"},"glance":{}}""")))
    }

    @Test fun senseUlladaTotBuit() {
        val v = AvuiJson.view("{}")
        assertNull(v.glance.weather)
        assertTrue(v.glance.events.isEmpty() && v.glance.follows.isEmpty())
        assertEquals("", v.settings.place)
    }

    @Test fun elsQueSegueixesEnOrdreIElsNousEsperen() {
        val v = AvuiJson.view(raw)
        val f = GlanceLogic.follows(v)
        assertEquals(listOf("f1", "f2", "f3"), f.map { it.id })
        assertEquals("Barça – Madrid", f[0].title)
        // El Girona FC encara no té resposta: surt amb el nom i res més.
        assertEquals("Girona FC", f[2].name); assertEquals("", f[2].title)
    }

    private val zone = ZoneId.of("Europe/Madrid")
    private val now = ZonedDateTime.of(2026, 11, 24, 19, 0, 0, 0, zone) // dimarts

    @Test fun quanEnParaulesDeLaPersona() {
        assertEquals("avui a les 21:00", GlanceLogic.whenLabel("ca", "2026-11-24T21:00:00+01:00", now))
        assertEquals("demà a les 21:00", GlanceLogic.whenLabel("ca", "2026-11-25T20:00:00Z", now))
        assertEquals("dissabte a les 21:00", GlanceLogic.whenLabel("ca", "2026-11-28T21:00:00+01:00", now))
        assertEquals("dissabte 5 de desembre a la 01:00", GlanceLogic.whenLabel("ca", "2026-12-05T01:00:00+01:00", now))
        assertEquals("Saturday at 21:00", GlanceLogic.whenLabel("en", "2026-11-28T21:00:00+01:00", now))
        assertEquals("tomorrow at 21:00", GlanceLogic.whenLabel("en", "2026-11-25T21:00:00+01:00", now))
        assertEquals("", GlanceLogic.whenLabel("ca", "", now))
        assertEquals("", GlanceLogic.whenLabel("ca", "aviat", now))
    }

    @Test fun compteEnrere() {
        assertEquals("d’aquí a 2 h", GlanceLogic.countdown("ca", "2026-11-24T21:00:30+01:00", now))
        assertEquals("d’aquí a 1 h 5 min", GlanceLogic.countdown("ca", "2026-11-24T20:05:00+01:00", now))
        assertEquals("in 12 min", GlanceLogic.countdown("en", "2026-11-24T19:12:00+01:00", now))
        assertEquals("d’aquí a 1 min", GlanceLogic.countdown("ca", "2026-11-24T19:00:20+01:00", now))
        assertEquals("d’aquí a 4 dies", GlanceLogic.countdown("ca", "2026-11-28T21:00:00+01:00", now))
        assertEquals("in 1 day", GlanceLogic.countdown("en", "2026-11-25T21:00:00+01:00", now))
        // Ha començat fa poc: és ara; fa estona: ja no en diu res.
        assertEquals("Ara mateix", GlanceLogic.countdown("ca", "2026-11-24T18:00:00+01:00", now))
        assertEquals("", GlanceLogic.countdown("ca", "2026-11-24T15:00:00+01:00", now))
        assertEquals("", GlanceLogic.countdown("ca", "", now))
    }

    @Test fun horesDelCalendariIDelSol() {
        val e = AvuiEvent("2026-11-24T18:00:00+01:00", "2026-11-24T19:00:00+01:00", "Dentista", "Personal")
        assertEquals("18:00", GlanceLogic.eventTime("ca", e, zone))
        // Les de tot el dia van de mitjanit a mitjanit (contracte de l'API).
        val allDay = AvuiEvent("2026-11-25T00:00:00+01:00", "2026-11-26T00:00:00+01:00", "Aniversari", "")
        assertEquals("Tot el dia", GlanceLogic.eventTime("ca", allDay, zone))
        assertEquals("00:00", GlanceLogic.eventTime("ca", AvuiEvent("2026-11-25T00:00:00+01:00", "", "Cap d'any", ""), zone))
        assertEquals(0, GlanceLogic.eventDay(e, now))
        assertEquals(1, GlanceLogic.eventDay(allDay, now))
        assertFalse(GlanceLogic.eventPast(AvuiEvent("2026-11-24T00:00:00+01:00", "2026-11-25T00:00:00+01:00", "Vaga", ""), now))
        assertTrue(GlanceLogic.eventPast(AvuiEvent("2026-11-24T17:00:00+01:00", "2026-11-24T18:30:00+01:00", "Reunió", ""), now))
        assertFalse(GlanceLogic.eventPast(e, now))
        // Passada la posta, el cel serè és de nit.
        val w = AvuiJson.view(raw).glance.weather!!
        assertTrue(GlanceLogic.night(w, now.toLocalTime()))
        assertFalse(GlanceLogic.night(w, java.time.LocalTime.of(12, 0)))
    }

    /** Servidor de proves: apunta cada petició («POST /api/… » + cos) i respon el que digui `reply`. */
    private class FakeServer(val status: (String, String) -> Int = { _, _ -> 200 }, val reply: (String, String) -> String) : java.io.Closeable {
        val requests: MutableList<Pair<String, String>> = Collections.synchronizedList(mutableListOf())
        val socket = ServerSocket(0)
        val worker = thread(isDaemon = true) {
            try {
                while (!socket.isClosed) socket.accept().use { s ->
                    val input = s.getInputStream().buffered()
                    fun line(): String {
                        val b = java.io.ByteArrayOutputStream()
                        while (true) { val c = input.read(); if (c < 0 || c == 10) break; if (c != 13) b.write(c) }
                        return b.toString("UTF-8")
                    }
                    val request = line()
                    var length = 0
                    while (true) {
                        val h = line()
                        if (h.isEmpty()) break
                        if (h.startsWith("Content-Length:", true)) length = h.substringAfter(':').trim().toInt()
                    }
                    val body = String(input.readNBytes(length), Charsets.UTF_8)
                    requests += request to body
                    val code = status(request, body)
                    val bytes = reply(request, body).toByteArray(Charsets.UTF_8)
                    s.getOutputStream().apply {
                        write("HTTP/1.1 $code X\r\nContent-Type: application/json\r\nContent-Length: ${bytes.size}\r\nConnection: close\r\n\r\n".toByteArray())
                        write(bytes); flush()
                    }
                }
            } catch (_: java.net.SocketException) { }
        }
        fun conn() = Conn("127.0.0.1", socket.localPort.toString(), "fixture", "avui-glance-test")
        override fun close() { socket.close(); worker.join(1000) }
    }

    private fun waitUntil(check: () -> Boolean) {
        repeat(150) {
            shadowOf(Looper.getMainLooper()).idle()
            if (check()) return
            Thread.sleep(20)
        }
        assertTrue("asynchronous operation completed", check())
    }

    /** Un servidor que guarda el lloc i el que segueixes com ho faria el de veritat. */
    private fun avuiServer(): FakeServer {
        var place = "Girona"
        val follows = mutableListOf(JSONObject().put("id", "f1").put("kind", "team").put("name", "Barça"))
        fun settings() = JSONObject().put("quiet_from", "22:00").put("quiet_to", "08:00").put("level", "normal")
            .put("place", place).put("follows", org.json.JSONArray(follows.map { it }))
        return FakeServer { request, body ->
            if (request.startsWith("POST /api/winds/avui/settings")) {
                val b = JSONObject(body)
                if (b.has("place")) place = b.getString("place")
                b.optJSONObject("follow")?.let { f -> follows += JSONObject().put("id", "f${follows.size + 1}").put("kind", f.getString("kind")).put("name", f.getString("name")) }
                if (b.has("unfollow")) follows.removeAll { it.getString("id") == b.getString("unfollow") }
                settings().toString()
            } else JSONObject().put("date", "2026-11-24").put("settings", settings())
                .put("glance", JSONObject().put("follows", org.json.JSONArray())).toString()
        }
    }

    private fun loaded(server: FakeServer): GregalViewModel {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        vm.setConn(server.conn())
        vm.loadAvui()
        waitUntil { vm.avui.value != null }
        server.requests.clear()
        return vm
    }

    @Test fun segueixUnEquipIElDesa() {
        avuiServer().use { server ->
            val vm = loaded(server)
            vm.avuiFollow("team", "Girona FC")
            waitUntil { vm.avui.value?.settings?.follows?.size == 2 && server.requests.size >= 2 }
            val (request, body) = server.requests.first()
            assertTrue(request, request.startsWith("POST /api/winds/avui/settings "))
            val follow = JSONObject(body).getJSONObject("follow")
            assertEquals("team", follow.getString("kind")); assertEquals("Girona FC", follow.getString("name"))
            // Després torna a llegir la safata, perquè la ullada porti el nou.
            assertTrue(server.requests[1].first, server.requests[1].first.startsWith("GET /api/winds/avui "))
            assertEquals(listOf("Barça", "Girona FC"), vm.avui.value!!.settings.follows.map { it.name })
        }
    }

    @Test fun deixarDeSeguirElTreuAlMoment() {
        avuiServer().use { server ->
            val vm = loaded(server)
            vm.avuiUnfollow("f1")
            assertTrue(vm.avui.value!!.settings.follows.isEmpty())
            waitUntil { server.requests.isNotEmpty() }
            assertEquals("f1", JSONObject(server.requests.first().second).getString("unfollow"))
        }
    }

    @Test fun siElServidorDiuQueNoElMissatgeSurt() {
        FakeServer({ request, _ -> if (request.startsWith("POST")) 400 else 200 }) { request, _ ->
            if (request.startsWith("POST")) "digues què vols seguir\n"
            else JSONObject().put("date", "2026-11-24").put("settings", JSONObject().put("follows", org.json.JSONArray())).toString()
        }.use { server ->
            val vm = loaded(server)
            vm.avuiFollow("team", "?!")
            waitUntil { vm.avuiGlanceMsg.value != null }
            assertEquals("digues què vols seguir", vm.avuiGlanceMsg.value)
            assertTrue(vm.avui.value!!.settings.follows.isEmpty())
        }
    }

    @Test fun mentreEsBuscaTornaAMirarSol() {
        var answered = false
        FakeServer { _, _ ->
            val f = JSONObject().put("id", "barca").put("kind", "team").put("name", "Barça")
            JSONObject().put("date", "2026-11-24")
                .put("settings", JSONObject().put("follows", org.json.JSONArray().put(f)))
                .put("glance", JSONObject().put("follows", org.json.JSONArray().put(
                    if (answered) JSONObject(f.toString()).put("title", "Sabah – Barça") else f))).toString()
        }.use { server ->
            val vm = loaded(server)
            vm.loadAvui()
            waitUntil { server.requests.size == 1 && vm.avui.value != null }
            answered = true
            shadowOf(Looper.getMainLooper()).idleFor(java.time.Duration.ofSeconds(16))
            waitUntil { vm.avui.value?.glance?.follows?.firstOrNull()?.title == "Sabah – Barça" }
            // Amb la resposta ja no cal tornar-hi.
            val asked = server.requests.size
            shadowOf(Looper.getMainLooper()).idleFor(java.time.Duration.ofSeconds(40))
            assertEquals(asked, server.requests.size)
        }
    }

    /** Una safata amb la targeta d'una vigilància pausada (task 146); [fails] fa que el servidor no accepti el resume. */
    private fun pausaServer(fails: Boolean) = FakeServer({ request, _ -> if (fails && request.startsWith("POST")) 400 else 200 }) { request, _ ->
        if (request.startsWith("POST")) (if (fails) "resume only applies to a pausa card\n" else """{"ok":true}""")
        else JSONObject().put("date", "2026-11-24").put("now", org.json.JSONArray().put(JSONObject()
            .put("id", "p1").put("wind", "xaloc").put("source", "pausa").put("section", "now").put("state", "open")
            .put("title", "He deixat de mirar: preus del lloguer"))).toString()
    }

    @Test fun tornaHiReprenLaVigilanciaIHoDiu() {
        pausaServer(fails = false).use { server ->
            val vm = loaded(server)
            vm.avuiAct("p1", "resume")
            assertEquals("done", vm.avui.value!!.now.single().state)
            waitUntil { vm.notice.value.isNotEmpty() }
            val (request, body) = server.requests.single()
            assertTrue(request, request.startsWith("POST /api/winds/avui "))
            assertEquals("resume", JSONObject(body).getString("action"))
            assertEquals("p1", JSONObject(body).getString("id"))
            assertEquals("Watching it again.", vm.notice.value)
            assertEquals("done", vm.avui.value!!.now.single().state)
        }
    }

    @Test fun siNoEsPotRependreLaTargetaTornaIHoDiu() {
        pausaServer(fails = true).use { server ->
            val vm = loaded(server)
            vm.avuiAct("p1", "resume")
            waitUntil { vm.notice.value.isNotEmpty() }
            assertEquals("Could not resume the watch.", vm.notice.value)
            assertEquals("open", vm.avui.value!!.now.single().state)
        }
    }

    @Test fun posaElLloc() {
        avuiServer().use { server ->
            val vm = loaded(server)
            vm.setAvuiPlace("  Olot ")
            waitUntil { vm.avui.value?.settings?.place == "Olot" }
            assertEquals("Olot", JSONObject(server.requests.first().second).getString("place"))
        }
    }
}
