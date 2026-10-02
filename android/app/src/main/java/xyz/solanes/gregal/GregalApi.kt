package xyz.solanes.gregal

import org.json.JSONArray
import org.json.JSONObject

import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL
import java.nio.charset.StandardCharsets
import java.net.URLEncoder


/** Payload de /api/plan (pur, testeable sense Android). */
fun planPayload(task: String): Map<String, String> = mapOf("task" to task)

/** Payload de /api/office/edit (pur, testeable sense Android). */
fun officeEditPayload(id: String, op: String, sheet: String = "", cell: String = "",
                      value: String = "", find: String = "", replace: String = ""): Map<String, String> =
    mapOf("id" to id, "op" to op, "sheet" to sheet, "cell" to cell,
        "value" to value, "find" to find, "replace" to replace)

/**
 * Client de `gregal serve`. HttpURLConnection de la stdlib (sense
 * dependències) + org.json (inclòs a Android). El token va per capçalera
 * Bearer (o el servidor el rebutja amb 401).
 */
/**
 * Sessió del servidor que fa servir l'app. El servidor té diverses sessions
 * vives (a la web són pestanyes: la primera és `default`, «principal») i
 * cada petició en tria una amb la capçalera `X-Gregal-Session`.
 *
 * Sense això l'app queia SEMPRE a `default`, o sigui la pestanya principal
 * de la web: veies la conversa de l'escriptori i «nova conversa» esborrava
 * la SEVA conversa en comptes de la del mòbil. L'id ha de casar amb
 * `^[A-Za-z0-9_-]{1,64}$` (sessIDRe del hub).
 */
@Volatile
var sessioApp: String = "mobil"

class GregalApi(val base: String, val token: String) {

    private fun conn(path: String, method: String, body: String? = null): HttpURLConnection {
        val c = URL(base + path).openConnection() as HttpURLConnection
        c.requestMethod = method
        c.connectTimeout = 15_000
        c.readTimeout = 120_000
        if (token.isNotBlank()) c.setRequestProperty("Authorization", "Bearer $token")
        // Cada dispositiu, la seva conversa.
        if (sessioApp.isNotBlank()) c.setRequestProperty("X-Gregal-Session", sessioApp)
        if (body != null) {
            c.doOutput = true
            c.setRequestProperty("Content-Type", "application/json")
            c.outputStream.use { it.write(body.toByteArray(StandardCharsets.UTF_8)) }
        }
        return c
    }

    private fun read(c: HttpURLConnection): String {
        val code = c.responseCode
        val stream = if (code in 200..299) c.inputStream else (c.errorStream ?: c.inputStream)
        val text = stream.bufferedReader(StandardCharsets.UTF_8).use { it.readText() }
        if (code == 401) throw ApiException("El servidor demana token (401). Revisa el token.")
        if (code !in 200..299) throw ApiException("HTTP $code: ${text.take(200)}")
        return text
    }

    /** Descobreix capacitats abans d'usar la cua v2. */
    fun health(): ServiceHealth {
        val o = JSONObject(read(conn("/api/health", "GET")))
        val caps = mutableMapOf<String, Boolean>()
        val c = o.optJSONObject("capabilities")
        if (c != null) for (k in c.keys()) caps[k] = c.optBoolean(k)
        return ServiceHealth(
            status = o.optString("status"), protocol = o.optString("protocol"),
            instanceId = o.optString("instance_id"), capabilities = caps,
        )
    }

    fun getState(): ServerState {
        val o = JSONObject(read(conn("/api/state", "GET")))
        return ServerState(
            roles = o.optJSONArray("role")?.toStrList() ?: emptyList(),
            currentRole = o.optString("current_role"),
            mode = o.optString("mode"),
            model = o.optString("model"),
            provider = o.optString("provider"),
            verify = o.optString("verify", "manual"),
            agentBusy = o.optBoolean("agent_busy"),
            cwd = o.optString("cwd"),
            project = o.optString("project"),
            branch = o.optString("branch"),
            usedTokens = o.optInt("used_tokens"),
            contextWindow = o.optInt("context_window"),
        )
    }

