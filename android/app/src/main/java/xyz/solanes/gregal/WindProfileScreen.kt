package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.ArrowBack
import androidx.compose.material.icons.rounded.Edit
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties

@Composable
internal fun WindProfileDialog(vm: GregalViewModel, lang: String, wind: Wind, onDismiss: () -> Unit) {
    val profile by vm.windProfile.collectAsState()
    val busy by vm.windProfileBusy.collectAsState()
    val error by vm.windProfileError.collectAsState()
    val chats by vm.windChat.collectAsState()
    LaunchedEffect(wind.id) { vm.loadWindProfile(wind.id); vm.loadWindChat(wind.id); vm.loadJobs() }
    val current = profile?.takeIf { it.wind.id == wind.id }
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        WindProfileContent(vm, lang, current?.wind ?: wind, current, busy, error, chats[wind.id].orEmpty(),
            onDismiss = onDismiss,
            onRetry = { vm.loadWindProfile(wind.id) },
            onEdit = { onDismiss(); vm.showWindCreator(true, current?.wind ?: wind) },
            onSaveKnowledge = { text, done -> vm.updateWindKnowledge(wind.id, text, done) })
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun WindProfileContent(vm: GregalViewModel, lang: String, wind: Wind, profile: WindProfile?, busy: Boolean, error: String?,
    messages: List<GregalViewModel.PetMsg>, onDismiss: () -> Unit, onRetry: () -> Unit, onEdit: () -> Unit,
    onSaveKnowledge: (String, (Boolean) -> Unit) -> Unit) {
    var tab by remember(wind.id) { mutableStateOf(0) }
    var editing by remember(wind.id) { mutableStateOf(false) }
    var draft by remember(wind.id) { mutableStateOf("") }
    var saving by remember { mutableStateOf(false) }
    var saveError by remember { mutableStateOf(false) }
    var confirmForget by remember { mutableStateOf(false) }
    Scaffold(containerColor = Superficie, modifier = Modifier.fillMaxSize().statusBarsPadding().navigationBarsPadding(),
        topBar = { TopAppBar(
            title = { Text(wind.name) },
            navigationIcon = { IconButton(onClick = onDismiss, enabled = !saving) { Icon(Icons.Rounded.ArrowBack, tr(lang, "app.vents.back")) } },
            actions = { IconButton(onClick = onEdit, enabled = !busy && !saving && profile != null) { Icon(Icons.Rounded.Edit, tr(lang, "app.company.windEdit")) } },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie)) }) { padding ->
        Column(Modifier.fillMaxSize().padding(padding)) {
            val accent = Color(wind.argb)
            Box(
                Modifier.fillMaxWidth()
                    .background(Brush.verticalGradient(listOf(accent.copy(alpha = 0.22f), Color.Transparent)))
                    .padding(bottom = 20.dp, top = 8.dp),
                contentAlignment = Alignment.Center,
            ) {
            Column(horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(10.dp)) {
                WindHalo(accent = accent, style = wind.faceStyle, size = 92, mood = WindMood.HAPPY)
                Text(wind.name,
                    style = androidx.compose.ui.text.TextStyle(
                        brush = Brush.linearGradient(listOf(Ink, accent)),
                        fontWeight = FontWeight.Bold, fontSize = 26.sp),
                )
                if (wind.role.isNotBlank()) Text(wind.role, color = Muted, modifier = Modifier.padding(horizontal = 24.dp))
            }
            }
            TabRow(selectedTabIndex = tab, containerColor = Superficie) {
                listOf("character", "memory", "activity").forEachIndexed { index, key ->
                    Tab(selected = tab == index, onClick = { tab = index }, text = { Text(tr(lang, "app.vents.$key")) })
                }
            }
            Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(20.dp), verticalArrangement = Arrangement.spacedBy(14.dp)) {
                if (busy) LinearProgressIndicator(Modifier.fillMaxWidth())
                if (error != null) {
                    Text(error, color = MaterialTheme.colorScheme.error)
                    OutlinedButton(onClick = onRetry) { Text(tr(lang, "app.vents.retry")) }
                }
                when (tab) {
                    0 -> {
                        Text(tr(lang, "app.vents.personality"), color = Ink, fontWeight = FontWeight.SemiBold)
                        Surface(shape = RoundedCornerShape(16.dp), color = accent.copy(alpha = 0.08f),
                            border = androidx.compose.foundation.BorderStroke(1.dp, accent.copy(alpha = 0.35f))) {
                            SelectionContainer { Text(wind.persona.ifBlank { tr(lang, "app.vents.noPersonality") }, color = Tinta, lineHeight = 23.sp,
                                modifier = Modifier.padding(14.dp)) }
                        }
                        OutlinedButton(onClick = onEdit, enabled = profile != null && !busy) { Text(tr(lang, "app.company.windEdit")) }
                    }
                    1 -> if (profile != null) {
                        Text(tr(lang, "app.vents.topicMemory"), color = Ink, fontWeight = FontWeight.SemiBold)
                        Text(tr(lang, "app.vents.topicMemoryHelp"), color = Muted, fontSize = 13.sp)
                        if (editing) {
                            OutlinedTextField(draft, { draft = it }, enabled = !saving, minLines = 5,
                                label = { Text(tr(lang, "app.vents.topicMemory")) }, modifier = Modifier.fillMaxWidth())
                            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                Button(onClick = {
                                    saving = true; saveError = false
                                    onSaveKnowledge(draft) { success -> saving = false; saveError = !success; if (success) editing = false }
                                }, enabled = !saving) { Text(tr(lang, if (saving) "app.vents.saving" else "app.vents.saveChanges")) }
                                TextButton(onClick = { editing = false }, enabled = !saving) { Text(tr(lang, "app.company.windCancel")) }
                            }
                        } else {
                            SelectionContainer { Text(profile.knowledge.ifBlank { tr(lang, "app.vents.noKnowledge") }, color = Tinta, lineHeight = 23.sp) }
                            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                OutlinedButton(onClick = { draft = profile.knowledge; editing = true; saveError = false }, enabled = !saving) { Text(tr(lang, "app.company.windEdit")) }
                                TextButton(onClick = { confirmForget = true }, enabled = profile.knowledge.isNotBlank() && !saving) { Text(tr(lang, "app.vents.forgetKnowledge")) }
                            }
                        }
                        if (saveError) Text(tr(lang, "app.vents.saveError"), color = MaterialTheme.colorScheme.error)
                        HorizontalDivider(color = Muted.copy(alpha = .15f))
                        Text(tr(lang, "app.vents.personalMemory"), color = Ink, fontWeight = FontWeight.SemiBold)
                        Text(tr(lang, "app.vents.personalMemoryHelp"), color = Muted, fontSize = 13.sp)
                        SelectionContainer { Text(profile.personalMemory.ifBlank { tr(lang, "app.vents.noPersonalMemory") }, color = Tinta, lineHeight = 23.sp) }
                    }
                    2 -> {
                        Text(tr(lang, "app.vents.activityHelp"), color = Muted, fontSize = 13.sp)
                        heartbeatRow(lang, vm, wind)
                        remindersRow(lang, vm, wind)
                        if (messages.isEmpty()) Text(tr(lang, "app.vents.noActivity"), color = Tinta)
                        messages.forEach { message ->
                            Surface(color = Color(wind.argb).copy(alpha = .08f), shape = MaterialTheme.shapes.medium) {
                                Column(Modifier.fillMaxWidth().padding(14.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                                    Text(if (message.from == "user") tr(lang, "app.vents.you") else wind.name, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 12.sp)
                                    SelectionContainer { Text(message.text, color = Tinta, fontSize = 14.sp) }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
    if (confirmForget) AlertDialog(onDismissRequest = { confirmForget = false },
        title = { Text(tr(lang, "app.vents.forgetKnowledge")) }, text = { Text(tr(lang, "app.vents.forgetHelp")) },
        confirmButton = { TextButton(onClick = {
            confirmForget = false; saving = true; saveError = false
            onSaveKnowledge("") { success -> saving = false; saveError = !success }
        }) { Text(tr(lang, "app.vents.forgetKnowledge")) } },
        dismissButton = { TextButton(onClick = { confirmForget = false }) { Text(tr(lang, "app.company.windCancel")) } })
}

/** Interruptor del cor del vent: quan s'encén, el vent vigila el seu tema
 *  amb cerques web i parla només si troba novetats que valguin la pena;
 *  es reprograma ell sol per quan acabi el proper esdeveniment del tema
 *  (p. ex. un partit). */
@Composable
private fun heartbeatRow(lang: String, vm: GregalViewModel, wind: Wind) {
    val jobs by vm.jobs.collectAsState()
    val job = jobs.firstOrNull { it.id == "wind-" + wind.id }
    var pending by remember(wind.id) { mutableStateOf(false) }
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) {
            Text(tr(lang, "app.company.windHeartbeat"), color = Ink, fontWeight = FontWeight.SemiBold)
            Text(tr(lang, "app.company.windHeartbeatOn"), color = Muted, fontSize = 13.sp)
            if (job != null && job.nextDue.isNotBlank() && job.enabled)
                Text(tr(lang, "app.company.windHeartbeatNext", job.nextDue), color = Muted, fontSize = 12.sp)
            if (job != null && job.lastAt.isNotBlank())
                Text(tr(lang, "app.company.windHeartbeatLast", job.lastAt), color = Muted, fontSize = 12.sp)
        }
        Switch(checked = job?.enabled == true, onCheckedChange = { on ->
            if (!pending) {
                pending = true
                vm.ensureWindHeartbeat(wind, on)
                // Refresca l'estat quan el job s'ha creat/esborrat.
                vm.loadJobs()
                pending = false
            }
        }, enabled = !pending)
    }
}

/** Avisos que el vent ha programat: text + quan + cancel·lar. */
@Composable
private fun remindersRow(lang: String, vm: GregalViewModel, wind: Wind) {
    val jobs by vm.jobs.collectAsState()
    val pending = jobs.filter { it.kind == "remind" && it.enabled && it.id.startsWith("remind-" + wind.id + "-") }
    if (pending.isEmpty()) return
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(tr(lang, "app.vents.reminders"), color = Ink, fontWeight = FontWeight.SemiBold)
        pending.forEach { j ->
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text(j.prompt, color = Tinta, fontSize = 14.sp)
                    val whenDue = j.nextDue.ifBlank { j.nextDueAt }
                    if (whenDue.isNotBlank()) Text(whenDue, color = Muted, fontSize = 12.sp)
                }
                TextButton(onClick = { vm.deleteJob(j.id) }) { Text(tr(lang, "app.company.windCancel")) }
            }
        }
    }
}
