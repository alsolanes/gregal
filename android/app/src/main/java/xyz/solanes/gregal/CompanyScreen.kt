package xyz.solanes.gregal

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.EnterTransition
import androidx.compose.animation.ExitTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.togetherWith
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
import androidx.compose.material.icons.rounded.ExpandLess
import androidx.compose.material.icons.rounded.ExpandMore
import androidx.compose.material.icons.rounded.Groups
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.Delete
import androidx.compose.material.icons.rounded.Forum
import androidx.compose.material.icons.rounded.Menu
import androidx.compose.material.icons.rounded.Tune
import androidx.compose.material.icons.rounded.Warning
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
import androidx.compose.ui.platform.LocalContext
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
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.text.ClickableText
import kotlinx.coroutines.flow.conflate
import kotlinx.coroutines.launch
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.compositeOver
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.zIndex
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
        VentsHome(vm, lang, openDrawer, openPet = { })
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
fun VentsHome(vm: GregalViewModel, lang: String, openDrawer: () -> Unit, openLibrary: () -> Unit = {}, openPet: (Pet?) -> Unit) {
    var feinesOpen by remember { mutableStateOf(false) }
    var cornerOpen by remember { mutableStateOf(false) }
    var gamesOpen by remember { mutableStateOf(false) }
    // Inici (preguntar primer) o Xats (la llista de converses).
    var tab by rememberSaveable { mutableStateOf("home") }
    val askText = rememberAskText()
    // Una foto des de l'inici: es puja i la referència va al camp, com als xats.
    val homePhoto = androidx.activity.compose.rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.GetContent()
    ) { uri -> if (uri != null) vm.attachImage(uri) { ref -> askText.value = (ref + " " + askText.value).trim() } }
    // Un document (PDF, Word, Excel…) com a context: es puja i el missatge en duu la ruta,
    // com als xats dels vents; el vent el llegeix al servidor.
    val homeDoc = androidx.activity.compose.rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.GetContent()
    ) { uri -> if (uri != null) vm.attachDocument(uri) { path -> askText.value = appendDocMention(askText.value, path) } }
    val programats by vm.programats.collectAsState()
    val connections by vm.connections.collectAsState()
    val userName by vm.usuari.collectAsState()
    val windTasks by vm.windTasks.collectAsState()
    // Feines en marxa: es tornen a mirar cada 8 s (i cada minut si no n'hi ha
    // cap), així el progrés es reprèn en obrir l'app i l'avís arriba en acabar.
    // Només amb la pantalla visible: en segon pla ja vigila WindWatch.
    val anyRunning = windTasks.orEmpty().any { it.running }
    LifecyclePoll(anyRunning) {
        while (true) {
            vm.loadWindTasks()
            kotlinx.coroutines.delay(if (anyRunning) 8_000 else 60_000)
        }
    }
    val winds by vm.winds.collectAsState()
    val rotllanes by vm.rotllanes.collectAsState()
    val chats by vm.windChat.collectAsState()
    val rChats by vm.rotllanaChat.collectAsState()
    val wmeta by vm.windsMeta.collectAsState()
    val capHome by vm.capHome.collectAsState()
    val busy by vm.windBusy.collectAsState()
    val rBusy by vm.rotllanaBusy.collectAsState()
    val live = vm.windLive.collectAsState()
    val news by vm.windNews.collectAsState()
    val newsCount by vm.windNewsCount.collectAsState()
    val conn by vm.conn.collectAsState()
    // Veus proactives per obrir: es consulten en obrir els Vents i, amb l'app
    // oberta, cada 20 s (en segon pla, WindWatch cada 15 min). Un missatge nou
    // d'un vent arriba gairebé al moment, com als grans assistents.
    // B7: stream d'esdeveniments amb cursor (VentsStream.kt); si no hi és, sondeig de 20 s.
    NovetatsLive(conn) { vm.refreshWindNews() }
    // La mascota: en obrir els Vents (o canviar de lloc) diu què ha passat mentrestant.
    LaunchedEffect(conn) { vm.loadCap() }
    // Racó, Partits i Arena són plugins: només surten si el servidor els té encesos.
    val plugins by vm.plugins.collectAsState()
    LaunchedEffect(conn) { vm.loadPlugins() }
    // «Avui» a l'inici: els recordatoris que vénen i el resum del matí.
    val homeCards = tab == "home" && winds.isNotEmpty()
    LaunchedEffect(conn, homeCards) { if (homeCards) { vm.loadProgramats(); vm.loadConnections(); vm.loadTodayBrief() } }
    val todayBrief by vm.todayBrief.collectAsState()
    // «Avui»: la safata dels vents (VentsAvui.kt). Es llegeix en obrir els Vents
    // (per a la insígnia de la pestanya), en arribar veus noves i en entrar-hi.
    val avui by vm.avui.collectAsState()
    val avuiError by vm.avuiError.collectAsState()
    val weatherMap by vm.weather.collectAsState()
    LaunchedEffect(conn, news, tab) { vm.loadAvui() }
    // Amb la llista ja llegida, un plugin que el servidor no té no hi és: els
    // Partits i l'Arena només viuen al servidor de Gregal Jocs (gregal-jocs).
    // Mentre no se sap (llista buida), tot es mostra com abans.
    fun pluginOn(id: String) = if (plugins.isEmpty()) true else plugins[id] == true
    // El permís d'avisos ja no es demana aquí (B7): una sola política
    // contextual i durable a NotifPermission.kt.
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
    // La portada en té prou amb l'últim missatge de cada conversa (cada fil es
    // carrega en obrir-lo); en tornar d'una conversa es refresca. Per cercar
    // dins dels missatges, sí que calen tots els fils.
    LaunchedEffect(conn, open, winds.size, rotllanes.size, windTasks, news) {
        if (open == null) vm.loadSummaries()
    }
    LaunchedEffect(searching) {
        if (searching) { winds.forEach { vm.loadWindChat(it.id) }; rotllanes.forEach { vm.loadRotllanaChat(it.id) } }
    }
    val summaries by vm.summaries.collectAsState()
    val capUi by vm.cap.collectAsState()
    var whereOpen by remember { mutableStateOf(false) }
    var moreMenu by remember { mutableStateOf(false) }
    var exploreOpen by remember { mutableStateOf(false) }
    // Vents net: la capçalera només porta el menú, el títol i el «+»; el
    // directori i Explora s'obren des del calaix (o des de les cares de la portada).
    val simple by vm.simpleVents.collectAsState()
    val panel by vm.ventsPanel.collectAsState()
    var panelShown by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(panel) {
        when (panel) {
            "directory" -> { exploreOpen = false; searching = false; manage = true }
            "explore" -> { manage = false; searching = false; exploreOpen = true }
        }
        panelShown = panel
    }
    // En tancar-lo (enrere, triar una eina…), el calaix torna a marcar «Vents».
    LaunchedEffect(manage, exploreOpen, panelShown) {
        if (panelShown != null && !manage && !exploreOpen) { panelShown = null; vm.openVentsPanel(null) }
    }
    androidx.activity.compose.BackHandler(enabled = exploreOpen && !manage && open == null && !creator.first) { exploreOpen = false }
    var libraryOpen by remember { mutableStateOf(false) }
    var inboxFilter by rememberSaveable { mutableStateOf("all") }
    androidx.activity.compose.BackHandler(enabled = manage && open == null && !creator.first) { manage = false }
    androidx.activity.compose.BackHandler(enabled = searching && !manage && open == null && !creator.first && creation == null) {
        searching = false; query = ""
    }
    val crewUi by vm.crew.collectAsState()
    crewUi?.let { CrewSheet(vm, lang, it, winds) }
    // Rotllanes compartides amb altres persones del servidor (SharedChats.kt).
    SharedChatsSync(vm)
    val sharedChats by SharedChatsStore.chats.collectAsState()
    val sharedBusy by SharedChatsStore.busy.collectAsState()
    var sharedCreate by remember { mutableStateOf(false) }
    if (creator.first) {
        WindCreatorScreen(vm, lang, creator.second)
    } else if (creation != null) {
        ConversationCreator(vm, lang, winds, group = creation == "group") { creation = null }
    } else if (open != null) {
        val motion = LocalWindMotion.current
        AnimatedContent(
            targetState = open,
            transitionSpec = {
                if (motion) (fadeIn(tween(100), initialAlpha = .85f) + slideInHorizontally(tween(130)) { it / 24 })
                    .togetherWith(fadeOut(tween(75)))
                else EnterTransition.None togetherWith ExitTransition.None
            },
            label = "conversa",
        ) { target ->
            val key = target.orEmpty()
            val id = key.substringAfter(':')
            val wind = if (key.startsWith("wind:")) winds.firstOrNull { it.id == id } else null
            val rot = if (key.startsWith("rot:")) rotllanes.firstOrNull { it.id == id } else null
            when {
                key == "cap" -> VentetConversation(vm, lang, winds) { vm.openVentsChat(null) }
                key.startsWith("shared:") && sharedChats.any { it.id == id } ->
                    SharedChatConversation(vm, lang, sharedChats.first { it.id == id }) { vm.openVentsChat(null) }
                wind != null -> WindConversation(vm, lang, wind, { vm.openVentsChat(null) }) { wikiFromChat = ChatTarget(wind.name, wind.id, null) }
                rot != null -> RotllanaConversation(vm, lang, rot, winds, { vm.openVentsChat(null) }) { wikiFromChat = ChatTarget(rot.name, null, rot.id) }
                else -> Column(Modifier.fillMaxSize().background(Superficie).padding(20.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                    TextButton(onClick = { vm.openVentsChat(null) }) { Text(tr(lang, "app.vents.back")) }
                    Text(if (lang == "en") "Waiting for the conversation data" else "Esperant les dades de la conversa", color = Ink)
                    Text(if (lang == "en") "The notification's conversation will open when its data arrives. You can reload without sending the work again." else "El fil de l'avís s'obrirà quan arribin les dades. Pots recarregar sense tornar a enviar la feina.", color = Muted, fontSize = 13.sp)
                    OutlinedButton(onClick = { vm.loadWinds(); vm.loadRotllanes() }) { Text(if (lang == "en") "Reload" else "Recarrega") }
                }
            }
        }
    } else {
        Scaffold(containerColor = Superficie,
            topBar = {
                Column {
                    TopAppBar(
                        title = { Text(tr(lang, if (manage) "app.vents.myWinds" else "app.vents.title"), style = GregalType.title, color = Ink, maxLines = 1, overflow = TextOverflow.Ellipsis) },
                        navigationIcon = {
                            IconButton(onClick = { if (manage) manage = false else openDrawer() }) {
                                Icon(if (manage) Icons.Rounded.ArrowBack else Icons.Rounded.Menu,
                                    tr(lang, if (manage) "app.vents.back" else "app.nav.menu"), tint = Ink)
                            }
                        },
                        actions = {
                            // On estàs connectat ja ho diu el calaix; la cerca, a «Xats».
                            if (!manage && !simple) WhereChip(vm, lang) { whereOpen = true }
                            if (!manage && (!simple || searching || tab == "chats")) {
                                IconButton(onClick = { searching = !searching; if (!searching) query = "" }) {
                                    Icon(if (searching) Icons.Rounded.Close else Icons.Rounded.Search,
                                        tr(lang, if (searching) "app.vents.closeSearch" else "app.vents.search"), tint = Ink)
                                }
                            }
                            Box {
                                IconButton(onClick = { menu = true }) {
                                    Icon(Icons.Rounded.Add, tr(lang, "app.vents.create"), tint = Ink)
                                }
                                DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                                    DropdownMenuItem(text = { Text(tr(lang, "app.vents.newChat")) }, onClick = { menu = false; creation = "chat" })
                                    DropdownMenuItem(text = { Text(tr(lang, "app.company.rotllanaNew")) }, onClick = { menu = false; creation = "group" })
                                    DropdownMenuItem(text = { Text(tr(lang, "app.shared.new")) }, onClick = { menu = false; sharedCreate = true })
                                    DropdownMenuItem(text = { Text(tr(lang, "app.company.windNew")) }, onClick = { menu = false; vm.showWindCreator(true) })
                                    DropdownMenuItem(text = { Text(tr(lang, "app.crew.title")) }, onClick = { menu = false; vm.openCrew() })
                                }
                            }
                        },
                        colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie),
                    )
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
                    if (!manage && !searching) {
                        // Els tres filtres deixen espai a la dreta per a les dues
                        // accions que abans omplien la capçalera: així el títol
                        // («Vents») hi cap sencer en lloc de quedar tallat.
                        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                            Box(Modifier.weight(1f)) {
                                AskTabs(lang, tab, unread = newsCount.values.sum(), today = avui?.now?.count { it.state == "open" } ?: 0) { tab = it }
                            }
                            if (!simple) {
                                IconButton(onClick = { exploreOpen = !exploreOpen }) {
                                    Icon(Icons.Rounded.AutoAwesome, ventsCopy(lang, "explore"), tint = if (exploreOpen) Sea else Ink)
                                }
                                IconButton(onClick = { manage = true }, modifier = Modifier.padding(end = 6.dp)) {
                                    Icon(Icons.Rounded.Groups, tr(lang, "app.vents.directory"), tint = Ink)
                                }
                            }
                        }
                        if (tab == "chats") VentsInboxFilters(lang, inboxFilter, unread = newsCount.values.sum()) { inboxFilter = it }
                        // Explora puja des de baix, a sobre de la portada: abans
                        // s'obria enmig de la capçalera i empenyia tot cap avall.
                        if (exploreOpen) ModalBottomSheet(onDismissRequest = { exploreOpen = false }, containerColor = Superficie) {
                            Column(Modifier.verticalScroll(rememberScrollState()).padding(bottom = 16.dp).testTag("explore-sheet")) {
                                VentsExploreSheet(lang, pluginOn("corner"), pluginOn("games"),
                                    onCorner = { cornerOpen = true; exploreOpen = false },
                                    onGames = { gamesOpen = true; exploreOpen = false },
                                    onLibrary = { libraryOpen = true; openLibrary(); exploreOpen = false },
                                    onActivity = { exploreOpen = false; vm.go(Screen.Automatitzacions) },
                                    onSaved = { exploreOpen = false; vm.go(Screen.Desat) },
                                    onWiki = { wikiOpen = true; exploreOpen = false },
                                    onFeines = { feinesOpen = true; exploreOpen = false },
                                    onCrew = { vm.openCrew(); exploreOpen = false },
                                    onWorkspace = if (vm.simpleVents.value) null else ({ exploreOpen = false; vm.go(Screen.Carpeta) }))
                            }
                        }
                    }
                }
            },
            ) { pad ->
            val home = !manage && !searching && tab == "home" && winds.isNotEmpty()
            val inboxQuery = if (home) "" else query
            val inbox = if (manage) emptyList() else remember(winds, rotllanes, chats, rChats, inboxQuery, summaries) {
                ventsInbox(winds, rotllanes, chats, rChats, inboxQuery, summaries)
            }
            if (home) {
                // La portada: salutació, lluïssor i el compositor al mig. Quan hi ha
                // conversa, aquesta mateixa pantalla passa a ser el xat —amb la
                // lluïssor al darrere i el camp a baix— en lloc de saltar a una altra.
                val capSt = capUi.state
                val chatting = capSt.history.isNotEmpty() || capUi.busy
                val homeList = rememberLazyListState()
                val homeFollow = rememberChatFollow(homeList, "ventet-home")
                ReadReplyFromStart(homeList, homeFollow, capUi.busy, capReplyKey(capUi))
                // El missatge que no ha arribat torna al camp, també aquí.
                LaunchedEffect(capUi.failedText) {
                    val t = capUi.failedText
                    if (t.isNotBlank() && askText.value.isBlank()) askText.value = t
                }
                VentsOpening(
                    lang, userName, askText.value, { askText.value = it }, capUi.busy,
                    onPhoto = { homePhoto.launch("image/*") }, onDoc = { homeDoc.launch("*/*") },
                    onSend = { text ->
                        // Al Ventet: respon ell o ho passa a qui toca, i el xat es
                        // queda aquí mateix; res de canviar de finestra.
                        homeFollow.requestFollow(); vm.askCap(text); askText.value = ""
                    },
                    modifier = Modifier.fillMaxSize().padding(pad),
                    chat = if (chatting) ({ VentetThread(vm, lang, winds, homeList, Color(capSt.argb), showEmpty = false) }) else null,
                    chatHeader = if (chatting) ({
                        HomeThreadHeader(lang, capSt.name, Color(capSt.argb), capSt.faceStyle, (capUi.busy && capUi.live.isBlank()) || capUi.going)
                    }) else null,
                    top = { HomeFacesRow(lang, winds, news, busy, onTalk = { vm.openVentsChat("wind:" + it.id) }, onAll = { manage = true }) },
                ) {
                    val newsRows = inbox.filter { it.key.startsWith("wind:") && it.key.substringAfter(':') in news }
                    TodaySection(lang, todayCards(lang, newsRows, news, windTasks.orEmpty().filter { it.running },
                        emptyList(), null,
                        onOpen = { vm.openVentsChat(it) }, onWork = { feinesOpen = true },
                        onReminders = { vm.go(Screen.Automatitzacions) }, onBriefing = { vm.go(Screen.Automatitzacions) }).take(3),
                        brief = todayBrief, briefWind = winds.firstOrNull { it.id == todayBrief?.windId },
                        onBrief = { todayBrief?.windId?.takeIf { it.isNotBlank() }?.let { vm.openVentsChat("wind:$it") } })
                    Spacer(Modifier.height(16.dp))
                }
            } else if (!manage && !searching && tab == "today") {
                val ww = remember(winds) { weatherWind(winds) }
                val forecast = ww?.let { weatherMap[it.id] }?.takeIf { it.now != null }
                LaunchedEffect(conn, ww?.id) { ww?.let { vm.loadWeather(it.id) } }
                AvuiTray(
                    lang, avui, winds, avuiError,
                    onAct = { id, action -> vm.avuiAct(id, action) },
                    onRun = { card, action -> vm.avuiRun(card, action) },
                    onSettings = { s, tune -> vm.saveAvuiSettings(s, tune) },
                    onRetry = { vm.loadAvui() },
                    modifier = Modifier.padding(pad),
                    weather = if (ww != null && forecast != null) ({
                        WeatherCard(lang, forecast, Color(ww.argb), compact = true) { vm.openVentsChat("wind:" + ww.id) }
                    }) else null,
                )
            } else LazyColumn(Modifier.fillMaxSize().padding(pad), contentPadding = PaddingValues(bottom = 24.dp)) {
                if (manage) {
                    item { WindsModelRow(vm, lang) }
                    item { NewCrewMembersCard(vm, lang) }
                    item { SquadsRow(vm, lang) }
                    item { Text(tr(lang, "app.vents.manageHelp"), color = Muted, fontSize = 13.sp, lineHeight = 18.sp, modifier = Modifier.padding(horizontal = 20.dp, vertical = 12.dp)) }
                    if (winds.isEmpty()) item { VentsEmpty(lang, false) }
                    items(winds, key = { it.id }) { w ->
                        DirectoryRow(lang, w, onProfile = { profile = w }) { manage = false; vm.openVentsChat("wind:" + w.id) }
                    }
                } else {
                    // El Ventet viu a la seva secció; a la portada només si ho
                    // has triat a Preferències (les converses primer).
                    // Portada: el Ventet fixat a dalt i les converses just a sota. Els
                    // personatges són al directori; programats, biblioteca i feines,
                    // al menú «+»; la feina en marxa, una línia que només surt si n'hi ha.
                    if (query.isBlank() && capHome) item { VentetRow(lang, capUi, summaries["cap"]) { vm.openVentsChat("cap") } }
                    // El Racó: la pàgina que fan els vents sols.
                    if (query.isBlank() && anyRunning) item {
                        RunningWorkLine(lang, windTasks.orEmpty().count { it.running }) { feinesOpen = true }
                    }
                    /** Fila amb favorit: estrella viva si hi és, i el menú llarg la marca. */
                    @Composable
                    fun InboxRow(entry: VentsInboxRow, showSectionHeader: String?) {
                        if (showSectionHeader != null) {
                            Text(showSectionHeader.uppercase(), color = Muted, style = GregalType.overline,
                                modifier = Modifier.padding(start = 20.dp, top = 18.dp, bottom = 4.dp))
                        }
                        val id = entry.key.substringAfter(':')
                        val group = entry.key.startsWith("rot:")
                        val thinking = if (group) id in rBusy else id in busy
                        val unread = !group && id in news
                        val face = entry.members.firstOrNull()
                        val mood = if (thinking) {
                            // Only this row observes its mood, not the timestamps and progress of every turn.
                            val activeMood by remember(live, id) {
                                derivedStateOf(structuralEqualityPolicy()) { live.value[id]?.mood }
                            }
                            activeMood
                        } else if (unread) WindMood.NEWS else null
                        ConversationRow(entry.title, entry.preview, face?.let { Color(it.argb) } ?: Sea,
                            face?.faceStyle ?: WindStyle(), faces = if (group) entry.members else emptyList(),
                            busy = thinking, mood = mood,
                            unreadCount = if (unread) (newsCount[id] ?: 1) else 0,
                            timestamp = entry.timestamp,
                            favorite = entry.key in wmeta.favorites,
                            onFavorite = { vm.toggleFavorite(entry.key) },
                            onClick = { vm.openVentsChat(entry.key) },
                            onLongClick = {
                                if (group) rotllanes.firstOrNull { it.id == id }?.let { rowTarget = RowTarget.Rot(it) }
                                else face?.let { rowTarget = RowTarget.Dm(it) }
                            }, lang = lang)
                    }

                    val rows = inbox.filter { row ->
                        when (inboxFilter) {
                            "unread" -> row.key.startsWith("wind:") && row.key.substringAfter(':') in news
                            "groups" -> row.key.startsWith("rot:")
                            else -> true
                        }
                    }
                    if (rows.isEmpty()) item {
                        if (query.isBlank() && inboxFilter == "unread") EmptyInboxFilter(lang, "unread", ventsCopy(lang, "all")) { inboxFilter = "all" }
                        else if (query.isBlank() && inboxFilter == "groups") EmptyInboxFilter(lang, "groups", tr(lang, "app.shared.new")) { sharedCreate = true }
                        // Compte nou, sense cap vent: la benvinguda amb la colla del dia a dia.
                        else if (query.isBlank() && winds.isEmpty()) VentsWelcome(vm, lang)
                        else if (query.isBlank()) VentsEmpty(lang, true)
                        else Column(Modifier.fillMaxWidth().padding(32.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            Text(tr(lang, "app.vents.searchEmpty"), color = Ink, fontWeight = FontWeight.SemiBold)
                            Text(tr(lang, "app.vents.searchEmptyHelp"), color = Muted, fontSize = 14.sp)
                        }
                    }
                    // Les converses de veritat primer; els vents amb qui encara no
                    // has parlat, a sota amb el seu títol. Abans tot anava barrejat i
                    // una fila «Xaloc · Música» semblava una conversa buida.
                    if (inboxFilter != "unread") sharedChatRows(
                        sharedChats.filter { query.isBlank() || it.name.contains(query, ignoreCase = true) },
                        sharedBusy, lang) { vm.openVentsChat("shared:$it") }
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
    if (libraryOpen) WindLibraryDialog(vm, lang) { libraryOpen = false }
    if (sharedCreate) SharedChatCreator(vm, lang, winds, onClose = { sharedCreate = false }) { vm.openVentsChat("shared:$it") }
    SharedInvitePopup(vm, lang) { vm.openVentsChat("shared:$it") }
    if (programatsOpen) ProgramatsDialog(vm, lang) { programatsOpen = false }
    if (feinesOpen) FeinesDialog(vm, lang, onOpenThread = { vm.openVentsChat("rot:$it") }) { feinesOpen = false }
    if (cornerOpen) CornerDialog(vm, lang) { cornerOpen = false }
    if (gamesOpen) GamesDialog(vm, lang) { gamesOpen = false }
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
    if (whereOpen) WhereDialog(vm, lang) { whereOpen = false }
    rowTarget?.let { t ->
        ChatRowDialog(vm, lang, t, onToSection = { key -> sectionTargetKey = key; sectionPicker = true }) { rowTarget = null }
    }
}

/** L'estat d'un vent en paraules (mai només amb la insígnia); null si està en repòs. */
internal fun windStateLabel(lang: String, mood: WindMood, busy: Boolean): String? = when (mood) {
    WindMood.SEARCHING -> tr(lang, "app.vents.gridSearching")
    WindMood.READING -> if (lang == "en") "reading…" else "està llegint…"
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

/**
 * Primer cop (o compte nou): sense vents no hi ha res a fer. Un toc posa la
 * colla del dia a dia (compres, llibres, esports, música i organització) i
 * ja es pot parlar; qui vulgui, en crea un de seu.
 */
@Composable
private fun VentsWelcome(vm: GregalViewModel, lang: String) {
    val squads by vm.squads.collectAsState()
    val user by vm.usuari.collectAsState()
    var activating by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) { vm.loadSquads() }
    val daily = squads.firstOrNull { it.id == "daily" }
    Column(Modifier.fillMaxWidth().padding(horizontal = 22.dp, vertical = 24.dp),
        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Text(if (user.isNotBlank()) tr(lang, "app.vents.welcomeHi", user) else tr(lang, "app.vents.welcomeHiAnon"),
            color = Ink, fontSize = 26.sp, fontWeight = FontWeight.ExtraBold, textAlign = TextAlign.Center)
        Text(tr(lang, "app.vents.welcomeBody"), color = Muted, fontSize = 15.sp, lineHeight = 21.sp, textAlign = TextAlign.Center)
        if (daily != null) Surface(shape = RoundedCornerShape(26.dp), color = Sea.copy(alpha = 0.09f),
            border = androidx.compose.foundation.BorderStroke(1.dp, Sea.copy(alpha = 0.30f)), modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(18.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly) {
                    daily.members.take(5).forEach { m ->
                        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            WindHalo(accent = Color(windArgb(m.color)), style = windStyleOf(m.style).copy(kit = m.key), size = 50, mood = WindMood.HAPPY)
                            Text(m.name, color = Ink, fontSize = 11.sp, fontWeight = FontWeight.SemiBold, maxLines = 1)
                        }
                    }
                }
                Text(if (lang == "ca") daily.members.joinToString(" · ") { it.ca } else daily.members.joinToString(" · ") { it.en },
                    color = Muted, fontSize = 12.sp, textAlign = TextAlign.Center, lineHeight = 16.sp)
                androidx.compose.material3.Button(onClick = {
                    activating = true
                    vm.activateSquad(daily.id, lang) { activating = false }
                }, enabled = !activating, shape = RoundedCornerShape(16.dp), modifier = Modifier.fillMaxWidth().height(50.dp)) {
                    if (activating) TypingDots(Ink) else Text(tr(lang, "app.vents.welcomeStart"), fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                }
            }
        }
        androidx.compose.material3.TextButton(onClick = { vm.showWindCreator(true) }) {
            Text(tr(lang, "app.vents.welcomeOwn"), color = Sea, fontSize = 14.sp)
        }
    }
}

@Composable
private fun VentsEmpty(lang: String, conversations: Boolean) {
    Column(Modifier.fillMaxWidth().padding(horizontal = 32.dp, vertical = 30.dp),
        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(12.dp)) {
        // Trio de cares flotants: el buit també somriu.
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(2.dp)) {
            // Els colors del tema (accent, sorra i aqua): els pastels fixos
            // d'abans quedaven esvaïts damunt del tema clar.
            Box(Modifier.offset(x = 12.dp)) { WindHalo(Sea, windStyleOf("esn"), size = 52) }
            Box(Modifier.offset(y = (-10).dp)) { WindHalo(Sand, windStyleOf("bsn"), size = 72) }
            Box(Modifier.offset(x = (-12).dp)) { WindHalo(Onada, windStyleOf("pso"), size = 52) }
        }
        Text(tr(lang, if (conversations) "app.vents.emptyTitle" else "app.company.windEmptyTitle"), color = Ink,
            fontSize = 21.sp, fontWeight = FontWeight.ExtraBold, textAlign = TextAlign.Center)
        Text(tr(lang, if (conversations) "app.vents.emptyBody" else "app.company.windEmptyBody"), color = Muted,
            fontSize = 14.sp, textAlign = TextAlign.Center)
    }
}

/**
 * La colla que ja tens ha crescut (p. ex. la de cada dia amb Llevantada i
 * Oreig): una targeta petita amb les cares i «Afegeix-los». Els que ja tenien
 * la colla no els reben sols.
 */
@Composable
private fun NewCrewMembersCard(vm: GregalViewModel, lang: String) {
    val squads by vm.squads.collectAsState()
    val winds by vm.winds.collectAsState()
    var adding by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) { vm.loadSquads() }
    val (squad, missing) = remember(squads, winds) { missingSquadMembers(squads, winds) } ?: return
    val names = missing.map { m -> m.name + " (" + (if (lang == "ca") m.ca else m.en).lowercase() + ")" }
    val joined = if (names.size == 1) names[0] else names.dropLast(1).joinToString(", ") + tr(lang, "app.crew.newAnd") + names.last()
    Surface(shape = RoundedCornerShape(20.dp), color = Sea.copy(alpha = 0.08f),
        border = androidx.compose.foundation.BorderStroke(1.dp, Sea.copy(alpha = 0.30f)),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 5.dp).testTag("crew-new")) {
        Row(Modifier.padding(horizontal = 12.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Row(horizontalArrangement = Arrangement.spacedBy((-10).dp)) {
                missing.take(3).forEach { m ->
                    WindHalo(accent = Color(windArgb(m.color)), style = windStyleOf(m.style).copy(kit = m.key), size = 34, mood = WindMood.HAPPY, showKit = false)
                }
            }
            Text(tr(lang, "app.crew.newMembers", joined), color = Ink, style = GregalType.bodySmall, modifier = Modifier.weight(1f))
            androidx.compose.material3.TextButton(onClick = {
                adding = true
                vm.activateSquad(squad.id, lang) { adding = false; vm.loadSquads() }
            }, enabled = !adding) {
                if (adding) TypingDots(Sea) else Text(tr(lang, "app.crew.addThem"), color = Sea, fontWeight = FontWeight.SemiBold)
            }
        }
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
                // Icona de 44 dp (la de 64 dels estats buits feia la targeta el doble d'alta).
                TintedIcon(Icons.Rounded.Groups, Sea, box = 44.dp, glyph = 22.dp)
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(tr(lang, "app.vents.squads"), style = GregalType.heading, color = Ink)
                    Text(tr(lang, "app.vents.squadsHelp"), style = GregalType.bodySmall, color = Muted, maxLines = 2, overflow = TextOverflow.Ellipsis)
                }
                Icon(if (expanded) Icons.Rounded.ExpandLess else Icons.Rounded.ExpandMore,
                    contentDescription = null, tint = Muted, modifier = Modifier.size(20.dp))
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
                                vm.activateSquad(s.id, lang) { activating = null }
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
            TintedIcon(Icons.Rounded.AutoAwesome, Sea, box = 44.dp, glyph = 22.dp)
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(tr(lang, "app.vents.defaultModel"), style = GregalType.heading, color = Ink)
                Text(
                    windsModel.ifBlank { tr(lang, "app.vents.defaultModelSameAsWork") },
                    style = GregalType.bodySmall, color = Muted, maxLines = 1, overflow = TextOverflow.Ellipsis,
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
        Text(title.uppercase(), color = Muted, style = GregalType.overline,
            modifier = Modifier.weight(1f))
        if (favorite) IconButton(onClick = onAddSection) {
            Icon(Icons.Rounded.Folder, tr(lang, "app.vents.newSection"), tint = Muted, modifier = Modifier.size(16.dp))
        }
    }
}

