package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** Vista prèvia al xat: mateixes regles que md-preview.js i md.js (gregal-web). */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class WebPreviewTest {
    @Test fun nomesHtmlISvgSencers() {
        assertEquals("html", WebPreviewLogic.kind("html", "<!doctype html><html><body><h1>Hola</h1></body></html>"))
        assertEquals("svg", WebPreviewLogic.kind("svg", "<svg viewBox=\"0 0 10 10\"><circle r=\"4\"/></svg>"))
        assertEquals("", WebPreviewLogic.kind("svg", "<svg viewBox=\"0 0 10 10\">"))
        assertEquals("", WebPreviewLogic.kind("kotlin", "<div></div>"))
        assertEquals("", WebPreviewLogic.kind("html", "només text"))
    }

    @Test fun elDocumentNoPotSortirALaXarxa() {
        for ((k, c) in listOf("html" to "<p>hola</p>", "html" to "<html><head><title>t</title></head><body></body></html>", "svg" to "<svg></svg>")) {
            val doc = WebPreviewLogic.srcdoc(k, c)
            assertTrue(doc.contains("connect-src 'none'"))
            val t = doc.indexOf("<title>")
            if (t >= 0) assertTrue(doc.indexOf("Content-Security-Policy") < t)
        }
    }

    @Test fun laTargetaNomesAcceptaAdrecesDePrevisualitzacio() {
        val ok = parseWebSpec("""{"title":"Calculadora propines","path":"projectes/c/index.html","url":"/p/4c606c7157c70716/projectes/c/index.html"}""")
        assertEquals("Calculadora propines", ok?.title)
        assertNull(parseWebSpec("""{"title":"x","url":"https://dolent.cat/x"}"""))
        assertNull(parseWebSpec("""{"title":"x","url":"/api/winds/doc?name=a"}"""))
        val rich = groupRich(splitMd("Ja la tens:\n\n```gregal-web\n{\"title\":\"Joc\",\"path\":\"projectes/joc/index.html\",\"url\":\"/p/0123456789abcdef/projectes/joc/index.html\"}\n```"))
        assertTrue(rich.any { it is Rich.Card && it.card is RichCard.Web })
    }
}
