package xyz.solanes.gregal

/** Model de dades de l'API de `gregal serve` (rutes /api/...). */

data class ChatMsg(val role: String, val text: String, val kind: String = "text", val at: Long = 0L)
// kind: text | activity | error | goal — at: ms d'època (0 = sense segell)

data class ParallelResult(
    val role: String,
    val provider: String,
    val model: String,
    val reply: String,
    val error: String = "",
)

data class GithubResult(val output: String = "", val error: String = "")

/** Estat del document office pujat al servidor (per id). */
data class OfficeDoc(val id: String = "", val name: String = "", val kind: String = "",
                     val text: String = "", val result: String = "", val error: String = "")

/** Proveïdor del servidor: key ve emmascarada ((buida) | ${VAR} | sí). */
data class ProviderInfo(val name: String = "", val url: String = "",
                        val key: String = "", val usedBy: String = "") {
    fun hasKey(): Boolean = key.isNotBlank() && key != "(buida)"
}

/** Text visible d'un missatge del transcript (/api/resume): content string
 *  (cas normal) o per parts (missatges amb imatge: [{type:text,...},
 *  {type:image_url,...}]). Buit si és soroll (tool_calls sense text).
 *  Treballa sobre el JSON cru (com parseProviders): org.json no existeix
 *  als tests unitaris de la JVM. Pur, testeable. */
fun transcriptText(objJson: String): String {
    val ci = objJson.indexOf("\"content\"")
    if (ci < 0) return ""
    var i = objJson.indexOf(':', ci + 9)
    if (i < 0) return ""
    i++
    while (i < objJson.length && objJson[i].isWhitespace()) i++
    if (i >= objJson.length) return ""
    if (objJson[i] == '"') return unquoteJson(objJson, i)
    if (objJson[i] != '[') return ""
    // Array de parts: recull tots els "text" i marca les imatges.
    var d = 0
    var instr = false
    var esc = false
    var end = -1
    for (j in i until objJson.length) {
        val c = objJson[j]
        if (instr) {
            if (esc) esc = false else if (c == '\\') esc = true else if (c == '"') instr = false
        } else {
            when (c) {
                '"' -> instr = true
                '[' -> d++
                ']' -> { d--; if (d == 0) { end = j; break } }
            }
        }
    }
    if (end < 0) return ""
    val arr = objJson.substring(i, end + 1)
    val out = StringBuilder()
    val re = Regex("\"text\"\\s*:\\s*\"((?:[^\"\\\\]|\\\\.)*)\"")
    re.findAll(arr).forEach { m -> out.append(unescapeJson(m.groupValues[1])) }
    val imgs = Regex("\"type\"\\s*:\\s*\"image_url\"").findAll(arr).count()
    repeat(imgs) { out.append("🖼️") }
    return out.toString()
}

private fun unquoteJson(s: String, start: Int): String {
    val out = StringBuilder()
    var esc = false
    var i = start + 1
    while (i < s.length) {
        val c = s[i]
        if (esc) {
            when (c) {
                'n' -> out.append('\n')
                't' -> out.append('\t')
                'r' -> out.append('\r')
                'u' -> {
                    val hex = s.substring(i + 1, minOf(i + 5, s.length))
                    out.append(hex.toIntOrNull(16)?.toChar() ?: '?')
                    i += 4
                }
                else -> out.append(c)
            }
            esc = false
        } else {
            if (c == '\\') esc = true else if (c == '"') break else out.append(c)
        }
        i++
    }
    return out.toString()
}

private fun unescapeJson(s: String): String =
    s.replace("\\n", "\n").replace("\\t", "\t").replace("\\r", "\r")
        .replace("\\\"", "\"").replace("\\\\", "\\")