/** «Comença un xat»: el tall entre converses i vents sense parlar. */
@Composable
internal fun InboxDivider(lang: String) {
    Text(tr(lang, "app.vents.startChat").uppercase(), color = Muted, style = GregalType.overline,
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
    /** Quants missatges esperen resposta; 0 = cap insígnia. */
    unreadCount: Int = 0,
    onClick: () -> Unit,
    onLongClick: (() -> Unit)? = null,
    lang: String = "ca",
) {
    val unread = mood == WindMood.NEWS
    val state = if (unread) null else windStateLabel(lang, mood ?: WindMood.IDLE, busy)
    val receipt = remember(timestamp, lang) {
        if (timestamp > 0L) inboxWhen(timestamp, lang) else ""
    }
    val stacked = remember(faces) { faces.drop(1) }
    Row(
        Modifier.fillMaxWidth().padding(horizontal = 8.dp).heightIn(min = 68.dp)
            .clip(RoundedCornerShape(16.dp))
            .background(if (unread) Sea.copy(alpha = .07f) else Color.Transparent)
            .combinedClickable(onClick = onClick, onLongClick = onLongClick)
            .padding(start = 12.dp, end = 8.dp, top = 8.dp, bottom = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        // Un vent sol porta l'eina del seu ofici també a la llista: és com es
        // distingeixen els especialistes d'un cop d'ull, no només pel color.
        WindHalo(accent, style, size = 44, stacked = stacked, online = false,
            mood = mood ?: if (busy) WindMood.THINKING else WindMood.IDLE, showKit = faces.size <= 1)
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(3.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                Text(title, fontSize = 15.sp, fontWeight = if (unread) FontWeight.Bold else FontWeight.SemiBold,
                    color = Ink, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                if (receipt.isNotBlank()) Text(receipt, color = Muted, fontSize = 11.sp)
            }
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(state ?: subtitle, fontSize = 13.sp, color = if (unread) Ink else Muted,
                    fontWeight = if (unread) FontWeight.Medium else FontWeight.Normal,
                    maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                if (busy) TypingDots(Sea)
                else if (unread) {
                    // Amb el número: un «3» diu quants missatges t'esperen, un
                    // punt només deia que n'hi havia algun.
                    val badge = unreadBadge(unreadCount)
                    if (badge.isEmpty()) Box(Modifier.size(7.dp).background(Sea, CircleShape))
                    else Surface(shape = CircleShape, color = Sea) {
                        Text(
                            badge, color = OnSea, fontSize = 11.sp, fontWeight = FontWeight.Bold,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 1.dp),
                        )
                    }
                }
            }
        }
        // L'estrella, al marge dret: dins de la línia del nom l'estirava i
        // separava massa el nom del text de sota.
        if (onFavorite != null) IconButton(
            onClick = onFavorite,
            // 48 dp (abans 36): la fila ja en fa 68, no creix, i l'estrella es toca a la primera.
            modifier = Modifier.semantics {
                contentDescription = if (favorite) ventsCopy(lang, "removeFavorite") else ventsCopy(lang, "addFavorite")
                selected = favorite
            },
        ) {
            Icon(
                if (favorite) Icons.Rounded.Star else Icons.Rounded.StarBorder,
                // L'estrella apagada en gris quedava invisible: l'accent
                // ple marca el favorit i el contorn fosc el desmarcat.
                contentDescription = null, tint = if (favorite) Sea else Ink.copy(alpha = .55f),
                modifier = Modifier.size(19.dp))
        }
    }
}

/** El xat d'un vent a pantalla completa: capçalera amb la cara, bombolles, input. */
@Composable
internal fun WindConversation(vm: GregalViewModel, lang: String, wind: Wind, onBack: () -> Unit, onWiki: () -> Unit = {}) {
    val chats by vm.windChat.collectAsState()
    val loadingChat by vm.windChatLoading.collectAsState()
    val busy by vm.windBusy.collectAsState()
    val live by vm.windLive.collectAsState()
    val turnActionsAll by vm.turnActions.collectAsState()
    val msgs = chats[wind.id].orEmpty()
    val clipboard = LocalClipboardManager.current
    var field by rememberSaveable(wind.id, stateSaver = TextFieldValue.Saver) { mutableStateOf(TextFieldValue("")) }
    val accent = Color(wind.argb)
    val winds by vm.winds.collectAsState()
    // Mencions @Nom: el desplegable segueix el cursor; en enviar,
    // si hi ha @UnAltre, la feina es deriva al seu xat (handoff).
    val mention = activeMention(field.text, field.selection.start, winds)
    // Mentrestant el vent treballa, enviar = steer: el missatge entra al
    // context del torn sense aturar-lo (el ViewModel ho decideix).
    val canSend = field.text.isNotBlank()
    var profile by remember(wind.id) { mutableStateOf(false) }
    var askTransfer by remember(wind.id) { mutableStateOf(false) }
    var showPerm by remember(wind.id) { mutableStateOf(false) }
    var sessionsOpen by remember(wind.id) { mutableStateOf(false) }
    var watchOpen by remember(wind.id) { mutableStateOf(false) }
    var computerOpen by remember(wind.id) { mutableStateOf(false) }
    var whyId by remember(wind.id) { mutableStateOf("") }
    val st by vm.state.collectAsState()
    val pickDocs = androidx.activity.compose.rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.GetMultipleContents()
    ) { uris ->
        uris.take(3).forEach { uri ->
            vm.attachDocument(uri) { path ->
                val next = appendDocMention(field.text, path)
                field = TextFieldValue(next, TextRange(next.length))
            }
        }
    }
    // Una foto: es puja i queda al camp la imatge (es veu al xat) amb la ruta.
    val pickPhoto = androidx.activity.compose.rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.GetContent()
    ) { uri ->
        if (uri != null) vm.attachImage(uri) { next ->
            val txt = (next + " " + field.text).trim()
            field = TextFieldValue(txt, TextRange(txt.length))
        }
    }
    val listState = rememberLazyListState()
    LaunchedEffect(wind.id) { vm.loadWindChat(wind.id) }
    // Un recordatori o un cor que parla amb el xat obert: el missatge nou entra
    // al moment (la consulta de novetats és cada 20 s), sense haver de sortir i tornar a entrar.
    val openNews by vm.windNewsCount.collectAsState()
    val openNewsTs = openNews[wind.id]
    LaunchedEffect(openNewsTs) { if (openNewsTs != null) { vm.loadWindChat(wind.id); vm.markWindNewsSeen(wind.id) } }
    // B7: fil visible = novetats llegides a tot arreu; obrir des d'un avís porta al missatge exacte.
    val chatConn by vm.conn.collectAsState()
    ThreadReadEffect(chatConn, wind.id, openNewsTs)
    FocusMessageEffect("wind:${wind.id}", listState, msgs.map { it.id })
    val follow = rememberChatFollow(listState, wind.id)
    var liveFrom by remember(wind.id) { mutableIntStateOf(Int.MAX_VALUE) }
    LaunchedEffect(wind.id, msgs.size) { if (msgs.isNotEmpty() && liveFrom == Int.MAX_VALUE) liveFrom = msgs.size }

    val isBusy = busy.contains(wind.id)
    // Al fil, cada missatge és l'element del seu índex (l'estat buit només surt sense missatges).
    ReadReplyFromStart(listState, follow, isBusy, currentReplyKey(msgs))
    Box(Modifier.fillMaxSize().background(Superficie)) {
        Column(Modifier.fillMaxSize()) {
        // La capçalera diu qui és; «què està fent» es diu al final del fil.
        VentsChatHeader(lang, wind.name,
            wind.role,
            accent, wind.faceStyle,
            if (isBusy) live[wind.id]?.mood ?: WindMood.THINKING else WindMood.IDLE,
            onBack = onBack, onProfile = { profile = true },
            onWiki = onWiki,
            onSessions = { sessionsOpen = true; vm.loadWindSessions(wind.id) },
            onTransfer = if (vm.canTransferToServer()) ({ askTransfer = true }) else null,
            onComputer = { computerOpen = true },
            onWatch = { watchOpen = true })
        // Missatges
        // Quan el viewport s'encongeix de cop (s'obre el teclat) i érem al
        // final, segueix-lo: el teclat no ha de tapar l'últim missatge.
        val scope = rememberCoroutineScope()
        val awayFromLatest = msgs.isNotEmpty() && !follow.following
        Box(Modifier.weight(1f)) {
        LazyColumn(
            Modifier.fillMaxSize(),
            state = listState,
            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 10.dp),
            verticalArrangement = Arrangement.spacedBy(0.dp),
        ) {
            if (msgs.isEmpty()) {
                // Mentre el fil arriba del servidor, l'esquelet: ensenyar el
                // «comença la conversa» un instant i treure'l de cop feia
                // l'entrada pobra i el xat, buit.
                item {
                    if (wind.id in loadingChat) ChatSkeleton(accent)
                    else ChatEmptyState(
                        face = { WindHalo(size = 88, accent = accent, style = wind.faceStyle, mood = WindMood.HAPPY, tappable = true) },
                        title = tr(lang, "app.vents.emptyChatTitle"),
                        hint = tr(lang, "app.company.windHello", wind.name),
                        // Com al Grok: tres primeres frases a un toc, segons l'ofici del vent.
                        suggestions = windStarters(lang, wind.specialty),
                        onPick = { follow.requestFollow(); vm.sendWindMessage(wind.id, it) },
                    )
                }
            }
            items(msgs.size, key = { idx -> idx }, contentType = { idx -> msgs[idx].from }) { idx ->
                // Les lambdes capturen el missatge, no la llista: cada fragment del
                // streaming crea una llista nova i, si la capturessin, totes les
                // bombolles visibles es tornarien a compondre, no només l'última.
                val m = msgs[idx]
                // Aire segons bàndol: juntes les del mateix, separades en canviar.
                val gap = if (idx > 0 && m.from != msgs[idx - 1].from) 16.dp else 4.dp
                Column(Modifier.padding(top = gap)) {
                    BubbleIn("dm#$idx", animate = idx >= liveFrom) {
                        WindBubble(m, accent, lang = lang,
                            onCopy = {
                                clipboard.setText(AnnotatedString(stripMdInline(m.text)))
                                scope.launch { vm.notify(tr(lang, "app.vents.copied")) }
                            },
                            onContinue = if (!isBusy) ({ follow.requestFollow(); vm.sendWindContinue(wind.id) }) else null,
                            onRetry = if (!isBusy) ({ follow.requestFollow(); vm.retryLastWindTurn(wind.id) }) else null,
                            onTransfer = if (vm.canTransferToServer() && !isBusy) ({ askTransfer = true }) else null,
                            onDiscard = if (m.kind == "queued") ({ vm.discardQueued(wind.id, m.ts) }) else null,
                            onCopied = { scope.launch { vm.notify(tr(lang, "app.vents.linkCopied")) } })
                    }
                    // B6: un missatge que ve d'una responsabilitat diu per què.
                    if (m.proactive) TextButton(onClick = { whyId = m.id },
                        modifier = Modifier.padding(start = 4.dp).testTag("msg-why")) {
                        Text(respCopy(lang, "whyShort"), color = accent, fontSize = 13.sp)
                    }
                }
            }
            // Respostes ràpides que el vent proposa al final del seu missatge.
            if (!isBusy) {
                val quick = msgs.lastOrNull()?.takeIf { it.from != "user" && !it.streaming }?.let { extractChoices(it.text) }.orEmpty()
                if (quick.isNotEmpty()) item(key = "quick") {
                    ChoiceChips(quick, accent) { pick -> follow.requestFollow(); vm.sendWindMessage(wind.id, pick) }
                }
            }
            // Prova del que el vent ha programat de debò en aquest torn, amb «Desfés».
            val turnActs = turnActionsAll[wind.id].orEmpty()
            if (turnActs.isNotEmpty() && msgs.lastOrNull()?.from != "user") item(key = "turn-actions") {
                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    for (a in turnActs) TurnActionCard(lang, a, accent) { vm.undoTurnAction(wind.id, a) }
                }
            }
            // Un sol lloc per «què està fent»: la cara petita i els punts al final
            // del fil, amb l'acció en marxa al costat («Cercant al web…»). La
            // capçalera i la franja de dalt ja no en diuen res.
            val liveNow = live[wind.id]
            if (busy.contains(wind.id) && msgs.lastOrNull()?.streaming != true) {
                item(key = "typing") {
                    WindTypingBubble(lang, wind, accent, liveNow?.mood ?: WindMood.THINKING,
                        label = liveNow?.let { l -> windVisibleWork(lang, l) ?: windStallNote(lang, l, quietNow(true)) })
                }
            }
        }
        if (awayFromLatest) LatestPill(lang) {
            follow.requestFollow(); scope.launch { listState.scrollToLatest() }
        }
        }
        // Aprovacions i preguntes del vent: el torn espera aquí la teva resposta.
        WindAskCards(vm, lang, accent, isBusy)
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
        VentsComposer(lang, field, { field = it }, tr(lang, "app.company.windChatHint", wind.name), canSend,
            busy = busy.contains(wind.id), accent = accent, st = st,
            onAttachDoc = { pickDocs.launch("*/*") },
            onAttachImage = { pickPhoto.launch("image/*") },
            onMode = { vm.setMode(it) },
            onPerm = { showPerm = true },
            onWorkspace = if (vm.simpleVents.value) null else ({ vm.go(Screen.Carpeta) }),
            onStop = if (busy.contains(wind.id)) ({ vm.cancelWindTurn(wind.id) }) else null,
            onPreview = { vm.openPreview() },
            showModePill = !vm.simpleVents.value) {
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
                follow.requestFollow(); vm.sendWindMessage(wind.id, body); field = TextFieldValue("")
            }
        }
        }
    }
    if (profile) WindProfileDialog(vm, lang, wind) { profile = false }
    if (askTransfer) TransferConfirmDialog(vm, lang, "wind", wind.id) { askTransfer = false }
    if (showPerm) PermissionDialog(st, lang, onPick = { vm.setAccess(it); showPerm = false }, onClose = { showPerm = false })
    if (sessionsOpen) WindSessionsSheet(vm, lang, wind) { sessionsOpen = false }
    if (watchOpen) ResponsabilitatsSheet(chatConn, lang, winds, accent) { watchOpen = false }
    if (computerOpen) OrdinadorSheet(chatConn, lang, accent) { computerOpen = false }
    if (whyId.isNotBlank()) RespWhySheet(chatConn, lang, whyId) { whyId = "" }
}

