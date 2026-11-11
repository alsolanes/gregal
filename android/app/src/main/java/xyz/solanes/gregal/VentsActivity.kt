package xyz.solanes.gregal

import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Delete
import androidx.compose.material.icons.rounded.KeyboardArrowDown
import androidx.compose.material.icons.rounded.KeyboardArrowUp
import androidx.compose.material.icons.rounded.Menu
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material.icons.rounded.Pause
import androidx.compose.material.icons.rounded.PlayArrow
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.json.JSONArray
import org.json.JSONObject
import java.time.LocalDate
import java.time.OffsetDateTime
import java.time.ZoneId

// Activitat i Desat dels vents: el que faran sols (GET /api/winds/automations)
// i el que han fet per a tu (GET /api/winds/saved). Una sola font per a la
// pantalla global i per a les pestanyes del perfil de cada vent.

/** Una cosa que un vent farà sol: recordatori, seguiment, vigilància o rutina. */
data class WindAutomation(
    val id: String,
    val kind: String,
    val wind: String,
    val windName: String,
    val windColor: String,
    val windStyle: String,
    val title: String,
    val detail: String,
    val repeat: String,
    val time: String,
    val intervalH: Double,
    val enabled: Boolean,
    val nextAt: String,
    val lastStatus: String,
    val canPause: Boolean,
    /** Text sencer i editable (recordatori o consigna de la rutina). */
    val text: String = "",
    /** El que l'usuari ha ensenyat a la rutina, i quants exemples té. */
    val howto: String = "",
    val refs: Int = 0,
)

data class WindAutomationCounts(val total: Int = 0, val today: Int = 0, val week: Int = 0, val paused: Int = 0)

data class WindAutomations(val items: List<WindAutomation>, val counts: WindAutomationCounts, val error: String? = null)

/** Una cosa que un vent ha fet: document, web, gràfic, imatge, fitxa o font. */
data class WindSaved(
    val kind: String,
    val title: String,
    val url: String,
    val spec: String,
    val id: String,
    val wind: String,
    val windName: String,
    val where: String,
    val ts: Long,
)

object WindActivityJson {
    fun automations(raw: String): WindAutomations {
        val o = JSONObject(raw)
        val arr = o.optJSONArray("items") ?: JSONArray()
        val items = (0 until arr.length()).mapNotNull { i ->
            val x = arr.optJSONObject(i) ?: return@mapNotNull null
            WindAutomation(
                x.optString("id"), x.optString("kind"), x.optString("wind"), x.optString("wind_name"),
                x.optString("wind_color"), x.optString("wind_style"), x.optString("title"), x.optString("detail"),
                x.optString("repeat"), x.optString("time"), x.optDouble("interval_h", 0.0),
                x.optBoolean("enabled"), x.optString("next_at"), x.optString("last_status"), x.optBoolean("can_pause"),
                x.optString("text"), x.optString("howto"), x.optInt("refs"),
            )
        }
        val c = o.optJSONObject("counts") ?: JSONObject()
        return WindAutomations(items, WindAutomationCounts(c.optInt("total"), c.optInt("today"), c.optInt("week"), c.optInt("paused")))
    }

    fun saved(raw: String): List<WindSaved> {
        val arr = JSONObject(raw).optJSONArray("items") ?: JSONArray()
        return (0 until arr.length()).mapNotNull { i ->
            val x = arr.optJSONObject(i) ?: return@mapNotNull null
            WindSaved(
                x.optString("kind"), x.optString("title"), x.optString("url"), x.optString("spec"), x.optString("id"),
                x.optString("wind"), x.optString("wind_name"), x.optString("where"), x.optLong("ts"),
            )
        }
    }
}

fun GregalApi.getWindAutomations(wind: String = ""): WindAutomations {
    val q = if (wind.isBlank()) "" else "?wind=" + java.net.URLEncoder.encode(wind, "UTF-8")
    return WindActivityJson.automations(get("/api/winds/automations$q").toString())
}

fun GregalApi.getWindSaved(wind: String = ""): List<WindSaved> {
    val q = if (wind.isBlank()) "" else "?wind=" + java.net.URLEncoder.encode(wind, "UTF-8")
    return WindActivityJson.saved(get("/api/winds/saved$q").toString())
}

