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

    // --- fulls de càlcul (077) ---

    private val sheet = DocsJson.doc("""{"id":"s1","title":"Pressupost","kind":"sheet","version":4,"blocks":[
        {"id":"f1","md":"","name":"Despeses","cells":{"A1":"Lloguer","B1":"800","B2":"=B1*2"},"values":{"A1":"Lloguer","B1":"800","B2":"1600"},"version":3,"author":"agent"},
        {"id":"f2","md":"","name":"Resum","cells":{"A1":"=Despeses!B2"},"values":{"A1":"1600"},"version":4,"author":"agent"}]}""")

    @Test
    fun sheetEventsAreAppliedInPlace() {
        assertEquals("f1", sheet.shownSheet()?.id)
        assertEquals("Despeses", sheet.shownSheet()?.name)
        assertEquals("1600", sheet.shownSheet()?.values?.get("B2"))
        val out = sheet.showSheet("f2")
            .apply(ev("""{"op":"replace","block":{"id":"f1","md":"","name":"Despeses","cells":{"A1":"Lloguer","B1":"900","B2":"=B1*2"},"values":{"A1":"Lloguer","B1":"900","B2":"1800"},"version":5},"version":5,"author":"agent"}"""))
            // Un full que canvia només de valors perquè un altre ha canviat: replace sense versió de bloc nova.
            .apply(ev("""{"op":"replace","block":{"id":"f2","md":"","name":"Resum","cells":{"A1":"=Despeses!B2"},"values":{"A1":"1800"},"version":4},"version":5,"author":"agent"}"""))
            .apply(ev("""{"op":"insert","block":{"id":"f3","md":"","name":"Notes","version":6},"after":"f2","version":6,"author":"agent"}"""))
        assertEquals(listOf("Despeses", "Resum", "Notes"), out.blocks.map { it.name })
        assertEquals("1800", out.blocks[0].values["B2"])
        assertEquals("f2", out.shownSheet()?.id)
        assertEquals("1800", out.shownSheet()?.values?.get("A1"))
        assertEquals(emptyMap<String, String>(), out.blocks[2].cells)
        // El full que es mirava, esborrat: es mostra el primer.
        assertEquals("f1", out.apply(ev("""{"op":"delete","block":{"id":"f2"},"version":7,"author":"agent"}""")).shownSheet()?.id)
    }

    @Test
    fun theGridHasColumnLettersAndRoomToWrite() {
        assertEquals(listOf("A", "B", "Z", "AA", "AZ", "BA"), listOf(0, 1, 25, 26, 51, 52).map { sheetColumn(it) })
        // Unes quantes columnes i files buides per escriure-hi, més enllà de les dades.
        assertEquals(6 to 20, sheetSize(sheet.blocks[0]))
        assertEquals(9 to 45, sheetSize(DocBlock("f", cells = mapOf("H40" to "1", "A2" to "x"))))
        assertEquals(6 to 20, sheetSize(DocBlock("f")))
    }

    @Test
    fun aCellEditSendsTheCellWithTheSheetVersion() {
        val e = sheet.openCell("f1", "B2")
        assertEquals("=B1*2", e.edit?.text)
        assertNull(e.saveRequest())
        assertNull(e.typed("=B1*2 ").saveRequest())
        assertEquals(req("PUT", "/api/docs/s1/blocks/f1", """{"cells":{"B2":"=B1*3"},"base_version":3}"""), e.typed("=B1*3").saveRequest())
        // Buidar una cel·la l'esborra, no el full.
        assertEquals(req("PUT", "/api/docs/s1/blocks/f1", """{"cells":{"B2":""},"base_version":3}"""), e.typed("").saveRequest())
        // Una cel·la buida s'obre buida, i amb el scope del document.
        assertEquals(req("PUT", "/api/docs/s1/blocks/f1?scope=mine", """{"cells":{"C9":"12"},"base_version":3}"""),
            sheet.copy(scope = "mine").openCell("f1", "C9").typed("12").saveRequest())
    }

    @Test
    fun aSavedCellShowsTheNewValues() {
        val s = sheet.openCell("f1", "B1").typed("900").saved(200,
            """{"version":5,"block":{"id":"f1","md":"","name":"Despeses","cells":{"A1":"Lloguer","B1":"900","B2":"=B1*2"},"values":{"A1":"Lloguer","B1":"900","B2":"1800"},"version":5}}""")
        assertNull(s.doc.edit)
        assertNull(s.error)
        assertEquals(listOf("f1", "f2"), s.doc.blocks.map { it.id })
        assertEquals("1800", s.doc.blocks[0].values["B2"])
        // Buidada, el full es queda.
        val cleared = sheet.openCell("f1", "B1").typed("").saved(200,
            """{"version":5,"block":{"id":"f1","md":"","name":"Despeses","cells":{"A1":"Lloguer"},"values":{"A1":"Lloguer"},"version":5}}""")
        assertEquals(listOf("f1", "f2"), cleared.doc.blocks.map { it.id })
        assertNull(cleared.doc.blocks[0].cells["B1"])
    }

    @Test
    fun aCellConflictKeepsMyInputWithTheNotice() {
        val s = sheet.openCell("f1", "B1").typed("1000").saved(409,
            """{"error":"block changed","block":{"id":"f1","md":"","name":"Despeses","cells":{"B1":"850"},"values":{"B1":"850"},"version":6}}""")
        val e = s.doc.edit!!
        assertEquals("1000", e.text)
        assertEquals("850", e.conflict?.cells?.get("B1"))
        assertEquals("850", s.doc.blocks[0].values["B1"])
        assertNull(s.error)
        // Tornar a desar trepitja la versió que s'ha vist.
        assertEquals(req("PUT", "/api/docs/s1/blocks/f1", """{"cells":{"B1":"1000"},"base_version":6}"""), s.doc.saveRequest())
    }

    @Test
    fun aSheetDownloadsAsExcelAndIsCreatedInMyDocuments() {
        val url = docExportPath(DocRef("s1", "Pressupost 2027", "sheet", "mine"))
        assertEquals("/api/docs/s1/export?format=xlsx&name=Pressupost+2027.xlsx&scope=mine", url)
        assertTrue(isServerDocLink(url))
        assertEquals("Pressupost 2027.xlsx", serverDocName(url))
        assertEquals(req("POST", "/api/docs?scope=mine", """{"title":"Pressupost","kind":"sheet"}"""), docCreateRequest("Pressupost", "sheet"))
        assertFalse(sheet.blank)
    }

    // --- presentacions (074) ---

    private val deck = DocsJson.doc("""{"id":"k1","title":"Llançament","kind":"deck","version":5,"blocks":[
        {"id":"s1","md":"","slide":{"layout":"title","title":"Llançament 2027","subtitle":"Pla de l'equip"},"version":2,"author":"agent"},
        {"id":"s2","md":"","slide":{"layout":"content","title":"Objectius","bullets":["Créixer","  Mercats nous"],"chart":{"type":"bar","categories":["A"],"series":[{"name":"x","values":[1]}]},"notes":"Breu"},"version":5,"author":"agent"}]}""")

    @Test
    fun deckEventsAreAppliedInPlace() {
        assertEquals("Llançament 2027", slideText(deck.blocks[0].slide, "title"))
        assertEquals(listOf("Créixer", "  Mercats nous"), slideBullets(deck.blocks[1].slide))
        val out = deck
            .apply(ev("""{"op":"replace","block":{"id":"s1","md":"","slide":{"layout":"title","title":"Llançament 2028"},"version":6},"version":6,"author":"agent"}"""))
            .apply(ev("""{"op":"insert","block":{"id":"s3","md":"","slide":{"layout":"big_number","number":"42%","caption":"Quota"},"version":7},"after":"s1","version":7,"author":"agent"}"""))
            .apply(ev("""{"op":"delete","block":{"id":"s2"},"version":8,"author":"agent"}"""))
        assertEquals(listOf("s1", "s3"), out.blocks.map { it.id })
        assertEquals("Llançament 2028", slideText(out.blocks[0].slide, "title"))
        assertEquals("", slideText(out.blocks[0].slide, "subtitle"))
        assertEquals("42%", slideText(out.blocks[1].slide, "number"))
        // La diapositiva que s'edita es queda amb el que la persona hi escriu.
        val editing = deck.open("s1").typedField("title", "Meu")
            .apply(ev("""{"op":"replace","block":{"id":"s1","md":"","slide":{"layout":"title","title":"De l'agent"},"version":6},"version":6,"author":"agent"}"""))
        assertEquals("Meu", editing.edit?.fields?.get("title"))
        assertEquals("De l'agent", slideText(editing.blocks[0].slide, "title"))
    }

    @Test
    fun aSlideFormShowsTheFieldsOfItsLayout() {
        assertEquals(listOf("title", "subtitle", "notes"), slideFields("title"))
        assertEquals(listOf("title", "body", "bullets", "notes"), slideFields("content"))
        assertEquals(listOf("title", "left_title", "left", "right_title", "right", "notes"), slideFields("comparison"))
        assertEquals(listOf("title", "number", "caption", "notes"), slideFields("big_number"))
        assertEquals(listOf("title", "image", "body", "notes"), slideFields("image"))
        assertEquals(listOf("notes"), slideFields("blank"))
        assertEquals(10, slideLayouts.size)
        val e = deck.open("s2").edit!!
        assertEquals(mapOf("title" to "Objectius", "body" to "", "bullets" to "Créixer\n  Mercats nous", "notes" to "Breu"), e.fields)
        assertEquals(5, e.base)
    }

    @Test
    fun eachSlideActionBuildsItsRequest() {
        val open = deck.open("s2")
        // Sense tocar (o només amb espais) no es desa res.
        assertNull(open.saveRequest())
        assertNull(open.typedField("title", "Objectius ").saveRequest())
        // Es desa la diapositiva sencera: el gràfic, que el formulari no mostra, es queda.
        assertEquals(req("PUT", "/api/docs/k1/blocks/s2?scope=mine",
            """{"slide":{"layout":"content","title":"Objectius 2027","bullets":["Créixer","  Mercats nous","Marge"],"chart":{"type":"bar","categories":["A"],"series":[{"name":"x","values":[1]}]},"body":"Tres eixos"},"base_version":5}"""),
            deck.copy(scope = "mine").open("s2").typedField("title", "Objectius 2027").typedField("body", " Tres eixos")
                .typedField("bullets", "Créixer\n  Mercats nous\n\nMarge").typedField("notes", "").saveRequest())
        assertEquals(req("DELETE", "/api/docs/k1/blocks/s2?base_version=5"), open.deleteRequest())
        // Una diapositiva nova va després de l'última oberta; sense cap d'oberta, al final; en una buida, al principi.
        assertEquals("s1", deck.open("s1").skipped().doc.slideAfter)
        assertEquals("s2", deck.slideAfter)
        assertEquals("", deck.copy(blocks = emptyList()).slideAfter)
        assertEquals(req("POST", "/api/docs/k1/blocks?scope=mine", """{"after":"s1","slide":{"layout":"two_columns"}}"""),
            deck.copy(scope = "mine").addSlideRequest("s1", "two_columns"))
    }

    @Test
    fun aSavedOrAddedOrDeletedSlideUpdatesTheDeck() {
        val s = deck.open("s1").typedField("title", "Nou").saved(200,
            """{"version":6,"block":{"id":"s1","md":"","slide":{"layout":"title","title":"Nou"},"version":6}}""")
        assertNull(s.doc.edit)
        assertEquals("Nou", slideText(s.doc.blocks[0].slide, "title"))
        assertEquals(listOf("s1", "s2"), s.doc.blocks.map { it.id })
        // Un DELETE respon sense bloc: la diapositiva se'n va.
        val gone = deck.open("s1").saved(200, """{"version":6}""")
        assertEquals(listOf("s2"), gone.doc.blocks.map { it.id })
        assertNull(gone.doc.edit)
        // La nova s'insereix al seu lloc i se n'obre el formulari.
        val added = deck.slideAdded("s1", 200, """{"version":6,"blocks":[{"id":"s9","md":"","slide":{"layout":"two_columns"},"version":6}]}""")
        assertEquals(listOf("s1", "s9", "s2"), added.doc.blocks.map { it.id })
        assertEquals("s9", added.doc.edit?.id)
        assertEquals("s9", added.doc.slideAfter)
        assertEquals(listOf("title", "left_title", "left", "right_title", "right", "notes"), added.doc.edit?.fields?.keys?.toList())
        assertNull(added.error)
        assertEquals("bad layout", deck.slideAdded("s1", 400, """{"error":"bad layout"}""").error)
    }

    @Test
    fun aSlideConflictKeepsMyFieldsWithTheNotice() {
        val s = deck.open("s2").typedField("title", "Meus objectius").saved(409,
            """{"error":"block changed","block":{"id":"s2","md":"","slide":{"layout":"content","title":"Objectius de l'agent","table":[["a","b"]]},"version":7}}""")
        val e = s.doc.edit!!
        assertNull(s.error)
        assertEquals("Meus objectius", e.fields["title"])
        assertEquals("Objectius de l'agent", slideText(e.conflict?.slide, "title"))
        assertEquals("Objectius de l'agent", slideText(s.doc.blocks[1].slide, "title"))
        // Tornar a desar trepitja la versió vista, sobre la diapositiva d'ara (la taula es queda).
        assertEquals(req("PUT", "/api/docs/k1/blocks/s2",
            """{"slide":{"layout":"content","title":"Meus objectius","table":[["a","b"]],"bullets":["Créixer","  Mercats nous"],"notes":"Breu"},"base_version":7}"""),
            s.doc.saveRequest())
        assertEquals(req("DELETE", "/api/docs/k1/blocks/s2?base_version=7"), s.doc.deleteRequest())
        // Esborrar també pot topar amb un canvi de l'agent: la diapositiva es queda.
        val d = deck.open("s2").saved(409, """{"error":"block changed","block":{"id":"s2","md":"","slide":{"layout":"content","title":"Altre"},"version":8}}""")
        assertEquals(8, d.doc.edit?.base)
        assertEquals(listOf("s1", "s2"), d.doc.blocks.map { it.id })
    }

    @Test
    fun aDeckDownloadsAsPowerPointAndIsCreatedInMyDocuments() {
        val url = docExportPath(DocRef("k1", "Pitch 2027", "deck", "mine"))
        assertEquals("/api/docs/k1/export?format=pptx&name=Pitch+2027.pptx&scope=mine", url)
        assertTrue(isServerDocLink(url))
        assertEquals("Pitch 2027.pptx", serverDocName(url))
        assertEquals(req("POST", "/api/docs?scope=mine", """{"title":"Pitch","kind":"deck"}"""), docCreateRequest("Pitch", "deck"))
    }

    // --- l'editor de la web en un WebView (083) ---

    @Test
    fun aTextDocumentOpensTheWebPageWithTheKeyOutsideTheUrl() {
        val mine = DocRef("m1", "Carta", "doc", "mine")
        val page = docPageUrl("https://gregal.example:8080", mine, "mobil")
        assertEquals("https://gregal.example:8080/app/doc/m1?session=mobil&scope=mine", page)
        // Un document del projecte no du scope; l'id va codificat com un tros de ruta.
        assertEquals("https://gregal.example:8080/app/doc/p%201?session=mobil", docPageUrl("https://gregal.example:8080", DocRef("p 1", "Informe"), "mobil"))
        // La clau va a la capçalera de la càrrega i al pont que llegeix doc-page.js, mai a la URL.
        assertFalse("clau-secreta" in page)
        assertEquals(mapOf("Authorization" to "Bearer clau-secreta"), docPageHeaders("clau-secreta"))
        assertEquals(emptyMap<String, String>(), docPageHeaders(""))
        assertEquals("clau-secreta", DocPageBridge({ "clau-secreta" }) { "en" }.token())
        // Només els documents de text; les presentacions i els fulls tenen la seva pantalla.
        assertTrue(doc.inWebEditor)
        assertFalse(deck.inWebEditor)
        assertFalse(DocsJson.doc("""{"id":"s1","title":"Pressupost","kind":"sheet","blocks":[]}""").inWebEditor)
    }

    @Test
    fun theWebEditorKeepsWordDownloadAndNeverNavigatesAway() {
        val mine = DocRef("m1", "Carta", "doc", "mine")
        assertEquals("/api/docs/m1/export?format=docx&name=Carta.docx&scope=mine", docExportPath(mine))
        val page = docPageUrl("http://10.0.0.2:8080", mine, "mobil")
        // Només la pàgina mateixa es carrega al WebView: un enllaç s'obre fora, i Enrere surt del document com a les altres pantalles.
        assertTrue(docPageKeeps(page, page))
        assertFalse(docPageKeeps("https://example.com/", page))
        assertFalse(docPageKeeps("http://10.0.0.2:8080/app/", page))
    }

    @Test
    fun theBridgeGivesTheCurrentKeyAfterTheConnectionChangesIt() {
        var key = "clau-vella"
        val bridge = DocPageBridge({ key }) { "en" }
        key = "clau-nova"
        assertEquals("clau-nova", bridge.token())
    }

    // doc-page.js no té cap idioma desat al WebView: el pren del pont, i la barra de l'editor surt en el de l'app.
    @Test
    fun theBridgeGivesThePageTheAppLanguage() {
        var lang = "en"
        val bridge = DocPageBridge({ "k" }) { lang }
        assertEquals("en", bridge.lang())
        lang = "ca"
        assertEquals("ca", bridge.lang())
    }

    // --- el que faltava a Android (notes de 063-083) ---

    private val sheets = DocsJson.doc("""{"id":"f1","title":"Pressupost","kind":"sheet","version":4,
        "blocks":[{"id":"h1","name":"Full1","cells":{"A1":"1"},"version":2},{"id":"h2","name":"Full2","cells":{},"version":3}]}""")

    @Test
    fun aHugeCellAddressDoesNotMakeAHugeGrid() {
        val (cols, rows) = sheetSize(DocBlock("h1", cells = mapOf("ZZZ999999" to "x")))
        assertEquals(100, cols)
        assertEquals(10_000, rows)
        assertEquals(6 to 20, sheetSize(DocBlock("h1", cells = mapOf("A1" to "1"))))
    }

    @Test
    fun aReconnectKeepsTheTabTheSlideAndTheOpenEditor() {
        val seen = sheets.showSheet("h2").openCell("h2", "B2").typed("42")
        val reread = seen.kept(sheets.copy(version = 6))
        assertEquals("h2", reread.shownSheet()?.id)
        assertEquals("42", reread.edit?.text)
        assertEquals(6, reread.version)
        // Si el full de l'editor ja no hi és, l'editor no es queda penjant.
        assertNull(seen.kept(sheets.copy(blocks = sheets.blocks.take(1))).edit)
        assertEquals("s2", deck.open("s2").kept(deck).current)
        // Un altre document no hereta res.
        assertEquals("", seen.kept(deck).sheet)
    }

    @Test
    fun anEmptySpreadsheetCanAddASheetWithTheFirstFreeName() {
        val empty = sheets.copy(blocks = emptyList(), scope = "mine")
        assertEquals(req("POST", "/api/docs/f1/blocks?scope=mine", """{"after":"","name":"Full1"}"""), empty.addSheetRequest("Full"))
        assertEquals(req("POST", "/api/docs/f1/blocks", """{"after":"h2","name":"Full3"}"""), sheets.addSheetRequest("Full"))
        // El nom no es repeteix encara que un altre full es digui igual en minúscules.
        assertEquals(req("POST", "/api/docs/f1/blocks", """{"after":"h3","name":"Full4"}"""),
            sheets.copy(blocks = sheets.blocks + DocBlock("h3", name = "full3")).addSheetRequest("Full"))
        val added = sheets.sheetAdded(200, """{"version":5,"blocks":[{"id":"h9","name":"Full3","cells":{},"version":5}]}""")
        assertEquals(listOf("h1", "h2", "h9"), added.doc.blocks.map { it.id })
        assertEquals("h9", added.doc.shownSheet()?.id)
        assertEquals("bad name", sheets.sheetAdded(400, """{"error":"bad name"}""").error)
    }

    @Test
    fun theLayoutOfAnOpenSlideCanBeChanged() {
        val open = deck.open("s2").chooseLayout("two_columns")
        // El títol escrit es queda; els camps són els del layout nou.
        assertEquals(listOf("title", "left_title", "left", "right_title", "right", "notes"), open.edit?.fields?.keys?.toList())
        assertEquals("Objectius", open.edit?.fields?.get("title"))
        val sent = open.typedField("left", "Ara").saveRequest()!!
        assertTrue(""""layout":"two_columns"""" in sent.body!!)
        assertTrue(""""left":"Ara"""" in sent.body!!)
        // Triar un altre layout ja és un canvi per desar.
        assertTrue(deck.open("s2").chooseLayout("title_only").saveRequest() != null)
    }

    @Test
    fun aWebImageOfASlideIsShownAndAProjectPathIsNot() {
        fun slide(image: String) = DocsJson.doc("""{"id":"k","kind":"deck","blocks":[{"id":"s","slide":{"layout":"image","image":"$image"}}]}""").blocks[0].slide
        assertEquals("https://example.com/a.png", slideImageUrl(slide(" https://example.com/a.png")))
        assertEquals("http://example.com/a.png", slideImageUrl(slide("http://example.com/a.png")))
        assertNull(slideImageUrl(slide("imatges/a.png")))
        assertNull(slideImageUrl(null))
    }
}
