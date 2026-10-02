package xyz.solanes.gregal

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import android.net.Uri
import android.util.Base64
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID

/** Pantalles de l'app. */
enum class Screen { Pairing, Chat, Sessions, Goals, Jobs, Office, Flows, Prefs }

data class Conn(val host: String, val port: String, val token: String, val user: String = "") {
    fun api() = GregalApi(baseUrlOf(host, port), token)
}

class GregalViewModel(app: Application) : AndroidViewModel(app) {
    private val appContext = app.applicationContext
    private val _screen = MutableStateFlow(Screen.Pairing)
    val screen: StateFlow<Screen> = _screen

    private val _conn = MutableStateFlow(Conn("", "", ""))
    val conn: StateFlow<Conn> = _conn

    private val _connected = MutableStateFlow(false)
    val connected: StateFlow<Boolean> = _connected

    // Negociació del protocol: la cua durable només s'activa si també pot
    // transportar interaccions estructurades; altrament /api/agent conserva
    // les aprovacions i preguntes SSE amb el payload complet.
    private val _v2Runs = MutableStateFlow(false)
    val v2Runs: StateFlow<Boolean> = _v2Runs
    private var eventCursor: Long = 0L
    private var activeRunId: Long? = null

    private val _state = MutableStateFlow(ServerState())
    val state: StateFlow<ServerState> = _state

    private val _msgs = MutableStateFlow<List<ChatMsg>>(emptyList())
    val msgs: StateFlow<List<ChatMsg>> = _msgs

    private val _sending = MutableStateFlow(false)
    val sending: StateFlow<Boolean> = _sending

    private val _approve = MutableStateFlow<ApproveReq?>(null)
    val approve: StateFlow<ApproveReq?> = _approve

    private val _question = MutableStateFlow<QuestionReq?>(null)
    val question: StateFlow<QuestionReq?> = _question

    private val _sessions = MutableStateFlow<List<SessionInfo>>(emptyList())
    val sessions: StateFlow<List<SessionInfo>> = _sessions

    private val _active = MutableStateFlow<ActiveAgent?>(null)
    val active: StateFlow<ActiveAgent?> = _active

    private val _goals = MutableStateFlow<List<GoalItem>>(emptyList())
    val goals: StateFlow<List<GoalItem>> = _goals
    private val _jobs = MutableStateFlow<List<JobItem>>(emptyList())
    val jobs: StateFlow<List<JobItem>> = _jobs
    private val _runs = MutableStateFlow<List<RunItem>>(emptyList())
    val runs: StateFlow<List<RunItem>> = _runs

    private val _models = MutableStateFlow<Map<String, List<String>>>(emptyMap())
    val models: StateFlow<Map<String, List<String>>> = _models

    private val _usuari = MutableStateFlow("")
    val usuari: StateFlow<String> = _usuari

    private val _notice = MutableStateFlow("")
    val notice: StateFlow<String> = _notice

    // Actualitzacions (OTA). Sistema portat del de Marea: manifest públic →
    // caché 6 h → avís → descàrrega amb sha256 → instal·lador. La versió
    // silenciada es recorda i la comprovació de l'arrencada no molesta.
    private val _update = MutableStateFlow<UpdateCheckResult?>(null)
    val update: StateFlow<UpdateCheckResult?> = _update
    private val _updateMsg = MutableStateFlow("")
    val updateMsg: StateFlow<String> = _updateMsg
    private val _updateProg = MutableStateFlow(-1)   // -1 aturat, 0..100 baixant
    val updateProg: StateFlow<Int> = _updateProg
    private val _avisObert = MutableStateFlow(false)
    val avisObert: StateFlow<Boolean> = _avisObert

    /** versionCode instal·lat (per comparar amb el manifest). */
    fun versionCodeInstalada(): Int = try {
        @Suppress("DEPRECATION")
        appContext.packageManager.getPackageInfo(appContext.packageName, 0).versionCode
    } catch (_: Exception) { 0 }

    fun versionNameInstalada(): String = try {
        appContext.packageManager.getPackageInfo(appContext.packageName, 0).versionName ?: "?"
    } catch (_: Exception) { "?" }

    fun descartaAvis() { _avisObert.value = false }

    /** No tornar a avisar d'aquesta versió (es recorda entre arrencades). */
    fun ignoraAquestaVersio() {
        val r = _update.value ?: return
        prefsStore?.updateSkipped = r.latestVersionCode
        _avisObert.value = false
        cancelAvisActualitzacio(appContext)
        _updateMsg.value = appText("app.update.skipped", r.latestVersion)
    }

    private fun aplicaResultat(r: UpdateCheckResult, silent: Boolean) {
        _update.value = r
        if (!r.isAvailable) {
            _updateMsg.value = if (r.latestVersion.isBlank()) appText("app.update.unavailable")
            else appText("app.update.current", r.latestVersion)
            return
        }
        _updateMsg.value = appText("app.update.availableStatus", r.latestVersion, r.currentVersion)
        val skipped = prefsStore?.updateSkipped ?: 0
        if (silent && avisAmagat(r.latestVersionCode, skipped)) return
        avisaActualitzacio(appContext, r.latestVersion, r.changelog)
        _avisObert.value = true
    }

