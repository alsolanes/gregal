package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Visibility
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.net.URLEncoder

/**
 * Responsabilitats (B6): el que Gregal vigila per a la persona, amb les fonts,
 * què pot fer sol, el pressupost i les últimes actuacions. Es pot pausar,
 * reprendre i acabar, i qualsevol missatge que en surti respon «per què ho
 * has fet?».
 *
 * Textos propis (com ordinadorCopy de B3 i threadCopy de B1): AppStrings i
 * Strings són de l'integrador.
 */
internal fun respCopy(lang: String, key: String): String {
    val en = lang == "en"
    return when (key) {
        "title" -> if (en) "Responsibilities" else "Responsabilitats"
        "intro" -> if (en) "What Gregal keeps an eye on for you. It only speaks when something changes and it has done the work."
            else "El que Gregal vigila per a tu. Només parla quan canvia alguna cosa i ja ha fet la feina."
        "empty" -> if (en) "Nothing yet. Say «keep an eye on…» in a chat, or add one here."
            else "Encara res. Digues «vigila…» en un xat, o afegeix-ne una aquí."
        "active" -> if (en) "Active" else "Activa"
        "paused" -> if (en) "Paused" else "Pausada"
        "done" -> if (en) "Finished" else "Acabada"
        "waitingComputer" -> if (en) "Starting your computer" else "Encenent el teu ordinador"
        "heartbeat" -> if (en) "Watch of the wind" else "Cor del vent"
        "pause" -> if (en) "Pause" else "Pausa"
        "resume" -> if (en) "Resume" else "Reprèn"
        "finish" -> if (en) "Finish" else "Acaba"
        "delete" -> if (en) "Delete" else "Esborra"
        "why" -> if (en) "Why did you do this?" else "Per què ho has fet?"
        "whyShort" -> if (en) "Why?" else "Per què?"
        "objective" -> if (en) "Goal" else "Objectiu"
        "sources" -> if (en) "Sources" else "Fonts"
        "canDo" -> if (en) "Does on its own" else "Fa sol"
        "askFirst" -> if (en) "Asks you first" else "Et pregunta abans"
        "askAlways" -> if (en) "Sending, buying or publishing: always asks you" else "Enviar, comprar o publicar: sempre et pregunta"
        "explore" -> if (en) "look things up" else "consultar"
        "prepare" -> if (en) "prepare (compare, summarise, draft)" else "preparar (comparar, resumir, esborrany)"
        "budget" -> if (en) "Today" else "Avui"
        "preps" -> if (en) "preparations" else "preparacions"
        "msgs" -> if (en) "messages" else "missatges"
        "changes" -> if (en) "What changed" else "Què ha canviat"
        "result" -> if (en) "What it prepared" else "Què ha preparat"
        "proposal" -> if (en) "Proposed (not done)" else "Proposat (no fet)"
        "held" -> if (en) "Delivered quietly: today's message budget was used up." else "Lliurat en silenci: el pressupost de missatges d'avui era esgotat."
        "pending" -> if (en) "changes waiting" else "canvis pendents"
        "checked" -> if (en) "checked" else "comprovat"
        "new" -> if (en) "New responsibility" else "Nova responsabilitat"
        "wind" -> if (en) "Wind" else "Vent"
        "url" -> if (en) "Public page to watch (https://…)" else "Pàgina pública a vigilar (https://…)"
        "agenda" -> if (en) "My reminders and calendar" else "Els meus recordatoris i calendari"
        "every" -> if (en) "Check every (minutes)" else "Comprova cada (minuts)"
        "create" -> if (en) "Create" else "Crea"
        "cancel" -> if (en) "Cancel" else "Cancel·la"
        "close" -> if (en) "Close" else "Tanca"
        "error" -> if (en) "Could not load responsibilities." else "No s'han pogut carregar les responsabilitats."
        "notFromResp" -> if (en) "This message did not come from a responsibility." else "Aquest missatge no ve de cap responsabilitat."
        "act.prepared" -> if (en) "Prepared and told you" else "Ho ha preparat i t'ho ha dit"
        "act.silent" -> if (en) "Changed, but not relevant" else "Ha canviat, però no era rellevant"
        "act.deduplicated" -> if (en) "Same as before: kept quiet" else "El mateix d'abans: ha callat"
        "act.deferred" -> if (en) "Out of today's budget: waiting" else "Sense pressupost avui: en espera"
        "act.error" -> if (en) "Could not prepare" else "No ho ha pogut preparar"
        "act.computer_unavailable" -> if (en) "Your computer did not start" else "L'ordinador no ha arrencat"
        "act.pause" -> if (en) "Paused by you" else "Pausada per tu"
        "act.resume" -> if (en) "Resumed by you" else "Represa per tu"
        "act.finish", "act.finished" -> if (en) "Finished" else "Acabada"
        else -> key
    }
}

