package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
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
import androidx.compose.material.icons.rounded.Checklist
import androidx.compose.material.icons.rounded.Place
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
import androidx.compose.material.icons.rounded.Bookmark
import androidx.compose.material.icons.rounded.WbSunny
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.foundation.relocation.bringIntoViewRequester
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
import androidx.compose.ui.platform.testTag
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
        Column(Modifier.fillMaxSize().padding(horizontal = 14.dp, vertical = 8.dp)) {
            Column(Modifier.weight(1f).verticalScroll(rememberScrollState())) {
                Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(horizontal = 6.dp)) {
                    GregalMark(36.dp)
                    Spacer(Modifier.width(12.dp))
                    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text("Gregal", fontSize = 22.sp, fontWeight = FontWeight.Bold, color = Ink, letterSpacing = (-0.3).sp)
                        Text(if (local) tr(lang, "app.server.thisDevice") else server, color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                }
                Spacer(Modifier.height(14.dp))
                Box(Modifier.padding(horizontal = 6.dp).clip(RoundedCornerShape(50)).clickable { nav(Screen.Carpeta) }) {
                    ProjectPill(project.ifEmpty { tr(lang, "app.nav.project") }, branch)
                }
                Spacer(Modifier.height(12.dp))
                ModeSwitcher(lang, activeAppArea(screen, simple), { area -> close(); vm.switchArea(area) },
                    Modifier.fillMaxWidth().padding(horizontal = 6.dp).testTag("area-switcher"))
                Spacer(Modifier.height(12.dp))
                DrawerRow(tr(lang, "nav.new"), false, { vm.newSession(); nav(Screen.Chat) }) { Icon(Icons.Rounded.Add, null, tint = Sea, modifier = Modifier.size(22.dp)) }

                DrawerSection(tr(lang, "app.nav.sectionWinds"))
                DrawerRow(tr(lang, "nav.company"), screen == Screen.Company, { nav(Screen.Company) }) { DrawerWindIcon(vm) }
                DrawerRow(tr(lang, "app.nav.ventet"), screen == Screen.Ventet, { nav(Screen.Ventet) }, Icons.Rounded.Toys)
                DrawerRow(tr(lang, "nav.notes"), screen == Screen.Notes, { nav(Screen.Notes); vm.loadNotes() }, Icons.Rounded.EditNote)

                DrawerSection(tr(lang, "app.nav.sectionAgent"))
                DrawerRow(tr(lang, "app.nav.chat"), screen == Screen.Chat, { nav(Screen.Chat) }, Icons.Rounded.ChatBubbleOutline)
                DrawerRow(tr(lang, "app.work.title"), screen == Screen.Work, { nav(Screen.Work) }, Icons.Rounded.Construction)
                DrawerRow(tr(lang, "nav.convs"), screen == Screen.Sessions, { nav(Screen.Sessions); vm.loadSessions() }, Icons.Rounded.Forum)
                DrawerRow(tr(lang, "nav.backlog"), screen == Screen.Backlog, { nav(Screen.Backlog); vm.loadBacklog() }, Icons.Rounded.Checklist)

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
            }
            HorizontalDivider(Modifier.padding(vertical = 6.dp, horizontal = 8.dp), color = Line)
            DrawerRow(tr(lang, "prefs.title"), screen == Screen.Prefs, { nav(Screen.Prefs); vm.openPreferences() }, Icons.Rounded.Settings)
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
    val codeScreens = setOf(Screen.Chat, Screen.Sessions, Screen.Carpeta, Screen.Work)
    var codeOpen by rememberSaveable { mutableStateOf(false) }
    CappedFontScale(1.5f) {
        Column(Modifier.fillMaxSize().padding(horizontal = 14.dp, vertical = 8.dp)) {
            Column(Modifier.weight(1f).verticalScroll(rememberScrollState())) {
                Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(horizontal = 6.dp)) {
                    GregalMark(36.dp)
                    Spacer(Modifier.width(12.dp))
                    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text("Gregal Vents", fontSize = 22.sp, fontWeight = FontWeight.Bold, color = Ink, letterSpacing = (-0.3).sp)
                        Text(if (local) tr(lang, "app.server.thisDevice") else if (user.isNotBlank()) "$user · $server" else server, color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                }
                Spacer(Modifier.height(18.dp))
                ModeSwitcher(lang, activeAppArea(screen, true), { area -> close(); vm.switchArea(area) },
                    Modifier.fillMaxWidth().padding(horizontal = 6.dp).testTag("area-switcher"))
                Spacer(Modifier.height(12.dp))
                val home = screen == Screen.Company || screen == Screen.Ventet
                val winds by vm.winds.collectAsState()
                val tasks by vm.windTasks.collectAsState()
                val en = lang == "en"
                // Un sol nivell, igual que l'escriptori (renderNav a winds-workspace.js):
                // començar; Avui, Feina, Desat i la colla; els vents; les converses;
                // i la resta plegada. Abans: Gregal, Avui, Feina, Vigilant,
                // Automatitzacions i A prop teu com a files soltes, i tres blocs més.
                val gregal = winds.firstOrNull { it.id == "gregal" }
                DrawerRow(if (en) "New conversation" else "Nova conversa", false, { close(); vm.openVentsPanel(null); nav(Screen.Company) }) {
                    Icon(Icons.Rounded.Add, null, tint = Sea, modifier = Modifier.size(22.dp))
                }
                Spacer(Modifier.height(4.dp))
                DrawerRow(if (en) "Today" else "Avui", home && panel == "today", { close(); vm.openVentsPanel("today"); nav(Screen.Company) }, Icons.Rounded.WbSunny)
                val running = tasks.orEmpty().count { it.running }
                var workOpen by rememberSaveable { mutableStateOf(false) }
                DrawerGroup(if (en) "Work" else "Feina", Icons.Rounded.Construction, workOpen || screen == Screen.Automatitzacions, { workOpen = !workOpen }, badge = running) {
                    DrawerRow(if (en) "In progress" else "En marxa", false, { close(); vm.openVentsPanel("work") }, Icons.Rounded.Construction)
                    DrawerRow(if (en) "Watching" else "Vigilant", false, { close(); vm.openVentsPanel("watch") }, Icons.Rounded.Visibility)
                    DrawerRow(if (en) "Automations" else "Automatitzacions", screen == Screen.Automatitzacions && panel != "local", { nav(Screen.Automatitzacions) }, Icons.Rounded.Alarm)
                }
                var savedOpen by rememberSaveable { mutableStateOf(false) }
                DrawerGroup(if (en) "Saved" else "Desat", Icons.Rounded.Bookmark, savedOpen || screen in setOf(Screen.Desat, Screen.Notes, Screen.Webs), { savedOpen = !savedOpen }) {
                    DrawerRow(if (en) "Saved" else "Desat", screen == Screen.Desat, { nav(Screen.Desat) }, Icons.Rounded.Bookmark)
                    DrawerRow(tr(lang, "nav.notes"), screen == Screen.Notes, { nav(Screen.Notes); vm.loadNotes() }, Icons.Rounded.EditNote)
                    DrawerRow(webCopy(lang, "title"), screen == Screen.Webs, { nav(Screen.Webs); vm.loadWebs() }, Icons.Rounded.Language)
                }
                DrawerRow(if (en) "Your winds" else "Els teus vents", false, { close(); vm.openVentsPanel(null); nav(Screen.Company); vm.openCrew() }, Icons.Rounded.Groups)
                // Els vents: Gregal primer i tots els especialistes (abans només cinc).
                DrawerSection(if (en) "Winds" else "Vents")
                DrawerRow(gregal?.name ?: tr(lang, "nav.company"), home && panel == null, { close(); vm.openVentsPanel(null); nav(Screen.Company) }) { DrawerWindIcon(vm) }
                // Gregal i els cinc amb qui has parlat fa menys (com l'escriptori); la
                // resta, a «Tots els vents»: amb 20 o 30 vents el calaix no s'acabava.
                val chats by vm.windChat.collectAsState()
                val recent = drawerRecentWinds(winds, chats)
                recent.forEach { w ->
                    DrawerRow(w.name + if (w.role.isNotBlank()) " · " + w.role else "", false, { close(); vm.openVentsPanel(null); vm.openVentsChat("wind:" + w.id) }) {
                        WindHalo(Color(w.argb), w.faceStyle, size = 24, online = false, showKit = false)
                    }
                }
                if (winds.size > recent.size + 1) DrawerRow((if (en) "All winds · " else "Tots els vents · ") + winds.size, home && panel == "directory",
                    { close(); vm.openVentsPanel("directory") }, Icons.Rounded.Groups, muted = true)
                DrawerRow(if (en) "New specialist" else "Nou especialista", false, { close(); vm.showWindCreator(true) }, Icons.Rounded.Add, muted = true)
                DrawerSection(if (en) "Conversations" else "Converses")
                DrawerRow(askCopy(lang, "chats"), false, { close(); vm.openVentsPanel("chats") }, Icons.Rounded.Forum)
                // La resta, plegada: preferències, ordinador, guia, directori i programar des del mòbil.
                val showCode = codeOpen || screen in codeScreens
                var moreOpen by rememberSaveable { mutableStateOf(false) }
                DrawerGroup(if (en) "Settings & more" else "Configuració i més", Icons.Rounded.Settings, moreOpen || showCode || screen == Screen.Prefs, { moreOpen = !moreOpen }) {
                    DrawerRow(tr(lang, "prefs.title"), screen == Screen.Prefs, { nav(Screen.Prefs); vm.openPreferences() }, Icons.Rounded.Settings)
                    DrawerRow(if (en) "Computer" else "Ordinador", false, { close(); vm.openVentsPanel("computer") }, Icons.Rounded.Computer)
                    DrawerRow(tr(lang, "app.local.title"), false, { vm.openVentsPanel("local"); nav(Screen.Automatitzacions) }, Icons.Rounded.Place)
                    DrawerRow(guideIntro(lang).first, false, { close(); vm.openVentsPanel("guide") }, Icons.Rounded.AutoAwesome)
                    DrawerRow(tr(lang, "app.vents.myWinds"), home && panel == "directory", { close(); vm.openVentsPanel("directory") }, Icons.Rounded.Groups)
                    // Programar des del mòbil: l'agent de codi, les seves converses, el
                    // projecte i les feines.
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
                        DrawerRow(if (en) "Code work" else "Feina de codi", screen == Screen.Work, { nav(Screen.Work) }, Icons.Rounded.Construction)
                    }
                }
            }
            HorizontalDivider(Modifier.padding(vertical = 6.dp, horizontal = 8.dp), color = Line)
            DrawerRow(tr(lang, "app.server.switch"), false, { close(); vm.switchServer() }, Icons.Rounded.Dns, muted = true)
            if (vm.signedInWithAccount()) DrawerRow(tr(lang, "app.server.signOut", user), false, { close(); vm.signOut() }, Icons.AutoMirrored.Rounded.Logout, muted = true)
        }
    }
}

