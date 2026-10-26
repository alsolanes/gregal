package xyz.solanes.gregal

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.rememberScrollState
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
internal fun programatHora(iso: String, now: java.time.Instant = java.time.Instant.now(), lang: String = "ca"): String {
    if (iso.isBlank()) return ""
    return try {
        val at = Instant.parse(iso)
        val z = at.atZone(ZoneId.systemDefault())
        val today = now.atZone(ZoneId.systemDefault()).toLocalDate()
        val pattern = if (z.toLocalDate() == today) "HH:mm" else "EEE d MMM HH:mm"
        // La data surt en la llengua de l'app, no sempre en català.
        java.time.format.DateTimeFormatter.ofPattern(pattern, java.util.Locale.forLanguageTag(if (lang == "en") "en" else "ca"))
            .format(z)
    } catch (e: Exception) { "" }
}

/** Pròxima mirada d'un cor. Si ja tocava i no s'ha fet (servidor aturat),
 *  ho diu així en lloc de mostrar una hora passada com si fos futura. Pura. */
internal fun programatProperaMirada(iso: String, lang: String, now: java.time.Instant = java.time.Instant.now()): String {
    val hora = programatHora(iso, now, lang)
    if (hora.isEmpty()) return ""
    val vencuda = try { java.time.Instant.parse(iso).isBefore(now) } catch (e: Exception) { false }
    return tr(lang, if (vencuda) "app.programats.overdue" else "app.programats.next", hora)
}

@Composable
internal fun ProgramatsDialog(vm: GregalViewModel, lang: String, onDismiss: () -> Unit) {
    val programats by vm.programats.collectAsState()
    val rutines by vm.rutines.collectAsState()
    var showNovaRutina by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) { vm.loadProgramats(); vm.loadRutines() }
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(shape = MaterialTheme.shapes.large, color = MaterialTheme.colorScheme.background) {
            Column(Modifier.fillMaxWidth().fillMaxHeight(0.85f).padding(top = 16.dp)) {
                // Capçalera: títol i tanca.
                Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(tr(lang, "app.programats.title"), color = Ink, fontWeight = FontWeight.Bold, fontSize = 18.sp, modifier = Modifier.weight(1f))
                    IconButton(onClick = { showNovaRutina = true }) { Text("＋", color = Sea, fontSize = 22.sp) }
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
                                    title = r.windName + " · " + programatHora(r.at, lang = lang),
                                    body = r.text,
                                )
                            }
                        }
                        if (p.hearts.isNotEmpty()) {
                            item { SectionTitle(tr(lang, "app.programats.hearts")) }
                            items(p.hearts.sortedBy { it.nextAt }, key = { "h-" + it.wind }) { h ->
                                ProgramatRow(
                                    title = h.windName,
                                    body = programatProperaMirada(h.nextAt, lang),
                                )
                            }
                        }
                        val rs = rutines
                        if (rs != null && rs.isNotEmpty()) {
                            item { SectionTitle(tr(lang, "app.rutines.title")) }
                            items(rs.sortedBy { it.nextDueAt }, key = { it.id }) { ru ->
                                val quan = if (ru.kind == "daily") ru.time
                                else tr(lang, "app.rutines.each", (if (ru.intervalH >= 1) ru.intervalH.toInt().toString() else ru.intervalH.toString()))
                                ProgramatRow(
                                    title = ru.windName + " · " + quan,
                                    body = ru.prompt,
                                    onDelete = { vm.deleteRutina(ru.id) },
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
    if (showNovaRutina) NovaRutinaDialog(vm, lang) { showNovaRutina = false }
}

/** Diàleg «Nova rutina»: tria vent, quan corre i què vigila. La rutina obre
 * un torn real del vent (LLM + eines), no un text preescrit. */
@Composable
private fun NovaRutinaDialog(vm: GregalViewModel, lang: String, onDismiss: () -> Unit) {
    val winds by vm.winds.collectAsState()
    val busy by vm.rutinaBusy.collectAsState()
    var windId by remember { mutableStateOf(winds.firstOrNull()?.id ?: "") }
    var kind by remember { mutableStateOf("daily") }
    var hora by remember { mutableStateOf("09:00") }
    var hores by remember { mutableStateOf("6") }
    var prompt by remember { mutableStateOf("") }
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(shape = MaterialTheme.shapes.large, color = MaterialTheme.colorScheme.background) {
            Column(Modifier.fillMaxWidth().padding(24.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(tr(lang, "app.rutines.new"), color = Ink, fontWeight = FontWeight.Bold, fontSize = 18.sp)
                Text(tr(lang, "app.rutines.newHelp"), color = Muted, fontSize = 13.sp)
                // Vent
                Text(tr(lang, "app.rutines.wind"), color = Muted, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
                Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    winds.forEach { w ->
                        FilterChip(selected = w.id == windId, onClick = { windId = w.id }, label = { Text(w.name) })
                    }
                }
                // Quan
                Text(tr(lang, "app.rutines.when"), color = Muted, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    FilterChip(selected = kind == "daily", onClick = { kind = "daily" }, label = { Text(tr(lang, "app.rutines.daily")) })
                    FilterChip(selected = kind == "interval", onClick = { kind = "interval" }, label = { Text(tr(lang, "app.rutines.interval")) })
                }
                if (kind == "daily") {
                    OutlinedTextField(value = hora, onValueChange = { hora = it }, label = { Text(tr(lang, "app.rutines.timeLabel")) }, singleLine = true)
                } else {
                    OutlinedTextField(value = hores, onValueChange = { hores = it }, label = { Text(tr(lang, "app.rutines.hoursLabel")) }, singleLine = true)
                }
                // Què ha de fer
                OutlinedTextField(value = prompt, onValueChange = { prompt = it }, label = { Text(tr(lang, "app.rutines.promptLabel")) },
                    minLines = 3, modifier = Modifier.fillMaxWidth())
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End, verticalAlignment = Alignment.CenterVertically) {
                    if (busy) CircularProgressIndicator(color = Sea, modifier = Modifier.size(18.dp))
                    Spacer(Modifier.width(10.dp))
                    OutlinedButton(onClick = onDismiss) { Text(tr(lang, "app.rutines.cancel")) }
                    Spacer(Modifier.width(8.dp))
                    Button(
                        enabled = !busy && windId.isNotBlank() && prompt.isNotBlank(),
                        onClick = {
                            vm.createRutina(windId, kind, hora, hores.toDoubleOrNull() ?: 0.0, prompt, "") { ok -> if (ok) onDismiss() }
                        },
                    ) { Text(tr(lang, "app.rutines.create")) }
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
private fun ProgramatRow(title: String, body: String, onDelete: (() -> Unit)? = null) {
    Surface(shape = MaterialTheme.shapes.medium, color = SuperficieAlta.copy(alpha = .45f)) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 14.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Rounded.Schedule, null, tint = Sea, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(10.dp))
            Column(Modifier.weight(1f)) {
                Text(title, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp)
                Text(body, color = Muted, fontSize = 13.sp)
            }
            // Esborrar és per a les rutines pròpies; recordatoris i cors no es
            // toquen des d'aquí (el cor el governa el mateix vent).
            if (onDelete != null) {
                IconButton(onClick = onDelete) { Text("🗑", color = Muted, fontSize = 15.sp) }
            }
        }
    }
}