/** Fes-ho ara, pausa/reprèn o esborra una automatització pròpia. */
fun GregalApi.windAutomationAction(id: String, action: String): Boolean = when (action) {
    "run" -> post("/api/winds/rutines/run", JSONObject().put("id", id)).optBoolean("accepted", false)
    "pause" -> post("/api/winds/rutines/toggle", JSONObject().put("id", id).put("enabled", false)).has("id")
    "resume" -> post("/api/winds/rutines/toggle", JSONObject().put("id", id).put("enabled", true)).has("id")
    "delete" -> post("/api/winds/rutines/delete", JSONObject().put("id", id)).optBoolean("ok", false)
    else -> false
}

/** Encén o apaga els seguiments que el vent s'ofereix a fer pel seu compte. */
fun GregalApi.setWindFollowups(w: Wind, on: Boolean) {
    val payload = JSONObject(WindRequests.save(w.name, w.color, w.role, w.style, w.persona))
        .put("id", w.id).put("followups", if (on) "on" else "off")
    post("/api/winds", payload)
}

/** «avui 18:00», «demà 09:00», «dj. 21/11 09:00». Pura: testejable. */
fun automationWhen(lang: String, iso: String, today: LocalDate = LocalDate.now(), zone: ZoneId = ZoneId.systemDefault()): String {
    if (iso.isBlank()) return ""
    val t = try { OffsetDateTime.parse(iso).atZoneSameInstant(zone) } catch (_: Exception) { return "" }
    val hm = "%02d:%02d".format(t.hour, t.minute)
    val d = t.toLocalDate()
    return when (d) {
        today -> tr(lang, "act.today", hm)
        today.plusDays(1) -> tr(lang, "act.tomorrow", hm)
        else -> {
            val dow = tr(lang, "act.dow." + d.dayOfWeek.value)
            "%s %02d/%02d %s".format(dow, d.dayOfMonth, d.monthValue, hm)
        }
    }
}

/** La periodicitat en paraules. Pura. */
fun automationRepeat(lang: String, a: WindAutomation): String = when (a.repeat) {
    "daily" -> tr(lang, "act.daily", a.time)
    "interval" -> tr(lang, "act.every", if (a.intervalH % 1.0 == 0.0) a.intervalH.toInt().toString() else a.intervalH.toString())
    "watch" -> tr(lang, "act.watchRepeat")
    else -> tr(lang, "act.once")
}

/** Filtres de la pantalla d'activitat. Pura: els tests en comproven el tall. */
fun filterAutomations(items: List<WindAutomation>, filter: String, today: LocalDate = LocalDate.now(), zone: ZoneId = ZoneId.systemDefault()): List<WindAutomation> = when (filter) {
    "today" -> items.filter { a ->
        a.enabled && a.nextAt.isNotBlank() && runCatching { !OffsetDateTime.parse(a.nextAt).atZoneSameInstant(zone).toLocalDate().isAfter(today) }.getOrDefault(false)
    }
    "followup" -> items.filter { it.kind == "followup" }
    "reminder" -> items.filter { it.kind == "reminder" || it.kind == "task" }
    "watch" -> items.filter { it.kind == "watch" }
    "routine" -> items.filter { it.kind == "routine" }
    "paused" -> items.filter { !it.enabled }
    else -> items
}

@Composable
private fun FilterChips(options: List<Pair<String, String>>, selected: String, onPick: (String) -> Unit) {
    Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        for ((key, label) in options) {
            FilterChip(selected = selected == key, onClick = { onPick(key) }, label = { Text(label, fontSize = 13.sp) })
        }
    }
}

@Composable
private fun KindBadge(lang: String, kind: String) {
    // Colors del tema (cap hex literal fora de paleta): la insígnia respira
    // amb els cinc temes en comptes de trencar-los.
    val color = when (kind) {
        "followup" -> Sea
        "watch" -> Algua
        "routine" -> Lila
        else -> Sand
    }
    Surface(color = color.copy(alpha = 0.18f), shape = RoundedCornerShape(50)) {
        Text(tr(lang, "act.kind.$kind"), color = color, fontSize = 11.sp, fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp))
    }
}

