package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import java.net.ConnectException
import java.net.SocketTimeoutException

@RunWith(RobolectricTestRunner::class)
class VentsOfflineTest {
    @get:Rule val tmp = TemporaryFolder()

    private fun msg(from: String, text: String, ts: Long = 0L, kind: String = "", streaming: Boolean = false) =
        GregalViewModel.PetMsg(from, text, ts = ts, kind = kind, streaming = streaming)

    @Test fun laCopiaDelXatGuardaElsUltimsSenseLesBombollesEnDirecte() {
        val msgs = (1..80).map { msg(if (it % 2 == 0) "pet" else "user", "m$it", ts = it.toLong()) } +
            msg("pet", "a mig escriure", streaming = true) + msg("user", "a la cua", 99, kind = "queued")
        val got = OfflineLogic.decodeChat(OfflineLogic.encodeChat(msgs))!!
        assertEquals(OfflineLogic.CHAT_KEEP, got.size)
        assertEquals("m80", got.last().text)
        assertTrue(got.none { it.streaming || it.kind == "queued" })
        assertNull(OfflineLogic.decodeChat("no és json"))
    }

    @Test fun laCuaVaEnOrdreIUnPerVent() {
        var q = emptyList<OutboxItem>()
        q = OfflineLogic.enqueue(q, OutboxItem("xaloc", "segon", 20))
        q = OfflineLogic.enqueue(q, OutboxItem("xaloc", "primer", 10))
        q = OfflineLogic.enqueue(q, OutboxItem("marinada", "hola", 15))
        // Un reintent amb la mateixa hora torna al seu lloc, no es duplica.
        q = OfflineLogic.enqueue(q, OutboxItem("xaloc", "primer", 10))
        assertEquals(listOf("primer", "hola", "segon"), q.map { it.text })
        assertEquals(q, OfflineLogic.decodeOutbox(OfflineLogic.encodeOutbox(q)))
        val next = OfflineLogic.nextToSend(q, busy = emptySet())
        assertEquals(setOf("primer", "hola"), next.map { it.text }.toSet())
        assertEquals(listOf("hola"), OfflineLogic.nextToSend(q, busy = setOf("xaloc")).map { it.text })
    }

    @Test fun elXatMostraElQueEsperaAlFinal() {
        val hist = listOf(msg("user", "hola", 1), msg("pet", "ei", 2), msg("user", "vell", 3, kind = "queued"))
        val q = listOf(OutboxItem("xaloc", "nou", 50), OutboxItem("altre", "no", 40))
        val got = OfflineLogic.withQueued(hist, q, "xaloc")
        assertEquals(listOf("hola", "ei", "nou"), got.map { it.text })
        assertEquals("queued", got.last().kind)
    }

    @Test fun recordatorisSenseServidorSonenUnCop() {
        val now = 1_000_000_000L
        val list = listOf(
            MirroredReminder("remind-xaloc-1", "xaloc", "Xaloc", now - 60_000, "Truca al dentista"),
            MirroredReminder("remind-xaloc-2", "xaloc", "Xaloc", now + 60_000, "Encara no"),
            MirroredReminder("remind-xaloc-0", "xaloc", "Xaloc", now - OfflineLogic.STALE_MS - 1, "Massa vell"),
        )
        val due = OfflineLogic.dueOffline(list, emptyMap(), now)
        assertEquals(listOf("remind-xaloc-1"), due.map { it.id })
        val shown = mapOf("remind-xaloc-1" to now)
        assertTrue(OfflineLogic.dueOffline(list, shown, now).isEmpty())
        // Quan el servidor torna i el dispara, la seva veu no torna a sonar.
        val noves = listOf(
            WindNovetat(1, "xaloc", "Xaloc", "remind", "Truca al dentista", "remind-xaloc-1"),
            WindNovetat(2, "xaloc", "Xaloc", "heartbeat", "Ei!"),
        )
        assertEquals(listOf("Ei!"), OfflineLogic.notYetShown(noves, shown).map { it.text })
        assertEquals(listOf("remind-xaloc-2"), OfflineLogic.pruneOnline(list, now).map { it.id })
        assertEquals(shown, OfflineLogic.decodeShown(OfflineLogic.encodeShown(shown)))
        assertTrue(OfflineLogic.pruneShown(shown, now + OfflineLogic.SHOWN_KEEP_MS + 1).isEmpty())
    }