/** Llegeix la llista de GET /api/providers (pur, testeable). */
fun parseProviders(json: String): List<ProviderInfo> {
    val start = json.indexOf("\"providers\"")
    if (start < 0) return emptyList()
    val arrS = json.indexOf('[', start)
    if (arrS < 0) return emptyList()
    // Tanca l'array per balanceig (hi ha més camps després).
    var arrE = -1
    var d = 0
    var instr = false
    var esc = false
    for (i in arrS until json.length) {
        val c = json[i]
        if (instr) {
            if (esc) esc = false else if (c == '\\') esc = true else if (c == '"') instr = false
        } else {
            when (c) {
                '"' -> instr = true
                '[' -> d++
                ']' -> { if (--d == 0) { arrE = i; break } }
            }
        }
    }
    if (arrE <= arrS) return emptyList()
    return splitObjects(json.substring(arrS + 1, arrE)).map { o ->
        // used_by és array ["a/b"]: l'aplanem a text perquè parseFlat
        // talla pels carrers interiors.
        val flat = o.replace(Regex("\"used_by\"\\s*:\\s*\\[([^\\]]*)\\]")) { mr ->
            "\"used_by\":\"" + mr.groupValues[1].replace("\"", "").trim() + "\""
        }
        val m = parseFlat(flat)
        ProviderInfo(m["name"] ?: "", m["url"] ?: "", m["key"] ?: "",
            m["used_by"] ?: "")
    }
}

data class ServerState(
    val roles: List<String> = emptyList(),
    val currentRole: String = "",
    val mode: String = "",
    val model: String = "",
    val provider: String = "",
    val verify: String = "manual",
    val agentBusy: Boolean = false,
    val cwd: String = "",
    val project: String = "",
    val branch: String = "",
    // Mesurador de context (J4 de la web): el servidor estima el que ocupa la
    // conversa i diu la finestra del rol. contextWindow 0 = el rol no la
    // declara i el mesurador s'amaga, com a la web i al TUI.
    val usedTokens: Int = 0,
    val contextWindow: Int = 0,
) {
    /** Percentatge de context ocupat (0 si el rol no declara finestra). */
    val ctxPct: Int get() = contextPct(usedTokens, contextWindow)
}

/** Pregunta de l'agent (`question_request`): espera tria de l'usuari. */
data class QuestionOption(val label: String, val description: String)
data class QuestionReq(
    val key: String,
    val callId: String,
    val query: String,
    val options: List<QuestionOption>,
)

/** Opcions de /api/question: [{label, description}, …]. */
fun parseQuestionOptions(arr: org.json.JSONArray?): List<QuestionOption> {
    if (arr == null) return emptyList()
    return (0 until arr.length()).mapNotNull { i ->
        val o = arr.optJSONObject(i) ?: return@mapNotNull null
        QuestionOption(o.optString("label"), o.optString("description"))
    }
}

/** Nom llegible del rol (la web amaga think/reviewer tècnics). */
fun roleLabel(r: String, lang: String = "ca"): String {
    val key = when (r) {
        "chat" -> "app.role.chat"
        "think" -> "app.role.think"
        "code" -> "app.role.code"
        "reviewer" -> "app.role.reviewer"
        else -> return r
    }
    return tr(lang, key)
}

data class SessionInfo(val name: String, val msgs: Int, val moment: String,
                       val title: String = "", val at: String = "", val current: Boolean = false) {
    fun displayTitle(): String = title.ifBlank { name }
}

/** Graf del projecte (.gregal/flows): resum, node i detall per a la UI. */
data class FlowSummary(val name: String, val slug: String, val desc: String, val steps: Int)
data class FlowNode(val id: String, val kind: String, val title: String,
                    val task: String, val tool: String, val text: String) {
    fun label(): String = title.ifBlank { id }
    fun detail(): String = when (kind) {
        "agent" -> task
        "tool" -> tool
        else -> text
    }.take(160)
}
data class FlowEdge(val from: String, val to: String, val when_: String)
data class FlowDetail(val name: String, val desc: String,
                      val nodes: List<FlowNode>, val edges: List<FlowEdge>)
data class FlowStep(val node: String, val title: String, val error: String, val ms: Long)

/** Seccions estil ChatGPT: Avui, Ahir, Setmana, Mes, Abans (com convs.js). */
fun convGroup(at: String, now: Long = System.currentTimeMillis()): String {
    return try {
        val t = java.time.Instant.parse(at).toEpochMilli()
        val day = 86400000L
        val startToday = now - (now % day)
        val d = ((startToday - (t - (t % day))) / day).toInt()
        when {
            d <= 0 -> "today"
            d == 1 -> "yesterday"
            d < 7 -> "week"
            d < 30 -> "month"
            else -> "older"
        }
    } catch (_: Exception) {
        "older"
    }
}

