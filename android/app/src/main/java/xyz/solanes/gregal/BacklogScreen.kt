package xyz.solanes.gregal

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.Checklist
import androidx.compose.material.icons.rounded.PlayArrow
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CheckboxDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

// Pantalla Backlog: la mateixa llista de feina que la web, per mirar-la i
// moure-la des del mòbil. «Treballa-hi» la dona a l'agent a la conversa
// d'aquest dispositiu (al mòbil no s'obren worktrees).

private fun bx(lang: String, ca: String, en: String) = if (lang == "en") en else ca

private fun statusLabel(lang: String, s: String) = when (s) {
    "proposed" -> bx(lang, "Proposada", "Proposed")
    "ready" -> bx(lang, "A punt", "Ready")
    "in_progress" -> bx(lang, "En curs", "In progress")
    "in_review" -> bx(lang, "En revisió", "In review")
    "done" -> bx(lang, "Feta", "Done")
    "blocked" -> bx(lang, "Bloquejada", "Blocked")
    "dropped" -> bx(lang, "Descartada", "Dropped")
    else -> s
}

private fun typeLabel(lang: String, t: String) = when (t) {
    "feature" -> bx(lang, "Funcionalitat", "Feature")
    "bug" -> bx(lang, "Error", "Bug")
    "task" -> bx(lang, "Tasca", "Task")
    "spike" -> bx(lang, "Investigació", "Spike")
    "epic" -> bx(lang, "Èpica", "Epic")
    else -> t
}

private fun prioLabel(lang: String, p: String) = when (p) {
    "now" -> bx(lang, "Ara", "Now")
    "later" -> bx(lang, "Més endavant", "Later")
    else -> bx(lang, "Després", "Next")
}

@Composable
private fun statusColor(s: String): Color = when (s) {
    "ready" -> Onada
    "in_progress" -> Sea
    "in_review" -> Sand
    "done" -> Green
    "blocked" -> Red
    "dropped" -> Faint
    else -> Lila
}

/** Què demana el diàleg obert: un motiu, una prova o una tasca nova. */
private sealed class BacklogAsk {
    data class Reason(val id: Int, val status: String) : BacklogAsk()
    data class Evidence(val id: Int, val n: Int) : BacklogAsk()
    object Create : BacklogAsk()
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BacklogScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val state by vm.backlog.collectAsState()
    val loading by vm.loading.collectAsState()
    val carregant = "backlog" in loading
    var filter by rememberSaveable { mutableStateOf("open") }
    var expanded by rememberSaveable { mutableIntStateOf(0) }
    var ask by remember { mutableStateOf<BacklogAsk?>(null) }
    LaunchedEffect(Unit) { vm.loadBacklog() }
    // Mentre el revisor treballa, la llista es refresca sola fins al veredicte.
    val reviewing = state?.items.orEmpty().any { it.review?.running() == true || (it.status == "in_review" && it.review == null) }
    LaunchedEffect(reviewing, state) {
        if (reviewing) { kotlinx.coroutines.delay(4000); vm.loadBacklog() }
    }

    val items = state?.items.orEmpty()
    val visible = items.filter {
        when (filter) {
            "open" -> it.open
            "all" -> true
            else -> it.status == filter
        }
    }
    val sorted = if (filter == "open") {
        visible.sortedWith(compareBy<BacklogItem>({ BacklogJson.OPEN_ORDER.indexOf(it.status) },
            { listOf("now", "next", "later").indexOf(it.priority) }, { it.id }))
    } else visible.sortedByDescending { it.updated }

