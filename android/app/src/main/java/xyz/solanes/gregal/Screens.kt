package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.wrapContentWidth
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Search
import androidx.compose.material.icons.rounded.Flag
import androidx.compose.material.icons.rounded.Forum
import androidx.compose.material.icons.rounded.FolderOpen
import androidx.compose.material.icons.rounded.Lock
import androidx.compose.material.icons.rounded.Person
import androidx.compose.material.icons.rounded.Tag
import androidx.compose.material.icons.rounded.Dns
import androidx.compose.material.icons.automirrored.rounded.Undo
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.ChatBubbleOutline
import androidx.compose.material.icons.rounded.Check
import androidx.compose.material.icons.rounded.CompareArrows
import androidx.compose.material.icons.rounded.ContentCopy
import androidx.compose.material.icons.rounded.Delete
import androidx.compose.material.icons.rounded.Description
import androidx.compose.material.icons.rounded.ForkRight
import androidx.compose.material.icons.rounded.Image
import androidx.compose.material.icons.rounded.Lightbulb
import androidx.compose.material.icons.rounded.Menu
import androidx.compose.material.icons.rounded.PlayArrow
import androidx.compose.material.icons.rounded.Send
import androidx.compose.material.icons.rounded.TaskAlt
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.unit.dp
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.slideInVertically
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.ClickableText
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.rounded.KeyboardArrowDown
import androidx.compose.material.icons.rounded.Mic
import androidx.compose.material.icons.rounded.MoreVert
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.platform.LocalUriHandler
import kotlin.math.PI
import kotlin.math.sin
import android.content.Intent
import android.speech.RecognizerIntent
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.lifecycle.Lifecycle
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.lifecycle.repeatOnLifecycle

/** Amplada màxima de lectura del xat (tauleta, horitzontal), com a l'escriptori. */
val ChatMaxWidth = 760.dp

/** Identitat Gregal: mar fosc, verd marí (Sea) i sorra (Sand). Mai clons. */
// Paleta: ara a Theme.kt (LocalPal; getters Sea/Ink/… temàtics).

/** Fons degradat de mar profunda. */
/** Fons del mar amb el vel d'escuma de la part alta (identitat: `fons` →
 *  `abisme`). Abans era un degradat pla i l'app no tenia cap to de marca. */
@Composable
fun GregalBg(content: @Composable () -> Unit) = SeaBackdrop(Modifier.fillMaxSize(), content)

