package xyz.solanes.gregal

import android.app.Application
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import java.net.UnknownHostException
import java.time.Duration
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
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
 * Un torn de Codi amb l'app en segon pla: Android talla la xarxa i cada
 * consulta del registre falla com si el servidor no hi fos. El torn segueix
 * corrent al servidor, i l'app no l'ha de donar per perdut.
 */
@RunWith(RobolectricTestRunner::class)
@LooperMode(LooperMode.Mode.PAUSED)
class CodeTurnBackgroundTest {
    @After fun closeTurn() {
        AppVisibility.foreground = false
        GregalNotifications.workFinished(ApplicationProvider.getApplicationContext(), false)
        GregalNotifications.serviceGone()
    }

    private fun idleUntil(timeoutSec: Long, done: () -> Boolean) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(timeoutSec)
        while (!done() && System.nanoTime() < deadline) {
            shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(500))
            Thread.sleep(5)
        }
    }

    private fun connected(server: RunServer): GregalViewModel {
        val app = ApplicationProvider.getApplicationContext<Application>()
        val vm = GregalViewModel(app)
        // Com fa l'activitat: connect() llança syncCollective, que llegeix les prefs.
        vm.attachPrefs(Prefs(app))
        vm.setConn(Conn("127.0.0.1", server.port.toString(), "token", "alice"))
        vm.connect()
        idleUntil(10) { vm.connected.value }
        assertTrue("connectat", vm.connected.value)
        assertTrue("servidor amb la cua v2", vm.v2Runs.value)
        return vm
    }

    private fun errors(vm: GregalViewModel) = vm.msgs.value.filter { it.kind == "error" }

    @Test fun `en segon pla un torn sense xarxa espera en lloc de donar-se per perdut`() {
        // Més talls dels 30 reintents que es permetien, encara que OkHttp en reintenti algun.
        RunServer(dropEvents = 80).use { server ->
            val vm = connected(server)
            AppVisibility.foreground = false
            vm.send("hola")
            idleUntil(30) { !vm.sending.value }
            assertFalse("el torn acaba", vm.sending.value)
            assertTrue("s'han tallat totes les consultes previstes", server.dropped.get() >= 80)
            assertEquals(emptyList<ChatMsg>(), errors(vm))
            assertTrue("arriba la resposta", vm.msgs.value.any { it.role == "assistant" && it.text == RunServer.ANSWER })
        }
    }

    @Test fun `en tornar a primer pla es torna a enganxar al torn que corre`() {
        RunServer(dropEvents = 80).use { server ->
            val vm = connected(server)
            AppVisibility.foreground = true
            vm.send("hola")
            idleUntil(30) { !vm.sending.value }
            assertFalse("el torn s'atura sense xarxa", vm.sending.value)
            assertEquals("cap bombolla d'error amb l'adreça crua", emptyList<ChatMsg>(), errors(vm))
            assertFalse(vm.msgs.value.any { it.role == "assistant" && it.text == RunServer.ANSWER })

            server.toDrop.set(0)
            server.active.set(true)
            vm.onForegrounded()
            idleUntil(30) { vm.msgs.value.any { it.role == "assistant" && it.text == RunServer.ANSWER } && !vm.sending.value }
            assertTrue("els events del torn hi són", vm.msgs.value.any { it.role == "assistant" && it.text == RunServer.ANSWER })
            assertFalse("el torn acaba", vm.sending.value)
            assertEquals("la tasca no es repeteix", 1, vm.msgs.value.count { it.role == "user" && it.text == "hola" })
            assertEquals("el que ja s'havia vist abans del tall no es repeteix", 1,
                vm.msgs.value.count { it.role == "assistant" && it.text == RunServer.EARLY })
        }
    }

    @Test fun `un servidor que no es resol dona el missatge de connexio i no l'adreca`() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        val msg = vm.requestMessage(UnknownHostException("Unable to resolve host \"casa.example.net\": No address associated with hostname"))
        assertEquals(tr(vm.idioma.value, "app.status.requestConnection"), msg)
    }

    /**
     * Servidor v2 mínim amb un sol torn (id 7). Les primeres [dropEvents]
     * consultes del registre es tallen sense resposta, com quan no hi ha xarxa.
     */
    private class RunServer(dropEvents: Int) : AutoCloseable {
        companion object {
            const val EARLY = "Començo per llegir el codi."
            const val ANSWER = "Fet: tot llest."
        }

        private val socket = ServerSocket(0)
        val port: Int get() = socket.localPort
        val dropped = AtomicInteger()
        /** Consultes del registre que encara es tallaran. */
        val toDrop = AtomicInteger(dropEvents)
        /** El servidor diu que el torn encara corre (GET /api/active). */
        val active = AtomicBoolean(false)
        private val firstPageServed = AtomicBoolean(false)
        private val events = listOf(
            """{"id":1,"session_id":"mobil","run_id":7,"kind":"assistant","text":"$EARLY"}""",
            """{"id":2,"session_id":"mobil","run_id":7,"kind":"assistant","text":"$ANSWER"}""",
            """{"id":3,"session_id":"mobil","run_id":7,"kind":"run_completed","text":""}""",
        )
        private val acceptor = thread(isDaemon = true, name = "run-server") {
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
            val path = line.split(" ").getOrElse(1) { "" }
            val body = when {
                path.startsWith("/api/health") ->
                    """{"status":"ok","capabilities":{"runs":true,"durable_events":true,"events":true,"interactive_events":true}}"""
                path.startsWith("/api/state") -> """{"mode":"code"}"""
                line.startsWith("POST /api/v2/runs") -> """{"run":{"id":7,"state":"queued"},"cursor":0}"""
                path.startsWith("/api/v2/events") -> {
                    // La primera consulta arriba abans del tall i només porta el primer event.
                    val upTo = if (!firstPageServed.getAndSet(true)) 1 else events.size
                    if (upTo == events.size && toDrop.getAndUpdate { maxOf(0, it - 1) } > 0) {
                        dropped.incrementAndGet()
                        return
                    }
                    val after = path.substringAfter("after=").substringBefore('&').toLong()
                    val page = events.take(upTo).drop(after.toInt())
                    """{"events":[${page.joinToString(",")}],"cursor":$after,"next":${maxOf(after, upTo.toLong())}}"""
                }
                path.startsWith("/api/v2/runs/7") ->
                    """{"run":{"id":7,"state":"${if (toDrop.get() == 0) "completed" else "running"}"}}"""
                path.startsWith("/api/active") ->
                    if (active.get()) """{"active":{"id":7,"task":"hola","started_at":"2026-01-01T00:00:00Z"}}""" else "{}"
                else -> null
            }
            val out = client.getOutputStream()
            val bytes = (body ?: "{}").toByteArray(Charsets.UTF_8)
            val status = if (body == null) "404 Not Found" else "200 OK"
            out.write("HTTP/1.1 $status\r\nContent-Type: application/json\r\nContent-Length: ${bytes.size}\r\nConnection: close\r\n\r\n".toByteArray())
            out.write(bytes)
            out.flush()
        }

        override fun close() {
            socket.close()
            acceptor.join(1000)
        }
    }
}
