package xyz.solanes.gregal

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.DateRange
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material.icons.filled.Menu
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog

/** Etiqueta humana de la programació (pur, testeable). */
fun kindLabel(kind: String, time: String, intervalH: Double, lang: String = "ca"): String = when (kind) {
    "daily" -> tr(lang, "app.jobs.everyDayLabel", time.ifEmpty { "--:--" })
    "interval" -> tr(lang, "app.jobs.everyIntervalLabel", fmtHours(intervalH))
    else -> tr(lang, "app.jobs.manual")
}

fun fmtHours(h: Double): String {
    if (h <= 0) return "—"
    return if (h == h.toInt().toDouble()) "${h.toInt()} h" else "$h h"
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun JobsScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val jobs by vm.jobs.collectAsState()
    val runs by vm.runs.collectAsState()
    val haptic = LocalHapticFeedback.current
    var editing by remember { mutableStateOf<JobItem?>(null) }
    var viewing by remember { mutableStateOf<RunItem?>(null) }
    LaunchedEffect(Unit) { vm.loadJobs(); vm.loadRuns() }
    GregalBg {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = { TopAppBar(navigationIcon = { IconButton(onClick = onMenu) { Icon(Icons.Default.Menu, tr(lang, "app.nav.menu"), tint = Ink) } }, title = { Text(tr(lang, "app.jobs.title"), color = Ink) },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent)) },
            floatingActionButton = {
                FloatingActionButton(onClick = {
                    haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                    editing = JobItem()
                }, containerColor = Sea, contentColor = Color(0xFF0C2229)) {
                    Icon(Icons.Default.Add, tr(lang, "app.jobs.add"))
                }
            },
        ) { pad ->
            LazyColumn(Modifier.fillMaxSize().padding(pad).padding(12.dp)) {
                if (jobs.isEmpty() && runs.isEmpty()) item {
                    EmptyBoard("📰", tr(lang, "app.jobs.empty"), tr(lang, "app.jobs.emptyHelp"))
                }
                if (jobs.isNotEmpty()) item {
                    Text(tr(lang, "app.jobs.scheduled"), color = Muted, fontSize = 13.sp,
                        modifier = Modifier.padding(horizontal = 4.dp, vertical = 4.dp))
                }
                items(jobs) { j -> JobCard(j, vm, onEdit = { editing = it }, lang = lang) }
                if (runs.isNotEmpty()) item {
                    Text(tr(lang, "app.jobs.recentRuns"), color = Muted, fontSize = 13.sp,
                        modifier = Modifier.padding(horizontal = 4.dp, vertical = 4.dp))
                }
                items(runs) { r ->
                    RunCard(r, onOpen = { viewing = r }, lang = lang)
                }
            }
        }
    }
    editing?.let { e ->
        JobEditor(e, onDismiss = { editing = null }, onSave = {
            editing = null
            vm.saveJob(it)
        }, lang = lang)
    }
    viewing?.let { r ->
        RunDetail(r, onDismiss = { viewing = null; vm.loadRuns() }, lang = lang)
    }
}

