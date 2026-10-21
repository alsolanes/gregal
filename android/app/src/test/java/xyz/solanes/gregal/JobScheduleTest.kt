package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.OffsetDateTime

class JobScheduleTest {
    private val now = OffsetDateTime.parse("2026-10-18T07:00:00+02:00").toInstant().toEpochMilli()
    private fun at(s: String) = OffsetDateTime.parse(s).toInstant().toEpochMilli()

    private val news = JobItem(id = "news", name = "Notícies", kind = "daily", time = "08:00",
        nextDueAt = "2026-10-18T08:00:00+02:00", lastAt = "2026-10-17T08:03:10+02:00")
    private val watch = JobItem(id = "watch", name = "Vigila", kind = "interval", intervalH = 12.0,
        nextDueAt = "2026-10-18T06:30:00+02:00")
    private val match = JobItem(id = "wind-xaloc", name = "Cor de Xaloc", kind = "event",
        nextDueAt = "2026-10-18T22:50:00+02:00", lastAt = "2026-10-17T09:01:00+02:00")
    private val manual = JobItem(id = "m", name = "Manual", kind = "manual", nextDueAt = "2026-10-18T05:00:00+02:00")
    private val remind = JobItem(id = "remind-xaloc-1", name = "Recordatori Xaloc", kind = "remind",
        nextDueAt = "2026-10-18T07:30:00+02:00")
    private val paused = news.copy(id = "p", enabled = false, nextDueAt = "2026-10-18T06:00:00+02:00")

    @Test
    fun slotsOnlyForActiveScheduledJobs() {
        val s = JobSchedule.slots(listOf(news, manual, paused, watch, match, remind, JobItem(id = "x", kind = "daily", nextDueAt = "")))
        assertEquals(listOf("watch", "remind-xaloc-1", "news", "wind-xaloc"), s.map { it.id })
        assertEquals(at("2026-10-18T08:00:00+02:00"), s[2].dueAt)
    }

    @Test
    fun alarmIsTheNearestSlotButNeverInThePast() {
        val s = JobSchedule.slots(listOf(news))
        assertEquals(at("2026-10-18T08:00:00+02:00"), JobSchedule.nextAlarm(s, now))
        // Una franja perduda (mòbil apagat a les 8): sona de seguida.
        assertEquals(now + JobSchedule.MIN_DELAY_MS, JobSchedule.nextAlarm(JobSchedule.slots(listOf(watch)), now))
        assertNull(JobSchedule.nextAlarm(emptyList(), now))
    }

    @Test
    fun dueUsesTheBackendGrace() {
        assertEquals(listOf("watch"), JobSchedule.due(listOf(news, watch, manual, paused), now).map { it.id })
        val almost = at("2026-10-18T07:59:30+02:00")
        assertEquals(listOf("watch", "news"), JobSchedule.due(listOf(news, watch), almost).map { it.id })
    }

    @Test
    fun finishedWhenTheBackendRecordsANewRun() {
        assertFalse(JobSchedule.finished(news, news))
        assertFalse(JobSchedule.finished(news, null))
        assertTrue(JobSchedule.finished(news, news.copy(lastAt = "2026-10-18T08:02:00+02:00")))
        assertTrue(JobSchedule.finished(watch, watch.copy(lastAt = "2026-10-18T07:01:00+02:00")))
        assertFalse(JobSchedule.finished(watch, watch))
    }

    @Test
    fun cacheRoundTripsAndSurvivesOddNames() {
        val s = listOf(ScheduledSlot("a", "Nom\tamb\ntabs", 1L), ScheduledSlot("b", "", 2L))
        val back = JobSchedule.decode(JobSchedule.encode(s))
        assertEquals(listOf("a", "b"), back.map { it.id })
        assertEquals("Nom amb tabs", back[0].name)
        assertEquals(listOf(1L, 2L), back.map { it.dueAt })
        assertTrue(JobSchedule.decode("").isEmpty())
        assertTrue(JobSchedule.decode("brossa\n\tx\t").isEmpty())
    }

    @Test
    fun parseInstantAcceptsGoRfc3339() {
        assertEquals(at("2026-10-18T06:00:00Z"), JobSchedule.parseInstant("2026-10-18T08:00:00+02:00"))
        assertNull(JobSchedule.parseInstant(""))
        assertNull(JobSchedule.parseInstant("18-10 08:00"))
    }

    @Test
    fun excerptIsPlainText() {
        val md = """
            # Notícies tech · 18/10
            
            - **Nou mòbil** amb [enllaç](https://x.y)
            
            | Font | Tema |
            |---|---|
            | A | B |
        """.trimIndent()
        val e = JobSchedule.excerpt(md)
        assertEquals("Notícies tech · 18/10\nNou mòbil amb enllaç\nFont · Tema\nA · B", e)
        assertTrue(JobSchedule.excerpt("x".repeat(2000), 100).length <= 100)
    }

    @Test
    fun templatesAreValidJobs() {
        for (lang in listOf("ca", "en")) {
            val t = jobTemplates(lang)
            assertEquals(3, t.size)
            t.forEach {
                assertTrue(it.name.isNotBlank() && !it.name.startsWith("app."))
                assertTrue(it.prompt.length > 20)
                assertTrue(it.kind == "daily" || it.kind == "interval")
                if (it.kind == "daily") assertEquals(it.time, fmtHm(parseHm(it.time).first, parseHm(it.time).second))
                assertTrue(it.enabled)
            }
        }
        assertEquals("08:00", jobTemplates("en").first().time)
    }

    @Test
    fun timeHelpers() {
        assertEquals(7 to 5, parseHm("7:05"))
        assertEquals(8 to 0, parseHm("25:00"))
        assertEquals(8 to 0, parseHm("abc"))
        assertEquals("07:05", fmtHm(7, 5))
    }

    @Test
    fun pollDecisionOneShotFiredIsFinished() {
        // L'avís d'un sol tret s'apaga en executar-se: apagat AMB lastAt nou ÉS èxit.
        val before = JobItem(id = "remind-xaloc-1", kind = "remind", enabled = true)
        val fired = before.copy(enabled = false, lastAt = "2026-10-18T07:05:00+02:00")
        assertEquals(JobSchedule.PollDecision.FINISHED, JobSchedule.pollDecision(before, fired))
        // Apagat sense executar-se (o esborrat): calla sense notificar.
        val off = before.copy(enabled = false)
        assertEquals(JobSchedule.PollDecision.GONE, JobSchedule.pollDecision(before, off))
        assertEquals(JobSchedule.PollDecision.GONE, JobSchedule.pollDecision(before, null))
        // Encès sense canvis: segueix esperant.
        assertEquals(JobSchedule.PollDecision.WAIT, JobSchedule.pollDecision(before, before))
        // Diari executat (segueix encès, lastAt nou): també és èxit.
        val daily = before.copy(kind = "daily", enabled = true, lastAt = "")
        assertEquals(JobSchedule.PollDecision.FINISHED,
            JobSchedule.pollDecision(daily, daily.copy(lastAt = "2026-10-18T07:05:00+02:00")))
    }
}
