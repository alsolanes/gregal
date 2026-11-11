package xyz.solanes.gregal

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** «Webs i apps»: el que els vents publiquen, el que corre al servidor i els túnels. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class VentsWebsTest {
    private val base = "http://192.168.1.62:8097"

    @Test fun foldsSitesAppsAndTunnelsIntoOneList() {
        val shares = JSONObject("""{"shares":[
            {"id":"aaa","title":"Blog","kind":"site","url":"http://192.168.1.62:8097/share/aaa/","files":4},
            {"id":"bbb","title":"Una conversa","url":"/share/bbb/"}]}""")
        val apps = JSONObject("""{"apps":[{"id":"x1","name":"joc","status":"running","command":"node server.js","path":"/apps/x1/"}]}""")
        val powers = JSONObject("""{"tunnels":[{"name":"blog","url":"https://blog.trycloudflare.com","target":"http://localhost:8097","status":"open"}]}""")
        val items = parseWebs(base, shares, apps, powers)
        // Una conversa compartida (sense kind «site») no és una web publicada.
        assertEquals(listOf("site", "app", "tunnel"), items.map { it.kind })
        assertEquals("Blog", items[0].name)
        assertEquals("published", items[0].status)
        // Un camí relatiu es completa amb l'adreça del servidor.
        assertEquals("$base/apps/x1/", items[1].url)
        assertEquals("running", items[1].status)
        assertEquals("https://blog.trycloudflare.com", items[2].url)
    }

    @Test fun anyPartMayBeMissing() {
        // Un compte que no és administrador no té túnels (403): la resta es veu igual.
        assertTrue(parseWebs(base, null, null, null).isEmpty())
        val only = parseWebs(base, null, JSONObject("""{"apps":[{"id":"a","name":"","status":""}]}"""), null)
        assertEquals("a", only.single().name)
        assertEquals("stopped", only.single().status)
    }

    @Test fun textsExistInBothLanguages() {
        for (k in listOf("title", "sites", "apps", "tunnels", "open", "copy", "unpublish", "restart", "stop", "remove", "empty")) {
            assertTrue(k, webCopy("ca", k) != k && webCopy("en", k) != k)
        }
    }

    // B11: webs amb nom, servides per l'amfitrió (també amb l'ordinador aturat).
    @Test fun parsesNamedSites() {
        val o = JSONObject("""{"domain":"example.org","canPublish":true,"sites":[
            {"name":"aliments","host":"aliments.example.org","url":"https://aliments.example.org/","visibility":"public","version":4,"state":"live","canUndo":true},
            {"name":"casa","host":"/w/casa/","url":"","path":"/w/casa/","visibility":"lan","version":2,"state":"check_failed",
             "check":{"ok":false,"problems":["broken link falta.png (HTTP 404)"]}},
            {"name":"","visibility":"public"},
            {"name":"rara","visibility":"altre","state":"?"}]}""")
        val n = parseNamedWebs(base, o)
        assertTrue(n.canPublish)
        assertEquals("example.org", n.domain)
        assertEquals(listOf("aliments", "casa", "rara"), n.sites.map { it.name })
        assertEquals("https://aliments.example.org/", n.sites[0].url)
        assertEquals(4, n.sites[0].version)
        assertTrue(n.sites[0].canUndo)
        // Sense domini: el camí /w/<nom>/ es completa amb l'adreça del servidor.
        assertEquals("$base/w/casa/", n.sites[1].url)
        assertEquals(listOf("broken link falta.png (HTTP 404)"), n.sites[1].problems)
        // Valors desconeguts: el més prudent.
        assertEquals("private", n.sites[2].visibility)
        assertEquals("draft", n.sites[2].state)
        assertTrue(parseNamedWebs(base, null).sites.isEmpty())
    }

    @Test fun namedSiteTextsInBothLanguages() {
        val keys = listOf("webs.list", "webs.version", "webs.open", "webs.share", "webs.undo", "webs.error", "webs.public.confirm") +
            WEB_VISIBILITIES.map { "webs.vis.$it" } + listOf("live", "draft", "check_failed").map { "webs.state.$it" }
        for (k in keys) {
            assertTrue(k, tr("ca", k) != k && tr("en", k) != k && tr("ca", k) != tr("en", k) || k == "webs.version")
        }
    }
}
