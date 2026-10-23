package xyz.solanes.gregal

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.ui.draw.drawBehind
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.Delete
import androidx.compose.material.icons.rounded.Groups
import androidx.compose.material.icons.rounded.Info
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.Edit
import androidx.compose.material.icons.rounded.Menu
import androidx.compose.material.icons.rounded.NotificationsActive
import androidx.compose.material.icons.rounded.Send
import androidx.compose.material.icons.rounded.TravelExplore
import androidx.compose.material.icons.rounded.ArrowBack
import androidx.compose.material.icons.rounded.Construction
import androidx.compose.material.icons.rounded.Create
import androidx.compose.material.icons.rounded.PersonAddAlt
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.text.ClickableText
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.ui.layout.onSizeChanged
import kotlinx.coroutines.launch
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.sp

/**
 * CompanyScreen — el mode company de Gregal: els teus vents.
 *
 * Els vents (companions amb nom de vent) al centre: cara viva del seu color,
 * xat amb context de projecte, i al voltant la resta del company (proposta
 * de memòria, check-in, corral). La feina proactiva es programa a Feines.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CompanyScreen(vm: GregalViewModel, openDrawer: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    Box(Modifier.fillMaxSize()) {
        VentsHome(vm, lang, openDrawer) { }
        Box(Modifier.align(Alignment.BottomCenter).imePadding().padding(start = 16.dp, end = 16.dp, bottom = 88.dp)) {
            NoticeHost(vm)
        }
    }
}

/**
 * VentsHome: l'app de xats dels vents.
 *
 * Llista de converses (vents i rotllanes) a l'estil Telegram + xat obert
 * a pantalla completa. Els vents i els seus xats deixen de ser una secció
 * dins d'una altra pantalla: són EL producte.
 */
@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
fun VentsHome(vm: GregalViewModel, lang: String, openDrawer: () -> Unit, openPet: (Pet?) -> Unit) {
    val winds by vm.winds.collectAsState()
    val rotllanes by vm.rotllanes.collectAsState()
    val chats by vm.windChat.collectAsState()
    val rChats by vm.rotllanaChat.collectAsState()
    val busy by vm.windBusy.collectAsState()
    val rBusy by vm.rotllanaBusy.collectAsState()
    val live by vm.windLive.collectAsState()
    val news by vm.windNews.collectAsState()
    val conn by vm.conn.collectAsState()
    // Veus proactives per obrir: es consulten en obrir els Vents i cada minut.
    LaunchedEffect(conn) { while (true) { vm.refreshWindNews(); kotlinx.coroutines.delay(60_000) } }
    val ventJobs by vm.jobs.collectAsState()
    // Les notificacions es demanen soles quan hi ha avisos (13+): sense el
    // diàleg del sistema el permís queda denegat i no sona mai res.
    val notifCtx = androidx.compose.ui.platform.LocalContext.current
    var notifAsked by remember { mutableStateOf(false) }
    val notifLauncher = androidx.activity.compose.rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.RequestPermission()
    ) { vm.loadJobs() }
    LaunchedEffect(ventJobs) {
        if (android.os.Build.VERSION.SDK_INT >= 33 && !notifAsked &&
            ventJobs.any { it.enabled && (it.kind == "event" || it.kind == "remind") } &&
            androidx.core.content.ContextCompat.checkSelfPermission(
                notifCtx, android.Manifest.permission.POST_NOTIFICATIONS
            ) != android.content.pm.PackageManager.PERMISSION_GRANTED
        ) {
            notifAsked = true
            notifLauncher.launch(android.Manifest.permission.POST_NOTIFICATIONS)
        }
    }
    val open by vm.ventsOpen.collectAsState()
    var creation by remember { mutableStateOf<String?>(null) }
    var menu by remember { mutableStateOf(false) }
    var manage by remember { mutableStateOf(false) }
    var profile by remember { mutableStateOf<Wind?>(null) }
    var rowTarget by remember { mutableStateOf<RowTarget?>(null) }
    val creator by vm.windCreator.collectAsState()
    LaunchedEffect(Unit) { vm.loadWinds(); vm.loadRotllanes() }
    LaunchedEffect(winds.map { it.id }, rotllanes.map { it.id }) {
        winds.forEach { vm.loadWindChat(it.id) }
        rotllanes.forEach { vm.loadRotllanaChat(it.id) }
    }
    androidx.activity.compose.BackHandler(enabled = manage && open == null) { manage = false }
    if (creator.first) {
        WindCreatorScreen(vm, lang, creator.second)
    } else if (creation != null) {
        ConversationCreator(vm, lang, winds, group = creation == "group") { creation = null }
    } else if (open != null) {
        val id = open!!.substringAfter(':')
        val wind = if (open!!.startsWith("wind:")) winds.firstOrNull { it.id == id } else null
        val rot = if (open!!.startsWith("rot:")) rotllanes.firstOrNull { it.id == id } else null
        when {
            wind != null -> WindConversation(vm, lang, wind) { vm.openVentsChat(null) }
            rot != null -> RotllanaConversation(vm, lang, rot, winds) { vm.openVentsChat(null) }
            else -> LaunchedEffect(open) { vm.openVentsChat(null) }
        }
    } else {
        Scaffold(containerColor = Superficie,
            topBar = { TopAppBar(
                navigationIcon = { IconButton(onClick = openDrawer) { Icon(Icons.Rounded.Menu, tr(lang, "app.nav.menu")) } },
                title = { Text(tr(lang, "app.vents.title"), fontWeight = FontWeight.Bold) },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie)) },
            floatingActionButton = {
                Box {
                    FloatingActionButton(onClick = { menu = true }, containerColor = Sea, contentColor = OnSea) {
                        Icon(Icons.Rounded.Add, tr(lang, "app.vents.create"))
                    }
                    DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                        DropdownMenuItem(text = { Text(tr(lang, "app.vents.newChat")) }, onClick = { menu = false; creation = "chat" })
                        DropdownMenuItem(text = { Text(tr(lang, "app.company.rotllanaNew")) }, onClick = { menu = false; creation = "group" })
                        DropdownMenuItem(text = { Text(tr(lang, "app.company.windNew")) }, onClick = { menu = false; vm.showWindCreator(true) })
                    }
                }
            }) { pad ->
            Column(Modifier.fillMaxSize().padding(pad)) {
                VentsHero(lang, winds, tr(lang, if (conn.local) "app.vents.localMode" else "app.vents.serverMode", conn.host))
                VentsPillTabs(lang, manage, onManage = { manage = it })
                LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = 96.dp, top = 12.dp)) {
                    if (!manage && winds.isNotEmpty()) item {
                        VentsGrid(lang, winds, busy, live, news) { vm.openVentsChat("wind:" + it.id) }
                    }
                    if (manage) {
                        item { WindsModelRow(vm, lang) }
                        item { SquadsRow(vm, lang) }
                        item { Text(tr(lang, "app.vents.manageHelp"), color = Muted, fontSize = 13.sp, modifier = Modifier.padding(horizontal = 20.dp, vertical = 12.dp)) }
                        if (winds.isEmpty()) item { VentsEmpty(lang, false) }
                        items(winds, key = { it.id }) { w ->
                            ConversationRow(w.name, w.role, Color(w.argb), w.faceStyle, onClick = { profile = w })
                        }
                    } else {
                        if (rotllanes.isEmpty() && winds.none { chats[it.id].orEmpty().isNotEmpty() }) item { VentsEmpty(lang, true) }
                        items(winds.filter { chats[it.id].orEmpty().isNotEmpty() }, key = { "wind:" + it.id }) { w ->
                            ConversationRow(w.name, chats[w.id].orEmpty().lastOrNull()?.text.orEmpty(), Color(w.argb), w.faceStyle,
                                busy = busy.contains(w.id), mood = if (busy.contains(w.id)) live[w.id]?.mood else if (news.contains(w.id)) WindMood.NEWS else null,
                                onClick = { vm.openVentsChat("wind:" + w.id) },
                                onLongClick = { rowTarget = RowTarget.Dm(w) })
                        }
                        items(rotllanes, key = { "rot:" + it.id }) { r ->
                            val faces = r.members.mapNotNull { mid -> winds.firstOrNull { it.id == mid } }
                            ConversationRow(r.name, rChats[r.id].orEmpty().lastOrNull()?.text ?: faces.joinToString(" · ") { it.name },
                                faces.firstOrNull()?.let { Color(it.argb) } ?: Sea,
                                faces.firstOrNull()?.faceStyle ?: WindStyle(), faces = faces,
                                busy = rBusy.contains(r.id), mood = if (rBusy.contains(r.id)) live[r.id]?.mood else null,
                                onClick = { vm.openVentsChat("rot:" + r.id) },
                                onLongClick = { rowTarget = RowTarget.Rot(r) })
                        }
                    }
                }
            }
        }
    }
    profile?.let { w -> WindProfileDialog(vm, lang, w) { profile = null } }
    rowTarget?.let { t ->
        ChatRowDialog(vm, lang, t) { rowTarget = null }
    }
}

