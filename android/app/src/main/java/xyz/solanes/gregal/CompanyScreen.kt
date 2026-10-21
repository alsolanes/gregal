package xyz.solanes.gregal

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.Delete
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
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.text.ClickableText
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
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
    val conn by vm.conn.collectAsState()
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
                    if (manage) {
                        item { WindsModelRow(vm, lang) }
                        item { Text(tr(lang, "app.vents.manageHelp"), color = Muted, fontSize = 13.sp, modifier = Modifier.padding(horizontal = 20.dp, vertical = 12.dp)) }
                        if (winds.isEmpty()) item { VentsEmpty(lang, false) }
                        items(winds, key = { it.id }) { w ->
                            ConversationRow(w.name, w.role, Color(w.argb), w.faceStyle, onClick = { profile = w })
                        }
                    } else {
                        if (rotllanes.isEmpty() && winds.none { chats[it.id].orEmpty().isNotEmpty() }) item { VentsEmpty(lang, true) }
                        items(winds.filter { chats[it.id].orEmpty().isNotEmpty() }, key = { "wind:" + it.id }) { w ->
                            ConversationRow(w.name, chats[w.id].orEmpty().lastOrNull()?.text.orEmpty(), Color(w.argb), w.faceStyle,
                                busy = busy.contains(w.id), onClick = { vm.openVentsChat("wind:" + w.id) })
                        }
                        items(rotllanes, key = { "rot:" + it.id }) { r ->
                            val faces = r.members.mapNotNull { mid -> winds.firstOrNull { it.id == mid } }
                            ConversationRow(r.name, rChats[r.id].orEmpty().lastOrNull()?.text ?: faces.joinToString(" · ") { it.name },
                                faces.firstOrNull()?.let { Color(it.argb) } ?: Sea,
                                faces.firstOrNull()?.faceStyle ?: WindStyle(), faces = faces,
                                busy = rBusy.contains(r.id),
                                onClick = { vm.openVentsChat("rot:" + r.id) })
                        }
                    }
                }
            }
        }
    }
    profile?.let { w -> WindProfileDialog(vm, lang, w) { profile = null } }
}

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

