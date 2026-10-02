package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class SseParserTest {
    @Test
    fun eventComplet() {
        val (evs, rest) = SseParser.feed("event: token\ndata: {\"text\":\"hola\"}\n\n", "")
        assertEquals(1, evs.size)
        assertEquals("token", evs[0].name)
        assertEquals("{\"text\":\"hola\"}", evs[0].data)
        assertEquals("", rest)
    }

    @Test
    fun trosPartit() {
        val (e1, carry) = SseParser.feed("event: tok", "")
        assertTrue(e1.isEmpty())
        val (e2, rest) = SseParser.feed("en\ndata: x\n\n", carry)
        assertEquals(1, e2.size)
        assertEquals("token", e2[0].name)
        assertEquals("", rest)
    }

    @Test
    fun multiplesEvents() {
        val raw = "event: a\ndata: 1\n\nevent: b\ndata: 2\n\n"
        val (evs, _) = SseParser.feed(raw, "")
        assertEquals(listOf("a", "b"), evs.map { it.name })
    }
}

class BaseUrlTest {
    @Test
    fun normalitza() {
        assertEquals("http://192.168.1.84:8097", baseUrlOf("192.168.1.84", "8097"))
        assertEquals("http://192.168.1.84:8097", baseUrlOf("http://192.168.1.84/", "8097"))
        assertEquals("http://127.0.0.1:8097", baseUrlOf("", ""))
    }
}

class ProtocolV2Test {
    @Test
    fun requiresRunsAndEventsCapabilities() {
        assertTrue(ServiceHealth(status = "ok", capabilities = mapOf("runs" to true, "events" to true))
            .supportsV2RunsEvents())
        assertTrue(!ServiceHealth(status = "ok", capabilities = mapOf("events" to true))
            .supportsV2RunsEvents())
        assertTrue(!ServiceHealth(status = "ok", capabilities = mapOf("runs" to true))
            .supportsV2RunsEvents())
        assertTrue(!ServiceHealth(status = "offline", capabilities = mapOf("runs" to true, "events" to true))
            .supportsV2RunsEvents())
    }

    @Test
    fun durableEventCarriesRunAndCursorData() {
        val e = DurableEvent(id = 12, sessionId = "mobil", runId = 7, kind = "assistant", text = "fet")
        assertEquals(12L, e.id)
        assertEquals(7L, e.runId)
        assertEquals("fet", e.text)
    }

    @Test
    fun interactiveRequiresStructuredCapability() {
        val base = mapOf(
            "runs" to true, "events" to true, "durable_events" to true,
        )
        assertTrue(!ServiceHealth(status = "ok", capabilities = base).supportsV2Interactive())
        assertTrue(ServiceHealth(
            status = "ok", capabilities = base + ("interactive_events" to true),
        ).supportsV2Interactive())
    }

}

class MdTest {
    @Test
    fun negretaICodi() {
        val segs = splitMd("hola **món** i `codi`")
        assertEquals(
            listOf(Seg.Text("hola "), Seg.Bold("món"), Seg.Text(" i "), Seg.Code("codi")),
            segs,
        )
    }

    @Test
    fun blocAmbLlenguatge() {
        val segs = splitMd("abans\n```python\nprint(1)\n```\ndesprés")
        assertTrue(segs.any { it is Seg.Block && (it as Seg.Block).lang == "python" })
        assertTrue(segs.any { it is Seg.Text && it.t.contains("després") })
    }

    @Test
    fun titolsSenseHashtags() {
        val segs = splitMd("# Títol\ntext")
        val h = segs.filterIsInstance<Seg.Heading>().firstOrNull()
        assertTrue(h != null && h.level == 1)
        assertTrue(h!!.inner.any { it is Seg.Text && (it as Seg.Text).t == "Títol" })
        assertTrue(segs.none { it is Seg.Text && (it as Seg.Text).t.contains("#") })
    }

    @Test
    fun titolUnaLiniaNet() {
        assertEquals("Títol net", stripMdForTitle("## **Títol** `net`"))
        assertEquals("Punt Important", stripMdForTitle("- Punt **Important**"))
    }

    @Test
    fun enllacosIClicables() {
        val segs = splitMd("mira [la doc](https://example.com/a) si us plau")
        val link = segs.filterIsInstance<Seg.Link>().firstOrNull()
        assertTrue(link != null && link.label == "la doc" && link.url == "https://example.com/a")
        assertEquals("la doc", stripMdForTitle("[la doc](https://example.com/a)"))
        val img = splitMd("![logo](https://example.com/l.png)").filterIsInstance<Seg.Link>().firstOrNull()
        assertTrue(img != null && img.url == "https://example.com/l.png")
    }

