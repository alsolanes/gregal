package xyz.solanes.gregal

import androidx.compose.ui.graphics.Color
import org.junit.Assert.*
import org.junit.Test

/** Cursiva simple *text*: títols d'obres que els models escriuen així. */
class MdItalicTest {
    @Test fun `cursiva simple es parseja`() {
        val segs = splitMd("llegeix *He Who Fights With Monsters* de ShirtAloon")
        assertEquals(
            listOf(Seg.Text("llegeix "), Seg.Italic("He Who Fights With Monsters"), Seg.Text(" de ShirtAloon")),
            segs,
        )
    }
    @Test fun `negreta doble no es trenca`() {
        val segs = splitMd("hola **món** fi")
        assertTrue(segs.any { it is Seg.Bold && (it as Seg.Bold).t == "món" })
        assertTrue(segs.none { it is Seg.Italic })
    }
    @Test fun `estel dins paraula no es cursiva`() {
        val segs = splitMd("2*3 són 6")
        assertTrue(segs.none { it is Seg.Italic })
    }
    @Test fun `estel amb espais no es cursiva`() {
        val segs = splitMd("* no tancat i * tampoc")
        assertTrue(segs.none { it is Seg.Italic })
        assertEquals("a * b * c", stripMdInline("a * b * c"))
    }
    @Test fun `el directe neteja la cursiva`() {
        assertEquals("llegeix Dune avui", stripMdInline("llegeix *Dune* avui"))
        assertEquals("hola món fi", stripMdInline("hola **món** fi"))
    }
    @Test fun `render no perd el text en cursiva`() {
        val rich = renderSegs(listOf(Seg.Italic("Dune")), Color.Black, link = Color.Blue)
        assertEquals("Dune", rich.text)
    }
}
