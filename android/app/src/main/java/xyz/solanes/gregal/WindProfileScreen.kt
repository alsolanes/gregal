package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.lazy.LazyColumn
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import kotlinx.coroutines.launch

@Composable
internal fun WindProfileDialog(vm: GregalViewModel, lang: String, wind: Wind, onDismiss: () -> Unit) {
    val profile by vm.windProfile.collectAsState()
    val busy by vm.windProfileBusy.collectAsState()
    val error by vm.windProfileError.collectAsState()
    val chats by vm.windChat.collectAsState()
    val rutines by vm.rutines.collectAsState()
    LaunchedEffect(wind.id) { vm.loadWindProfile(wind.id); vm.loadVentDetall(wind.id); vm.loadJobs(); vm.loadRutines() }
    val current = profile?.takeIf { it.wind.id == wind.id }
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        WindProfileContent(vm, lang, current?.wind ?: wind, current, busy, error, chats[wind.id].orEmpty(), rutines.orEmpty(),
            onDismiss = onDismiss,
            onRetry = { vm.loadWindProfile(wind.id) },
            onEdit = { onDismiss(); vm.showWindCreator(true, current?.wind ?: wind) },
            onSaveKnowledge = { text, done -> vm.updateWindKnowledge(wind.id, text, done) },
            onChat = { onDismiss(); vm.openVentsChat("wind:" + wind.id) })
    }
}

