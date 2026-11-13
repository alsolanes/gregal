package xyz.solanes.gregal

import androidx.compose.foundation.layout.imePadding

import androidx.compose.foundation.layout.heightIn
import androidx.compose.material.icons.automirrored.rounded.KeyboardArrowRight
import androidx.compose.material.icons.rounded.Palette
import androidx.compose.material.icons.rounded.Link
import androidx.compose.material.icons.rounded.CalendarMonth
import androidx.compose.material.icons.rounded.MailOutline
import androidx.compose.material.icons.rounded.Settings
import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.ui.draw.clip
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.toggleable
import androidx.compose.ui.semantics.Role
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.Toys
import androidx.compose.material.icons.rounded.Dns
import androidx.compose.material.icons.rounded.FolderOpen
import androidx.compose.material.icons.rounded.SystemUpdate
import androidx.compose.material.icons.rounded.Verified
import androidx.compose.material.icons.rounded.Lock
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.TextButton
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.material.icons.rounded.Menu
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.Checkbox
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Preferències: aparença i llengua. Paritat amb el panell prefs de la web
 *  (mateixes opcions de tema; densitat/mida les cobreix el sistema). */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PrefsScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val tema by vm.tema.collectAsState()
    val temaApp by vm.temaApp.collectAsState()
    val idioma by vm.idioma.collectAsState()
    val idiomaChoice by vm.idiomaChoice.collectAsState()
    val capHome by vm.capHome.collectAsState()
    val expressiveMotion by vm.expressiveMotion.collectAsState()
    val st by vm.state.collectAsState()
    val updateMsg by vm.updateMsg.collectAsState()
    val info by vm.update.collectAsState()
    val prog by vm.updateProg.collectAsState()
    // Només s'ensenya la targeta si el manifest és més nou que el
    // versionCode instal·lat (si no, seria un botó que no fa res).
    val teNova = info?.takeIf { it.isAvailable }
    fun t(k: String, vararg args: Any?) = tr(idioma, k, *args)
    // Com es diuen els vents: el camp comença amb el que hi ha desat.
    var nomSing by remember { mutableStateOf(VentNom.singular) }
    var nomPlur by remember { mutableStateOf(VentNom.plural) }
    var nomsDesats by remember { mutableStateOf(false) }
    val conn by vm.conn.collectAsState()
    val admin by vm.isAdmin.collectAsState()
    val simple by vm.simpleVents.collectAsState()
    val requestedPage by vm.prefsPage.collectAsState()
    val page = requestedPage.takeIf { it.available(!simple, admin) } ?: PrefsPage.Index
    val scroll = androidx.compose.runtime.key(page) { rememberScrollState() }
    fun open(target: PrefsPage) = vm.openPreferences(target)
    GregalBg {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                GregalTopBar(page.title(idioma), idioma,
                    onMenu = if (page == PrefsPage.Index) onMenu else null,
                    onBack = if (page != PrefsPage.Index) ({ vm.back() }) else null)
            },
            snackbarHost = { NoticeHost(vm) },
        ) { pad ->
            Column(Modifier.fillMaxSize().padding(pad).imePadding().verticalScroll(scroll)
                .padding(start = 16.dp, end = 16.dp, bottom = 32.dp)) {
                when (page) {
                    PrefsPage.Index -> {
                        PrefsLink(PrefsPage.General, idioma, if (idioma == "en") "Language, connection, notifications and updates" else "Idioma, connexió, avisos i actualitzacions", ::open)
                        PrefsLink(PrefsPage.Appearance, idioma, if (idioma == "en") "Theme and personalisation" else "Tema i personalització", ::open)
                        PrefsSection("Agent")
                        PrefsLink(PrefsPage.Models, idioma, if (idioma == "en") "Current model and provider" else "Model actual i proveïdor", ::open)
                        if (!simple) PrefsLink(PrefsPage.Permissions, idioma, if (idioma == "en") "Approvals and reviewer" else "Aprovacions i revisor", ::open)
                        PrefsSection(if (idioma == "en") "Application" else "Aplicació")
                        PrefsLink(PrefsPage.Connections, idioma, if (idioma == "en") "Accounts, calendar and email" else "Comptes, calendari i correu", ::open)
                        if (admin) PrefsLink(PrefsPage.Server, idioma, if (idioma == "en") "Server administration" else "Administració del servidor", ::open)
                    }
                    PrefsPage.General -> {
                        PrefsGroup(Modifier.fillMaxWidth()) {
                            Text(t("prefs.lang"), color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                            Spacer(Modifier.height(10.dp))
                            ChoicePills(listOf("system" to t("app.prefs.languageSystem"), "ca" to "Català", "en" to "English"), idiomaChoice, { vm.setIdioma(it) })
                        }
                        PrefsSection(t("app.prefs.server"))
                        SettingsRow(Icons.Rounded.Dns, if (conn.local) t("app.server.thisDevice") else "${conn.host}:${conn.port}",
                            trailing = { TextButton(onClick = { vm.switchServer() }) { Text(t("app.server.switch")) } })
                        if (!conn.local) PrefsLink(PrefsPage.Notifications, idioma, if (idioma == "en") "Device notification status" else "Estat dels avisos del dispositiu", ::open)
                        PreferenceSwitchRow(t("prefs.codingTools"), t("prefs.codingToolsHelp"), !simple) { vm.setSimpleVents(!it) }
                        PrefsLink(PrefsPage.About, idioma, "Gregal Android ${vm.versionNameInstalada()}", ::open)
                    }
                    PrefsPage.About -> {
                        PrefsSection(t("app.settings.version"))
                        PrefsGroup(Modifier.fillMaxWidth(), padding = PaddingValues(vertical = 4.dp)) {
                            SettingsRow(
                                Icons.Rounded.SystemUpdate, "Gregal Android ${vm.versionNameInstalada()}",
                                updateMsg.ifBlank { "vc${vm.versionCodeInstalada()}" }, tint = Lila,
                                trailing = { TextButton(onClick = { vm.checkUpdate() }) { Text(t("app.settings.checkUpdates"), color = Sea) } },
                            )
                        }
                        teNova?.let { r ->
                            Spacer(Modifier.height(8.dp))
                            GlassCard(Modifier.fillMaxWidth(), hi = true) {
                                Column(Modifier.padding(12.dp)) {
                                    Text(t("app.update.install", r.latestVersion), color = Sea, fontSize = 15.sp)
                                    if (r.changelog.isNotEmpty()) {
                                        Spacer(Modifier.height(4.dp))
                                        Text(r.changelog, color = Tinta, style = GregalType.bodySmall)
                                    }
                                    Spacer(Modifier.height(8.dp))
                                    if (prog >= 0) {
                                        // Baixant: barra + percentatge, sense botons.
                                        LinearProgressIndicator(
                                            progress = { prog / 100f },
                                            modifier = Modifier.fillMaxWidth(),
                                            color = Sea, trackColor = Line,
                                        )
                                        Spacer(Modifier.height(4.dp))
                                        Text(t("app.update.downloadProgress", prog), color = Muted, fontSize = 12.sp)
                                    } else {
                                        Row(verticalAlignment = Alignment.CenterVertically) {
                                            Button(onClick = { vm.baixaIInstalla() }) { Text(t("app.update.downloadInstall")) }
                                            Spacer(Modifier.width(8.dp))
                                            OutlinedButton(onClick = { vm.ignoraAquestaVersio() }) {
                                                Text(t("app.update.silence"))
                                            }
                                        }
                                        Spacer(Modifier.height(4.dp))
                                        Text(
                                            t("app.update.shaHelp"),
                                            color = Faint, fontSize = 11.sp,
                                        )
                                    }
                                }
                            }
                        }
                    }
                    PrefsPage.Appearance -> {
                        PrefsLink(PrefsPage.Theme, idioma, t("prefs.theme.system"), ::open)
                        PrefsLink(PrefsPage.Palette, idioma, t("prefs.tema." + temaApp), ::open)
                        PrefsLink(PrefsPage.Names, idioma,
                            listOf(VentNom.singular, VentNom.plural).filter { it.isNotBlank() }.joinToString(" · ")
                                .ifBlank { t("prefs.ventsNameHelp") }, ::open)
                        PreferenceSwitchRow(t("prefs.capHome"), t("prefs.capHomeHelp"), capHome) { vm.setCapHome(it) }
                        PreferenceSwitchRow(t("prefs.motion"), t("prefs.motionHelp"), expressiveMotion) { vm.setExpressiveMotion(it) }
                    }
                    PrefsPage.Theme -> {
                        Text(t("prefs.theme"), color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                        Spacer(Modifier.height(10.dp))
                        ChoicePills(listOf("sistema" to t("prefs.theme.system"), "fosc" to t("prefs.theme.dark"), "clar" to t("prefs.theme.light")), tema, { vm.setTema(it) })
                        Spacer(Modifier.height(8.dp))
                        Text(t("app.prefs.themeHelp"), color = Muted, fontSize = 12.5.sp, lineHeight = 17.sp)
                    }
                    PrefsPage.Palette -> {
                        Text(t("prefs.appTheme"), color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                        Spacer(Modifier.height(8.dp))
                        Text(t("prefs.appThemeHelp"), color = Muted, fontSize = 12.5.sp, lineHeight = 17.sp)
                        Spacer(Modifier.height(10.dp))
                        // El mostrari ensenya la paleta de debò: dues bombolles i
                        // l'accent, amb el clar/fosc que tens triat ara mateix.
                        val foscAra = esFosc(tema, isSystemInDarkTheme())
                        Row(
                            Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
                            horizontalArrangement = Arrangement.spacedBy(10.dp),
                        ) {
                            for (tem in GregalTemes) {
                                TemaMostra(
                                    tem = tem, fosc = foscAra, triat = temaApp == tem.id,
                                    nom = t("prefs.tema." + tem.id),
                                ) { vm.setTemaApp(tem.id) }
                            }
                        }
                        Spacer(Modifier.height(18.dp))
                    }
                    PrefsPage.Names -> {
                        PrefsSection(t("prefs.ventsName"))
                        PrefsGroup(Modifier.fillMaxWidth(), padding = PaddingValues(vertical = 10.dp)) {
                            Column(Modifier.fillMaxWidth().padding(horizontal = 12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                                Text(t("prefs.ventsNameHelp"), color = Muted, fontSize = 12.5.sp, lineHeight = 17.sp)
                                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                                    androidx.compose.material3.OutlinedTextField(nomSing, { nomSing = it.take(20) }, singleLine = true,
                                        label = { Text(t("prefs.ventsNameSing")) }, modifier = Modifier.weight(1f))
                                    androidx.compose.material3.OutlinedTextField(nomPlur, { nomPlur = it.take(20) }, singleLine = true,
                                        label = { Text(t("prefs.ventsNamePlur")) }, modifier = Modifier.weight(1f))
                                }
                                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                                    Button(onClick = { vm.desaNomsDelVent(nomSing, nomPlur); nomsDesats = true }) { Text(t("prefs.ventsNameSave")) }
                                    if (nomsDesats) Text(t("prefs.ventsNameSaved"), color = Sea, fontSize = 12.5.sp)
                                    Spacer(Modifier.weight(1f))
                                    TextButton(onClick = { nomSing = ""; nomPlur = ""; vm.desaNomsDelVent("", ""); nomsDesats = false }) {
                                        Text(t("prefs.ventsNameReset"))
                                    }
                                }
                            }
                        }
                    }
                    PrefsPage.Notifications -> {
                        if (!conn.local) PushStatusRow(conn, idioma)
                    }
                    PrefsPage.Provider -> {
                        if (conn.local || admin) ProviderSetup(vm, current = st.provider, change = true)
                    }
                    PrefsPage.Models -> {
                        SettingsRow(Icons.Rounded.AutoAwesome, t("app.prefs.model"),
                            st.provider.ifBlank { "—" } + if (st.model.isNotBlank()) " · " + st.model else "")
                        if (conn.local || admin) PrefsLink(PrefsPage.Provider, idioma, st.provider, ::open)
                        if (!simple) {
                            PrefsSection(t("prefs.verify"))
                            PrefsGroup(Modifier.fillMaxWidth(), padding = PaddingValues(vertical = 4.dp)) {
                                Row(
                                    Modifier.fillMaxWidth().toggleable(st.verify == "auto", role = Role.Switch) { vm.setVerify(it) }
                                        .padding(horizontal = 16.dp, vertical = 12.dp),
                                    verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp),
                                ) {
                                    Box(Modifier.size(36.dp).background(Green.copy(alpha = 0.14f), RoundedCornerShape(11.dp)), contentAlignment = Alignment.Center) {
                                        Icon(Icons.Rounded.Verified, null, tint = Green, modifier = Modifier.size(20.dp))
                                    }
                                    Text(t("prefs.verify.help"), color = Tinta, fontSize = 14.sp, lineHeight = 19.sp, modifier = Modifier.weight(1f))
                                    Switch(checked = st.verify == "auto", onCheckedChange = null,
                                        colors = SwitchDefaults.colors(checkedTrackColor = Sea, checkedThumbColor = OnSea))
                                }
                            }

                        }
                    }
                    PrefsPage.Permissions -> {
                        PrefsGroup(Modifier.fillMaxWidth()) {
                            for (mode in listOf("ask", "edits", "full")) {
                                val selected = st.access.ifBlank { "edits" } == mode
                                Row(Modifier.fillMaxWidth().selectable(selected, role = Role.RadioButton) { vm.setAccess(mode) }
                                    .heightIn(min = 64.dp).padding(vertical = 12.dp),
                                    verticalAlignment = Alignment.CenterVertically) {
                                    RadioButton(selected, onClick = null)
                                    Spacer(Modifier.width(12.dp))
                                    Column(Modifier.weight(1f)) {
                                        Text(t("app.perm.$mode"), color = Ink, fontSize = 15.sp)
                                        Text(t("app.perm.${mode}Help"), color = Muted, fontSize = 12.5.sp, lineHeight = 18.sp)
                                    }
                                }
                            }
                        }

                        PrefsSection(t("prefs.reviewer"))
                        PrefsGroup(Modifier.fillMaxWidth(), padding = PaddingValues(vertical = 4.dp)) {
                            Row(
                                Modifier.fillMaxWidth().toggleable(st.autoReview, role = Role.Switch) { vm.setReviewer(it) }
                                    .padding(horizontal = 16.dp, vertical = 12.dp),
                                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp),
                            ) {
                                Box(Modifier.size(36.dp).background(Sand.copy(alpha = 0.14f), RoundedCornerShape(11.dp)), contentAlignment = Alignment.Center) {
                                    Icon(Icons.Rounded.Lock, null, tint = Sand, modifier = Modifier.size(20.dp))
                                }
                                Text(t("prefs.reviewer.help"), color = Tinta, fontSize = 14.sp, lineHeight = 19.sp, modifier = Modifier.weight(1f))
                                Switch(checked = st.autoReview, onCheckedChange = null,
                                    colors = SwitchDefaults.colors(checkedTrackColor = Sea, checkedThumbColor = OnSea))
                            }
                        }
                    }
                    PrefsPage.Connections -> {
                        PrefsLink(PrefsPage.Accounts, idioma, if (idioma == "en") "Services, status and permissions" else "Serveis, estat i permisos", ::open)
                        PrefsLink(PrefsPage.CalendarMail, idioma, "Google Calendar · Gmail", ::open)
                    }
                    PrefsPage.Accounts -> ConnexionsEntry(conn, idioma)
                    PrefsPage.CalendarMail -> {
                        PrefsLink(PrefsPage.Calendar, idioma, t("app.conn.calName"), ::open)
                        PrefsLink(PrefsPage.Mail, idioma, t("app.conn.mailUser"), ::open)
                        PrefsLink(PrefsPage.Briefing, idioma, t("app.conn.briefHelp"), ::open)
                    }
                    PrefsPage.Calendar, PrefsPage.Mail, PrefsPage.Briefing -> ConnectionsCard(vm, idioma, page)
                    PrefsPage.Server -> {
                        if (!conn.local && admin) InviteCard(vm, idioma, conn.host + if (conn.host.startsWith("https://")) "" else ":" + conn.port)
                        if (admin) ServerPowersCard(vm, idioma)
                    }
                }
            }
        }
    }
}

