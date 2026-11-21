package xyz.solanes.gregal

import android.app.Application
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import java.util.Collections
import java.util.concurrent.TimeUnit
import kotlin.concurrent.thread
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.LooperMode

/**
 * Escriure des de la portada d'inici comença una conversa nova: primer es crea
 * la sessió del vent i el missatge va a aquella sessió, no al fil d'abans.
 */
@RunWith(RobolectricTestRunner::class)
@LooperMode(LooperMode.Mode.PAUSED)
class WindStartConversationTest {

    @Test fun `una conversa nova crea la sessio abans d'enviar-hi el missatge`() {
        Server().use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(Conn("127.0.0.1", server.port.toString(), "token", "alice"))
            // Sense accents: el servidor de prova llegeix el cos per caràcters, no per bytes.
            vm.startWindConversation("gregal", "Que faig dissabte?")
            val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(10)
            fun chat() = server.requests.firstOrNull { it.startsWith("POST /api/winds/chat") }
            var opened = false
            while (chat() == null && System.nanoTime() < deadline) {
                shadowOf(Looper.getMainLooper()).idle()
                // Com la pantalla: en obrir-se la conversa, en carrega el fil.
                if (!opened && vm.ventsOpen.value == "wind:gregal") { opened = true; vm.loadWindChat("gregal") }
                Thread.sleep(5)
            }
            val created = server.requests.indexOfFirst { it.startsWith("POST /api/winds/sessions") }
            val sent = server.requests.indexOfFirst { it.startsWith("POST /api/winds/chat") }
            assertTrue("s'ha de crear la sessió: ${server.requests}", created >= 0)
            assertTrue("el missatge s'ha d'enviar després: ${server.requests}", sent > created)
            assertTrue("a la sessió nova: ${server.requests[sent]}", server.requests[sent].contains("session=company-gregal-s9"))
            // El fil que s'obre és el nou: cap càrrega de l'historial abans de crear la sessió
            // ni d'una altra sessió (abans, la conversa s'obria amb el fil antic).
            server.requests.forEachIndexed { i, r ->
                if (r.startsWith("GET /api/winds/chat?")) {
                    assertTrue("historial carregat abans de la sessió: $r", i > created)
                    assertTrue("historial d'una altra sessió: $r", r.contains("session=company-gregal-s9"))
                }
            }
        }
    }

    /** Desa la primera línia de cada petició; la sessió nova i el torn tenen resposta, la resta un JSON buit. */
    private class Server : AutoCloseable {
        private val socket = ServerSocket(0)
        val port: Int get() = socket.localPort
        val requests: MutableList<String> = Collections.synchronizedList(mutableListOf())
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
                line.startsWith("POST /api/winds/sessions") -> json("""{"wind":"gregal","name":"company-gregal-s9","sessions":[]}""")
                line.startsWith("POST /api/winds/chat/stream") -> {
                    out.write("HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nConnection: close\r\n\r\n".toByteArray())
                    out.write("event: done\ndata: {\"reply\":\"D'acord\",\"wind\":\"gregal\",\"history\":[]}\n\n".toByteArray())
                }
                line.startsWith("POST /api/winds/chat") -> json("""{"reply":"D'acord","wind":"gregal","history":[]}""")
                else -> json("{}")
            }
            out.flush()
        }

        override fun close() { socket.close(); acceptor.join(1000) }
    }
}