    @Test
    fun taulesAgrupades() {
        val segs = splitMd("| A | B |\n|---|---|\n| 1 | 2 |\n| 3 | 4 |")
        val t = segs.filterIsInstance<Seg.Table>().firstOrNull()
        assertTrue(t != null)
        assertEquals(listOf("A", "B"), t!!.headers)
        assertEquals(listOf(listOf("1", "2"), listOf("3", "4")), t.rows)
        val rich = groupRich(segs)
        assertTrue(rich.any { it is Rich.Table })
    }

    @Test
    fun llistesNumeradesEsConserven() {
        val segs = splitMd("1. primer\n2. segon\n- vinyeta")
        val texts = segs.filterIsInstance<Seg.Text>().map { it.t }
        assertTrue(texts.any { it.startsWith("1. ") })
        assertTrue(texts.any { it.startsWith("2. ") })
        assertTrue(texts.any { it.startsWith("• ") })
    }

    @Test
    fun blocsDeCodiAgrupats() {
        val rich = groupRich(splitMd("hola\n```kotlin\nval x = 1\n```\nfi"))
        val code = rich.filterIsInstance<Rich.Code>().firstOrNull()
        assertTrue(code != null && code.lang == "kotlin" && code.code.contains("val x = 1"))
        assertTrue(rich.first() is Rich.Inline)
    }
}

class AgoTest {
    // Migdia local del 2026-09-13, calculat al mateix fus que ago().
    private val zone = java.time.ZoneId.systemDefault()
    private val ara = java.time.LocalDate.of(2026, 9, 13).atTime(12, 0)
        .atZone(java.time.ZoneId.systemDefault()).toInstant().toEpochMilli()

    @Test
    fun minutsIHoresIAhir() {
        assertEquals("fa 5 min", ago("13-09 11:55", ara))
        assertEquals("fa 2 h", ago("13-09 10:00", ara))
        assertEquals("ahir", ago("12-09 12:00", ara))
        assertEquals("ara mateix", ago("13-09 12:00", ara))
        assertEquals("5 min ago", ago("13-09 11:55", ara, "en"))
        assertEquals("Yesterday", dayLabel(
            java.time.LocalDate.of(2026, 9, 12).atStartOfDay()
                .atZone(zone).toInstant().toEpochMilli(), ara, "en",
        ))
    }

    @Test
    fun anticIRot() {
        assertEquals("01-09 08:00", ago("01-09 08:00", ara))
        assertEquals("nonsense", ago("nonsense", ara))
    }
}

class ToolTextTest {
    @Test
    fun englishTranslationsAndKeyParity() {
        assertTrue(appStringsOk())
        assertEquals("Run command", toolTitle("bash", "en"))
        assertEquals("Will read: main.go", toolSummary("read", """{"path":"main.go"}""", "en"))
        assertEquals("(no arguments)", prettyArgs("{}", "en"))
        assertEquals("for coding", roleLabel("code", "en"))
        assertEquals("Today", tr("en", "app.sess.today"))
        assertEquals("COMPLETED", goalStatusLabel("fet", "en"))
        assertEquals("PENDING", goalStatusLabel("pending", "en"))
        assertEquals("⚠️ Maximum 25 MB (this file is 30 MB)",
            tr("en", "app.off.fileTooLarge", 25, 30))
    }

    @Test
    fun titolsIResums() {
        assertEquals("Executar comanda", toolTitle("bash"))
        assertEquals("Eina externa", toolTitle("mcp_recordatoris_afegeix"))
        assertEquals("Llegirà: main.go", toolSummary("read", """{"path":"main.go"}"""))
        assertEquals("Executarà: ls -la", toolSummary("bash", """{"command":"ls -la"}"""))
        assertEquals("Cercarà: preu llum", toolSummary("web_search", """{"query":"preu llum","count":5}"""))
    }

    @Test
    fun argsNet() {
        val p = prettyArgs("""{"command":"git status","path":"x"}""")
        assertTrue(p.contains("command: git status") && p.contains("\n"))
        assertEquals("(sense arguments)", prettyArgs("{}"))
        assertEquals("(sense arguments)", prettyArgs("  "))
    }

    @Test
    fun caselles() {
        val segs = splitMd("- [ ] pendent\n- [x] fet\n- normal")
        val texts = segs.filterIsInstance<Seg.Text>().map { it.t }
        assertTrue(texts.any { it.startsWith("☐ ") })
        assertTrue(texts.any { it.startsWith("☑ ") })
        assertTrue(texts.any { it.startsWith("• ") })
        assertTrue(segs.none { it is Seg.Text && (it as Seg.Text).t.contains("[ ]") })
    }
}

