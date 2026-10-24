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
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.Logout
import androidx.compose.material.icons.automirrored.rounded.Undo
import androidx.compose.material.icons.rounded.AccountTree
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.ChatBubbleOutline
import androidx.compose.material.icons.rounded.Code
import androidx.compose.material.icons.rounded.Construction
import androidx.compose.material.icons.rounded.Description
import androidx.compose.material.icons.rounded.Flag
import androidx.compose.material.icons.rounded.Forum
import androidx.compose.material.icons.rounded.Schedule
import androidx.compose.material.icons.rounded.Settings
import androidx.compose.material.icons.rounded.Tag
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
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
            Box(Modifier.padding(horizontal = 6.dp)) { ProjectPill(project.ifEmpty { tr(lang, "app.nav.project") }, branch) }
            Spacer(Modifier.height(16.dp))
            PrimaryPill(tr(lang, "nav.new"), { vm.newSession(); nav(Screen.Chat) }, Modifier.fillMaxWidth(), icon = Icons.Rounded.Add)

            DrawerSection(tr(lang, "app.nav.sectionWinds"))
            DrawerRow(tr(lang, "nav.company"), screen == Screen.Company, { nav(Screen.Company) }) { DrawerWindIcon(vm) }

            DrawerSection(tr(lang, "app.nav.sectionAgent"))
            DrawerRow(tr(lang, "app.nav.chat"), screen == Screen.Chat, { nav(Screen.Chat) }, Icons.Rounded.ChatBubbleOutline)
            DrawerRow(tr(lang, "app.work.title"), screen == Screen.Work, { nav(Screen.Work) }, Icons.Rounded.Construction)
            DrawerRow(tr(lang, "nav.convs"), screen == Screen.Sessions, { nav(Screen.Sessions); vm.loadSessions() }, Icons.Rounded.Forum)
            DrawerRow(tr(lang, "nav.goals"), screen == Screen.Goals, { nav(Screen.Goals); vm.loadGoals() }, Icons.Rounded.Flag)

            DrawerSection(tr(lang, "app.nav.sectionAutomate"))
            DrawerRow(tr(lang, "nav.activity"), screen == Screen.Jobs, { nav(Screen.Jobs); vm.loadJobs(); vm.loadRuns() }, Icons.Rounded.Schedule)
            DrawerRow(tr(lang, "graphs.title"), screen == Screen.Flows, { nav(Screen.Flows); vm.loadFlows() }, Icons.Rounded.AccountTree)

            DrawerSection(tr(lang, "app.nav.sectionTools"))
            DrawerRow(tr(lang, "nav.office"), screen == Screen.Office, { nav(Screen.Office) }, Icons.Rounded.Description)
            DrawerRow(tr(lang, "app.retro.title"), screen == Screen.Retro, { close(); vm.openRetro() }, Icons.Rounded.Tag)
            // Al dispositiu no hi ha la CLI gh: GitHub només en remot.
            if (!local) DrawerRow(tr(lang, "app.nav.github"), false, { close(); vm.obreGithub() }, Icons.Rounded.Code)
            DrawerRow(tr(lang, "app.nav.undoTurn"), false, { close(); vm.openRewind() }, Icons.AutoMirrored.Rounded.Undo)

            HorizontalDivider(Modifier.padding(vertical = 12.dp, horizontal = 8.dp), color = Line)
            DrawerRow(tr(lang, "prefs.title"), screen == Screen.Prefs, { nav(Screen.Prefs) }, Icons.Rounded.Settings)
            DrawerRow(tr(lang, "app.server.switch"), false, { close(); vm.switchServer() }, Icons.AutoMirrored.Rounded.Logout, muted = true)
        }
    }
}

@Composable
private fun DrawerSection(label: String) {
    Text(
        label.uppercase(), fontSize = 11.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 1.4.sp, color = Faint,
        modifier = Modifier.padding(start = 14.dp, top = 20.dp, bottom = 6.dp),
    )
}

@Composable
private fun DrawerRow(label: String, selected: Boolean, onClick: () -> Unit, icon: ImageVector, muted: Boolean = false) =
    DrawerRow(label, selected, onClick, muted) { Icon(icon, null, modifier = Modifier.size(22.dp)) }

/** Una entrada del calaix: compacta, amb la seleccionada en píndola tintada i un punt. */
@Composable
private fun DrawerRow(label: String, selected: Boolean, onClick: () -> Unit, muted: Boolean = false, icon: @Composable () -> Unit) {
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
        if (selected) Box(Modifier.size(6.dp).background(Sea, CircleShape))
    }
}