/** Quiet settings groups, with no repeated frame around each control. */
@Composable
private fun PrefsGroup(modifier: Modifier = Modifier, padding: PaddingValues = PaddingValues(12.dp), content: @Composable androidx.compose.foundation.layout.ColumnScope.() -> Unit) {
    Column(modifier.padding(padding), content = content)
}

@Composable
private fun PrefsLink(page: PrefsPage, lang: String, detail: String, open: (PrefsPage) -> Unit) {
    val icon = when (page) {
        PrefsPage.Appearance, PrefsPage.Palette, PrefsPage.Theme -> Icons.Rounded.Palette
        PrefsPage.Models -> Icons.Rounded.AutoAwesome
        PrefsPage.Permissions -> Icons.Rounded.Lock
        PrefsPage.Connections, PrefsPage.Accounts -> Icons.Rounded.Link
        PrefsPage.CalendarMail, PrefsPage.Calendar -> Icons.Rounded.CalendarMonth
        PrefsPage.Mail -> Icons.Rounded.MailOutline
        PrefsPage.Server -> Icons.Rounded.Dns
        else -> Icons.Rounded.Settings
    }
    Row(Modifier.fillMaxWidth().clickable { open(page) }.heightIn(min = 72.dp).padding(horizontal = 8.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
        Icon(icon, null, tint = Muted, modifier = Modifier.size(24.dp))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(page.title(lang), color = Ink, fontSize = 15.sp)
            Text(detail, color = Muted, fontSize = 12.5.sp, lineHeight = 18.sp)
        }
        Icon(Icons.AutoMirrored.Rounded.KeyboardArrowRight, null, tint = Muted)
    }
}
/** Títol de secció de preferències: el mateix estil discret de tota l'app. */
@Composable
private fun PrefsSection(text: String) {
    Text(
        text, fontSize = 12.sp, fontWeight = FontWeight.SemiBold, color = Faint,
        modifier = Modifier.padding(start = 4.dp, top = 22.dp, bottom = 8.dp),
    )
}