    fun getModels(): Pair<Map<String, List<String>>, String> {
        val o = JSONObject(read(conn("/api/models", "GET")))
        val out = mutableMapOf<String, List<String>>()
        val m = o.optJSONObject("models") ?: JSONObject()
        for (k in m.keys()) out[k] = m.optJSONArray(k)?.toStrList() ?: emptyList()
        return out to o.optString("current")
    }

    fun post(path: String, payload: JSONObject): JSONObject {
        return JSONObject(read(conn(path, "POST", payload.toString())))
    }

    /**
     * Entra amb usuari i contrasenya (POST /api/login) i torna el token de
     * sessio. El servidor el desa a passwords.json/tokens.json i el token
     * dura fins que es tanqui la sessio.
     *
     * Serveix per al servidor amb usuaris configurats; si el servidor no en
     * te cap, la resposta porta un error i el cami bo es el token de sempre.
     */
    fun login(usuari: String, contrasenya: String): String {
        val o = post("/api/login", JSONObject().put("user", usuari).put("password", contrasenya))
        return o.optString("token")
    }

    /** Qui soc (GET /api/me): nom, arrels, carpeta de treball i si es admin. */
    fun me(): MeInfo {
        val o = JSONObject(read(conn("/api/me", "GET")))
        return MeInfo(
            user = o.optString("user"),
            home = o.optString("home"),
            admin = o.optBoolean("admin"),
            roots = o.optJSONArray("roots")?.toStrList() ?: emptyList(),
        )
    }

    /** Respon la pregunta de l'agent (POST /api/question {key, answer}). */
    fun answerQuestion(key: String, answer: String): Boolean {
        val o = post("/api/question", JSONObject().put("key", key).put("answer", answer))
        return o.optBoolean("ok")
    }

    /** Compara els rols configurats alhora (local + cloud/revisor). */
    fun parallel(message: String): List<ParallelResult> {
        val root = post("/api/parallel", JSONObject().put("message", message))
        val a = root.optJSONArray("results") ?: JSONArray()
        return (0 until a.length()).map { i ->
            val o = a.getJSONObject(i)
            ParallelResult(
                role = o.optString("role"), provider = o.optString("provider"),
                model = o.optString("model"), reply = o.optString("reply"),
                error = o.optString("error"),
            )
        }
    }

    fun github(tool: String, action: String, number: Int = 0, query: String = "", repo: String = ""): GithubResult {
        return try {
            val o = post("/api/github", JSONObject()
                .put("tool", tool).put("action", action).put("number", number)
                .put("query", query).put("repo", repo))
            GithubResult(output = o.optString("output"))
        } catch (e: Exception) {
            GithubResult(error = e.message ?: "error GitHub")
        }
    }

    /** Office: puja un docx/xlsx/pptx (base64) → id per llegir/editar al servidor. */
    fun officeUpload(name: String, dataB64: String): OfficeDoc {
        return try {
            val o = post("/api/office/upload", JSONObject().put("name", name).put("data", dataB64))
            OfficeDoc(id = o.optString("id"), name = o.optString("name"),
                kind = o.optString("kind"), text = "", result = "pujat ${o.optInt("size")} bytes")
        } catch (e: Exception) {
            OfficeDoc(error = e.message ?: "error pujant")
        }
    }

    /** Office: llegeix el text/dades del document pujat. */
    fun officeRead(id: String): OfficeDoc {
        return try {
            val o = post("/api/office/read", JSONObject().put("id", id))
            OfficeDoc(id = id, name = o.optString("name"), kind = o.optString("kind"),
                text = o.optString("text"))
        } catch (e: Exception) {
            OfficeDoc(id = id, error = e.message ?: "error llegint")
        }
    }

