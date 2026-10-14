package xyz.solanes.gregal

/*
 * El treball d'un torn com a la web (beginWorkGroup a index.html): totes les
 * eines, el raonament i la narració intermèdia van dins d'UNA targeta
 * plegable; la resposta final queda fora. Pur (sense org.json ni android.jar)
 * perquè els tests de JVM ho puguin provar tal qual.
 */

// --- JSON mínim (els tests de JVM no tenen org.json) ---

object MiniJson {
    /** Parseja un JSON a Map/List/String/Double/Boolean/null; null si no és vàlid. */
    fun parse(src: String): Any? = try {
        val p = P(src)
        p.ws()
        val v = p.value()
        p.ws()
        if (p.i != src.length) null else v
    } catch (_: Exception) { null }

    fun obj(src: String): Map<String, Any?>? {
        @Suppress("UNCHECKED_CAST")
        return parse(src) as? Map<String, Any?>
    }

    private class P(val s: String) {
        var i = 0
        fun ws() { while (i < s.length && s[i].isWhitespace()) i++ }
        fun value(): Any? {
            ws()
            if (i >= s.length) throw IllegalStateException("eof")
            return when (s[i]) {
                '{' -> obj()
                '[' -> arr()
                '"' -> str()
                't' -> lit("true", true)
                'f' -> lit("false", false)
                'n' -> lit("null", null)
                else -> num()
            }
        }
        fun lit(w: String, v: Any?): Any? {
            if (!s.startsWith(w, i)) throw IllegalStateException("lit")
            i += w.length
            return v
        }
        fun num(): Double {
            val st = i
            while (i < s.length && (s[i].isDigit() || s[i] in "+-.eE")) i++
            return s.substring(st, i).toDouble()
        }
        fun str(): String {
            i++ // "
            val b = StringBuilder()
            while (true) {
                val c = s[i++]
                when (c) {
                    '"' -> return b.toString()
                    '\\' -> {
                        when (val e = s[i++]) {
                            'n' -> b.append('\n'); 't' -> b.append('\t'); 'r' -> b.append('\r')
                            'b' -> b.append('\b'); 'f' -> b.append('\u000c')
                            'u' -> { b.append(s.substring(i, i + 4).toInt(16).toChar()); i += 4 }
                            else -> b.append(e)
                        }
                    }
                    else -> b.append(c)
                }
            }
        }
        fun arr(): List<Any?> {
            i++
            val out = mutableListOf<Any?>()
            ws()
            if (s[i] == ']') { i++; return out }
            while (true) {
                out += value()
                ws()
                when (s[i++]) { ',' -> continue; ']' -> return out; else -> throw IllegalStateException("arr") }
            }
        }
        fun obj(): Map<String, Any?> {
            i++
            val out = LinkedHashMap<String, Any?>()
            ws()
            if (s[i] == '}') { i++; return out }
            while (true) {
                ws()
                val k = str()
                ws()
                if (s[i++] != ':') throw IllegalStateException("obj")
                out[k] = value()
                ws()
                when (s[i++]) { ',' -> continue; '}' -> return out; else -> throw IllegalStateException("obj") }
            }
        }
    }
}

// --- Resum humà d'una crida d'eina (port de internal/web/app/tool-summary.js) ---

data class ToolSum(val label: String, val target: String = "", val meta: String = "")

private val TOOL_EN = mapOf(
    "read" to "Read", "write" to "Write", "edit" to "Edit", "patch" to "Edit", "glob" to "Find files", "grep" to "Search",
    "bash" to "Run", "bash_background" to "Start", "bash_output" to "Check output", "bash_kill" to "Stop process",
    "web_search" to "Search the web", "web_fetch" to "Fetch", "read_image" to "View image",
    "gh_issue" to "GitHub issue", "gh_pr" to "Pull request", "git_commit" to "Commit", "gh_pr_create" to "Create pull request",
    "office_read" to "Read document", "office_open" to "Open document", "office_edit" to "Edit document", "office_create" to "Create document",
    "dashboard_save" to "Save to dashboard", "delegate" to "Delegate", "question" to "Ask", "todowrite" to "Update plan", "todoread" to "Read plan",
    "skill" to "Open skill", "preview" to "Preview", "browser" to "Browser", "mcp" to "MCP", "symbols" to "Find symbols",
    "knowledge_search" to "Search knowledge", "knowledge_note" to "Save note",
    "lines" to "lines", "line" to "line", "from" to "from", "in" to "in", "edits" to "edits", "edit1" to "edit",
    "tasks" to "tasks", "charts" to "charts", "chart1" to "chart", "files" to "files",
)
private val TOOL_CA = mapOf(
    "read" to "Llegeix", "write" to "Escriu", "edit" to "Edita", "patch" to "Edita", "glob" to "Cerca fitxers", "grep" to "Cerca",
    "bash" to "Executa", "bash_background" to "Engega", "bash_output" to "Mira la sortida", "bash_kill" to "Atura el procés",
    "web_search" to "Cerca a la web", "web_fetch" to "Obre", "read_image" to "Mira la imatge",
    "gh_issue" to "Issue de GitHub", "gh_pr" to "Pull request", "git_commit" to "Fes commit", "gh_pr_create" to "Obre una pull request",
    "office_read" to "Llegeix el document", "office_open" to "Obre el document", "office_edit" to "Edita el document", "office_create" to "Crea el document",
    "dashboard_save" to "Desa al dashboard", "delegate" to "Delega", "question" to "Pregunta", "todowrite" to "Actualitza el pla", "todoread" to "Llegeix el pla",
    "skill" to "Obre la skill", "preview" to "Previsualitza", "browser" to "Navegador", "mcp" to "MCP", "symbols" to "Cerca símbols",
    "knowledge_search" to "Cerca al coneixement", "knowledge_note" to "Desa una nota",
    "lines" to "línies", "line" to "línia", "from" to "des de", "in" to "a", "edits" to "canvis", "edit1" to "canvi",
    "tasks" to "tasques", "charts" to "gràfics", "chart1" to "gràfic", "files" to "fitxers",
)