fun convGroupLabel(g: String, lang: String): String = when (g) {
    "today" -> tr(lang, "app.sess.today")
    "yesterday" -> tr(lang, "app.sess.yesterday")
    "week" -> tr(lang, "app.sess.week")
    "month" -> tr(lang, "app.sess.month")
    else -> tr(lang, "app.sess.older")
}

data class LiveEvent(val kind: String, val text: String)

data class ActiveAgent(
    val id: Int,
    val task: String,
    val startedAt: String,
    val role: String,
    val mode: String,
    val project: String,
    val events: List<LiveEvent>,
)

data class GoalItem(val id: String, val title: String, val body: String, val status: String)

fun goalStatusLabel(status: String, lang: String = "ca"): String =
    tr(lang, if (status == "fet") "app.goals.completed" else "app.goals.pending")

/** Job programat del servidor (butlletins desatesos). */
data class JobItem(
    val id: String = "",
    val name: String = "",
    val prompt: String = "",
    val kind: String = "manual", // daily | interval | manual
    val time: String = "08:00",
    val intervalH: Double = 24.0,
    val enabled: Boolean = true,
    val notify: Boolean = true,
    val lastStatus: String = "",
    val nextDue: String = "",
)

/** Execució d'un job (output en markdown per al render ric). */
data class RunItem(
    val id: String,
    val jobId: String,
    val jobName: String,
    val startedAt: String,
    val finishedAt: String,
    val status: String,
    val output: String,
    val error: String,
)

data class ApproveReq(val key: String, val callId: String, val name: String, val args: String)

data class CheckpointInfo(val seq: Int, val op: String, val path: String, val at: String)

/** Parseja la llista de /api/checkpoints (pura: testeable sense servidor ni
 *  org.json — els unit tests JVM no tenen el stub d'Android).
 *  Només entén la forma del servidor: array d'objectes plans amb valors
 *  string (amb escapaments \" \\ /) o enters. Prou per a checkpoints. */
fun parseCheckpoints(json: String): List<CheckpointInfo> {
    val objs = splitObjects(json.trim().removePrefix("[").removeSuffix("]"))
    return objs.map { o ->
        val m = parseFlat(o)
        CheckpointInfo(m["seq"]?.toIntOrNull() ?: 0, m["op"] ?: "", m["path"] ?: "", m["at"] ?: "")
    }
}

private fun splitObjects(s: String): List<String> {
    val out = mutableListOf<String>()
    var depth = 0
    var instr = false
    var esc = false
    var start = -1
    for (i in s.indices) {
        val c = s[i]
        if (instr) {
            if (esc) esc = false else if (c == '\\') esc = true else if (c == '"') instr = false
        } else {
            when (c) {
                '"' -> instr = true
                '{' -> { if (depth++ == 0) start = i }
                '}' -> { if (--depth == 0 && start >= 0) { out.add(s.substring(start, i + 1)); start = -1 } }
            }
        }
    }
    return out
}

private fun parseFlat(obj: String): Map<String, String> {
    val m = mutableMapOf<String, String>()
    var i = obj.indexOf('{') + 1
    while (i < obj.length) {
        while (i < obj.length && (obj[i].isWhitespace() || obj[i] == ',' || obj[i] == '}')) i++
        if (i >= obj.length || obj[i] != '"') break
        val (key, ni) = readStr(obj, i)
        i = ni
        while (i < obj.length && (obj[i].isWhitespace() || obj[i] == ':')) i++
        if (i < obj.length && obj[i] == '"') {
            val (v, nj) = readStr(obj, i)
            m[key] = v
            i = nj
        } else {
            val j = i
            while (i < obj.length && obj[i] !in ",}") i++
            m[key] = obj.substring(j, i).trim()
        }
    }
    return m
}

private fun readStr(s: String, from: Int): Pair<String, Int> {
    val b = StringBuilder()
    var i = from + 1
    while (i < s.length) {
        val c = s[i]
        if (c == '\\' && i + 1 < s.length) {
            b.append(s[i + 1])
            i += 2
        } else if (c == '"') {
            return b.toString() to i + 1
        } else {
            b.append(c)
            i++
        }
    }
    return b.toString() to i
}

/** Esdeveniment SSE del servidor. */
data class SseEvent(val name: String, val data: String)

