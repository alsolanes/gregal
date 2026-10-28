package xyz.solanes.gregal

import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * Carpeta de treball: el que es llegeix de /api/workspaces i /api/dirs.
 *
 * Contractes (internal/web/workspace.go i internal/web/dirs.go):
 *  - GET  /api/workspaces → {current, project, branch, workspaces:[{path,name,last,missing}]}
 *  - POST /api/workspaces {path} → la mateixa forma, o 400 amb el motiu
 *  - GET  /api/dirs?path= → {path, name, git, parent, home, roots, dirs:[{name,path,git}], hidden, truncat}
 *
 * El cas que importa: la carpeta que ja no existeix ha de sortir marcada i no
 * s'ha de poder triar, i un camí llarg s'ha de veure sencer pel final (el
 * principi mossegat) perquè el que distingeix un projecte és la cua.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class CarpetaTest {

    private fun ws(raw: String) = parseWorkspaces(JSONObject(raw))
    private fun dirs(raw: String) = parseDirs(JSONObject(raw))

    @Test
    fun `workspaces llegeix la carpeta actual i la branca`() {
        val info = ws(
            """{"current":"/home/aleix/gregal-live","project":"gregal-live","branch":"main","workspaces":[]}""",
        )
        assertEquals("/home/aleix/gregal-live", info.current)
        assertEquals("gregal-live", info.project)
        assertEquals("main", info.branch)
        assertTrue(info.list.isEmpty())
    }

    @Test
    fun `un projecte desaparegut queda marcat i no es pot triar`() {
        val info = ws(
            """{"current":"/home/aleix/gregal-live","project":"gregal-live","branch":"main",
               "workspaces":[
                 {"path":"/home/aleix/gregal-live","name":"gregal-live","last":"2026-10-28T23:00:00Z"},
                 {"path":"/home/aleix/antic","name":"antic","last":"2026-09-01T10:00:00Z","missing":true}
               ]}""",
        )
        assertEquals(2, info.list.size)
        assertFalse(info.list[0].missing)
        assertTrue(info.list[1].missing)
        assertEquals("antic", info.list[1].name)
        // La data arriba en ISO: s'ha de poder ordenar, no quedar a zero.
        assertTrue(info.list[0].last > 0L)
        assertTrue(info.list[0].last > info.list[1].last)
    }

    @Test
    fun `un projecte sense name treu el nom del camí`() {
        val info = ws("""{"current":"/x","workspaces":[{"path":"/home/aleix/news-site/"}]}""")
        assertEquals("news-site", info.list.single().name)
    }

    @Test
    fun `dirs marca els repositoris i la carpeta de dalt`() {
        val d = dirs(
            """{"path":"/home/aleix","name":"aleix","git":false,"parent":"/home","home":"/home/aleix",
               "roots":["/"],"hidden":12,"truncat":false,
               "dirs":[{"name":"gregal-live","path":"/home/aleix/gregal-live","git":true},
                       {"name":"notes","path":"/home/aleix/notes"}]}""",
        )
        assertEquals("/home", d.parent)
        assertEquals("/home/aleix", d.home)
        assertEquals(listOf("/"), d.roots)
        assertEquals(12, d.hidden)
        assertEquals(2, d.dirs.size)
        assertTrue(d.dirs[0].git)
        assertFalse(d.dirs[1].git)
        assertEquals("gregal-live", d.dirs[0].name)
    }

    @Test
    fun `l'arrel del disc no té pare i el navegador ho ha de poder ensenyar`() {
        val d = dirs("""{"path":"/","parent":"","dirs":[]}""")
        assertEquals("", d.parent)
        assertFalse(d.git)
    }

    @Test
    fun `la llista tallada i les carpetes amagades es diuen`() {
        val d = dirs("""{"path":"/x","hidden":3,"truncat":true,"dirs":[]}""")
        assertTrue(d.truncated)
        assertEquals(3, d.hidden)
    }

    @Test
    fun `el camí de casa s'escriu amb titlla`() {
        assertEquals("~/gregal-live", camiCurt("/home/aleix/gregal-live"))
        assertEquals("~/gregal-live/android", camiCurt("/home/aleix/gregal-live/android"))
    }

    @Test
    fun `un camí llarg es mossega pel principi`() {
        val llarg = "/home/aleix/projectes/2026/client-molt-llarg/codi/font/principal/composables"
        val curt = camiCurt(llarg, maxim = 30)
        assertEquals(30, curt.length)
        assertTrue(curt.startsWith("…"))
        assertTrue(curt.endsWith("/composables"))
    }

    @Test
    fun `les inicials del projecte surten de les paraules`() {
        assertEquals("GL", inicialsProjecte("gregal-live"))
        assertEquals("NS", inicialsProjecte("news_site"))
        assertEquals("MA", inicialsProjecte("-marea-"))
        assertEquals("?", inicialsProjecte("   "))
    }

    @Test
    fun `l'error de feina en curs s'explica en català`() {
        val msg = missatgeCarpeta(
            "HTTP 400: stop the current run before changing its working folder, or open a new project",
            "ca",
        )
        assertTrue(msg.contains("feina en curs"))
        assertFalse(msg.contains("HTTP"))
        assertFalse(msg.contains("working folder"))
    }

    @Test
    fun `una carpeta que no existeix es diu clara`() {
        val msg = missatgeCarpeta("HTTP 400: no existeix: /home/aleix/inventat", "ca")
        assertEquals("Aquesta carpeta no existeix al servidor.", msg)
    }

    @Test
    fun `un error desconegut no es menja`() {
        assertEquals("HTTP 500: disc ple", missatgeCarpeta("HTTP 500: disc ple", "ca"))
        assertEquals("No s'ha pogut canviar la carpeta.", missatgeCarpeta("", "ca"))
    }
}
