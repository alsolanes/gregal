package xyz.solanes.gregal

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.border
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
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
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.Groups
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.Menu
import androidx.compose.material.icons.rounded.Tune
import androidx.compose.material.icons.rounded.Search
import androidx.compose.material.icons.rounded.Close
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.platform.testTag
import androidx.compose.material.icons.rounded.ArrowBack
import androidx.compose.material.icons.rounded.Star
import androidx.compose.material.icons.rounded.StarBorder
import androidx.compose.material.icons.rounded.Folder
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.text.ClickableText
import androidx.compose.ui.layout.onSizeChanged
import kotlinx.coroutines.launch
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.compositeOver
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
    var feinesOpen by remember { mutableStateOf(false) }
    val windTasks by vm.windTasks.collectAsState()
    // Feines en marxa: es tornen a mirar cada 8 s (i cada minut si no n'hi ha
    // cap), així el progrés es reprèn en obrir l'app i l'avís arriba en acabar.
    val anyRunning = windTasks.orEmpty().any { it.running }
    LaunchedEffect(anyRunning) {
        while (true) {
            vm.loadWindTasks()
            kotlinx.coroutines.delay(if (anyRunning) 8_000 else 60_000)
        }
    }
    // El fil d'una feina en marxa rep missatges de la colla: es torna a llegir
    // perquè la conversa pugi a dalt amb l'hora bona.
    LaunchedEffect(windTasks) {
        windTasks.orEmpty().filter { it.running && it.thread.isNotBlank() }.forEach { vm.loadRotllanaChat(it.thread) }
    }
    val winds by vm.winds.collectAsState()
    val rotllanes by vm.rotllanes.collectAsState()
    val chats by vm.windChat.collectAsState()
    val rChats by vm.rotllanaChat.collectAsState()
    val wmeta by vm.windsMeta.collectAsState()
    val busy by vm.windBusy.collectAsState()
    val rBusy by vm.rotllanaBusy.collectAsState()
    val live by vm.windLive.collectAsState()
    val news by vm.windNews.collectAsState()
    val conn by vm.conn.collectAsState()
    // Veus proactives per obrir: es consulten en obrir els Vents i cada minut.
    LaunchedEffect(conn) { while (true) { vm.refreshWindNews(); kotlinx.coroutines.delay(60_000) } }
    // La mascota: en obrir els Vents (o canviar de lloc) diu què ha passat mentrestant.
    LaunchedEffect(conn) { vm.loadCap() }
    // Quan un vent parla sol (recordatori, rutina) es torna a llegir el seu
    // xat: així puja a dalt de la llista.
    LaunchedEffect(news) { news.forEach { vm.loadWindChat(it) } }
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
    var manage by rememberSaveable { mutableStateOf(false) }
    var searching by rememberSaveable { mutableStateOf(false) }
    var query by rememberSaveable { mutableStateOf("") }
    var profile by remember { mutableStateOf<Wind?>(null) }
    var rowTarget by remember { mutableStateOf<RowTarget?>(null) }
    var sectionPicker by remember { mutableStateOf(false) }
    var sectionTargetKey by remember { mutableStateOf<String?>(null) }
    var programatsOpen by remember { mutableStateOf(false) }
    var wikiOpen by remember { mutableStateOf(false) }
    var wikiFromChat by remember { mutableStateOf<ChatTarget?>(null) }
    val wikiBusy by vm.wikiBusy.collectAsState()
    val creator by vm.windCreator.collectAsState()
    LaunchedEffect(Unit) { vm.loadWinds(); vm.loadRotllanes(); vm.loadWindsMeta() }
    LaunchedEffect(winds.map { it.id }, rotllanes.map { it.id }) {
        winds.forEach { vm.loadWindChat(it.id) }
        rotllanes.forEach { vm.loadRotllanaChat(it.id) }
    }
    androidx.activity.compose.BackHandler(enabled = manage && open == null && !creator.first) { manage = false }
    androidx.activity.compose.BackHandler(enabled = searching && !manage && open == null && !creator.first && creation == null) {
        searching = false; query = ""
    }
    val crewUi by vm.crew.collectAsState()
    crewUi?.let { CrewSheet(vm, lang, it, winds) }
    if (creator.first) {
        WindCreatorScreen(vm, lang, creator.second)
    } else if (creation != null) {
        ConversationCreator(vm, lang, winds, group = creation == "group") { creation = null }
    } else if (open != null) {
        val id = open!!.substringAfter(':')
        val wind = if (open!!.startsWith("wind:")) winds.firstOrNull { it.id == id } else null
        val rot = if (open!!.startsWith("rot:")) rotllanes.firstOrNull { it.id == id } else null
        when {
            wind != null -> WindConversation(vm, lang, wind, { vm.openVentsChat(null) }) { wikiFromChat = ChatTarget(wind.name, wind.id, null) }
            rot != null -> RotllanaConversation(vm, lang, rot, winds, { vm.openVentsChat(null) }) { wikiFromChat = ChatTarget(rot.name, null, rot.id) }
            else -> LaunchedEffect(open) { vm.openVentsChat(null) }
        }
    } else {
        Scaffold(containerColor = Superficie,
            topBar = {
                Column {
                    TopAppBar(
                        title = { Text(tr(lang, if (manage) "app.vents.myWinds" else "app.vents.title"), fontSize = 20.sp, fontWeight = FontWeight.SemiBold, color = Ink) },
                        navigationIcon = {
                            IconButton(onClick = { if (manage) manage = false else openDrawer() }) {
                                Icon(if (manage) Icons.Rounded.ArrowBack else Icons.Rounded.Menu,
                                    tr(lang, if (manage) "app.vents.back" else "app.nav.menu"), tint = Ink)
                            }
                        },
                        actions = {
                            if (!manage) {
                                IconButton(onClick = { searching = !searching; if (!searching) query = "" }) {
                                    Icon(if (searching) Icons.Rounded.Close else Icons.Rounded.Search,
                                        tr(lang, if (searching) "app.vents.closeSearch" else "app.vents.search"), tint = Ink)
                                }
                                IconButton(onClick = { manage = true }) {
                                    Icon(Icons.Rounded.Tune, tr(lang, "app.vents.manage"), tint = Ink)
                                }
                            }
                            Box {
                                IconButton(onClick = { menu = true }) {
                                    Icon(Icons.Rounded.Add, tr(lang, "app.vents.create"), tint = Ink)
                                }
                                DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                                    DropdownMenuItem(text = { Text(tr(lang, "app.vents.newChat")) }, onClick = { menu = false; creation = "chat" })
                                    DropdownMenuItem(text = { Text(tr(lang, "app.company.rotllanaNew")) }, onClick = { menu = false; creation = "group" })
                                    DropdownMenuItem(text = { Text(tr(lang, "app.company.windNew")) }, onClick = { menu = false; vm.showWindCreator(true) })
                                    DropdownMenuItem(text = { Text(tr(lang, "app.crew.title")) }, onClick = { menu = false; vm.openCrew() })
                                }
                            }
                        },
                        colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie),
                    )
                    if (!searching && !manage) WhereSwitch(vm, lang)
                    if (searching && !manage) {
                        OutlinedTextField(
                            query, { query = it }, singleLine = true,
                            placeholder = { Text(tr(lang, "app.vents.searchHint"), fontSize = 14.sp) },
                            leadingIcon = { Icon(Icons.Rounded.Search, null) },
                            shape = RoundedCornerShape(24.dp),
                            modifier = Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 6.dp),
                            colors = OutlinedTextFieldDefaults.colors(
                                focusedContainerColor = SuperficieAlta.copy(alpha = .45f),
                                unfocusedContainerColor = SuperficieAlta.copy(alpha = .45f),
                                focusedBorderColor = Sea, unfocusedBorderColor = Color.Transparent,
                            ),
                        )
                    }
                }
            }) { pad ->
            LazyColumn(Modifier.fillMaxSize().padding(pad), contentPadding = PaddingValues(bottom = 24.dp)) {
                if (manage) {
                    item { WindsModelRow(vm, lang) }
                    item { SquadsRow(vm, lang) }
                    item { Text(tr(lang, "app.vents.manageHelp"), color = Muted, fontSize = 13.sp, lineHeight = 18.sp, modifier = Modifier.padding(horizontal = 20.dp, vertical = 12.dp)) }
                    if (winds.isEmpty()) item { VentsEmpty(lang, false) }
                    items(winds, key = { it.id }) { w ->
                        ConversationRow(w.name, w.role, Color(w.argb), w.faceStyle, onClick = { profile = w })
                    }
                } else {
                    if (query.isBlank()) item { CapHero(vm, lang, winds) }
                    if (winds.isNotEmpty() && query.isBlank()) item {
                        VentsCrew(lang, winds, busy, live, news) { vm.openVentsChat("wind:" + it.id) }
                    }
                    // Programats, wiki i feines en una sola fila: les converses
                    // han de quedar a dalt, no sota tres targetes.
                    if (query.isBlank() && (winds.isNotEmpty() || rotllanes.isNotEmpty())) item {
                        VentsShortcuts(lang, windTasks.orEmpty().count { it.running }, !windTasks.isNullOrEmpty(),
                            onProgramats = { programatsOpen = true }, onWiki = { wikiOpen = true }, onFeines = { feinesOpen = true })
                    }
                    /** Fila amb favorit: estrella viva si hi és, i el menú llarg la marca. */
                    @Composable
                    fun InboxRow(entry: VentsInboxRow, showSectionHeader: String?) {
                        if (showSectionHeader != null) {
                            Text(showSectionHeader.uppercase(), color = Muted, fontSize = 11.sp,
                                fontWeight = FontWeight.SemiBold, letterSpacing = 0.8.sp,
                                modifier = Modifier.padding(start = 20.dp, top = 18.dp, bottom = 4.dp))
                        }
                        val id = entry.key.substringAfter(':')
                        val group = entry.key.startsWith("rot:")
                        val thinking = if (group) id in rBusy else id in busy
                        val unread = !group && id in news
                        val face = entry.members.firstOrNull()
                        ConversationRow(entry.title, entry.preview, face?.let { Color(it.argb) } ?: Sea,
                            face?.faceStyle ?: WindStyle(), faces = if (group) entry.members else emptyList(),
                            busy = thinking, mood = if (thinking) live[id]?.mood else if (unread) WindMood.NEWS else null,
                            timestamp = entry.timestamp,
                            favorite = entry.key in wmeta.favorites,
                            onFavorite = { vm.toggleFavorite(entry.key) },
                            onClick = { vm.openVentsChat(entry.key) },
                            onLongClick = {
                                if (group) rotllanes.firstOrNull { it.id == id }?.let { rowTarget = RowTarget.Rot(it) }
                                else face?.let { rowTarget = RowTarget.Dm(it) }
                            }, lang = lang)
                    }

                    val rows = ventsInbox(winds, rotllanes, chats, rChats, query)
                    if (rows.isEmpty()) item {
                        if (query.isBlank()) VentsEmpty(lang, true)
                        else Column(Modifier.fillMaxWidth().padding(32.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            Text(tr(lang, "app.vents.searchEmpty"), color = Ink, fontWeight = FontWeight.SemiBold)
                            Text(tr(lang, "app.vents.searchEmptyHelp"), color = Muted, fontSize = 14.sp)
                        }
                    }
                    // Les converses de veritat primer; els vents amb qui encara no
                    // has parlat, a sota amb el seu títol. Abans tot anava barrejat i
                    // una fila «Xaloc · Música» semblava una conversa buida.
                    val firstIdle = rows.indexOfFirst { !it.hasMessages }
                    val idleKey = if (query.isBlank() && firstIdle > 0) rows[firstIdle].key else null
                    // Grups: favorits a dalt, després les seccions temàtiques, i la
                    // resta a baix; dins de cada grup, la més recent primer.
                    val grouped = if (query.isBlank()) groupInbox(rows, wmeta, lang) else null
                    if (grouped != null) {
                        grouped.forEach { g ->
                            item(key = "hdr:" + g.header) {
                                InboxHeader(g.header, count = g.rows.size, favorite = g.isFavorites, lang = lang, onAddSection = { sectionPicker = true })
                            }
                            items(g.rows, key = { it.key }) { entry ->
                                if (entry.key == idleKey) InboxDivider(lang)
                                InboxRow(entry, null)
                            }
                        }
                    } else {
                        items(rows, key = { it.key }) { entry ->
                            if (entry.key == idleKey) InboxDivider(lang)
                            InboxRow(entry, null)
                        }
                    }
                }
            }
        }
    }
    if (programatsOpen) ProgramatsDialog(vm, lang) { programatsOpen = false }
    if (feinesOpen) FeinesDialog(vm, lang, onOpenThread = { vm.openVentsChat("rot:$it") }) { feinesOpen = false }
    if (wikiOpen) WikiDialog(vm, lang) { wikiOpen = false }
    // Triar o crear una secció temàtica: des del botó de la capçalera dels
    // favorits (secció nova) o des del menú llarg d'una fila (afegir-hi).
    if (sectionPicker) SectionPickerDialog(vm, lang, sectionTargetKey) { sectionPicker = false; sectionTargetKey = null }
    // «Resumeix a la wiki» des d'un xat: confirmació + estat en viu i avís en acabar.
    wikiFromChat?.let { target ->
        AlertDialog(
            onDismissRequest = { if (!wikiBusy) wikiFromChat = null },
            title = { Text(tr(lang, "app.wiki.summarize")) },
            text = { Text(if (wikiBusy) tr(lang, "app.wiki.summarizing") else tr(lang, "app.wiki.emptyHelp"), color = if (wikiBusy) Muted else Tinta) },
            confirmButton = {
                TextButton(enabled = !wikiBusy, onClick = {
                    vm.summarizeToWiki(target.windId, target.rotId) { ok ->
                        if (ok) vm.notify(tr(lang, "app.wiki.summarizeDone"))
                    }
                    wikiFromChat = null
                }) { Text(tr(lang, "app.wiki.summarize"), color = Sea) }
            },
            dismissButton = { TextButton(enabled = !wikiBusy, onClick = { wikiFromChat = null }) { Text("✕", color = Muted) } },
        )
    }
    profile?.let { w -> WindProfileDialog(vm, lang, w) { profile = null } }
    rowTarget?.let { t ->
        ChatRowDialog(vm, lang, t, onToSection = { key -> sectionTargetKey = key; sectionPicker = true }) { rowTarget = null }
    }
}