@Composable
fun PairingScreen(vm: GregalViewModel, prefs: Prefs) {
    val lang by vm.idioma.collectAsState()
    var host by remember { mutableStateOf(prefs.host) }
    var port by remember { mutableStateOf(prefs.port.ifEmpty { "8097" }) }
    var usuari by remember { mutableStateOf(prefs.usuari) }
    var contrasenya by remember { mutableStateOf("") }
    var token by remember { mutableStateOf(prefs.token) }
    // El token queda amagat: amb usuaris configurats ja no cal, pero el
    // servidor sense comptes (us local) nomes enten aquest cami.
    var ambToken by remember { mutableStateOf(prefs.token.isNotBlank()) }
    val notice by vm.notice.collectAsState()
    val local by vm.localState.collectAsState()
    val localOk = remember { vm.localAvailable() }
    // El remot és l'opció: plegat si l'app pot treballar sola, obert si
    // l'havies triat abans o si aquesta build no porta el backend.
    var remot by remember { mutableStateOf(!localOk || prefs.target == Prefs.TARGET_REMOTE) }
    GregalBg {
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).imePadding().padding(horizontal = 28.dp, vertical = 36.dp),
            verticalArrangement = Arrangement.Center,
        ) {
            Spacer(Modifier.height(32.dp))
            GregalMark(64.dp, alive = true)
            Spacer(Modifier.height(14.dp))
            Text("Gregal", style = MaterialTheme.typography.displaySmall, color = Ink)
            Text(tr(lang, "app.pair.subtitle"), color = Muted, style = MaterialTheme.typography.bodyLarge)
            Spacer(Modifier.height(28.dp))
            if (localOk) {
                // Aquest dispositiu: el mateix backend que l'escriptori, dins l'app.
                GlassCard(Modifier.fillMaxWidth(), hi = !remot) {
                    Column(Modifier.padding(16.dp)) {
                        Text(tr(lang, "app.server.thisDevice"), color = Ink, fontSize = 17.sp, fontWeight = FontWeight.SemiBold)
                        Spacer(Modifier.height(4.dp))
                        Text(tr(lang, "app.server.thisDeviceHelp"), color = Muted, fontSize = 13.sp)
                        Spacer(Modifier.height(12.dp))
                        val starting = local is LocalState.Starting
                        Button(onClick = { vm.useThisDevice() }, enabled = !starting, shape = RoundedCornerShape(14.dp),
                            modifier = Modifier.fillMaxWidth().height(52.dp)) {
                            Text(tr(lang, if (starting) "app.server.starting" else "app.server.useThisDevice"), fontSize = 16.sp)
                        }
                        if (starting) {
                            Spacer(Modifier.height(8.dp))
                            LinearProgressIndicator(Modifier.fillMaxWidth(), color = Sea, trackColor = Line)
                        }
                        val failed = local as? LocalState.Failed
                        if (failed != null) {
                            Spacer(Modifier.height(10.dp))
                            Text(tr(lang, "app.local.failed", failed.message), color = Sand, fontSize = 13.sp)
                            if (failed.log.isNotBlank()) {
                                Spacer(Modifier.height(6.dp))
                                Text(failed.log.takeLast(1200), color = Muted, fontSize = 11.sp,
                                    fontFamily = FontFamily.Monospace)
                            }
                        }
                    }
                }
                Spacer(Modifier.height(14.dp))
                TextButton(onClick = { remot = !remot }, modifier = Modifier.fillMaxWidth()) {
                    Text(tr(lang, if (remot) "app.server.hideRemote" else "app.server.showRemote"),
                        color = Muted, fontSize = 13.sp)
                }
            }
            if (remot) {
                Text(tr(lang, "app.server.remoteHelp"), color = Muted, fontSize = 13.sp)
                Spacer(Modifier.height(10.dp))
                val field = RoundedCornerShape(14.dp)
                OutlinedTextField(host, { host = it }, label = { Text(tr(lang, "app.pair.host")) },
                    leadingIcon = { Icon(Icons.Rounded.Dns, null, tint = Muted) }, shape = field,
                    modifier = Modifier.fillMaxWidth(), singleLine = true)
                Spacer(Modifier.height(10.dp))
                OutlinedTextField(port, { port = it }, label = { Text(tr(lang, "app.pair.port")) },
                    leadingIcon = { Icon(Icons.Rounded.Tag, null, tint = Muted) }, shape = field,
                    keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = androidx.compose.ui.text.input.KeyboardType.Number),
                    modifier = Modifier.fillMaxWidth(), singleLine = true)
                Spacer(Modifier.height(10.dp))
                OutlinedTextField(usuari, { usuari = it }, label = { Text(tr(lang, "app.pair.user")) },
                    leadingIcon = { Icon(Icons.Rounded.Person, null, tint = Muted) }, shape = field,
                    modifier = Modifier.fillMaxWidth(), singleLine = true)
                Spacer(Modifier.height(10.dp))
                OutlinedTextField(contrasenya, { contrasenya = it }, label = { Text(tr(lang, "app.pair.password")) },
                    leadingIcon = { Icon(Icons.Rounded.Lock, null, tint = Muted) }, shape = field,
                    modifier = Modifier.fillMaxWidth(), singleLine = true,
                    visualTransformation = PasswordVisualTransformation())
                Spacer(Modifier.height(18.dp))
                val login: @Composable () -> Unit = { Text(tr(lang, "app.pair.login"), fontSize = 16.sp) }
                if (localOk) {
                    OutlinedButton(onClick = { vm.entra(host, port, usuari, contrasenya) }, shape = RoundedCornerShape(14.dp),
                        modifier = Modifier.fillMaxWidth().height(52.dp)) { login() }
                } else {
                    Button(onClick = { vm.entra(host, port, usuari, contrasenya) }, shape = RoundedCornerShape(14.dp),
                        modifier = Modifier.fillMaxWidth().height(52.dp)) { login() }
                }
                TextButton(onClick = { ambToken = !ambToken }, modifier = Modifier.fillMaxWidth()) {
                    Text(tr(lang, if (ambToken) "app.pair.hideToken" else "app.pair.showToken"),
                        color = Muted, fontSize = 13.sp)
                }
                if (ambToken) {
                    OutlinedTextField(token, { token = it }, label = { Text(tr(lang, "app.pair.token")) },
                        modifier = Modifier.fillMaxWidth(), singleLine = true)
                    Spacer(Modifier.height(8.dp))
                    OutlinedButton(onClick = { vm.connectRemote(host, port, token) },
                        modifier = Modifier.fillMaxWidth()) { Text(tr(lang, "app.pair.connect")) }
                }
            }
            val ctx = LocalContext.current
            val ver = remember {
                try {
                    @Suppress("DEPRECATION")
                    ctx.packageManager.getPackageInfo(ctx.packageName, 0).versionName
                } catch (_: Exception) { null }
            }
            if (ver != null) {
                Spacer(Modifier.height(16.dp))
                Text("Gregal Android v$ver", color = Muted, fontSize = 12.sp,
                    textAlign = TextAlign.Center, modifier = Modifier.fillMaxWidth())
            }
            if (notice.isNotEmpty()) {
                Spacer(Modifier.height(12.dp))
                ConnectionNotice(notice, lang, onRetry = {
                    // Reintenta allò que s'havia triat: el dispositiu o el remot.
                    if (prefs.target == Prefs.TARGET_LOCAL && localOk) vm.useThisDevice()
                    else vm.connectRemote(host, port, token)
                })
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ChatScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val msgs by vm.msgs.collectAsState()
    val sending by vm.sending.collectAsState()
    val st by vm.state.collectAsState()
    val approve by vm.approve.collectAsState()
    val notice by vm.notice.collectAsState()
    val snack = remember { SnackbarHostState() }
    var input by remember { mutableStateOf("") }
    // L'esborrany viu al ViewModel (i a les prefs): el que has escrit no es
    // perd en girar el mòbil, canviar de pantalla ni morir el procés.
    val draft by vm.draft.collectAsState()
    LaunchedEffect(draft) { if (input != draft) input = draft }
    fun setInput(v: String) { input = v; vm.saveDraft(v) }
    fun consumeInput(): String { val t = input; setInput(""); return t }
    var showModelMenu by remember { mutableStateOf(false) }
    // El diàleg de GitHub viu al ViewModel perquè també s'obre des del
    // calaix (a la barra de dalt ja no hi ha icones d'eines).
    val showGithub by vm.ghObert.collectAsState()
    var ghTool by remember { mutableStateOf("gh_pr") }
    var ghAction by remember { mutableStateOf("list") }
    var ghNumber by remember { mutableStateOf("") }
    var ghQuery by remember { mutableStateOf("") }
    var ghRepo by remember { mutableStateOf("") }
    var follow by remember { mutableStateOf(true) }
    val queued by vm.queued.collectAsState()
    val plan by vm.plan.collectAsState()
    val planning by vm.planning.collectAsState()
    val ctx = LocalContext.current
    val voice = rememberLauncherForActivityResult(ActivityResultContracts.StartActivityForResult()) { res ->
        res.data?.getStringArrayListExtra(RecognizerIntent.EXTRA_RESULTS)?.firstOrNull()
            ?.trim()?.takeIf { it.isNotEmpty() }?.let { dit ->
                setInput((input.trim() + " " + dit).trim())
            }
    }
    val pendingImgs by vm.pendingImgs.collectAsState()
    val turnT0 by vm.turnStart.collectAsState()
    val statusLine by vm.statusLine.collectAsState()
    val question by vm.question.collectAsState()
    val showRewind by vm.showRewind.collectAsState()
    val checkpoints by vm.checkpoints.collectAsState()
    val live by vm.live.collectAsState()
    val needsProvider by vm.needsProvider.collectAsState()
    val activity by vm.activity.collectAsState()
    val planFirst by vm.planFirst.collectAsState()
    var showPlus by remember { mutableStateOf(false) }
    var showPerm by remember { mutableStateOf(false) }
    var confirmNew by remember { mutableStateOf(false) }
    val pickImgs = rememberLauncherForActivityResult(ActivityResultContracts.GetMultipleContents()) { uris ->
        uris.take(3).forEach { vm.addImage(it) }
    }
    val voiceOk = remember {
        try {
            ctx.packageManager.queryIntentActivities(
                Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH), 0,
            ).isNotEmpty()
        } catch (_: Exception) { false }
    }
    val listState = rememberLazyListState()
    val scope = rememberCoroutineScope()
    val haptic = LocalHapticFeedback.current
    var composerFocused by remember { mutableStateOf(false) }
    val busy = sending || st.agentBusy

    //Segueix el final només si l'usuari ja era a baix: pujar a rellegir
    // no ha de ser interromput per cada lot de streaming.
    // Deixa de seguir el final només quan la persona arrossega cap amunt; hi
    // torna quan arriba a baix. (Abans el primer layout ja l'apagava.)
    LaunchedEffect(listState) {
        listState.interactionSource.interactions.collect { i ->
            if (i is androidx.compose.foundation.interaction.DragInteraction.Start) follow = false
        }
    }
    LaunchedEffect(listState) {
        // Si el contingut creix (targeta que s'obre, text en viu), es queda a baix.
        snapshotFlow { listState.canScrollForward }.collect { more ->
            if (!more) follow = true
            else if (follow && !listState.isScrollInProgress) {
                val n = listState.layoutInfo.totalItemsCount
                if (n > 0) listState.scrollToItem(n - 1)
            }
        }
    }
    LaunchedEffect(notice) { if (notice.isNotEmpty()) { snack.showSnackbar(notice); vm.clearNotice() } }
    LaunchedEffect(showModelMenu) { if (showModelMenu) vm.loadModels() }

    Scaffold(
        containerColor = Color.Transparent,
        topBar = {
            TopAppBar(
                navigationIcon = {
                    IconButton(onClick = onMenu) { Icon(Icons.Rounded.Menu, tr(lang, "app.nav.menu"), tint = Ink) }
                },
                title = {
                    // Estil ChatGPT: el títol és el model, centrat, i prou.
                    // El projecte és al capdamunt del calaix.
                    CappedFontScale(1.3f) {
                    Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                        TextButton(onClick = { showModelMenu = true }) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Text(
                                        st.model.ifEmpty { st.currentRole.ifEmpty { "Gregal" } },
                                        color = Ink, fontSize = 16.sp, fontWeight = FontWeight.SemiBold,
                                        maxLines = 1, overflow = TextOverflow.Ellipsis,
                                    )
                                    Icon(Icons.Rounded.KeyboardArrowDown, null, tint = Muted, modifier = Modifier.size(18.dp))
                                }
                                if (st.project.isNotBlank()) {
                                    Text(
                                        st.project + if (st.branch.isNotBlank()) " · " + st.branch else "",
                                        color = Faint, fontSize = 11.5.sp, maxLines = 1, overflow = TextOverflow.Ellipsis,
                                    )
                                }
                            }
                        }
                        ModelMenu(vm, st, showModelMenu, { showModelMenu = false })
                    }
                    }
                },
                actions = {
                    // Una sola acció, com ChatGPT: conversa nova. Desfés i
                    // GitHub són al calaix; pla i comparació al desplegable.
                    IconButton(onClick = { if (busy) confirmNew = true else { vm.newSession(); vm.go(Screen.Chat) } }) {
                        Icon(Icons.Rounded.Add, tr(lang, "nav.new"), tint = Ink, modifier = Modifier.size(24.dp))
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent),
            )
        },
        snackbarHost = { SnackbarHost(snack) },
        bottomBar = {
            // Mentre falta el proveïdor no hi ha res a enviar, i amb el
            // teclat obert el composer tapava «Desa i comença».
            // A la tauleta, el composer té la mateixa amplada de lectura que el xat.
            if (!(msgs.isEmpty() && needsProvider)) Column(
                Modifier.imePadding().fillMaxWidth()
                    .wrapContentWidth(Alignment.CenterHorizontally).widthIn(max = ChatMaxWidth),
            ) {
                if (st.agentBusy && !sending) AgentBusyStrip(lang)
                if (queued) {
                    Surface(
                        color = CodeDark, shape = RoundedCornerShape(14.dp),
                        modifier = Modifier.fillMaxWidth().padding(horizontal = 18.dp, vertical = 2.dp),
                    ) {
                        Text(tr(lang, "app.chat.queued"),
                            color = Sea, fontSize = 13.sp, textAlign = TextAlign.Center,
                            modifier = Modifier.padding(horizontal = 12.dp, vertical = 7.dp))
                    }
                }
                if (planning || plan != null) {
                    Card(
                        modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 4.dp),
                        colors = CardDefaults.cardColors(containerColor = PillDark),
                        shape = RoundedCornerShape(18.dp),
                    ) {
                        Column(Modifier.padding(14.dp)) {
                            Text(if (planning) tr(lang, "app.chat.planning") else tr(lang, "app.chat.planLabel"),
                                color = Sea, fontWeight = FontWeight.Bold, fontSize = 13.sp)
                            if (plan != null) {
                                Spacer(Modifier.height(6.dp))
                                Text(renderMd(plan!!.take(1500), CodeDark),
                                    color = Ink, fontSize = 13.sp)
                                Spacer(Modifier.height(8.dp))
                                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                    FilledTonalButton(onClick = {
                                        haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                                        vm.runPlan()
                                    }) {
                                        Icon(Icons.Rounded.PlayArrow, tr(lang, "app.chat.execute"))
                                        Spacer(Modifier.width(4.dp))
                                        Text(tr(lang, "app.chat.execute"))
                                    }
                                    OutlinedButton(onClick = { vm.dismissPlan() }) {
                                        Text(tr(lang, "app.chat.discard"), color = Muted)
                                    }
                                }
                            }
                        }
                    }
                }
                if (pendingImgs.isNotEmpty()) {
                    Row(
                        Modifier.fillMaxWidth().padding(horizontal = 16.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text("🖼×${pendingImgs.size}", color = Sea, fontSize = 14.sp)
                        Spacer(Modifier.width(8.dp))
                        pendingImgs.forEachIndexed { i, _ ->
                            TextButton(onClick = { vm.removeImage(i) }, modifier = Modifier.semantics {
                                contentDescription = tr(lang, "btn.delete") + " ${i + 1}"
                            }) {
                                Text("✕${i + 1}", color = Muted, fontSize = 13.sp)
                            }
                        }
                    }
                }
                // Xips del que no és el normal: mode especial i permisos oberts.
                val special = st.mode in setOf("goal", "autonomous", "debug", "inspect")
                if (special || st.permissive || st.autoReview) {
                    Row(
                        Modifier.fillMaxWidth().padding(horizontal = 14.dp, vertical = 2.dp),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        if (special) {
                            ComposerChip(
                                "◆ " + tr(lang, "app.mode." + st.mode), Lila, null,
                                onClick = { showPlus = true },
                                onClose = { vm.setMode("code") },
                            )
                        }
                        if (st.permissive || st.autoReview) {
                            ComposerChip(
                                tr(lang, if (st.autoReview) "app.perm.review" else "app.perm.auto"), Sand,
                                { PermIcon(true, Sand) }, onClick = { showPerm = true }, onClose = null,
                            )
                        }
                    }
                }
                if (busy && input.isNotBlank()) {
                    Text(tr(lang, "app.steer.hint"), color = Faint, fontSize = 11.5.sp,
                        modifier = Modifier.padding(start = 26.dp, bottom = 2.dp))
                }
                // Mesurador de context fora de la pastilla (estil ChatGPT: el
                // composer només té controls). S'amaga sol si no hi ha res
                // gastat o el rol no declara finestra.
                TideGaugeBar(
                    st.usedTokens, st.contextWindow, cells = 8,
                    modifier = Modifier.padding(start = 26.dp, bottom = 1.dp),
                )
                Surface(
                    modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp)
                        .shadow(if (composerFocused) 6.dp else 2.dp, RoundedCornerShape(24.dp)),
                    shape = RoundedCornerShape(24.dp), color = PillDark,
                    border = BorderStroke(
                        1.dp,
                        if (composerFocused) Sea.copy(alpha = 0.55f) else Line,
                    ),
                ) {
                    Column(
                        Modifier.fillMaxWidth().padding(horizontal = 10.dp, vertical = 6.dp),
                    ) {
                        BasicTextField(
                            value = input,
                            onValueChange = { setInput(it) },
                            modifier = Modifier.fillMaxWidth().padding(horizontal = 4.dp, vertical = 10.dp)
                                .onFocusChanged { composerFocused = it.isFocused },
                            textStyle = MaterialTheme.typography.bodyLarge.copy(color = Ink),
                            cursorBrush = androidx.compose.ui.graphics.SolidColor(Sea),
                            maxLines = 6,
                            decorationBox = { inner ->
                                if (input.isEmpty()) Text(composerHint(st.mode, lang), color = Muted,
                                    style = MaterialTheme.typography.bodyLarge, maxLines = 1,
                                    overflow = TextOverflow.Ellipsis)
                                inner()
                            },
                        )
                        Row(
                            Modifier.fillMaxWidth(),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Box {
                                IconButton(onClick = { showPlus = true }, modifier = Modifier.size(44.dp)) {
                                    Icon(Icons.Rounded.Add, tr(lang, "app.mode.more"), tint = Muted, modifier = Modifier.size(23.dp))
                                }
                                ComposerPlusMenu(
                                    expanded = showPlus, onClose = { showPlus = false }, lang = lang, st = st,
                                    canUseInput = !busy && input.isNotBlank(), canAttach = !sending && pendingImgs.size < 3,
                                    onAttach = { pickImgs.launch("image/*") },
                                    onPlanOnce = { haptic.performHapticFeedback(HapticFeedbackType.LongPress); vm.requestPlan(consumeInput()) },
                                    onCompare = { haptic.performHapticFeedback(HapticFeedbackType.LongPress); vm.parallel(consumeInput()) },
                                    onMode = { vm.setMode(it) },
                                    onPerm = { showPerm = true },
                                )
                            }
                            Spacer(Modifier.width(2.dp))
                            CappedFontScale(1.3f) {
                                AskActToggle(
                                    act = st.mode != "chat", enabled = !sending, lang = lang,
                                    onChange = { act ->
                                        haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                                        vm.setMode(if (act) "code" else "chat")
                                    },
                                )
                            }
                            if (st.mode == "code") {
                                Spacer(Modifier.width(4.dp))
                                CappedFontScale(1.3f) { PlanFirstToggle(planFirst, lang) { vm.setPlanFirst(it) } }
                            }
                            Spacer(Modifier.weight(1f))
                            SendStopButton(
                                busy = busy, canSend = !sending && input.isNotBlank(), voice = voiceOk, lang = lang,
                                onSend = {
                                    haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                                    follow = true; vm.submit(consumeInput())
                                },
                                onInterrupt = {
                                    haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                                    follow = true; vm.submit(consumeInput(), interrupt = true)
                                },
                                onStop = {
                                    haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                                    vm.cancelCurrent()
                                },
                                onVoice = {
                                    try {
                                        voice.launch(
                                            Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).apply {
                                                putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
                                                putExtra(RecognizerIntent.EXTRA_LANGUAGE, if (lang == "en") "en-US" else "ca-ES")
                                                putExtra(RecognizerIntent.EXTRA_PROMPT, tr(lang, "app.chat.voicePrompt"))
                                            },
                                        )
                                    } catch (_: Exception) {
                                        vm.notify(tr(lang, "app.chat.voiceUnavailable"))
                                    }
                                },
                            )
                        }
                    }
                }
            }
        },
    ) { pad ->
        if (msgs.isEmpty() && needsProvider) {
            // Primer ús en aquest dispositiu: sense cap clau, el primer torn
            // acabava en un 401. Abans d'escriure, connecta un proveïdor.
            Column(
                Modifier.fillMaxSize().padding(pad).imePadding().verticalScroll(rememberScrollState()).padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Spacer(Modifier.height(8.dp))
                GregalMark(56.dp, alive = true)
                Text(tr(lang, "app.setup.welcome"), style = MaterialTheme.typography.headlineSmall, color = Ink,
                    textAlign = TextAlign.Center)
                ProviderSettingsCta(lang) { vm.go(Screen.Prefs) }
            }
        } else if (msgs.isEmpty()) {
            // Estat buit: la salutació i prou. El model es veu a la
            // capçalera, i els xips de suggeriments competien amb el text.
            Column(
                Modifier.fillMaxSize().padding(pad),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                GregalMark(76.dp, alive = true)
                Spacer(Modifier.height(18.dp))
                Text(tr(lang, "app.chat.empty"), style = MaterialTheme.typography.headlineMedium, color = Ink,
                    textAlign = TextAlign.Center, modifier = Modifier.padding(horizontal = 24.dp))
                if (st.project.isNotBlank()) {
                    Spacer(Modifier.height(14.dp))
                    ProjectPill(st.project, st.branch)
                }
            }
        } else {
            Box(Modifier.fillMaxSize().padding(pad)) {
                val rows = remember(msgs, lang) { chatRows(msgs, lang) }
                // L'última fila («end») és a rows.size (+1 si hi ha text en viu):
                // es pot demanar abans del primer layout, a diferència de layoutInfo.
                val endIndex = rows.size + if (live.isNotEmpty()) 1 else 0
                LaunchedEffect(endIndex, live.length / 80, msgs.lastOrNull()?.work?.steps?.size) {
                    if (follow) listState.scrollToItem(endIndex)
                }
                // A la tauleta les línies de tota l'amplada no es llegien: com a
                // l'escriptori, una columna centrada d'amplada de lectura.
                LazyColumn(
                    Modifier.fillMaxHeight().align(Alignment.TopCenter).widthIn(max = ChatMaxWidth)
                        .fillMaxWidth().padding(horizontal = 12.dp),
                    state = listState,
                ) {
                    items(
                        count = rows.size,
                        key = { ri -> when (val r = rows[ri]) { is UiRow.Day -> "d:" + r.label + ri; is UiRow.Msg -> r.msg.id } },
                        contentType = { ri -> when (val r = rows[ri]) { is UiRow.Day -> 0; is UiRow.Msg -> r.msg.kind.hashCode() } },
                    ) { ri ->
                        when (val r = rows[ri]) {
                            is UiRow.Day -> DayDivider(r.label)
                            is UiRow.Msg -> {
                                val m = r.msg
                                val index = r.index
                                when {
                                    m.kind == "work" && m.work != null -> WorkGroupCard(
                                        m.work, if (m.work.running) activity else null,
                                        if (m.work.running) statusLine else "", lang,
                                        onRetro = if (m.work.runId > 0) ({ vm.openRetro(m.work.runId) }) else null,
                                    )
                                    m.kind == "activity" -> {
                                        // Avisos sense torn (cua, comparació…): una targeta per tram.
                                        if (index == 0 || msgs[index - 1].kind != "activity") {
                                            val run = remember(msgs, index) { msgs.drop(index).takeWhile { it.kind == "activity" } }
                                            ActivityCluster(run, lang)
                                        }
                                    }
                                    else -> MsgRow(m, index, lang) { vm.notify(tr(lang, "app.chat.copied")) }
                                }
                            }
                        }
                    }
                    if (live.isNotEmpty()) item(key = "live", contentType = "live") { LiveAnswer(live, lang) }
                    item(key = "end", contentType = "end") { Spacer(Modifier.height(8.dp)) }
                }
                if (!follow && msgs.isNotEmpty()) {
                    Surface(
                        onClick = {
                            follow = true
                            scope.launch { listState.animateScrollToItem((listState.layoutInfo.totalItemsCount - 1).coerceAtLeast(0)) }
                        },
                        shape = CircleShape, color = Superficie, shadowElevation = 4.dp,
                        border = BorderStroke(1.dp, Line),
                        modifier = Modifier.align(Alignment.BottomCenter).padding(10.dp).size(44.dp),
                    ) {
                        Box(contentAlignment = Alignment.Center) {
                            Icon(Icons.Rounded.KeyboardArrowDown, tr(lang, "app.chat.scrollBottom"), tint = Ink)
                        }
                    }
                }
            }
        }
    }

    approve?.let { a ->
        val summary = remember(a.key, a.args, lang) { toolSummary(a.name, a.args, lang) }
        val pretty = remember(a.key, a.args, lang) { prettyArgs(a.args, lang) }
        AlertDialog(
            onDismissRequest = {},
            title = { Text("🔐 ${toolTitle(a.name, lang)}") },
            text = {
                Column(Modifier.verticalScroll(rememberScrollState())) {
                    Text(summary, color = Ink, fontSize = 14.sp)
                    Spacer(Modifier.height(8.dp))
                    Surface(
                        shape = RoundedCornerShape(10.dp), color = CodeDark,
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Text(
                            pretty.take(800), color = Sand, fontSize = 12.sp,
                            fontFamily = androidx.compose.ui.text.font.FontFamily.Monospace,
                            modifier = Modifier.padding(10.dp),
                        )
                    }
                }
            },
            // Tres botons apilats a tota l'amplada: amb el text en català no
            // hi cabien en una fila i «Denega» quedava sol a sota.
            confirmButton = {
                Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = {
                        haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                        vm.answerApprove(true)
                    }, modifier = Modifier.fillMaxWidth().height(48.dp)) { Text(tr(lang, "btn.allow")) }
                    FilledTonalButton(onClick = {
                        haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                        vm.answerApprove(true, remember = true)
                    }, modifier = Modifier.fillMaxWidth().height(48.dp)) { Text(tr(lang, "app.approve.always")) }
                    OutlinedButton(onClick = {
                        haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                        vm.answerApprove(false)
                    }, modifier = Modifier.fillMaxWidth().height(48.dp)) { Text(tr(lang, "btn.deny"), color = Red) }
                }
            },
        )
    }

    // Pregunta de l'agent (`question_request`): la web i el TUI l'esperen i
    // el mòbil ni la veia — el torn es quedava 120 s aturat sense dir res.
    question?.let { q ->
        var free by remember(q.key) { mutableStateOf("") }
        AlertDialog(
            onDismissRequest = {},
            title = { Text(tr(lang, "app.chat.questionTitle")) },
            text = {
                Column(Modifier.verticalScroll(rememberScrollState())) {
                    Text(q.query, color = Ink, fontSize = 15.sp)
                    if (q.options.isNotEmpty()) {
                        Spacer(Modifier.height(10.dp))
                        q.options.forEach { o ->
                            TextButton(onClick = {
                                haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                                vm.answerQuestion(q.key, o.label)
                            }, modifier = Modifier.fillMaxWidth()) {
                                Column(Modifier.fillMaxWidth()) {
                                    Text("◇ ${o.label}", color = Sea, fontSize = 15.sp)
                                    if (o.description.isNotEmpty()) {
                                        Text(o.description, color = Muted, fontSize = 12.sp)
                                    }
                                }
                            }
                        }
                    }
                    Spacer(Modifier.height(10.dp))
                    OutlinedTextField(
                        free, { free = it },
                        label = { Text(tr(lang, "app.chat.questionFree")) },
                        modifier = Modifier.fillMaxWidth(),
                    )
                    Spacer(Modifier.height(4.dp))
                    Text(tr(lang, "app.chat.questionTimeoutHelp"),
                        color = Faint, fontSize = 11.sp)
                }
            },
            confirmButton = {
                Button(onClick = {
                    haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                    vm.answerQuestion(q.key, free)
                }) { Text(tr(lang, "btn.send")) }
            },
            dismissButton = {
                OutlinedButton(onClick = { vm.answerQuestion(q.key, "") }) {
                    Text(tr(lang, "app.chat.questionLeave"))
                }
            },
        )
    }

    if (confirmNew) {
        AlertDialog(
            onDismissRequest = { confirmNew = false },
            text = { Text(tr(lang, "app.chat.newConfirm"), color = Ink) },
            confirmButton = {
                Button(onClick = { confirmNew = false; vm.newSession(); vm.go(Screen.Chat) }) {
                    Text(tr(lang, "app.chat.newConfirmOk"))
                }
            },
            dismissButton = { TextButton(onClick = { confirmNew = false }) { Text(tr(lang, "btn.close")) } },
        )
    }

    if (showPerm) {
        PermissionDialog(st, lang, onPick = { on, review -> vm.setPermissive(on, review); showPerm = false },
            onClose = { showPerm = false })
    }

    if (showRewind) {
        AlertDialog(
            onDismissRequest = { vm.closeRewind() },
            title = { Text(tr(lang, "rewind.title")) },
            text = {
                Column(Modifier.verticalScroll(rememberScrollState())) {
                    Text(tr(lang, "app.chat.rewindDescription"),
                        color = Muted, fontSize = 13.sp)
                    Spacer(Modifier.height(8.dp))
                    if (checkpoints.isEmpty()) {
                        Text(tr(lang, "app.chat.rewindEmpty"), color = Ink, fontSize = 14.sp)
                    } else {
                        checkpoints.forEach { c ->
                            TextButton(onClick = { vm.rewindTo(c.seq) }) {
                                Text("#${c.seq} ${c.op} ${c.path}", color = Sea, fontSize = 14.sp)
                            }
                        }
                    }
                }
            },
            confirmButton = {
                if (checkpoints.isNotEmpty()) {
                    Button(onClick = { vm.rewindAll() }) { Text(tr(lang, "rewind.all")) }
                }
            },
            dismissButton = {
                TextButton(onClick = { vm.closeRewind() }) { Text(tr(lang, "btn.close")) }
            },
        )
    }

    if (showGithub) {
        val actions = if (ghTool == "gh_issue") listOf(
            "list" to tr(lang, "app.chat.githubList"), "view" to tr(lang, "app.chat.githubView"))
        else listOf("list" to tr(lang, "app.chat.githubList"), "view" to tr(lang, "app.chat.githubView"),
            "diff" to tr(lang, "app.chat.githubDiff"), "checks" to tr(lang, "app.chat.githubChecks"))
        LaunchedEffect(ghTool) { if (actions.none { it.first == ghAction }) ghAction = "list" }
        val gh by vm.github.collectAsState()
        AlertDialog(
            onDismissRequest = { vm.tancaGithub() },
            title = { Text(tr(lang, "app.chat.githubTitle")) },
            text = {
                Column(Modifier.verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(7.dp)) {
                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        TextButton(onClick = { ghTool = "gh_issue" }) { Text(if (ghTool == "gh_issue") "● Issue" else "Issue", color = Sea) }
                        TextButton(onClick = { ghTool = "gh_pr" }) { Text(if (ghTool == "gh_pr") "● PR" else "PR", color = Sea) }
                    }
                    // FlowRow: amb quatre accions (PR) la quarta no hi cabia i
                    // quedava una píndola buida i aixafada a la dreta.
                    @OptIn(androidx.compose.foundation.layout.ExperimentalLayoutApi::class)
                    androidx.compose.foundation.layout.FlowRow(
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                        verticalArrangement = Arrangement.spacedBy(2.dp),
                    ) {
                        actions.forEach { (value, label) ->
                            OutlinedButton(onClick = { ghAction = value }) { Text(if (ghAction == value) "✓ $label" else label, maxLines = 1) }
                        }
                    }
                    OutlinedTextField(ghNumber, { ghNumber = it }, label = { Text(tr(lang, "app.chat.githubNumber")) }, singleLine = true)
                    OutlinedTextField(ghQuery, { ghQuery = it }, label = { Text(tr(lang, "app.chat.githubSearch")) }, singleLine = true)
                    OutlinedTextField(ghRepo, { ghRepo = it }, label = { Text(tr(lang, "app.chat.githubRepo")) }, singleLine = true)
                    if (gh.error.isNotBlank()) Text("⚠️ ${gh.error}", color = Red, fontSize = 12.sp)
                    if (gh.output.isNotBlank()) Text(gh.output.take(5000), color = Ink, fontSize = 11.sp,
                        fontFamily = androidx.compose.ui.text.font.FontFamily.Monospace)
                }
            },
            confirmButton = {
                Button(onClick = { vm.github(ghTool, ghAction, ghNumber.toIntOrNull() ?: 0, ghQuery, ghRepo) }) {
                    Text(tr(lang, "app.chat.githubRun"))
                }
            },
            dismissButton = { TextButton(onClick = { vm.tancaGithub() }) { Text(tr(lang, "btn.close")) } },
        )
    }

}

