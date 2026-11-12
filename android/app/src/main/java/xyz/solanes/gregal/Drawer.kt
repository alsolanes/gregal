package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.rounded.Link
import androidx.compose.material.icons.rounded.Computer
import androidx.compose.material.icons.rounded.Alarm
import androidx.compose.material.icons.rounded.Visibility
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.Logout
import androidx.compose.material.icons.automirrored.rounded.Undo
import androidx.compose.material.icons.rounded.Dns
import androidx.compose.material.icons.rounded.Folder
import androidx.compose.material.icons.rounded.EditNote
import androidx.compose.material.icons.rounded.Language
import androidx.compose.material.icons.rounded.AccountTree
import androidx.compose.material.icons.rounded.Toys
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.ChatBubbleOutline
import androidx.compose.material.icons.rounded.Code
import androidx.compose.material.icons.rounded.Construction
import androidx.compose.material.icons.rounded.Description
import androidx.compose.material.icons.rounded.Flag
import androidx.compose.material.icons.rounded.FolderOpen
import androidx.compose.material.icons.rounded.Forum
import androidx.compose.material.icons.rounded.Today
import androidx.compose.material.icons.rounded.Schedule
import androidx.compose.material.icons.rounded.Settings
import androidx.compose.material.icons.rounded.Tag
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.ExpandLess
import androidx.compose.material.icons.rounded.ExpandMore
import androidx.compose.material.icons.rounded.Groups
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * El calaix de navegació, agrupat per a què serveix cada lloc: els vents
 * primer (la cara de l'app), després l'agent, l'automatització i les eines;
 * les preferències al peu. La capçalera diu on estàs connectat i en quin
 * projecte, i la conversa nova és l'única acció principal.
 */
@Composable
fun GregalDrawerContent(
    vm: GregalViewModel,
    lang: String,
    screen: Screen,
    local: Boolean,
    server: String,
    project: String,
    branch: String,
    nav: (Screen) -> Unit,
    close: () -> Unit,
) {
    val simple by vm.simpleVents.collectAsState()
    if (simple) { SimpleVentsDrawer(vm, lang, screen, local, server, project, branch, nav, close); return }
    CappedFontScale(1.5f) {
        Column(Modifier.verticalScroll(rememberScrollState()).padding(horizontal = 14.dp, vertical = 18.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(horizontal = 6.dp)) {
                GregalMark(36.dp)
                Spacer(Modifier.width(12.dp))
                Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text("Gregal", fontSize = 22.sp, fontWeight = FontWeight.Bold, color = Ink, letterSpacing = (-0.3).sp)
                    StatusPill(if (local) tr(lang, "app.server.thisDevice") else server, Green)
                }
            }
            Spacer(Modifier.height(14.dp))
            Box(Modifier.padding(horizontal = 6.dp).clip(RoundedCornerShape(50)).clickable { nav(Screen.Carpeta) }) {
                ProjectPill(project.ifEmpty { tr(lang, "app.nav.project") }, branch)
            }
            Spacer(Modifier.height(16.dp))
            PrimaryPill(tr(lang, "nav.new"), { vm.newSession(); nav(Screen.Chat) }, Modifier.fillMaxWidth(), icon = Icons.Rounded.Add)

            DrawerSection(tr(lang, "app.nav.sectionWinds"))
            DrawerRow(tr(lang, "nav.company"), screen == Screen.Company, { nav(Screen.Company) }) { DrawerWindIcon(vm) }
            DrawerRow(tr(lang, "app.nav.ventet"), screen == Screen.Ventet, { nav(Screen.Ventet) }, Icons.Rounded.Toys)
            DrawerRow(tr(lang, "nav.notes"), screen == Screen.Notes, { nav(Screen.Notes); vm.loadNotes() }, Icons.Rounded.EditNote)

            DrawerSection(tr(lang, "app.nav.sectionAgent"))
            DrawerRow(tr(lang, "app.nav.chat"), screen == Screen.Chat, { nav(Screen.Chat) }, Icons.Rounded.ChatBubbleOutline)
            DrawerRow(tr(lang, "app.work.title"), screen == Screen.Work, { nav(Screen.Work) }, Icons.Rounded.Construction)
            DrawerRow(tr(lang, "nav.convs"), screen == Screen.Sessions, { nav(Screen.Sessions); vm.loadSessions() }, Icons.Rounded.Forum)
            DrawerRow(tr(lang, "nav.goals"), screen == Screen.Goals, { nav(Screen.Goals); vm.loadGoals() }, Icons.Rounded.Flag)

            DrawerSection(tr(lang, "app.nav.sectionAutomate"))
            // Jobs i GitHub són de l'administrador del servidor: un altre compte rebria 403.
            val admin by vm.isAdmin.collectAsState()
            if (local || admin) DrawerRow(tr(lang, "nav.activity"), screen == Screen.Jobs, { nav(Screen.Jobs); vm.loadJobs(); vm.loadRuns() }, Icons.Rounded.Schedule)
            DrawerRow(tr(lang, "graphs.title"), screen == Screen.Flows, { nav(Screen.Flows); vm.loadFlows() }, Icons.Rounded.AccountTree)
            DrawerRow(tr(lang, "nav.automations"), screen == Screen.Automatitzacions, {
                nav(Screen.Automatitzacions); vm.loadAutomations(""); vm.loadJobs(); vm.loadFlows()
            }, Icons.Rounded.Schedule)

            DrawerSection(tr(lang, "app.nav.sectionTools"))
            DrawerRow(tr(lang, "app.carpeta.title"), screen == Screen.Carpeta, { nav(Screen.Carpeta) }, Icons.Rounded.FolderOpen)
            DrawerRow(webCopy(lang, "title"), screen == Screen.Webs, { nav(Screen.Webs); vm.loadWebs() }, Icons.Rounded.Language)
            DrawerRow(tr(lang, "nav.office"), screen == Screen.Office, { nav(Screen.Office) }, Icons.Rounded.Description)
            DrawerRow(tr(lang, "app.retro.title"), screen == Screen.Retro, { close(); vm.openRetro() }, Icons.Rounded.Tag)
            // Al dispositiu no hi ha la CLI gh: GitHub només en remot.
            if (!local && admin) DrawerRow(tr(lang, "app.nav.github"), false, { close(); vm.obreGithub() }, Icons.Rounded.Code)
            DrawerRow(tr(lang, "app.nav.undoTurn"), false, { close(); vm.openRewind() }, Icons.AutoMirrored.Rounded.Undo)

            HorizontalDivider(Modifier.padding(vertical = 12.dp, horizontal = 8.dp), color = Line)
            DrawerRow(tr(lang, "prefs.title"), screen == Screen.Prefs, { nav(Screen.Prefs) }, Icons.Rounded.Settings)
            DrawerRow(tr(lang, "app.server.switch"), false, { close(); vm.switchServer() }, Icons.AutoMirrored.Rounded.Logout, muted = true)
            // Amb compte (el servidor de casa), sortir de debò: el token es tanca al servidor.
            if (vm.signedInWithAccount()) DrawerRow(tr(lang, "app.server.signOut", vm.usuari.collectAsState().value), false, { close(); vm.signOut() }, Icons.AutoMirrored.Rounded.Logout, muted = true)
        }
    }
}

