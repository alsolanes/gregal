package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class WorkTest {
    @Test
    fun miniJson() {
        val o = MiniJson.obj("""{"a":"x\ny","n":3,"l":[1,"b",true,null],"o":{"k":"é"}}""")!!
        assertEquals("x\ny", o["a"])
        assertEquals(3.0, o["n"])
        assertEquals(listOf(1.0, "b", true, null), o["l"])
        assertEquals("é", (o["o"] as Map<*, *>)["k"])
        assertNull(MiniJson.obj("{\"path\": \"trunc"))
        assertNull(MiniJson.obj("[1,2]"))
    }

    @Test
    fun toolSummaryLikeTheWeb() {
        assertEquals(ToolSum("Read", "src/stats.py", ""), summarizeTool("read", """{"path":"src/stats.py"}""", "en"))
        assertEquals("lines 10–40", summarizeTool("read", """{"path":"a.go","offset":10,"limit":31}""", "en").meta)
        assertEquals(ToolSum("Edit", "a.py", "+1 −2"), summarizeTool("edit", """{"path":"a.py","old_string":"x\ny","new_string":"z"}""", "en"))
        assertEquals("python -m unittest discover", summarizeTool("bash", """{"command":"python -m unittest\n  discover"}""", "en").target)
        assertEquals("2 línies", summarizeTool("write", """{"path":"n.txt","content":"a\nb\n"}""", "ca").meta)
        assertEquals("2 tasks", summarizeTool("delegate", """{"tasks":[{"prompt":"find tests"},{"prompt":"read docs"}]}""", "en").meta)
        val mcp = summarizeTool("mcp_notes_create", """{"title":"x","tags":["a"]}""", "en")
        assertEquals("MCP · notes › create", mcp.label)
        assertEquals("title: x · tags: [\"a\"]", mcp.target)
        // JSON trencat: es mostra com a text, mai peta.
        assertEquals(ToolSum("Read", "{\"path\": \"trunc", ""), summarizeTool("read", "{\"path\": \"trunc", "en"))
        assertEquals("2 charts", summarizeTool("dashboard_save", """{"dashboard":"Vendes","plots":[{},{}]}""", "en").meta)
    }

    @Test
    fun workGroupLifecycle() {
        var w = WorkGroup(started = 1000)
        w = w.add(WorkStep("c1", StepKind.Tool, "Read", "a.py", state = StepState.Running, tool = "read"))
        w = w.add(WorkStep("c2", StepKind.Tool, "Run", "go test", state = StepState.Running, tool = "bash"))
        w = w.finishTool("c1", "read", "1: hola", false)
        assertEquals(StepState.Ok, w.steps[0].state)
        assertEquals("1: hola", w.steps[0].detail)
        // Sense id: tanca l'últim en curs amb el mateix nom.
        w = w.finishTool("", "bash", "ERROR: exit 1", toolOutputFailed("ERROR: exit 1"))
        assertEquals(StepState.Failed, w.steps[1].state)
        assertEquals(2, w.toolCount)
        assertEquals(1, w.failedCount)
        assertTrue(w.running)
        w = w.upsert(WorkStep("dx", StepKind.Delegate, "Subagent", state = StepState.Running))
        w = w.upsert(WorkStep("dx", StepKind.Delegate, "Subagent", state = StepState.Ok))
        assertEquals(3, w.steps.size)
        w = w.add(WorkStep("c3", StepKind.Tool, "Read", state = StepState.Running)).close(5000, failed = false)
        assertFalse(w.running)
        assertEquals(StepState.Ok, w.steps.last().state)
    }

    @Test
    fun progressAndDurations() {
        assertEquals("think", progressLine(mapOf("phase" to "model", "state" to "receiving", "reasoning" to "40", "text" to "0", "tool" to "0"), "en").first)
        assertEquals("write", progressLine(mapOf("phase" to "model", "state" to "receiving", "reasoning" to "40", "text" to "12", "tool" to "0"), "en").first)
        assertEquals("model", progressLine(mapOf("phase" to "model", "state" to "waiting"), "en").first)
        assertEquals("Running · Run", progressLine(mapOf("phase" to "tools", "tools" to "bash"), "en").second)
        assertEquals("8s", shortDuration(8_400))
        assertEquals("1m 05s", shortDuration(65_000))
        assertEquals("1h 02m", shortDuration(3_720_000))
    }

    @Test
    fun friendlyErrors() {
        assertTrue(friendlyError("model step stopped after 4m0s of inactivity", "en").startsWith("The model went quiet"))
        assertTrue(friendlyError("HTTP 401 from https://opencode.ai/zen/v1", "ca").contains("clau d'API"))
        assertFalse(friendlyError("HTTP 401 from https://opencode.ai/zen/v1", "en").contains("https://"))
        assertEquals("something odd", friendlyError("something odd", "en"))
    }
}