/** Avís de xarxa comprensible: sense stack traces ni IPs gegants al xat. */
@Composable
fun ConnectionNotice(text: String, lang: String, onRetry: () -> Unit) {
    Surface(shape = RoundedCornerShape(16.dp), color = Red.copy(alpha = 0.14f), modifier = Modifier.fillMaxWidth()) {
        Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
            Text("!", color = Red, fontWeight = FontWeight.Bold, fontSize = 18.sp)
            Spacer(Modifier.width(8.dp))
            Text(text, color = Ink, modifier = Modifier.weight(1f), fontSize = 14.sp)
            TextButton(onClick = onRetry) { Text(tr(lang, "app.chat.retry"), color = Sea) }
        }
    }
}

@Composable
fun AgentBusyStrip(lang: String) {
    Surface(color = CodeDark, shape = RoundedCornerShape(14.dp),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 18.dp, vertical = 2.dp)) {
        Text(tr(lang, "app.chat.busyNotice"),
            color = Sea, fontSize = 13.sp, textAlign = TextAlign.Center,
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 7.dp))
    }
}

@Composable
fun ModeBadge(mode: String, modifier: Modifier = Modifier, lang: String = "ca") {
    val label = tr(lang, when (mode) {
        "chat" -> "app.modeBadgeChat"
        "inspect" -> "app.modeBadgeInspect"
        "goal" -> "app.modeBadgeGoal"
        else -> "app.modeBadgeCode"
    })
    Surface(shape = RoundedCornerShape(10.dp), color = CodeDark, modifier = modifier) {
        Text(label, color = Sea, fontSize = 10.sp, fontWeight = FontWeight.Bold,
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 6.dp))
    }
}