@OptIn(ExperimentalMaterial3Api::class, ExperimentalFoundationApi::class)
@Composable
internal fun WindProfileContent(vm: GregalViewModel, lang: String, wind: Wind, profile: WindProfile?, busy: Boolean, error: String?,
    messages: List<GregalViewModel.PetMsg>, routines: List<GregalApi.WindRutina>, onDismiss: () -> Unit, onRetry: () -> Unit, onEdit: () -> Unit,
    onSaveKnowledge: (String, (Boolean) -> Unit) -> Unit, onChat: () -> Unit = {}) {
    var tab by remember(wind.id) { mutableStateOf(0) }
    var editing by remember(wind.id) { mutableStateOf(false) }
    var draft by remember(wind.id) { mutableStateOf("") }
    var saving by remember { mutableStateOf(false) }
    var saveError by remember { mutableStateOf(false) }
    var confirmForget by remember { mutableStateOf(false) }
    var createRoutine by remember { mutableStateOf(false) }
    val accent = Color(wind.argb)
    val buttonContent = if (contrastWith(accent, Ink) >= 4.5) Ink else Superficie
    Scaffold(containerColor = Superficie, modifier = Modifier.fillMaxSize().statusBarsPadding().navigationBarsPadding(),
        bottomBar = {
            Surface(color = Superficie) {
                // Dins d'un diàleg els insets del sistema arriben a zero i el
                // botó quedava mig tapat per la barra de gestos: si l'inset no
                // diu res, deixa-hi un marge.
                val navBottom = WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding()
                Button(onClick = onChat, modifier = Modifier.fillMaxWidth()
                    .padding(start = 16.dp, end = 16.dp, top = 16.dp, bottom = 16.dp + maxOf(0.dp, 24.dp - navBottom)),
                    colors = ButtonDefaults.buttonColors(containerColor = accent, contentColor = buttonContent)) {
                    Text(tr(lang, "app.vents.talkTo"))
                }
            }
        },
        topBar = { TopAppBar(
            title = { Text(tr(lang, "app.vents.profile"), fontSize = 18.sp, fontWeight = FontWeight.SemiBold) },
            navigationIcon = { IconButton(onClick = onDismiss, enabled = !saving) { Icon(Icons.Rounded.ArrowBack, tr(lang, "app.vents.back")) } },
            actions = { IconButton(onClick = onEdit, enabled = !busy && !saving && profile != null) { Icon(Icons.Rounded.Edit, tr(lang, "app.company.windEdit")) } },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie)) }) { padding ->
        // Tot en una llista: la cara i el nom s'emporten mitja pantalla i, si
        // es queden fixes, la informació del vent no té lloc. Ara el hero
        // llisca i les pestanyes es queden enganxades a dalt.
        LazyColumn(Modifier.fillMaxSize().padding(padding)) {
            item(key = "hero") {
            Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp).padding(top = 14.dp, bottom = 10.dp),
                horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp)) {
                WindHalo(accent = accent, style = wind.faceStyle, size = 88, mood = WindMood.HAPPY, tappable = true)
                Text(wind.name, color = Ink, fontWeight = FontWeight.Bold, fontSize = 24.sp, textAlign = TextAlign.Center)
                Text(wind.role.ifBlank { tr(lang, "app.vents.noPersonality") }, color = Muted, fontSize = 14.sp,
                    lineHeight = 19.sp, textAlign = TextAlign.Center)
            }
            }
            stickyHeader(key = "tabs") {
            TabRow(selectedTabIndex = tab, containerColor = Superficie, contentColor = Ink,
                indicator = { positions ->
                    positions.getOrNull(tab)?.let { pos ->
                        with(TabRowDefaults) {
                            SecondaryIndicator(Modifier.tabIndicatorOffset(pos), height = 3.dp, color = accent)
                        }
                    }
                }) {
                listOf("character", "memory", "scheduled", "saved").forEachIndexed { index, key ->
                    Tab(selected = tab == index, onClick = { tab = index }, text = { Text(tr(lang, "app.vents.$key")) })
                }
            }
            }
            item(key = "content") {
            Column(Modifier.fillMaxWidth().padding(20.dp), verticalArrangement = Arrangement.spacedBy(14.dp)) {
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
                        HorizontalDivider(color = Muted.copy(alpha = .15f))
                        VentMemorySection(vm, lang, wind)
                        HorizontalDivider(color = Muted.copy(alpha = .15f))
                        VentSkillsSection(vm, lang, wind)
                    }
                    3 -> {
                        Text(tr(lang, "saved.help"), color = Muted, fontSize = 13.sp)
                        WindSavedGallery(vm, lang, wind.id)
                        HorizontalDivider(color = Muted.copy(alpha = .15f))
                        WindCardsSection(vm, lang, wind)
                    }
                    2 -> {
                        Text(tr(lang, "app.vents.plansHelp"), color = Muted, fontSize = 13.sp)
                        Button(onClick = { createRoutine = true }, modifier = Modifier.fillMaxWidth(), colors = ButtonDefaults.buttonColors(containerColor = Ink, contentColor = Superficie)) {
                            Text(if (lang == "en") "Create routine" else "Crea una rutina")
                        }
                        bgHealthRow(lang, vm)
                        heartbeatRow(lang, vm, wind)
                        followupsRow(lang, vm, wind)
                        WindAutomationList(vm, lang, wind.id, showSummary = false)
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
    if (createRoutine) NovaRutinaDialog(vm, lang, preselectedWind = wind.id, onDismiss = { createRoutine = false })
}

/** Salut del segon pla (mode local): avisa si falta permís d'alarmes
 *  exactes o de notificacions (llavors els avisos arriben tard o mai),
 *  i deixa provar el so a l'instant. */
@Composable
private fun bgHealthRow(lang: String, vm: GregalViewModel) {
    val healthState by vm.bgHealth.collectAsState()
    val health = healthState ?: return
    val ctx = androidx.compose.ui.platform.LocalContext.current
    if (!health.exactAlarms) Surface(shape = RoundedCornerShape(14.dp),
        color = Red.copy(alpha = 0.12f),
        border = androidx.compose.foundation.BorderStroke(1.dp, Red.copy(alpha = 0.5f))) {
        Column(Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(tr(lang, "app.vents.exactAlarmNeeded"), color = Ink, fontSize = 13.sp)
            OutlinedButton(onClick = { ctx.startActivity(JobAlarms.exactAlarmSettings(ctx)) }) {
                Text(tr(lang, "app.vents.exactAlarmAllow"))
            }
        }
    }
    if (!health.notifications) Surface(shape = RoundedCornerShape(14.dp),
        color = Red.copy(alpha = 0.12f),
        border = androidx.compose.foundation.BorderStroke(1.dp, Red.copy(alpha = 0.5f))) {
        Column(Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(tr(lang, "app.vents.notifNeeded"), color = Ink, fontSize = 13.sp)
            OutlinedButton(onClick = { ctx.startActivity(JobAlarms.notificationSettings(ctx)) }) {
                Text(tr(lang, "app.vents.notifAllow"))
            }
        }
    }
    if (!health.unrestrictedBattery) Surface(shape = RoundedCornerShape(14.dp),
        color = Red.copy(alpha = 0.12f),
        border = androidx.compose.foundation.BorderStroke(1.dp, Red.copy(alpha = 0.5f))) {
        Column(Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(tr(lang, "app.vents.batteryNeeded"), color = Ink, fontSize = 13.sp)
            OutlinedButton(onClick = { ctx.startActivity(JobAlarms.batterySettings(ctx)) }) {
                Text(tr(lang, "app.vents.batteryAllow"))
            }
        }
    }
    OutlinedButton(onClick = { JobNotifications.test(ctx) }) { Text(tr(lang, "app.vents.testNotif")) }
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
                vm.ensureWindHeartbeat(wind, on) { pending = false }
            }
        }, enabled = !pending)
    }
}