    GregalBg {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                GregalTopBar("Backlog", lang, onMenu = onMenu,
                    subtitle = state?.let { s ->
                        s.project + " · " + if (s.storage == "repo") bx(lang, "a .gregal/backlog/", "in .gregal/backlog/") else bx(lang, "desat a Gregal", "saved in Gregal")
                    },
                    actions = { RefreshAction(lang, carregant) { vm.loadBacklog() } })
            },
            floatingActionButton = {
                FloatingActionButton(onClick = { ask = BacklogAsk.Create }, containerColor = Sea) {
                    Icon(Icons.Rounded.Add, bx(lang, "Tasca nova", "New item"), tint = OnSea)
                }
            },
            snackbarHost = { NoticeHost(vm) },
        ) { pad ->
            LazyColumn(Modifier.fillMaxSize().padding(pad).pullToRefresh(carregant) { vm.loadBacklog() }.padding(horizontal = 12.dp)) {
                item {
                    Row(Modifier.padding(vertical = 6.dp), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        listOf("open" to bx(lang, "Obertes", "Open"), "done" to bx(lang, "Fetes", "Done"), "all" to bx(lang, "Totes", "All")).forEach { (k, l) ->
                            val n = items.count { if (k == "open") it.open else if (k == "all") true else it.status == k }
                            FilterChip(selected = filter == k, onClick = { filter = k }, label = { Text("$l · $n") })
                        }
                    }
                }
                if (items.isEmpty() && carregant) item { LoadingBoard(lang) }
                else if (items.isEmpty()) item {
                    EmptyBoard(Icons.Rounded.Checklist, bx(lang, "Encara cap tasca", "No items yet"),
                        bx(lang, "Crea’n una amb el botó + o demana-ho a l’agent: «posa al backlog que…».",
                            "Create one with the + button or ask the agent: “add to the backlog that…”."))
                }
                items(sorted, key = { it.id }) { it ->
                    BacklogCard(vm, lang, it, expanded == it.id, { expanded = if (expanded == it.id) 0 else it.id }) { a -> ask = a }
                }
                item { Spacer(Modifier.height(88.dp)) }
            }
        }
    }

    when (val a = ask) {
        is BacklogAsk.Reason -> TextAsk(
            title = if (a.status == "blocked") bx(lang, "Què la bloqueja?", "What blocks it?") else bx(lang, "Per què es descarta?", "Why is it dropped?"),
            hint = if (a.status == "blocked") bx(lang, "I qui ho pot resoldre", "And who can unblock it") else "",
            lang = lang, onDismiss = { ask = null },
        ) { r -> ask = null; vm.backlogAct(BacklogJson.move(a.id, a.status, r)) }
        is BacklogAsk.Evidence -> TextAsk(
            title = bx(lang, "Quina prova ho demostra?", "What proves it?"),
            hint = bx(lang, "Test, comanda, fitxer o captura", "Test, command, file or screenshot"),
            lang = lang, onDismiss = { ask = null },
        ) { ev -> ask = null; vm.backlogAct(BacklogJson.check(a.id, a.n, true, ev)) }
        BacklogAsk.Create -> CreateItem(lang, onDismiss = { ask = null }) { body -> ask = null; vm.backlogAct(body, bx(lang, "Creada com a proposada.", "Created as proposed.")) }
        null -> {}
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun BacklogCard(vm: GregalViewModel, lang: String, it: BacklogItem, open: Boolean, toggle: () -> Unit, ask: (BacklogAsk) -> Unit) {
    val c = statusColor(it.status)
    Card(
        modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp).clickable { toggle() },
        colors = CardDefaults.cardColors(containerColor = PillDark),
    ) {
        Column(Modifier.padding(14.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text("#${it.id}", color = Faint, fontFamily = FontFamily.Monospace, fontSize = GregalSize.caption)
                Spacer(Modifier.width(6.dp))
                Text(typeLabel(lang, it.type), color = Muted, fontSize = GregalSize.label, fontWeight = FontWeight.Bold)
                Spacer(Modifier.weight(1f))
                Surface(shape = RoundedCornerShape(8.dp), color = c.copy(alpha = 0.16f)) {
                    Text(statusLabel(lang, it.status), color = c, fontSize = GregalSize.label, fontWeight = FontWeight.Bold,
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp))
                }
            }
            Text(it.title, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = GregalSize.large, modifier = Modifier.padding(top = 4.dp))
            val meta = buildList {
                add(prioLabel(lang, it.priority))
                if (it.size.isNotBlank()) add(it.size)
                if (it.criteria.isNotEmpty()) add("${it.doneCriteria}/${it.criteria.size} " + bx(lang, "criteris", "criteria"))
            }
            Text(meta.joinToString(" · "), color = Muted, fontSize = GregalSize.caption)
            if (it.reason.isNotBlank() && (it.status == "blocked" || it.status == "dropped")) {
                Text(it.reason, color = Ink, fontSize = GregalSize.small, modifier = Modifier.padding(top = 6.dp))
            }
            if (!open) return@Column

            if (it.why.isNotBlank()) {
                Section(if (it.type == "bug") bx(lang, "Què passa", "What happens") else bx(lang, "Perquè", "Why"))
                Text(it.why, color = Ink, fontSize = GregalSize.body)
            }
            ReviewBlock(lang, it)
            if (it.criteria.isNotEmpty()) {
                Section(bx(lang, "Criteris d’acceptació", "Acceptance criteria"))
                it.criteria.forEachIndexed { i, cr ->
                    Row(verticalAlignment = Alignment.Top) {
                        Checkbox(checked = cr.done, colors = CheckboxDefaults.colors(checkedColor = Green), onCheckedChange = { on ->
                            if (on) ask(BacklogAsk.Evidence(it.id, i + 1)) else vm.backlogAct(BacklogJson.check(it.id, i + 1, false))
                        })
                        Column(Modifier.padding(top = 12.dp)) {
                            Text(cr.text, color = if (cr.done) Muted else Ink, fontSize = GregalSize.body)
                            if (cr.evidence.isNotBlank()) Text(bx(lang, "Prova: ", "Evidence: ") + cr.evidence, color = Faint, fontSize = GregalSize.caption, fontFamily = FontFamily.Monospace)
                        }
                    }
                }
            }
            if (it.outOfScope.isNotBlank()) {
                Section(bx(lang, "Fora d’abast", "Out of scope"))
                Text(it.outOfScope, color = Ink, fontSize = GregalSize.body)
            }
            Spacer(Modifier.height(10.dp))
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                val move = { s: String -> vm.backlogAct(BacklogJson.move(it.id, s)) }
                when (it.status) {
                    "proposed" -> {
                        FilledTonalButton(onClick = { move("ready") }) { Text(bx(lang, "Marca-la a punt", "Mark ready")) }
                        OutlinedButton(onClick = { ask(BacklogAsk.Reason(it.id, "dropped")) }) { Text(bx(lang, "Descarta", "Drop")) }
                    }
                    "ready", "blocked" -> {
                        if (it.status == "ready" && it.type != "epic") FilledTonalButton(onClick = { vm.workOnBacklog(it.id) }) {
                            Icon(Icons.Rounded.PlayArrow, null); Spacer(Modifier.width(4.dp)); Text(bx(lang, "Treballa-hi", "Work on it"))
                        }
                        if (it.status == "blocked") FilledTonalButton(onClick = { move("ready") }) { Text(bx(lang, "Desbloqueja", "Unblock")) }
                        else OutlinedButton(onClick = { ask(BacklogAsk.Reason(it.id, "blocked")) }) { Text(bx(lang, "Bloqueja", "Block")) }
                        OutlinedButton(onClick = { ask(BacklogAsk.Reason(it.id, "dropped")) }) { Text(bx(lang, "Descarta", "Drop")) }
                    }
                    "in_progress" -> {
                        FilledTonalButton(onClick = { move("in_review") }) { Text(bx(lang, "Envia a revisió", "Send to review")) }
                        OutlinedButton(onClick = { vm.workOnBacklog(it.id) }) { Text(bx(lang, "Continua", "Continue")) }
                        OutlinedButton(onClick = { ask(BacklogAsk.Reason(it.id, "blocked")) }) { Text(bx(lang, "Bloqueja", "Block")) }
                    }
                    "in_review" -> {
                        val running = it.review?.running() == true
                        FilledTonalButton(enabled = !running, onClick = { move("done") }) { Text(bx(lang, "Marca-la feta", "Mark done")) }
                        if (!running) OutlinedButton(onClick = { vm.backlogAct(BacklogJson.review(it.id), bx(lang, "El revisor s’hi ha posat.", "The reviewer is on it.")) }) {
                            Text(if (it.review == null) bx(lang, "Revisa-la", "Review it") else bx(lang, "Revisa de nou", "Review again"))
                        }
                        OutlinedButton(onClick = { move("in_progress") }) { Text(bx(lang, "Torna a en curs", "Back to in progress")) }
                    }
                    else -> OutlinedButton(onClick = { move("proposed") }) { Text(bx(lang, "Reobre", "Reopen")) }
                }
            }
            if (it.log.isNotEmpty()) {
                Section(bx(lang, "Registre", "Log"))
                it.log.takeLast(6).reversed().forEach { l ->
                    Text((l.at.take(16).replace('T', ' ')) + " · " + l.who.ifBlank { bx(lang, "tu", "you") }, color = Faint, fontSize = GregalSize.label, fontFamily = FontFamily.Monospace)
                    Text(l.text, color = Ink, fontSize = GregalSize.small, modifier = Modifier.padding(bottom = 4.dp))
                }
            }
        }
    }
}