/** Graella de mascotes (estil Dots): cada vent en gran amb el seu estat. */
@Composable
internal fun VentsGrid(
    lang: String, winds: List<Wind>, busy: Set<String>,
    live: Map<String, WindLive> = emptyMap(), news: Set<String> = emptySet(), onOpen: (Wind) -> Unit,
) {
    @OptIn(ExperimentalLayoutApi::class)
    FlowRow(
        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
        maxItemsInEachRow = 2,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        winds.forEach { w ->
            val thinking = busy.contains(w.id)
            val mood = when {
                thinking -> live[w.id]?.mood ?: WindMood.THINKING
                news.contains(w.id) -> WindMood.NEWS
                else -> WindMood.HAPPY
            }
            val accent = Color(w.argb)
            Column(
                Modifier.weight(1f).clip(RoundedCornerShape(18.dp))
                    // Fons tenyit amb el color propi del vent, com les targetes wind-desk.
                    .background(Brush.linearGradient(
                        listOf(accent.copy(alpha = 0.13f), Superficie)
                    ))
                    // Ombra dura de 3 px del color del vent, sota la targeta.
                    .drawBehind {
                        drawRoundRect(
                            color = accent.copy(alpha = 0.13f),
                            topLeft = Offset(0f, size.height - 3.dp.toPx()),
                            size = Size(size.width, 3.dp.toPx()),
                            cornerRadius = CornerRadius(18.dp.toPx() / 2),
                        )
                    }
                    .clickable { onOpen(w) }
                    .semantics { contentDescription = w.name }
                    .padding(vertical = 14.dp, horizontal = 8.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                WindHalo(
                    accent = accent, style = w.faceStyle, size = 88,
                    mood = mood,
                )
                Text(
                    w.name, fontSize = 16.sp, fontWeight = FontWeight.Bold, color = Ink, maxLines = 1,
                    overflow = TextOverflow.Ellipsis, textAlign = TextAlign.Center,
                )
                // L'estat sempre amb paraules, no només amb la insígnia.
                Text(
                    when (mood) {
                        WindMood.SEARCHING -> tr(lang, "app.vents.gridSearching")
                        WindMood.WRITING -> tr(lang, "app.vents.gridWriting")
                        WindMood.WORKING -> tr(lang, "app.vents.gridWorking")
                        WindMood.DISCUSSING -> tr(lang, "app.vents.gridDiscussing")
                        WindMood.NEWS -> tr(lang, "app.vents.gridNews")
                        else -> if (thinking) tr(lang, "app.vents.gridThinking") else w.role.ifBlank { tr(lang, "app.vents.gridOnline") }
                    },
                    fontSize = 12.sp, color = if (mood == WindMood.NEWS) Ink else Muted, maxLines = 1,
                    overflow = TextOverflow.Ellipsis, textAlign = TextAlign.Center,
                )
            }
        }
    }
}
                    // Graella de mascotes (estil Dots): cada vent en gran amb el
                    // seu estat; la reina de l'inici.
@Composable
private fun VentsEmpty(lang: String, conversations: Boolean) {
    Column(Modifier.fillMaxWidth().padding(horizontal = 32.dp, vertical = 30.dp),
        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(12.dp)) {
        // Trio de cares flotants: el buit també somriu.
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(2.dp)) {
            Box(Modifier.offset(x = 12.dp)) { WindHalo(Color(0xFF8FB7FF), windStyleOf("esn"), size = 52) }
            Box(Modifier.offset(y = (-10).dp)) { WindHalo(Color(0xFFF3BD8B), windStyleOf("bsn"), size = 72) }
            Box(Modifier.offset(x = (-12).dp)) { WindHalo(Color(0xFF9BE3C3), windStyleOf("pso"), size = 52) }
        }
        Text(tr(lang, if (conversations) "app.vents.emptyTitle" else "app.company.windEmptyTitle"), color = Ink,
            fontSize = 21.sp, fontWeight = FontWeight.ExtraBold, textAlign = TextAlign.Center)
        Text(tr(lang, if (conversations) "app.vents.emptyBody" else "app.company.windEmptyBody"), color = Muted,
            fontSize = 14.sp, textAlign = TextAlign.Center)
    }
}

/** Capçalera de secció a la llista de converses. */
@Composable
private fun SectionHeader(text: String) {
    Text(text, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = Faint,
        modifier = Modifier.fillMaxWidth().padding(start = 20.dp, top = 10.dp, bottom = 2.dp))
}

/** Colles predefinides: targeta que activa els especialistes que falten. */
@Composable
private fun SquadsRow(vm: GregalViewModel, lang: String) {
    val squads by vm.squads.collectAsState()
    val winds by vm.winds.collectAsState()
    var expanded by remember { mutableStateOf(false) }
    var activating by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(Unit) { vm.loadSquads() }
    if (squads.isEmpty()) return
    Surface(
        shape = RoundedCornerShape(20.dp),
        color = Sea.copy(alpha = 0.08f),
        border = androidx.compose.foundation.BorderStroke(1.dp, Sea.copy(alpha = 0.30f)),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 5.dp)
            .clip(RoundedCornerShape(20.dp)).clickable { expanded = !expanded },
    ) {
        Column(Modifier.padding(horizontal = 12.dp, vertical = 10.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                TintedIcon(Icons.Rounded.Groups, Sea)
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(tr(lang, "app.vents.squads"), fontSize = 15.sp, fontWeight = FontWeight.Bold, color = Ink)
                    Text(tr(lang, "app.vents.squadsHelp"), fontSize = 13.sp, color = Muted, maxLines = 2, overflow = TextOverflow.Ellipsis)
                }
                Text(if (expanded) "▾" else "▸", color = Muted)
            }
            if (expanded) {
                squads.forEach { s ->
                    val have = s.members.count { m -> winds.any { w -> w.specialty == m.key && w.id.startsWith("w-") } }
                    val done = have == s.members.size
                    Row(
                        Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp))
                            .background(Superficie.copy(alpha = 0.7f))
                            .clickable(enabled = !done && activating == null) {
                                activating = s.id
                                vm.activateSquad(s.id, lang)
                            }
                            .padding(horizontal = 10.dp, vertical = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                    ) {
                        // Mini preview de les cares dels membres.
                        Row(horizontalArrangement = Arrangement.spacedBy((-10).dp)) {
                            s.members.take(5).forEach { m ->
                                WindHalo(accent = Color(windArgb(m.color)), style = windStyleOf(m.style).copy(kit = m.key), size = 34, mood = WindMood.HAPPY)
                            }
                        }
                        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(1.dp)) {
                            Text(if (lang == "ca") s.ca else s.en, fontSize = 14.sp, fontWeight = FontWeight.Bold, color = Ink)
                            Text(
                                if (done) tr(lang, "app.vents.squadDone", s.members.size)
                                else tr(lang, "app.vents.squadCount", have, s.members.size),
                                fontSize = 12.sp, color = Muted,
                            )
                        }
                        if (done) Text("✓", color = Sea, fontSize = 16.sp, fontWeight = FontWeight.Bold)
                        else if (activating == s.id) TypingDots(Sea)
                    }
                }
            }
        }
    }
}

