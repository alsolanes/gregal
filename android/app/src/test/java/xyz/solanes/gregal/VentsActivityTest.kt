package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import java.time.LocalDate
import java.time.ZoneId

@RunWith(RobolectricTestRunner::class)
class VentsActivityTest {
    private val zone = ZoneId.of("Europe/Madrid")
    private val today = LocalDate.of(2026, 11, 5)

    private fun item(id: String, kind: String, next: String, enabled: Boolean = true) = WindAutomation(
        id = id, kind = kind, wind = "xaloc", windName = "Xaloc", windColor = "#EFA68F", windStyle = "pso",
        title = id, detail = "", repeat = if (kind == "routine") "daily" else "once", time = "08:00", intervalH = 0.0,
        enabled = enabled, nextAt = next, lastStatus = "", canPause = kind == "routine",
    )

    @Test fun parsejaLaLlistaIElsComptes() {
        val raw = """{"items":[{"id":"remind-xaloc-1","kind":"followup","wind":"xaloc","wind_name":"Xaloc","title":"Com ha anat?",
            "repeat":"once","enabled":true,"next_at":"2026-11-05T18:00:00+01:00","can_pause":false}],
            "counts":{"total":1,"today":1,"week":1,"paused":0}}"""
        val got = WindActivityJson.automations(raw)
        assertEquals(1, got.items.size)
        assertEquals("followup", got.items[0].kind)
        assertEquals(1, got.counts.today)
    }

    @Test fun diuQuanEnParaules() {
        assertEquals("avui 18:00", automationWhen("ca", "2026-11-05T18:00:00+01:00", today, zone))
        assertEquals("tomorrow 09:30", automationWhen("en", "2026-11-06T09:30:00+01:00", today, zone))
        assertEquals("dl. 09/11 08:00", automationWhen("ca", "2026-11-09T08:00:00+01:00", today, zone))
        assertEquals("", automationWhen("ca", "", today, zone))
    }

    @Test fun filtraPerTipusIAvui() {
        val items = listOf(
            item("a", "followup", "2026-11-05T18:00:00+01:00"),
            item("b", "reminder", "2026-11-07T18:00:00+01:00"),
            item("c", "routine", "2026-11-06T08:00:00+01:00", enabled = false),
            item("d", "task", "2026-11-05T20:00:00+01:00"),
        )
        assertEquals(listOf("a", "d"), filterAutomations(items, "today", today, zone).map { it.id })
        assertEquals(listOf("b", "d"), filterAutomations(items, "reminder").map { it.id })
        assertEquals(listOf("c"), filterAutomations(items, "paused").map { it.id })
        assertEquals(4, filterAutomations(items, "all").size)
    }

    @Test fun desatParsejaIDiaRelatiu() {
        val raw = """{"items":[{"kind":"doc","title":"Informe.pdf","url":"/api/winds/doc?name=Informe.pdf","wind":"xaloc","wind_name":"Xaloc","ts":1762300000}]}"""
        val got = WindActivityJson.saved(raw)
        assertEquals("doc", got[0].kind)
        assertTrue(isServerDocLink(got[0].url))
        val ts = today.atTime(10, 0).atZone(zone).toEpochSecond()
        assertEquals("avui", savedDay("ca", ts, today, zone))
        assertEquals("yesterday", savedDay("en", ts - 86_400, today, zone))
    }

    @Test fun lesCadenesNovesExisteixenEnLesDuesLlengues() {
        for (k in listOf("act.title", "act.followupCard", "act.kind.followup", "saved.title", "saved.kind.chart", "act.followups", "nav.saved")) {
            assertTrue(k, STR_APP_CA.containsKey(k) && STR_APP_EN.containsKey(k))
        }
    }
}