/** Una conversa de la llista: targeta amb halo, nom i darrer missatge. */
@Composable
private fun ConversationRow(
    title: String,
    subtitle: String,
    accent: Color,
    style: WindStyle,
    busy: Boolean = false,
    faces: List<Wind> = emptyList(),
    onClick: () -> Unit,
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
            .clip(RoundedCornerShape(20.dp)).clickable(onClick = onClick),
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
private fun WindConversation(vm: GregalViewModel, lang: String, wind: Wind, onBack: () -> Unit) {
    val chats by vm.windChat.collectAsState()
    val busy by vm.windBusy.collectAsState()
    val msgs = chats[wind.id].orEmpty()
    var text by remember(wind.id) { mutableStateOf("") }
    val accent = Color(wind.argb)
    var profile by remember(wind.id) { mutableStateOf(false) }
    val listState = rememberLazyListState()
    LaunchedEffect(wind.id) { vm.loadWindChat(wind.id) }
    // Obre sempre a l'últim: salt instantani en obrir (espera les mesures
    // de la llista o el salt es perd), i després només baixa sol si ja hi eres.
    var openedAtEnd by remember(wind.id) { mutableStateOf(false) }
    LaunchedEffect(msgs.size) {
        if (msgs.isEmpty()) return@LaunchedEffect
        if (!openedAtEnd) {
            var tries = 0
            while (listState.layoutInfo.visibleItemsInfo.isEmpty() && tries < 60) {
                withFrameNanos { }
                tries++
            }
            listState.scrollToItem(msgs.size - 1)
            openedAtEnd = true
        } else {
            val lastVisible = listState.layoutInfo.visibleItemsInfo.lastOrNull()?.index
            if (lastVisible == null || lastVisible >= msgs.size - 3) listState.animateScrollToItem(msgs.size - 1)
        }
    }

    val isBusy = busy.contains(wind.id)
    Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(accent.copy(alpha = 0.14f), Superficie)))) {
        ChatAmbient(accent)
        Column(Modifier.fillMaxSize()) {
        // Capçalera espectacular: degradat del vent + halo + estat viu.
        Row(
            Modifier.fillMaxWidth()
                .background(Brush.verticalGradient(listOf(accent.copy(alpha = 0.38f), accent.copy(alpha = 0.10f))))
                .clickable { profile = true }.padding(horizontal = 8.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Surface(shape = CircleShape, color = Superficie.copy(alpha = 0.85f), shadowElevation = 3.dp) {
                IconButton(onClick = onBack) { Icon(Icons.Rounded.ArrowBack, tr(lang, "app.vents.back"), tint = Ink) }
            }
            WindHalo(size = 52, accent = accent, style = wind.faceStyle,
                mood = if (isBusy) WindMood.THINKING else WindMood.HAPPY)
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(wind.name, fontSize = 19.sp, fontWeight = FontWeight.ExtraBold, color = Ink)
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Box(Modifier.size(8.dp).background(if (isBusy) Sand else Green, CircleShape))
                    Text(
                        if (isBusy) tr(lang, "app.company.windThinking", wind.name)
                        else wind.role.ifBlank { tr(lang, "app.company.windHello", wind.name) },
                        fontSize = 12.sp, color = Ink.copy(alpha = 0.75f),
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
        LazyColumn(
            Modifier.weight(1f).fillMaxWidth(),
            state = listState,
            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 10.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            if (msgs.isEmpty()) {
                item {
                    Text(
                        tr(lang, "app.company.windHello", wind.name),
                        fontSize = 14.sp, color = Tinta, textAlign = TextAlign.Center,
                        modifier = Modifier.fillMaxWidth().padding(vertical = 24.dp),
                    )
                }
            }
            items(msgs.size) { idx -> BubbleIn(msgs[idx].text + "#" + idx) { WindBubble(msgs[idx], accent, lang = lang) } }
            if (busy.contains(wind.id)) {
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
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            OutlinedTextField(
                value = text,
                onValueChange = { text = it },
                placeholder = { Text(tr(lang, "app.company.windChatHint", wind.name), fontSize = 14.sp) },
                leadingIcon = { WindFace(size = 30, mood = if (text.isNotBlank()) WindMood.HAPPY else WindMood.IDLE, accent = accent, style = wind.faceStyle) },
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
                color = if (text.isNotBlank() && !busy.contains(wind.id)) accent else SuperficieAlta,
                shadowElevation = if (text.isNotBlank() && !busy.contains(wind.id)) 8.dp else 0.dp,
                modifier = Modifier.size(50.dp).clip(CircleShape)
                    .clickable(enabled = text.isNotBlank() && !busy.contains(wind.id)) { vm.sendWindMessage(wind.id, text); text = "" },
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Icon(Icons.Rounded.Send, tr(lang, "app.vents.send"),
                        tint = if (text.isNotBlank() && !busy.contains(wind.id)) Color.White else Muted,
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
    val msgs = chats[rot.id].orEmpty()
    var text by remember(rot.id) { mutableStateOf("") }
    var groupProfile by remember(rot.id) { mutableStateOf(false) }
    var speakerProfile by remember(rot.id) { mutableStateOf<Wind?>(null) }
    val listState = rememberLazyListState()
    val faces = rot.members.mapNotNull { mid -> winds.firstOrNull { it.id == mid } }
    LaunchedEffect(rot.id) { vm.loadRotllanaChat(rot.id) }
    // Obre sempre a l'últim: salt instantani en obrir (espera les mesures
    // de la llista o el salt es perd), i després només baixa sol si ja hi eres.
    var openedAtEnd by remember(rot.id) { mutableStateOf(false) }
    LaunchedEffect(msgs.size) {
        if (msgs.isEmpty()) return@LaunchedEffect
        if (!openedAtEnd) {
            var tries = 0
            while (listState.layoutInfo.visibleItemsInfo.isEmpty() && tries < 60) {
                withFrameNanos { }
                tries++
            }
            listState.scrollToItem(msgs.size - 1)
            openedAtEnd = true
        } else {
            val lastVisible = listState.layoutInfo.visibleItemsInfo.lastOrNull()?.index
            if (lastVisible == null || lastVisible >= msgs.size - 3) listState.animateScrollToItem(msgs.size - 1)
        }
    }

    val rotBusy = busy.contains(rot.id)
    val rotAccent = faces.firstOrNull()?.let { Color(it.argb) } ?: Sea
    Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Sea.copy(alpha = 0.12f), Superficie)))) {
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
                    Box(Modifier.size(8.dp).background(if (rotBusy) Sand else Green, CircleShape))
                    Text(
                        faces.joinToString(", ") { it.name }, fontSize = 12.sp,
                        color = Ink.copy(alpha = 0.75f), maxLines = 1, overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f, fill = false),
                    )
                    if (rotBusy) TypingDots(rotAccent)
                }
            }
            IconButton(onClick = { groupProfile = true }) { Icon(Icons.Rounded.Info, tr(lang, "app.vents.members"), tint = Ink) }
        }
        LazyColumn(
            Modifier.weight(1f).fillMaxWidth(),
            state = listState,
            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 10.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            if (msgs.isEmpty()) item {
                Text(tr(lang, "app.vents.threadHello", rot.name), color = Muted, textAlign = TextAlign.Center,
                    modifier = Modifier.fillMaxWidth().padding(24.dp))
            }
            items(msgs.size) { idx ->
                val resolved = resolveWindMessage(msgs[idx], faces)
                BubbleIn(resolved.message.text + "#" + idx) {
                WindBubble(resolved.message, resolved.wind?.let { Color(it.argb) } ?: Sea,
                    lang = lang, speaker = resolved.wind,
                    speakerName = resolved.name.ifBlank { tr(lang, "app.vents.windSpeaker") },
                    onSpeakerClick = { resolved.wind?.let { speakerProfile = it } })
                }
            }
            if (busy.contains(rot.id)) {
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
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            OutlinedTextField(
                value = text,
                onValueChange = { text = it },
                placeholder = { Text(tr(lang, "app.company.rotllanaHint"), fontSize = 14.sp) },
                leadingIcon = { WindFace(size = 30, accent = rotAccent, style = faces.firstOrNull()?.faceStyle ?: WindStyle()) },
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
                color = if (text.isNotBlank() && !busy.contains(rot.id)) rotAccent else SuperficieAlta,
                shadowElevation = if (text.isNotBlank() && !busy.contains(rot.id)) 8.dp else 0.dp,
                modifier = Modifier.size(50.dp).clip(CircleShape)
                    .clickable(enabled = text.isNotBlank() && !busy.contains(rot.id)) { vm.sendRotllanaMessage(rot.id, text); text = "" },
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Icon(Icons.Rounded.Send, tr(lang, "app.vents.send"),
                        tint = if (text.isNotBlank() && !busy.contains(rot.id)) Color.White else Muted,
                        modifier = Modifier.size(22.dp))
                }
            }
        }
        }
    }
    if (groupProfile) GroupProfileDialog(vm, lang, rot, winds) { groupProfile = false }
    speakerProfile?.let { WindProfileDialog(vm, lang, it) { speakerProfile = null } }
}