/** Una fila: cara del vent, què farà, quan, i les accions a mà. */
@Composable
fun WindAutomationRow(lang: String, a: WindAutomation, showWind: Boolean, onAction: (WindAutomation, String) -> Unit,
                      extra: @Composable (WindAutomation) -> Unit = {}) {
    var open by remember(a.id) { mutableStateOf(false) }
    var confirmDelete by remember(a.id) { mutableStateOf(false) }
    val accent = runCatching { Color(android.graphics.Color.parseColor(a.windColor)) }.getOrDefault(Color(0xFFFF9F00))
    Surface(color = Ink.copy(alpha = if (a.enabled) 0.04f else 0.02f), shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth().clickable { open = !open }) {
        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                if (showWind) WindFace(size = 36, accent = accent, style = windStyleOf(a.windStyle))
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(a.title, color = if (a.enabled) Ink else Muted, fontWeight = FontWeight.SemiBold, fontSize = 14.sp,
                        maxLines = if (open) 6 else 2, overflow = TextOverflow.Ellipsis)
                    if (!open && a.detail.isNotBlank() && a.detail != a.title) Text(a.detail, color = Muted, fontSize = 12.sp,
                        maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
                        KindBadge(lang, a.kind)
                        // Absoluta i, si és a prop, també relativa: «avui 18:00 · d'aquí a 2 h».
                        val whenText = if (!a.enabled) tr(lang, "act.paused")
                            else listOf(automationWhen(lang, a.nextAt), automationRelative(lang, a.nextAt)).filter { it.isNotBlank() }.joinToString(" · ")
                        Text(listOfNotNull(if (showWind) a.windName else null, whenText.ifBlank { null }, automationRepeat(lang, a).takeIf { a.repeat != "once" })
                            .joinToString(" · "), color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                }
                Icon(if (open) Icons.Rounded.KeyboardArrowUp else Icons.Rounded.KeyboardArrowDown, tr(lang, "app.vents.more"),
                    tint = Muted, modifier = Modifier.size(22.dp))
            }
            if (open) {
                if (a.detail.isNotBlank() && a.detail != a.title) Text(a.detail, color = Tinta, fontSize = 13.sp, lineHeight = 18.sp)
                if (a.lastStatus == "error") Text(tr(lang, "act.lastError"), color = Red, fontSize = 12.sp)
                extra(a)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    if (a.enabled) AssistChip(onClick = { onAction(a, "run") }, label = { Text(tr(lang, "act.runNow")) },
                        leadingIcon = { Icon(Icons.Rounded.PlayArrow, null, Modifier.size(18.dp)) })
                    if (a.canPause) AssistChip(onClick = { onAction(a, if (a.enabled) "pause" else "resume") },
                        label = { Text(tr(lang, if (a.enabled) "act.pause" else "act.resume")) },
                        leadingIcon = { Icon(if (a.enabled) Icons.Rounded.Pause else Icons.Rounded.PlayArrow, null, Modifier.size(18.dp)) })
                    AssistChip(onClick = { confirmDelete = true }, label = { Text(tr(lang, if (a.kind == "followup") "act.undo" else "act.delete")) },
                        leadingIcon = { Icon(Icons.Rounded.Delete, null, Modifier.size(18.dp)) })
                }
            }
        }
    }
    if (confirmDelete) {
        AlertDialog(
            onDismissRequest = { confirmDelete = false },
            title = { Text(tr(lang, "act.deleteTitle")) },
            text = { Text(a.title, maxLines = 4, overflow = TextOverflow.Ellipsis) },
            confirmButton = { TextButton(onClick = { confirmDelete = false; onAction(a, "delete") }) { Text(tr(lang, "act.delete")) } },
            dismissButton = { TextButton(onClick = { confirmDelete = false }) { Text(tr(lang, "act.cancel")) } },
        )
    }
}

/** Resum de dalt: què passarà avui, aquesta setmana i què hi ha en pausa. */
@Composable
fun WindAutomationSummary(lang: String, c: WindAutomationCounts) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        for ((label, n) in listOf(tr(lang, "act.sumToday") to c.today, tr(lang, "act.sumWeek") to c.week, tr(lang, "act.sumPaused") to c.paused)) {
            Surface(color = Ink.copy(alpha = 0.04f), shape = MaterialTheme.shapes.medium, modifier = Modifier.weight(1f)) {
                Column(Modifier.padding(vertical = 10.dp, horizontal = 12.dp)) {
                    Text(n.toString(), color = Ink, fontSize = 20.sp, fontWeight = FontWeight.SemiBold)
                    Text(label, color = Muted, fontSize = 12.sp)
                }
            }
        }
    }
}