/** Un grup plegable de la barra (Feina, Desat, Configuració): una fila i, obert, les seves files. */
@Composable
@OptIn(androidx.compose.foundation.ExperimentalFoundationApi::class)
private fun DrawerGroup(label: String, icon: androidx.compose.ui.graphics.vector.ImageVector, open: Boolean, toggle: () -> Unit, badge: Int = 0, content: @Composable () -> Unit) {
    // En obrir-lo tu, el calaix baixa fins a les opcions: «Configuració i més»
    // és a baix de tot i, si no, s'obria fora de la pantalla i semblava mort.
    val bring = remember { androidx.compose.foundation.relocation.BringIntoViewRequester() }
    var openedByTap by remember { mutableStateOf(false) }
    DrawerRow(label, false, { openedByTap = !open; toggle() }, icon, badge = badge)
    if (open) Column(Modifier.padding(start = 14.dp).bringIntoViewRequester(bring)) { content() }
    LaunchedEffect(open, openedByTap) {
        if (open && openedByTap) { kotlinx.coroutines.delay(60); bring.bringIntoView(); openedByTap = false }
    }
}

@Composable
private fun DrawerSection(label: String) {
    Text(
        label, fontSize = 12.sp, fontWeight = FontWeight.SemiBold, color = Faint,
        modifier = Modifier.padding(start = 14.dp, top = 20.dp, bottom = 6.dp),
    )
}