internal data class RespSrc(
    val kind: String, val url: String, val label: String,
    val checkedAt: String, val changedAt: String, val error: String,
)

internal data class RespAct(
    val at: String, val kind: String, val sources: List<String>, val changes: List<String>,
    val runId: Long, val messageId: String, val resultId: String, val result: String,
    val proposal: String, val held: Boolean, val note: String,
)

internal data class RespItem(
    val id: String, val wind: String, val windName: String, val kind: String, val objective: String,
    val sources: List<RespSrc>, val allowed: List<String>, val askFirst: List<String>, val state: String,
    val everyMinutes: Int, val pending: Int, val waitingComputer: Boolean,
    val prepsToday: Int, val prepsBudget: Int, val msgsToday: Int, val msgsBudget: Int,
    val nextAt: String, val log: List<RespAct>,
)

internal data class RespWhy(val item: RespItem, val act: RespAct)

private fun JSONArray?.strings(): List<String> =
    if (this == null) emptyList() else (0 until length()).map { optString(it) }.filter { it.isNotBlank() }

internal fun parseRespAct(o: JSONObject): RespAct = RespAct(
    at = o.optString("at"), kind = o.optString("kind"), sources = o.optJSONArray("sources").strings(),
    changes = o.optJSONArray("changes").strings(), runId = o.optLong("runId"), messageId = o.optString("messageId"),
    resultId = o.optString("resultId"), result = o.optString("result"), proposal = o.optString("proposal"),
    held = o.optBoolean("held"), note = o.optString("note"),
)

internal fun parseRespItem(o: JSONObject): RespItem {
    val srcs = o.optJSONArray("sources")
    val log = o.optJSONArray("log")
    val budget = o.optJSONObject("budget") ?: JSONObject()
    val spent = o.optJSONObject("spent") ?: JSONObject()
    return RespItem(
        id = o.optString("id"), wind = o.optString("wind"), windName = o.optString("windName"),
        kind = o.optString("kind"), objective = o.optString("objective"),
        sources = if (srcs == null) emptyList() else (0 until srcs.length()).map { i ->
            val s = srcs.optJSONObject(i) ?: JSONObject()
            RespSrc(s.optString("kind"), s.optString("url"), s.optString("label"), s.optString("checkedAt"),
                s.optString("changedAt"), s.optString("error"))
        },
        allowed = o.optJSONArray("allowed").strings(), askFirst = o.optJSONArray("askFirst").strings(),
        state = o.optString("state"), everyMinutes = o.optInt("everyMinutes"), pending = o.optInt("pending"),
        waitingComputer = o.optBoolean("waitingComputer"),
        prepsToday = spent.optInt("preps"), prepsBudget = budget.optInt("prepsPerDay"),
        msgsToday = spent.optInt("messages"), msgsBudget = budget.optInt("messagesPerDay"),
        nextAt = o.optString("nextAt"),
        log = if (log == null) emptyList() else (0 until log.length()).mapNotNull { log.optJSONObject(it)?.let(::parseRespAct) },
    )
}

internal fun parseResponsabilitats(o: JSONObject): List<RespItem> {
    val arr = o.optJSONArray("responsabilitats") ?: return emptyList()
    return (0 until arr.length()).mapNotNull { arr.optJSONObject(it)?.let(::parseRespItem) }
}

internal fun parseRespWhy(o: JSONObject): RespWhy? {
    val r = o.optJSONObject("responsabilitat") ?: return null
    val a = o.optJSONObject("act") ?: return null
    return RespWhy(parseRespItem(r), parseRespAct(a))
}

/** La clau de l'estat que es mostra: esperar l'ordinador passa per davant. */
internal fun respStateKey(r: RespItem): String = when {
    r.waitingComputer && r.state == "active" -> "waitingComputer"
    r.state in setOf("active", "paused", "done") -> r.state
    else -> "active"
}

/** Les accions que admet: un cor es pausa però no s'acaba ni s'esborra. */
internal fun respActions(r: RespItem): List<String> = when (r.state) {
    "active" -> if (r.kind == "heartbeat") listOf("pause") else listOf("pause", "finish")
    "paused" -> if (r.kind == "heartbeat") listOf("resume") else listOf("resume", "finish")
    else -> if (r.kind == "heartbeat") emptyList() else listOf("delete")
}