fun oneLine(s: String?, max: Int): String {
    val t = (s ?: "").replace(Regex("\\s+"), " ").trim()
    return if (t.length > max) t.take(max - 1) + "…" else t
}

private fun lineCount(s: String): Int = if (s.isEmpty()) 0 else s.removeSuffix("\n").split('\n').size

private fun jsonInline(v: Any?): String = when (v) {
    null -> "null"
    is String -> v
    is Double -> if (v % 1.0 == 0.0) v.toLong().toString() else v.toString()
    is List<*> -> v.joinToString(",", "[", "]") { if (it is String) "\"$it\"" else jsonInline(it) }
    is Map<*, *> -> v.entries.joinToString(",", "{", "}") { "\"${it.key}\":${if (it.value is String) "\"${it.value}\"" else jsonInline(it.value)}" }
    else -> v.toString()
}

fun summarizeTool(name: String, args: String, lang: String = "ca"): ToolSum {
    val t = if (lang == "en") TOOL_EN else TOOL_CA
    val tool = name
    var label = t[tool] ?: tool
    val o = MiniJson.obj(args) ?: return ToolSum(label, oneLine(args, 160))
    fun s(k: String) = o[k] as? String ?: ""
    fun n(k: String) = (o[k] as? Double)?.toInt() ?: 0
    var target = ""
    var meta = ""
    when (tool) {
        "read" -> {
            target = s("path")
            val off = n("offset"); val lim = n("limit")
            meta = when {
                off > 1 && lim > 0 -> "${t["lines"]} $off–${off + lim - 1}"
                lim > 0 -> "${t["lines"]} 1–$lim"
                off > 1 -> "${t["from"]} ${t["line"]} $off"
                else -> ""
            }
        }
        "write" -> { target = s("path"); val c = lineCount(s("content")); if (c > 0) meta = "$c ${if (c == 1) t["line"] else t["lines"]}" }
        "edit" -> { target = s("path"); meta = "+${lineCount(s("new_string"))} −${lineCount(s("old_string"))}" }
        "patch" -> { target = s("path"); val c = (o["edits"] as? List<*>)?.size ?: 0; if (c > 0) meta = "$c ${if (c == 1) t["edit1"] else t["edits"]}" }
        "glob" -> { target = s("pattern"); if (s("dir").isNotEmpty()) meta = "${t["in"]} ${s("dir")}" }
        "symbols" -> { target = if (s("query").isNotEmpty()) "\"${oneLine(s("query"), 80)}\"" else ""; meta = listOf(s("kind"), s("file")).filter { it.isNotEmpty() }.joinToString(" · ") }
        "grep" -> {
            target = if (s("pattern").isNotEmpty()) "\"${oneLine(s("pattern"), 80)}\"" else ""
            meta = listOf(if (s("dir").isNotEmpty()) "${t["in"]} ${s("dir")}" else "", s("include")).filter { it.isNotEmpty() }.joinToString(" · ")
        }
        "bash", "bash_background" -> target = oneLine(s("command"), 200)
        "bash_output", "bash_kill" -> target = s("id")
        "web_search" -> target = oneLine(s("query"), 120)
        "knowledge_search" -> { target = if (s("query").isNotEmpty()) "\"${oneLine(s("query"), 110)}\"" else ""; meta = s("collection") }
        "knowledge_note" -> { target = oneLine(s("title").ifEmpty { s("text") }, 110); meta = s("collection") }
        "web_fetch" -> { target = s("url"); if (s("find").isNotEmpty()) meta = "\"${oneLine(s("find"), 40)}\"" }
        "read_image", "office_read", "office_open", "office_create" -> target = s("path")
        "office_edit" -> { target = s("path"); meta = listOf(s("op"), s("sheet"), s("cell")).filter { it.isNotEmpty() }.joinToString(" · ") }
        "gh_issue", "gh_pr" -> {
            target = listOf(s("action"), if (n("number") > 0) "#${n("number")}" else "",
                if (s("query").isNotEmpty()) "\"${oneLine(s("query"), 60)}\"" else "").filter { it.isNotEmpty() }.joinToString(" ")
            meta = s("repo")
        }
        "git_commit" -> {
            target = oneLine(s("message").lineSequence().firstOrNull(), 120)
            val paths = (o["paths"] as? List<*>)?.size ?: 0
            meta = if (paths > 0) "$paths ${t["files"]}" else if (o["all"] == true) "all" else ""
        }
        "gh_pr_create" -> { target = oneLine(s("title"), 120); meta = listOf(if (s("base").isNotEmpty()) "→ ${s("base")}" else "", if (o["draft"] == true) "draft" else "").filter { it.isNotEmpty() }.joinToString(" · ") }
        "delegate" -> {
            val tasks = o["tasks"] as? List<*> ?: emptyList<Any?>()
            target = oneLine(((tasks.firstOrNull() as? Map<*, *>)?.get("prompt") as? String)?.lineSequence()?.firstOrNull(), 110)
            if (tasks.size > 1) meta = "${tasks.size} ${t["tasks"]}"
        }
        "question" -> target = oneLine(s("query"), 140)
        "dashboard_save" -> { target = s("dashboard"); (o["plots"] as? List<*>)?.let { meta = "${it.size} ${if (it.size == 1) t["chart1"] else t["charts"]}" } }
        "skill" -> target = s("nom").ifEmpty { s("name") }
        "preview" -> target = s("title").ifEmpty { s("kind") }
        "browser" -> target = listOf(s("action"), s("url").ifEmpty { s("target").ifEmpty { s("key") } }).filter { it.isNotEmpty() }.joinToString(" ")
        "todowrite", "todoread" -> {}
        else -> {
            Regex("^mcp_([^_]+)_(.+)$").find(tool)?.let { label = "${t["mcp"]} · ${it.groupValues[1]} › ${it.groupValues[2]}" }
            target = oneLine(o.entries.joinToString(" · ") { "${it.key}: ${jsonInline(it.value)}" }, 160)
        }
    }
    return ToolSum(label, target, meta)
}

