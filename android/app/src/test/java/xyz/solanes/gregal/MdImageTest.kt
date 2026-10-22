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
    @Test fun `enllac normal continua sent Link`() {
        val segs = splitMd("mira [gato](https://x.example/gato.png) ja")
        assertTrue(segs.filterIsInstance<Seg.Image>().isEmpty())
        assertEquals(1, segs.filterIsInstance<Seg.Link>().size)
    }
    @Test fun `groupRich treu la imatge a bloc propi`() {
        val rich = groupRich(splitMd("text ![a](https://x.example/a.png) més"))
        assertEquals(3, rich.size)
        val img = rich[1] as Rich.Image
        assertEquals("https://x.example/a.png", img.url)
    }
    @Test fun `streaming net sense marca d'imatge`() {
        assertEquals("mira gato ja", stripMdInline("mira ![gato](https://x.example/gato.png) ja"))
    }
}