@Composable
private fun ReviewBlock(lang: String, it: BacklogItem) {
    val r = it.review
    if (r == null && it.status != "in_review") return
    Section(bx(lang, "Revisió independent", "Independent review"))
    val (text, color) = when {
        r == null -> bx(lang, "Encara no s’ha revisat.", "Not reviewed yet.") to Muted
        r.running() -> bx(lang, "Revisant: mira el diff, passa les comprovacions i jutja cada criteri…", "Reviewing: reading the diff, running the checks and judging each criterion…") to Sand
        r.status == "running" -> bx(lang, "La revisió es va quedar a mitges. Torna-la a demanar.", "The review did not finish. Ask for it again.") to Muted
        r.status == "met" -> bx(lang, "Superada. Ja la pots marcar com a feta.", "Passed. You can mark it done.") to Green
        r.status == "not_met" -> bx(lang, "No superada: ", "Not passed: ") + r.summary to Red
        else -> bx(lang, "El revisor ha fallat: ", "The reviewer failed: ") + r.summary to Red
    }
    Text(text, color = color, fontSize = GregalSize.body)
    r?.checks?.forEach { c ->
        Text((if (c.passed) "✓ " else "✗ ") + c.command, color = if (c.passed) Green else Red, fontSize = GregalSize.caption, fontFamily = FontFamily.Monospace)
    }
    r?.criteria?.forEachIndexed { n, c ->
        val (label, col) = when (c.status) {
            "MET" -> bx(lang, "Complert", "Met") to Green
            "NOT MET" -> bx(lang, "No complert", "Not met") to Red
            else -> bx(lang, "Sense comprovar", "Unverified") to Sand
        }
        Text("${n + 1}. $label · ${c.text}", color = col, fontSize = GregalSize.small, modifier = Modifier.padding(top = 4.dp))
        if (c.evidence.isNotBlank()) Text(c.evidence, color = Faint, fontSize = GregalSize.caption, fontFamily = FontFamily.Monospace)
    }
}