// --- Model del treball d'un torn ---

enum class StepKind { Tool, Think, Note, Blocked, Review, Delegate, Info, Warn }
enum class StepState { Running, Ok, Failed }

data class WorkStep(
    val id: String,
    val kind: StepKind,
    val label: String,
    val target: String = "",
    val meta: String = "",
    val detail: String = "",
    val state: StepState = StepState.Ok,
    val tool: String = "",
)

data class WorkGroup(
    val steps: List<WorkStep> = emptyList(),
    val started: Long = 0L,
    val ended: Long = 0L,
    val failed: Boolean = false,
) {
    val running: Boolean get() = ended == 0L
    val toolCount: Int get() = steps.count { it.kind == StepKind.Tool }
    val failedCount: Int get() = steps.count { it.state == StepState.Failed }

    fun add(s: WorkStep): WorkGroup = copy(steps = steps + s)

    /** Tanca el pas d'eina amb aquest id (o l'últim en curs amb el mateix nom). */
    fun finishTool(id: String, tool: String, output: String, failed: Boolean): WorkGroup {
        var idx = if (id.isNotEmpty()) steps.indexOfLast { it.id == id } else -1
        if (idx < 0) idx = steps.indexOfLast { it.kind == StepKind.Tool && it.state == StepState.Running && (tool.isEmpty() || it.tool == tool) }
        if (idx < 0) return this
        val s = steps[idx]
        val upd = s.copy(state = if (failed) StepState.Failed else StepState.Ok, detail = output.take(4000))
        return copy(steps = steps.toMutableList().also { it[idx] = upd })
    }

    /** Actualitza (o afegeix) un pas per id: subagents que canvien d'estat. */
    fun upsert(s: WorkStep): WorkGroup {
        val idx = steps.indexOfFirst { it.id == s.id }
        return if (idx < 0) add(s) else copy(steps = steps.toMutableList().also { it[idx] = s })
    }

    /** Tanca el torn: els passos encara oberts queden com a acabats. */
    fun close(at: Long, failed: Boolean): WorkGroup = copy(
        ended = at, failed = failed,
        steps = steps.map { if (it.state == StepState.Running) it.copy(state = if (failed) StepState.Failed else StepState.Ok) else it },
    )
}

