package xyz.solanes.gregal

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.time.Duration
import java.time.Instant
import java.time.OffsetDateTime
import java.time.ZoneId

// B8 — Programacions: editar, ajornar i cancel·lar recordatoris sobre el mateix
// id, i ensenyar una rutina («fes-ho així la propera vegada»). Les crides són
// extensions de GregalApi (com la resta de l'activitat dels vents) i els textos
// viuen aquí fins que l'integrador els mogui a AppStrings.kt (paritat ca/en a mà;
// la web els porta inline a vents-activity.js).

internal val STR_B8_CA: Map<String, String> = mapOf(
    "b8.edit" to "Edita",
    "b8.snooze" to "Ajorna",
    "b8.snooze10" to "10 min",
    "b8.snooze60" to "1 h",
    "b8.snoozeTomorrow" to "Demà",
    "b8.teach" to "Ensenya-li",
    "b8.text" to "Què t'ha de dir",
    "b8.when" to "Quan (09:00, 21/11 18:30, +2h…)",
    "b8.whenHelp" to "Hora del teu mòbil (%1\$s). Buit = no la canviïs.",
    "b8.save" to "Desa",
    "b8.cancel" to "Cancel·la",
    "b8.saved" to "Desat",
    "b8.failed" to "No s'ha pogut desar: %1\$s",
    "b8.editReminder" to "Edita el recordatori",
    "b8.editRoutine" to "Edita la rutina",
    "b8.prompt" to "Què ha de fer",
    "b8.howto" to "Com ho vols (una indicació per línia)",
    "b8.howtoShown" to "Com ho vols: %1\$s",
    "b8.refs" to "%1\$s exemples desats",
    "b8.refsClear" to "Treu els exemples desats",
    "b8.teachTitle" to "Fes-ho així la propera vegada",
    "b8.teachHelp" to "Desa una indicació a aquesta rutina. La pots revisar i editar quan vulguis; només és text, no aprèn de vídeos ni demostracions.",
    "b8.instruction" to "Indicació (p. ex. «només els tres més barats, en una taula»)",
    "b8.useLast" to "Guarda l'últim resultat com a exemple",
    "b8.daily" to "Cada dia a",
    "b8.every" to "Cada (hores)",
    "b8.inMin" to "d'aquí a %1\$s min",
    "b8.inHours" to "d'aquí a %1\$s h",
    "b8.agoMin" to "fa %1\$s min",
    "b8.prefsApply" to "Aquestes preferències valen per a tot el que fa aquest {vent}: el xat, les rutines, el que vigila i els recordatoris amb feina. Corregeix-les aquí un cop i ja està.",
)

internal val STR_B8_EN: Map<String, String> = mapOf(
    "b8.edit" to "Edit",
    "b8.snooze" to "Snooze",
    "b8.snooze10" to "10 min",
    "b8.snooze60" to "1 h",
    "b8.snoozeTomorrow" to "Tomorrow",
    "b8.teach" to "Teach it",
    "b8.text" to "What it should tell you",
    "b8.when" to "When (09:00, 21/11 18:30, +2h…)",
    "b8.whenHelp" to "Your phone's time (%1\$s). Empty = keep it.",
    "b8.save" to "Save",
    "b8.cancel" to "Cancel",
    "b8.saved" to "Saved",
    "b8.failed" to "Couldn't save: %1\$s",
    "b8.editReminder" to "Edit reminder",
    "b8.editRoutine" to "Edit routine",
    "b8.prompt" to "What it should do",
    "b8.howto" to "How you want it (one instruction per line)",
    "b8.howtoShown" to "How you want it: %1\$s",
    "b8.refs" to "%1\$s saved examples",
    "b8.refsClear" to "Remove saved examples",
    "b8.teachTitle" to "Do it like this next time",
    "b8.teachHelp" to "Saves an instruction to this routine. You can review and edit it any time; it's only text, it doesn't learn from videos or demonstrations.",
    "b8.instruction" to "Instruction (e.g. “only the three cheapest, in a table”)",
    "b8.useLast" to "Keep the last result as an example",
    "b8.daily" to "Every day at",
    "b8.every" to "Every (hours)",
    "b8.inMin" to "in %1\$s min",
    "b8.inHours" to "in %1\$s h",
    "b8.agoMin" to "%1\$s min ago",
    "b8.prefsApply" to "These preferences apply to everything this {vent} does: chat, routines, what it watches and reminders with a task. Correct them here once and that's it.",
)

