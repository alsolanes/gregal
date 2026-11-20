package xyz.solanes.gregal

import android.app.Application
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import java.time.Duration
import java.util.concurrent.TimeUnit
import kotlin.concurrent.thread
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.LooperMode

/**
 * El text que s'escriu en directe al xat de Codi i el missatge en què es tanca
 * (en una fase d'eina o en acabar el torn) són el mateix element de la llista:
 * si canviés de clau, la llista saltaria en aquell moment.
 */
@RunWith(RobolectricTestRunner::class)
@LooperMode(LooperMode.Mode.PAUSED)
class LiveReplyKeyTest {
    @After fun closeTurn() {
        GregalNotifications.workFinished(ApplicationProvider.getApplicationContext(), false)
        GregalNotifications.serviceGone()
    }

    private val events = listOf(
        "token" to """{"text":"Primera "}""",
        "token" to """{"text":"part."}""",
        "tool_call" to """{"name":"read_file","id":"c1","args":{"path":"a.txt"}}""",
        "tool_result" to """{"name":"read_file","id":"c1","output":"ok"}""",
        "token" to """{"text":"Segona "}""",
        "token" to """{"text":"part."}""",
        "done" to "{}",
    )

    @Test fun `el text en directe es tanca en un missatge amb la mateixa clau`() {
        StreamServer(events).use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(Conn("127.0.0.1", server.port.toString(), "token", "alice"))
            vm.send("hola")
            val seen = mutableMapOf<String, Long>()
            val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(10)
            while (vm.sending.value && System.nanoTime() < deadline) {
                shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(20))
                if (vm.live.value.isNotBlank()) seen[vm.live.value] = vm.liveId.value
                Thread.sleep(5)
            }
            assertFalse("el torn acaba", vm.sending.value)
            val replies = vm.msgs.value.filter { it.role == "assistant" && it.kind == "text" }
            assertEquals(listOf("Primera part.", "Segona part."), replies.map { it.text })
            for (m in replies) {
                val ids = seen.filterKeys { m.text.startsWith(it.trim()) }.values.toSet()
                assertTrue("s'ha vist en directe: ${m.text}", ids.isNotEmpty())
                assertEquals("mateixa clau en directe i acabat: ${m.text}", setOf(m.id), ids)
            }
        }
    }

    /** Respon el torn amb els events SSE d'un en un, i qualsevol altra petició amb un 404. */
    private class StreamServer(private val events: List<Pair<String, String>>) : AutoCloseable {
        private val socket = ServerSocket(0)
        val port: Int get() = socket.localPort
        private val acceptor = thread(isDaemon = true, name = "stream-server") {
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
            val out = client.getOutputStream()
            if (!line.startsWith("POST /api/chat") && !line.startsWith("POST /api/agent")) {
                out.write("HTTP/1.1 404 Not Found\r\nContent-Type: application/json\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}".toByteArray())
                out.flush()
                return
            }
            out.write("HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nConnection: close\r\n\r\n".toByteArray())
            out.flush()
            for ((name, data) in events) {
                Thread.sleep(250)
                out.write("event: $name\ndata: $data\n\n".toByteArray(Charsets.UTF_8))
                out.flush()
            }
        }

        override fun close() {
            socket.close()
            acceptor.join(1000)
        }
    }
}