    /**
     * Comprova si hi ha versió nova. `silent` = arrencada: no diu res si no
     * n'hi ha o si l'has silenciada, però avisa per notificació si en surt.
     */
    fun checkUpdate(silent: Boolean = false) {
        if (ANDROID_OTA_URL.isBlank()) return
        viewModelScope.launch {
            if (!silent) _updateMsg.value = appText("app.update.checking")
            val actual = versionCodeInstalada()
            val res = withContext(Dispatchers.IO) {
                // Primer la caché (6 h): no truquem el servidor a cada
                // arrencada, i si la xarxa falla encara serveix.
                val cau = llegeixCache(appContext.cacheDir, System.currentTimeMillis())
                if (cau != null && cau.hiHaNova(actual) && isTrustedDownloadUrl(ANDROID_OTA_URL, cau.downloadUrl)) {
                    return@withContext UpdateCheckResult(
                        isAvailable = true, latestVersion = cau.versionName,
                        currentVersion = versionNameInstalada(), changelog = cau.changelog,
                        downloadUrl = cau.downloadUrl, latestVersionCode = cau.versionCode,
                        sha256 = cau.sha256, sizeBytes = cau.midaDeclarada(),
                    )
                }
                var text: String? = null
                var últimError = ""
                for (intent in 0 until 3) {
                    try {
                        text = fetchTextAbsolut(ANDROID_OTA_URL)
                        break
                    } catch (e: Exception) {
                        últimError = e.message ?: "xarxa"
                        if (intent < 2) delay(2000L * (1L shl intent))
                    }
                }
                val cos = text
                if (cos == null) {
                    // Sense xarxa: si la caché (encara que sigui vella) diu
                    // que n'hi ha, val més avisar que callar.
                    if (cau != null && cau.hiHaNova(actual)) {
                        return@withContext UpdateCheckResult(
                            isAvailable = true, latestVersion = cau.versionName,
                            currentVersion = versionNameInstalada(), changelog = cau.changelog,
                            downloadUrl = cau.downloadUrl, latestVersionCode = cau.versionCode,
                            sha256 = cau.sha256, sizeBytes = cau.midaDeclarada(),
                        )
                    }
                    _updateMsg.value = appText("app.update.checkFailed", últimError)
                    return@withContext UpdateCheckResult(currentVersion = versionNameInstalada())
                }
                val info = parseUpdateText(cos)
                if (!manifestPlausible(info) || !isTrustedDownloadUrl(ANDROID_OTA_URL, info.downloadUrl)) {
                    return@withContext UpdateCheckResult(currentVersion = versionNameInstalada())
                }
                desaCache(appContext.cacheDir, cos, System.currentTimeMillis())
                UpdateCheckResult(
                    isAvailable = info.hiHaNova(actual), latestVersion = info.versionName,
                    currentVersion = versionNameInstalada(), changelog = info.changelog,
                    downloadUrl = info.downloadUrl, latestVersionCode = info.versionCode,
                    sha256 = info.sha256, sizeBytes = info.midaDeclarada(),
                )
            }
            aplicaResultat(res, silent)
        }
    }

    /** Baixa l'APK verificat i obre l'instal·lador del sistema. */
    fun baixaIInstalla() {
        val r = _update.value ?: return
        if (!r.isAvailable || r.downloadUrl.isBlank()) return
        if (_updateProg.value in 0..99) return   // ja baixant
        viewModelScope.launch {
            _updateMsg.value = appText("app.update.downloading", r.latestVersion)
            _updateProg.value = 0
            val apk = withContext(Dispatchers.IO) {
                baixaApk(
                    appContext, r.downloadUrl, ANDROID_OTA_URL,
                    r.sha256, r.sizeBytes,
                ) { p -> _updateProg.value = p }
            }
            _updateProg.value = -1
            if (apk == null) {
                _updateMsg.value = appText("app.update.downloadFailed")
                return@launch
            }
            netejaCache(appContext.cacheDir)
            _updateMsg.value = appText("app.update.verified")
            avisaBaixada(appContext, apk, r.latestVersion)
            if (!installaApk(appContext, apk)) {
                _updateMsg.value = appText("app.update.allowUnknown")
            }
        }
    }

    // Rellotge del torn i línia d'estat. La web i el TUI diuen quant fa que
    // l'agent hi és i què està passant (reintent, compactació, ampliació de
    // pressupost); el mòbil només deia «treballant…» i no s'hi veia res més.
    private val _turnStart = MutableStateFlow(0L)
    val turnStart: StateFlow<Long> = _turnStart
    private val _statusLine = MutableStateFlow("")
    val statusLine: StateFlow<String> = _statusLine

    // Esborrany del composer: el que has escrit sobreviu a girar el mòbil, a
    // canviar de pantalla i a morir el procés (com `gregal_draft` a la web).
    private val _draft = MutableStateFlow("")
    val draft: StateFlow<String> = _draft

    /** Desa l'esborrany (cada tecla; SharedPreferences ja ho agrupa). */
    fun saveDraft(v: String) {
        _draft.value = v
        if (::prefsStore.isInitialized) prefsStore.esborrany = v
    }

    fun clearDraft() {
        _draft.value = ""
        if (::prefsStore.isInitialized) prefsStore.esborrany = ""
    }

    // Aparença i llengua (paritat amb les prefs de la web). El ViewModel
    // exposa flows perquè el tema es repinti sol en canviar-lo.
    private lateinit var prefsStore: Prefs
    private val _tema = MutableStateFlow("sistema")
    val tema: StateFlow<String> = _tema
    private val _idioma = MutableStateFlow("en")
    val idioma: StateFlow<String> = _idioma

    /** Enllaça el magatzem de prefs (tema/idioma/esborrany desats). */
    fun attachPrefs(p: Prefs) {
        prefsStore = p
        _tema.value = p.tema
        _idioma.value = p.idioma
        _usuari.value = p.usuari
        if (p.esborrany.isNotEmpty()) _draft.value = p.esborrany
        // A partir d'aquí totes les peticions van a la sessió del mòbil, no a
        // la `default` (la pestanya «principal» de la web).
        sessioApp = p.appSession.ifBlank { "mobil" }
    }

    fun setTema(v: String) {
        _tema.value = v
        if (::prefsStore.isInitialized) prefsStore.tema = v
    }

    /** Canvia l'idioma de la UI i de l'agent (/api/lang, nivell app). */
    fun setIdioma(v: String) {
        _idioma.value = v
        if (::prefsStore.isInitialized) prefsStore.idioma = v
        viewModelScope.launch {
            try { withContext(Dispatchers.IO) { _conn.value.api().post("/api/lang", JSONObject().put("lang", v)) } } catch (_: Exception) {}
        }
    }

    private val _github = MutableStateFlow(GithubResult())
    val github: StateFlow<GithubResult> = _github

    // Una sola tasca pendent: si el servidor respon 409, no fem perdre el text.
    private var queuedTask: String? = null
    private val _queued = MutableStateFlow(false)
    val queued: StateFlow<Boolean> = _queued

    // El backend pot emetre molts tokens/segon: els agrupem a ~20 fps per no
    // fer saltar el LazyColumn mentre es llegeix una resposta llarga.
    private val tokenLock = Any()
    private val tokenBuffer = StringBuilder()
    private var tokenFlush: Job? = null
    // El torn ha pintat text per tokens: l'event final el substitueix.
    private var streamedTurn = false

    fun go(s: Screen) { _screen.value = s }

    fun newChat() {
        _msgs.value = emptyList()
        _screen.value = Screen.Chat
    }