/** tr() amb les claus de B8 primer. */
internal fun trB8(lang: String, key: String, vararg args: Any?): String {
    val v = (if (lang == "en") STR_B8_EN else STR_B8_CA)[key] ?: return tr(lang, key, *args)
    val base = if (args.isEmpty()) v else String.format(java.util.Locale.ROOT, v, *args)
    return VentNom.aplica(base, lang)
}

/** «d'aquí a 25 min», «d'aquí a 3 h», «fa 5 min»; buit si és lluny (>12 h) o il·legible. Pura. */
internal fun automationRelative(lang: String, iso: String, now: Instant = Instant.now()): String {
    if (iso.isBlank()) return ""
    val at = try { OffsetDateTime.parse(iso).toInstant() } catch (_: Exception) { return "" }
    val min = Duration.between(now, at).toMinutes()
    return when {
        min in 0..59 -> trB8(lang, "b8.inMin", maxOf(min, 1L).toString())
        min in 60..(12 * 60) -> trB8(lang, "b8.inHours", ((min + 30) / 60).toString())
        min in -59..-1 -> trB8(lang, "b8.agoMin", (-min).toString())
        else -> ""
    }
}

/** Edita / ajorna / pausa / cancel·la un recordatori (mateix id). Torna el missatge del servidor. */
fun GregalApi.windRemindEdit(id: String, op: String, at: String = "", text: String = "", minutes: Int = 0,
                             tz: String = ZoneId.systemDefault().id): String {
    val body = JSONObject().put("id", id).put("op", op).put("tz", tz)
    if (at.isNotBlank()) body.put("at", at)
    if (text.isNotBlank()) body.put("text", text)
    if (minutes > 0) body.put("minutes", minutes)
    return post("/api/winds/remind/edit", body).optString("message")
}

/** Edita una rutina pròpia sense canviar-ne l'id. Els null no es toquen. */
fun GregalApi.windRutinaEdit(id: String, prompt: String? = null, howto: String? = null, kind: String? = null,
                             time: String? = null, intervalH: Double? = null, clearRefs: Boolean = false): JSONObject {
    val body = JSONObject().put("id", id)
    prompt?.let { body.put("prompt", it) }
    howto?.let { body.put("howto", it) }
    kind?.let { body.put("kind", it) }
    time?.let { body.put("time", it) }
    intervalH?.let { body.put("interval_h", it) }
    if (clearRefs) body.put("keep_refs", JSONArray())
    return post("/api/winds/rutines/edit", body)
}

/** «Fes-ho així la propera vegada»: indicació i, si cal, l'últim resultat com a exemple. */
fun GregalApi.windRutinaTeach(id: String, instruction: String, useLast: Boolean): JSONObject =
    post("/api/winds/rutines/teach", JSONObject().put("id", id).put("instruction", instruction).put("ref_last", useLast))

/** Executa una crida de B8 fora del fil principal i refresca l'activitat. */
@Composable
internal fun rememberB8Runner(vm: GregalViewModel, lang: String): (suspend (GregalApi) -> String?, () -> Unit) -> Unit {
    val scope = rememberCoroutineScope()
    return { call, after ->
        val conn = vm.conn.value
        scope.launch {
            val r = runCatching { withContext(Dispatchers.IO) { call(conn.api()) } }
            // El missatge del servidor és per al model (en català): l'app diu el seu.
            r.onSuccess { vm.notify(trB8(lang, "b8.saved")); after() }
            r.onFailure { e -> vm.notify(trB8(lang, "b8.failed", e.message.orEmpty())) }
            vm.refreshAutomations()
        }
    }
}

