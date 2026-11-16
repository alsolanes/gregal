package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/* El visor a pantalla completa (Viewer.kt): què s'hi pot obrir d'una
 * conversa, en quin ordre, i com s'hi navega. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class ViewerTest {
    private val fence = "```"
    private val photo = "![Montserrat](https://upload.wikimedia.org/m.jpg)"
    private val svg = "${fence}svg\n<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"10\" height=\"10\"><rect width=\"10\" height=\"10\"/></svg>\n$fence"
    private val web = "${fence}gregal-web\n{\"title\":\"Calculadora\",\"path\":\"calc/index.html\",\"url\":\"/p/0123456789ab/calc/index.html\"}\n$fence"

    @Test
    fun aConversationListsEverythingThatCanOpenLargeInOrder() {
        val items = ViewerLogic.itemsOfTexts(listOf("Mira:\n\n$photo\n\nI el dibuix:\n\n$svg", "La web:\n\n$web"))
        assertEquals(3, items.size)
        assertTrue(items[0] is ViewItem.Image)
        assertEquals("svg", (items[1] as ViewItem.Html).kind)
        assertEquals("Calculadora", (items[2] as ViewItem.Web).spec.title)
    }

    @Test
    fun plainCodeAndTextAreNotPreviews() {
        assertTrue(ViewerLogic.itemsOfTexts(listOf("Hola.\n\n${fence}kotlin\nfun x() = 1\n$fence")).isEmpty())
    }

    @Test
    fun theSamePhotoTwiceIsOnePage() {
        assertEquals(1, ViewerLogic.itemsOfTexts(listOf(photo, "Una altra vegada: $photo")).size)
    }

    @Test
    fun openingFindsItsPlaceOrShowsAlone() {
        val items = ViewerLogic.itemsOfTexts(listOf("$photo\n\n$svg\n\n$web"))
        val (list, at) = ViewerLogic.place(items, items[2])
        assertEquals(items, list)
        assertEquals(2, at)
        val stray = ViewItem.Image("https://e.org/x.png", "x")
        assertEquals(listOf<ViewItem>(stray) to 0, ViewerLogic.place(items, stray))
    }

    @Test
    fun linksForOpeningOutsideAndSharing() {
        val items = ViewerLogic.itemsOfTexts(listOf("$photo\n\n$svg\n\n$web"))
        assertEquals("https://upload.wikimedia.org/m.jpg", ViewerLogic.link(items[0], "http://10.0.2.2:8197"))
        assertEquals("", ViewerLogic.link(items[1], "http://10.0.2.2:8197"))
        assertEquals("http://10.0.2.2:8197/p/0123456789ab/calc/index.html", ViewerLogic.link(items[2], "http://10.0.2.2:8197/"))
        // Les fotos dels vents viuen al servidor amb clau: no s'obren fora.
        assertEquals("", ViewerLogic.link(ViewItem.Image("/api/winds/image?id=abc", ""), "http://h"))
    }

    @Test
    fun pagesWithTheirOwnGesturesChangeWithArrows() {
        val items = ViewerLogic.itemsOfTexts(listOf("$photo\n\n$svg\n\n$web"))
        assertFalse("a photo pinches to zoom", ViewerLogic.swipes(items[0]))
        assertFalse("a page scrolls itself", ViewerLogic.swipes(items[1]))
        assertFalse(ViewerLogic.swipes(items[2]))
    }

    @Test
    fun showUsesTheConversationList() {
        val items = ViewerLogic.itemsOfTexts(listOf("$photo\n\n$svg"))
        Viewer.items = items
        Viewer.show(items[1])
        assertEquals(items to 1, Viewer.open)
        Viewer.close()
        assertNull(Viewer.open)
    }

    @Test
    fun anSvgFillsTheScreenOnlyInTheViewer() {
        val code = "<svg viewBox=\"0 0 10 10\"><rect width=\"10\" height=\"10\"/></svg>"
        val big = WebPreviewLogic.srcdoc("svg", code, fill = true)
        assertTrue(big.contains("width=device-width") && big.contains("svg{display:block;width:100%;height:auto"))
        assertTrue("still isolated", big.contains("Content-Security-Policy"))
        assertFalse(WebPreviewLogic.srcdoc("svg", code).contains("display:block;width:100%;height:auto"))
        // Sense viewBox: se'n fa un de width/height, i la mida fixa fora.
        val fixed = WebPreviewLogic.scalableSvg("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"200\" height=\"120px\"><circle r=\"5\"/></svg>")
        assertTrue(fixed, fixed.startsWith("<svg viewBox=\"0 0 200 120\" xmlns="))
        assertFalse(fixed, fixed.contains("width=") || fixed.contains("height=\"120"))
        assertTrue("children keep their sizes", WebPreviewLogic.scalableSvg("<svg viewBox=\"0 0 9 9\"><rect width=\"3\" height=\"3\"/></svg>").contains("<rect width=\"3\" height=\"3\"/>"))
    }

    // Un enllaç a un fitxer del servidor amb l'adreça de l'ordinador (127.0.0.1)
    // o relatiu s'obre contra el servidor de l'app; un enllaç de fora, igual.
    @Test
    fun serverLinksOpenAgainstTheConnectedServer() {
        val base = "http://192.168.1.62:8097/"
        assertEquals("http://192.168.1.62:8097/f/c46947cafe41a8d206d7f011/index.html",
            WebPreviewLogic.serverLink("http://127.0.0.1:8097/f/c46947cafe41a8d206d7f011/index.html", base))
        assertEquals("http://192.168.1.62:8097/p/abc123/x.html", WebPreviewLogic.serverLink("/p/abc123/x.html", base))
        assertEquals("http://192.168.1.62:8097/api/winds/image?id=1", WebPreviewLogic.serverLink("http://localhost:8097/api/winds/image?id=1", base))
        assertEquals("https://acb.com/es/noticia", WebPreviewLogic.serverLink("https://acb.com/es/noticia", base))
        assertEquals("http://127.0.0.1:3000/app", WebPreviewLogic.serverLink("http://127.0.0.1:3000/app", base))
    }
}