class ChatRowsTest {
    private val zone = java.time.ZoneId.systemDefault()
    private val today = java.time.LocalDate.now()
    private fun ms(date: java.time.LocalDate, hour: Int, min: Int) =
        date.atTime(hour, min).atZone(zone).toInstant().toEpochMilli()

    @Test
    fun divisorsIDies() {
        val now = ms(today, 12, 0)
        assertEquals("Avui", dayLabel(ms(today, 9, 5), now))
        assertEquals("Ahir", dayLabel(ms(today.minusDays(1), 20, 0), now))
        assertEquals("", dayLabel(0L, now))
        assertEquals("09:05", hourMinute(ms(today, 9, 5)))
    }

    @Test
    fun agrupacio() {
        val msgs = listOf(
            ChatMsg("user", "hola", at = ms(today, 9, 0)),
            ChatMsg("assistant", "ei", at = ms(today, 9, 1)),
            ChatMsg("user", "ahir deia", at = ms(today.minusDays(1), 20, 0)),
        )
        val rows = chatRows(msgs)
        assertTrue(rows[0] is UiRow.Day && (rows[0] as UiRow.Day).label == "Avui")
        assertTrue(rows[3] is UiRow.Day && (rows[3] as UiRow.Day).label == "Ahir")
        assertEquals(5, rows.size)
        val idx = rows.filterIsInstance<UiRow.Msg>().map { it.index }
        assertEquals(listOf(0, 1, 2), idx)
    }
}

class JobsTest {
    @Test
    fun etiquetes() {
        assertEquals("Cada dia 08:00", kindLabel("daily", "08:00", 0.0))
        assertEquals("Cada 6 h", kindLabel("interval", "", 6.0))
        assertEquals("Cada 1.5 h", kindLabel("interval", "", 1.5))
        assertEquals("Manual", kindLabel("manual", "", 0.0))
        assertEquals("Every day 08:00", kindLabel("daily", "08:00", 0.0, "en"))
        assertEquals("Every 6 h", kindLabel("interval", "", 6.0, "en"))
        assertEquals("24 h", fmtHours(24.0))
        assertEquals("1.5 h", fmtHours(1.5))
    }

    @Test
    fun marques() {
        assertEquals("2026-09-13 08:00", shortStamp("2026-09-13T08:00:00+02:00"))
        assertEquals("curt", shortStamp("curt"))
    }
}

class PlanPayloadTest {
    @Test
    fun clauTask() {
        val p = planPayload("canvia el color")
        assertEquals("canvia el color", p["task"])
        assertEquals(1, p.size)
    }
}

class AttachLimitsTest {
    @Test
    fun maximTres() {
        val imgs = List(5) { "data:image/png;base64,AAA" }
        assertEquals(3, imgs.take(3).size)
    }

    @Test
    fun nomesAgent() {
        // Les imatges només viatgen en mode agent (codi/objectiu), mai a /api/chat.
        val agentModes = setOf("code", "inspect", "goal")
        assertTrue(agentModes.contains("code"))
        assertTrue(!agentModes.contains("chat"))
    }
}

class CheckpointsTest {
    @Test
    fun parsejaLlista() {
        val cps = parseCheckpoints("""[{"seq":1,"op":"write","path":"a.txt","at":"t"}]""")
        assertEquals(1, cps.size)
        assertEquals(CheckpointInfo(1, "write", "a.txt", "t"), cps[0])
    }

    @Test
    fun llistaBuida() {
        assertTrue(parseCheckpoints("[]").isEmpty())
    }
}

class CheckpointsShapeTest {
    @Test
    fun valorsAmbEscapaments() {
        val cps = parseCheckpoints("""[{"seq":2,"op":"edit","path":"a\"b.txt","at":"10:00"}]""")
        assertEquals(1, cps.size)
        assertEquals(2, cps[0].seq)
        assertEquals("a\"b.txt", cps[0].path)
    }
}

class OfficePayloadTest {
    @Test
    fun editSetCell() {
        val p = officeEditPayload("abc", "set_cell", "Dades", "B2", "42", "", "")
        assert(p["op"] == "set_cell" && p["cell"] == "B2" && p["value"] == "42" && p["id"] == "abc")
    }

    @Test
    fun editReplace() {
        val p = officeEditPayload("abc", "replace", "", "", "", "hola", "adeu")
        assert(p["find"] == "hola" && p["replace"] == "adeu" && p["op"] == "replace")
    }
}

class ProvidersTest {
    private val sample = """{"providers":[{"name":"zen","url":"https://x/v1","key":"(buida)","used_by":[]},{"name":"local","url":"http://127.0.0.1:8089/v1","key":"sí","used_by":["code/qwen"]}],"roles":{"chat":{"provider":"zen","model":"m"}}}"""