/** ARGB d'un color hex del servidor (#RRGGBB). */
private fun windArgb(hex: String): Long = runCatching {
    val v = hex.removePrefix("#")
    (0xFF000000L or v.toLong(16))
}.getOrDefault(0xFFBFCCA8L)

/** Model per defecte dels vents: fila + diàleg amb el catàleg del servidor. */
@Composable
private fun WindsModelRow(vm: GregalViewModel, lang: String) {
    val windsModel by vm.windsModel.collectAsState()
    var picker by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) { vm.loadModels() }
    Surface(
        shape = RoundedCornerShape(20.dp),
        color = Sea.copy(alpha = 0.08f),
        border = androidx.compose.foundation.BorderStroke(1.dp, Sea.copy(alpha = 0.30f)),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 5.dp)
            .clip(RoundedCornerShape(20.dp)).clickable(onClick = { picker = true }),
    ) {
        Row(
            Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            TintedIcon(Icons.Rounded.AutoAwesome, Sea)
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(tr(lang, "app.vents.defaultModel"), fontSize = 15.sp, fontWeight = FontWeight.Bold, color = Ink)
                Text(
                    windsModel.ifBlank { tr(lang, "app.vents.defaultModelSameAsWork") },
                    fontSize = 13.sp, color = Muted, maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
    if (picker) WindsModelDialog(vm, lang) { picker = false }
}

@Composable
private fun WindsModelDialog(vm: GregalViewModel, lang: String, onDismiss: () -> Unit) {
    val providers by vm.providers.collectAsState()
    val models by vm.models.collectAsState()
    var query by remember { mutableStateOf("") }
    LaunchedEffect(Unit) { vm.loadProviders(); vm.loadModels() }
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(shape = RoundedCornerShape(20.dp), color = Superficie, shadowElevation = 8.dp) {
            Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(tr(lang, "app.vents.pickWindsModel"), fontSize = 17.sp, fontWeight = FontWeight.Bold, color = Ink)
                OutlinedTextField(
                    value = query, onValueChange = { query = it }, singleLine = true,
                    placeholder = { Text(tr(lang, "app.picker.search"), fontSize = 14.sp) },
                    shape = RoundedCornerShape(14.dp), modifier = Modifier.fillMaxWidth(),
                )
                val groups = configuredModelChoices(providers, models, query)
                LazyColumn(Modifier.fillMaxWidth().heightIn(max = 320.dp)) {
                    if (groups.isEmpty()) item {
                        Text(tr(lang, "app.picker.noModels"), Modifier.padding(12.dp), color = Muted)
                    }
                    groups.forEach { (provider, choices) ->
                        item(key = "provider:$provider") {
                            Text(provider, Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
                                color = Muted, fontWeight = FontWeight.SemiBold, fontSize = 12.sp)
                        }
                        items(choices, key = { it.ref }) { c ->
                            Row(
                                Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp))
                                    .clickable(onClick = { vm.setWindsModel(c.ref); onDismiss() })
                                    .padding(horizontal = 12.dp, vertical = 10.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                            ) {
                                Text(c.model, color = Ink, fontSize = 15.sp, modifier = Modifier.weight(1f))
                            }
                        }
                    }
                }
            }
        }
    }
}

/** Quina fila de la llista s'està editant: DM amb un vent o rotllana. */
private sealed interface RowTarget {
    data class Dm(val wind: Wind) : RowTarget
    data class Rot(val rot: Rotllana) : RowTarget
}

/** Accions d'una fila de la llista: canviar el nom o esborrar el xat. */
@Composable
private fun ChatRowDialog(vm: GregalViewModel, lang: String, target: RowTarget, onClose: () -> Unit) {
    var mode by remember(target) { mutableStateOf(0) } // 0 menú, 1 nom, 2 confirmar
    val currentName = when (target) {
        is RowTarget.Dm -> target.wind.name
        is RowTarget.Rot -> target.rot.name
    }
    var name by remember(target) { mutableStateOf(currentName) }
    val deleteLabel = tr(lang, if (target is RowTarget.Dm) "app.vents.chatClear" else "app.vents.chatDeleteRot")
    val confirmText = tr(
        lang,
        if (target is RowTarget.Dm) "app.vents.chatConfirmClear" else "app.vents.chatConfirmDeleteRot",
        currentName,
    )
    androidx.compose.ui.window.Dialog(onDismissRequest = onClose) {
        Surface(shape = RoundedCornerShape(20.dp), color = Superficie, shadowElevation = 6.dp) {
            Column(Modifier.padding(horizontal = 20.dp, vertical = 16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(currentName, fontSize = 17.sp, fontWeight = FontWeight.ExtraBold, color = Ink,
                    maxLines = 1, overflow = TextOverflow.Ellipsis)
                when (mode) {
                    1 -> {
                        OutlinedTextField(
                            value = name, onValueChange = { name = it },
                            label = { Text(tr(lang, "app.vents.chatNameLabel")) },
                            singleLine = true, shape = RoundedCornerShape(14.dp),
                            modifier = Modifier.fillMaxWidth(),
                        )
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                            TextButton(onClick = onClose) { Text(tr(lang, "app.vents.chatCancel")) }
                            TextButton(onClick = {
                                when (target) {
                                    is RowTarget.Dm -> vm.renameWind(target.wind, name)
                                    is RowTarget.Rot -> vm.renameRotllana(target.rot, name)
                                }
                                onClose()
                            }) { Text(tr(lang, "app.vents.chatSave")) }
                        }
                    }
                    2 -> {
                        Text(confirmText, fontSize = 14.sp, color = Tinta)
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                            TextButton(onClick = onClose) { Text(tr(lang, "app.vents.chatCancel")) }
                            TextButton(onClick = {
                                when (target) {
                                    is RowTarget.Dm -> vm.clearWindChat(target.wind.id)
                                    is RowTarget.Rot -> vm.deleteRotllana(target.rot.id)
                                }
                                onClose()
                            }) { Text(tr(lang, "app.vents.chatDelete"), color = MaterialTheme.colorScheme.error) }
                        }
                    }
                    else -> {
                        TextButton(
                            onClick = { mode = 1 },
                            modifier = Modifier.fillMaxWidth(),
                            contentPadding = PaddingValues(vertical = 10.dp),
                        ) { Text(tr(lang, "app.vents.chatRename"), modifier = Modifier.fillMaxWidth()) }
                        TextButton(
                            onClick = { mode = 2 },
                            modifier = Modifier.fillMaxWidth(),
                            contentPadding = PaddingValues(vertical = 10.dp),
                        ) { Text(deleteLabel, color = MaterialTheme.colorScheme.error, modifier = Modifier.fillMaxWidth()) }
                    }
                }
            }
        }
    }
}

