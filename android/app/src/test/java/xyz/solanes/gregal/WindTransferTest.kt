package xyz.solanes.gregal

import android.app.Application
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import java.io.Closeable
import java.net.ServerSocket
import java.net.SocketException
import java.util.Collections
import kotlinx.coroutines.flow.MutableStateFlow
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.LooperMode
import kotlin.concurrent.thread

/**
 * Traspàs al servidor amb servidors HTTP falsos (cap model real):
 * export + import + verificació + obertura del clon; reintent amb la
 * mateixa identitat; conflicte i verificació fallida sense canviar de
 * mode ni de conversa; l'original no rep cap escriptura.
 */
@RunWith(RobolectricTestRunner::class)
@LooperMode(LooperMode.Mode.PAUSED)
class WindTransferTest {
    @Test fun serverNamePrefersHostnamesOverRawIp() {
        assertEquals("", displayServerName(""))
        assertEquals("", displayServerName("10.0.2.2"))
        assertEquals("", displayServerName("192.168.1.84"))
        assertEquals("", displayServerName("127.0.0.1"))
        assertEquals("", displayServerName("localhost"))
        assertEquals("", displayServerName("http://10.0.2.2:8097"))
        assertEquals("vents.exemple.cat", displayServerName("vents.exemple.cat"))
        assertEquals("vents.exemple.cat", displayServerName("https://vents.exemple.cat"))
        assertEquals("vents.exemple.cat", displayServerName("https://vents.exemple.cat:443/gregal"))
        assertEquals("nas", displayServerName("nas"))
    }

    private class Fixture(val handler: (method: String, path: String, body: String) -> Pair<Int, String>) : Closeable {
        val socket = ServerSocket(0)
        val requests = Collections.synchronizedList(mutableListOf<Triple<String, String, String>>())
        val worker = thread(isDaemon = true) {
            try {
                while (!socket.isClosed) socket.accept().use { s ->
                    val reader = s.getInputStream().bufferedReader()
                    val first = reader.readLine().orEmpty().split(" ")
                    val method = first.getOrElse(0) { "GET" }
                    val path = first.getOrElse(1) { "/" }
                    var length = 0
                    while (true) {
                        val line = reader.readLine() ?: break
                        if (line.isEmpty()) break
                        if (line.startsWith("Content-Length:", ignoreCase = true)) length = line.substringAfter(':').trim().toInt()
                    }
                    val chars = CharArray(length)
                    var offset = 0
                    while (offset < length) {
                        val n = reader.read(chars, offset, length - offset)
                        if (n < 0) break
                        offset += n
                    }
                    val body = String(chars, 0, offset)
                    val (code, resp) = try {
                        handler(method, path, body)
                    } catch (e: AssertionError) {
                        500 to "{\"error\":\"fixture: ${e.message}\"}"
                    }
                    requests += Triple(method, path, body)
                    val bytes = resp.toByteArray(Charsets.UTF_8)
                    s.getOutputStream().apply {
                        write("HTTP/1.1 $code Response\r\nContent-Type: application/json\r\nContent-Length: ${bytes.size}\r\nConnection: close\r\n\r\n".toByteArray())
                        write(bytes)
                        flush()
                    }
                }
            } catch (_: SocketException) { }
        }

        fun closeable(): Closeable = this
        override fun close() {
            socket.close()
            worker.join(1000)
        }
    }

    private val bundleBreeze = """{"version":1,"kind":"wind","id":"breeze","winds":[{"profile":{"id":"breeze","name":"Brisa","color":"#A8CDE2","persona":"Calma","role":"recerca","style":"pso"},"memory":"","knowledge":"","skills":[]}],"history":[{"role":"user","text":"Hola","ts":100},{"role":"assistant","text":"Hola!","ts":101}],"consent":{"memory":false,"knowledge":false,"skills":false}}"""

    private fun importResult(transferId: String, replayed: Boolean) =
        """{"transferId":"$transferId","kind":"wind","id":"copy-w-b1","idMap":{"breeze":"copy-w-b1"},"counts":{"winds":1,"messages":2,"memoryBytes":0,"knowledgeBytes":0,"skills":0},"verifyPath":"/api/winds/chat?wind=copy-w-b1","replayed":$replayed}"""

    private val stateJson = """{"role":[],"current_role":"","mode":"","model":"","provider":"","verify":"manual","agent_busy":false,"cwd":"","project":"","branch":"","used_tokens":0,"context_window":0,"permissive":false,"auto_review":true,"access":"edits","transcript":[]}"""