/**
 * Vents net: quatre llocs, com les apps de companys. Els vents (la portada),
 * el directori dels teus vents, Explora (programats, desats, wiki, biblioteca,
 * feines…) i les notes. El Ventet és la mateixa portada. Programar des del
 * mòbil continua a mà, plegat a «Codi» perquè no competeixi amb els vents.
 */
@Composable
private fun SimpleVentsDrawer(
    vm: GregalViewModel, lang: String, screen: Screen, local: Boolean, server: String,
    project: String, branch: String,
    nav: (Screen) -> Unit, close: () -> Unit,
) {
    val user by vm.usuari.collectAsState()
    val panel by vm.ventsPanel.collectAsState()
    val codeScreens = setOf(Screen.Chat, Screen.Sessions, Screen.Carpeta, Screen.Work, Screen.Webs)
    var codeOpen by rememberSaveable { mutableStateOf(false) }
    CappedFontScale(1.5f) {
        Column(Modifier.verticalScroll(rememberScrollState()).padding(horizontal = 14.dp, vertical = 18.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(horizontal = 6.dp)) {
                GregalMark(36.dp)
                Spacer(Modifier.width(12.dp))
                Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text("Gregal Vents", fontSize = 22.sp, fontWeight = FontWeight.Bold, color = Ink, letterSpacing = (-0.3).sp)
                    StatusPill(if (local) tr(lang, "app.server.thisDevice") else if (user.isNotBlank()) "$user · $server" else server, Green)
                }
            }
            Spacer(Modifier.height(18.dp))
            val home = screen == Screen.Company || screen == Screen.Ventet
            val winds by vm.winds.collectAsState()
            val tasks by vm.windTasks.collectAsState()
            val en = lang == "en"
            // Direcció visual §2.6: poques coses, totes útils. Gregal és la portada;
            // els especialistes són canals; els recursos, a sota.
            val gregal = winds.firstOrNull { it.id == "gregal" }
            DrawerRow(gregal?.name ?: tr(lang, "nav.company"), home && panel == null, { vm.openVentsPanel(null); nav(Screen.Company) }) { DrawerWindIcon(vm) }
            val running = tasks.orEmpty().count { it.running }
            DrawerRow(if (en) "Work" else "Feina" + "", false, { close(); vm.openVentsPanel("work") }, Icons.Rounded.Construction, badge = running)
            DrawerRow(if (en) "Watching" else "Vigilant", false, { close(); vm.openVentsPanel("watch") }, Icons.Rounded.Visibility)
            DrawerRow(if (en) "Reminders" else "Recordatoris", screen == Screen.Automatitzacions, { nav(Screen.Automatitzacions) }, Icons.Rounded.Alarm)
            DrawerSection(if (en) "Channels" else "Canals")
            winds.filter { it.id != "gregal" }.take(5).forEach { w ->
                DrawerRow(w.name + if (w.role.isNotBlank()) " · " + w.role else "", false, { close(); vm.openVentsPanel(null); vm.openVentsChat("wind:" + w.id) }) {
                    WindHalo(Color(w.argb), w.faceStyle, size = 24, online = false, showKit = false)
                }
            }
            DrawerRow(askCopy(lang, "chats"), false, { close(); vm.openVentsPanel("chats") }, Icons.Rounded.Forum, muted = true)
            DrawerRow(if (en) "New specialist" else "Nou especialista", false, { close(); vm.showWindCreator(true) }, Icons.Rounded.Add, muted = true)
            DrawerSection(if (en) "Resources" else "Recursos")
            DrawerRow(if (en) "Computer" else "Ordinador", false, { close(); vm.openVentsPanel("computer") }, Icons.Rounded.Computer)
            DrawerRow(if (en) "Connections" else "Connexions", screen == Screen.Prefs, { nav(Screen.Prefs) }, Icons.Rounded.Link)
            DrawerRow(webCopy(lang, "title"), screen == Screen.Webs, { nav(Screen.Webs); vm.loadWebs() }, Icons.Rounded.Language)
            DrawerRow(tr(lang, "nav.notes"), screen == Screen.Notes, { nav(Screen.Notes); vm.loadNotes() }, Icons.Rounded.EditNote)
            HorizontalDivider(Modifier.padding(vertical = 8.dp, horizontal = 8.dp), color = Line)
            DrawerRow(guideIntro(lang).first, false, { close(); vm.openVentsPanel("guide") }, Icons.Rounded.AutoAwesome)
            DrawerRow(tr(lang, "app.vents.myWinds"), home && panel == "directory", { close(); vm.openVentsPanel("directory") }, Icons.Rounded.Groups, muted = true)
            // Programar des del mòbil: l'agent de codi, les seves converses, el
            // projecte i les feines, plegats. (Objectius, gràfics, Office i Retro
            // continuen a Preferències → Eines de programació.)
            val showCode = codeOpen || screen in codeScreens
            DrawerRow(tr(lang, "app.nav.sectionCode"), false, { codeOpen = !showCode }, muted = true) {
                Icon(if (showCode) Icons.Rounded.ExpandLess else Icons.Rounded.ExpandMore, null, modifier = Modifier.size(22.dp))
            }
            if (showCode) Column(Modifier.padding(start = 12.dp)) {
                DrawerRow(tr(lang, "app.nav.codeChat"), screen == Screen.Chat, { nav(Screen.Chat) }, Icons.Rounded.Code)
                DrawerRow(tr(lang, "nav.convs"), screen == Screen.Sessions, { nav(Screen.Sessions); vm.loadSessions() }, Icons.Rounded.Forum)
                DrawerRow(
                    project.ifEmpty { tr(lang, "app.nav.project") } + if (branch.isNotBlank()) " · $branch" else "",
                    screen == Screen.Carpeta, { nav(Screen.Carpeta) }, Icons.Rounded.FolderOpen,
                )
                DrawerRow(tr(lang, "app.work.title"), screen == Screen.Work, { nav(Screen.Work) }, Icons.Rounded.Construction)
                DrawerRow(webCopy(lang, "title"), screen == Screen.Webs, { nav(Screen.Webs); vm.loadWebs() }, Icons.Rounded.Language)
            }
            HorizontalDivider(Modifier.padding(vertical = 12.dp, horizontal = 8.dp), color = Line)
            DrawerRow(tr(lang, "prefs.title"), screen == Screen.Prefs, { nav(Screen.Prefs) }, Icons.Rounded.Settings)
            DrawerRow(tr(lang, "app.server.switch"), false, { close(); vm.switchServer() }, Icons.Rounded.Dns, muted = true)
            if (vm.signedInWithAccount()) DrawerRow(tr(lang, "app.server.signOut", user), false, { close(); vm.signOut() }, Icons.AutoMirrored.Rounded.Logout, muted = true)
        }
    }
}

