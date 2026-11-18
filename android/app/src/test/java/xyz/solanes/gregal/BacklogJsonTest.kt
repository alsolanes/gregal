package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class BacklogJsonTest {
    @Test
    fun llegeixElQueTornaElServidor() {
        val raw = """{"root":"C:/p","project":"p","storage":"repo","items":[
            {"id":2,"type":"bug","title":"Crash","status":"in_review","priority":"now","size":"S",
             "depends_on":[1],"why":"Data loss","criteria":[{"text":"Saves","done":true,"evidence":"TestSave"},{"text":"No dialog"}],
             "log":[{"at":"2026-11-18T10:00:00Z","who":"agent · s1","text":"created"}],"updated":"2026-11-18T10:00:00Z"},
            {"id":3,"type":"task","title":"Old","status":"dropped","reason":"dup of #2"}]}"""
        val s = BacklogJson.parse(raw)
        assertEquals("repo", s.storage)
        assertEquals(2, s.items.size)
        val bug = s.items[0]
        assertEquals(listOf(1), bug.dependsOn)
        assertEquals(1, bug.doneCriteria)
        assertEquals("TestSave", bug.criteria[0].evidence)
        assertEquals("agent · s1", bug.log[0].who)
        assertTrue(bug.open)
        assertFalse(s.items[1].open)
        assertEquals("next", s.items[1].priority)
    }

    @Test
    fun llegeixLaRevisio() {
        val raw = """{"items":[{"id":1,"type":"task","title":"T","status":"in_review",
            "review":{"status":"not_met","at":"2026-11-18T10:00:00Z","summary":"README unchanged",
              "checks":[{"command":"make check","passed":true}],
              "criteria":[{"text":"a","status":"MET","evidence":"f.go:3"},{"text":"b","status":"UNVERIFIED"}]}},
            {"id":2,"type":"task","title":"U","status":"in_review","review":{"status":"running","at":"2026-11-18T10:00:00Z"}}]}"""
        val s = BacklogJson.parse(raw)
        val r = s.items[0].review!!
        assertEquals("not_met", r.status)
        assertEquals("README unchanged", r.summary)
        assertTrue(r.checks[0].passed)
        assertEquals("UNVERIFIED", r.criteria[1].status)
        val running = s.items[1].review!!
        assertTrue(running.running(now = running.atMillis + 60_000))
        assertFalse("als 30 minuts es dona per perduda", running.running(now = running.atMillis + 31 * 60_000L))
    }

    @Test
    fun lErrorDelServidorEsMostraNet() {
        assertEquals("moving to blocked needs a reason",
            BacklogJson.errorText("HTTP 400: {\"error\":\"moving to blocked needs a reason\"}"))
        assertEquals("timeout", BacklogJson.errorText("timeout"))
    }

    @Test
    fun creaPortaElsCriterisNets() {
        val b = BacklogJson.create("feature", " Export CSV ", "why", "a\n\n  b  \n", "S", "now")
        assertEquals("Export CSV", b.getString("title"))
        assertEquals(2, b.getJSONArray("criteria").length())
        assertEquals("b", b.getJSONArray("criteria").getString(1))
    }
}
