package xyz.solanes.gregal

import android.content.Context
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.slideInVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowForward
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * La conversa amb el Ventet: el mateix xat que els vents (capçalera, fil i
 * camp d'escriptura fix a baix). Les delegacions són part del fil: la
 * proposta amb les cares de qui ho farà, «endavant» o una altra opció, i
 * després l'enllaç a la conversa que n'ha sortit. Menjar, jugar i vestir-lo
 * són al seu perfil (tocant la capçalera).
 */
@Composable
internal fun VentetConversation(vm: GregalViewModel, lang: String, winds: List<Wind>, onBack: () -> Unit) {
    val ui by vm.cap.collectAsState()
    val st = ui.state
    val accent = Color(st.argb)
    var field by rememberSaveable(stateSaver = TextFieldValue.Saver) { mutableStateOf(TextFieldValue("")) }
    var profile by remember { mutableStateOf(false) }
    var work by remember { mutableStateOf(false) }
    var computer by remember { mutableStateOf(false) }
    val tasks by vm.windTasks.collectAsState()
    val running = tasks.orEmpty().count { it.running }
    val listState = rememberLazyListState()
    LaunchedEffect(Unit) { vm.loadCap() }
    // El missatge que no ha arribat torna al camp: no es perd res.
    LaunchedEffect(ui.failedText) {
        val t = ui.failedText
        if (t.isNotBlank() && field.text.isBlank()) field = TextFieldValue(t, TextRange(t.length))
    }
    val follow = rememberChatFollow(listState, "ventet")
    ReadReplyFromStart(listState, follow, ui.busy, capReplyKey(ui))
    // Sense «tens novetats» a la cara: les novetats ja són al fil, amb el seu separador.
    val mood = when {
        ui.busy || ui.going -> WindMood.THINKING
        ui.pending != null -> WindMood.HAPPY
        else -> WindMood.IDLE
    }
    Box(Modifier.fillMaxSize().background(Superficie).testTag("ventet-chat")) {
        Column(Modifier.fillMaxSize()) {
            // El subtítol només diu feina de debò: res de «coordinador» permanent.
            VentsChatHeader(lang, st.name,
                when {
                    ui.going -> tr(lang, "app.cap.going")
                    ui.busy -> tr(lang, "app.cap.thinking")
                    else -> ""
                },
                accent, st.faceStyle, mood, onBack = onBack, onProfile = { profile = true }, centered = true,
                onComputer = { computer = true })
            VentetThread(vm, lang, winds, listState, accent, Modifier.weight(1f), showEmpty = true)
            // Es pot escriure sempre (l'esborrany es conserva); enviar espera que acabi el torn.
            VentsComposer(lang, field, { field = it }, tr(lang, "app.cap.hint", st.name),
                canSend = field.text.isNotBlank() && !ui.busy,
                busy = ui.busy || ui.going, accent = accent,
                workCount = running, onWork = { work = true }) {
                follow.requestFollow(); vm.askCap(field.text); field = TextFieldValue("")
            }
        }
    }
    if (profile) VentetProfile(vm, lang) { profile = false }
    if (computer) OrdinadorSheet(vm.conn.collectAsState().value, lang, accent) { computer = false }
    if (work) FeinesDialog(vm, lang, onOpenThread = { vm.openVentsChat("rot:$it") }) { work = false }
}

/**
 * El fil del Ventet: els missatges, les propostes de delegació, el resum del
 * que ha passat i l'error si no ha arribat. El fan servir les dues pantalles
 * que el mostren —la conversa sencera i la portada amb la lluïssor— perquè
 * sigui exactament el mateix xat i no dues coses que s'assemblen.
 */

/** Claus dels elements en directe del fil (la pregunta i la resposta que s'escriu). */
internal const val CAP_KEY_ASKED = "live:asked"
internal const val CAP_KEY_LIVE = "live:reply"

/** La clau d'un missatge del fil del Ventet: l'id; l'índex només si el servidor no en dona. */
internal fun capItemKey(history: List<CapMsg>, index: Int): String = threadItemKey(history[index].id, index)

/** La resposta del torn del Ventet: en directe, la bombolla viva; en acabar, l'últim missatge seu. */
internal fun capReplyKey(ui: GregalViewModel.CapUi): Any? = when {
    ui.busy -> if (ui.live.isNotBlank()) CAP_KEY_LIVE else null
    ui.state.history.lastOrNull()?.from == "cap" -> capItemKey(ui.state.history, ui.state.history.lastIndex)
    else -> null
}

/**
 * On comencen les novetats d'aquest fil, per al separador local «Una novetat
 * per a tu». La marca (l'id de l'últim missatge vist) viu al telèfon: no és un
 * missatge del model ni passa pel servidor. Es llegeix un sol cop en obrir
 * (sobreviu a girar la pantalla) i s'avança mentre el fil és a la vista.
 */
@Composable
private fun rememberUnreadIndex(thread: String, history: List<CapMsg>): Int? {
    val ctx = LocalContext.current
    val prefs = remember { ctx.getSharedPreferences("vents_thread_seen", Context.MODE_PRIVATE) }
    val seenAtOpen by rememberSaveable(thread) { mutableStateOf(prefs.getString(thread, "").orEmpty()) }
    val lastId = history.lastOrNull()?.id.orEmpty()
    LaunchedEffect(thread, lastId) {
        if (lastId.isNotBlank()) prefs.edit().putString(thread, lastId).apply()
    }
    return remember(history, seenAtOpen) {
        firstUnreadIndex(history.map { it.id }, history.map { it.from }, seenAtOpen)
    }
}

@Composable
internal fun VentetThread(
    vm: GregalViewModel,
    lang: String,
    winds: List<Wind>,
    listState: LazyListState,
    accent: Color,
    modifier: Modifier = Modifier,
    /** A la portada el fil només surt quan hi ha conversa: allà l'estat buit és la salutació. */
    showEmpty: Boolean = true,
) {
    val ui by vm.cap.collectAsState()
    val rotllanes by vm.rotllanes.collectAsState()
    val st = ui.state
    val history = st.history
    val slots = remember(history) { threadSlots(history.map { ThreadEntry(if (it.from == "user") "user" else "cap", it.ts) }) }
    val unread = rememberUnreadIndex("cap", history)
    // Només els missatges que arriben en directe entren amb moviment; en obrir, tot de cop.
    var liveFrom by remember { mutableIntStateOf(Int.MAX_VALUE) }
    LaunchedEffect(history.isNotEmpty()) { if (history.isNotEmpty() && liveFrom == Int.MAX_VALUE) liveFrom = history.size }
    LazyColumn(modifier.fillMaxWidth(), state = listState,
        contentPadding = PaddingValues(horizontal = 12.dp, vertical = 10.dp)) {
        if (history.isEmpty() && showEmpty) item {
            if (ui.loadError) CapLoadError(lang) { vm.loadCap() }
            else ChatEmptyState(
                face = { WindHalo(accent, st.faceStyle, size = 88, mood = WindMood.HAPPY, tappable = true) },
                title = st.name,
                hint = tr(lang, "app.cap.hello"),
            )
        }
        items(history.size, key = { capItemKey(history, it) }, contentType = { history[it].from }) { i ->
            val m = history[i]
            val slot = slots[i]
            Column(Modifier.padding(top = if (i == 0) 0.dp else if (slot.firstOfRun) GregalChat.gapBetween else GregalChat.gapInRun)) {
                if (i == unread) ThreadDivider(threadCopy(lang, "unread"), accent = true)
                slot.separatorAt?.let { ThreadDivider(threadSeparatorLabel(lang, it)) }
                MessageIn(animate = i >= liveFrom) {
                    when {
                        m.action == "link" -> CapLinkCard(lang, m, accent, winds, rotllanes) { vm.openVentsChat(m.link) }
                        else -> Column {
                            WindBubble(GregalViewModel.PetMsg(if (m.from == "user") "user" else "pet", m.text, ts = m.ts, id = m.id), accent, lang = lang,
                                tail = slot.lastOfRun, showMeta = slot.lastOfRun)
                            if (m.delegates) {
                                val pending = ui.pending
                                if (i == history.lastIndex && pending != null) {
                                    // La proposta: una targeta de 15 dp al mateix fil. En
                                    // resoldre's queda com a rebut d'una línia (CapDecisionDone).
                                    Surface(color = Superficie, shape = GregalShape.card,
                                        border = androidx.compose.foundation.BorderStroke(1.dp, Line),
                                        modifier = Modifier.fillMaxWidth().padding(top = 6.dp)) {
                                        Box(Modifier.padding(12.dp)) { CapDecisionRow(vm, lang, pending, winds, ui.course, ui.going) }
                                    }
                                } else CapDecisionDone(lang, m, winds)
                            }
                        }
                    }
                }
            }
        }
        // Mentre respon: la teva pregunta ja al fil (abans no hi sortia fins a
        // tenir la resposta) i, quan arriba, el text en directe.
        if (ui.busy && ui.asked.isNotBlank() && history.lastOrNull()?.let { it.from == "user" && it.text == ui.asked } != true) {
            item(key = CAP_KEY_ASKED) {
                Column(Modifier.padding(top = if (history.lastOrNull()?.from == "user") GregalChat.gapInRun else GregalChat.gapBetween)) {
                    MessageIn(animate = true) { WindBubble(GregalViewModel.PetMsg("user", ui.asked), accent, lang = lang) }
                }
            }
        }
        if (ui.busy && ui.live.isNotBlank()) item(key = CAP_KEY_LIVE) {
            Column(Modifier.padding(top = GregalChat.gapBetween)) {
                WindBubble(GregalViewModel.PetMsg("pet", ui.live, streaming = true), accent, lang = lang)
            }
        }
        if ((ui.busy && ui.live.isBlank()) || ui.going) item {
            Row(Modifier.fillMaxWidth().padding(vertical = 8.dp), horizontalArrangement = Arrangement.Center,
                verticalAlignment = Alignment.CenterVertically) {
                TypingDots(accent)
                Text("  " + tr(lang, if (ui.going) "app.cap.going" else "app.cap.thinking"), fontSize = 13.sp, color = Muted)
            }
        }
        if (ui.error && !ui.busy) item {
            Column(Modifier.fillMaxWidth().padding(vertical = 8.dp).testTag("cap-send-error"),
                horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Text(tr(lang, "app.cap.error"), color = Red, fontSize = 14.sp, textAlign = TextAlign.Center)
                TextButton(onClick = { vm.capRetry() }) { Text(tr(lang, "app.cap.retry"), color = Sea) }
            }
        }
    }
}

/** No s'ha pogut carregar el Ventet: es diu i es pot tornar a provar. */
@Composable
private fun CapLoadError(lang: String, onRetry: () -> Unit) {
    Column(Modifier.fillMaxWidth().padding(32.dp).testTag("cap-load-error"),
        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(tr(lang, "app.cap.loadError"), color = Ink, fontSize = 15.sp, textAlign = TextAlign.Center)
        Button(onClick = onRetry, colors = ButtonDefaults.buttonColors(containerColor = Sea, contentColor = OnSea)) {
            Text(tr(lang, "app.cap.retry"))
        }
    }
}

/** Una proposta ja resolta (o deixada): qui la feia, en petit. */
@Composable
private fun CapDecisionDone(lang: String, m: CapMsg, winds: List<Wind>) {
    val who = m.winds.mapNotNull { id -> winds.firstOrNull { it.id == id } }
    val names = who.map { it.name } + m.create.map { it.name }
    // Propostes antigues (abans es desava menys) o vents que ja no hi són: res a dir.
    if (names.isEmpty() && m.action != "crew") return
    Row(Modifier.padding(start = 6.dp, top = 4.dp), verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        who.take(3).forEach { WindFace(size = 20, accent = Color(it.argb), style = it.faceStyle) }
        Text(when (m.action) {
            "crew" -> tr(lang, "app.cap.toCrew")
            "rotllana" -> tr(lang, "app.cap.toGroup", capRotllanaName(names, lang))
            else -> tr(lang, "app.cap.toWind", names.firstOrNull().orEmpty())
        }, color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

/** On ha anat a parar una delegació: un resultat del fil que obre aquella conversa. */
@Composable
private fun CapLinkCard(lang: String, m: CapMsg, accent: Color, winds: List<Wind>, rotllanes: List<Rotllana>, onOpen: () -> Unit) {
    val id = m.link.substringAfter(':')
    val wind = if (m.link.startsWith("wind:")) winds.firstOrNull { it.id == id } else null
    val rot = if (m.link.startsWith("rot:")) rotllanes.firstOrNull { it.id == id } else null
    val faces = wind?.let { listOf(it) } ?: rot?.members?.mapNotNull { mid -> winds.firstOrNull { it.id == mid } }.orEmpty()
    val exists = wind != null || rot != null
    ResultCard(
        title = m.text,
        subtitle = tr(lang, if (exists) "app.cap.openChat" else "app.cap.chatGone"),
        kind = resultKindOf(m.link),
        enabled = exists,
        leading = {
            faces.firstOrNull()?.let { WindHalo(Color(it.argb), it.faceStyle, size = 34, stacked = faces.drop(1), online = false) }
                ?: WindHalo(accent, windStyleOf("pso"), size = 34, online = false)
        },
        modifier = Modifier.testTag("cap-link"),
        onOpen = onOpen,
    )
}

/**
 * Un separador del fil: l'hora («Avui · 09:41») o, amb [accent], el local
 * «Una novetat per a tu». Text 12 sp entre dues línies fines; no és un missatge.
 */
@Composable
internal fun ThreadDivider(text: String, accent: Boolean = false) {
    Row(Modifier.fillMaxWidth().padding(top = 6.dp, bottom = 10.dp).testTag(if (accent) "thread-unread" else "thread-time"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        HorizontalDivider(Modifier.weight(1f), color = if (accent) Sea.copy(alpha = .45f) else Line)
        Text(text, style = GregalType.chatMeta, color = if (accent) Ink else Muted,
            fontWeight = if (accent) FontWeight.SemiBold else FontWeight.Normal, textAlign = TextAlign.Center)
        HorizontalDivider(Modifier.weight(1f), color = if (accent) Sea.copy(alpha = .45f) else Line)
    }
}

/**
 * L'entrada d'un missatge nou: fos + 8 dp cap amunt en 180 ms. Respecta
 * «reduir animacions» (LocalWindMotion) i no anima el que ja hi era en obrir.
 */
@Composable
internal fun MessageIn(animate: Boolean, content: @Composable () -> Unit) {
    if (!animate || !LocalWindMotion.current) { content(); return }
    val state = remember { MutableTransitionState(false).apply { targetState = true } }
    val shift = with(LocalDensity.current) { 8.dp.roundToPx() }
    AnimatedVisibility(state, enter = fadeIn(tween(180)) + slideInVertically(tween(180)) { shift }) { content() }
}
