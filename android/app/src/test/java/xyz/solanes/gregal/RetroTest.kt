package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class RetroTest {
    private val json = """{"key":"run-5","latest":{"run_id":5,"running":false,"key":"run-5"},"emojis":["👍","🎉"],
      "channel":{"key":"run-5","title":"Run #5","degraded":false,
        "agents":[{"id":"coordinator","name":"Coordinator","avatar":"coordinator"},{"id":"researcher","name":"Researcher","avatar":"researcher"}],
        "messages":[
          {"id":"m1","author":"coordinator","text":"I read 4 files","receipts":["e1","e5"],"reactions":[{"emoji":"👍","by":["researcher","user"]}],"at":"2026-10-14T08:46:00Z"},
          {"id":"m2","author":"user","text":"why no tests?","at":"2026-10-14T08:47:00Z"},
          {"id":"m3","thread_id":"m2","author":"coordinator","text":"Fair question","concede":true,"at":"2026-10-14T08:47:30Z"}
        ]}}"""

    @Test
    fun parsesChannelAndLatest() {
        val info = parseRetroInfo(json)
        assertEquals(5L, info.latestRun)
        assertEquals(listOf("👍", "🎉"), info.emojis)
        val ch = info.channel!!
        assertEquals("Run #5", ch.title)
        assertEquals("Researcher", ch.agent("researcher")?.name)
        assertEquals(listOf("e1", "e5"), ch.messages[0].receipts)
        assertTrue(ch.messages[0].reactions[0].mine)
        assertTrue(ch.messages[2].concede)
    }

    @Test
    fun threadsReadUnderTheirParent() {
        val rows = retroOrdered(parseRetroInfo(json).channel!!.messages)
        assertEquals(listOf("m1" to false, "m2" to false, "m3" to true), rows.map { it.first.id to it.second })
    }

    @Test
    fun emptyOrBrokenResponses() {
        assertNull(parseRetroInfo("""{"key":"","latest":{"run_id":0}}""").channel)
        assertEquals(RETRO_DEFAULT_EMOJIS, parseRetroInfo("not json").emojis)
    }
}

class RetroReviewTest {
    private fun f(id: String, sev: String, thread: String = "") = RetroMsg(id, thread, "qa", "t", kind = "finding", severity = sev)

    @org.junit.Test
    fun findingsCountedAndFilteredBySeverity() {
        val msgs = listOf(
            RetroMsg("m1", "", "coordinator", "did it"),
            RetroMsg("m2", "", "qa", "weak tests", kind = "verdict"),
            f("m3", "high"), f("m4", "odd"), f("m5", "medium"), f("m6", "high", thread = "m5"),
        )
        org.junit.Assert.assertEquals(mapOf("high" to 1, "low" to 1, "medium" to 1), retroFindingCounts(msgs))
        org.junit.Assert.assertEquals(6, retroFilter(msgs, "").size)
        org.junit.Assert.assertEquals(listOf("m3", "m4", "m5"), retroFilter(msgs, "all").map { it.id })
        org.junit.Assert.assertEquals(listOf("m4"), retroFilter(msgs, "low").map { it.id })
    }

    @org.junit.Test
    fun parsesReviewFields() {
        val ch = parseRetroChannelResponse("""{"channel":{"key":"run-1","title":"t","agents":[],"messages":[
            {"id":"m1","author":"qa","text":"x","kind":"finding","severity":"high","title":"No tests","fix_suggestion":"Add tests","fix_run":7}]}}""")!!
        val m = ch.messages[0]
        org.junit.Assert.assertTrue(m.isFinding)
        org.junit.Assert.assertEquals("high", m.sev)
        org.junit.Assert.assertEquals("No tests", m.title)
        org.junit.Assert.assertEquals(7L, m.fixRun)
    }
}
