package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assert.assertNull
import org.junit.Test

// Documents vius (Docs.kt): l'event doc_changed, la llista del projecte i
// els events `block` aplicats a la llista de blocs. JVM pura.
class LiveDocTest {

    @Test
    fun docChangedGivesTheDocument() {
        assertEquals(DocRef("a1b2c3d4e5f6", "Pla de llançament"),
            DocsJson.changed("""{"id":"a1b2c3d4e5f6","title":"Pla de llançament"}"""))
    }

    @Test
    fun docChangedWithoutIdIsIgnored() {
        assertNull(DocsJson.changed("""{"title":"x"}"""))
        assertNull(DocsJson.changed("not json"))
    }

    @Test
    fun listReadsTheItems() {
        val raw = """{"items":[{"id":"d1","title":"Informe","kind":"doc","version":4,"updated":"2026-11-20T10:00:00Z"},
            {"id":"d2","title":"Notes","kind":"doc","version":1,"updated":"2026-11-19T10:00:00Z"}]}"""
        assertEquals(listOf(DocRef("d1", "Informe"), DocRef("d2", "Notes")), DocsJson.list(raw))
        assertEquals(emptyList<DocRef>(), DocsJson.list("""{"items":[]}"""))
    }

    private val doc = DocsJson.doc("""{"id":"d1","title":"Informe","kind":"doc","version":3,
        "blocks":[{"id":"b1","md":"# Títol","version":1,"author":"agent"},{"id":"b2","md":"Primer paràgraf","version":2,"author":"agent"}]}""")

    private fun ev(raw: String) = DocsJson.change(raw)!!

    private fun mds(d: LiveDoc) = d.blocks.map { it.id to it.md }

    @Test
    fun eventsAreAppliedInPlace() {
        val out = listOf(
            ev("""{"op":"insert","block":{"id":"b0","md":"Entrada"},"version":4,"author":"agent"}"""),
            ev("""{"op":"insert","block":{"id":"b3","md":"Segon"},"after":"b2","version":5,"author":"agent"}"""),
            ev("""{"op":"insert","block":{"id":"b4","md":"Tercer"},"after":"b3","version":5,"author":"agent"}"""),
            ev("""{"op":"replace","block":{"id":"b2","md":"Primer, revisat"},"version":6,"author":"user"}"""),
            ev("""{"op":"delete","block":{"id":"b1"},"version":7,"author":"agent"}"""),
        ).fold(doc) { d, c -> d.apply(c) }
        assertEquals(listOf("b0" to "Entrada", "b2" to "Primer, revisat", "b3" to "Segon", "b4" to "Tercer"), mds(out))
        assertEquals(7, out.version)
    }

    @Test
    fun eventsTheDocumentAlreadyHadAreDropped() {
        val out = doc.apply(ev("""{"op":"insert","block":{"id":"b2","md":"Primer paràgraf"},"after":"b1","version":3,"author":"agent"}"""))
        assertEquals(mds(doc), mds(out))
        assertEquals(3, out.version)
    }

    @Test
    fun unknownBlocks() {
        val out = doc
            .apply(ev("""{"op":"replace","block":{"id":"zz","md":"res"},"version":4,"author":"agent"}"""))
            .apply(ev("""{"op":"delete","block":{"id":"zz"},"version":5,"author":"agent"}"""))
        assertEquals(mds(doc), mds(out))
        assertEquals(5, out.version)
        // Inserit després d'un bloc que no hi és: al final, com a la web.
        val end = doc.apply(ev("""{"op":"insert","block":{"id":"b9","md":"Final"},"after":"zz","version":6,"author":"agent"}"""))
        assertEquals(listOf("b1", "b2", "b9"), end.blocks.map { it.id })
    }

    @Test
    fun exportLinkIsDownloadedLikeOtherDocuments() {
        val url = docExportPath(DocRef("d1", "Informe final"))
        assertTrue(isServerDocLink(url))
        assertEquals("Informe final.docx", serverDocName(url))
        assertTrue(isServerDocLink("/api/docs/d1/export?format=docx"))
        assertFalse(isServerDocLink("/api/docs/d1/stream"))
    }

    // --- edició (063): les regles de doc-view.js ---

    private fun req(m: String, p: String, b: String? = null) = DocRequest(m, p, b)

