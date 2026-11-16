package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** Tempesta a l'Android: l'esdeveniment del servidor, el títol del tauler i la petició. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class TempestaTest {
    @Test fun llegeixElsAgentsIElTitol() {
        val data = """{"agents":[{"index":0,"angle":"direct","label":"Direct","ca":"Directe","en":"Direct","model":"zen/deepseek-v4.1-flash","state":"done"},
            {"index":1,"angle":"sources","ca":"Fonts","en":"Sources","model":"glm-5.3-flash","state":"working"},
            {"index":2,"angle":"skeptic","ca":"Advocat del diable","en":"Devil's advocate","model":"deepseek-v4.1-flash","state":"failed"}]}"""
        val a = TempestaAgent.listOf(data)
        assertEquals(3, a.size)
        assertEquals("Directe", a[0].label("ca"))
        assertEquals("Sources", a[1].label("en"))
        assertEquals("deepseek-v4.1-flash", a[0].model)
        assertEquals("3 vents hi treballen alhora", tempestaHeadline("ca", a))
        assertEquals("Ajuntant el que han trobat", tempestaHeadline("ca", a.map { it.copy(state = "done") }))
        assertTrue(TempestaAgent.listOf("no és json").isEmpty())
    }

    @Test fun laPeticioPortaElFlagNomesSiCal() {
        assertTrue(WindRequests.chat("gregal", "hola", tempesta = true).contains("\"tempesta\":true"))
        assertFalse(WindRequests.chat("gregal", "hola").contains("tempesta"))
    }
}

class MdEscapeTest {
    @org.junit.Test fun asteriscEscapatEsLiteral() {
        val segs = parseInline("""~2,60 € \* i 2\*3""")
        val text = segs.filterIsInstance<Seg.Text>().joinToString("") { it.t }
        org.junit.Assert.assertEquals("~2,60 € * i 2*3", text)
        org.junit.Assert.assertTrue(segs.none { it is Seg.Italic })
    }
}
