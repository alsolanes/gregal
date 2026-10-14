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