@Composable
private fun PreferenceSwitchRow(title: String, help: String, checked: Boolean, onChecked: (Boolean) -> Unit) {
    Row(Modifier.fillMaxWidth().toggleable(checked, role = Role.Switch, onValueChange = onChecked).padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(3.dp)) {
            Text(title, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
            Text(help, color = Muted, fontSize = 12.5.sp, lineHeight = 17.sp)
        }
        Switch(checked = checked, onCheckedChange = null, colors = SwitchDefaults.colors(checkedTrackColor = Sea, checkedThumbColor = OnSea))
    }
}

/** Un quadre del selector de temes: mini conversa amb la paleta de debò. */
@Composable
private fun TemaMostra(tem: GregalTema, fosc: Boolean, triat: Boolean, nom: String, onPick: () -> Unit) {
    val p = tem.perMode(fosc)
    Column(
        Modifier.width(96.dp)
            .clip(RoundedCornerShape(14.dp))
            .background(p.surface)
            .border(if (triat) 2.dp else 1.dp, if (triat) p.sea else p.lineHi, RoundedCornerShape(14.dp))
            .clickable(onClick = onPick)
            .padding(9.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box(Modifier.fillMaxWidth().height(52.dp).clip(RoundedCornerShape(10.dp)).background(p.deepTop)) {
            Column(Modifier.fillMaxSize().padding(6.dp), verticalArrangement = Arrangement.spacedBy(5.dp)) {
                Box(Modifier.width(38.dp).height(10.dp).clip(RoundedCornerShape(6.dp)).background(p.surfaceHi))
                Box(
                    Modifier.width(26.dp).height(10.dp).clip(RoundedCornerShape(6.dp))
                        .background(p.sea).align(Alignment.End),
                )
                Box(Modifier.width(30.dp).height(6.dp).clip(RoundedCornerShape(4.dp)).background(p.surfaceHi))
            }
        }
        Spacer(Modifier.height(7.dp))
        Text(
            nom, color = if (triat) p.sea else p.text, fontSize = 12.sp, maxLines = 1,
            fontWeight = if (triat) FontWeight.SemiBold else FontWeight.Normal,
        )
    }
}


/** Genera un codi d'un sol ús i el comparteix amb l'adreça del servidor. */
@Composable
private fun InviteCard(vm: GregalViewModel, lang: String, server: String) {
    val ctx = LocalContext.current
    var code by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    PrefsGroup(Modifier.fillMaxWidth()) {
        Text(tr(lang, "app.invite.title"), color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.height(4.dp))
        Text(tr(lang, "app.invite.help"), color = Muted, fontSize = 12.5.sp, lineHeight = 17.sp)
        Spacer(Modifier.height(10.dp))
        if (code.isNotEmpty()) {
            androidx.compose.foundation.text.selection.SelectionContainer {
                Text(code, color = Ink, fontSize = 15.sp, fontFamily = androidx.compose.ui.text.font.FontFamily.Monospace)
            }
            Spacer(Modifier.height(8.dp))
            androidx.compose.material3.Button(onClick = {
                val text = tr(lang, "app.invite.share", server, code)
                val send = android.content.Intent(android.content.Intent.ACTION_SEND).setType("text/plain").putExtra(android.content.Intent.EXTRA_TEXT, text)
                ctx.startActivity(android.content.Intent.createChooser(send, tr(lang, "app.invite.title")))
            }, shape = RoundedCornerShape(14.dp), modifier = Modifier.fillMaxWidth()) { Text(tr(lang, "app.invite.shareButton")) }
        } else {
            androidx.compose.material3.OutlinedButton(onClick = {
                busy = true
                vm.createInvitation { code = it; busy = false }
            }, enabled = !busy, shape = RoundedCornerShape(14.dp), modifier = Modifier.fillMaxWidth()) { Text(tr(lang, "app.invite.create")) }
        }
    }
}

/**
 * Poders de servidor: els vents de l'administrador fan de sysadmin (webs,
 * desplegaments, túnels de Cloudflare). Les notes són el que saben de la
 * màquina; les llegeixen a cada torn.
 */
@Composable
private fun ServerPowersCard(vm: GregalViewModel, lang: String) {
    val sp by vm.serverPowers.collectAsState()
    LaunchedEffect(Unit) { vm.loadServerPowers() }
    val p = sp ?: return
    var notes by remember(p.notes) { mutableStateOf(p.notes) }
    var saving by remember { mutableStateOf(false) }
    PrefsGroup(Modifier.fillMaxWidth()) {
        PreferenceSwitchRowPlain(tr(lang, "app.powers.title"), tr(lang, "app.powers.help"), p.enabled) { vm.setServerPowers(enabled = it) }
        if (p.tools.isNotEmpty()) Text(tr(lang, "app.powers.machine", p.tools.joinToString(", ")), color = Muted, fontSize = 12.5.sp, lineHeight = 17.sp)
        p.tunnels.forEach { (url, what) -> Text(url + "  →  " + what, color = Ink, fontSize = 12.5.sp) }
        if (p.enabled) {
            Spacer(Modifier.height(10.dp))
            Text(tr(lang, "app.powers.notes"), color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
            Text(tr(lang, "app.powers.notesHelp"), color = Muted, fontSize = 12.5.sp, lineHeight = 17.sp)
            Spacer(Modifier.height(6.dp))
            androidx.compose.material3.OutlinedTextField(notes, { notes = it }, modifier = Modifier.fillMaxWidth(), minLines = 4, maxLines = 12,
                textStyle = androidx.compose.ui.text.TextStyle(fontFamily = androidx.compose.ui.text.font.FontFamily.Monospace, fontSize = 13.sp))
            Spacer(Modifier.height(6.dp))
            androidx.compose.material3.OutlinedButton(onClick = { saving = true; vm.setServerPowers(notes = notes) { saving = false } },
                enabled = !saving && notes != p.notes, shape = RoundedCornerShape(14.dp)) { Text(tr(lang, "app.powers.save")) }
        }
    }
}

@Composable
private fun PreferenceSwitchRowPlain(title: String, help: String, checked: Boolean, onChecked: (Boolean) -> Unit) {
    Row(Modifier.fillMaxWidth().toggleable(checked, role = Role.Switch, onValueChange = onChecked).padding(vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(3.dp)) {
            Text(title, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
            Text(help, color = Muted, fontSize = 12.5.sp, lineHeight = 17.sp)
        }
        Switch(checked = checked, onCheckedChange = null, colors = SwitchDefaults.colors(checkedTrackColor = Sea, checkedThumbColor = OnSea))
    }
}

/**
 * Google Calendar (adreça secreta iCal), Gmail (contrasenya d'aplicació) i el
 * resum del matí d'aquest compte. Els vents només llegeixen: mai no envien.
 */
@Composable
private fun ConnectionsCard(vm: GregalViewModel, lang: String, page: PrefsPage) {
    val c by vm.connections.collectAsState()
    val msg by vm.connectionsMsg.collectAsState()
    val winds by vm.winds.collectAsState()
    val uris = androidx.compose.ui.platform.LocalUriHandler.current
    LaunchedEffect(Unit) { vm.loadConnections() }
    val cs = c
    if (cs == null) {
        Text(msg.ifBlank { if (lang == "en") "Loading connections…" else "Carregant connexions…" }, color = Muted)
        TextButton(onClick = { vm.loadConnections() }) { Text(if (lang == "en") "Retry" else "Torna-ho a provar") }
        return
    }
    var calName by remember { mutableStateOf("") }
    var calUrl by remember { mutableStateOf("") }
    var mailUser by remember(cs.mailUser) { mutableStateOf(cs.mailUser) }
    var mailPass by remember { mutableStateOf("") }
    var briefOn by remember(cs.briefingOn) { mutableStateOf(cs.briefingOn) }
    var briefTime by remember(cs.briefingTime) { mutableStateOf(cs.briefingTime) }
    var briefWind by remember(cs.briefingWind, winds) { mutableStateOf(cs.briefingWind.ifBlank { (winds.firstOrNull { it.specialty == "organisation" } ?: winds.firstOrNull())?.id.orEmpty() }) }
    PrefsGroup(Modifier.fillMaxWidth()) {
        Text(tr(lang, "app.conn.help"), color = Muted, fontSize = 12.5.sp, lineHeight = 17.sp)
        Spacer(Modifier.height(12.dp))
        if (page == PrefsPage.Calendar) {
            Text("Google Calendar", color = Ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
            Text(tr(lang, "app.conn.calHelp"), color = Muted, fontSize = 12.sp, lineHeight = 16.sp,
                modifier = Modifier.clickable { uris.openUri("https://calendar.google.com/calendar/r/settings") })
            cs.calendars.forEach { (name, url) ->
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("$name · $url", color = Ink, fontSize = 12.5.sp, modifier = Modifier.weight(1f))
                    TextButton(onClick = { vm.saveConnections(org.json.JSONObject().put("remove_calendar", name), tr(lang, "app.conn.removed")) }) { Text(tr(lang, "app.conn.remove"), color = Sea) }
                }
            }
            androidx.compose.material3.OutlinedTextField(calName, { calName = it }, label = { Text(tr(lang, "app.conn.calName")) }, singleLine = true, modifier = Modifier.fillMaxWidth())
            androidx.compose.material3.OutlinedTextField(calUrl, { calUrl = it.trim() }, label = { Text(tr(lang, "app.conn.calUrl")) }, singleLine = true, modifier = Modifier.fillMaxWidth())
            androidx.compose.material3.OutlinedButton(onClick = {
                vm.saveConnections(org.json.JSONObject().put("add_calendar", org.json.JSONObject().put("name", calName).put("url", calUrl)), tr(lang, "app.conn.calAdded"))
                calName = ""; calUrl = ""
            }, enabled = calUrl.isNotBlank(), shape = RoundedCornerShape(14.dp)) { Text(tr(lang, "app.conn.add")) }

            Spacer(Modifier.height(12.dp))
        }
        if (page == PrefsPage.Mail) {
            Text("Gmail", color = Ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
            Text(tr(lang, "app.conn.mailHelp"), color = Muted, fontSize = 12.sp, lineHeight = 16.sp,
                modifier = Modifier.clickable { uris.openUri("https://myaccount.google.com/apppasswords") })
            androidx.compose.material3.OutlinedTextField(mailUser, { mailUser = it.trim() }, label = { Text(tr(lang, "app.conn.mailUser")) }, singleLine = true, modifier = Modifier.fillMaxWidth())
            androidx.compose.material3.OutlinedTextField(mailPass, { mailPass = it }, label = { Text(tr(lang, if (cs.mailConnected) "app.conn.mailPassSaved" else "app.conn.mailPass")) },
                singleLine = true, visualTransformation = androidx.compose.ui.text.input.PasswordVisualTransformation(), modifier = Modifier.fillMaxWidth())
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                androidx.compose.material3.OutlinedButton(onClick = {
                    vm.saveConnections(org.json.JSONObject().put("mail", org.json.JSONObject().put("host", "imap.gmail.com").put("port", 993).put("user", mailUser).put("password", mailPass)), tr(lang, "app.conn.mailOk"))
                    mailPass = ""
                }, enabled = mailUser.isNotBlank() && (mailPass.isNotBlank() || cs.mailConnected), shape = RoundedCornerShape(14.dp)) {
                    Text(tr(lang, if (cs.mailConnected) "app.conn.save" else "app.conn.connect"))
                }
                if (cs.mailConnected) TextButton(onClick = { vm.saveConnections(org.json.JSONObject().put("remove_mail", true), tr(lang, "app.conn.mailOff")) }) { Text(tr(lang, "app.conn.disconnect"), color = Sea) }
            }

            Spacer(Modifier.height(12.dp))
        }
        if (page == PrefsPage.Briefing) {
            PreferenceSwitchRowPlain(tr(lang, "app.conn.brief"), tr(lang, "app.conn.briefHelp"), briefOn) { briefOn = it }
            if (briefOn) {
                androidx.compose.material3.OutlinedTextField(briefTime, { briefTime = it.trim() }, label = { Text(tr(lang, "app.conn.briefTime")) }, singleLine = true, modifier = Modifier.fillMaxWidth())
                Text(tr(lang, "app.conn.briefWind"), color = Muted, fontSize = 12.5.sp)
                Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    winds.forEach { w ->
                        androidx.compose.material3.FilterChip(selected = w.id == briefWind, onClick = { briefWind = w.id }, label = { Text(w.name) })
                    }
                }
            }
            androidx.compose.material3.OutlinedButton(onClick = {
                vm.saveConnections(org.json.JSONObject().put("briefing", org.json.JSONObject().put("enabled", briefOn).put("time", briefTime).put("wind", briefWind)),
                    tr(lang, if (briefOn) "app.conn.briefOk" else "app.conn.briefOff"))
            }, enabled = !briefOn || (briefWind.isNotBlank() && Regex("^\\d{2}:\\d{2}$").matches(briefTime)), shape = RoundedCornerShape(14.dp)) { Text(tr(lang, "app.conn.save")) }
        }
        Row(verticalAlignment = Alignment.CenterVertically) {
            TextButton(onClick = { vm.testConnections() }) { Text(tr(lang, "app.conn.test"), color = Sea) }
            if (msg.isNotBlank()) Text(msg, color = Muted, fontSize = 12.5.sp, modifier = Modifier.weight(1f))
        }
    }
}