private fun composerHint(mode: String, lang: String): String = when (mode) {
    "chat" -> tr(lang, "composer.ph.chat")
    "inspect" -> tr(lang, "app.chat.composerInspect")
    "goal" -> tr(lang, "app.chat.composerGoal")
    else -> tr(lang, "app.chat.composerCode")
}


/** Files del xat: divisors de dia i missatges (índex estable per animar). */
sealed interface UiRow {
    data class Day(val label: String) : UiRow
    data class Msg(val msg: ChatMsg, val index: Int) : UiRow
}

/** Agrupa missatges amb divisors de dia (pur, testeable). */
fun chatRows(msgs: List<ChatMsg>, lang: String = "ca"): List<UiRow> {
    val out = mutableListOf<UiRow>()
    msgs.forEachIndexed { i, m ->
        val k = dayKey(m.at)
        // Els missatges recuperats del servidor no porten hora (-1): no
        // posem cap divisor entre ells i el primer missatge amb hora.
        val prev = if (i > 0) dayKey(msgs[i - 1].at) else -2
        if (k >= 0 && (i == 0 || (prev >= 0 && prev != k))) {
            val label = dayLabel(m.at, lang = lang)
            if (label.isNotEmpty()) out += UiRow.Day(label)
        }
        out += UiRow.Msg(m, i)
    }
    return out
}

