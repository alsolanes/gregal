package xyz.solanes.gregal

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.compose.setContent
import androidx.compose.animation.AnimatedContent
import androidx.activity.viewModels
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.verticalScroll
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.Row
import androidx.compose.ui.Alignment
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material.icons.rounded.ChatBubbleOutline
import androidx.compose.material.icons.rounded.Construction
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.automirrored.rounded.Logout
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Tag
import androidx.compose.material.icons.rounded.Code
import androidx.compose.material.icons.rounded.AccountTree
import androidx.compose.material.icons.rounded.Schedule
import androidx.compose.material.icons.rounded.Flag
import androidx.compose.material.icons.rounded.Forum
import androidx.compose.material.icons.automirrored.rounded.Undo
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.DateRange
import androidx.compose.material.icons.rounded.Description
import androidx.compose.material.icons.rounded.ForkRight
import androidx.compose.material.icons.rounded.List
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material.icons.rounded.Settings
import androidx.compose.material.icons.rounded.Share
import androidx.compose.material.icons.rounded.Star
import androidx.compose.material3.DrawerValue
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalDrawerSheet
import androidx.compose.material3.ModalNavigationDrawer
import androidx.compose.material3.NavigationDrawerItem
import androidx.compose.material3.Text
import androidx.compose.material3.rememberDrawerState
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.SideEffect
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberCoroutineScope
import androidx.activity.compose.BackHandler
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch

/** Connexió desada (SharedPreferences: host, port, token). */
class Prefs(val ctx: Context) {
    private val p = ctx.getSharedPreferences("gregal", Context.MODE_PRIVATE)
    val host: String get() = p.getString("host", "") ?: ""
    val port: String get() = p.getString("port", "8097") ?: "8097"
    val token: String get() = p.getString("token", "") ?: ""
    /**
     * Nom de l'usuari que ha entrat. Amb el servidor multiusuari el token
     * pertany a algu, i la salutacio i les carpetes depenen de qui sigui.
     */
    var usuari: String
        get() = p.getString("usuari", "") ?: ""
        set(v) { p.edit().putString("usuari", v.trim()).apply() }
    fun save(host: String, port: String, token: String, usuari: String = "") {
        p.edit().putString("host", host.trim()).putString("port", port.trim())
            .putString("token", token.trim())
            .putString("usuari", usuari.trim()).apply()
    }
    // Aparença i llengua, com les prefs de la web (gregal_tema/gregal_lang).
    var tema: String
        get() = p.getString("tema", "sistema") ?: "sistema"
        set(v) { p.edit().putString("tema", v).apply() }
    /** Missing/automatic preference follows only the device's primary locale. */
    val idiomaChoice: String
        get() = p.getString("idioma", "system").let { if (it == "ca" || it == "en") it else "system" }
    var idioma: String
        get() = when (idiomaChoice) {
            "ca" -> "ca"
            "en" -> "en"
            else -> if (ctx.resources.configuration.locales[0]?.language == "ca") "ca" else "en"
        }
        set(v) { p.edit().putString("idioma", if (v == "ca" || v == "en") v else "system").apply() }
    /** Esborrany del composer: el que havies escrit no es perd. */
    var esborrany: String
        get() = p.getString("esborrany", "") ?: ""
        set(v) { p.edit().putString("esborrany", v).apply() }
    /** versionCode d'una actualització que has decidit silenciar (0 = cap). */
    var updateSkipped: Int
        get() = p.getInt("updateSkipped", 0)
        set(v) { p.edit().putInt("updateSkipped", v).apply() }
    /**
     * Sessió del servidor pròpia d'aquest mòbil. Sense això l'app anava a la
     * sessió `default`, que és la pestanya «principal» de la web: veies la
     * conversa de l'escriptori i «nova conversa» esborrava la seua.
     */
    var appSession: String
        get() = p.getString("appSession", "mobil") ?: "mobil"
        set(v) { p.edit().putString("appSession", v).apply() }