/** La llista d'activitat amb filtres; `wind` buit = tots els vents. */
@Composable
fun WindAutomationList(vm: GregalViewModel, lang: String, wind: String = "", showSummary: Boolean = true) {
    val data by vm.automations.collectAsState()
    LaunchedEffect(wind) { vm.loadAutomations(wind) }
    var filter by remember { mutableStateOf("all") }
    val got = data[wind]
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        if (got == null) {
            LinearProgressIndicator(Modifier.fillMaxWidth())
            return@Column
        }
        if (got.error != null) {
            // Ha fallat la càrrega: no és el mateix que «no hi ha res programat».
            Surface(color = MaterialTheme.colorScheme.errorContainer, shape = MaterialTheme.shapes.medium) {
                Column(Modifier.fillMaxWidth().padding(16.dp)) {
                    Text(tr(lang, "act.loadError"), color = MaterialTheme.colorScheme.onErrorContainer, fontSize = 14.sp)
                    TextButton(onClick = { vm.loadAutomations(wind) }) { Text(tr(lang, "app.chat.retry")) }
                }
            }
            return@Column
        }
        if (got.items.isEmpty()) {
            Surface(color = Ink.copy(alpha = 0.04f), shape = MaterialTheme.shapes.medium) {
                Text(tr(lang, if (wind.isBlank()) "act.emptyAll" else "act.emptyWind"), color = Tinta, fontSize = 14.sp, lineHeight = 20.sp,
                    modifier = Modifier.padding(16.dp))
            }
            return@Column
        }
        if (showSummary) WindAutomationSummary(lang, got.counts)
        val kinds = got.items.map { it.kind }.toSet()
        val options = buildList {
            add("all" to tr(lang, "act.f.all"))
            if (got.counts.today > 0) add("today" to tr(lang, "act.f.today"))
            if ("followup" in kinds) add("followup" to tr(lang, "act.f.followup"))
            if ("reminder" in kinds || "task" in kinds) add("reminder" to tr(lang, "act.f.reminder"))
            if ("watch" in kinds) add("watch" to tr(lang, "act.f.watch"))
            if ("routine" in kinds) add("routine" to tr(lang, "act.f.routine"))
            if (got.counts.paused > 0) add("paused" to tr(lang, "act.f.paused"))
        }
        if (options.size > 2) FilterChips(options, filter) { filter = it }
        val shown = filterAutomations(got.items, filter)
        if (shown.isEmpty()) Text(tr(lang, "act.noneHere"), color = Muted, fontSize = 13.sp)
        for (a in shown) {
            key(a.id) { WindAutomationRow(lang, a, showWind = wind.isBlank(), onAction = { item, action -> vm.automationAction(item, action, wind) },
                extra = { AutomationB8Actions(vm, lang, it) }) }
        }
    }
}

/** Galeria del que han fet els vents. `wind` buit = tots. */
@Composable
fun WindSavedGallery(vm: GregalViewModel, lang: String, wind: String = "", onOpenCard: ((WindSaved) -> Unit)? = null) {
    val data by vm.saved.collectAsState()
    LaunchedEffect(wind) { vm.loadSaved(wind) }
    var windFilter by remember { mutableStateOf("") }
    val uri = LocalUriHandler.current
    val got = data[wind]
    // Si encara no han fet res però han citat fonts, s'obre per les fonts.
    var filter by remember(got == null) { mutableStateOf(if (got != null && got.none { it.kind != "link" }) "link" else "") }
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        if (got == null) {
            LinearProgressIndicator(Modifier.fillMaxWidth())
            return@Column
        }
        // Les fonts citades són moltes: no tapen la feina feta si no les demanes.
        val made = got.filter { it.kind != "link" }
        if (got.isEmpty()) {
            Surface(color = Ink.copy(alpha = 0.04f), shape = MaterialTheme.shapes.medium) {
                Text(tr(lang, if (wind.isBlank()) "saved.emptyAll" else "saved.emptyWind"), color = Tinta, fontSize = 14.sp, lineHeight = 20.sp,
                    modifier = Modifier.padding(16.dp))
            }
            return@Column
        }
        val kinds = got.map { it.kind }.toSet()
        val options = buildList {
            add("" to tr(lang, "saved.f.made"))
            for (k in listOf("doc", "web", "chart", "image", "card", "link")) if (k in kinds) add(k to tr(lang, "saved.kind.$k"))
        }
        FilterChips(options, filter) { filter = it }
        if (wind.isBlank()) {
            val winds = got.map { it.wind to it.windName }.distinct()
            if (winds.size > 1) FilterChips(listOf("" to tr(lang, "saved.allWinds")) + winds, windFilter) { windFilter = it }
        }
        val shown = (if (filter.isBlank()) made else got.filter { it.kind == filter })
            .filter { windFilter.isBlank() || it.wind == windFilter }
        if (shown.isEmpty()) Text(tr(lang, "saved.noneHere"), color = Muted, fontSize = 13.sp)
        for (s in shown.take(150)) {
            key(s.kind + s.url + s.id + s.spec.hashCode()) { SavedRow(lang, s, showWind = wind.isBlank(), onOpen = {
                when {
                    s.kind == "card" -> onOpenCard?.invoke(s)
                    s.url.isNotBlank() -> try { uri.openUri(s.url) } catch (_: Exception) { }
                }
            }) }
        }
    }
}

