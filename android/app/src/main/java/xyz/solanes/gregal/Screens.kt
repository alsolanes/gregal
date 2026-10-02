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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Undo
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.ChatBubbleOutline
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.CompareArrows
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Description
import androidx.compose.material.icons.filled.ForkRight
import androidx.compose.material.icons.filled.Image
import androidx.compose.material.icons.filled.Lightbulb
import androidx.compose.material.icons.filled.Menu
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Send
import androidx.compose.material.icons.filled.TaskAlt
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
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.Mic
import androidx.compose.material.icons.filled.MoreVert
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
    GregalBg {
        Column(Modifier.fillMaxSize().padding(28.dp), verticalArrangement = Arrangement.Center) {
            Text("≋", fontSize = 48.sp, color = Sea)
            Text("Gregal", style = MaterialTheme.typography.headlineLarge,
                fontWeight = FontWeight.Bold, color = Ink)
            Text(tr(lang, "app.pair.subtitle"), color = Sand)
            Spacer(Modifier.height(20.dp))
            OutlinedTextField(host, { host = it }, label = { Text(tr(lang, "app.pair.host")) },
                modifier = Modifier.fillMaxWidth(), singleLine = true)
            Spacer(Modifier.height(8.dp))
            OutlinedTextField(port, { port = it }, label = { Text(tr(lang, "app.pair.port")) },
                modifier = Modifier.fillMaxWidth(), singleLine = true)
            Spacer(Modifier.height(8.dp))
            OutlinedTextField(usuari, { usuari = it }, label = { Text(tr(lang, "app.pair.user")) },
                modifier = Modifier.fillMaxWidth(), singleLine = true)
            Spacer(Modifier.height(8.dp))
            OutlinedTextField(contrasenya, { contrasenya = it }, label = { Text(tr(lang, "app.pair.password")) },
                modifier = Modifier.fillMaxWidth(), singleLine = true,
                visualTransformation = PasswordVisualTransformation())
            Spacer(Modifier.height(16.dp))
            Button(onClick = { vm.entra(host, port, usuari, contrasenya) },
                modifier = Modifier.fillMaxWidth()) { Text(tr(lang, "app.pair.login")) }
            TextButton(onClick = { ambToken = !ambToken }, modifier = Modifier.fillMaxWidth()) {
                Text(tr(lang, if (ambToken) "app.pair.hideToken" else "app.pair.showToken"),
                    color = Muted, fontSize = 13.sp)
            }
            if (ambToken) {
                OutlinedTextField(token, { token = it }, label = { Text(tr(lang, "app.pair.token")) },
                    modifier = Modifier.fillMaxWidth(), singleLine = true)
                Spacer(Modifier.height(8.dp))
                OutlinedButton(onClick = {
                    prefs.save(host, port, token, prefs.usuari)
                    vm.setConn(Conn(host, port, token, prefs.usuari))
                    vm.connect { vm.go(Screen.Chat) }
                }, modifier = Modifier.fillMaxWidth()) { Text(tr(lang, "app.pair.connect")) }
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
                    prefs.save(host, port, token)
                    vm.setConn(Conn(host, port, token))
                    vm.connect { vm.go(Screen.Chat) }
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
    LaunchedEffect(listState) {
        snapshotFlow {
            val info = listState.layoutInfo
            info.visibleItemsInfo.lastOrNull()?.index to info.totalItemsCount
        }.collect { (last, total) ->
            follow = total == 0 || (last != null && last >= total - 1)
        }
    }
    LaunchedEffect(msgs.size) { if (follow && msgs.isNotEmpty()) listState.animateScrollToItem(msgs.size - 1) }
    LaunchedEffect(notice) { if (notice.isNotEmpty()) { snack.showSnackbar(notice); vm.clearNotice() } }
    LaunchedEffect(showModelMenu) { if (showModelMenu) vm.loadModels() }

    Scaffold(
        containerColor = Color.Transparent,
        topBar = {
            TopAppBar(
                navigationIcon = {
                    IconButton(onClick = onMenu) { Icon(Icons.Default.Menu, tr(lang, "app.nav.menu"), tint = Ink) }
                },
                title = {
                    // Estil ChatGPT: el títol és el model, centrat, i prou.
                    // El projecte és al capdamunt del calaix.
                    Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                        TextButton(onClick = { showModelMenu = true }) {
                            Text(
                                (st.model.ifEmpty { st.currentRole.ifEmpty { "Gregal" } }) + " ▾",
                                color = Ink, fontSize = 16.sp, fontWeight = FontWeight.SemiBold,
                                maxLines = 1, overflow = TextOverflow.Ellipsis,
                            )
                        }
                        ModelMenu(
                            vm, st, showModelMenu, { showModelMenu = false },
                            onPlan = {
                                haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                                vm.requestPlan(consumeInput())
                            },
                            onParallel = {
                                haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                                vm.parallel(consumeInput())
                            },
                            potEines = !busy && input.isNotBlank(),
                        )
                    }
                },
                actions = {
                    // Una sola acció, com ChatGPT: conversa nova. Desfés i
                    // GitHub són al calaix; pla i comparació al desplegable.
                    IconButton(onClick = { vm.newSession(); vm.go(Screen.Chat) }) {
                        Icon(Icons.Default.Add, tr(lang, "nav.new"), tint = Ink, modifier = Modifier.size(24.dp))
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent),
            )
        },
        snackbarHost = { SnackbarHost(snack) },
        bottomBar = {
            Column(Modifier.imePadding()) {
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
                                        Icon(Icons.Default.PlayArrow, tr(lang, "app.chat.execute"))
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
                            TextButton(onClick = { vm.removeImage(i) }) {
                                Text("✕${i + 1}", color = Muted, fontSize = 13.sp)
                            }
                        }
                    }
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
                        .shadow(2.dp, RoundedCornerShape(24.dp)),
                    shape = RoundedCornerShape(24.dp), color = PillDark,
                    border = BorderStroke(
                        1.dp,
                        if (composerFocused) Sea.copy(alpha = 0.55f) else Color.Transparent,
                    ),
                ) {
                    Column(
                        Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 7.dp),
                    ) {
                        BasicTextField(
                            value = input,
                            onValueChange = { setInput(it) },
                            modifier = Modifier.fillMaxWidth().padding(vertical = 12.dp)
                                .onFocusChanged { composerFocused = it.isFocused },
                            textStyle = MaterialTheme.typography.bodyLarge.copy(color = Ink),
                            maxLines = 4,
                            decorationBox = { inner ->
                                if (input.isEmpty()) Text(composerHint(st.mode, lang), color = Muted,
                                    style = MaterialTheme.typography.bodyLarge)
                                inner()
                            },
                        )
                        Row(
                            Modifier.fillMaxWidth(),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            // Estil ChatGPT: adjuntar a l'esquerra, veu i
                            // enviar a la dreta. El mode ja el diu el
                            // placeholder, i el mesurador viu a fora.
                            IconButton(
                                onClick = { pickImgs.launch("image/*") },
                                enabled = !sending && pendingImgs.size < 3,
                                modifier = Modifier.size(40.dp),
                            ) {
                                Icon(Icons.Default.Add,
                                    tr(lang, "app.chat.addImage") + if (pendingImgs.isNotEmpty()) " (${pendingImgs.size}/3)" else "",
                                    tint = if (!sending && pendingImgs.size < 3) Muted else Faint,
                                    modifier = Modifier.size(22.dp))
                            }
                            Spacer(Modifier.weight(1f))
                        if (voiceOk) {
                            IconButton(
                                onClick = {
                                    try {
                                        voice.launch(
                                            Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).apply {
                                                putExtra(
                                                    RecognizerIntent.EXTRA_LANGUAGE_MODEL,
                                                    RecognizerIntent.LANGUAGE_MODEL_FREE_FORM,
                                                )
                                                putExtra(RecognizerIntent.EXTRA_LANGUAGE, if (lang == "en") "en-US" else "ca-ES")
                                                putExtra(RecognizerIntent.EXTRA_PROMPT, tr(lang, "app.chat.voicePrompt"))
                                            },
                                        )
                                    } catch (_: Exception) {
                                        vm.notify(tr(lang, "app.chat.voiceUnavailable"))
                                    }
                                },
                                modifier = Modifier.size(44.dp),
                            ) {
                                Icon(Icons.Default.Mic, tr(lang, "app.chat.dictate"), tint = Muted,
                                    modifier = Modifier.size(22.dp))
                            }
                        }
                        IconButton(
                            onClick = {
                                haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                                follow = true; vm.send(consumeInput())
                            },
                            // Si només hi ha un agent extern ocupat, encara deixem
                            // enviar: ViewModel posa el text a la cua i no es perd.
                            enabled = !sending && input.isNotBlank(),
                            modifier = Modifier.size(44.dp),
                        ) {
                            Surface(color = if (!sending && input.isNotBlank()) Sea else CodeDark,
                                shape = CircleShape, modifier = Modifier.size(34.dp)) {
                                Box(contentAlignment = Alignment.Center) {
                                    Icon(Icons.Default.Send, tr(lang, "app.chat.send"), tint = if (!sending && input.isNotBlank()) Abyss else Muted,
                                        modifier = Modifier.size(17.dp))
                                }
                            }
                        }
                        } // fi fila d'accions
                    } // fi columna del composer
                }
            }
        },
    ) { pad ->
        if (msgs.isEmpty()) {
            // Estat buit: la salutació i prou. El model es veu a la
            // capçalera, i els xips de suggeriments competien amb el text.
            Column(
                Modifier.fillMaxSize().padding(pad),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text("≋", fontSize = 44.sp, color = Sea)
                Spacer(Modifier.height(10.dp))
                Text(tr(lang, "app.chat.empty"), fontSize = 26.sp, color = Ink,
                    textAlign = TextAlign.Center, modifier = Modifier.padding(horizontal = 24.dp))
            }
        } else {
            Box(Modifier.fillMaxSize().padding(pad)) {
                val rows = remember(msgs, lang) { chatRows(msgs, lang) }
                LazyColumn(
                    Modifier.fillMaxSize().padding(horizontal = 12.dp),
                    state = listState,
                ) {
                    items(rows.size) { ri ->
                        when (val r = rows[ri]) {
                            is UiRow.Day -> DayDivider(r.label)
                            is UiRow.Msg -> {
                                val m = r.msg
                                val index = r.index
                                if (m.kind == "activity") {
                                    // Una sola targeta per tram consecutiu: no enterra la resposta
                                    // final amb crides d'eines, verificacions o reasoning.
                                    if (index == 0 || msgs[index - 1].kind != "activity") {
                                        ActivityCluster(msgs.drop(index).takeWhile { it.kind == "activity" }, lang)
                                    }
                                } else {
                                    MsgRow(m, index, lang) { vm.notify(tr(lang, "app.chat.copied")) }
                                }
                            }
                        }
                    }
                    if (busy) item {
                        Column(Modifier.fillMaxWidth().padding(horizontal = 10.dp, vertical = 6.dp)) {
                            WorkingStrip(turnT0, lang)
                            if (statusLine.isNotEmpty()) {
                                Spacer(Modifier.height(3.dp))
                                Text(statusLine, color = Sand, fontSize = 12.sp, maxLines = 2)
                            }
                        }
                    }
                }
                if (!follow && msgs.isNotEmpty()) {
                    FilledTonalButton(
                        onClick = {
                            follow = true
                            scope.launch { listState.animateScrollToItem(msgs.size - 1) }
                        },
                        modifier = Modifier.align(Alignment.BottomCenter).padding(12.dp),
                    ) {
                        Icon(Icons.Default.KeyboardArrowDown, tr(lang, "app.chat.scrollBottom"))
                        Spacer(Modifier.width(4.dp))
                        Text(tr(lang, "btn.toBottom"))
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
            confirmButton = {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    TextButton(onClick = {
                        haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                        vm.answerApprove(true, remember = true)
                    }) { Text(tr(lang, "nav.new"), color = Sea) }
                    Spacer(Modifier.width(4.dp))
                    Button(onClick = {
                        haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                        vm.answerApprove(true)
                    }) { Text(tr(lang, "btn.allow")) }
                }
            },
            dismissButton = {
                OutlinedButton(onClick = {
                    haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                    vm.answerApprove(false)
                }) { Text(tr(lang, "btn.deny")) }
            },
        )
    }

    // Pregunta de l'agent (`question_request`): la web i el TUI l'esperen i
    // el mòbil ni la veia — el torn es quedava 120 s aturat sense dir res.
    question?.let { q ->
        var free by remember(q.key) { mutableStateOf("") }
        AlertDialog(
            onDismissRequest = { vm.answerQuestion(q.key, "") },
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
                    Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                        actions.forEach { (value, label) ->
                            OutlinedButton(onClick = { ghAction = value }) { Text(if (ghAction == value) "✓ $label" else label) }
                        }
                    }
                    OutlinedTextField(ghNumber, { ghNumber = it }, label = { Text(tr(lang, "app.chat.githubNumber")) }, singleLine = true)
                    OutlinedTextField(ghQuery, { ghQuery = it }, label = { Text(tr(lang, "app.chat.githubSearch")) }, singleLine = true)
                    OutlinedTextField(ghRepo, { ghRepo = it }, label = { Text(tr(lang, "app.chat.githubRepo")) }, singleLine = true)
                    if (gh.error.isNotBlank()) Text("⚠️ ${gh.error}", color = Color(0xFFFFB4AB), fontSize = 12.sp)
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
    Surface(shape = RoundedCornerShape(16.dp), color = Color(0xFF42252A), modifier = Modifier.fillMaxWidth()) {
        Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
            Text("!", color = Color(0xFFFFB4AB), fontWeight = FontWeight.Bold, fontSize = 18.sp)
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
        if (k >= 0 && (i == 0 || dayKey(msgs[i - 1].at) != k)) {
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
                    Icon(Icons.Default.ContentCopy, tr(uiLang, "app.code.copy"), tint = Muted, modifier = Modifier.size(14.dp))
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
fun EmptyBoard(mark: String, title: String, subtitle: String) {
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
    val provMsg by vm.provMsg.collectAsState()
    var keyFor by remember { mutableStateOf<String?>(null) }
    var keyVal by remember { mutableStateOf("") }
    LaunchedEffect(expanded) { if (expanded) vm.loadProviders() }
    DropdownMenu(expanded = expanded, onDismissRequest = onClose) {
        // Les dues eines que abans eren icones sempre visibles al composer:
        // aquí ja hi són quan les busques, i no competeixen amb l'enviar.
        if (onPlan != null) {
            DropdownMenuItem(
                text = { Text(tr(lang, "app.menu.plan"), color = if (potEines) Ink else Muted) },
                enabled = potEines,
                onClick = { onPlan(); onClose() },
            )
        }
        if (onParallel != null) {
            DropdownMenuItem(
                text = { Text(tr(lang, "app.menu.compare"), color = if (potEines) Ink else Muted) },
                enabled = potEines,
                onClick = { onParallel(); onClose() },
            )
        }
        HorizontalDivider(color = Line)
        Text(tr(lang, "app.menu.provider"), Modifier.padding(horizontal = 12.dp, vertical = 4.dp),
            fontSize = 12.sp, color = Muted, fontWeight = FontWeight.Bold)
        if (providers.isEmpty()) {
            DropdownMenuItem(text = { Text(tr(lang, "app.menu.loading"), color = Muted) }, onClick = { vm.loadProviders() })
        }
        providers.forEach { p ->
            DropdownMenuItem(
                text = {
                    Row(verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        // Punt: verd = clau posada, buit = falta clau.
                        val dot = if (p.hasKey()) "●" else "○"
                        Text(dot, color = if (p.hasKey()) Sea else Muted, fontSize = 12.sp)
                        Column {
                            Text(p.name, fontSize = 14.sp)
                            if (!p.hasKey()) Text(tr(lang, "app.menu.providerMissingKey"),
                                color = Sea, fontSize = 11.sp)
                            else if (p.usedBy.isNotBlank()) Text(p.usedBy.take(40),
                                color = Muted, fontSize = 11.sp)
                        }
                    }
                },
                onClick = { keyVal = ""; keyFor = p.name },
            )
        }
        if (provMsg.isNotBlank()) {
            Text(provMsg.take(80), Modifier.padding(horizontal = 12.dp, vertical = 2.dp),
                fontSize = 11.sp, color = Sea)
        }
        Text(tr(lang, "app.menu.mode"), Modifier.padding(horizontal = 12.dp, vertical = 4.dp),
            fontSize = 12.sp, color = Muted, fontWeight = FontWeight.Bold)
        listOf(
            "code" to tr(lang, "app.chat.modeCode"),
            "inspect" to tr(lang, "app.chat.modeInspect"),
            "chat" to tr(lang, "app.chat.modeChat"),
            "goal" to tr(lang, "app.chat.modeGoal"),
        ).forEach { (v, l) ->
            DropdownMenuItem(
                text = { Text(l) },
                trailingIcon = { if (st.mode == v) Icon(Icons.Default.Check, null, tint = Sea) },
                onClick = { vm.setMode(v); onClose() },
            )
        }
        Text(tr(lang, "app.menu.role"), Modifier.padding(horizontal = 12.dp, vertical = 4.dp),
            fontSize = 12.sp, color = Muted, fontWeight = FontWeight.Bold)
        st.roles.forEach { r ->
            DropdownMenuItem(
                text = { Text(roleLabel(r, lang)) },
                trailingIcon = { if (st.currentRole == r) Icon(Icons.Default.Check, null, tint = Sea) },
                onClick = { vm.setRole(r); onClose() },
            )
        }
        Text(tr(lang, "app.menu.model", st.provider), Modifier.padding(horizontal = 12.dp, vertical = 4.dp),
            fontSize = 12.sp, color = Muted, fontWeight = FontWeight.Bold)
        val ids = models[st.provider] ?: models.values.flatten().distinct().take(15)
        if (ids.isEmpty()) {
            DropdownMenuItem(text = { Text(tr(lang, "app.menu.loading"), color = Muted) }, onClick = {})
        }
        ids.take(15).forEach { id ->
            DropdownMenuItem(
                text = { Text(id.take(34), fontSize = 14.sp) },
                trailingIcon = { if (id == st.model) Icon(Icons.Default.Check, null, tint = Sea) },
                onClick = { vm.setModel("${st.provider}/$id"); onClose() },
            )
        }
        DropdownMenuItem(
            text = { Text(tr(lang, "app.menu.refresh"), color = Sea) },
            onClick = { vm.refresh(); vm.loadModels() },
        )
    }
    // Diàleg de clau api: surt en tocar un proveïdor.
    val kf = keyFor
    if (kf != null) {
        AlertDialog(
            onDismissRequest = { keyFor = null },
            title = { Text(tr(lang, "app.menu.keyTitle", kf)) },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(7.dp)) {
                    Text(tr(lang, "app.menu.keyHelp"),
                        color = Muted, fontSize = 13.sp)
                    OutlinedTextField(keyVal, { keyVal = it }, label = { Text(tr(lang, "app.menu.keyPlaceholder")) },
                        singleLine = true,
                        visualTransformation = androidx.compose.ui.text.input.PasswordVisualTransformation(),
                        keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(
                            keyboardType = androidx.compose.ui.text.input.KeyboardType.Password))
                    Text(tr(lang, "app.menu.keyMemory"),
                        color = Muted, fontSize = 11.sp)
                    if (provMsg.isNotBlank()) Text(provMsg.take(120), color = Sea, fontSize = 12.sp)
                }
            },
            confirmButton = {
                TextButton(onClick = { vm.setProviderKey(kf, keyVal) }) { Text(tr(lang, "btn.save")) }
            },
            dismissButton = {
                Row {
                    TextButton(onClick = { vm.testProvider(kf) }) { Text(tr(lang, "app.menu.test")) }
                    TextButton(onClick = { keyFor = null }) { Text(tr(lang, "btn.close")) }
                }
            },
        )
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
    LaunchedEffect(Unit) {
        vm.loadSessions()
        while (true) {
            delay(3000)
            vm.loadActive()
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
            topBar = { TopAppBar(navigationIcon = { IconButton(onClick = onMenu) { Icon(Icons.Default.Menu, tr(lang, "app.nav.menu"), tint = Ink) } }, title = { Text(tr(lang, "nav.convs"), color = Ink) }, actions = {
                IconButton(onClick = { vm.newSession(); vm.go(Screen.Chat) }) {
                    Icon(Icons.Default.Add, tr(lang, "nav.new"), tint = Ink)
                }
            }, colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent)) },
        ) { pad ->
            LazyColumn(Modifier.fillMaxSize().padding(pad).padding(12.dp)) {
                active?.let { run ->
                    item { ActiveAgentCard(run, lang) }
                    item { Spacer(Modifier.height(12.dp)) }
                }
                if (sessions.isNotEmpty()) item {
                    OutlinedTextField(filtre, { filtre = it }, Modifier.fillMaxWidth(),
                        label = { Text(tr(lang, "app.sess.search")) }, singleLine = true)
                    Spacer(Modifier.height(4.dp))
                }
                if (llista.isEmpty() && active == null) item {
                    EmptyBoard("≋", tr(lang, "app.sess.empty"), "")
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
                                        Icon(Icons.Default.ChatBubbleOutline, null, tint = Sea,
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
                                    Icon(Icons.Default.Delete, tr(lang, "app.sess.delete"), tint = Muted)
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
                    run.events.forEach { event ->
                        Text(renderMd(event.text.take(500), CodeDark), color = if (event.kind == "error") MaterialTheme.colorScheme.error else Ink,
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
    LaunchedEffect(Unit) { vm.loadGoals() }
    GregalBg {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = { TopAppBar(navigationIcon = { IconButton(onClick = onMenu) { Icon(Icons.Default.Menu, tr(lang, "app.nav.menu"), tint = Ink) } }, title = { Text(tr(lang, "app.goals.title"), color = Ink) },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent)) },
        ) { pad ->
            LazyColumn(Modifier.fillMaxSize().padding(pad).padding(12.dp)) {
                if (goals.isEmpty()) item {
                    EmptyBoard("≋", tr(lang, "app.goals.empty"), tr(lang, "app.goals.emptyHelp"))
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
                                    Icon(Icons.Default.TaskAlt, null, tint = Sea,
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
                                    Icon(Icons.Default.PlayArrow, tr(lang, "app.goals.run"))
                                    Spacer(Modifier.width(4.dp))
                                    Text(tr(lang, "app.goals.run"))
                                }
                                var menu by remember { mutableStateOf(false) }
                                Box {
                                    IconButton(onClick = { menu = true }) {
                                        Icon(Icons.Default.MoreVert, tr(lang, "app.goals.more"), tint = Muted)
                                    }
                                    DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                                        DropdownMenuItem(
                                            text = { Text(tr(lang, "btn.delete"), color = Ink) },
                                            leadingIcon = {
                                                Icon(Icons.Default.Delete, null, tint = Muted)
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
        title = { Text("≋ Gregal ${info.latestVersion}", color = Ink) },
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
