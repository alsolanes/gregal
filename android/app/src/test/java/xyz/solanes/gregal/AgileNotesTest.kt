package xyz.solanes.gregal

import android.app.Application
import android.graphics.Bitmap
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.core.app.ApplicationProvider
import kotlinx.coroutines.runBlocking
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.annotation.LooperMode

/** La desada automàtica de la nota àgil: una sola nota, en ordre, mai buida ni repetida. */
class NoteDraftSaverTest {
    private fun saver(calls: MutableList<NoteInput>, conflictAt: Int = -1) =
        NoteDraftSaver(blankDraft(NotesQuery(notebook = "Feina", project = "/p"))) { input ->
            calls += input
            if (calls.size - 1 == conflictAt) null
            else Note(id = "n1", title = input.title, body = input.body, updated = "v${calls.size}")
        }

    @Test fun buidaNoEsDesa() = runBlocking {
        val calls = mutableListOf<NoteInput>()
        assertNull(saver(calls).flush("", "  "))
        assertTrue(calls.isEmpty())
    }

    @Test fun elPrimerCaracterLaCreaIDespresLaReescriu() = runBlocking {
        val calls = mutableListOf<NoteInput>()
        val s = saver(calls)
        assertNotNull(s.flush("", "h"))
        assertNotNull(s.flush("", "hola"))
        // El mateix text no es torna a enviar.
        assertNull(s.flush("", "hola"))
        assertEquals(2, calls.size)
        assertEquals("", calls[0].id); assertEquals("", calls[0].base)
        assertEquals("Feina", calls[0].notebook); assertEquals("/p", calls[0].project)
        assertEquals("n1", calls[1].id); assertEquals("v1", calls[1].base)
        // El projecte només s'envia en crear-la.
        assertNull(calls[1].project)
        assertEquals("n1", s.id)
    }

    @Test fun ambConflicteSAtura() = runBlocking {
        val calls = mutableListOf<NoteInput>()
        val s = saver(calls, conflictAt = 1)
        s.flush("", "a"); s.flush("", "ab")
        assertTrue(s.conflict)
        assertNull(s.flush("", "abc"))
        assertEquals(2, calls.size)
    }

    @Test fun lEsborranyHeretaElFiltre() {
        val d = blankDraft(NotesQuery(tag = "casa", project = "-"))
        assertEquals(listOf("casa"), d.tags)
        assertNull(d.project)
        assertEquals("", d.body)
    }
}

/** Obrir Notes: una nota en blanc, el cursor al cos, i es desa sola en escriure. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], qualifiers = "w393dp-h851dp")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@LooperMode(LooperMode.Mode.PAUSED)
class AgileNoteUiTest {
    @get:Rule val ui = createComposeRule()

    @Test fun obreEnBlancAmbElCursorAlCosIDesaSola() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext<Application>())
        val saved = mutableListOf<NoteInput>()
        val others = NotesPage(listOf(Note("a", "Llista de la compra", excerpt = "llet, pa"), Note("b", "Idees", excerpt = "Un jardí vertical")),
            emptyList(), emptyList(), 2)
        ui.setContent {
            CompositionLocalProvider(LocalWindMotion provides false, LocalPal provides DarkPal) {
                MaterialTheme(colorScheme = DarkPal.scheme()) {
                    AgileNote(vm, "ca", blankDraft(NotesQuery()), others, {}, {}, {}, {},
                        save = { saved += it; Note("n1", it.title, it.body, updated = "v1") })
                }
            }
        }
        ui.waitForIdle()
        ui.onNodeWithTag("agile-body").assertIsFocused()
        ui.onNodeWithText("Escriu: es desa sol.").assertExists()
        ui.onNodeWithText("Llista de la compra").assertExists()
        // El cursor parpelleja (mai no està «quiet»): es pinta la finestra directament.
        ui.runOnUiThread {
            val activity = androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry.getInstance()
                .getActivitiesInStage(androidx.test.runner.lifecycle.Stage.RESUMED).single()
            val view = activity.window.decorView
            val shot = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888)
            view.draw(android.graphics.Canvas(shot))
            java.io.File("build/notes-ui/agile-dark.png").also { it.parentFile!!.mkdirs() }.outputStream().use {
                shot.compress(Bitmap.CompressFormat.PNG, 100, it)
            }
        }
        ui.onNodeWithTag("agile-body").performTextInput("Trucar a la Marta")
        ui.mainClock.advanceTimeBy(NOTE_AUTOSAVE_MS + 200)
        ui.waitUntil(3_000) { saved.isNotEmpty() }
        assertEquals("Trucar a la Marta", saved.single().body)
        assertEquals("", saved.single().id)
    }
}
