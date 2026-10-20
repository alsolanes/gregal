package xyz.solanes.gregal

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.ArrowBack
import androidx.compose.material.icons.rounded.Edit
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
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
    LaunchedEffect(wind.id) { vm.loadWindProfile(wind.id); vm.loadWindChat(wind.id) }
    val current = profile?.takeIf { it.wind.id == wind.id }
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        WindProfileContent(lang, current?.wind ?: wind, current, busy, error, chats[wind.id].orEmpty(),
            onDismiss = onDismiss,
            onRetry = { vm.loadWindProfile(wind.id) },
            onEdit = { onDismiss(); vm.showWindCreator(true, current?.wind ?: wind) },
            onSaveKnowledge = { text, done -> vm.updateWindKnowledge(wind.id, text, done) })
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun WindProfileContent(lang: String, wind: Wind, profile: WindProfile?, busy: Boolean, error: String?,
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
            Column(Modifier.fillMaxWidth().padding(bottom = 20.dp), horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(10.dp)) {
                WindFace(size = 92, accent = Color(wind.argb), style = wind.faceStyle)
                Text(wind.name, color = Ink, fontWeight = FontWeight.Bold, fontSize = 24.sp)
                if (wind.role.isNotBlank()) Text(wind.role, color = Muted, modifier = Modifier.padding(horizontal = 24.dp))
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
                        SelectionContainer { Text(wind.persona.ifBlank { tr(lang, "app.vents.noPersonality") }, color = Tinta, lineHeight = 23.sp) }
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