/** Divisor de dia centrat amb filets. */
@Composable
fun DayDivider(label: String) {
    Row(Modifier.fillMaxWidth().padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically) {
        HorizontalDivider(Modifier.weight(1f), color = Muted.copy(alpha = 0.35f))
        Surface(color = CodeDark, shape = RoundedCornerShape(10.dp),
            modifier = Modifier.padding(horizontal = 8.dp)) {
            Text(label, color = Muted, fontSize = 11.sp,
                modifier = Modifier.padding(horizontal = 10.dp, vertical = 3.dp))
        }
        HorizontalDivider(Modifier.weight(1f), color = Muted.copy(alpha = 0.35f))
    }
}

/** Punt viu que respira per a l'agent actiu. */
@Composable
fun PulseDot() {
    val t = rememberInfiniteTransition(label = "pols")
    val a by t.animateFloat(
        initialValue = 1f, targetValue = 0.25f,
        animationSpec = infiniteRepeatable(tween(900)), label = "alfa",
    )
    Box(Modifier.size(10.dp).background(Sea.copy(alpha = a), CircleShape))
}

@Composable
fun AssistantResponse(text: String, lang: String = "ca") {
    val blocks = remember(text) { groupRich(splitMd(text)) }
    Column(Modifier.fillMaxWidth().padding(horizontal = 6.dp, vertical = 6.dp)) {
        blocks.forEach { b ->
            when (b) {
                is Rich.Inline -> BodyText(renderSegs(b.segs, CodeDark, link = Sea))
                is Rich.Code -> CodeBlock(b.lang, b.code, lang)
                is Rich.Table -> MdTable(b.headers, b.rows, lang)
            }
        }
    }
}

/** Cos amb interlineat generós i enllaços clicables. */
@Composable
fun BodyText(t: AnnotatedString) {
    val uri = LocalUriHandler.current
    ClickableText(
        t,
        style = MaterialTheme.typography.bodyLarge.copy(
            fontSize = 16.sp, lineHeight = 25.sp, color = Ink,
        ),
        onClick = { off ->
            t.getStringAnnotations("URL", off, off).firstOrNull()?.let { uri.openUri(it.item) }
        },
    )
}

