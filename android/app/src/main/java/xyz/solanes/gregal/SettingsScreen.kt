package xyz.solanes.gregal

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.Code
import androidx.compose.material.icons.rounded.Computer
import androidx.compose.material.icons.rounded.Construction
import androidx.compose.material.icons.rounded.ExpandLess
import androidx.compose.material.icons.rounded.ExpandMore
import androidx.compose.material.icons.rounded.FolderOpen
import androidx.compose.material.icons.rounded.Forum
import androidx.compose.material.icons.rounded.Groups
import androidx.compose.material.icons.rounded.Place
import androidx.compose.material.icons.rounded.Settings
import androidx.compose.material3.Icon
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

/**
 * Configuració de Vents: el que abans es desplegava al peu del calaix, ara
 * en una pàgina. Cada fila obre el mateix lloc que obria, i Enrere hi torna.
 */
@Composable
fun SettingsScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val st by vm.state.collectAsState()
    var codeOpen by rememberSaveable { mutableStateOf(false) }
    Scaffold(containerColor = Color.Transparent, snackbarHost = { NoticeHost(vm) }, topBar = {
        GregalTopBar(tr(lang, "app.settings.title"), lang, onMenu = onMenu)
    }) { pad ->
        Column(Modifier.fillMaxSize().padding(pad).verticalScroll(rememberScrollState()).padding(bottom = 32.dp)) {
            SettingsRow(Icons.Rounded.Settings, tr(lang, "prefs.title"), tr(lang, "app.settings.prefsAbout"), onClick = { vm.openPreferences() })
            SettingsRow(Icons.Rounded.Computer, tr(lang, "app.computer.name"), tr(lang, "app.settings.computerAbout"), onClick = { vm.openVentsPanel("computer") })
            SettingsRow(Icons.Rounded.Place, tr(lang, "app.local.title"), tr(lang, "app.settings.localAbout"),
                onClick = { vm.openVentsPanel("local"); vm.go(Screen.Automatitzacions) })
            SettingsRow(Icons.Rounded.AutoAwesome, guideIntro(lang).first, tr(lang, "app.settings.guideAbout"), onClick = { vm.openVentsPanel("guide") })
            SettingsRow(Icons.Rounded.Groups, tr(lang, "app.vents.myWinds"), tr(lang, "app.settings.myWindsAbout"), onClick = { vm.openVentsPanel("directory") })
            // Programar des del mòbil: l'agent de codi, les seves converses, el projecte i les feines.
            SettingsRow(Icons.Rounded.Code, tr(lang, "app.nav.sectionCode"), tr(lang, "app.settings.codeAbout"), onClick = { codeOpen = !codeOpen }) {
                Icon(if (codeOpen) Icons.Rounded.ExpandLess else Icons.Rounded.ExpandMore, null, tint = Muted)
            }
            if (codeOpen) Column(Modifier.padding(start = 34.dp)) {
                SettingsRow(Icons.Rounded.Code, tr(lang, "app.nav.codeChat"), onClick = { vm.go(Screen.Chat) })
                SettingsRow(Icons.Rounded.Forum, tr(lang, "nav.convs"), onClick = { vm.go(Screen.Sessions); vm.loadSessions() })
                SettingsRow(Icons.Rounded.FolderOpen, st.project.ifEmpty { tr(lang, "app.nav.project") } + if (st.branch.isNotBlank()) " · ${st.branch}" else "",
                    onClick = { vm.go(Screen.Carpeta) })
                SettingsRow(Icons.Rounded.Construction, tr(lang, "app.settings.codeWork"), onClick = { vm.go(Screen.Work) })
            }
        }
    }
}