/** Editar un recordatori: text i hora (en la zona del mòbil). */
@Composable
internal fun EditReminderDialog(vm: GregalViewModel, lang: String, a: WindAutomation, onDismiss: () -> Unit) {
    val run = rememberB8Runner(vm, lang)
    var text by remember(a.id) { mutableStateOf(a.text.ifBlank { a.title }) }
    var at by remember(a.id) { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(trB8(lang, "b8.editReminder")) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(text, { text = it }, label = { Text(trB8(lang, "b8.text")) }, minLines = 2, modifier = Modifier.fillMaxWidth())
                OutlinedTextField(at, { at = it }, label = { Text(trB8(lang, "b8.when")) }, singleLine = true, modifier = Modifier.fillMaxWidth())
                Text(trB8(lang, "b8.whenHelp", ZoneId.systemDefault().id), color = Muted, fontSize = 12.sp)
            }
        },
        confirmButton = {
            TextButton(enabled = !busy && text.isNotBlank(), onClick = {
                busy = true
                run({ api -> api.windRemindEdit(a.id, "edit", at = at.trim(), text = text.trim()) }) { onDismiss() }
            }) { Text(trB8(lang, "b8.save")) }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text(trB8(lang, "b8.cancel")) } },
    )
}

/** Editar una rutina: què fa, quan, i com ho vol l'usuari. */
@Composable
internal fun EditRoutineDialog(vm: GregalViewModel, lang: String, a: WindAutomation, onDismiss: () -> Unit) {
    val run = rememberB8Runner(vm, lang)
    var prompt by remember(a.id) { mutableStateOf(a.text.ifBlank { a.detail }) }
    var howto by remember(a.id) { mutableStateOf(a.howto) }
    var time by remember(a.id) { mutableStateOf(a.time) }
    var hours by remember(a.id) { mutableStateOf(if (a.intervalH > 0) WindTrade.hours(a.intervalH) else "") }
    var clearRefs by remember(a.id) { mutableStateOf(false) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(trB8(lang, "b8.editRoutine")) },
        text = {
            Column(Modifier.verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(prompt, { prompt = it }, label = { Text(trB8(lang, "b8.prompt")) }, minLines = 2, modifier = Modifier.fillMaxWidth())
                if (a.repeat == "daily") OutlinedTextField(time, { time = it }, label = { Text(trB8(lang, "b8.daily")) }, singleLine = true)
                else OutlinedTextField(hours, { hours = it }, label = { Text(trB8(lang, "b8.every")) }, singleLine = true)
                OutlinedTextField(howto, { howto = it }, label = { Text(trB8(lang, "b8.howto")) }, minLines = 3, modifier = Modifier.fillMaxWidth())
                if (a.refs > 0) Row(verticalAlignment = Alignment.CenterVertically) {
                    Checkbox(clearRefs, { clearRefs = it })
                    Text(trB8(lang, "b8.refsClear") + " (" + a.refs + ")", fontSize = 13.sp)
                }
            }
        },
        confirmButton = {
            TextButton(enabled = prompt.isNotBlank(), onClick = {
                run({ api ->
                    api.windRutinaEdit(a.id, prompt = prompt.trim(), howto = howto.trim(),
                        time = if (a.repeat == "daily") time.trim() else null,
                        intervalH = if (a.repeat == "interval") hours.replace(',', '.').toDoubleOrNull() else null,
                        clearRefs = clearRefs)
                    null
                }) { onDismiss() }
            }) { Text(trB8(lang, "b8.save")) }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text(trB8(lang, "b8.cancel")) } },
    )
}