/** El que el vent recorda de tu (memòria pròpia, editable). */
@Composable
private fun VentMemorySection(vm: GregalViewModel, lang: String, wind: Wind) {
    val detall by vm.ventDetall.collectAsState()
    val (mem, _) = detall[wind.id] ?: ("" to emptyList<GregalApi.Coneixement>())
    var editing by remember(wind.id) { mutableStateOf(false) }
    var draft by remember(wind.id) { mutableStateOf("") }
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(tr(lang, "app.vents.memoryTitle"), color = Ink, fontWeight = FontWeight.SemiBold)
        if (editing) {
            var saving by remember(wind.id) { mutableStateOf(false) }
            OutlinedTextField(draft, { draft = it }, enabled = !saving, minLines = 4, modifier = Modifier.fillMaxWidth())
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(onClick = { saving = true; vm.saveVentMemoria(wind.id, draft) { ok -> saving = false; if (ok) editing = false } }, enabled = !saving) { Text(tr(lang, if (saving) "app.vents.saving" else "app.vents.chatSave")) }
                TextButton(onClick = { editing = false }, enabled = !saving) { Text(tr(lang, "app.vents.chatCancel")) }
            }
        } else {
            SelectionContainer {
                Text(mem.ifBlank { tr(lang, "app.vents.memoryEmpty") }, color = Tinta, fontSize = 14.sp, lineHeight = 22.sp)
            }
            OutlinedButton(onClick = { draft = mem; editing = true }) { Text(tr(lang, "app.vents.memoryEdit")) }
        }
    }
}

/** Estudi «fes-te expert»: el vent busca informació del seu tema i la deixa
 *  com a coneixement. La cara s'anima segons la fase (cercant → llegint →
 *  escrivint) i la barra mostra com en va. */
@Composable
private fun expertRow(vm: GregalViewModel, lang: String, wind: Wind) {
    val expert by vm.windExpert.collectAsState()
    val est = expert[wind.id]
    LaunchedEffect(wind.id) { vm.loadWindExpertOnce(wind.id) }
    Surface(shape = RoundedCornerShape(14.dp), color = Color(wind.argb).copy(alpha = 0.07f),
        border = androidx.compose.foundation.BorderStroke(1.dp, Color(wind.argb).copy(alpha = 0.30f)),
        modifier = Modifier.fillMaxWidth()) {
        if (est != null && est.actiu) {
            Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    WindFace(size = 44, accent = Color(wind.argb), style = wind.faceStyle,
                        mood = when (est.fase) {
                            "lectura" -> WindMood.READING
                            "apren" -> WindMood.WRITING
                            else -> WindMood.SEARCHING
                        })
                    Column(Modifier.weight(1f)) {
                        Text(tr(lang, "app.vents.expertStudying"), color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp)
                        Text(tr(lang, "app.vents.expertStudyingHelp", est.tema.ifBlank { wind.role }), color = Muted, fontSize = 12.sp)
                    }
                }
                LinearProgressIndicator(progress = { est.progres }, modifier = Modifier.fillMaxWidth(),
                    color = Color(wind.argb), trackColor = Color(wind.argb).copy(alpha = 0.15f))
            }
        } else if (est != null && est.exit) {
            Row(Modifier.fillMaxWidth().padding(12.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                WindFace(size = 44, accent = Color(wind.argb), style = wind.faceStyle, mood = WindMood.DONE)
                Column(Modifier.weight(1f)) {
                    Text(tr(lang, "app.vents.expertDone"), color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp)
                    Text(tr(lang, "app.vents.expertDoneHelp", est.tema.ifBlank { wind.role }), color = Muted, fontSize = 12.sp)
                }
                TextButton(onClick = { vm.startWindExpert(wind.id, wind.role) }) { Text(tr(lang, "app.vents.expertAgain")) }
            }
        } else {
            Row(Modifier.fillMaxWidth().padding(12.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                WindFace(size = 44, accent = Color(wind.argb), style = wind.faceStyle, mood = WindMood.IDLE)
                Column(Modifier.weight(1f)) {
                    Text(tr(lang, "app.vents.expertTitle"), color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp)
                    Text(tr(lang, "app.vents.expertHelp", wind.role), color = Muted, fontSize = 12.sp)
                }
                Button(onClick = { vm.startWindExpert(wind.id, wind.role) }, colors = ButtonDefaults.buttonColors(containerColor = Color(wind.argb))) {
                    Text(tr(lang, "app.vents.expertGo"))
                }
            }
        }
    }
}