    /** Office: set_cell (xlsx) o replace (docx/pptx) → resultat + text fresc. */
    fun officeEdit(id: String, op: String, sheet: String = "", cell: String = "",
                   value: String = "", find: String = "", replace: String = ""): OfficeDoc {
        return try {
            val o = post("/api/office/edit", JSONObject(officeEditPayload(id, op, sheet, cell, value, find, replace)))
            OfficeDoc(id = id, text = o.optString("text"), result = o.optString("result"))
        } catch (e: Exception) {
            OfficeDoc(id = id, error = e.message ?: "error editant")
        }
    }

    /** Office: baixa el fitxer editat (bytes per desar al mòbil). */
    fun officeDownload(id: String): ByteArray {
        val c = conn("/api/office/download?id=" + id, "GET")
        val code = c.responseCode
        if (code == 401) throw ApiException("El servidor demana token (401). Revisa el token.")
        if (code !in 200..299) throw ApiException("HTTP $code baixant el document")
        return c.inputStream.use { it.readBytes() }
    }

    /** Proveïdors: llista del servidor amb estat de clau (emmascarada). */
    fun getProviders(): List<ProviderInfo> {
        val c = conn("/api/providers", "GET")
        val code = c.responseCode
        if (code == 401) throw ApiException("El servidor demana token (401). Revisa el token.")
        if (code !in 200..299) throw ApiException("HTTP $code llistant proveïdors")
        return parseProviders(read(c))
    }

    /** Desa la clau d'un proveïdor (servidor: en memòria, o ${VAR} per persistir). */
    fun setProviderKey(name: String, key: String): String {
        val o = post("/api/providers", JSONObject().put("action", "key").put("name", name).put("key", key))
        return o.optString("msg").ifBlank { "desat" }
    }

    /** Prova la connexió d'un proveïdor (GET /models amb la seva clau). */
    fun testProvider(name: String): String {
        val o = post("/api/providers", JSONObject().put("action", "test").put("name", name))
        return o.optString("msg").ifBlank { "ok" }
    }

    /** Pla read-only: /api/plan {"task"} → text del pla. */
    fun postPlan(task: String): String {
        return post("/api/plan", JSONObject(planPayload(task))).optString("plan")
    }

    /** Checkpoints del journal: GET /api/checkpoints → llista. */
    fun getCheckpoints(): List<CheckpointInfo> =
        parseCheckpoints(read(conn("/api/checkpoints", "GET")))

    /** Desfés-ho tot: POST /api/rewind → resum. */
    fun postRewind(): String =
        post("/api/rewind", JSONObject()).optString("summary", "canvis desfets")

    /** Torna al checkpoint seq: POST /api/rewind-to → resum. */
    fun postRewindTo(seq: Int): String =
        post("/api/rewind-to", JSONObject().put("seq", seq)).optString("summary", "canvis desfets")

    fun getSessions(): List<SessionInfo> {
        val a = JSONArray(read(conn("/api/sessions", "GET")))
        return (0 until a.length()).map {
            val o = a.getJSONObject(it)
            SessionInfo(o.optString("name"), o.optInt("msgs"), o.optString("when"),
                o.optString("title"), o.optString("at"), o.optBoolean("current"))
        }
    }

    // ---- Contracte de torns v2 (cua + events durables) ----

    /** Envia un torn a la cua compartida; retorna immediatament. */
    fun submitRun(task: String, mode: String = "", images: List<String> = emptyList(),
                  idempotencyKey: String = ""): RunV2 {
        val payload = JSONObject().put("task", task)
        if (mode.isNotBlank()) payload.put("mode", mode)
        if (images.isNotEmpty()) payload.put("images", JSONArray(images))
        if (idempotencyKey.isNotBlank()) payload.put("idempotency_key", idempotencyKey)
        val root = post("/api/v2/runs", payload)
        return parseRun(root.optJSONObject("run") ?: root)
    }

