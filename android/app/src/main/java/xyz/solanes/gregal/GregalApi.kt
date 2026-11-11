package xyz.solanes.gregal

import kotlinx.serialization.ExperimentalSerializationApi
import kotlinx.serialization.SerialName
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.encodeToString
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.longOrNull
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okhttp3.ResponseBody.Companion.asResponseBody
import okhttp3.sse.EventSource
import okhttp3.sse.EventSourceListener
import okhttp3.sse.EventSources
import org.json.JSONArray
import org.json.JSONObject
import java.io.IOException
import java.net.URLEncoder
import java.nio.charset.StandardCharsets
import java.util.concurrent.TimeUnit

/**
 * El lector JSON de les respostes del servidor. Cap camp és obligatori: un
 * servidor més antic que no n'enviï algun no trenca res (pren el valor per
 * defecte del model), els camps que no coneixem s'ignoren i un `null` val
 * com si no hi fos. Tolera números i booleans entre cometes, com feia org.json.
 */
@OptIn(ExperimentalSerializationApi::class)
val GregalJson: Json = Json {
    ignoreUnknownKeys = true
    isLenient = true
    coerceInputValues = true
    explicitNulls = false
    encodeDefaults = true
}

/** Text d'un valor qualsevol, com `optString` d'org.json: un objecte o una llista surten com a JSON. */
internal fun JsonElement?.looseString(): String = when (this) {
    null, JsonNull -> ""
    is JsonPrimitive -> content
    else -> toString()
}

/** `optString(key)` sobre un objecte de kotlinx. */
internal fun JsonObject.str(key: String): String = this[key].looseString()

/** Payload de /api/plan (pur, testeable sense Android). */
fun planPayload(task: String): Map<String, String> = mapOf("task" to task)

/** Payload de /api/office/edit (pur, testeable sense Android). */
fun officeEditPayload(id: String, op: String, sheet: String = "", cell: String = "",
                      value: String = "", find: String = "", replace: String = ""): Map<String, String> =
    mapOf("id" to id, "op" to op, "sheet" to sheet, "cell" to cell,
        "value" to value, "find" to find, "replace" to replace)