    fun disconnect() {
        _connected.value = false
        _screen.value = Screen.Pairing
    }

    fun setConn(c: Conn) { _conn.value = c }

    /**
     * Entra amb usuari i contrasenya (POST /api/login).
     *
     * Aixo es fa ABANS de tenir connexio: no hi ha token encara, i per tant
     * no es pot fer servir la connexio desada. El token que torna es desa i
     * a partir d'aqui tot va com sempre.
     *
     * Amb un servidor sense usuaris configurats aquest cami no serveix (no
     * hi ha comptes): es queda el camp de token de sempre.
     */
    fun entra(host: String, port: String, usuari: String, contrasenya: String) {
        viewModelScope.launch {
            try {
                val api = GregalApi(baseUrlOf(host, port), "")
                val tok = withContext(Dispatchers.IO) { api.login(usuari, contrasenya) }
                if (tok.isBlank()) {
                    notify(appText("app.status.loginNoToken"))
                    return@launch
                }
                _usuari.value = usuari.trim()
                prefsStore?.save(host, port, tok, usuari)
                setConn(Conn(host, port, tok, usuari.trim()))
                connect { go(Screen.Chat) }
            } catch (e: Exception) {
                val m = e.message ?: ""
                notify(if (m.contains("401")) appText("app.status.loginInvalid")
                    else appText("app.status.loginFailed", m.take(120)))
            }
        }
    }

    private fun appText(key: String, vararg args: Any?): String = tr(_idioma.value, key, *args)

    fun notify(t: String) { _notice.value = t }
    fun clearNotice() { _notice.value = "" }

    /** Prova la connexió i carrega l'estat. */
    fun connect(onOk: () -> Unit = {}) {
        viewModelScope.launch {
            try {
                val api = _conn.value.api()
                // Health és només descoberta: un servidor antic pot no tenir
                // l'endpoint i això no ha d'impedir connectar per la via v1.
                val health = withContext(Dispatchers.IO) {
                    try { api.health() } catch (_: Exception) { null }
                }
                _v2Runs.value = health?.supportsV2Interactive() == true
                val st = withContext(Dispatchers.IO) { api.getState() }
                _state.value = st
                _connected.value = true
                clearNotice()
                onOk()
            } catch (e: Exception) {
                _connected.value = false
                notify(connectionMessage(e))
            }
        }
    }

    fun refresh() {
        viewModelScope.launch {
            try {
                _state.value = withContext(Dispatchers.IO) { _conn.value.api().getState() }
            } catch (e: Exception) {
                notify(appText("app.status.refresh", e.message.orEmpty()))
            }
        }
    }

    private val _pendingImgs = MutableStateFlow<List<String>>(emptyList())
    val pendingImgs: StateFlow<List<String>> = _pendingImgs

    /** Afegeix una imatge (SAF Uri) com a data URL, màxim 3 i 8MB. */
    fun addImage(uri: Uri) {
        if (_pendingImgs.value.size >= 3) { notify(appText("app.status.imageLimit")); return }
        try {
            val cr = appContext.contentResolver
            val mime = cr.getType(uri) ?: ""
            if (!mime.startsWith("image/")) { notify(appText("app.status.imageType")); return }
            cr.openInputStream(uri)?.use { ins ->
                val raw = ins.readBytes()
                if (raw.size > 8 * 1024 * 1024) { notify(appText("app.status.imageLarge")); return }
                val b64 = Base64.encodeToString(raw, Base64.NO_WRAP)
                _pendingImgs.value = _pendingImgs.value + "data:$mime;base64,$b64"
            } ?: notify(appText("app.status.imageRead"))
        } catch (e: Exception) { notify(appText("app.status.imageError", e.message.orEmpty())) }
    }

    fun removeImage(i: Int) {
        _pendingImgs.value = _pendingImgs.value.filterIndexed { k, _ -> k != i }
    }