@Composable
private fun VentSkillsSection(vm: GregalViewModel, lang: String, wind: Wind) {
    val detall by vm.ventDetall.collectAsState()
    val (_, coneix) = detall[wind.id] ?: ("" to emptyList())
    var editor by remember(wind.id) { mutableStateOf<GregalApi.Coneixement?>(null) }
    var creating by remember(wind.id) { mutableStateOf(false) }
    var confirmDel by remember(wind.id) { mutableStateOf<GregalApi.Coneixement?>(null) }
    expertRow(vm, lang, wind)
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
            Text(tr(lang, "app.vents.skillsTitle"), color = Ink, fontWeight = FontWeight.SemiBold)
            TextButton(onClick = { creating = true }) { Text("+ " + tr(lang, "app.vents.skillsNew")) }
        }
        if (coneix.isEmpty()) Text(tr(lang, "app.vents.skillsEmpty"), color = Muted, fontSize = 13.sp)
        coneix.forEach { c ->
            Surface(shape = RoundedCornerShape(14.dp), color = Color(wind.argb).copy(alpha = 0.07f),
                border = androidx.compose.foundation.BorderStroke(1.dp, Color(wind.argb).copy(alpha = 0.30f)),
                modifier = Modifier.fillMaxWidth().clickable { editor = c }) {
                Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(if (c.titol.isNotBlank()) c.titol else c.nom, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp)
                    if (c.descripcio.isNotBlank()) Text(c.descripcio, color = Muted, fontSize = 13.sp)
                }
            }
        }
    }
    if (creating) ConeixementEditor(vm, lang, wind, null, onClose = { creating = false })
    editor?.let { c -> ConeixementEditor(vm, lang, wind, c, onClose = { editor = null }, onDelete = { confirmDel = c; editor = null }) }
    confirmDel?.let { c ->
        AlertDialog(onDismissRequest = { confirmDel = null },
            title = { Text(tr(lang, "app.vents.skillDelete")) },
            text = { Text(tr(lang, "app.vents.skillConfirmDelete", c.nom)) },
            confirmButton = { TextButton(onClick = { vm.deleteConeixement(wind.id, c.nom); confirmDel = null }) {
                Text(tr(lang, "app.vents.chatDelete"), color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton(onClick = { confirmDel = null }) { Text(tr(lang, "app.vents.chatCancel")) } })
    }
}

/** Editor d'un coneixement: nom + descripció (índex) + cos (markdown). */
@Composable
private fun ConeixementEditor(vm: GregalViewModel, lang: String, wind: Wind, existing: GregalApi.Coneixement?, onClose: () -> Unit, onDelete: () -> Unit = {}) {
    var nom by remember { mutableStateOf(existing?.nom.orEmpty()) }
    var titol by remember { mutableStateOf(existing?.titol.orEmpty()) }
    var desc by remember { mutableStateOf(existing?.descripcio.orEmpty()) }
    var cos by remember { mutableStateOf(existing?.cos.orEmpty()) }
    var loading by remember { mutableStateOf(existing != null && existing.cos.isBlank()) }
    LaunchedEffect(existing?.nom) {
        if (existing != null && existing.cos.isBlank()) {
            try {
                val ple = vm.apiConeixement(wind.id, existing.nom)
                titol = ple.titol; desc = ple.descripcio; cos = ple.cos
            } catch (_: Exception) { }
            loading = false
        }
    }
    Dialog(onDismissRequest = onClose) {
        Surface(shape = RoundedCornerShape(20.dp), color = Superficie, shadowElevation = 6.dp) {
            Column(Modifier.padding(20.dp).verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                Text(if (existing == null) tr(lang, "app.vents.skillsNew") else existing.nom, fontWeight = FontWeight.Bold, fontSize = 17.sp, color = Ink)
                if (loading) {
                    CircularProgressIndicator()
                } else {
                    if (existing == null) OutlinedTextField(nom, { nom = it }, label = { Text(tr(lang, "app.vents.skillName")) }, singleLine = true, modifier = Modifier.fillMaxWidth())
                    OutlinedTextField(titol, { titol = it }, label = { Text(tr(lang, "app.vents.skillTitle")) }, singleLine = true, modifier = Modifier.fillMaxWidth())
                    OutlinedTextField(desc, { desc = it }, label = { Text(tr(lang, "app.vents.skillDesc")) }, modifier = Modifier.fillMaxWidth())
                    OutlinedTextField(cos, { cos = it }, label = { Text(tr(lang, "app.vents.skillBody")) }, minLines = 5, modifier = Modifier.fillMaxWidth())
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End, verticalAlignment = Alignment.CenterVertically) {
                        if (existing != null) TextButton(onClick = onDelete) {
                            Text(tr(lang, "app.vents.chatDelete"), color = MaterialTheme.colorScheme.error)
                        }
                        TextButton(onClick = onClose) { Text(tr(lang, "app.vents.chatCancel")) }
                        Button(onClick = {
                            vm.saveConeixement(wind.id, GregalApi.Coneixement(nom.ifBlank { existing?.nom.orEmpty() }, titol, desc, cos)) { onClose() }
                        }, enabled = desc.isNotBlank() && cos.isNotBlank()) { Text(tr(lang, "app.vents.chatSave")) }
                    }
                }
            }
        }
    }
}


