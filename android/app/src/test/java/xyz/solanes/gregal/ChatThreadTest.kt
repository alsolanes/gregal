package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test
import java.util.TimeZone

/** Les regles pures del fil de l'assistent (B1). */
class ChatThreadTest {

    // ─── Bombolla o superfície de lectura ────────────────────────────────

    @Test fun `resposta curta va en bombolla`() {
        assertEquals(ChatSurface.BUBBLE, chatSurfaceFor("Fet! T'he deixat l'itinerari a la carpeta."))
    }

    @Test fun `resposta amb llista i negreta continua en bombolla`() {
        assertEquals(ChatSurface.BUBBLE, chatSurfaceFor("Tres opcions:\n- **Bus** directe\n- Tren\n- Taxi"))
    }

    @Test fun `una taula va a la superficie de lectura`() {
        val t = "Preus:\n\n| Hotel | €/nit |\n|---|---|\n| Mar | 90 |\n| Port | 110 |\n"
        assertEquals(ChatSurface.READING, chatSurfaceFor(t))
    }

    @Test fun `un bloc de codi va a la superficie de lectura`() {
        assertEquals(ChatSurface.READING, chatSurfaceFor("Prova això:\n\n```kotlin\nval x = 1\n```\n"))
    }

    @Test fun `mes de 900 caracters va a la superficie de lectura`() {
        assertEquals(ChatSurface.BUBBLE, chatSurfaceFor("a".repeat(READING_CHARS)))
        assertEquals(ChatSurface.READING, chatSurfaceFor("a".repeat(READING_CHARS + 1)))
    }

    @Test fun `els espais del voltant no compten`() {
        assertEquals(ChatSurface.BUBBLE, chatSurfaceFor("  hola  " + " ".repeat(2000)))
    }

    @Test fun `la regla es la mateixa en directe que acabada`() {
        val t = "| a | b |\n|---|---|\n| 1 | 2 |\n"
        assertEquals(chatSurfaceFor(t, richBlocks(t)), chatSurfaceFor(t, richBlocksLive(t)))
    }

    // ─── Claus de la llista ──────────────────────────────────────────────

    @Test fun `la clau es l'id i nomes l'index si no n'hi ha`() {
        assertEquals("m:h-abc", threadItemKey("h-abc", 3))
        assertEquals("i:3", threadItemKey("", 3))
    }

    @Test fun `la clau del Ventet no depen de la posicio`() {
        val a = listOf(CapMsg("user", "hola", id = "m-1"), CapMsg("cap", "bon dia", id = "m-2"))
        val b = listOf(CapMsg("cap", "abans", id = "m-0")) + a
        assertEquals(capItemKey(a, 1), capItemKey(b, 2))
    }

    @Test fun `la resposta en directe i l'acabada fan servir claus del fil`() {
        val hist = listOf(CapMsg("user", "hola", id = "m-1"), CapMsg("cap", "bon dia", id = "m-2"))
        val done = GregalViewModel.CapUi(state = CapState(history = hist))
        assertEquals("m:m-2", capReplyKey(done))
        val live = done.copy(busy = true, live = "escric…")
        assertEquals(CAP_KEY_LIVE, capReplyKey(live))
        assertNull(capReplyKey(done.copy(busy = true)))
    }

    // ─── Ratxes i separadors ─────────────────────────────────────────────

    private val t0 = 1_762_850_000L // segons Unix

    @Test fun `missatges seguits del mateix parlant fan ratxa`() {
        val s = threadSlots(listOf(
            ThreadEntry("cap", t0), ThreadEntry("cap", t0 + 5), ThreadEntry("user", t0 + 60), ThreadEntry("cap", t0 + 70),
        ))
        assertEquals(listOf(true, false, true, true), s.map { it.firstOfRun })
        assertEquals(listOf(false, true, true, true), s.map { it.lastOfRun })
    }

    @Test fun `el separador surt al primer missatge i despres de mitja hora`() {
        val s = threadSlots(listOf(
            ThreadEntry("cap", t0), ThreadEntry("cap", t0 + 29 * 60), ThreadEntry("cap", t0 + 29 * 60 + 31 * 60),
        ))
        assertEquals(t0 * 1000, s[0].separatorAt)
        assertNull(s[1].separatorAt)
        assertNotNull(s[2].separatorAt)
        // El separador trenca la ratxa: cua a l'anterior i aire a sobre.
        assertTrue(s[1].lastOfRun)
        assertTrue(s[2].firstOfRun)
    }

