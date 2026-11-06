package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class NotesTest {
    @Test fun parsesListAndDetail() {
        val page = NotesJson.page("""{"notes":[{"id":"aa","title":"Compra","notebook":"Casa","tags":["mercat"],"hashtags":["Mercat","urgent"],"pinned":true,"edited_by":"Xaloc","tasks_open":1,"tasks_total":3,"updated":"2026-11-06T10:00:00Z"}],"tags":[{"name":"mercat","count":2}],"notebooks":[{"name":"Casa","count":1}],"total":4}""")
        assertEquals(4, page.total)
        val n = page.notes.single()
        assertEquals(listOf("mercat", "urgent"), n.allTags)
        assertTrue(n.pinned)
        assertEquals("Xaloc", n.editedBy)
        assertEquals(NoteCount("Casa", 1), page.notebooks.single())
        val d = NotesJson.detail("""{"note":{"id":"bb","title":"T","body":"x"},"backlinks":[{"id":"aa","title":"Compra"}]}""")
        assertEquals("x", d.note.body)
        assertEquals(listOf("aa" to "Compra"), d.backlinks)
    }

    @Test fun linksBecomeNoteLinksOutsideCode() {
        val out = linkifyNotes("Mira [[Viatge a Menorca]] i [[llista|la llista]] `[[codi]]`\n```\n[[bloc]]\n```")
        assertTrue(out, out.contains("[Viatge a Menorca](gregal-note:Viatge+a+Menorca)"))
        assertTrue(out, out.contains("[la llista](gregal-note:llista)"))
        assertTrue(out, out.contains("`[[codi]]`"))
        assertTrue(out, out.contains("\n[[bloc]]\n"))
        assertEquals("Viatge a Menorca", noteLinkTitle("gregal-note:Viatge+a+Menorca"))
        assertNull(noteLinkTitle("https://example.com"))
        // Md.kt en fa un enllaç de debò.
        val link = parseInline(out.lineSequence().first()).filterIsInstance<Seg.Link>().first()
        assertEquals("gregal-note:Viatge+a+Menorca", link.url)
    }

    @Test fun toggleTaskSkipsCode() {
        val body = "- [ ] pa\n```\n- [ ] no\n```\n- [x] llet\n* [ ] ous"
        assertEquals("- [x] pa\n```\n- [ ] no\n```\n- [x] llet\n* [ ] ous", toggleNoteTask(body, 0))
        assertEquals("- [ ] pa\n```\n- [ ] no\n```\n- [ ] llet\n* [ ] ous", toggleNoteTask(body, 1))
        assertEquals("- [ ] pa\n```\n- [ ] no\n```\n- [x] llet\n* [x] ous", toggleNoteTask(body, 2))
    }

    @Test fun blocksSplitTasksFromText() {
        val b = noteBlocks("# Compra\nper dissabte\n- [ ] pa\n  - [x] **llet**\n\nfi")
        assertEquals(4, b.size)
        assertEquals(NoteBlock.Text("# Compra\nper dissabte"), b[0])
        assertEquals(NoteBlock.Task(0, false, "pa", 0), b[1])
        assertEquals(NoteBlock.Task(1, true, "**llet**", 1), b[2])
        assertEquals(NoteBlock.Text("fi"), b[3])
    }

    @Test fun tagsAreCleaned() {
        assertEquals(listOf("música", "Pendents", "idees-noves"), parseNoteTags("música, #Pendents ; musica\nidees noves"))
        assertEquals(emptyList<String>(), parseNoteTags(" , "))
        assertEquals("cafe", foldNote(" Cafè "))
    }

    @Test fun agendaParsesAndGroups() {
        val page = NotesJson.page("""{"notes":[],"total":0,"agenda":[{"note_id":"a","note_title":"Reunió","index":1,"text":"envia","due":"2026-11-12","time":"09:30"}]}""")
        assertEquals(AgendaItem("a", "Reunió", 1, "envia", "2026-11-12", "09:30"), page.agenda.single())
        val today = java.time.LocalDate.of(2026, 11, 6)
        val items = listOf("2026-11-01", "2026-11-06", "2026-11-13", "2026-11-14").map { AgendaItem("n", "t", 0, it, it, "") }
        val g = agendaGroups(items, today)
        assertEquals(listOf(1, 1, 1, 1), listOf("overdue", "today", "week", "later").map { g[it].orEmpty().size })
    }

    @Test fun meetingTemplateHasDatedTask() {
        val m = meetingTemplate("ca", java.time.LocalDate.of(2026, 11, 6))
        assertEquals("Reunions", m.notebook)
        assertTrue(m.body, m.body.contains("**Data:** 2026-11-06"))
        assertTrue(m.body, m.body.contains("- [ ]  @2026-11-13"))
        assertTrue(meetingTemplate("en", java.time.LocalDate.of(2026, 11, 6)).body.contains("Action items"))
    }

    @Test fun captureTextFromShareAndShortcut() {
        assertEquals("", noteCaptureText(QUICK_NOTE_ACTION, null, null))
        assertEquals("comprar pa", noteCaptureText(android.content.Intent.ACTION_SEND, null, " comprar pa "))
        assertEquals("# Article\n\nhttps://x.cat/a", noteCaptureText(android.content.Intent.ACTION_SEND, "Article", "https://x.cat/a"))
        assertEquals("Títol", noteCaptureText(android.content.Intent.ACTION_SEND, "Títol", ""))
        assertNull(noteCaptureText(android.content.Intent.ACTION_SEND, "", ""))
        assertNull(noteCaptureText(android.content.Intent.ACTION_VIEW, "a", "b"))
    }

    @Test fun reminderFieldsRoundTrip() {
        val page = NotesJson.page("""{"notes":[{"id":"a","title":"T","no_remind":true}],"agenda":[{"note_id":"a","due":"2026-11-12","remind_at":"2026-11-12T09:00:00+01:00"}]}""")
        assertTrue(page.notes.single().noRemind)
        assertEquals("2026-11-12T09:00:00+01:00", page.agenda.single().remindAt)
        val json = NotesJson.input(NoteInput("a", "T", "", "", emptyList(), false, false, "", noRemind = true))
        assertTrue(json.getBoolean("no_remind"))
    }

    @Test fun stringsExistInBothLanguages() {
        for (k in listOf("nav.notes", "notes.help", "notes.private", "notes.locked", "notes.conflict", "notes.askPrompt")) {
            assertTrue(k, STR_APP_CA.containsKey(k) && STR_APP_EN.containsKey(k))
        }
        assertEquals("Llegeix la meva nota «Compra» i ", tr("ca", "notes.askPrompt", "Compra"))
    }
}