/** Bloc de codi amb capçalera de llenguatge i còpia. */
@Composable
fun CodeBlock(lang: String, code: String, uiLang: String = "ca") {
    val clipboard = LocalClipboardManager.current
    Card(
        modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp),
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(containerColor = CodeDark),
    ) {
        Column {
            Row(
                Modifier.fillMaxWidth().padding(start = 12.dp, end = 4.dp, top = 2.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    lang.ifEmpty { tr(uiLang, "app.code.language") }, color = Muted, fontSize = 12.sp,
                    fontWeight = FontWeight.Medium, modifier = Modifier.weight(1f),
                )
                TextButton(onClick = { clipboard.setText(AnnotatedString(code)) }) {
                    Icon(Icons.Rounded.ContentCopy, tr(uiLang, "app.code.copy"), tint = Muted, modifier = Modifier.size(14.dp))
                    Spacer(Modifier.width(4.dp))
                    Text(tr(uiLang, "app.code.copy"), color = Muted, fontSize = 12.sp)
                }
            }
            Text(
                code.trimEnd(), color = Ink, fontFamily = androidx.compose.ui.text.font.FontFamily.Monospace,
                fontSize = 13.sp, modifier = Modifier.padding(start = 12.dp, end = 12.dp, bottom = 12.dp),
            )
        }
    }
}

/** Taula markdown simple: capçalera sorra i files amb divisor. */
@Composable
fun MdTable(headers: List<String>, rows: List<List<String>>, lang: String = "ca") {
    Card(
        modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp),
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(containerColor = CodeDark),
    ) {
        Column(Modifier.padding(horizontal = 10.dp, vertical = 8.dp)) {
            TableRow(headers, header = true)
            HorizontalDivider(color = Muted.copy(alpha = 0.4f), modifier = Modifier.padding(vertical = 4.dp))
            rows.take(12).forEach { TableRow(it, header = false) }
            if (rows.size > 12) {
                Text(tr(lang, "app.table.moreFiles", rows.size - 12), color = Muted, fontSize = 12.sp,
                    modifier = Modifier.padding(4.dp))
            }
        }
    }
}

@Composable
private fun TableRow(cells: List<String>, header: Boolean) {
    Row(Modifier.fillMaxWidth()) {
        cells.forEach { c ->
            Text(
                renderSegs(parseInline(c), CodeDark, link = Sea),
                modifier = Modifier.weight(1f).padding(4.dp),
                fontWeight = if (header) FontWeight.Bold else FontWeight.Normal,
                color = if (header) Sand else Ink,
                fontSize = 13.sp,
            )
        }
    }
}

/**
 * Col·lapsa traces internes (eines, resultats, verificació i reasoning).
 * La resposta normal no es barreja amb aquests passos i continua visible.
 */
@Composable
fun ActivityCluster(steps: List<ChatMsg>, lang: String = "ca") {
    var expanded by remember(steps.firstOrNull()?.text, steps.size) { mutableStateOf(false) }
    val calls = steps.count { it.text.startsWith("🔧") }
    val blocked = steps.count { it.text.startsWith("⛔️") }
    val title = when {
        blocked == 1 -> tr(lang, "app.activity.blockedOne")
        blocked > 1 -> tr(lang, "app.activity.blocked", blocked)
        calls == 1 -> tr(lang, "app.activity.toolsOne")
        calls > 1 -> tr(lang, "app.activity.tools", calls)
        steps.size == 1 -> tr(lang, "app.activity.stepsOne")
        else -> tr(lang, "app.activity.steps", steps.size)
    }
    Card(
        onClick = { expanded = !expanded },
        modifier = Modifier.fillMaxWidth().padding(horizontal = 4.dp, vertical = 5.dp),
        colors = CardDefaults.cardColors(containerColor = CodeDark),
    ) {
        Column(Modifier.padding(horizontal = 12.dp, vertical = 10.dp)) {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Text(if (expanded) "⌃" else "⌄", color = Sea, fontSize = 16.sp)
                Spacer(Modifier.width(8.dp))
                Text(title, color = Muted, fontSize = 13.sp, fontWeight = FontWeight.Medium,
                    modifier = Modifier.weight(1f))
                Text(tr(lang, if (expanded) "app.activity.hide" else "app.activity.show"), color = Sea, fontSize = 12.sp)
            }
            if (expanded) {
                Spacer(Modifier.height(8.dp))
                steps.forEach { step ->
                    Text(step.text.take(900), color = Ink, fontSize = 13.sp,
                        modifier = Modifier.padding(start = 24.dp, bottom = 7.dp))
                }
            }
        }
    }
}

/** Estat buit amb marca marina (sessions, objectius). */
@Composable
fun EmptyBoard(icon: androidx.compose.ui.graphics.vector.ImageVector, title: String, subtitle: String) {
    Column(Modifier.fillMaxWidth().padding(vertical = 56.dp, horizontal = 32.dp),
        horizontalAlignment = Alignment.CenterHorizontally) {
        TintedIcon(icon, Sea)
        Spacer(Modifier.height(16.dp))
        Text(title, color = Ink, style = MaterialTheme.typography.titleMedium, textAlign = TextAlign.Center)
        if (subtitle.isNotBlank()) {
            Text(subtitle, color = Muted, fontSize = 13.5.sp, lineHeight = 19.sp, textAlign = TextAlign.Center,
                modifier = Modifier.padding(top = 6.dp))
        }
    }
}

/** Píndola del projecte (carpeta · branca): que sempre quedi clar on treballa. */
@Composable
fun ProjectPill(project: String, branch: String) {
    Surface(shape = RoundedCornerShape(50), color = CodeDark) {
        Row(Modifier.padding(horizontal = 12.dp, vertical = 7.dp), verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Rounded.FolderOpen, null, tint = Sand, modifier = Modifier.size(16.dp))
            Spacer(Modifier.width(6.dp))
            Text(project, color = Ink, fontSize = 13.sp, fontWeight = FontWeight.Medium, maxLines = 1)
            if (branch.isNotBlank()) {
                Spacer(Modifier.width(8.dp))
                Icon(Icons.Rounded.ForkRight, null, tint = Muted, modifier = Modifier.size(14.dp))
                Spacer(Modifier.width(2.dp))
                Text(branch, color = Muted, fontSize = 12.5.sp, maxLines = 1)
            }
        }
    }
}

@Composable
fun EmptyBoardText(mark: String, title: String, subtitle: String) {
    Column(Modifier.fillMaxWidth().padding(vertical = 48.dp, horizontal = 32.dp),
        horizontalAlignment = Alignment.CenterHorizontally) {
        Text(mark, fontSize = 44.sp, color = Sea.copy(alpha = 0.7f))
        Spacer(Modifier.height(8.dp))
        Text(title, color = Ink, fontSize = 17.sp, fontWeight = FontWeight.SemiBold)
        Text(subtitle, color = Muted, fontSize = 13.sp, textAlign = TextAlign.Center,
            modifier = Modifier.padding(top = 4.dp))
    }
}

/**
 * Bombolla d'usuari a la dreta; l'assistent en text lliure, sense insígnia ni
 * hora (estil ChatGPT). Per copiar qualsevol resposta, es manté premut.
 */
@Composable
fun MsgRow(m: ChatMsg, stable: Int, lang: String = "ca", onCopyNotice: () -> Unit = {}) {
    val clipboard = LocalClipboardManager.current
    when {
        m.role == "user" -> Row(Modifier.fillMaxWidth().padding(vertical = 4.dp),
            horizontalArrangement = Arrangement.End) {
            // Bombolla de l'usuari com a la web: superfície `night`, vora
            // d'escuma fina i radi asimètric (10/10/3/10). Abans era una
            // bombolla verda de 20 dp que no era de cap tema.
            Surface(
                shape = RoundedCornerShape(18.dp),
                color = Superficie,
            ) {
                Column(Modifier.padding(horizontal = 12.dp, vertical = 9.dp)) {
                    Text(m.text, color = Ink, style = MaterialTheme.typography.bodyLarge.copy(
                        fontSize = 15.sp, lineHeight = 22.sp))
                }
            }
        }
        m.kind == "activity" -> Text(
            m.text, Modifier.fillMaxWidth().padding(horizontal = 6.dp, vertical = 3.dp),
            color = Muted, fontSize = 14.sp, fontStyle = FontStyle.Italic,
        )
        m.kind == "error" -> Text(
            m.text, Modifier.fillMaxWidth().padding(horizontal = 6.dp, vertical = 3.dp),
            color = if (LocalPal.current == DarkPal) Red else LightPal.red, fontSize = 15.sp,
        )
        // L'animació corre un sol cop per missatge nou (clau = posició):
        // el streaming actualitza el text al mateix índex sense rellançar-la.
        else -> {
            val vis = remember(stable) {
                MutableTransitionState(false).apply { targetState = true }
            }
            Column(Modifier.fillMaxWidth()) {
                AnimatedVisibility(
                    visibleState = vis,
                    enter = fadeIn() + slideInVertically { it / 4 },
                ) {
                    Row(Modifier.fillMaxWidth()) {
                        // Sense insígnia de l'agent: en un xat 1:1 el text ja
                        // se sap de qui és (com ChatGPT). El que hi havia era
                        // un requadre ≋ a cada resposta.
                        Box(
                            Modifier.fillMaxWidth().pointerInput(m.text, m.at) {
                                detectTapGestures(onLongPress = {
                                    clipboard.setText(AnnotatedString(m.text))
                                    onCopyNotice()
                                })
                            }
                        ) { AssistantResponse(m.text, lang) }
                    }
                }
            }
        }
    }
}