/**
 * Entrada d'una bombolla nova: només fos (sense lliscar), perquè l'ítem
 * ocupa l'alçada des del primer fotograma i l'auto-scroll no es desquadra.
 */
@Composable
private fun BubbleIn(key: String, content: @Composable () -> Unit) {
    if (!LocalWindMotion.current) {
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
@Composable
internal fun WindBubble(m: GregalViewModel.PetMsg, accent: Color, lang: String = "ca", speaker: Wind? = null,
    speakerName: String = "", onSpeakerClick: (() -> Unit)? = null) {
    val mine = m.from == "user"
    Row(
        Modifier.fillMaxWidth(),
        horizontalArrangement = if (mine) Arrangement.End else Arrangement.Start,
    ) {
        if (mine) {
            Surface(
                shape = RoundedCornerShape(
                    topStart = 16.dp, topEnd = 16.dp,
                    bottomStart = 16.dp, bottomEnd = 4.dp,
                ),
                color = Sea,
                shadowElevation = 4.dp,
                modifier = Modifier.widthIn(max = 300.dp),
            ) {
                Text(
                    m.text,
                    fontSize = 14.sp,
                    color = OnSea,
                    modifier = Modifier.padding(horizontal = 13.dp, vertical = 9.dp),
                )
            }
            return@Row
        }
        // Cara del vent fora de la targeta: l'agent té cos.
        if (speaker != null) {
            Box(Modifier.padding(top = 2.dp).clip(CircleShape).clickable(enabled = onSpeakerClick != null, onClick = { onSpeakerClick?.invoke() })) {
                WindHalo(accent = Color(speaker.argb), style = speaker.faceStyle, size = 38)
            }
        }
        // Halo del color del vent darrere la targeta: respira sense ombres.
        val bubbleShape = RoundedCornerShape(
            topStart = 4.dp, topEnd = 16.dp,
            bottomStart = 16.dp, bottomEnd = 16.dp,
        )
        Box(Modifier.background(accent.copy(alpha = 0.13f), bubbleShape).padding(3.dp)) {
        Surface(
            shape = bubbleShape,
            color = Superficie,
            border = androidx.compose.foundation.BorderStroke(1.dp, accent.copy(alpha = 0.60f)),
            shadowElevation = 0.dp,
            modifier = Modifier.widthIn(max = 300.dp),
        ) {
            val uri = LocalUriHandler.current
            Column(Modifier.padding(horizontal = 12.dp, vertical = 9.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                if (speakerName.isNotBlank()) {
                    Row(Modifier.then(if (speaker != null && onSpeakerClick != null) Modifier.clickable(onClick = onSpeakerClick) else Modifier),
                        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        Box(Modifier.size(9.dp).background(accent, CircleShape))
                        Text(speakerName, color = Ink, fontSize = 12.sp, fontWeight = FontWeight.ExtraBold)
                        Surface(shape = RoundedCornerShape(999.dp), color = accent.copy(alpha = 0.16f)) {
                            Text("vent", color = Ink.copy(alpha = 0.7f), fontSize = 10.sp,
                                modifier = Modifier.padding(horizontal = 7.dp, vertical = 1.dp))
                        }
                    }
                }
                // En directe el markdown parcial es trencaria: text pla fins al done.
                if (m.streaming) {
                    Text(m.text.ifBlank { "…" }, fontSize = 14.sp, lineHeight = 21.sp, color = Tinta)
                } else { for (b in groupRich(splitMd(m.text))) {
                    when (b) {
                        is Rich.Inline -> {
                            val rich = renderSegs(b.segs, CodeDark, link = accent)
                            ClickableText(
                                rich,
                                style = MaterialTheme.typography.bodyMedium.copy(fontSize = 14.sp, lineHeight = 21.sp, color = Tinta),
                                onClick = { off ->
                                    rich.getStringAnnotations("URL", off, off).firstOrNull()?.let { uri.openUri(it.item) }
                                },
                            )
                        }
                        is Rich.Code -> Text(
                            b.code,
                            fontSize = 12.sp,
                            fontFamily = FontFamily.Monospace,
                            color = Ink,
                            modifier = Modifier
                                .fillMaxWidth()
                                .background(CodeDark, RoundedCornerShape(8.dp))
                                .padding(8.dp),
                        )
                        is Rich.Table -> MdTable(b.headers, b.rows, lang)
                    }
                }
                }
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
                    mood = if (busy.contains(w.id)) WindMood.THINKING else WindMood.IDLE,
                    accent = Color(w.argb),
                    style = w.faceStyle,
                )
            }
        }
    }
}