@Composable
private fun SavedRow(lang: String, s: WindSaved, showWind: Boolean, onOpen: () -> Unit) {
    val dark = LocalPal.current.dark
    val chart = remember(s.spec) { if (s.kind == "chart") parseChartSpec(s.spec) else null }
    Surface(color = Ink.copy(alpha = 0.04f), shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth().then(if (s.kind != "chart") Modifier.clickable(onClick = onOpen) else Modifier)) {
        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(tr(lang, "saved.icon.${s.kind}"), fontSize = 18.sp)
                Column(Modifier.weight(1f)) {
                    // El gràfic ja porta el títol a dins: aquí només qui i quan.
                    if (chart == null) Text(s.title.ifBlank { tr(lang, "saved.kind.${s.kind}") }, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp,
                        maxLines = 2, overflow = TextOverflow.Ellipsis)
                    val meta = listOfNotNull(
                        if (showWind) s.windName else null,
                        s.where.ifBlank { null },
                        if (s.ts > 0) savedDay(lang, s.ts) else null,
                    ).joinToString(" · ")
                    if (meta.isNotBlank()) Text(meta, color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
            when {
                chart != null -> ChartBlock(chart, Sea, dark)
                s.kind == "image" -> MdImage(s.url, s.title, Sea, lang)
            }
        }
    }
}

/** «avui», «ahir» o la data curta. Pura. */
fun savedDay(lang: String, ts: Long, today: LocalDate = LocalDate.now(), zone: ZoneId = ZoneId.systemDefault()): String {
    val secs = if (ts > 100_000_000_000L) ts / 1000 else ts
    val d = java.time.Instant.ofEpochSecond(secs).atZone(zone).toLocalDate()
    return when (d) {
        today -> tr(lang, "saved.today")
        today.minusDays(1) -> tr(lang, "saved.yesterday")
        else -> "%02d/%02d/%d".format(d.dayOfMonth, d.monthValue, d.year)
    }
}

/** Targeta d'una acció real del torn: «🔔 Seguiment · dj. 21/11 18:00» + Desfés. */
@Composable
fun TurnActionCard(lang: String, a: WindTurnAction, accent: Color, onUndo: () -> Unit) {
    var undoing by remember(a.id) { mutableStateOf(false) }
    Surface(color = accent.copy(alpha = 0.10f), shape = RoundedCornerShape(14.dp),
        border = androidx.compose.foundation.BorderStroke(1.dp, accent.copy(alpha = 0.35f))) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Text(if (a.kind == "followup") "🔁" else "🔔", fontSize = 18.sp)
            Column(Modifier.weight(1f)) {
                val whenText = automationWhen(lang, a.atIso)
                Text(tr(lang, if (a.kind == "followup") "act.followupCard" else "act.reminderCard", whenText),
                    color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 13.sp)
                if (a.text.isNotBlank()) Text(a.text, color = Tinta, fontSize = 13.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
            if (a.id.isNotBlank()) TextButton(onClick = { undoing = true; onUndo() }, enabled = !undoing) { Text(tr(lang, "act.undo")) }
        }
    }
}

/** Desat: el que han fet tots els {vents}, amb filtres per tipus i per {vent}. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun DesatScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    Scaffold(containerColor = Superficie, snackbarHost = { NoticeHost(vm) }, topBar = {
        TopAppBar(
            title = { Text(tr(lang, "saved.title"), fontSize = 18.sp, fontWeight = FontWeight.SemiBold) },
            navigationIcon = { IconButton(onClick = onMenu) { Icon(Icons.Rounded.Menu, tr(lang, "app.nav.menu")) } },
            actions = { IconButton(onClick = { vm.loadSaved("") }) { Icon(Icons.Rounded.Refresh, tr(lang, "auto.refresh")) } },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie),
        )
    }) { pad ->
        androidx.compose.foundation.lazy.LazyColumn(Modifier.fillMaxSize().padding(pad), contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp)) {
            item(key = "help") { Text(tr(lang, "saved.help"), color = Muted, fontSize = 13.sp, lineHeight = 18.sp) }
            item(key = "gallery") { WindSavedGallery(vm, lang) }
        }
    }
}