/** Desplegable de model/rol/mode sota el títol. */
@Composable
fun ModelMenu(
    vm: GregalViewModel,
    st: ServerState,
    expanded: Boolean,
    onClose: () -> Unit,
    onPlan: (() -> Unit)? = null,
    onParallel: (() -> Unit)? = null,
    potEines: Boolean = false,
) {
    val lang by vm.idioma.collectAsState()
    val models by vm.models.collectAsState()
    val providers by vm.providers.collectAsState()
    LaunchedEffect(expanded) { if (expanded) { vm.loadProviders(); vm.loadModels() } }
    DropdownMenu(expanded = expanded, onDismissRequest = onClose) {
        ModelPickerContent(lang, providers, models, st,
            onSelect = { vm.setModel(it); onClose() },
            onSettings = { onClose(); vm.go(Screen.Prefs) })
        HorizontalDivider(color = Line)
        st.roles.forEach { role ->
            DropdownMenuItem(text = { Text(roleLabel(role, lang)) },
                trailingIcon = { if (st.currentRole == role) Icon(Icons.Rounded.Check, null, tint = Sea) },
                onClick = { vm.setRole(role); onClose() })
        }
        if (onPlan != null) DropdownMenuItem(
            text = { Text(tr(lang, "app.menu.plan")) }, enabled = potEines,
            onClick = { onPlan(); onClose() })
        if (onParallel != null) DropdownMenuItem(
            text = { Text(tr(lang, "app.menu.compare")) }, enabled = potEines,
            onClick = { onParallel(); onClose() })
        DropdownMenuItem(text = { Text(tr(lang, "app.menu.refresh"), color = Sea) },
            onClick = { vm.refresh(); vm.loadProviders(); vm.loadModels() })
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SessionsScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val sessions by vm.sessions.collectAsState()
    val active by vm.active.collectAsState()
    var filtre by remember { mutableStateOf("") }
    var confirma by remember { mutableStateOf<String?>(null) }
    val loading by vm.loading.collectAsState()
    val carregant = "sessions" in loading
    LaunchedEffect(Unit) { vm.loadSessions() }
    // L'agent actiu es refresca cada 3 s, però només amb la pantalla visible
    // (RESUMED): en segon pla abans seguia trucant el servidor sense fi.
    val lifecycleOwner = LocalLifecycleOwner.current
    LaunchedEffect(lifecycleOwner) {
        lifecycleOwner.repeatOnLifecycle(Lifecycle.State.RESUMED) {
            while (true) {
                delay(3000)
                vm.loadActive()
            }
        }
    }
    // Cerca per títol o nom + seccions per data (com convs.js de la web).
    val llista = remember(sessions, filtre) {
        val f = filtre.trim().lowercase()
        sessions.filter { f.isEmpty() || it.displayTitle().lowercase().contains(f) || it.name.lowercase().contains(f) }
    }
    val grups = remember(llista) {
        val ordre = listOf("today", "yesterday", "week", "month", "older")
        llista.groupBy { convGroup(it.at.ifBlank { it.moment }) }
            .toSortedMap(compareBy { ordre.indexOf(it).let { i -> if (i < 0) 9 else i } })
    }
    GregalBg {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = { TopAppBar(navigationIcon = { IconButton(onClick = onMenu) { Icon(Icons.Rounded.Menu, tr(lang, "app.nav.menu"), tint = Ink) } }, title = { Text(tr(lang, "nav.convs"), color = Ink) }, actions = {
                RefreshAction(lang, carregant) { vm.loadSessions() }
                IconButton(onClick = { vm.newSession(); vm.go(Screen.Chat) }) {
                    Icon(Icons.Rounded.Add, tr(lang, "nav.new"), tint = Ink)
                }
            }, colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent)) },
            snackbarHost = { NoticeHost(vm) },
        ) { pad ->
            LazyColumn(Modifier.fillMaxSize().padding(pad).pullToRefresh(carregant) { vm.loadSessions() }.padding(12.dp)) {
                active?.let { run ->
                    item { ActiveAgentCard(run, lang) }
                    item { Spacer(Modifier.height(12.dp)) }
                }
                if (sessions.isNotEmpty()) item {
                    OutlinedTextField(filtre, { filtre = it }, Modifier.fillMaxWidth(),
                        placeholder = { Text(tr(lang, "app.sess.search")) }, singleLine = true,
                        leadingIcon = { Icon(Icons.Rounded.Search, null, tint = Muted) },
                        shape = RoundedCornerShape(50))
                    Spacer(Modifier.height(4.dp))
                }
                if (sessions.isEmpty() && active == null && carregant) item { LoadingBoard(lang) }
                else if (llista.isEmpty() && active == null) item {
                    EmptyBoard(Icons.Rounded.Forum, tr(lang, "app.sess.empty"), "")
                }
                grups.forEach { (g, items) ->
                    item {
                        Text(convGroupLabel(g, lang), color = Muted, fontSize = 13.sp,
                            modifier = Modifier.padding(horizontal = 4.dp, vertical = 6.dp))
                    }
                    items(items, key = { it.name }) { s ->
                        Card(
                            onClick = { vm.resumeSession(s.name) },
                            modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp),
                            colors = CardDefaults.cardColors(containerColor = PillDark),
                        ) {
                            Row(Modifier.padding(14.dp), verticalAlignment = Alignment.CenterVertically) {
                                Surface(shape = RoundedCornerShape(12.dp), color = CodeDark,
                                    modifier = Modifier.size(40.dp)) {
                                    Box(contentAlignment = Alignment.Center) {
                                        Icon(Icons.Rounded.ChatBubbleOutline, null, tint = Sea,
                                            modifier = Modifier.size(20.dp))
                                    }
                                }
                                Spacer(Modifier.width(12.dp))
                                Column(Modifier.weight(1f)) {
                                    Text(s.displayTitle(), fontWeight = FontWeight.SemiBold, fontSize = 16.sp, color = Ink)
                                    val count = if (s.msgs == 1) tr(lang, "app.sess.oneMessage")
                                        else tr(lang, "app.sess.messages", s.msgs)
                                    Text("$count · ${ago(s.moment, lang = lang)}" +
                                        if (s.current) " · ${tr(lang, "app.sess.current")}" else "",
                                        style = MaterialTheme.typography.bodySmall, color = Muted)
                                }
                                IconButton(onClick = { confirma = s.name }) {
                                    Icon(Icons.Rounded.Delete, tr(lang, "app.sess.delete"), tint = Muted)
                                }
                            }
                        }
                    }
                }
            }
            confirma?.let { nom ->
                AlertDialog(
                    onDismissRequest = { confirma = null },
                    title = { Text(tr(lang, "app.sess.delete"), color = Ink) },
                    text = { Text(nom, color = Muted, fontSize = 14.sp) },
                    confirmButton = {
                        TextButton(onClick = { vm.deleteSession(nom); confirma = null }) {
                            Text(tr(lang, "app.sess.delete"), color = Sea)
                        }
                    },
                    dismissButton = {
                        TextButton(onClick = { confirma = null }) { Text(tr(lang, "btn.close")) }
                    },
                )
            }
        }
    }
}