@Composable
private fun Section(title: String) {
    Text(title.uppercase(), color = Muted, fontSize = GregalSize.label, fontWeight = FontWeight.Bold, modifier = Modifier.padding(top = 12.dp, bottom = 4.dp))
}

@Composable
private fun TextAsk(title: String, hint: String, lang: String, onDismiss: () -> Unit, onOk: (String) -> Unit) {
    var text by remember { mutableStateOf("") }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = { OutlinedTextField(text, { text = it }, placeholder = { Text(hint) }, modifier = Modifier.fillMaxWidth()) },
        confirmButton = { TextButton(enabled = text.isNotBlank(), onClick = { onOk(text.trim()) }) { Text(bx(lang, "D’acord", "OK")) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text(bx(lang, "Cancel·la", "Cancel")) } },
    )
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun CreateItem(lang: String, onDismiss: () -> Unit, onCreate: (org.json.JSONObject) -> Unit) {
    var type by remember { mutableStateOf("feature") }
    var title by remember { mutableStateOf("") }
    var why by remember { mutableStateOf("") }
    var criteria by remember { mutableStateOf("") }
    var size by remember { mutableStateOf("") }
    var prio by remember { mutableStateOf("next") }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(bx(lang, "Tasca nova", "New item")) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    BacklogJson.TYPES.forEach { t -> FilterChip(selected = type == t, onClick = { type = t }, label = { Text(typeLabel(lang, t)) }) }
                }
                OutlinedTextField(title, { title = it }, label = { Text(bx(lang, "Títol", "Title")) },
                    placeholder = { Text(bx(lang, "En imperatiu i concret", "Imperative and specific")) }, modifier = Modifier.fillMaxWidth())
                OutlinedTextField(why, { why = it }, label = { Text(if (type == "bug") bx(lang, "Què passa", "What happens") else bx(lang, "Perquè", "Why")) },
                    modifier = Modifier.fillMaxWidth(), minLines = 2)
                OutlinedTextField(criteria, { criteria = it }, label = { Text(bx(lang, "Criteris (un per línia)", "Criteria (one per line)")) },
                    modifier = Modifier.fillMaxWidth(), minLines = 3)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    listOf("S", "M", "L").forEach { s -> FilterChip(selected = size == s, onClick = { size = if (size == s) "" else s }, label = { Text(s) }) }
                    listOf("now", "next", "later").forEach { p -> FilterChip(selected = prio == p, onClick = { prio = p }, label = { Text(prioLabel(lang, p)) }) }
                }
            }
        },
        confirmButton = {
            TextButton(enabled = title.isNotBlank(), onClick = { onCreate(BacklogJson.create(type, title, why, criteria, size, prio)) }) { Text(bx(lang, "Crea-la", "Create")) }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text(bx(lang, "Cancel·la", "Cancel")) } },
    )
}