/** Una conversa de la llista: targeta amb halo, nom i darrer missatge. */
@Composable
@OptIn(ExperimentalFoundationApi::class)
private fun ConversationRow(
    title: String,
    subtitle: String,
    accent: Color,
    style: WindStyle,
    busy: Boolean = false,
    mood: WindMood? = null,
    faces: List<Wind> = emptyList(),
    onClick: () -> Unit,
    onLongClick: (() -> Unit)? = null,
) {
    // ATENCIÓ: shadowElevation ha de ser 0 amb color translúcid: amb elevació
    // el Surface composa un fons blanc opac darrere la columna de text
    // (verificat a l'emulador). La profunditat la donen el halo i la vora.
    Surface(
        shape = RoundedCornerShape(20.dp),
        color = accent.copy(alpha = 0.12f),
        border = androidx.compose.foundation.BorderStroke(1.dp, accent.copy(alpha = 0.40f)),
        shadowElevation = 0.dp,
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 5.dp)
            .clip(RoundedCornerShape(20.dp)).combinedClickable(
                onClick = onClick,
                onLongClick = onLongClick,
            ),
    ) {
        Row(
            Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            // Avatar amb halo; la rotllana apila la segona cara.
            WindHalo(
                accent = accent, style = style, size = 48,
                stacked = faces.drop(1), online = !busy,
                mood = mood ?: if (busy) WindMood.THINKING else WindMood.HAPPY,
            )
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text(title, fontSize = 16.sp, fontWeight = FontWeight.Bold, color = Ink, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
                    if (busy) TypingDots(accent)
                }
                Text(
                    if (busy) "…" else subtitle,
                    fontSize = 13.sp, color = if (busy) accent else Muted,
                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
            }
            if (faces.size > 1) {
                Surface(shape = RoundedCornerShape(999.dp), color = accent.copy(alpha = 0.20f)) {
                    Text("◍ ${faces.size}", color = Ink, fontSize = 11.sp, fontWeight = FontWeight.Bold,
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp))
                }
            }
        }
    }
}

/** El xat d'un vent a pantalla completa: capçalera amb la cara, bombolles, input. */
@Composable
internal fun WindConversation(vm: GregalViewModel, lang: String, wind: Wind, onBack: () -> Unit) {
    val chats by vm.windChat.collectAsState()
    val busy by vm.windBusy.collectAsState()
    val live by vm.windLive.collectAsState()
    val msgs = chats[wind.id].orEmpty()
    val clipboard = LocalClipboardManager.current
    var field by remember(wind.id) { mutableStateOf(TextFieldValue("")) }
    val accent = Color(wind.argb)
    val winds by vm.winds.collectAsState()
    // Mencions @Nom: el desplegable segueix el cursor; en enviar,
    // si hi ha @UnAltre, la feina es deriva al seu xat (handoff).
    val mention = activeMention(field.text, field.selection.start, winds)
    val canSend = field.text.isNotBlank() && !busy.contains(wind.id)
    var profile by remember(wind.id) { mutableStateOf(false) }
    val listState = rememberLazyListState()
    LaunchedEffect(wind.id) { vm.loadWindChat(wind.id) }
    // Obre sempre a l'últim: un sol scrollToItem amb ítems sense mesurar
    // infravalora; avança per passos fins clavar el final, i després només
    // baixa sol si ja hi eres.
    var openedAtEnd by remember(wind.id) { mutableStateOf(false) }
    // A partir de quin índex els missatges són "en directe" (animen):
    // tot l'historial inicial apareix de cop, sense fades.
    var liveFrom by remember(wind.id) { mutableStateOf(Int.MAX_VALUE) }
    LaunchedEffect(wind.id, msgs.size) {
        if (msgs.isEmpty()) return@LaunchedEffect
        if (!openedAtEnd) {
            // Entrada instantània: un sol salt (sense animació) posiciona
            // la primera maqueta al final; l'usuari no veu cap desplaçament.
            // Amb ítems sense mesurar pot quedar curt: verificació curta
            // amb salts instantanis (0,8 s màx, sense xarxa animada).
            try {
                listState.scrollToItem(msgs.size - 1)
            } catch (_: Exception) { }
            repeat(8) {
                kotlinx.coroutines.delay(100)
                val li = listState.layoutInfo
                val darrer = li.visibleItemsInfo.lastOrNull()
                // L'últim ha d'estar DINS del viewport, no al prefetch:
                // offset+size més enllà del final vol dir que encara no es veu.
                if (darrer != null && darrer.index >= msgs.size - 1 &&
                    darrer.offset + darrer.size <= li.viewportEndOffset
                ) {
                    openedAtEnd = true
                    liveFrom = msgs.size
                    return@LaunchedEffect
                }
                try {
                    listState.scrollToItem(msgs.size - 1)
                } catch (_: Exception) {
                    // Índex encara fora de la maqueta: el proper intent ho torna a provar.
                }
            }
            openedAtEnd = true
            liveFrom = msgs.size
        } else {
            val lastVisible = listState.layoutInfo.visibleItemsInfo.lastOrNull()?.index
            if (lastVisible == null || lastVisible >= msgs.size - 3) listState.animateScrollToItem(msgs.size - 1)
        }
    }

    val isBusy = busy.contains(wind.id)
    Box(Modifier.fillMaxSize().background(Superficie)) {
        ChatAmbient(accent)
        Column(Modifier.fillMaxSize()) {
        // Capçalera espectacular: degradat del vent + halo + estat viu.
        Row(
            Modifier.fillMaxWidth()
                .background(Brush.verticalGradient(listOf(accent.copy(alpha = 0.30f), accent.copy(alpha = 0.06f))))
                .clickable { profile = true }.padding(horizontal = 8.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Surface(shape = CircleShape, color = Superficie.copy(alpha = 0.85f), shadowElevation = 3.dp) {
                IconButton(onClick = onBack) { Icon(Icons.Rounded.ArrowBack, tr(lang, "app.vents.back"), tint = Ink) }
            }
            WindHalo(size = 52, accent = accent, style = wind.faceStyle,
                mood = when {
                    isBusy -> live[wind.id]?.mood ?: WindMood.THINKING
                    msgs.any { it.streaming } -> WindMood.TALKING
                    else -> WindMood.HAPPY
                })
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(wind.name, fontSize = 19.sp, fontWeight = FontWeight.ExtraBold, color = Ink)
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text(
                        if (isBusy) tr(lang, "app.company.windThinking", wind.name)
                        else wind.role.ifBlank { tr(lang, "app.company.windHello", wind.name) },
                        fontSize = 12.sp, color = Ink.copy(alpha = 0.6f),
                        maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false),
                    )
                    if (isBusy) TypingDots(accent)
                }
            }
            IconButton(onClick = { profile = true }) {
                Icon(Icons.Rounded.Info, tr(lang, "app.vents.profile"), tint = Ink, modifier = Modifier.size(20.dp))
            }
        }
        // Missatges
        // Quan el viewport s'encongeix de cop (s'obre el teclat) i érem al
        // final, segueix-lo: el teclat no ha de tapar l'últim missatge.
        val scope = rememberCoroutineScope()
        var lastViewportH by remember(wind.id) { mutableStateOf(0) }
        LazyColumn(
            Modifier.weight(1f).fillMaxWidth().onSizeChanged { size ->
                val h = size.height
                if (lastViewportH > 0 && h < lastViewportH - 200 && msgs.isNotEmpty()) {
                    val lastVisible = listState.layoutInfo.visibleItemsInfo.lastOrNull()?.index
                    if (lastVisible == null || lastVisible >= msgs.size - 3) {
                        scope.launch {
                            try { listState.scrollToItem(msgs.size - 1) } catch (_: Exception) { }
                        }
                    }
                }
                lastViewportH = h
            },
            state = listState,
            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 10.dp),
            verticalArrangement = Arrangement.spacedBy(0.dp),
        ) {
            if (msgs.isEmpty()) {
                item {
                    ChatEmptyState(
                        face = { WindHalo(size = 88, accent = accent, style = wind.faceStyle, mood = WindMood.HAPPY, tappable = true) },
                        title = tr(lang, "app.vents.emptyChatTitle"),
                        hint = tr(lang, "app.company.windHello", wind.name),
                    )
                }
            }
            items(msgs.size, key = { idx -> idx }) { idx ->
                // Aire segons bàndol: juntes les del mateix, separades en canviar.
                val gap = if (idx > 0 && msgs[idx].from != msgs[idx - 1].from) 16.dp else 4.dp
                Column(Modifier.padding(top = gap)) {
                    BubbleIn("dm#$idx", animate = idx >= liveFrom) {
                        WindBubble(msgs[idx], accent, lang = lang,
                            onCopy = {
                                clipboard.setText(AnnotatedString(stripMdInline(msgs[idx].text)))
                                scope.launch { vm.notify(tr(lang, "app.vents.copied")) }
                            },
                            onContinue = if (!isBusy) ({ vm.sendWindContinue(wind.id) }) else null)
                    }
                }
            }
            // Peu "hi pensa" només si no hi ha bombolla de directe: la
            // bombolla ja mostra la fase (Cercant/Pensant…); si no, surten
            // dos avisos apilats.
            if (busy.contains(wind.id) && msgs.lastOrNull()?.streaming != true) {
                item {
                    Row(
                        Modifier.fillMaxWidth().padding(vertical = 4.dp),
                        horizontalArrangement = Arrangement.Center,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        TypingDots(accent)
                        Text(
                            "  " + tr(lang, "app.company.windThinking", wind.name),
                            fontSize = 12.sp, color = accent,
                        )
                    }
                }
            }
        }
        // Input píndola amb el color del vent + enviament amb resplendor.
        // El desplegable de mencions va just a sobre del camp.
        mention?.let { q ->
            MentionSuggestions(lang, q, accent) { w ->
                val handle = "@" + w.name + " "
                field = TextFieldValue(
                    field.text.replaceRange(q.at, field.selection.start, handle),
                    TextRange(q.at + handle.length),
                )
            }
        }
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            OutlinedTextField(
                value = field,
                onValueChange = { field = it },
                placeholder = { Text(tr(lang, "app.company.windChatHint", wind.name), fontSize = 14.sp) },
                shape = RoundedCornerShape(26.dp),
                modifier = Modifier.weight(1f),
                maxLines = 3,
                keyboardOptions = KeyboardOptions(
                    capitalization = KeyboardCapitalization.Sentences,
                    autoCorrect = true,
                    keyboardType = KeyboardType.Text,
                ),
                colors = OutlinedTextFieldDefaults.colors(
                    focusedBorderColor = accent, unfocusedBorderColor = accent.copy(alpha = 0.35f),
                    focusedContainerColor = Superficie, unfocusedContainerColor = Superficie,
                ),
            )
            Surface(
                shape = CircleShape,
                color = if (canSend) accent else SuperficieAlta,
                shadowElevation = if (canSend) 8.dp else 0.dp,
                modifier = Modifier.size(50.dp).clip(CircleShape)
                    .clickable(enabled = canSend) {
                        val body = field.text
                        val target = resolveMention(body, winds)
                        if (target != null && target.id != wind.id) {
                            // Handoff: la feina viatja al xat de l'altre vent.
                            if (busy.contains(target.id)) {
                                vm.notify(tr(lang, "app.vents.handoffBusy", target.name))
                            } else {
                                vm.sendWindMessage(target.id, body)
                                vm.notify(tr(lang, "app.vents.handoffSent", target.name))
                                field = TextFieldValue("")
                            }
                        } else {
                            vm.sendWindMessage(wind.id, body); field = TextFieldValue("")
                        }
                    },
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Icon(Icons.Rounded.Send, tr(lang, "app.vents.send"),
                        tint = if (canSend) Color.White else Muted,
                        modifier = Modifier.size(22.dp))
                }
            }
        }
        }
    }
    if (profile) WindProfileDialog(vm, lang, wind) { profile = false }
}

