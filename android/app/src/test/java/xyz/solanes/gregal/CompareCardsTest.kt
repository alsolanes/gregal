package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/* Comparació i xifres clau (CompareCards.kt), com md.js a l'escriptori. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class CompareCardsTest {
    private val fence = "```"

    @Test
    fun aComparisonHasItsItemsPhotosAndSharedRows() {
        val json = """{"title":"Guepard o lleopard","items":[{"name":"Guepard","image":"https://upload.wikimedia.org/c.jpg","facts":{"Velocitat":"110 km/h","Pes":"35-70 kg"}},{"name":"Lleopard","image":"javascript:x","facts":[{"label":"Velocitat","value":"58 km/h"},{"label":"Caça","value":"De nit"}],"pick":true}],"verdict":"El guepard corre."}"""
        val spec = parseCompareSpec(json)!!
        assertEquals(listOf("Guepard", "Lleopard"), spec.items.map { it.name })
        assertEquals("https://upload.wikimedia.org/c.jpg", spec.items[0].image)
        assertEquals("only http(s) photos", "", spec.items[1].image)
        assertEquals(listOf("Velocitat", "Pes", "Caça"), spec.keys)
        assertEquals("", spec.value(spec.items[1], "Pes"))
        assertTrue(spec.items[1].pick)
        assertEquals("El guepard corre.", spec.verdict)
    }

    @Test
    fun oneThingIsNotAComparison() {
        assertNull(parseCompareSpec("""{"title":"x","items":[{"name":"A"}]}"""))
        val segs = splitMd("${fence}gregal-compare\n{\"title\":\"x\",\"items\":[{\"name\":\"A\"}]}\n$fence")
        assertTrue("stays as code", segs.any { it is Seg.Block })
    }

    @Test
    fun keyFiguresAreTiles() {
        val spec = parseFactsSpec("""{"title":"El cor","facts":[{"value":"300 g","label":"de pes"},{"value":"60-80","label":"batecs per minut","note":"en repòs"},{"value":"","label":"buit"}]}""")!!
        assertEquals(2, spec.facts.size)
        assertEquals("en repòs", spec.facts[1].note)
        assertNull("one figure is not a card", parseFactsSpec("""{"facts":[{"value":"1","label":"a"}]}"""))
    }

    @Test
    fun theMarkdownBecomesCardsAndTheViewerSeesThePhotos() {
        val text = "Mira:\n\n${fence}gregal-compare\n{\"title\":\"Felins\",\"items\":[{\"name\":\"Guepard\",\"image\":\"https://e.org/c.jpg\"},{\"name\":\"Lleopard\",\"image\":\"https://e.org/l.jpg\"}]}\n$fence\n\n${fence}gregal-facts\n{\"facts\":[{\"value\":\"110\",\"label\":\"km/h\"},{\"value\":\"58\",\"label\":\"km/h\"}]}\n$fence"
        val cards = richBlocks(text).filterIsInstance<Rich.Card>().map { it.card }
        assertTrue(cards.any { it is RichCard.Compare })
        assertTrue(cards.any { it is RichCard.Facts })
        val items = ViewerLogic.itemsOfTexts(listOf(text))
        assertEquals(listOf("https://e.org/c.jpg", "https://e.org/l.jpg"), items.filterIsInstance<ViewItem.Image>().map { it.url })
    }
    @Test
    fun upToEightItemsWithLogosPhotosAndSummaryPoints() {
        val many = (1..10).joinToString(",") { """{"name":"N$it"}""" }
        assertEquals(8, parseCompareSpec("""{"title":"t","items":[$many]}""")!!.items.size)
        val spec = parseCompareSpec("""{"title":"Programes","items":[
            {"name":"Partit A","image":"https://e.org/a.jpg","images":["https://e.org/a.jpg","https://e.org/a2.jpg","https://e.org/a3.jpg"],"logo":"https://e.org/a.png","url":"https://a.example/programa","points":["Un","Dos","Tres","4","5","6","7"]},
            {"name":"Partit B","logo":"ftp://x","points":"no"}]}""")!!
        val a = spec.items[0]
        assertEquals(listOf("https://e.org/a.jpg", "https://e.org/a2.jpg"), a.photos)
        assertEquals("https://e.org/a.jpg", a.image)
        assertEquals("https://e.org/a.png", a.logo)
        assertEquals("https://a.example/programa", a.url)
        assertEquals(6, a.points.size)
        assertEquals("", spec.items[1].logo)
        assertTrue("items with points page one by one", spec.paged)
        assertFalse(parseCompareSpec("""{"title":"t","items":[{"name":"A"},{"name":"B"}]}""")!!.paged)
    }
}
