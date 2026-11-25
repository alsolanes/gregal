package xyz.solanes.gregal

import android.app.Application
import android.os.Looper
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import java.net.Socket
import java.time.Duration
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.TimeUnit
import kotlin.concurrent.thread
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.launch
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.LooperMode

/** Seguir un document des de la pantalla de Gregal Docs, contra un servidor de veritat (un socket). */
@RunWith(RobolectricTestRunner::class)
@LooperMode(LooperMode.Mode.PAUSED)
class DocFollowTest {
    private val sheetJson = """{"id":"f1","title":"Pressupost","kind":"sheet","version":3,
        "blocks":[{"id":"h1","name":"Full1","cells":{"A1":"1"},"version":2},{"id":"h2","name":"Full2","cells":{},"version":3}]}"""

    @Test fun aServerRestartingWith503IsRetriedInsteadOfGivingUp() {
        DocServer { req, n -> if (req.startsWith("GET /api/docs/f1/stream") && n == 1) "503" to "bad gateway" else null }.use { s ->
            val vm = vm(s)
            val job = CoroutineScope(Dispatchers.Default).launch { vm.followDoc(DocRef("f1", "Pressupost", "sheet")) }
            try {
                waitFor { s.count("GET /api/docs/f1/stream") >= 2 }
                assertEquals(2, s.count("GET /api/docs/f1/stream"))
                assertEquals("f1", vm.doc.value?.id)
            } finally { job.cancel() }
        }
    }

    @Test fun leavingTheScreenCutsTheStreamAtOnce() {
        DocServer { _, _ -> null }.use { s ->
            val vm = vm(s)
            val job = CoroutineScope(Dispatchers.Default).launch { vm.followDoc(DocRef("f1", "Pressupost", "sheet")) }
            waitFor { s.count("GET /api/docs/f1/stream") == 1 && vm.doc.value != null }
            job.cancel()
            // El servidor no envia res (el ping és cada 15 s): sense tallar la crida, la feina no acabaria.
            waitFor(3) { job.isCompleted }
            assertTrue(job.isCompleted)
            waitFor(3) { s.closedStreams > 0 }
            assertEquals(1, s.closedStreams)
        }
    }

    @Test fun aTextDocumentIsOnlyReadTheWebPageFollowsIt() {
        DocServer { req, _ -> if (req.startsWith("GET /api/docs/d1 ")) "200" to """{"id":"d1","title":"Carta","kind":"doc","version":1,"blocks":[]}""" else null }.use { s ->
            val vm = vm(s)
            val job = CoroutineScope(Dispatchers.Default).launch { vm.followDoc(DocRef("d1", "Carta")) }
            waitFor { job.isCompleted }
            assertTrue(job.isCompleted)
            assertEquals("Carta", vm.doc.value?.title)
            assertEquals(0, s.count("GET /api/docs/d1/stream"))
        }
    }

    // AndroidView dona WRAP_CONTENT a una vista sense mides, i un WebView així mesura la pàgina
    // pel contingut: html,body{height:100%} de /app/doc feia 0 px i el document es veia en negre.
    @Test fun theTextEditorWebViewFillsItsBoxSoThePageHasAHeight() {
        val w = docWebView(ApplicationProvider.getApplicationContext(), "http://10.0.2.2:8097/app/doc/d1?session=mobil", { "k" }, { "en" }) {}
        assertEquals(android.view.ViewGroup.LayoutParams.MATCH_PARENT, w.layoutParams.height)
        assertEquals(android.view.ViewGroup.LayoutParams.MATCH_PARENT, w.layoutParams.width)
        assertTrue(w.settings.javaScriptEnabled)
    }

    @Test fun aDocumentWrittenWhileTheListIsOpenAppearsInIt() {
        DocServer { req, n ->
            if (req.startsWith("GET /api/docs?scope=mine")) "200" to (if (n == 1) """{"items":[]}""" else """{"items":[{"id":"n1","title":"Nou"}]}""") else null
        }.use { s ->
            val vm = vm(s)
            vm.loadDocs(false)
            waitFor { s.count("GET /api/docs?scope=mine") == 1 }
            repeat(20) { idle(); Thread.sleep(5) }
            vm.openDocsHome()
            assertEquals(Screen.Docs, vm.screen.value)
            vm.handleWindAskEvent(GregalApi.WindStreamEvent.DocChanged(DocRef("n1", "Nou", scope = "mine")), "mestral")
            waitFor { vm.myDocs.value.isNotEmpty() }
            assertEquals(listOf("n1"), vm.myDocs.value.map { it.id })
        }
    }