@Composable
private fun DrawerRow(label: String, selected: Boolean, onClick: () -> Unit, icon: ImageVector, muted: Boolean = false, badge: Int = 0) =
    DrawerRow(label, selected, onClick, muted, badge) { Icon(icon, null, modifier = Modifier.size(22.dp)) }

/** Files tàctils de 48 dp, amb selecció suau i sense indicador duplicat. */
@Composable
private fun DrawerRow(label: String, selected: Boolean, onClick: () -> Unit, muted: Boolean = false, badge: Int = 0, icon: @Composable () -> Unit) {
    val shape = RoundedCornerShape(14.dp)
    Row(
        Modifier.fillMaxWidth().clip(shape)
            .background(if (selected) SeaSoft(0.14f) else Color.Transparent)
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
            label, fontSize = 15.sp, fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
            color = when { selected -> Ink; muted -> Muted; else -> Tinta },
            maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f),
        )
        if (badge > 0) Box(Modifier.background(Sea, CircleShape).padding(horizontal = 7.dp, vertical = 1.dp)) {
            Text(badge.toString(), color = OnSea, fontSize = 11.sp, fontWeight = FontWeight.Bold)
        }

    }
}

/** Els vents del calaix: els cinc amb l'últim missatge més recent (sense Gregal, que va a part); si n'hi ha menys, els primers de la llista. */
internal fun drawerRecentWinds(winds: List<Wind>, chats: Map<String, List<GregalViewModel.PetMsg>>, max: Int = 5): List<Wind> {
    val others = winds.filter { it.id != "gregal" }
    val last = { w: Wind -> chats[w.id].orEmpty().maxOfOrNull { it.ts } ?: 0L }
    val used = others.filter { last(it) > 0 }.sortedByDescending(last)
    return (used + others.filterNot { it in used }).take(max)
}
