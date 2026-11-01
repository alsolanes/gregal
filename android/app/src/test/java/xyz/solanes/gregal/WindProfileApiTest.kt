package xyz.solanes.gregal

import android.app.Application
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlin.concurrent.thread
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.LooperMode

/** Explicit HTTP test fixtures; no real model is called by these contract tests. */
@RunWith(RobolectricTestRunner::class)
@LooperMode(LooperMode.Mode.PAUSED)
class WindProfileApiTest {
    @Test fun profileSeparatesActualPersonalityTopicAndPersonalMemory() {
        val p = WindProfile.parse("""{"wind":{"id":"stable","name":"Marinada","color":"#EFC69A","role":"calma","style":"bsn","persona":"Parlo amb calma."},"knowledge":"- El tema que conec","personal_memory":"- Una preferència consentida"}""")
        assertEquals("stable", p.wind.id)
        assertEquals("Parlo amb calma.", p.wind.persona)
        assertEquals("- El tema que conec", p.knowledge)
        assertEquals("- Una preferència consentida", p.personalMemory)
    }

    @Test fun libraryKeepsRecordIdentityKindsAndPrivateSnapshotBodies() {
        val items = GregalApi.parseWindLibrary("""{"items":[{"id":"record-7","kind":"archived","wind":{"id":"wind-2","name":"Marinada","persona":"Calma"},"memory":"private memory","knowledge":"topic","skills":[{"Nom":"notes","Titol":"Notes","Descripcio":"Description","Cos":"original body","Font":"private-path-not-displayed"}],"created":"2026-10-30T10:00:00Z"},{"id":"version-1","kind":"version","wind":{"id":"wind-2"}},{"id":"recovered-wind-3","kind":"recovered","wind":{"id":"wind-3"}}]}""")
        assertEquals("record-7", items[0].id)
        assertEquals("wind-2", items[0].wind.id)
        assertEquals("private memory", items[0].memory)
        assertEquals("original body", items[0].skills.single().cos)
        assertTrue(items[0].restorable)
        assertFalse(items[1].restorable)
        assertTrue(items[2].restorable)
        assertEquals("2026-10-30T10:00:00Z", items[0].created)
    }

    @Test fun libraryRejectsOldServerHtmlAndMissingItemsInsteadOfShowingAnEmptyArchive() {
        listOf("<html><body>Gregal</body></html>", "{}", "{\"items\":null}").forEach { raw ->
            assertTrue(runCatching { GregalApi.parseWindLibrary(raw) }.isFailure)
        }
        assertTrue(GregalApi.parseWindLibrary("{\"items\":[]}").isEmpty())
        assertTrue(appStringsOk())
        listOf("ca", "en").forEach { lang ->
            assertFalse(tr(lang, "app.library.restoreConfirm", "Marinada").contains("%1"))
            assertTrue(tr(lang, "app.library.assignConfirm", "Marinada", "Xaloc").contains("Xaloc"))
        }
    }

    @Test fun saveWindSendsStableIdWhenRenaming() {
        val server = ServerSocket(0)
        var requestBody = ""
        val worker = thread(isDaemon = true) {
            server.accept().use { s ->
                val reader = s.getInputStream().bufferedReader()
                var length = 0
                while (true) {
                    val line = reader.readLine() ?: break
                    if (line.isEmpty()) break
                    if (line.startsWith("Content-Length:", ignoreCase = true)) length = line.substringAfter(':').trim().toInt()
                }
                val chars = CharArray(length)
                var offset = 0
                while (offset < length) { val n = reader.read(chars, offset, length - offset); if (n < 0) break; offset += n }
                requestBody = String(chars)
                s.getOutputStream().write("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}".toByteArray())
            }
        }
        try {
            Conn("127.0.0.1", server.localPort.toString(), "test", "alice").api().saveWind("Un altre nom", "#EFC69A", "calma", "bsn", "Calm", "stable-id")
            worker.join(2000)
            assertEquals("stable-id", JSONObject(requestBody).getString("id"))
            assertEquals("Un altre nom", JSONObject(requestBody).getString("name"))
        } finally { server.close(); worker.join(1000) }
    }

