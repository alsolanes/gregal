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
import androidx.compose.material.icons.rounded.Alarm
import androidx.compose.material.icons.rounded.Checklist
import androidx.compose.material.icons.rounded.Visibility
import androidx.compose.material.icons.rounded.History
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.Article
import androidx.compose.material.icons.automirrored.rounded.Logout
import androidx.compose.material.icons.automirrored.rounded.Undo
import androidx.compose.material.icons.rounded.Dns
import androidx.compose.material.icons.rounded.Folder
import androidx.compose.material.icons.rounded.EditNote
import androidx.compose.material.icons.rounded.Language
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
import androidx.compose.material.icons.rounded.Groups
import androidx.compose.material.icons.rounded.ExpandLess
import androidx.compose.material.icons.rounded.ExpandMore
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
 * El calaix de navegació, agrupat per a què serveix cada lloc: l'agent,
 * l'automatització i les eines; les preferències al peu. Els vents no hi
 * són: el selector Codi | Vents ja hi porta. La capçalera diu on estàs connectat i en quin
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
    if (simple) { SimpleVentsDrawer(vm, lang, screen, local, server, nav, close); return }
    val windBusy by vm.windBusy.collectAsState()
    val windNews by vm.windNews.collectAsState()
    val windAttention = windBusy.isNotEmpty() || windNews.isNotEmpty()
    CappedFontScale(1.5f) {
        Column(Modifier.fillMaxSize().padding(horizontal = 14.dp, vertical = 8.dp)) {
            Column(Modifier.weight(1f).verticalScroll(rememberScrollState())) {
                Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(horizontal = 6.dp)) {
                    GregalMark(36.dp)
                    Spacer(Modifier.width(12.dp))
                    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text("Gregal", fontSize = GregalSize.title, fontWeight = FontWeight.Bold, color = Ink, letterSpacing = (-0.3).sp)
                        Text(if (local) tr(lang, "app.server.thisDevice") else server, color = Muted, fontSize = GregalSize.caption, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                }
                Spacer(Modifier.height(14.dp))
                Box(Modifier.padding(horizontal = 6.dp).clip(RoundedCornerShape(50)).clickable { nav(Screen.Carpeta) }) {
                    ProjectPill(project.ifEmpty { tr(lang, "app.nav.project") }, branch)
                }
                Spacer(Modifier.height(12.dp))
                ModeSwitcher(lang, activeAppArea(screen, simple), windAttention, { area -> close(); vm.switchArea(area) },
                    Modifier.fillMaxWidth().padding(horizontal = 6.dp).testTag("area-switcher"))
                Spacer(Modifier.height(12.dp))
                DrawerRow(tr(lang, "nav.new"), false, { vm.newSession(); nav(Screen.Chat) }) { Icon(Icons.Rounded.Add, null, tint = Sea, modifier = Modifier.size(22.dp)) }
                Spacer(Modifier.height(6.dp))
                DrawerRow(tr(lang, "app.nav.chat"), screen == Screen.Chat, { nav(Screen.Chat) }, Icons.Rounded.ChatBubbleOutline)
                DrawerRow(tr(lang, "app.work.title"), screen == Screen.Work, { nav(Screen.Work) }, Icons.Rounded.Construction)
                DrawerRow(tr(lang, "nav.convs"), screen == Screen.Sessions, { nav(Screen.Sessions); vm.loadSessions() }, Icons.Rounded.Forum)
                DrawerRow(tr(lang, "nav.backlog"), screen == Screen.Backlog, { nav(Screen.Backlog); vm.loadBacklog() }, Icons.Rounded.Checklist)
                DrawerRow(tr(lang, "nav.automations"), screen in setOf(Screen.Automatitzacions, Screen.Jobs, Screen.Flows), {
                    nav(Screen.Automatitzacions); vm.loadAutomations(""); vm.loadJobs(); vm.loadFlows()
                }, Icons.Rounded.Schedule)
                Spacer(Modifier.height(6.dp))
                DrawerRow(tr(lang, "app.carpeta.title"), screen == Screen.Carpeta, { nav(Screen.Carpeta) }, Icons.Rounded.FolderOpen)
                DrawerRow(tr(lang, "nav.notes"), screen == Screen.Notes, { nav(Screen.Notes); vm.loadNotes() }, Icons.Rounded.EditNote)
                DrawerRow(tr(lang, "docs.home"), screen == Screen.Docs, { close(); vm.openDocsHome() }, Icons.AutoMirrored.Rounded.Article)
                // GitHub és de l'administrador del servidor (un altre compte rebria 403),
                // i al dispositiu no hi ha la CLI gh: només en remot.
                val admin by vm.isAdmin.collectAsState()
                if (!local && admin) DrawerRow(tr(lang, "app.nav.github"), false, { close(); vm.obreGithub() }, Icons.Rounded.Code)
            }
            HorizontalDivider(Modifier.padding(vertical = 6.dp, horizontal = 8.dp), color = Line)
            DrawerRow(tr(lang, "prefs.title"), screen == Screen.Prefs, { nav(Screen.Prefs); vm.openPreferences() }, Icons.Rounded.Settings)
            DrawerRow(tr(lang, "app.server.switch"), false, { close(); vm.switchServer() }, Icons.AutoMirrored.Rounded.Logout, muted = true)
            // Amb compte (el servidor de casa), sortir de debò: el token es tanca al servidor.
            if (vm.signedInWithAccount()) DrawerRow(tr(lang, "app.server.signOut", vm.usuari.collectAsState().value), false, { close(); vm.signOut() }, Icons.AutoMirrored.Rounded.Logout, muted = true)
            UserAvatarRow(vm, lang) { nav(Screen.Prefs); vm.openPreferences(PrefsPage.General) }
        }
    }
}