    @Test fun tappingAnotherCellSavesTheTypedOneFirst() {
        DocServer { req, _ ->
            if (req.startsWith("PUT /api/docs/f1/blocks/h1")) "200" to """{"version":4,"block":{"id":"h1","name":"Full1","cells":{"A1":"1","B1":"7"},"version":4}}""" else null
        }.use { s ->
            val vm = vm(s)
            doc(vm).value = DocsJson.doc(sheetJson).openCell("h1", "B1").typed("7")
            vm.editCell("h1", "C1")
            waitFor { vm.doc.value?.edit?.cell == "C1" }
            assertEquals(1, s.count("PUT /api/docs/f1/blocks/h1"))
            assertEquals("7", vm.doc.value?.blocks?.get(0)?.cells?.get("B1"))
        }
    }

    @Test fun switchingTabsClosesAnUntouchedEditorAndTappingAnotherSlideOpensIt() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        doc(vm).value = DocsJson.doc(sheetJson).openCell("h1", "B1")
        vm.showSheet("h2")
        assertEquals("h2", vm.doc.value?.shownSheet()?.id)
        assertNull(vm.doc.value?.edit)
        doc(vm).value = DocsJson.doc("""{"id":"k1","kind":"deck","version":2,"blocks":[
            {"id":"s1","slide":{"layout":"title","title":"A"},"version":1},{"id":"s2","slide":{"layout":"title","title":"B"},"version":2}]}""").open("s1")
        vm.editDocBlock("s2")
        assertEquals("s2", vm.doc.value?.edit?.id)
    }

    private fun vm(s: DocServer) = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        .apply { setConn(Conn("127.0.0.1", s.port.toString(), "doc-follow")) }

    private fun doc(vm: GregalViewModel) = GregalViewModel::class.java.getDeclaredField("_doc")
        .apply { isAccessible = true }.get(vm) as MutableStateFlow<LiveDoc?>

    private fun idle() = shadowOf(Looper.getMainLooper()).idleFor(Duration.ofMillis(20))

    private fun waitFor(seconds: Long = 8, ok: () -> Boolean) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(seconds)
        while (!ok() && System.nanoTime() < deadline) { idle(); Thread.sleep(10) }
    }

    /**
     * Respon el que diu [answer] (codi i cos) o, si torna null: el full f1 en JSON, un stream
     * obert sense events per a /stream, i {} per a la resta. [answer] rep la línia de la petició i quantes n'hi ha hagut d'iguals.
     */
    private inner class DocServer(private val answer: (String, Int) -> Pair<String, String>?) : AutoCloseable {
        private val socket = ServerSocket(0)
        val port: Int get() = socket.localPort
        private val lines = CopyOnWriteArrayList<String>()
        @Volatile var closedStreams = 0
        private val acceptor = thread(isDaemon = true) {
            while (!socket.isClosed) {
                val client = try { socket.accept() } catch (_: Exception) { break }
                thread(isDaemon = true) { client.use { serve(it) } }
            }
        }

        fun count(prefix: String) = lines.count { it.startsWith(prefix) }

        private fun serve(c: Socket) {
            val r = c.getInputStream().bufferedReader()
            val line = r.readLine() ?: return
            var length = 0
            while (true) {
                val h = r.readLine() ?: break
                if (h.isEmpty()) break
                if (h.startsWith("Content-Length:", true)) length = h.substringAfter(':').trim().toInt()
            }
            repeat(length) { r.read() }
            lines += line
            val key = line.substringBeforeLast(' ')
            val out = c.getOutputStream()
            val given = answer(line, lines.count { it.substringBeforeLast(' ') == key })
            if (given == null && line.startsWith("GET /api/docs/f1/stream")) {
                out.write("HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\n\r\n".toByteArray()); out.flush()
                // Res a dir fins que el client talla.
                try { while (r.read() >= 0) Unit } catch (_: Exception) {}
                closedStreams++
                return
            }
            val (code, body) = given ?: ("200" to if (line.startsWith("GET /api/docs/f1 ")) sheetJson else "{}")
            val bytes = body.toByteArray()
            out.write(("HTTP/1.1 $code X\r\nContent-Type: application/json\r\nContent-Length: ${bytes.size}\r\nConnection: close\r\n\r\n").toByteArray())
            out.write(bytes); out.flush()
        }

        override fun close() { socket.close(); acceptor.join(1000) }
    }
}
