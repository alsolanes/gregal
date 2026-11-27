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

    @Test fun cadaAgentPortaLaDescripcioEnLaLlenguaDeLApp() {
        val data = """{"agents":[{"index":0,"angle":"direct","ca":"Directe","en":"Direct","desc_ca":"Respon pas a pas.","desc_en":"Answers step by step.","model":"m","state":"working"}]}"""
        val a = TempestaAgent.listOf(data).single()
        assertEquals("Respon pas a pas.", a.desc("ca"))
        assertEquals("Answers step by step.", a.desc("en"))
    }

    @Test fun unServidorVellSenseDescripcioSegueixFuncionant() {
        val data = """{"agents":[{"index":0,"angle":"direct","ca":"Directe","en":"Direct","model":"m","state":"done"}]}"""
        val a = TempestaAgent.listOf(data).single()
        assertEquals("Directe", a.label("ca"))
        assertEquals("done", a.state)
        assertEquals("", a.desc("ca"))
        assertEquals("", a.desc("en"))
    }

    @Test fun elsModelsDAFonsEsLlegeixenIEsDesen() {
        val m = TempestaModels.parse("""{"models":{"direct":"zen/deepseek-v4.1-flash","sources":"","skeptic":"glm/glm-5.3-flash"}}""")
        assertEquals(mapOf("direct" to "zen/deepseek-v4.1-flash", "sources" to "", "skeptic" to "glm/glm-5.3-flash", "synth" to ""), m)
        val p = org.json.JSONObject(TempestaModels.payload("skeptic", ""))
        assertEquals("skeptic", p.getString("slot"))
        assertEquals("", p.getString("model"))
        assertEquals("zen/x", org.json.JSONObject(TempestaModels.payload("synth", "zen/x")).getString("model"))
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

class DrawerWindsTest {
    private fun w(id: String) = Wind(id, id.replaceFirstChar { it.uppercase() }, "#8193FF", "")

    @org.junit.Test fun gregalAPartICincRecentsPrimer() {
        val winds = listOf(w("gregal"), w("a"), w("b"), w("c"), w("d"), w("e"), w("f"), w("g"))
        val chats = mapOf(
            "f" to listOf(GregalViewModel.PetMsg("pet", "x", ts = 300L)),
            "c" to listOf(GregalViewModel.PetMsg("pet", "x", ts = 200L)),
            "gregal" to listOf(GregalViewModel.PetMsg("pet", "x", ts = 900L)),
        )
        val got = drawerRecentWinds(winds, chats).map { it.id }
        org.junit.Assert.assertEquals(listOf("f", "c", "a", "b", "d"), got)
    }
}

@org.junit.runner.RunWith(org.robolectric.RobolectricTestRunner::class)
@org.robolectric.annotation.Config(sdk = [34])
class TidyLogicTest {
    @org.junit.Test fun llegeixElsDuplicatsIElQueTeCadascun() {
        val raw = """{"total":11,"groups":[{"reason":"same_name","keep":"w-1","winds":[{"id":"w-1","name":"Marinada","role":"Compres","msgs":7,"threads":2,"memory":30,"knowledge":0,"jobs":0},{"id":"marinada","name":"Marinada","role":"calma","msgs":0,"threads":0,"memory":0,"knowledge":0,"jobs":0}]}],"test":[{"id":"fdsaf","name":"fdsaf","msgs":0}],"unused":[]}"""
        val r = TidyLogic.parse(raw)
        org.junit.Assert.assertEquals(2, r.issues)
        org.junit.Assert.assertEquals("w-1", r.groups.single().keep)
        org.junit.Assert.assertEquals("Compres · 7 missatges · 2 converses · amb memòria", TidyLogic.line("ca", r.groups[0].winds[0]))
        org.junit.Assert.assertEquals("calma · sense ús", TidyLogic.line("ca", r.groups[0].winds[1]))
    }
}