@Composable
private fun DrawerSection(label: String) {
    Text(
        label.uppercase(), style = GregalType.overline, color = Faint,
        modifier = Modifier.padding(start = 14.dp, top = 20.dp, bottom = 6.dp),
    )
}

@Composable
private fun DrawerRow(label: String, selected: Boolean, onClick: () -> Unit, icon: ImageVector, muted: Boolean = false, badge: Int = 0) =
    DrawerRow(label, selected, onClick, muted, badge) { Icon(icon, null, modifier = Modifier.size(22.dp)) }

/** Una entrada del calaix: compacta, amb la seleccionada en píndola tintada i un punt. */
@Composable
private fun DrawerRow(label: String, selected: Boolean, onClick: () -> Unit, muted: Boolean = false, badge: Int = 0, icon: @Composable () -> Unit) {
    val shape = RoundedCornerShape(14.dp)
    Row(
        Modifier.fillMaxWidth().clip(shape)
            .background(if (selected) Sea.copy(alpha = 0.14f) else Color.Transparent)
            .clickable(onClick = onClick)
            .semantics { this.selected = selected }
            .heightIn(min = 48.dp).padding(horizontal = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CompositionLocalProvider(LocalContentColor provides when { selected -> Sea; muted -> Faint; else -> Muted }) {
            Box(Modifier.size(26.dp), contentAlignment = Alignment.Center) { icon() }
        }
        Spacer(Modifier.width(14.dp))
        Text(
            label, fontSize = 15.sp, fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
            color = when { selected -> Ink; muted -> Muted; else -> Tinta },
            maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f),
        )
        if (badge > 0) Box(Modifier.background(Sea, CircleShape).padding(horizontal = 7.dp, vertical = 1.dp)) {
            Text(badge.toString(), color = OnSea, fontSize = 11.sp, fontWeight = FontWeight.Bold)
        }
        if (selected) Box(Modifier.size(6.dp).background(Sea, CircleShape))
    }
}