    @Test fun laLlistaDelServidorSubstitueixIUnTornAfegeix() {
        val a = MirroredReminder("a", "xaloc", "Xaloc", 10, "a")
        val b = MirroredReminder("b", "marinada", "Marinada", 20, "b")
        val c = MirroredReminder("c", "xaloc", "Xaloc", 30, "c")
        assertEquals(listOf("c"), OfflineLogic.mergeReminders(listOf(a, b), listOf(c), "").map { it.id })
        assertEquals(listOf("b", "c"), OfflineLogic.mergeReminders(listOf(a, b), listOf(c), "xaloc").map { it.id })
        assertEquals(listOf("a", "b", "c"), OfflineLogic.mergeReminders(listOf(a, b), listOf(c), null).map { it.id })
        assertEquals(OfflineLogic.decodeReminders(OfflineLogic.encodeReminders(listOf(a, b))), listOf(a, b))
    }

    @Test fun nomesElsAvisosDunSolTretEsVigilen() {
        fun item(id: String, kind: String, repeat: String, next: String, enabled: Boolean = true) = WindAutomation(
            id = id, kind = kind, wind = "xaloc", windName = "Xaloc", windColor = "", windStyle = "pso",
            title = "t-$id", detail = "", repeat = repeat, time = "", intervalH = 0.0,
            enabled = enabled, nextAt = next, lastStatus = "", canPause = false,
        )
        val got = OfflineLogic.remindersOf(listOf(
            item("r", "reminder", "once", "2026-11-07T09:00:00+01:00"),
            item("f", "followup", "once", "2026-11-08T21:30:00+01:00"),
            item("d", "routine", "daily", "2026-11-07T08:00:00+01:00"),
            item("p", "reminder", "once", "2026-11-07T09:00:00+01:00", enabled = false),
            item("x", "reminder", "once", ""),
        ))
        assertEquals(listOf("r", "f"), got.map { it.id })
        assertEquals("t-r", got[0].text)
    }

    @Test fun nomesEsCuaSiElServidorNoHaRebutRes() {
        assertTrue(isUnreachable(ConnectException("Failed to connect to /10.0.2.2:8240")))
        assertTrue(isUnreachable(RuntimeException("wrap", ConnectException("refused"))))
        assertTrue(isUnreachable(SocketTimeoutException("failed to connect to host (port 8240) after 15000ms")))
        assertTrue(isUnreachable(ApiException("HTTP 502: bad gateway")))
        // Un torn que ja corria i s'ha tallat per temps de lectura: el servidor el té.
        assertFalse(isUnreachable(SocketTimeoutException("Read timed out")))
        assertFalse(isUnreachable(ApiException("HTTP 500: model error")))
        assertFalse(isUnreachable(ApiException("HTTP 401: server requires a valid token")))
    }

    @Test fun laCopiaEsPerServidor() {
        val a = VentsOfflineStore(tmp.newFolder("a"))
        val b = VentsOfflineStore(tmp.newFolder("b"))
        a.saveWinds("""{"winds":[{"id":"xaloc","name":"Xaloc"}]}""")
        a.saveChat("xaloc", listOf(msg("user", "hola", 1), msg("pet", "ei", 2)))
        a.saveOutbox(listOf(OutboxItem("xaloc", "després", 3)))
        assertEquals("xaloc", Wind.parse(a.windsRaw()!!).single().id)
        assertEquals(listOf("hola", "ei"), a.chat("xaloc")!!.map { it.text })
        assertEquals(1, a.outbox().size)
        assertTrue(a.savedAt() > 0)
        assertNull(b.windsRaw())
        assertNull(b.chat("xaloc"))
        assertTrue(b.outbox().isEmpty())
    }

    @Test fun lesCadenesNovesHiSonEnLesDuesLlengues() {
        for (k in listOf("app.offline.chip", "app.offline.chipQueued", "app.offline.pending", "app.offline.discard",
            "app.offline.retry", "app.offline.back", "app.offline.backSending", "app.offline.copyAt", "app.where.change", "app.where.phoneOther")) {
            assertTrue(k, STR_APP_CA.containsKey(k) && STR_APP_EN.containsKey(k))
        }
    }

    @Test fun ambMoltsRecordatorisEsVigilenElsMesPropers() {
        val now = 1_000_000L
        val times = (1..60).map { now + it * 60_000L }.reversed() + listOf(now - 3_600_000L)
        val keep = WindWatchLogic.keepDue(times, now)
        assertEquals(32, keep.size)
        assertEquals(now + 60_000L, keep.first())
        assertTrue(keep.none { it < now })
    }

    @Test fun unTornFallitDiuElMotiuQuanEsConegut() {
        assertTrue(turnFailureText("ca", "provider [url] HTTP 401: You didn't provide an API key").contains("clau d'API"))
        assertTrue(turnFailureText("en", "HTTP 429 rate limit").contains("limiting"))
        assertEquals("", turnFailureText("ca", "context deadline exceeded"))
    }
}