/** «consultar, preparar»: el que fa sol, en llenguatge normal. */
internal fun respCanDo(lang: String, r: RespItem): String =
    r.allowed.ifEmpty { listOf("explore", "prepare") }.joinToString(", ") { respCopy(lang, it) }

internal fun respSourceLabel(s: RespSrc): String = s.label.ifBlank { s.url.ifBlank { s.kind } }

/** «Avui: 1/6 preparacions · 0/3 missatges». */
internal fun respBudgetLine(lang: String, r: RespItem): String =
    respCopy(lang, "budget") + ": ${r.prepsToday}/${r.prepsBudget} " + respCopy(lang, "preps") +
        " · ${r.msgsToday}/${r.msgsBudget} " + respCopy(lang, "msgs")

// --- API (crides pròpies sobre els mètodes genèrics de GregalApi) ---

internal fun GregalApi.responsabilitats(): List<RespItem> = parseResponsabilitats(get("/api/winds/responsabilitats"))

internal fun GregalApi.respAction(action: String, id: String): JSONObject =
    post("/api/winds/responsabilitats", JSONObject().put("action", action).put("id", id))

internal fun GregalApi.respCreate(wind: String, objective: String, url: String, agenda: Boolean, every: Int): JSONObject {
    val o = JSONObject().put("action", "create").put("wind", wind).put("objective", objective)
        .put("agenda", agenda).put("everyMinutes", every)
    if (url.isNotBlank()) o.put("urls", JSONArray().put(url.trim()))
    return post("/api/winds/responsabilitats", o)
}

internal fun GregalApi.respWhy(messageId: String): RespWhy? =
    parseRespWhy(get("/api/winds/responsabilitats/why?message_id=" + URLEncoder.encode(messageId, "UTF-8")))

// --- UI ---

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun ResponsabilitatsSheet(conn: Conn, lang: String, winds: List<Wind>, accent: Color = Color.Unspecified, onClose: () -> Unit) {
    val scope = rememberCoroutineScope()
    var items by remember { mutableStateOf<List<RespItem>?>(null) }
    var error by remember { mutableStateOf("") }
    var creating by remember { mutableStateOf(false) }
    var why by remember { mutableStateOf<RespWhy?>(null) }
    val acc = if (accent != Color.Unspecified) accent else Sea
    suspend fun reload() {
        try {
            items = withContext(Dispatchers.IO) { conn.api().responsabilitats() }
            error = ""
        } catch (e: Exception) {
            if (e is CancellationException) throw e
            error = respCopy(lang, "error")
        }
    }
    LaunchedEffect(Unit) { reload() }
    fun act(action: String, id: String) = scope.launch {
        try { withContext(Dispatchers.IO) { conn.api().respAction(action, id) } } catch (e: Exception) { if (e is CancellationException) throw e }
        reload()
    }
    ModalBottomSheet(onDismissRequest = onClose, containerColor = Superficie,
        sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp).padding(bottom = 16.dp).verticalScroll(rememberScrollState())
            .testTag("resp-sheet"), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text(respCopy(lang, "title"), color = Ink, fontSize = 18.sp, fontWeight = FontWeight.SemiBold,
                modifier = Modifier.semantics { heading() })
            Text(respCopy(lang, "intro"), color = Muted, fontSize = 13.sp)
            when {
                error.isNotBlank() -> Text(error, color = Red, fontSize = 13.sp)
                items == null -> {}
                items!!.isEmpty() && !creating -> Text(respCopy(lang, "empty"), color = Muted, fontSize = 14.sp, modifier = Modifier.testTag("resp-empty"))
            }
            items.orEmpty().forEach { r ->
                RespCard(r, lang, acc, onAction = { a -> act(a, r.id) },
                    onWhy = { last -> why = RespWhy(r, last) })
            }
            if (creating) {
                RespCreateForm(lang, winds, acc, onCancel = { creating = false }) { wind, obj, url, agenda, every ->
                    scope.launch {
                        try {
                            withContext(Dispatchers.IO) { conn.api().respCreate(wind, obj, url, agenda, every) }
                            creating = false
                        } catch (e: Exception) {
                            if (e is CancellationException) throw e
                            error = e.message.orEmpty().ifBlank { respCopy(lang, "error") }
                        }
                        reload()
                    }
                }
            } else if (winds.isNotEmpty()) {
                RespButton(respCopy(lang, "new"), primary = false, accent = acc, modifier = Modifier.fillMaxWidth().testTag("resp-new")) { creating = true }
            }
        }
    }
    why?.let { w -> RespWhySheetContent(w, lang, onClose = { why = null }) }
}

