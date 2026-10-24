package xyz.solanes.gregal

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Named single-wind chats use the same durable thread store as groups. */
@Composable
internal fun ConversationCreator(vm: GregalViewModel, lang: String, winds: List<Wind>, group: Boolean, onDone: () -> Unit) {
    var saving by remember { mutableStateOf(false) }
    var failed by remember { mutableStateOf(false) }
    val resum by vm.windsResum.collectAsState()
    LaunchedEffect(Unit) { vm.loadWindsResum() }
    ConversationPicker(lang, winds, group, onDone, resum = resum, onCreate = { name, members ->
        saving = true; failed = false
        vm.createRotllana(name, members, onError = { saving = false; failed = true }, onDone = { saving = false; onDone() })
    }, busy = saving, error = failed)
}

internal fun canCreateConversation(name: String, members: Collection<String>, group: Boolean): Boolean =
    name.isNotBlank() && members.distinct().size >= if (group) 2 else 1

/** Fila de membre amb cara, nom, rol i de què és expert (creació i edició). */
@Composable
internal fun WindMemberRow(lang: String, wind: Wind, resum: GregalApi.WindResum?,
    checked: Boolean, onToggle: (Boolean) -> Unit, faceSize: Int = 48) {
    Row(Modifier.fillMaxWidth().clickable { onToggle(!checked) }
        .padding(vertical = 8.dp), verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        WindFace(size = faceSize, accent = Color(wind.argb), style = wind.faceStyle)
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(1.dp)) {
            Text(wind.name, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
            val rol = resum?.rol?.ifBlank { null } ?: wind.role
            if (rol.isNotBlank()) Text(rol, color = Muted, fontSize = 13.sp)
            val experts = resum?.coneixements.orEmpty().map { it.titol.ifBlank { it.nom } }.filter { it.isNotBlank() }
            if (experts.isNotEmpty()) Text(tr(lang, "app.vents.knowsAbout", experts.joinToString(", ")),
                color = Muted, fontSize = 12.sp)
        }
        Checkbox(checked = checked, onCheckedChange = onToggle)
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun ConversationPicker(lang: String, winds: List<Wind>, group: Boolean, onDismiss: () -> Unit,
    resum: Map<String, GregalApi.WindResum> = emptyMap(),
    onCreate: (String, List<String>) -> Unit, busy: Boolean = false, error: Boolean = false) {
    var name by rememberSaveable { mutableStateOf("") }
    var members by rememberSaveable { mutableStateOf(setOf<String>()) }
    androidx.activity.compose.BackHandler { if (!busy) onDismiss() }
    Scaffold(containerColor = Superficie,
        topBar = { TopAppBar(
            title = { Text(tr(lang, if (group) "app.company.rotllanaNew" else "app.vents.newChat")) },
            navigationIcon = { TextButton(onClick = onDismiss, enabled = !busy) { Text(tr(lang, "app.vents.back")) } },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie)) },
        bottomBar = {
            Surface(color = Superficie) {
                Button(onClick = { onCreate(name.trim(), members.toList()) },
                    enabled = !busy && canCreateConversation(name, members, group),
                    modifier = Modifier.fillMaxWidth().navigationBarsPadding().imePadding().padding(20.dp).heightIn(min = 48.dp)) {
                    Text(tr(lang, if (busy) "app.vents.saving" else "app.vents.startChat"))
                }
            }
        }) { pad ->
        Column(Modifier.fillMaxSize().padding(pad).verticalScroll(rememberScrollState()).padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp)) {
            OutlinedTextField(name, { name = it }, singleLine = true,
                label = { Text(tr(lang, "app.vents.chatName")) }, modifier = Modifier.fillMaxWidth())
            if (error) Text(tr(lang, "app.vents.saveError"), color = MaterialTheme.colorScheme.error)
            Text(tr(lang, if (group) "app.vents.pickGroup" else "app.vents.pickChat"), color = Muted)
            if (winds.isEmpty()) Text(tr(lang, "app.vents.needWind"), color = Ink)
            winds.forEach { wind ->
                WindMemberRow(lang, wind, resum[wind.id],
                    checked = wind.id in members,
                    onToggle = { checked -> members = if (checked) members + wind.id else members - wind.id })
            }
        }
    }
}

@Composable
internal fun GroupProfileDialog(vm: GregalViewModel, lang: String, rot: Rotllana, winds: List<Wind>, onDismiss: () -> Unit) {
    var members by remember(rot.id) { mutableStateOf(rot.members.toSet()) }
    var saving by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf(false) }
    var selectedProfile by remember { mutableStateOf<Wind?>(null) }
    val minimum = if (rot.members.size > 1) 2 else 1
    AlertDialog(onDismissRequest = { if (!saving) onDismiss() }, containerColor = Superficie,
        title = { Text(rot.name) },
        text = {
            Column(Modifier.verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(tr(lang, "app.company.rotllanaMembers"), color = Muted)
                val resum by vm.windsResum.collectAsState()
                LaunchedEffect(Unit) { vm.loadWindsResum() }
                winds.forEach { wind ->
                    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Row(Modifier.weight(1f).clickable { selectedProfile = wind }.padding(vertical = 4.dp),
                            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                            WindFace(size = 40, accent = Color(wind.argb), style = wind.faceStyle)
                            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(1.dp)) {
                                Text(wind.name, color = Ink)
                                val rol = resum[wind.id]?.rol?.ifBlank { null } ?: wind.role
                                if (rol.isNotBlank()) Text(rol, color = Muted, fontSize = 13.sp)
                                val experts = resum[wind.id]?.coneixements.orEmpty()
                                    .map { it.titol.ifBlank { it.nom } }.filter { it.isNotBlank() }
                                if (experts.isNotEmpty()) Text(tr(lang, "app.vents.knowsAbout", experts.joinToString(", ")),
                                    color = Muted, fontSize = 12.sp)
                            }
                        }
                        Checkbox(checked = wind.id in members, enabled = !saving, onCheckedChange = { checked -> members = if (checked) members + wind.id else members - wind.id })
                    }
                }
                if (error) Text(tr(lang, "app.vents.saveError"), color = MaterialTheme.colorScheme.error)
            }
        },
        confirmButton = { Button(onClick = {
            saving = true; error = false
            vm.editRotllana(rot.copy(members = members.toList())) { success ->
                saving = false; if (success) onDismiss() else error = true
            }
        }, enabled = !saving && members.size >= minimum && members != rot.members.toSet()) {
            Text(tr(lang, if (saving) "app.vents.saving" else "app.vents.saveChanges"))
        } },
        dismissButton = { TextButton(onClick = onDismiss, enabled = !saving) { Text(tr(lang, "app.vents.close")) } })
    selectedProfile?.let { WindProfileDialog(vm, lang, it) { selectedProfile = null } }
}
