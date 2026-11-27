package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

/** Imatges al xat: ![alt](url) rendeja foto, no enllaç trencat. */
class MdImageTest {
    @Test fun `parseja imatge inline`() {
        val segs = splitMd("mira ![gato](https://x.example/gato.png) ja")
        val img = segs.filterIsInstance<Seg.Image>().single()
        assertEquals("gato", img.alt)
        assertEquals("https://x.example/gato.png", img.url)
    }
    // «![](url)» és una foto (com a l'escriptori), no text amb l'adreça.
    @Test fun `imatge sense text alternatiu`() {
        val rich = groupRich(splitMd("Abadia:\n![](https://x.example/abadia.jpg)\nFi."))
        val img = rich.filterIsInstance<Rich.Image>().single()
        assertEquals("https://x.example/abadia.jpg", img.url)
        assertEquals("", img.alt)
        assertFalse(rich.filterIsInstance<Rich.Inline>().any { r -> r.segs.any { it is Seg.Text && it.t.contains("https://") } })
    }
    @Test fun `enllac sense text continua sense ser res`() {
        assertTrue(splitMd("[](https://x.example/a)").none { it is Seg.Link || it is Seg.Image })
    }
    @Test fun `enllac normal continua sent Link`() {
        val segs = splitMd("mira [gato](https://x.example/gato.png) ja")
        assertTrue(segs.filterIsInstance<Seg.Image>().isEmpty())
        assertEquals(1, segs.filterIsInstance<Seg.Link>().size)
    }
    // Un enllaç amb un esquema inventat pel model (gregal://docs/…) no
    // obre res: es queda el text, sense enllaç ni markdown cru.
    @Test fun `enllac amb esquema inventat queda com a text`() {
        val segs = splitMd("[Obre el document](gregal://docs/zjq2kr3jf3k2) i prou")
        assertTrue(segs.none { it is Seg.Link })
        val text = segs.filterIsInstance<Seg.Text>().joinToString("") { it.t }
        assertTrue(text, text.contains("Obre el document") && !text.contains("gregal://") && !text.contains("]("))
    }
    @Test fun `enllac al document viu continua sent Link`() {
        assertEquals("/api/docs/zjq2kr3jf3k2/export?format=docx", splitMd("[Quadern](/api/docs/zjq2kr3jf3k2/export?format=docx)").filterIsInstance<Seg.Link>().single().url)
    }
    @Test fun `groupRich treu la imatge a bloc propi`() {
        val rich = groupRich(splitMd("text ![a](https://x.example/a.png) més"))
        assertEquals(3, rich.size)
        val img = rich[1] as Rich.Image
        assertEquals("https://x.example/a.png", img.url)
    }
    @Test fun `cap franja buida al voltant d'una imatge`() {
        val rich = groupRich(splitMd("Aquí la tens:\n\n![a](https://x.example/a.png)\n\nI una altra cosa."))
        assertEquals(3, rich.size)
        val before = (rich[0] as Rich.Inline).segs.filterIsInstance<Seg.Text>().joinToString("") { it.t }
        val after = (rich[2] as Rich.Inline).segs.filterIsInstance<Seg.Text>().joinToString("") { it.t }
        assertEquals("Aquí la tens:", before)
        assertEquals("I una altra cosa.", after)
        assertEquals(1, groupRich(splitMd("![a](https://x.example/a.png)\n\n")).size)
    }
    @Test fun `una imatge vertical es veu sencera`() {
        assertEquals(0.8f, chatImageBoxRatio(600, 900), 0.001f)
        assertEquals(2f, chatImageBoxRatio(1200, 600), 0.001f)
        assertEquals(4f / 3f, chatImageBoxRatio(0, 0), 0.001f)
    }
    @Test fun `el peu diu que és i d'on surt`() {
        assertEquals("Camp Nou · upload.wikimedia.org", chatImageCaption("Camp Nou", "https://upload.wikimedia.org/x/a.jpg"))
        assertEquals("example.com", chatImageCaption(" ", "https://www.example.com/a.png?x=1"))
    }
    @Test fun `streaming net sense marca d'imatge`() {
        assertEquals("mira gato ja", stripMdInline("mira ![gato](https://x.example/gato.png) ja"))
    }
}
