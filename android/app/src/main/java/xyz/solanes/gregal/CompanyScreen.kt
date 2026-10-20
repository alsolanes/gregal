package xyz.solanes.gregal

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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.text.ClickableText
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
    val conn by vm.conn.collectAsState()
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
                Text(tr(lang, if (conn.local) "app.vents.localMode" else "app.vents.serverMode", conn.host),
                    color = Muted, fontSize = 12.sp, modifier = Modifier.padding(horizontal = 20.dp, vertical = 4.dp))
                TabRow(selectedTabIndex = if (manage) 1 else 0, containerColor = Superficie) {
                    Tab(selected = !manage, onClick = { manage = false }, text = { Text(tr(lang, "app.vents.conversations")) })
                    Tab(selected = manage, onClick = { manage = true }, text = { Text(tr(lang, "app.vents.myWinds")) })
                }
                LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = 96.dp, top = 12.dp)) {
                    if (manage) {
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
    Column(Modifier.fillMaxWidth().padding(horizontal = 32.dp, vertical = 42.dp),
        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(12.dp)) {
        WindFace(size = 88, accent = Color(0xFFF3BD8B), style = windStyleOf("bsn"))
        Text(tr(lang, if (conversations) "app.vents.emptyTitle" else "app.company.windEmptyTitle"), color = Ink,
            fontSize = 21.sp, fontWeight = FontWeight.SemiBold, textAlign = TextAlign.Center)
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

/** Una conversa de la llista: cara, nom, darrer missatge. */
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
    Row(
        Modifier.fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        // Avatar: el primer vent del grup; rotllanes amb cara apilada.
        Box(contentAlignment = Alignment.Center) {
            WindFace(size = 48, accent = accent, style = style)
            if (faces.size > 1) {
                faces.getOrNull(1)?.let {
                    Box(Modifier.align(Alignment.BottomEnd)) { WindFace(size = 28, accent = Color(it.argb), style = it.faceStyle) }
                }
            }
        }
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(title, fontSize = 16.sp, fontWeight = FontWeight.SemiBold, color = Ink, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(
                subtitle,
                fontSize = 13.sp, color = if (busy) Sea else Muted,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
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
    LaunchedEffect(msgs.size) { if (msgs.isNotEmpty()) listState.animateScrollToItem(msgs.size - 1) }

    Column(Modifier.fillMaxSize().background(accent.copy(alpha = 0.05f))) {
        // Capçalera: cara viva + nom + editar
        Row(
            Modifier.fillMaxWidth().background(accent.copy(alpha = 0.12f)).clickable { profile = true }.padding(horizontal = 8.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            IconButton(onClick = onBack) { Icon(Icons.Rounded.ArrowBack, tr(lang, "app.vents.back"), tint = Ink) }
            WindFace(size = 44, mood = if (busy.contains(wind.id)) WindMood.THINKING else WindMood.IDLE, accent = accent, style = wind.faceStyle)
            Column(Modifier.weight(1f)) {
                Text(wind.name, fontSize = 17.sp, fontWeight = FontWeight.Bold, color = Ink)
                if (wind.role.isNotBlank()) Text(wind.role, fontSize = 12.sp, color = Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            IconButton(onClick = { profile = true }) {
                Icon(Icons.Rounded.Info, tr(lang, "app.vents.profile"), tint = Muted, modifier = Modifier.size(20.dp))
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
            items(msgs.size) { idx -> WindBubble(msgs[idx], accent, lang = lang) }
            if (busy.contains(wind.id)) {
                item {
                    Text(
                        tr(lang, "app.company.windThinking", wind.name),
                        fontSize = 12.sp, color = Muted, textAlign = TextAlign.Center,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
        }
        // Input
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            OutlinedTextField(
                value = text,
                onValueChange = { text = it },
                placeholder = { Text(tr(lang, "app.company.windChatHint", wind.name), fontSize = 14.sp) },
                shape = RoundedCornerShape(22.dp),
                modifier = Modifier.weight(1f),
                maxLines = 3,
            )
            FilledIconButton(
                onClick = { vm.sendWindMessage(wind.id, text); text = "" },
                enabled = text.isNotBlank() && !busy.contains(wind.id),
                shape = androidx.compose.foundation.shape.CircleShape,
            ) { Icon(Icons.Rounded.Send, tr(lang, "app.vents.send"), tint = Color.White) }
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
    LaunchedEffect(msgs.size) { if (msgs.isNotEmpty()) listState.animateScrollToItem(msgs.size - 1) }

    Column(Modifier.fillMaxSize()) {
        Row(
            Modifier.fillMaxWidth().background(Sea.copy(alpha = 0.10f)).clickable { groupProfile = true }.padding(horizontal = 8.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            IconButton(onClick = onBack) { Icon(Icons.Rounded.ArrowBack, tr(lang, "app.vents.back"), tint = Ink) }
            Box {
                WindFace(size = 44, accent = faces.firstOrNull()?.let { Color(it.argb) } ?: Sea, style = faces.firstOrNull()?.faceStyle ?: WindStyle())
                faces.getOrNull(1)?.let {
                    Box(Modifier.align(Alignment.BottomEnd)) { WindFace(size = 28, accent = Color(it.argb), style = it.faceStyle) }
                }
            }
            Column(Modifier.weight(1f)) {
                Text(rot.name, fontSize = 17.sp, fontWeight = FontWeight.Bold, color = Ink)
                Text(faces.joinToString(", ") { it.name }, fontSize = 12.sp, color = Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
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
                WindBubble(resolved.message, resolved.wind?.let { Color(it.argb) } ?: Sea,
                    lang = lang, speaker = resolved.wind,
                    speakerName = resolved.name.ifBlank { tr(lang, "app.vents.windSpeaker") },
                    onSpeakerClick = { resolved.wind?.let { speakerProfile = it } })
            }
            if (busy.contains(rot.id)) {
                item {
                    Text(tr(lang, "app.company.windThinking", "…"), fontSize = 12.sp, color = Muted, textAlign = TextAlign.Center, modifier = Modifier.fillMaxWidth())
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
                shape = RoundedCornerShape(22.dp),
                modifier = Modifier.weight(1f),
                maxLines = 3,
            )
            FilledIconButton(
                onClick = { vm.sendRotllanaMessage(rot.id, text); text = "" },
                enabled = text.isNotBlank() && !busy.contains(rot.id),
                shape = androidx.compose.foundation.shape.CircleShape,
            ) { Icon(Icons.Rounded.Send, tr(lang, "app.vents.send"), tint = Color.White) }
        }
    }
    if (groupProfile) GroupProfileDialog(vm, lang, rot, winds) { groupProfile = false }
    speakerProfile?.let { WindProfileDialog(vm, lang, it) { speakerProfile = null } }
}

/**
 * Bombolla d'un vent: ell a l'esquerra (color seu), tu a la dreta (mar).
 * El text del vent és markdown: enllaços clicables, negreta i codi.
 */
@Composable
internal fun WindBubble(m: GregalViewModel.PetMsg, accent: Color, lang: String = "ca", speaker: Wind? = null,
    speakerName: String = "", onSpeakerClick: (() -> Unit)? = null) {
    val mine = m.from == "user"
    Row(
        Modifier.fillMaxWidth(),
        horizontalArrangement = if (mine) Arrangement.End else Arrangement.Start,
    ) {
        Surface(
            shape = RoundedCornerShape(
                topStart = 14.dp, topEnd = 14.dp,
                bottomStart = if (mine) 14.dp else 3.dp,
                bottomEnd = if (mine) 3.dp else 14.dp,
            ),
            color = if (mine) Sea.copy(alpha = 0.14f) else accent.copy(alpha = 0.14f),
            modifier = Modifier.widthIn(max = 320.dp),
        ) {
            if (mine) {
                Text(
                    m.text,
                    fontSize = 14.sp,
                    color = Tinta,
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
                )
            } else {
                val uri = LocalUriHandler.current
                Column(Modifier.padding(horizontal = 12.dp, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    if (speakerName.isNotBlank()) {
                        Row(Modifier.then(if (speaker != null && onSpeakerClick != null) Modifier.clickable(onClick = onSpeakerClick) else Modifier),
                            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            if (speaker != null) WindFace(size = 28, accent = Color(speaker.argb), style = speaker.faceStyle)
                            Text(speakerName, color = Ink, fontSize = 12.sp, fontWeight = FontWeight.Bold)
                        }
                    }
                    for (b in groupRich(splitMd(m.text))) {
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
