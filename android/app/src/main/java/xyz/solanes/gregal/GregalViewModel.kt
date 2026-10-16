package xyz.solanes.gregal

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.coroutineScope
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
enum class Screen { Pairing, Chat, Sessions, Goals, Jobs, Office, Flows, Prefs, Retro }

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

    // Càrregues de llistes en curs ("sessions", "goals", "jobs", "runs",
    // "flows"): les pantalles no ensenyen el tauler buit mentre arriben.
    private val _loading = MutableStateFlow<Set<String>>(emptySet())
    val loading: StateFlow<Set<String>> = _loading
    private inline fun <T> loadingOf(key: String, block: () -> T): T {
        _loading.value = _loading.value + key
        try { return block() } finally { _loading.value = _loading.value - key }
    }

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

    /**
     * L'arrencada (reconnexió automàtica + comprovar actualitzacions) ja s'ha
     * fet amb aquest ViewModel: girar la pantalla recrea l'activitat però no
     * l'ha de repetir (ni treure't al Pairing si la reconnexió fallés).
     */
    var startupDone = false

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

    private val _queued = MutableStateFlow(false)
    val queued: StateFlow<Boolean> = _queued

    // El backend pot emetre molts tokens/segon: els agrupem a ~20 fps per no
    // fer saltar el LazyColumn mentre es llegeix una resposta llarga.
    private val tokenLock = Any()
    private val tokenBuffer = StringBuilder()
    private var tokenFlush: Job? = null

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

    /**
     * Si en obrir l'app aquesta sessió ja té un torn corrent (l'havies
     * començat abans o des de l'escriptori), el torna a mostrar en viu.
     */
    private fun attachToRunning() {
        if (!_v2Runs.value || _sending.value) return
        viewModelScope.launch {
            val api = _conn.value.api()
            val act = try { withContext(Dispatchers.IO) { api.getActive() } } catch (_: Exception) { null } ?: return@launch
            if (_sending.value || act.id <= 0) return@launch
            val lastUser = _msgs.value.lastOrNull { it.role == "user" }?.text?.trim()
            if (act.task.isNotBlank() && lastUser != act.task.trim()) push(ChatMsg("user", act.task))
            val started = try { java.time.Instant.parse(act.startedAt).toEpochMilli() } catch (_: Exception) { System.currentTimeMillis() }
            _sending.value = true
            _turnStart.value = started
            turnErrorShown = false
            answerId = 0L
            beginTurn()
            updateWork { it.copy(started = started) }
            var ok = false
            try {
                // Posa't al dia: només els events posteriors a l'últim
                // run_queued d'aquest id (registres antics reutilitzaven ids).
                val id = act.id.toLong()
                val mine = mutableListOf<DurableEvent>()
                var cur = 0L
                while (true) {
                    val page = withContext(Dispatchers.IO) { api.getEvents(cur, 500, sessioApp) }
                    for (e in page.events) if (e.runId == id) {
                        if (e.kind == "run_queued") mine.clear()
                        mine += e
                    }
                    if (page.next <= cur || page.events.size < 500) { cur = maxOf(cur, page.next); break }
                    cur = page.next
                }
                if (cur > eventCursor) eventCursor = cur
                mine.filter { it.kind !in setOf("run_completed", "run_failed", "run_cancelled") }.forEach { receiveDurable(it) }
                ok = followRun(api, id)
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                showError(requestMessage(e))
            }
            endTurn(!ok)
            _sending.value = false
            _turnStart.value = 0L
            _statusLine.value = ""
            refresh()
        }
    }

    /** Prova la connexió i carrega l'estat; `retries` per a l'arrencada (servidor que encara s'engega). */
    fun connect(retries: Int = 0, onOk: () -> Unit = {}) {
        viewModelScope.launch {
            var left = retries
            while (true) {
                try {
                    connectOnce()
                    onOk()
                    return@launch
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    val offline = (e.message ?: "").let { it.contains("failed to connect", true) || it.contains("timed out", true) || it.contains("refused", true) }
                    if (left-- > 0 && offline) {
                        notify(appText("app.work.reconnecting"))
                        delay(2500)
                        continue
                    }
                    _connected.value = false
                    notify(connectionMessage(e))
                    return@launch
                }
            }
        }
    }

    private suspend fun connectOnce() {
        val api = _conn.value.api()
        // Health és només descoberta: un servidor antic pot no tenir
        // l'endpoint i això no ha d'impedir connectar per la via v1.
        val health = withContext(Dispatchers.IO) {
            try { api.health() } catch (_: Exception) { null }
        }
        _v2Runs.value = health?.supportsV2Interactive() == true
        val st = withContext(Dispatchers.IO) { api.getState() }
        _state.value = st
        // Obrir l'app ha de mostrar la conversa on eres, no un xat buit.
        if (_msgs.value.isEmpty() && st.transcript.isNotEmpty()) _msgs.value = st.transcript
        _connected.value = true
        clearNotice()
        attachToRunning()
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
        val mime = try { appContext.contentResolver.getType(uri) ?: "" } catch (_: Exception) { "" }
        if (!mime.startsWith("image/")) { notify(appText("app.status.imageType")); return }
        // Mida primer, lectura i base64 a IO: una foto gran no ha de penjar la UI.
        viewModelScope.launch {
            when (val r = readUriBase64(appContext, uri, 8L * 1024 * 1024)) {
                is UriRead.Ok ->
                    if (_pendingImgs.value.size >= 3) notify(appText("app.status.imageLimit"))
                    else _pendingImgs.value = _pendingImgs.value + "data:$mime;base64,${r.base64}"
                is UriRead.TooLarge -> notify(appText("app.status.imageLarge"))
                UriRead.Unreadable -> notify(appText("app.status.imageRead"))
                is UriRead.Failed -> notify(appText("app.status.imageError", r.message))
            }
        }
    }

    fun removeImage(i: Int) {
        _pendingImgs.value = _pendingImgs.value.filterIndexed { k, _ -> k != i }
    }

    /** Envia al xat (/api/chat) o a l'agent (/api/agent) segons el mode. */
    fun answerQuestion(key: String, answer: String) {
        val q = _question.value ?: return
        _question.value = null
        val label = appText("app.chat.questionAnswer", answer.ifBlank { appText("app.chat.questionEmpty") })
        if (workId != 0L) step(StepKind.Info, label) else push(ChatMsg("assistant", label, "activity"))
        setActivity("model", appText("app.work.thinking"))
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
        val imgs = _pendingImgs.value.take(3)
        _pendingImgs.value = emptyList()
        push(ChatMsg("user", t))
        if (imgs.isNotEmpty()) push(ChatMsg("user", "🖼️×${imgs.size}"))
        runTurn(t, imgs, forceAgent)
    }

    // --- #retro ---

    private val _retro = MutableStateFlow<RetroChannel?>(null)
    val retro: StateFlow<RetroChannel?> = _retro
    /** "" en repòs; "loading" | "starting" | "replying"; o un avís per mostrar. */
    private val _retroState = MutableStateFlow("")
    val retroState: StateFlow<String> = _retroState
    private val _retroEmojis = MutableStateFlow(RETRO_DEFAULT_EMOJIS)
    val retroEmojis: StateFlow<List<String>> = _retroEmojis
    private var retroKey = ""

    /** Obre el #retro d'un torn (0 = l'últim acabat) i, si encara no té xat, l'engega. */
    fun openRetro(runId: Long = 0L) {
        _screen.value = Screen.Retro
        if (_retroState.value in setOf("loading", "starting", "replying")) return
        _retroState.value = "loading"
        viewModelScope.launch {
            try {
                val api = _conn.value.api()
                val info = withContext(Dispatchers.IO) { api.getRetro(runId = runId) }
                _retroEmojis.value = info.emojis
                retroKey = info.key
                if (info.channel != null && info.channel.messages.isNotEmpty()) {
                    _retro.value = info.channel
                    _retroState.value = ""
                    return@launch
                }
                val run = if (runId > 0) runId else info.latestRun
                when {
                    run <= 0L && info.key.isBlank() -> { _retro.value = null; _retroState.value = appText("app.retro.none") }
                    run == info.latestRun && info.latestRunning -> { _retro.value = null; _retroState.value = appText("app.retro.running") }
                    else -> {
                        _retroState.value = "starting"
                        val ch = withContext(Dispatchers.IO) { api.startRetro(info.key, run) }
                        _retro.value = ch
                        if (ch != null) retroKey = ch.key
                        _retroState.value = ""
                    }
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _retroState.value = requestMessage(e)
            }
        }
    }

    /** Pregunta o rebat als agents; @nom adreça el missatge a un de concret. */
    fun sendRetro(text: String, thread: String = "") {
        val t = text.trim()
        val ch = _retro.value ?: return
        if (t.isEmpty() || _retroState.value == "replying") return
        // Es veu a l'instant; la resposta del servidor porta el canal sencer.
        _retro.value = ch.copy(messages = ch.messages + RetroMsg("local-${nextMsgId()}", thread, "user", t))
        _retroState.value = "replying"
        viewModelScope.launch {
            try {
                val out = withContext(Dispatchers.IO) { _conn.value.api().retroMessage(ch.key.ifBlank { retroKey }, t, thread) }
                if (out != null) _retro.value = out
                _retroState.value = ""
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _retroState.value = requestMessage(e)
            }
        }
    }

    /** «Arregla-ho»: l'arranjament d'una troballa passa a ser un torn. */
    fun fixRetro(messageId: String) {
        val ch = _retro.value ?: return
        viewModelScope.launch {
            try {
                val out = withContext(Dispatchers.IO) { _conn.value.api().retroFix(ch.key.ifBlank { retroKey }, messageId) }
                if (out != null) _retro.value = out
                val run = out?.messages?.firstOrNull { it.id == messageId }?.fixRun ?: 0L
                notify(appText("app.retro.fixStarted").replace("{n}", run.toString()))
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                notify(requestMessage(e))
            }
        }
    }

    fun reactRetro(messageId: String, emoji: String) {
        val ch = _retro.value ?: return
        val m = ch.messages.firstOrNull { it.id == messageId } ?: return
        val had = m.reactions.any { it.emoji == emoji && it.mine }
        // Optimista: la reacció canvia al moment.
        val reactions = if (had) m.reactions.map { if (it.emoji == emoji) it.copy(by = it.by - "user") else it }.filter { it.by.isNotEmpty() }
            else if (m.reactions.any { it.emoji == emoji }) m.reactions.map { if (it.emoji == emoji) it.copy(by = it.by + "user") else it }
            else m.reactions + RetroReaction(emoji, listOf("user"))
        _retro.value = ch.copy(messages = ch.messages.map { if (it.id == messageId) it.copy(reactions = reactions) else it })
        viewModelScope.launch {
            try {
                val out = withContext(Dispatchers.IO) { _conn.value.api().retroReact(ch.key, messageId, emoji, !had) }
                if (out != null) _retro.value = out
            } catch (e: CancellationException) {
                throw e
            } catch (_: Exception) { }
        }
    }

    /** Modes que van a l'agent (la web: code|autonomous|debug, més inspect i goal). */
    private fun isAgentMode(m: String) = m in setOf("code", "inspect", "goal", "autonomous", "debug")

    /**
     * Un torn sencer: obre el treball plegable, consumeix els events en ordre
     * (un sol fil, el principal) i el tanca. Els events no es llancen en
     * corrutines soltes: abans el torn es donava per acabat abans de pintar
     * la resposta final.
     */
    private fun runTurn(t: String, imgs: List<String>, forceAgent: Boolean = false) {
        _sending.value = true
        _turnStart.value = System.currentTimeMillis()
        _statusLine.value = ""
        val agentMode = forceAgent || isAgentMode(_state.value.mode)
        turnErrorShown = false
        answerId = 0L
        beginTurn()
        viewModelScope.launch {
            var answered = false
            var failed = false
            try {
                GregalNotifications.workStarted(appContext)
                if (_v2Runs.value) {
                    val mode = if (agentMode) _state.value.mode.takeIf { isAgentMode(it) } ?: "code" else "chat"
                    answered = runV2Turn(t, mode, if (agentMode) imgs else emptyList())
                    failed = !answered
                } else {
                    val path = if (agentMode) "/api/agent" else "/api/chat"
                    val payload = JSONObject().put(if (agentMode) "task" else "message", t)
                    if (agentMode && imgs.isNotEmpty()) payload.put("images", JSONArray(imgs))
                    val api = _conn.value.api()
                    // El fil de xarxa només encua; el principal pinta en ordre.
                    coroutineScope {
                        val ch = Channel<SseEvent>(Channel.UNLIMITED)
                        launch(Dispatchers.IO) {
                            try { api.stream(path, payload) { ch.trySend(it) } } finally { ch.close() }
                        }
                        for (ev in ch) onSse(ev)
                    }
                    answered = !turnErrorShown
                    failed = turnErrorShown
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                failed = true
                if ((e.message ?: "").contains("409")) {
                    // Ja hi ha una bombolla d'usuari: guarda-la per reintentar,
                    // no la facis perdre ni hi afegeixis un error tècnic.
                    queueAfterBusy(t, alreadyShown = true)
                } else if (!turnErrorShown) {
                    push(ChatMsg("assistant", requestMessage(e), "error"))
                }
            }
            endTurn(failed)
            GregalNotifications.workFinished(appContext, answered, t, lastAssistantText())
            _sending.value = false
            _turnStart.value = 0L
            _statusLine.value = ""
            refresh()
            // Missatges que no van arribar a entrar al torn: ara com a missatge nou.
            if (afterTurn.isNotEmpty()) {
                val next = afterTurn.joinToString("\n\n")
                afterTurn.clear()
                send(next)
            }
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

    // --- Torn en curs: treball plegable, text en viu, activitat ---

    /** Text que el model escriu ara (fora de _msgs: no copia la llista a cada token). */
    private val _live = MutableStateFlow("")
    val live: StateFlow<String> = _live
    /** Què fa l'agent ara mateix, i des de quan (capçalera del treball). */
    private val _activity = MutableStateFlow<Activity?>(null)
    val activity: StateFlow<Activity?> = _activity
    private var workId = 0L          // ChatMsg "work" del torn en curs
    private var answerId = 0L        // resposta pintada aquest torn (pot ser narració)
    private var turnErrorShown = false

    private fun beginTurn() {
        synchronized(tokenLock) { tokenBuffer.clear() }
        _live.value = ""
        val now = System.currentTimeMillis()
        val m = ChatMsg("assistant", "", "work", at = now, work = WorkGroup(started = now))
        workId = m.id
        _msgs.value = _msgs.value + m
        setActivity("start", appText("app.work.starting"))
    }

    private fun updateWork(f: (WorkGroup) -> WorkGroup) {
        val id = workId
        if (id == 0L) return
        val cur = _msgs.value
        val i = cur.indexOfLast { it.id == id }
        if (i < 0) return
        val w = cur[i].work ?: return
        val nw = f(w)
        if (nw !== w) _msgs.value = cur.toMutableList().also { it[i] = cur[i].copy(work = nw) }
    }

    private fun step(kind: StepKind, label: String, target: String = "", meta: String = "",
                     detail: String = "", state: StepState = StepState.Ok) {
        updateWork { it.add(WorkStep("s${nextMsgId()}", kind, label, target, meta, detail, state)) }
    }

    private fun setActivity(key: String, text: String) {
        val a = _activity.value
        _activity.value = if (a != null && a.key == key) a.copy(text = text)
            else Activity(key, text, System.currentTimeMillis())
    }

    /** La narració escrita abans d'una eina és part del treball, no la resposta. */
    private fun narrationToWork() {
        flushTokens()
        val live = _live.value.trim()
        _live.value = ""
        if (live.isNotEmpty()) step(StepKind.Note, oneLine(live, 140), detail = live)
        val aid = answerId
        if (aid != 0L) {
            answerId = 0L
            val cur = _msgs.value
            val i = cur.indexOfLast { it.id == aid }
            if (i >= 0) {
                val t = cur[i].text
                _msgs.value = cur.filterIndexed { k, _ -> k != i }
                step(StepKind.Note, oneLine(t, 140), detail = t)
            }
        }
    }

    private fun endTurn(failed: Boolean) {
        flushTokens()
        val live = _live.value.trim()
        _live.value = ""
        if (live.isNotEmpty()) push(ChatMsg("assistant", live))
        val id = workId
        workId = 0L
        if (id != 0L) {
            val cur = _msgs.value
            val i = cur.indexOfLast { it.id == id }
            if (i >= 0) {
                val w = cur[i].work
                _msgs.value = if (w == null || w.steps.isEmpty()) cur.filterIndexed { k, _ -> k != i }
                else cur.toMutableList().also { it[i] = cur[i].copy(work = w.close(System.currentTimeMillis(), failed)) }
            }
        }
        _activity.value = null
    }

    private fun appendLive(tok: String) {
        if (tok.isEmpty()) return
        synchronized(tokenLock) {
            tokenBuffer.append(tok)
            if (tokenFlush?.isActive != true) {
                // ~16 fps: prou fluid i sense refer el markdown a cada token.
                tokenFlush = viewModelScope.launch {
                    delay(60)
                    flushTokens()
                }
            }
        }
    }

    private fun flushTokens() {
        val text = synchronized(tokenLock) {
            val out = tokenBuffer.toString()
            tokenBuffer.clear()
            out
        }
        if (text.isNotEmpty()) {
            _live.value = _live.value + text
            if (_activity.value?.key != "write") setActivity("write", appText("app.work.writing"))
        }
    }

    private fun onSse(ev: SseEvent) {
        val o = try { JSONObject(ev.data) } catch (_: Exception) { JSONObject() }
        onEvent(ev.name, o)
    }

    private fun showError(raw: String) {
        if (turnErrorShown) return
        turnErrorShown = true
        push(ChatMsg("assistant", friendlyError(raw.removePrefix("⚠️").trim()
            .ifEmpty { appText("app.status.requestFailed") }, _idioma.value), "error"))
    }

    /**
     * Un sol renderer per a la via SSE i la durable. Els events que no
     * coneix s'ignoren: abans sortien com a files de JSON cru.
     */
    private fun onEvent(kind: String, o: JSONObject, text: String = "") {
        val lang = _idioma.value
        fun f(k: String) = o.optString(k)
        if (kind != "token") flushTokens()
        when (kind) {
            "token" -> appendLive(f("text").ifEmpty { text })
            "assistant", "text" -> {
                val t = f("text").ifEmpty { text }
                _live.value = ""
                if (t.isNotBlank()) {
                    val m = ChatMsg("assistant", t)
                    answerId = m.id
                    push(m)
                }
            }
            "thinking", "thought", "reasoning" -> {
                narrationToWork()
                val t = f("text").ifEmpty { f("content") }.ifEmpty { text.removePrefix("💭").trim() }
                // Si el raonament és el mateix text que la narració, no el repeteixis.
                val dup = msgs.value.lastOrNull { it.id == workId }?.work?.steps?.lastOrNull()
                    ?.let { it.kind == StepKind.Note && it.detail.trim() == t.trim() } == true
                if (t.isNotBlank() && !dup) step(StepKind.Think, appText("app.work.thought"), oneLine(t, 140), detail = t)
            }
            "tool_call" -> {
                narrationToWork()
                val name = f("name").ifEmpty { text.removePrefix("🔧").trim() }
                val args = o.opt("args")?.toString().orEmpty()
                val sum = summarizeTool(name, args, lang)
                val id = f("id").ifEmpty { "c${nextMsgId()}" }
                updateWork { it.add(WorkStep(id, StepKind.Tool, sum.label, sum.target, sum.meta, "", StepState.Running, name)) }
                setActivity("tool:$id", sum.label + if (sum.target.isNotEmpty()) " · " + oneLine(sum.target, 48) else "")
            }
            "tool_result" -> {
                val out = f("output").ifEmpty { text.substringAfter(": ", "") }
                updateWork { it.finishTool(f("id"), f("name"), out, toolOutputFailed(out)) }
                setActivity("model", appText("app.work.thinking"))
            }
            "blocked" -> {
                val r = f("reason").ifEmpty { text.removePrefix("⛔️").trim() }
                updateWork { it.finishTool(f("id"), "", r, true) }
                step(StepKind.Blocked, appText("app.work.blocked"), oneLine(r, 140), detail = r, state = StepState.Failed)
            }
            "review" -> {
                val d = f("decision")
                step(StepKind.Review, appText("app.work.review"), listOf(d, f("name")).filter { it.isNotEmpty() }.joinToString(" · "),
                    "", f("reason"), if (d == "deny" || d == "block" || d == "reject") StepState.Failed else StepState.Ok)
            }
            "delegate" -> {
                val st = f("status").lowercase()
                val state = when {
                    "fail" in st || "error" in st -> StepState.Failed
                    st in setOf("done", "completed", "ok", "finished", "success") -> StepState.Ok
                    else -> StepState.Running
                }
                val agent = f("agent").ifEmpty { "subagent" }
                val meta = if (o.optInt("max_steps") > 0) "${o.optInt("step")}/${o.optInt("max_steps")}" else f("tool")
                val task = f("task").lineSequence().firstOrNull { it.isNotBlank() }.orEmpty()
                updateWork { it.upsert(WorkStep("d" + f("id").ifEmpty { agent }, StepKind.Delegate,
                    appText("app.work.subagent", agent), oneLine(task, 110), meta,
                    f("output").ifEmpty { f("message") }.take(4000), state)) }
                if (state == StepState.Running) setActivity("delegate:$agent", appText("app.work.subagent", agent) + "…")
            }
            "progress" -> {
                val p = mutableMapOf<String, String>()
                o.keys().forEach { k -> p[k] = o.optString(k) }
                val (k, t) = progressLine(p, lang)
                // Mentre corre una eina concreta, la seva línia és més útil.
                if (!(k.startsWith("tools") && _activity.value?.key?.startsWith("tool:") == true)) setActivity(k, t)
            }
            "status", "route" -> f("message").ifEmpty { text }.takeIf { it.isNotBlank() }?.let { _statusLine.value = it }
            "fallback" -> step(StepKind.Warn, appText("app.chat.fallback", f("model")))
            "verify_start" -> setActivity("verify", appText("app.chat.verifyStart", f("cmd")))
            "verify" -> {
                val v = f("verdict").ifEmpty { text.removePrefix("✅").trim() }
                step(StepKind.Review, appText("app.work.verify"), oneLine(v, 120), detail = f("detail"),
                    state = if (v.contains("fail", true)) StepState.Failed else StepState.Ok)
            }
            "compact" -> step(StepKind.Info, appText("app.chat.compacted"))
            "budget" -> step(StepKind.Warn, f("message").ifEmpty { text })
            "steps_exhausted" -> push(ChatMsg("assistant", appText("app.chat.stepsExhausted", o.optInt("max")), "error"))
            "tool_text" -> step(StepKind.Info, f("message").ifBlank { appText("app.chat.toolTextFallback") })
            "escalate", "gated" -> step(StepKind.Info, oneLine(f("message").ifBlank { f("reason") }.ifBlank { text }, 140))
            "autonomous_checkpoint" -> {
                val checks = o.optJSONArray("checks")?.let { a -> (0 until a.length()).map { a.optString(it) } }.orEmpty()
                step(StepKind.Info, appText("app.work.checkpoint", o.optInt("number")),
                    oneLine(checks.joinToString(" · "), 120), detail = f("review"))
            }
            "bet" -> step(StepKind.Info, appText("app.work.bet"), oneLine(f("command"), 100),
                f("p").takeIf { it.isNotEmpty() }?.let { "p=$it" }.orEmpty(),
                state = if (o.has("passed") && !o.optBoolean("passed")) StepState.Failed else StepState.Ok)
            "goal" -> push(ChatMsg("assistant", "◆ ${f("title")} (${f("id")})", "goal"))
            "approve_request" -> {
                _approve.value = ApproveReq(f("key"), f("call_id"), f("name"), o.opt("args")?.toString().orEmpty())
                setActivity("approve", appText("app.work.waitApprove"))
            }
            "question_request" -> {
                _question.value = QuestionReq(f("key"), f("call_id"), f("query"), parseQuestionOptions(o.optJSONArray("options")))
                setActivity("question", appText("app.work.waitAnswer"))
            }
            "question_timeout" -> { _question.value = null; step(StepKind.Warn, appText("app.chat.questionTimeout")) }
            "approve_timeout" -> { _approve.value = null; step(StepKind.Warn, appText("app.chat.approvalTimeoutDenied")) }
            "error" -> showError(f("message").ifEmpty { text })
            "run_failed" -> showError(text.ifEmpty { f("error") })
            "run_queued" -> setActivity("queued", appText("app.chat.runQueued"))
            "run_started" -> setActivity("start", appText("app.work.starting"))
            "run_cancelled" -> step(StepKind.Warn, appText("app.chat.cancelled"))
            // Missatges escrits mentre treballava (POST /api/steer).
            "steer" -> { steerDelivered(f("text").ifEmpty { text }); setActivity("model", appText("app.steer.applying")) }
            "steer_restart" -> { synchronized(tokenLock) { tokenBuffer.clear() }; _live.value = "" }
            "steer_unread" -> {
                val raw = text.ifEmpty { o.toString() }
                val arr = try { JSONObject(raw).optJSONArray("texts") } catch (_: Exception) { null } ?: o.optJSONArray("texts")
                if (arr != null) for (i in 0 until arr.length()) afterTurn += arr.optString(i)
            }
            else -> {} // done, run_completed, usage, lane, handoff…: res a pintar
        }
    }

    private fun push(m: ChatMsg) {
        val stamped = if (m.at == 0L) m.copy(at = System.currentTimeMillis()) else m
        _msgs.value = _msgs.value + stamped
    }

    // Cua de missatges escrits mentre un altre client ocupa l'agent (via
    // antiga): no se'n perd cap i hi ha un sol vigilant.
    private val queuedTasks = ArrayDeque<String>()
    private var waiter: Job? = null

    private fun queueAfterBusy(text: String, alreadyShown: Boolean = false) {
        queuedTasks.addLast(text)
        _queued.value = true
        if (!alreadyShown) push(ChatMsg("user", text))
        push(ChatMsg("assistant", appText("app.chat.queuedNotice"), "activity"))
        waitUntilFree()
    }

    /** Espera que l'agent quedi lliure (sondeja l'estat cada 3 s). */
    fun waitUntilFree() {
        if (waiter?.isActive == true) return
        waiter = viewModelScope.launch {
            repeat(100) {
                delay(3000)
                try {
                    _state.value = withContext(Dispatchers.IO) { _conn.value.api().getState() }
                } catch (_: Exception) { }
                if (!_state.value.agentBusy && !_sending.value) {
                    val pending = queuedTasks.removeFirstOrNull()
                    _queued.value = queuedTasks.isNotEmpty()
                    if (pending == null) return@launch
                    push(ChatMsg("assistant", appText("app.chat.continueQueued"), "activity"))
                    runTurn(pending, emptyList())
                    if (queuedTasks.isEmpty()) return@launch
                }
            }
            if (queuedTasks.isNotEmpty()) notify(appText("app.chat.busyTimeout"))
        }
    }

    /**
     * Submit + consum d'events durables. Un sol GET per volta; el torn acaba
     * amb l'event terminal del registre (i, si cal, confirmant /runs/{id}).
     * Els talls de xarxa curts no fan fallar el torn: es reintenta amb espera.
     */
    private suspend fun runV2Turn(text: String, mode: String, images: List<String>): Boolean {
        val api = _conn.value.api()
        val run = withContext(Dispatchers.IO) {
            api.submitRun(text, mode, images, UUID.randomUUID().toString())
        }
        if (run.id <= 0L) throw ApiException("no run returned by the server")
        // El registre és de tota la sessió: el que hi havia abans no és d'aquest torn.
        if (run.cursor > eventCursor) eventCursor = run.cursor
        return followRun(api, run.id)
    }

    /**
     * Segueix un torn pel registre durable fins que acaba. Serveix per als
     * torns nous i per enganxar-se a un que ja corria en obrir l'app.
     */
    private suspend fun followRun(api: GregalApi, runId: Long): Boolean {
        val run = RunV2(id = runId)
        activeRunId = run.id
        updateWork { it.copy(runId = runId) }
        var final = ""
        var runError = ""
        var errors = 0
        var lastCheck = System.currentTimeMillis()
        try {
            while (final.isEmpty()) {
                val page = try {
                    withContext(Dispatchers.IO) { api.getEvents(eventCursor, 200, sessioApp) }.also { errors = 0 }
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    if (++errors > 30) throw e
                    setActivity("net", appText("app.work.reconnecting"))
                    delay(minOf(5000L, 400L * errors))
                    continue
                }
                if (page.next > eventCursor) eventCursor = page.next
                for (ev in page.events) {
                    if (ev.runId != run.id) continue
                    receiveDurable(ev)
                    when (ev.kind) {
                        "run_completed" -> final = "completed"
                        "run_failed" -> { final = "failed"; runError = ev.text }
                        "run_cancelled" -> final = "cancelled"
                    }
                }
                if (final.isNotEmpty()) break
                val now = System.currentTimeMillis()
                if (page.events.isEmpty() && now - lastCheck > 4000) {
                    lastCheck = now
                    val cur = try {
                        withContext(Dispatchers.IO) { api.getRun(run.id) }
                    } catch (e: CancellationException) { throw e } catch (_: Exception) { null }
                    if (cur != null && cur.state in setOf("completed", "failed", "cancelled")) {
                        // Buida el que s'hagi escrit just abans de tancar.
                        val tail = withContext(Dispatchers.IO) { api.getEvents(eventCursor, 200, sessioApp) }
                        if (tail.next > eventCursor) eventCursor = tail.next
                        tail.events.filter { it.runId == run.id }.forEach { receiveDurable(it) }
                        final = cur.state
                        runError = cur.error
                    }
                }
                if (final.isEmpty()) delay(if (page.events.size >= 200) 0L else if (page.events.isEmpty()) 450L else 150L)
            }
            return when (final) {
                "failed" -> { showError(runError.ifBlank { appText("app.chat.turnFailed") }); false }
                "cancelled" -> false
                else -> true
            }
        } finally {
            activeRunId = null
        }
    }

    /** Traducció del registre durable al mateix renderer de la via SSE. */
    private fun receiveDurable(ev: DurableEvent) {
        when (ev.kind) {
            "approve_request", "question_request" -> {
                val o = durablePayload(ev)
                if (o != null) onEvent(ev.kind, o)
                else push(ChatMsg("assistant", appText(if (ev.kind == "approve_request") "app.chat.approvalNoPayload" else "app.chat.questionNoPayload"), "error"))
            }
            else -> {
                val o = if (ev.payload.isNotBlank()) try { JSONObject(ev.payload) } catch (_: Exception) { JSONObject() } else JSONObject()
                onEvent(ev.kind, o, ev.text)
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
        setActivity("cancel", appText("app.work.stopping"))
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

    private fun requestMessage(e: Exception): String {
        val m = e.message.orEmpty()
        return when {
            m.contains("401") -> appText("app.status.requestBadToken")
            m.contains("failed to connect", true) || m.contains("unreachable", true) -> appText("app.status.requestConnection")
            m.isNotBlank() -> friendlyError(m, _idioma.value)
            else -> appText("app.status.requestFailed")
        }
    }

    fun answerApprove(ok: Boolean, remember: Boolean = false) {
        val a = _approve.value ?: return
        _approve.value = null
        val label = when {
            ok && remember -> appText("app.chat.approvalAllowedAlways", a.name)
            ok -> appText("app.chat.approvalAllowed", a.name)
            else -> appText("app.chat.approvalDenied", a.name)
        }
        if (workId != 0L) step(StepKind.Info, label, state = if (ok) StepState.Ok else StepState.Failed)
        else push(ChatMsg("assistant", label, "activity"))
        setActivity("model", appText("app.work.thinking"))
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
                val (sessions, active) = loadingOf("sessions") {
                    withContext(Dispatchers.IO) { api.getSessions() to api.getActive() }
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
    fun setMode(mode: String) {
        // Optimista: el composer canvia a l'instant; refresh() ho confirma.
        _state.value = _state.value.copy(mode = mode)
        simplePost("/api/mode", JSONObject().put("mode", mode))
    }

    /** «Primer el pla» (com la web): a Actua, l'enviament demana un pla abans. */
    private val _planFirst = MutableStateFlow(false)
    val planFirst: StateFlow<Boolean> = _planFirst
    fun setPlanFirst(on: Boolean) { _planFirst.value = on }

    /** Bloqueig de permisos: manual, sense preguntar o revisor automàtic. */
    fun setPermissive(on: Boolean, review: Boolean) {
        _state.value = _state.value.copy(permissive = on && !review, autoReview = review)
        simplePost("/api/permissive", JSONObject().put("on", on).put("review", review))
    }

    /** Envia segons el composer: amb «primer el pla» a Actua, primer el pla.
     *  Si l'agent ja treballa, el missatge li arriba ara (steer). */
    fun submit(text: String, interrupt: Boolean = false) {
        when {
            _sending.value && workId != 0L -> steer(text, interrupt)
            _planFirst.value && _state.value.mode == "code" -> requestPlan(text)
            else -> send(text)
        }
    }

    /**
     * Missatge per al torn en curs (com els altres agents de codi): entra al
     * següent punt segur i el model el té en compte; amb interrupt, talla la
     * crida al model que estigui fent. Si el torn acabava just ara, s'envia
     * com un missatge nou en acabar.
     */
    fun steer(text: String, interrupt: Boolean = false) {
        val t = text.trim()
        if (t.isEmpty()) return
        updateWork { it.add(WorkStep("u${nextMsgId()}", StepKind.User,
            appText(if (interrupt) "app.steer.tagInterrupt" else "app.steer.tag"), oneLine(t, 200),
            appText("app.steer.pending"), t, StepState.Running)) }
        setActivity("steer", appText("app.steer.applying"))
        viewModelScope.launch {
            val ok = try {
                withContext(Dispatchers.IO) { _conn.value.api().steer(t, interrupt) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                notify(requestMessage(e)); null
            }
            if (ok == false) afterTurn += t
        }
    }

    /** Missatges que el torn ja no ha pogut llegir: s'envien en acabar. */
    private val afterTurn = mutableListOf<String>()

    private fun steerDelivered(text: String) {
        updateWork { w ->
            val i = w.steps.indexOfLast { it.kind == StepKind.User && it.state == StepState.Running && it.detail == text }
            if (i < 0) w.add(WorkStep("u${nextMsgId()}", StepKind.User, appText("app.steer.tag"), oneLine(text, 200), appText("app.steer.read"), text, StepState.Ok))
            else w.copy(steps = w.steps.toMutableList().also { it[i] = it[i].copy(state = StepState.Ok, meta = appText("app.steer.read")) })
        }
    }
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
                _flows.value = loadingOf("flows") { withContext(Dispatchers.IO) { _conn.value.api().getFlows() } }
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
                _goals.value = loadingOf("goals") { withContext(Dispatchers.IO) { _conn.value.api().getGoals() } }
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
                _jobs.value = loadingOf("jobs") { withContext(Dispatchers.IO) { _conn.value.api().getJobs() } }
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
                _runs.value = loadingOf("runs") { withContext(Dispatchers.IO) { _conn.value.api().getRuns(jobId) } }
            } catch (e: Exception) { notify(appText("app.status.runs", e.message.orEmpty())) }
        }
    }
}
