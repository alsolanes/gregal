package xyz.solanes.gregal

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Menu
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
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
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Grafs del projecte: llistar, veure els passos, executar amb progrés en
 *  directe i esborrar. L'editor visual és cosa d'escriptori; al mòbil,
 *  executar i vigilar. Paritat funcional amb la vista de grafs de la web. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun FlowsScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val flows by vm.flows.collectAsState()
    val flow by vm.flow.collectAsState()
    val log by vm.flowLog.collectAsState()
    val running by vm.flowRunning.collectAsState()
    val answer by vm.flowAnswer.collectAsState()
    var input by remember { mutableStateOf("") }
    var confirma by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(Unit) { vm.loadFlows() }

    GregalBg {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                TopAppBar(
                    navigationIcon = {
                        if (flow == null) IconButton(onClick = onMenu) { Icon(Icons.Default.Menu, tr(lang, "app.nav.menu"), tint = Ink) }
                        else IconButton(onClick = { vm.closeFlow() }) { Icon(Icons.Default.ArrowBack, tr(lang, "app.flow.back"), tint = Ink) }
                    },
                    title = { Text(flow?.name ?: tr(lang, "graphs.title"), color = Ink) },
                    actions = {
                        val f = flow
                        if (f != null && !running) IconButton(onClick = { confirma = f.name }) {
                            Icon(Icons.Default.Delete, tr(lang, "btn.delete"), tint = Ink)
                        }
                    },
                    colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent))
            },
        ) { pad ->
            val f = flow
            if (f == null) {
                LazyColumn(Modifier.fillMaxSize().padding(pad).padding(12.dp)) {
                    if (flows.isEmpty()) item {
                        EmptyBoard("◈", tr(lang, "graphs.title"), tr(lang, "graphs.lead"))
                    }
                    items(flows, key = { it.slug.ifBlank { it.name } }) { fl ->
                        Card(
                            onClick = { vm.openFlow(fl.name) },
                            modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp),
                            colors = CardDefaults.cardColors(containerColor = PillDark),
                        ) {
                            Column(Modifier.padding(14.dp)) {
                                Text(fl.name, color = Ink, fontSize = 16.sp)
                                if (fl.desc.isNotBlank()) Text(fl.desc, color = Muted, fontSize = 13.sp)
                                Text(tr(lang, "app.flow.steps", fl.steps), color = Sea, fontSize = 12.sp)
                            }
                        }
                    }
                }
            } else {
                LazyColumn(Modifier.fillMaxSize().padding(pad).padding(12.dp)) {
                    if (f.desc.isNotBlank()) item {
                        Text(f.desc, color = Muted, fontSize = 13.sp)
                        Spacer(Modifier.height(8.dp))
                    }
                    items(f.nodes) { n ->
                        Row(Modifier.fillMaxWidth().padding(vertical = 4.dp),
                            verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                when (n.kind) { "agent" -> "◆"; "tool" -> "🔧"; else -> "📝" },
                                fontSize = 16.sp)
                            Spacer(Modifier.width(10.dp))
                            Column(Modifier.weight(1f)) {
                                Text(n.label(), color = Ink, fontSize = 15.sp)
                                if (n.detail().isNotBlank()) Text(n.detail(), color = Muted, fontSize = 12.sp)
                            }
                        }
                    }
                    item {
                        Spacer(Modifier.height(10.dp))
                        OutlinedTextField(input, { input = it }, Modifier.fillMaxWidth(),
                            label = { Text(tr(lang, "app.flow.input")) }, singleLine = true,
                            enabled = !running)
                        Spacer(Modifier.height(8.dp))
                        Button(onClick = { vm.runFlow(f.name, input) },
                            enabled = !running, modifier = Modifier.fillMaxWidth()) {
                            Icon(Icons.Default.PlayArrow, null)
                            Spacer(Modifier.width(6.dp))
                            Text(tr(lang, "btn.run"))
                        }
                        if (running) {
                            Spacer(Modifier.height(8.dp))
                            LinearProgressIndicator(Modifier.fillMaxWidth())
                        }
                    }
                    if (log.isNotEmpty()) item {
                        Spacer(Modifier.height(10.dp))
                        Text(tr(lang, "app.flow.progress"), color = Muted, fontSize = 13.sp)
                    }
                    items(log) { s ->
                        val tit = s.title.ifBlank { s.node }.ifBlank { tr(lang, "app.flow.step") }
                        Text((if (s.error.isNotBlank()) "✗ " else "✓ ") + tit +
                            (if (s.ms > 0) " · ${s.ms} ms" else ""),
                            color = if (s.error.isNotBlank()) Sand else Ink, fontSize = 13.sp,
                            modifier = Modifier.padding(vertical = 2.dp))
                        if (s.error.isNotBlank()) Text(s.error, color = Sand, fontSize = 12.sp)
                    }
                    if (answer.isNotBlank()) item {
                        Spacer(Modifier.height(10.dp))
                        Card(Modifier.fillMaxWidth(),
                            colors = CardDefaults.cardColors(containerColor = BubbleDark)) {
                            Text(answer, color = Ink, fontSize = 14.sp,
                                modifier = Modifier.padding(12.dp))
                        }
                        Spacer(Modifier.height(8.dp))
                        OutlinedButton(onClick = { vm.go(Screen.Chat) },
                            modifier = Modifier.fillMaxWidth()) {
                            Text(tr(lang, "app.flow.toChat"))
                        }
                    }
                }
            }
            confirma?.let { nom ->
                AlertDialog(
                    onDismissRequest = { confirma = null },
                    title = { Text(tr(lang, "btn.delete"), color = Ink) },
                    text = { Text(nom, color = Muted, fontSize = 14.sp) },
                    confirmButton = {
                        TextButton(onClick = { vm.deleteFlow(nom); confirma = null }) {
                            Text(tr(lang, "btn.delete"), color = Sea)
                        }
                    },
                    dismissButton = {
                        TextButton(onClick = { confirma = null }) { Text(tr(lang, "btn.close")) }
                    },
                )
            }
        }
    }
}
