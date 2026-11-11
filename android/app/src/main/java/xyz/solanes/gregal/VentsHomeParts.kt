package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.ChevronRight
import androidx.compose.material.icons.rounded.Groups
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog

// Peces de la portada dels Vents: abans la llista quedava sota el selector
// mòbil/servidor, la seva explicació, la tira de personatges i tres accessos.

/**
 * On treballen els vents, en petit a la capçalera. Amb servidor és un estat,
 * no un selector: el nom del servidor i si respon; sense connexió, «còpia»
 * i quants missatges esperen. Tocar-lo obre el detall (i, a part, el canvi).
 */
@Composable
internal fun WhereChip(vm: GregalViewModel, lang: String, onClick: () -> Unit) {
    val conn by vm.conn.collectAsState()
    val reach by vm.serverReach.collectAsState()
    val switching by vm.switchingWhere.collectAsState()
    val offline by vm.offline.collectAsState()
    val outbox by vm.outbox.collectAsState()
    val onServer = !conn.local
    // La provabilitat del servidor només es mira mentre es veu la portada.
    LifecyclePoll(onServer) { while (true) { vm.checkServerReach(); kotlinx.coroutines.delay(60_000) } }
    val server = vm.savedServerName().ifBlank { tr(lang, "app.where.server") }
    val label = when {
        !onServer -> tr(lang, "app.where.phoneShort")
        offline && outbox.isNotEmpty() -> tr(lang, "app.offline.chipQueued", outbox.size)
        offline -> tr(lang, "app.offline.chip")
        else -> server
    }
    val dot = when {
        !onServer && !switching -> Muted
        offline && !switching -> Sand
        else -> reachDot(reach, switching)
    }
    // 40 dp d'alt (abans 32): és un control, no una etiqueta.
    Row(
        Modifier.padding(end = 2.dp).heightIn(min = 40.dp).clip(GregalShape.pill).background(SuperficieAlta.copy(alpha = .55f))
            .clickable(onClick = onClick).padding(horizontal = 12.dp, vertical = 6.dp).widthIn(max = 150.dp)
            .semantics { contentDescription = tr(lang, "app.where.title") + ": " + label }.testTag("where-chip"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Box(Modifier.size(8.dp).clip(CircleShape).background(dot))
        Text(label, color = Ink, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

/** Hora de la còpia en paraules curtes («avui 14:32»). */
internal fun offlineCopyWhen(lang: String, ms: Long): String =
    if (ms <= 0L) "" else automationWhen(lang, java.time.Instant.ofEpochMilli(ms).atZone(java.time.ZoneId.systemDefault()).toOffsetDateTime().toString())

/** Estat del servidor: si respon, o que veus la còpia del mòbil i què espera a sortir. */
@Composable
internal fun WhereStatus(vm: GregalViewModel, lang: String) {
    val offline by vm.offline.collectAsState()
    val outbox by vm.outbox.collectAsState()
    val copyAt by vm.offlineCopyAt.collectAsState()
    val server = vm.savedServerName().ifBlank { tr(lang, "app.where.server") }
    Column(Modifier.fillMaxWidth().padding(horizontal = 22.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Box(Modifier.size(9.dp).clip(CircleShape).background(if (offline) Sand else Green))
            Text(server, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(tr(lang, if (offline) "app.offline.state" else "app.offline.online"), color = Muted, fontSize = 13.sp)
        }
        if (offline) {
            val whenText = offlineCopyWhen(lang, copyAt)
            Text(if (whenText.isBlank()) tr(lang, "app.offline.copy") else tr(lang, "app.offline.copyAt", whenText),
                color = Muted, fontSize = 13.sp, lineHeight = 18.sp)
            if (outbox.isNotEmpty()) Text(tr(lang, "app.offline.queued", outbox.size), color = Ink, fontSize = 13.sp)
            TextButton(onClick = { vm.retryServerNow() }, contentPadding = PaddingValues(horizontal = 0.dp)) {
                Text(tr(lang, "app.offline.retry"), color = Sea, fontWeight = FontWeight.SemiBold)
            }
        } else {
            Text(tr(lang, "app.where.serverHint"), color = Muted, fontSize = 13.sp, lineHeight = 18.sp)
        }
    }
}

/**
 * El diàleg de la xapa. Amb servidor, primer l'estat; canviar al mòbil
 * queda a sota, plegat: és rar i no s'hi ha de caure per error.
 */
@Composable
internal fun WhereDialog(vm: GregalViewModel, lang: String, onClose: () -> Unit) {
    val conn by vm.conn.collectAsState()
    var showSwitch by remember { mutableStateOf(conn.local) }
    Dialog(onDismissRequest = onClose) {
        Surface(shape = RoundedCornerShape(24.dp), color = Superficie) {
            Column(Modifier.padding(vertical = 18.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                Text(tr(lang, "app.where.title"), color = Ink, fontSize = 18.sp, fontWeight = FontWeight.Bold,
                    modifier = Modifier.padding(horizontal = 22.dp))
                if (!conn.local) WhereStatus(vm, lang)
                if (showSwitch) WhereSwitch(vm, lang)
                Row(Modifier.fillMaxWidth().padding(horizontal = 12.dp), horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically) {
                    if (!showSwitch) TextButton(onClick = { showSwitch = true }) {
                        Text(tr(lang, "app.where.change"), color = Muted, fontSize = 13.sp)
                    } else Spacer(Modifier.width(1.dp))
                    TextButton(onClick = onClose) { Text(tr(lang, "app.cap.close"), color = Sea) }
                }
            }
        }
    }
}

/** El Ventet, fixat a dalt de les converses: l'últim que ha dit i si té novetats. */
@Composable
internal fun VentetRow(lang: String, ui: GregalViewModel.CapUi, summary: ConvoSummary?, onClick: () -> Unit) {
    val st = ui.state
    val last = st.history.lastOrNull { it.text.isNotBlank() }
    val preview = when {
        ui.busy -> tr(lang, "app.cap.thinking")
        ui.pending != null -> ui.pending.decision.reply
        st.briefCount > 0 && st.brief.isNotBlank() -> st.brief
        last != null -> (if (last.from == "user") tr(lang, "app.vents.you") + ": " else "") + last.text
        summary != null && summary.text.isNotBlank() -> summary.text
        else -> tr(lang, "app.cap.role")
    }
    val ts = last?.ts?.let { msgMillis(it) }?.takeIf { it > 0 } ?: summary?.ts ?: 0L
    Box(Modifier.testTag("ventet-row")) {
        ConversationRow(st.name, stripMdInline(preview).replace(Regex("\\s+"), " ").trim(), Color(st.argb), st.faceStyle,
            busy = ui.busy,
            mood = when {
                ui.busy -> WindMood.THINKING
                ui.pending != null || st.briefCount > 0 -> WindMood.NEWS
                else -> null
            },
            timestamp = ts, onClick = onClick, lang = lang)
    }
}

/** Feina en marxa: una sola línia, i només quan n'hi ha. */
@Composable
internal fun RunningWorkLine(lang: String, running: Int, onClick: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable(
            onClick = onClick,
            onClickLabel = tr(lang, "app.vents.openWork"),
        ).padding(horizontal = 20.dp, vertical = 10.dp)
            .semantics {
                role = Role.Button
                contentDescription = tr(lang, "app.feines.runningLine", running)
            }.testTag("running-work"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        CircularProgressIndicator(Modifier.size(16.dp), color = Sea, strokeWidth = 2.dp)
        Text(tr(lang, "app.feines.runningLine", running), color = Ink, fontSize = 14.sp, modifier = Modifier.weight(1f))
        Icon(Icons.Rounded.ChevronRight, contentDescription = null, tint = Muted, modifier = Modifier.size(20.dp))
    }
}

/** Una fila del directori: el vent, el seu ofici i «Parla-hi». */
@Composable
internal fun DirectoryRow(lang: String, w: Wind, onProfile: () -> Unit, onTalk: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable(
            onClick = onProfile,
            onClickLabel = tr(lang, "app.vents.openProfile"),
        ).padding(horizontal = 16.dp, vertical = 10.dp)
            .semantics {
                role = Role.Button
                contentDescription = "${w.name}: ${w.role}"
            }.testTag("dir-${w.id}"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        WindHalo(Color(w.argb), w.faceStyle, size = 44, online = false, showKit = true)
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(w.name, color = Ink, fontSize = 16.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (w.role.isNotBlank()) Text(w.role, color = Muted, fontSize = 14.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
        }
        OutlinedButton(
            onClick = onTalk,
            contentPadding = PaddingValues(horizontal = 14.dp, vertical = 6.dp),
            modifier = Modifier.semantics { contentDescription = tr(lang, "app.vents.talkToNamed", w.name) },
        ) {
            Text(tr(lang, "app.vents.talk"), color = Ink, fontSize = 14.sp)
        }
    }
}

/**
 * Els teus vents, a dalt de la portada: com les apps de companys, la cara és
 * el que reconeixes. Un toc obre la conversa amb aquell vent; qui té novetats
 * ho diu amb la cara i el nom destacat. «Tots» porta al directori.
 */
@Composable
internal fun HomeFacesRow(
    lang: String, winds: List<Wind>, news: Set<String>, busy: Set<String>,
    onTalk: (Wind) -> Unit, onAll: () -> Unit,
) {
    if (winds.isEmpty()) return
    androidx.compose.foundation.lazy.LazyRow(
        Modifier.fillMaxWidth().padding(top = 6.dp).testTag("home-faces"),
        contentPadding = PaddingValues(horizontal = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        items(winds.size, key = { winds[it].id }) { i ->
            val w = winds[i]
            val fresh = w.id in news
            Column(
                Modifier.width(72.dp).clip(RoundedCornerShape(18.dp)).clickable { onTalk(w) }
                    .semantics(mergeDescendants = true) { role = Role.Button; contentDescription = w.name }
                    .padding(vertical = 6.dp).testTag("home-face-${w.id}"),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                WindHalo(Color(w.argb), w.faceStyle, size = 46,
                    mood = when { w.id in busy -> WindMood.THINKING; fresh -> WindMood.NEWS; else -> WindMood.IDLE },
                    online = false)
                Text(w.name, color = if (fresh) Ink else Muted, fontSize = 12.sp,
                    fontWeight = if (fresh) FontWeight.SemiBold else FontWeight.Medium,
                    maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        item(key = "all") {
            Column(
                Modifier.width(72.dp).clip(RoundedCornerShape(18.dp)).clickable(onClick = onAll)
                    .semantics(mergeDescendants = true) { role = Role.Button }
                    .padding(vertical = 6.dp).testTag("home-faces-all"),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                Box(Modifier.size(60.dp), contentAlignment = Alignment.Center) {
                    Box(Modifier.size(46.dp).clip(CircleShape).background(SuperficieAlta), contentAlignment = Alignment.Center) {
                        Icon(Icons.Rounded.Groups, null, tint = Ink, modifier = Modifier.size(22.dp))
                    }
                }
                Text(tr(lang, "app.vents.homeAll"), color = Muted, fontSize = 12.sp, fontWeight = FontWeight.Medium, maxLines = 1)
            }
        }
    }
}

/** El vent escriu: la seva cara petita i els punts, a l'esquerra com un missatge seu. */
@Composable
internal fun WindTypingBubble(lang: String, wind: Wind, accent: Color, mood: WindMood,
                                /** Què està fent ara («Cercant al web…»). Aquí i enlloc més. */
                                label: String? = null) {
    Row(
        Modifier.padding(top = 12.dp, bottom = 4.dp)
            .semantics(mergeDescendants = true) { contentDescription = tr(lang, "app.company.windThinking", wind.name) }
            .testTag("wind-typing"),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        WindFace(size = 30, mood = mood, accent = accent, style = wind.faceStyle)
        Box(
            Modifier.clip(RoundedCornerShape(18.dp)).background(accent.copy(alpha = .16f))
                .padding(horizontal = 14.dp, vertical = 12.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                TypingDots(accent)
                // L'acció en marxa, al costat dels punts: un sol lloc per
                // «què està fent» (la capçalera i la franja ja no ho diuen).
                if (!label.isNullOrBlank()) Text(label, color = accent.copy(alpha = .85f), style = GregalType.caption, maxLines = 1)
            }
        }
    }
}
