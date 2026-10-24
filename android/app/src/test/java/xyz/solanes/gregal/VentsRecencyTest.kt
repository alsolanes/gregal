package xyz.solanes.gregal

import android.content.Context
import androidx.lifecycle.viewModelScope
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class VentsRecencyTest {
    private val context: Context get() = ApplicationProvider.getApplicationContext()
    private val prefs get() = context.getSharedPreferences("recency-test", Context.MODE_PRIVATE)
    private val conn = Conn("server", "8080", "secret-token", "alice")
    private val history = listOf(GregalViewModel.PetMsg("user", "private question", ts = 123L), GregalViewModel.PetMsg("pet", "private answer"))
    private fun legacy() = history.map { it.copy(ts = 0L) }
    @Before fun clear() { prefs.edit().clear().commit() }

    @Test fun successfulReceiptSurvivesNewInstanceWithoutPersistingContentOrSecrets() {
        val completed = VentsRecency(prefs).completed(conn, "wind", "one", history, 456L)
        assertEquals(123L, completed.first().ts)
        assertEquals(456L, completed.last().ts)
        val restored = VentsRecency(prefs).restore(conn, "wind", "one", legacy())
        assertEquals(listOf(123L, 456L), restored.map { it.ts })
        val stored = prefs.all.toString()
        listOf("secret-token", "private question", "private answer", "alice", "server").forEach { assertFalse(stored.contains(it)) }
    }

    @Test fun accountsConnectionsAndConversationKindsAreIsolated() {
        val store = VentsRecency(prefs)
        store.completed(conn, "wind", "one", history, 456L)
        listOf(conn.copy(user = "bob"), conn.copy(token = "another"), conn.copy(host = "other"), conn.copy(port = "9090")).forEach {
            assertEquals(0L, store.restore(it, "wind", "one", legacy()).last().ts)
        }
        assertEquals(0L, store.restore(conn, "rot", "one", legacy()).last().ts)
        assertEquals(0L, store.restore(conn, "wind", "two", legacy()).last().ts)
    }

    @Test fun localIdentityDoesNotDependOnEphemeralPortOrToken() {
        val store = VentsRecency(prefs)
        store.completed(conn.copy(local = true), "wind", "one", history, 456L)
        assertEquals(456L, store.restore(conn.copy(local = true, port = "12345", token = "restarted"), "wind", "one", legacy()).last().ts)
    }

    @Test fun changedTailOrEarlierHistoryNeverReusesReceipt() {
        val store = VentsRecency(prefs)
        store.completed(conn, "wind", "one", history, 456L)
        assertEquals(0L, store.restore(conn, "wind", "one", legacy().dropLast(1) + history.last().copy(text = "external answer")).last().ts)
        assertEquals(0L, store.restore(conn, "wind", "one", listOf(history.first().copy(text = "external question")) + legacy().last()).last().ts)
    }

    @Test fun unknownEmptyAndStreamingHistoriesNeverAcquireDates() {
        val store = VentsRecency(prefs)
        assertTrue(store.restore(conn, "wind", "one", emptyList()).isEmpty())
        assertEquals(0L, store.restore(conn, "wind", "one", legacy()).last().ts)
        store.completed(conn, "wind", "one", emptyList(), 456L)
        store.completed(conn, "wind", "one", legacy().dropLast(1) + history.last().copy(streaming = true), 456L)
        store.completed(conn, "wind", "one", listOf(GregalViewModel.PetMsg("pet", "")), 456L)
        assertTrue(prefs.all.isEmpty())
    }

    @Test fun viewModelPersistsSuccessfulTurnsAcrossRecreationForBothTransportsAndContinue() {
        for (group in listOf(false, true)) for (stream in listOf(false, true)) for (cont in listOf(false, true)) {
            val initial = if (cont) "[{\"from\":\"pet\",\"text\":\"start\"}]" else "[]"
            val result = if (cont) "[{\"from\":\"pet\",\"text\":\"startanswer\"}]" else
                "[{\"from\":\"user\",\"text\":\"question\"},{\"from\":\"pet\",\"text\":\"answer\"}]"
            var done = false
            val server = ReceiptServer { method, path ->
                val reply = if (group) "{\"reply\":{\"wind\":\"one\",\"text\":\"answer\"},\"history\":$result}" else "{\"reply\":\"answer\"}"
                when {
                    path.endsWith("/stream") && !stream -> ""
                    path.endsWith("/stream") -> { done = true; "event: done\ndata: $reply\n\n" }
                    method == "POST" -> { done = true; reply }
                    path.endsWith("/chat") || path.endsWith("/rotllana") -> "{\"history\":${if (done) result else initial}}"
                    else -> "{}"
                }
            }
            try {
                val app = ApplicationProvider.getApplicationContext<android.app.Application>()
                val connection = Conn("127.0.0.1", server.address.port.toString(), "receipt-$group-$stream-$cont")
                fun vm() = GregalViewModel(app).apply { setConn(connection) }
                fun messages(vm: GregalViewModel) = (if (group) vm.rotllanaChat.value else vm.windChat.value)["one"].orEmpty()
                fun request(vm: GregalViewModel, action: () -> Unit) {
                    val scope = vm.viewModelScope.coroutineContext[kotlinx.coroutines.Job]!!
                    val before = scope.children.toSet()
                    action()
                    val job = scope.children.single { it !in before }
                    val deadline = System.nanoTime() + java.util.concurrent.TimeUnit.SECONDS.toNanos(5)
                    while (!job.isCompleted && System.nanoTime() < deadline) {
                        org.robolectric.Shadows.shadowOf(android.os.Looper.getMainLooper()).idle()
                        Thread.sleep(5)
                    }
                    assertTrue("request completed", job.isCompleted)
                }
                val first = vm()
                request(first) { if (group) first.loadRotllanaChat("one") else first.loadWindChat("one") }
                assertTrue(messages(first).all { it.ts == 0L })
                request(first) {
                    if (group) { if (cont) first.sendRotllanaContinue("one") else first.sendRotllanaMessage("one", "question") }
                    else { if (cont) first.sendWindContinue("one") else first.sendWindMessage("one", "question") }
                }
                val observed = messages(first)
                assertTrue("completion receipt group=$group stream=$stream cont=$cont", observed.last().ts > 0L)
                if (!cont) assertTrue("original user send time survives server history", observed.first().ts > 0L)
                val second = vm()
                request(second) { if (group) second.loadRotllanaChat("one") else second.loadWindChat("one") }
                assertEquals(observed.map { it.ts }, messages(second).map { it.ts })
            } finally { server.stop() }
        }
    }

    private class ReceiptServer(private val response: (String, String) -> String) {
        private val socket = java.net.ServerSocket(0)
        val address: java.net.InetSocketAddress get() = socket.localSocketAddress as java.net.InetSocketAddress
        private val worker = kotlin.concurrent.thread(isDaemon = true) {
            try {
                while (!socket.isClosed) socket.accept().use { client ->
                    client.soTimeout = 5000
                    val reader = client.getInputStream().bufferedReader()
                    val first = reader.readLine().split(' ')
                    var length = 0
                    while (true) {
                        val header = reader.readLine() ?: break
                        if (header.isEmpty()) break
                        if (header.startsWith("Content-Length:", true)) length = header.substringAfter(':').trim().toInt()
                    }
                    repeat(length) { reader.read() }
                    val body = response(first[0], first[1].substringBefore('?')).toByteArray()
                    val headers = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: ${body.size}\r\nConnection: close\r\n\r\n"
                    client.getOutputStream().apply { write(headers.toByteArray()); write(body); flush() }
                }
            } catch (e: java.net.SocketException) { if (!socket.isClosed) throw e }
        }
        fun stop() { socket.close(); worker.join(1000) }
    }

    @Test fun restoredReceiptsKeepInboxOrderAndUnknownHistoryUndated() {
        val store = VentsRecency(prefs)
        store.completed(conn, "wind", "a", history, 400L)
        store.completed(conn, "rot", "g", history, 900L)
        val recreated = VentsRecency(prefs)
        val winds = listOf(Wind("a", "A", "#123456", ""), Wind("b", "B", "#123456", ""))
        val rows = ventsInbox(winds, listOf(Rotllana("g", "G", listOf("a"))),
            mapOf("a" to recreated.restore(conn, "wind", "a", legacy()), "b" to recreated.restore(conn, "wind", "b", legacy())),
            mapOf("g" to recreated.restore(conn, "rot", "g", legacy())))
        assertEquals(listOf("rot:g", "wind:a", "wind:b"), rows.map { it.key })
        assertEquals(listOf(900L, 400L, 0L), rows.map { it.timestamp })
    }

    @Test fun unchangedContinuationDoesNotRedateAnOldMessage() {
        val store = VentsRecency(prefs)
        val previous = store.completed(conn, "rot", "one", history, 456L)
        val noNewReply = store.completed(conn, "rot", "one", legacy(), 999L, previous)
        assertEquals(456L, noNewReply.last().ts)
        assertEquals(456L, VentsRecency(prefs).restore(conn, "rot", "one", legacy()).last().ts)
    }

    @Test fun clearRemovesOnlyOwnConversation() {
        val store = VentsRecency(prefs)
        store.completed(conn, "wind", "one", history, 456L)
        store.completed(conn, "rot", "one", history, 789L)
        store.clear(conn, "wind", "one")
        assertEquals(0L, store.restore(conn, "wind", "one", legacy()).last().ts)
        assertEquals(789L, store.restore(conn, "rot", "one", legacy()).last().ts)
    }
}
