package xyz.solanes.gregal

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * L'eina preview de l'agent obre la previsualització també al mòbil: abans
 * l'event s'ignorava i l'usuari no veia res.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class PreviewTest {
    @get:Rule val ui = createComposeRule()

    @Test fun llegeixLaSortidaDeLEina() {
        val r = parsePreviewRequest("""{"kind":"html","content":"<h1>Hola</h1>","title":"Web"}""")
        assertEquals(PreviewReq("html", "<h1>Hola</h1>", "Web"), r)
        assertNull(parsePreviewRequest("""{"kind":"html","content":"<h1>Hola</h1>""" + "…"))
        assertNull(parsePreviewRequest("""{"kind":"exe","content":"x"}"""))
        assertNull(parsePreviewRequest("""{"kind":"html","content":"  "}"""))
        assertNull(parsePreviewRequest("ERROR: preview content must contain 1 to 200000 bytes"))
    }

    @Test fun localhostVaAlServidor() {
        assertEquals("http://192.168.1.20:5173/app?x=1#a", previewUrlFor("http://localhost:5173/app?x=1#a", "192.168.1.20", false))
        assertEquals("http://192.168.1.20:3000/", previewUrlFor("http://127.0.0.1:3000/", "http://192.168.1.20:8097", false))
        assertEquals("http://vents.exemple.cat:3000/", previewUrlFor("http://localhost:3000/", "https://vents.exemple.cat", false))
        // Amb el servidor al mateix mòbil, localhost ja és bo.
        assertEquals("http://localhost:3000/", previewUrlFor("http://localhost:3000/", "127.0.0.1", true))
        assertNull(previewUrlFor("http://example.com/", "192.168.1.20", false))
        assertNull(previewUrlFor("http://user:pw@localhost:3000/", "192.168.1.20", false))
        assertNull(previewUrlFor("file:///etc/passwd", "192.168.1.20", false))
    }

    private fun event(vm: GregalViewModel, kind: String, o: JSONObject) {
        val m = GregalViewModel::class.java.getDeclaredMethod("onEvent", String::class.java, JSONObject::class.java, String::class.java)
        m.isAccessible = true
        m.invoke(vm, kind, o, "")
    }

    @Test fun elResultatDeLEinaObreLaPreview() {
        val vm = GregalViewModel(ApplicationProvider.getApplicationContext())
        val html = "<!doctype html><h1>Web</h1>" + "x".repeat(5000)
        val args = JSONObject().put("kind", "html").put("content", html).put("title", "Botiga")
        event(vm, "tool_call", JSONObject().put("id", "c1").put("name", "preview").put("args", args))
        assertNull(vm.preview.value)
        // Servidor antic: el registre durable retalla la sortida a 3000 caràcters.
        event(vm, "tool_result", JSONObject().put("id", "c1").put("name", "preview").put("output", args.toString().take(3000) + "…"))
        assertEquals(PreviewReq("html", html, "Botiga"), vm.preview.value)
        assertTrue(vm.previewOpen.value)
        vm.showPreview(false)
        assertFalse(vm.previewOpen.value)
        // Una preview que falla no obre res.
        vm.clearPreview()
        event(vm, "tool_call", JSONObject().put("id", "c2").put("name", "preview").put("args", args))
        event(vm, "tool_result", JSONObject().put("id", "c2").put("name", "preview").put("output", "ERROR: preview URL must use a localhost HTTP endpoint"))
        assertNull(vm.preview.value)
    }

    @Test fun pintaMarkdownIAdrecaNoAccessible() {
        var req by mutableStateOf(PreviewReq("markdown", "# Pla\n\nPrimer pas", "Pla de treball"))
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    androidx.compose.material3.Surface(Modifier.fillMaxSize()) {
                        PreviewSheet(req, Conn("192.168.1.20", "8097", "t"), "en") {}
                    }
                }
            }
        }
        ui.onNodeWithText("Pla de treball").assertExists()
        ui.onNodeWithText("Primer pas", substring = true).assertExists()
        req = PreviewReq("url", "http://example.com/")
        ui.waitForIdle()
        ui.onNodeWithText("Can't load", substring = true).assertExists()
        req = PreviewReq("url", "http://localhost:5173/")
        ui.waitForIdle()
        ui.onNodeWithText("http://192.168.1.20:5173/").assertExists()
        ui.onNodeWithText("Open in browser").assertExists()
    }

    @Test fun lEnllacPublicatIOnFunciona() {
        val id = "a".repeat(40)
        val lan = parsePublished("""{"id":"$id","path":"/share/$id/","url":"","public":false}""", "http://192.168.1.20:8097")!!
        assertEquals("http://192.168.1.20:8097/share/$id/", lan.link)
        assertEquals("network", publishedReach(lan, Conn("192.168.1.20", "8097", "t")))
        assertEquals("phone", publishedReach(lan, Conn("127.0.0.1", "8097", "t", local = true)))
        // Connectat per domini, l'enllaç ja és d'internet.
        assertEquals("", publishedReach(lan, Conn("https://vents.exemple.cat", "", "t")))
        val pub = parsePublished("""{"id":"$id","path":"/share/$id/","url":"https://vents.exemple.cat/share/$id/","public":true}""", "http://192.168.1.20:8097")!!
        assertEquals("https://vents.exemple.cat/share/$id/", pub.link)
        assertEquals("", publishedReach(pub, Conn("192.168.1.20", "8097", "t")))
        assertNull(parsePublished("""{"error":"x"}""", "http://h"))
    }

    @Test fun elDialegDeLEnllac() {
        val id = "b".repeat(40)
        val site = PublishedSite(id, "http://192.168.1.20:8097/share/$id/", false)
        var unpublished = false
        ui.setContent {
            CompositionLocalProvider(LocalPal provides LightPal) {
                MaterialTheme(colorScheme = LightPal.scheme()) {
                    PreviewSheet(PreviewReq("html", "<h1>x</h1>", "Forn"), Conn("192.168.1.20", "8097", "t"), "en", site, onPublish = {}) {}
                    PublishDialog(PublishState(site = site, dialog = true), Conn("192.168.1.20", "8097", "t"), "en",
                        onRepublish = {}, onUnpublish = { unpublished = true }, onClose = {})
                }
            }
        }
        ui.onNodeWithText("Link").assertExists()
        ui.onNodeWithText(site.link).assertExists()
        ui.onNodeWithText("your home network", substring = true).assertExists()
        ui.onNodeWithText("Unpublish").performClick()
        assertTrue(unpublished)
    }
}