    private fun vmWired(source: Fixture, dest: Fixture): GregalViewModel {
        val ctx = ApplicationProvider.getApplicationContext<Application>()
        val vm = GregalViewModel(ctx)
        val prefs = Prefs(ctx)
        prefs.save("127.0.0.1", dest.socket.localPort.toString(), "tok", "tester")
        GregalViewModel::class.java.getDeclaredField("prefsStore").apply { isAccessible = true }.set(vm, prefs)
        @Suppress("UNCHECKED_CAST")
        val conn = GregalViewModel::class.java.getDeclaredField("_conn").apply { isAccessible = true }.get(vm) as MutableStateFlow<Conn>
        conn.value = Conn("127.0.0.1", source.socket.localPort.toString(), "tok", "tester", local = true)
        return vm
    }

    private fun waitUntil(check: () -> Boolean) {
        repeat(200) {
            shadowOf(Looper.getMainLooper()).idle()
            if (check()) return
            Thread.sleep(25)
        }
        assertTrue("asynchronous transfer completed", check())
    }

    private fun destHandler(import: (transferId: String, bundle: String) -> Pair<Int, String>, verify: String): (String, String, String) -> Pair<Int, String> =
        { method, path, body ->
            when {
                method == "POST" && path == "/api/winds/transfer" -> {
                    val o = JSONObject(body)
                    import(o.getString("transferId"), o.getJSONObject("bundle").toString())
                }
                method == "GET" && path.startsWith("/api/winds/chat") -> 200 to verify
                method == "GET" && path == "/api/winds" ->
                    200 to """{"winds":[{"id":"copy-w-b1","name":"Brisa","color":"#A8CDE2","role":"recerca","style":"pso","persona":"Calma"}]}"""
                method == "GET" && path == "/api/winds/rotllanes" -> 200 to """{"rotllanes":[]}"""
                method == "GET" && path == "/api/state" -> 200 to stateJson
                else -> 404 to """{"error":"no fixture for $method $path"}"""
            }
        }

    @Test fun transferCopiesWindVerifiesAndOpensClone() {
        val seenImport = mutableListOf<String>()
        val source = Fixture { method, path, _ ->
            assertEquals("GET", method)
            assertTrue(path.startsWith("/api/winds/transfer"))
            assertTrue(path.contains("kind=wind") && path.contains("id=breeze"))
            assertTrue(path.contains("memory=false") && path.contains("knowledge=false") && path.contains("skills=false"))
            200 to bundleBreeze
        }
        val dest = Fixture(destHandler(
            import = { tid, bundle ->
                assertTrue(tid.matches(Regex("[A-Za-z0-9_-]{8,100}")))
                assertEquals(JSONObject(bundleBreeze).toString(), JSONObject(bundle).toString())
                seenImport += tid
                200 to importResult(tid, false)
            },
            verify = """{"wind":{"id":"copy-w-b1","name":"Brisa"},"messages":[{"role":"user","text":"Hola"}]}"""))
        source.use {
            dest.use {
                val vm = vmWired(source, dest)
                vm.transferConversationToServer("wind", "breeze", memory = false, knowledge = false, skills = false)
                waitUntil { vm.ventsOpen.value == "wind:copy-w-b1" }
                waitUntil { !vm.transferBusy.value }
                // L'original no rep cap escriptura: només l'export de lectura.
                assertTrue(source.requests.all { it.first == "GET" })
                assertEquals(listOf(1), listOf(seenImport.size))
                // El mode ha canviat al servidor desat per obrir el clon.
                @Suppress("UNCHECKED_CAST")
                val conn = GregalViewModel::class.java.getDeclaredField("_conn").apply { isAccessible = true }.get(vm) as MutableStateFlow<Conn>
                assertFalse(conn.value.local)
            }
        }
    }