/** Una responsabilitat: estat, objectiu, fonts, què fa sola i accions. */
@Composable
internal fun RespCard(r: RespItem, lang: String, accent: Color, onAction: (String) -> Unit, onWhy: (RespAct) -> Unit) {
    val stateKey = respStateKey(r)
    Column(
        Modifier.fillMaxWidth().clip(GregalShape.card).background(SuperficieAlta).border(1.dp, Line, GregalShape.card)
            .padding(14.dp).testTag("resp-card-" + r.id),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Icon(Icons.Rounded.Visibility, null, tint = if (r.state == "active") accent else Muted, modifier = Modifier.size(18.dp))
            Text(if (r.kind == "heartbeat") respCopy(lang, "heartbeat") + (if (r.windName.isNotBlank()) " · " + r.windName else "")
                else r.windName, color = Muted, fontSize = 12.sp, modifier = Modifier.weight(1f))
            Text(respCopy(lang, stateKey), color = if (r.state == "active") accent else Muted, fontSize = 12.sp,
                fontWeight = FontWeight.SemiBold, modifier = Modifier.testTag("resp-state"))
        }
        Text(r.objective, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
        if (r.sources.isNotEmpty()) {
            Text(respCopy(lang, "sources") + ": " + r.sources.joinToString(" · ") { respSourceLabel(it) },
                color = Muted, fontSize = 12.sp, maxLines = 2)
        }
        r.sources.firstOrNull { it.error.isNotBlank() }?.let { Text(it.error, color = Red, fontSize = 12.sp) }
        Text(respCopy(lang, "canDo") + ": " + respCanDo(lang, r) + ". " + respCopy(lang, "askAlways") + ".",
            color = Muted, fontSize = 12.sp)
        Text(respBudgetLine(lang, r) + if (r.pending > 0) " · ${r.pending} " + respCopy(lang, "pending") else "",
            color = Muted, fontSize = 12.sp)
        r.log.firstOrNull()?.let { last ->
            Text(respCopy(lang, "act." + last.kind), color = Tinta, fontSize = 12.sp, modifier = Modifier.testTag("resp-last"))
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            respActions(r).forEach { a ->
                RespButton(respCopy(lang, a), primary = a == "resume", accent = accent, modifier = Modifier.testTag("resp-$a")) { onAction(a) }
            }
            r.log.firstOrNull { it.kind == "prepared" }?.let { last ->
                RespButton(respCopy(lang, "whyShort"), primary = false, accent = accent, modifier = Modifier.testTag("resp-why")) { onWhy(last) }
            }
        }
    }
}

@Composable
private fun RespButton(label: String, primary: Boolean, accent: Color, modifier: Modifier = Modifier, onClick: () -> Unit) {
    Box(
        modifier.heightIn(min = 48.dp).clip(GregalShape.sm).background(if (primary) accent else Color.Transparent)
            .then(if (primary) Modifier else Modifier.border(1.dp, Line, GregalShape.sm))
            .semantics { role = Role.Button }.clickable(onClick = onClick).padding(horizontal = 14.dp),
        contentAlignment = Alignment.Center,
    ) { Text(label, color = if (primary) OnSea else Ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold) }
}