    @Test
    fun eachActionBuildsItsRequest() {
        // b2 és a la versió 2 i el document a la 3: la base que s'envia és la del bloc.
        assertEquals(req("PUT", "/api/docs/d1/blocks/b2", """{"md":"Primer, revisat","base_version":2}"""),
            doc.open("b2").typed("Primer, revisat\n").saveRequest())
        assertEquals(req("DELETE", "/api/docs/d1/blocks/b1?base_version=1"), doc.open("b1").typed("  ").saveRequest())
        assertEquals(req("POST", "/api/docs/d1/blocks", """{"after":"b2","md":"Nou"}"""), doc.openNew("b2").typed("Nou").saveRequest())
    }

    @Test
    fun savingWithoutChangesSendsNothing() {
        assertNull(doc.open("b2").saveRequest())
        assertNull(doc.open("b2").typed("Primer paràgraf\n").saveRequest())
        assertNull(doc.openNew("b2").typed(" ").saveRequest())
        val s = doc.open("b2").skipped()
        assertNull(s.doc.edit)
        assertEquals("b2", s.at)
        assertNull(doc.openNew("b2").skipped().at)
    }

    @Test
    fun savedAnswersCloseTheEditor() {
        val put = doc.open("b2").typed("Revisat").saved(200, """{"version":4,"block":{"id":"b2","md":"Revisat","version":4,"author":"user"}}""")
        assertNull(put.doc.edit)
        assertEquals("b2", put.at)
        assertEquals(DocBlock("b2", "Revisat", 4), put.doc.blocks[1])
        val del = doc.open("b1").typed("").saved(200, """{"version":4}""")
        assertEquals(listOf("b2"), del.doc.blocks.map { it.id })
        assertNull(del.at)
    }

    @Test
    fun aPostAnswerIsInsertedAndTheStreamCopyIgnored() {
        val s = doc.openNew("b1").typed("A\n\nB").saved(200,
            """{"version":4,"blocks":[{"id":"n1","md":"A","version":4},{"id":"n2","md":"B","version":4}]}""")
        assertEquals(listOf("b1", "n1", "n2", "b2"), s.doc.blocks.map { it.id })
        assertEquals("n2", s.at)
        val again = s.doc
            .apply(ev("""{"op":"insert","block":{"id":"n1","md":"A","version":4},"after":"b1","version":4,"author":"user"}"""))
            .apply(ev("""{"op":"insert","block":{"id":"n2","md":"B","version":4},"after":"n1","version":4,"author":"user"}"""))
        assertEquals(s.doc.blocks, again.blocks)
    }

    @Test
    fun aConflictShowsTheCurrentTextAndKeepsMine() {
        val s = doc.open("b2").typed("El meu text").saved(409,
            """{"error":"block changed","block":{"id":"b2","md":"El de l'agent","version":5,"author":"agent"}}""")
        val e = s.doc.edit!!
        assertEquals("El meu text", e.text)
        assertEquals(DocBlock("b2", "El de l'agent", 5), e.conflict)
        assertNull(s.at)
        assertNull(s.error)
        // Desar ara trepitja la versió que s'ha vist.
        assertEquals(req("PUT", "/api/docs/d1/blocks/b2", """{"md":"El meu text","base_version":5}"""), s.doc.saveRequest())
        // Després d'un 409 es desa encara que el text sigui el que s'havia obert.
        assertEquals("PUT", doc.open("b2").saved(409, """{"error":"x","block":{"id":"b2","md":"Altre","version":5}}""").doc.saveRequest()?.method)
    }

    @Test
    fun otherErrorsKeepTheText() {
        val s = doc.open("b2").typed("El meu text").saved(404, """{"error":"unknown block"}""")
        assertEquals("unknown block", s.error)
        assertEquals("El meu text", s.doc.edit?.text)
        assertEquals("HTTP 502", doc.open("b2").typed("x").saved(502, "<html>").error)
    }

    @Test
    fun eventsForTheEditedBlockDoNotReplaceTheEditor() {
        val out = doc.open("b2").typed("El meu text")
            .apply(ev("""{"op":"replace","block":{"id":"b2","md":"De l'agent","version":4},"version":4,"author":"agent"}"""))
            .apply(ev("""{"op":"replace","block":{"id":"b1","md":"# Nou títol","version":5},"version":5,"author":"agent"}"""))
            .apply(ev("""{"op":"delete","block":{"id":"b2"},"version":6,"author":"agent"}"""))
        assertEquals("El meu text", out.edit?.text)
        assertEquals(2, out.edit?.base)
        assertEquals(listOf("b1" to "# Nou títol", "b2" to "De l'agent"), mds(out))
        // Sense editor, el mateix delete s'aplica.
        val closed = out.copy(edit = null).apply(ev("""{"op":"delete","block":{"id":"b2"},"version":7,"author":"agent"}"""))
        assertEquals(listOf("b1"), closed.blocks.map { it.id })
    }

