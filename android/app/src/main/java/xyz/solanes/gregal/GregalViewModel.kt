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
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import android.net.Uri
import android.util.Base64
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID

/** ID estable del job diari de check-in del company (backend: jobs.json). */
const val CompanyJobId = "company-checkin-daily"

/** Prompt del job de check-in: el company resumeix i pregunta, curt. */
const val COMPANY_CHECKIN_PROMPT = "Fes una revisió proactiva i de només lectura del projecte actiu. Consulta els objectius oberts, l’estat documentat dels serveis i les darreres proves o canvis que puguis verificar. No inventis observacions: inclou el fitxer, prova, estat o font que sosté cada afirmació, i acaba amb un únic pas següent recomanat. No modifiquis fitxers, no reiniciïs ni despleguis serveis. Si cal actuar, proposa-ho i espera aprovació explícita. Respon en català, breu i útil."

/** Pantalles de l'app. */
enum class Screen { Pairing, Chat, Sessions, Goals, Jobs, Office, Flows, Prefs, Retro, Company, Work }

/**
 * On parla l'app. `local` és el servidor d'aquest mateix dispositiu (el
 * backend Go dins l'APK): el port i el token els posa LocalServer.
 */
data class Conn(val host: String, val port: String, val token: String, val user: String = "",
                val local: Boolean = false) {
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
    /** Salut del segon pla (mode local): alarmes exactes, notificacions, bateria. */
    private val _bgHealth = MutableStateFlow<BackgroundHealth?>(null)
    val bgHealth: StateFlow<BackgroundHealth?> = _bgHealth
    private val _runs = MutableStateFlow<List<RunItem>>(emptyList())
    val runs: StateFlow<List<RunItem>> = _runs

    private val _models = MutableStateFlow<Map<String, List<String>>>(emptyMap())
    val models: StateFlow<Map<String, List<String>>> = _models

    private val _windsModel = MutableStateFlow("")
    val windsModel: StateFlow<String> = _windsModel

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
                val cau = llegeixCache(appContext.cacheDir, System.currentTimeMillis(), abisDelMobil())
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
                val info = parseUpdateText(cos, abisDelMobil())
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
    private val _idiomaChoice = MutableStateFlow("system")
    val idiomaChoice: StateFlow<String> = _idiomaChoice

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
        _idiomaChoice.value = p.idiomaChoice
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

    /** Local UI preference is authoritative, never replaced by server defaults. */
    fun setIdioma(v: String) {
        if (!::prefsStore.isInitialized) return
        prefsStore.idioma = v
        _idiomaChoice.value = prefsStore.idiomaChoice
        _idioma.value = prefsStore.idioma
        syncAgentLanguage()
    }

    fun refreshSystemLanguage() {
        if (!::prefsStore.isInitialized || prefsStore.idiomaChoice != "system") return
        val resolved = prefsStore.idioma
        if (_idioma.value != resolved) {
            _idioma.value = resolved
            syncAgentLanguage()
        }
    }

    private fun syncAgentLanguage() {
        if (!_connected.value) return
        val c = _conn.value
        val language = _idioma.value
        viewModelScope.launch {
            try { withContext(Dispatchers.IO) {
                if (_conn.value == c) c.api().post("/api/lang", JSONObject().put("lang", language))
            } } catch (_: Exception) {}
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

    // --- Pila de navegació: enrere ha de tirar enrere, no matar l'app ---

    /** Pantalles per on has passat (sense l'actual). La primera és l'arrel (Vents). */
    private val _backStack = MutableStateFlow<List<Screen>>(emptyList())
    val backStack: StateFlow<List<Screen>> = _backStack

    fun go(s: Screen) {
        val cur = _screen.value
        if (s != cur) {
            _backStack.value = if (cur == Screen.Pairing || s == Screen.Pairing) emptyList() else _backStack.value + cur
            _screen.value = s
        }
    }

    /** Hi ha alguna pantalla a la qual tornar (o no som a l'arrel)? */
    val canGoBack: Boolean get() = _backStack.value.isNotEmpty() || _screen.value != Screen.Company

    /** Neteja la pila (només per a proves). */
    fun resetBackStackForTest() { _backStack.value = emptyList() }

    /** Conversa oberta dins els Vents ("wind:<id>" / "rot:<id>"), perquè l'enrere la tanqui. */
    private val _ventsOpen = MutableStateFlow<String?>(null)
    val ventsOpen: StateFlow<String?> = _ventsOpen
    fun openVentsChat(id: String?) { _ventsOpen.value = id }

    /**
     * Enrere: torna a la pantalla anterior si n'hi ha; si ja ets a l'arrel i no
     * tens pila, torna a Vents; retorna false només quan ja ets a Vents i no hi
     * ha on tornar (l'app pot sortir).
     */
    fun back(): Boolean {
        val stack = _backStack.value
        if (stack.isNotEmpty()) {
            _screen.value = stack.last()
            _backStack.value = stack.dropLast(1)
            return true
        }
        if (_screen.value != Screen.Company) {
            _screen.value = Screen.Company
            return true
        }
        return false
    }

    fun newChat() {
        _msgs.value = emptyList()
        go(Screen.Chat)
    }

    fun disconnect() {
        _connected.value = false
        resetServerState()
        go(Screen.Pairing)
    }

    // --- On corre Gregal: aquest dispositiu o un servidor remot ---

    /** Estat del backend local (per a la pantalla de connexió). */
    val localState: StateFlow<LocalState> = LocalServer.state

    fun localAvailable(): Boolean = LocalServer.available(appContext)

    /**
     * Canviar de servidor és canviar de món: converses, cursor d'events i
     * estat són de l'altre. Res del que es veia ha de passar a l'altre.
     */
    // Identifies a connection generation, including switching away and back.
    private var companionEpoch = 0L
    // Per conversation: each GET supersedes older GETs; sending invalidates them
    // before optimistic history is published, even if they arrive after busy clears.
    private val windHistoryRequests = mutableMapOf<String, Long>()
    private val rotllanaHistoryRequests = mutableMapOf<String, Long>()
    private fun isCompanionOrigin(source: Conn, epoch: Long) =
        _conn.value == source && companionEpoch == epoch

    private fun resetServerState() {
        companionEpoch++
        windHistoryRequests.clear()
        rotllanaHistoryRequests.clear()
        _winds.value = emptyList()
        _windChat.value = emptyMap()
        _windBusy.value = emptySet()
        _rotllanes.value = emptyList()
        _rotllanaChat.value = emptyMap()
        _rotllanaBusy.value = emptySet()
        _activeWind.value = null
        _ventsOpen.value = null
        _windCreator.value = false to null
        profileRequest++
        _windProfile.value = null
        _windProfileBusy.value = false
        _windProfileError.value = null
        _company.value = CompanyState()
        _checkin.value = null
        _pets.value = emptyList()
        _petCheckins.value = emptyMap()
        _petChat.value = emptyMap()
        _petChatBusy.value = emptySet()
        _backStack.value = emptyList()
        _msgs.value = emptyList()
        _state.value = ServerState()
        _sessions.value = emptyList()
        _active.value = null
        _approve.value = null
        _question.value = null
        _goals.value = emptyList()
        _jobs.value = emptyList()
        _runs.value = emptyList()
        _models.value = emptyMap()
        _providers.value = emptyList()
        _catalog.value = emptyList()
        _provMsg.value = ""
        _needsProvider.value = false
        _providerSetupBusy.value = false
        _retro.value = null
        eventCursor = 0L
        activeRunId = null
    }

    /**
     * L'app passa a segon pla. Si l'agent ja esperava una aprovació o una
     * resposta (va arribar amb l'app oberta), ara cal avisar: si no, el torn
     * es quedava aturat sense que ho sabessis.
     */
    fun onBackgrounded() {
        if (_approve.value != null || _question.value != null) GregalNotifications.needsYou(appContext)
    }

    /** Fes servir Gregal en aquest dispositiu (el mode per defecte). */
    fun useThisDevice(retries: Int = 3) {
        if (!_conn.value.local) resetServerState()
        prefsStore?.target = Prefs.TARGET_LOCAL
        // En local els jobs els dispara l'app: el vigilant remot sobra.
        WindWatch.cancel(appContext)
        setConn(Conn("127.0.0.1", "", "", local = true))
        LocalServer.resetCrashes()
        connect(retries) { go(landing()) }
    }

    /** Fes servir un servidor remot; el local s'atura per no gastar bateria. */
    private fun useRemote(c: Conn) {
        if (_conn.value.local || _conn.value.host != c.host || _conn.value.port != c.port) resetServerState()
        prefsStore?.target = Prefs.TARGET_REMOTE
        // Al servidor remot els jobs els programa ell: res a despertar aquí,
        // però sí a vigilar les veus dels vents per avisar amb notificació.
        JobAlarms.clear(appContext)
        WindWatch.schedule(appContext)
        setConn(c)
        viewModelScope.launch { try { LocalServer.stop() } catch (_: Exception) {} }
    }

    /** Connecta amb el token (servidor sense comptes d'usuari). */
    fun connectRemote(host: String, port: String, token: String) {
        val user = prefsStore?.usuari.orEmpty()
        prefsStore?.save(host, port, token, user)
        useRemote(Conn(host, port, token, user))
        connect { go(Screen.Company) }
    }

    /** Reprèn la connexió desada en obrir l'app. */
    fun startSaved(p: Prefs) {
        when {
            p.target == Prefs.TARGET_LOCAL && localAvailable() -> useThisDevice(retries = 3)
            p.host.isNotBlank() -> {
                setConn(Conn(p.host, p.port, p.token, p.usuari))
                WindWatch.schedule(appContext)
                connect(retries = 8) { go(landing()) }
            }
        }
    }

    /** Torna a la pantalla de connexió per triar un altre servidor. */
    fun switchServer() = disconnect()

    /** Engega (o reaprofita) el backend local i hi apunta la connexió. */
    private suspend fun ensureLocal() {
        val r = LocalServer.ensure(appContext, _idioma.value)
        val c = _conn.value
        if (c.port != r.port.toString() || c.token != r.token) {
            _conn.value = c.copy(host = "127.0.0.1", port = r.port.toString(), token = r.token)
        }
    }

    // --- Primer ús: connectar un proveïdor ---

    private val _needsProvider = MutableStateFlow(false)
    /** En local i sense cap clau: el xat buit ensenya la configuració del proveïdor. */
    val needsProvider: StateFlow<Boolean> = _needsProvider
    private val _catalog = MutableStateFlow<List<CatalogEntry>>(emptyList())
    val catalog: StateFlow<List<CatalogEntry>> = _catalog
    private val _providerSetupBusy = MutableStateFlow(false)
    val providerSetupBusy: StateFlow<Boolean> = _providerSetupBusy

    /** Primer cop en local sense cap clau: el xat demana connectar un proveïdor. */
    private fun hintMissingKey() { loadProviders() }

    fun loadCatalog() {
        val c = _conn.value
        viewModelScope.launch {
            try {
                val entries = parseCatalog(withContext(Dispatchers.IO) { c.api().providersJson() })
                if (_conn.value == c) _catalog.value = entries
            } catch (e: Exception) {
                if (_conn.value == c) _provMsg.value = appText("app.status.providers", e.message.orEmpty())
            }
        }
    }

    /**
     * Clau + preset en un pas, com el web: la clau es desa (al dispositiu, al
     * magatzem de claus) i tots els rols passen al proveïdor triat.
     */
    fun connectProvider(name: String, key: String, onOk: () -> Unit = {}) {
        if (name.isBlank() || key.isBlank() || _providerSetupBusy.value) return
        val c = _conn.value
        val usePreset = _catalog.value.any { it.name == name }
        _providerSetupBusy.value = true
        _provMsg.value = ""
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) {
                    if (_conn.value != c) return@withContext
                    val api = c.api()
                    api.setProviderKey(name, key)
                    if (_conn.value == c && usePreset) api.presetProvider(name)
                }
                if (_conn.value != c) return@launch
                _provMsg.value = appText("app.setup.ready")
                refresh()
                loadProviders()
                loadModels()
                onOk()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                if (_conn.value == c) _provMsg.value = appText("app.menu.providerSaveError", e.message.orEmpty())
            } finally {
                if (_conn.value == c) _providerSetupBusy.value = false
            }
        }
    }

    fun setConn(c: Conn) {
        if (_conn.value != c) resetServerState()
        _conn.value = c
    }

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
                useRemote(Conn(host, port, tok, usuari.trim()))
                connect { go(Screen.Company) }
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
                    if (_conn.value.local) ensureLocal()
                    connectOnce()
                    syncAgentLanguage()
                    onOk()
                    syncJobAlarms()
                    loadWinds()
                    loadRotllanes()
                    hintMissingKey()
                    restoreLocalSession()
                    return@launch
                } catch (e: CancellationException) {
                    throw e
                } catch (e: LocalServerException) {
                    // El backend del dispositiu no arrenca: ho diu la pantalla
                    // de connexió amb el log; reintentar no ho arreglarà sol.
                    _connected.value = false
                    notify(appText("app.local.failed", e.message.orEmpty()))
                    return@launch
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
        go(Screen.Retro)
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

    /** Canvia el mood del #retro: el servidor reescriu els missatges
    d'agent ja publicats amb el to nou (mateixos IDs i ordre). */
    fun setRetroMood(persona: String) {
        val ch = _retro.value ?: return
        if (persona == ch.persona || _retroState.value == "rewriting") return
        if (persona !in RETRO_MOODS.map { it.id }) return
        _retroState.value = "rewriting"
        viewModelScope.launch {
            try {
                val out = withContext(Dispatchers.IO) { _conn.value.api().retroRewrite(ch.key.ifBlank { retroKey }, persona) }
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
            rememberLocalSession()
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
        val c = _conn.value
        viewModelScope.launch {
            try {
                val entries = withContext(Dispatchers.IO) { c.api().getProviders() }
                if (_conn.value == c) {
                    _providers.value = entries
                    _needsProvider.value = entries.none { it.hasKey() }
                }
            } catch (e: Exception) {
                if (_conn.value == c) _provMsg.value = appText("app.status.providers", e.message.orEmpty())
            }
        }
    }

    fun setProviderKey(name: String, key: String) {
        val c = _conn.value
        _provMsg.value = appText("app.status.providerSaving")
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { if (_conn.value == c) c.api().setProviderKey(name, key) }
                if (_conn.value != c) return@launch
                _provMsg.value = appText("app.menu.keySaved")
                loadProviders()
                loadModels()
            } catch (e: Exception) {
                if (_conn.value == c) _provMsg.value = appText("app.menu.providerSaveError", e.message.orEmpty())
            }
        }
    }

    fun testProvider(name: String) {
        val c = _conn.value
        _provMsg.value = appText("app.status.providerTesting")
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { if (_conn.value == c) c.api().testProvider(name) }
                if (_conn.value == c) _provMsg.value = appText("app.menu.providerTested")
            } catch (e: Exception) {
                if (_conn.value == c) _provMsg.value = appText("app.menu.providerTestError", e.message.orEmpty())
            }
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
    /** S'ha premut Atura en aquest torn: els errors que en vinguin no són errors. */
    @Volatile private var stopRequested = false

    private fun beginTurn() {
        stopRequested = false
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
                else cur.toMutableList().also { it[i] = cur[i].copy(work = w.close(System.currentTimeMillis(), failed, stopped = stopRequested)) }
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
                GregalNotifications.needsYou(appContext)
            }
            "question_request" -> {
                _question.value = QuestionReq(f("key"), f("call_id"), f("query"), parseQuestionOptions(o.optJSONArray("options")))
                setActivity("question", appText("app.work.waitAnswer"))
                GregalNotifications.needsYou(appContext)
            }
            "question_timeout" -> { _question.value = null; step(StepKind.Warn, appText("app.chat.questionTimeout")) }
            "approve_timeout" -> { _approve.value = null; step(StepKind.Warn, appText("app.chat.approvalTimeoutDenied")) }
            // Aturar és una acció teva, no un error: el «torn aturat per
            // l'usuari» del servidor sortia en vermell sota «Torn cancel·lat».
            "error" -> if (!stopRequested) showError(f("message").ifEmpty { text })
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
    private suspend fun followRun(api0: GregalApi, runId: Long): Boolean {
        var api = api0
        val run = RunV2(id = runId)
        var missing = 0
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
                    // En local, si el backend ha caigut es torna a engegar
                    // (potser en un altre port): segueix pel nou.
                    if (_conn.value.local && errors % 3 == 0) {
                        try { ensureLocal(); api = _conn.value.api() } catch (_: Exception) {}
                    }
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
                        withContext(Dispatchers.IO) { api.getRun(run.id) }.also { missing = 0 }
                    } catch (e: CancellationException) { throw e } catch (e: Exception) {
                        // Un servidor reiniciat ja no coneix el torn i, si és
                        // antic, tampoc el tanca al registre: sense això
                        // s'esperava el final per sempre.
                        if ((e.message ?: "").contains("HTTP 404") && ++missing >= 2) {
                            final = "failed"
                            runError = appText("app.chat.runLost")
                        }
                        null
                    }
                    if (final.isNotEmpty()) break
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
        stopRequested = true
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

    /**
     * En local, recorda en quina conversa érem. Si el sistema mata l'app,
     * el backend mor amb ella (línia de vida) i el nou arrenca amb una
     * conversa buida: sense això, reobrir l'app semblava haver-ho perdut tot.
     */
    private fun rememberLocalSession() {
        if (!_conn.value.local) return
        viewModelScope.launch {
            val cur = try {
                withContext(Dispatchers.IO) { _conn.value.api().getSessions() }.firstOrNull { it.current }
            } catch (_: Exception) { null } ?: return@launch
            prefsStore.localSession = cur.name
        }
    }

    /** Després d'un reinici del backend local, torna a la conversa d'abans. */
    private fun restoreLocalSession() {
        if (!_conn.value.local || _msgs.value.isNotEmpty() || _sending.value) return
        val name = prefsStore.localSession
        if (name.isBlank()) return
        viewModelScope.launch {
            val known = try {
                withContext(Dispatchers.IO) { _conn.value.api().getSessions() }
            } catch (_: Exception) { return@launch }
            if (known.any { it.current && it.name == name }) return@launch
            if (known.none { it.name == name }) { prefsStore.localSession = ""; return@launch }
            // La pantalla ja l'ha triada la connexió (pot ser l'execució
            // d'una notificació de job): només es recupera la conversa.
            resumeSession(name, navigate = false)
        }
    }

    fun resumeSession(name: String, navigate: Boolean = true) {
        if (_conn.value.local) prefsStore.localSession = name
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
                if (navigate) go(Screen.Chat)
            } catch (e: Exception) { notify(appText("app.status.sessionResume", e.message.orEmpty())) }
        }
    }

    fun newSession() {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { _conn.value.api().post("/api/new", JSONObject()) }
                if (_conn.value.local) prefsStore.localSession = ""
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
        val c = _conn.value
        viewModelScope.launch {
            try {
                val (m, _, winds) = withContext(Dispatchers.IO) { c.api().getModels() }
                if (_conn.value == c) {
                    _models.value = m
                    _windsModel.value = winds
                }
            } catch (e: Exception) {
                if (_conn.value == c) notify(appText("app.status.models", e.message.orEmpty()))
            }
        }
    }

    /** Model per defecte dels vents: el tria l'usuari, el valida el servidor. */
    fun setWindsModel(model: String) {
        viewModelScope.launch {
            try {
                val selected = withContext(Dispatchers.IO) { _conn.value.api().setWindsModel(model) }
                val winds = selected.optString("selection")
                if (winds.isNotBlank()) _windsModel.value = winds
                loadModels()
                notify(appText("app.vents.modelSelected", winds.ifBlank { model }))
            } catch (e: Exception) {
                notify(appText("app.status.modelChange", e.message.orEmpty()))
            }
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

    // ---- Mode company (estil Dot): memòria personal + check-ins ----

    private val _company = MutableStateFlow(CompanyState())
    val company: StateFlow<CompanyState> = _company
    private val _checkin = MutableStateFlow<CompanyCheckin?>(null)
    val checkin: StateFlow<CompanyCheckin?> = _checkin

    fun loadCompany() {
        viewModelScope.launch {
            try {
                _company.value = loadingOf("company") { withContext(Dispatchers.IO) { _conn.value.api().getCompany() } }
            } catch (e: Exception) { notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    fun acceptFact(fact: String) {
        viewModelScope.launch {
            try {
                _company.value = withContext(Dispatchers.IO) { _conn.value.api().companyAccept(fact) }
            } catch (e: Exception) { notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    fun dismissFacts() {
        viewModelScope.launch {
            try {
                _company.value = withContext(Dispatchers.IO) { _conn.value.api().companyDismiss() }
            } catch (e: Exception) { notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    fun forgetCompany(term: String) {
        viewModelScope.launch {
            try {
                _company.value = withContext(Dispatchers.IO) { _conn.value.api().companyForget(term) }
                notify(appText("app.company.forgotten"))
            } catch (e: Exception) { notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    /**
     * Job diari de revisió del projecte actiu (per defecte 08:00, editable a Feines).
     * Idempotent: preserva l'horari, l'estat i les notificacions configurades per l'usuari.
     */
    fun ensureCheckinJob() {
        viewModelScope.launch {
            try {
                val api = _conn.value.api()
                val jobs = withContext(Dispatchers.IO) { api.getJobs() }
                val existing = jobs.firstOrNull { it.id == CompanyJobId }
                if (existing == null) {
                    withContext(Dispatchers.IO) {
                        api.saveJob(JobItem(
                            id = CompanyJobId,
                            name = appText("app.company.jobName"),
                            prompt = COMPANY_CHECKIN_PROMPT,
                            kind = "daily", time = "08:00",
                            enabled = true, notify = true,
                        ))
                    }
                } else if (existing.prompt != COMPANY_CHECKIN_PROMPT || existing.name != appText("app.company.jobName")) {
                    // Actualitza el contingut sense trepitjar l'horari o les opcions de notificació.
                    withContext(Dispatchers.IO) {
                        api.saveJob(existing.copy(name = appText("app.company.jobName"), prompt = COMPANY_CHECKIN_PROMPT))
                    }
                }
                if (_conn.value.local) {
                    val current = withContext(Dispatchers.IO) { api.getJobs() }
                    JobAlarms.sync(appContext, current)
                }
            } catch (e: Exception) { /* el job és optatiu: mai no tomba la pantalla */ }
        }
    }

    fun triggerCheckin() {
        viewModelScope.launch {
            try {
                _checkin.value = withContext(Dispatchers.IO) { _conn.value.api().companyCheckin() }
            } catch (e: Exception) { notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    /** Inicia una tasca real al workspace amb el motor d'agent i les aprovacions habituals. */
    fun startCompanyWork(request: String, makeChanges: Boolean) {
        val mode = if (makeChanges) "code" else "inspect"
        val task = CompanyWorkPrompt.build(request, makeChanges)
        if (task.isBlank() || _sending.value) return
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) {
                    val api = _conn.value.api()
                    if (makeChanges) {
                        api.post("/api/permissive", JSONObject().put("on", false).put("review", false))
                    }
                    api.post("/api/mode", JSONObject().put("mode", mode))
                }
                _state.value = if (makeChanges) {
                    _state.value.copy(mode = mode, permissive = false, autoReview = false)
                } else {
                    _state.value.copy(mode = mode)
                }
                go(Screen.Chat)
                send(task, forceAgent = true)
            } catch (e: Exception) { notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    // ---- Mascotes del company ----

    // ---- Vents: companions personals amb nom de vent ----
    private val _winds = MutableStateFlow<List<Wind>>(emptyList())
    val winds: StateFlow<List<Wind>> = _winds

    private val _windChat = MutableStateFlow<Map<String, List<PetMsg>>>(emptyMap())
    val windChat: StateFlow<Map<String, List<PetMsg>>> = _windChat

    private val _windBusy = MutableStateFlow<Set<String>>(emptySet())
    val windBusy: StateFlow<Set<String>> = _windBusy

    /** El vent actiu al botó flotant (null = cap). */
    private val _activeWind = MutableStateFlow<Wind?>(null)
    val activeWind: StateFlow<Wind?> = _activeWind

    fun loadWinds() {
        val source = _conn.value
        val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val list = withContext(Dispatchers.IO) { source.api().getWinds() }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _winds.value = list
                _activeWind.value = list.firstOrNull { it.id == _activeWind.value?.id } ?: list.firstOrNull()
                list.forEach { loadWindChat(it.id) }
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) notify(appText("app.status.company", e.message.orEmpty()))
            }
        }
    }

    private val _windProfile = MutableStateFlow<WindProfile?>(null)
    val windProfile: StateFlow<WindProfile?> = _windProfile
    private val _windProfileBusy = MutableStateFlow(false)
    val windProfileBusy: StateFlow<Boolean> = _windProfileBusy
    private val _windProfileError = MutableStateFlow<String?>(null)
    val windProfileError: StateFlow<String?> = _windProfileError
    private var profileRequest = 0L

    fun loadWindProfile(id: String) {
        val source = _conn.value; val epoch = companionEpoch; val request = ++profileRequest
        _windProfile.value = null
        _windProfileBusy.value = true
        _windProfileError.value = null
        viewModelScope.launch {
            try {
                val profile = withContext(Dispatchers.IO) { source.api().getWindProfile(id) }
                if (isCompanionOrigin(source, epoch) && request == profileRequest) _windProfile.value = profile
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch) && request == profileRequest)
                    _windProfileError.value = appText("app.status.company", e.message.orEmpty())
            } finally {
                if (isCompanionOrigin(source, epoch) && request == profileRequest) _windProfileBusy.value = false
            }
        }
    }

    fun updateWindKnowledge(id: String, text: String, onDone: (Boolean) -> Unit = {}) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val result = withContext(Dispatchers.IO) { source.api().updateWindKnowledge(id, text) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                if (_windProfile.value?.wind?.id == id) _windProfile.value = result
                onDone(true)
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) {
                    notify(appText("app.status.company", e.message.orEmpty())); onDone(false)
                }
            }
        }
    }

    fun createWind(name: String, color: String, role: String, style: String = "pso", persona: String = "", id: String = "", onDone: (Boolean) -> Unit = {}) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().saveWind(name, color, role, style, persona, id) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                // Read back the exact catalogue before reporting success to the creator.
                val list = withContext(Dispatchers.IO) { source.api().getWinds() }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _winds.value = list
                _activeWind.value = list.firstOrNull { it.id == _activeWind.value?.id } ?: list.firstOrNull()
                if (id.isNotBlank() && _windProfile.value?.wind?.id == id) loadWindProfile(id)
                onDone(true)
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) {
                    notify(appText("app.status.company", e.message.orEmpty())); onDone(false)
                }
            }
        }
    }

    // Named conversations: one member is an individual thread, 2+ a rotllana.
    private val _rotllanes = MutableStateFlow<List<Rotllana>>(emptyList())
    val rotllanes: StateFlow<List<Rotllana>> = _rotllanes
    private val _rotllanaChat = MutableStateFlow<Map<String, List<PetMsg>>>(emptyMap())
    val rotllanaChat: StateFlow<Map<String, List<PetMsg>>> = _rotllanaChat
    private val _rotllanaBusy = MutableStateFlow<Set<String>>(emptySet())
    val rotllanaBusy: StateFlow<Set<String>> = _rotllanaBusy

    fun loadRotllanes() {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val list = withContext(Dispatchers.IO) { source.api().getRotllanes() }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _rotllanes.value = list
                list.forEach { loadRotllanaChat(it.id) }
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) notify(appText("app.status.company", e.message.orEmpty()))
            }
        }
    }

    fun createRotllana(name: String, members: List<String>, onError: () -> Unit = {}, onDone: () -> Unit = {}) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val result = withContext(Dispatchers.IO) { source.api().saveRotllana(name, members) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                val id = result.optJSONObject("rotllana")?.optString("id").orEmpty()
                val list = withContext(Dispatchers.IO) { source.api().getRotllanes() }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _rotllanes.value = list
                onDone()
                if (id.isNotBlank() && list.any { it.id == id }) openVentsChat("rot:" + id)
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) { notify(appText("app.status.company", e.message.orEmpty())); onError() }
            }
        }
    }

    fun editRotllana(rot: Rotllana, onDone: (Boolean) -> Unit = {}) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().saveRotllana(rot.name, rot.members, rot.id) }
                val list = withContext(Dispatchers.IO) { source.api().getRotllanes() }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _rotllanes.value = list
                onDone(true)
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) { notify(appText("app.status.company", e.message.orEmpty())); onDone(false) }
            }
        }
    }

    fun deleteRotllana(id: String) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().deleteRotllana(id) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _rotllanes.value = _rotllanes.value.filterNot { it.id == id }
                _rotllanaChat.value = _rotllanaChat.value - id
                if (_ventsOpen.value == "rot:$id") _ventsOpen.value = null
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    fun loadRotllanaChat(id: String) {
        val source = _conn.value; val epoch = companionEpoch
        if (_rotllanaBusy.value.contains(id)) return
        val request = (rotllanaHistoryRequests[id] ?: 0L) + 1
        rotllanaHistoryRequests[id] = request
        viewModelScope.launch {
            try {
                val msgs = withContext(Dispatchers.IO) { source.api().getRotllanaChat(id) }
                if (isCompanionOrigin(source, epoch) && request == rotllanaHistoryRequests[id] &&
                    !_rotllanaBusy.value.contains(id))
                    _rotllanaChat.value = _rotllanaChat.value + (id to msgs)
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    fun sendRotllanaMessage(rotllanaId: String, message: String, wind: String = "") {
        if (message.isBlank() || _rotllanaBusy.value.contains(rotllanaId)) return
        val source = _conn.value; val epoch = companionEpoch
        rotllanaHistoryRequests[rotllanaId] = (rotllanaHistoryRequests[rotllanaId] ?: 0L) + 1
        val before = _rotllanaChat.value[rotllanaId].orEmpty()
        _rotllanaBusy.value = _rotllanaBusy.value + rotllanaId
        _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to (before + PetMsg("user", message, ts = System.currentTimeMillis())))
        viewModelScope.launch {
            try {
                try {
                    // Directe primer; el done porta l'historial amb veus.
                    var buf = ""
                    var streamSpeaker = ""
                    val (_, _, hist) = withContext(Dispatchers.IO) {
                        source.api().rotllanaChatStream(rotllanaId, message, wind) { e ->
                            if (!isCompanionOrigin(source, epoch)) return@rotllanaChatStream
                            when (e) {
                                is GregalApi.WindStreamEvent.Phase -> {
                                    val sp = e.phase.removePrefix("speaker:")
                                    if (sp != e.phase) {
                                        // Nova veu: la bombolla anterior queda com a missatge.
                                        commitRotllanaStreamBubble(rotllanaId, buf, streamSpeaker)
                                        buf = ""
                                        streamSpeaker = sp
                                        val who = _winds.value.firstOrNull { it.id == sp }?.name.orEmpty()
                                        setStreamingBubble(rotllanaId, streamPhaseText("speaker") + " " + who, rotllana = true)
                                    } else {
                                        buf = ""
                                        setStreamingBubble(rotllanaId, streamPhaseText(e.phase), rotllana = true)
                                    }
                                }
                                is GregalApi.WindStreamEvent.Token -> {
                                    buf += e.token
                                    setStreamingBubble(rotllanaId, buf, rotllana = true)
                                }
                                else -> {}
                            }
                        }
                    }
                    if (!isCompanionOrigin(source, epoch)) return@launch
                    dropStreamingBubble(rotllanaId, rotllana = true)
                    _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to hist.ifEmpty { before + PetMsg("pet", "") })
                } catch (_: Exception) {
                    dropStreamingBubble(rotllanaId, rotllana = true)
                    val (wind, text, hist) = withContext(Dispatchers.IO) { source.api().rotllanaChat(rotllanaId, message, wind) }
                    if (!isCompanionOrigin(source, epoch)) return@launch
                    val speaker = _winds.value.firstOrNull { it.id == wind }
                    _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to hist.ifEmpty {
                        before + PetMsg("user", message, ts = System.currentTimeMillis()) + PetMsg("pet", text, wind, speaker?.name.orEmpty())
                    })
                }
                // El torn pot haver programat avisos (remind): posa al dia
                // els jobs perquè l'alarma soni encara que tanquis l'app.
                if (isCompanionOrigin(source, epoch)) loadJobs()
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) {
                    notify(appText("app.status.company", e.message.orEmpty()))
                    _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to before)
                }
            } finally {
                if (isCompanionOrigin(source, epoch)) _rotllanaBusy.value = _rotllanaBusy.value - rotllanaId
            }
        }
    }

    /** Encén el cor d'un vent: un job que vigila el seu tema amb cerques web
     *  i parla només si troba novetats que valguin la pena. El vent es
     *  reprograma ell sol per quan acabi el proper esdeveniment del seu
     *  tema (partit, llançament…): el primer cop corre ara i ell decideix
     *  quan mirar altra vegada. L'id és wind-<vent>: el backend el tracta. */
    fun ensureWindHeartbeat(wind: Wind, on: Boolean) {
        viewModelScope.launch {
            try {
                val api = _conn.value.api()
                val jobId = "wind-" + wind.id
                val jobs = withContext(Dispatchers.IO) { api.getJobs() }
                val existing = jobs.firstOrNull { it.id == jobId }
                if (on && existing == null) {
                    withContext(Dispatchers.IO) {
                        api.saveJob(JobItem(
                            id = jobId,
                            name = appText("app.company.windHeartbeatName", wind.name),
                            prompt = "Cor de " + wind.name + ": vigila el tema «" + wind.role + "» i parla només si trobes novetats que valguin la pena.",
                            kind = "event", time = "",
                            enabled = true, notify = true,
                        ))
                        // Primera mirada ara: el mateix vent programa la següent.
                        api.runJob(jobId)
                    }
                } else if (!on && existing != null) {
                    withContext(Dispatchers.IO) { api.deleteJob(jobId) }
                }
                if (_conn.value.local) {
                    val current = withContext(Dispatchers.IO) { api.getJobs() }
                    JobAlarms.sync(appContext, current)
                }
            } catch (e: Exception) { notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    /** A failed suggestion must also finish the creator's pending state. */
    fun suggestWind(name: String, role: String, onError: () -> Unit = {}, onDone: (persona: String, style: String) -> Unit) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val (persona, style) = withContext(Dispatchers.IO) { source.api().suggestWind(name, role) }
                if (isCompanionOrigin(source, epoch)) onDone(persona, style)
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) { notify(appText("app.status.company", e.message.orEmpty())); onError() }
            }
        }
    }

    /** Memòria del vent sobre l'usuari + els seus coneixements (perfil). */
    private val _ventDetall = MutableStateFlow<Map<String, Pair<String, List<GregalApi.Coneixement>>>>(emptyMap())
    val ventDetall: StateFlow<Map<String, Pair<String, List<GregalApi.Coneixement>>>> = _ventDetall.asStateFlow()

    fun loadVentDetall(windId: String) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val mem = withContext(Dispatchers.IO) { source.api().getWindMemoria(windId) }
                val coneix = withContext(Dispatchers.IO) { source.api().listConeixements(windId) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _ventDetall.value = _ventDetall.value + (windId to (mem to coneix))
            } catch (_: Exception) { }
        }
    }

    fun saveVentMemoria(windId: String, memoria: String) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().saveWindMemoria(windId, memoria) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                val (oldMem, coneix) = _ventDetall.value[windId] ?: ("" to emptyList())
                _ventDetall.value = _ventDetall.value + (windId to (memoria to coneix))
            } catch (_: Exception) { }
        }
    }

    fun saveConeixement(windId: String, c: GregalApi.Coneixement, onDone: () -> Unit = {}) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val nom = withContext(Dispatchers.IO) { source.api().saveConeixement(windId, c) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                val (mem, coneix) = _ventDetall.value[windId] ?: ("" to emptyList())
                val nou = c.copy(nom = nom)
                _ventDetall.value = _ventDetall.value + (windId to (mem to (coneix.filterNot { it.nom == nom } + nou)))
                onDone()
            } catch (_: Exception) { }
        }
    }

    /** Llegeix un coneixement sencer (per a l'editor del perfil). */
    suspend fun apiConeixement(windId: String, nom: String): GregalApi.Coneixement =
        withContext(Dispatchers.IO) { _conn.value.api().getConeixement(windId, nom) }

    fun deleteConeixement(windId: String, nom: String) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().deleteConeixement(windId, nom) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                val (mem, coneix) = _ventDetall.value[windId] ?: ("" to emptyList())
                _ventDetall.value = _ventDetall.value + (windId to (mem to coneix.filterNot { it.nom == nom }))
            } catch (_: Exception) { }
        }
    }

    /** Buida el xat amb un vent (DELETE /api/winds/chat): el vent es queda. */
    fun clearWindChat(id: String) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().clearWindChat(id) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _windChat.value = _windChat.value + (id to emptyList())
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    /** Canvia el nom d'un vent (el xat es queda amb el mateix id). */
    fun renameWind(wind: Wind, name: String) {
        val clean = name.trim()
        if (clean.isEmpty() || clean == wind.name) return
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) {
                    source.api().saveWind(clean, wind.color, wind.role, wind.style, wind.persona, id = wind.id)
                }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _winds.value = _winds.value.map { if (it.id == wind.id) it.copy(name = clean) else it }
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    /** Canvia el títol d'una rotllana (mateix id i membres). */
    fun renameRotllana(rot: Rotllana, name: String) {
        val clean = name.trim()
        if (clean.isEmpty() || clean == rot.name) return
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().saveRotllana(clean, rot.members, id = rot.id) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _rotllanes.value = _rotllanes.value.map { if (it.id == rot.id) it.copy(name = clean) else it }
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    fun deleteWind(id: String) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().deleteWind(id) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                if (_activeWind.value?.id == id) _activeWind.value = null
                _windChat.value = _windChat.value - id
                if (_ventsOpen.value == "wind:$id") _ventsOpen.value = null
                loadWinds(); loadRotllanes()
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    fun setActiveWind(w: Wind?) { _activeWind.value = w }

    private val _windCreator = MutableStateFlow<Pair<Boolean, Wind?>>(false to null)
    val windCreator: StateFlow<Pair<Boolean, Wind?>> = _windCreator
    fun showWindCreator(show: Boolean, wind: Wind? = null) { _windCreator.value = show to wind }

    fun loadWindChat(windId: String) {
        val source = _conn.value; val epoch = companionEpoch
        if (_windBusy.value.contains(windId)) return
        val request = (windHistoryRequests[windId] ?: 0L) + 1
        windHistoryRequests[windId] = request
        viewModelScope.launch {
            try {
                val history = withContext(Dispatchers.IO) { source.api().getWindChat(windId) }
                if (isCompanionOrigin(source, epoch) && request == windHistoryRequests[windId] &&
                    !_windBusy.value.contains(windId))
                    _windChat.value = _windChat.value + (windId to history)
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    /**
     * Bombolla de directe: crea o actualitza l'últim missatge en streaming
     * del torn (fase o text parcial). El done la substitueix per la final.
     */
    private fun setStreamingBubble(windId: String, text: String, rotllana: Boolean = false) {
        val bubble = PetMsg("pet", text, streaming = true)
        if (rotllana) {
            val cur = _rotllanaChat.value[windId].orEmpty()
            _rotllanaChat.value = _rotllanaChat.value + (windId to
                if (cur.lastOrNull()?.streaming == true) cur.dropLast(1) + bubble else cur + bubble)
        } else {
            val cur = _windChat.value[windId].orEmpty()
            _windChat.value = _windChat.value + (windId to
                if (cur.lastOrNull()?.streaming == true) cur.dropLast(1) + bubble else cur + bubble)
        }
    }

    /** Tanca la bombolla en directe com a missatge del vent que parlava. */
    private fun commitRotllanaStreamBubble(rotllanaId: String, buf: String, speakerId: String) {
        if (buf.isBlank()) return
        val speaker = _winds.value.firstOrNull { it.id == speakerId }
        val cur = _rotllanaChat.value[rotllanaId].orEmpty().filterNot { it.streaming }
        _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to
            (cur + PetMsg("pet", buf, speakerId, speaker?.name.orEmpty())))
    }

    private fun dropStreamingBubble(windId: String, rotllana: Boolean = false) {
        if (rotllana) {
            val cur = _rotllanaChat.value[windId].orEmpty()
            if (cur.lastOrNull()?.streaming == true)
                _rotllanaChat.value = _rotllanaChat.value + (windId to cur.dropLast(1))
        } else {
            val cur = _windChat.value[windId].orEmpty()
            if (cur.lastOrNull()?.streaming == true)
                _windChat.value = _windChat.value + (windId to cur.dropLast(1))
        }
    }

    /** Fase del directe en la llengua de l'app. */
    private fun streamPhaseText(phase: String): String = when (phase) {
        "searching" -> appText("app.vents.streamSearching")
        "scheduling" -> appText("app.vents.streamScheduling")
        "thinking" -> appText("app.vents.streamThinking")
        "speaker" -> appText("app.vents.streamSpeaker")
        else -> appText("app.vents.streamWorking")
    }

    fun sendWindMessage(windId: String, text: String) {
        val msg = text.trim()
        if (msg.isEmpty() || _windBusy.value.contains(windId)) return
        val source = _conn.value; val epoch = companionEpoch
        windHistoryRequests[windId] = (windHistoryRequests[windId] ?: 0L) + 1
        _windChat.value = _windChat.value + (windId to (_windChat.value[windId].orEmpty() + PetMsg("user", msg, ts = System.currentTimeMillis())))
        _windBusy.value = _windBusy.value + windId
        viewModelScope.launch {
            try {
                try {
                    // Directe primer: el text creix a la bombolla; si el
                    // servidor no té /stream (o cau), clàssic de reserva.
                    var buf = ""
                    val r = withContext(Dispatchers.IO) {
                        source.api().windChatStream(windId, msg) { e ->
                            if (!isCompanionOrigin(source, epoch)) return@windChatStream
                            when (e) {
                                is GregalApi.WindStreamEvent.Phase -> {
                                    buf = ""
                                    setStreamingBubble(windId, streamPhaseText(e.phase))
                                }
                                is GregalApi.WindStreamEvent.Token -> {
                                    buf += e.token
                                    setStreamingBubble(windId, buf)
                                }
                                else -> {}
                            }
                        }
                    }
                    if (isCompanionOrigin(source, epoch)) {
                        dropStreamingBubble(windId)
                        _windChat.value = _windChat.value + (windId to (_windChat.value[windId].orEmpty() + PetMsg("pet", r.text, parcial = r.parcial)))
                        // Prova visible que el vent ha actuat de debò: només sona
                        // quan l'eina remind s'ha cridat (mai d'una promesa buida).
                        if (r.reminds.isNotEmpty()) notify(appText("app.vents.remindSet", r.reminds.joinToString(", ")))
                    }
                } catch (_: Exception) {
                    dropStreamingBubble(windId)
                    val r = withContext(Dispatchers.IO) { source.api().windChat(windId, msg) }
                    if (isCompanionOrigin(source, epoch)) {
                        _windChat.value = _windChat.value + (windId to (_windChat.value[windId].orEmpty() + PetMsg("pet", r.text, parcial = r.parcial)))
                        if (r.reminds.isNotEmpty()) notify(appText("app.vents.remindSet", r.reminds.joinToString(", ")))
                    }
                }
                // El vent pot haver programat un avís (remind): posa al dia
                // els jobs perquè l'alarma soni encara que tanquis l'app.
                if (isCompanionOrigin(source, epoch)) loadJobs()
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch))
                    notify(appText("app.status.company", e.message.orEmpty()))
            } finally {
                if (isCompanionOrigin(source, epoch)) _windBusy.value = _windBusy.value - windId
            }
        }
    }

    /**
     * «Continua»: reprèn l'última resposta parcial del vent pel mateix fil.
     * No afegeix cap missatge d'usuari: el servidor reprèn l'historial amb
     * pressupost nou. Si la continuació arriba sencera, el flag cau sol.
     */
    fun sendWindContinue(windId: String) {
        if (_windBusy.value.contains(windId)) return
        val source = _conn.value; val epoch = companionEpoch
        windHistoryRequests[windId] = (windHistoryRequests[windId] ?: 0L) + 1
        _windBusy.value = _windBusy.value + windId
        viewModelScope.launch {
            try {
                try {
                    var buf = ""
                    val r = withContext(Dispatchers.IO) {
                        source.api().windChatStream(windId, "", cont = true) { e ->
                            if (!isCompanionOrigin(source, epoch)) return@windChatStream
                            when (e) {
                                is GregalApi.WindStreamEvent.Phase -> {
                                    buf = ""
                                    setStreamingBubble(windId, streamPhaseText(e.phase))
                                }
                                is GregalApi.WindStreamEvent.Token -> {
                                    buf += e.token
                                    setStreamingBubble(windId, buf)
                                }
                                else -> {}
                            }
                        }
                    }
                    if (isCompanionOrigin(source, epoch)) {
                        dropStreamingBubble(windId)
                        // La parcial es fusiona amb l'anterior: un sol missatge.
                        val cur = _windChat.value[windId].orEmpty()
                        val idx = cur.indexOfLast { it.from == "pet" && !it.streaming }
                        val merged = if (idx >= 0) {
                            cur.toMutableList().also { it[idx] = it[idx].copy(text = it[idx].text + r.text, parcial = r.parcial) }
                        } else cur + PetMsg("pet", r.text, parcial = r.parcial)
                        _windChat.value = _windChat.value + (windId to merged)
                        if (r.reminds.isNotEmpty()) notify(appText("app.vents.remindSet", r.reminds.joinToString(", ")))
                    }
                } catch (_: Exception) {
                    dropStreamingBubble(windId)
                    val r = withContext(Dispatchers.IO) { source.api().windChat(windId, "", cont = true) }
                    if (isCompanionOrigin(source, epoch)) {
                        val cur = _windChat.value[windId].orEmpty()
                        val idx = cur.indexOfLast { it.from == "pet" && !it.streaming }
                        val merged = if (idx >= 0) {
                            cur.toMutableList().also { it[idx] = it[idx].copy(text = it[idx].text + r.text, parcial = r.parcial) }
                        } else cur + PetMsg("pet", r.text, parcial = r.parcial)
                        _windChat.value = _windChat.value + (windId to merged)
                        if (r.reminds.isNotEmpty()) notify(appText("app.vents.remindSet", r.reminds.joinToString(", ")))
                    }
                }
                if (isCompanionOrigin(source, epoch)) loadJobs()
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch))
                    notify(appText("app.status.company", e.message.orEmpty()))
            } finally {
                if (isCompanionOrigin(source, epoch)) _windBusy.value = _windBusy.value - windId
            }
        }
    }

    /**
     * «Continua» a la rotllana: reprèn l'última resposta del vent que la
     * va deixar a mitges (speakerId de la parcial). El servidor retorna
     * l'historial sencer amb el flag posat al dia.
     */
    fun sendRotllanaContinue(rotllanaId: String, wind: String = "") {
        if (_rotllanaBusy.value.contains(rotllanaId)) return
        val source = _conn.value; val epoch = companionEpoch
        rotllanaHistoryRequests[rotllanaId] = (rotllanaHistoryRequests[rotllanaId] ?: 0L) + 1
        _rotllanaBusy.value = _rotllanaBusy.value + rotllanaId
        viewModelScope.launch {
            try {
                try {
                    var buf = ""
                    val (_, _, hist) = withContext(Dispatchers.IO) {
                        source.api().rotllanaChatStream(rotllanaId, "", wind, cont = true) { e ->
                            if (!isCompanionOrigin(source, epoch)) return@rotllanaChatStream
                            when (e) {
                                is GregalApi.WindStreamEvent.Phase -> {
                                    buf = ""
                                    setStreamingBubble(rotllanaId, streamPhaseText(e.phase), rotllana = true)
                                }
                                is GregalApi.WindStreamEvent.Token -> {
                                    buf += e.token
                                    setStreamingBubble(rotllanaId, buf, rotllana = true)
                                }
                                else -> {}
                            }
                        }
                    }
                    if (!isCompanionOrigin(source, epoch)) return@launch
                    dropStreamingBubble(rotllanaId, rotllana = true)
                    _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to hist.ifEmpty { _rotllanaChat.value[rotllanaId].orEmpty() })
                } catch (_: Exception) {
                    dropStreamingBubble(rotllanaId, rotllana = true)
                    val (_, _, hist) = withContext(Dispatchers.IO) { source.api().rotllanaChat(rotllanaId, "", wind, cont = true) }
                    if (isCompanionOrigin(source, epoch))
                        _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to hist.ifEmpty { _rotllanaChat.value[rotllanaId].orEmpty() })
                }
                if (isCompanionOrigin(source, epoch)) loadJobs()
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) notify(appText("app.status.company", e.message.orEmpty()))
            } finally {
                if (isCompanionOrigin(source, epoch)) _rotllanaBusy.value = _rotllanaBusy.value - rotllanaId
            }
        }
    }

    private val _pets = MutableStateFlow<List<Pet>>(emptyList())
    val pets: StateFlow<List<Pet>> = _pets

    private val _petCheckins = MutableStateFlow<Map<String, PetCheckin>>(emptyMap())
    val petCheckins: StateFlow<Map<String, PetCheckin>> = _petCheckins

    fun loadPets() {
        viewModelScope.launch {
            try {
                _pets.value = withContext(Dispatchers.IO) { _conn.value.api().getPets() }
            } catch (e: Exception) { notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    // Xat amb mascota: missatges en memòria + enviament
    data class PetMsg(val from: String, val text: String, val speakerId: String = "", val speakerName: String = "", val streaming: Boolean = false, val parcial: Boolean = false, val ts: Long = 0L)

    private val _petChat = MutableStateFlow<Map<String, List<PetMsg>>>(emptyMap())
    val petChat: StateFlow<Map<String, List<PetMsg>>> = _petChat

    private val _petChatBusy = MutableStateFlow<Set<String>>(emptySet())
    val petChatBusy: StateFlow<Set<String>> = _petChatBusy

    fun loadPetChat(petId: String) {
        viewModelScope.launch {
            try {
                val history = withContext(Dispatchers.IO) { _conn.value.api().getPetChat(petId) }
                _petChat.value = _petChat.value + (petId to history)
            } catch (e: Exception) { notify(appText("app.status.company", e.message.orEmpty())) }
        }
    }

    fun sendPetMessage(petId: String, text: String) {
        val msg = text.trim()
        if (msg.isEmpty()) return
        viewModelScope.launch {
            _petChat.value = _petChat.value + (petId to (_petChat.value[petId].orEmpty() + PetMsg("user", msg, ts = System.currentTimeMillis())))
            _petChatBusy.value = _petChatBusy.value + petId
            try {
                val reply = withContext(Dispatchers.IO) { _conn.value.api().petChat(petId, msg) }
                _petChat.value = _petChat.value + (petId to (_petChat.value[petId].orEmpty() + PetMsg("pet", reply)))
            } catch (e: Exception) {
                _petChat.value = _petChat.value + (petId to (_petChat.value[petId].orEmpty() + PetMsg("pet", appText("app.status.company", e.message.orEmpty()))))
            } finally {
                _petChatBusy.value = _petChatBusy.value - petId
            }
        }
    }

    fun triggerPetCheckin(petId: String) {
        viewModelScope.launch {
            try {
                val c = withContext(Dispatchers.IO) { _conn.value.api().petCheckin(petId) }
                _petCheckins.value = _petCheckins.value + (petId to c)
            } catch (e: Exception) { notify(appText("app.status.company", e.message.orEmpty())) }
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
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val jobs = loadingOf("jobs") { withContext(Dispatchers.IO) { source.api().getJobs() } }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _jobs.value = jobs
                if (source.local) JobAlarms.sync(appContext, jobs)
                _bgHealth.value = if (source.local) JobAlarms.health(appContext) else null
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) notify(appText("app.status.jobs", e.message.orEmpty())) }
        }
    }

    /**
     * En local, l'alarma del sistema és qui dispara els jobs: cada cop que
     * l'app es connecta posa al dia la programació (i, si se n'ha perdut
     * alguna franja, l'alarma sona de seguida).
     */
    private suspend fun syncJobAlarms() {
        if (!_conn.value.local) return
        try {
            val list = withContext(Dispatchers.IO) { _conn.value.api().getJobs() }
            JobAlarms.sync(appContext, list)
        } catch (_: Exception) {
        }
    }

    // Execució a obrir (s'ha tocat la notificació d'un job).
    private val _openRun = MutableStateFlow<String?>(null)
    val openRun: StateFlow<String?> = _openRun

    fun openRunFromNotification(id: String) {
        _openRun.value = id
        if (_connected.value) go(Screen.Jobs)
    }

    fun openRunShown() { _openRun.value = null }

    /** Després de connectar: la pantalla de la notificació tocada o, per
     *  defecte, els vents — són la cara de l'app, no un calaix. */
    private fun landing(): Screen = if (_openRun.value != null) Screen.Jobs else Screen.Company

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
