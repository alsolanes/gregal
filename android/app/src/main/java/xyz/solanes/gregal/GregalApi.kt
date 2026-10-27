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
        c.readTimeout = 240_000
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
        if (code == 401) throw ApiException("HTTP 401: server requires a valid token")
        if (code !in 200..299) throw ApiException("HTTP $code: ${text.take(200)}")
        return text
    }

    /** Missatge per al torn en curs. false si ja no n'hi ha cap (409). */
    fun steer(text: String, interrupt: Boolean): Boolean {
        val c = conn("/api/steer", "POST", JSONObject().put("text", text).put("interrupt", interrupt).toString())
        if (c.responseCode == 409) return false
        read(c)
        return true
    }

    // --- #retro (xat amb els agents sobre un torn acabat) ---

    fun getRetro(key: String = "", runId: Long = 0L): RetroInfo {
        val q = when {
            key.isNotBlank() -> "?key=" + URLEncoder.encode(key, "UTF-8")
            runId > 0 -> "?run_id=$runId"
            else -> ""
        }
        return parseRetroInfo(read(conn("/api/retro$q", "GET")))
    }

    fun startRetro(key: String, runId: Long): RetroChannel? {
        val p = JSONObject()
        if (key.isNotBlank()) p.put("key", key)
        if (runId > 0) p.put("run_id", runId)
        return parseRetroChannelResponse(read(conn("/api/retro/start", "POST", p.toString())))
    }

    fun retroMessage(key: String, text: String, thread: String = ""): RetroChannel? {
        val p = JSONObject().put("key", key).put("text", text)
        if (thread.isNotBlank()) p.put("thread_id", thread)
        return parseRetroChannelResponse(read(conn("/api/retro/message", "POST", p.toString())))
    }

    /** Engega l'arranjament proposat en un missatge com un torn nou. */
    fun retroFix(key: String, messageId: String): RetroChannel? {
        val p = JSONObject().put("key", key).put("message_id", messageId)
        return parseRetroChannelResponse(read(conn("/api/retro/fix", "POST", p.toString())))
    }

    /** Canvia el to del canal i regenera els missatges d'agent (mateixos IDs i ordre). */
    fun retroRewrite(key: String, persona: String): RetroChannel? =
        parseRetroChannelResponse(read(conn("/api/retro/rewrite", "POST", RetroRequests.rewrite(key, persona))))

    fun retroReact(key: String, messageId: String, emoji: String, add: Boolean): RetroChannel? {
        val p = JSONObject().put("key", key).put("message_id", messageId).put("emoji", emoji).put("action", if (add) "add" else "remove")
        return parseRetroChannelResponse(read(conn("/api/retro/react", "POST", p.toString())))
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
            permissive = o.optBoolean("permissive"),
            autoReview = o.optBoolean("auto_review"),
            transcript = o.optJSONArray("transcript")?.let { a ->
                (0 until a.length()).mapNotNull { i ->
                    val m = a.optJSONObject(i) ?: return@mapNotNull null
                    val role = m.optString("role")
                    val text = m.optString("content")
                    if ((role == "user" || role == "assistant") && text.isNotBlank()) ChatMsg(role, text) else null
                }
            } ?: emptyList(),
        )
    }

    fun getModels(): Triple<Map<String, List<String>>, String, String> {
        val o = JSONObject(read(conn("/api/models?configured_only=1", "GET")))
        val out = mutableMapOf<String, List<String>>()
        val m = o.optJSONObject("models") ?: JSONObject()
        for (k in m.keys()) out[k] = m.optJSONArray(k)?.toStrList() ?: emptyList()
        return Triple(out, o.optString("current"), o.optString("vents_selected"))
    }

    /** Model per defecte dels vents (xat, rotllana i cor), sense tocar el de la feina. */
    fun setWindsModel(model: String): JSONObject {
        return post("/api/model", JSONObject().put("model", model).put("scope", "vents"))
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
            GithubResult(error = e.message ?: "GitHub request failed")
        }
    }

    /** Office: puja un docx/xlsx/pptx (base64) → id per llegir/editar al servidor. */
    fun officeUpload(name: String, dataB64: String): OfficeDoc {
        return try {
            val o = post("/api/office/upload", JSONObject().put("name", name).put("data", dataB64))
            OfficeDoc(id = o.optString("id"), name = o.optString("name"),
                kind = o.optString("kind"), text = "", result = "uploaded ${o.optInt("size")} bytes")
        } catch (e: Exception) {
            OfficeDoc(error = e.message ?: "upload failed")
        }
    }

    /** Office: llegeix el text/dades del document pujat. */
    fun officeRead(id: String): OfficeDoc {
        return try {
            val o = post("/api/office/read", JSONObject().put("id", id))
            OfficeDoc(id = id, name = o.optString("name"), kind = o.optString("kind"),
                text = o.optString("text"))
        } catch (e: Exception) {
            OfficeDoc(id = id, error = e.message ?: "read failed")
        }
    }

    /** Office: set_cell (xlsx) o replace (docx/pptx) → resultat + text fresc. */
    fun officeEdit(id: String, op: String, sheet: String = "", cell: String = "",
                   value: String = "", find: String = "", replace: String = ""): OfficeDoc {
        return try {
            val o = post("/api/office/edit", JSONObject(officeEditPayload(id, op, sheet, cell, value, find, replace)))
            OfficeDoc(id = id, text = o.optString("text"), result = o.optString("result"))
        } catch (e: Exception) {
            OfficeDoc(id = id, error = e.message ?: "edit failed")
        }
    }

    /** Office: baixa el fitxer editat (bytes per desar al mòbil). */
    fun officeDownload(id: String): ByteArray {
        val c = conn("/api/office/download?id=" + id, "GET")
        val code = c.responseCode
        if (code == 401) throw ApiException("HTTP 401: server requires a valid token")
        if (code !in 200..299) throw ApiException("HTTP $code downloading document")
        return c.inputStream.use { it.readBytes() }
    }

    /** Proveïdors: llista del servidor amb estat de clau (emmascarada). */
    fun getProviders(): List<ProviderInfo> {
        val c = conn("/api/providers", "GET")
        val code = c.responseCode
        if (code == 401) throw ApiException("HTTP 401: server requires a valid token")
        if (code !in 200..299) throw ApiException("HTTP $code listing providers")
        return parseProviders(read(c))
    }

    /** JSON cru de /api/providers (hi ha el catàleg per al primer ús). */
    fun providersJson(): String = read(conn("/api/providers", "GET"))

    /** Preset: tots els rols passen a aquest proveïdor amb el seu model per defecte. */
    fun presetProvider(name: String): String =
        post("/api/providers", JSONObject().put("action", "preset").put("name", name)).optString("msg")

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
        return parseRun(root.optJSONObject("run") ?: root).copy(cursor = root.optLong("cursor"))
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
                text = e.optString("text"),
                payload = structuredInteractivePayload(e.toString()).ifEmpty { rawPayload(e) },
            )
        }
        return EventsPage(events, root.optLong("cursor", after), root.optLong("next", after))
    }

    /** Payload estructurat de qualsevol event (eines, progrés, subagents). */
    private fun rawPayload(e: JSONObject): String = when (val p = e.opt("payload")) {
        is JSONObject -> p.toString()
        is String -> p.takeIf { it.trimStart().startsWith("{") }.orEmpty()
        else -> ""
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

    // ---- Mode company (estil Dot): memòria personal + check-ins ----

    fun getCompany(): CompanyState =
        CompanyState.parse(read(conn("/api/companion/memory", "GET")))

    fun companyAccept(fact: String): CompanyState {
        val o = post("/api/companion/memory", JSONObject(CompanionRequests.accept(fact)))
        fun arr(a: JSONArray?): List<String> =
            a?.let { (0 until it.length()).map { i -> it.optString(i) } } ?: emptyList()
        return CompanyState(state = "none", facts = emptyList(), notes = arr(o.optJSONArray("notes")))
    }

    fun companyDismiss(): CompanyState {
        post("/api/companion/memory", JSONObject(CompanionRequests.dismiss()))
        return CompanyState(state = "none", facts = emptyList(), notes = emptyList())
    }

    fun companyForget(term: String): CompanyState {
        val o = post("/api/companion/memory", JSONObject(CompanionRequests.forget(term)))
        fun arr(a: JSONArray?): List<String> =
            a?.let { (0 until it.length()).map { i -> it.optString(i) } } ?: emptyList()
        return CompanyState(state = "none", facts = emptyList(), notes = arr(o.optJSONArray("notes")))
    }

    fun companyCheckin(): CompanyCheckin =
        CompanyCheckin.parse(post("/api/companion/checkin", JSONObject()).toString())

    fun getPets(): List<Pet> =
        Pet.parse(read(conn("/api/companion/pets", "GET")))

    fun petCheckin(petId: String): PetCheckin =
        PetCheckin.parse(post("/api/companion/checkin", JSONObject(PetRequests.checkin(petId))).toString())

    fun petChat(petId: String, message: String): String {
        val o = post("/api/companion/chat", JSONObject(PetRequests.chat(petId, message)))
        return o.optString("reply")
    }

    fun getPetChat(petId: String): List<GregalViewModel.PetMsg> {
        val o = JSONObject(read(conn("/api/companion/chat?pet=$petId", "GET")))
        val a = o.optJSONArray("history") ?: JSONArray()
        return (0 until a.length()).mapNotNull { i ->
            val item = a.optJSONObject(i) ?: return@mapNotNull null
            GregalViewModel.PetMsg(item.optString("from"), item.optString("text"), item.optString("speaker_id"), item.optString("speaker_name"), ts = item.optLong("ts", 0L))
        }
    }

    fun getWinds(): List<Wind> =
        Wind.parse(read(conn("/api/winds", "GET")))

    fun saveWind(name: String, color: String, role: String, style: String = "pso", persona: String = "", id: String = "") {
        val payload = JSONObject(WindRequests.save(name, color, role, style, persona))
        if (id.isNotBlank()) payload.put("id", id)
        post("/api/winds", payload)
    }

    fun getWindProfile(id: String): WindProfile = WindProfile.parse(
        read(conn("/api/winds/profile?wind=${java.net.URLEncoder.encode(id, "UTF-8")}", "GET")))

    fun updateWindKnowledge(id: String, text: String): WindProfile = WindProfile.parse(
        post("/api/winds/memory", JSONObject().put("wind", id).put("text", text)).toString())

    /** Proposta de persona i cara per a un vent nou (feta amb la IA). */
    fun suggestWind(name: String, role: String): Pair<String, String> {
        val o = post("/api/winds/suggest", JSONObject().put("name", name).put("role", role))
        return Pair(o.optString("persona"), o.optString("style").ifBlank { "pso" })
    }

    fun getRotllanes(): List<Rotllana> =
        RotllanaRequests.parse(read(conn("/api/winds/rotllanes", "GET")))

    fun saveRotllana(name: String, members: List<String>, id: String = ""): JSONObject {
        val payload = JSONObject(RotllanaRequests.save(name, members))
        if (id.isNotBlank()) payload.put("id", id)
        return post("/api/winds/rotllanes", payload)
    }

    fun deleteRotllana(id: String) {
        read(conn("/api/winds/rotllanes", "DELETE", JSONObject().put("id", id).toString()))
    }

    /** Buida el xat amb un vent (DELETE /api/winds/chat): el vent es queda. */
    fun clearWindChat(windId: String) {
        read(conn("/api/winds/chat", "DELETE", JSONObject().put("wind", windId).toString()))
    }

    /** Memòria del vent sobre l'usuari (markdown, editable). */
    fun getWindMemoria(windId: String): String {
        val o = JSONObject(read(conn("/api/winds/memoria?wind=$windId", "GET")))
        return o.optString("memoria")
    }

    fun saveWindMemoria(windId: String, memoria: String) {
        read(conn("/api/winds/memoria", "PUT", JSONObject().put("wind", windId).put("memoria", memoria).toString()))
    }

    /** Coneixement d'un vent: resum de catàleg. */
    data class Coneixement(val nom: String, val titol: String, val descripcio: String, val cos: String = "")

    fun listConeixements(windId: String): List<Coneixement> {
        val o = JSONObject(read(conn("/api/winds/coneixements?wind=$windId", "GET")))
        val a = o.optJSONArray("coneixements") ?: JSONArray()
        return (0 until a.length()).mapNotNull { i ->
            val c = a.optJSONObject(i) ?: return@mapNotNull null
            Coneixement(c.optString("nom"), c.optString("titol"), c.optString("descripcio"))
        }
    }

    fun getConeixement(windId: String, nom: String): Coneixement {
        val o = JSONObject(read(conn("/api/winds/coneixement?wind=$windId&nom=$nom", "GET")))
        return Coneixement(o.optString("nom"), o.optString("titol"), o.optString("descripcio"), o.optString("cos"))
    }

    fun saveConeixement(windId: String, c: Coneixement): String {
        val o = post("/api/winds/coneixements", JSONObject().put("wind", windId).put("nom", c.nom)
            .put("titol", c.titol).put("descripcio", c.descripcio).put("cos", c.cos))
        return o.optString("nom", c.nom)
    }

    fun deleteConeixement(windId: String, nom: String) {
        read(conn("/api/winds/coneixements", "DELETE", JSONObject().put("wind", windId).put("nom", nom).toString()))
    }

    /** Resum de tots els vents: rol + índex de coneixements (selector de membres). */
    data class WindResum(val id: String, val nom: String, val rol: String, val coneixements: List<Coneixement>)

    /** Estat d'una sessió d'estudi «fes-te expert» d'un vent. */
    data class WindExpert(
        val wind: String,
        val tema: String,
        val fase: String,      // "" | cerca | apren | fet | error
        val pas: Int,
        val passos: Int,
        val iniciat: Long,     // unix s
        val acabat: Long,      // unix s
        val error: String = "",
    ) {
        val actiu get() = fase == "cerca" || fase == "lectura" || fase == "apren"
        val exit get() = fase == "fet"
        val progres: Float get() = if (passos <= 0) 0f else (pas.coerceIn(0, passos) / passos.toFloat()) * .9f + (if (exit) .1f else 0f)
    }

    fun getWindsResum(): List<WindResum> {
        val o = JSONObject(read(conn("/api/winds/resum", "GET")))
        val a = o.optJSONArray("vents") ?: JSONArray()
        return (0 until a.length()).mapNotNull { i ->
            val v = a.optJSONObject(i) ?: return@mapNotNull null
            val c = v.optJSONArray("coneixements") ?: JSONArray()
            WindResum(v.optString("id"), v.optString("nom"), v.optString("rol"),
                (0 until c.length()).mapNotNull { j ->
                    val e = c.optJSONObject(j) ?: return@mapNotNull null
                    Coneixement(e.optString("nom"), e.optString("titol"), e.optString("descripcio"))
                })
        }
    }

    /** Colles predefinides (Dia a dia, Ciència de dades, Software i seguretat). */
    data class SquadMember(val key: String, val name: String, val ca: String, val en: String, val color: String, val style: String)
    data class Squad(val id: String, val ca: String, val en: String, val members: List<SquadMember>)

    fun getSquads(): List<Squad> {
        val o = JSONObject(read(conn("/api/winds/squads", "GET")))
        val a = o.optJSONArray("squads") ?: JSONArray()
        return (0 until a.length()).mapNotNull { i ->
            val s = a.optJSONObject(i) ?: return@mapNotNull null
            val m = s.optJSONArray("members") ?: JSONArray()
            Squad(s.optString("id"), s.optString("ca"), s.optString("en"),
                (0 until m.length()).mapNotNull { j ->
                    val e = m.optJSONObject(j) ?: return@mapNotNull null
                    SquadMember(e.optString("key"), e.optString("name"), e.optString("ca"), e.optString("en"), e.optString("color"), e.optString("style"))
                })
        }
    }

    /** Qui pot fer un encàrrec: vents que ja hi encaixen i especialistes nous. */
    fun crewPropose(task: String, lang: String): CrewProposal =
        CrewProposal.parse(post("/api/winds/crew/propose", JSONObject().put("task", task).put("lang", lang)))

    /** Convoca la colla: crea els vents nous, obre la rotllana i torna'n l'id. */
    fun crewStart(task: String, lang: String, p: CrewProposal): String {
        val body = JSONObject().put("task", task).put("lang", lang)
            .put("reuse", JSONArray(p.reuse.map { it.id }))
            .put("create", JSONArray(p.create.map { it.json() }))
        return post("/api/winds/crew/start", body).optJSONObject("rotllana")?.optString("id").orEmpty()
    }

    /** Pas de la feina d'una colla convocada: discussing, waiting, working, done, failed. */
    fun crewStatus(rotllanaId: String): Pair<Boolean, String> {
        val o = JSONObject(read(conn("/api/winds/crew/status?rotllana=" + java.net.URLEncoder.encode(rotllanaId, "UTF-8"), "GET")))
        return o.optBoolean("running") to o.optString("step")
    }

    /** Activa una colla: crea els especialistes que falten (idempotent). */
    fun activateSquad(id: String, lang: String): Boolean {
        val o = post("/api/winds/squads", JSONObject().put("id", id).put("lang", lang))
        return o.optBoolean("ok", false)
    }

    /** Envia un missatge al grup; respon el vent que toca (o el demanat). */
    fun rotllanaChat(rotllanaId: String, message: String, wind: String = "", cont: Boolean = false): Triple<String, String, List<GregalViewModel.PetMsg>> {
        val o = post("/api/winds/rotllana", JSONObject(if (cont) RotllanaRequests.chatContinue(rotllanaId, wind) else RotllanaRequests.chat(rotllanaId, message, wind)))
        val reply = o.optJSONObject("reply") ?: JSONObject()
        val a = o.optJSONArray("history") ?: JSONArray()
        val hist = (0 until a.length()).mapNotNull { i ->
            val item = a.optJSONObject(i) ?: return@mapNotNull null
            GregalViewModel.PetMsg(item.optString("from"), item.optString("text"), item.optString("speaker_id"), item.optString("speaker_name"), ts = item.optLong("ts", 0L))
        }
        return Triple(reply.optString("wind"), reply.optString("text"),
            if (o.optBoolean("parcial", false)) markParcial(hist) else hist)
    }

    fun getRotllanaChat(rotllanaId: String): List<GregalViewModel.PetMsg> {
        val o = JSONObject(read(conn("/api/winds/rotllana?rotlla=$rotllanaId", "GET")))
        val a = o.optJSONArray("history") ?: JSONArray()
        return (0 until a.length()).mapNotNull { i ->
            val item = a.optJSONObject(i) ?: return@mapNotNull null
            GregalViewModel.PetMsg(item.optString("from"), item.optString("text"), item.optString("speaker_id"), item.optString("speaker_name"), ts = item.optLong("ts", 0L))
        }
    }

    fun deleteWind(id: String) {
        read(conn("/api/winds", "DELETE", JSONObject().put("id", id).toString()))
    }

    fun windChat(windId: String, message: String, cont: Boolean = false): WindReply {
        val o = post("/api/winds/chat", JSONObject(if (cont) WindRequests.chatContinue(windId) else WindRequests.chat(windId, message)))
        return windReplyOf(o)
    }

    /** Feines dels vents (resum: estat, qui, fitxers i resultat). */
    fun windTasks(): List<WindTask> {
        val o = JSONObject(read(conn("/api/winds/tasks?summary=1", "GET")))
        val a = o.optJSONArray("tasks") ?: JSONArray()
        fun ms(s: String): Long = try { java.time.Instant.parse(s).toEpochMilli() } catch (_: Exception) { 0L }
        return (0 until a.length()).map {
            val t = a.getJSONObject(it)
            val agents = t.optJSONObject("agents")
            val files = t.optJSONArray("files") ?: JSONArray()
            WindTask(
                id = t.optString("id"), task = t.optString("task"), workspace = t.optString("workspace"),
                thread = t.optString("thread"), created = ms(t.optString("created")), updated = ms(t.optString("updated")),
                status = t.optString("status"),
                agents = agents?.keys()?.asSequence()?.sorted()?.map { k -> agents.optString(k) }?.filter { n -> n.isNotBlank() }?.toList().orEmpty(),
                files = (0 until files.length()).map { i -> files.optString(i) }.filter { f -> f.isNotBlank() },
                result = t.optString("result"), error = t.optString("error"),
            )
        }
    }

    /** Text d'un fitxer (GET /api/file). null + motiu si és binari o massa gran. */
    fun readTextFile(path: String): Pair<String?, String?> {
        val o = JSONObject(read(conn("/api/file?path=" + java.net.URLEncoder.encode(path, "UTF-8"), "GET")))
        return when {
            o.optBoolean("binary") -> null to "binary"
            o.optBoolean("too_big") -> null to "too_big"
            else -> o.optString("content") to null
        }
    }

    /** Veus proactives posteriors a desDe (unix). Lleuger: el vigilant ho
     *  crida en segon pla sense descarregar historials. */
    fun windNovetats(desDe: Long): List<WindNovetat> {
        val o = JSONObject(read(conn("/api/winds/novetats?des_de=$desDe", "GET")))
        val a = o.optJSONArray("novetats") ?: JSONArray()
        return (0 until a.length()).mapNotNull {
            val j = a.optJSONObject(it) ?: return@mapNotNull null
            val text = j.optString("text")
            if (text.isBlank()) null else WindNovetat(
                ts = j.optLong("ts"),
                wind = j.optString("wind"),
                name = j.optString("name"),
                source = j.optString("source"),
                text = text,
            )
        }
    }

    /** Resposta d'un torn amb streaming (SSE per POST). */
    sealed interface WindStreamEvent {
        data class Phase(val phase: String) : WindStreamEvent
        data class Token(val token: String) : WindStreamEvent
        data class DoneWind(val reply: WindReply) : WindStreamEvent
        data class DoneRotllana(val wind: String, val text: String, val history: List<GregalViewModel.PetMsg>, val parcial: Boolean = false, val silenci: Boolean = false) : WindStreamEvent
        data class Error(val message: String) : WindStreamEvent
    }

    /**
     * Llegeix un SSE (event:/data: per línies) cridant onEvent a mesura que
     * arriba. Puro excepte la lectura: parseWindStreamEvent es pot provar sol.
     */
    fun streamPost(path: String, payload: JSONObject, onEvent: (WindStreamEvent) -> Unit) {
        val c = conn(path, "POST", payload.toString())
        val code = c.responseCode
        if (code !in 200..299) {
            val err = try { c.errorStream?.bufferedReader(StandardCharsets.UTF_8)?.readText().orEmpty() } catch (_: Exception) { "" }
            throw ApiException("HTTP $code: ${err.take(200)}")
        }
        var ev: String? = null
        val data = StringBuilder()
        c.inputStream.bufferedReader(StandardCharsets.UTF_8).useLines { lines ->
            lines.forEach { line ->
                when {
                    line.startsWith("event:") -> { ev = line.substringAfter("event:").trim(); data.clear() }
                    line.startsWith("data:") -> {
                        if (data.isNotEmpty()) data.append("\n")
                        data.append(line.substringAfter("data:").trim())
                    }
                    line.isBlank() -> {
                        parseWindStreamEvent(ev, data.toString())?.let { e ->
                            if (e is WindStreamEvent.Error) throw ApiException(e.message.ifBlank { "stream error" })
                            onEvent(e)
                        }
                        ev = null; data.clear()
                    }
                }
            }
        }
    }

    fun parseWindStreamEvent(event: String?, data: String): WindStreamEvent? {
        if (data.isEmpty()) return null
        val o = try { JSONObject(data) } catch (_: Exception) { return null }
        return when (event) {
            "phase" -> WindStreamEvent.Phase(o.optString("phase", "working"))
            "token" -> WindStreamEvent.Token(o.optString("token"))
            "done" -> if (o.has("history") && o.opt("reply") is JSONObject) {
                val reply = o.optJSONObject("reply") ?: JSONObject()
                val a = o.optJSONArray("history") ?: JSONArray()
                val hist = (0 until a.length()).mapNotNull { i ->
                    val item = a.optJSONObject(i) ?: return@mapNotNull null
                    GregalViewModel.PetMsg(item.optString("from"), item.optString("text"), item.optString("speaker_id"), item.optString("speaker_name"), ts = item.optLong("ts", 0L))
                }
                val parcial = o.optBoolean("parcial", false)
                WindStreamEvent.DoneRotllana(reply.optString("wind"), reply.optString("text"),
                    if (parcial) markParcial(hist) else hist, parcial, o.optBoolean("silenci", false))
            } else {
                WindStreamEvent.DoneWind(windReplyOf(o))
            }
            "error" -> WindStreamEvent.Error(o.optString("error", "error"))
            else -> null
        }
    }

    private fun windReplyOf(o: JSONObject): WindReply {
        val acts = o.optJSONArray("actions")
        val reminds = (0 until (acts?.length() ?: 0)).mapNotNull { i ->
            acts?.optJSONObject(i)?.takeIf { it.optString("kind") == "remind" }?.optString("at")
        }.filter { it.isNotBlank() }
        return WindReply(o.optString("reply"), reminds, o.optBoolean("parcial", false))
    }

    /** Marca l'última resposta del vent com a parcial (botó «continua»). */
    private fun markParcial(hist: List<GregalViewModel.PetMsg>): List<GregalViewModel.PetMsg> {
        val idx = hist.indexOfLast { it.from == "pet" && !it.streaming }
        if (idx < 0) return hist
        return hist.toMutableList().also { it[idx] = it[idx].copy(parcial = true) }
    }

    /**
     * Xat amb un vent amb emissió en directe. Torna el Done final; si el
     * servidor no té /stream (antic) o cau abans del done, llança i el
     * cridant cau al clàssic.
     */
    fun windChatStream(windId: String, message: String, cont: Boolean = false, onEvent: (WindStreamEvent) -> Unit): WindReply {
        var done: WindReply? = null
        streamPost("/api/winds/chat/stream", JSONObject(if (cont) WindRequests.chatContinue(windId) else WindRequests.chat(windId, message))) { e ->
            if (e is WindStreamEvent.DoneWind) done = e.reply else onEvent(e)
        }
        return done ?: throw ApiException("stream tancat sense done")
    }

    fun rotllanaChatStream(rotllanaId: String, message: String, wind: String = "", cont: Boolean = false, onEvent: (WindStreamEvent) -> Unit): Triple<String, String, List<GregalViewModel.PetMsg>> {
        var done: WindStreamEvent.DoneRotllana? = null
        streamPost("/api/winds/rotllana/stream", JSONObject(if (cont) RotllanaRequests.chatContinue(rotllanaId, wind) else RotllanaRequests.chat(rotllanaId, message, wind))) { e ->
            if (e is WindStreamEvent.DoneRotllana) done = e else onEvent(e)
        }
        val d = done ?: throw ApiException("stream tancat sense done")
        return Triple(d.wind, d.text, d.history)
    }

    fun getWindChat(windId: String): List<GregalViewModel.PetMsg> {
        val o = JSONObject(read(conn("/api/winds/chat?wind=$windId", "GET")))
        val a = o.optJSONArray("history") ?: JSONArray()
        return (0 until a.length()).mapNotNull { i ->
            val item = a.optJSONObject(i) ?: return@mapNotNull null
            GregalViewModel.PetMsg(item.optString("from"), item.optString("text"), item.optString("speaker_id"), item.optString("speaker_name"), ts = item.optLong("ts", 0L))
        }
    }

    /** Rutina d'un vent: feina programada que corre el vent de veres (LLM + eines). */
    data class WindRutina(
        val id: String, val wind: String, val windName: String, val name: String,
        val kind: String, val time: String, val intervalH: Double, val prompt: String,
        val enabled: Boolean, val nextDue: String, val nextDueAt: String, val lastStatus: String,
    )

    /** Llista les rutines dels vents (daily/interval): el vent hi corre de veres. */
    fun getWindRutines(): List<WindRutina> {
        val o = JSONObject(read(conn("/api/winds/rutines", "GET")))
        val arr = o.optJSONArray("rutines") ?: JSONArray()
        return (0 until arr.length()).map { i ->
            val x = arr.getJSONObject(i)
            WindRutina(
                x.optString("id"), x.optString("wind"), x.optString("wind_name"), x.optString("name"),
                x.optString("kind"), x.optString("time"), x.optDouble("interval_h"), x.optString("prompt"),
                x.optBoolean("enabled"), x.optString("next_due"), x.optString("next_due_at"), x.optString("last_status"),
            )
        }
    }

    /** Crea una rutina de vent: quan toca, el vent obre un torn real (cerca, pensa, parla). */
    fun createWindRutina(windId: String, kind: String, time: String, intervalH: Double, prompt: String, name: String): Boolean {
        val o = post("/api/winds/rutina", JSONObject()
            .put("wind", windId).put("kind", kind).put("time", time)
            .put("interval_h", intervalH).put("prompt", prompt).put("name", name))
        return o.has("job")
    }

    /** Esborra una rutina de vent. */
    fun deleteWindRutina(id: String): Boolean {
        val o = post("/api/winds/rutines/delete", JSONObject().put("id", id))
        return o.optBoolean("ok", false)
    }

    /** Preferència d'organització: una secció temàtica de la llista de converses. */
    data class WindSection(
        val id: String, val name: String, val items: List<String>,
    )

    /** Favorits i seccions de la llista de converses; iguals a totes les pantalles. */
    data class WindsMeta(
        val favorites: List<String>, val sections: List<WindSection>,
    ) {
        fun isFavorite(key: String): Boolean = key in favorites
        fun sectionOf(key: String): WindSection? = sections.firstOrNull { s -> key in s.items }
    }

    /** Llegeix els favorits i les seccions (buits si no n'hi ha). */
    fun getWindsMeta(): WindsMeta {
        val o = JSONObject(read(conn("/api/winds/meta", "GET")))
        val m = o.optJSONObject("meta") ?: JSONObject()
        val favs = m.optJSONArray("favorites") ?: JSONArray()
        val secs = m.optJSONArray("sections") ?: JSONArray()
        return WindsMeta(
            (0 until favs.length()).map { favs.optString(it) },
            (0 until secs.length()).map { i ->
                val s = secs.getJSONObject(i)
                val items = s.optJSONArray("items") ?: JSONArray()
                WindSection(s.optString("id"), s.optString("name"), (0 until items.length()).map { items.optString(it) })
            },
        )
    }

    /** Desa favorits i seccions sencers (la UI hi envia sempre l'estat complet). */
    fun saveWindsMeta(meta: WindsMeta): Boolean {
        val sections = JSONArray()
        meta.sections.forEach { s ->
            val items = JSONArray()
            s.items.forEach(items::put)
            sections.put(JSONObject().put("id", s.id).put("name", s.name).put("items", items))
        }
        val favorites = JSONArray()
        meta.favorites.forEach(favorites::put)
        val o = post("/api/winds/meta", JSONObject().put("favorites", favorites).put("sections", sections))
        return o.optJSONObject("meta") != null
    }

    /** Missatges programats i estadístiques dels vents (recordatoris, cors). */
    fun getWindsProgramats(): WindsProgramats {
        val o = JSONObject(read(conn("/api/winds/programats", "GET")))
        fun str(x: JSONObject, k: String) = x.optString(k)
        val rem = o.optJSONArray("reminders") ?: JSONArray()
        val hearts = o.optJSONArray("hearts") ?: JSONArray()
        val stats = o.optJSONArray("stats") ?: JSONArray()
        return WindsProgramats(
            reminders = (0 until rem.length()).map { i ->
                val x = rem.getJSONObject(i)
                ProgramatReminder(x.optString("id"), x.optString("wind_name"), x.optString("at"), x.optString("text"))
            },
            hearts = (0 until hearts.length()).map { i ->
                val x = hearts.getJSONObject(i)
                ProgramatHeart(x.optString("wind"), x.optString("wind_name"), x.optString("next_at"), x.optString("last_status"))
            },
            stats = (0 until stats.length()).map { i ->
                val x = stats.getJSONObject(i)
                ProgramatStats(x.optString("wind_name"), x.optInt("beats"), x.optInt("spoke"), x.optInt("silent"), x.optInt("reminders"), str(x, "last_at"))
            },
        )
    }

    /** Estudi «fes-te expert»: engega una sessió (POST) o consulta el progrés (GET). */
    fun startWindExpert(windId: String, tema: String): Boolean {
        val o = post("/api/winds/expert", JSONObject().put("wind", windId).put("tema", tema))
        return o.optBoolean("ok", false)
    }

    /** Wiki de l'espai vents: pàgines de resum amb markdown, gràfics i imatges. */
    data class WikiPage(
        val nom: String, val titol: String, val descripcio: String,
        val cos: String, val updated: String,
    )

    fun getWikiPages(): List<WikiPage> {
        val o = JSONObject(read(conn("/api/winds/wiki", "GET")))
        val a = o.optJSONArray("pages") ?: JSONArray()
        return (0 until a.length()).map { wikiFromJSON(a.getJSONObject(it)) }
    }

    fun getWikiPage(nom: String): WikiPage {
        val o = JSONObject(read(conn("/api/winds/wiki/${java.net.URLEncoder.encode(nom, "UTF-8")}", "GET")))
        return wikiFromJSON(o.getJSONObject("page"))
    }

    fun saveWikiPage(p: WikiPage): Boolean {
        val o = post("/api/winds/wiki", JSONObject()
            .put("nom", p.nom).put("titol", p.titol).put("descripcio", p.descripcio).put("cos", p.cos))
        return o.has("page")
    }

    fun deleteWikiPage(nom: String): Boolean = try {
        val c = conn("/api/winds/wiki/${java.net.URLEncoder.encode(nom, "UTF-8")}", "DELETE")
        c.requestMethod = "DELETE"
        JSONObject(read(c)).optBoolean("ok", false)
    } catch (e: Exception) { false }

    /** Resumeix l'última conversa (vent o rotllana) i la desa com a pàgina nova. */
    fun summarizeToWiki(wind: String? = null, rotllana: String? = null, titol: String = ""): WikiPage? {
        val body = JSONObject()
        if (wind != null) body.put("wind", wind)
        if (rotllana != null) body.put("rotllana", rotllana)
        if (titol.isNotBlank()) body.put("titol", titol)
        val o = post("/api/winds/wiki/summary", body)
        return if (o.has("page")) wikiFromJSON(o.getJSONObject("page")) else null
    }

    private fun wikiFromJSON(j: JSONObject): WikiPage = WikiPage(
        j.optString("nom"), j.optString("titol"), j.optString("descripcio"),
        j.optString("cos"), j.optString("updated"),
    )

    fun getWindExpert(windId: String): WindExpert {
        val o = JSONObject(read(conn("/api/winds/expert?wind=${java.net.URLEncoder.encode(windId, "UTF-8")}", "GET")))
        return WindExpert(o.optString("wind"), o.optString("tema"), o.optString("fase"),
            o.optInt("pas"), o.optInt("passos"), o.optLong("iniciat", 0L), o.optLong("acabat", 0L), o.optString("error"))
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
                lastAt = j.optString("last_at"), nextDueAt = j.optString("next_due_at"),
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

    /**
     * Franja programada (l'alarma d'Android): s'encua darrere d'un torn en
     * marxa. false si ja n'hi ha un altre en marxa o el job està pausat (409).
     */
    fun runScheduledJob(id: String): Boolean {
        val c = conn("/api/jobs/run", "POST", JSONObject().put("id", id).put("scheduled", true).toString())
        if (c.responseCode == 409) return false
        read(c)
        return true
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
        if (code == 401) throw ApiException("HTTP 401: server requires a valid token")
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