@Composable
private fun WindCardsSection(vm: GregalViewModel, lang: String, wind: Wind) {
    val scope = rememberCoroutineScope()
    var cards by remember(wind.id) { mutableStateOf<List<org.json.JSONObject>>(emptyList()) }
    var title by remember(wind.id) { mutableStateOf("") }; var prompt by remember(wind.id) { mutableStateOf("") }
    var loading by remember(wind.id) { mutableStateOf(false) }; var error by remember(wind.id) { mutableStateOf("") }
    val ca = lang != "en"
    suspend fun load() { val data = vm.windCards(wind.id).optJSONArray("cards") ?: org.json.JSONArray(); cards = (0 until data.length()).map { data.getJSONObject(it) } }
    LaunchedEffect(wind.id) { try { load() } catch (e: Exception) { error = e.message.orEmpty() } }
    Text(if (ca) "Fitxes consultables" else "Reference cards", fontWeight = FontWeight.SemiBold, color = Ink)
    Text(if (ca) "Informació per consultar i actualitzar." else "Information to check and refresh.", color = Muted)
    if (error.isNotBlank()) Text(error, color = MaterialTheme.colorScheme.error)
    cards.forEach { card ->
        Surface(shape = RoundedCornerShape(14.dp), color = Color(wind.argb).copy(alpha = .08f), modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(card.optString("title"), color = Ink, fontWeight = FontWeight.SemiBold)
                Text(card.optString("updated").ifBlank { if (ca) "Encara no actualitzada" else "Not updated yet" }, color = Muted, fontSize = 12.sp)
                SelectionContainer { Text(card.optString("content").ifBlank { if (ca) "Prem Actualitza per cercar informació." else "Press Refresh to research this topic." }, color = Tinta) }
                OutlinedButton(enabled = !loading, onClick = { scope.launch { loading = true; error = ""; try { vm.windCardAction(wind.id, "refresh", id = card.optString("id")); load() } catch (e: Exception) { error = e.message.orEmpty() } finally { loading = false } } }) { Text(if (loading) { if (ca) "Actualitzant…" else "Refreshing…" } else { if (ca) "Actualitza" else "Refresh" }) }
            }
        }
    }
    OutlinedTextField(title, { title = it }, label = { Text(if (ca) "Nom de la fitxa" else "Card title") }, modifier = Modifier.fillMaxWidth())
    OutlinedTextField(prompt, { prompt = it }, label = { Text(if (ca) "Què ha de cercar?" else "What should be researched?") }, modifier = Modifier.fillMaxWidth(), minLines = 2)
    Button(enabled = !loading && title.isNotBlank() && prompt.isNotBlank(), onClick = { scope.launch { loading = true; error = ""; try { vm.windCardAction(wind.id, "create", title = title, prompt = prompt); title = ""; prompt = ""; load() } catch (e: Exception) { error = e.message.orEmpty() } finally { loading = false } } }) { Text(if (ca) "Crea la fitxa" else "Create card") }
}

/** Interruptor dels seguiments proactius: el vent s'ofereix a tornar-hi. */
@Composable
private fun followupsRow(lang: String, vm: GregalViewModel, wind: Wind) {
    val winds by vm.winds.collectAsState()
    val on = (winds.firstOrNull { it.id == wind.id } ?: wind).followups != "off"
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) {
            Text(tr(lang, "act.followups"), color = Ink, fontWeight = FontWeight.SemiBold)
            Text(tr(lang, "act.followupsHelp"), color = Muted, fontSize = 13.sp, lineHeight = 18.sp)
        }
        Switch(checked = on, onCheckedChange = { vm.setWindFollowups(wind, it) })
    }
}