    private val delB2 = """{"op":"delete","block":{"id":"b2"},"version":4,"author":"agent"}"""

    @Test
    fun aNewBlockEditorMovesUpWhenItsAnchorIsDeleted() {
        val out = doc.openNew("b2").typed("Nou").apply(ev(delB2))
        assertEquals(listOf("b1"), out.blocks.map { it.id })
        assertEquals("b1", out.edit?.after)
        assertEquals("Nou", out.edit?.text)
        assertEquals(req("POST", "/api/docs/d1/blocks", """{"after":"b1","md":"Nou"}"""), out.saveRequest())
        // El primer bloc: el nou anirà al principi.
        val top = doc.openNew("b1").apply(ev("""{"op":"delete","block":{"id":"b1"},"version":4,"author":"agent"}"""))
        assertEquals("", top.edit?.after)
    }

    @Test
    fun aDeletedBlockLeavesWhenItsEditorCloses() {
        val gone = doc.open("b2").apply(ev(delB2))
        assertEquals(listOf("b1", "b2"), gone.blocks.map { it.id })
        // Sense tocar: es tanca i el bloc se'n va, com a la web.
        assertEquals(listOf("b1"), gone.skipped().doc.blocks.map { it.id })
        // Tocat: el servidor diu 404; es tanca, el bloc se'n va i es diu.
        val s = gone.typed("El meu text").saved(404, """{"error":"unknown block"}""")
        assertNull(s.doc.edit)
        assertEquals(listOf("b1"), s.doc.blocks.map { it.id })
        assertEquals("unknown block", s.error)
    }

    @Test
    fun versionNeverGoesBack() {
        val s = doc.open("b2").typed("x").saved(200, """{"version":4,"block":{"id":"b2","md":"x","version":4}}""").doc
            .apply(ev("""{"op":"replace","block":{"id":"b1","md":"y","version":6},"version":6,"author":"agent"}"""))
            .apply(ev("""{"op":"replace","block":{"id":"b2","md":"x","version":4},"version":4,"author":"user"}"""))
        assertEquals(6, s.version)
    }

    // --- inici de Gregal Docs (071) ---

    private val mineRaw = """{"items":[{"id":"m1","title":"Carta","kind":"doc","version":2,"updated":"2026-11-22T10:00:00Z"},
        {"id":"m2","title":"Pitch","kind":"deck","version":1,"updated":"2026-11-21T10:00:00Z"}]}"""

    @Test
    fun eachListOpensItsDocumentsWithItsScope() {
        val mine = DocsJson.list(mineRaw, "mine")
        assertEquals(listOf(DocRef("m1", "Carta", "doc", "mine"), DocRef("m2", "Pitch", "deck", "mine")), mine)
        assertEquals("/api/docs/m1?scope=mine", docPath(mine[0]))
        assertEquals("/api/docs/m1/stream?scope=mine", docPath(mine[0], "/stream"))
        assertEquals("/api/docs/m1/export?format=docx&name=Carta.docx&scope=mine", docExportPath(mine[0]))
        // Els del projecte no duen scope.
        val project = DocsJson.list("""{"items":[{"id":"p1","title":"Informe","kind":"doc"}]}""")
        assertEquals("/api/docs/p1", docPath(project[0]))
        assertEquals("/api/docs/p1/stream", docPath(project[0], "/stream"))
        // Un document d'un vent avisa amb scope mine (066).
        assertEquals("mine", DocsJson.changed("""{"id":"w1","title":"Carta","scope":"mine"}""")?.scope)
    }

