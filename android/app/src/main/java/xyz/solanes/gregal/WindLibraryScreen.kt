package xyz.solanes.gregal

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties

internal fun libraryDate(value: String, lang: String): String = runCatching {
    java.time.Instant.parse(value).atZone(java.time.ZoneId.systemDefault()).format(
        java.time.format.DateTimeFormatter.ofLocalizedDateTime(java.time.format.FormatStyle.MEDIUM, java.time.format.FormatStyle.SHORT)
            .withLocale(java.util.Locale.forLanguageTag(lang)))
}.getOrDefault(value.substringBefore('T'))

/** Authenticated information recovery. No transcript/export/import promises. */
@Composable
internal fun WindLibraryDialog(vm: GregalViewModel, lang: String, onDismiss: () -> Unit) {
    val state by vm.windLibrary.collectAsState()
    // Reset selection, confirmation and private snapshot bodies on EVERY origin generation.
    key(state.generation) {
        var selected by remember { mutableStateOf<GregalApi.WindLibraryItem?>(null) }
        var filter by remember { mutableStateOf("archive") }
        var assigning by remember { mutableStateOf(false) }
        var target by remember { mutableStateOf<Wind?>(null) }
        var memory by remember { mutableStateOf(false) }
        var knowledge by remember { mutableStateOf(false) }
        var confirm by remember { mutableStateOf(false) }
        var completed by remember { mutableStateOf(false) }
        val pending = state.pending != null
        LaunchedEffect(Unit) { vm.loadWindLibrary() }
        val dismiss = { if (!pending) onDismiss() }
        Dialog(onDismissRequest = dismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
            Surface(color = MaterialTheme.colorScheme.background, shape = MaterialTheme.shapes.large,
                modifier = Modifier.fillMaxWidth().padding(12.dp)) {
                Column(Modifier.fillMaxWidth().fillMaxHeight(.90f).padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(tr(lang, "app.library.title"), color = Ink, fontWeight = FontWeight.SemiBold,
                            fontSize = 18.sp, modifier = Modifier.weight(1f))
                        TextButton(onClick = dismiss, enabled = !pending) { Text(tr(lang, "app.library.close")) }
                    }
                    if (selected == null) {
                        Text(tr(lang, "app.library.about"), color = Muted, fontSize = 13.sp)
                        Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                            listOf("archive", "active", "history").forEach { f ->
                                FilterChip(selected = filter == f, onClick = { filter = f },
                                    label = { Text(tr(lang, "app.library.$f")) })
                            }
                        }
                    } else {
                        TextButton(onClick = {
                            if (assigning) { assigning = false; confirm = false }
                            else { selected = null; completed = false }
                        }, enabled = !pending) { Text(tr(lang, "app.library.back")) }
                    }
                    if (pending || state.loading) {
                        LinearProgressIndicator(Modifier.fillMaxWidth(), color = Sea)
                        Text(tr(lang, if (pending) "app.library.pending" else "app.library.loading"), color = Muted, fontSize = 13.sp)
                    }
                    state.error?.let { error ->
                        Text(error, color = MaterialTheme.colorScheme.error, fontSize = 13.sp)
                        if (selected == null) TextButton(onClick = vm::loadWindLibrary, enabled = !pending) {
                            Text(tr(lang, "app.library.retry"))
                        }
                    }
                    if (completed) Text(tr(lang, "app.library.success"), color = Ink, fontSize = 13.sp)
                    val item = selected
                    if (item == null) {
                        val rows = state.items.orEmpty().filter {
                            when (filter) { "archive" -> it.restorable; "active" -> it.kind == "active"; else -> !it.restorable && it.kind != "active" }
                        }
                        LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            if (rows.isEmpty() && !state.loading && state.error == null) item {
                                Text(tr(lang, "app.library.empty"), color = Muted, modifier = Modifier.padding(vertical = 24.dp))
                            }
                            items(rows, key = { it.id }) { record ->
                                Surface(onClick = { selected = record; completed = false }, color = SuperficieAlta,
                                    shape = MaterialTheme.shapes.medium, modifier = Modifier.fillMaxWidth()) {
                                    Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                                        Text(record.wind.name, color = Ink, fontWeight = FontWeight.SemiBold)
                                        Text(libraryKind(lang, record.kind), color = Muted, fontSize = 12.sp)
                                        if (record.wind.role.isNotBlank()) Text(record.wind.role, color = Muted, fontSize = 13.sp)
                                        if (record.created.isNotBlank() && record.kind != "active") Text(libraryDate(record.created, lang), color = Muted, fontSize = 11.sp)
                                        Text(tr(lang, "app.library.inspect") + " ›", color = Sea, fontSize = 13.sp)
                                    }
                                }
                            }
                        }
                        TextButton(onClick = vm::loadWindLibrary, enabled = !state.loading) { Text(tr(lang, "app.library.refresh")) }
                    } else {
                        val identityActive = state.items.orEmpty().any { it.kind == "active" && it.wind.id == item.wind.id }
                        LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                            item {
                                Text(item.wind.name, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 19.sp)
                                Text(libraryKind(lang, item.kind), color = Muted, fontSize = 12.sp)
                                if (item.wind.role.isNotBlank()) Text(item.wind.role, color = Muted)
                                if (!item.restorable) Text(tr(lang, "app.library.versionHint"), color = Muted, fontSize = 13.sp)
                                else if (identityActive) Text(tr(lang, "app.library.identityActive"), color = Muted, fontSize = 13.sp)
                            }
                            if (assigning) {
                                val targets = state.items.orEmpty().filter { it.kind == "active" && it.wind.id != item.wind.id }
                                item {
                                    Text(tr(lang, "app.library.target"), color = Ink, fontWeight = FontWeight.SemiBold)
                                    if (targets.isEmpty()) Text(tr(lang, "app.library.noTargets"), color = Muted)
                                }
                                items(targets, key = { "target-" + it.id }) { t ->
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        RadioButton(selected = target?.id == t.wind.id, enabled = !pending,
                                            onClick = { target = t.wind; confirm = false })
                                        Text(t.wind.name, color = Ink)
                                    }
                                }
                                item {
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        Checkbox(memory, enabled = !pending, onCheckedChange = { memory = it; confirm = false })
                                        Text(tr(lang, "app.library.memoryOption"), color = Ink)
                                    }
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        Checkbox(knowledge, enabled = !pending, onCheckedChange = { knowledge = it; confirm = false })
                                        Text(tr(lang, "app.library.knowledgeOption"), color = Ink)
                                    }
                                }
                            }
                            item { LibraryBody(tr(lang, "app.library.persona"), item.wind.persona, lang) }
                            item { LibraryBody(tr(lang, "app.library.memory"), item.memory, lang) }
                            item { LibraryBody(tr(lang, "app.library.knowledge"), item.knowledge, lang) }
                            item { Text(tr(lang, "app.library.skills"), color = Ink, fontWeight = FontWeight.SemiBold) }
                            if (item.skills.isEmpty()) item { Text(tr(lang, "app.library.noContent"), color = Muted) }
                            items(item.skills, key = { "skill-" + it.nom }) { skill ->
                                LibraryBody(skill.titol.ifBlank { skill.nom }, listOf(skill.descripcio, skill.cos).filter { it.isNotBlank() }.joinToString("\n\n"), lang)
                            }
                        }
                        if (confirm) {
                            Text(if (assigning) tr(lang, "app.library.assignConfirm", item.wind.name, target?.name.orEmpty())
                                else tr(lang, "app.library.restoreConfirm", item.wind.name), color = Ink, fontSize = 13.sp)
                            Row {
                                TextButton(enabled = !pending, onClick = { confirm = false }) { Text(tr(lang, "app.library.cancel")) }
                                Button(enabled = !pending, onClick = {
                                    val done: (Boolean) -> Unit = { ok -> if (ok) { confirm = false; completed = true } }
                                    if (assigning) target?.let { vm.assignWindLibrary(item, it, memory, knowledge, done) }
                                    else vm.restoreWindLibrary(item, done)
                                }) { Text(tr(lang, if (state.error == null) "app.library.confirm" else "app.library.retry")) }
                            }
                        } else if (!completed) {
                            if (assigning) {
                                Button(onClick = { confirm = true }, enabled = !pending && target != null && (memory || knowledge)) {
                                    Text(tr(lang, "app.library.confirm"))
                                }
                            } else {
                                if (item.restorable && !identityActive) Button(onClick = { confirm = true }, enabled = !pending) {
                                    Text(tr(lang, "app.library.restore"))
                                }
                                OutlinedButton(onClick = { assigning = true; target = null; memory = false; knowledge = false }, enabled = !pending) {
                                    Text(tr(lang, "app.library.assign"))
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

private fun libraryKind(lang: String, kind: String): String = tr(lang,
    "app.library.kind." + kind.takeIf { it in setOf("active", "archived", "recovered", "version") }.orEmpty().ifBlank { "unknown" })

@Composable
private fun LibraryBody(title: String, body: String, lang: String) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(title, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp)
        SelectionContainer { Text(body.ifBlank { tr(lang, "app.library.noContent") }, color = Muted, fontSize = 13.sp) }
    }
}