    private class LibraryServer(val reply: (String, String) -> Pair<Int, String>) : java.io.Closeable {
        val socket = ServerSocket(0)
        val worker = thread(isDaemon = true) {
            try {
                while (!socket.isClosed) socket.accept().use { s ->
                    val reader = s.getInputStream().bufferedReader()
                    val request = reader.readLine().orEmpty()
                    var length = 0
                    while (true) {
                        val line = reader.readLine() ?: break
                        if (line.isEmpty()) break
                        if (line.startsWith("Content-Length:", true)) length = line.substringAfter(':').trim().toInt()
                    }
                    val chars = CharArray(length)
                    var offset = 0
                    while (offset < length) { val n = reader.read(chars, offset, length - offset); if (n < 0) break; offset += n }
                    val (code, body) = reply(request, String(chars))
                    val bytes = body.toByteArray(Charsets.UTF_8)
                    s.getOutputStream().apply {
                        val type = if (body.startsWith("event:")) "text/event-stream" else "application/json"
                        write("HTTP/1.1 $code Response\r\nContent-Type: $type\r\nContent-Length: ${bytes.size}\r\nConnection: close\r\n\r\n".toByteArray())
                        write(bytes); flush()
                    }
                }
            } catch (_: java.net.SocketException) { }
        }
        fun conn() = Conn("127.0.0.1", socket.localPort.toString(), "fixture", "library-test")
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

    private val archiveJson = """{"id":"archive-7","kind":"archived","wind":{"id":"wind-2","name":"Marinada","color":"#AABBCC","role":"calma","style":"pso","persona":"Calma"},"memory":"private memory","knowledge":"topic","skills":[]}"""
    private fun libraryJson(active: Boolean = false): String = "{\"items\":[$archiveJson" +
        (if (active) "," + archiveJson.replace("archive-7", "active-wind-2").replace("archived", "active") else "") + "]}"

    @Test fun libraryRestoreWaitsForExactReadbackAndRetryReconcilesWithoutDuplicatingRestore() {
        var restored = false
        var posts = 0
        var rejectReadback = true
        val readbackStarted = CountDownLatch(1)
        val releaseReadback = CountDownLatch(1)
        LibraryServer { request, body ->
            when {
                request.startsWith("POST /api/winds/library") -> {
                    assertEquals("archive-7", JSONObject(body).getString("source"))
                    assertEquals("restore", JSONObject(body).getString("action"))
                    posts++; restored = true; 200 to "{\"ok\":true}"
                }
                request.startsWith("GET /api/winds/library") -> {
                    if (restored && rejectReadback) {
                        readbackStarted.countDown(); releaseReadback.await(4, TimeUnit.SECONDS)
                        rejectReadback = false
                        503 to "unavailable"
                    } else 200 to libraryJson(restored)
                }
                else -> 200 to "{\"winds\":[{\"id\":\"wind-2\",\"name\":\"Marinada\"}]}"
            }
        }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(server.conn())
            val item = GregalApi.parseWindLibrary(libraryJson()).single()
            var result: Boolean? = null
            vm.restoreWindLibrary(item) { result = it }
            shadowOf(Looper.getMainLooper()).idle()
            waitUntil { readbackStarted.count == 0L }
            assertEquals("archive-7", vm.windLibrary.value.pending)
            assertNull(result)
            releaseReadback.countDown()
            waitUntil { result != null }
            assertEquals(false, result)
            assertNotNull(vm.windLibrary.value.error)
            result = null
            vm.restoreWindLibrary(item) { result = it }
            waitUntil { result != null }
            assertEquals(true, result)
            assertEquals(1, posts)
            assertEquals("wind-2", vm.winds.value.single().id)
            assertNull(vm.windLibrary.value.pending)
        }
    }