/** Descoberta del contracte compartit del servei. */
data class ServiceHealth(
    val status: String = "",
    val protocol: String = "",
    val instanceId: String = "",
    val capabilities: Map<String, Boolean> = emptyMap(),
) {
    /** La cua v2 només s'activa quan el servidor anuncia les dues peces. */
    fun supportsV2RunsEvents(): Boolean =
        status == "ok" && capabilities["runs"] == true && capabilities["events"] == true

    /**
     * Interaccions v2 només són segures si el servidor conserva el payload
     * estructurat (key/call_id/options), no només el text d'activitat.
     * Mantinguem la via legacy sencera mentre aquesta capacitat no hi sigui.
     */
    fun supportsV2Interactive(): Boolean =
        status == "ok" && capabilities["runs"] == true &&
            capabilities["durable_events"] == true &&
            (capabilities["events"] == true || capabilities["event_stream"] == true) &&
            capabilities["interactive_events"] == true
}

data class RunV2(
    val id: Long = 0L,
    val state: String = "",
    val session: String = "",
    val workspace: String = "",
    val error: String = "",
)

data class DurableEvent(
    val id: Long = 0L,
    val sessionId: String = "",
    val runId: Long = 0L,
    val kind: String = "",
    val text: String = "",
    /** JSON estructurat d'una aprovació/pregunta, si el servidor l'envia. */
    val payload: String = "",
)

/** Extreu el payload d'una interacció durable en qualsevol forma compatible. */
fun structuredInteractivePayload(eventJson: String): String {
    val e = try { org.json.JSONObject(eventJson) } catch (_: Exception) { return "" }
    fun objectWithKey(value: Any?): org.json.JSONObject? {
        val o = when (value) {
            is org.json.JSONObject -> value
            is String -> try { org.json.JSONObject(value) } catch (_: Exception) { null }
            else -> null
        } ?: return null
        for (key in listOf("payload", "interaction", "data")) {
            objectWithKey(o.opt(key))?.let { return it }
        }
        return o.takeIf { it.optString("key").isNotBlank() }
    }
    for (key in listOf("payload", "data", "details", "interaction", "meta")) {
        objectWithKey(e.opt(key))?.let { return it.toString() }
    }
    if (e.optString("key").isNotBlank()) {
        val out = org.json.JSONObject().put("key", e.optString("key"))
        for (key in listOf("call_id", "name", "args", "query", "options")) {
            if (e.has(key)) out.put(key, e.opt(key))
        }
        return out.toString()
    }
    return objectWithKey(e.optString("text"))?.toString() ?: ""
}

data class EventsPage(
    val events: List<DurableEvent> = emptyList(),
    val cursor: Long = 0L,
    val next: Long = 0L,
)

/** Parser SSE pur (testeable sense Android). */
object SseParser {
    /**
     * Consumeix un tros de text i retorna els events complets (separats per
     * línia en blanc). `carry` és el resto incomplet de la crida anterior.
     */
    fun feed(chunk: String, carry: String): Pair<List<SseEvent>, String> {
        val buf = carry + chunk
        val events = mutableListOf<SseEvent>()
        var ev = ""
        var data = StringBuilder()
        var start = 0
        var i = 0
        var lastEnd = 0
        while (i < buf.length) {
            val nl = buf.indexOf('\n', i)
            if (nl < 0) break
            val line = buf.substring(i, nl).trimEnd('\r')
            i = nl + 1
            if (line.isEmpty()) {
                if (ev.isNotEmpty() && data.isNotEmpty()) {
                    events += SseEvent(ev, data.toString())
                }
                ev = ""
                data = StringBuilder()
                lastEnd = i
            } else if (line.startsWith("event:")) {
                ev = line.substringAfter("event:").trim()
            } else if (line.startsWith("data:")) {
                data.append(line.substringAfter("data:").trim())
            }
        }
        start = lastEnd
        return events to buf.substring(start)
    }
}

/** Construeix la base URL normalitzada. */
fun baseUrlOf(host: String, port: String): String {
    val h = host.trim().ifEmpty { "127.0.0.1" }
    val p = port.trim().ifEmpty { "8097" }
    val withScheme = if (h.contains("://")) h else "http://$h"
    return withScheme.trimEnd('/') + ":" + p
}


/** Qui hi ha darrere la connexio (GET /api/me). */
data class MeInfo(
    val user: String = "",
    val home: String = "",
    val admin: Boolean = false,
    val roots: List<String> = emptyList(),
)