    /** Envia al xat (/api/chat) o a l'agent (/api/agent) segons el mode. */
    fun answerQuestion(key: String, answer: String) {
        val q = _question.value ?: return
        _question.value = null
        push(ChatMsg("assistant", appText("app.chat.questionAnswer", answer.ifBlank { appText("app.chat.questionEmpty") }), "activity"))
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { _conn.value.api().answerQuestion(key, answer) }
            } catch (e: Exception) {
                // Si el servidor ja l'ha donada per caducada (404) no és cap
                // error de l'usuari: l'agent continua sol.
                if (!(e.message ?: "").contains("404")) {
                    notify(appText("app.status.questionSend", e.message.orEmpty()))
                }
            }
        }
        if (q.callId.isEmpty()) _statusLine.value = ""
    }

    fun send(text: String, forceAgent: Boolean = false) {
        val t = text.trim()
        if (t.isEmpty() || _sending.value) return
        // La cua v2 ja serialitza per sessió. Amb la via antiga mantenim el
        // comportament anterior de guardar el text fins que quedi lliure.
        if (_state.value.agentBusy && !_v2Runs.value) {
            queueAfterBusy(t)
            return
        }
        _sending.value = true
        streamedTurn = false
        _turnStart.value = System.currentTimeMillis()
        _statusLine.value = ""
        push(ChatMsg("user", t))
        val agentMode = forceAgent || _state.value.mode == "code" || _state.value.mode == "inspect" || _state.value.mode == "goal"
        val path = if (agentMode) "/api/agent" else "/api/chat"
        val key = if (agentMode) "task" else "message"
        val imgs = if (agentMode) _pendingImgs.value.take(3) else emptyList()
        if (imgs.isNotEmpty()) push(ChatMsg("user", "🖼️×${imgs.size}"))
        _pendingImgs.value = emptyList()
        viewModelScope.launch {
            var answered = false
            try {
                GregalNotifications.workStarted(appContext)
                if (_v2Runs.value) {
                    val mode = if (agentMode) _state.value.mode else "chat"
                    answered = runV2Turn(t, mode, imgs)
                } else {
                    withContext(Dispatchers.IO) {
                        val payload = JSONObject().put(key, t)
                        if (imgs.isNotEmpty()) payload.put("images", JSONArray(imgs))
                        _conn.value.api().stream(path, payload) { ev ->
                            receiveSse(ev)
                        }
                    }
                    flushTokens()
                    answered = true
                }
            } catch (e: Exception) {
                if ((e.message ?: "").contains("409")) {
                    // Ja hi ha una bombolla d'usuari: guarda-la per reintentar,
                    // no la facis perdre ni hi afegeixis un error tècnic.
                    queueAfterBusy(t, alreadyShown = true)
                } else {
                    push(ChatMsg("assistant", requestMessage(e), "error"))
                }
            }
            GregalNotifications.workFinished(appContext, answered, t, lastAssistantText())
            _sending.value = false
            _turnStart.value = 0L
            _statusLine.value = ""
            refresh()
        }
    }

    /** Compara els models (operació explícita; no duplica el cost dels torns normals). */
    fun parallel(text: String) {
        val t = text.trim()
        if (t.isEmpty() || _sending.value) return
        if (_state.value.agentBusy) { notify(appText("app.chat.compareBusy")); return }
        _sending.value = true
        push(ChatMsg("user", "⇄ $t"))
        viewModelScope.launch {
            try {
                val rows = withContext(Dispatchers.IO) { _conn.value.api().parallel(t) }
                rows.forEach { r ->
                    val title = "◆ ${r.role} · ${r.provider}/${r.model}"
                    push(ChatMsg("assistant", if (r.error.isNotBlank()) "$title\n⚠️ ${r.error}" else "$title\n${r.reply}"))
                }
                if (rows.isEmpty()) notify(appText("app.chat.compareEmpty"))
            } catch (e: Exception) {
                push(ChatMsg("assistant", appText("app.status.compareError", requestMessage(e)), "error"))
            }
            _sending.value = false
            refresh()
        }
    }

    fun github(tool: String, action: String, number: Int = 0, query: String = "", repo: String = "") {
        viewModelScope.launch {
            _github.value = withContext(Dispatchers.IO) { _conn.value.api().github(tool, action, number, query, repo) }
        }
    }

    // --- office: puja, llegeix, edita, baixa (el fitxer viu al mòbil) ---

    private val _office = MutableStateFlow(OfficeDoc())
    val office: StateFlow<OfficeDoc> = _office

    fun officeUpload(name: String, dataB64: String) {
        _office.value = OfficeDoc(name = name, result = appText("app.off.uploading"))
        viewModelScope.launch {
            val up = withContext(Dispatchers.IO) { _conn.value.api().officeUpload(name, dataB64) }
            if (up.error.isNotBlank() || up.id.isBlank()) { _office.value = up; return@launch }
            _office.value = withContext(Dispatchers.IO) { _conn.value.api().officeRead(up.id) }
        }
    }

    fun officeEdit(op: String, sheet: String = "", cell: String = "",
                   value: String = "", find: String = "", replace: String = "") {
        val id = _office.value.id
        if (id.isBlank()) return
        _office.value = _office.value.copy(result = appText("app.off.applying"), error = "")
        viewModelScope.launch {
            val r = withContext(Dispatchers.IO) {
                _conn.value.api().officeEdit(id, op, sheet, cell, value, find, replace)
            }
            _office.value = _office.value.copy(
                text = r.text.ifBlank { _office.value.text },
                result = r.result, error = r.error)
        }
    }

    /** Baixa el fitxer editat (per desar-lo al mòbil des de Screens). */
    suspend fun officeDownloadBytes(): ByteArray? {
        val id = _office.value.id
        if (id.isBlank()) return null
        return try {
            withContext(Dispatchers.IO) { _conn.value.api().officeDownload(id) }
        } catch (_: Exception) { null }
    }

    fun officeClear() { _office.value = OfficeDoc() }

    // --- proveïdors: claus api ---

    private val _providers = MutableStateFlow(listOf<ProviderInfo>())
    val providers: StateFlow<List<ProviderInfo>> = _providers
    private val _provMsg = MutableStateFlow("")
    val provMsg: StateFlow<String> = _provMsg

    fun loadProviders() {
        viewModelScope.launch {
            _providers.value = try {
                withContext(Dispatchers.IO) { _conn.value.api().getProviders() }
            } catch (e: Exception) {
                _provMsg.value = appText("app.status.providers", e.message.orEmpty())
                _providers.value
            }
        }
    }

    fun setProviderKey(name: String, key: String) {
        _provMsg.value = appText("app.status.providerSaving")
        viewModelScope.launch {
            _provMsg.value = try {
                withContext(Dispatchers.IO) { _conn.value.api().setProviderKey(name, key) }
                appText("app.menu.keySaved")
            } catch (e: Exception) { appText("app.menu.providerSaveError", e.message.orEmpty()) }
            loadProviders()
        }
    }

    fun testProvider(name: String) {
        _provMsg.value = appText("app.status.providerTesting")
        viewModelScope.launch {
            _provMsg.value = try {
                withContext(Dispatchers.IO) { _conn.value.api().testProvider(name) }
                appText("app.menu.providerTested")
            } catch (e: Exception) { appText("app.menu.providerTestError", e.message.orEmpty()) }
        }
    }

    /** Rep SSE des del fil de xarxa; només muta la UI al main thread. */
    private fun receiveSse(ev: SseEvent) {
        if (ev.name == "token") {
            val text = try { JSONObject(ev.data).optString("text") } catch (_: Exception) { "" }
            if (text.isEmpty()) return
            synchronized(tokenLock) {
                tokenBuffer.append(text)
                if (tokenFlush?.isActive != true) {
                    tokenFlush = viewModelScope.launch {
                        delay(50)
                        flushTokens()
                    }
                }
            }
            return
        }
        viewModelScope.launch {
            flushTokens()
            handleSse(ev)
        }
    }

    private fun flushTokens() {
        val text = synchronized(tokenLock) {
            val out = tokenBuffer.toString()
            tokenBuffer.clear()
            out
        }
        if (text.isNotEmpty()) appendAssistant(text)
    }

    private fun handleSse(ev: SseEvent) {
        val o = try { JSONObject(ev.data) } catch (_: Exception) { JSONObject() }
        when (ev.name) {
            "token" -> appendAssistant(o.optString("text"))
            // El servidor emet el text per tokens I el final sencer: si el
            // torn ja ha streamejat, el final substitueix (no duplica).
            "assistant" -> {
                val text = o.optString("text")
                if (text.isNotBlank()) {
                    val cur = _msgs.value
                    val last = cur.lastOrNull()
                    if (streamedTurn && last != null && last.role == "assistant" && last.kind == "text") {
                        _msgs.value = cur.dropLast(1) + last.copy(text = text)
                    } else {
                        push(ChatMsg("assistant", text))
                    }
                }
                streamedTurn = false
            }
            // El raonament d'un pas amb eines ja ha sortit per tokens: la
            // bombolla d'activitat el representa, no cal el text duplicat.
            "thinking", "thought", "reasoning" -> {
                dropStreamedDraft()
                push(ChatMsg("assistant",
                    "💭 ${o.optString("text").ifEmpty { o.optString("content") }}", "activity"))
            }
            "tool_call" -> {
                dropStreamedDraft()
                push(ChatMsg("assistant",
                    "🔧 ${o.optString("name")} ${o.optString("args").take(120)}", "activity"))
            }
            "tool_result" -> push(ChatMsg("assistant",
                "↳ ${o.optString("name")}: ${o.optString("output").take(300)}", "activity"))
            "blocked" -> push(ChatMsg("assistant",
                "⛔️ ${o.optString("reason")}", "activity"))
            "approve_request" -> _approve.value = ApproveReq(
                o.optString("key"), o.optString("call_id"),
                o.optString("name"), o.optString("args"))
            "verify" -> push(ChatMsg("assistant",
                "✅ ${o.optString("verdict")}\n${o.optString("detail").take(500)}", "activity"))
            "goal" -> push(ChatMsg("assistant",
                "◆ ${o.optString("title")} (${o.optString("id")})", "goal"))
            "fallback" -> push(ChatMsg("assistant",
                appText("app.chat.fallback", o.optString("model")), "activity"))
            "error" -> push(ChatMsg("assistant", "⚠️ ${o.optString("message")}", "error"))
            "done" -> { streamedTurn = false }
            // --- Esdeveniments que el mòbil ignorava i que la web ja pinta.
            // Sense això, una feina aturada (passos esgotats, compactació,
            // pressupost) semblava una feina acabada.
            "status" -> _statusLine.value = o.optString("message")
            "compact" -> {
                _statusLine.value = appText("app.chat.compactedStatus", o.optInt("abans"), o.optInt("despres"))
                push(ChatMsg("assistant", appText("app.chat.compacted"), "activity"))
            }
            "budget" -> {
                _statusLine.value = o.optString("message")
                push(ChatMsg("assistant", "⚠️ ${o.optString("message")}", "error"))
            }
            "verify_start" -> _statusLine.value = appText("app.chat.verifyStart", o.optString("cmd"))
            "steps_exhausted" -> {
                _statusLine.value = ""
                push(ChatMsg("assistant",
                    appText("app.chat.stepsExhausted", o.optInt("max")), "error"))
            }
            "tool_text" -> push(ChatMsg("assistant",
                o.optString("message").ifBlank { appText("app.chat.toolTextFallback") },
                "activity"))
            "escalate" -> push(ChatMsg("assistant",
                "◆ ${o.optString("message").ifBlank { o.optString("reason") }}", "activity"))
            "gated" -> push(ChatMsg("assistant",
                "🔒 ${o.optString("message").ifBlank { o.optString("reason") }}", "activity"))
            "question_request" -> _question.value = QuestionReq(
                o.optString("key"), o.optString("call_id"),
                o.optString("query"), parseQuestionOptions(o.optJSONArray("options")),
            )
            "question_timeout" -> {
                _question.value = null
                push(ChatMsg("assistant",
                    appText("app.chat.questionTimeout"), "activity"))
            }
            "approve_timeout" -> {
                _approve.value = null
                push(ChatMsg("assistant",
                    appText("app.chat.approvalTimeoutDenied"), "activity"))
            }
        }
    }

    private fun appendAssistant(tok: String) {
        if (tok.isEmpty()) return
        streamedTurn = true
        val cur = _msgs.value
        val last = cur.lastOrNull()
        _msgs.value = if (last != null && last.role == "assistant" && last.kind == "text") {
            cur.dropLast(1) + last.copy(text = last.text + tok)
        } else {
            cur + ChatMsg("assistant", tok)
        }
    }

    /** Treu l'esborrany streamejat del torn (el representa l'activitat). */
    private fun dropStreamedDraft() {
        if (!streamedTurn) return
        val cur = _msgs.value
        val last = cur.lastOrNull()
        if (last != null && last.role == "assistant" && last.kind == "text") {
            _msgs.value = cur.dropLast(1)
        }
        streamedTurn = false
    }

    private fun push(m: ChatMsg) {
        val stamped = if (m.at == 0L) m.copy(at = System.currentTimeMillis()) else m
        _msgs.value = _msgs.value + stamped
    }

    /** Guarda un sol missatge i el reintenta quan l'agent acaba. */
    private fun queueAfterBusy(text: String, alreadyShown: Boolean = false) {
        queuedTask = text
        _queued.value = true
        if (!alreadyShown) push(ChatMsg("user", text))
        push(ChatMsg("assistant", appText("app.chat.queuedNotice"), "activity"))
        waitUntilFree()
    }

    /** Espera que l'agent quedi lliure (sondeja l'estat cada 3 s). */
    fun waitUntilFree() {
        viewModelScope.launch {
            repeat(40) {
                delay(3000)
                try {
                    _state.value = withContext(Dispatchers.IO) { _conn.value.api().getState() }
                } catch (_: Exception) { }
                if (!_state.value.agentBusy) {
                    val pending = queuedTask
                    queuedTask = null
                    _queued.value = false
                    if (pending != null) {
                        push(ChatMsg("assistant", appText("app.chat.continueQueued"), "activity"))
                        // Evita una segona bombolla: l'original ja és al xat.
                        sendQueued(pending)
                    } else {
                        push(ChatMsg("assistant", appText("app.chat.free"), "activity"))
                    }
                    return@launch
                }
            }
            notify(appText("app.chat.busyTimeout"))
        }
    }

    private fun sendQueued(text: String) {
        if (_sending.value) return
        _sending.value = true
        val agentMode = _state.value.mode == "code" || _state.value.mode == "inspect" || _state.value.mode == "goal"
        val path = if (agentMode) "/api/agent" else "/api/chat"
        val key = if (agentMode) "task" else "message"
        viewModelScope.launch {
            var answered = false
            try {
                GregalNotifications.workStarted(appContext)
                if (_v2Runs.value) {
                    answered = runV2Turn(text, _state.value.mode, emptyList())
                } else {
                    withContext(Dispatchers.IO) {
                        val payload = JSONObject().put(key, text)
                        _conn.value.api().stream(path, payload) { receiveSse(it) }
                    }
                    flushTokens()
                    answered = true
                }
            } catch (e: Exception) {
                push(ChatMsg("assistant", requestMessage(e), "error"))
            }
            GregalNotifications.workFinished(appContext, answered, text, lastAssistantText())
            _sending.value = false
            refresh()
        }
    }

    /**
     * Submit + consum d'events durables. El cursor es conserva entre torns i
     * reconnects dins del ViewModel; així una desconnexió no duplica text.
     */
    private suspend fun runV2Turn(text: String, mode: String, images: List<String>): Boolean {
        val api = _conn.value.api()
        val run = withContext(Dispatchers.IO) {
            api.submitRun(text, mode, images, UUID.randomUUID().toString())
        }
        if (run.id <= 0L) throw ApiException("El servidor no ha tornat cap execució")
        activeRunId = run.id
        var current = run
        try {
            while (current.state !in setOf("completed", "failed", "cancelled")) {
                val page = withContext(Dispatchers.IO) {
                    api.getEvents(eventCursor, 100, sessioApp)
                }
                if (page.next > eventCursor) eventCursor = page.next
                page.events.filter { it.runId == run.id }.forEach { receiveDurable(it) }
                current = withContext(Dispatchers.IO) { api.getRun(run.id) }
                if (current.state !in setOf("completed", "failed", "cancelled")) delay(500)
            }
            // Consumeix l'última tanda escrita abans que l'estat es tanqui.
            val page = withContext(Dispatchers.IO) { api.getEvents(eventCursor, 100, sessioApp) }
            if (page.next > eventCursor) eventCursor = page.next
            page.events.filter { it.runId == run.id }.forEach { receiveDurable(it) }
            if (current.state == "failed") {
                throw ApiException(current.error.ifBlank { "El torn ha fallat" })
            }
            if (current.state == "cancelled") {
                push(ChatMsg("assistant", appText("app.chat.cancelled"), "activity"))
                return false
            }
            flushTokens()
            return true
        } finally {
            activeRunId = null
        }
    }

    /** Traducció del registre durable al mateix renderer de la via SSE. */
    private fun receiveDurable(ev: DurableEvent) {
        viewModelScope.launch {
            when (ev.kind) {
                "text", "assistant" -> if (ev.text.isNotBlank()) push(ChatMsg("assistant", ev.text))
                "done" -> _statusLine.value = ""
                "approve_request" -> {
                    val o = durablePayload(ev)
                    if (o != null) {
                        _approve.value = ApproveReq(
                            o.optString("key"), o.optString("call_id"),
                            o.optString("name"), o.optString("args"),
                        )
                    } else {
                        push(ChatMsg("assistant", appText("app.chat.approvalNoPayload"), "error"))
                    }
                }
                "question_request" -> {
                    val o = durablePayload(ev)
                    if (o != null) {
                        _question.value = QuestionReq(
                            o.optString("key"), o.optString("call_id"),
                            o.optString("query"), parseQuestionOptions(o.optJSONArray("options")),
                        )
                    } else {
                        push(ChatMsg("assistant", appText("app.chat.questionNoPayload"), "error"))
                    }
                }
                "question_timeout" -> {
                    _question.value = null
                    push(ChatMsg("assistant", appText("app.chat.questionTimeout"), "activity"))
                }
                "approve_timeout" -> {
                    _approve.value = null
                    push(ChatMsg("assistant", appText("app.chat.approvalTimeout"), "activity"))
                }
                "error", "run_failed" -> push(ChatMsg("assistant", "⚠️ ${ev.text}", "error"))
                "run_queued" -> _statusLine.value = appText("app.chat.runQueued")
                "run_started" -> _statusLine.value = appText("app.chat.runStarted")
                "run_completed" -> _statusLine.value = ""
                "run_cancelled" -> push(ChatMsg("assistant", "⛔️ ${ev.text}", "activity"))
                else -> if (ev.text.isNotBlank()) push(ChatMsg("assistant", ev.text, "activity"))
            }
        }
    }

    /** Payload estructurat d'un event interactiu, amb tolerància a wrappers. */
    private fun durablePayload(ev: DurableEvent): JSONObject? {
        if (ev.payload.isBlank()) return null
        var o = try { JSONObject(ev.payload) } catch (_: Exception) { return null }
        repeat(3) {
            val nested = listOf("payload", "interaction", "data")
                .firstNotNullOfOrNull { key -> o.optJSONObject(key) }
            if (nested == null) return@repeat
            o = nested
        }
        return o.takeIf { it.optString("key").isNotBlank() }
    }

    /** Atura el torn actual; usa l'ID v2 quan està disponible. */
    fun cancelCurrent() {
        viewModelScope.launch {
            try {
                val id = activeRunId
                withContext(Dispatchers.IO) {
                    if (_v2Runs.value && id != null) _conn.value.api().cancelRun(id)
                    else _conn.value.api().cancelLegacy()
                }
            } catch (e: Exception) { notify(appText("app.status.cancel", e.message.orEmpty())) }
        }
    }

    /** Darrera resposta d'assistent en text (per a l'extracte de l'avís). */
    private fun lastAssistantText(): String =
        _msgs.value.lastOrNull { it.role == "assistant" && it.kind == "text" }?.text.orEmpty()

    private fun connectionMessage(e: Exception): String = when {
        (e.message ?: "").contains("401") -> appText("app.status.connectionBadToken")
        (e.message ?: "").contains("failed to connect", true) ||
            (e.message ?: "").contains("timed out", true) ->
            appText("app.status.connectionOffline")
        else -> appText("app.status.connectionFailed")
    }

    private fun requestMessage(e: Exception): String = when {
        (e.message ?: "").contains("401") -> appText("app.status.requestBadToken")
        (e.message ?: "").contains("failed to connect", true) ->
            appText("app.status.requestConnection")
        else -> appText("app.status.requestFailed")
    }

    fun answerApprove(ok: Boolean, remember: Boolean = false) {
        val a = _approve.value ?: return
        _approve.value = null
        push(ChatMsg("assistant",
            when {
                ok && remember -> appText("app.chat.approvalAllowedAlways", a.name)
                ok -> appText("app.chat.approvalAllowed", a.name)
                else -> appText("app.chat.approvalDenied", a.name)
            }, "activity"))
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) {
                    _conn.value.api().post("/api/approve",
                        JSONObject().put("key", a.key).put("approve", ok).put("remember", remember))
                }
            } catch (e: Exception) {
                push(ChatMsg("assistant", appText("app.chat.approvalError", e.message.orEmpty()), "error"))
            }
        }
    }

    // --- sessions ---

    fun loadSessions() {
        viewModelScope.launch {
            try {
                val api = _conn.value.api()
                val (sessions, active) = withContext(Dispatchers.IO) {
                    api.getSessions() to api.getActive()
                }
                _sessions.value = sessions
                _active.value = active
            } catch (e: Exception) { notify(appText("app.status.sessionsLoad", e.message.orEmpty())) }
        }
    }

    /** Actualitza només la sessió que ocupa el worker, sense tocar el xat local. */
    fun loadActive() {
        viewModelScope.launch {
            try {
                _active.value = withContext(Dispatchers.IO) { _conn.value.api().getActive() }
            } catch (_: Exception) { /* una actualització visual no ha d'omplir el xat d'errors */ }
        }
    }

    fun resumeSession(name: String) {
        viewModelScope.launch {
            try {
                val o = withContext(Dispatchers.IO) {
                    _conn.value.api().post("/api/resume", JSONObject().put("name", name))
                }
                val tr = o.optJSONArray("transcript")
                val serverMsgs = o.optInt("msgs", -1)
                val list = mutableListOf<ChatMsg>()
                if (tr != null) for (i in 0 until tr.length()) {
                    val m = tr.getJSONObject(i)
                    val role = m.optString("role")
                    val text = transcriptText(m.toString())
                    if ((role == "user" || role == "assistant") && text.isNotBlank()) {
                        list += ChatMsg(if (role == "user") "user" else "assistant", text)
                    }
                }
                _msgs.value = list
                if (list.isEmpty() && serverMsgs > 0) {
                    notify(appText("app.status.sessionHidden", serverMsgs))
                }
                refresh()
                go(Screen.Chat)
            } catch (e: Exception) { notify(appText("app.status.sessionResume", e.message.orEmpty())) }
        }
    }

    fun newSession() {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { _conn.value.api().post("/api/new", JSONObject()) }
                _msgs.value = emptyList()
                refresh()
                go(Screen.Chat)
            } catch (e: Exception) { notify(appText("app.status.sessionNew", e.message.orEmpty())) }
        }
    }

    // --- rewind: desfés canvis de l'agent (paritat amb el web) ---

    private val _checkpoints = MutableStateFlow<List<CheckpointInfo>>(emptyList())
    val checkpoints: StateFlow<List<CheckpointInfo>> = _checkpoints
    private val _showRewind = MutableStateFlow(false)
    val showRewind: StateFlow<Boolean> = _showRewind
    // Diàleg de GitHub: viu aquí perquè s'obre des del calaix (la barra de
    // dalt ja no té icones d'eines).
    private val _ghObert = MutableStateFlow(false)
    val ghObert: StateFlow<Boolean> = _ghObert

    fun obreGithub() { _ghObert.value = true }
    fun tancaGithub() { _ghObert.value = false }

    fun openRewind() {
        viewModelScope.launch {
            try {
                _checkpoints.value = withContext(Dispatchers.IO) { _conn.value.api().getCheckpoints() }
                _showRewind.value = true
            } catch (e: Exception) { notify(appText("app.status.checkpoints", e.message.orEmpty())) }
        }
    }

    fun closeRewind() { _showRewind.value = false }

    fun rewindAll() {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { _conn.value.api().postRewind() }
                _showRewind.value = false
                notify(appText("app.status.rewindAllDone"))
                refresh()
            } catch (e: Exception) { notify(appText("app.status.rewind", e.message.orEmpty())) }
        }
    }

    fun rewindTo(seq: Int) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { _conn.value.api().postRewindTo(seq) }
                _showRewind.value = false
                notify(appText("app.status.rewindToDone", seq))
                refresh()
            } catch (e: Exception) { notify(appText("app.status.rewind", e.message.orEmpty())) }
        }
    }

    // --- controls: rol, mode, model ---

    fun setRole(role: String) = simplePost("/api/role", JSONObject().put("role", role))
    fun setMode(mode: String) = simplePost("/api/mode", JSONObject().put("mode", mode))
    /** Revisor automàtic on/off (la web: auto/manual; aquí interruptor). */
    fun setVerify(auto: Boolean) = simplePost("/api/verify", JSONObject().put("mode", if (auto) "auto" else "manual"))

    /** Esborra una conversa i refresca la llista. */
    fun deleteSession(name: String) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { _conn.value.api().deleteSession(name) }
                loadSessions()
            } catch (e: Exception) {
                notify(appText("app.status.sessionDelete", e.message.orEmpty()))
            }
        }
    }

    // ---- Grafs: llista, detall, execució SSE i esborrat. ----

    private val _flows = MutableStateFlow<List<FlowSummary>>(emptyList())
    val flows: StateFlow<List<FlowSummary>> = _flows
    private val _flow = MutableStateFlow<FlowDetail?>(null)
    val flow: StateFlow<FlowDetail?> = _flow
    private val _flowLog = MutableStateFlow<List<FlowStep>>(emptyList())
    val flowLog: StateFlow<List<FlowStep>> = _flowLog
    private val _flowRunning = MutableStateFlow(false)
    val flowRunning: StateFlow<Boolean> = _flowRunning
    private val _flowAnswer = MutableStateFlow("")
    val flowAnswer: StateFlow<String> = _flowAnswer

    fun loadFlows() {
        viewModelScope.launch {
            try {
                _flows.value = withContext(Dispatchers.IO) { _conn.value.api().getFlows() }
            } catch (e: Exception) {
                notify(appText("app.status.flows", e.message.orEmpty()))
            }
        }
    }

    fun openFlow(name: String) {
        _flowLog.value = emptyList()
        _flowAnswer.value = ""
        viewModelScope.launch {
            try {
                _flow.value = withContext(Dispatchers.IO) { _conn.value.api().getFlow(name) }
            } catch (e: Exception) {
                notify(appText("app.status.flowOpen", e.message.orEmpty()))
            }
        }
    }

    fun closeFlow() {
        _flow.value = null
        _flowLog.value = emptyList()
        _flowAnswer.value = ""
    }

    /** Executa el graf (GET SSE: start/step/done). El resultat queda al xat. */
    fun runFlow(name: String, input: String) {
        if (_flowRunning.value) return
        _flowRunning.value = true
        _flowLog.value = emptyList()
        _flowAnswer.value = ""
        viewModelScope.launch {
            try {
                val q = "name=" + java.net.URLEncoder.encode(name, "UTF-8") + "&auto=1" +
                    (if (input.isBlank()) "" else "&input=" + java.net.URLEncoder.encode(input, "UTF-8"))
                withContext(Dispatchers.IO) {
                    _conn.value.api().streamGet("/api/flows/run?$q") { ev ->
                        try {
                            val o = JSONObject(ev.data)
                            when (ev.name) {
                                "step" -> _flowLog.value = _flowLog.value + FlowStep(
                                    o.optString("node"), o.optString("title"),
                                    o.optString("error"), o.optLong("ms"))
                                "done" -> {
                                    _flowAnswer.value = o.optString("answer")
                                    if (o.optString("error").isNotBlank()) {
                                        _flowLog.value = _flowLog.value + FlowStep(
                                            "", "", o.optString("error"), 0)
                                    }
                                }
                            }
                        } catch (_: Exception) {
                        }
                    }
                }
            } catch (e: Exception) {
                notify(appText("app.status.flowRun", e.message.orEmpty()))
            } finally {
                _flowRunning.value = false
            }
        }
    }

    fun deleteFlow(name: String) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { _conn.value.api().deleteFlow(name) }
                closeFlow()
                loadFlows()
            } catch (e: Exception) {
                notify(appText("app.status.flowDelete", e.message.orEmpty()))
            }
        }
    }
    /** Canvia només a un model anunciat pel servidor i confirma l'estat final. */
    fun setModel(model: String) {
        viewModelScope.launch {
            try {
                val selected = withContext(Dispatchers.IO) {
                    _conn.value.api().post("/api/model", JSONObject().put("model", model))
                }
                val expectedProvider = selected.optString("provider")
                val expectedModel = selected.optString("model")
                val after = withContext(Dispatchers.IO) { _conn.value.api().getState() }
                if (after.provider != expectedProvider || after.model != expectedModel) {
                    throw ApiException(appText("app.status.modelNotConfirmed"))
                }
                _state.value = after
                notify(appText("app.status.modelSelected", after.model))
            } catch (e: Exception) {
                notify(appText("app.status.modelChange", e.message.orEmpty()))
            }
        }
    }

    private fun simplePost(path: String, payload: JSONObject) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { _conn.value.api().post(path, payload) }
                refresh()
            } catch (e: Exception) { notify(appText("app.status.error", e.message.orEmpty())) }
        }
    }

    fun loadModels() {
        viewModelScope.launch {
            try {
                val (m, _) = withContext(Dispatchers.IO) { _conn.value.api().getModels() }
                _models.value = m
            } catch (e: Exception) { notify(appText("app.status.models", e.message.orEmpty())) }
        }
    }

    // --- objectius ---

    private val _plan = MutableStateFlow<String?>(null)
    val plan: StateFlow<String?> = _plan
    private val _planning = MutableStateFlow(false)
    val planning: StateFlow<Boolean> = _planning

    /** Demana un pla read-only (/api/plan): explora sense actuar. */
    fun requestPlan(task: String) {
        val t = task.trim()
        if (t.isEmpty() || _planning.value || _sending.value) return
        if (_state.value.agentBusy) {
            notify(appText("app.status.planBusy"))
            return
        }
        _planning.value = true
        viewModelScope.launch {
            try {
                val p = withContext(Dispatchers.IO) {
                    _conn.value.api().postPlan(t)
                }
                if (p.isBlank()) notify(appText("app.status.planEmpty"))
                else _plan.value = p
            } catch (e: Exception) { notify(appText("app.status.plan", e.message.orEmpty())) }
            _planning.value = false
            refresh()
        }
    }

    fun dismissPlan() { _plan.value = null }

    /** Executa el pla: mode codi + la tasca com a torn d'agent. */
    fun runPlan() {
        val p = _plan.value ?: return
        _plan.value = null
        setMode("code")
        send(p, forceAgent = true)
    }

    fun loadGoals() {
        viewModelScope.launch {
            try {
                _goals.value = withContext(Dispatchers.IO) { _conn.value.api().getGoals() }
            } catch (e: Exception) { notify(appText("app.status.goals", e.message.orEmpty())) }
        }
    }

    fun runGoal(id: String) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) {
                    _conn.value.api().post("/api/goal",
                        JSONObject().put("action", "task").put("id", id))
                }
                notify(appText("app.status.goalStarted"))
                loadGoals()
            } catch (e: Exception) { notify(appText("app.status.goal", e.message.orEmpty())) }
        }
    }

    fun deleteGoal(id: String) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) {
                    _conn.value.api().post("/api/goal",
                        JSONObject().put("action", "delete").put("id", id))
                }
                loadGoals()
            } catch (e: Exception) { notify(appText("app.status.goal", e.message.orEmpty())) }
        }
    }

    // --- jobs programats ---

    fun loadJobs() {
        viewModelScope.launch {
            try {
                _jobs.value = withContext(Dispatchers.IO) { _conn.value.api().getJobs() }
            } catch (e: Exception) { notify(appText("app.status.jobs", e.message.orEmpty())) }
        }
    }

    fun saveJob(j: JobItem) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { _conn.value.api().saveJob(j) }
                loadJobs()
            } catch (e: Exception) { notify(appText("app.status.job", e.message.orEmpty())) }
        }
    }

    fun toggleJob(j: JobItem) = saveJob(j.copy(enabled = !j.enabled))

    fun deleteJob(id: String) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { _conn.value.api().deleteJob(id) }
                loadJobs()
            } catch (e: Exception) { notify(appText("app.status.job", e.message.orEmpty())) }
        }
    }

    fun runJobNow(id: String) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { _conn.value.api().runJob(id) }
                notify(appText("app.status.jobStarted"))
            } catch (e: Exception) { notify(appText("app.status.job", e.message.orEmpty())) }
        }
    }

    fun loadRuns(jobId: String = "") {
        viewModelScope.launch {
            try {
                _runs.value = withContext(Dispatchers.IO) { _conn.value.api().getRuns(jobId) }
            } catch (e: Exception) { notify(appText("app.status.runs", e.message.orEmpty())) }
        }
    }
}
