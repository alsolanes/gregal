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

    private fun bash(command: String, lang: String = "en") =
        summarizeTool("bash", "{\"command\":${jsonStr(command)}}", lang)

    private fun jsonStr(s: String) = "\"" + s.replace("\\", "\\\\").replace("\"", "\\\"").replace("\n", "\\n") + "\""

    private fun exploreBash(command: String) = exploreTool("bash", "{\"command\":${jsonStr(command)}}")

    @Test
    fun readOnlyShellCommandsReadAsWhatTheyDo() {
        assertEquals(ToolSum("List", "internal/web, desktop", ""), bash("ls internal/web desktop 2>/dev/null | head -50"))
        assertEquals(ToolSum("Read", "internal/web/index.html", "lines 1190–1205"), bash("sed -n 1190,1205p internal/web/index.html"))
        assertEquals(ToolSum("Read", "a.js, b.js", ""), bash("cat a.js b.js"))
        assertEquals(ToolSum("Search", "\"work-group\"", "in internal/web/app"), bash("grep -rn \"work-group\" internal/web/app | head -40"))
        assertEquals(ToolSum("Search", "\"TODO\"", ""), bash("cd /c/dev/gregal && rg -n TODO"))
        assertEquals("Llista", bash("ls src", "ca").label)
        assertEquals("línies 1–40", bash("sed -n 1,40p a.go", "ca").meta)
    }

    @Test
    fun testsAndGitGetTheirOwnVerbOtherCommandsKeepRun() {
        assertEquals(ToolSum("Test", "go test ./internal/web -run TestX", ""), bash("go test ./internal/web -run TestX"))
        assertEquals("Test", bash("npm test").label)
        assertEquals("Test", bash("node --test internal/web/app/x.test.cjs").label)
        assertEquals("Test", bash("pytest -q tests").label)
        assertEquals("Test", bash("cargo test --lib").label)
        assertEquals("Test", bash("go test ./... 2>&1 | tail -20").label)
        // Només una ordre sola té el verb: una cadena pot fer una altra cosa després.
        assertEquals(ToolSum("Run", "go test ./... && rm -rf x", ""), bash("go test ./... && rm -rf x"))
        assertEquals("Run", bash("git status && rm -rf x").label)
        assertEquals("Read", bash("sed -n -E 1,5p a.go").label)
        assertEquals(ToolSum("Git", "status --short", ""), bash("git status --short"))
        assertEquals(ToolSum("Run", "npm run build", ""), bash("npm run build"))
        assertEquals("Prova", bash("go test ./...", "ca").label)
        assertEquals("Executa", bash("npm run build", "ca").label)
        // bash_background no és mai exploració: engega alguna cosa.
        assertEquals("Start", summarizeTool("bash_background", """{"command":"ls"}""", "en").label)
    }

    @Test
    fun exploreListsWhatACallLooksAtAndNothingForCallsThatChangeThings() {
        assertEquals(listOf(ExploreAct("read", "a.go")), exploreTool("read", """{"path":"a.go"}"""))
        assertEquals(listOf(ExploreAct("search", "x")), exploreTool("grep", """{"pattern":"x"}"""))
        assertEquals(listOf(ExploreAct("list", "*.go")), exploreTool("glob", """{"pattern":"*.go"}"""))
        assertEquals(listOf(ExploreAct("read", "a.go"), ExploreAct("search", "y")), exploreBash("cat a.go; grep -n y b.go | head"))
        for (command in listOf(
            "cat a > b", "sed -i s/a/b/ f", "find . -name x -delete", "find . -exec rm {} ;",
            "rm -rf out", "ls; npm install", "go test ./...", "git status", "cat a | tee b", "cat $(rm x)",
            // sed: in situ amagat en opcions combinades o abreujades, un segon -e o un fitxer de guió.
            "sed -n -Ei 1p f", "sed -n -ri 1,5p f", "sed -n --in 1p f", "sed -n -e \"w /tmp/x\" -e 1p f",
            "sed -n -ew/tmp/x 1p f", "sed -n --expression=w/tmp/x 1p f", "sed -n -f evil.sed -e 1p f", "sed -n --file=evil.sed 1p f",
            // Filtres després d'una canonada que escriuen, i cercadors o llistadors que executen o escriuen.
            "ls | sort -o out", "ls | sort --output=out", "ls | sort --compress-program=sh", "ls | uniq - out.txt",
            "rg --pre ./evil.sh x", "rg --pre=sh x .", "rg --hostname-bin=./evil x", "grep -rn x . | rg --pre sh y",
            "tree -o out.txt src", "find . -fprint0 out",
            "cat `rm x`", "cat \"\$(rm x)\"", "cat < a", "ls & rm x", "echo 'open",
            // Cometes que no tanquen on sembla, expansions que executen i un \r enganxat a la redirecció.
            "ls \$'\\'' ; touch pwned #'", "cat \$'\\'' >out #'", "ls #'\ntouch pwned2 #'",
            "printf -v x %s 'a[\$(touch p)]'; ls \$[x]", "printf -v x %s '\$(touch p)'; ls \${x@P}",
            "ls \${x:='\$(touch p)'} 2>/dev/null; ls \${x@P}", "ls \"\${x@P}\"", "ls >&1\rx", "tree -R src",
            // Els mateixos casos que tool-summary.test.cjs: qualsevol $ o # fora de cometes simples,
            // caràcters de control i opcions llargues de tree.
            "cat \$'a\\x27; rm -rf x'", "cat a #'\nrm -rf x'", "printf -v x \"%s\" \"\$(id)\"; echo \${x@P}", "cat \$[1+1]",
            "cat \${HOME}", "ls 2>&1\r", "tree -R -H . src", "ls \$HOME", "ls a#b", "tree --fromfile list",
        )) {
            assertNull(command, exploreBash(command))
        }
        assertNull(exploreTool("edit", """{"path":"a.go"}"""))
        assertNull(exploreTool("bash", "not json"))
    }

    @Test
    fun exploreSummaryCountsDistinctFilesSearchesAndListings() {
        val calls = listOf(
            exploreBash("ls internal/web desktop"),
            exploreTool("read", """{"path":"a.go"}"""),
            exploreBash("sed -n 1,9p a.go; cat b.go"),
            exploreTool("grep", """{"pattern":"x"}"""),
        )
        assertEquals(ToolSum("Explored", "2 files · 1 search · 1 listing"), exploreSummary(calls, "en"))
        assertEquals("2 fitxers · 1 cerca · 1 llistat", exploreSummary(calls, "ca").target)
        assertEquals("1 fitxer", exploreSummary(listOf(exploreTool("read", """{"path":"a"}""")), "ca").target)
    }

    @Test
    fun consecutiveExploringStepsFoldIntoOneRow() {
        fun tool(id: String, name: String, args: String) =
            WorkStep(id, StepKind.Tool, name, tool = name, explore = exploreTool(name, args))
        val steps = listOf(
            tool("1", "read", """{"path":"a.go"}"""),
            tool("2", "bash", """{"command":"ls src"}"""),
            WorkStep("n", StepKind.Note, "Note"),
            tool("3", "grep", """{"pattern":"x"}"""),
            tool("4", "edit", """{"path":"a.go"}"""),
            tool("5", "bash", """{"command":"cat a.go"}"""),
            tool("6", "glob", """{"pattern":"*.go"}"""),
            tool("7", "bash", """{"command":"sed -n 1,5p a.go"}"""),
        )
        assertEquals(
            listOf(listOf("1", "2"), listOf("n"), listOf("3"), listOf("4"), listOf("5", "6", "7")),
            foldExplore(steps).map { row -> row.map { it.id } },
        )
    }

    private fun args(vararg kv: Pair<String, String>) =
        kv.joinToString(",", "{", "}") { "${jsonStr(it.first)}:${jsonStr(it.second)}" }

    @Test
    fun sentenceDescribesARunOfCallsByItsMostTellingCall() {
        fun c(name: String, vararg kv: Pair<String, String>) = name to args(*kv)
        assertEquals("Ha consultat el calendari", toolSentence(listOf(c("calendar", "action" to "today")), "ca"))
        assertEquals("Checked the mail", toolSentence(listOf(c("mail", "action" to "unread")), "en"))
        assertEquals("Ha mirat els recordatoris", toolSentence(listOf(c("remind_list")), "ca"))
        assertEquals("Ha consultat la web", toolSentence(listOf(c("web_search", "query" to "x"), c("web_fetch", "url" to "https://a.dev")), "ca"))
        // Llegir i després editar és editar.
        assertEquals("Edited files", toolSentence(listOf(c("read", "path" to "a.go"), c("edit", "path" to "a.go", "old_string" to "a", "new_string" to "b")), "en"))
        assertEquals("Ha passat els tests", toolSentence(listOf(c("bash", "command" to "go test ./...")), "ca"))
        assertEquals("Ran a command", toolSentence(listOf(c("bash", "command" to "npm run build")), "en"))
        assertEquals("Searched the code", toolSentence(listOf(c("bash", "command" to "ls src"), c("grep", "pattern" to "x")), "en"))
        assertEquals("Ha mirat els fitxers", toolSentence(listOf(c("read", "path" to "a.go")), "ca"))
        assertEquals("Used github", toolSentence(listOf(c("mcp_github_create_issue", "title" to "t")), "en"))
        assertEquals("Ha fet servir profile", toolSentence(listOf(c("profile", "action" to "get")), "ca"))
        assertEquals("Worked with git", toolSentence(listOf(c("bash", "command" to "git status")), "en"))
        assertEquals("Ha fet servir el navegador", toolSentence(listOf(c("browser", "action" to "open")), "ca"))
        assertEquals("Handed work to helpers", toolSentence(listOf(c("delegate")), "en"))
    }

    @Test
    fun chipNamesWhatACallTouchedInAFewWords() {
        assertEquals(ToolChip("edit", "login.go +1 −2"), toolChip("edit", args("path" to "C:/p/internal/login.go", "old_string" to "a\nb", "new_string" to "c"), "en"))
        assertEquals(ToolChip("read", "app.ts"), toolChip("read", args("path" to "src/app.ts"), "en"))
        // Les rutes de Windows també: el nom del fitxer, no la ruta sencera.
        assertEquals(ToolChip("read", "app.ts"), toolChip("read", args("path" to "C:\\p\\src\\app.ts"), "en"))
        assertEquals(ToolChip("edit", "login.go +1 −1"), toolChip("edit", args("path" to "C:\\p\\login.go", "old_string" to "a", "new_string" to "b"), "en"))
        assertEquals(ToolChip("edit", "n.txt +2"), toolChip("write", args("path" to "n.txt", "content" to "a\nb\n"), "en"))
        assertEquals(ToolChip("test", "go test ./internal/..."), toolChip("bash", args("command" to "cd x && go test ./internal/..."), "en"))
        assertEquals(ToolChip("run", "npm run build"), toolChip("bash", args("command" to "npm run build"), "en"))
        assertEquals(ToolChip("git", "git status --short"), toolChip("bash", args("command" to "git status --short"), "en"))
        assertEquals(ToolChip("web", "pkg.go.dev"), toolChip("web_fetch", args("url" to "https://pkg.go.dev/crypto/subtle"), "en"))
        assertEquals(ToolChip("tool", "calendar · today"), toolChip("calendar", args("action" to "today"), "ca"))
        assertEquals(ToolChip("tool", "remind_list"), toolChip("remind_list", "{}", "ca"))
        assertTrue(toolChip("bash", args("command" to "x".repeat(200)), "en").text.length <= 48)
        // Una fila «Explorat» porta els seus comptes.
        val fold = listOf(
            WorkStep("1", StepKind.Tool, "Read", tool = "read", args = args("path" to "a.go"), explore = exploreTool("read", args("path" to "a.go"))),
            WorkStep("2", StepKind.Tool, "Search", tool = "grep", args = args("pattern" to "x"), explore = exploreTool("grep", args("pattern" to "x"))),
            WorkStep("3", StepKind.Tool, "Search", tool = "grep", args = args("pattern" to "y"), explore = exploreTool("grep", args("pattern" to "y"))),
        )
        assertEquals(ToolChip("search", "1 fitxer · 2 cerques"), stepChip(fold, "ca").first)
        // L'estat de la fila és el de les crides de dins.
        assertEquals("done", stepChip(fold, "ca").second.state)
        assertEquals("running", stepChip(fold.take(2) + fold[2].copy(state = StepState.Running), "ca").second.state)
        assertEquals("failed", stepChip(listOf(fold[0].copy(state = StepState.Failed), fold[1], fold[2].copy(state = StepState.Running)), "ca").second.state)
    }

    @Test
    fun outcomeFailsOnErrorsButAReadOnlyCommandThatFindsNothingIsQuiet() {
        val grep = args("command" to "grep -rn foo src")
        assertEquals(ToolOutcome("quiet", "sense resultats"), toolOutcome("bash", grep, "ERROR: exit: exit status 1\n", "ca"))
        assertEquals(ToolOutcome("quiet", "nothing found"), toolOutcome("bash", args("command" to "ls nope"), "ERROR: exit: exit status 2\nls: cannot access", "en"))
        // Una ordre que fa coses (o uns tests) que surt amb error sí que ha fallat.
        assertEquals(ToolOutcome("failed", "codi 2"), toolOutcome("bash", args("command" to "npm run build"), "ERROR: exit: exit status 2\nTS2304", "ca"))
        assertEquals(ToolOutcome("failed", "exit 127"), toolOutcome("bash", grep, "ERROR: exit: exit status 127\n", "en"))
        assertEquals(ToolOutcome("failed", ""), toolOutcome("read", args("path" to "a"), "ERROR: no such file", "en"))
        assertEquals(ToolOutcome("failed", ""), toolOutcome("bash", args("command" to "curl x"), "TOOL BLOCKED: patró bloquejat: curl", "en"))
        assertEquals(ToolOutcome("done", ""), toolOutcome("calendar", "{}", "No events.", "en"))
        // Sense l'«exit: » del mig també es llegeix el codi.
        assertEquals(ToolOutcome("failed", "exit 3"), toolOutcome("bash", grep, "ERROR: exit status 3\n", "en"))
    }

    @Test
    fun theExitLineOfACommandIsNotRepeatedInItsOutput() {
        assertEquals("TS2304: x\n", toolOutputShown("bash", "ERROR: exit: exit status 2\nTS2304: x\n"))
        assertEquals("ERROR: no such file", toolOutputShown("read", "ERROR: no such file"))
        assertEquals("ok\n", toolOutputShown("bash", "ok\n"))
    }

    @Test
    fun aQuietCallIsNotAFailedStepAndAFailedOneOpensRed() {
        val w = WorkGroup(started = 1)
            .add(WorkStep("g", StepKind.Tool, "Search", state = StepState.Running, tool = "bash", args = args("command" to "grep -rn foo src")))
            .add(WorkStep("b", StepKind.Tool, "Run", state = StepState.Running, tool = "bash", args = args("command" to "npm run build")))
            .add(WorkStep("x", StepKind.Tool, "Run", state = StepState.Running, tool = "bash", args = args("command" to "curl x")))
            .finishTool("g", "bash", "ERROR: exit: exit status 1\n", true)
            .finishTool("b", "bash", "ERROR: exit: exit status 2\nTS2304", true)
            .finishTool("x", "bash", "TOOL BLOCKED: patró bloquejat: curl", false)
        assertEquals(listOf(StepState.Ok, StepState.Failed, StepState.Failed), w.steps.map { it.state })
        assertEquals(ToolOutcome("quiet", "sense resultats"), stepChip(listOf(w.steps[0]), "ca").second)
        assertEquals(ToolOutcome("failed", "codi 2"), stepChip(listOf(w.steps[1]), "ca").second)
        assertEquals(ToolOutcome("running", ""), stepChip(listOf(WorkStep("r", StepKind.Tool, "Run", state = StepState.Running, tool = "bash")), "ca").second)
    }

    @Test
    fun callsJoinTheNarrationBeforeThemOrASentenceOfTheirOwn() {
        fun tool(id: String, name: String, a: String) =
            WorkStep(id, StepKind.Tool, name, tool = name, args = a, explore = exploreTool(name, a))
        val steps = listOf(
            WorkStep("n1", StepKind.Note, "Busco on es valida"),
            tool("1", "read", args("path" to "a.go")),
            tool("2", "grep", args("pattern" to "x")),
            tool("3", "edit", args("path" to "a.go")),
            WorkStep("t", StepKind.Think, "Thought"),
            tool("4", "calendar", args("action" to "today")),
            WorkStep("n2", StepKind.Note, "Fet"),
        )
        val lines = workLines(steps)
        assertEquals(listOf("n1", "t", null, "n2"), lines.map { it.step?.id })
        // La narració es queda les crides que la segueixen; l'exploració, plegada, és una pastilla.
        assertEquals(listOf(listOf("1", "2"), listOf("3")), lines[0].calls.map { row -> row.map { it.id } })
        assertEquals(emptyList<List<WorkStep>>(), lines[1].calls)
        // Després d'un raonament no hi ha narració: la frase la posa Gregal.
        assertEquals(listOf(listOf("4")), lines[2].calls.map { row -> row.map { it.id } })
        assertEquals(emptyList<List<WorkStep>>(), lines[3].calls)
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
    fun progressNamesTheFileAndTool() {
        fun line(vararg kv: Pair<String, String>, lang: String = "ca") =
            progressLine(mapOf("phase" to "model", "state" to "receiving", "reasoning" to "500", "text" to "0") + kv, lang)
        assertEquals("Escrivint index.html · 12k caràcters", line("tool" to "12000", "tool_name" to "write", "tool_target" to "index.html").second)
        assertEquals("Writing index.html · 12.3k characters", line("tool" to "12345", "tool_name" to "write", "tool_target" to "index.html", lang = "en").second)
        assertEquals("Preparant bash · 40 caràcters", line("tool" to "40", "tool_name" to "bash").second)
        assertEquals("Preparant una eina · 40 caràcters", line("tool" to "40").second)
        assertEquals("", line("tool" to "40", "tool_name" to "bash", "tail" to "x").third)
    }

    @Test
    fun progressThinkingShowsSizeAndTail() {
        val p = progressLine(mapOf("phase" to "model", "state" to "receiving", "reasoning" to "67000", "text" to "0", "tool" to "0",
            "tail" to "  plan\nthe  layout "), "ca")
        assertEquals("think", p.first)
        assertEquals("Pensant · 67k caràcters", p.second)
        assertEquals("…plan the layout", p.third)
        assertEquals("Thinking · 40 characters", progressLine(mapOf("phase" to "model", "state" to "receiving", "reasoning" to "40", "text" to "0", "tool" to "0"), "en").second)
    }

    @Test
    fun progressFromOldServerWithoutNewFields() {
        fun line(vararg kv: Pair<String, String>) = progressLine(mapOf("phase" to "model") + kv, "ca")
        assertEquals("Esperant el model…", line("state" to "waiting").second)
        assertEquals("Escrivint…", line("state" to "receiving", "reasoning" to "40", "text" to "12", "tool" to "0").second)
        assertEquals("Escrivint…", line("state" to "receiving").second)
        assertEquals("", line("state" to "receiving", "reasoning" to "40", "text" to "0", "tool" to "0").third)
        assertEquals("Executant · Executa", progressLine(mapOf("phase" to "tools", "tools" to "bash"), "ca").second)
    }

    @Test
    fun friendlyErrors() {
        assertTrue(friendlyError("model step stopped after 4m0s of inactivity", "en").startsWith("The model went quiet"))
        assertTrue(friendlyError("HTTP 401 from https://opencode.ai/zen/v1", "ca").contains("clau d'API"))
        assertFalse(friendlyError("HTTP 401 from https://opencode.ai/zen/v1", "en").contains("https://"))
        assertEquals("something odd", friendlyError("something odd", "en"))
    }

    @Test
    fun stoppedTurnIsNotAnError() {
        val w = WorkGroup(started = 1L).close(10L, failed = true, stopped = true)
        org.junit.Assert.assertFalse(w.failed)
        org.junit.Assert.assertTrue(w.stopped)
        val f = WorkGroup(started = 1L).close(10L, failed = true)
        org.junit.Assert.assertTrue(f.failed)
        org.junit.Assert.assertFalse(f.stopped)
    }
}