/**
 * Sessions d'un vent: els seus fils de conversa. La marcada és l'oberta;
 * tocar-ne un altre el fa actiu i el fil es repinta amb el seu historial.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun WindSessionsSheet(vm: GregalViewModel, lang: String, wind: Wind, onClose: () -> Unit) {
    val sessions by vm.windSessions.collectAsState()
    val loading by vm.windSessionsLoading.collectAsState()
    val list = sessions[wind.id].orEmpty()
    val isLoading = wind.id in loading
    val accent = Color(wind.argb)
    ModalBottomSheet(onDismissRequest = onClose, containerColor = Superficie) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp).padding(bottom = 24.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                WindHalo(accent, wind.faceStyle, size = 30, mood = WindMood.IDLE)
                Column(Modifier.weight(1f)) {
                    Text(wind.name, color = Ink, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
                    Text(tr(lang, "app.wind.sessions"), color = Muted, fontSize = 12.sp)
                }
                TextButton(onClick = { vm.newWindSession(wind.id); onClose() }, contentPadding = PaddingValues(horizontal = 12.dp)) {
                    Icon(Icons.Rounded.Add, null, tint = accent, modifier = Modifier.size(16.dp))
                    Spacer(Modifier.width(4.dp))
                    Text(tr(lang, "nav.new"), color = accent, fontSize = 13.sp)
                }
            }
            if (isLoading && list.isEmpty()) {
                Text(tr(lang, "app.sess.loading"), color = Faint, fontSize = 13.sp, modifier = Modifier.padding(vertical = 12.dp))
            } else if (list.isEmpty()) {
                Text(tr(lang, "app.wind.sessionsEmpty"), color = Faint, fontSize = 13.sp, modifier = Modifier.padding(vertical = 12.dp))
            } else {
                list.forEach { s ->
                    var confirmDelete by remember(s.name) { mutableStateOf(false) }
                    Row(
                        Modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp))
                            .background(if (s.current) accent.copy(alpha = 0.12f) else SuperficieAlta)
                            .clickable { vm.openWindSession(wind.id, s.name); onClose() }
                            .padding(horizontal = 14.dp, vertical = 12.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                            Text(s.title.ifBlank { wind.name }, color = Ink, fontSize = 14.sp, fontWeight = if (s.current) FontWeight.SemiBold else FontWeight.Normal, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            Text(if (s.msgs > 0) tr(lang, "app.wind.sessionMsgs", s.msgs) + " · " + s.whenText else s.whenText, color = Muted, fontSize = 11.sp)
                        }
                        if (s.current) {
                            Text(tr(lang, "app.wind.sessionCurrent"), color = accent, fontSize = 11.sp, fontWeight = FontWeight.Medium)
                        } else if (confirmDelete) {
                            TextButton(onClick = { vm.deleteWindSession(wind.id, s.name) }, contentPadding = PaddingValues(horizontal = 8.dp)) {
                                Text(tr(lang, "app.sess.delete"), color = Red, fontSize = 12.sp)
                            }
                        } else {
                            IconButton(onClick = { confirmDelete = true }) {
                                Icon(Icons.Rounded.Delete, tr(lang, "app.sess.delete"), tint = Muted, modifier = Modifier.size(16.dp))
                            }
                        }
                    }
                }
            }
        }
    }
}

/** El xat d'una rotllana a pantalla completa. */
@Composable
private fun RotllanaConversation(vm: GregalViewModel, lang: String, rot: Rotllana, winds: List<Wind>, onBack: () -> Unit, onWiki: () -> Unit = {}) {
    val chats by vm.rotllanaChat.collectAsState()
    val loadingChat by vm.rotllanaChatLoading.collectAsState()
    val busy by vm.rotllanaBusy.collectAsState()
    val live by vm.windLive.collectAsState()
    val msgs = chats[rot.id].orEmpty()
    val clipboard = LocalClipboardManager.current
    var field by rememberSaveable(rot.id, stateSaver = TextFieldValue.Saver) { mutableStateOf(TextFieldValue("")) }
    var groupProfile by remember(rot.id) { mutableStateOf(false) }
    var askTransfer by remember(rot.id) { mutableStateOf(false) }
    var showPerm by remember(rot.id) { mutableStateOf(false) }
    var speakerProfile by remember(rot.id) { mutableStateOf<Wind?>(null) }
    val st by vm.state.collectAsState()
    val pickDocs = androidx.activity.compose.rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.GetMultipleContents()
    ) { uris ->
        uris.take(3).forEach { uri ->
            vm.attachDocument(uri) { path ->
                val next = appendDocMention(field.text, path)
                field = TextFieldValue(next, TextRange(next.length))
            }
        }
    }
    // Una foto: es puja i queda al camp la imatge (es veu al xat) amb la ruta.
    val pickPhoto = androidx.activity.compose.rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.GetContent()
    ) { uri ->
        if (uri != null) vm.attachImage(uri) { next ->
            val txt = (next + " " + field.text).trim()
            field = TextFieldValue(txt, TextRange(txt.length))
        }
    }
    val listState = rememberLazyListState()
    val faces = rot.members.mapNotNull { mid -> winds.firstOrNull { it.id == mid } }
    // Mencions @Nom: etiquetar un membre dirigeix el torn (només respon ell).
    val mention = activeMention(field.text, field.selection.start, faces)
    // Mentrestant la rotllana treballa, enviar = steer (el ViewModel decideix).
    val canSend = field.text.isNotBlank()
    LaunchedEffect(rot.id) { vm.loadRotllanaChat(rot.id) }
    val follow = rememberChatFollow(listState, rot.id)
    var liveFrom by remember(rot.id) { mutableIntStateOf(Int.MAX_VALUE) }
    LaunchedEffect(rot.id, msgs.size) { if (msgs.isNotEmpty() && liveFrom == Int.MAX_VALUE) liveFrom = msgs.size }

    val rotBusy = busy.contains(rot.id)
    val rotAccent = faces.firstOrNull()?.let { Color(it.argb) } ?: Sea
    // El debat de la colla (discussió i progrés) es plega: cada tirada és una
    // línia que s'obre amb un toc. Clau: l'índex del primer missatge de la tirada.
    val openDebates = remember(rot.id) { mutableStateListOf<Int>() }
    Box(Modifier.fillMaxSize().background(Superficie)) {
        Column(Modifier.fillMaxSize()) {
        val crewSteps by vm.crewStep.collectAsState()
        val crewStep = crewSteps[rot.id]
        VentsChatHeader(lang, rot.name,
            // El pas de la colla i «què està fent» es diuen al final del fil.
            faces.joinToString(", ") { it.name },
            rotAccent, faces.firstOrNull()?.faceStyle ?: WindStyle(),
            if (rotBusy) live[rot.id]?.mood ?: WindMood.THINKING else WindMood.IDLE, faces,
            onBack = onBack, onProfile = { groupProfile = true },
            onWiki = onWiki,
            onTransfer = if (vm.canTransferToServer()) ({ askTransfer = true }) else null)
        // Quan el viewport s'encongeix de cop (s'obre el teclat) i érem al
        // final, segueix-lo: el teclat no ha de tapar l'últim missatge.
        val scope = rememberCoroutineScope()
        val awayFromLatest = msgs.isNotEmpty() && !follow.following
        Box(Modifier.weight(1f)) {
        LazyColumn(
            Modifier.fillMaxSize(),
            state = listState,
            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 10.dp),
            verticalArrangement = Arrangement.spacedBy(0.dp),
        ) {
            if (msgs.isEmpty()) item {
                if (rot.id in loadingChat) ChatSkeleton(rotAccent)
                else ChatEmptyState(
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
                val run = debateRunStart(msgs, idx)
                if (run >= 0 && run != idx && run !in openDebates) return@items
                if (run == idx) {
                    DebateHeader(lang, msgs, idx, faces, run in openDebates) {
                        if (run in openDebates) openDebates.remove(run) else openDebates.add(run)
                    }
                    if (run !in openDebates) return@items
                }
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
                if (msgs[idx].kind == "result") Text("✅ " + tr(lang, "app.crew.result"), color = Muted, fontSize = 12.sp,
                    fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(start = 8.dp, bottom = 4.dp))
                if (msgs[idx].kind == "round") Text("· " + tr(lang, "app.crew.round"), color = Muted, fontSize = 12.sp,
                    fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(start = 8.dp, bottom = 4.dp))
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
                    onContinue = if (!busy.contains(rot.id)) ({ follow.requestFollow(); vm.sendRotllanaContinue(rot.id, resolved.message.speakerId) }) else null,
                    onRetry = if (!busy.contains(rot.id)) ({ follow.requestFollow(); vm.retryLastRotllanaTurn(rot.id) }) else null,
                    onTransfer = if (vm.canTransferToServer() && !busy.contains(rot.id)) ({ askTransfer = true }) else null,
                    onCopied = { scope.launch { vm.notify(tr(lang, "app.vents.linkCopied")) } })
                }
                if (msgs[idx].kind == "question" && idx == msgs.lastIndex) CrewQuestionCard(lang, rotAccent)
                }
            }
            // Respostes ràpides que l'últim vent proposa al final del seu missatge.
            if (!busy.contains(rot.id)) {
                val proposer = msgs.lastOrNull()?.takeIf { it.from != "user" && !it.streaming }
                val quick = proposer?.let { extractChoices(it.text) }.orEmpty()
                // La proposta la continua qui l'ha feta: abans responia el vent que menys havia parlat.
                if (quick.isNotEmpty()) item(key = "quick") {
                    ChoiceChips(quick, rotAccent) { pick -> follow.requestFollow(); vm.sendRotllanaMessage(rot.id, pick, proposer?.speakerId.orEmpty()) }
                }
            }
            // Un sol lloc per «què està fent» la colla: els punts al final del
            // fil, amb el pas o l'acció al costat. La capçalera només diu qui hi és.
            if (busy.contains(rot.id) && msgs.lastOrNull()?.streaming != true) {
                val liveRot = live[rot.id]
                item {
                    // El rellotge del silenci només pot córrer dins del composable.
                    val what = liveRot?.let { l -> windVisibleWork(lang, l) ?: windStallNote(lang, l, quietNow(true)) }
                        ?: crewStep?.let { tr(lang, "app.crew.step.$it") }
                    Row(
                        Modifier.fillMaxWidth().padding(vertical = 4.dp),
                        horizontalArrangement = Arrangement.Center,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        TypingDots(rotAccent)
                        if (what != null) Text("  " + what, fontSize = 12.sp, color = rotAccent)
                    }
                }
            }
        }
        if (awayFromLatest) LatestPill(lang) {
            follow.requestFollow(); scope.launch { listState.scrollToLatest() }
        }
        }
        // Aprovacions i preguntes dels vents del grup: el torn espera aquí la teva resposta.
        WindAskCards(vm, lang, rotAccent, busy.contains(rot.id))
        mention?.let { q ->
            MentionSuggestions(lang, q, rotAccent) { w ->
                val handle = "@" + w.name + " "
                field = TextFieldValue(
                    field.text.replaceRange(q.at, field.selection.start, handle),
                    TextRange(q.at + handle.length),
                )
            }
        }
        val waitingAnswer = msgs.lastOrNull()?.kind == "question"
        VentsComposer(lang, field, { field = it }, tr(lang, if (waitingAnswer) "app.crew.answerHint" else "app.company.rotllanaHint"), canSend,
            busy = busy.contains(rot.id), accent = rotAccent, st = st,
            onAttachDoc = { pickDocs.launch("*/*") },
            onAttachImage = { pickPhoto.launch("image/*") },
            onMode = { vm.setMode(it) },
            onPerm = { showPerm = true },
            onWorkspace = if (vm.simpleVents.value) null else ({ vm.go(Screen.Carpeta) }),
            onStop = if (busy.contains(rot.id)) ({ vm.cancelRotllanaTurn(rot.id) }) else null,
            onPreview = { vm.openPreview() },
            showModePill = !vm.simpleVents.value) {
            val body = field.text
            val target = resolveMention(body, faces)
            follow.requestFollow(); vm.sendRotllanaMessage(rot.id, body, target?.id.orEmpty()); field = TextFieldValue("")
        }
        }
    }
    if (groupProfile) GroupProfileDialog(vm, lang, rot, winds) { groupProfile = false }
    speakerProfile?.let { WindProfileDialog(vm, lang, it) { speakerProfile = null } }
    if (askTransfer) TransferConfirmDialog(vm, lang, "rot", rot.id) { askTransfer = false }
    if (showPerm) PermissionDialog(st, lang, onPick = { vm.setAccess(it); showPerm = false }, onClose = { showPerm = false })
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