    /** Llegeix events persistents posteriors al cursor, filtrats per sessió. */
    fun getEvents(after: Long, limit: Int = 100, session: String = sessioApp): EventsPage {
        val path = "/api/v2/events?after=$after&limit=$limit&session=" +
            URLEncoder.encode(session, "UTF-8")
        val root = JSONObject(read(conn(path, "GET")))
        val a = root.optJSONArray("events") ?: JSONArray()
        val events = (0 until a.length()).map { i ->
            val e = a.getJSONObject(i)
            DurableEvent(
                id = e.optLong("id"), sessionId = e.optString("session_id"),
                runId = e.optLong("run_id"), kind = e.optString("kind"),
                text = e.optString("text"), payload = structuredInteractivePayload(e.toString()),
            )
        }
        return EventsPage(events, root.optLong("cursor", after), root.optLong("next", after))
    }

    /** Consulta l'estat d'un torn concret. */
    fun getRun(id: Long): RunV2 {
        val root = JSONObject(read(conn("/api/v2/runs/$id", "GET")))
        return parseRun(root.optJSONObject("run") ?: root)
    }

    /** Cancel·la un torn concret de la cua v2. */
    fun cancelRun(id: Long): Boolean {
        val root = post("/api/v2/runs/$id/cancel", JSONObject())
        return root.optBoolean("cancelled", root.optBoolean("ok"))
    }

    /** Cancel·lació de compatibilitat per servidors antics. */
    fun cancelLegacy(): Boolean = post("/api/agent/cancel", JSONObject()).optBoolean("ok", true)

    private fun parseRun(o: JSONObject): RunV2 = RunV2(
        id = o.optLong("id"), state = o.optString("state"),
        session = o.optString("session"), workspace = o.optString("workspace"),
        error = o.optString("error"),
    )


    /** Esborra una conversa (DELETE /api/sessions?name=). */
    fun deleteSession(name: String) {
        read(conn("/api/sessions?name=" + java.net.URLEncoder.encode(name, "UTF-8"), "DELETE"))
    }

    /** Mode del revisor: off|manual|auto|both|strict (paritat prefs web). */
    fun setVerify(mode: String) {
        post("/api/verify", JSONObject().put("mode", mode))
    }

    // ---- Grafs (/api/flows*): llistar, veure, esborrar i executar. ----

    fun getFlows(): List<FlowSummary> {
        val o = JSONObject(read(conn("/api/flows", "GET")))
        val a = o.optJSONArray("flows") ?: JSONArray()
        return (0 until a.length()).map {
            val f = a.getJSONObject(it)
            FlowSummary(f.optString("name"), f.optString("slug"),
                f.optString("desc"), f.optInt("steps"))
        }
    }

    fun getFlow(name: String): FlowDetail {
        val o = JSONObject(read(conn("/api/flows/load?name=" + enc(name), "GET")))
        val f = o.getJSONObject("flow")
        val nodes = (f.optJSONArray("nodes") ?: JSONArray()).let { a ->
            (0 until a.length()).map {
                val n = a.getJSONObject(it)
                FlowNode(n.optString("id"), n.optString("kind"), n.optString("title"),
                    n.optString("task"), n.optString("tool"), n.optString("text"))
            }
        }
        val edges = (f.optJSONArray("edges") ?: JSONArray()).let { a ->
            (0 until a.length()).map {
                val e = a.getJSONObject(it)
                FlowEdge(e.optString("from"), e.optString("to"), e.optString("when"))
            }
        }
        return FlowDetail(f.optString("name"), f.optString("desc"), nodes, edges)
    }

    fun deleteFlow(name: String) {
        post("/api/flows/delete", JSONObject().put("name", name))
    }

    private fun enc(s: String): String = java.net.URLEncoder.encode(s, "UTF-8")

    fun getActive(): ActiveAgent? {
        val root = JSONObject(read(conn("/api/active", "GET")))
        if (root.isNull("active")) return null
        val a = root.optJSONObject("active") ?: return null
        val events = a.optJSONArray("events") ?: JSONArray()
        return ActiveAgent(
            id = a.optInt("id"), task = a.optString("task"), startedAt = a.optString("started_at"),
            role = a.optString("role"), mode = a.optString("mode"), project = a.optString("project"),
            events = (0 until events.length()).map { i ->
                val e = events.getJSONObject(i)
                LiveEvent(e.optString("kind", "activity"), e.optString("text"))
            },
        )
    }

