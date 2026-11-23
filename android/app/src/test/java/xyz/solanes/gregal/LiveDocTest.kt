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
        "blocks":[{"id":"b1","md":"# Títol","version":1,"author":"agent"},{"id":"b2","md":"Primer paràgraf","version":3,"author":"agent"}]}""")

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
}
