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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Undo
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.DateRange
import androidx.compose.material.icons.filled.Description
import androidx.compose.material.icons.filled.ForkRight
import androidx.compose.material.icons.filled.List
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Share
import androidx.compose.material.icons.filled.Star
import androidx.compose.material3.DrawerValue
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalDrawerSheet
import androidx.compose.material3.ModalNavigationDrawer
import androidx.compose.material3.NavigationDrawerItem
import androidx.compose.material3.Text
import androidx.compose.material3.rememberDrawerState
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
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
        // El tema real es pinta al setContent; aquí `fons` (#0F172A) per al
        // flaix inicial — abans era el fons pre-identitat (#101418).
        window.statusBarColor = Color(0xFF0F172A).toArgb()
        window.navigationBarColor = Color(0xFF0F172A).toArgb()
        @Suppress("DEPRECATION")
        window.decorView.systemUiVisibility = 0
        val prefs = Prefs(this)
        vm.attachPrefs(prefs)
        creaCanalActualitzacions(this)
        if (Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            notificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
        if (prefs.host.isNotBlank()) {
            vm.setConn(Conn(prefs.host, prefs.port, prefs.token, prefs.usuari))
            vm.connect { vm.go(Screen.Chat) }
        }
        // Comprovació silenciosa d'actualitzacions a l'arrencada: si n'hi ha
        // i no l'has silenciada, surt la targeta; si no, no diu res.
        vm.checkUpdate(silent = true)
        setContent {
            val tema by vm.tema.collectAsState()
            val foscSistema = isSystemInDarkTheme()
            val pal = paletteFor(tema, foscSistema)
            val scheme = pal.scheme()
            window.statusBarColor = pal.deepTop.toArgb()
            window.navigationBarColor = pal.deepTop.toArgb()
            CompositionLocalProvider(LocalPal provides pal) {
            MaterialTheme(colorScheme = scheme) {
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
                            ModalDrawerSheet {
                                Column(Modifier.padding(16.dp)) {
                                    Text("≋ Gregal", fontSize = 22.sp,
                                        fontWeight = FontWeight.Bold, color = Ink)
                                    // El projecte viu aquí des que el títol de
                                    // dalt és només el model (estil ChatGPT).
                                    Text("▣ " + st.project.ifEmpty { tr(lang, "app.nav.project") },
                                        fontSize = 14.sp, color = Sand,
                                        fontWeight = FontWeight.SemiBold)
                                    Text("${conn.host}:${conn.port}",
                                        fontSize = 12.sp, color = Muted)
                                    TideLine(width = 26, working = false,
                                        modifier = Modifier.padding(top = 2.dp))
                                    Spacer(Modifier.height(12.dp))
                                    NavigationDrawerItem(
                                        label = { Text(tr(lang, "nav.new")) },
                                        icon = { Icon(Icons.Default.Add, null) },
                                        selected = screen == Screen.Chat,
                                        onClick = { vm.newSession(); nav(Screen.Chat) },
                                    )
                                    NavigationDrawerItem(
                                        label = { Text(tr(lang, "nav.convs")) },
                                        icon = { Icon(Icons.Default.List, null) },
                                        selected = screen == Screen.Sessions,
                                        onClick = { nav(Screen.Sessions); vm.loadSessions() },
                                    )
                                    NavigationDrawerItem(
                                        label = { Text(tr(lang, "nav.goals")) },
                                        icon = { Icon(Icons.Default.Star, null) },
                                        selected = screen == Screen.Goals,
                                        onClick = { nav(Screen.Goals); vm.loadGoals() },
                                    )
                                    NavigationDrawerItem(
                                        label = { Text(tr(lang, "nav.activity")) },
                                        icon = { Icon(Icons.Default.DateRange, null) },
                                        selected = screen == Screen.Jobs,
                                        onClick = { nav(Screen.Jobs); vm.loadJobs(); vm.loadRuns() },
                                    )
                                    NavigationDrawerItem(
                                        label = { Text(tr(lang, "nav.office")) },
                                        icon = { Icon(Icons.Default.Description, null) },
                                        selected = screen == Screen.Office,
                                        onClick = { nav(Screen.Office) },
                                    )
                                    // GitHub surt de la barra de dalt i viu aquí.
                                    NavigationDrawerItem(
                                        label = { Text(tr(lang, "app.nav.github")) },
                                        icon = { Icon(Icons.Default.ForkRight, null) },
                                        selected = false,
                                        onClick = { scope.launch { drawer.close() }; vm.obreGithub() },
                                    )
                                    // I desfés, que abans era l'única icona de dalt.
                                    NavigationDrawerItem(
                                        label = { Text(tr(lang, "app.nav.undoTurn")) },
                                        icon = { Icon(Icons.AutoMirrored.Filled.Undo, null) },
                                        selected = false,
                                        onClick = { scope.launch { drawer.close() }; vm.openRewind() },
                                    )
                                    NavigationDrawerItem(
                                        label = { Text(tr(lang, "graphs.title")) },
                                        icon = { Icon(Icons.Default.Share, null) },
                                        selected = screen == Screen.Flows,
                                        onClick = { nav(Screen.Flows); vm.loadFlows() },
                                    )
                                    NavigationDrawerItem(
                                        label = { Text(tr(lang, "prefs.title")) },
                                        icon = { Icon(Icons.Default.Settings, null) },
                                        selected = screen == Screen.Prefs,
                                        onClick = { nav(Screen.Prefs) },
                                    )
                                    NavigationDrawerItem(
                                        label = { Text(tr(lang, "btn.disconnect")) },
                                        icon = { Icon(Icons.Default.Refresh, null) },
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