@Composable
private fun RespCreateForm(lang: String, winds: List<Wind>, accent: Color, onCancel: () -> Unit,
                           onCreate: (wind: String, objective: String, url: String, agenda: Boolean, every: Int) -> Unit) {
    var wind by remember { mutableStateOf(winds.firstOrNull()?.id.orEmpty()) }
    var objective by remember { mutableStateOf("") }
    var url by remember { mutableStateOf("") }
    var agenda by remember { mutableStateOf(false) }
    var every by remember { mutableStateOf("60") }
    Column(Modifier.fillMaxWidth().clip(GregalShape.card).border(1.dp, Line, GregalShape.card).padding(14.dp).testTag("resp-form"),
        verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(respCopy(lang, "new"), color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
        Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            winds.take(6).forEach { w ->
                val sel = w.id == wind
                Box(Modifier.heightIn(min = 40.dp).clip(GregalShape.sm).background(if (sel) Color(w.argb).copy(alpha = .25f) else Color.Transparent)
                    .border(1.dp, if (sel) Color(w.argb) else Line, GregalShape.sm).semantics { role = Role.RadioButton }
                    .clickable { wind = w.id }.padding(horizontal = 10.dp), contentAlignment = Alignment.Center) {
                    Text(w.name, color = Ink, fontSize = 13.sp)
                }
            }
        }
        OutlinedTextField(objective, { objective = it }, label = { Text(respCopy(lang, "objective")) }, modifier = Modifier.fillMaxWidth().testTag("resp-objective"))
        OutlinedTextField(url, { url = it }, label = { Text(respCopy(lang, "url")) }, singleLine = true, modifier = Modifier.fillMaxWidth().testTag("resp-url"))
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(respCopy(lang, "agenda"), color = Ink, fontSize = 14.sp, modifier = Modifier.weight(1f))
            Switch(agenda, { agenda = it })
        }
        OutlinedTextField(every, { v -> every = v.filter { it.isDigit() }.take(5) }, label = { Text(respCopy(lang, "every")) }, singleLine = true, modifier = Modifier.fillMaxWidth())
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            RespButton(respCopy(lang, "cancel"), primary = false, accent = accent, modifier = Modifier.weight(1f)) { onCancel() }
            val ok = wind.isNotBlank() && objective.isNotBlank() && (url.isNotBlank() || agenda)
            RespButton(respCopy(lang, "create"), primary = ok, accent = accent, modifier = Modifier.weight(1f).testTag("resp-create")) {
                if (ok) onCreate(wind, objective.trim(), url.trim(), agenda, every.toIntOrNull() ?: 60)
            }
        }
    }
}

/**
 * «Per què ho has fet?» d'un missatge del fil (B6). Per a l'integrador: sota
 * un missatge amb relatedRunId/resultRefs, obrir `RespWhySheet(conn, lang, msg.id)`.
 */
@Composable
internal fun RespWhySheet(conn: Conn, lang: String, messageId: String, onClose: () -> Unit) {
    var why by remember(messageId) { mutableStateOf<RespWhy?>(null) }
    var missing by remember(messageId) { mutableStateOf(false) }
    LaunchedEffect(messageId) {
        try { why = withContext(Dispatchers.IO) { conn.api().respWhy(messageId) }; missing = why == null }
        catch (e: Exception) { if (e is CancellationException) throw e; missing = true }
    }
    val w = why
    if (w != null) RespWhySheetContent(w, lang, onClose)
    else if (missing) RespWhySheetContent(null, lang, onClose)
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun RespWhySheetContent(why: RespWhy?, lang: String, onClose: () -> Unit) {
    ModalBottomSheet(onDismissRequest = onClose, containerColor = Superficie) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp).padding(bottom = 20.dp).verticalScroll(rememberScrollState())) {
            RespWhyBody(why, lang)
        }
    }
}

/** El contingut de «per què?»: responsabilitat, fonts, què ha canviat i què n'ha fet. */
@Composable
internal fun RespWhyBody(why: RespWhy?, lang: String) {
    Column(Modifier.fillMaxWidth().testTag("resp-why-body"), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(respCopy(lang, "why"), color = Ink, fontSize = 17.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.semantics { heading() })
        if (why == null) {
            Text(respCopy(lang, "notFromResp"), color = Muted, fontSize = 14.sp)
            return@Column
        }
        val r = why.item
        val a = why.act
        Label(respCopy(lang, "objective")); Text(r.objective, color = Ink, fontSize = 14.sp)
        Label(respCopy(lang, "sources"))
        (a.sources.ifEmpty { r.sources.map { respSourceLabel(it) } }).forEach { Text("· $it", color = Ink, fontSize = 13.sp) }
        if (a.changes.isNotEmpty()) {
            Label(respCopy(lang, "changes"))
            a.changes.forEach { Text(it, color = Tinta, fontSize = 13.sp, maxLines = 8) }
        }
        Label(respCopy(lang, "canDo")); Text(respCanDo(lang, r) + ". " + respCopy(lang, "askAlways") + ".", color = Ink, fontSize = 13.sp)
        if (a.proposal.isNotBlank()) {
            Label(respCopy(lang, "proposal")); Text(a.proposal, color = Ink, fontSize = 13.sp)
        }
        if (a.held) Text(respCopy(lang, "held"), color = Muted, fontSize = 12.sp)
    }
}

@Composable
private fun Label(text: String) {
    Text(text, color = Muted, fontSize = 12.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 4.dp))
}
