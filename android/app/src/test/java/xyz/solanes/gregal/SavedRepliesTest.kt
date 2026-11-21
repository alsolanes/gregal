package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import android.os.Looper
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.unit.dp
import androidx.test.core.app.ApplicationProvider
import java.net.ServerSocket
import java.util.Collections
import kotlin.concurrent.thread
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** Desat: una resposta es desa com a nota amb tipus, es troba a Desat i s'hi veu com al xat. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class SavedRepliesTest {
    @get:Rule val ui = createComposeRule()

    /** Servidor de proves: apunta cada petició («POST /api/notes …» + cos) i respon el que digui `reply`. */
    private class FakeServer(val code: Int = 200, val reply: (String) -> String) : java.io.Closeable {
        val requests: MutableList<Pair<String, String>> = Collections.synchronizedList(mutableListOf())
        val socket = ServerSocket(0)
        val worker = thread(isDaemon = true) {
            try {
                while (!socket.isClosed) socket.accept().use { s ->
                    // Content-Length compta bytes: el cos (amb accents) es llegeix en bytes, no en caràcters.
                    val input = s.getInputStream().buffered()
                    fun line(): String {
                        val b = java.io.ByteArrayOutputStream()
                        while (true) { val c = input.read(); if (c < 0 || c == 10) break; if (c != 13) b.write(c) }
                        return b.toString("UTF-8")
                    }
                    val request = line()
                    var length = 0
                    while (true) {
                        val h = line()
                        if (h.isEmpty()) break
                        if (h.startsWith("Content-Length:", true)) length = h.substringAfter(':').trim().toInt()
                    }
                    requests += request to String(input.readNBytes(length), Charsets.UTF_8)
                    val bytes = reply(request).toByteArray(Charsets.UTF_8)
                    s.getOutputStream().apply {
                        write("HTTP/1.1 $code Response\r\nContent-Type: application/json\r\nContent-Length: ${bytes.size}\r\nConnection: close\r\n\r\n".toByteArray())
                        write(bytes); flush()
                    }
                }
            } catch (_: java.net.SocketException) { }
        }
        fun conn() = Conn("127.0.0.1", socket.localPort.toString(), "fixture", "saved-test")
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

    private val recipe = "Aquí la tens:\n\n```gregal-recipe\n{\"title\":\"Crema de carbassa\",\"servings\":2," +
        "\"ingredients\":[{\"name\":\"carbassa\",\"qty\":\"500 g\"}],\"steps\":[{\"text\":\"Bull-ho\",\"minutes\":20}]}\n```\n"
    private val recipeNote = JSONObject().put("id", "r1").put("title", "Crema de carbassa").put("kind", "recipe")
        .put("body", recipe).put("updated", "2026-11-21T10:00:00Z")

    @Test fun desarUnaRespostaLaEnviaAmbTipusAuto() {
        FakeServer { recipeNote.toString() }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(server.conn())
            val got = kotlinx.coroutines.runBlocking { vm.saveReply(recipe) }
            val (request, body) = server.requests.single()
            assertTrue(request, request.startsWith("POST /api/notes "))
            val sent = JSONObject(body)
            assertEquals("save", sent.getString("action"))
            assertEquals(recipe, sent.getString("body"))
            assertEquals("auto", sent.getString("kind"))
            // Nota nova: sense id ni títol, que el servidor treu de la targeta.
            assertFalse(sent.has("id"))
            assertFalse(sent.has("title"))
            assertEquals("r1", got.id)
            assertEquals("recipe", got.kind)
        }
    }

    @Test fun elFullDeLaRespostaDesaIObreElDesat() {
        FakeServer { r -> if (r.startsWith("POST")) recipeNote.toString() else JSONObject().put("note", recipeNote).toString() }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(server.conn())
            ui.setContent {
                CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                    MaterialTheme(colorScheme = DarkPal.scheme()) {
                        ShareSheetBody(vm, "ca", ShareTarget(recipe, "Xaloc"), onDismiss = {})
                    }
                }
            }
            ui.onNodeWithTag("share-save").assertExists().performClick()
            ui.waitUntil(3_000) { ui.onAllNodesWithText("Desat ✓").fetchSemanticsNodes().isNotEmpty() }
            ui.onNodeWithText("Obre").performClick()
            waitUntil { vm.openNote.value != null }
            assertEquals(Screen.Notes, vm.screen.value)
            assertEquals("r1", vm.openNote.value!!.note.id)
        }
    }

    @Test fun siDesarFallaHoDiuDinsDelFull() {
        FakeServer(code = 500) { """{"error":"disc ple"}""" }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(server.conn())
            ui.setContent {
                CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                    MaterialTheme(colorScheme = DarkPal.scheme()) {
                        ShareSheetBody(vm, "ca", ShareTarget(recipe, "Xaloc"), onDismiss = {})
                    }
                }
            }
            ui.onNodeWithTag("share-save").performClick()
            // El full tapa els avisos de sota: l'error ha de sortir al mateix full, i «Desa» s'hi queda per tornar-hi.
            ui.waitUntil(3_000) { ui.onAllNodesWithTag("share-status").fetchSemanticsNodes().isNotEmpty() }
            ui.onNodeWithTag("share-status").assertTextContains("disc ple", substring = true)
            ui.onNodeWithTag("share-save").assertExists()
        }
    }

    @Test fun elXipDeLaPropostaDesaAmbTitolITipusSenseLaMarca() {
        val reply = recipe + "\n```gregal-keep\n{\"title\":\"Crema per a dos\",\"kind\":\"recipe\"}\n```\n"
        FakeServer { recipeNote.toString() }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(server.conn())
            ui.setContent {
                CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                    MaterialTheme(colorScheme = DarkPal.scheme()) {
                        Column {
                            WindBubble(GregalViewModel.PetMsg("pet", reply, ts = 1, id = "m-1"), Sea, lang = "ca", vm = vm)
                            ShareSheetBody(vm, "ca", ShareTarget(reply, "Xaloc"), onDismiss = {})
                        }
                    }
                }
            }
            assertTrue(ui.onAllNodesWithText("gregal-keep", substring = true).fetchSemanticsNodes().isEmpty())
            ui.onNodeWithText("Desar «Crema per a dos»?").performClick()
            ui.waitUntil(3_000) { ui.onAllNodesWithText("Desat ✓").fetchSemanticsNodes().size == 2 }
            val (request, body) = server.requests.single()
            assertTrue(request, request.startsWith("POST /api/notes "))
            val sent = JSONObject(body)
            assertEquals(recipe, sent.getString("body"))
            assertEquals("Crema per a dos", sent.getString("title"))
            assertEquals("recipe", sent.getString("kind"))
            // El full de compartir sap que ja està desada: «Desa» no hi torna a sortir.
            ui.onNodeWithTag("share-saved").assertExists()
            ui.onNodeWithTag("share-save").assertDoesNotExist()
        }
    }

    @Test fun elXipDiuDesarEnCatalaIEnAngles() {
        assertEquals("Save «Soup»?", tr("en", "saved.keep.ask", "Soup"))
        assertEquals("Desar «Sopa»?", tr("ca", "saved.keep.ask", "Sopa"))
    }

    private val page = NotesPage(
        listOf(
            Note("a", "Truita de patates", kind = "recipe", updated = "2026-10-20T09:00:00Z"),
            Note("b", "Ruta a Montserrat", kind = "route", pinned = true, updated = "2026-10-12T09:00:00Z"),
            Note("c", "Pressupost del viatge", kind = "facts", updated = "2026-10-02T09:00:00Z"),
        ),
        emptyList(), emptyList(), 3,
        kinds = listOf(NoteCount("recipe", 1), NoteCount("route", 1), NoteCount("facts", 1)),
    )

    private fun renderSaved(pal: GregalPalette, kinds: MutableList<String> = mutableListOf(), opened: MutableList<String> = mutableListOf()) {
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides pal) {
                MaterialTheme(colorScheme = pal.scheme()) {
                    Column(Modifier.fillMaxSize().background(pal.surface).padding(16.dp)) {
                        SavedNotes("ca", page, query = "", kind = "", onQuery = {}, onKind = { kinds += it }, onOpen = { opened += it.id })
                    }
                }
            }
        }
        ui.waitForIdle()
    }

    private fun shot(name: String) {
        ui.runOnUiThread {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).single()
            val view = activity.window.decorView
            val bmp = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
            view.draw(android.graphics.Canvas(bmp))
            java.io.File("build/desat-ui/$name.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
                bmp.compress(Bitmap.CompressFormat.PNG, 100, it)
            }
        }
    }

    @Test fun desatMostraElsDesatsAmbCercaIXipsClar() {
        val kinds = mutableListOf<String>()
        val opened = mutableListOf<String>()
        renderSaved(LightPal, kinds, opened)
        ui.onNodeWithTag("saved-search").assertExists()
        ui.onNodeWithText("Receptes · 1").assertExists()
        ui.onNodeWithText("Rutes · 1").assertExists()
        ui.onNodeWithText("Xifres · 1").assertExists()
        // La fixada, primer, encara que sigui la més vella.
        val rows = ui.onAllNodesWithTag("saved-note").fetchSemanticsNodes()
        assertEquals(3, rows.size)
        val pinnedTop = ui.onNodeWithText("📌 Ruta a Montserrat").getBoundsInRoot().top
        val recipeTop = ui.onNodeWithText("Truita de patates").getBoundsInRoot().top
        assertTrue(pinnedTop < recipeTop)
        ui.onNodeWithText("Receptes · 20/10/2026").assertExists()
        shot("desat-light")
        ui.onNodeWithText("Receptes · 1").performClick()
        assertEquals(listOf("recipe"), kinds)
        ui.onNodeWithText("Truita de patates").performClick()
        assertEquals(listOf("a"), opened)
    }

    @Test fun desatMostraElsDesatsFosc() {
        renderSaved(DarkPal)
        ui.onNodeWithText("Rutes · 1").assertExists()
        shot("desat-dark")
    }

    @Test fun lesEtiquetesDelsTipusEnAnglesIDefecte() {
        assertEquals("Recipes", savedKindLabel("en", "recipe"))
        assertEquals("Comparacions", savedKindLabel("ca", "compare"))
        assertEquals("Webs", savedKindLabel("en", "web"))
        assertTrue(appStringsOk())
    }

    @Test fun unaNotaDesadaPintaLaReceptaComAlXat() {
        FakeServer { r ->
            if (r.startsWith("GET /api/notes?id=")) JSONObject().put("note", recipeNote).toString() else "{}"
        }.use { server ->
            val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
            vm.setConn(server.conn())
            vm.openNote("r1")
            waitUntil { vm.openNote.value != null }
            ui.setContent {
                CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides LightPal) {
                    MaterialTheme(colorScheme = LightPal.scheme()) { NotesScreen(vm) {} }
                }
            }
            ui.waitForIdle()
            ui.onNodeWithTag("gregal-recipe").assertExists()
            ui.onNodeWithText("Bull-ho", substring = true).assertExists()
            assertTrue(ui.onAllNodesWithText("\"ingredients\"", substring = true).fetchSemanticsNodes().isEmpty())
        }
    }
}
