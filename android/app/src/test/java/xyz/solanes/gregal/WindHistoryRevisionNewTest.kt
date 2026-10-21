package xyz.solanes.gregal

import android.app.Application
import android.os.Looper
import androidx.lifecycle.viewModelScope
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import java.util.concurrent.ConcurrentLinkedQueue
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlin.concurrent.thread
import kotlinx.coroutines.Job
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.LooperMode

@RunWith(RobolectricTestRunner::class)
@LooperMode(LooperMode.Mode.PAUSED)
class WindHistoryRevisionNewTest {
    private fun vm(server: HistoryServer) =
        GregalViewModel(ApplicationProvider.getApplicationContext<Application>()).apply {
            setConn(Conn("127.0.0.1", server.port.toString(), "token", "alice"))
        }

    // Observe completion of the actual ViewModel request, not just socket writes.
    private fun request(vm: GregalViewModel, action: () -> Unit): Job {
        val before = vm.viewModelScope.coroutineContext[Job]!!.children.toSet()
        action()
        return vm.viewModelScope.coroutineContext[Job]!!.children.single { it !in before }
    }

    private fun completed(job: Job) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (!job.isCompleted && System.nanoTime() < deadline) {
            shadowOf(Looper.getMainLooper()).idle()
            Thread.sleep(5)
        }
        assertTrue("ViewModel request completed", job.isCompleted)
    }

    private fun texts(vm: GregalViewModel, group: Boolean, id: String = "same-id") =
        (if (group) vm.rotllanaChat.value[id] else vm.windChat.value[id]).orEmpty().map { it.text }

    private fun load(vm: GregalViewModel, group: Boolean, id: String = "same-id") =
        request(vm) { if (group) vm.loadRotllanaChat(id) else vm.loadWindChat(id) }

    private fun endpoint(group: Boolean, id: String = "same-id") =
        if (group) "/api/winds/rotllana?rotlla=$id" else "/api/winds/chat?wind=$id"

    private fun history(text: String) = """{"history":[{"from":"pet","text":"$text"}]}"""

    @Test fun delayedWindGetCannotOverwriteCompletedPost() = staleGetAfterPost(group = false)
    @Test fun delayedRotllanaGetCannotOverwriteCompletedPost() = staleGetAfterPost(group = true)
    @Test fun latestWindGetWinsWhenResponsesArriveInReverseOrder() = latestGetWins(group = false)
    @Test fun latestRotllanaGetWinsWhenResponsesArriveInReverseOrder() = latestGetWins(group = true)

    private fun latestGetWins(group: Boolean) {
        HistoryServer().use { server ->
            val vm = vm(server)
            val old = server.expect("GET ${endpoint(group)}", history("older request"))
            val first = load(vm, group)
            old.awaitRequest()
            val fresh = server.expect("GET ${endpoint(group)}", history("latest request"))
            val second = load(vm, group)
            fresh.awaitRequest()
            fresh.release.countDown()
            completed(second)
            assertEquals(listOf("latest request"), texts(vm, group))
            old.release.countDown()
            completed(first)
            server.assertHealthy()
            assertEquals("older GET must not replace the latest requested snapshot",
                listOf("latest request"), texts(vm, group))
        }
    }

    private fun staleGetAfterPost(group: Boolean) {
        HistoryServer().use { server ->
            val vm = vm(server)
            val old = server.expect("GET ${endpoint(group)}", history("old snapshot"))
            val get = load(vm, group)
            old.awaitRequest()
            // El directe primer: el client prova /stream i cau al clàssic
            // només si falla. El done tanca el torn igual que el POST.
            val streamBody = if (group)
                "event: phase\ndata: {\"phase\":\"thinking\"}\n\n" +
                "event: token\ndata: {\"token\":\"fresh \"}\n\n" +
                "event: token\ndata: {\"token\":\"reply\"}\n\n" +
                "event: done\ndata: " +
                "{\"reply\":{\"wind\":\"tramuntana\",\"text\":\"fresh reply\"}," +
                "\"history\":[{\"from\":\"user\",\"text\":\"new message\"}," +
                "{\"from\":\"pet\",\"text\":\"fresh reply\",\"speaker_id\":\"tramuntana\",\"speaker_name\":\"Tramuntana\"}]," +
                "\"actions\":[]}\n\n"
            else
                "event: phase\ndata: {\"phase\":\"thinking\"}\n\n" +
                "event: token\ndata: {\"token\":\"fresh \"}\n\n" +
                "event: token\ndata: {\"token\":\"reply\"}\n\n" +
                "event: done\ndata: " +
                "{\"reply\":\"fresh reply\",\"wind\":\"same-id\"," +
                "\"history\":[{\"from\":\"user\",\"text\":\"new message\"},{\"from\":\"pet\",\"text\":\"fresh reply\"}]," +
                "\"actions\":[]}\n\n"
            val post = server.expect(
                "POST ${if (group) "/api/winds/rotllana/stream" else "/api/winds/chat/stream"}",
                streamBody,
            )
            val send = request(vm) {
                if (group) vm.sendRotllanaMessage("same-id", "new message")
                else vm.sendWindMessage("same-id", "new message")
            }
            post.awaitRequest()
            post.release.countDown()
            completed(send)
            assertFalse("POST has cleared busy before releasing old GET",
                (if (group) vm.rotllanaBusy.value else vm.windBusy.value).contains("same-id"))
            assertEquals(listOf("new message", "fresh reply"), texts(vm, group))
            old.release.countDown()
            completed(get)
            server.assertHealthy()
            assertEquals("late pre-send GET must not erase completed turn",
                listOf("new message", "fresh reply"), texts(vm, group))
        }
    }

    /** Real concurrent HTTP exchanges; each response is released explicitly. */
    private class HistoryServer : AutoCloseable {
        private val socket = ServerSocket(0)
        val port: Int get() = socket.localPort
        private val workers = mutableListOf<Thread>()
        private val exchanges = mutableListOf<Exchange>()
        private val failures = ConcurrentLinkedQueue<Throwable>()

        class Exchange {
            val requested = CountDownLatch(1)
            val release = CountDownLatch(1)
            fun awaitRequest() {
                shadowOf(Looper.getMainLooper()).idle()
                assertTrue("HTTP request arrived", requested.await(5, TimeUnit.SECONDS))
            }
        }

        fun expect(target: String, json: String): Exchange {
            val exchange = Exchange()
            exchanges += exchange
            workers += thread(isDaemon = true, name = "history-response") {
                try {
                    socket.accept().use { client ->
                        client.soTimeout = 5000
                        val reader = client.getInputStream().bufferedReader()
                        val line = reader.readLine()
                        assertEquals("$target HTTP/1.1", line)
                        var length = 0
                        while (true) {
                            val header = reader.readLine() ?: break
                            if (header.isEmpty()) break
                            if (header.startsWith("Content-Length:", ignoreCase = true))
                                length = header.substringAfter(':').trim().toInt()
                        }
                        repeat(length) { check(reader.read() != -1) }
                        exchange.requested.countDown()
                        check(exchange.release.await(10, TimeUnit.SECONDS)) { "response not released" }
                        val body = json.toByteArray(Charsets.UTF_8)
                        val headers = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: ${body.size}\r\nConnection: close\r\n\r\n"
                        client.getOutputStream().apply {
                            write(headers.toByteArray(Charsets.UTF_8)); write(body); flush()
                        }
                    }
                } catch (failure: Throwable) {
                    if (!socket.isClosed) failures += failure
                    exchange.requested.countDown()
                }
            }
            return exchange
        }

        fun assertHealthy() {
            failures.peek()?.let { throw AssertionError("HTTP fixture failed", it) }
        }

        override fun close() {
            exchanges.forEach { it.release.countDown() }
            socket.close()
            workers.forEach { it.join(1000) }
        }
    }
}
