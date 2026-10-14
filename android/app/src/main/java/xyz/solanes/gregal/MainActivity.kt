package xyz.solanes.gregal

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.compose.setContent
import androidx.compose.animation.AnimatedContent
import androidx.activity.viewModels
import androidx.compose.foundation.layout.Box
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
import androidx.compose.material.icons.automirrored.rounded.Logout
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material.icons.Icons
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
    var idioma: String
        get() = p.getString("idioma", "en") ?: "en"
        set(v) { p.edit().putString("idioma", v).apply() }
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
        if (!vm.startupDone) {
            vm.startupDone = true
            vm.attachPrefs(prefs)
            if (prefs.host.isNotBlank()) {
                vm.setConn(Conn(prefs.host, prefs.port, prefs.token, prefs.usuari))
                vm.connect(retries = 8) { vm.go(Screen.Chat) }
            }
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
                    ModalNavigationDrawer(
                        drawerState = drawer,
                        drawerContent = {
                            val drawerColors = androidx.compose.material3.NavigationDrawerItemDefaults.colors(
                                selectedContainerColor = Sea.copy(alpha = 0.14f),
                                selectedIconColor = Sea, selectedTextColor = Ink,
                                unselectedIconColor = Muted, unselectedTextColor = Tinta,
                            )
                            ModalDrawerSheet(drawerContainerColor = Superficie) {
                                Column(Modifier.padding(16.dp)) {
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        GregalMark(30.dp)
                                        Spacer(Modifier.width(10.dp))
                                        Text("Gregal", fontSize = 22.sp, fontWeight = FontWeight.Bold, color = Ink)
                                    }
                                    Spacer(Modifier.height(6.dp))
                                    // El projecte viu aquí des que el títol de
                                    // dalt és només el model (estil ChatGPT).
                                    ProjectPill(st.project.ifEmpty { tr(lang, "app.nav.project") }, st.branch)
                                    Text("${conn.host}:${conn.port}", fontSize = 12.sp, color = Faint,
                                        modifier = Modifier.padding(start = 4.dp, top = 6.dp))
                                    Spacer(Modifier.height(14.dp))
                                    // Conversa nova com a acció principal (no com un
                                    // element «seleccionat» de la llista).
                                    androidx.compose.material3.OutlinedButton(
                                        onClick = { vm.newSession(); nav(Screen.Chat) },
                                        border = androidx.compose.foundation.BorderStroke(1.dp, Sea.copy(alpha = 0.5f)),
                                        modifier = Modifier.fillMaxWidth().height(48.dp),
                                    ) {
                                        Icon(Icons.Rounded.Add, null, tint = Sea, modifier = Modifier.size(20.dp))
                                        Spacer(Modifier.width(8.dp))
                                        Text(tr(lang, "nav.new"), color = Sea, fontWeight = FontWeight.SemiBold)
                                    }
                                    Spacer(Modifier.height(10.dp))
                                    NavigationDrawerItem(
                                        colors = drawerColors,
                                        label = { Text(tr(lang, "app.nav.chat")) },
                                        icon = { Icon(Icons.Rounded.ChatBubbleOutline, null) },
                                        selected = screen == Screen.Chat,
                                        onClick = { nav(Screen.Chat) },
                                    )
                                    NavigationDrawerItem(
                                        colors = drawerColors,
                                        label = { Text(tr(lang, "nav.convs")) },
                                        icon = { Icon(Icons.Rounded.Forum, null) },
                                        selected = screen == Screen.Sessions,
                                        onClick = { nav(Screen.Sessions); vm.loadSessions() },
                                    )
                                    NavigationDrawerItem(
                                        colors = drawerColors,
                                        label = { Text(tr(lang, "nav.goals")) },
                                        icon = { Icon(Icons.Rounded.Flag, null) },
                                        selected = screen == Screen.Goals,
                                        onClick = { nav(Screen.Goals); vm.loadGoals() },
                                    )
                                    NavigationDrawerItem(
                                        colors = drawerColors,
                                        label = { Text(tr(lang, "nav.activity")) },
                                        icon = { Icon(Icons.Rounded.Schedule, null) },
                                        selected = screen == Screen.Jobs,
                                        onClick = { nav(Screen.Jobs); vm.loadJobs(); vm.loadRuns() },
                                    )
                                    androidx.compose.material3.HorizontalDivider(Modifier.padding(vertical = 8.dp, horizontal = 12.dp), color = Line)
                                    NavigationDrawerItem(
                                        colors = drawerColors,
                                        label = { Text(tr(lang, "nav.office")) },
                                        icon = { Icon(Icons.Rounded.Description, null) },
                                        selected = screen == Screen.Office,
                                        onClick = { nav(Screen.Office) },
                                    )
                                    // GitHub surt de la barra de dalt i viu aquí.
                                    NavigationDrawerItem(
                                        colors = drawerColors,
                                        label = { Text(tr(lang, "app.nav.github")) },
                                        icon = { Icon(Icons.Rounded.Code, null) },
                                        selected = false,
                                        onClick = { scope.launch { drawer.close() }; vm.obreGithub() },
                                    )
                                    // I desfés, que abans era l'única icona de dalt.
                                    NavigationDrawerItem(
                                        colors = drawerColors,
                                        label = { Text(tr(lang, "app.nav.undoTurn")) },
                                        icon = { Icon(Icons.AutoMirrored.Rounded.Undo, null) },
                                        selected = false,
                                        onClick = { scope.launch { drawer.close() }; vm.openRewind() },
                                    )
                                    NavigationDrawerItem(
                                        colors = drawerColors,
                                        label = { Text(tr(lang, "graphs.title")) },
                                        icon = { Icon(Icons.Rounded.AccountTree, null) },
                                        selected = screen == Screen.Flows,
                                        onClick = { nav(Screen.Flows); vm.loadFlows() },
                                    )
                                    androidx.compose.material3.HorizontalDivider(Modifier.padding(vertical = 8.dp, horizontal = 12.dp), color = Line)
                                    NavigationDrawerItem(
                                        colors = drawerColors,
                                        label = { Text(tr(lang, "prefs.title")) },
                                        icon = { Icon(Icons.Rounded.Settings, null) },
                                        selected = screen == Screen.Prefs,
                                        onClick = { nav(Screen.Prefs) },
                                    )
                                    NavigationDrawerItem(
                                        colors = drawerColors,
                                        label = { Text(tr(lang, "btn.disconnect"), color = Muted) },
                                        icon = { Icon(Icons.AutoMirrored.Rounded.Logout, null, tint = Muted) },
                                        selected = false,
                                        onClick = { scope.launch { drawer.close() }; vm.disconnect() },
                                    )
                                }
                            }
                        },
                    ) {
                        GregalBg {
                            AnimatedContent(targetState = screen, label = "pantalla") { s ->
                                when (s) {
                                    Screen.Chat -> ChatScreen(vm) { scope.launch { drawer.open() } }
                                    Screen.Sessions -> SessionsScreen(vm) { scope.launch { drawer.open() } }
                                    Screen.Goals -> GoalsScreen(vm) { scope.launch { drawer.open() } }
                                    Screen.Jobs -> JobsScreen(vm) { scope.launch { drawer.open() } }
                                    Screen.Office -> OfficeScreen(vm, { scope.launch { drawer.open() } }) { nav(Screen.Chat) }
                                    Screen.Flows -> FlowsScreen(vm) { scope.launch { drawer.open() } }
                                    Screen.Prefs -> PrefsScreen(vm) { scope.launch { drawer.open() } }
                                    Screen.Pairing -> ChatScreen(vm) { scope.launch { drawer.open() } }
                                }
                            }
                        }
                    }
                }
                UpdateAvis(vm)
                }
            }
            }
        }
    }
    override fun onStart() {
        super.onStart()
        AppVisibility.foreground = true
    }

    override fun onStop() {
        AppVisibility.foreground = false
        super.onStop()
    }
}