    /**
     * On corre Gregal: TARGET_LOCAL (el backend dins l'app) o TARGET_REMOTE
     * (un servidor en un altre ordinador). Sense valor desat: qui ja tenia
     * un servidor configurat (versions anteriors) continua amb el remot; una
     * instal·lació nova comença en local.
     */
    var target: String
        get() = p.getString("target", null) ?: if (host.isNotBlank()) TARGET_REMOTE else TARGET_LOCAL
        set(v) { p.edit().putString("target", v).apply() }
    /** Port i token del servidor local: es reutilitzen entre arrencades. */
    var localPort: Int
        get() = p.getInt("localPort", 0)
        set(v) { p.edit().putInt("localPort", v).apply() }
    var localToken: String
        get() = p.getString("localToken", "") ?: ""
        set(v) { p.edit().putString("localToken", v).apply() }
    /** Conversa on era l'app en local, per reprendre-la si el backend es reinicia. */
    var localSession: String
        get() = p.getString("localSession", "") ?: ""
        set(v) { p.edit().putString("localSession", v).apply() }

    companion object {
        const val TARGET_LOCAL = "local"
        const val TARGET_REMOTE = "remote"
    }
}

class MainActivity : ComponentActivity() {
    private val vm: GregalViewModel by viewModels()
    private val notificationPermission = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (!granted) vm.notify(tr(vm.idioma.value, "app.notifications.permissionMissing"))
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // El tema real es pinta al setContent; aquí `fons` (#13151B) per al
        // flaix inicial.
        window.statusBarColor = DarkPal.deepTop.toArgb()
        window.navigationBarColor = DarkPal.deepTop.toArgb()
        // Context d'aplicació: el ViewModel sobreviu a l'activitat (girar).
        val prefs = Prefs(applicationContext)
        creaCanalActualitzacions(this)
        if (savedInstanceState == null && Build.VERSION.SDK_INT >= 33 &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            notificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
        // Només la primera vegada per a aquest ViewModel: en girar la pantalla
        // l'activitat es recrea però l'estat (connexió, xat) segueix viu.
        // Notificació d'un job tocada: s'obre aquella execució (abans de
        // connectar, perquè la primera pantalla ja sigui la d'activitat).
        if (savedInstanceState == null) openRunFrom(intent)
        if (!vm.startupDone) {
            vm.startupDone = true
            vm.attachPrefs(prefs)
            // Per defecte, Gregal en aquest dispositiu; si havies triat un
            // servidor remot, aquell.
            vm.startSaved(prefs)
            // Comprovació silenciosa d'actualitzacions a l'arrencada: si n'hi ha
            // i no l'has silenciada, surt la targeta; si no, no diu res.
            vm.checkUpdate(silent = true)
        }
        setContent {
            val tema by vm.tema.collectAsState()
            val foscSistema = isSystemInDarkTheme()
            val pal = paletteFor(tema, foscSistema)
            val scheme = pal.scheme()
            val dark = when (tema) {
                "clar" -> false
                "fosc" -> true
                else -> foscSistema
            }
            val view = LocalView.current
            // Colors de barra i icones fora de la composició, i icones fosques
            // sobre el tema clar (abans quedaven blanques sobre blanc).
            SideEffect {
                window.statusBarColor = pal.deepTop.toArgb()
                window.navigationBarColor = pal.deepTop.toArgb()
                WindowCompat.getInsetsController(window, view).apply {
                    isAppearanceLightStatusBars = !dark
                    isAppearanceLightNavigationBars = !dark
                }
            }
            CompositionLocalProvider(LocalPal provides pal) {
            MaterialTheme(colorScheme = scheme, typography = gregalTypography()) {
                val screen by vm.screen.collectAsState()
                val connected by vm.connected.collectAsState()
                val conn by vm.conn.collectAsState()
                val lang by vm.idioma.collectAsState()
                val st by vm.state.collectAsState()
                // L'avis d'actualització viu per sobre de qualsevol pantalla:
                // si l'has silenciada o no n'hi ha, no es veu res.
                Box(Modifier.fillMaxSize()) {
                if (!connected) {
                    PairingScreen(vm, prefs)
                } else {
                    val drawer = rememberDrawerState(DrawerValue.Closed)
                    val scope = rememberCoroutineScope()
                    fun nav(s: Screen) {
                        scope.launch { drawer.close() }
                        vm.go(s)
                    }
                    // Enrere: primer tanca el calaix si és obert; si no, tira
                    // enrere de debò (pila de pantalles); només surt de l'app
                    // des de l'arrel sense pila.
                    val backDepth by vm.backStack.collectAsState()
                    val screenState by vm.screen.collectAsState()
                    val ventsOpen by vm.ventsOpen.collectAsState()
                    BackHandler(
                        enabled = drawer.isOpen || (screenState == Screen.Company && ventsOpen != null) || backDepth.isNotEmpty() || screenState != Screen.Company,
                    ) {
                        when {
                            drawer.isOpen -> scope.launch { drawer.close() }
                            screenState == Screen.Company && ventsOpen != null -> vm.openVentsChat(null)
                            else -> vm.back()
                        }
                    }
                    ModalNavigationDrawer(
                        drawerState = drawer,
                        drawerContent = {
                            ModalDrawerSheet(drawerContainerColor = Superficie) {
                                GregalDrawerContent(
                                    vm, lang, screen, conn.local, "${conn.host}:${conn.port}", st.project, st.branch,
                                    nav = { nav(it) }, close = { scope.launch { drawer.close() } },
                                )
                            }
                        },
                    ) {
                        GregalBg {
                            AnimatedContent(targetState = screen, label = "pantalla") { s ->
                                when (s) {
                                    Screen.Chat -> ChatScreen(vm) { scope.launch { drawer.open() } }
                                    Screen.Company -> androidx.compose.runtime.key(conn) { CompanyScreen(vm) { scope.launch { drawer.open() } } }
                                    Screen.Work -> WorkScreen(vm) { scope.launch { drawer.open() } }
                                    Screen.Sessions -> SessionsScreen(vm) { scope.launch { drawer.open() } }
                                    Screen.Goals -> GoalsScreen(vm) { scope.launch { drawer.open() } }
                                    Screen.Jobs -> JobsScreen(vm) { scope.launch { drawer.open() } }
                                    Screen.Office -> OfficeScreen(vm, { scope.launch { drawer.open() } }) { nav(Screen.Chat) }
                                    Screen.Flows -> FlowsScreen(vm) { scope.launch { drawer.open() } }
                                    Screen.Prefs -> PrefsScreen(vm) { scope.launch { drawer.open() } }
                                    Screen.Retro -> RetroScreen(vm, { scope.launch { drawer.open() } }) { nav(Screen.Chat) }
                                    Screen.Pairing -> ChatScreen(vm) { scope.launch { drawer.open() } }
                                }
                            }
                        }
                    }
                }
                UpdateAvis(vm)
                // El vent actiu acompanya per totes les pantalles EXCEPTE als
                // mateixos vents i al xat de feina: allà ja hi ets, i el FAB
                // es barrejaria amb els botons de crear conversa o taparia
                // el botó d'enviar del composer.
                if (screen != Screen.Company && screen != Screen.Chat) {
                    WindFab(vm) { vm.go(Screen.Company) }
                }
                }
            }
            }
        }
    }
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        openRunFrom(intent)
    }

    private fun openRunFrom(intent: Intent?) {
        intent?.getStringExtra(JobAlarms.EXTRA_RUN)?.takeIf { it.isNotBlank() }?.let {
            intent.removeExtra(JobAlarms.EXTRA_RUN)
            vm.openRunFromNotification(it)
        }
        intent?.getStringExtra(WindNotifications.EXTRA_WIND)?.takeIf { it.isNotBlank() }?.let {
            intent.removeExtra(WindNotifications.EXTRA_WIND)
            vm.openVentsChat("wind:" + it)
        }
        intent?.getStringExtra(WindNotifications.EXTRA_ROT)?.takeIf { it.isNotBlank() }?.let {
            intent.removeExtra(WindNotifications.EXTRA_ROT)
            vm.openVentsChat("rot:" + it)
        }
    }

    override fun onStart() {
        super.onStart()
        AppVisibility.foreground = true
        vm.refreshSystemLanguage()
    }

    override fun onConfigurationChanged(newConfig: android.content.res.Configuration) {
        super.onConfigurationChanged(newConfig)
        vm.refreshSystemLanguage()
    }

    override fun onStop() {
        AppVisibility.foreground = false
        vm.onBackgrounded()
        super.onStop()
    }
}