    @Test fun libraryAssignmentSendsOnlyCheckedInformationAndRejectsChangedSourceAndConflict() {
        val targetJson = """{"id":"active-target","kind":"active","wind":{"id":"target","name":"Xaloc"},"memory":"existing","knowledge":"keep topic","skills":[]}"""
        var assigned = false
        var posts = 0
        var changed = false
        var conflict = false
        fun snapshot(): String = "{\"items\":[" + (if (changed) archiveJson.replace("private memory", "edited elsewhere") else archiveJson) + "," +
            (if (assigned) targetJson.replace("existing", "existing\\n\\nprivate memory") else targetJson) + "]}"
        LibraryServer { request, body ->
            when {
                request.startsWith("POST") -> {
                    posts++
                    val payload = JSONObject(body)
                    assertEquals("archive-7", payload.getString("source"))
                    assertEquals("target", payload.getString("target"))
                    assertTrue(payload.getBoolean("memory"))
                    assertFalse(payload.getBoolean("knowledge"))
                    if (conflict) 409 to "knowledge conflict" else { assigned = true; 200 to "{\"ok\":true}" }
                }
                request.startsWith("GET /api/winds/library") -> 200 to snapshot()
                else -> 200 to "{\"winds\":[{\"id\":\"target\",\"name\":\"Xaloc\"}]}"
            }
        }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(server.conn())
            val records = GregalApi.parseWindLibrary(snapshot())
            var result: Boolean? = null
            changed = true
            vm.assignWindLibrary(records[0], records[1].wind, true, false) { result = it }
            waitUntil { result != null }
            assertEquals(false, result); assertEquals(0, posts)
            changed = false; conflict = true; result = null
            vm.assignWindLibrary(records[0], records[1].wind, true, false) { result = it }
            waitUntil { result != null }
            assertEquals(false, result); assertNotNull(vm.windLibrary.value.error)
            conflict = false; result = null
            vm.assignWindLibrary(records[0], records[1].wind, true, false) { result = it }
            waitUntil { result != null }
            assertEquals(true, result)
            val active = vm.windLibrary.value.items!!.single { it.wind.id == "target" }
            assertEquals("existing\n\nprivate memory", active.memory)
            assertEquals("keep topic", active.knowledge)
        }
    }

    @Test fun memorySaveReturnsFalseForDifferentReadbackAndKeepsAcknowledgedCache() {
        var actual = "old value"
        LibraryServer { request, _ ->
            if (request.startsWith("PUT")) 200 to "{}"
            else 200 to JSONObject().put("memoria", actual).toString()
        }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(server.conn())
            var result: Boolean? = null
            vm.saveVentMemoria("wind-2", "draft") { result = it }
            waitUntil { result != null }
            assertEquals(false, result)
            assertNull(vm.ventDetall.value["wind-2"])
            actual = "draft"; result = null
            vm.saveVentMemoria("wind-2", "draft") { result = it }
            waitUntil { result != null }
            assertEquals(true, result)
            assertEquals("draft", vm.ventDetall.value["wind-2"]?.first)
        }
    }

    @Test fun acceptedStreamFailureNeverSubmitsTheSameWorkAgain() {
        val classic = java.util.concurrent.atomic.AtomicInteger()
        LibraryServer { request, _ ->
            val method = request.substringBefore(" ")
            val path = request.substringAfter(" ").substringBefore(" ")
            200 to when {
                path.endsWith("/stream") -> "event: error\ndata: {\"error\":\"provider timed out\"}\n\n"
                method == "POST" && (path == "/api/winds/chat" || path == "/api/winds/rotllana") -> {
                    classic.incrementAndGet()
                    if (path.endsWith("rotllana")) "{\"reply\":{\"wind\":\"w\",\"text\":\"duplicate\"},\"history\":[]}" else "{\"reply\":\"duplicate\"}"
                }
                else -> "{}"
            }
        }.use { server ->
            val connection = server.conn()
            val app = ApplicationProvider.getApplicationContext<Application>()
            val vm = GregalViewModel(app)
            @Suppress("UNCHECKED_CAST")
            (GregalViewModel::class.java.getDeclaredField("_conn").apply { isAccessible = true }
                .get(vm) as kotlinx.coroutines.flow.MutableStateFlow<Conn>).value = connection
            vm.sendWindMessage("w", "Do this once")
            waitUntil { vm.windBusy.value.isEmpty() }
            vm.sendRotllanaMessage("r", "Do this once")
            waitUntil { vm.rotllanaBusy.value.isEmpty() }
            assertEquals("an accepted SSE error must not trigger another POST", 0, classic.get())
            assertEquals("Do this once", vm.windChat.value["w"]?.first()?.text)
            assertEquals("Do this once", vm.rotllanaChat.value["r"]?.first()?.text)
        }
    }

    @Test fun missingStreamRouteFallsBackOnlyOnceForLegacyServers() {
        val posts = java.util.concurrent.atomic.AtomicInteger()
        LibraryServer { request, _ ->
            when {
                request.startsWith("POST /api/winds/chat/stream ") -> 404 to "absent"
                request.startsWith("POST /api/winds/chat ") -> { posts.incrementAndGet(); 200 to "{\"reply\":\"Legacy reply\"}" }
                else -> 200 to "{}"
            }
        }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            @Suppress("UNCHECKED_CAST")
            (GregalViewModel::class.java.getDeclaredField("_conn").apply { isAccessible = true }
                .get(vm) as kotlinx.coroutines.flow.MutableStateFlow<Conn>).value = server.conn()
            vm.sendWindMessage("w", "One operation")
            waitUntil { vm.windBusy.value.isEmpty() }
            assertEquals(1, posts.get())
            assertEquals("Legacy reply", vm.windChat.value["w"]?.last()?.text)
        }
    }

    @Test fun heartbeatEnablingDisabledJobWaitsForVerifiedEnabledReadback() {
        var enabled = false
        var acceptWrite = false
        LibraryServer { request, body ->
            if (request.startsWith("POST")) {
                assertTrue(JSONObject(body).getBoolean("enabled"))
                if (acceptWrite) enabled = true
                200 to "{}"
            } else 200 to "{\"jobs\":[{\"id\":\"wind-wind-2\",\"name\":\"Heartbeat\",\"enabled\":$enabled}]}"
        }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(server.conn())
            val wind = Wind("wind-2", "Marinada", "#AABBCC", "calma")
            var result: Boolean? = null
            vm.ensureWindHeartbeat(wind, true) { result = it }
            waitUntil { result != null }
            assertEquals(false, result)
            assertTrue(vm.jobs.value.isEmpty())
            acceptWrite = true; result = null
            vm.ensureWindHeartbeat(wind, true) { result = it }
            waitUntil { result != null }
            assertEquals(true, result)
            assertTrue(vm.jobs.value.single().enabled)
        }
    }

    @Test fun failedSuggestionReleasesPendingCreatorState() {
        val server = ServerSocket(0)
        val done = CountDownLatch(1)
        val worker = thread(isDaemon = true) {
            server.accept().use { s ->
                val reader = s.getInputStream().bufferedReader()
                while (!reader.readLine().isNullOrEmpty()) { }
                s.getOutputStream().write("HTTP/1.1 503 Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".toByteArray())
                done.countDown()
            }
        }
        try {
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(Conn("127.0.0.1", server.localPort.toString(), "test", "alice"))
            var failed = false
            vm.suggestWind("Marinada", "calma", onError = { failed = true }) { _, _ -> fail("unexpected success") }
            shadowOf(Looper.getMainLooper()).idle()
            assertTrue(done.await(3, TimeUnit.SECONDS))
            repeat(15) { Thread.sleep(20); shadowOf(Looper.getMainLooper()).idle() }
            assertTrue("pending UI receives failure callback", failed)
        } finally { server.close(); worker.join(1000) }
    }
}