    fun getGoals(): List<GoalItem> {
        val o = JSONObject(read(conn("/api/goal", "GET")))
        val a = o.optJSONArray("goals") ?: JSONArray()
        return (0 until a.length()).map {
            val g = a.getJSONObject(it)
            GoalItem(g.optString("id"), g.optString("title"), g.optString("body"), g.optString("status"))
        }
    }

    fun getJobs(): List<JobItem> {
        val o = JSONObject(read(conn("/api/jobs", "GET")))
        val a = o.optJSONArray("jobs") ?: JSONArray()
        return (0 until a.length()).map {
            val j = a.getJSONObject(it)
            JobItem(
                id = j.optString("id"), name = j.optString("name"), prompt = j.optString("prompt"),
                kind = j.optString("kind", "manual"), time = j.optString("time"),
                intervalH = j.optDouble("interval_h"), enabled = j.optBoolean("enabled"),
                notify = j.optBoolean("notify", true), lastStatus = j.optString("last_status"),
                nextDue = j.optString("next_due"),
            )
        }
    }

    fun saveJob(j: JobItem): JobItem {
        val o = post("/api/jobs/save", JSONObject()
            .put("id", j.id).put("name", j.name).put("prompt", j.prompt)
            .put("kind", j.kind).put("time", j.time).put("interval_h", j.intervalH)
            .put("enabled", j.enabled).put("notify", j.notify))
        val r = o.optJSONObject("job") ?: return j
        return j.copy(id = r.optString("id", j.id))
    }

    fun deleteJob(id: String) {
        post("/api/jobs/delete", JSONObject().put("id", id))
    }

    fun runJob(id: String) {
        post("/api/jobs/run", JSONObject().put("id", id))
    }

    fun getRuns(jobId: String = "", n: Int = 30): List<RunItem> {
        val o = JSONObject(read(conn("/api/jobs/runs?job=$jobId&n=$n", "GET")))
        val a = o.optJSONArray("runs") ?: JSONArray()
        return (0 until a.length()).map {
            val r = a.getJSONObject(it)
            RunItem(
                id = r.optString("id"), jobId = r.optString("job_id"), jobName = r.optString("job_name"),
                startedAt = r.optString("started_at"), finishedAt = r.optString("finished_at"),
                status = r.optString("status"), output = r.optString("output"), error = r.optString("error"),
            )
        }
    }

    /**
     * Crida SSE (/api/chat o /api/agent). Cada event arriba a [onEvent];
     * retorna en acabar l'stream. Llança ApiException en error HTTP.
     */
    fun stream(path: String, payload: JSONObject, onEvent: (SseEvent) -> Unit) {
        val c = conn(path, "POST", payload.toString())
        streamRead(c, onEvent)
    }

    /** SSE per GET (execució de grafs): mateix Bearer, mateix parser. */
    fun streamGet(path: String, onEvent: (SseEvent) -> Unit) {
        streamRead(conn(path, "GET"), onEvent)
    }

    private fun streamRead(c: java.net.HttpURLConnection, onEvent: (SseEvent) -> Unit) {
        val code = c.responseCode
        if (code == 401) throw ApiException("El servidor demana token (401). Revisa el token.")
        if (code !in 200..299) {
            val t = try {
                c.errorStream?.bufferedReader(StandardCharsets.UTF_8)?.use { it.readText() } ?: ""
            } catch (_: Exception) { "" }
            throw ApiException("HTTP $code: ${t.take(200)}")
        }
        BufferedReader(InputStreamReader(c.inputStream, StandardCharsets.UTF_8)).use { r ->
            var carry = ""
            val buf = CharArray(4096)
            while (true) {
                val n = r.read(buf)
                if (n < 0) break
                val (evs, rest) = SseParser.feed(String(buf, 0, n), carry)
                carry = rest
                evs.forEach(onEvent)
            }
        }
    }

    private fun JSONArray.toStrList(): List<String> =
        (0 until length()).map { optString(it) }
}

class ApiException(msg: String) : Exception(msg)
