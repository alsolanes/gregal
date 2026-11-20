package xyz.solanes.gregal

import android.app.Application
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import java.time.Duration
import java.util.concurrent.TimeUnit
import kotlin.concurrent.thread
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.LooperMode

/** Cada sessió del xat de Codi és una conversa: la llista recorda on eres a cadascuna, no una per a totes. */
@RunWith(RobolectricTestRunner::class)
@LooperMode(LooperMode.Mode.PAUSED)
class CodeChatKeyTest {
    @Test fun `reprendre una altra sessio canvia de conversa i tornar-hi recupera la mateixa`() {
        ResumeServer().use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(Conn("127.0.0.1", server.port.toString(), "token", "alice"))
            fun resume(name: String): String {
                val before = server.resumed
                vm.resumeSession(name, navigate = false)
                val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
                while ((server.resumed == before || vm.codeChat.value == "") && System.nanoTime() < deadline) {
                    shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(20)); Thread.sleep(5)
                }
                repeat(20) { shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(20)); Thread.sleep(5) }
                return vm.codeChat.value
            }
            val a = resume("A")
            val b = resume("B")
            assertNotEquals("una altra sessió, una altra conversa", a, b)
            assertEquals("la mateixa sessió, la mateixa conversa", a, resume("A"))
        }
    }

    /** Respon /api/resume amb un fil buit i qualsevol altra petició amb un 404. */
    private class ResumeServer : AutoCloseable {
        private val socket = ServerSocket(0)
        val port: Int get() = socket.localPort
        @Volatile var resumed = 0
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
            val ok = line.startsWith("POST /api/resume")
            val body = if (ok) """{"transcript":[]}""" else "{}"
            client.getOutputStream().apply {
                write(("HTTP/1.1 ${if (ok) "200 OK" else "404 Not Found"}\r\nContent-Type: application/json\r\n" +
                    "Content-Length: ${body.length}\r\nConnection: close\r\n\r\n$body").toByteArray())
                flush()
            }
            if (ok) resumed++
        }

        override fun close() {
            socket.close()
            acceptor.join(1000)
        }
    }
}
