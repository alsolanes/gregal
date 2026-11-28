package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

/*
 * «Posa ordre» als vents (winds_tidy.go): duplicats (mateix nom o ofici),
 * vents de prova i vents sense ús. Ajuntar porta tot el de l'un a l'altre
 * (memòria, converses, rutines i grups) i arxiva el que sobra; arxivat es pot
 * recuperar. Igual que a l'escriptori.
 */

data class TidyWind(val id: String, val name: String, val role: String, val msgs: Int, val threads: Int, val memory: Int, val knowledge: Int, val jobs: Int)
data class TidyGroup(val reason: String, val keep: String, val winds: List<TidyWind>)
data class TidyReport(val total: Int, val groups: List<TidyGroup>, val test: List<TidyWind>, val unused: List<TidyWind>) {
    val issues: Int get() = groups.sumOf { it.winds.size - 1 } + test.size + unused.size
}

object TidyLogic {
    private fun winds(a: JSONArray?): List<TidyWind> = if (a == null) emptyList() else (0 until a.length()).mapNotNull { i ->
        a.optJSONObject(i)?.let { TidyWind(it.optString("id"), it.optString("name"), it.optString("role"), it.optInt("msgs"), it.optInt("threads"), it.optInt("memory"), it.optInt("knowledge"), it.optInt("jobs")) }
    }

    fun parse(raw: String): TidyReport {
        val o = JSONObject(raw)
        val g = o.optJSONArray("groups") ?: JSONArray()
        return TidyReport(
            o.optInt("total"),
            (0 until g.length()).mapNotNull { i -> g.optJSONObject(i)?.let { TidyGroup(it.optString("reason"), it.optString("keep"), winds(it.optJSONArray("winds"))) } },
            winds(o.optJSONArray("test")), winds(o.optJSONArray("unused")),
        )
    }

    /** La línia de sota del nom: ofici i el que té (missatges, memòria, rutines). */
    fun line(lang: String, w: TidyWind): String {
        val en = lang == "en"
        val bits = buildList {
            if (w.msgs > 0) add(if (en) "${w.msgs} messages" else "${w.msgs} missatges")
            if (w.threads > 1) add(if (en) "${w.threads} chats" else "${w.threads} converses")
            if (w.memory > 0 || w.knowledge > 0) add(if (en) "with memory" else "amb memòria")
            if (w.jobs > 0) add(if (en) "${w.jobs} routines" else "${w.jobs} rutines")
            if (isEmpty()) add(if (en) "unused" else "sense ús")
        }
        return (if (w.role.isNotBlank()) w.role + " · " else "") + bits.joinToString(" · ")
    }