    @Test fun transferRetryReusesIdentityInsteadOfCloningTwice() {
        val seenImport = mutableListOf<String>()
        val source = Fixture { _, path, _ ->
            assertTrue(path.startsWith("/api/winds/transfer"))
            200 to bundleBreeze
        }
        val dest = Fixture(destHandler(
            import = { tid, _ ->
                seenImport += tid
                200 to importResult(tid, seenImport.size > 1)
            },
            verify = """{"wind":{"id":"copy-w-b1"},"messages":[]}"""))
        source.use {
            dest.use {
                val vm = vmWired(source, dest)
                vm.transferConversationToServer("wind", "breeze", false, false, false)
                waitUntil { vm.ventsOpen.value == "wind:copy-w-b1" }
                waitUntil { !vm.transferBusy.value }
                // Torna al mòbil i reintenta el mateix traspàs: mateix transferId.
                @Suppress("UNCHECKED_CAST")
                val conn = GregalViewModel::class.java.getDeclaredField("_conn").apply { isAccessible = true }.get(vm) as MutableStateFlow<Conn>
                conn.value = Conn("127.0.0.1", source.socket.localPort.toString(), "tok", "tester", local = true)
                vm.transferConversationToServer("wind", "breeze", false, false, false)
                waitUntil { seenImport.size == 2 }
                waitUntil { !vm.transferBusy.value }
                assertEquals(seenImport[0], seenImport[1])
            }
        }
    }

    @Test fun transferConflictKeepsModeAndConversation() {
        val source = Fixture { _, _, _ -> 200 to bundleBreeze }
        val dest = Fixture(destHandler(
            import = { _, _ -> 409 to "transferId conflicts with another bundle" },
            verify = """{}"""))
        source.use {
            dest.use {
                val vm = vmWired(source, dest)
                vm.transferConversationToServer("wind", "breeze", false, false, false)
                waitUntil { !vm.transferBusy.value && vm.ventsOpen.value == null }
                @Suppress("UNCHECKED_CAST")
                val conn = GregalViewModel::class.java.getDeclaredField("_conn").apply { isAccessible = true }.get(vm) as MutableStateFlow<Conn>
                assertTrue(conn.value.local)
                assertNull(vm.ventsOpen.value)
            }
        }
    }

    @Test fun transferVerifyMismatchAbortsBeforeNavigating() {
        val source = Fixture { _, _, _ -> 200 to bundleBreeze }
        val dest = Fixture(destHandler(
            import = { tid, _ -> 200 to importResult(tid, false) },
            verify = """{"wind":{"id":"something-else"}}"""))
        source.use {
            dest.use {
                val vm = vmWired(source, dest)
                vm.transferConversationToServer("wind", "breeze", false, false, false)
                waitUntil { !vm.transferBusy.value && dest.requests.any { it.first == "GET" && it.second.startsWith("/api/winds/chat") } }
                shadowOf(Looper.getMainLooper()).idle()
                assertNull(vm.ventsOpen.value)
                @Suppress("UNCHECKED_CAST")
                val conn = GregalViewModel::class.java.getDeclaredField("_conn").apply { isAccessible = true }.get(vm) as MutableStateFlow<Conn>
                assertTrue(conn.value.local)
            }
        }
    }

    @Test fun retryResendsLastUserTextThroughSend() {
        val ctx = ApplicationProvider.getApplicationContext<Application>()
        val vm = GregalViewModel(ctx)
        @Suppress("UNCHECKED_CAST")
        val conn = GregalViewModel::class.java.getDeclaredField("_conn").apply { isAccessible = true }.get(vm) as MutableStateFlow<Conn>
        conn.value = Conn("127.0.0.1", "1", "", local = true)
        @Suppress("UNCHECKED_CAST")
        val chats = GregalViewModel::class.java.getDeclaredField("_windChat").apply { isAccessible = true }.get(vm) as MutableStateFlow<Map<String, List<GregalViewModel.PetMsg>>>
        chats.value = mapOf("w1" to listOf(GregalViewModel.PetMsg("user", "Hola", ts = 1_700_000_000_000L)))
        vm.retryLastWindTurn("w1")
        // El reenviament afegeix el mateix text d'usuari i, sense servidor,
        // la targeta d'error honesta (cap text nou inventat pel client).
        waitUntil { (chats.value["w1"]?.count { it.from == "user" && it.text == "Hola" } ?: 0) == 2 }
        waitUntil { !vm.windBusy.value.contains("w1") }
        assertTrue(chats.value["w1"]?.any { it.kind == "error" } == true)
    }

    @Test fun retryWithoutUserTextDoesNothing() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext())
        vm.retryLastWindTurn("missing")
        vm.retryLastRotllanaTurn("missing")
        assertTrue(vm.windBusy.value.isEmpty())
        assertTrue(vm.rotllanaBusy.value.isEmpty())
    }
}