/**
 * Vents net: quatre llocs, com les apps de companys. Els vents (la portada),
 * el directori dels teus vents, Explora (programats, desats, wiki, biblioteca,
 * feines…) i les notes. El Ventet és la mateixa portada. Programar des del
 * mòbil continua a mà, a la pàgina de Configuració, perquè no competeixi amb els vents.
 */
@Composable
private fun SimpleVentsDrawer(
    vm: GregalViewModel, lang: String, screen: Screen, local: Boolean, server: String,
    nav: (Screen) -> Unit, close: () -> Unit,
) {
    val user by vm.usuari.collectAsState()
    val panel by vm.ventsPanel.collectAsState()
    val windBusy by vm.windBusy.collectAsState()
    val windNews by vm.windNews.collectAsState()
    val windAttention = windBusy.isNotEmpty() || windNews.isNotEmpty()
    CappedFontScale(1.5f) {
        Column(Modifier.fillMaxSize().padding(horizontal = 14.dp, vertical = 8.dp)) {
            Column(Modifier.weight(1f).verticalScroll(rememberScrollState())) {
                Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(horizontal = 6.dp)) {
                    GregalMark(36.dp)
                    Spacer(Modifier.width(12.dp))
                    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text("Gregal Vents", fontSize = GregalSize.title, fontWeight = FontWeight.Bold, color = Ink, letterSpacing = (-0.3).sp)
                        Text(if (local) tr(lang, "app.server.thisDevice") else if (user.isNotBlank()) "$user · $server" else server, color = Muted, fontSize = GregalSize.caption, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                }
                Spacer(Modifier.height(18.dp))
                ModeSwitcher(lang, activeAppArea(screen, true), windAttention, { area -> close(); vm.switchArea(area) },
                    Modifier.fillMaxWidth().padding(horizontal = 6.dp).testTag("area-switcher"))
                Spacer(Modifier.height(12.dp))
                val home = screen == Screen.Company || screen == Screen.Ventet
                val winds by vm.winds.collectAsState()
                val tasks by vm.windTasks.collectAsState()
                val en = lang == "en"
                // Un sol nivell, igual que l'escriptori (renderNav a winds-workspace.js):
                // començar i Avui; les converses, a dalt perquè no calgui baixar-hi;
                // els vents; la resta d'espais; i Configuració al peu, com a pàgina.
                val gregal = winds.firstOrNull { it.id == "gregal" }
                DrawerRow(if (en) "New conversation" else "Nova conversa", false, { close(); vm.openVentsPanel(null); vm.openVentsChat(null); nav(Screen.Company) }) {
                    Icon(Icons.Rounded.Add, null, tint = Sea, modifier = Modifier.size(22.dp))
                }
                Spacer(Modifier.height(4.dp))
                DrawerRow(if (en) "Today" else "Avui", home && panel == "today", { close(); vm.openVentsPanel("today"); nav(Screen.Company) }, Icons.Rounded.WbSunny)
                DrawerSection(if (en) "Conversations" else "Converses")
                // Les converses, a soles i en grup, de la més nova (com renderThreads a la web):
                // cinc, i la resta plegada sota «Mostra'n més». La que tens oberta sempre es veu.
                val chats by vm.windChat.collectAsState()
                val groups by vm.rotllanes.collectAsState()
                val groupChats by vm.rotllanaChat.collectAsState()
                val summaries by vm.summaries.collectAsState()
                val openConv by vm.ventsOpen.collectAsState()
                val convs = remember(winds, groups, chats, groupChats, summaries) {
                    ventsInbox(winds, groups, chats, groupChats, summaries = summaries).filter { it.hasMessages }
                }
                var allConvs by rememberSaveable { mutableStateOf(false) }
                val shownConvs = if (allConvs) convs else convs.filterIndexed { i, c -> i < 5 || c.key == openConv }
                shownConvs.forEach { c ->
                    DrawerRow(c.title, home && panel == null && c.key == openConv, { close(); vm.openVentsPanel(null); vm.openVentsChat(c.key) }) {
                        val w = c.members.singleOrNull()
                        if (c.key.startsWith("wind:") && w != null) WindHalo(Color(w.argb), w.faceStyle, size = 24, online = false, showKit = false)
                        else Icon(Icons.Rounded.Groups, null, modifier = Modifier.size(22.dp))
                    }
                }
                if (convs.size > 5) {
                    val hidden = convs.size - shownConvs.size
                    if (allConvs) DrawerRow(tr(lang, "app.drawer.showLess"), false, { allConvs = false }, Icons.Rounded.ExpandLess, muted = true)
                    else if (hidden > 0) DrawerRow(tr(lang, "app.drawer.showMore", hidden), false, { allConvs = true }, Icons.Rounded.ExpandMore, muted = true)
                }
                DrawerRow(tr(lang, "app.drawer.allConversations"), false, { close(); vm.openVentsPanel("chats") }, Icons.Rounded.Forum)
                // Els vents: Gregal primer i tots els especialistes (abans només cinc).
                DrawerSection(if (en) "Winds" else "Vents")
                // Marcada a la portada, o amb el fil de Gregal obert si no surt a Converses: una sola fila marcada.
                val gregalHere = openConv == null || (openConv == "wind:gregal" && shownConvs.none { it.key == openConv })
                DrawerRow(gregal?.name ?: tr(lang, "nav.company"), home && panel == null && gregalHere, { close(); vm.openVentsPanel(null); vm.openVentsChat(if (gregal != null) "wind:gregal" else null); nav(Screen.Company) }) { DrawerWindIcon(vm) }
                // Gregal i els cinc amb qui has parlat fa menys (com l'escriptori); la
                // resta, a «Tots els vents»: amb 20 o 30 vents el calaix no s'acabava.
                val newsCount by vm.windNewsCount.collectAsState()
                val arrival by vm.windArrival.collectAsState()
                val recent = drawerRecentWinds(winds, chats, newsCount)
                recent.forEach { w ->
                    DrawerRow(w.name + if (w.role.isNotBlank()) " · " + w.role else "", false, { close(); vm.openVentsPanel(null); vm.openVentsChat("wind:" + w.id) },
                        badge = newsCount[w.id] ?: 0) {
                        NewsBounce(arrival?.takeIf { it.wind == w.id }) {
                            WindHalo(Color(w.argb), w.faceStyle, size = 24, online = false, showKit = false)
                        }
                    }
                }
                if (winds.size > recent.size + 1) DrawerRow((if (en) "All winds · " else "Tots els vents · ") + winds.size, home && panel == "directory",
                    { close(); vm.openVentsPanel("directory") }, Icons.Rounded.Groups, muted = true)
                Spacer(Modifier.height(12.dp))
                val running = tasks.orEmpty().count { it.running }
                DrawerRow(tr(lang, "docs.home"), screen == Screen.Docs, { close(); vm.openDocsHome() }, Icons.AutoMirrored.Rounded.Article)
                DrawerRow(tr(lang, "nav.automations"), screen == Screen.Automatitzacions, { nav(Screen.Automatitzacions) }, Icons.Rounded.Alarm, badge = running)
                DrawerRow(if (en) "Saved" else "Desat", screen == Screen.Desat, { nav(Screen.Desat) }, Icons.Rounded.Bookmark)
            }
            HorizontalDivider(Modifier.padding(vertical = 6.dp, horizontal = 8.dp), color = Line)
            DrawerRow(tr(lang, "app.settings.title"), screen == Screen.Settings || screen == Screen.Prefs, { nav(Screen.Settings) }, Icons.Rounded.Settings)
            DrawerRow(tr(lang, "app.server.switch"), false, { close(); vm.switchServer() }, Icons.Rounded.Dns, muted = true)
            if (vm.signedInWithAccount()) DrawerRow(tr(lang, "app.server.signOut", user), false, { close(); vm.signOut() }, Icons.AutoMirrored.Rounded.Logout, muted = true)
            UserAvatarRow(vm, lang) { nav(Screen.Prefs); vm.openPreferences(PrefsPage.General) }
        }
    }
}