/** El xat d'una rotllana a pantalla completa. */
@Composable
private fun RotllanaConversation(vm: GregalViewModel, lang: String, rot: Rotllana, winds: List<Wind>, onBack: () -> Unit) {
    val chats by vm.rotllanaChat.collectAsState()
    val busy by vm.rotllanaBusy.collectAsState()
    val live by vm.windLive.collectAsState()
    val msgs = chats[rot.id].orEmpty()
    val clipboard = LocalClipboardManager.current
    var field by remember(rot.id) { mutableStateOf(TextFieldValue("")) }
    var groupProfile by remember(rot.id) { mutableStateOf(false) }
    var speakerProfile by remember(rot.id) { mutableStateOf<Wind?>(null) }
    val listState = rememberLazyListState()
    val faces = rot.members.mapNotNull { mid -> winds.firstOrNull { it.id == mid } }
    // Mencions @Nom: etiquetar un membre dirigeix el torn (només respon ell).
    val mention = activeMention(field.text, field.selection.start, faces)
    val canSend = field.text.isNotBlank() && !busy.contains(rot.id)
    LaunchedEffect(rot.id) { vm.loadRotllanaChat(rot.id) }
    // Obre sempre a l'últim: un sol scrollToItem amb ítems sense mesurar
    // infravalora; espera el layout, verifica i torna a acostar fins
    // clavar el final.
    var openedAtEnd by remember(rot.id) { mutableStateOf(false) }
    var liveFrom by remember(rot.id) { mutableStateOf(Int.MAX_VALUE) }
    LaunchedEffect(rot.id, msgs.size) {
        if (msgs.isEmpty()) return@LaunchedEffect
        if (!openedAtEnd) {
            // Entrada instantània: un sol salt (sense animació) posiciona
            // la primera maqueta al final; verificació curta amb salts
            // instantanis (0,8 s màx, sense xarxa animada).
            try {
                listState.scrollToItem(msgs.size - 1)
            } catch (_: Exception) { }
            repeat(8) {
                kotlinx.coroutines.delay(100)
                val li = listState.layoutInfo
                val darrer = li.visibleItemsInfo.lastOrNull()
                // L'últim ha d'estar DINS del viewport, no al prefetch:
                // offset+size més enllà del final vol dir que encara no es veu.
                if (darrer != null && darrer.index >= msgs.size - 1 &&
                    darrer.offset + darrer.size <= li.viewportEndOffset
                ) {
                    openedAtEnd = true
                    liveFrom = msgs.size
                    return@LaunchedEffect
                }
                try {
                    listState.scrollToItem(msgs.size - 1)
                } catch (_: Exception) {
                    // Índex encara fora de la maqueta: el proper intent ho torna a provar.
                }
            }
            openedAtEnd = true
            liveFrom = msgs.size
        } else {
            val lastVisible = listState.layoutInfo.visibleItemsInfo.lastOrNull()?.index
            if (lastVisible == null || lastVisible >= msgs.size - 3) listState.animateScrollToItem(msgs.size - 1)
        }
    }

    val rotBusy = busy.contains(rot.id)
    val rotAccent = faces.firstOrNull()?.let { Color(it.argb) } ?: Sea
    Box(Modifier.fillMaxSize().background(Superficie)) {
        ChatAmbient(rotAccent)
        Column(Modifier.fillMaxSize()) {
        Row(
            Modifier.fillMaxWidth()
                .background(Brush.verticalGradient(listOf(rotAccent.copy(alpha = 0.28f), Sea.copy(alpha = 0.10f))))
                .clickable { groupProfile = true }.padding(horizontal = 8.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Surface(shape = CircleShape, color = Superficie.copy(alpha = 0.85f), shadowElevation = 3.dp) {
                IconButton(onClick = onBack) { Icon(Icons.Rounded.ArrowBack, tr(lang, "app.vents.back"), tint = Ink) }
            }
            WindHalo(
                accent = rotAccent, style = faces.firstOrNull()?.faceStyle ?: WindStyle(),
                size = 52, stacked = faces.drop(1),
                mood = when {
                    rotBusy -> live[rot.id]?.mood ?: WindMood.THINKING
                    msgs.any { it.streaming } -> WindMood.TALKING
                    else -> WindMood.HAPPY
                },
            )
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text(rot.name, fontSize = 19.sp, fontWeight = FontWeight.ExtraBold, color = Ink, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
                    Surface(shape = RoundedCornerShape(999.dp), color = rotAccent.copy(alpha = 0.22f)) {
                        Text("◍ ${faces.size}", color = Ink, fontSize = 11.sp, fontWeight = FontWeight.Bold,
                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp))
                    }
                }
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    if (rotBusy) TypingDots(rotAccent)
                    else Text(
                        faces.joinToString(", ") { it.name }, fontSize = 12.sp,
                        color = Ink.copy(alpha = 0.6f), maxLines = 1, overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f, fill = false),
                    )
                }
            }
            IconButton(onClick = { groupProfile = true }) { Icon(Icons.Rounded.Info, tr(lang, "app.vents.members"), tint = Ink) }
        }
        // Quan el viewport s'encongeix de cop (s'obre el teclat) i érem al
        // final, segueix-lo: el teclat no ha de tapar l'últim missatge.
        val scope = rememberCoroutineScope()
        var lastViewportH by remember(rot.id) { mutableStateOf(0) }
        LazyColumn(
            Modifier.weight(1f).fillMaxWidth().onSizeChanged { size ->
                val h = size.height
                if (lastViewportH > 0 && h < lastViewportH - 200 && msgs.isNotEmpty()) {
                    val lastVisible = listState.layoutInfo.visibleItemsInfo.lastOrNull()?.index
                    if (lastVisible == null || lastVisible >= msgs.size - 3) {
                        scope.launch {
                            try { listState.scrollToItem(msgs.size - 1) } catch (_: Exception) { }
                        }
                    }
                }
                lastViewportH = h
            },
            state = listState,
            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 10.dp),
            verticalArrangement = Arrangement.spacedBy(0.dp),
        ) {
            if (msgs.isEmpty()) item {
                ChatEmptyState(
                    face = {
                        WindHalo(
                            size = 88, accent = rotAccent,
                            style = faces.firstOrNull()?.faceStyle ?: WindStyle(),
                            stacked = faces.drop(1), mood = WindMood.HAPPY, tappable = true,
                        )
                    },
                    title = tr(lang, "app.vents.emptyThreadTitle"),
                    hint = tr(lang, "app.vents.threadHello", rot.name),
                )
            }
            items(msgs.size, key = { idx -> idx }) { idx ->
                val resolved = resolveWindMessage(msgs[idx], faces)
                // La cara i el nom només al primer missatge de cada ratxa
                // del mateix parlant: si no, cada bombolla repetia cara +
                // insígnia i el xat semblava ple de cercles verds.
                // (from només diu user/pet i l'historial antic no porta
                // speakerId: la ratxa va pel parlant ja resolt.)
                val firstOfRun = idx == 0 || runKeyOf(resolved) != runKeyOf(resolveWindMessage(msgs[idx - 1], faces))
                val continuation = !firstOfRun && resolved.message.from != "user"
                // Aire segons bàndol: juntes les del mateix, separades en canviar.
                val gap = if (firstOfRun && idx > 0) 16.dp else 4.dp
                Column(Modifier.padding(top = gap)) {
                BubbleIn("rot#$idx", animate = idx >= liveFrom) {
                WindBubble(resolved.message, if (resolved.message.from == "user") rotAccent else resolved.wind?.let { Color(it.argb) } ?: Sea,
                    lang = lang, speaker = if (firstOfRun) resolved.wind else null,
                    speakerName = if (firstOfRun) resolved.name.ifBlank { tr(lang, "app.vents.windSpeaker") } else "",
                    onSpeakerClick = { resolved.wind?.let { speakerProfile = it } },
                    indent = if (continuation) 52.dp else 0.dp,
                    onCopy = {
                        clipboard.setText(AnnotatedString(stripMdInline(resolved.message.text)))
                        scope.launch { vm.notify(tr(lang, "app.vents.copied")) }
                    },
                    onContinue = if (!busy.contains(rot.id)) ({ vm.sendRotllanaContinue(rot.id, resolved.message.speakerId) }) else null)
                }
                }
            }
            // Peu "hi pensa" només si no hi ha bombolla de directe (mateix
            // motiu que al xat directe: evitar l'avís duplicat).
            if (busy.contains(rot.id) && msgs.lastOrNull()?.streaming != true) {
                item {
                    Row(
                        Modifier.fillMaxWidth().padding(vertical = 4.dp),
                        horizontalArrangement = Arrangement.Center,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        TypingDots(rotAccent)
                        Text(
                            "  " + tr(lang, "app.company.windThinking", "…"),
                            fontSize = 12.sp, color = rotAccent,
                        )
                    }
                }
            }
        }
        mention?.let { q ->
            MentionSuggestions(lang, q, rotAccent) { w ->
                val handle = "@" + w.name + " "
                field = TextFieldValue(
                    field.text.replaceRange(q.at, field.selection.start, handle),
                    TextRange(q.at + handle.length),
                )
            }
        }
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            OutlinedTextField(
                value = field,
                onValueChange = { field = it },
                placeholder = { Text(tr(lang, "app.company.rotllanaHint"), fontSize = 14.sp) },
                shape = RoundedCornerShape(26.dp),
                modifier = Modifier.weight(1f),
                maxLines = 3,
                keyboardOptions = KeyboardOptions(
                    capitalization = KeyboardCapitalization.Sentences,
                    autoCorrect = true,
                    keyboardType = KeyboardType.Text,
                ),
                colors = OutlinedTextFieldDefaults.colors(
                    focusedBorderColor = rotAccent, unfocusedBorderColor = rotAccent.copy(alpha = 0.35f),
                    focusedContainerColor = Superficie, unfocusedContainerColor = Superficie,
                ),
            )
            Surface(
                shape = CircleShape,
                color = if (canSend) rotAccent else SuperficieAlta,
                shadowElevation = if (canSend) 8.dp else 0.dp,
                modifier = Modifier.size(50.dp).clip(CircleShape)
                    .clickable(enabled = canSend) {
                        val body = field.text
                        // @Nom dirigeix el torn: només respon l'etiquetat.
                        val target = resolveMention(body, faces)
                        vm.sendRotllanaMessage(rot.id, body, target?.id.orEmpty()); field = TextFieldValue("")
                    },
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Icon(Icons.Rounded.Send, tr(lang, "app.vents.send"),
                        tint = if (canSend) Color.White else Muted,
                        modifier = Modifier.size(22.dp))
                }
            }
        }
        }
    }
    if (groupProfile) GroupProfileDialog(vm, lang, rot, winds) { groupProfile = false }
    speakerProfile?.let { WindProfileDialog(vm, lang, it) { speakerProfile = null } }
}

