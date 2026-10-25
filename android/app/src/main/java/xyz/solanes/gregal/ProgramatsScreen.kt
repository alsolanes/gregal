package xyz.solanes.gregal

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Schedule
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import java.time.Instant
import java.time.ZoneId

/**
 * Programats — la vista dels avisos proactius dels vents: què diran i quan
 * (recordatoris), quin vent vigila què i quan tornarà a mirar (cors), i les
 * estadístiques de cada vent: quants batecs, quants han acabat en missatge i
 * quants en silenci (res que valgués la pena). Totes les dades venen del
 * backend (/api/winds/programats); res inventat.
 */

/** La fila «Programats» de la llista de vents: discreta, una sola. */
@Composable
internal fun ProgramatsButton(lang: String, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        shape = MaterialTheme.shapes.medium,
        color = SuperficieAlta.copy(alpha = .30f),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 6.dp),
    ) {
        Row(Modifier.padding(horizontal = 14.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Rounded.Schedule, null, tint = Sea, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(10.dp))
            Text(tr(lang, "app.programats.open"), color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp, modifier = Modifier.weight(1f))
            Text("›", color = Muted, fontSize = 18.sp)
        }
    }
}

/** «dimarts 04:15» — el dia només si no és avui. */
internal fun programatHora(iso: String, now: java.time.Instant = java.time.Instant.now()): String {
    if (iso.isBlank()) return ""
    return try {
        val at = Instant.parse(iso)
        val z = at.atZone(ZoneId.systemDefault())
        val today = now.atZone(ZoneId.systemDefault()).toLocalDate()
        val pattern = if (z.toLocalDate() == today) "HH:mm" else "EEE d MMM HH:mm"
        java.time.format.DateTimeFormatter.ofPattern(pattern, java.util.Locale.forLanguageTag("ca"))
            .format(z)
    } catch (e: Exception) { "" }
}

@Composable
internal fun ProgramatsDialog(vm: GregalViewModel, lang: String, onDismiss: () -> Unit) {
    val programats by vm.programats.collectAsState()
    LaunchedEffect(Unit) { vm.loadProgramats() }
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(shape = MaterialTheme.shapes.large, color = MaterialTheme.colorScheme.background) {
            Column(Modifier.fillMaxWidth().fillMaxHeight(0.85f).padding(top = 16.dp)) {
                // Capçalera: títol i tanca.
                Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(tr(lang, "app.programats.title"), color = Ink, fontWeight = FontWeight.Bold, fontSize = 18.sp, modifier = Modifier.weight(1f))
                    IconButton(onClick = onDismiss) { Text("✕", color = Muted, fontSize = 16.sp) }
                }
                val p = programats
                val perr by vm.programatsError.collectAsState()
                when {
                    p == null -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        CircularProgressIndicator(color = Sea)
                    }
                    perr != null -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(10.dp), modifier = Modifier.padding(32.dp)) {
                            Text("🌤", fontSize = 40.sp)
                            Text(tr(lang, "app.programats.loadFail"), color = MaterialTheme.colorScheme.error, fontWeight = FontWeight.SemiBold)
                            Text(perr ?: "", color = Muted, fontSize = 13.sp)
                            OutlinedButton(onClick = { vm.loadProgramats() }) { Text(tr(lang, "app.jobs.retry")) }
                        }
                    }
                    p.reminders.isEmpty() && p.hearts.isEmpty() && p.stats.isEmpty() -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp), modifier = Modifier.padding(32.dp)) {
                            Text("🌤", fontSize = 40.sp)
                            Text(tr(lang, "app.programats.empty"), color = Ink, fontWeight = FontWeight.SemiBold)
                            Text(tr(lang, "app.programats.emptyHelp"), color = Muted, fontSize = 13.sp)
                        }
                    }
                    else -> LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(start = 20.dp, end = 20.dp, bottom = 24.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        if (p.reminders.isNotEmpty()) {
                            item { SectionTitle(tr(lang, "app.programats.reminders")) }
                            items(p.reminders.sortedBy { it.at }, key = { it.id }) { r ->
                                ProgramatRow(
                                    title = r.windName + " · " + programatHora(r.at),
                                    body = r.text,
                                )
                            }
                        }
                        if (p.hearts.isNotEmpty()) {
                            item { SectionTitle(tr(lang, "app.programats.hearts")) }
                            items(p.hearts.sortedBy { it.nextAt }, key = { "h-" + it.wind }) { h ->
                                ProgramatRow(
                                    title = h.windName,
                                    body = tr(lang, "app.programats.next", programatHora(h.nextAt)),
                                )
                            }
                        }
                        if (p.stats.isNotEmpty()) {
                            item { SectionTitle(tr(lang, "app.programats.stats")) }
                            items(p.stats.sortedByDescending { it.beats }, key = { "s-" + it.windName }) { st ->
                                val ratio = if (st.beats > 0) st.spoke * 100 / st.beats else 0
                                ProgramatRow(
                                    title = st.windName,
                                    body = tr(lang, "app.programats.statLine", st.beats.toString(), st.spoke.toString(), st.silent.toString(), ratio.toString()),
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun SectionTitle(text: String) {
    Text(text, color = Muted, fontSize = 12.sp, fontWeight = FontWeight.SemiBold,
        letterSpacing = 1.sp, modifier = Modifier.padding(top = 8.dp))
}

@Composable
private fun ProgramatRow(title: String, body: String) {
    Surface(shape = MaterialTheme.shapes.medium, color = SuperficieAlta.copy(alpha = .45f)) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 14.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Rounded.Schedule, null, tint = Sea, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(10.dp))
            Column(Modifier.weight(1f)) {
                Text(title, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp)
                Text(body, color = Muted, fontSize = 13.sp)
            }
        }
    }
}