    @Test fun `missatges sense hora no posen separador`() {
        val s = threadSlots(listOf(ThreadEntry("cap", 0), ThreadEntry("user", 0), ThreadEntry("user", 0)))
        assertTrue(s.all { it.separatorAt == null })
        assertEquals(listOf(true, true, false), s.map { it.firstOfRun })
    }

    @Test fun `hora en millis i en segons es la mateixa`() {
        assertEquals(t0 * 1000, tsToMillis(t0))
        assertEquals(t0 * 1000, tsToMillis(t0 * 1000))
        assertEquals(0L, tsToMillis(0))
    }

    @Test fun `etiqueta del separador avui, ahir i altres dies`() {
        val utc = TimeZone.getTimeZone("UTC")
        val now = 1_762_854_060_000L // 2025-11-11 09:41 UTC
        assertEquals("Avui · 09:41", threadSeparatorLabel("ca", now, now, utc))
        assertEquals("Today · 09:41", threadSeparatorLabel("en", now, now, utc))
        assertEquals("Ahir · 09:41", threadSeparatorLabel("ca", now - 86_400_000L, now, utc))
        val old = threadSeparatorLabel("ca", now - 5 * 86_400_000L, now, utc)
        assertTrue(old, old.startsWith("6 ") && old.endsWith(" · 09:41"))
    }

    // ─── Separador «Una novetat per a tu» ────────────────────────────────

    @Test fun `la novetat va davant del primer missatge despres de l'ultim vist`() {
        val ids = listOf("a", "b", "c", "d")
        val from = listOf("user", "cap", "cap", "cap")
        assertEquals(2, firstUnreadIndex(ids, from, "b"))
    }

    @Test fun `sense marca o amb marca desconeguda no hi ha separador`() {
        val ids = listOf("a", "b")
        val from = listOf("user", "cap")
        assertNull(firstUnreadIndex(ids, from, ""))
        assertNull(firstUnreadIndex(ids, from, "zz"))
    }

    @Test fun `si tot ja s'havia vist no hi ha separador`() {
        assertNull(firstUnreadIndex(listOf("a", "b"), listOf("user", "cap"), "b"))
    }

    @Test fun `un missatge teu despres de la marca vol dir que hi eres`() {
        assertNull(firstUnreadIndex(listOf("a", "b", "c"), listOf("cap", "user", "cap"), "a"))
    }

    @Test fun `el missatge proactiu del contracte es una novetat`() {
        val fixture = java.io.File("../../docs/assistant-contract/fixtures/missatge-proactiu.json")
        if (!fixture.exists()) return
        // org.json no és disponible als tests purs: els dos camps, amb una expressió.
        val raw = fixture.readText()
        fun field(k: String) = Regex("\"" + k + """"\s*:\s*"([^"]*)"""").find(raw)!!.groupValues[1]
        val hist = listOf(CapMsg("user", "Prepara'm el viatge", id = "m-1"), CapMsg("cap", "Hi poso", id = "m-2"),
            CapMsg("cap", field("text"), id = field("id")))
        assertEquals(2, firstUnreadIndex(hist.map { it.id }, hist.map { it.from }, "m-2"))
        assertEquals(ChatSurface.BUBBLE, chatSurfaceFor(hist[2].text))
    }

    // ─── Resultats ───────────────────────────────────────────────────────

    @Test fun `el tipus del resultat surt del nom o de l'adreca`() {
        assertEquals(ResultKind.DOCUMENT, resultKindOf("itinerari-v2.pdf"))
        assertEquals(ResultKind.SHEET, resultKindOf("pressupost.XLSX"))
        assertEquals(ResultKind.IMAGE, resultKindOf("foto.jpeg"))
        assertEquals(ResultKind.WEB, resultKindOf("https://example.test/a"))
        assertEquals(ResultKind.CHAT, resultKindOf("wind:xaloc"))
        assertEquals(ResultKind.CHAT, resultKindOf("rot:r1"))
        assertEquals(ResultKind.FILE, resultKindOf("res-itinerari-v2"))
    }

    @Test fun `la presentacio porta tres suggeriments en tots dos idiomes`() {
        for (lang in listOf("ca", "en")) {
            val s = openingSuggestions(lang)
            assertEquals(3, s.size)
            assertTrue(s.none { it.startsWith("suggest") })
        }
        assertTrue(threadCopy("ca", "intro").format("Gregal").contains("Gregal"))
    }
}