    @Test
    fun aWindDocumentUsesMyScopeAndACodiDocumentNever() {
        // El doc_changed del flux d'un vent (WindDocCardTest) i el d'un xat de Codi.
        val wind = DocsJson.changed("""{"id":"w1","title":"Carta","scope":"mine"}""")!!
        val codi = DocsJson.changed("""{"id":"w1","title":"Carta"}""")!!
        // La llista on es marca el document obert (DocScreen compara id i scope).
        assertTrue(wind in DocsJson.list("""{"items":[{"id":"w1","title":"Carta","kind":"doc"}]}""", "mine"))
        assertTrue(codi in DocsJson.list("""{"items":[{"id":"w1","title":"Carta","kind":"doc"}]}"""))
        assertEquals("/api/docs/w1?scope=mine", docPath(wind))
        assertEquals("/api/docs/w1/stream?scope=mine", docPath(wind, "/stream"))
        assertEquals("/api/docs/w1/export?format=docx&name=Carta.docx&scope=mine", docExportPath(wind))
        assertEquals("/api/docs/w1", docPath(codi))
        assertEquals("/api/docs/w1/stream", docPath(codi, "/stream"))
        assertEquals("/api/docs/w1/export?format=docx&name=Carta.docx", docExportPath(codi))
        // followDoc copia el scope del ref al document llegit; les edicions el porten.
        val windDoc = doc.copy(scope = wind.scope)
        val codiDoc = doc.copy(scope = codi.scope)
        assertEquals(req("PUT", "/api/docs/d1/blocks/b2?scope=mine", """{"md":"X","base_version":2}"""), windDoc.open("b2").typed("X").saveRequest())
        assertEquals(req("DELETE", "/api/docs/d1/blocks/b1?base_version=1&scope=mine"), windDoc.open("b1").typed("").saveRequest())
        assertEquals(req("POST", "/api/docs/d1/blocks?scope=mine", """{"after":"b2","md":"Nou"}"""), windDoc.openNew("b2").typed("Nou").saveRequest())
        assertEquals(req("PUT", "/api/docs/d1/blocks/b2", """{"md":"X","base_version":2}"""), codiDoc.open("b2").typed("X").saveRequest())
        assertEquals(req("DELETE", "/api/docs/d1/blocks/b1?base_version=1"), codiDoc.open("b1").typed("").saveRequest())
        assertEquals(req("POST", "/api/docs/d1/blocks", """{"after":"b2","md":"Nou"}"""), codiDoc.openNew("b2").typed("Nou").saveRequest())
    }

    @Test
    fun theProjectSectionNeedsACodiChatWithAProject() {
        assertEquals("gregal", docsProjectName(cwd = "/src/gregal", project = "gregal", vents = false))
        assertEquals("/src/gregal", docsProjectName(cwd = "/src/gregal", project = "", vents = false))
        assertNull(docsProjectName(cwd = "", project = "", vents = false))
        assertNull(docsProjectName(cwd = "/src/gregal", project = "gregal", vents = true))
    }

    @Test
    fun blockEditsOfAPersonalDocumentCarryTheScope() {
        val mine = doc.copy(scope = "mine")
        assertEquals(req("PUT", "/api/docs/d1/blocks/b2?scope=mine", """{"md":"X","base_version":2}"""), mine.open("b2").typed("X").saveRequest())
        assertEquals(req("DELETE", "/api/docs/d1/blocks/b1?base_version=1&scope=mine"), mine.open("b1").typed("").saveRequest())
        assertEquals(req("POST", "/api/docs/d1/blocks?scope=mine", """{"after":"","md":"Nou"}"""), mine.openNew("").typed("Nou").saveRequest())
    }

    @Test
    fun aNewDocumentIsCreatedInMyDocuments() {
        assertEquals(req("POST", "/api/docs?scope=mine", """{"title":"Carta","kind":"doc"}"""), docCreateRequest("Carta"))
    }

    @Test
    fun anEmptyDocumentShowsABlockToWriteIn() {
        val created = DocsJson.doc("""{"id":"n1","title":"Carta","kind":"doc","version":0,"blocks":null}""").copy(scope = "mine")
        assertTrue(created.blank)
        // Tocar el bloc buit obre un editor al principi; mentre s'escriu ja no hi és.
        val writing = created.openNew("")
        assertFalse(writing.blank)
        assertEquals(req("POST", "/api/docs/n1/blocks?scope=mine", """{"after":"","md":"Hola"}"""), writing.typed("Hola").saveRequest())
        // Tancat sense res, torna.
        assertTrue(writing.skipped().doc.blank)
        // Esborrat l'últim bloc, torna.
        assertTrue(doc.copy(blocks = doc.blocks.take(1)).open("b1").typed("").saved(200, """{"version":4}""").doc.blank)
        assertFalse(doc.blank)
        // Una presentació no s'escriu com a markdown.
        assertFalse(DocsJson.doc("""{"id":"k1","title":"Pitch","kind":"deck","version":0,"blocks":[]}""").blank)
    }
}