    fun report(api: GregalApi): TidyReport = parse(api.get("/api/winds/tidy").toString())
    fun merge(api: GregalApi, keep: String, from: String) { api.post("/api/winds/tidy", JSONObject().put("action", "merge").put("keep", keep).put("from", from)) }
    fun archive(api: GregalApi, id: String) { api.post("/api/winds/tidy", JSONObject().put("action", "archive").put("id", id)) }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun TidySheet(vm: GregalViewModel, lang: String, onDismiss: () -> Unit) {
    val en = lang == "en"
    val scope = rememberCoroutineScope()
    var report by remember { mutableStateOf<TidyReport?>(null) }
    var status by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var confirm by remember { mutableStateOf<Pair<String, suspend () -> Unit>?>(null) }
    suspend fun reload() {
        report = try { withContext(Dispatchers.IO) { TidyLogic.report(vm.conn.value.api()) } } catch (e: Exception) {
            if (e is CancellationException) throw e
            status = e.message.orEmpty(); null
        }
    }
    LaunchedEffect(Unit) { reload() }
    ModalBottomSheet(onDismissRequest = onDismiss, containerColor = SuperficieOpaca) {
        Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).padding(horizontal = 20.dp, vertical = 6.dp).testTag("tidy-sheet"),
            verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text(if (en) "Tidy up your winds" else "Posa ordre als vents", color = Ink, fontSize = GregalSize.title, fontWeight = FontWeight.SemiBold)
            Text(if (en) "Merging moves everything from one wind into the other (memory, chats, routines and groups) and archives the leftover. Archived winds can be restored from the library."
                else "Ajuntar porta tot el d'un vent a l'altre (memòria, converses, rutines i grups) i arxiva el que sobra. Arxivat vol dir que el pots recuperar des de la biblioteca.",
                color = Muted, fontSize = GregalSize.small, lineHeight = 18.sp)
            val r = report
            if (r == null) {
                if (status.isBlank()) CircularProgressIndicator(Modifier.size(24.dp), color = Sea, strokeWidth = 2.dp)
            } else {
                r.groups.forEach { g ->
                    var keep by remember(g) { mutableStateOf(g.keep) }
                    TidyBox(if (g.reason == "same_name") (if (en) "Same name: " else "Mateix nom: ") + g.winds.first().name
                        else (if (en) "Same trade: " else "Mateix ofici: ") + g.winds.first().role) {
                        g.winds.forEach { w ->
                            Row(Modifier.fillMaxWidth().clip(RoundedCornerShape(10.dp)).clickable { keep = w.id }.padding(vertical = 4.dp),
                                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                RadioButton(selected = keep == w.id, onClick = { keep = w.id }, colors = RadioButtonDefaults.colors(selectedColor = Sea))
                                TidyName(lang, w, Modifier.weight(1f))
                            }
                        }
                        val into = g.winds.firstOrNull { it.id == keep }
                        Button(onClick = {
                            val others = g.winds.filter { it.id != keep }
                            confirm = ((if (en) "Everything from " else "Tot el de ") + others.joinToString(", ") { it.name } +
                                (if (en) " will move into " else " passarà a ") + into?.name.orEmpty() +
                                (if (en) ". Then they will be archived (they can be restored)." else ". Després s'arxivaran (es poden recuperar).")) to {
                                others.forEach { o -> TidyLogic.merge(vm.conn.value.api(), keep, o.id) }
                            }
                        }, enabled = !busy, colors = ButtonDefaults.buttonColors(containerColor = Sea, contentColor = OnSea),
                            modifier = Modifier.testTag("tidy-merge")) {
                            Text((if (en) "Merge into " else "Ajunta-les a ") + into?.name.orEmpty())
                        }
                    }
                }
                listOf((if (en) "Look like tests" else "Semblen de prova") to r.test, (if (en) "Never used" else "Sense cap ús") to r.unused).forEach { (title, list) ->
                    if (list.isNotEmpty()) TidyBox(title) {
                        list.forEach { w ->
                            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                TidyName(lang, w, Modifier.weight(1f))
                                OutlinedButton(onClick = {
                                    confirm = ((if (en) "Archive " else "S'arxivarà ") + w.name + (if (en) " (it can be restored from the library)." else " (es pot recuperar des de la biblioteca).")) to {
                                        TidyLogic.archive(vm.conn.value.api(), w.id)
                                    }
                                }, enabled = !busy) { Text(if (en) "Archive" else "Arxiva", color = Ink) }
                            }
                        }
                    }
                }
                if (r.issues == 0) Text(if (en) "All tidy: no duplicates or test winds." else "Tot en ordre: no hi ha duplicats ni vents de prova.", color = Ink, fontSize = GregalSize.body)
            }
            if (status.isNotBlank()) Text(status, color = Muted, fontSize = GregalSize.small, modifier = Modifier.testTag("tidy-status"))
            Spacer(Modifier.height(16.dp))
        }
    }
    confirm?.let { (text, action) ->
        AlertDialog(onDismissRequest = { confirm = null }, text = { Text(text) },
            confirmButton = {
                TextButton(onClick = {
                    confirm = null
                    busy = true; status = if (en) "Working…" else "Fent-ho…"
                    scope.launch {
                        try {
                            withContext(Dispatchers.IO) { action() }
                            status = if (en) "Done." else "Fet."
                            vm.loadWinds()
                            reload()
                        } catch (e: Exception) {
                            if (e is CancellationException) throw e
                            status = LocalLogic.serverMessage(e).ifBlank { e.message.orEmpty() }
                        } finally { busy = false }
                    }
                }) { Text(if (en) "Go ahead" else "Endavant") }
            },
            dismissButton = { TextButton(onClick = { confirm = null }) { Text(if (en) "Cancel" else "Cancel·la") } })
    }
}

@Composable
private fun TidyBox(title: String, content: @Composable ColumnScope.() -> Unit) {
    val shape = RoundedCornerShape(14.dp)
    Column(Modifier.fillMaxWidth().clip(shape).border(1.dp, Line, shape).padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(title, color = Ink, fontSize = GregalSize.body, fontWeight = FontWeight.SemiBold)
        content()
    }
}

@Composable
private fun TidyName(lang: String, w: TidyWind, modifier: Modifier) {
    Column(modifier) {
        Text(w.name, color = Ink, fontSize = GregalSize.body, fontWeight = FontWeight.SemiBold)
        Text(TidyLogic.line(lang, w), color = Muted, fontSize = GregalSize.caption, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}