/** Bytes d'una foto penjada al servidor (id de la pujada), amb la clau. */
internal fun gregalImageBytes(ctx: android.content.Context, id: String): ByteArray? = try {
    if (id.isBlank()) null else {
        val p = ctx.getSharedPreferences("gregal", android.content.Context.MODE_PRIVATE)
        val token = p.getString("token", "").orEmpty()
        val host = p.getString("host", "").orEmpty()
        val port = p.getString("port", "8097").orEmpty()
        if (token.isBlank() || host.isBlank()) null
        else GregalApi(baseUrlOf(host, port), token).officeDownload(id)
    }
} catch (_: Exception) { null }

/** Bitmap del xat a partir dels bytes, reduït a 1024 px de costat com a màxim. */
internal fun decodeChatBytes(bytes: ByteArray?): ImageBitmap? {
    if (bytes == null || bytes.isEmpty() || bytes.size > 8 * 1024 * 1024) return null
    return try {
        val probe = android.graphics.BitmapFactory.Options()
        probe.inJustDecodeBounds = true
        android.graphics.BitmapFactory.decodeByteArray(bytes, 0, bytes.size, probe)
        var sample = 1
        while (maxOf(probe.outWidth, probe.outHeight) / sample > 1024) sample *= 2
        val opts = android.graphics.BitmapFactory.Options()
        opts.inSampleSize = sample
        android.graphics.BitmapFactory.decodeByteArray(bytes, 0, bytes.size, opts)?.asImageBitmap()
    } catch (_: Exception) { null }
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
    val ctx = LocalContext.current
    LaunchedEffect(url) {
        if (bmp == null && !failed) {
            val got = withContext(Dispatchers.IO) {
                // «gregal-image:<id>» és una foto que has penjat tu al servidor:
                // es baixa amb la clau, com els documents.
                if (url.startsWith("gregal-image:")) {
                    decodeChatBytes(gregalImageBytes(ctx, url.removePrefix("gregal-image:")))
                } else loadChatBitmap(url)
            }
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
internal fun ChatEmptyState(face: @Composable () -> Unit, title: String, hint: String,
    suggestions: List<String> = emptyList(), onPick: (String) -> Unit = {}) {
    Column(
        Modifier.fillMaxWidth().padding(vertical = 36.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        BubbleIn("empty-face") { face() }
        Text(title, fontSize = 16.sp, fontWeight = FontWeight.ExtraBold, color = Ink, textAlign = TextAlign.Center)
        Text(hint, fontSize = 13.sp, color = Muted, textAlign = TextAlign.Center, modifier = Modifier.padding(horizontal = 40.dp))
        if (suggestions.isNotEmpty()) {
            Spacer(Modifier.height(6.dp))
            suggestions.forEach { q ->
                Surface(onClick = { onPick(q) }, shape = RoundedCornerShape(18.dp), color = SuperficieAlta.copy(alpha = 0.55f),
                    border = androidx.compose.foundation.BorderStroke(1.dp, Line), modifier = Modifier.padding(horizontal = 28.dp).fillMaxWidth()) {
                    Text(q, color = Ink, fontSize = 14.sp, modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp))
                }
            }
        }
    }
}

/** Primeres frases suggerides per a un xat buit, segons l'ofici del vent. */
internal fun windStarters(lang: String, specialty: String): List<String> {
    val ca = lang == "ca"
    return when (specialty) {
        "shopping" -> if (ca) listOf("Compara’m dos mòbils per menys de 400 €", "Quina cafetera em convé per a dues persones?", "Avisa’m si baixa de preu una cosa que vull")
            else listOf("Compare two phones under €400 for me", "Which coffee maker suits two people?", "Tell me if something I want drops in price")
        "books" -> if (ca) listOf("Recomana’m una novel·la per a aquest cap de setmana", "Un llibre com l’últim que m’ha agradat", "Què val la pena llegir aquest any?")
            else listOf("Recommend a novel for this weekend", "A book like the last one I loved", "What is worth reading this year?")
        "sports" -> if (ca) listOf("Quan juga el meu equip?", "Com va quedar el partit d’ahir?", "Fes-me un resum de la jornada")
            else listOf("When does my team play next?", "How did yesterday’s match end?", "Give me a round-up of the matchday")
        "music" -> if (ca) listOf("Descobreix-me música nova com la que escolto", "Quins concerts hi ha aquest mes a prop?", "Fes-me una llista per cuinar")
            else listOf("Find me new music like what I listen to", "Which concerts are near me this month?", "Make me a playlist for cooking")
        "organisation" -> if (ca) listOf("Recorda’m demà a les 9 que truqui al metge", "Organitza’m la setmana", "Fes-me una llista de la compra")
            else listOf("Remind me tomorrow at 9 to call the doctor", "Plan my week", "Make me a shopping list")
        else -> if (ca) listOf("Què em proposes per avui?", "Explica’m alguna cosa interessant del que t’agrada", "Recorda’m d’aquí a una hora que faci un descans")
            else listOf("What do you suggest for today?", "Tell me something interesting about what you love", "Remind me in an hour to take a break")
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
internal fun WindBubble(raw: GregalViewModel.PetMsg, accent: Color, lang: String = "ca", speaker: Wind? = null,
    speakerName: String = "", onSpeakerClick: (() -> Unit)? = null, indent: Dp = 0.dp,
    onCopy: (() -> Unit)? = null, onContinue: (() -> Unit)? = null,
    onRetry: (() -> Unit)? = null, onTransfer: (() -> Unit)? = null,
    /** Avís opcional en copiar un enllaç (p. ex. «Enllaç copiat»). */
    onCopied: (() -> Unit)? = null,
    /** Missatge a la cua sense connexió: treu-lo abans que surti. */
    onDiscard: (() -> Unit)? = null,
    /** Última bombolla de la seva ratxa: porta la cua (cantonada de 6 dp). */
    tail: Boolean = true,
    /** L'hora (i l'altaveu) només una vegada per ratxa: a l'última. */
    showMeta: Boolean = true) {
    // El bloc ```choices són botons de resposta ràpida, no text de la bombolla.
    val m = remember(raw) { if (raw.from == "user") raw else raw.copy(text = withoutChoices(raw.text)) }
    val mine = m.from == "user"
    if (m.kind == "error") {
        // Un torn interromput no és una resposta: avís tenyit amb l'acció a
        // sota (reintentar el mateix text o continuar al servidor).
        Surface(color = Sand.copy(alpha = 0.12f), shape = RoundedCornerShape(14.dp),
            border = androidx.compose.foundation.BorderStroke(1.dp, Sand.copy(alpha = 0.55f)),
            modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp)) {
            Column(Modifier.padding(horizontal = 14.dp, vertical = 12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Row(verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Icon(Icons.Rounded.Warning, contentDescription = null, tint = Sand, modifier = Modifier.size(20.dp).padding(top = 1.dp))
                    Text(m.text, color = Ink, fontSize = 13.sp, lineHeight = 18.sp, modifier = Modifier.weight(1f))
                }
                if (onRetry != null || onTransfer != null) {
                    Row(horizontalArrangement = Arrangement.spacedBy(4.dp), verticalAlignment = Alignment.CenterVertically) {
                        if (onRetry != null) TextButton(onClick = onRetry) {
                            Text(tr(lang, "app.vents.retry"), color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                        }
                        if (onTransfer != null) TextButton(onClick = onTransfer) {
                            Text(tr(lang, "app.vents.transferToServer"), color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                        }
                    }
                }
            }
        }
        return
    }
    val displayName = speakerName.ifBlank { speaker?.name.orEmpty() }
    // Bombolla pròpia plena amb l'accent de l'app: la grisa (Ink a l'11%) es
    // confonia amb les dels vents, tenyides i fosques també. Ara el que
    // escrius tu es veu d'una ullada, com a qualsevol missatgeria.
    val ownBackground = Sea
    val ownText = OnSea
    // L'hora arriba del backend en segons Unix; els missatges creats en aquesta
    // sessió (enviats per l'app) són en millis. Unificat a millis una sola volta.
    val tsMillis = remember(m.ts) { if (m.ts > 10_000_000_000L) m.ts else m.ts * 1000L }
    val receipt = remember(tsMillis, lang) {
        if (tsMillis > 0L) java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
            .format(java.util.Date(tsMillis)) else ""
    }
    if (mine) {
        Column(Modifier.fillMaxWidth().padding(start = indent), horizontalAlignment = Alignment.End) {
            // Ample màxim 78 %: el Box el limita i la bombolla s'hi arrauleix a la dreta.
            Box(Modifier.fillMaxWidth(GregalChat.userWidth), contentAlignment = Alignment.CenterEnd) {
            Surface(
                shape = if (tail) GregalShape.bubbleTail(mine = true) else GregalShape.bubble,
                color = ownBackground,
                shadowElevation = 0.dp,
                // Prémer llarg copia el missatge en text pla (sense marques).
                modifier = Modifier.combinedClickable(
                    onClick = {}, onLongClick = { onCopy?.invoke() },
                ),
            ) {
                // Les fotos que envies tu també s'han de veure: amb text pla
                // sortia «![foto](gregal-image:…)» en lloc de la foto.
                val ownImages = remember(m.text) { richBlocks(m.text).filterIsInstance<Rich.Image>() }
                val ownText2 = remember(m.text) { stripMarkdownImages(m.text) }
                Column(
                    Modifier.padding(horizontal = GregalChat.bubblePadH, vertical = GregalChat.bubblePadV),
                    verticalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    for (img in ownImages) MdImage(img.url, img.alt, ownText, lang)
                    if (ownText2.isNotBlank()) Text(ownText2, style = GregalType.chatBody, color = ownText)
                }
            }
            }
            // Rebut d'hora com el dots («Read 2:59 PM»): només als enviats
            // en aquesta sessió (l'historial antic no porta hora).
            if (m.kind == "queued") {
                // Escrit sense connexió: surt sol quan el servidor torni.
                Row(Modifier.padding(top = 2.dp, end = 4.dp), verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text("⏳ " + tr(lang, "app.offline.pending"), fontSize = 11.sp, color = Sand)
                    // Un toc de debò (abans era la mida de la lletra, uns 16 dp).
                    if (onDiscard != null) Text(tr(lang, "app.offline.discard"), fontSize = 12.sp, color = Ink.copy(alpha = 0.7f),
                        fontWeight = FontWeight.SemiBold, modifier = Modifier.clip(GregalShape.xs).clickable(onClick = onDiscard)
                            .padding(horizontal = 10.dp, vertical = 12.dp).testTag("discard-queued"))
                }
            } else if (receipt.isNotEmpty() && showMeta) {
                Text(receipt, style = GregalType.chatMeta, color = Muted,
                    modifier = Modifier.padding(top = 3.dp, end = 4.dp))
            }
        }
        return
    }
    // Bombolla o superfície de lectura (regla pura a ChatThread.kt): les
    // respostes curtes van en bombolla suau com a qualsevol missatgeria; les
    // taules, el codi i les respostes llargues, a tot l'ample i sense caixa,
    // que dins d'una bombolla quedaven estretes i feixugues.
    val blocks = remember(m.text, m.streaming) { if (m.streaming) richBlocksLive(m.text) else richBlocks(m.text) }
    val reading = remember(m.text, blocks) { chatSurfaceFor(m.text, blocks) == ChatSurface.READING }
    Row(
        Modifier.fillMaxWidth().padding(start = indent),
        horizontalArrangement = Arrangement.Start,
    ) {
        Box(
            if (reading) Modifier.fillMaxWidth()
            else Modifier.fillMaxWidth(GregalChat.assistantWidth).widthIn(max = GregalChat.assistantMax),
            contentAlignment = Alignment.CenterStart,
        ) {
            val dark = LocalPal.current.dark
            Surface(
                shape = if (tail) GregalShape.bubbleTail(mine = false) else GregalShape.bubble,
                color = if (reading) Color.Transparent else SuperficieAlta,
                shadowElevation = 0.dp,
                // Prémer llarg copia la resposta en text pla (sense marques).
                modifier = (if (reading) Modifier.fillMaxWidth().widthIn(max = 640.dp) else Modifier).combinedClickable(
                    onClick = {}, onLongClick = { onCopy?.invoke() },
                ),
            ) {
                val uri = LocalUriHandler.current
                // Color d'enllaç llegible + blocs parsejats: tot amb remember perquè
                // no es recalculi a cada recomposició (streaming, scroll, teclat).
                // El parse viu a més en cau global: en tornar a la pantalla no se'n
                // refà ni un.
                val link = remember(accent, dark) { readableLink(accent, dark) }
                // El codi en línia, dins la bombolla (que ja és surfaceHi), sobre la superfície.
                val codeBg = if (reading) CodeDark else Superficie
                Column(
                    if (reading) Modifier.padding(horizontal = 6.dp, vertical = 6.dp)
                    else Modifier.padding(horizontal = GregalChat.bubblePadH, vertical = GregalChat.bubblePadV),
                    verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    if (displayName.isNotBlank()) {
                        Text(displayName, color = link, fontSize = 11.sp, fontWeight = FontWeight.SemiBold,
                            modifier = Modifier.then(if (onSpeakerClick != null) Modifier
                                .heightIn(min = 32.dp).clickable(onClick = onSpeakerClick) else Modifier))
                    }
                    // En directe el markdown parcial es trencaria: text pla net
                    // (sense marques crues) fins al done. El cursor que
                    // pampallugueja diu «ho està escrivint ara».
                    // En directe es pinta igual que acabat (sense cau: cada prefix és
                    // nou). Abans era text pla fins al done i, en canviar a
                    // markdown, títols, llistes i taules saltaven de mida de cop.
                    if (m.streaming && m.text.isBlank()) TypingCursor(accent)
                    run { for (b in blocks) {
                        when (b) {
                            is Rich.Inline -> {
                                val rich = remember(b, link, codeBg) { renderSegs(b.segs, codeBg, link = link) }
                                ClickableText(
                                    rich,
                                    style = MaterialTheme.typography.bodyMedium.merge(GregalType.chatBody).copy(color = Tinta),
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
                    if (m.streaming && m.text.isNotBlank()) TypingCursor(accent)
                    // Enllaços del missatge com a pastilles per copiar d'un toc:
                    // al mòbil, obrir-los al navegador és fàcil; copiar-los, no.
                    if (!m.streaming) MessageLinks(lang, m.text, link, onCopied = onCopied)
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
                    // L'hora i l'altaveu per escoltar la resposta (com els grans assistents).
                    if (!m.streaming && showMeta) Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(top = 2.dp)) {
                        if (tsMillis > 0L) Text(receipt, style = GregalType.chatMeta, color = Muted)
                        Spacer(Modifier.weight(1f, fill = false).widthIn(min = 8.dp))
                        ReadAloudButton("msg:" + m.ts + ":" + m.text.hashCode(), m.text, lang, Tinta.copy(alpha = 0.6f))
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
                    if (ui.error) CrewError(lang, ui.errorText)
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
                    if (ui.error) CrewError(lang, ui.errorText)
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
    // La provabilitat del servidor es mira només mentre es veu l'interruptor.
    LifecyclePoll(onServer) {
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
                val dot = reachDot(reach, switching)
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
            onServer -> tr(lang, "app.where.phoneOther")
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

/** «No s'ha pogut fer» i, a sota, el motiu de debò (es pot seleccionar per copiar-lo). */
@Composable
private fun CrewError(lang: String, detail: String) {
    Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(tr(lang, "app.crew.error"), color = Red, fontSize = 13.sp)
        if (detail.isNotBlank()) androidx.compose.foundation.text.selection.SelectionContainer {
            Text(detail, color = Muted, fontSize = 11.sp, lineHeight = 14.sp, modifier = Modifier.testTag("crew-error-detail"))
        }
    }
}

/** On comença la tirada de debat on és idx (-1 si no és debat). */
internal fun debateRunStart(msgs: List<GregalViewModel.PetMsg>, idx: Int): Int {
    if (msgs.getOrNull(idx)?.kind != "debate") return -1
    var i = idx
    while (i > 0 && msgs[i - 1].kind == "debate") i--
    return i
}

/** La línia d'un debat plegat: qui hi ha parlat, quantes intervencions i l'última. */
@Composable
private fun DebateHeader(lang: String, msgs: List<GregalViewModel.PetMsg>, start: Int, faces: List<Wind>, open: Boolean, onToggle: () -> Unit) {
    var end = start
    while (end + 1 < msgs.size && msgs[end + 1].kind == "debate") end++
    val run = msgs.subList(start, end + 1)
    val who = run.mapNotNull { m -> faces.firstOrNull { it.id == m.speakerId } }.distinctBy { it.id }
    val last = stripMdInline(run.last().text.substringAfter(": ")).replace(Regex("\\s+"), " ").trim()
    Surface(onClick = onToggle, shape = RoundedCornerShape(14.dp), color = SuperficieAlta.copy(alpha = .45f),
        modifier = Modifier.fillMaxWidth().padding(top = 10.dp).testTag("crew-debate")) {
        Row(Modifier.padding(horizontal = 12.dp, vertical = 9.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            who.take(4).forEach { WindFace(size = 22, accent = Color(it.argb), style = it.faceStyle) }
            Column(Modifier.weight(1f)) {
                Text(tr(lang, "app.crew.debate", run.size), color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                if (!open) Text(last, color = Muted, fontSize = 13.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
            Text(if (open) "▴" else "▾", color = Muted, fontSize = 16.sp)
        }
    }
}

/** La colla ha preguntat i espera: la resposta del camp de sota la reprèn. */
@Composable
private fun CrewQuestionCard(lang: String, accent: Color) {
    Surface(shape = RoundedCornerShape(14.dp), color = accent.copy(alpha = .14f),
        modifier = Modifier.fillMaxWidth().padding(top = 6.dp).testTag("crew-question")) {
        Text("❓ " + tr(lang, "app.crew.waitingAnswer"), color = Ink, fontSize = 13.sp, lineHeight = 18.sp,
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 9.dp))
    }
}

/** Follow is user intent, not proximity after a newly appended item changes layout. */
internal class ChatFollowState {
    var following by mutableStateOf(true)
        private set
    var dragging by mutableStateOf(false)
        private set
    /**
     * Si la persona vol el final. No és el mateix que [following]: en obrir un
     * fil es «segueix» perquè el fil encara cap a la pantalla, però ningú no ha
     * demanat que el portin a la cua quan creixi.
     */
    var wantsEnd by mutableStateOf(false)
        private set
    fun requestFollow() { following = true; wantsEnd = true }
    /** Deixa de seguir el final sense que hi hagi cap dit: per llegir una resposta des del principi. */
    fun pause() { following = false; wantsEnd = false }
    fun startReading() { dragging = true; following = false; wantsEnd = false }
    fun stopDragging() { dragging = false }
}

/**
 * El fil s'allarga més que la pantalla per primer cop: fins ara no hi havia res
 * a fer scroll i el que s'estava llegint era el final. Si la persona no ha
 * demanat el final (no ha enviat res ni ha baixat a mà), no se la hi porta: es
 * queda on és i surt «Últims ↓». Sense això, el primer creixement del fil
 * arrencava la vista de cop —el «es torna a pintar» que es veia a la conversa.
 */
internal fun shouldFollowOnGrowth(wasScrollable: Boolean, scrollable: Boolean, wantsEnd: Boolean): Boolean =
    !(!wasScrollable && scrollable && !wantsEnd)

@Composable
internal fun rememberChatFollow(list: LazyListState, key: Any): ChatFollowState {
    val follow = remember(list, key) { ChatFollowState() }
    var wasScrollable by remember(list, key) { mutableStateOf(list.canScrollForward) }
    LaunchedEffect(list, follow) {
        list.interactionSource.interactions.collect { interaction ->
            when (interaction) {
                is androidx.compose.foundation.interaction.DragInteraction.Start -> follow.startReading()
                is androidx.compose.foundation.interaction.DragInteraction.Stop,
                is androidx.compose.foundation.interaction.DragInteraction.Cancel -> {
                    follow.stopDragging()
                    if (!list.canScrollForward) follow.requestFollow()
                }
            }
        }
    }
    LaunchedEffect(list, follow) {
        snapshotFlow {
            when {
                follow.dragging || list.isScrollInProgress -> null
                !list.canScrollForward -> LatestChatLayout(atEnd = true)
                !follow.following -> null
                else -> {
                    val info = list.layoutInfo
                    val last = info.visibleItemsInfo.lastOrNull()
                    LatestChatLayout(false, info.totalItemsCount, info.viewportEndOffset,
                        last?.index ?: -1, last?.offset ?: 0, last?.size ?: 0)
                }
            }
        }.conflate().collect { layout ->
            if (layout == null || follow.dragging || list.isScrollInProgress) return@collect
            val scrollable = list.canScrollForward
            if (!list.canScrollForward) follow.requestFollow()
            else if (follow.following) {
                if (!shouldFollowOnGrowth(wasScrollable, scrollable, follow.wantsEnd)) follow.pause()
                else {
                    // A drag can cancel the scroll mutation without ending the observer.
                    kotlinx.coroutines.coroutineScope { launch { list.scrollToLatest() }.join() }
                }
            }
            wasScrollable = scrollable
        }
    }
    return follow
}

private data class LatestChatLayout(
    val atEnd: Boolean,
    val count: Int = 0,
    val viewportEnd: Int = 0,
    val lastIndex: Int = -1,
    val lastOffset: Int = 0,
    val lastSize: Int = 0,
)


/**
 * En acabar un torn, una resposta que no cap a la pantalla es llegeix des del
 * principi: es posa el seu inici a dalt i es deixa de seguir el final (surt
 * «Últims ↓»). Sense això, la vista quedava a la cua i es començava a llegir
 * pel final. Mentre arriba en directe, es continua seguint el final.
 * [replyIndex]: l'element de la llista on és l'última resposta del vent.
 */
@Composable
internal fun ReadReplyFromStart(list: LazyListState, follow: ChatFollowState, busy: Boolean, replyKey: Any?) {
    var wasBusy by remember(list) { mutableStateOf(busy) }
    // La resposta pot arribar just després que el torn acabi: es llegeix la clau d'ara.
    val reply by rememberUpdatedState(replyKey)
    // En directe (el text creix): se segueix el final fins que l'inici de la
    // resposta arriba a dalt; aleshores la vista s'hi queda i surt «Últims ↓».
    // Abans se seguia la cua fins al final i, en acabar, es saltava enrere a
    // l'inici: amb el text en directe era un salt cada resposta llarga.
    LaunchedEffect(busy, list) {
        if (!busy) return@LaunchedEffect
        snapshotFlow {
            val key = reply ?: return@snapshotFlow null
            if (!follow.following || follow.dragging) return@snapshotFlow null
            val info = list.layoutInfo
            info.visibleItemsInfo.firstOrNull { it.key == key }?.let { it.index to (it.offset - info.viewportStartOffset) }
        }.collect { hit ->
            val (index, top) = hit ?: return@collect
            if (top < 0 && follow.following && !follow.dragging) {
                follow.pause()
                list.scrollToItem(index)
            }
        }
    }
    LaunchedEffect(busy) {
        val finished = wasBusy && !busy
        wasBusy = busy
        // Si ja no se segueix el final (fixada a l'inici o la persona llegeix), no es mou res.
        if (!finished || !follow.following) return@LaunchedEffect
        // Que la resposta definitiva ja sigui al layout abans de mesurar-la.
        kotlinx.coroutines.delay(120)
        val key = reply ?: return@LaunchedEffect
        val info = list.layoutInfo
        val viewport = info.viewportEndOffset - info.viewportStartOffset
        val item = info.visibleItemsInfo.firstOrNull { it.key == key } ?: return@LaunchedEffect
        if (item.size <= viewport * 3 / 4 || !follow.following) return@LaunchedEffect
        follow.pause()
        list.animateScrollToItem(item.index)
    }
}

/** La clau de la resposta del torn en curs: l'últim missatge que no és teu, si va després de l'últim teu. */
internal fun currentReplyKey(msgs: List<GregalViewModel.PetMsg>): Any? {
    val lastUser = msgs.indexOfLast { it.from == "user" }
    val reply = msgs.indexOfLast { it.from != "user" && it.from != "system" }
    return if (reply > lastUser) reply else null
}

/** A tall last bubble must be aligned at its bottom, not its top. */
internal suspend fun LazyListState.scrollToLatest(animate: Boolean = false) {
    val last = layoutInfo.totalItemsCount - 1
    if (last < 0) return
    if (animate) animateScrollToItem(last, Int.MAX_VALUE) else scrollToItem(last, Int.MAX_VALUE)
}

/** Píndola que apareix quan has pujat a llegir: un toc i tornes a l'últim missatge. */
@Composable
internal fun BoxScope.LatestPill(lang: String, onClick: () -> Unit) {
    Surface(onClick = onClick, shape = RoundedCornerShape(999.dp), color = Ink,
        modifier = Modifier.align(Alignment.BottomEnd).padding(end = 18.dp, bottom = 14.dp)) {
        Text(tr(lang, "app.vents.latest"), color = Superficie, fontSize = 12.sp,
            modifier = Modifier.padding(horizontal = 14.dp, vertical = 9.dp))
    }
}

/**
 * Claus per fila de xat. La posició no val: quan el fil es torna a omplir
 * (o n'arriba un de més antic) totes les bombolles canvien de lloc, la llista
 * sencera es recompon i l'scroll salta a una altra banda. Amb una clau
 * d'identitat, el que ja es veia es queda on era.
 *
 * `idOf` ha de ser estable en el temps (una marca del servidor, no el text:
 * un missatge en directe canvia a cada fragment). Si dues files coincideixen
 * —mateix segon, mateix parlant— es desenmpalen amb un contador.
 */
internal fun <T> chatRowKeys(items: List<T>, idOf: (T) -> String): List<String> {
    val used = HashSet<String>(items.size)
    val seen = HashMap<String, Int>(items.size)
    return items.map { item ->
        val base = idOf(item)
        var n = seen.merge(base, 1) { a, b -> a + b } ?: 1
        var key = if (n == 1) base else "$base#$n"
        // Un identificador que ja porta el sufix no pot xocar amb el d'una altra fila.
        while (!used.add(key)) {
            n++; seen[base] = n; key = "$base#$n"
        }
        key
    }
}
