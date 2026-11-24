package xyz.solanes.gregal

import android.app.Application
import androidx.compose.foundation.layout.Box
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import java.util.Collections
import kotlin.concurrent.thread
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import org.robolectric.annotation.LooperMode

/**
 * Una novetat d'un vent es nota al moment (tasca 098): amb l'app oberta, la
 * cara del vent duu insígnia i salta, baixa un bàner «Ponent t'ha escrit: …»
 * i les converses recents es tornen a llegir (també en acabar un torn).
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w360dp-h780dp-xhdpi")
@LooperMode(LooperMode.Mode.PAUSED)
class VentsArrivalTest {
    @get:Rule val ui = createAndroidComposeRule<androidx.activity.ComponentActivity>()

    private val gregal = Wind("gregal", "Gregal", "#8193FF", "Assistent")
    private val ponent = Wind("ponent", "Ponent", "#7FB3D5", "El temps")
    private val nowS get() = System.currentTimeMillis() / 1000

    private val old = """{"ts":${System.currentTimeMillis() / 1000 - 60},"wind":"gregal","name":"Gregal","source":"heart","text":"Fa una estona","id":"n-old"}"""

    private fun novetat(id: String, text: String) =
        """{"ts":$nowS,"wind":"ponent","name":"Ponent","source":"routine","text":"$text","message_id":"m-$id","id":"$id"}"""

    private fun vm(server: Server) = GregalViewModel(ApplicationProvider.getApplicationContext<Application>()).also {
        it.setConn(Conn("127.0.0.1", server.port.toString(), "token", "alice"))
    }

    /** Espera fent córrer el fil principal: els ViewModel hi llancen la feina. */
    private fun waitFor(what: String, cond: () -> Boolean) {
        val deadline = System.nanoTime() + 10_000_000_000L
        while (!cond()) {
            assertTrue("no ha passat: $what", System.nanoTime() < deadline)
            shadowOf(Looper.getMainLooper()).idle()
            Thread.sleep(5)
        }
    }

    /** Primera lectura amb una novetat vella de Gregal: a partir d'aquí, el que arribi és nou. */
    private fun firstRead(server: Server, vm: GregalViewModel) {
        server.novetats = listOf(old)
        vm.refreshWindNews()
        waitFor("primera lectura") { vm.windNewsCount.value["gregal"] == 1 }
    }

    @Test fun `una novetat nova fa saltar el banner i la insignia a la portada`() {
        Server().use { server ->
            val vm = vm(server)
            ui.mainClock.autoAdvance = false
            ui.setContent {
                CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                    MaterialTheme(colorScheme = LightPal.scheme()) {
                        val news by vm.windNewsCount.collectAsState()
                        val arrival by vm.windArrival.collectAsState()
                        Box {
                            VentsStartPage("ca", "Anna", listOf(gregal, ponent), emptyList(), "", {}, busy = false,
                                onSend = {}, onWind = {}, onThread = {}, news = news, bounce = arrival)
                            WindArrivalBanner(vm, "ca", listOf(gregal, ponent))
                        }
                    }
                }
            }
            firstRead(server, vm)
            ui.mainClock.advanceTimeBy(500)
            ui.onNodeWithTag("wind-arrival").assertDoesNotExist()

            server.novetats = listOf(old, novetat("n1", "# Barcelona: Daily Weather Bulletin\\nSol i 18 graus"))
            vm.refreshWindNews()
            waitFor("arriba la novetat") { vm.windArrival.value != null }
            ui.mainClock.advanceTimeBy(800)
            ui.onNodeWithTag("wind-arrival").assertIsDisplayed()
            ui.onNodeWithText("Ponent t'ha escrit: Barcelona: Daily Weather Bulletin").assertExists()
            ui.onNodeWithTag("start-wind-badge-ponent", useUnmergedTree = true).assertExists()

            // Tocar-lo obre el xat de Ponent al missatge de la novetat.
            ui.onNodeWithTag("wind-arrival").performClick()
            ui.mainClock.advanceTimeBy(800)
            assertEquals("wind:ponent", vm.ventsOpen.value)
            assertEquals("m-n1", MessageFocus.request.value?.messageId)
            MessageFocus.request.value = null
            assertNull(vm.windArrival.value)
            ui.onNodeWithTag("wind-arrival").assertDoesNotExist()
        }
    }

    @Test fun `lliscar el banner cap amunt el treu`() {
        Server().use { server ->
            val vm = vm(server)
            ui.mainClock.autoAdvance = false
            ui.setContent {
                CompositionLocalProvider(LocalPal provides DarkPal, LocalWindMotion provides false) {
                    MaterialTheme(colorScheme = DarkPal.scheme()) { WindArrivalBanner(vm, "en", listOf(ponent)) }
                }
            }
            firstRead(server, vm)
            server.novetats = listOf(old, novetat("n2", "Rain at five"))
            vm.refreshWindNews()
            waitFor("arriba la novetat") { vm.windArrival.value != null }
            ui.mainClock.advanceTimeBy(800)
            ui.onNodeWithText("Ponent wrote to you: Rain at five").assertExists()
            ui.onNodeWithTag("wind-arrival").performTouchInput { swipeUp(startY = bottom, endY = top - 300f) }
            ui.mainClock.advanceTimeBy(800)
            assertNull(vm.windArrival.value)
            assertNull(vm.ventsOpen.value)
            ui.onNodeWithTag("wind-arrival").assertDoesNotExist()
        }
    }

    @Test fun `les novetats que ja hi eren en obrir l'app no fan banner`() {
        Server().use { server ->
            server.novetats = listOf(novetat("n0", "Ahir"))
            val vm = vm(server)
            vm.refreshWindNews()
            waitFor("primera lectura") { vm.windNewsCount.value["ponent"] == 1 }
            assertNull(vm.windArrival.value)
        }
    }

    @Test fun `una novetat recarrega les recents i la mes nova va primer`() {
        Server().use { server ->
            val vm = vm(server)
            firstRead(server, vm)
            val before = server.count("GET /api/winds/threads")
            server.novetats = listOf(old, novetat("n3", "Bon dia"))
            vm.refreshWindNews()
            waitFor("recents rellegides") { server.count("GET /api/winds/threads") > before && vm.windThreads.value.isNotEmpty() }
            assertEquals(listOf("company-ponent-s2", "company-garbi-s1"), vm.windThreads.value.map { it.name })
        }
    }

    @Test fun `en acabar un torn es tornen a llegir les recents`() {
        Server().use { server ->
            val vm = vm(server)
            vm.sendWindMessage("garbi", "Hola")
            waitFor("el torn acaba") { server.count("POST /api/winds/chat") >= 1 && "garbi" !in vm.windBusy.value }
            waitFor("recents rellegides") { server.count("GET /api/winds/threads") >= 1 }
            assertTrue(server.requests.indexOfFirst { it.startsWith("GET /api/winds/threads") } >
                server.requests.indexOfFirst { it.startsWith("POST /api/winds/chat") })
        }
    }

    @Test fun `al calaix el vent amb novetats surt encara que no hi hagis parlat`() {
        val winds = listOf(gregal) + (1..6).map { Wind("v$it", "V$it", "#888888", "") } + ponent
        assertEquals(listOf("v1", "v2", "v3", "v4", "v5"), drawerRecentWinds(winds, emptyMap()).map { it.id })
        assertEquals("ponent", drawerRecentWinds(winds, emptyMap(), mapOf("ponent" to 2)).first().id)
    }

    /** Servidor de prova: novetats canviables, dues recents desordenades i un torn que acaba de seguida. */
    private class Server : AutoCloseable {
        private val socket = ServerSocket(0)
        val port: Int get() = socket.localPort
        val requests: MutableList<String> = Collections.synchronizedList(mutableListOf())
        @Volatile var novetats: List<String> = emptyList()
        fun count(prefix: String) = synchronized(requests) { requests.count { it.startsWith(prefix) } }
        private val acceptor = thread(isDaemon = true) {
            while (!socket.isClosed) {
                val client = try { socket.accept() } catch (_: Exception) { break }
                thread(isDaemon = true) { client.use { answer(it) } }
            }
        }

        private fun answer(client: java.net.Socket) {
            val reader = client.getInputStream().bufferedReader()
            val line = reader.readLine() ?: return
            var length = 0
            while (true) {
                val header = reader.readLine() ?: break
                if (header.isEmpty()) break
                if (header.startsWith("Content-Length:", ignoreCase = true)) length = header.substringAfter(':').trim().toInt()
            }
            repeat(length) { reader.read() }
            requests += line
            val out = client.getOutputStream()
            fun json(body: String) = out.write(("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: ${body.toByteArray().size}\r\nConnection: close\r\n\r\n" + body).toByteArray())
            when {
                line.startsWith("GET /api/winds/novetats") -> json("""{"novetats":[${novetats.joinToString(",")}]}""")
                // Desordenades a posta: la més nova ha d'anar primer.
                line.startsWith("GET /api/winds/threads") -> json("""{"threads":[
                    {"wind":"garbi","name":"company-garbi-s1","title":"Pla de la setmana","at":"2026-11-24T09:00:00+01:00","msgs":4},
                    {"wind":"ponent","name":"company-ponent-s2","title":"El temps","at":"2026-11-24T08:30:00Z","msgs":2}]}""")
                line.startsWith("POST /api/winds/chat/stream") -> {
                    out.write("HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nConnection: close\r\n\r\n".toByteArray())
                    out.write("event: done\ndata: {\"reply\":\"Fet\",\"wind\":\"garbi\",\"history\":[]}\n\n".toByteArray())
                }
                line.startsWith("POST /api/winds/chat") -> json("""{"reply":"Fet","wind":"garbi","history":[]}""")
                else -> json("{}")
            }
            out.flush()
        }

        override fun close() { socket.close(); acceptor.join(1000) }
    }
}