@Composable
fun JobCard(j: JobItem, vm: GregalViewModel, onEdit: (JobItem) -> Unit, lang: String = "ca") {
    val haptic = LocalHapticFeedback.current
    Card(
        modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp),
        colors = CardDefaults.cardColors(containerColor = PillDark),
    ) {
        Row(Modifier.padding(14.dp), verticalAlignment = Alignment.CenterVertically) {
            Surface(shape = RoundedCornerShape(12.dp), color = CodeDark,
                modifier = Modifier.size(38.dp)) {
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    Icon(Icons.Default.DateRange, null, tint = Sea,
                        modifier = Modifier.size(20.dp))
                }
            }
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text(stripMdForTitle(j.name.ifEmpty { tr(lang, "app.jobs.unnamed") }),
                    fontWeight = FontWeight.SemiBold, fontSize = 16.sp, color = Ink)
                Text(kindLabel(j.kind, j.time, j.intervalH, lang) +
                    (if (j.nextDue.isNotEmpty()) " · ${tr(lang, "app.jobs.next", j.nextDue)}" else "") +
                    (if (j.lastStatus.isNotEmpty()) " · ${tr(lang, "app.jobs.last", j.lastStatus)}" else ""),
                    style = androidx.compose.material3.MaterialTheme.typography.bodySmall, color = Muted)
                Row(verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                    FilledTonalButton(onClick = {
                        haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                        vm.runJobNow(j.id)
                    }) {
                        Icon(Icons.Default.PlayArrow, tr(lang, "app.jobs.runNow"))
                        Spacer(Modifier.width(4.dp))
                        Text(tr(lang, "app.jobs.runNow"))
                    }
                    var menu by remember { mutableStateOf(false) }
                    Box {
                        IconButton(onClick = { menu = true }) {
                            Icon(Icons.Default.MoreVert, tr(lang, "app.jobs.more"), tint = Muted)
                        }
                        DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                            DropdownMenuItem(
                                text = { Text(tr(lang, "app.jobs.edit"), color = Ink) },
                                leadingIcon = { Icon(Icons.Default.Edit, null, tint = Muted) },
                                onClick = { menu = false; onEdit(j) },
                            )
                            DropdownMenuItem(
                                text = { Text(tr(lang, "btn.delete"), color = Ink) },
                                leadingIcon = { Icon(Icons.Default.Delete, null, tint = Muted) },
                                onClick = { menu = false; vm.deleteJob(j.id) },
                            )
                        }
                    }
                }
            }
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Switch(checked = j.enabled, onCheckedChange = { vm.toggleJob(j) })
                Text(tr(lang, if (j.enabled) "app.jobs.active" else "app.jobs.paused"), color = Muted, fontSize = 10.sp)
            }
        }
    }
}

@Composable
fun RunCard(r: RunItem, onOpen: () -> Unit, lang: String = "ca") {
    Card(
        onClick = onOpen,
        modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp),
        colors = CardDefaults.cardColors(containerColor = PillDark),
    ) {
        Row(Modifier.padding(14.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(stripMdForTitle(r.jobName.ifEmpty { tr(lang, "app.jobs.emptyJobName") }),
                    fontWeight = FontWeight.SemiBold, fontSize = 16.sp, color = Ink)
                Text(shortStamp(r.startedAt),
                    style = androidx.compose.material3.MaterialTheme.typography.bodySmall, color = Muted)
                if (r.status == "ok" && r.output.isNotBlank()) {
                    Text(stripMdForTitle(r.output).take(120),
                        color = Muted, fontSize = 13.sp,
                        modifier = Modifier.padding(top = 4.dp))
                }
                if (r.status != "ok" && r.error.isNotBlank()) {
                    Text(r.error.take(120), color = Muted, fontSize = 13.sp,
                        modifier = Modifier.padding(top = 4.dp))
                }
            }
            Spacer(Modifier.width(12.dp))
            Surface(shape = RoundedCornerShape(8.dp), color = CodeDark) {
                Text(
                    tr(lang, if (r.status == "ok") "app.jobs.done" else "app.jobs.error"),
                    color = if (r.status == "ok") Sea else Color(0xFFE08A8A),
                    fontSize = 10.sp, fontWeight = FontWeight.Bold,
                    modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
                )
            }
        }
    }
}