/** Un grup plegable de la barra (Feina, Desat): una fila i, obert, les seves files. */
@Composable
@OptIn(androidx.compose.foundation.ExperimentalFoundationApi::class)
private fun DrawerGroup(label: String, icon: androidx.compose.ui.graphics.vector.ImageVector, open: Boolean, toggle: () -> Unit, badge: Int = 0, content: @Composable () -> Unit) {
    // En obrir-lo tu, el calaix baixa fins a les opcions: Feina i Desat són
    // a baix de tot i, si no, s'obrien fora de la pantalla i semblaven morts.
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
        label, fontSize = GregalSize.caption, fontWeight = FontWeight.SemiBold, color = Faint,
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
            label, fontSize = GregalSize.body, fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
            color = when { selected -> Ink; muted -> Muted; else -> Tinta },
            maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f),
        )
        if (badge > 0) Box(Modifier.background(Sea, CircleShape).padding(horizontal = 7.dp, vertical = 1.dp)) {
            Text(badge.toString(), color = OnSea, fontSize = GregalSize.label, fontWeight = FontWeight.Bold)
        }

    }
}

/**
 * Els vents del calaix: primer els que tenen novetats per llegir, després els
 * cinc amb l'últim missatge més recent (sense Gregal, que va a part); si n'hi
 * ha menys, els primers de la llista.
 */
internal fun drawerRecentWinds(winds: List<Wind>, chats: Map<String, List<GregalViewModel.PetMsg>>, news: Map<String, Int> = emptyMap(), max: Int = 5): List<Wind> {
    val others = winds.filter { it.id != "gregal" }
    val last = { w: Wind -> chats[w.id].orEmpty().maxOfOrNull { it.ts } ?: 0L }
    val fresh = others.filter { (news[it.id] ?: 0) > 0 }
    val used = others.filter { last(it) > 0 && it !in fresh }.sortedByDescending(last)
    return (fresh + used + others.filterNot { it in used || it in fresh }).take(max)
}