/** L'estat d'un vent en paraules (mai només amb la insígnia); null si està en repòs. */
internal fun windStateLabel(lang: String, mood: WindMood, busy: Boolean): String? = when (mood) {
    WindMood.SEARCHING -> tr(lang, "app.vents.gridSearching")
    WindMood.WRITING -> tr(lang, "app.vents.gridWriting")
    WindMood.WORKING -> tr(lang, "app.vents.gridWorking")
    WindMood.DISCUSSING -> tr(lang, "app.vents.gridDiscussing")
    WindMood.NEWS -> tr(lang, "app.vents.gridNews")
    else -> if (busy) tr(lang, "app.vents.gridThinking") else null
}

/** Una sola fila de mascotes: les converses sempre queden al primer viewport. */
@Composable
internal fun VentsCrew(
    lang: String, winds: List<Wind>, busy: Set<String>,
    live: Map<String, WindLive> = emptyMap(), news: Set<String> = emptySet(), onOpen: (Wind) -> Unit,
) {
    LazyRow(
        modifier = Modifier.fillMaxWidth().testTag("vents-strip"),
        // Columnes de 78 dp: la cinquena mascota treu mitja cara pel marge i
        // convida a lliscar (abans només en sobresortia un tros d'etiqueta).
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 12.dp, bottom = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        items(winds, key = { it.id }) { w ->
            val thinking = w.id in busy
            val mood = when {
                thinking -> live[w.id]?.mood ?: WindMood.THINKING
                w.id in news -> WindMood.NEWS
                else -> WindMood.IDLE
            }
            Column(
                Modifier.width(78.dp).clip(RoundedCornerShape(18.dp))
                    .clickable { onOpen(w) }.semantics { contentDescription = w.name }
                    .padding(vertical = 4.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(5.dp),
            ) {
                WindHalo(Color(w.argb), w.faceStyle, size = 64, mood = mood, online = false, showKit = true)
                Text(w.name, color = Ink, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                windStateLabel(lang, mood, thinking)?.let {
                    Text(it, color = Muted, fontSize = 10.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
        }
    }
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
private fun ChatRowDialog(vm: GregalViewModel, lang: String, target: RowTarget, onToSection: (String) -> Unit, onClose: () -> Unit) {
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
                        val sectionKey = when (target) {
                            is RowTarget.Dm -> "wind:" + target.wind.id
                            is RowTarget.Rot -> "rot:" + target.rot.id
                        }
                        TextButton(
                            onClick = { mode = 1 },
                            modifier = Modifier.fillMaxWidth(),
                            contentPadding = PaddingValues(vertical = 10.dp),
                        ) { Text(tr(lang, "app.vents.chatRename"), modifier = Modifier.fillMaxWidth()) }
                        TextButton(
                            onClick = { onToSection(sectionKey); onClose() },
                            modifier = Modifier.fillMaxWidth(),
                            contentPadding = PaddingValues(vertical = 10.dp),
                        ) { Text(tr(lang, "app.vents.toSection"), modifier = Modifier.fillMaxWidth()) }
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

/** Un grup de la bústia: favorits, una secció o la resta de converses. */
internal data class InboxGroup(
    val header: String,
    val rows: List<VentsInboxRow>,
    val isFavorites: Boolean = false,
)

/**
 *parteix la bústia en grups: favorits primer, després les seccions temàtiques
 * (en l'ordre que les has creat) i al final la resta. Dins de cada grup es manté
 * l'ordre per recència. Les files d'una secció no es repeteixen a la resta.
 */
internal fun groupInbox(rows: List<VentsInboxRow>, meta: GregalApi.WindsMeta, lang: String = "ca"): List<InboxGroup> {
    val groups = mutableListOf<InboxGroup>()
    val claimed = mutableSetOf<String>()
    val favs = rows.filter { it.key in meta.favorites }
    if (favs.isNotEmpty()) {
        favs.forEach { claimed.add(it.key) }
        groups += InboxGroup(tr(lang, "app.vents.favorites"), favs, isFavorites = true)
    }
    for (s in meta.sections) {
        val inSec = rows.filter { it.key in s.items && it.key !in claimed }
        if (inSec.isEmpty()) continue
        inSec.forEach { claimed.add(it.key) }
        groups += InboxGroup(s.name, inSec)
    }
    val rest = rows.filter { it.key !in claimed }
    if (rest.isNotEmpty()) groups += InboxGroup(tr(lang, "app.vents.allChats"), rest)
    return groups
}

/** Capçalera d'un grup de la bústia: títol, compte i (als favorits) crear secció. */
@Composable
internal fun InboxHeader(title: String, count: Int, favorite: Boolean = false, lang: String = "ca", onAddSection: () -> Unit = {}) {
    Row(
        Modifier.fillMaxWidth().padding(start = 20.dp, end = 12.dp, top = 18.dp, bottom = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(title.uppercase(), color = Muted, fontSize = 11.sp,
            fontWeight = FontWeight.SemiBold, letterSpacing = 0.8.sp,
            modifier = Modifier.weight(1f))
        if (favorite) IconButton(onClick = onAddSection, modifier = Modifier.size(28.dp)) {
            Icon(Icons.Rounded.Folder, tr(lang, "app.vents.newSection"), tint = Muted, modifier = Modifier.size(16.dp))
        }
    }
}

/** «Comença un xat»: el tall entre converses i vents sense parlar. */
@Composable
internal fun InboxDivider(lang: String) {
    Text(tr(lang, "app.vents.startChat").uppercase(), color = Muted, fontSize = 11.sp,
        fontWeight = FontWeight.SemiBold, letterSpacing = 0.8.sp,
        modifier = Modifier.padding(start = 20.dp, top = 18.dp, bottom = 4.dp))
}

/** Fila de missatgeria, sense targeta: la mascota porta tot el color. */
@OptIn(ExperimentalFoundationApi::class)
@Composable
internal fun ConversationRow(
    title: String,
    subtitle: String,
    accent: Color,
    style: WindStyle,
    busy: Boolean = false,
    mood: WindMood? = null,
    faces: List<Wind> = emptyList(),
    timestamp: Long = 0L,
    favorite: Boolean = false,
    onFavorite: (() -> Unit)? = null,
    onClick: () -> Unit,
    onLongClick: (() -> Unit)? = null,
    lang: String = "ca",
) {
    val unread = mood == WindMood.NEWS
    val state = if (unread) null else windStateLabel(lang, mood ?: WindMood.IDLE, busy)
    val receipt = remember(timestamp, lang) {
        if (timestamp > 0L) inboxWhen(timestamp, lang) else ""
    }
    Row(
        Modifier.fillMaxWidth().heightIn(min = 80.dp)
            .combinedClickable(onClick = onClick, onLongClick = onLongClick)
            .padding(horizontal = 20.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        // Un vent sol porta l'eina del seu ofici també a la llista: és com es
        // distingeixen els especialistes d'un cop d'ull, no només pel color.
        WindHalo(accent, style, size = 48, stacked = faces.drop(1), online = false,
            mood = mood ?: if (busy) WindMood.THINKING else WindMood.IDLE, showKit = faces.size <= 1)
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                Text(title, fontSize = 15.sp, fontWeight = if (unread) FontWeight.Bold else FontWeight.SemiBold,
                    color = Ink, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                if (onFavorite != null) Icon(
                    if (favorite) Icons.Rounded.Star else Icons.Rounded.StarBorder,
                    null, tint = if (favorite) Sea else Muted,
                    modifier = Modifier.size(16.dp).clickable(onClick = onFavorite))
                if (receipt.isNotBlank()) Text(receipt, color = Muted, fontSize = 11.sp)
            }
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(state ?: subtitle, fontSize = 13.sp, color = if (unread) Ink else Muted,
                    fontWeight = if (unread) FontWeight.Medium else FontWeight.Normal,
                    maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                if (busy) TypingDots(Sea)
                else if (unread) Box(Modifier.size(7.dp).background(Sea, CircleShape))
            }
        }
    }
}

/** El xat d'un vent a pantalla completa: capçalera amb la cara, bombolles, input. */
@Composable
internal fun WindConversation(vm: GregalViewModel, lang: String, wind: Wind, onBack: () -> Unit, onWiki: () -> Unit = {}) {
    val chats by vm.windChat.collectAsState()
    val busy by vm.windBusy.collectAsState()
    val live by vm.windLive.collectAsState()
    val msgs = chats[wind.id].orEmpty()
    val clipboard = LocalClipboardManager.current
    var field by rememberSaveable(wind.id, stateSaver = TextFieldValue.Saver) { mutableStateOf(TextFieldValue("")) }
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
        Column(Modifier.fillMaxSize()) {
        VentsChatHeader(lang, wind.name,
            if (isBusy) tr(lang, "app.company.windThinking", wind.name) else wind.role,
            accent, wind.faceStyle,
            if (isBusy) live[wind.id]?.mood ?: WindMood.THINKING else WindMood.IDLE,
            onBack = onBack, onProfile = { profile = true },
            onWiki = onWiki)
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
        VentsComposer(lang, field, { field = it }, tr(lang, "app.company.windChatHint", wind.name), canSend) {
            val body = field.text
            val target = resolveMention(body, winds)
            if (target != null && target.id != wind.id) {
                if (busy.contains(target.id)) vm.notify(tr(lang, "app.vents.handoffBusy", target.name))
                else {
                    vm.sendWindMessage(target.id, body)
                    vm.notify(tr(lang, "app.vents.handoffSent", target.name))
                    field = TextFieldValue("")
                }
            } else {
                vm.sendWindMessage(wind.id, body); field = TextFieldValue("")
            }
        }
        }
    }
    if (profile) WindProfileDialog(vm, lang, wind) { profile = false }
}

/** El xat d'una rotllana a pantalla completa. */
@Composable
private fun RotllanaConversation(vm: GregalViewModel, lang: String, rot: Rotllana, winds: List<Wind>, onBack: () -> Unit, onWiki: () -> Unit = {}) {
    val chats by vm.rotllanaChat.collectAsState()
    val busy by vm.rotllanaBusy.collectAsState()
    val live by vm.windLive.collectAsState()
    val msgs = chats[rot.id].orEmpty()
    val clipboard = LocalClipboardManager.current
    var field by rememberSaveable(rot.id, stateSaver = TextFieldValue.Saver) { mutableStateOf(TextFieldValue("")) }
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
        Column(Modifier.fillMaxSize()) {
        val crewSteps by vm.crewStep.collectAsState()
        val crewStep = crewSteps[rot.id]
        VentsChatHeader(lang, rot.name,
            when {
                crewStep != null -> tr(lang, "app.crew.step.$crewStep")
                // El títol de la rotllana és la primera pregunta: «Recomana'm… hi
                // pensa» no deia res. Qui pensa és la colla.
                rotBusy -> tr(lang, "app.company.groupThinking")
                else -> faces.joinToString(", ") { it.name }
            },
            rotAccent, faces.firstOrNull()?.faceStyle ?: WindStyle(),
            if (rotBusy) live[rot.id]?.mood ?: WindMood.THINKING else WindMood.IDLE, faces,
            onBack = onBack, onProfile = { groupProfile = true },
            onWiki = onWiki)
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
                // Aire segons bàndol: juntes les del mateix, separades en canviar.
                val gap = if (firstOfRun && idx > 0) 16.dp else 4.dp
                Column(Modifier.padding(top = gap)) {
                BubbleIn("rot#$idx", animate = idx >= liveFrom) {
                WindBubble(resolved.message, if (resolved.message.from == "user") rotAccent else resolved.wind?.let { Color(it.argb) } ?: Sea,
                    lang = lang, speaker = if (firstOfRun) resolved.wind else null,
                    speakerName = if (firstOfRun) resolved.name.ifBlank { tr(lang, "app.vents.windSpeaker") } else "",
                    onSpeakerClick = { resolved.wind?.let { speakerProfile = it } },
                    indent = 0.dp,
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
                            "  " + tr(lang, "app.company.groupThinking"),
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
        VentsComposer(lang, field, { field = it }, tr(lang, "app.company.rotllanaHint"), canSend) {
            val body = field.text
            val target = resolveMention(body, faces)
            vm.sendRotllanaMessage(rot.id, body, target?.id.orEmpty()); field = TextFieldValue("")
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
        bmp != null -> ChatImageFrame(bmp!!, url, alt, lang)
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
    val displayName = speakerName.ifBlank { speaker?.name.orEmpty() }
    // Bombolla pròpia suau en tots dos temes: el blanc ple sobre fons fosc
    // enlluernava i pesava més que les respostes.
    val ownBackground = Ink.copy(alpha = 0.11f)
    val ownText = Ink
    // L'hora arriba del backend en segons Unix; els missatges creats en aquesta
    // sessió (enviats per l'app) són en millis. Unificat a millis una sola volta.
    val tsMillis = remember(m.ts) { if (m.ts > 10_000_000_000L) m.ts else m.ts * 1000L }
    val receipt = remember(tsMillis, lang) {
        if (tsMillis > 0L) java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
            .format(java.util.Date(tsMillis)) else ""
    }
    if (mine) {
        Column(Modifier.fillMaxWidth().padding(start = indent), horizontalAlignment = Alignment.End) {
            Surface(
                shape = RoundedCornerShape(
                    topStart = 20.dp, topEnd = 20.dp,
                    bottomStart = 20.dp, bottomEnd = 6.dp,
                ),
                color = ownBackground,
                shadowElevation = 0.dp,
                // Prémer llarg copia el missatge en text pla (sense marques).
                modifier = Modifier.widthIn(max = 300.dp).combinedClickable(
                    onClick = {}, onLongClick = { onCopy?.invoke() },
                ),
            ) {
                Text(
                    m.text,
                    fontSize = 15.sp,
                    color = ownText,
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
        // La identitat del parlant queda dins la bombolla, sense avatar repetit ni etiqueta superposada.
        val bubbleShape = RoundedCornerShape(
            topStart = 20.dp, topEnd = 20.dp,
            bottomStart = 20.dp, bottomEnd = 20.dp,
        )
        Box {
            // Bombolla tenyida amb el color del vent: clara en tema clar (un
            // vel suau), més viva en tema fosc. Dots/Grok: cada persona
            // respira el seu propi color.
            val dark = LocalPal.current == DarkPal
            val tinted = if (dark) accent.copy(alpha = .30f).compositeOver(Superficie)
                else accent.copy(alpha = .14f).compositeOver(Superficie)
            // Una taula, un gràfic, codi o una imatge necessiten l'amplada de
            // la pantalla: a 300 dp una taula de cinc columnes no hi cabia.
            val rich = remember(m.text) { m.text.contains("\n|") || m.text.contains("```") || m.text.contains("![") }
            Surface(
                shape = bubbleShape,
                color = tinted,
                shadowElevation = 0.dp,
                // Prémer llarg copia la resposta en text pla (sense marques).
                modifier = Modifier.widthIn(max = if (rich) 600.dp else 300.dp).combinedClickable(
                    onClick = {}, onLongClick = { onCopy?.invoke() },
                ),
            ) {
                val uri = LocalUriHandler.current
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
                        .padding(vertical = 10.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    if (displayName.isNotBlank()) {
                        Text(displayName, color = link, fontSize = 11.sp, fontWeight = FontWeight.SemiBold,
                            modifier = Modifier.then(if (onSpeakerClick != null) Modifier
                                .heightIn(min = 32.dp).clickable(onClick = onSpeakerClick) else Modifier))
                    }
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
                            Text(tr(lang, "app.vents.continue"), color = link,
                                fontSize = 13.sp, fontWeight = FontWeight.Bold)
                        }
                    }
                    // Hora de la bombolla: tot missatge durador en porta una;
                    // els antics (ts=0) i els que estan en directe, no.
                    if (tsMillis > 0L && !m.streaming) {
                        Text(receipt, fontSize = 10.sp,
                            color = Tinta.copy(alpha = 0.55f),
                            modifier = Modifier.padding(top = 2.dp),
                        )
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
    val live by vm.windLive.collectAsState()
    val news by vm.windNews.collectAsState()
    val w = wind ?: return
    Box(
        Modifier
            .fillMaxWidth()
            .fillMaxHeight(),
        // A l'esquerra: la dreta és dels botons d'acció de cada pantalla
        // (afegir una feina, crear…), i la cara no els ha de tapar.
        contentAlignment = Alignment.BottomStart,
    ) {
        Surface(
            shape = CircleShape,
            shadowElevation = 6.dp,
            color = Superficie,
            border = androidx.compose.foundation.BorderStroke(1.dp, Line),
            modifier = Modifier
                .padding(16.dp)
                .size(58.dp)
                .clip(CircleShape)
                .clickable(onClick = onOpen)
                .semantics { contentDescription = "Vent actiu: ${w.name}" },
        ) {
            Box(contentAlignment = Alignment.Center) {
                WindFace(
                    size = 48,
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

/**
 * «Convoca una colla»: escrius l'encàrrec, Gregal proposa qui el pot fer (els
 * vents que ja hi encaixen i especialistes nous amb el seu color i ofici) i,
 * en acceptar, s'obre la rotllana on en parlen i després el fan.
 */
@Composable
private fun CrewSheet(vm: GregalViewModel, lang: String, ui: GregalViewModel.CrewUi, winds: List<Wind>) {
    var task by remember(ui.task) { mutableStateOf(ui.task) }
    Dialog(onDismissRequest = { if (!ui.starting) vm.closeCrew() }, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(shape = RoundedCornerShape(28.dp), color = Superficie, modifier = Modifier.fillMaxWidth().padding(16.dp)) {
            Column(Modifier.verticalScroll(rememberScrollState()).padding(20.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(tr(lang, "app.crew.title"), color = Ink, fontSize = 20.sp, fontWeight = FontWeight.Bold)
                Text(tr(lang, "app.crew.intro"), color = Muted, fontSize = 13.5.sp, lineHeight = 19.sp)
                OutlinedTextField(task, { task = it }, enabled = !ui.loading && !ui.starting,
                    placeholder = { Text(tr(lang, "app.crew.hint"), color = Faint) }, minLines = 2, maxLines = 5,
                    shape = RoundedCornerShape(16.dp), modifier = Modifier.fillMaxWidth())
                val p = ui.proposal
                if (p == null) {
                    if (ui.loading) Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        TypingDots(Sea); Text(tr(lang, "app.crew.thinking"), color = Muted, fontSize = 13.sp)
                    }
                    if (ui.error) Text(tr(lang, "app.crew.error"), color = Red, fontSize = 13.sp)
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp), modifier = Modifier.fillMaxWidth()) {
                        TextButton(onClick = { vm.closeCrew() }) { Text(tr(lang, "app.crew.cancel"), color = Muted) }
                        Spacer(Modifier.weight(1f))
                        PrimaryPill(tr(lang, "app.crew.propose"), { vm.proposeCrew(task) }, enabled = task.isNotBlank() && !ui.loading)
                    }
                } else {
                    if (p.plan.isNotBlank()) Text(p.plan, color = Tinta, fontSize = 14.sp, lineHeight = 20.sp)
                    p.reuse.forEach { r ->
                        val w = winds.firstOrNull { it.id == r.id } ?: Wind(r.id, r.name, "#BFCCA8", r.role)
                        CrewMemberRow(lang, w, r.why, isNew = false)
                    }
                    p.create.forEach { c -> CrewMemberRow(lang, c.wind, c.why, isNew = true) }
                    if (ui.error) Text(tr(lang, "app.crew.error"), color = Red, fontSize = 13.sp)
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically, modifier = Modifier.fillMaxWidth()) {
                        TextButton(onClick = { vm.closeCrew() }, enabled = !ui.starting) { Text(tr(lang, "app.crew.cancel"), color = Muted) }
                        Spacer(Modifier.weight(1f))
                        if (ui.starting) TypingDots(Sea)
                        else PrimaryPill(tr(lang, "app.crew.start"), { vm.startCrew() })
                    }
                }
            }
        }
    }
}

@Composable
private fun CrewMemberRow(lang: String, w: Wind, why: String, isNew: Boolean) {
    val accent = Color(w.argb)
    Row(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(18.dp))
            .background(if (isNew) Sea.copy(alpha = 0.08f) else SuperficieAlta.copy(alpha = 0.5f))
            .border(1.dp, if (isNew) Sea.copy(alpha = 0.35f) else Line, RoundedCornerShape(18.dp))
            .padding(10.dp),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        WindFace(size = 48, accent = accent, style = w.faceStyle, showKit = true)
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(w.name, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.Bold)
                if (isNew) Text(tr(lang, "app.crew.new").uppercase(), color = Sea, fontSize = 10.sp, fontWeight = FontWeight.SemiBold,
                    letterSpacing = 0.8.sp, modifier = Modifier.background(Sea.copy(alpha = 0.14f), RoundedCornerShape(999.dp)).padding(horizontal = 7.dp, vertical = 2.dp))
            }
            Text(w.role, color = Muted, fontSize = 12.5.sp)
            if (why.isNotBlank()) Text(why, color = Tinta, fontSize = 12.5.sp, lineHeight = 17.sp)
        }
    }
}

/** Un xat des d'on demanem un resum a la wiki (vent o rotllana). */
internal data class ChatTarget(val nom: String, val windId: String?, val rotId: String?)

/**
 * On viuen els vents, d'un toc: «Al mòbil» (tot es queda aquí, sense cap
 * ordinador) o al servidor desat (feines llargues que continuen amb l'app
 * tancada). El punt diu si el servidor respon. Cada lloc té els seus vents i
 * converses: el text de sota ho recorda.
 */
@Composable
internal fun WhereSwitch(vm: GregalViewModel, lang: String) {
    val conn by vm.conn.collectAsState()
    val reach by vm.serverReach.collectAsState()
    val switching by vm.switchingWhere.collectAsState()
    val local by vm.localState.collectAsState()
    val onServer = !conn.local
    val canLocal = remember { vm.localAvailable() }
    LaunchedEffect(onServer) {
        while (true) {
            vm.checkServerReach()
            kotlinx.coroutines.delay(60_000)
        }
    }
    val serverName = vm.savedServerName().ifBlank { tr(lang, "app.where.server") }
    Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 4.dp)) {
        Row(
            Modifier.fillMaxWidth().clip(RoundedCornerShape(22.dp)).background(SuperficieAlta.copy(alpha = .55f)).padding(4.dp),
            horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            @Composable
            fun Option(selected: Boolean, enabled: Boolean, onClick: () -> Unit, content: @Composable RowScope.() -> Unit) {
                Row(
                    Modifier.weight(1f).clip(RoundedCornerShape(18.dp))
                        .background(if (selected) Sea.copy(alpha = .22f) else Color.Transparent)
                        .clickable(enabled = enabled && !selected && !switching, onClick = onClick)
                        .semantics { this.selected = selected }
                        .padding(vertical = 9.dp, horizontal = 10.dp),
                    horizontalArrangement = Arrangement.Center,
                    verticalAlignment = Alignment.CenterVertically,
                    content = content,
                )
            }
            Option(!onServer, canLocal, { vm.switchWhere(false) }) {
                Text("📱 " + tr(lang, "app.where.phone"), color = if (!onServer) Ink else Muted, fontSize = 14.sp,
                    fontWeight = if (!onServer) FontWeight.SemiBold else FontWeight.Normal, maxLines = 1)
            }
            Option(onServer, true, { vm.switchWhere(true) }) {
                val dot = when {
                    switching -> Sand
                    reach == true -> Color(0xFF6FCF97)
                    reach == false -> Color(0xFFE57373)
                    else -> Muted
                }
                Box(Modifier.size(8.dp).clip(CircleShape).background(dot))
                Spacer(Modifier.width(7.dp))
                Text(if (vm.hasSavedServer()) serverName else tr(lang, "app.where.addServer"),
                    color = if (onServer) Ink else Muted, fontSize = 14.sp,
                    fontWeight = if (onServer) FontWeight.SemiBold else FontWeight.Normal,
                    maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        val hint = when {
            switching || local is LocalState.Starting -> tr(lang, "app.where.switching")
            onServer -> tr(lang, "app.where.serverHint")
            reach == false && vm.hasSavedServer() -> tr(lang, "app.where.serverDown", serverName)
            else -> tr(lang, "app.where.phoneHint")
        }
        Text(hint, color = Muted, fontSize = 12.sp, lineHeight = 16.sp, modifier = Modifier.padding(start = 6.dp, top = 5.dp))
    }
}


/** Triar secció: llistar les que hi ha (afegir-hi la fila) o crear-ne una de nova. */
@Composable
private fun SectionPickerDialog(vm: GregalViewModel, lang: String, targetKey: String?, onClose: () -> Unit) {
    val meta by vm.windsMeta.collectAsState()
    var name by remember { mutableStateOf("") }
    androidx.compose.ui.window.Dialog(onDismissRequest = onClose) {
        Surface(shape = RoundedCornerShape(20.dp), color = Superficie, shadowElevation = 6.dp) {
            Column(Modifier.padding(horizontal = 20.dp, vertical = 16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                Text(tr(lang, "app.vents.sections"), fontSize = 17.sp, fontWeight = FontWeight.ExtraBold, color = Ink)
                // Seccions que ja existeixen: un toc afegeix (o treu) la conversa.
                meta.sections.forEach { sec ->
                    val inIt = targetKey != null && targetKey in sec.items
                    TextButton(
                        onClick = {
                            if (targetKey != null) vm.toggleSectionItem(sec.id, targetKey)
                            onClose()
                        },
                        modifier = Modifier.fillMaxWidth(),
                        contentPadding = PaddingValues(vertical = 8.dp),
                    ) { Text((if (inIt) "✓ " else "") + sec.name + " · " + sec.items.size, modifier = Modifier.fillMaxWidth()) }
                }
                if (meta.sections.isEmpty()) Text(tr(lang, "app.vents.sectionsEmpty"), color = Muted, fontSize = 13.sp)
                OutlinedTextField(
                    value = name, onValueChange = { name = it },
                    label = { Text(tr(lang, "app.vents.newSectionName")) },
                    singleLine = true, shape = RoundedCornerShape(14.dp),
                    modifier = Modifier.fillMaxWidth(),
                )
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                    TextButton(onClick = onClose) { Text(tr(lang, "app.vents.chatCancel")) }
                    TextButton(
                        onClick = {
                            val n = name.trim()
                            if (n.isNotEmpty()) { vm.createSection(n, targetKey); onClose() }
                        },
                        enabled = name.trim().isNotEmpty(),
                    ) { Text(tr(lang, "app.vents.createSection")) }
                }
            }
        }
    }
}