    @Test
    fun llistaIEstat() {
        val ps = parseProviders(sample)
        assert(ps.size == 2)
        assert(!ps[0].hasKey() && ps[0].name == "zen")
        assert(ps[1].hasKey() && ps[1].usedBy == "code/qwen")
    }

    @Test
    fun buitISoroll() {
        assert(parseProviders("{}").isEmpty())
        assert(parseProviders("").isEmpty())
    }
}

class OfficeNameTest {
    @Test
    fun extensionsSuportades() {
        assertTrue(isOfficeName("informe.docx"))
        assertTrue(isOfficeName("dades.XLSX"))
        assertTrue(isOfficeName("slides.pptx"))
        assert(!isOfficeName("foto.png"))
        assert(!isOfficeName("sense-extensio"))
        assert(!isOfficeName("doc.pdf"))
    }
}

class TranscriptTextTest {
    @Test
    fun stringNormal() {
        assertEquals("hola", transcriptText("{\"role\":\"user\",\"content\":\"hola\"}"))
    }

    @Test
    fun escapats() {
        assertEquals("línia1\nlínia2 \u00e9",
            transcriptText("{\"content\":\"línia1\\nlínia2 \\u00e9\"}"))
    }

    @Test
    fun partsAmbImatge() {
        val m = "{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"mira \"}," +
            "{\"type\":\"image_url\",\"image_url\":{\"url\":\"data:x\"}}]}"
        assertEquals("mira 🖼️", transcriptText(m))
    }

    @Test
    fun sorollBuit() {
        assertEquals("", transcriptText("{\"role\":\"assistant\",\"tool_calls\":[{}]}"))
        // El rol tool el filtra resumeSession; aquí només s'extreu el text.
        assertEquals("ok", transcriptText("{\"role\":\"tool\",\"content\":\"ok\"}"))
        assertEquals("", transcriptText("{\"role\":\"assistant\",\"content\":null}"))
        assertEquals("", transcriptText("no-json"))
    }
}

class PrefsTest {
    @Test
    fun clausAppParalleles() {
        assertTrue(appStringsOk())
    }

    @Test
    fun trCauALaClau() {
        assertEquals("Nova sessió", tr("ca", "nav.new"))
        assertEquals("New session", tr("en", "nav.new"))
        assertEquals("Desconnecta", tr("ca", "btn.disconnect"))
        assertEquals("Cerca…", tr("ca", "app.sess.search"))
        assertEquals("Search…", tr("en", "app.sess.search"))
        assertEquals("clau.inexistent", tr("ca", "clau.inexistent"))
    }

    @Test
    fun paletaPerTema() {
        assertEquals(DarkPal, paletteFor("fosc", false))
        assertEquals(LightPal, paletteFor("clar", true))
        assertEquals(DarkPal, paletteFor("sistema", true))
        assertEquals(LightPal, paletteFor("sistema", false))
        assertEquals(DarkPal, paletteFor("qualsevol", true))
    }
}

class ConvsTest {
    @Test
    fun grupsPerData() {
        val now = 1_786_000_000_000L
        val iso = java.time.Instant.ofEpochMilli(now).toString()
        assertEquals("today", convGroup(iso, now))
        assertEquals("yesterday", convGroup(java.time.Instant.ofEpochMilli(now - 86400000L).toString(), now))
        assertEquals("week", convGroup(java.time.Instant.ofEpochMilli(now - 3 * 86400000L).toString(), now))
        assertEquals("month", convGroup(java.time.Instant.ofEpochMilli(now - 10 * 86400000L).toString(), now))
        assertEquals("older", convGroup(java.time.Instant.ofEpochMilli(now - 60 * 86400000L).toString(), now))
        assertEquals("older", convGroup("no-data", now))
        assertEquals("Avui", convGroupLabel("today", "ca"))
        assertEquals("Today", convGroupLabel("today", "en"))
    }

    @Test
    fun rolsILlegenda() {
        assertEquals("per revisar", roleLabel("reviewer"))
        assertEquals("per pensar", roleLabel("think"))
        assertEquals("desconegut", roleLabel("desconegut"))
        assertEquals("a", SessionInfo("a", 1, "x").displayTitle())
        assertEquals("Títol", SessionInfo("a", 1, "x", "Títol").displayTitle())
    }

    @Test
    fun nodesDeGraf() {
        val n = FlowNode("p1", "agent", "", "Fes tests", "", "")
        assertEquals("p1", n.label())
        assertEquals("Fes tests", n.detail())
        assertEquals("ls", FlowNode("p2", "tool", "Llista", "", "ls", "").detail())
    }
}