/** Editor de job: nom, ordre, tipus, hora/interval i avís. */
@Composable
fun JobEditor(base: JobItem, onDismiss: () -> Unit, onSave: (JobItem) -> Unit, lang: String = "ca") {
    var name by remember { mutableStateOf(base.name) }
    var prompt by remember { mutableStateOf(base.prompt) }
    var kind by remember { mutableStateOf(base.kind.ifEmpty { "daily" }) }
    var time by remember { mutableStateOf(base.time.ifEmpty { "08:00" }) }
    var interval by remember { mutableStateOf(if (base.intervalH > 0) trimHours(base.intervalH) else "24") }
    var notify by remember { mutableStateOf(base.notify) }
    val valid = name.isNotBlank() && prompt.isNotBlank() &&
        (kind != "daily" || time.matches(Regex("\\d{1,2}:\\d{2}"))) &&
        (kind != "interval" || interval.toDoubleOrNull()?.let { it > 0 } == true)
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(tr(lang, if (base.id.isEmpty()) "app.jobs.newTitle" else "app.jobs.editTitle")) },
        text = {
            Column(Modifier.verticalScroll(rememberScrollState())) {
                OutlinedTextField(name, { name = it }, label = { Text(tr(lang, "app.jobs.name")) },
                    singleLine = true, modifier = Modifier.fillMaxWidth())
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(prompt, { prompt = it }, label = { Text(tr(lang, "app.jobs.prompt")) },
                    minLines = 3, modifier = Modifier.fillMaxWidth())
                Spacer(Modifier.height(8.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    listOf(
                        "daily" to tr(lang, "app.jobs.daily"),
                        "interval" to tr(lang, "app.jobs.interval"),
                        "manual" to tr(lang, "app.jobs.manual"),
                    ).forEach { (k, label) ->
                        OutlinedButton(
                            onClick = { kind = k },
                            modifier = Modifier.weight(1f),
                        ) { Text(label, color = if (kind == k) Sea else Muted, fontSize = 12.sp) }
                    }
                }
                if (kind == "daily") {
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(time, { time = it }, label = { Text(tr(lang, "app.jobs.time")) },
                        singleLine = true, modifier = Modifier.fillMaxWidth())
                }
                if (kind == "interval") {
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(interval, { interval = it }, label = { Text(tr(lang, "app.jobs.everyHours")) },
                        singleLine = true, modifier = Modifier.fillMaxWidth())
                }
                Spacer(Modifier.height(4.dp))
                Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                    Text(tr(lang, "app.jobs.telegram"), color = Ink, fontSize = 14.sp, modifier = Modifier.weight(1f))
                    Switch(checked = notify, onCheckedChange = { notify = it })
                }
            }
        },
        confirmButton = {
            Button(
                enabled = valid,
                onClick = {
                    onSave(base.copy(
                        name = name.trim(), prompt = prompt.trim(), kind = kind,
                        time = time.trim(), intervalH = interval.toDoubleOrNull() ?: 24.0,
                        notify = notify,
                    ))
                },
            ) { Text(tr(lang, "btn.save")) }
        },
        dismissButton = { OutlinedButton(onClick = onDismiss) { Text(tr(lang, "btn.close")) } },
    )
}

fun trimHours(h: Double): String =
    if (h == h.toInt().toDouble()) h.toInt().toString() else h.toString()

/** Lectura rica d'una execució: capçalera + butlletí amb el render del xat. */
@Composable
fun RunDetail(r: RunItem, onDismiss: () -> Unit, lang: String = "ca") {
    Dialog(onDismissRequest = onDismiss) {
        Card(
            modifier = Modifier.fillMaxWidth().padding(vertical = 24.dp),
            colors = CardDefaults.cardColors(containerColor = DeepBottom),
            shape = RoundedCornerShape(20.dp),
        ) {
            Column(Modifier.padding(18.dp)) {
                Text(stripMdForTitle(r.jobName.ifEmpty { tr(lang, "app.jobs.emptyJobName") }),
                    fontWeight = FontWeight.Bold, fontSize = 20.sp, color = Ink)
                Text(shortStamp(r.startedAt) +
                    (if (r.status == "ok") " · ${tr(lang, "app.jobs.doneWord")}" else " · ${tr(lang, "app.jobs.errorWord")}"),
                    color = Muted, fontSize = 13.sp, modifier = Modifier.padding(top = 2.dp))
                Spacer(Modifier.height(8.dp))
                Column(Modifier.weight(1f, fill = false).verticalScroll(rememberScrollState())) {
                    if (r.status == "ok" && r.output.isNotBlank()) {
                        AssistantResponse(r.output, lang)
                    } else {
                        Text(r.error.ifEmpty { tr(lang, "app.jobs.noDetail") }, color = Muted, fontSize = 14.sp)
                    }
                }
                Spacer(Modifier.height(12.dp))
                Button(onClick = onDismiss, modifier = Modifier.fillMaxWidth()) { Text(tr(lang, "btn.close")) }
            }
        }
    }
}
