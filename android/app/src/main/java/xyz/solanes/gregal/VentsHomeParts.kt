package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog

// Peces de la portada dels Vents: abans la llista quedava sota el selector
// mòbil/servidor, la seva explicació, la tira de personatges i tres accessos.

/** On treballen els vents, en petit a la capçalera: toca'l i surt el selector. */
@Composable
internal fun WhereChip(vm: GregalViewModel, lang: String, onClick: () -> Unit) {
    val conn by vm.conn.collectAsState()
    val reach by vm.serverReach.collectAsState()
    val switching by vm.switchingWhere.collectAsState()
    val onServer = !conn.local
    LaunchedEffect(onServer) {
        while (true) { vm.checkServerReach(); kotlinx.coroutines.delay(60_000) }
    }
    val label = if (onServer) vm.savedServerName().ifBlank { tr(lang, "app.where.server") } else tr(lang, "app.where.phoneShort")
    val dot = when {
        switching -> Sand
        !onServer -> Muted
        reach == true -> Color(0xFF6FCF97)
        reach == false -> Color(0xFFE57373)
        else -> Muted
    }
    Row(
        Modifier.padding(end = 2.dp).clip(RoundedCornerShape(16.dp)).background(SuperficieAlta.copy(alpha = .55f))
            .clickable(onClick = onClick).padding(horizontal = 10.dp, vertical = 6.dp).widthIn(max = 120.dp)
            .semantics { contentDescription = tr(lang, "app.where.title") + ": " + label }.testTag("where-chip"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Box(Modifier.size(8.dp).clip(CircleShape).background(dot))
        Text(label, color = Ink, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

/** El selector de sempre (mòbil o servidor, amb l'explicació), dins un diàleg. */
@Composable
internal fun WhereDialog(vm: GregalViewModel, lang: String, onClose: () -> Unit) {
    Dialog(onDismissRequest = onClose) {
        Surface(shape = RoundedCornerShape(24.dp), color = Superficie) {
            Column(Modifier.padding(vertical = 18.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                Text(tr(lang, "app.where.title"), color = Ink, fontSize = 18.sp, fontWeight = FontWeight.Bold,
                    modifier = Modifier.padding(horizontal = 22.dp))
                WhereSwitch(vm, lang)
                Row(Modifier.fillMaxWidth().padding(horizontal = 12.dp), horizontalArrangement = Arrangement.End) {
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
        Modifier.fillMaxWidth().clickable(onClick = onClick).padding(horizontal = 20.dp, vertical = 10.dp).testTag("running-work"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        CircularProgressIndicator(Modifier.size(16.dp), color = Sea, strokeWidth = 2.dp)
        Text(tr(lang, "app.feines.runningLine", running), color = Ink, fontSize = 14.sp, modifier = Modifier.weight(1f))
        Text("›", color = Muted, fontSize = 18.sp)
    }
}

/** Una fila del directori: el vent, el seu ofici i «Parla-hi». */
@Composable
internal fun DirectoryRow(lang: String, w: Wind, onProfile: () -> Unit, onTalk: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable(onClick = onProfile).padding(horizontal = 16.dp, vertical = 10.dp).testTag("dir-${w.id}"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        WindHalo(Color(w.argb), w.faceStyle, size = 44, online = false, showKit = true)
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(w.name, color = Ink, fontSize = 16.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (w.role.isNotBlank()) Text(w.role, color = Muted, fontSize = 14.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
        }
        OutlinedButton(onClick = onTalk, contentPadding = PaddingValues(horizontal = 14.dp, vertical = 6.dp)) {
            Text(tr(lang, "app.vents.talk"), color = Ink, fontSize = 14.sp)
        }
    }
}