/** La feina que ocupa el worker: observable però no resumible mentre corre. */
@Composable
fun ActiveAgentCard(run: ActiveAgent, lang: String = "ca") {
    var expanded by remember(run.id) { mutableStateOf(false) }
    Card(
        onClick = { expanded = !expanded },
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = BubbleDark),
    ) {
        Column(Modifier.padding(14.dp)) {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                PulseDot()
                Spacer(Modifier.width(8.dp))
                Text(tr(lang, "app.sess.activeTitle"), color = Ink, fontWeight = FontWeight.Bold,
                    modifier = Modifier.weight(1f))
                Text(tr(lang, if (expanded) "app.sess.hide" else "app.sess.show"), color = Sea, fontSize = 12.sp)
            }
            val mode = tr(lang, when (run.mode) {
                "chat" -> "app.modeLabelChat"
                "inspect" -> "app.modeLabelInspect"
                "goal" -> "app.modeLabelGoal"
                else -> "app.modeLabelCode"
            })
            Text("${run.project} · ${run.role} · $mode", color = Sand, fontSize = 12.sp,
                modifier = Modifier.padding(top = 4.dp))
            Text(stripMdForTitle(run.task).take(180), color = Ink, fontSize = 14.sp,
                modifier = Modifier.padding(top = 7.dp))
            Text(tr(lang, "app.sess.readOnlyRefresh"), color = Muted, fontSize = 12.sp,
                modifier = Modifier.padding(top = 7.dp))
            if (expanded) {
                Spacer(Modifier.height(9.dp))
                if (run.events.isEmpty()) {
                    Text(tr(lang, "app.sess.noActivity"), color = Muted, fontSize = 13.sp)
                } else {
                    val codeBg = CodeDark
                    // Es repinta cada 3 s amb el sondeig: el markdown, només si canvia.
                    val rendered = remember(run.events, codeBg) {
                        run.events.map { renderMd(it.text.take(500), codeBg) }
                    }
                    run.events.forEachIndexed { i, event ->
                        Text(rendered[i], color = if (event.kind == "error") MaterialTheme.colorScheme.error else Ink,
                            fontSize = 13.sp, modifier = Modifier.padding(vertical = 3.dp))
                    }
                }
            } else if (run.events.isNotEmpty()) {
                Text(stripMdForTitle(run.events.last().text).take(110), color = Muted, fontSize = 12.sp,
                    modifier = Modifier.padding(top = 8.dp))
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun GoalsScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val goals by vm.goals.collectAsState()
    val haptic = LocalHapticFeedback.current
    val loading by vm.loading.collectAsState()
    val carregant = "goals" in loading
    LaunchedEffect(Unit) { vm.loadGoals() }
    GregalBg {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = { TopAppBar(navigationIcon = { IconButton(onClick = onMenu) { Icon(Icons.Rounded.Menu, tr(lang, "app.nav.menu"), tint = Ink) } }, title = { Text(tr(lang, "app.goals.title"), color = Ink) },
                actions = { RefreshAction(lang, carregant) { vm.loadGoals() } },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent)) },
            snackbarHost = { NoticeHost(vm) },
        ) { pad ->
            LazyColumn(Modifier.fillMaxSize().padding(pad).pullToRefresh(carregant) { vm.loadGoals() }.padding(12.dp)) {
                if (goals.isEmpty() && carregant) item { LoadingBoard(lang) }
                else if (goals.isEmpty()) item {
                    EmptyBoard(Icons.Rounded.Flag, tr(lang, "app.goals.empty"), tr(lang, "app.goals.emptyHelp"))
                }
                items(goals) { g ->
                    Card(
                        modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp),
                        colors = CardDefaults.cardColors(containerColor = PillDark),
                    ) {
                        Row(Modifier.padding(14.dp), verticalAlignment = Alignment.CenterVertically) {
                            Surface(shape = RoundedCornerShape(12.dp), color = CodeDark,
                                modifier = Modifier.size(38.dp)) {
                                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                                    Icon(Icons.Rounded.TaskAlt, null, tint = Sea,
                                        modifier = Modifier.size(20.dp))
                                }
                            }
                            Spacer(Modifier.width(12.dp))
                            Column(Modifier.weight(1f)) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Text(
                                    stripMdForTitle(g.title), fontWeight = FontWeight.SemiBold,
                                    fontSize = 16.sp, color = Ink, modifier = Modifier.weight(1f),
                                )
                                if (g.status.isNotBlank()) {
                                    Surface(shape = RoundedCornerShape(8.dp), color = CodeDark) {
                                        Text(
                                            goalStatusLabel(g.status, lang),
                                            color = Sea, fontSize = 10.sp,
                                            fontWeight = FontWeight.Bold,
                                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
                                        )
                                    }
                                }
                            }
                            Text(g.id,
                                style = MaterialTheme.typography.bodySmall, color = Muted)
                            if (g.body.isNotBlank()) {
                                Text(renderMd(g.body.take(600), CodeDark),
                                    color = Ink, fontSize = 13.sp,
                                    modifier = Modifier.padding(top = 6.dp))
                            }
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(4.dp),
                            ) {
                                FilledTonalButton(onClick = {
                                    haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                                    vm.runGoal(g.id)
                                }) {
                                    Icon(Icons.Rounded.PlayArrow, null)
                                    Spacer(Modifier.width(4.dp))
                                    Text(tr(lang, "app.goals.run"))
                                }
                                var menu by remember { mutableStateOf(false) }
                                Box {
                                    IconButton(onClick = { menu = true }) {
                                        Icon(Icons.Rounded.MoreVert, tr(lang, "app.goals.more"), tint = Muted)
                                    }
                                    DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                                        DropdownMenuItem(
                                            text = { Text(tr(lang, "btn.delete"), color = Ink) },
                                            leadingIcon = {
                                                Icon(Icons.Rounded.Delete, null, tint = Muted)
                                            },
                                            onClick = { menu = false; vm.deleteGoal(g.id) },
                                        )
                                    }
                                }
                            }
                            }
                        }
                    }
                }
            }
        }
    }
}

/** Desa bytes a Descàrregues via MediaStore (scoped storage, sense permisos). Torna missatge. */
fun saveToDownloads(ctx: android.content.Context, name: String, bytes: ByteArray, lang: String = "ca"): String {
    return try {
        val safe = if (name.isBlank()) "gregal-office" else name.substringAfterLast('/').take(120)
        val values = android.content.ContentValues().apply {
            put(android.provider.MediaStore.Downloads.DISPLAY_NAME, safe)
            put(android.provider.MediaStore.Downloads.MIME_TYPE, when {
                safe.endsWith(".docx", true) -> "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
                safe.endsWith(".xlsx", true) -> "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                safe.endsWith(".pptx", true) -> "application/vnd.openxmlformats-officedocument.presentationml.presentation"
                else -> "application/octet-stream"
            })
            put(android.provider.MediaStore.Downloads.RELATIVE_PATH, android.os.Environment.DIRECTORY_DOWNLOADS)
        }
        val uri = ctx.contentResolver.insert(android.provider.MediaStore.Downloads.EXTERNAL_CONTENT_URI, values)
            ?: return tr(lang, "app.off.downloadCreateError")
        ctx.contentResolver.openOutputStream(uri)?.use { it.write(bytes) }
        tr(lang, "app.off.downloadSaved", safe)
    } catch (e: Exception) { tr(lang, "app.off.downloadWriteError", e.message.orEmpty()) }
}

/**
 * Avis d'actualització: surt a l'arrencada quan el manifest publica una
 * versió més nova que la instal·lada (i no l'has silenciada). Es pot
 * actualitzar des d'aquí mateix — baixa, verifica sha256 i obre
 * l'instal·lador —, deixar-ho per a més tard o silenciar aquesta versió.
 */
@Composable
fun UpdateAvis(vm: GregalViewModel) {
    val lang by vm.idioma.collectAsState()
    val obert by vm.avisObert.collectAsState()
    val res by vm.update.collectAsState()
    val prog by vm.updateProg.collectAsState()
    val info = res?.takeIf { it.isAvailable }
    if (!obert || info == null) return
    AlertDialog(
        onDismissRequest = { vm.descartaAvis() },
        title = { Text("Gregal ${info.latestVersion}", color = Ink) },
        text = {
            Column {
                Text(tr(lang, "app.update.currentVersion", info.currentVersion), color = Muted, fontSize = 13.sp)
                if (info.changelog.isNotBlank()) {
                    Spacer(Modifier.height(6.dp))
                    Text(info.changelog, color = Tinta, fontSize = 13.sp)
                }
                if (prog >= 0) {
                    Spacer(Modifier.height(10.dp))
                    LinearProgressIndicator(
                        progress = { prog / 100f },
                        modifier = Modifier.fillMaxWidth(),
                        color = Sea, trackColor = Line,
                    )
                    Spacer(Modifier.height(4.dp))
                    Text(tr(lang, "app.update.downloadProgress", prog), color = Muted, fontSize = 12.sp)
                }
            }
        },
        confirmButton = {
            if (prog < 0) {
                Button(onClick = { vm.baixaIInstalla() }) { Text(tr(lang, "app.update.downloadInstall")) }
            } else {
                Text(tr(lang, "app.update.downloading", info.latestVersion), color = Muted, fontSize = 12.sp)
            }
        },
        dismissButton = {
            if (prog < 0) {
                Row {
                    TextButton(onClick = { vm.descartaAvis() }) { Text(tr(lang, "app.update.later")) }
                    TextButton(onClick = { vm.ignoraAquestaVersio() }) { Text(tr(lang, "app.update.silence")) }
                }
            }
        },
    )
}
