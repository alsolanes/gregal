package xyz.solanes.gregal

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
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
enum class Screen { Pairing, Chat, Sessions, Goals, Jobs, Office, Flows, Automatitzacions, Desat, Notes, Prefs, Retro, Company, Work, Ventet, Carpeta, Preview, Webs }

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
    private val ventsRecency = VentsRecency(app.getSharedPreferences("vents-observed-receipts", 0))
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

    /**
     * L'última preview que l'agent ha obert amb l'eina preview i si es veu.
     * Abans el mòbil la ignorava: el treball deia «Previsualitza» i no
     * s'obria res.
     */
    private val _preview = MutableStateFlow<PreviewReq?>(null)
    val preview: StateFlow<PreviewReq?> = _preview
    private val _previewOpen = MutableStateFlow(false)
    val previewOpen: StateFlow<Boolean> = _previewOpen
    /** Arguments de les crides preview en curs: el registre durable retalla la sortida. */
    private val previewArgs = mutableMapOf<String, String>()
    fun showPreview(open: Boolean) { _previewOpen.value = open && _preview.value != null }
    fun clearPreview() { _previewOpen.value = false; _preview.value = null; _published.value = PublishState() }

    /**
     * Publica la preview al servidor amb un enllaç que obre qualsevol, sense
     * l'app ni el token (share_site.go). Tornar a publicar fa servir el mateix
     * id: l'enllaç es queda, el contingut passa a ser el d'ara.
     */
    private val _published = MutableStateFlow(PublishState())
    val published: StateFlow<PublishState> = _published
    fun showPublished(open: Boolean) { _published.value = _published.value.copy(dialog = open, error = if (open) _published.value.error else "") }
    fun publishPreview() {
        val req = _preview.value?.takeIf { it.kind != "url" } ?: return
        val cur = _published.value
        if (cur.busy) return
        _published.value = cur.copy(busy = true, error = "", dialog = true)
        viewModelScope.launch {
            _published.value = try {
                val site = withContext(Dispatchers.IO) { publishPreviewSite(_conn.value, req, cur.site?.id.orEmpty()) }
                PublishState(site = site, dialog = true)
            } catch (e: CancellationException) { throw e } catch (e: Exception) {
                cur.copy(busy = false, error = requestMessage(e), dialog = true)
            }
        }
    }
    fun unpublishPreview() {
        val site = _published.value.site ?: return
        _published.value = _published.value.copy(busy = true, error = "")
        viewModelScope.launch {
            _published.value = try {
                withContext(Dispatchers.IO) { _conn.value.api().deleteShare(site.id) }
                PublishState()
            } catch (e: CancellationException) { throw e } catch (e: Exception) {
                _published.value.copy(busy = false, error = requestMessage(e))
            }
        }
    }

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
    private val _programats = MutableStateFlow<WindsProgramats?>(null)
    val programats: StateFlow<WindsProgramats?> = _programats
    private val _programatsError = MutableStateFlow<String?>(null)
    val programatsError: StateFlow<String?> = _programatsError
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
    private val otaUrl: String get() = androidOtaUrl(appContext.packageName)

    fun checkUpdate(silent: Boolean = false) {
        if (otaUrl.isBlank()) return
        viewModelScope.launch {
            if (!silent) _updateMsg.value = appText("app.update.checking")
            val actual = versionCodeInstalada()
            val res = withContext(Dispatchers.IO) {
                // Primer la caché (6 h): no truquem el servidor a cada
                // arrencada, i si la xarxa falla encara serveix.
                val cau = llegeixCache(appContext.cacheDir, System.currentTimeMillis(), abisDelMobil())
                if (cau != null && cau.hiHaNova(actual) && isTrustedDownloadUrl(otaUrl, cau.downloadUrl)) {
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
                        text = fetchTextAbsolut(otaUrl)
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
                if (!manifestPlausible(info) || !isTrustedDownloadUrl(otaUrl, info.downloadUrl)) {
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
                    appContext, r.downloadUrl, otaUrl,
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
    /** Quin tema (paleta) vol l'usuari: gregal, nit, calma, paper o grafit. */
    private val _temaApp = MutableStateFlow("gregal")
    val temaApp: StateFlow<String> = _temaApp
    private val _idioma = MutableStateFlow("en")
    val idioma: StateFlow<String> = _idioma
    private val _idiomaChoice = MutableStateFlow("system")
    val idiomaChoice: StateFlow<String> = _idiomaChoice
    private val _expressiveMotion = MutableStateFlow(true)
    val expressiveMotion: StateFlow<Boolean> = _expressiveMotion

    /**
     * L'arrencada (reconnexió automàtica + comprovar actualitzacions) ja s'ha
     * fet amb aquest ViewModel: girar la pantalla recrea l'activitat però no
     * l'ha de repetir (ni treure't al Pairing si la reconnexió fallés).
     */
    var startupDone = false

    /** Enllaça el magatzem de prefs (tema/idioma/esborrany desats). */
    fun attachPrefs(p: Prefs) {
        prefsStore = p
        // Com es diuen els vents en aquest mòbil: si la persona els ha posat un
        // altre nom, tots els textos en fan servir el mot.
        VentNom.singular = p.ventSingular
        VentNom.plural = p.ventPlural
        _capHome.value = p.capHome
        _tema.value = p.tema
        _temaApp.value = p.temaApp
        _idioma.value = p.idioma
        _idiomaChoice.value = p.idiomaChoice
        _expressiveMotion.value = p.expressiveMotion
        _simpleVents.value = p.simpleVents
        _usuari.value = p.usuari
        if (p.esborrany.isNotEmpty()) _draft.value = p.esborrany
        // A partir d'aquí totes les peticions van a la sessió del mòbil, no a
        // la `default` (la pestanya «principal» de la web).
        sessioApp = p.appSession.ifBlank { "mobil" }
        // Un 401 de la connexió d'ara vol dir que la sessió ja no val
        // (contrasenya canviada, compte esborrat, sessió tancada): cap a entrar.
        GregalApi.onUnauthorized = { base, token -> viewModelScope.launch { sessionExpired(base, token) } }
    }

    /** Desa com es diuen els vents (buit = els noms de sempre). */
    fun desaNomsDelVent(singular: String, plural: String) {
        prefsStore?.desaNomsDelVent(singular, plural)
        prefsStore?.collectiveAsked = true
        _askCollective.value = false
        _suggestedNames.value = emptyList()
        // El servidor també ho ha de saber: els noms que proposa (creador,
        // colles) s'hi adapten.
        val c = _conn.value
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { c.api().setWindCollective(singular.trim(), plural.trim()) }
                if (_conn.value == c) loadSuggestedNames()
            } catch (e: Exception) { if (e is CancellationException) throw e }
        }
    }

    private val _askCollective = MutableStateFlow(false)
    /** Cal preguntar (un sol cop) com es diuen els ajudants. */
    val askCollective: StateFlow<Boolean> = _askCollective.asStateFlow()

    /** «Ara no»: no es torna a preguntar; es pot canviar a la configuració. */
    fun skipCollective() {
        prefsStore?.collectiveAsked = true
        _askCollective.value = false
        notify(appText("collective.later"))
        // El servidor també ho recorda: la web no tornarà a preguntar.
        val c = _conn.value
        viewModelScope.launch {
            try { withContext(Dispatchers.IO) { c.api().setWindCollective(VentNom.singular, VentNom.plural) } }
            catch (e: Exception) { if (e is CancellationException) throw e }
        }
    }

    /**
     * En connectar: el nom triat al mòbil mana i el servidor l'aprèn; si al
     * mòbil no n'hi ha i al servidor sí (un altre dispositiu), s'adopta.
     * Si no n'hi ha enlloc i encara no s'ha preguntat, es pregunta.
     */
    private suspend fun syncCollective() {
        val p = prefsStore ?: return
        val c = _conn.value
        try {
            val server = withContext(Dispatchers.IO) { c.api().getWindCollective() }
            when {
                // El servidor ja ho sap (triat aquí, a la web o a un altre mòbil): mana ell.
                server.asked -> {
                    if (server.plural != VentNom.plural || server.singular != VentNom.singular) p.desaNomsDelVent(server.singular, server.plural)
                    p.collectiveAsked = true
                }
                // Triat en aquest mòbil abans que el servidor ho sabés: l'hi diem.
                VentNom.plural.isNotBlank() ->
                    withContext(Dispatchers.IO) { c.api().setWindCollective(VentNom.singular, VentNom.plural) }
            }
        } catch (e: Exception) { if (e is CancellationException) throw e }
        if (VentNom.plural.isNotBlank()) p.collectiveAsked = true
        _askCollective.value = !p.collectiveAsked
    }

    private val _suggestedNames = MutableStateFlow<List<String>>(emptyList())
    /** Noms proposats al creador, al gust del nom col·lectiu (buit = els de sempre). */
    val suggestedNames: StateFlow<List<String>> = _suggestedNames.asStateFlow()

    fun loadSuggestedNames() {
        val c = _conn.value
        viewModelScope.launch {
            try {
                val names = withContext(Dispatchers.IO) { c.api().getWindNames(WindSuggestions.all.size, _idioma.value) }
                if (_conn.value == c) _suggestedNames.value = names
            } catch (e: Exception) { if (e is CancellationException) throw e }
        }
    }

    fun setTema(v: String) {
        _tema.value = v
        if (::prefsStore.isInitialized) prefsStore.tema = v
    }

    private val _simpleVents = MutableStateFlow(false)
    /** Vents net: el menú i el xat només ensenyen el que és dels vents. */
    val simpleVents: StateFlow<Boolean> = _simpleVents
    fun setSimpleVents(v: Boolean) {
        _simpleVents.value = v
        if (::prefsStore.isInitialized) prefsStore.simpleVents = v
    }

    private val _ventsPanel = MutableStateFlow<String?>(null)
    /** Un panell de la portada dels Vents que s'ha demanat des de fora (el calaix):
     *  "directory" (els teus vents) o "explore". La portada l'obre i el buida. */
    val ventsPanel: StateFlow<String?> = _ventsPanel
    fun openVentsPanel(panel: String?) {
        _ventsPanel.value = panel
        if (panel != null) go(Screen.Company)
    }

    fun setExpressiveMotion(v: Boolean) {
        _expressiveMotion.value = v
        if (::prefsStore.isInitialized) prefsStore.expressiveMotion = v
    }

    /** Canvia la paleta sense tocar el mode clar/fosc. */
    fun setTemaApp(v: String) {
        _temaApp.value = v
        if (::prefsStore.isInitialized) prefsStore.temaApp = v
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
    /** El Ventet a dalt dels Vents (a Preferències). Per defecte té secció pròpia. */
    private val _capHome = MutableStateFlow(false)
    val capHome: StateFlow<Boolean> = _capHome

    fun setCapHome(v: Boolean) {
        if (::prefsStore.isInitialized) prefsStore.capHome = v
        _capHome.value = v
    }

    private val _ventsOpen = MutableStateFlow<String?>(null)
    val ventsOpen: StateFlow<String?> = _ventsOpen
    /** El Ventet és una conversa dels Vents: la secció del menú i els avisos hi porten. */
    fun showVentet() {
        _ventsOpen.value = "cap"
        if (_screen.value == Screen.Ventet) _screen.value = Screen.Company else go(Screen.Company)
    }

    fun openVentsChat(id: String?) {
        // El que es pinta en el mateix fotograma del toc: la insígnia baixa i,
        // si el fil encara no és a la mà, l'esquelet. Sense això la pantalla
        // ensenyava un estat buit i els missatges apareixien de cop.
        if (id == null) {
            _ventsOpen.value = null
            return
        }
        val bare = id.substringAfter(':')
        if (id.startsWith("wind:")) {
            markWindNewsSeen(bare)
            if (_windChat.value[bare].isNullOrEmpty()) _windChatLoading.value = _windChatLoading.value + bare
        } else if (id.startsWith("rot:")) {
            if (_rotllanaChat.value[bare].isNullOrEmpty()) _rotllanaChatLoading.value = _rotllanaChatLoading.value + bare
        }
        _ventsOpen.value = id
        if (id.startsWith("rot:")) followCrew(bare)
        // Tocar una notificació ha d'obrir la conversa que l'ha provocada: si
        // només desàvem el vent, l'app es quedava a la pantalla on era i
        // semblava que no fes res.
        if (_connected.value) go(Screen.Company)
    }

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
    private val selectedWindThreads = mutableMapOf<String, String>()
    private val windHistoryRequests = mutableMapOf<String, Long>()
    private val rotllanaHistoryRequests = mutableMapOf<String, Long>()
    private fun isCompanionOrigin(source: Conn, epoch: Long) =
        _conn.value == source && companionEpoch == epoch

    private fun resetServerState() {
        companionEpoch++
        expertPolling.values.toList().forEach { it.cancel() }
        expertPolling.clear()
        crewPolling.values.toList().forEach { it.cancel() }
        crewPolling.clear()
        _crewStep.value = emptyMap()
        windHistoryRequests.clear()
        selectedWindThreads.clear()
        rotllanaHistoryRequests.clear()
        _winds.value = emptyList()
        _offline.value = false
        probeJob?.cancel()
        probeJob = null
        _outbox.value = emptyList()
        _automations.value = emptyMap()
        _saved.value = emptyMap()
        _notes.value = null
        _openNote.value = null
        notesQuery = NotesQuery()
        _turnActions.value = emptyMap()
        _suggestedNames.value = emptyList()
        _windChat.value = emptyMap()
        _windBusy.value = emptySet()
        _windLive.value = emptyMap()
        _lastWindReply.value = null
        _windNews.value = emptySet()
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
        libraryRequest++
        _windLibrary.value = WindLibraryState(generation = companionEpoch)
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
        _wikiPages.value = null
        _wikiError.value = null
        _wikiBusy.value = false
        _rutines.value = null
        _rutinesError.value = null
        _programats.value = null
        _programatsError.value = null
        _plugins.value = emptyMap()
        _windsMeta.value = GregalApi.WindsMeta(emptyList(), emptyList())
        _windExpert.value = emptyMap()
        _ventDetall.value = emptyMap()
        _windsResum.value = emptyMap()
        _summaries.value = emptyMap()
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
        _windTasks.value = null
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
        // Feina en marxa al servidor: el vigilant hi torna en 3 minuts per
        // avisar quan acabi (sense, esperava la passada normal de 15).
        if (!_conn.value.local && _windTasks.value.orEmpty().any { it.running }) {
            WindWatch.schedule(appContext, WindWatch.SOON_MS)
        }
        // Els sondeigs d'estat (agent lliure, expert, colla) viuen al
        // viewModelScope i sobrevivien al segon pla: amb l'app tancada la
        // ràdio seguia picant cada 2-3 s. Aquí s'aturen; en tornar, la
        // primera passada de cada pantalla els reactiva.
        pauseBackgroundPolling()
    }

    /** Atura els sondeigs que només tenen sentit amb l'app a primer pla. */
    private fun pauseBackgroundPolling() {
        waiter?.cancel(); waiter = null
        expertPolling.values.toList().forEach { it.cancel() }
        expertPolling.clear()
        crewPolling.values.toList().forEach { it.cancel() }
        crewPolling.clear()
    }

    /**
     * En tornar a primer pla després d'haver aturat els sondeigs: una
     * passada immediata de feines i veus noves (els sondeigs de pantalla
     * ja es reactiven sols amb LifecyclePoll).
     */
    fun onForegrounded() {
        if (_connected.value && !_conn.value.local) {
            loadWindTasks()
            refreshWindNews()
        }
    }

    /** Switching views must not kill an in-flight phone turn or its embedded server. */
    private fun workPreventsOriginSwitch(): Boolean = _sending.value || _windBusy.value.isNotEmpty() ||
        _rotllanaBusy.value.isNotEmpty() || (_conn.value.local && _windTasks.value.orEmpty().any { it.running })

    private fun warnOriginBusy() = notify(if (_idioma.value == "en")
        "A turn is still running here. Finish it before switching where it runs."
        else "Aquí encara hi ha una feina en marxa. Acaba-la abans de canviar on s'executa.")

    /** Fes servir Gregal en aquest dispositiu (el mode per defecte). */
    fun useThisDevice(retries: Int = 3) {
        if (!_conn.value.local && workPreventsOriginSwitch()) { warnOriginBusy(); return }
        if (!_conn.value.local) resetServerState()
        prefsStore?.target = Prefs.TARGET_LOCAL
        // En local els jobs els dispara l'app; el vigilant segueix el
        // servidor desat (si n'hi ha) i cada avís porta el seu origen.
        WindWatch.schedule(appContext)
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
        if (workPreventsOriginSwitch()) { warnOriginBusy(); return }
        // La clau no és de cap compte: el nom d'un compte d'abans no hi pinta res.
        _usuari.value = ""
        prefsStore?.save(host, port, token, "")
        useRemote(Conn(host, port, token, ""))
        connect { go(Screen.Company) }
    }

    /** Reprèn la connexió desada en obrir l'app. */
    fun startSaved(p: Prefs) {
        when {
            p.target == Prefs.TARGET_LOCAL && localAvailable() -> useThisDevice(retries = 3)
            // Sense token (has sortit o la sessió ha caducat) toca entrar de nou.
            p.host.isNotBlank() && p.token.isNotBlank() -> {
                setConn(Conn(p.host, p.port, p.token, p.usuari))
                WindWatch.schedule(appContext)
                connect(retries = 8) { go(landing()) }
            }
        }
    }

    /** Torna a la pantalla de connexió per triar un altre servidor. */
    fun switchServer() = disconnect()

    private val _connections = MutableStateFlow<GregalApi.Connections?>(null)
    /** Calendari, correu i resum del matí d'aquest compte (null mentre carrega). */
    val connections: StateFlow<GregalApi.Connections?> = _connections
    private val _connectionsMsg = MutableStateFlow("")
    val connectionsMsg: StateFlow<String> = _connectionsMsg

    /**
     * Aprovacions i preguntes que els vents esperen (flux de la conversa +
     * /api/winds/pending). El torn del vent està aturat fins que respons.
     */
    private val _windAsks = MutableStateFlow<List<GregalApi.WindAsk>>(emptyList())
    val windAsks: StateFlow<List<GregalApi.WindAsk>> = _windAsks.asStateFlow()

    internal fun handleWindAskEvent(e: GregalApi.WindStreamEvent) {
        when (e) {
            is GregalApi.WindStreamEvent.Ask -> _windAsks.value = _windAsks.value.filter { it.key != e.ask.key } + e.ask
            is GregalApi.WindStreamEvent.AskResolved -> _windAsks.value = _windAsks.value.filter { it.key != e.key }
            is GregalApi.WindStreamEvent.DoneRotllana -> if (e.failed.isNotEmpty()) {
                notify(appText("wind.turn.incomplete", e.failed.joinToString(", ")))
            }
            else -> {}
        }
    }

    /** Tornar a trobar el que espera (en obrir un xat, o mentre un vent treballa). */
    fun loadWindAsks() {
        val c = _conn.value
        if (c.host.isBlank() && !c.local) return // sense servidor: res a preguntar
        viewModelScope.launch {
            try { _windAsks.value = withContext(Dispatchers.IO) { c.api().getWindAsks() } }
            catch (e: Exception) { if (e is CancellationException) throw e }
        }
    }

    /** Respon: aprovar o rebutjar, o triar una opció (o escriure-la). Desbloqueja el torn. */
    fun answerWindAsk(key: String, approve: Boolean = false, answer: String = "") {
        val c = _conn.value
        val wind = _windAsks.value.firstOrNull { it.key == key }?.wind.orEmpty()
        _windAsks.value = _windAsks.value.filter { it.key != key }
        viewModelScope.launch {
            try { withContext(Dispatchers.IO) { c.api().answerWindAsk(key, approve, answer) } }
            catch (e: Exception) { if (e is CancellationException) throw e; notify(e.message.orEmpty()); return@launch }
            if ((approve || answer.isNotBlank()) && wind.isNotBlank() && wind !in _windBusy.value) watchResumedWind(wind)
        }
    }

    /** Vents amb un torn reprès al servidor després d'una decisió (B4): el fil diu que treballen. */
    private val _windResuming = MutableStateFlow<Set<String>>(emptySet())
    val windResuming: StateFlow<Set<String>> = _windResuming

    /** El torn aparcat continua al servidor, fora d'aquest xat: el fil ensenya que treballa i,
     *  quan arriba el missatge (la resposta o una altra pregunta), el llegeix i deixa de dir-ho. */
    private fun watchResumedWind(windId: String) {
        if (windId in _windResuming.value) return
        _windResuming.value = _windResuming.value + windId
        val before = _windChat.value[windId]?.size ?: 0
        viewModelScope.launch {
            try {
                repeat(360) {
                    delay(2500)
                    if (windId in _windBusy.value) return@launch
                    loadWindChat(windId)
                    val now = _windChat.value[windId].orEmpty()
                    if (now.size > before && now.lastOrNull()?.from != "user") { loadWindAsks(); return@launch }
                }
            } finally { _windResuming.value = _windResuming.value - windId }
        }
    }

    /** «Webs i apps»: el que els vents han publicat, el que corre al servidor i els túnels. */
    private val _webs = MutableStateFlow(WebsState())
    internal val webs: StateFlow<WebsState> = _webs.asStateFlow()

    fun loadWebs() {
        val c = _conn.value
        viewModelScope.launch {
            try {
                val items = withContext(Dispatchers.IO) {
                    val api = c.api()
                    // Cada part per separat: els túnels són només de l'administrador (403 si no).
                    val shares = runCatching { api.get("/api/shares") }.getOrNull()
                    val apps = runCatching { api.get("/api/apps") }.getOrNull()
                    val powers = runCatching { api.get("/api/server/powers") }.getOrNull()
                    if (shares == null && apps == null && powers == null) throw java.io.IOException("no server")
                    parseWebs(api.base, shares, apps, powers)
                }
                _webs.value = WebsState(items, loaded = true)
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                _webs.value = _webs.value.copy(loaded = true, error = e.message.orEmpty())
            }
        }
    }

    /** unpublish (una web), restart/stop/remove (una app). */
    internal fun webAction(item: WebItem, action: String) {
        val c = _conn.value
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) {
                    val api = c.api()
                    when {
                        item.kind == "site" && action == "unpublish" -> api.deleteShare(item.id)
                        item.kind == "app" -> api.post("/api/apps", JSONObject().put("action", action).put("id", item.id))
                        else -> Unit
                    }
                }
                loadWebs()
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                notify(e.message.orEmpty())
            }
        }
    }

    /** El resum del matí d'avui, si ja ha sortit: va a dalt d'«Avui» a l'inici dels Vents. */
    data class TodayBrief(val windId: String, val windName: String, val at: String, val text: String)
    private val _todayBrief = MutableStateFlow<TodayBrief?>(null)
    val todayBrief: StateFlow<TodayBrief?> = _todayBrief.asStateFlow()

    fun loadTodayBrief() {
        val c = _conn.value
        viewModelScope.launch {
            _todayBrief.value = try {
                withContext(Dispatchers.IO) {
                    val rut = c.api().getWindRutines().firstOrNull { it.id.startsWith("briefing-") && it.enabled } ?: return@withContext null
                    val (started, text) = c.api().getWindRutinaLastOutput(rut.id) ?: return@withContext null
                    val at = runCatching { java.time.OffsetDateTime.parse(started).atZoneSameInstant(java.time.ZoneId.systemDefault()) }.getOrNull()
                        ?: return@withContext null
                    if (at.toLocalDate() != java.time.LocalDate.now()) return@withContext null
                    TodayBrief(rut.wind, rut.windName, "%02d:%02d".format(at.hour, at.minute), text)
                }
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                null
            }
        }
    }

    // --- «Avui»: la safata dels vents (VentsAvui.kt) ---
    private val _avui = MutableStateFlow<AvuiView?>(null)
    val avui: StateFlow<AvuiView?> = _avui.asStateFlow()
    private val _avuiError = MutableStateFlow(false)
    val avuiError: StateFlow<Boolean> = _avuiError.asStateFlow()

    fun loadAvui() {
        val c = _conn.value
        viewModelScope.launch {
            try {
                val v = withContext(Dispatchers.IO) { c.api().avui() }
                if (_conn.value == c) { _avui.value = v; _avuiError.value = false }
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                if (_conn.value == c) _avuiError.value = true
            }
        }
    }

    /**
     * Una decisió sobre una targeta. La targeta canvia d'estat al moment (la
     * safata l'anima fora) i, si el servidor no ho accepta, torna on era.
     */
    fun avuiAct(id: String, action: String) {
        val c = _conn.value
        val before = _avui.value ?: return
        _avui.value = AvuiLogic.apply(before, id, action)
        viewModelScope.launch {
            val ok = try { withContext(Dispatchers.IO) { c.api().avuiAction(id, action) } } catch (e: Exception) {
                if (e is CancellationException) throw e
                false
            }
            if (!ok && _conn.value == c) _avui.value = AvuiLogic.revert(_avui.value ?: before, before, id)
        }
    }

    /** Una acció de la targeta: obre la conversa amb el vent i li diu el que demana. */
    fun avuiRun(card: AvuiCard, action: AvuiAction) {
        if (card.wind.isBlank()) return
        openVentsChat("wind:" + card.wind)
        if (action.prompt.isNotBlank()) sendWindMessage(card.wind, action.prompt)
    }

    /** Desa hores tranquil·les, mode tranquil i (si no és null) el que ha de tenir en compte el resum de demà. */
    fun saveAvuiSettings(next: AvuiSettings, tune: String? = null) {
        val c = _conn.value
        val before = _avui.value
        if (before != null) _avui.value = before.copy(settings = next.copy(tune = tune ?: before.settings.tune))
        viewModelScope.launch {
            try {
                val saved = withContext(Dispatchers.IO) { c.api().saveAvuiSettings(next, tune) }
                if (_conn.value == c) _avui.value?.let { _avui.value = it.copy(settings = saved) }
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                if (_conn.value == c && before != null) _avui.value?.let { _avui.value = it.copy(settings = before.settings) }
            }
        }
    }

    fun loadConnections() {
        val c = _conn.value
        viewModelScope.launch {
            _connections.value = try { withContext(Dispatchers.IO) { c.api().getConnections() } } catch (e: Exception) {
                if (e is CancellationException) throw e
                null
            }
        }
    }

    /** Desa un canvi de connexions i torna a llegir l'estat; l'error es mostra a la targeta. */
    fun saveConnections(body: org.json.JSONObject, okText: String) {
        val c = _conn.value
        _connectionsMsg.value = "…"
        viewModelScope.launch {
            _connectionsMsg.value = try {
                withContext(Dispatchers.IO) { c.api().postConnections(body) }
                _connections.value = withContext(Dispatchers.IO) { c.api().getConnections() }
                okText
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                "⚠ " + e.message.orEmpty().substringAfter(": ").take(240)
            }
        }
    }

    fun testConnections() {
        val c = _conn.value
        _connectionsMsg.value = "…"
        viewModelScope.launch {
            _connectionsMsg.value = try { withContext(Dispatchers.IO) { c.api().testConnections() }.ifBlank { "—" } } catch (e: Exception) {
                if (e is CancellationException) throw e
                "⚠ " + e.message.orEmpty().take(240)
            }
        }
    }

    private val _serverPowers = MutableStateFlow<GregalApi.ServerPowers?>(null)
    /** Poders de servidor (només l'admin): null mentre no se sap o si no n'hi ha. */
    val serverPowers: StateFlow<GregalApi.ServerPowers?> = _serverPowers

    fun loadServerPowers() {
        val c = _conn.value
        viewModelScope.launch {
            _serverPowers.value = try { withContext(Dispatchers.IO) { c.api().getServerPowers() } } catch (e: Exception) {
                if (e is CancellationException) throw e
                null
            }
        }
    }

    fun setServerPowers(enabled: Boolean? = null, notes: String? = null, onDone: (Boolean) -> Unit = {}) {
        val c = _conn.value
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { c.api().setServerPowers(enabled, notes) }
                _serverPowers.value = withContext(Dispatchers.IO) { c.api().getServerPowers() }
                onDone(true)
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                companyError(e); onDone(false)
            }
        }
    }

    /** Crea un codi d'invitació (només l'admin d'un servidor amb comptes). */
    fun createInvitation(onCode: (String) -> Unit) {
        val c = _conn.value
        viewModelScope.launch {
            try {
                val code = withContext(Dispatchers.IO) { c.api().createInvitation() }
                if (code.isNotBlank()) onCode(code) else notify(appText("app.invite.failed", ""))
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                notify(appText("app.invite.failed", if ((e.message ?: "").contains("404")) appText("app.invite.notConfigured") else e.message.orEmpty().take(120)))
            }
        }
    }

    /** Hi ha un compte (usuari i contrasenya) obert en aquest servidor. */
    fun signedInWithAccount(): Boolean = !_conn.value.local && _conn.value.user.isNotBlank()

    /**
     * Surt del compte: tanca la sessió al servidor i oblida el token en
     * aquest mòbil. L'adreça i el nom es queden per tornar a entrar d'un toc.
     */
    fun signOut() {
        val c = _conn.value
        if (c.local || workPreventsOriginSwitch()) { if (!c.local) warnOriginBusy(); return }
        viewModelScope.launch(Dispatchers.IO) { runCatching { c.api().logout() } }
        prefsStore?.save(c.host, c.port, "", c.user)
        disconnect()
        _signInReason.value = appText("app.server.signedOut")
    }

    private val _signInReason = MutableStateFlow("")
    /**
     * Per què tornes a la pantalla d'entrar (has sortit, la sessió ha caducat).
     * No va pel snackbar: la pantalla que marxa se l'enduia abans de mostrar-lo.
     */
    val signInReason: StateFlow<String> = _signInReason

    private fun sessionExpired(base: String, token: String) {
        val c = _conn.value
        if (c.local || !_connected.value || c.token != token || baseUrlOf(c.host, c.port) != base) return
        prefsStore?.save(c.host, c.port, "", c.user)
        disconnect()
        _signInReason.value = appText("app.server.sessionExpired")
    }

    // --- Mòbil o servidor, d'un toc ---

    // Les preferències encara no hi són en els tests de pantalla (i abans de
    // MainActivity): el selector no ha de petar per això.
    private val prefsOrNull: Prefs? get() = if (::prefsStore.isInitialized) prefsStore else null

    private val _serverReach = MutableStateFlow<Boolean?>(null)
    /** Si el servidor desat respon (null = no n'hi ha cap o encara no se sap). */
    val serverReach: StateFlow<Boolean?> = _serverReach
    private val _switchingWhere = MutableStateFlow(false)
    val switchingWhere: StateFlow<Boolean> = _switchingWhere

    /** Hi ha un servidor desat amb credencials (encara que ara siguis al mòbil). */
    fun hasSavedServer(): Boolean = prefsOrNull?.let { it.host.isNotBlank() && it.token.isNotBlank() } == true

    /** El nom curt del servidor desat («vents.exemple.cat»), per al selector. */
    fun savedServerName(): String = displayServerName(prefsOrNull?.host.orEmpty())

    /** Comprova (en 4 s com a molt) si el servidor desat respon amb la sessió desada. */
    fun checkServerReach() {
        val p = prefsOrNull ?: return
        if (!hasSavedServer()) { _serverReach.value = null; return }
        if (!_conn.value.local && _connected.value && !_offline.value) { _serverReach.value = true; return }
        viewModelScope.launch {
            val ok = withContext(Dispatchers.IO) { pingServer(p.host, p.port, p.token) }
            _serverReach.value = ok
            if (ok && _offline.value) retryServerNow()
        }
    }

    /** El servidor contesta alguna cosa (encara que sigui un 401) en pocs segons. */
    private fun serverAnswers(host: String, port: String): Boolean = try {
        val c = java.net.URL(baseUrlOf(host, port) + "/api/health").openConnection() as java.net.HttpURLConnection
        c.connectTimeout = 4_000
        c.readTimeout = 4_000
        try { c.responseCode !in setOf(502, 503, 504) } finally { c.disconnect() }
    } catch (_: Exception) { false }

    private fun pingServer(host: String, port: String, token: String): Boolean = try {
        val c = java.net.URL(baseUrlOf(host, port) + "/api/me").openConnection() as java.net.HttpURLConnection
        c.connectTimeout = 4_000
        c.readTimeout = 4_000
        c.setRequestProperty("Authorization", "Bearer $token")
        try { c.responseCode == 200 } finally { c.disconnect() }
    } catch (_: Exception) { false }

    /**
     * Canvia d'un toc on viuen els vents: aquest mòbil o el servidor desat.
     * Abans calia obrir el menú, «Canvia de servidor» i tornar a escriure les
     * dades. Si el servidor no respon, no es marxa del mòbil: s'avisa i prou.
     * Sense servidor desat, s'obre la pantalla per afegir-ne un.
     */
    fun switchWhere(toServer: Boolean) {
        if (_switchingWhere.value || toServer == !_conn.value.local) return
        if (workPreventsOriginSwitch()) { warnOriginBusy(); return }
        if (!toServer) {
            useThisDevice()
            return
        }
        val p = prefsOrNull ?: return
        if (!hasSavedServer()) {
            p.target = Prefs.TARGET_REMOTE
            disconnect()
            return
        }
        _switchingWhere.value = true
        viewModelScope.launch {
            try {
                val ok = withContext(Dispatchers.IO) { pingServer(p.host, p.port, p.token) }
                _serverReach.value = ok
                if (!ok) {
                    notify(appText("app.where.serverDown", savedServerName()))
                    return@launch
                }
                useRemote(Conn(p.host, p.port, p.token, p.usuari))
                connect(retries = 2) { go(Screen.Company) }
            } finally {
                _switchingWhere.value = false
            }
        }
    }

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

    /**
     * Afegeix un proveïdor nou al servidor (nom + URL + clau). En mode
     * servidor el proveïdor viu a la configuració del servidor: aquí només
     * s'hi envia l'ordre i es refresca el que es veu.
     */
    fun addProvider(nom: String, url: String, key: String, onOk: () -> Unit = {}) {
        if (_providerSetupBusy.value) return
        val problema = problemaProveidor(nom, url)
        if (problema != null) {
            _provMsg.value = appText(problema)
            return
        }
        if (key.isBlank()) {
            _provMsg.value = appText("app.setup.errKey")
            return
        }
        val c = _conn.value
        _providerSetupBusy.value = true
        _provMsg.value = ""
        viewModelScope.launch {
            try {
                val msg = withContext(Dispatchers.IO) {
                    if (_conn.value != c) return@withContext ""
                    c.api().addProvider(nom, url, key)
                }
                if (_conn.value != c) return@launch
                _provMsg.value = msg.ifBlank { appText("app.setup.ready") }
                refresh()
                loadProviders()
                loadCatalog()
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

    private val _registering = MutableStateFlow(false)
    val registering: StateFlow<Boolean> = _registering
    fun registerAccount(host: String, port: String, code: String, user: String, password: String) {
        if (_registering.value) return
        _registering.value = true
        viewModelScope.launch {
            try {
                val api = GregalApi(baseUrlOf(host, port), "")
                withContext(Dispatchers.IO) { api.post("/api/register", org.json.JSONObject().put("code", code.trim()).put("name", user.trim()).put("password", password)) }
                notify(if (_idioma.value == "ca") "Compte creat. Entrant…" else "Account created. Signing in…")
                entra(host, port, user, password)
            } catch (e: Exception) {
                val needsNew = e.message.orEmpty().contains("request a new invitation")
                notify(if (needsNew) { if (_idioma.value == "ca") "No s’ha pogut crear el compte. Demana un codi nou a l’administrador." else "Could not create the account. Ask the administrator for a new code." } else { if (_idioma.value == "ca") "No s’ha pogut crear el compte: comprova el codi i les dades." else "Could not create the account: check the invitation and details." })
            } finally { _registering.value = false }
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
            // Amb còpia al mòbil, els vents surten de seguida (abans: la portada buida
            // «comença amb una colla» uns segons) i el servidor els posa al dia.
            _conn.value.takeIf { !it.local && _winds.value.isEmpty() }?.let { src ->
                withContext(Dispatchers.IO) { offlineStore(src)?.windsRaw() }?.let { if (_conn.value == src && _winds.value.isEmpty()) showCachedWinds(it) }
            }
            while (true) {
                try {
                    if (_conn.value.local) ensureLocal()
                    // Amb còpia al mòbil, un servidor que no contesta no fa esperar
                    // tres temps d'espera seguits (45 s): una prova curta i, si res, la còpia.
                    val src = _conn.value
                    if (!src.local && offlineStore(src)?.windsRaw() != null &&
                        !withContext(Dispatchers.IO) { serverAnswers(src.host, src.port) }) {
                        enterOffline(src, onOk)
                        return@launch
                    }
                    connectOnce()
                    syncAgentLanguage()
                    onOk()
                    // No bloquegen la portada: els vents es carreguen ja.
                    launch { syncCollective() }
                    launch { syncJobAlarms() }
                    _offline.value = false
                    withContext(Dispatchers.IO) { reloadOutbox() }
                    loadWinds()
                    loadRotllanes()
                    _ventsOpen.value?.takeIf { it.startsWith("rot:") }?.let { followCrew(it.substringAfter(':')) }
                    if (!_conn.value.local) { loadAutomations(""); flushOutbox() }
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
                    // Servidor que no respon però amb còpia al mòbil: s'obre amb
                    // la còpia i es reprova sol (abans: pantalla de connexió).
                    val src = _conn.value
                    if (!src.local && isUnreachable(e) && offlineStore(src)?.windsRaw() != null) {
                        enterOffline(src, onOk)
                        return@launch
                    }
                    val offline = (e.message ?: "").let { it.contains("failed to connect", true) || it.contains("timed out", true) || it.contains("refused", true) }
                    if (left-- > 0 && offline) {
                        notify(appText("app.work.reconnecting"))
                        delay(2500)
                        continue
                    }
                    _connected.value = false
                    // La sessió desada ja no val (compte esborrat, contrasenya canviada):
                    // cap a entrar, amb el motiu a la vista i sense el token mort.
                    if ((e.message ?: "").contains("401") && !_conn.value.local && _conn.value.user.isNotBlank()) {
                        prefsStore?.save(_conn.value.host, _conn.value.port, "", _conn.value.user)
                        _signInReason.value = appText("app.server.sessionExpired")
                        go(Screen.Pairing)
                        return@launch
                    }
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
        // Les tres peticions són independents: en paral·lel, l'arrencada
        // espera la més lenta en lloc de la suma de les tres.
        val (health, admin, st) = coroutineScope {
            val health = async(Dispatchers.IO) { try { api.health() } catch (_: Exception) { null } }
            // Qui ets en aquest servidor: un compte que no és admin no pot veure
            // els jobs ni GitHub, i l'app no els ha de demanar (sortia un error).
            // Un servidor antic sense /api/me és d'un sol usuari: admin.
            val admin = async(Dispatchers.IO) { try { api.me().admin } catch (_: Exception) { true } }
            val st = async(Dispatchers.IO) { api.getState() }
            Triple(health.await(), admin.await(), st.await())
        }
        _v2Runs.value = health?.supportsV2Interactive() == true
        _isAdmin.value = admin
        _state.value = st
        // Obrir l'app ha de mostrar la conversa on eres, no un xat buit.
        if (_msgs.value.isEmpty() && st.transcript.isNotEmpty()) _msgs.value = st.transcript
        _connected.value = true
        _signInReason.value = ""
        clearNotice()
        attachToRunning()
    }

    private val _isAdmin = MutableStateFlow(true)
    /** Aquest compte administra el servidor (en local i amb la clau, sempre). */
    val isAdmin: StateFlow<Boolean> = _isAdmin

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
            } else if (_queue.value.isNotEmpty() && !stopRequested) {
                val next = _queue.value.first()
                _queue.value = _queue.value.drop(1)
                send(next)
            } else if (stopRequested) _queue.value = emptyList()
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
            val read = withContext(Dispatchers.IO) { _conn.value.api().officeRead(up.id) }
            _office.value = read.copy(path = up.path, kind = read.kind.ifBlank { up.kind })
        }
    }

    /** Adjunta un document (PDF o Office) al xat: el puja i torna la ruta
     *  perquè el compositor hi posi «@ruta». El servidor n'afegeix el text en
     *  enviar i l'agent el pot llegir sencer (pdf_read) o editar (pdf_edit). */
    fun attachDocument(uri: Uri, onReady: (String) -> Unit) {
        var name = "document"
        try {
            appContext.contentResolver.query(uri, null, null, null, null)?.use { c ->
                val i = c.getColumnIndex(android.provider.OpenableColumns.DISPLAY_NAME)
                if (c.moveToFirst() && i >= 0) name = c.getString(i) ?: name
            }
        } catch (_: Exception) {}
        if (!isOfficeName(name)) { notify(appText("app.off.fileTypeError", name)); return }
        viewModelScope.launch {
            when (val r = readUriBase64(appContext, uri, 25L * 1024 * 1024)) {
                is UriRead.Ok -> {
                    notify(appText("app.composer.docUploading", name))
                    val up = withContext(Dispatchers.IO) { _conn.value.api().officeUpload(name, r.base64) }
                    if (up.error.isNotBlank() || up.path.isBlank()) {
                        notify(appText("app.composer.docError", up.error.ifBlank { name }))
                        return@launch
                    }
                    _office.value = up.copy(result = "")
                    notify(appText("app.composer.docAdded", up.name.ifBlank { name }))
                    onReady(up.path)
                }
                is UriRead.TooLarge -> notify(appText("app.off.fileTooLarge", 25,
                    if (r.size > 0) r.size / 1024 / 1024 else 25))
                UriRead.Unreadable -> notify(appText("app.off.fileReadError"))
                is UriRead.Failed -> notify(appText("app.off.fileOpenError", r.message))
            }
        }
    }

    /**
     * Una foto del mòbil: puja-la pel camí dels documents i deixa al camp la
     * imatge (perquè la vegis al xat) seguida de la ruta (perquè el vent pugui
     * obrir el fitxer amb les eines).
     */
    fun attachImage(uri: Uri, onReady: (String) -> Unit) {
        var name = "imatge.jpg"
        try {
            appContext.contentResolver.query(uri, null, null, null, null)?.use { c ->
                val i = c.getColumnIndex(android.provider.OpenableColumns.DISPLAY_NAME)
                if (c.moveToFirst() && i >= 0) name = c.getString(i) ?: name
            }
        } catch (_: Exception) {}
        if (!isImageName(name)) name = "imatge-" + System.currentTimeMillis() + ".jpg"
        viewModelScope.launch {
            when (val r = readUriBase64(appContext, uri, 20L * 1024 * 1024)) {
                is UriRead.Ok -> {
                    notify(appText("app.composer.imageUploading", name))
                    // HEIC → JPEG al mòbil mateix (també val en mode local, sense servidor).
                    val jpeg = withContext(Dispatchers.Default) { heicToJpegBase64(r.base64) }
                    val data = jpeg ?: r.base64
                    if (jpeg != null) name = name.substringBeforeLast('.') + ".jpg"
                    val up = withContext(Dispatchers.IO) { _conn.value.api().officeUpload(name, data) }
                    if (up.error.isNotBlank() || up.path.isBlank() || up.id.isBlank()) {
                        notify(appText("app.composer.docError", up.error.ifBlank { name }))
                        return@launch
                    }
                    notify(appText("app.composer.imageAdded", up.name.ifBlank { name }))
                    val label = up.name.ifBlank { name }
                    onReady("![" + label + "](gregal-image:" + up.id + ") " + appendDocMention("", up.path).trim())
                }
                is UriRead.TooLarge -> notify(appText("app.off.fileTooLarge", 20,
                    if (r.size > 0) r.size / 1024 / 1024 else 20))
                UriRead.Unreadable -> notify(appText("app.off.fileReadError"))
                is UriRead.Failed -> notify(appText("app.off.fileOpenError", r.message))
            }
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

    /** Un enllaç de document d'una resposta (el PDF que ha fet un vent):
     *  es baixa amb la clau i es desa a Descàrregues. */
    fun downloadServerDoc(url: String, label: String = "") {
        val source = _conn.value
        val name = serverDocName(url, label.ifBlank { "document" })
        notify(appText("app.composer.docDownloading", name))
        viewModelScope.launch {
            val msg = try {
                val bytes = withContext(Dispatchers.IO) { source.api().downloadServerDoc(url) }
                withContext(Dispatchers.IO) { saveToDownloads(appContext, name, bytes, idioma.value) }
            } catch (e: Exception) {
                appText("app.off.downloadWriteError", e.message.orEmpty())
            }
            notify(msg)
        }
    }

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
    /**
     * El raonament va al panell de treball. El text que ja s'ha vist, NO: es
     * queda a la conversa. Abans s'esborrava el missatge (i el que s'estava
     * escrivint) quan el model encara havia de cridar una eina, i es veia com
     * si la resposta s'esborrés sola a mitges.
     */
    private fun narrationToWork() {
        flushTokens()
        val live = _live.value.trim()
        _live.value = ""
        if (live.isNotEmpty()) step(StepKind.Note, oneLine(live, 140), detail = live)
        // El proper text serà un missatge nou, no continuació del que ja s'ha vist.
        answerId = 0L
    }

    /** El text que s'està escrivint es tanca com a missatge abans de les eines. */
    private fun settleLiveAsMessage() {
        flushTokens()
        val live = _live.value.trim()
        if (live.isEmpty()) return
        _live.value = ""
        val m = ChatMsg("assistant", live)
        answerId = m.id
        push(m)
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
                settleLiveAsMessage()
                val name = f("name").ifEmpty { text.removePrefix("🔧").trim() }
                val args = o.opt("args")?.toString().orEmpty()
                val sum = summarizeTool(name, args, lang)
                val id = f("id").ifEmpty { "c${nextMsgId()}" }
                updateWork { it.add(WorkStep(id, StepKind.Tool, sum.label, sum.target, sum.meta, "", StepState.Running, name)) }
                if (name == "preview") previewArgs[id] = args
                setActivity("tool:$id", sum.label + if (sum.target.isNotEmpty()) " · " + oneLine(sum.target, 48) else "")
            }
            "tool_result" -> {
                val out = f("output").ifEmpty { text.substringAfter(": ", "") }
                updateWork { it.finishTool(f("id"), f("name"), out, toolOutputFailed(out)) }
                if (f("name") == "preview") {
                    val args = previewArgs.remove(f("id"))
                    if (!toolOutputFailed(out)) (parsePreviewRequest(out) ?: parsePreviewRequest(args))?.let {
                        _preview.value = it
                        _previewOpen.value = true
                    }
                }
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
                _question.value = QuestionReq(f("key"), f("call_id"), f("query"), parseQuestionOptions(o.optJSONArray("options")?.toString()))
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
    /**
     * Canvia el mode. «act» torna a l'últim mode d'Actua amb el seu permís
     * (el servidor el recorda); la resta són ids interns (chat, plan, goal…).
     */
    fun setMode(mode: String) {
        // Optimista: el composer canvia a l'instant; refresh() ho confirma.
        val cur = _state.value
        val optimistic = when {
            mode != "act" -> mode
            uiModeOf(cur.mode) == "act" -> cur.mode
            cur.access == "full" -> "autonomous"
            else -> "code"
        }
        _state.value = cur.copy(mode = optimistic)
        simplePost("/api/mode", JSONObject().put("mode", mode))
    }

    /** Permís d'Actua (com la web i el TUI): ask, edits o full. Hi passa. */
    fun setAccess(access: String) {
        val cur = _state.value
        val mode = if (access == "full") "autonomous" else if (cur.mode == "debug") "debug" else "code"
        _state.value = cur.copy(access = access, mode = mode, permissive = access == "full")
        simplePost("/api/access", JSONObject().put("access", access))
    }

    /**
     * El menú de permisos de Codi (com la web): «Només lectura» passa a mode
     * chat; un altre permís des de només lectura passa primer a act i després
     * desa el permís, en ordre.
     */
    fun pickPermission(pick: String) {
        val steps = permSteps(_state.value.mode, pick)
        if (steps.none { it.first == "access" }) { setMode("chat"); return }
        val cur = _state.value
        val mode = if (pick == "full") "autonomous" else if (cur.mode == "debug") "debug" else "code"
        _state.value = cur.copy(access = pick, mode = mode, permissive = pick == "full")
        val c = _conn.value
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) {
                    val api = c.api()
                    steps.forEach { (k, v) -> api.post(if (k == "mode") "/api/mode" else "/api/access", JSONObject().put(k, v)) }
                }
                refresh()
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                notify(appText("app.status.error", e.message.orEmpty()))
            }
        }
    }

    /** Revisor de permisos (Preferències): no toca el permís d'Actua. */
    fun setReviewer(on: Boolean) {
        val cur = _state.value
        _state.value = cur.copy(autoReview = on)
        simplePost("/api/permissive", JSONObject().put("on", cur.permissive && !on).put("review", on))
    }

    /** Envia segons el composer: a Planifica, primer el pla.
     *  Si l'agent ja treballa, el missatge li arriba ara (steer). */
    fun submit(text: String, interrupt: Boolean = false) {
        when {
            // Amb l'agent treballant, escriure posa el missatge a la cua (com a la web amb
            // Alt+Enter); toca'l o prem «Ara» per dir-li ja (steer). Mantenir premut interromp.
            _sending.value && interrupt && workId != 0L -> steer(text, true)
            _sending.value -> enqueue(text)
            _state.value.mode == "plan" -> requestPlan(text)
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

    /** Cua de missatges escrits mentre l'agent treballa: surten sols en acabar el torn. */
    private val _queue = MutableStateFlow<List<String>>(emptyList())
    val queue: StateFlow<List<String>> = _queue

    fun enqueue(text: String) {
        val t = text.trim()
        if (t.isEmpty()) return
        _queue.value = _queue.value + t
    }

    fun removeQueued(index: Int) {
        _queue.value = _queue.value.filterIndexed { i, _ -> i != index }
    }

    /** «Ara»: el missatge de la cua passa a ser un steer del torn en curs. */
    fun steerQueued(index: Int) {
        val t = _queue.value.getOrNull(index) ?: return
        if (!_sending.value || workId == 0L) return // sense torn viu es queda a la cua
        removeQueued(index)
        steer(t, false)
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
    /** Nivell de raonament (think): el servidor el desa per usuari. */
    fun setThink(level: String) {
        val cur = _state.value
        _state.value = cur.copy(think = if (level == "default") "" else level)
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { _conn.value.api().post("/api/think", JSONObject().put("level", level)) }
                _state.value = withContext(Dispatchers.IO) { _conn.value.api().getState() }
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                _state.value = cur
                notify(requestMessage(e))
            }
        }
    }

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
    // La tasca del pla: «Executa el pla» l'envia al davant perquè l'agent
    // d'Actua sàpiga què li van demanar (com al web).
    private var planTask = ""

    /** Demana un pla read-only (/api/plan): explora sense actuar. */
    fun requestPlan(task: String) {
        val t = task.trim()
        if (t.isEmpty() || _planning.value || _sending.value) return
        if (_state.value.agentBusy) {
            notify(appText("app.status.planBusy"))
            return
        }
        _planning.value = true
        planTask = t
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

    /** Executa el pla: passa a Actua (amb el seu permís) i fa el torn d'agent. */
    fun runPlan() {
        val p = _plan.value ?: return
        _plan.value = null
        setMode("act")
        val task = planTask.also { planTask = "" }
        val prompt = if (task.isBlank()) p else task + "\n\n" + appText("app.runPlanTask") + "\n" + p
        send(prompt, forceAgent = true)
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
            } catch (e: Exception) { companyError(e) }
        }
    }

    fun acceptFact(fact: String) {
        viewModelScope.launch {
            try {
                _company.value = withContext(Dispatchers.IO) { _conn.value.api().companyAccept(fact) }
            } catch (e: Exception) { companyError(e) }
        }
    }

    fun dismissFacts() {
        viewModelScope.launch {
            try {
                _company.value = withContext(Dispatchers.IO) { _conn.value.api().companyDismiss() }
            } catch (e: Exception) { companyError(e) }
        }
    }

    fun forgetCompany(term: String) {
        viewModelScope.launch {
            try {
                _company.value = withContext(Dispatchers.IO) { _conn.value.api().companyForget(term) }
                notify(appText("app.company.forgotten"))
            } catch (e: Exception) { companyError(e) }
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
            } catch (e: Exception) { companyError(e) }
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
            } catch (e: Exception) { companyError(e) }
        }
    }

    // ---- Mascotes del company ----

    // ---- Vents: companions personals amb nom de vent ----
    private val _winds = MutableStateFlow<List<Wind>>(emptyList())
    val winds: StateFlow<List<Wind>> = _winds

    private val _windChat = MutableStateFlow<Map<String, List<PetMsg>>>(emptyMap())
    val windChat: StateFlow<Map<String, List<PetMsg>>> = _windChat
    /**
     * Fils que s'estan demanant al servidor sense cap missatge a la mà. És el
     * que permet obrir un xat amb l'esquelet en lloc d'un «comença la
     * conversa» fals que desapareix mig segon després.
     */
    private val _windChatLoading = MutableStateFlow<Set<String>>(emptySet())
    val windChatLoading: StateFlow<Set<String>> = _windChatLoading

    /** Sessions (fils) de cada vent: llista + la que és oberta. */
    private val _windSessions = MutableStateFlow<Map<String, List<WindSessionInfo>>>(emptyMap())
    val windSessions: StateFlow<Map<String, List<WindSessionInfo>>> = _windSessions
    private val _windSessionsLoading = MutableStateFlow<Set<String>>(emptySet())
    val windSessionsLoading: StateFlow<Set<String>> = _windSessionsLoading

    private val _windBusy = MutableStateFlow<Set<String>>(emptySet())
    val windBusy: StateFlow<Set<String>> = _windBusy

    /** Torn en directe per xat (id del vent o de la rotllana): la cara en mostra l'estat. */
    private val _windLive = MutableStateFlow<Map<String, WindLive>>(emptyMap())
    val windLive: StateFlow<Map<String, WindLive>> = _windLive
    private fun live(id: String, change: (WindLive) -> WindLive) {
        // Cada token del streaming passa per aquí. Si només canviava l'hora de
        // l'últim esdeveniment, publicar-ho feia recompondre tot el xat a cada
        // fragment: l'hora només serveix per a «sense novetats des de fa N s»,
        // i n'hi ha prou de refrescar-la un cop per segon.
        val now = System.currentTimeMillis()
        val cur = _windLive.value[id]
        val next = change(cur ?: WindLive()).copy(lastEventAt = now)
        val same = cur != null && cur == next.copy(lastEventAt = cur.lastEventAt)
        if (!same || now - cur!!.lastEventAt >= 1_000) _windLive.value = _windLive.value + (id to next)
        val target = if (id in _rotllanaBusy.value) "rot:$id" else "wind:$id"
        GregalNotifications.workUpdated(appContext, target, windProgressLabel(_idioma.value, next))
    }
    private fun endLive(id: String) { _windLive.value = _windLive.value - id }

    private val _lastWindReply = MutableStateFlow<Pair<String, Long>?>(null)
    internal val lastWindReply = _lastWindReply.asStateFlow()

    private fun endWindReply(id: String, complete: Boolean) {
        if (complete) _lastWindReply.value = id to System.currentTimeMillis()
        _windBusy.value = _windBusy.value - id
        endLive(id)
    }

    /** Vents amb una veu proactiva que encara no has obert (cor, recordatoris). */
    private val _windNews = MutableStateFlow<Set<String>>(emptySet())
    val windNews: StateFlow<Set<String>> = _windNews
    /** Quants missatges pendents té cada vent: la insígnia diu quants, no només que n'hi ha. */
    private val _windNewsCount = MutableStateFlow<Map<String, Int>>(emptyMap())
    val windNewsCount: StateFlow<Map<String, Int>> = _windNewsCount
    private val newsPrefs get() = getApplication<Application>().getSharedPreferences("gregal_winds_seen", android.content.Context.MODE_PRIVATE)

    fun refreshWindNews() {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val prefs = newsPrefs
                var floor = prefs.getLong("floor", 0L)
                if (floor == 0L) {
                    floor = System.currentTimeMillis() / 1000 - WindNewsLogic.FIRST_RUN_WINDOW
                    prefs.edit().putLong("floor", floor).apply()
                }
                val list = withContext(Dispatchers.IO) { source.api().windNovetats(floor) }
                val seen = prefs.all.mapNotNull { (k, v) -> if (k.startsWith("w:") && v is Long) k.removePrefix("w:") to v else null }.toMap()
                if (isCompanionOrigin(source, epoch)) {
                    _windNews.value = WindNewsLogic.unseen(list, seen, floor)
                    _windNewsCount.value = WindNewsLogic.unseenCounts(list, seen, floor)
                }
            } catch (_: Exception) {
                // Sense servidor o en mode local: cap novetat, cap soroll.
            }
        }
    }

    fun markWindNewsSeen(id: String) {
        newsPrefs.edit().putLong("w:$id", System.currentTimeMillis() / 1000).apply()
        _windNews.value = _windNews.value - id
        _windNewsCount.value = _windNewsCount.value - id
    }

    /** El vent actiu al botó flotant (null = cap). */
    private val _activeWind = MutableStateFlow<Wind?>(null)
    val activeWind: StateFlow<Wind?> = _activeWind

    private var windsRequest = 0L

    // --- El servidor mana, el mòbil fa de reserva (VentsOffline.kt) ---

    private val _offline = MutableStateFlow(false)
    /** El servidor no respon: el que es veu és la còpia del mòbil. */
    val offline: StateFlow<Boolean> = _offline.asStateFlow()
    private val _offlineCopyAt = MutableStateFlow(0L)
    /** Quan es va desar la còpia que es mostra (ms). */
    val offlineCopyAt: StateFlow<Long> = _offlineCopyAt.asStateFlow()
    private val _outbox = MutableStateFlow<List<OutboxItem>>(emptyList())
    /** Missatges escrits sense connexió que esperen el servidor. */
    val outbox: StateFlow<List<OutboxItem>> = _outbox.asStateFlow()
    private var probeJob: Job? = null
    private var netCallback: android.net.ConnectivityManager.NetworkCallback? = null

    private fun offlineStore(source: Conn = _conn.value): VentsOfflineStore? =
        if (source.local || source.host.isBlank()) null else VentsOffline.store(appContext, NotificationOrigins.of(source).id)

    private fun reloadOutbox(source: Conn = _conn.value) { _outbox.value = offlineStore(source)?.outbox().orEmpty() }

    private fun showCachedWinds(raw: String) {
        val list = try { Wind.parse(raw) } catch (_: Exception) { return }
        _winds.value = list
        _activeWind.value = list.firstOrNull { it.id == _activeWind.value?.id } ?: list.firstOrNull()
    }

    private fun cacheChat(source: Conn, windId: String) {
        val store = offlineStore(source) ?: return
        val msgs = _windChat.value[windId].orEmpty()
        viewModelScope.launch(Dispatchers.IO) { store.saveChat(windId, msgs) }
    }

    /** Obrir l'app sense servidor però amb còpia: dins, amb la còpia, i reprovant sol. */
    private fun enterOffline(source: Conn, onOk: () -> Unit) {
        val store = offlineStore(source) ?: return
        _connected.value = true
        _signInReason.value = ""
        clearNotice()
        reloadOutbox(source)
        store.windsRaw()?.let { showCachedWinds(it) }
        goOffline(source)
        onOk()
        _ventsOpen.value?.takeIf { it.startsWith("wind:") }?.let { loadWindChat(it.substringAfter(':')) }
    }

    /**
     * Error d'una crida als vents. Si és que el servidor no respon, no és un
     * error per ensenyar (sortia «failed to connect … after 15000ms»): l'app
     * passa a la còpia del mòbil i la xapa ja ho diu.
     */
    private fun companyError(e: Exception) {
        val source = _conn.value
        if (!source.local && (isUnreachable(e) || _offline.value)) { goOffline(source); return }
        notify(appText("app.status.company", e.message.orEmpty()))
    }

    private fun goOffline(source: Conn) {
        if (source.local || _conn.value != source) return
        if (!_offline.value) {
            _offline.value = true
            _serverReach.value = false
            _offlineCopyAt.value = offlineStore(source)?.savedAt() ?: 0L
        }
        startProbe(source)
    }

    /** Reprova el servidor: de seguida i després cada cop més espaiat (fins a 1 min). */
    private fun startProbe(source: Conn) {
        if (probeJob?.isActive == true) return
        watchNetwork()
        probeJob = viewModelScope.launch {
            var wait = 8_000L
            while (_offline.value && _conn.value == source) {
                delay(wait)
                val ok = withContext(Dispatchers.IO) { pingServer(source.host, source.port, source.token) }
                if (ok && backOnline(source)) return@launch
                wait = minOf(wait * 2, 60_000L)
            }
        }
    }

    /** «Torna-ho a provar» o la xarxa ha tornat: prova ara, sense esperar la propera passada. */
    fun retryServerNow() {
        val source = _conn.value
        if (source.local || !_offline.value) return
        probeJob?.cancel()
        probeJob = viewModelScope.launch {
            val ok = withContext(Dispatchers.IO) { pingServer(source.host, source.port, source.token) }
            if (ok && backOnline(source)) return@launch
            probeJob = null
            startProbe(source)
        }
    }

    private fun watchNetwork() {
        if (netCallback != null) return
        val cm = appContext.getSystemService(android.net.ConnectivityManager::class.java) ?: return
        val cb = object : android.net.ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: android.net.Network) {
                viewModelScope.launch { delay(1_500); retryServerNow() }
            }
        }
        try { cm.registerDefaultNetworkCallback(cb); netCallback = cb } catch (_: Exception) {}
    }

    override fun onCleared() {
        netCallback?.let { cb ->
            try { appContext.getSystemService(android.net.ConnectivityManager::class.java)?.unregisterNetworkCallback(cb) } catch (_: Exception) {}
        }
        netCallback = null
        super.onCleared()
    }

    /** El servidor torna: estat fresc, la llista, el xat obert i la cua surt. */
    private suspend fun backOnline(source: Conn): Boolean {
        if (_conn.value != source) return true
        try { connectOnce() } catch (e: CancellationException) { throw e } catch (_: Exception) { return false }
        _offline.value = false
        _serverReach.value = true
        syncAgentLanguage()
        syncJobAlarms()
        loadWinds()
        loadRotllanes()
        _ventsOpen.value?.takeIf { it.startsWith("wind:") }?.let { loadWindChat(it.substringAfter(':')) }
        loadAutomations("")
        reloadOutbox(source)
        val waiting = _outbox.value.size
        flushOutbox()
        notify(if (waiting > 0) appText("app.offline.backSending", waiting) else appText("app.offline.back"))
        return true
    }

    private fun enqueueOffline(source: Conn, windId: String, text: String, ts: Long) {
        val store = offlineStore(source) ?: return
        val items = OfflineLogic.enqueue(store.outbox(), OutboxItem(windId, text, ts))
        store.saveOutbox(items)
        _outbox.value = items
        _windChat.value = _windChat.value + (windId to OfflineLogic.withQueued(_windChat.value[windId].orEmpty(), items, windId))
    }

    /** Treu un missatge de la cua abans que surti. */
    fun discardQueued(windId: String, ts: Long) {
        val store = offlineStore() ?: return
        val items = store.outbox().filterNot { it.wind == windId && it.ts == ts }
        store.saveOutbox(items)
        _outbox.value = items
        _windChat.value = _windChat.value + (windId to _windChat.value[windId].orEmpty().filterNot { it.kind == "queued" && it.ts == ts })
    }

    /** Envia el primer de la cua de cada vent lliure; en acabar cada torn, el següent. */
    private fun flushOutbox() {
        val source = _conn.value
        if (source.local || _offline.value) return
        val store = offlineStore(source) ?: return
        for (item in OfflineLogic.nextToSend(store.outbox(), _windBusy.value)) {
            val rest = store.outbox().filterNot { it.wind == item.wind && it.ts == item.ts }
            store.saveOutbox(rest)
            _outbox.value = rest
            _windChat.value = _windChat.value + (item.wind to _windChat.value[item.wind].orEmpty().filterNot { it.kind == "queued" && it.ts == item.ts })
            sendWindMessage(item.wind, item.text, item.ts)
        }
    }

    /**
     * Els recordatoris del servidor desat també els vigila el mòbil: si a
     * l'hora el servidor no respon, l'avís surt igualment (WindWatch).
     */
    private fun mirrorReminders(source: Conn, fresh: List<MirroredReminder>, wind: String?) {
        if (source.local) return
        val store = offlineStore(source) ?: return
        if (NotificationOrigins.remote(Prefs(appContext))?.id != NotificationOrigins.of(source).id) return
        val merged = OfflineLogic.mergeReminders(store.reminders(), fresh, wind)
        store.saveReminders(merged)
        WindWatch.expect(appContext, merged.map { it.at })
    }

    private fun forgetMirrored(source: Conn, id: String) {
        val store = offlineStore(source) ?: return
        store.saveReminders(store.reminders().filterNot { it.id == id })
    }

    fun loadWinds() {
        val source = _conn.value
        val epoch = companionEpoch
        val request = ++windsRequest
        viewModelScope.launch {
            try {
                val raw = withContext(Dispatchers.IO) { source.api().getWindsRaw() }
                val list = Wind.parse(raw)
                if (!isCompanionOrigin(source, epoch) || request != windsRequest) return@launch
                _winds.value = list
                _activeWind.value = list.firstOrNull { it.id == _activeWind.value?.id } ?: list.firstOrNull()
                withContext(Dispatchers.IO) { offlineStore(source)?.saveWinds(raw) }
            } catch (e: Exception) {
                if (!isCompanionOrigin(source, epoch)) return@launch
                if (!source.local && isUnreachable(e)) {
                    val cached = withContext(Dispatchers.IO) { offlineStore(source)?.windsRaw() }
                    if (cached != null && _winds.value.isEmpty()) showCachedWinds(cached)
                    goOffline(source)
                    if (cached != null) return@launch
                }
                companyError(e)
            }
        }
    }

    data class WindLibraryState(
        val generation: Long = 0L,
        val items: List<GregalApi.WindLibraryItem>? = null,
        val loading: Boolean = false, val pending: String? = null,
        val error: String? = null, val success: Boolean = false,
    )
    private val _windLibrary = MutableStateFlow(WindLibraryState())
    val windLibrary: StateFlow<WindLibraryState> = _windLibrary
    private var libraryRequest = 0L

    fun loadWindLibrary() {
        if (_windLibrary.value.pending != null) return
        val source = _conn.value; val epoch = companionEpoch; val request = ++libraryRequest
        _windLibrary.value = _windLibrary.value.copy(loading = true, error = null)
        viewModelScope.launch {
            try {
                val items = withContext(Dispatchers.IO) { source.api().getWindLibrary() }
                if (isCompanionOrigin(source, epoch) && request == libraryRequest)
                    _windLibrary.value = _windLibrary.value.copy(items = items, loading = false)
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch) && request == libraryRequest)
                    _windLibrary.value = _windLibrary.value.copy(items = _windLibrary.value.items.orEmpty(),
                        loading = false, error = appText("app.library.loadError"))
            }
        }
    }

    fun restoreWindLibrary(item: GregalApi.WindLibraryItem, onDone: (Boolean) -> Unit = {}) =
        mutateWindLibrary(item, null, true, true, onDone)

    fun assignWindLibrary(item: GregalApi.WindLibraryItem, target: Wind, memory: Boolean,
                          knowledge: Boolean, onDone: (Boolean) -> Unit = {}) =
        mutateWindLibrary(item, target, memory, knowledge, onDone)

    private fun mutateWindLibrary(item: GregalApi.WindLibraryItem, target: Wind?, memory: Boolean,
                                  knowledge: Boolean, onDone: (Boolean) -> Unit) {
        if (_windLibrary.value.pending != null) { onDone(false); return }
        val source = _conn.value; val epoch = companionEpoch; val request = ++libraryRequest
        _windLibrary.value = _windLibrary.value.copy(pending = item.id, loading = false, error = null, success = false)
        viewModelScope.launch {
            try {
                require(if (target == null) item.restorable else target.id != item.wind.id && (memory || knowledge))
                val api = source.api()
                val before = withContext(Dispatchers.IO) { api.getWindLibrary() }
                if (!isCompanionOrigin(source, epoch) || request != libraryRequest) return@launch
                val current = before.singleOrNull { it.id == item.id }
                check(current != null && item.sameInformation(current)) { "libraryChanged" }
                val targetId = target?.id ?: item.wind.id
                val activeBefore = before.singleOrNull { it.kind == "active" && it.wind.id == targetId }
                if (target != null) check(activeBefore?.wind == target) { "libraryChanged" }
                // Restoring after an uncertain response reconciles first: do not duplicate the POST.
                if (target == null && activeBefore != null) {
                    check(libraryRestoreMatches(item, activeBefore)) { "libraryConflict" }
                } else {
                    withContext(Dispatchers.IO) {
                        if (target == null) api.restoreWindLibrary(item.id)
                        else api.assignWindLibrary(item.id, target.id, memory, knowledge)
                    }
                }
                if (!isCompanionOrigin(source, epoch)) return@launch
                val after = withContext(Dispatchers.IO) { api.getWindLibrary() }
                val active = after.singleOrNull { it.kind == "active" && it.wind.id == targetId }
                check(active != null && if (target == null) libraryRestoreMatches(item, active)
                    else libraryAssignmentMatches(item, activeBefore!!, active, memory, knowledge)) { "libraryReadback" }
                val winds = withContext(Dispatchers.IO) { api.getWinds() }
                check(winds.any { it.id == targetId }) { "libraryReadback" }
                if (!isCompanionOrigin(source, epoch) || request != libraryRequest) return@launch
                ++windsRequest // Invalidate catalog GETs started before the verified mutation.
                _winds.value = winds
                _activeWind.value = winds.firstOrNull { it.id == _activeWind.value?.id } ?: winds.firstOrNull()
                _ventDetall.value = _ventDetall.value - targetId
                _windsResum.value = _windsResum.value - targetId
                if (_windProfile.value?.wind?.id == targetId) loadWindProfile(targetId)
                _windLibrary.value = _windLibrary.value.copy(items = after, pending = null, success = true, error = null)
                onDone(true)
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch) && request == libraryRequest) {
                    val key = when {
                        e.message == "libraryChanged" -> "app.library.changed"
                        e.message == "libraryConflict" || e.message.orEmpty().contains("HTTP 409") -> "app.library.conflict"
                        else -> "app.library.saveError"
                    }
                    _windLibrary.value = _windLibrary.value.copy(pending = null, error = appText(key))
                    onDone(false)
                }
            }
        }
    }

    private fun libraryRestoreMatches(source: GregalApi.WindLibraryItem, active: GregalApi.WindLibraryItem): Boolean =
        source.wind.copy(kit = "") == active.wind.copy(kit = "") &&
            (source.memory.isBlank() || active.memory.contains(source.memory.trim())) &&
            (source.knowledge.isBlank() || active.knowledge.contains(source.knowledge.trim())) &&
            source.skills.all { sk -> active.skills.any { it == sk } }

    private fun libraryAssignmentMatches(source: GregalApi.WindLibraryItem, before: GregalApi.WindLibraryItem,
                                         after: GregalApi.WindLibraryItem, memory: Boolean, knowledge: Boolean): Boolean {
        fun merge(old: String, incoming: String): String = when {
            incoming.isBlank() -> old
            old.isBlank() -> incoming
            old.contains(incoming.trim()) -> old
            else -> old.trimEnd('\r', '\n') + "\n\n" + incoming
        }
        return before.wind == after.wind &&
            after.memory == (if (memory) merge(before.memory, source.memory) else before.memory) &&
            after.knowledge == (if (knowledge) merge(before.knowledge, source.knowledge) else before.knowledge) &&
            after.skills.sortedBy { it.nom } == (if (knowledge) (before.skills + source.skills).distinctBy { it.nom }
                else before.skills).sortedBy { it.nom }
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
                check(profile.wind.id == id) { "Profile identity does not match" }
                if (isCompanionOrigin(source, epoch) && request == profileRequest) _windProfile.value = profile
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch) && request == profileRequest)
                    _windProfileError.value = appText("app.status.company", e.message.orEmpty())
            } finally {
                if (isCompanionOrigin(source, epoch) && request == profileRequest) _windProfileBusy.value = false
            }
        }
    }

    suspend fun windCards(windId: String): org.json.JSONObject {
        val source = _conn.value; val epoch = companionEpoch
        val result = withContext(Dispatchers.IO) { source.api().get("/api/winds/cards?wind=" + java.net.URLEncoder.encode(windId, "UTF-8")) }
        if (!isCompanionOrigin(source, epoch)) throw IllegalStateException("Connection changed")
        return result
    }
    suspend fun windCardAction(windId: String, action: String, id: String = "", title: String = "", prompt: String = ""): org.json.JSONObject {
        val source = _conn.value; val epoch = companionEpoch
        val payload = org.json.JSONObject().put("wind", windId).put("action", action).put("id", id).put("title", title).put("prompt", prompt)
        val result = withContext(Dispatchers.IO) { source.api().post("/api/winds/cards", payload) }
        if (!isCompanionOrigin(source, epoch)) throw IllegalStateException("Connection changed")
        return result
    }

    /** Activa una vigilància de l'ofici: una rutina del vent (POST /api/winds/rutina), com a l'escriptori. */
    fun activateTradeWatch(windId: String, watch: WindTradeWatch, lang: String, onDone: (Boolean) -> Unit = {}) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            val ok = try {
                withContext(Dispatchers.IO) { source.api().post("/api/winds/rutina", watch.rutina(windId, lang)).has("job") }
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                false
            }
            if (!isCompanionOrigin(source, epoch)) return@launch
            if (ok) loadRutines()
            onDone(ok)
        }
    }

    fun updateWindKnowledge(id: String, text: String, onDone: (Boolean) -> Unit = {}) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val result = withContext(Dispatchers.IO) {
                    val api = source.api()
                    api.updateWindKnowledge(id, text)
                    api.getWindProfile(id)
                }
                check(result.wind.id == id && result.knowledge == text) { "Knowledge readback does not match" }
                if (!isCompanionOrigin(source, epoch)) return@launch
                if (_windProfile.value?.wind?.id == id) _windProfile.value = result
                onDone(true)
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) {
                    companyError(e); onDone(false)
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
                    companyError(e); onDone(false)
                }
            }
        }
    }

    // Named conversations: one member is an individual thread, 2+ a rotllana.
    private val _rotllanes = MutableStateFlow<List<Rotllana>>(emptyList())
    val rotllanes: StateFlow<List<Rotllana>> = _rotllanes
    private val _rotllanaChat = MutableStateFlow<Map<String, List<PetMsg>>>(emptyMap())
    val rotllanaChat: StateFlow<Map<String, List<PetMsg>>> = _rotllanaChat
    private val _rotllanaChatLoading = MutableStateFlow<Set<String>>(emptySet())
    val rotllanaChatLoading: StateFlow<Set<String>> = _rotllanaChatLoading
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
                if (isCompanionOrigin(source, epoch)) companyError(e)
            }
        }
    }

    fun createRotllana(name: String, members: List<String>, onError: () -> Unit = {}, onDone: () -> Unit = {}, onOpened: (String) -> Unit = {}) {
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
                if (id.isNotBlank() && list.any { it.id == id }) { openVentsChat("rot:" + id); onOpened(id) }
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) { companyError(e); onError() }
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
                if (isCompanionOrigin(source, epoch)) { companyError(e); onDone(false) }
            }
        }
    }

    fun deleteRotllana(id: String) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().deleteRotllana(id) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                crewPolling.remove(id)?.cancel()
                _crewStep.value = _crewStep.value - id
                _rotllanes.value = _rotllanes.value.filterNot { it.id == id }
                ventsRecency.clear(source, "rot", id)
                _rotllanaChat.value = _rotllanaChat.value - id
                if (_ventsOpen.value == "rot:$id") _ventsOpen.value = null
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) companyError(e) }
        }
    }

    fun loadRotllanaChat(id: String) {
        val source = _conn.value; val epoch = companionEpoch
        if (_rotllanaBusy.value.contains(id)) { _rotllanaChatLoading.value = _rotllanaChatLoading.value - id; return }
        val request = (rotllanaHistoryRequests[id] ?: 0L) + 1
        rotllanaHistoryRequests[id] = request
        val skeleton = _rotllanaChat.value[id].isNullOrEmpty()
        if (skeleton) _rotllanaChatLoading.value = _rotllanaChatLoading.value + id
        viewModelScope.launch {
            try {
                val msgs = withContext(Dispatchers.IO) { source.api().getRotllanaChat(id) }
                if (isCompanionOrigin(source, epoch) && request == rotllanaHistoryRequests[id] &&
                    !_rotllanaBusy.value.contains(id))
                    _rotllanaChat.value = _rotllanaChat.value + (id to ventsRecency.restore(source, "rot", id, msgs))
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) companyError(e) }
            finally { if (skeleton) _rotllanaChatLoading.value = _rotllanaChatLoading.value - id }
        }
    }

    /** Últim text de vent: de l'historial si en porta, si no el text directe. */
    private fun lastPetText(hist: List<PetMsg>, fallback: String): String =
        hist.lastOrNull { it.from == "pet" }?.text.orEmpty().ifBlank { fallback }

    fun sendRotllanaMessage(rotllanaId: String, rawMessage: String, wind: String = "") {
        // Com al xat 1:1: sense salts ni espais de més (un Enter de més deixava
        // la bombolla amb una línia buida).
        val message = rawMessage.trim()
        if (message.isEmpty()) return
        // El torn treballa: el missatge hi entra com a steer (sense aturar
        // res); si el servidor ja no té el torn viu, com a missatge nou.
        if (_rotllanaBusy.value.contains(rotllanaId)) {
            steerRotllana(rotllanaId, message)
            return
        }
        val source = _conn.value; val epoch = companionEpoch
        rotllanaHistoryRequests[rotllanaId] = (rotllanaHistoryRequests[rotllanaId] ?: 0L) + 1
        val before = _rotllanaChat.value[rotllanaId].orEmpty()
        _rotllanaBusy.value = _rotllanaBusy.value + rotllanaId
        live(rotllanaId) { WindLive() }
        val sentHistory = before + PetMsg("user", message, ts = System.currentTimeMillis())
        _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to sentHistory)
        viewModelScope.launch {
            var answered = false
            var answer = ""
            var partial = false
            var turnError = ""
            val target = "rot:$rotllanaId"
            val displayName = _rotllanes.value.firstOrNull { it.id == rotllanaId }?.name.orEmpty().ifBlank { if (_idioma.value == "en") "Group" else "Rotllana" }
            GregalNotifications.workStarted(appContext, target, displayName)
            try {
                try {
                    // Directe primer; el done porta l'historial amb veus.
                    val buf = WindStreamText()
                    var streamSpeaker = ""
                    val (_, replyText, hist) = withContext(Dispatchers.IO) {
                        source.api().rotllanaChatStream(rotllanaId, message, wind) { e ->
                            if (!isCompanionOrigin(source, epoch)) return@rotllanaChatStream
                            when (e) {
                                is GregalApi.WindStreamEvent.Phase -> {
                                    val sp = e.phase.removePrefix("speaker:")
                                    if (sp != e.phase) {
                                        // Una veu que respon una altra veu del mateix torn: discussió.
                                        val answering = streamSpeaker.isNotEmpty() && streamSpeaker != sp
                                        live(rotllanaId) { it.copy(phase = "thinking", writing = false, receivedText = false, tool = "", voices = if (answering) it.voices + 1 else it.voices) }
                                        // Nova veu: la bombolla anterior queda com a missatge.
                                        commitRotllanaStreamBubble(rotllanaId, buf.take(), streamSpeaker)
                                        streamSpeaker = sp
                                        val who = _winds.value.firstOrNull { it.id == sp }?.name.orEmpty()
                                        live(rotllanaId) { it.copy(speaker = who) }
                                        // The shared status row identifies the speaker; do not add a fake reply bubble.
                                    } else {
                                        // Una eina pel mig no esborra el que ja s'ha llegit.
                                        commitRotllanaStreamBubble(rotllanaId, buf.take(), streamSpeaker)
                                        live(rotllanaId) { it.copy(phase = e.phase, writing = false, receivedText = false, tool = "") }
                                        dropStreamingBubble(rotllanaId, rotllana = true)
                                    }
                                }
                                is GregalApi.WindStreamEvent.Activity -> {
                                    live(rotllanaId) { it.copy(phase = e.phase, tool = e.tool, step = e.step, writing = false) }
                                }
                                is GregalApi.WindStreamEvent.Token -> {
                                    buf.append(e.token)?.let { text ->
                                        live(rotllanaId) { it.copy(writing = true, receivedText = true) }
                                        setStreamingBubble(rotllanaId, text, rotllana = true)
                                    }
                                }
                                else -> handleWindAskEvent(e)
                            }
                        }
                    }
                    if (!isCompanionOrigin(source, epoch)) return@launch
                    dropStreamingBubble(rotllanaId, rotllana = true)
                    _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to ventsRecency.completed(source, "rot", rotllanaId,
                        hist.ifEmpty { sentHistory + if (replyText.isBlank()) emptyList() else listOf(PetMsg("pet", replyText)) }, System.currentTimeMillis(), sentHistory))
                    partial = hist.lastOrNull { it.from == "pet" }?.parcial == true
                    replyText.let { if (it.isNotBlank()) { answered = true; answer = it } }
                } catch (_: WindStreamUnavailable) {
                    dropStreamingBubble(rotllanaId, rotllana = true)
                    val (wind, text, hist) = withContext(Dispatchers.IO) { source.api().rotllanaChat(rotllanaId, message, wind) }
                    if (!isCompanionOrigin(source, epoch)) return@launch
                    val speaker = _winds.value.firstOrNull { it.id == wind }
                    _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to ventsRecency.completed(source, "rot", rotllanaId, hist.ifEmpty {
                        sentHistory + PetMsg("pet", text, wind, speaker?.name.orEmpty(), ts = System.currentTimeMillis())
                    }, System.currentTimeMillis(), sentHistory))
                    partial = hist.lastOrNull { it.from == "pet" }?.parcial == true
                    text.let { if (it.isNotBlank()) { answered = true; answer = it } }
                }
                // El torn pot haver programat avisos (remind): posa al dia
                // els jobs perquè l'alarma soni encara que tanquis l'app.
                if (isCompanionOrigin(source, epoch)) loadJobs()
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                turnError = turnFailed(source, epoch, rotllanaId, true, e)
            } finally {
                aturats.remove(rotllanaId) // un «atura» que arriba tard no amaga el proper error
                if (isCompanionOrigin(source, epoch)) {
                    _rotllanaBusy.value = _rotllanaBusy.value - rotllanaId; endLive(rotllanaId)
                    if (before.lastOrNull()?.kind == "question") followCrew(rotllanaId)
                }
                GregalNotifications.workFinished(appContext, answered && isCompanionOrigin(source, epoch), message, answer,
                    target, displayName, partial, if (isCompanionOrigin(source, epoch)) turnError else "")
            }
        }
    }

    /** Carrega els missatges programats i les estadístiques dels vents. */
    fun loadProgramats() {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val p = loadingOf("programats") { withContext(Dispatchers.IO) { source.api().getWindsProgramats() } }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _programatsError.value = null
                _programats.value = p
            } catch (e: Exception) {
                if (!isCompanionOrigin(source, epoch)) return@launch
                // Mateixa regla que la wiki: cap spinner etern si falla.
                _programatsError.value = e.message ?: "error"
                if (_programats.value == null) _programats.value = WindsProgramats(emptyList(), emptyList(), emptyList())
            }
        }
    }

    /** Activitat dels vents (el que faran sols), per vent; "" = tots. */
    private val _automations = MutableStateFlow<Map<String, WindAutomations>>(emptyMap())
    val automations: StateFlow<Map<String, WindAutomations>> = _automations.asStateFlow()

    fun loadAutomations(wind: String = "") {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val got = withContext(Dispatchers.IO) { source.api().getWindAutomations(wind) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _automations.value = _automations.value + (wind to got)
                mirrorReminders(source, OfflineLogic.remindersOf(got.items), wind)
            } catch (e: Exception) {
                if (!isCompanionOrigin(source, epoch)) return@launch
                _automations.value = _automations.value + (wind to WindAutomations(emptyList(), WindAutomationCounts(), e.message ?: "error"))
                notify(appText("act.loadError", e.message.orEmpty()))
            }
        }
    }

    /** Fes-ho ara / pausa / reprèn / esborra; després refresca la llista global i la del vent. */
    fun automationAction(item: WindAutomation, action: String, wind: String = "") {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val ok = withContext(Dispatchers.IO) { source.api().windAutomationAction(item.id, action) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                notify(appText(if (ok) "act.done.$action" else "act.failed"))
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) notify(appText("act.failedWhy", e.message.orEmpty()))
            }
            for (w in setOf("", wind, item.wind)) if (w == "" || _automations.value.containsKey(w)) loadAutomations(w)
            if (action == "delete" || action == "resume" || action == "pause") loadJobs()
        }
    }

    /** Targetes de l'últim torn de cada vent: avisos i seguiments que ha programat de debò. */
    private val _turnActions = MutableStateFlow<Map<String, List<WindTurnAction>>>(emptyMap())
    val turnActions: StateFlow<Map<String, List<WindTurnAction>>> = _turnActions.asStateFlow()

    private fun showTurnActions(windId: String, r: WindReply) {
        _turnActions.value = _turnActions.value + (windId to r.actions)
        val name = _winds.value.firstOrNull { it.id == windId }?.name.orEmpty()
        mirrorReminders(_conn.value, r.actions.mapNotNull { a ->
            JobSchedule.parseInstant(a.atIso)?.let { MirroredReminder(a.id, windId, name, it, a.text) }
        }, null)
        if (r.actions.isNotEmpty()) refreshAutomations()
        else if (r.reminds.isNotEmpty()) notify(appText("app.vents.remindSet", r.reminds.joinToString(", ")))
    }

    /** Desfà un avís o seguiment des de la seva targeta al xat. */
    fun undoTurnAction(windId: String, a: WindTurnAction) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            val ok = withContext(Dispatchers.IO) { runCatching { source.api().windAutomationAction(a.id, "delete") }.getOrDefault(false) }
            if (!isCompanionOrigin(source, epoch)) return@launch
            if (ok) {
                _turnActions.value = _turnActions.value + (windId to _turnActions.value[windId].orEmpty().filter { it.id != a.id })
                forgetMirrored(source, a.id)
                notify(appText("act.done.delete"))
                refreshAutomations()
            } else notify(appText("act.failed"))
        }
    }

    /** Torna a llegir les llistes d'activitat ja obertes (global i per vent). */
    fun refreshAutomations() { for (w in _automations.value.keys) loadAutomations(w) }

    /** El que han fet els vents (documents, webs, gràfics, imatges, fitxes, fonts). */
    private val _saved = MutableStateFlow<Map<String, List<WindSaved>>>(emptyMap())
    val saved: StateFlow<Map<String, List<WindSaved>>> = _saved.asStateFlow()

    fun loadSaved(wind: String = "") {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val got = withContext(Dispatchers.IO) { source.api().getWindSaved(wind) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _saved.value = _saved.value + (wind to got)
            } catch (e: Exception) {
                if (!isCompanionOrigin(source, epoch)) return@launch
                if (_saved.value[wind] == null) _saved.value = _saved.value + (wind to emptyList())
                notify(appText("saved.loadError", e.message.orEmpty()))
            }
        }
    }

    // ---- Notes (Notes.kt): el quadern de l'usuari, compartit amb els vents ----
    private val _notes = MutableStateFlow<NotesPage?>(null)
    val notes: StateFlow<NotesPage?> = _notes.asStateFlow()
    private val _openNote = MutableStateFlow<NoteDetail?>(null)
    val openNote: StateFlow<NoteDetail?> = _openNote.asStateFlow()
    var notesQuery = NotesQuery()
        private set

    /** La llista amb aquests filtres (per defecte, els últims). */
    fun loadNotes(query: NotesQuery = notesQuery) {
        notesQuery = query
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val got = withContext(Dispatchers.IO) { source.api().getNotes(query) }
                if (!isCompanionOrigin(source, epoch) || query != notesQuery) return@launch
                _notes.value = got
            } catch (e: Exception) {
                if (!isCompanionOrigin(source, epoch)) return@launch
                if (_notes.value == null) _notes.value = NotesPage(emptyList(), emptyList(), emptyList(), 0)
                notify(appText("notes.loadError", e.message.orEmpty()))
            }
        }
    }

    fun openNote(id: String) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val got = withContext(Dispatchers.IO) { source.api().getNote(id) }
                if (isCompanionOrigin(source, epoch)) _openNote.value = got
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) notify(appText("notes.loadError", e.message.orEmpty()))
            }
        }
    }

    fun closeNote() { _openNote.value = null; loadNotes() }

    /** [[Títol]]: obre la nota amb aquest títol o, si no n'hi ha, en proposa una de nova. */
    fun openNoteByTitle(title: String, onMissing: (String) -> Unit) {
        val source = _conn.value
        viewModelScope.launch {
            try {
                val hit = withContext(Dispatchers.IO) { source.api().getNotes(NotesQuery(q = title)) }
                    .notes.firstOrNull { foldNote(it.title) == foldNote(title) }
                if (hit != null) openNote(hit.id) else onMissing(title)
            } catch (e: Exception) { notify(appText("notes.loadError", e.message.orEmpty())) }
        }
    }

    /**
     * Desa sense avisos ni obrir res (la nota àgil es desa sola mentre
     * s'escriu). null = la nota ha canviat a fora (409); llança si falla.
     */
    suspend fun saveNoteQuiet(input: NoteInput): Note? {
        val source = _conn.value
        return withContext(Dispatchers.IO) { source.api().saveNote(input) }
    }

    /** Desa; onDone(false) si la nota ha canviat mentre s'editava. */
    fun saveNote(input: NoteInput, onDone: (Boolean) -> Unit = {}) {
        val source = _conn.value
        viewModelScope.launch {
            try {
                val saved = withContext(Dispatchers.IO) { source.api().saveNote(input) }
                if (saved == null) { onDone(false); return@launch }
                onDone(true)
                notify(appText("notes.saved"))
                openNote(saved.id)
                loadNotes()
            } catch (e: Exception) {
                notify(appText("notes.saveError", e.message.orEmpty()))
                onDone(true)
            }
        }
    }

    fun toggleNoteTask(n: Note, index: Int) = setNoteBody(n, toggleNoteTask(n.body, index))

    /**
     * Canvia el cos d'una nota oberta i el desa pel camí de sempre, amb la
     * versió que es veia com a base: si ha canviat mentre tant, es torna a
     * obrir la nova (el projecte i la resta no es toquen).
     */
    fun setNoteBody(n: Note, body: String) {
        _openNote.value = _openNote.value?.let { d -> if (d.note.id == n.id) d.copy(note = d.note.copy(body = body)) else d }
        saveNote(NoteInput(n.id, n.title, body, n.notebook, n.tags, n.private, n.locked, n.updated, n.noRemind)) { ok ->
            if (!ok) { notify(appText("notes.changed")); openNote(n.id) }
        }
    }

    /**
     * Demana a la IA una versió nova de la nota (/api/notes/ai): onDone amb
     * la proposta, o null si no hi ha res a canviar o ha fallat (ja avisat).
     * No desa res.
     */
    fun noteAI(action: String, instruction: String, title: String, body: String, onDone: (String?) -> Unit) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            val proposal = try {
                val r = withContext(Dispatchers.IO) { source.api().noteAI(action, instruction, title, body) }
                if (!r.changed || r.body.isBlank()) { notify(appText("notes.ai.same")); null } else r.body
            } catch (e: Exception) {
                val msg = e.message.orEmpty()
                notify(if (noteAINeedsModel(msg)) appText("notes.ai.noModel") else appText("notes.ai.error", msg))
                null
            }
            onDone(if (isCompanionOrigin(source, epoch)) proposal else null)
        }
    }

    /** Text que arriba de fora (Comparteix, drecera): Notes l'obre a l'editor. */
    private val _noteCapture = MutableStateFlow<String?>(null)
    val noteCapture: StateFlow<String?> = _noteCapture.asStateFlow()

    fun startNoteCapture(text: String) {
        _noteCapture.value = text
        go(Screen.Notes)
    }

    fun takeNoteCapture(): String? = _noteCapture.value.also { _noteCapture.value = null }

    /** Captura ràpida: una nota sense obrir l'editor (la primera línia fa de títol). */
    fun quickNote(text: String) {
        val source = _conn.value
        val q = notesQuery
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) {
                    source.api().saveNote(NoteInput("", "", text, q.notebook, listOfNotNull(q.tag.ifBlank { null }), false, false, "", project = q.newProject))
                }
                notify(appText("notes.noted"))
                loadNotes()
            } catch (e: Exception) { notify(appText("notes.saveError", e.message.orEmpty())) }
        }
    }

    /** Marca feta una tasca de l'agenda dins de la seva nota. */
    fun tickAgenda(item: AgendaItem) {
        val source = _conn.value
        viewModelScope.launch {
            try {
                val ok = withContext(Dispatchers.IO) {
                    val n = source.api().getNote(item.noteId).note
                    source.api().saveNote(NoteInput(n.id, n.title, toggleNoteTask(n.body, item.index), n.notebook, n.tags, n.private, n.locked, n.updated, n.noRemind)) != null
                }
                notify(appText(if (ok) "notes.done" else "notes.changed"))
                loadNotes()
            } catch (e: Exception) { notify(appText("notes.saveError", e.message.orEmpty())) }
        }
    }

    fun pinNote(n: Note, pinned: Boolean) {
        val source = _conn.value
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().pinNote(n.id, pinned) }
                _openNote.value = _openNote.value?.let { d -> if (d.note.id == n.id) d.copy(note = d.note.copy(pinned = pinned)) else d }
                loadNotes()
            } catch (e: Exception) { notify(appText("notes.saveError", e.message.orEmpty())) }
        }
    }

    fun deleteNote(id: String) {
        val source = _conn.value
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().deleteNote(id) }
                _openNote.value = null
                notify(appText("notes.deleted"))
                loadNotes()
            } catch (e: Exception) { notify(appText("notes.saveError", e.message.orEmpty())) }
        }
    }

    /** Seguiments proactius d'un vent: encès o apagat. */
    fun setWindFollowups(wind: Wind, on: Boolean) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().setWindFollowups(wind, on) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _winds.value = _winds.value.map { if (it.id == wind.id) it.copy(followups = if (on) "on" else "off") else it }
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) notify(appText("act.failedWhy", e.message.orEmpty()))
            }
        }
    }

    /** Wiki de l'espai vents: pàgines de resum. */
    private val _wikiPages = MutableStateFlow<List<GregalApi.WikiPage>?>(null)
    val wikiPages: StateFlow<List<GregalApi.WikiPage>?> = _wikiPages.asStateFlow()
    private val _wikiBusy = MutableStateFlow(false)
    val wikiBusy: StateFlow<Boolean> = _wikiBusy.asStateFlow()

    /** Darrer error en carregar la wiki; null vol dir cap error conegut. */
    private val _wikiError = MutableStateFlow<String?>(null)
    val wikiError: StateFlow<String?> = _wikiError.asStateFlow()

    // --- Feines dels vents (servidor o mòbil) ---

    private val _windTasks = MutableStateFlow<List<WindTask>?>(null)
    val windTasks: StateFlow<List<WindTask>?> = _windTasks.asStateFlow()
    private val _windTasksError = MutableStateFlow("")
    /** Per què no s'han pogut llegir les feines ("" si tot bé): una llista buida no ho ha d'amagar. */
    val windTasksError: StateFlow<String> = _windTasksError.asStateFlow()

    // --- Carpeta de treball: on treballen els vents ---
    private val _workspaces = MutableStateFlow<WorkspacesInfo?>(null)
    val workspaces: StateFlow<WorkspacesInfo?> = _workspaces.asStateFlow()

    /** La carpeta que s'està mirant al navegador (no és la triada fins que es toca). */
    private val _carpeta = MutableStateFlow<DirsInfo?>(null)
    val carpeta: StateFlow<DirsInfo?> = _carpeta.asStateFlow()

    private val _carpetaBusy = MutableStateFlow(false)
    val carpetaBusy: StateFlow<Boolean> = _carpetaBusy.asStateFlow()

    /** Error de l'última acció amb carpeta, amb el que cal repetir si es reintenta. */
    private val _carpetaError = MutableStateFlow<String?>(null)
    val carpetaError: StateFlow<String?> = _carpetaError.asStateFlow()

    /** Darrera carpeta demanada: el botó de reintentar-la torna a obrir. */
    private var ultimaCarpeta: String? = null

    /**
     * Llegeix les feines. Si una que anava en marxa acaba, ho diu: el mateix
     * avís que el vigilant dona amb l'app tancada.
     */
    fun loadWindTasks() {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val tasks = withContext(Dispatchers.IO) { source.api().windTasks() }
                if (!isCompanionOrigin(source, epoch)) return@launch
                val before = _windTasks.value.orEmpty()
                _windTasks.value = tasks
                _windTasksError.value = ""
                feinesAcabades(before, tasks).firstOrNull()?.let { t ->
                    val key = when (t.status) {
                        "done" -> "app.feines.finished"
                        "interrupted" -> "app.feines.interruptedNotice"
                        else -> "app.feines.failedNotice"
                    }
                    notify(appText(key, oneLine(t.task, 60)))
                }
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                if (isCompanionOrigin(source, epoch)) {
                    _windTasksError.value = e.message.orEmpty().ifBlank { "error" }.take(160)
                    if (_windTasks.value == null) _windTasks.value = emptyList()
                }
            }
        }
    }

    /**
     * Carrega la carpeta actual i els projectes recents (/api/workspaces).
     * Surt de la pantalla del selector i també la fa servir Preferències per
     * ensenyar on s'està treballant.
     */
    fun loadWorkspaces() {
        val source = _conn.value
        viewModelScope.launch {
            try {
                val info = withContext(Dispatchers.IO) { source.api().workspaces() }
                if (_conn.value != source) return@launch
                _workspaces.value = info
                _carpetaError.value = null
            } catch (e: Exception) {
                if (_conn.value == source) _carpetaError.value = e.message ?: ""
            }
        }
    }

    // --- Previsualització: la web que el vent acaba de fer ---
    // El servidor publica la carpeta de treball en lectura; aquí només es
    // munta la URL del servidor on ja s'està connectat (el mateix host que
    // fa servir l'API), així funciona igual a casa i al servidor.

    private val _webInfo = MutableStateFlow(PreviewInfo())
    val webInfo: StateFlow<PreviewInfo> = _webInfo.asStateFlow()
    private val _webUrl = MutableStateFlow("")
    val webUrl: StateFlow<String> = _webUrl.asStateFlow()
    private val _webTitle = MutableStateFlow("")
    val webTitle: StateFlow<String> = _webTitle.asStateFlow()
    private val _webError = MutableStateFlow("")
    val webError: StateFlow<String> = _webError.asStateFlow()

    /** URL completa (servidor + clau + fitxer) d'una web de la carpeta. */
    fun previewUrlOf(path: String): String {
        val c = _conn.value
        val base = _webInfo.value.base
        // Sense clau encara no hi ha res a obrir: millor buit que l'arrel del
        // servidor, que ensenyaria la pàgina d'entrada de Gregal.
        if (base.isBlank()) return ""
        return baseUrlOf(c.host, c.port).trimEnd('/') + base + path.trimStart('/')
    }

    /** Demana al servidor quines webs hi ha a la carpeta de treball. */
    fun loadPreview() {
        val source = _conn.value
        viewModelScope.launch {
            try {
                val info = withContext(Dispatchers.IO) { source.api().preview() }
                if (_conn.value != source) return@launch
                _webInfo.value = info
                _webError.value = ""
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                if (_conn.value == source) _webError.value = e.message ?: ""
            }
        }
    }

    /** Obre la previsualització d'un fitxer (sense path, l'índex de la carpeta). */
    fun openPreview(path: String = "", title: String = "") {
        _webUrl.value = previewUrlOf(path)
        _webTitle.value = title.ifBlank {
            path.substringAfterLast('/').ifBlank { appText("app.preview.title") }
        }
        go(Screen.Preview)
        if (_webInfo.value.base.isNotBlank()) return
        // La clau encara no la tenim: es demana i, quan arriba, s'omple la URL.
        val source = _conn.value
        viewModelScope.launch {
            runCatching { withContext(Dispatchers.IO) { source.api().preview() } }
                .onSuccess { info ->
                    if (_conn.value != source) return@onSuccess
                    _webInfo.value = info
                    _webError.value = ""
                    _webUrl.value = previewUrlOf(path)
                }
                .onFailure { if (_conn.value == source) _webError.value = it.message ?: "" }
        }
    }

    /**
     * Obre una carpeta al navegador (/api/dirs). Sense path obre la que ja
     * tens triada; és el que fa la pantalla en entrar-hi.
     */
    fun obrirCarpeta(path: String? = null) {
        val source = _conn.value
        ultimaCarpeta = path
        _carpetaBusy.value = true
        viewModelScope.launch {
            try {
                val info = withContext(Dispatchers.IO) { source.api().dirs(path) }
                if (_conn.value != source) return@launch
                _carpeta.value = info
                _carpetaError.value = null
            } catch (e: Exception) {
                if (_conn.value == source) _carpetaError.value = e.message ?: ""
            } finally {
                if (_conn.value == source) _carpetaBusy.value = false
            }
        }
    }

    /** Tria la carpeta on han de treballar els vents (POST /api/workspaces). */
    fun triarCarpeta(path: String) {
        val source = _conn.value
        _carpetaBusy.value = true
        viewModelScope.launch {
            try {
                val info = withContext(Dispatchers.IO) { source.api().setWorkspace(path) }
                if (_conn.value != source) return@launch
                _workspaces.value = info
                _carpetaError.value = null
                // El servidor mana: si ha normalitzat el camí, el navegador
                // ha d'ensenyar el de debò i no el que s'ha escrit.
                obrirCarpeta(info.current)
                notify(appText("app.carpeta.changed", info.project.ifBlank { info.current }))
                refresh()
            } catch (e: Exception) {
                if (_conn.value == source) {
                    // L'error del servidor sol ser català; si és el del torn en
                    // curs, s'explica què cal fer en lloc de deixar-lo cru.
                    val msg = (e.message ?: "").removePrefix("HTTP 400: ")
                    _carpetaError.value = msg
                }
            } finally {
                if (_conn.value == source) _carpetaBusy.value = false
            }
        }
    }

    /** Reintenta l'última acció de carpeta que ha fallat. */
    fun reintentarCarpeta() {
        _carpetaError.value = null
        obrirCarpeta(ultimaCarpeta)
    }

    fun readTaskFile(path: String, done: (String?, String?) -> Unit) {
        val source = _conn.value
        viewModelScope.launch {
            try {
                val (text, why) = withContext(Dispatchers.IO) { source.api().readTextFile(path) }
                done(text, when (why) {
                    null -> null
                    "binary" -> appText("app.feines.binary")
                    else -> appText("app.feines.tooBig")
                })
            } catch (e: Exception) {
                done(null, e.message ?: "error")
            }
        }
    }

    fun loadWiki() {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val pages = withContext(Dispatchers.IO) { source.api().getWikiPages() }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _wikiError.value = null
                _wikiPages.value = pages
            } catch (e: Exception) {
                if (!isCompanionOrigin(source, epoch)) return@launch
                // Mai spinner etern: si la càrrega falla, la pantalla ha de poder
                // sortir i tornar-hi. Guardem el missatge i aixequem el null.
                _wikiError.value = e.message ?: "error"
                if (_wikiPages.value == null) _wikiPages.value = emptyList()
            }
        }
    }

    /** Llegeix una pàgina sencera (el cos no ve a la llista). */
    suspend fun lecturaDePagina(nom: String): GregalApi.WikiPage? {
        val source = _conn.value
        return withContext(Dispatchers.IO) { runCatching { source.api().getWikiPage(nom) }.getOrNull() }
    }

    /** Desa una pàgina editada. */
    fun saveWikiPage(p: GregalApi.WikiPage, onDone: (Boolean) -> Unit = {}) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            val ok = withContext(Dispatchers.IO) { runCatching { source.api().saveWikiPage(p) }.getOrDefault(false) }
            if (isCompanionOrigin(source, epoch)) { _wikiBusy.value = false; onDone(ok) }
            if (ok) loadWiki()
        }
    }

    fun deleteWikiPage(nom: String) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            withContext(Dispatchers.IO) { runCatching { source.api().deleteWikiPage(nom) } }
            if (isCompanionOrigin(source, epoch)) loadWiki()
        }
    }

    /** Rutines de vent: feines programades que obren un torn real del vent. */
    private val _rutines = MutableStateFlow<List<GregalApi.WindRutina>?>(null)
    val rutines: StateFlow<List<GregalApi.WindRutina>?> = _rutines.asStateFlow()
    private val _rutinesError = MutableStateFlow<String?>(null)
    val rutinesError: StateFlow<String?> = _rutinesError.asStateFlow()
    private val _rutinaBusy = MutableStateFlow(false)
    val rutinaBusy: StateFlow<Boolean> = _rutinaBusy.asStateFlow()

    fun loadRutines() {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val rs = withContext(Dispatchers.IO) { source.api().getWindRutines() }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _rutinesError.value = null
                _rutines.value = rs
            } catch (e: Exception) {
                if (!isCompanionOrigin(source, epoch)) return@launch
                // Cap spinner etern: error visible i «Torna-hi».
                _rutinesError.value = e.message ?: "error"
                if (_rutines.value == null) _rutines.value = emptyList()
            }
        }
    }

    fun createRutina(windId: String, kind: String, time: String, intervalH: Double, prompt: String, name: String, onDone: (Boolean) -> Unit) {
        val source = _conn.value; val epoch = companionEpoch
        _rutinaBusy.value = true
        viewModelScope.launch {
            val ok = withContext(Dispatchers.IO) { runCatching { source.api().createWindRutina(windId, kind, time, intervalH, prompt, name) }.getOrDefault(false) }
            if (isCompanionOrigin(source, epoch)) {
                _rutinaBusy.value = false
                if (ok) { loadRutines(); refreshAutomations() } else notify(appText("app.rutines.createFail", ""))
                onDone(ok)
            }
        }
    }

    fun deleteRutina(id: String) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            withContext(Dispatchers.IO) { runCatching { source.api().deleteWindRutina(id) } }
            if (isCompanionOrigin(source, epoch)) loadRutines()
        }
    }

    /** Organització de la llista de converses: favorits i seccions (desades al servidor). */
    // Plugins del servidor: id → encès. Buit = no se sap (tot visible).
    private val _plugins = MutableStateFlow<Map<String, Boolean>>(emptyMap())
    val plugins: StateFlow<Map<String, Boolean>> = _plugins.asStateFlow()

    fun loadPlugins() {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            val m = withContext(Dispatchers.IO) { runCatching { source.api().getPlugins() }.getOrNull() }
            if (m != null && isCompanionOrigin(source, epoch)) _plugins.value = m
        }
    }

    private val _windsMeta = MutableStateFlow<GregalApi.WindsMeta>(GregalApi.WindsMeta(emptyList(), emptyList()))
    val windsMeta: StateFlow<GregalApi.WindsMeta> = _windsMeta.asStateFlow()

    fun loadWindsMeta() {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val m = withContext(Dispatchers.IO) { source.api().getWindsMeta() }
                if (isCompanionOrigin(source, epoch)) _windsMeta.value = m
            } catch (_: Exception) { /* keep the last acknowledged organization; the UI can retry */ }
        }
    }

    /** Desa la meta sencera; si el servidor no respon, no toca l'estat local. */
    private fun pushWindsMeta(m: GregalApi.WindsMeta, onDone: (Boolean) -> Unit = {}) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            val ok = withContext(Dispatchers.IO) { runCatching { source.api().saveWindsMeta(m) }.getOrDefault(false) }
            if (isCompanionOrigin(source, epoch)) {
                if (ok) _windsMeta.value = m
                onDone(ok)
            }
        }
    }

    /** Marca o desmarca una conversa com a favorita (clau «wind:<id>»/«rot:<id>»). */
    fun toggleFavorite(key: String) {
        val m = _windsMeta.value
        val favs = if (key in m.favorites) m.favorites - key else m.favorites + key
        pushWindsMeta(GregalApi.WindsMeta(favs, m.sections))
    }

    /** Afegeix una conversa a una secció; si ja hi era, la treu. */
    fun toggleSectionItem(sectionId: String, key: String) {
        val m = _windsMeta.value
        val sections = m.sections.map { s ->
            if (s.id != sectionId) s
            else GregalApi.WindSection(s.id, s.name, if (key in s.items) s.items - key else s.items + key)
        }
        pushWindsMeta(GregalApi.WindsMeta(m.favorites, sections))
    }

    /** Crea una secció nova (id curt a partir del nom) i hi posa la conversa. */
    fun createSection(name: String, firstItem: String? = null, onDone: (Boolean) -> Unit = {}) {
        val m = _windsMeta.value
        val id = name.trim().lowercase().replace(Regex("[^a-z0-9àèéíòóúçñïü-]+"), "-").trim('-').take(40).ifBlank { "seccio" }
        if (m.sections.any { it.id == id }) { onDone(false); return }
        val items = if (firstItem != null) listOf(firstItem) else emptyList()
        pushWindsMeta(GregalApi.WindsMeta(m.favorites, m.sections + GregalApi.WindSection(id, name.trim(), items)), onDone)
    }

    /** Esborra una secció sencera (les converses tornen a la llista general). */
    fun deleteSection(sectionId: String) {
        val m = _windsMeta.value
        pushWindsMeta(GregalApi.WindsMeta(m.favorites, m.sections.filter { it.id != sectionId }))
    }

    /** Resumeix l'última conversa (vent o rotllana) a una pàgina de wiki. */
    fun summarizeToWiki(wind: String? = null, rotllana: String? = null, onDone: (Boolean) -> Unit = {}) {
        val source = _conn.value; val epoch = companionEpoch
        _wikiBusy.value = true
        viewModelScope.launch {
            val page = withContext(Dispatchers.IO) { runCatching { source.api().summarizeToWiki(wind, rotllana) }.getOrNull() }
            if (isCompanionOrigin(source, epoch)) {
                _wikiBusy.value = false
                if (page != null) { loadWiki(); onDone(true) } else { notify(appText("app.wiki.summarizeFail", "")); onDone(false) }
            }
        }
    }

    /** Estudi «fes-te expert»: estat per vent (fase, pas) i engegar-la. */
    private val _windExpert = MutableStateFlow<Map<String, GregalApi.WindExpert>>(emptyMap())
    val windExpert: StateFlow<Map<String, GregalApi.WindExpert>> = _windExpert.asStateFlow()
    private val expertPolling = mutableMapOf<String, Job>()

    fun startWindExpert(windId: String, tema: String, onError: () -> Unit = {}) {
        val source = _conn.value
        val epoch = companionEpoch
        viewModelScope.launch {
            val ok = runCatching { withContext(Dispatchers.IO) { source.api().startWindExpert(windId, tema) } }.getOrDefault(false)
            if (!isCompanionOrigin(source, epoch)) return@launch
            if (ok) { expertPolling[windId]?.cancel(); expertPolling[windId] = pollWindExpert(windId, source, epoch) }
            else onError()
        }
    }

    private fun pollWindExpert(windId: String, source: Conn, epoch: Long): Job = viewModelScope.launch {
        while (isCompanionOrigin(source, epoch)) {
            val est = runCatching { withContext(Dispatchers.IO) { source.api().getWindExpert(windId) } }.getOrNull()
            if (est != null) {
                if (!isCompanionOrigin(source, epoch)) break
                _windExpert.update { it + (windId to est) }
                if (est.actiu) {
                    delay(2000)
                } else {
                    expertPolling.remove(windId)
                    if (est.exit) {
                        // Ja és expert: refresca coneixements i marcador de novetats.
                        loadVentDetall(windId)
                        refreshWindNews()
                    }
                    break
                }
            } else {
                delay(3000)
            }
        }
    }

    fun loadWindExpertOnce(windId: String) {
        val source = _conn.value
        val epoch = companionEpoch
        viewModelScope.launch {
            val est = runCatching { withContext(Dispatchers.IO) { source.api().getWindExpert(windId) } }.getOrNull() ?: return@launch
            if (!isCompanionOrigin(source, epoch)) return@launch
            _windExpert.update { it + (windId to est) }
            if (est.actiu && windId !in expertPolling) expertPolling[windId] = pollWindExpert(windId, source, epoch)
        }
    }

    /** Encén el cor d'un vent: un job que vigila el seu tema amb cerques web
     *  i parla només si troba novetats que valguin la pena. El vent es
     *  reprograma ell sol per quan acabi el proper esdeveniment del seu
     *  tema (partit, llançament…): el primer cop corre ara i ell decideix
     *  quan mirar altra vegada. L'id és wind-<vent>: el backend el tracta. */
    fun ensureWindHeartbeat(wind: Wind, on: Boolean, onDone: (Boolean) -> Unit = {}) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val api = source.api()
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
                } else if (on && existing != null && !existing.enabled) {
                    withContext(Dispatchers.IO) { api.saveJob(existing.copy(enabled = true)) }
                } else if (!on && existing != null) {
                    withContext(Dispatchers.IO) { api.deleteJob(jobId) }
                }
                if (!isCompanionOrigin(source, epoch)) return@launch
                val current = withContext(Dispatchers.IO) { api.getJobs() }
                if (!isCompanionOrigin(source, epoch)) return@launch
                check(if (on) current.any { it.id == jobId && it.enabled } else current.none { it.id == jobId }) {
                    "Heartbeat state could not be verified"
                }
                _jobs.value = current
                if (source.local) JobAlarms.sync(appContext, current)
                refreshAutomations(); onDone(true)
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) { companyError(e); onDone(false) }
            }
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
                if (isCompanionOrigin(source, epoch)) { companyError(e); onError() }
            }
        }
    }

    /** Memòria del vent sobre l'usuari + els seus coneixements (perfil). */
    private val _ventDetall = MutableStateFlow<Map<String, Pair<String, List<GregalApi.Coneixement>>>>(emptyMap())
    val ventDetall: StateFlow<Map<String, Pair<String, List<GregalApi.Coneixement>>>> = _ventDetall.asStateFlow()

    /** Resum de tots els vents (rol + de què és expert) per al selector de membres. */
    private val _windsResum = MutableStateFlow<Map<String, GregalApi.WindResum>>(emptyMap())
    val windsResum: StateFlow<Map<String, GregalApi.WindResum>> = _windsResum.asStateFlow()

    fun loadWindsResum() {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val resum = withContext(Dispatchers.IO) { source.api().getWindsResum() }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _windsResum.value = resum.associateBy { it.id }
            } catch (_: Exception) { }
        }
    }

    private val _squads = MutableStateFlow<List<GregalApi.Squad>>(emptyList())
    val squads: StateFlow<List<GregalApi.Squad>> = _squads.asStateFlow()

    fun loadSquads() {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val sq = withContext(Dispatchers.IO) { source.api().getSquads() }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _squads.value = sq
            } catch (_: Exception) { }
        }
    }

    // --- La mascota, cap de gabinet ---

    /** El que hi ha pendent de dir «endavant»: el missatge i la decisió. */
    data class CapPending(val message: String, val decision: CapDecision)
    data class CapUi(val state: CapState = CapState(), val loaded: Boolean = false, val busy: Boolean = false,
                     val pending: CapPending? = null, val asked: String = "", val error: Boolean = false,
                     /** El que acaba de dir jugant (té prioritat uns segons a la bombolla). */
                     val say: String = "", val sayAt: Long = 0L,
                     /** Curs accelerat per als que no en saben (proposat, l'usuari el pot treure). */
                     val course: Boolean = true,
                     /** Moment de l'última pujada de nivell: l'app hi fa confeti. */
                     val levelUpAt: Long = 0L, val unlocked: List<String> = emptyList(), val going: Boolean = false,
                     /** No s'ha pogut carregar el Ventet (es diu, amb «torna-ho a provar»). */
                     val loadError: Boolean = false,
                     /** El missatge que no s'ha pogut enviar: torna al camp per no perdre'l. */
                     val failedText: String = "",
                     /** L'hora de l'última proposta deixada per a «ara no» (no torna a sortir). */
                     val dismissedTs: Long = 0L,
                     /** La resposta que s'escriu en directe (buida fora del torn). */
                     val live: String = "")
    private val _cap = MutableStateFlow(CapUi())
    val cap: StateFlow<CapUi> = _cap.asStateFlow()

    fun loadCap() {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val st = withContext(Dispatchers.IO) { source.api().cap(idioma.value) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                val dismissed = capPrefs()?.getLong("dismissed", 0L) ?: 0L
                // Una proposta pendent es refà del fil: es pot acceptar també
                // després de tornar a obrir l'app.
                val pending = if (_cap.value.busy || _cap.value.going) _cap.value.pending
                    else capPendingOf(st.history, dismissed)?.let { CapPending(it.first, it.second) }
                _cap.value = _cap.value.copy(state = st, loaded = true, loadError = false, dismissedTs = dismissed, pending = pending)
            } catch (_: Exception) {
                if (isCompanionOrigin(source, epoch)) _cap.value = _cap.value.copy(loaded = true, loadError = true)
            }
        }
    }

    /** Li diu una cosa a la mascota; amb force, l'usuari tria un altre camí per al mateix missatge. */
    fun askCap(rawMessage: String, force: String = "") {
        val message = rawMessage.trim()
        if (message.isEmpty() || _cap.value.busy) return
        val source = _conn.value; val epoch = companionEpoch
        _cap.value = _cap.value.copy(busy = true, asked = message, pending = null, error = false, failedText = "", live = "")
        viewModelScope.launch {
            try {
                // En directe: la resposta creix a la bombolla mentre s'escriu
                // (abans, 8-10 s de punts i el text sencer de cop). Un servidor
                // sense la ruta respon pel POST de sempre.
                val d = withContext(Dispatchers.IO) {
                    val buf = WindStreamText()
                    try {
                        source.api().capAskStream(message, idioma.value, force) { t ->
                            buf.append(t)?.let { text ->
                                if (isCompanionOrigin(source, epoch)) _cap.update { if (it.busy) it.copy(live = text) else it }
                            }
                        }
                    } catch (_: WindStreamUnavailable) {
                        source.api().capAsk(message, idioma.value, force)
                    }
                }
                if (!isCompanionOrigin(source, epoch)) return@launch
                val st = _cap.value.state.copy(history = d.history, brief = "", briefCount = 0, pet = d.pet ?: _cap.value.state.pet)
                _cap.value = _cap.value.copy(state = st, busy = false, say = "", course = true, live = "",
                    pending = if (d.delegates) CapPending(d.message.ifBlank { message }, d) else null)
                // «Endavant»: la proposta d'abans es fa ara, sense un altre botó.
                if (d.delegates && d.confirm) capGo()
            } catch (_: Exception) {
                if (isCompanionOrigin(source, epoch)) _cap.value = _cap.value.copy(busy = false, error = true, live = "",
                    failedText = if (force.isBlank()) message else _cap.value.failedText)
            }
        }
    }

    private fun capPrefs(): android.content.SharedPreferences? =
        try { appContext.getSharedPreferences("gregal_cap", android.content.Context.MODE_PRIVATE) } catch (_: Exception) { null }

    /** Una colla convocada des del Ventet: quan arrenca, el fil hi apunta l'enllaç. */
    private var crewFromCap: String? = null

    /** Endavant: fa el que la mascota ha decidit (o l'usuari ha triat). */
    fun capGo() {
        val ui = _cap.value
        val p = ui.pending ?: return
        if (ui.going) return
        val d = p.decision
        if (d.action == "crew") {
            // «Endavant» ja és la confirmació: la colla es proposa i es convoca
            // sola (la fitxa ensenya qui hi és mentre arrenca).
            _cap.value = ui.copy(pending = null)
            crewFromCap = d.task.ifBlank { p.message }
            openCrew(d.task.ifBlank { p.message }); proposeCrew(d.task.ifBlank { p.message }, autoStart = true)
            return
        }
        val source = _conn.value; val epoch = companionEpoch
        _cap.value = ui.copy(going = true)
        viewModelScope.launch {
            try {
                val r = withContext(Dispatchers.IO) { source.api().capGo(p.message, idioma.value, d.winds, d.create, ui.course) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                // Els vents nous i la rotllana han de ser a la llista abans d'obrir-los:
                // la pantalla tanca un xat que no coneix.
                val ws = withContext(Dispatchers.IO) { source.api().getWinds() }
                val rots = withContext(Dispatchers.IO) { source.api().getRotllanes() }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _winds.value = ws; _rotllanes.value = rots
                _cap.value = _cap.value.copy(pending = null, going = false)
                loadCap() // el fil ja porta l'enllaç a la conversa
                if (r.wind.isNotBlank()) { openVentsChat("wind:" + r.wind); sendWindMessage(r.wind, p.message) }
                else if (r.rotllana.isNotBlank()) { openVentsChat("rot:" + r.rotllana); sendRotllanaMessage(r.rotllana, p.message) }
                if (r.studying.isNotEmpty()) {
                    val names = r.studying.mapNotNull { id -> ws.firstOrNull { it.id == id }?.name }
                    notify(appText("app.cap.studying", capRotllanaName(names, idioma.value)))
                }
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) {
                    _cap.value = _cap.value.copy(going = false)
                    companyError(e)
                }
            }
        }
    }

    fun capToggleCourse() { _cap.value = _cap.value.copy(course = !_cap.value.course) }

    /** Cuidar-la i jugar-hi: l'estat ve del servidor, les animacions són de l'app. */
    fun capPlay(act: String) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val (pet, say, unlocked) = withContext(Dispatchers.IO) { source.api().capPlay(act, idioma.value) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                val before = _cap.value.state.pet.level
                val now = System.currentTimeMillis()
                _cap.value = _cap.value.copy(state = _cap.value.state.copy(pet = pet), say = say, sayAt = now,
                    levelUpAt = if (pet.level > before) now else _cap.value.levelUpAt,
                    unlocked = if (unlocked.isNotEmpty()) unlocked else _cap.value.unlocked)
            } catch (_: Exception) { }
        }
    }

    /** «Ara no»: la proposta queda al fil però no torna a demanar resposta. */
    fun capDismiss() {
        val ts = _cap.value.state.history.lastOrNull()?.ts ?: 0L
        capPrefs()?.edit()?.putLong("dismissed", ts)?.apply()
        _cap.value = _cap.value.copy(pending = null, dismissedTs = ts)
    }

    /** Torna a enviar el missatge que no ha arribat. */
    fun capRetry() {
        val text = _cap.value.failedText.ifBlank { _cap.value.asked }
        if (text.isNotBlank()) askCap(text)
    }

    // --- Resums de les converses (la llista de Vents) ---

    private val _summaries = MutableStateFlow<Map<String, ConvoSummary>>(emptyMap())
    val summaries: StateFlow<Map<String, ConvoSummary>> = _summaries.asStateFlow()

    /** Últim missatge de cada conversa: la portada ja no carrega tots els fils. */
    fun loadSummaries() {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val m = withContext(Dispatchers.IO) { source.api().summaries() }
                if (isCompanionOrigin(source, epoch)) _summaries.value = m
            } catch (_: Exception) { }
        }
    }

    fun renameCap(name: String) {
        val n = name.trim()
        if (n.isEmpty()) return
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val st = withContext(Dispatchers.IO) { source.api().capRename(n) }
                if (isCompanionOrigin(source, epoch)) _cap.value = _cap.value.copy(state = _cap.value.state.copy(name = st.name))
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) companyError(e) }
        }
    }

    /** Full de «Convoca una colla»: encàrrec, proposta i estat. null = tancat. */
    data class CrewUi(val task: String = "", val loading: Boolean = false, val proposal: CrewProposal? = null,
                      val starting: Boolean = false, val error: Boolean = false,
                      /** Per què ha fallat (l'error del servidor o de la xarxa), per dir-ho a la pantalla. */
                      val errorText: String = "")
    private val _crew = MutableStateFlow<CrewUi?>(null)
    val crew: StateFlow<CrewUi?> = _crew
    /** Pas de les colles convocades que treballen ara (id de rotllana -> pas). */
    private val _crewStep = MutableStateFlow<Map<String, String>>(emptyMap())
    val crewStep: StateFlow<Map<String, String>> = _crewStep

    fun openCrew(task: String = "") { _crew.value = CrewUi(task) }
    fun closeCrew() { _crew.value = null }

    /** L'error en paraules curtes: el codi i el que diu el servidor, o la xarxa. */
    private fun crewErrorText(e: Exception): String {
        val m = (e.message ?: e.javaClass.simpleName).replace(Regex("\\s+"), " ").trim()
        return when (e) {
            is java.net.SocketTimeoutException -> appText("app.crew.errTimeout")
            is java.net.UnknownHostException, is java.net.ConnectException -> appText("app.crew.errNetwork")
            else -> m.take(180)
        }
    }

    fun proposeCrew(task: String, autoStart: Boolean = false) {
        if (task.isBlank()) return
        val source = _conn.value; val epoch = companionEpoch
        _crew.value = CrewUi(task, loading = true)
        viewModelScope.launch {
            try {
                val p = withContext(Dispatchers.IO) { source.api().crewPropose(task.trim(), idioma.value) }
                if (isCompanionOrigin(source, epoch) && _crew.value?.task == task) {
                    _crew.value = CrewUi(task, proposal = p)
                    if (autoStart) startCrew()
                }
            } catch (e: Exception) {
                if (_crew.value?.task == task) _crew.value = CrewUi(task, error = true, errorText = crewErrorText(e))
            }
        }
    }

    fun startCrew() {
        val ui = _crew.value ?: return
        val p = ui.proposal ?: return
        val source = _conn.value; val epoch = companionEpoch
        _crew.value = ui.copy(starting = true, error = false, errorText = "")
        viewModelScope.launch {
            val id = try {
                withContext(Dispatchers.IO) { source.api().crewStart(ui.task.trim(), idioma.value, p) }
            } catch (e: Exception) {
                _crew.value = ui.copy(starting = false, error = true, errorText = crewErrorText(e))
                return@launch
            }
            if (!isCompanionOrigin(source, epoch)) return@launch
            if (id.isBlank()) {
                _crew.value = ui.copy(starting = false, error = true, errorText = "no rotllana id")
                return@launch
            }
            // Ja ha començat al servidor: a partir d'aquí res no és «no s'ha
            // pogut fer» (tornar-hi crearia els vents nous dues vegades).
            _crew.value = null
            followCrew(id)
            crewFromCap?.let { task ->
                crewFromCap = null
                viewModelScope.launch {
                    try {
                        withContext(Dispatchers.IO) { source.api().capLink("rot:$id", appText("app.cap.crewLink", oneLine(task, 60))) }
                        loadCap()
                    } catch (_: Exception) { }
                }
            }
            // El fil ha de ser a la llista abans d'obrir-lo: la pantalla de
            // Vents tanca un id que no coneix.
            val list = try { withContext(Dispatchers.IO) { source.api().getRotllanes() } } catch (_: Exception) { null }
            if (!isCompanionOrigin(source, epoch)) return@launch
            loadWinds()
            if (list != null && list.any { it.id == id }) {
                _rotllanes.value = list
                loadRotllanaChat(id)
                openVentsChat("rot:$id")
            } else {
                loadRotllanes()
                notify(appText("app.crew.startedElsewhere"))
            }
        }
    }

    private val crewPolling = mutableMapOf<String, Job>()

    /** Follow only server-reported progress; retry transient failures without clearing it. */
    fun followCrew(id: String) {
        if (id.isBlank() || crewPolling[id]?.isActive == true) return
        val source = _conn.value; val epoch = companionEpoch
        val job = viewModelScope.launch(start = kotlinx.coroutines.CoroutineStart.LAZY) {
            try {
                while (true) {
                    val status = try {
                        withContext(Dispatchers.IO) { source.api().crewStatus(id) }
                    } catch (e: CancellationException) { throw e } catch (_: Exception) { null }
                    if (!isCompanionOrigin(source, epoch)) return@launch
                    if (status != null) {
                        val step = status.second
                        // Mentre la colla treballa el pas no s'esborra: si el
                        // servidor en diu un sense text propi (o encara no en
                        // diu cap) es queda l'últim conegut. Esborrar-lo feia
                        // caure la capçalera a «hi pensa» i tornar al pas, un
                        // flaix cada pols.
                        _crewStep.value = when {
                            !status.first -> _crewStep.value - id
                            step in setOf("discussing", "waiting", "working") -> _crewStep.value + (id to step)
                            else -> _crewStep.value
                        }
                        loadRotllanaChat(id)
                        if (!status.first) return@launch
                    }
                    delay(4000)
                }
            } finally {
                if (crewPolling[id] === kotlinx.coroutines.currentCoroutineContext()[Job]) crewPolling.remove(id)
            }
        }
        crewPolling[id] = job
        job.start()
    }

    // --- El calaix i el temps de cada vent (WindCalaix.kt) ---
    private val _calaix = MutableStateFlow<Map<String, List<CalaixShelf>>>(emptyMap())
    val calaix: StateFlow<Map<String, List<CalaixShelf>>> = _calaix.asStateFlow()
    private val _weather = MutableStateFlow<Map<String, WeatherForecast>>(emptyMap())
    val weather: StateFlow<Map<String, WeatherForecast>> = _weather.asStateFlow()

    fun loadCalaix(windId: String) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val got = withContext(Dispatchers.IO) { source.api().getCalaix(windId) }
                if (isCompanionOrigin(source, epoch)) _calaix.update { it + (windId to got) }
            } catch (e: Exception) {
                if (e is CancellationException) throw e
            }
        }
    }

    /** remove / clear / primary / add. Afegir un lloc el fa principal i torna a llegir el temps. */
    fun calaixAction(windId: String, shelf: String, action: String, key: String? = null, item: Map<String, Any?>? = null) {
        val source = _conn.value; val epoch = companionEpoch
        // Treure una fila es veu al moment.
        if (action == "remove" && key != null) _calaix.update { m ->
            m + (windId to m[windId].orEmpty().map { sh -> if (sh.name == shelf) sh.copy(items = sh.items.filterNot { CalaixJson.rowKey(it) == key }) else sh })
        }
        viewModelScope.launch {
            try {
                val got = withContext(Dispatchers.IO) {
                    val api = source.api()
                    var d = api.calaixAction(windId, shelf, action, key, item)
                    if (action == "add" && item?.get("primary") == true) d = api.calaixAction(windId, shelf, "primary", key, null)
                    d
                }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _calaix.update { it + (windId to got) }
                if (got.any { it.kind == "places" }) loadWeather(windId)
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                if (isCompanionOrigin(source, epoch)) { companyError(e); loadCalaix(windId) }
            }
        }
    }

    fun loadWeather(windId: String) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val got = withContext(Dispatchers.IO) { source.api().getWeather(windId) }
                if (isCompanionOrigin(source, epoch)) _weather.update { if (got.place == null) it - windId else it + (windId to got) }
            } catch (e: Exception) {
                if (e is CancellationException) throw e
            }
        }
    }

    suspend fun searchWeatherPlaces(q: String): List<WeatherPlace> {
        val source = _conn.value
        return withContext(Dispatchers.IO) { source.api().searchWeatherPlaces(q) }
    }

    fun activateSquad(id: String, lang: String, onDone: (Boolean) -> Unit = {}) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().activateSquad(id, lang) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                loadWinds(); loadWindsResum()
                onDone(true)
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                if (isCompanionOrigin(source, epoch)) { companyError(e); onDone(false) }
            }
        }
    }

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

    fun saveVentMemoria(windId: String, memoria: String, onDone: (Boolean) -> Unit = {}) {
        val source = _conn.value; val epoch = companionEpoch
        viewModelScope.launch {
            try {
                val readback = withContext(Dispatchers.IO) {
                    val api = source.api()
                    api.saveWindMemoria(windId, memoria)
                    api.getWindMemoria(windId)
                }
                if (!isCompanionOrigin(source, epoch)) return@launch
                check(readback == memoria) { "Memory readback does not match" }
                val (_, coneix) = _ventDetall.value[windId] ?: ("" to emptyList())
                _ventDetall.value = _ventDetall.value + (windId to (readback to coneix))
                onDone(true)
            } catch (e: Exception) {
                if (isCompanionOrigin(source, epoch)) {
                    companyError(e)
                    onDone(false)
                }
            }
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
        val thread = selectedWindThreads[id].orEmpty()
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().clearWindChat(id, thread) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                _windChat.value = _windChat.value + (id to emptyList())
                ventsRecency.clear(source, "wind", id)
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) companyError(e) }
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
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) companyError(e) }
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
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) companyError(e) }
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
                ventsRecency.clear(source, "wind", id)
                if (_ventsOpen.value == "wind:$id") _ventsOpen.value = null
                loadWinds(); loadRotllanes()
            } catch (e: Exception) { if (isCompanionOrigin(source, epoch)) companyError(e) }
        }
    }

    fun setActiveWind(w: Wind?) { _activeWind.value = w }

    private val _windCreator = MutableStateFlow<Pair<Boolean, Wind?>>(false to null)
    val windCreator: StateFlow<Pair<Boolean, Wind?>> = _windCreator
    fun showWindCreator(show: Boolean, wind: Wind? = null) { _windCreator.value = show to wind }

    // --- sessions d'un vent: fils separats d'un mateix vent ---

    fun loadWindSessions(windId: String) {
        val source = _conn.value
        _windSessionsLoading.value = _windSessionsLoading.value + windId
        viewModelScope.launch {
            try {
                val list = withContext(Dispatchers.IO) { source.api().windSessions(windId) }
                _windSessions.value = _windSessions.value + (windId to list)
            } catch (_: Exception) {
            } finally {
                _windSessionsLoading.value = _windSessionsLoading.value - windId
            }
        }
    }

    /** Nova sessió del vent: el fil comença de zero; l'anterior queda desada. */
    fun newWindSession(windId: String) {
        if (windId in _windBusy.value) return
        val source = _conn.value; val epoch = companionEpoch
        windHistoryRequests[windId] = (windHistoryRequests[windId] ?: 0L) + 1
        viewModelScope.launch {
            try {
                val name = withContext(Dispatchers.IO) { source.api().windSessionNew(windId) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                selectedWindThreads[windId] = name
                ventsRecency.clear(source, "wind", windId)
                // El fil obert passa a ser el nou (buit): fora la còpia vella
                // i l'esquelet, com en obrir una conversa per primer cop.
                _windChat.value = _windChat.value + (windId to emptyList())
                loadWindSessions(windId)
            } catch (e: Exception) { notify(appText("app.status.sessionNew", e.message.orEmpty())) }
        }
    }

    /** Obre una sessió del vent: passa a ser l'activa i el fil es repinta. */
    fun openWindSession(windId: String, name: String) {
        if (windId in _windBusy.value) return
        val source = _conn.value; val epoch = companionEpoch
        windHistoryRequests[windId] = (windHistoryRequests[windId] ?: 0L) + 1
        _windChatLoading.value = _windChatLoading.value + windId
        viewModelScope.launch {
            try {
                val history = withContext(Dispatchers.IO) { source.api().windSessionOpen(windId, name) }
                if (!isCompanionOrigin(source, epoch)) return@launch
                selectedWindThreads[windId] = name
                ventsRecency.clear(source, "wind", windId)
                _windChat.value = _windChat.value + (windId to history)
                cacheChat(source, windId)
                loadWindSessions(windId)
            } catch (e: Exception) {
                notify(appText("app.status.sessionResume", e.message.orEmpty()))
            } finally {
                _windChatLoading.value = _windChatLoading.value - windId
            }
        }
    }

    /** Esborra una sessió del vent (no l'activa). */
    fun deleteWindSession(windId: String, name: String) {
        val source = _conn.value
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { source.api().windSessionDelete(windId, name) }
                loadWindSessions(windId)
            } catch (e: Exception) { notify(appText("app.status.sessionResume", e.message.orEmpty())) }
        }
    }

    fun loadWindChat(windId: String) {
        val source = _conn.value; val epoch = companionEpoch
        if (_windBusy.value.contains(windId)) { _windChatLoading.value = _windChatLoading.value - windId; return }
        val request = (windHistoryRequests[windId] ?: 0L) + 1
        windHistoryRequests[windId] = request
        val skeleton = _windChat.value[windId].isNullOrEmpty()
        // Sense servidor no s'espera cap temps d'espera: la còpia, de seguida.
        // Quan torni, backOnline() torna a llegir el xat obert.
        if (!source.local && _offline.value) {
            viewModelScope.launch {
                val cached = withContext(Dispatchers.IO) { offlineStore(source)?.chat(windId) }
                if (isCompanionOrigin(source, epoch) && _windChat.value[windId].isNullOrEmpty())
                    _windChat.value = _windChat.value + (windId to OfflineLogic.withQueued(cached.orEmpty(), _outbox.value, windId))
                _windChatLoading.value = _windChatLoading.value - windId
            }
            return
        }
        if (skeleton) _windChatLoading.value = _windChatLoading.value + windId
        viewModelScope.launch {
            // La còpia primer (sense esquelet), en paral·lel: la petició al servidor
            // surt igual de seguida i, quan arriba, la substitueix.
            if (skeleton && !source.local) launch {
                val cached = withContext(Dispatchers.IO) { offlineStore(source)?.chat(windId) }
                if (!cached.isNullOrEmpty() && isCompanionOrigin(source, epoch) && request == windHistoryRequests[windId] &&
                    _windChat.value[windId].isNullOrEmpty()) {
                    _windChat.value = _windChat.value + (windId to OfflineLogic.withQueued(cached, _outbox.value, windId))
                    _windChatLoading.value = _windChatLoading.value - windId
                }
            }
            try {
                val selected = selectedWindThreads[windId].orEmpty()
                val (thread, history) = withContext(Dispatchers.IO) { source.api().getWindThread(windId, selected) }
                if (isCompanionOrigin(source, epoch) && request == windHistoryRequests[windId] &&
                    !_windBusy.value.contains(windId)) {
                    if (thread.isNotEmpty()) selectedWindThreads[windId] = thread
                    _windChat.value = _windChat.value + (windId to OfflineLogic.withQueued(
                        ventsRecency.restore(source, "wind", windId, history), _outbox.value, windId))
                    cacheChat(source, windId)
                }
            } catch (e: Exception) {
                if (!isCompanionOrigin(source, epoch)) return@launch
                if (!source.local && isUnreachable(e)) {
                    // Sense servidor: la còpia del mòbil, amb el que espera a sortir al final.
                    val cached = withContext(Dispatchers.IO) { offlineStore(source)?.chat(windId) }
                    if (_windChat.value[windId].isNullOrEmpty() && !_windBusy.value.contains(windId))
                        _windChat.value = _windChat.value + (windId to OfflineLogic.withQueued(cached.orEmpty(), _outbox.value, windId))
                    goOffline(source)
                } else companyError(e)
            }
            finally { if (skeleton) _windChatLoading.value = _windChatLoading.value - windId }
        }
    }

    /**
     * Bombolla de directe: crea o actualitza l'últim missatge en streaming
     * del torn (fase o text parcial). El done la substitueix per la final.
     */
    private fun setStreamingBubble(windId: String, text: String, rotllana: Boolean = false) {
        val bubble = PetMsg("pet", text, streaming = true)
        val histories = if (rotllana) _rotllanaChat else _windChat
        histories.update { current ->
            val cur = current[windId].orEmpty()
            val next = if (cur.lastOrNull()?.streaming == true) {
                cur.toMutableList().apply { this[lastIndex] = bubble }
            } else cur + bubble
            current + (windId to next)
        }
    }

    /** Tanca la bombolla en directe com a missatge del vent que parlava. */
    private fun commitRotllanaStreamBubble(rotllanaId: String, buf: String, speakerId: String) {
        if (buf.isBlank()) return
        val speaker = _winds.value.firstOrNull { it.id == speakerId }
        val cur = _rotllanaChat.value[rotllanaId].orEmpty().filterNot { it.streaming }
        _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to
            (cur + PetMsg("pet", buf, speakerId, speaker?.name.orEmpty(), ts = System.currentTimeMillis())))
    }

    /**
     * Tanca la bombolla en directe com a missatge del vent (una sola veu).
     * Serveix a les fites del torn: una eina pel mig no esborra el que ja
     * s'ha llegit, que és el que passava abans («tokens were speculative»).
     */
    private fun commitWindStreamBubble(windId: String, buf: String) {
        if (buf.isBlank()) return
        val w = _winds.value.firstOrNull { it.id == windId }
        val cur = _windChat.value[windId].orEmpty().filterNot { it.streaming }
        // Sense segell: l'historial del servidor tampoc no en porta, i el rebut
        // del torn el posa qui tanca el fil (ventsRecency).
        _windChat.value = _windChat.value + (windId to
            (cur + PetMsg("pet", buf, windId, w?.name.orEmpty())))
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

    /** Atura el torn en curs d'un vent (POST /api/winds/cancel). */
    fun cancelWindTurn(windId: String) = cancelTurn("wind", windId)

    /** Atura el torn en curs d'una rotllana. */
    fun cancelRotllanaTurn(rotllanaId: String) = cancelTurn("rotllana", rotllanaId)

    // Torns aturats per la persona: el que tanca no és cap error i no ha de
    // sortir la targeta de «torn interromput».
    private val aturats = java.util.Collections.synchronizedSet(mutableSetOf<String>())

    /** aturat diu si aquest id l'ha aturat la persona (i ho consumeix). */
    private fun aturatAra(id: String): Boolean = aturats.remove(id)

    private fun cancelTurn(kind: String, id: String) {
        val source = _conn.value
        aturats.add(id)
        viewModelScope.launch {
            runCatching {
                GregalApi(baseUrlOf(source.host, source.port), source.token).cancelTurn(
                    windId = if (kind == "wind") id else "", rotllanaId = if (kind == "rotllana") id else "",
                )
            }
            if (isCompanionOrigin(source, companionEpoch)) {
                if (kind == "wind") { _windBusy.value = _windBusy.value - id; endLive(id) }
                else { _rotllanaBusy.value = _rotllanaBusy.value - id; endLive(id) }
                notify(appText("app.vents.stopped"))
            }
        }
    }

    // Steer: missatge lliurat al torn que treballa (POST /api/winds/steer).
    // La bombolla surt al xat com a missatge d'usuari normal; si el torn
    // ja s'ha acabat al servidor (409), es reenvia com a missatge nou.
    private fun steerWind(windId: String, msg: String, queuedTs: Long = 0L) {
        val source = _conn.value
        _windChat.value = _windChat.value + (windId to (_windChat.value[windId].orEmpty() + PetMsg("user", msg, ts = if (queuedTs > 0) queuedTs else System.currentTimeMillis())))
        viewModelScope.launch {
            val delivered = withContext(Dispatchers.IO) {
                runCatching { GregalApi(baseUrlOf(source.host, source.port), source.token).steerTurn(windId = windId, text = msg) }.getOrDefault(false)
            }
            if (!isCompanionOrigin(source, companionEpoch)) return@launch
            if (delivered) notify(appText("app.vents.steerSent"))
            else {
                notify(appText("app.vents.steerFailed"))
                sendWindMessage(windId, msg, queuedTs)
            }
        }
    }

    private fun steerRotllana(rotllanaId: String, msg: String) {
        val source = _conn.value
        _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to (_rotllanaChat.value[rotllanaId].orEmpty() + PetMsg("user", msg, ts = System.currentTimeMillis())))
        viewModelScope.launch {
            val delivered = withContext(Dispatchers.IO) {
                runCatching { GregalApi(baseUrlOf(source.host, source.port), source.token).steerTurn(rotllanaId = rotllanaId, text = msg) }.getOrDefault(false)
            }
            if (!isCompanionOrigin(source, companionEpoch)) return@launch
            if (delivered) notify(appText("app.vents.steerSent"))
            else {
                notify(appText("app.vents.steerFailed"))
                sendRotllanaMessage(rotllanaId, msg)
            }
        }
    }

    /**
     * Tanca un torn que ha acabat amb error i torna l'error per a la
     * notificació. Si l'ha aturat la persona no és cap error: ni targeta de
     * «torn interromput», ni avís, ni notificació de «ha fallat».
     */
    private fun turnFailed(source: Conn, epoch: Long, id: String, rotllana: Boolean, e: Exception): String {
        val stopped = aturatAra(id)
        if (stopped) return ""
        if (isCompanionOrigin(source, epoch)) {
            interruptedCompanion(id, rotllana, turnFailureText(_idioma.value, e.message.orEmpty()))
            companyError(e)
        }
        return e.message.orEmpty().ifBlank { "interrupted" }
    }

    /** Close transient UI honestly; never replay a possibly executed operation. */
    private fun interruptedCompanion(id: String, rotllana: Boolean, reason: String = "") {
        val state = _windLive.value[id]
        val store = if (rotllana) _rotllanaChat else _windChat
        val history = store.value[id].orEmpty()
        val kept = if (history.lastOrNull()?.streaming == true) {
            if (state?.receivedText == true) history.dropLast(1) + history.last().copy(streaming = false)
            else history.dropLast(1)
        } else history
        val explanation = reason.ifBlank {
            if (_idioma.value == "en") "Turn interrupted. It has not been submitted again. Check the saved work before retrying."
            else "Torn interromput. No l'he repetit automàticament. Revisa la feina desada abans de tornar-ho a provar."
        }
        store.value = store.value + (id to (kept + PetMsg("system", explanation, kind = "error")))
    }

    /** Fase del directe en la llengua de l'app. */
    private fun streamPhaseText(phase: String): String = when (phase) {
        "searching" -> appText("app.vents.streamSearching")
        "browsing" -> appText("app.vents.streamBrowsing")
        "scheduling" -> appText("app.vents.streamScheduling")
        "thinking" -> appText("app.vents.streamThinking")
        "speaker" -> appText("app.vents.streamSpeaker")
        else -> appText("app.vents.streamWorking")
    }

    fun sendWindMessage(windId: String, text: String) = sendWindMessage(windId, text, 0L)

    private fun sendWindMessage(windId: String, text: String, queuedTs: Long) {
        val msg = text.trim()
        if (msg.isEmpty()) return
        // El torn treballa: el missatge hi entra com a steer (sense aturar
        // res); si el servidor ja no té el torn viu, com a missatge nou.
        if (_windBusy.value.contains(windId)) {
            steerWind(windId, msg, queuedTs)
            return
        }
        val source = _conn.value; val epoch = companionEpoch
        // Sense servidor: a la cua, i surt sol quan torni (en l'ordre en què l'has escrit).
        if (!source.local && _offline.value) {
            enqueueOffline(source, windId, msg, if (queuedTs > 0) queuedTs else System.currentTimeMillis())
            return
        }
        var reached = false
        val thread = selectedWindThreads[windId].orEmpty()
        windHistoryRequests[windId] = (windHistoryRequests[windId] ?: 0L) + 1
        _windChat.value = _windChat.value + (windId to (_windChat.value[windId].orEmpty() + PetMsg("user", msg, ts = System.currentTimeMillis())))
        _windBusy.value = _windBusy.value + windId
        live(windId) { WindLive() }
        viewModelScope.launch {
            var answered = false
            var answer = ""
            var partial = false
            var turnError = ""
            val target = "wind:$windId"
            val displayName = _winds.value.firstOrNull { it.id == windId }?.name.orEmpty().ifBlank { "Vent" }
            GregalNotifications.workStarted(appContext, target, displayName)
            try {
                try {
                    // Directe primer: el text creix a la bombolla; si el
                    // servidor no té /stream (o cau), clàssic de reserva.
                    val buf = WindStreamText()
                    val r = withContext(Dispatchers.IO) {
                        source.api().windChatStream(windId, msg, session = thread) { e ->
                            reached = true
                            if (!isCompanionOrigin(source, epoch)) return@windChatStream
                            when (e) {
                                is GregalApi.WindStreamEvent.Phase -> {
                                    // Una eina pel mig no esborra el que ja s'ha llegit.
                                    commitWindStreamBubble(windId, buf.take())
                                    live(windId) { it.copy(phase = e.phase, writing = false, receivedText = false, tool = "") }
                                    dropStreamingBubble(windId)
                                }
                                is GregalApi.WindStreamEvent.Activity -> {
                                    live(windId) { it.copy(phase = e.phase, tool = e.tool, step = e.step, writing = false) }
                                }
                                is GregalApi.WindStreamEvent.Token -> {
                                    buf.append(e.token)?.let { text ->
                                        live(windId) { it.copy(writing = true, receivedText = true) }
                                        setStreamingBubble(windId, text)
                                    }
                                }
                                else -> handleWindAskEvent(e)
                            }
                        }
                    }
                    if (isCompanionOrigin(source, epoch)) {
                        dropStreamingBubble(windId)
                        _windChat.value = _windChat.value + (windId to ventsRecency.completed(source, "wind", windId,
                            _windChat.value[windId].orEmpty() + PetMsg("pet", r.text, parcial = r.parcial, ts = System.currentTimeMillis()), System.currentTimeMillis()))
                        partial = r.parcial
                        answered = r.text.isNotBlank()
                        answer = r.text
                        // Prova visible que el vent ha actuat de debò: només sona
                        // quan l'eina remind s'ha cridat (mai d'una promesa buida).
                        showTurnActions(windId, r)
                        WindWatch.reminded(appContext, r.remindAt, remote = !source.local) // B7: permís en context
                    }
                } catch (_: WindStreamUnavailable) {
                    dropStreamingBubble(windId)
                    val r = withContext(Dispatchers.IO) { source.api().windChat(windId, msg, session = thread) }
                    if (isCompanionOrigin(source, epoch)) {
                        _windChat.value = _windChat.value + (windId to ventsRecency.completed(source, "wind", windId,
                            _windChat.value[windId].orEmpty() + PetMsg("pet", r.text, parcial = r.parcial, ts = System.currentTimeMillis()), System.currentTimeMillis()))
                        partial = r.parcial
                        answered = r.text.isNotBlank()
                        answer = r.text
                        showTurnActions(windId, r)
                        WindWatch.reminded(appContext, r.remindAt, remote = !source.local) // B7: permís en context
                    }
                }
                // El vent pot haver programat un avís (remind): posa al dia
                // els jobs perquè l'alarma soni encara que tanquis l'app.
                if (isCompanionOrigin(source, epoch)) loadJobs()
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                if (!source.local && !reached && isUnreachable(e) && isCompanionOrigin(source, epoch)) {
                    // No ha arribat mai al servidor: res a repetir, va a la cua.
                    val cur = _windChat.value[windId].orEmpty().filterNot { it.streaming }
                    val i = cur.indexOfLast { it.from == "user" && it.text == msg }
                    _windChat.value = _windChat.value + (windId to (if (i >= 0) cur.take(i) + cur.drop(i + 1) else cur))
                    goOffline(source)
                    enqueueOffline(source, windId, msg, if (queuedTs > 0) queuedTs else System.currentTimeMillis())
                } else turnError = turnFailed(source, epoch, windId, false, e)
            } finally {
                aturats.remove(windId) // un «atura» que arriba tard no amaga el proper error
                if (isCompanionOrigin(source, epoch)) endWindReply(windId, answered && !partial && turnError.isEmpty())
                if (answered && isCompanionOrigin(source, epoch)) cacheChat(source, windId)
                if (isCompanionOrigin(source, epoch) && !_offline.value) flushOutbox()
                GregalNotifications.workFinished(appContext, answered && isCompanionOrigin(source, epoch), msg, answer,
                    target, displayName, partial, if (isCompanionOrigin(source, epoch)) turnError else "")
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
        val thread = selectedWindThreads[windId].orEmpty()
        windHistoryRequests[windId] = (windHistoryRequests[windId] ?: 0L) + 1
        _windBusy.value = _windBusy.value + windId
        live(windId) { WindLive() }
        viewModelScope.launch {
            var answered = false
            var answer = ""
            var partial = false
            var turnError = ""
            val target = "wind:$windId"
            val displayName = _winds.value.firstOrNull { it.id == windId }?.name.orEmpty().ifBlank { "Vent" }
            GregalNotifications.workStarted(appContext, target, displayName)
            try {
                try {
                    val buf = WindStreamText()
                    val r = withContext(Dispatchers.IO) {
                        source.api().windChatStream(windId, "", cont = true, session = thread) { e ->
                            if (!isCompanionOrigin(source, epoch)) return@windChatStream
                            when (e) {
                                is GregalApi.WindStreamEvent.Phase -> {
                                    // Una eina pel mig no esborra el que ja s'ha llegit.
                                    commitWindStreamBubble(windId, buf.take())
                                    live(windId) { it.copy(phase = e.phase, writing = false, receivedText = false, tool = "") }
                                    dropStreamingBubble(windId)
                                }
                                is GregalApi.WindStreamEvent.Activity -> {
                                    live(windId) { it.copy(phase = e.phase, tool = e.tool, step = e.step, writing = false) }
                                }
                                is GregalApi.WindStreamEvent.Token -> {
                                    buf.append(e.token)?.let { text ->
                                        live(windId) { it.copy(writing = true, receivedText = true) }
                                        setStreamingBubble(windId, text)
                                    }
                                }
                                else -> handleWindAskEvent(e)
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
                        _windChat.value = _windChat.value + (windId to if (r.text.isBlank()) merged else
                            ventsRecency.completed(source, "wind", windId, merged, System.currentTimeMillis()))
                        partial = r.parcial
                        if (r.text.isNotBlank()) { answered = true; answer = r.text }
                        showTurnActions(windId, r)
                        WindWatch.reminded(appContext, r.remindAt, remote = !source.local) // B7: permís en context
                    }
                } catch (_: WindStreamUnavailable) {
                    dropStreamingBubble(windId)
                    val r = withContext(Dispatchers.IO) { source.api().windChat(windId, "", cont = true, session = thread) }
                    if (isCompanionOrigin(source, epoch)) {
                        val cur = _windChat.value[windId].orEmpty()
                        val idx = cur.indexOfLast { it.from == "pet" && !it.streaming }
                        val merged = if (idx >= 0) {
                            cur.toMutableList().also { it[idx] = it[idx].copy(text = it[idx].text + r.text, parcial = r.parcial) }
                        } else cur + PetMsg("pet", r.text, parcial = r.parcial)
                        _windChat.value = _windChat.value + (windId to if (r.text.isBlank()) merged else
                            ventsRecency.completed(source, "wind", windId, merged, System.currentTimeMillis()))
                        partial = r.parcial
                        if (r.text.isNotBlank()) { answered = true; answer = r.text }
                        showTurnActions(windId, r)
                        WindWatch.reminded(appContext, r.remindAt, remote = !source.local) // B7: permís en context
                    }
                }
                if (isCompanionOrigin(source, epoch)) loadJobs()
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                turnError = turnFailed(source, epoch, windId, false, e)
            } finally {
                aturats.remove(windId) // un «atura» que arriba tard no amaga el proper error
                if (isCompanionOrigin(source, epoch)) endWindReply(windId, answered && !partial && turnError.isEmpty())
                GregalNotifications.workFinished(appContext, answered && isCompanionOrigin(source, epoch), "", answer,
                    target, displayName, partial, if (isCompanionOrigin(source, epoch)) turnError else "")
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
        val before = _rotllanaChat.value[rotllanaId].orEmpty()
        rotllanaHistoryRequests[rotllanaId] = (rotllanaHistoryRequests[rotllanaId] ?: 0L) + 1
        _rotllanaBusy.value = _rotllanaBusy.value + rotllanaId
        live(rotllanaId) { WindLive() }
        viewModelScope.launch {
            var answered = false
            var answer = ""
            var partial = false
            var turnError = ""
            val target = "rot:$rotllanaId"
            val displayName = _rotllanes.value.firstOrNull { it.id == rotllanaId }?.name.orEmpty().ifBlank { if (_idioma.value == "en") "Group" else "Rotllana" }
            GregalNotifications.workStarted(appContext, target, displayName)
            try {
                try {
                    val buf = WindStreamText()
                    val (_, replyText, hist) = withContext(Dispatchers.IO) {
                        source.api().rotllanaChatStream(rotllanaId, "", wind, cont = true) { e ->
                            if (!isCompanionOrigin(source, epoch)) return@rotllanaChatStream
                            when (e) {
                                is GregalApi.WindStreamEvent.Phase -> {
                                    // Una eina pel mig no esborra el que ja s'ha llegit.
                                    commitRotllanaStreamBubble(rotllanaId, buf.take(), wind)
                                    live(rotllanaId) { it.copy(phase = e.phase, writing = false, receivedText = false, tool = "") }
                                    dropStreamingBubble(rotllanaId, rotllana = true)
                                }
                                is GregalApi.WindStreamEvent.Activity -> {
                                    live(rotllanaId) { it.copy(phase = e.phase, tool = e.tool, step = e.step, writing = false) }
                                }
                                is GregalApi.WindStreamEvent.Token -> {
                                    buf.append(e.token)?.let { text ->
                                        live(rotllanaId) { it.copy(writing = true, receivedText = true) }
                                        setStreamingBubble(rotllanaId, text, rotllana = true)
                                    }
                                }
                                else -> handleWindAskEvent(e)
                            }
                        }
                    }
                    if (!isCompanionOrigin(source, epoch)) return@launch
                    dropStreamingBubble(rotllanaId, rotllana = true)
                    _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to if (hist.isEmpty()) _rotllanaChat.value[rotllanaId].orEmpty() else
                            ventsRecency.completed(source, "rot", rotllanaId, hist, System.currentTimeMillis(), before))
                    partial = hist.lastOrNull { it.from == "pet" }?.parcial == true
                    replyText.let { if (it.isNotBlank()) { answered = true; answer = it } }
                } catch (_: WindStreamUnavailable) {
                    dropStreamingBubble(rotllanaId, rotllana = true)
                    val (_, replyText, hist) = withContext(Dispatchers.IO) { source.api().rotllanaChat(rotllanaId, "", wind, cont = true) }
                    if (isCompanionOrigin(source, epoch))
                        _rotllanaChat.value = _rotllanaChat.value + (rotllanaId to if (hist.isEmpty()) _rotllanaChat.value[rotllanaId].orEmpty() else
                            ventsRecency.completed(source, "rot", rotllanaId, hist, System.currentTimeMillis(), before))
                    partial = hist.lastOrNull { it.from == "pet" }?.parcial == true
                    replyText.let { if (it.isNotBlank()) { answered = true; answer = it } }
                }
                if (isCompanionOrigin(source, epoch)) loadJobs()
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                turnError = turnFailed(source, epoch, rotllanaId, true, e)
            } finally {
                aturats.remove(rotllanaId) // un «atura» que arriba tard no amaga el proper error
                if (isCompanionOrigin(source, epoch)) { _rotllanaBusy.value = _rotllanaBusy.value - rotllanaId; endLive(rotllanaId) }
                GregalNotifications.workFinished(appContext, answered && isCompanionOrigin(source, epoch), "", answer,
                    target, displayName, partial, if (isCompanionOrigin(source, epoch)) turnError else "")
            }
        }
    }

    /**
     * «Torna-ho a provar» de la targeta d'error: reenvia l'últim text de
     * l'usuari pel send actual (mateix fil, sense escriure res de nou).
     */
    fun retryLastMainTurn() {
        val last = _msgs.value.lastOrNull { it.role == "user" && it.text.isNotBlank() }?.text.orEmpty()
        if (last.isNotBlank()) submit(last)
    }

    fun retryLastWindTurn(windId: String) {
        val last = _windChat.value[windId]?.lastOrNull { it.from == "user" }?.text.orEmpty()
        if (last.isNotBlank()) sendWindMessage(windId, last)
    }

    /** El mateix per a la rotllana: reenvia l'últim text pel send actual. */
    fun retryLastRotllanaTurn(rotllanaId: String, wind: String = "") {
        val last = _rotllanaChat.value[rotllanaId]?.lastOrNull { it.from == "user" }?.text.orEmpty()
        if (last.isNotBlank()) sendRotllanaMessage(rotllanaId, last, wind)
    }

    private val _transferBusy = MutableStateFlow(false)
    /** S'està copiant una conversa al servidor (export + import + verificació). */
    val transferBusy: StateFlow<Boolean> = _transferBusy

    /**
     * La conversa clonada viu a l'altra banda: cap endavant, mai enrere.
     * Reté l'últim transferId + bundle per conversa: reintentar el mateix
     * traspàs no clona dues vegades (el servidor el reprodueix).
     */
    private val transferAttempts = mutableMapOf<String, Pair<String, String>>()

    /**
     * Hi ha una altra instància on continuar: ets al mòbil i hi ha un
     * servidor desat amb credencials. Al servidor no hi ha on delegar.
     */
    fun canTransferToServer(): Boolean = _conn.value.local && hasSavedServer()

    /**
     * «Continua al servidor»: COPIA la conversa (vent o rotllana) al servidor
     * desat — historial, perfil, memòria/coneixements/skills si es marquen — i
     * obre la conversa clonada allà. L'original d'aquí no es toca ni s'esborra.
     * Si res falla (export, import, verificació), no canvia ni de mode ni de
     * conversa: avisa i et quedes on eres.
     */
    /**
     * El vent del servidor que es diu igual que aquest del mòbil (sense
     * distingir majúscules ni espais), per oferir d'afegir-hi la conversa en
     * lloc de crear-ne un segon. Null si no n'hi ha o el servidor no respon.
     */
    suspend fun serverNamesake(kind: String, id: String): Wind? {
        if (kind != "wind") return null
        val name = _winds.value.firstOrNull { it.id == id }?.name?.trim()?.lowercase().orEmpty()
        val prefs = prefsOrNull ?: return null
        if (name.isEmpty() || !canTransferToServer()) return null
        val dest = Conn(prefs.host, prefs.port, prefs.token, prefs.usuari)
        return try {
            withContext(Dispatchers.IO) { dest.api().getWinds() }.firstOrNull { it.name.trim().lowercase() == name }
        } catch (e: Exception) { if (e is CancellationException) throw e; null }
    }

    fun transferConversationToServer(kind: String, id: String, memory: Boolean, knowledge: Boolean, skills: Boolean, into: String = "") {
        if (_transferBusy.value) return
        if (!canTransferToServer()) return
        if (workPreventsOriginSwitch()) { warnOriginBusy(); return }
        val prefs = prefsOrNull ?: return
        val source = _conn.value
        val dest = Conn(prefs.host, prefs.port, prefs.token, prefs.usuari)
        _transferBusy.value = true
        notify(appText("app.vents.transferWorking"))
        viewModelScope.launch {
            try {
                val bundle = withContext(Dispatchers.IO) {
                    source.api().exportWindsTransfer(kind, id, memory, knowledge, skills).toString()
                }
                val key = "$kind:$id:$into"
                val transferId = transferAttempts[key]
                    ?.takeIf { it.second == bundle }?.first
                    ?: UUID.randomUUID().toString().replace("-", "")
                if (transferAttempts[key]?.second != bundle) transferAttempts[key] = transferId to bundle
                val result = withContext(Dispatchers.IO) {
                    dest.api().importWindsTransfer(transferId, org.json.JSONObject(bundle), into)
                }
                val newId = result.optString("id")
                val verifyPath = result.optString("verifyPath")
                if (newId.isBlank() || verifyPath.isBlank()) throw ApiException("empty transfer result")
                val check = withContext(Dispatchers.IO) { dest.api().getTransferVerify(verifyPath).toString() }
                if (!check.contains(newId)) throw ApiException("destination has no $newId yet")
                withContext(Dispatchers.IO) { dest.api().getWinds() }
                useRemote(dest)
                connect(retries = 2) { go(Screen.Company) }
                loadWinds(); loadRotllanes()
                openVentsChat((if (kind == "rot") "rot:" else "wind:") + newId)
                notify(if (into.isNotBlank()) appText("app.vents.transferJoined") else appText("app.vents.transferDone"))
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                notify(appText("app.vents.transferFailed", friendlyRunError(e.message.orEmpty()).ifBlank { e.message.orEmpty() }))
            } finally {
                _transferBusy.value = false
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
            } catch (e: Exception) { companyError(e) }
        }
    }

    // Xat amb mascota: missatges en memòria + enviament
    data class PetMsg(val from: String, val text: String, val speakerId: String = "", val speakerName: String = "", val streaming: Boolean = false, val parcial: Boolean = false, val ts: Long = 0L,
                      /** Fil d'una colla: "debate" (es plega), "question" (espera resposta), "result". */
                      val kind: String = "",
                      /** Id estable del missatge al servidor (buit en servidors antics o missatges locals). */
                      val id: String = "",
                      /** B6: run i resultats d'un missatge proactiu; obren «per què ho has fet?». */
                      val relatedRunId: Long = 0L,
                      val resultRefs: List<String> = emptyList()) {
        val proactive: Boolean get() = from != "user" && id.isNotBlank() && (relatedRunId > 0 || resultRefs.isNotEmpty())
    }

    private val _petChat = MutableStateFlow<Map<String, List<PetMsg>>>(emptyMap())
    val petChat: StateFlow<Map<String, List<PetMsg>>> = _petChat

    private val _petChatBusy = MutableStateFlow<Set<String>>(emptySet())
    val petChatBusy: StateFlow<Set<String>> = _petChatBusy

    fun loadPetChat(petId: String) {
        viewModelScope.launch {
            try {
                val history = withContext(Dispatchers.IO) { _conn.value.api().getPetChat(petId) }
                _petChat.value = _petChat.value + (petId to history)
            } catch (e: Exception) { companyError(e) }
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
            } catch (e: Exception) { companyError(e) }
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
        // Els jobs del servidor són de l'administrador: un altre compte rep 403.
        if (!source.local && !_isAdmin.value) { _jobs.value = emptyList(); return }
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

    /** Pantalla pendent que ha demanat una notificació. */
    private val _openScreen = MutableStateFlow<String?>(null)

    /**
     * Obre la pantalla que demana una notificació («chat», «office», «jobs»,
     * «vents»). Si l'app encara no està connectada, es recorda i s'aplica en
     * connectar: l'avís no es perd.
     */
    fun openScreenFromNotification(name: String) {
        val screen = pantallaDeNotificacio(name) ?: return
        if (_connected.value) go(screen) else _openScreen.value = name
    }

    private fun pantallaDeNotificacio(name: String): Screen? = when (name) {
        "chat" -> Screen.Chat
        "office" -> Screen.Office
        "jobs" -> Screen.Jobs
        "vents", "company" -> Screen.Company
        else -> null
    }

    fun openRunFromNotification(id: String) {
        _openRun.value = id
        if (_connected.value) go(Screen.Jobs)
    }

    /**
     * Tocar un avís obre la conversa a la instància que la va provocar
     * (aquest mòbil o el servidor desat). Un origen desconegut o que ja no
     * està desat no s'obre enlloc: s'avisa i prou, mai sota la instància
     * actual. Canviar d'instància amb feina en marxa s'avisa i s'espera.
     */
    fun openConversationFromNotification(originId: String, target: String) {
        val p = prefsOrNull
        val conn = p?.let { NotificationOrigins.resolve(it, originId) }
        if (conn == null) {
            notify(NotificationOrigins.unavailableMessage(_idioma.value))
            return
        }
        val same = conn.local == _conn.value.local &&
            (conn.local || (conn.host == _conn.value.host && conn.port == _conn.value.port && conn.token == _conn.value.token))
        if (!same) {
            if (workPreventsOriginSwitch()) { warnOriginBusy(); return }
            if (conn.local) useThisDevice() else useRemote(conn)
        }
        when {
            target.startsWith("wind:") || target.startsWith("rot:") -> openVentsChat(target)
            target.startsWith("run:") -> openRunFromNotification(target.removePrefix("run:"))
            target.startsWith("screen:") -> openScreenFromNotification(target.removePrefix("screen:"))
        }
    }

    fun openRunShown() { _openRun.value = null }

    /** Després de connectar: la pantalla de la notificació tocada o, per
     *  defecte, on es comença a xerrar. Amb Vents net (sense les eines de
     *  programació) és la portada dels vents; amb elles, el xat general. */
    private fun landing(): Screen {
        val home = if (_simpleVents.value) Screen.Company else Screen.Chat
        val screen = when {
            _openRun.value != null -> Screen.Jobs
            _openScreen.value != null -> pantallaDeNotificacio(_openScreen.value!!) ?: home
            else -> home
        }
        _openScreen.value = null
        return screen
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

    fun runJobNow(id: String, onDone: (ok: Boolean, message: String?) -> Unit = { _, _ -> }) {
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { _conn.value.api().runJob(id) }
                notify(appText("app.status.jobStarted"))
                onDone(true, null)
            } catch (e: Exception) {
                notify(appText("app.status.job", e.message.orEmpty()))
                onDone(false, e.message)
            }
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