/** Una sortida d'eina compta com a fallada si el servidor la marca així. */
fun toolOutputFailed(output: String): Boolean {
    val t = output.trimStart()
    return t.startsWith("ERROR", ignoreCase = false) || t.startsWith("error:", ignoreCase = true) ||
        t.startsWith("⛔") || t.startsWith("denied", ignoreCase = true)
}

/** Durada curta: 8s, 1m 05s, 1h 02m. */
fun shortDuration(ms: Long): String {
    val s = (ms / 1000).coerceAtLeast(0)
    return when {
        s < 60 -> "${s}s"
        s < 3600 -> "${s / 60}m ${(s % 60).toString().padStart(2, '0')}s"
        else -> "${s / 3600}h ${((s % 3600) / 60).toString().padStart(2, '0')}m"
    }
}

/**
 * Línia d'activitat en viu a partir d'un event `progress` del servidor
 * (internal/web/progres.go): fase model/tools/review/compact i estat.
 * Torna la clau i el text; la clau canvia quan canvia el que fa.
 */
fun progressLine(p: Map<String, String>, lang: String = "ca"): Pair<String, String> {
    val en = lang == "en"
    return when (p["phase"]) {
        "tools" -> {
            val tools = p["tools"].orEmpty()
            val label = tools.split(',', ' ').firstOrNull { it.isNotBlank() }?.let { summarizeTool(it, "", lang).label } ?: ""
            ("tools:$tools") to (if (en) "Running" else "Executant") + if (label.isNotEmpty()) " · $label" else "…"
        }
        "review" -> "review" to (if (en) "Reviewing the change…" else "Revisant el canvi…")
        "compact" -> "compact" to (if (en) "Compacting the context…" else "Compactant el context…")
        else -> when (p["state"]) {
            // Comptadors de caràcters de la crida en curs (progres.go).
            "receiving" -> when {
                (p["tool"]?.toIntOrNull() ?: 0) > 0 -> "plan" to (if (en) "Preparing the next step…" else "Preparant el pas següent…")
                (p["text"]?.toIntOrNull() ?: 0) == 0 && (p["reasoning"]?.toIntOrNull() ?: 0) > 0 ->
                    "think" to (if (en) "Thinking…" else "Pensant…")
                else -> "write" to (if (en) "Writing…" else "Escrivint…")
            }
            else -> "model" to (if (en) "Waiting for the model…" else "Esperant el model…")
        }
    }
}

/**
 * Error del servidor en paraules: el mateix diagnòstic que el retro de la
 * web (retroDiagnose), però per al xat. Torna el text original si no el
 * reconeix (millor el missatge real que un genèric).
 */
fun friendlyError(raw: String, lang: String = "ca"): String {
    val l = raw.lowercase()
    val en = lang == "en"
    fun pick(e: String, c: String) = if (en) e else c
    val hint = when {
        "http 401" in l || "http 403" in l || "unauthorized" in l || "invalid api key" in l ->
            pick("The provider rejected the API key (401/403). Check the key in the model menu.",
                "El proveïdor ha rebutjat la clau d'API (401/403). Revisa-la al menú del model.")
        "model is unavailable" in l || "model not found" in l || "http 404" in l ->
            pick("The provider says this model is unavailable. Pick another model at the top.",
                "El proveïdor diu que aquest model no està disponible. Tria'n un altre a dalt.")
        "http 429" in l || "rate limit" in l ->
            pick("The provider is rate-limiting (429). Wait a moment and try again.",
                "El proveïdor està limitant les peticions (429). Espera una mica i torna-ho a provar.")
        "inactivity" in l || "time limit" in l || "deadline exceeded" in l ->
            pick("The model went quiet for too long and the step was stopped. Try again or pick a faster model.",
                "El model ha estat massa estona sense respondre i s'ha aturat el pas. Torna-ho a provar o tria un model més ràpid.")
        "connection refused" in l || "no such host" in l ->
            pick("The server couldn't reach the provider. Check its address and the connection.",
                "El servidor no ha pogut contactar amb el proveïdor. Revisa'n l'adreça i la connexió.")
        else -> ""
    }
    val clean = oneLine(raw.replace(Regex("https?://\\S+"), "…"), 220)
    return if (hint.isEmpty()) clean else if (clean.isEmpty()) hint else "$hint\n$clean"
}

/** Què fa l'agent ara (clau estable per fase) i des de quan. */
data class Activity(val key: String, val text: String, val since: Long)