/** Desplegable de @mencions: cares candidates sobre el camp de text.
 *  Compartit pel xat directe i la rotllana. */
@Composable
private fun MentionSuggestions(lang: String, q: MentionQuery, accent: Color, onPick: (Wind) -> Unit) {
    Row(
        Modifier.fillMaxWidth().horizontalScroll(rememberScrollState())
            .padding(horizontal = 12.dp, vertical = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        q.cands.take(6).forEach { w ->
            Surface(
                shape = RoundedCornerShape(999.dp),
                color = accent.copy(alpha = 0.12f),
                modifier = Modifier.clip(RoundedCornerShape(999.dp)).clickable { onPick(w) }
                    .semantics { contentDescription = tr(lang, "app.vents.mentionPick", w.name) },
            ) {
                Row(
                    Modifier.padding(horizontal = 10.dp, vertical = 6.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    WindFace(size = 22, accent = Color(w.argb), style = w.faceStyle)
                    Text("@" + w.name, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = Ink)
                }
            }
        }
    }
}

/** Memòria d'imatges del xat (http): evita tornar a baixar en recomposar. */
private val chatImageCache = object : LinkedHashMap<String, ImageBitmap>(32, 0.75f, true) {
    override fun removeEldestEntry(eldest: MutableMap.MutableEntry<String, ImageBitmap>?) = size > 20
}

/** Baixa i redueix una imatge web per al xat (màx 1024px, 8MB). Blocant:
 *  cridar en IO. Només http(s); tota la resta, null. */
private fun loadChatBitmap(url: String): ImageBitmap? {
    if (!url.startsWith("http://") && !url.startsWith("https://")) return null
    return try {
        val conn = java.net.URL(url).openConnection() as java.net.HttpURLConnection
        conn.connectTimeout = 8000
        conn.readTimeout = 8000
        conn.instanceFollowRedirects = true
        conn.connect()
        if (conn.responseCode !in 200..299) return null
        if (conn.contentLength > 8 * 1024 * 1024) return null
        val bytes = conn.inputStream.use { it.readBytes() }
        if (bytes.size > 8 * 1024 * 1024 || bytes.isEmpty()) return null
        val probe = android.graphics.BitmapFactory.Options()
        probe.inJustDecodeBounds = true
        android.graphics.BitmapFactory.decodeByteArray(bytes, 0, bytes.size, probe)
        var sample = 1
        while (maxOf(probe.outWidth, probe.outHeight) / sample > 1024) sample *= 2
        val opts = android.graphics.BitmapFactory.Options()
        opts.inSampleSize = sample
        android.graphics.BitmapFactory.decodeByteArray(bytes, 0, bytes.size, opts)?.asImageBitmap()
    } catch (_: Exception) {
        null
    }
}

/** Imatge d'un missatge (estil Redde: el vent t'envia fitxers al xat).
 *  Placeholder → foto (toc = obre fora). Si falla, enllaç de text. */
@Composable
internal fun MdImage(url: String, alt: String, link: Color, lang: String) {
    val uri = LocalUriHandler.current
    var bmp by remember(url) { mutableStateOf(chatImageCache[url]) }
    var failed by remember(url) { mutableStateOf(false) }
    LaunchedEffect(url) {
        if (bmp == null && !failed) {
            val got = withContext(Dispatchers.IO) { loadChatBitmap(url) }
            if (got != null) {
                synchronized(chatImageCache) { chatImageCache[url] = got }
                bmp = got
            } else failed = true
        }
    }
    when {
        bmp != null -> Image(
            bitmap = bmp!!,
            contentDescription = tr(lang, "app.vents.imageContent", alt.ifBlank { url }),
            contentScale = ContentScale.FillWidth,
            modifier = Modifier.fillMaxWidth().heightIn(max = 240.dp)
                .clip(RoundedCornerShape(12.dp))
                .clickable { try { uri.openUri(url) } catch (_: Exception) { } },
        )
        failed -> Text(
            alt.ifBlank { url }, fontSize = 13.sp, color = link,
            textDecoration = TextDecoration.Underline,
            modifier = Modifier.clip(RoundedCornerShape(8.dp))
                .clickable { try { uri.openUri(url) } catch (_: Exception) { } }
                .padding(vertical = 2.dp),
        )
        else -> Surface(
            color = CodeDark, shape = RoundedCornerShape(12.dp),
            modifier = Modifier.fillMaxWidth().height(160.dp),
        ) {
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                CircularProgressIndicator(Modifier.size(24.dp), strokeWidth = 2.dp, color = link)
            }
        }
    }
}

/**
 * Xat encara buit: cara gran contenta + títol + pista, en comptes d'una
 * línia grisa. La cara entra per slot: un vent sol o la pila de la rotllana.
 */
@Composable
internal fun ChatEmptyState(face: @Composable () -> Unit, title: String, hint: String) {
    Column(
        Modifier.fillMaxWidth().padding(vertical = 36.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        BubbleIn("empty-face") { face() }
        Text(title, fontSize = 16.sp, fontWeight = FontWeight.ExtraBold, color = Ink, textAlign = TextAlign.Center)
        Text(hint, fontSize = 13.sp, color = Muted, textAlign = TextAlign.Center, modifier = Modifier.padding(horizontal = 40.dp))
    }
}

/**
 * Entrada d'una bombolla nova: només fos (sense lliscar), perquè l'ítem
 * ocupa l'alçada des del primer fotograma i l'auto-scroll no es desquadra.
 * La clau és estable per posició (NO pel text): si no, cada token del
 * streaming la reiniciava i la bombolla parpellejava.
 */
@Composable
private fun BubbleIn(key: String, animate: Boolean = true, content: @Composable () -> Unit) {
    // Només anima els missatges nous en directe: en obrir el xat tot
    // apareix de cop (si no, cada bombolla fa fade una per una i es veu
    // lent i lleig mentre la llista es posiciona).
    if (!LocalWindMotion.current || !animate) {
        content()
        return
    }
    var shown by remember(key) { mutableStateOf(false) }
    LaunchedEffect(key) { shown = true }
    AnimatedVisibility(
        visible = shown,
        enter = fadeIn(animationSpec = tween(220)),
    ) { content() }
}
@OptIn(ExperimentalFoundationApi::class)
@Composable
internal fun WindBubble(m: GregalViewModel.PetMsg, accent: Color, lang: String = "ca", speaker: Wind? = null,
    speakerName: String = "", onSpeakerClick: (() -> Unit)? = null, indent: Dp = 0.dp,
    onCopy: (() -> Unit)? = null, onContinue: (() -> Unit)? = null) {
    val mine = m.from == "user"
    // Lletra sobre el color del vent: blanca si el color és intens, tinta
    // si és pàl·lid (com la bombolla rosa del dots amb text blanc).
    val onAccent = remember(accent) {
        val lum = 0.2126f * accent.red + 0.7152f * accent.green + 0.0722f * accent.blue
        if (lum > 0.55f) Color(0xFF2B2118) else Color.White
    }
    val receipt = remember(m.ts, lang) {
        if (m.ts > 0L) java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
            .format(java.util.Date(m.ts)) else ""
    }
    if (mine) {
        Column(Modifier.fillMaxWidth().padding(start = indent), horizontalAlignment = Alignment.End) {
            Surface(
                shape = RoundedCornerShape(
                    topStart = 20.dp, topEnd = 20.dp,
                    bottomStart = 20.dp, bottomEnd = 6.dp,
                ),
                color = accent,
                shadowElevation = 2.dp,
                // Prémer llarg copia el missatge en text pla (sense marques).
                modifier = Modifier.widthIn(max = 300.dp).combinedClickable(
                    onClick = {}, onLongClick = { onCopy?.invoke() },
                ),
            ) {
                Text(
                    m.text,
                    fontSize = 15.sp,
                    color = onAccent,
                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp),
                )
            }
            // Rebut d'hora com el dots («Read 2:59 PM»): només als enviats
            // en aquesta sessió (l'historial antic no porta hora).
            if (receipt.isNotEmpty()) {
                Text(receipt, fontSize = 11.sp, color = Ink.copy(alpha = 0.55f),
                    modifier = Modifier.padding(top = 2.dp, end = 4.dp))
            }
        }
        return
    }
    Row(
        Modifier.fillMaxWidth().padding(start = indent),
        horizontalArrangement = Arrangement.Start,
    ) {
        // Cara del vent fora de la targeta: l'agent té cos.
        if (speaker != null) {
            Box(Modifier.padding(top = 14.dp, end = 8.dp).clip(CircleShape).clickable(enabled = onSpeakerClick != null, onClick = { onSpeakerClick?.invoke() })) {
                WindHalo(accent = Color(speaker.argb), style = speaker.faceStyle, size = 44,
                    mood = if (m.streaming) WindMood.TALKING else WindMood.HAPPY)
            }
        }
        // Bombolla del vent: clara com la del dots (la blanca ressalta sobre
        // el fons tenyit), cantonades grosses i ombra suau. La cua la marca
        // la cantonada superior.
        val bubbleShape = RoundedCornerShape(
            topStart = 6.dp, topEnd = 20.dp,
            bottomStart = 20.dp, bottomEnd = 20.dp,
        )
        // Pastilla amb el nom a cavall de la vora (com el «Jojo» del dots):
        // marge de sobra perquè no tapi mai la primera línia.
        Box(Modifier.padding(top = if (speakerName.isNotBlank()) 12.dp else 0.dp)) {
            Surface(
                shape = bubbleShape,
                color = Superficie,
                shadowElevation = 2.dp,
                // Prémer llarg copia la resposta en text pla (sense marques).
                modifier = Modifier.widthIn(max = 300.dp).combinedClickable(
                    onClick = {}, onLongClick = { onCopy?.invoke() },
                ),
            ) {
                val uri = LocalUriHandler.current
                val dark = isSystemInDarkTheme()
                // Color d'enllaç llegible + blocs parsejats: tot amb remember perquè
                // no es recalculi a cada recomposició (streaming, scroll, teclat).
                // El parse viu a més en cau global: en tornar a la pantalla no se'n
                // refà ni un.
                val link = remember(accent, dark) { readableLink(accent, dark) }
                val codeBg = CodeDark
                val blocks = remember(m.text) { richBlocks(m.text) }
                val plain = remember(m.text) { stripMdInline(m.text).ifBlank { "…" } }
                Column(
                    Modifier.padding(horizontal = 14.dp)
                        .padding(top = if (speakerName.isNotBlank()) 14.dp else 10.dp, bottom = 10.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    // En directe el markdown parcial es trencaria: text pla net
                    // (sense marques crues) fins al done.
                    if (m.streaming) {
                        Text(plain, fontSize = 15.sp, lineHeight = 22.sp, color = Tinta)
                    } else { for (b in blocks) {
                        when (b) {
                            is Rich.Inline -> {
                                val rich = remember(b, link, codeBg) { renderSegs(b.segs, codeBg, link = link) }
                                ClickableText(
                                    rich,
                                    style = MaterialTheme.typography.bodyMedium.copy(fontSize = 15.sp, lineHeight = 22.sp, color = Tinta),
                                    onClick = { off ->
                                        rich.getStringAnnotations("URL", off, off).firstOrNull()?.let { uri.openUri(it.item) }
                                    },
                                )
                            }
                            is Rich.Code -> Row(Modifier.horizontalScroll(rememberScrollState())) {
                                Text(
                                    b.code,
                                    fontSize = 12.sp,
                                    fontFamily = FontFamily.Monospace,
                                    softWrap = false,
                                    color = Ink,
                                    modifier = Modifier
                                        .background(CodeDark, RoundedCornerShape(8.dp))
                                        .padding(8.dp),
                                )
                            }
                            is Rich.Table -> MdTable(b.headers, b.rows, lang)
                            is Rich.Chart -> ChartBlock(b.spec, accent, dark)
                            is Rich.Image -> MdImage(
                                b.url, b.alt,
                                link, lang,
                            )
                        }
                    }
                    }
                    // Parcial: el vent s'ha quedat a mitges (pressupost exhaurit).
                    // El botó reprèn la resposta pel mateix fil, sense preguntar.
                    if (onContinue != null && m.parcial && !m.streaming) {
                        TextButton(onClick = onContinue, contentPadding = PaddingValues(0.dp)) {
                            Text(tr(lang, "app.vents.continue"), color = accent,
                                fontSize = 13.sp, fontWeight = FontWeight.Bold)
                        }
                    }
                }
            }
            if (speakerName.isNotBlank()) {
                Surface(shape = RoundedCornerShape(999.dp), color = accent,
                    shadowElevation = 2.dp,
                    modifier = Modifier.align(Alignment.TopStart).offset(x = 12.dp, y = (-11).dp)
                        .then(if (speaker != null && onSpeakerClick != null) Modifier.clickable(onClick = onSpeakerClick) else Modifier)) {
                    Text(speakerName, color = onAccent, fontSize = 12.sp, fontWeight = FontWeight.ExtraBold,
                        modifier = Modifier.padding(horizontal = 10.dp, vertical = 2.dp))
                }
            }
        }
    }
}

/** Avatar pixel-art 16x16 d'una mascota, dibuixat amb Canvas (PICO-8). */
@Composable
fun PetAvatar(petId: String, size: Int = 48) {
    val grid = remember(petId) { PetAvatars.grid(petId) }
    val px = remember(petId) { PetAvatars.pixels(grid) }
    Canvas(Modifier.size(size.dp)) {
        val cell = this.size.width / 16f
        for (y in 0 until 16) {
            for (x in 0 until 16) {
                drawRect(
                    color = Color(px[y][x]),
                    topLeft = Offset(x * cell, y * cell),
                    size = Size(cell, cell),
                )
            }
        }
    }
}


/** Icona del calaix: la carona del vent actiu (o un vent genèric). */
@Composable
fun DrawerWindIcon(vm: GregalViewModel) {
    val winds by vm.winds.collectAsState()
    val active by vm.activeWind.collectAsState()
    val wind = active ?: winds.firstOrNull()
    if (wind != null) {
        WindFace(size = 24, accent = Color(wind.argb), style = wind.faceStyle)
    } else {
        Icon(Icons.Rounded.AutoAwesome, null)
    }
}

/**
 * WindFab — el botó flotant del vent actiu: cara petita i viva a baix a la
 * dreta, sobre qualsevol pantalla. Un toc obre la pantalla Vents.
 */
@Composable
fun WindFab(vm: GregalViewModel, onOpen: () -> Unit) {
    val wind by vm.activeWind.collectAsState()
    val busy by vm.windBusy.collectAsState()
    val live by vm.windLive.collectAsState()
    val news by vm.windNews.collectAsState()
    val w = wind ?: return
    Box(
        Modifier
            .fillMaxWidth()
            .fillMaxHeight(),
        contentAlignment = Alignment.BottomEnd,
    ) {
        Surface(
            shape = CircleShape,
            shadowElevation = 6.dp,
            color = Superficie,
            modifier = Modifier
                .padding(16.dp)
                .size(64.dp)
                .clip(CircleShape)
                .clickable(onClick = onOpen)
                .semantics { contentDescription = "Vent actiu: ${w.name}" },
        ) {
            Box(contentAlignment = Alignment.Center) {
                WindFace(
                    size = 52,
                    mood = when {
                        busy.contains(w.id) -> live[w.id]?.mood ?: WindMood.THINKING
                        news.contains(w.id) -> WindMood.NEWS
                        else -> WindMood.IDLE
                    },
                    accent = Color(w.argb),
                    style = w.faceStyle,
                )
            }
        }
    }
}