/**
 * Client de `gregal serve`. OkHttp (i okhttp-sse per als fluxos) i
 * kotlinx.serialization per llegir les respostes. El token va per capçalera
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

    private fun request(path: String, method: String, body: String? = null): Request {
        val b = Request.Builder().url(base + path)
        if (token.isNotBlank()) b.header("Authorization", "Bearer $token")
        // Cada dispositiu, la seva conversa.
        if (sessioApp.isNotBlank()) b.header("X-Gregal-Session", sessioApp)
        val rb = when {
            body != null -> body.toByteArray(StandardCharsets.UTF_8).toRequestBody(JSON_TYPE)
            method == "POST" || method == "PUT" -> ByteArray(0).toRequestBody(JSON_TYPE)
            else -> null
        }
        if (rb != null) b.header("Content-Type", "application/json")
        return b.method(method, rb).build()
    }

    /** Executa la petició; qui la crida tanca la resposta (`use`). */
    private fun exec(path: String, method: String, body: String? = null): Response =
        http.newCall(request(path, method, body)).execute()

    private fun bodyText(r: Response): String =
        try { String(r.body.bytes(), StandardCharsets.UTF_8) } catch (_: Exception) { "" }

    private fun read(r: Response): String = r.use {
        val code = r.code
        val text = bodyText(r)
        if (code == 401) {
            if (token.isNotBlank()) onUnauthorized?.invoke(base, token)
            throw ApiException("HTTP 401: server requires a valid token")
        }
        if (code !in 200..299) throw ApiException("HTTP $code: ${text.take(200)}")
        text
    }

    private fun read(path: String, method: String = "GET", body: String? = null): String = read(exec(path, method, body))

    private inline fun <reified T> getAs(path: String): T = GregalJson.decodeFromString(read(path))

    private inline fun <reified T> postAs(path: String, payload: JSONObject): T =
        GregalJson.decodeFromString(read(path, "POST", payload.toString()))

    /** Missatge per al torn en curs. false si ja no n'hi ha cap (409). */
    fun steer(text: String, interrupt: Boolean): Boolean {
        val r = exec("/api/steer", "POST", JSONObject().put("text", text).put("interrupt", interrupt).toString())
        if (r.code == 409) { r.close(); return false }
        read(r)
        return true
    }

    // --- #retro (xat amb els agents sobre un torn acabat) ---

    fun getRetro(key: String = "", runId: Long = 0L): RetroInfo {
        val q = when {
            key.isNotBlank() -> "?key=" + URLEncoder.encode(key, "UTF-8")
            runId > 0 -> "?run_id=$runId"
            else -> ""
        }
        return parseRetroInfo(read("/api/retro$q"))
    }

    fun startRetro(key: String, runId: Long): RetroChannel? {
        val p = JSONObject()
        if (key.isNotBlank()) p.put("key", key)
        if (runId > 0) p.put("run_id", runId)
        return parseRetroChannelResponse(read("/api/retro/start", "POST", p.toString()))
    }

    fun retroMessage(key: String, text: String, thread: String = ""): RetroChannel? {
        val p = JSONObject().put("key", key).put("text", text)
        if (thread.isNotBlank()) p.put("thread_id", thread)
        return parseRetroChannelResponse(read("/api/retro/message", "POST", p.toString()))
    }

    /** Engega l'arranjament proposat en un missatge com un torn nou. */
    fun retroFix(key: String, messageId: String): RetroChannel? {
        val p = JSONObject().put("key", key).put("message_id", messageId)
        return parseRetroChannelResponse(read("/api/retro/fix", "POST", p.toString()))
    }

    /** Canvia el to del canal i regenera els missatges d'agent (mateixos IDs i ordre). */
    fun retroRewrite(key: String, persona: String): RetroChannel? =
        parseRetroChannelResponse(read("/api/retro/rewrite", "POST", RetroRequests.rewrite(key, persona)))

    fun retroReact(key: String, messageId: String, emoji: String, add: Boolean): RetroChannel? {
        val p = JSONObject().put("key", key).put("message_id", messageId).put("emoji", emoji).put("action", if (add) "add" else "remove")
        return parseRetroChannelResponse(read("/api/retro/react", "POST", p.toString()))
    }

    /** Descobreix capacitats abans d'usar la cua v2. */
    fun health(): ServiceHealth {
        val o = getAs<HealthDto>("/api/health")
        return ServiceHealth(status = o.status, protocol = o.protocol, instanceId = o.instanceId, capabilities = o.capabilities)
    }

    fun getState(): ServerState {
        val o = getAs<StateDto>("/api/state")
        return ServerState(
            roles = o.role,
            currentRole = o.currentRole,
            mode = o.mode,
            model = o.model,
            provider = o.provider,
            verify = o.verify,
            agentBusy = o.agentBusy,
            cwd = o.cwd,
            project = o.project,
            branch = o.branch,
            usedTokens = o.usedTokens,
            modelName = o.modelName,
            think = o.think,
            thinkOptions = o.thinkOptions,
            thinkEffective = o.thinkEffective,
            contextWindow = o.contextWindow,
            permissive = o.permissive,
            autoReview = o.autoReview,
            access = o.access.ifBlank { "edits" },
            transcript = o.transcript.mapNotNull { m ->
                val text = m.content.looseString()
                if ((m.role == "user" || m.role == "assistant") && text.isNotBlank()) ChatMsg(m.role, text) else null
            },
        )
    }

    fun getModels(): Triple<Map<String, List<String>>, String, String> {
        val o = getAs<ModelsDto>("/api/models?configured_only=1")
        return Triple(o.models, o.current, o.ventsSelected)
    }

    /** Model per defecte dels vents (xat, rotllana i cor), sense tocar el de la feina. */
    fun setWindsModel(model: String): JSONObject {
        return post("/api/model", JSONObject().put("model", model).put("scope", "vents"))
    }

    /** POST que torna l'objecte JSON tal qual (per a les pantalles que el llegeixen elles mateixes). */
    fun post(path: String, payload: JSONObject): JSONObject {
        return JSONObject(read(path, "POST", payload.toString()))
    }

    /** GET que torna un objecte JSON (el Racó dels vents, entre d'altres). */
    fun get(path: String): JSONObject = JSONObject(read(path))

    /** DELETE /api/shares/{id}: revoca un enllaç o despublica una web. */
    fun deleteShare(id: String) { read("/api/shares/" + URLEncoder.encode(id, "UTF-8"), "DELETE") }

    /**
     * Entra amb usuari i contrasenya (POST /api/login) i torna el token de
     * sessio. El servidor el desa a passwords.json/tokens.json i el token
     * dura fins que es tanqui la sessio.
     *
     * Serveix per al servidor amb usuaris configurats; si el servidor no en
     * te cap, la resposta porta un error i el cami bo es el token de sempre.
     */
    fun login(usuari: String, contrasenya: String): String =
        postAs<TokenDto>("/api/login", JSONObject().put("user", usuari).put("password", contrasenya)).token

    /** Calendari (Google Calendar per iCal), Gmail (IMAP) i resum del matí d'aquest compte. */
    data class Connections(
        val calendars: List<Pair<String, String>>, val mailUser: String, val mailConnected: Boolean,
        val briefingOn: Boolean, val briefingTime: String, val briefingWind: String,
    )

    fun getConnections(): Connections {
        val o = getAs<ConnectionsDto>("/api/connections")
        return Connections(
            o.calendars.map { it.name to it.url },
            o.mail.user, o.mail.hasPassword,
            o.briefing.enabled, o.briefing.time.ifBlank { "08:00" }, o.briefing.wind,
        )
    }

    /** Desa un canvi (add_calendar, remove_calendar, mail, remove_mail, briefing). */
    fun postConnections(body: JSONObject) { post("/api/connections", body) }

    /** Prova la connexió: text curt per mostrar. */
    fun testConnections(): String {
        val o = postAs<ConnectionsTestDto>("/api/connections", JSONObject().put("action", "test"))
        val parts = mutableListOf<String>()
        o.calendar?.let { parts += "📅 " + it.eventsNext7Days + " (7d)" }
        o.mail?.let { parts += "✉ " + (if (it.error != null) "⚠ " + it.error else it.unread.toString()) }
        return parts.joinToString(" · ")
    }

    /** Poders de servidor (admin): estat, eines de la màquina, túnels i notes. */
    data class ServerPowers(val enabled: Boolean, val tools: List<String>, val user: String, val notes: String, val tunnels: List<Pair<String, String>>)

    fun getServerPowers(): ServerPowers {
        val o = getAs<ServerPowersDto>("/api/server/powers")
        return ServerPowers(o.enabled, o.tools, o.user, o.notes, o.tunnels.map { it.url to (it.target + " · " + it.status) })
    }

    fun setServerPowers(enabled: Boolean? = null, notes: String? = null) {
        val body = JSONObject()
        if (enabled != null) body.put("enabled", enabled)
        if (notes != null) body.put("notes", notes)
        post("/api/server/powers", body)
    }

    /** Codi d'invitació d'un sol ús (admin): POST /api/server/invitations. */
    fun createInvitation(): String = postAs<InvitationDto>("/api/server/invitations", JSONObject()).code

    /** Tanca la sessió d'aquest token al servidor (POST /api/logout). */
    fun logout() { post("/api/logout", JSONObject()) }

    /** Qui soc (GET /api/me): nom, arrels, carpeta de treball i si es admin. */
    fun me(): MeInfo {
        val o = getAs<MeDto>("/api/me")
        return MeInfo(user = o.user, home = o.home, admin = o.admin, roots = o.roots)
    }

    // --- carpeta de treball: on treballen els vents ---

    /** GET /api/workspaces: carpeta actual, branca i projectes recents. */
    fun workspaces(): WorkspacesInfo = parseWorkspaces(read("/api/workspaces"))

    /**
     * POST /api/workspaces {path}: canvia la carpeta d'aquesta sessió.
     * El servidor la valida (ha d'existir i ser dins de les teves arrels) i
     * respon amb la llista nova; si falla, llença ApiException amb el motiu.
     */
    fun setWorkspace(path: String): WorkspacesInfo =
        parseWorkspaces(read("/api/workspaces", "POST", JSONObject().put("path", path).toString()))

    /** GET /api/dirs?path=: subdirectoris, per triar carpeta sense escriure rutes. */
    fun dirs(path: String? = null): DirsInfo {
        val query = if (path.isNullOrBlank()) "" else "?path=" + URLEncoder.encode(path, "UTF-8")
        return parseDirs(read("/api/dirs$query"))
    }

    /** GET /api/preview: què es pot previsualitzar de la carpeta de treball. */
    fun preview(): PreviewInfo {
        val o = getAs<PreviewDto>("/api/preview")
        val webs = o.webs.mapNotNull { w ->
            if (w.path.isBlank()) null else PreviewWeb(w.path, w.name.ifBlank { w.path }, w.size, w.mtime)
        }
        return PreviewInfo(root = o.root, base = o.base, index = o.index, webs = webs)
    }

    /** Respon la pregunta de l'agent (POST /api/question {key, answer}). */
    fun answerQuestion(key: String, answer: String): Boolean =
        postAs<OkDto>("/api/question", JSONObject().put("key", key).put("answer", answer)).ok ?: false

    /** Compara els rols configurats alhora (local + cloud/revisor). */
    fun parallel(message: String): List<ParallelResult> =
        postAs<ParallelDto>("/api/parallel", JSONObject().put("message", message)).results.map { o ->
            ParallelResult(role = o.role, provider = o.provider, model = o.model, reply = o.reply, error = o.error)
        }

    fun github(tool: String, action: String, number: Int = 0, query: String = "", repo: String = ""): GithubResult {
        return try {
            val o = postAs<GithubDto>("/api/github", JSONObject()
                .put("tool", tool).put("action", action).put("number", number)
                .put("query", query).put("repo", repo))
            GithubResult(output = o.output)
        } catch (e: Exception) {
            GithubResult(error = e.message ?: "GitHub request failed")
        }
    }

    /** Office i PDF: puja un pdf/docx/xlsx/pptx (base64) → id per llegir/editar
     *  al servidor i la ruta que es dona a l'agent. */
    fun officeUpload(name: String, dataB64: String): OfficeDoc {
        return try {
            val o = postAs<OfficeDto>("/api/office/upload", JSONObject().put("name", name).put("data", dataB64))
            OfficeDoc(id = o.id, name = o.name, kind = o.kind, text = "", result = "uploaded ${o.size} bytes", path = o.path)
        } catch (e: Exception) {
            OfficeDoc(error = e.message ?: "upload failed")
        }
    }

    /** Office: llegeix el text/dades del document pujat. */
    fun officeRead(id: String): OfficeDoc {
        return try {
            val o = postAs<OfficeDto>("/api/office/read", JSONObject().put("id", id))
            OfficeDoc(id = id, name = o.name, kind = o.kind, text = o.text)
        } catch (e: Exception) {
            OfficeDoc(id = id, error = e.message ?: "read failed")
        }
    }

    /** Office: set_cell (xlsx) o replace (docx/pptx) → resultat + text fresc. */
    fun officeEdit(id: String, op: String, sheet: String = "", cell: String = "",
                   value: String = "", find: String = "", replace: String = ""): OfficeDoc {
        return try {
            val o = postAs<OfficeDto>("/api/office/edit", JSONObject(officeEditPayload(id, op, sheet, cell, value, find, replace)))
            OfficeDoc(id = id, text = o.text, result = o.result)
        } catch (e: Exception) {
            OfficeDoc(id = id, error = e.message ?: "edit failed")
        }
    }

    /** Baixa els bytes d'una ruta del servidor (sense avisar del 401: no és la sessió que caduca). */
    private fun download(path: String): ByteArray = exec(path, "GET").use { r ->
        if (r.code == 401) throw ApiException("HTTP 401: server requires a valid token")
        if (r.code !in 200..299) throw ApiException("HTTP ${r.code} downloading document")
        r.body.bytes()
    }

    /** Office: baixa el fitxer editat (bytes per desar al mòbil). */
    /** Baixa un document de Gregal per l'enllaç relatiu d'una resposta
     *  (/api/winds/doc?… o /api/office/download?…), amb la clau. */
    fun downloadServerDoc(path: String): ByteArray {
        if (!isServerDocLink(path)) throw ApiException("not a Gregal document link")
        return download(path)
    }

    /** B13: la captura d'una acció de l'agent a l'ordinador (només aquest camí del servidor). */
    fun ordinadorShot(path: String): ByteArray {
        if (!path.startsWith("/api/ordinador/accions/captura?id=")) throw ApiException("not a computer action screenshot")
        return download(path)
    }

    fun officeDownload(id: String): ByteArray = download("/api/office/download?id=" + id)

    /** Proveïdors: llista del servidor amb estat de clau (emmascarada). */
    fun getProviders(): List<ProviderInfo> {
        val r = exec("/api/providers", "GET")
        if (r.code == 401) { r.close(); throw ApiException("HTTP 401: server requires a valid token") }
        if (r.code !in 200..299) { val code = r.code; r.close(); throw ApiException("HTTP $code listing providers") }
        return parseProviders(read(r))
    }

    /** JSON cru de /api/providers (hi ha el catàleg per al primer ús). */
    fun providersJson(): String = read("/api/providers")

    /** Preset: tots els rols passen a aquest proveïdor amb el seu model per defecte. */
    fun presetProvider(name: String): String =
        postAs<MsgDto>("/api/providers", JSONObject().put("action", "preset").put("name", name)).msg

    /** Desa la clau d'un proveïdor (servidor: en memòria, o ${VAR} per persistir). */
    fun setProviderKey(name: String, key: String): String =
        postAs<MsgDto>("/api/providers", JSONObject().put("action", "key").put("name", name).put("key", key)).msg.ifBlank { "desat" }

    /**
     * Afegeix un proveïdor nou (POST /api/providers {action:add}).
     * El servidor valida el nom i l'URL i el desa a la SEVA configuració; des
     * del mòbil això és una ordre al servidor, no una preferència del telèfon.
     */
    fun addProvider(name: String, url: String, key: String): String =
        postAs<MsgDto>(
            "/api/providers",
            JSONObject().put("action", "add").put("name", name.trim())
                .put("url", url.trim()).put("key", key.trim()),
        ).msg.ifBlank { "ok" }

    /** Prova la connexió d'un proveïdor (GET /models amb la seva clau). */
    fun testProvider(name: String): String =
        postAs<MsgDto>("/api/providers", JSONObject().put("action", "test").put("name", name)).msg.ifBlank { "ok" }

    /** Pla read-only: /api/plan {"task"} → text del pla. */
    fun postPlan(task: String): String = postAs<PlanDto>("/api/plan", JSONObject(planPayload(task))).plan

    /** Checkpoints del journal: GET /api/checkpoints → llista. */
    fun getCheckpoints(): List<CheckpointInfo> = parseCheckpoints(read("/api/checkpoints"))

    /** Desfés-ho tot: POST /api/rewind → resum. */
    fun postRewind(): String = postAs<SummaryDto>("/api/rewind", JSONObject()).summary

    /** Torna al checkpoint seq: POST /api/rewind-to → resum. */
    fun postRewindTo(seq: Int): String = postAs<SummaryDto>("/api/rewind-to", JSONObject().put("seq", seq)).summary

    fun getSessions(): List<SessionInfo> = getAs<List<SessionDto>>("/api/sessions").map { o ->
        SessionInfo(o.name, o.msgs, o.whenText, o.title, o.at, o.current)
    }

    // ---- Contracte de torns v2 (cua + events durables) ----

    /** Envia un torn a la cua compartida; retorna immediatament. */
    fun submitRun(task: String, mode: String = "", images: List<String> = emptyList(),
                  idempotencyKey: String = ""): RunV2 {
        val payload = JSONObject().put("task", task)
        if (mode.isNotBlank()) payload.put("mode", mode)
        if (images.isNotEmpty()) payload.put("images", JSONArray(images))
        if (idempotencyKey.isNotBlank()) payload.put("idempotency_key", idempotencyKey)
        val root = GregalJson.parseToJsonElement(read("/api/v2/runs", "POST", payload.toString())) as JsonObject
        return runOf(root).copy(cursor = GregalJson.decodeFromJsonElement<CursorDto>(root).cursor ?: 0L)
    }

    /** Llegeix events persistents posteriors al cursor, filtrats per sessió. */
    fun getEvents(after: Long, limit: Int = 100, session: String = sessioApp): EventsPage {
        val path = "/api/v2/events?after=$after&limit=$limit&session=" +
            URLEncoder.encode(session, "UTF-8")
        val root = getAs<EventsDto>(path)
        val events = root.events.map { e ->
            val d = GregalJson.decodeFromJsonElement<DurableEventDto>(e)
            DurableEvent(
                id = d.id, sessionId = d.sessionId, runId = d.runId, kind = d.kind, text = d.text,
                payload = structuredInteractivePayload(e.toString()).ifEmpty { rawPayload(e["payload"]) },
            )
        }
        return EventsPage(events, root.cursor ?: after, root.next ?: after)
    }

    /** Payload estructurat de qualsevol event (eines, progrés, subagents). */
    private fun rawPayload(p: JsonElement?): String = when {
        p is JsonObject -> p.toString()
        p is JsonPrimitive && p.isString -> p.content.takeIf { it.trimStart().startsWith("{") }.orEmpty()
        else -> ""
    }

    /** Consulta l'estat d'un torn concret. */
    fun getRun(id: Long): RunV2 = runOf(GregalJson.parseToJsonElement(read("/api/v2/runs/$id")) as JsonObject)

    /** Cancel·la un torn concret de la cua v2. */
    fun cancelRun(id: Long): Boolean {
        val root = postAs<CancelDto>("/api/v2/runs/$id/cancel", JSONObject())
        return root.cancelled ?: root.ok ?: false
    }

    /** Cancel·lació de compatibilitat per servidors antics. */
    fun cancelLegacy(): Boolean = postAs<OkDto>("/api/agent/cancel", JSONObject()).ok ?: true

    /** El torn ve dins de `run` o és l'objecte sencer (servidors antics). */
    private fun runOf(root: JsonObject): RunV2 {
        val o = GregalJson.decodeFromJsonElement<RunDto>(root["run"] as? JsonObject ?: root)
        return RunV2(id = o.id, state = o.state, session = o.session, workspace = o.workspace, error = o.error)
    }

    /** Esborra una conversa (DELETE /api/sessions?name=). */
    fun deleteSession(name: String) {
        read("/api/sessions?name=" + URLEncoder.encode(name, "UTF-8"), "DELETE")
    }

    /** Mode del revisor: off|manual|auto|both|strict (paritat prefs web). */
    fun setVerify(mode: String) {
        post("/api/verify", JSONObject().put("mode", mode))
    }

    // ---- Grafs (/api/flows*): llistar, veure, esborrar i executar. ----

    fun getFlows(): List<FlowSummary> =
        getAs<FlowsDto>("/api/flows").flows.map { f -> FlowSummary(f.name, f.slug, f.desc, f.steps) }

    fun getFlow(name: String): FlowDetail {
        val f = getAs<FlowLoadDto>("/api/flows/load?name=" + enc(name)).flow ?: throw ApiException("flow not found")
        return FlowDetail(f.name, f.desc,
            f.nodes.map { n -> FlowNode(n.id, n.kind, n.title, n.task, n.tool, n.text) },
            f.edges.map { e -> FlowEdge(e.from, e.to, e.whenText) })
    }

    fun deleteFlow(name: String) {
        post("/api/flows/delete", JSONObject().put("name", name))
    }

    private fun enc(s: String): String = URLEncoder.encode(s, "UTF-8")

    fun getActive(): ActiveAgent? {
        val a = getAs<ActiveDto>("/api/active").active ?: return null
        return ActiveAgent(
            id = a.id, task = a.task, startedAt = a.startedAt, role = a.role, mode = a.mode, project = a.project,
            events = a.events.map { e -> LiveEvent(e.kind, e.text) },
        )
    }

    fun getGoals(): List<GoalItem> =
        getAs<GoalsDto>("/api/goal").goals.map { g -> GoalItem(g.id, g.title, g.body, g.status) }

    // ---- Mode company (estil Dot): memòria personal + check-ins ----

    fun getCompany(): CompanyState = CompanyState.parse(read("/api/companion/memory"))

    fun companyAccept(fact: String): CompanyState {
        val o = postAs<NotesDto>("/api/companion/memory", JSONObject(CompanionRequests.accept(fact)))
        return CompanyState(state = "none", facts = emptyList(), notes = o.notes)
    }

    fun companyDismiss(): CompanyState {
        post("/api/companion/memory", JSONObject(CompanionRequests.dismiss()))
        return CompanyState(state = "none", facts = emptyList(), notes = emptyList())
    }

    fun companyForget(term: String): CompanyState {
        val o = postAs<NotesDto>("/api/companion/memory", JSONObject(CompanionRequests.forget(term)))
        return CompanyState(state = "none", facts = emptyList(), notes = o.notes)
    }

    fun companyCheckin(): CompanyCheckin =
        CompanyCheckin.parse(read("/api/companion/checkin", "POST", "{}"))

    fun getPets(): List<Pet> = Pet.parse(read("/api/companion/pets"))

    fun petCheckin(petId: String): PetCheckin =
        PetCheckin.parse(read("/api/companion/checkin", "POST", JSONObject(PetRequests.checkin(petId)).toString()))

    fun petChat(petId: String, message: String): String =
        postAs<ReplyTextDto>("/api/companion/chat", JSONObject(PetRequests.chat(petId, message))).reply

    fun getPetChat(petId: String): List<GregalViewModel.PetMsg> =
        getAs<HistoryDto>("/api/companion/chat?pet=$petId").history.map { it.toPetMsg() }

    fun getWinds(): List<Wind> = Wind.parse(getWindsRaw())

    /** La resposta tal qual: el mòbil la desa com a còpia per quan el servidor no respongui. */
    fun getWindsRaw(): String = read("/api/winds")

    fun saveWind(name: String, color: String, role: String, style: String = "pso", persona: String = "", id: String = "") {
        val payload = JSONObject(WindRequests.save(name, color, role, style, persona))
        if (id.isNotBlank()) payload.put("id", id)
        post("/api/winds", payload)
    }

    fun getWindProfile(id: String): WindProfile = WindProfile.parse(
        read("/api/winds/profile?wind=${URLEncoder.encode(id, "UTF-8")}"))

    fun updateWindKnowledge(id: String, text: String): WindProfile = WindProfile.parse(
        read("/api/winds/memory", "POST", JSONObject().put("wind", id).put("text", text).toString()))

    /** Proposta de persona i cara per a un vent nou (feta amb la IA). */
    fun suggestWind(name: String, role: String): Pair<String, String> {
        val o = postAs<SuggestDto>("/api/winds/suggest", JSONObject().put("name", name).put("role", role))
        return Pair(o.persona, o.style.ifBlank { "pso" })
    }

    fun getRotllanes(): List<Rotllana> = RotllanaRequests.parse(read("/api/winds/rotllanes"))

    fun saveRotllana(name: String, members: List<String>, id: String = ""): JSONObject {
        val payload = JSONObject(RotllanaRequests.save(name, members))
        if (id.isNotBlank()) payload.put("id", id)
        return post("/api/winds/rotllanes", payload)
    }

    fun deleteRotllana(id: String) {
        read("/api/winds/rotllanes", "DELETE", JSONObject().put("id", id).toString())
    }

    /** Buida el xat amb un vent (DELETE /api/winds/chat): el vent es queda. */
    fun clearWindChat(windId: String, session: String = "") {
        val body = JSONObject().put("wind", windId)
        read(windThreadPath("/api/winds/chat", session), "DELETE", body.toString())
    }

    /**
     * COPIA explícita cap al servidor (TRANSFER-API.md): només aquest DTO
     * creua d'instància. Exporta el bundle sencer (historial + perfil sempre;
     * memòria/coneixements/skills només si es marquen).
     */
    fun exportWindsTransfer(kind: String, id: String, memory: Boolean, knowledge: Boolean, skills: Boolean): JSONObject {
        val q = "kind=" + enc(kind) + "&id=" + enc(id) +
            "&memory=" + enc(memory.toString()) + "&knowledge=" + enc(knowledge.toString()) + "&skills=" + enc(skills.toString())
        return get("/api/winds/transfer?$q")
    }

    /** Importa un bundle amb un transferId propi: el servidor no clona mai dues vegades. */
    fun importWindsTransfer(transferId: String, bundle: JSONObject, into: String = ""): JSONObject =
        post("/api/winds/transfer", JSONObject().put("transferId", transferId).put("bundle", bundle)
            .apply { if (into.isNotBlank()) put("into", into) })

    /** Verifica la destinació abans de navegar-hi: l'endpoint el diu el mateix import. */
    fun getTransferVerify(verifyPath: String): JSONObject = get(verifyPath)

    /** Memòria del vent sobre l'usuari (markdown, editable). */
    fun getWindMemoria(windId: String): String = getAs<MemoriaDto>("/api/winds/memoria?wind=$windId").memoria

    fun saveWindMemoria(windId: String, memoria: String) {
        read("/api/winds/memoria", "PUT", JSONObject().put("wind", windId).put("memoria", memoria).toString())
    }

    /** Coneixement d'un vent: resum de catàleg. */
    data class Coneixement(val nom: String, val titol: String, val descripcio: String, val cos: String = "")

    fun listConeixements(windId: String): List<Coneixement> =
        getAs<ConeixementsDto>("/api/winds/coneixements?wind=$windId").coneixements.map { it.toConeixement(withBody = false) }

    fun getConeixement(windId: String, nom: String): Coneixement =
        getAs<ConeixementDto>("/api/winds/coneixement?wind=$windId&nom=$nom").toConeixement(withBody = true)

    fun saveConeixement(windId: String, c: Coneixement): String {
        val o = postAs<NomDto>("/api/winds/coneixements", JSONObject().put("wind", windId).put("nom", c.nom)
            .put("titol", c.titol).put("descripcio", c.descripcio).put("cos", c.cos))
        return o.nom ?: c.nom
    }

    fun deleteConeixement(windId: String, nom: String) {
        read("/api/winds/coneixements", "DELETE", JSONObject().put("wind", windId).put("nom", nom).toString())
    }

    /** Information snapshots, not conversation backups. Record IDs are NOT wind IDs. */
    data class WindLibraryItem(
        val id: String, val kind: String, val wind: Wind,
        val memory: String = "", val knowledge: String = "",
        val skills: List<Coneixement> = emptyList(), val created: String = "", val origin: String = "",
    ) {
        val restorable: Boolean get() = kind == "archived" || kind == "recovered"
        // Active/recovered snapshots get a new timestamp on every GET.
        fun sameInformation(other: WindLibraryItem): Boolean =
            id == other.id && kind == other.kind && wind == other.wind && memory == other.memory &&
                knowledge == other.knowledge && skills.sortedBy { it.nom } == other.skills.sortedBy { it.nom }
    }

    companion object {
        private val JSON_TYPE = "application/json".toMediaType()
        private val SSE_TYPE = "text/event-stream".toMediaType()

        /**
         * Un sol client per a tota l'app: comparteix connexions entre
         * instàncies. Mateixos temps que abans: 15 s per connectar i fins a
         * 4 minuts per resposta (un torn llarg); l'enviament (pujar un
         * document gran) té el mateix marge.
         */
        internal val http: OkHttpClient by lazy {
            OkHttpClient.Builder()
                .connectTimeout(15, TimeUnit.SECONDS)
                .readTimeout(240, TimeUnit.SECONDS)
                .writeTimeout(240, TimeUnit.SECONDS)
                .build()
        }

        /**
         * El servidor ha rebutjat el token (sessió tancada, contrasenya
         * canviada o compte esborrat). El ViewModel decideix si és la connexió
         * d'ara i, si ho és, torna a la pantalla d'entrar.
         */
        @Volatile
        var onUnauthorized: ((base: String, token: String) -> Unit)? = null

        fun parseWindLibrary(raw: String): List<WindLibraryItem> {
            val items = GregalJson.decodeFromString<LibraryDto>(raw).items ?: throw IllegalArgumentException("Invalid library: no items")
            return items.map { item ->
                val id = requireNotNull(item.id) { "Invalid library identity" }
                val kind = requireNotNull(item.kind) { "Invalid library kind" }
                val windObj = requireNotNull(item.wind) { "Invalid library wind" }
                val wind = Wind.parse(JsonObject(mapOf("winds" to JsonArray(listOf(windObj)))).toString()).single()
                require(id.isNotBlank() && wind.id.isNotBlank()) { "Invalid library identity" }
                WindLibraryItem(id, kind, wind, item.memory, item.knowledge,
                    item.skills.map { sk ->
                        val name = sk.name
                        require(name.isNotBlank()) { "Invalid library skill" }
                        Coneixement(name, sk.titleText, sk.descriptionText, sk.bodyText)
                    }, item.created, item.origin)
            }
        }
    }

    fun getWindLibrary(): List<WindLibraryItem> = parseWindLibrary(read("/api/winds/library"))

    fun restoreWindLibrary(sourceId: String) {
        val result = postAs<OkDto>("/api/winds/library", JSONObject().put("action", "restore").put("source", sourceId))
        check(result.ok == true) { "Restore was not acknowledged" }
    }

    fun assignWindLibrary(sourceId: String, targetId: String, memory: Boolean, knowledge: Boolean) {
        val result = postAs<OkDto>("/api/winds/library", JSONObject().put("action", "assign").put("source", sourceId)
            .put("target", targetId).put("memory", memory).put("knowledge", knowledge))
        check(result.ok == true) { "Assignment was not acknowledged" }
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

    fun getWindsResum(): List<WindResum> =
        getAs<ResumDto>("/api/winds/resum").vents.map { v ->
            WindResum(v.id, v.nom, v.rol, v.coneixements.map { it.toConeixement(withBody = false) })
        }

    /** Colles predefinides (Dia a dia, Ciència de dades, Software i seguretat). */
    data class SquadMember(val key: String, val name: String, val ca: String, val en: String, val color: String, val style: String)
    data class Squad(val id: String, val ca: String, val en: String, val members: List<SquadMember>)

    fun getSquads(): List<Squad> =
        getAs<SquadsDto>("/api/winds/squads").squads.map { s ->
            Squad(s.id, s.ca, s.en, s.members.map { e -> SquadMember(e.key, e.name, e.ca, e.en, e.color, e.style) })
        }

    /** Qui pot fer un encàrrec: vents que ja hi encaixen i especialistes nous. */
    fun crewPropose(task: String, lang: String): CrewProposal =
        CrewProposal.parse(post("/api/winds/crew/propose", JSONObject().put("task", task).put("lang", lang)))

    /** Convoca la colla: crea els vents nous, obre la rotllana i torna'n l'id. */
    fun crewStart(task: String, lang: String, p: CrewProposal): String {
        val body = JSONObject().put("task", task).put("lang", lang)
            .put("reuse", JSONArray(p.reuse.map { it.id }))
            .put("create", JSONArray(p.create.map { it.json() }))
        return postAs<CrewStartDto>("/api/winds/crew/start", body).rotllana?.id.orEmpty()
    }

    /** Pas de la feina d'una colla convocada: discussing, waiting, working, done, failed. */
    fun crewStatus(rotllanaId: String): Pair<Boolean, String> {
        val o = getAs<CrewStatusDto>("/api/winds/crew/status?rotllana=" + URLEncoder.encode(rotllanaId, "UTF-8"))
        val running = (o.running as? JsonPrimitive)?.takeIf { !it.isString }?.booleanOrNull
        require(running != null) { "Invalid crew status" }
        return running to o.step
    }

    /** Activa una colla: crea els especialistes que falten (idempotent). */
    fun activateSquad(id: String, lang: String): Boolean =
        postAs<OkDto>("/api/winds/squads", JSONObject().put("id", id).put("lang", lang)).ok ?: false

    /** Envia un missatge al grup; respon el vent que toca (o el demanat). */
    fun rotllanaChat(rotllanaId: String, message: String, wind: String = "", cont: Boolean = false): Triple<String, String, List<GregalViewModel.PetMsg>> {
        val o = postAs<RotllanaDoneDto>("/api/winds/rotllana", JSONObject(if (cont) RotllanaRequests.chatContinue(rotllanaId, wind) else RotllanaRequests.chat(rotllanaId, message, wind)))
        val hist = o.history.map { it.toPetMsg() }
        return Triple(o.reply?.wind.orEmpty(), o.reply?.text.orEmpty(), if (o.parcial) markParcial(hist) else hist)
    }

    fun getRotllanaChat(rotllanaId: String): List<GregalViewModel.PetMsg> =
        getAs<HistoryDto>("/api/winds/rotllana?rotlla=$rotllanaId").history.map { it.toPetMsg() }

    fun deleteWind(id: String) {
        read("/api/winds", "DELETE", JSONObject().put("id", id).toString())
    }

    fun windChat(windId: String, message: String, cont: Boolean = false, session: String = ""): WindReply =
        windReplyOf(read(windThreadPath("/api/winds/chat", session), "POST", windChatPayload(windId, message, cont).toString()))

    /** Sessions d'un vent: fils separats d'un mateix vent (com /api/sessions del xat general). */
    fun windSessions(windId: String): List<WindSessionInfo> =
        getAs<WindSessionsDto>("/api/winds/sessions?wind=" + URLEncoder.encode(windId, "UTF-8")).sessions.map { s ->
            WindSessionInfo(s.name, s.title, s.msgs, s.whenText, s.current)
        }

    /** Crea una sessió nova del vent i l'activa (el fil del vent comença de zero). */
    fun windSessionNew(windId: String): String =
        postAs<NameDto>("/api/winds/sessions", JSONObject().put("wind", windId)).name

    /** Obre una sessió concreta del vent: passa a ser l'activa i en torna l'historial. */
    fun windSessionOpen(windId: String, name: String): List<GregalViewModel.PetMsg> =
        postAs<HistoryDto>("/api/winds/session/open", JSONObject().put("wind", windId).put("name", name)).history.map { it.toPetMsg() }

    /** Esborra una sessió del vent (no l'activa). */
    fun windSessionDelete(windId: String, name: String) {
        read("/api/winds/sessions", "DELETE", JSONObject().put("wind", windId).put("name", name).toString())
    }

    /** Feines dels vents (resum: estat, qui, fitxers i resultat). */
    /** La mascota: nom, cara, fil i el resum del que ha passat des de l'última visita. */
    fun cap(lang: String): CapState =
        CapState.parse(read("/api/winds/cap?lang=" + URLEncoder.encode(lang, "UTF-8")))

    /** Li diu una cosa a la mascota; force = el camí que tria l'usuari ("self", "wind:<id>", "rotllana", "crew"). */
    fun capAsk(message: String, lang: String, force: String = ""): CapDecision =
        CapDecision.parse(read("/api/winds/cap", "POST", JSONObject().put("message", message).put("lang", lang).put("force", force).toString()))

    /** Cuidar-la: "pet", "feed:galeta", "game:nuvols:12", "sleep", "wear:gorra". */
    fun capPlay(act: String, lang: String): Triple<CapPet, String, List<String>> {
        val o = postAs<CapPlayDto>("/api/winds/cap/play", JSONObject().put("act", act).put("lang", lang))
        return Triple(o.pet ?: CapPet(), o.say, o.unlocked)
    }

    /** Endavant: crea els especialistes, obre el xat i, si cal, el curs accelerat. */
    fun capGo(message: String, lang: String, winds: List<String>, create: List<CapNew>, course: Boolean): CapGoResult {
        val o = postAs<CapGoDto>("/api/winds/cap/go", JSONObject().put("message", message).put("lang", lang)
            .put("winds", JSONArray(winds)).put("create", JSONArray(GregalJson.encodeToString(create))).put("course", course))
        return CapGoResult(o.wind, o.rotllana, o.studying, o.created.map { it.id })
    }

    /** Apunta al fil del Ventet on ha anat a parar una delegació. */
    fun capLink(link: String, text: String) {
        post("/api/winds/cap/link", JSONObject().put("link", link).put("text", text))
    }

    /** Últim missatge de cada conversa (wind:<id>, rot:<id>, cap) per a la llista. */
    fun summaries(): Map<String, ConvoSummary> =
        getAs<SummariesDto>("/api/winds/summaries").items.map { j ->
            ConvoSummary(j.key, j.from, j.speaker, j.text, msgMillis(j.ts), j.count)
        }.associateBy { it.key }

    fun capRename(name: String): CapState {
        val o = postAs<CapProfileDto>("/api/winds/cap/profile", JSONObject().put("name", name))
        return CapState(name = o.name, color = o.color, style = o.style)
    }

    fun windTasks(): List<WindTask> {
        fun ms(s: String): Long = try { java.time.Instant.parse(s).toEpochMilli() } catch (_: Exception) { 0L }
        return getAs<WindTasksDto>("/api/winds/tasks?summary=1").tasks.map { t ->
            WindTask(
                id = t.id, task = t.task, workspace = t.workspace,
                thread = t.thread, created = ms(t.created), updated = ms(t.updated),
                status = t.status,
                agents = t.agents.keys.sorted().map { k -> t.agents[k].looseString() }.filter { n -> n.isNotBlank() },
                files = t.files.filter { f -> f.isNotBlank() },
                result = t.result, error = t.error,
            )
        }
    }

    /** Text d'un fitxer (GET /api/file). null + motiu si és binari o massa gran. */
    fun readTextFile(path: String): Pair<String?, String?> {
        val o = getAs<FileDto>("/api/file?path=" + URLEncoder.encode(path, "UTF-8"))
        return when {
            o.binary -> null to "binary"
            o.tooBig -> null to "too_big"
            else -> o.content to null
        }
    }

    /** Veus proactives posteriors a desDe (unix). Lleuger: el vigilant ho
     *  crida en segon pla sense descarregar historials. */
    fun windNovetats(desDe: Long): List<WindNovetat> =
        AvuiJson.novetats(read("/api/winds/novetats?des_de=$desDe"))

    /** La safata «Avui» (GET /api/winds/avui). */
    fun avui(): AvuiView = AvuiJson.view(read("/api/winds/avui"))

    /** Una decisió sobre una targeta: done | later | dismiss | useful | notuseful | save | reopen. */
    fun avuiAction(id: String, action: String): Boolean =
        postAs<OkDto>("/api/winds/avui", JSONObject().put("id", id).put("action", action)).ok ?: false

    fun avuiSettings(): AvuiSettings = AvuiJson.settings(read("/api/winds/avui/settings"))

    /** Desa la configuració; [tune] null = no el toquis. */
    fun saveAvuiSettings(s: AvuiSettings, tune: String? = null): AvuiSettings {
        val body = JSONObject().put("quiet_from", s.quietFrom).put("quiet_to", s.quietTo).put("level", s.level)
        if (tune != null) body.put("tune", tune)
        return AvuiJson.settings(read("/api/winds/avui/settings", "POST", body.toString()))
    }

    /** Resposta d'un torn amb streaming (SSE per POST). */
    sealed interface WindStreamEvent {
        data class Phase(val phase: String) : WindStreamEvent
        data class Activity(val phase: String, val tool: String = "", val step: Int = 0) : WindStreamEvent
        data class Token(val token: String) : WindStreamEvent
        data class DoneWind(val reply: WindReply) : WindStreamEvent
        data class DoneRotllana(val wind: String, val text: String, val history: List<GregalViewModel.PetMsg>, val parcial: Boolean = false, val silenci: Boolean = false, val failed: List<String> = emptyList()) : WindStreamEvent
        data class Error(val message: String) : WindStreamEvent
        /** Un vent espera una resposta (aprovació o pregunta). */
        data class Ask(val ask: WindAsk) : WindStreamEvent
        /** La petició ja no espera (contestada, caducada o torn aturat). */
        data class AskResolved(val key: String) : WindStreamEvent
    }

    /** Una aprovació («approve») o una pregunta amb opcions («question») d'un vent. */
    data class WindAsk(
        val key: String, val type: String, val query: String = "",
        val options: List<Pair<String, String>> = emptyList(),
        val name: String = "", val kind: String = "", val what: String = "", val reason: String = "",
    )

    /** GET /api/winds/pending: el que els vents esperen (si l'app s'havia tancat). */
    fun getWindAsks(): List<WindAsk> {
        val arr = getAs<PendingDto>("/api/winds/pending").pending ?: throw IOException("resposta sense pending (servidor antic?)")
        return arr.map { it.toAsk() }.filter { it.key.isNotBlank() }
    }

    /** POST /api/winds/answer: respon una aprovació (approve) o una pregunta (answer). */
    fun answerWindAsk(key: String, approve: Boolean = false, answer: String = "") {
        post("/api/winds/answer", JSONObject().put("key", key).put("approve", approve).put("answer", answer))
    }

    /**
     * Llegeix un SSE (event:/data:) cridant onEvent a mesura que arriba.
     * Pur excepte la lectura: parseWindStreamEvent es pot provar sol.
     */
    fun streamPost(path: String, payload: JSONObject, onEvent: (WindStreamEvent) -> Unit) =
        sseLoop(path, payload) { ev, data ->
            val e = parseWindStreamEvent(ev, data)
            if (e is WindStreamEvent.Error) throw ApiException(e.message.ifBlank { "stream error" })
            if (e != null) onEvent(e)
            e is WindStreamEvent.DoneWind || e is WindStreamEvent.DoneRotllana
        }

    /**
     * El Ventet en flux (POST /api/winds/cap/stream): onText rep el text de la
     * resposta a mesura que s'escriu; torna la decisió del done. Sense la ruta
     * (servidor antic), WindStreamUnavailable i el POST de sempre.
     */
    fun capAskStream(message: String, lang: String, force: String = "", onText: (String) -> Unit): CapDecision {
        var done: CapDecision? = null
        sseLoop("/api/winds/cap/stream", JSONObject().put("message", message).put("lang", lang).put("force", force)) { ev, data ->
            val o = try { GregalJson.parseToJsonElement(data) as? JsonObject } catch (_: Exception) { null } ?: return@sseLoop false
            when (ev) {
                "token" -> { onText(o.str("token")); false }
                "error" -> throw ApiException(if (o.containsKey("error")) o.str("error") else "stream error")
                "done" -> { done = CapDecision.parse(data); true }
                else -> false
            }
        }
        return done ?: throw ApiException("stream tancat sense done")
    }

    /** Llegeix un flux SSE: onRaw(event, data) torna cert quan ja n'hi ha prou. */
    private fun sseLoop(path: String, payload: JSONObject, onRaw: (String?, String) -> Boolean) {
        val r = exec(path, "POST", payload.toString())
        r.use {
            val code = r.code
            // Only an explicit unsupported route may be retried as a classic POST.
            // Timeout, server error, or EOF may mean tools have ALREADY run.
            if (code in listOf(404, 405, 501)) throw WindStreamUnavailable()
            if (code !in 200..299) throw ApiException("HTTP $code: ${bodyText(r).take(200)}")
            if (r.header("Content-Type").orEmpty().startsWith("text/html")) throw WindStreamUnavailable()
            readEvents(r) { ev, data -> data.isNotEmpty() && onRaw(ev, data) }
        }
    }

    /**
     * Passa els events d'una resposta SSE ja oberta (okhttp-sse) a [onEvent],
     * que torna cert per plegar. Una excepció de [onEvent] o de la xarxa es
     * rellança. El cos es llegeix com a flux encara que el servidor no en
     * declari el tipus (com feia el lector d'abans): okhttp-sse només mira la
     * capçalera.
     */
    private fun readEvents(r: Response, onEvent: (type: String?, data: String) -> Boolean) {
        val body = r.body
        val stream = if (body.contentType()?.let { it.type == "text" && it.subtype == "event-stream" } == true) r
            else r.newBuilder().body(body.source().asResponseBody(SSE_TYPE, -1L)).build()
        var failure: Throwable? = null
        var stopped = false
        EventSources.processResponse(stream, object : EventSourceListener() {
            override fun onEvent(eventSource: EventSource, id: String?, type: String?, data: String) {
                if (stopped) return
                try {
                    if (onEvent(type, data)) { stopped = true; eventSource.cancel() }
                } catch (t: Throwable) {
                    failure = t; stopped = true; eventSource.cancel()
                }
            }

            override fun onFailure(eventSource: EventSource, t: Throwable?, response: Response?) {
                if (!stopped && failure == null) failure = t ?: IOException("SSE failed (HTTP ${response?.code})")
            }
        })
        failure?.let { throw it }
    }

    fun parseWindStreamEvent(event: String?, data: String): WindStreamEvent? {
        if (data.isEmpty()) return null
        val o = try { GregalJson.parseToJsonElement(data) as? JsonObject } catch (_: Exception) { null } ?: return null
        return when (event) {
            "phase" -> WindStreamEvent.Phase(if (o.containsKey("phase")) o.str("phase") else "working")
            "activity" -> {
                val a = GregalJson.decodeFromJsonElement<ActivityDto>(o)
                WindStreamEvent.Activity(a.phase, a.tool.take(48), a.step.coerceAtLeast(0))
            }
            "token" -> WindStreamEvent.Token(o.str("token"))
            "done" -> if (o.containsKey("history") && o["reply"] is JsonObject) {
                val d = GregalJson.decodeFromJsonElement<RotllanaDoneDto>(o)
                val hist = d.history.map { it.toPetMsg() }
                WindStreamEvent.DoneRotllana(d.reply?.wind.orEmpty(), d.reply?.text.orEmpty(),
                    if (d.parcial) markParcial(hist) else hist, d.parcial, d.silenci, d.failed)
            } else {
                WindStreamEvent.DoneWind(windReplyOf(o))
            }
            "error" -> WindStreamEvent.Error(if (o.containsKey("error")) o.str("error") else "error")
            "approve_request", "question_request" ->
                GregalJson.decodeFromJsonElement<WindAskDto>(o).toAsk().takeIf { it.key.isNotBlank() }?.let { WindStreamEvent.Ask(it) }
            "ask_resolved" -> WindStreamEvent.AskResolved(o.str("key"))
            else -> null
        }
    }

    private fun windReplyOf(raw: String): WindReply = windReplyOf(GregalJson.parseToJsonElement(raw) as JsonObject)

    private fun windReplyOf(o: JsonObject): WindReply {
        val d = GregalJson.decodeFromJsonElement<WindReplyDto>(o)
        val reminds = d.actions.filter { it.kind == "remind" }.map { it.at }.filter { it.isNotBlank() }
        val follow = d.actions.filter { it.kind == "remind" || it.kind == "followup" }
        val remindAt = follow.mapNotNull { a ->
            runCatching { java.time.OffsetDateTime.parse(a.atIso).toInstant().toEpochMilli() }.getOrNull()
        }
        val actions = follow.map { a -> WindTurnAction(a.id, a.kind, a.atIso, a.text) }
        return WindReply(d.reply, reminds, d.parcial, remindAt, actions)
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
    private fun windChatPayload(windId: String, message: String, cont: Boolean): JSONObject =
        JSONObject(if (cont) WindRequests.chatContinue(windId) else WindRequests.chat(windId, message))

    // Older servers ignore this query parameter, but reject unknown JSON fields.
    private fun windThreadPath(path: String, session: String): String {
        if (session.isEmpty()) return path
        val separator = if ('?' in path) '&' else '?'
        return path + separator + "session=" + URLEncoder.encode(session, "UTF-8")
    }

    fun windChatStream(windId: String, message: String, cont: Boolean = false, session: String = "", onEvent: (WindStreamEvent) -> Unit): WindReply {
        var done: WindReply? = null
        streamPost(windThreadPath("/api/winds/chat/stream", session), windChatPayload(windId, message, cont)) { e ->
            if (e is WindStreamEvent.DoneWind) done = e.reply else onEvent(e)
        }
        return done ?: throw ApiException("stream tancat sense done")
    }

    fun rotllanaChatStream(rotllanaId: String, message: String, wind: String = "", cont: Boolean = false, onEvent: (WindStreamEvent) -> Unit): Triple<String, String, List<GregalViewModel.PetMsg>> {
        var done: WindStreamEvent.DoneRotllana? = null
        streamPost("/api/winds/rotllana/stream", JSONObject(if (cont) RotllanaRequests.chatContinue(rotllanaId, wind) else RotllanaRequests.chat(rotllanaId, message, wind))) { e ->
            if (e is WindStreamEvent.DoneRotllana) done = e
            onEvent(e)
        }
        val d = done ?: throw ApiException("stream tancat sense done")
        return Triple(d.wind, d.text, d.history)
    }

    /** Atura el torn en curs d'un vent o d'una rotllana (POST /api/winds/cancel). */
    fun cancelTurn(windId: String = "", rotllanaId: String = "") {
        val body = JSONObject()
        if (windId.isNotEmpty()) body.put("wind", windId)
        if (rotllanaId.isNotEmpty()) body.put("rotllana", rotllanaId)
        read("/api/winds/cancel", "POST", body.toString())
    }

    // Steer: missatge lliurat al torn que treballa (200) o rebutjat (409,
    // no hi ha torn viu: el client l'ha d'enviar com a missatge nou).
    fun steerTurn(windId: String = "", rotllanaId: String = "", text: String): Boolean {
        val body = JSONObject().put("text", text)
        if (windId.isNotEmpty()) body.put("kind", "wind").put("id", windId)
        if (rotllanaId.isNotEmpty()) body.put("kind", "rotllana").put("id", rotllanaId)
        return try {
            read("/api/winds/steer", "POST", body.toString())
            true
        } catch (e: ApiException) {
            if (e.message?.startsWith("HTTP 409") == true) false else throw e
        }
    }

    fun getWindChat(windId: String): List<GregalViewModel.PetMsg> = getWindThread(windId).second

    fun getWindThread(windId: String, session: String = ""): Pair<String, List<GregalViewModel.PetMsg>> {
        val o = getAs<HistoryDto>(windThreadPath("/api/winds/chat?wind=$windId", session))
        return o.session to o.history.map { it.toPetMsg() }
    }

    /** Rutina d'un vent: feina programada que corre el vent de veres (LLM + eines). */
    data class WindRutina(
        val id: String, val wind: String, val windName: String, val name: String,
        val kind: String, val time: String, val intervalH: Double, val prompt: String,
        val enabled: Boolean, val nextDue: String, val nextDueAt: String, val lastStatus: String,
    )

    /** Llista les rutines dels vents (daily/interval): el vent hi corre de veres. */
    /** L'última execució correcta d'una rutina: (inici ISO, text que va dir), o null. */
    fun getWindRutinaLastOutput(id: String): Pair<String, String>? {
        val runs = getAs<RunsDto>("/api/winds/rutines/runs?id=" + URLEncoder.encode(id, "UTF-8")).runs
        for (x in runs) {
            val out = x.output.trim()
            if (x.status == "ok" && out.isNotEmpty()) return x.startedAt to out
        }
        return null
    }

    fun getWindRutines(): List<WindRutina> =
        getAs<RutinesDto>("/api/winds/rutines").rutines.map { x ->
            WindRutina(x.id, x.wind, x.windName, x.name, x.kind, x.time, x.intervalH, x.prompt,
                x.enabled, x.nextDue, x.nextDueAt, x.lastStatus)
        }

    /** Crea una rutina de vent: quan toca, el vent obre un torn real (cerca, pensa, parla). */
    fun createWindRutina(windId: String, kind: String, time: String, intervalH: Double, prompt: String, name: String): Boolean {
        val o = postAs<JobRefDto>("/api/winds/rutina", JSONObject()
            .put("wind", windId).put("kind", kind).put("time", time)
            .put("interval_h", intervalH).put("prompt", prompt).put("name", name))
        return o.job != null
    }

    /** Esborra una rutina de vent. */
    fun deleteWindRutina(id: String): Boolean =
        postAs<OkDto>("/api/winds/rutines/delete", JSONObject().put("id", id)).ok ?: false

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

    /**
     * Plugins del servidor (Racó, Partits, Arena): id → encès. Un servidor
     * antic sense /api/plugins torna un mapa buit (i tot es mostra).
     */
    fun getPlugins(): Map<String, Boolean> =
        getAs<PluginsDto>("/api/plugins").plugins.associate { p -> p.id to p.enabled }

    /** Llegeix els favorits i les seccions (buits si no n'hi ha). */
    /** Com es diuen els ajudants en aquest compte del servidor ({singular, plural}). */
    data class Collective(val singular: String, val plural: String, val asked: Boolean)

    fun getWindCollective(): Collective =
        getAs<CollectiveDto>("/api/winds/collective").let { Collective(it.singular, it.plural, it.asked) }

    fun setWindCollective(singular: String, plural: String) {
        post("/api/winds/collective", JSONObject().put("singular", singular).put("plural", plural))
    }

    /** Noms per a ajudants nous, al gust del nom col·lectiu triat. */
    fun getWindNames(n: Int, lang: String): List<String> =
        getAs<NamesDto>("/api/winds/names?n=$n&lang=$lang").names.filter { it.isNotBlank() }

    fun getWindsMeta(): WindsMeta {
        val m = getAs<MetaDto>("/api/winds/meta").meta ?: MetaBodyDto()
        return WindsMeta(m.favorites, m.sections.map { s -> WindSection(s.id, s.name, s.items) })
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
        val o = postAs<MetaDto>("/api/winds/meta", JSONObject().put("favorites", favorites).put("sections", sections))
        return o.meta != null
    }

    /** Missatges programats i estadístiques dels vents (recordatoris, cors). */
    fun getWindsProgramats(): WindsProgramats {
        val o = getAs<ProgramatsDto>("/api/winds/programats")
        return WindsProgramats(
            reminders = o.reminders.map { x -> ProgramatReminder(x.id, x.windName, x.at, x.text) },
            hearts = o.hearts.map { x -> ProgramatHeart(x.wind, x.windName, x.nextAt, x.lastStatus) },
            stats = o.stats.map { x -> ProgramatStats(x.windName, x.beats, x.spoke, x.silent, x.reminders, x.lastAt) },
        )
    }

    /** Estudi «fes-te expert»: engega una sessió (POST) o consulta el progrés (GET). */
    fun startWindExpert(windId: String, tema: String): Boolean =
        postAs<OkDto>("/api/winds/expert", JSONObject().put("wind", windId).put("tema", tema)).ok ?: false

    /** Wiki de l'espai vents: pàgines de resum amb markdown, gràfics i imatges. */
    data class WikiPage(
        val nom: String, val titol: String, val descripcio: String,
        val cos: String, val updated: String,
    )

    fun getWikiPages(): List<WikiPage> = getAs<WikiPagesDto>("/api/winds/wiki").pages.map { it.toPage() }

    fun getWikiPage(nom: String): WikiPage =
        (getAs<WikiPageRefDto>("/api/winds/wiki/${URLEncoder.encode(nom, "UTF-8")}").page ?: throw ApiException("wiki page not found")).toPage()

    fun saveWikiPage(p: WikiPage): Boolean {
        val o = postAs<WikiPageRefDto>("/api/winds/wiki", JSONObject()
            .put("nom", p.nom).put("titol", p.titol).put("descripcio", p.descripcio).put("cos", p.cos))
        return o.page != null
    }

    fun deleteWikiPage(nom: String): Boolean = try {
        GregalJson.decodeFromString<OkDto>(read("/api/winds/wiki/${URLEncoder.encode(nom, "UTF-8")}", "DELETE")).ok ?: false
    } catch (e: Exception) { false }

    /** Resumeix l'última conversa (vent o rotllana) i la desa com a pàgina nova. */
    fun summarizeToWiki(wind: String? = null, rotllana: String? = null, titol: String = ""): WikiPage? {
        val body = JSONObject()
        if (wind != null) body.put("wind", wind)
        if (rotllana != null) body.put("rotllana", rotllana)
        if (titol.isNotBlank()) body.put("titol", titol)
        return postAs<WikiPageRefDto>("/api/winds/wiki/summary", body).page?.toPage()
    }

    fun getWindExpert(windId: String): WindExpert {
        val o = getAs<ExpertDto>("/api/winds/expert?wind=${URLEncoder.encode(windId, "UTF-8")}")
        return WindExpert(o.wind, o.tema, o.fase, o.pas, o.passos, o.iniciat, o.acabat, o.error)
    }

    fun getJobs(): List<JobItem> =
        getAs<JobsDto>("/api/jobs").jobs.map { j ->
            JobItem(
                id = j.id, name = j.name, prompt = j.prompt,
                kind = j.kind, time = j.time,
                intervalH = j.intervalH, enabled = j.enabled,
                notify = j.notify, lastStatus = j.lastStatus,
                nextDue = j.nextDue,
                lastAt = j.lastAt, nextDueAt = j.nextDueAt,
            )
        }

    fun saveJob(j: JobItem): JobItem {
        val o = postAs<JobRefDto>("/api/jobs/save", JSONObject()
            .put("id", j.id).put("name", j.name).put("prompt", j.prompt)
            .put("kind", j.kind).put("time", j.time).put("interval_h", j.intervalH)
            .put("enabled", j.enabled).put("notify", j.notify))
        val r = o.job as? JsonObject ?: return j
        return j.copy(id = if (r.containsKey("id")) r.str("id") else j.id)
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
        val r = exec("/api/jobs/run", "POST", JSONObject().put("id", id).put("scheduled", true).toString())
        if (r.code == 409) { r.close(); return false }
        read(r)
        return true
    }

    fun getRuns(jobId: String = "", n: Int = 30): List<RunItem> =
        getAs<RunsDto>("/api/jobs/runs?job=$jobId&n=$n").runs.map { r ->
            RunItem(
                id = r.id, jobId = r.jobId, jobName = r.jobName,
                startedAt = r.startedAt, finishedAt = r.finishedAt,
                status = r.status, output = r.output, error = r.error,
            )
        }

    /**
     * Crida SSE (/api/chat o /api/agent). Cada event arriba a [onEvent];
     * retorna en acabar l'stream. Llança ApiException en error HTTP.
     */
    fun stream(path: String, payload: JSONObject, onEvent: (SseEvent) -> Unit) {
        streamRead(exec(path, "POST", payload.toString()), onEvent)
    }

    /** SSE per GET (execució de grafs): mateix Bearer, mateix parser. */
    fun streamGet(path: String, onEvent: (SseEvent) -> Unit) {
        streamRead(exec(path, "GET"), onEvent)
    }

    private fun streamRead(r: Response, onEvent: (SseEvent) -> Unit) = r.use {
        val code = r.code
        if (code == 401) throw ApiException("HTTP 401: server requires a valid token")
        if (code !in 200..299) throw ApiException("HTTP $code: ${bodyText(r).take(200)}")
        // Només els events amb nom i dades (com feia SseParser).
        readEvents(r) { type, data ->
            if (!type.isNullOrEmpty() && data.isNotEmpty()) onEvent(SseEvent(type, data))
            false
        }
    }
}

class ApiException(msg: String) : Exception(msg)

/** Known absent endpoint only: safe to fall back without executing work twice. */
class WindStreamUnavailable : Exception("SSE endpoint unavailable")

/**
 * L'hora d'un missatge en mil·lisegons. El servidor la desa en segons; abans
 * es llegia tal qual i tots els xats semblaven del gener de 1970, de manera
 * que els que acabaves d'escriure al mòbil sempre quedaven a dalt.
 */
internal fun msgMillis(ts: Long): Long = if (ts in 1 until 100_000_000_000L) ts * 1000 else ts


// --- Parsers de la carpeta de treball (a part per poder-los provar) ---

/** `last` arriba com a data ISO o com a mil·lisegons: es tolera totes dues. */
internal fun instantsDeWorkspace(raw: JsonElement?): Long {
    if (raw !is JsonPrimitive) return 0L
    if (!raw.isString) raw.longOrNull?.let { return it }
    val txt = raw.content
    txt.toLongOrNull()?.let { return it }
    txt.toDoubleOrNull()?.let { if (!raw.isString) return it.toLong() }
    return try {
        java.time.Instant.parse(txt).toEpochMilli()
    } catch (_: Exception) {
        0L
    }
}

internal fun parseWorkspaces(raw: String): WorkspacesInfo {
    val o = GregalJson.decodeFromString<WorkspacesDto>(raw)
    return WorkspacesInfo(
        current = o.current,
        project = o.project,
        branch = o.branch,
        list = o.workspaces.filter { it.path.isNotBlank() }.map { e ->
            WorkspaceInfo(
                path = e.path,
                name = e.name.ifBlank { e.path.trimEnd('/').substringAfterLast('/') },
                last = instantsDeWorkspace(e.last),
                missing = e.missing,
            )
        },
    )
}

internal fun parseDirs(raw: String): DirsInfo {
    val o = GregalJson.decodeFromString<DirsDto>(raw)
    return DirsInfo(
        path = o.path,
        name = o.name,
        git = o.git,
        parent = o.parent,
        home = o.home,
        roots = o.roots,
        dirs = o.dirs.filter { it.path.isNotBlank() }.map { e ->
            DirEntry(name = e.name.ifBlank { e.path.substringAfterLast('/') }, path = e.path, git = e.git)
        },
        hidden = o.hidden,
        truncated = o.truncat || o.truncated,
    )
}

/** Lectura pura de les respostes de la safata «Avui» i de les novetats (es prova sense xarxa). */
object AvuiJson {
    fun novetats(raw: String): List<WindNovetat> =
        GregalJson.decodeFromString<NovetatsDto>(raw).novetats.mapNotNull(::novetat)

    /** Resposta de GET /api/winds/novetats?id=… (lectura d'un push). */
    fun novetat(raw: String): WindNovetat? = GregalJson.decodeFromString<NovetatDto1>(raw).novetat?.let(::novetat)

    private fun novetat(j: NovetatDto): WindNovetat? =
        if (j.text.isBlank()) null else WindNovetat(
            ts = j.ts, wind = j.wind, name = j.name, source = j.source, text = j.text,
            job = j.job, card = j.card, quiet = j.quiet, messageId = j.messageId,
            id = j.id, owner = j.owner, read = j.read,
        )

    private fun card(j: AvuiCardDto): AvuiCard = AvuiCard(
        id = j.id, wind = j.wind, windName = j.windName,
        source = j.source, section = j.section.ifBlank { "fyi" },
        title = j.title, why = j.why, body = j.body,
        actions = j.actions.mapNotNull { a -> if (a.label.isBlank()) null else AvuiAction(a.label, a.prompt) },
        state = j.state.ifBlank { "open" }, feedback = j.feedback,
        saved = j.saved, held = j.held,
        created = j.created, days = j.days,
    )

    private fun cards(a: List<AvuiCardDto>): List<AvuiCard> = a.map(::card).filter { it.id.isNotBlank() }

    private fun settings(o: AvuiSettingsDto?): AvuiSettings {
        if (o == null) return AvuiSettings()
        return AvuiSettings(
            quietFrom = o.quietFrom.ifBlank { "22:00" },
            quietTo = o.quietTo.ifBlank { "08:00" },
            level = if (o.level == "calm") "calm" else "normal",
            tune = o.tune,
        )
    }

    /** La configuració de la safata (GET o POST /api/winds/avui/settings). */
    fun settings(raw: String): AvuiSettings = settings(GregalJson.decodeFromString<AvuiSettingsDto>(raw))

    fun view(raw: String): AvuiView {
        val o = GregalJson.decodeFromString<AvuiViewDto>(raw)
        return AvuiView(
            date = o.date,
            now = cards(o.now), fyi = cards(o.fyi), ahead = cards(o.ahead), done = cards(o.done),
            held = o.held, settings = settings(o.settings),
        )
    }

    /** «HH:MM» vàlid (00:00–23:59). */
    fun validTime(s: String): Boolean = Regex("""([01]\d|2[0-3]):[0-5]\d""").matches(s.trim())
}

// --- Models de lectura (kotlinx.serialization) ---
// Tots els camps tenen valor per defecte: un servidor antic que no els envia
// no trenca res. Els noms són els del JSON del servidor; després es passen
// als models de l'app (ServerState, PetMsg…).

@Serializable
internal class HealthDto(
    val status: String = "", val protocol: String = "",
    @SerialName("instance_id") val instanceId: String = "",
    val capabilities: Map<String, Boolean> = emptyMap(),
)

@Serializable
internal class TranscriptDto(val role: String = "", val content: JsonElement? = null)

@Serializable
internal class StateDto(
    val role: List<String> = emptyList(),
    @SerialName("current_role") val currentRole: String = "",
    val mode: String = "", val model: String = "", val provider: String = "",
    val verify: String = "manual",
    @SerialName("agent_busy") val agentBusy: Boolean = false,
    val cwd: String = "", val project: String = "", val branch: String = "",
    @SerialName("used_tokens") val usedTokens: Int = 0,
    @SerialName("model_name") val modelName: String = "",
    val think: String = "",
    @SerialName("think_options") val thinkOptions: List<String> = emptyList(),
    @SerialName("think_effective") val thinkEffective: String = "",
    @SerialName("context_window") val contextWindow: Int = 0,
    val permissive: Boolean = false,
    @SerialName("auto_review") val autoReview: Boolean = false,
    val access: String = "edits",
    val transcript: List<TranscriptDto> = emptyList(),
)

@Serializable
internal class ModelsDto(
    val models: Map<String, List<String>> = emptyMap(),
    val current: String = "",
    @SerialName("vents_selected") val ventsSelected: String = "",
)

@Serializable internal class TokenDto(val token: String = "")
@Serializable internal class InvitationDto(val code: String = "")
@Serializable internal class MsgDto(val msg: String = "")
@Serializable internal class PlanDto(val plan: String = "")
@Serializable internal class SummaryDto(val summary: String = "canvis desfets")
@Serializable internal class OkDto(val ok: Boolean? = null)
@Serializable internal class CancelDto(val cancelled: Boolean? = null, val ok: Boolean? = null)
@Serializable internal class ReplyTextDto(val reply: String = "")
@Serializable internal class NotesDto(val notes: List<String> = emptyList())
@Serializable internal class MemoriaDto(val memoria: String = "")
@Serializable internal class NomDto(val nom: String? = null)
@Serializable internal class NameDto(val name: String = "")
@Serializable internal class NamesDto(val names: List<String> = emptyList())
@Serializable internal class SuggestDto(val persona: String = "", val style: String = "")
@Serializable internal class GithubDto(val output: String = "")
@Serializable internal class JobRefDto(val job: JsonElement? = null)
@Serializable internal class CursorDto(val cursor: Long? = null)

@Serializable
internal class OfficeDto(
    val id: String = "", val name: String = "", val kind: String = "", val size: Int = 0,
    val path: String = "", val text: String = "", val result: String = "",
)

@Serializable internal class NameUrlDto(val name: String = "", val url: String = "")
@Serializable internal class MailDto(val user: String = "", @SerialName("has_password") val hasPassword: Boolean = false)
@Serializable internal class BriefingDto(val enabled: Boolean = false, val time: String = "", val wind: String = "")

@Serializable
internal class ConnectionsDto(
    val calendars: List<NameUrlDto> = emptyList(),
    val mail: MailDto = MailDto(),
    val briefing: BriefingDto = BriefingDto(),
)

@Serializable internal class CalendarTestDto(@SerialName("events_next_7_days") val eventsNext7Days: Int = 0)
@Serializable internal class MailTestDto(val error: String? = null, val unread: Int = 0)
@Serializable internal class ConnectionsTestDto(val calendar: CalendarTestDto? = null, val mail: MailTestDto? = null)

@Serializable internal class TunnelDto(val url: String = "", val target: String = "", val status: String = "")

@Serializable
internal class ServerPowersDto(
    val enabled: Boolean = false, val tools: List<String> = emptyList(), val user: String = "",
    val notes: String = "", val tunnels: List<TunnelDto> = emptyList(),
)

@Serializable
internal class MeDto(val user: String = "", val home: String = "", val admin: Boolean = false, val roots: List<String> = emptyList())

@Serializable internal class PreviewWebDto(val path: String = "", val name: String = "", val size: Long = 0L, val mtime: String = "")

@Serializable
internal class PreviewDto(
    val root: String = "", val base: String = "", val index: String = "", val webs: List<PreviewWebDto> = emptyList(),
)

@Serializable
internal class ParallelItemDto(
    val role: String = "", val provider: String = "", val model: String = "", val reply: String = "", val error: String = "",
)

@Serializable internal class ParallelDto(val results: List<ParallelItemDto> = emptyList())

@Serializable
internal class SessionDto(
    val name: String = "", val msgs: Int = 0, @SerialName("when") val whenText: String = "",
    val title: String = "", val at: String = "", val current: Boolean = false,
)

@Serializable
internal class RunDto(
    val id: Long = 0L, val state: String = "", val session: String = "", val workspace: String = "", val error: String = "",
)

@Serializable
internal class DurableEventDto(
    val id: Long = 0L,
    @SerialName("session_id") val sessionId: String = "",
    @SerialName("run_id") val runId: Long = 0L,
    val kind: String = "", val text: String = "",
)

@Serializable
internal class EventsDto(val events: List<JsonObject> = emptyList(), val cursor: Long? = null, val next: Long? = null)

@Serializable internal class FlowItemDto(val name: String = "", val slug: String = "", val desc: String = "", val steps: Int = 0)
@Serializable internal class FlowsDto(val flows: List<FlowItemDto> = emptyList())

@Serializable
internal class FlowNodeDto(
    val id: String = "", val kind: String = "", val title: String = "", val task: String = "", val tool: String = "", val text: String = "",
)

@Serializable internal class FlowEdgeDto(val from: String = "", val to: String = "", @SerialName("when") val whenText: String = "")

@Serializable
internal class FlowDto(
    val name: String = "", val desc: String = "",
    val nodes: List<FlowNodeDto> = emptyList(), val edges: List<FlowEdgeDto> = emptyList(),
)

@Serializable internal class FlowLoadDto(val flow: FlowDto? = null)

@Serializable internal class LiveEventDto(val kind: String = "activity", val text: String = "")

@Serializable
internal class ActiveAgentDto(
    val id: Int = 0, val task: String = "", @SerialName("started_at") val startedAt: String = "",
    val role: String = "", val mode: String = "", val project: String = "", val events: List<LiveEventDto> = emptyList(),
)

@Serializable internal class ActiveDto(val active: ActiveAgentDto? = null)

@Serializable internal class GoalDto(val id: String = "", val title: String = "", val body: String = "", val status: String = "")
@Serializable internal class GoalsDto(val goals: List<GoalDto> = emptyList())

/** Un missatge d'un fil (vent, rotllana, mascota): `ts` en segons o mil·lisegons. */
@Serializable
internal class PetMsgDto(
    val from: String = "", val text: String = "",
    @SerialName("speaker_id") val speakerId: String = "",
    @SerialName("speaker_name") val speakerName: String = "",
    val ts: Long = 0L, val kind: String = "", val id: String = "",
    val relatedRunId: Long = 0L, val resultRefs: List<String> = emptyList(),
) {
    fun toPetMsg() = GregalViewModel.PetMsg(from, text, speakerId, speakerName, ts = msgMillis(ts), kind = kind, id = id,
        relatedRunId = relatedRunId, resultRefs = resultRefs)
}

@Serializable internal class HistoryDto(val history: List<PetMsgDto> = emptyList(), val session: String = "")

@Serializable internal class WindTextDto(val wind: String = "", val text: String = "")

@Serializable
internal class RotllanaDoneDto(
    val reply: WindTextDto? = null,
    val history: List<PetMsgDto> = emptyList(),
    val parcial: Boolean = false,
    val silenci: Boolean = false,
    val failed: List<String> = emptyList(),
)

@Serializable
internal class TurnActionDto(
    val id: String = "", val kind: String = "", val at: String = "",
    @SerialName("at_iso") val atIso: String = "", val text: String = "",
)

@Serializable
internal class WindReplyDto(val reply: String = "", val parcial: Boolean = false, val actions: List<TurnActionDto> = emptyList())

@Serializable internal class ActivityDto(val phase: String = "working", val tool: String = "", val step: Int = 0)

@Serializable internal class AskOptionDto(val label: String = "", val description: String = "")

@Serializable
internal class WindAskDto(
    val key: String = "", val type: String = "", val query: String = "",
    val options: List<AskOptionDto> = emptyList(),
    val name: String = "", val kind: String = "", val what: String = "", val reason: String = "",
) {
    fun toAsk() = GregalApi.WindAsk(
        key = key, type = type.ifBlank { "approve" }, query = query,
        options = options.map { it.label to it.description }.filter { it.first.isNotBlank() },
        name = name, kind = kind, what = what, reason = reason,
    )
}

@Serializable internal class PendingDto(val pending: List<WindAskDto>? = null)

@Serializable
internal class ConeixementDto(val nom: String = "", val titol: String = "", val descripcio: String = "", val cos: String = "") {
    fun toConeixement(withBody: Boolean) = GregalApi.Coneixement(nom, titol, descripcio, if (withBody) cos else "")
}

@Serializable internal class ConeixementsDto(val coneixements: List<ConeixementDto> = emptyList())

/** Una habilitat de la biblioteca: el servidor la desa amb els noms de Go (Nom, Titol…). */
@Serializable
internal class LibrarySkillDto(
    @SerialName("Nom") val goNom: String? = null, @SerialName("Titol") val goTitol: String? = null,
    @SerialName("Descripcio") val goDescripcio: String? = null, @SerialName("Cos") val goCos: String? = null,
    val nom: String = "", val titol: String = "", val descripcio: String = "", val cos: String = "",
) {
    val name get() = goNom ?: nom
    val titleText get() = goTitol ?: titol
    val descriptionText get() = goDescripcio ?: descripcio
    val bodyText get() = goCos ?: cos
}

@Serializable
internal class LibraryItemDto(
    val id: String? = null, val kind: String? = null, val wind: JsonObject? = null,
    val memory: String = "", val knowledge: String = "",
    val skills: List<LibrarySkillDto> = emptyList(), val created: String = "", val origin: String = "",
)

@Serializable internal class LibraryDto(val items: List<LibraryItemDto>? = null)

@Serializable
internal class ResumItemDto(val id: String = "", val nom: String = "", val rol: String = "", val coneixements: List<ConeixementDto> = emptyList())

@Serializable internal class ResumDto(val vents: List<ResumItemDto> = emptyList())

@Serializable
internal class SquadMemberDto(
    val key: String = "", val name: String = "", val ca: String = "", val en: String = "", val color: String = "", val style: String = "",
)

@Serializable
internal class SquadDto(val id: String = "", val ca: String = "", val en: String = "", val members: List<SquadMemberDto> = emptyList())

@Serializable internal class SquadsDto(val squads: List<SquadDto> = emptyList())

@Serializable internal class IdDto(val id: String = "")
@Serializable internal class CrewStartDto(val rotllana: IdDto? = null)
@Serializable internal class CrewStatusDto(val running: JsonElement? = null, val step: String = "")

@Serializable
internal class WindSessionDto(
    val name: String = "", val title: String = "", val msgs: Int = 0,
    @SerialName("when") val whenText: String = "", val current: Boolean = false,
)

@Serializable internal class WindSessionsDto(val sessions: List<WindSessionDto> = emptyList())

@Serializable
internal class CapPlayDto(val pet: CapPet? = null, val say: String = "", val unlocked: List<String> = emptyList())

@Serializable
internal class CapGoDto(
    val wind: String = "", val rotllana: String = "", val studying: List<String> = emptyList(), val created: List<IdDto> = emptyList(),
)

@Serializable internal class CapProfileDto(val name: String = "", val color: String = "", val style: String = "")

@Serializable
internal class SummaryItemDto(
    val key: String = "", val from: String = "", val speaker: String = "", val text: String = "", val ts: Long = 0L, val count: Int = 0,
)

@Serializable internal class SummariesDto(val items: List<SummaryItemDto> = emptyList())

@Serializable
internal class WindTaskDto(
    val id: String = "", val task: String = "", val workspace: String = "", val thread: String = "",
    val created: String = "", val updated: String = "", val status: String = "",
    val agents: Map<String, JsonElement> = emptyMap(), val files: List<String> = emptyList(),
    val result: String = "", val error: String = "",
)

@Serializable internal class WindTasksDto(val tasks: List<WindTaskDto> = emptyList())

@Serializable
internal class FileDto(val binary: Boolean = false, @SerialName("too_big") val tooBig: Boolean = false, val content: String = "")

@Serializable
internal class RunItemDto(
    val id: String = "",
    @SerialName("job_id") val jobId: String = "",
    @SerialName("job_name") val jobName: String = "",
    @SerialName("started_at") val startedAt: String = "",
    @SerialName("finished_at") val finishedAt: String = "",
    val status: String = "", val output: String = "", val error: String = "",
)

@Serializable internal class RunsDto(val runs: List<RunItemDto> = emptyList())

@Serializable
internal class RutinaDto(
    val id: String = "", val wind: String = "", @SerialName("wind_name") val windName: String = "", val name: String = "",
    val kind: String = "", val time: String = "", @SerialName("interval_h") val intervalH: Double = Double.NaN, val prompt: String = "",
    val enabled: Boolean = false, @SerialName("next_due") val nextDue: String = "",
    @SerialName("next_due_at") val nextDueAt: String = "", @SerialName("last_status") val lastStatus: String = "",
)

@Serializable internal class RutinesDto(val rutines: List<RutinaDto> = emptyList())

@Serializable internal class PluginDto(val id: String = "", val enabled: Boolean = true)
@Serializable internal class PluginsDto(val plugins: List<PluginDto> = emptyList())

@Serializable internal class CollectiveDto(val singular: String = "", val plural: String = "", val asked: Boolean = false)

@Serializable internal class SectionDto(val id: String = "", val name: String = "", val items: List<String> = emptyList())
@Serializable internal class MetaBodyDto(val favorites: List<String> = emptyList(), val sections: List<SectionDto> = emptyList())
@Serializable internal class MetaDto(val meta: MetaBodyDto? = null)

@Serializable
internal class ReminderDto(val id: String = "", @SerialName("wind_name") val windName: String = "", val at: String = "", val text: String = "")

@Serializable
internal class HeartDto(
    val wind: String = "", @SerialName("wind_name") val windName: String = "",
    @SerialName("next_at") val nextAt: String = "", @SerialName("last_status") val lastStatus: String = "",
)

@Serializable
internal class StatsDto(
    @SerialName("wind_name") val windName: String = "", val beats: Int = 0, val spoke: Int = 0,
    val silent: Int = 0, val reminders: Int = 0, @SerialName("last_at") val lastAt: String = "",
)

@Serializable
internal class ProgramatsDto(
    val reminders: List<ReminderDto> = emptyList(), val hearts: List<HeartDto> = emptyList(), val stats: List<StatsDto> = emptyList(),
)

@Serializable
internal class WikiPageDto(
    val nom: String = "", val titol: String = "", val descripcio: String = "", val cos: String = "", val updated: String = "",
) {
    fun toPage() = GregalApi.WikiPage(nom, titol, descripcio, cos, updated)
}

@Serializable internal class WikiPagesDto(val pages: List<WikiPageDto> = emptyList())
@Serializable internal class WikiPageRefDto(val page: WikiPageDto? = null)

@Serializable
internal class ExpertDto(
    val wind: String = "", val tema: String = "", val fase: String = "", val pas: Int = 0, val passos: Int = 0,
    val iniciat: Long = 0L, val acabat: Long = 0L, val error: String = "",
)

@Serializable
internal class JobDto(
    val id: String = "", val name: String = "", val prompt: String = "",
    val kind: String = "manual", val time: String = "",
    @SerialName("interval_h") val intervalH: Double = 0.0, val enabled: Boolean = false,
    val notify: Boolean = true, @SerialName("last_status") val lastStatus: String = "",
    @SerialName("next_due") val nextDue: String = "",
    @SerialName("last_at") val lastAt: String = "", @SerialName("next_due_at") val nextDueAt: String = "",
)

@Serializable internal class JobsDto(val jobs: List<JobDto> = emptyList())

@Serializable
internal class WorkspaceDto(val path: String = "", val name: String = "", val last: JsonElement? = null, val missing: Boolean = false)

@Serializable
internal class WorkspacesDto(
    val current: String = "", val project: String = "", val branch: String = "",
    val workspaces: List<WorkspaceDto> = emptyList(),
)

@Serializable internal class DirEntryDto(val name: String = "", val path: String = "", val git: Boolean = false)

@Serializable
internal class DirsDto(
    val path: String = "", val name: String = "", val git: Boolean = false, val parent: String = "", val home: String = "",
    val roots: List<String> = emptyList(), val dirs: List<DirEntryDto> = emptyList(),
    val hidden: Int = 0, val truncat: Boolean = false, val truncated: Boolean = false,
)

@Serializable
internal class NovetatDto(
    val ts: Long = 0L, val wind: String = "", val name: String = "", val source: String = "", val text: String = "",
    val job: String = "", val card: String = "", val quiet: Boolean = false,
    @SerialName("message_id") val messageId: String = "",
    val id: String = "", val owner: String = "", val read: Boolean = false,
)

@Serializable internal class NovetatDto1(val novetat: NovetatDto? = null)

@Serializable internal class NovetatsDto(val novetats: List<NovetatDto> = emptyList())

@Serializable internal class AvuiActionDto(val label: String = "", val prompt: String = "")

@Serializable
internal class AvuiCardDto(
    val id: String = "", val wind: String = "", @SerialName("wind_name") val windName: String = "",
    val source: String = "", val section: String = "", val title: String = "", val why: String = "", val body: String = "",
    val actions: List<AvuiActionDto> = emptyList(), val state: String = "", val feedback: String = "",
    val saved: Boolean = false, val held: Boolean = false, val created: Long = 0L, val days: Int = 0,
)

@Serializable
internal class AvuiSettingsDto(
    @SerialName("quiet_from") val quietFrom: String = "", @SerialName("quiet_to") val quietTo: String = "",
    val level: String = "", val tune: String = "",
)

@Serializable
internal class AvuiViewDto(
    val date: String = "",
    val now: List<AvuiCardDto> = emptyList(), val fyi: List<AvuiCardDto> = emptyList(),
    val ahead: List<AvuiCardDto> = emptyList(), val done: List<AvuiCardDto> = emptyList(),
    val held: Int = 0, val settings: AvuiSettingsDto? = null,
)
