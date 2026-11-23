package xyz.solanes.gregal

import android.app.Application
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import androidx.lifecycle.viewModelScope
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** El document que escriu un vent (066): el flux en dona la targeta i la targeta l'obre. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class WindDocCardTest {
    @get:Rule val ui = createComposeRule()
    private val api = GregalApi("http://127.0.0.1:9", "")
    private val carta = DocRef("w1", "Carta de benvinguda", "doc", "mine")

    @Test fun docChangedOfAWindIsParsedWithItsScope() {
        val e = api.parseWindStreamEvent("doc_changed", """{"id":"w1","title":"Carta de benvinguda","scope":"mine"}""")
        assertEquals(GregalApi.WindStreamEvent.DocChanged(carta), e)
        assertNull(api.parseWindStreamEvent("doc_changed", """{"title":"sense id"}"""))
    }

    @Test fun theWindChatShowsOneCardPerDocument() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.DocChanged(carta), "mestral")
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.DocChanged(carta.copy(title = "Carta")), "mestral")
        val msgs = vm.windChat.value["mestral"].orEmpty()
        assertEquals(listOf("doc"), msgs.map { it.kind })
        assertEquals(carta, msgs[0].doc)
        assertEquals("Carta de benvinguda", msgs[0].text)
    }

    @Test fun theCardGoesBeforeTheLiveBubble() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        chats(vm).value = mapOf("mestral" to listOf(GregalViewModel.PetMsg("pet", "Escric", streaming = true)))
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.DocChanged(carta), "mestral")
        assertEquals(listOf("doc", ""), vm.windChat.value["mestral"].orEmpty().map { it.kind })
    }

    @Test fun aContinuationIsMergedIntoTheAnswerNotIntoTheCard() {
        val stream = "event: doc_changed\ndata: {\"id\":\"w1\",\"title\":\"Carta de benvinguda\",\"scope\":\"mine\"}\n\n" +
            "event: done\ndata: {\"reply\":\" i acabada.\"}\n\n"
        val server = java.net.ServerSocket(0)
        val worker = kotlin.concurrent.thread(isDaemon = true) {
            try {
                while (!server.isClosed) server.accept().use { c ->
                    val r = c.getInputStream().bufferedReader()
                    val path = r.readLine().split(' ')[1].substringBefore('?')
                    var length = 0
                    while (true) {
                        val h = r.readLine() ?: break
                        if (h.isEmpty()) break
                        if (h.startsWith("Content-Length:", true)) length = h.substringAfter(':').trim().toInt()
                    }
                    repeat(length) { r.read() }
                    val sse = path.endsWith("/chat/stream")
                    val body = (if (sse) stream else "{}").toByteArray()
                    c.getOutputStream().apply {
                        write(("HTTP/1.1 200 OK\r\nContent-Type: ${if (sse) "text/event-stream" else "application/json"}\r\n" +
                            "Content-Length: ${body.size}\r\nConnection: close\r\n\r\n").toByteArray())
                        write(body); flush()
                    }
                }
            } catch (e: java.net.SocketException) { if (!server.isClosed) throw e }
        }
        try {
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(Conn("127.0.0.1", server.localPort.toString(), "doc-card"))
            chats(vm).value = mapOf("mestral" to listOf(GregalViewModel.PetMsg("pet", "Carta començada", parcial = true)))
            val scope = vm.viewModelScope.coroutineContext[kotlinx.coroutines.Job]!!
            val before = scope.children.toSet()
            vm.sendWindContinue("mestral")
            val job = scope.children.single { it !in before }
            val deadline = System.nanoTime() + java.util.concurrent.TimeUnit.SECONDS.toNanos(5)
            while (!job.isCompleted && System.nanoTime() < deadline) {
                org.robolectric.Shadows.shadowOf(android.os.Looper.getMainLooper()).idle()
                Thread.sleep(5)
            }
            val msgs = vm.windChat.value["mestral"].orEmpty()
            assertEquals(listOf("Carta començada i acabada.", "Carta de benvinguda"), msgs.map { it.text })
            assertEquals(listOf("", "doc"), msgs.map { it.kind })
            assertFalse(msgs[0].parcial)
        } finally { server.close(); worker.join(1000) }
    }

    private fun chats(vm: GregalViewModel) = GregalViewModel::class.java.getDeclaredField("_windChat")
        .apply { isAccessible = true }.get(vm) as kotlinx.coroutines.flow.MutableStateFlow<Map<String, List<GregalViewModel.PetMsg>>>

    @Test fun tappingTheCardOpensTheDocumentScreenWithMyScope() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        val wind = Wind("mestral", "Mestral", "#A8CDE2", "recerca")
        vm.handleWindAskEvent(GregalApi.WindStreamEvent.DocChanged(carta), wind.id)
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal, LocalWindMotion provides false) {
                MaterialTheme(colorScheme = LightPal.scheme()) { WindConversation(vm, "ca", wind, onBack = {}) }
            }
        }
        ui.onNodeWithTag("doc-card").performClick()
        assertEquals(carta, vm.docOpen.value)
        assertEquals(Screen.Docs, vm.screen.value)
    }
}