/** «Fes-ho així la propera vegada» des de la fila de la rutina. */
@Composable
internal fun TeachRoutineDialog(vm: GregalViewModel, lang: String, a: WindAutomation, onDismiss: () -> Unit) {
    val run = rememberB8Runner(vm, lang)
    var instruction by remember(a.id) { mutableStateOf("") }
    var useLast by remember(a.id) { mutableStateOf(a.lastStatus == "ok") }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(trB8(lang, "b8.teachTitle")) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(trB8(lang, "b8.teachHelp"), color = Muted, fontSize = 13.sp)
                OutlinedTextField(instruction, { instruction = it }, label = { Text(trB8(lang, "b8.instruction")) }, minLines = 2, modifier = Modifier.fillMaxWidth())
                if (a.lastStatus == "ok") Row(verticalAlignment = Alignment.CenterVertically) {
                    Checkbox(useLast, { useLast = it })
                    Text(trB8(lang, "b8.useLast"), fontSize = 13.sp)
                }
            }
        },
        confirmButton = {
            TextButton(enabled = instruction.isNotBlank() || useLast, onClick = {
                run({ api -> api.windRutinaTeach(a.id, instruction.trim(), useLast); null }) { onDismiss() }
            }) { Text(trB8(lang, "b8.save")) }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text(trB8(lang, "b8.cancel")) } },
    )
}

/** Accions de B8 dins la fila desplegada d'una automatització. */
@Composable
internal fun AutomationB8Actions(vm: GregalViewModel, lang: String, a: WindAutomation) {
    val run = rememberB8Runner(vm, lang)
    var dialog by remember(a.id) { mutableStateOf("") }
    val isReminder = a.kind == "reminder" || a.kind == "task" || a.kind == "followup"
    val isRoutine = a.kind == "routine"
    if (!isReminder && !isRoutine) return
    if (isRoutine && a.howto.isNotBlank()) Text(trB8(lang, "b8.howtoShown", a.howto.lines().map { it.trim().removePrefix("- ").trim() }.filter { it.isNotBlank() }.joinToString(" · ")), color = Tinta, fontSize = 13.sp, lineHeight = 18.sp)
    if (isRoutine && a.refs > 0) Text(trB8(lang, "b8.refs", a.refs.toString()), color = Muted, fontSize = 12.sp)
    Row(Modifier.horizontalScrollCompat(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        AssistChip(onClick = { dialog = "edit" }, label = { Text(trB8(lang, "b8.edit")) })
        if (isReminder) {
            AssistChip(onClick = { run({ api -> api.windRemindEdit(a.id, "snooze", minutes = 10) }) {} }, label = { Text(trB8(lang, "b8.snooze") + " " + trB8(lang, "b8.snooze10")) })
            AssistChip(onClick = { run({ api -> api.windRemindEdit(a.id, "snooze", minutes = 60) }) {} }, label = { Text(trB8(lang, "b8.snooze60")) })
            AssistChip(onClick = {
                // Demà a la mateixa hora (o a les 09:00 si no en té), en la zona del mòbil.
                val zone = ZoneId.systemDefault()
                val base = runCatching { OffsetDateTime.parse(a.nextAt).atZoneSameInstant(zone) }.getOrNull()
                val t = java.time.ZonedDateTime.now(zone).plusDays(1)
                    .withHour(base?.hour ?: 9).withMinute(base?.minute ?: 0).withSecond(0).withNano(0)
                val iso = java.time.format.DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ssXXX").format(t)
                run({ api -> api.windRemindEdit(a.id, "reschedule", at = iso) }) {}
            }, label = { Text(trB8(lang, "b8.snoozeTomorrow")) })
        }
        if (isRoutine) AssistChip(onClick = { dialog = "teach" }, label = { Text(trB8(lang, "b8.teach")) })
    }
    when (dialog) {
        "edit" -> if (isRoutine) EditRoutineDialog(vm, lang, a) { dialog = "" } else EditReminderDialog(vm, lang, a) { dialog = "" }
        "teach" -> TeachRoutineDialog(vm, lang, a) { dialog = "" }
    }
}

@Composable
private fun Modifier.horizontalScrollCompat(): Modifier = this.then(Modifier.horizontalScroll(rememberScrollState()))
