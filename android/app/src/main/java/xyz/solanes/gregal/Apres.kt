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
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

/**
 * Aprèn les teves preferències (com dots i Duet), i ho diu: «📝 Ho recordaré»
 * al fil quan aprèn alguna cosa d'un torn, i «Què sap de tu» amb tot el que
 * saben la memòria personal i la de cada vent, per oblidar-ne cada cosa
 * (internal/web/winds_apres.go). La web fa el mateix a memoryRow i apresView.
 */
data class ApresGroup(val wind: String, val name: String, val notes: List<String>)

fun GregalApi.getApres(): List<ApresGroup> {
    val o = get("/api/winds/apres")
    fun list(a: JSONArray?) = a?.let { arr -> (0 until arr.length()).map { arr.optString(it).trim() }.filter { it.isNotEmpty() } }.orEmpty()
    val out = mutableListOf<ApresGroup>()
    list(o.optJSONArray("personal")).takeIf { it.isNotEmpty() }?.let { out += ApresGroup("", "", it) }
    val winds = o.optJSONArray("winds") ?: JSONArray()
    for (i in 0 until winds.length()) {
        val w = winds.optJSONObject(i) ?: continue
        val notes = list(w.optJSONArray("notes"))
        if (notes.isNotEmpty()) out += ApresGroup(w.optString("id"), w.optString("name"), notes)
    }
    return out
}

fun GregalApi.forgetApres(wind: String, facts: List<String>): Boolean =
    post("/api/winds/apres", JSONObject().put("wind", wind).put("all", JSONArray(facts))).optBoolean("ok", false)

internal fun apresCopy(lang: String, key: String, name: String = ""): String {
    val en = lang == "en"
    return when (key) {
        "title" -> if (en) "What it knows about you" else "Què sap de tu"
        "help" -> if (en) "What Gregal and the team have learned about you over time. They use it to get things right; forget anything here."
            else "El que Gregal i l'equip han après de tu amb el temps. Ho fan servir per encertar-la; aquí pots oblidar el que vulguis."
        "personal" -> if (en) "About you" else "Sobre tu"
        "personalSub" -> if (en) "All the winds use this." else "Ho fan servir tots els vents."
        "windSub" -> if (en) "What $name knows." else "El que en sap $name."
        "empty" -> if (en) "It has not learned anything about you yet. The more you talk, the better it gets."
            else "Encara no ha après res de tu. Com més hi parlis, més t'encertarà."
        "remember" -> if (en) "I will remember" else "Ho recordaré"
        "forget" -> if (en) "Forget it" else "Oblida-ho"
        "forgotten" -> if (en) "Forgotten." else "Oblidat."
        "error" -> if (en) "Could not load it." else "No s'ha pogut carregar."
        else -> key
    }
}

/** L'avís al fil: «📝 Ho recordaré · …» amb «Oblida-ho». */
@Composable
internal fun MemoryNotice(lang: String, facts: List<String>, onForget: () -> Unit) {
    var forgot by remember(facts) { mutableStateOf(false) }
    Row(
        Modifier.fillMaxWidth().padding(start = 46.dp, top = 2.dp, bottom = 6.dp).clip(RoundedCornerShape(12.dp))
            .border(1.dp, Line, RoundedCornerShape(12.dp)).padding(start = 12.dp).testTag("memory-notice"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Text("📝", fontSize = 15.sp)
        Column(Modifier.weight(1f).padding(vertical = 8.dp)) {
            Text(apresCopy(lang, "remember"), color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 12.sp)
            Text(if (forgot) apresCopy(lang, "forgotten") else facts.joinToString(" · "), color = Muted, fontSize = 13.sp)
        }
        if (!forgot) TextButton(onClick = { forgot = true; onForget() }, modifier = Modifier.testTag("memory-forget")) {
            Text(apresCopy(lang, "forget"), color = Ink, fontSize = 13.sp)
        }
    }
}

@Composable
internal fun ApresDialog(conn: Conn, lang: String, onDismiss: () -> Unit) {
    var groups by remember { mutableStateOf<List<ApresGroup>?>(null) }
    var error by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    LaunchedEffect(conn) {
        groups = try { withContext(Dispatchers.IO) { conn.api().getApres() } } catch (e: Exception) {
            if (e is kotlinx.coroutines.CancellationException) throw e
            error = true; null
        }
    }
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(shape = MaterialTheme.shapes.large, color = MaterialTheme.colorScheme.background) {
            Column(Modifier.fillMaxWidth().fillMaxHeight(0.9f).padding(top = 16.dp).testTag("apres-dialog")) {
                Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(apresCopy(lang, "title"), color = Ink, fontWeight = FontWeight.Bold, fontSize = 18.sp, modifier = Modifier.weight(1f))
                    IconButton(onClick = onDismiss, modifier = Modifier.semantics { contentDescription = if (lang == "en") "Close" else "Tanca" }) { Text("✕", color = Muted, fontSize = 16.sp) }
                }
                Column(Modifier.verticalScroll(rememberScrollState()).padding(horizontal = 16.dp, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text(apresCopy(lang, "help"), color = Muted, fontSize = 13.sp, lineHeight = 18.sp, modifier = Modifier.padding(horizontal = 4.dp))
                    val g = groups
                    when {
                        g != null -> ApresContent(lang, g) { wind, fact ->
                            groups = g.map { if (it.wind == wind) it.copy(notes = it.notes - fact) else it }.filter { it.notes.isNotEmpty() }
                            scope.launch { runCatching { withContext(Dispatchers.IO) { conn.api().forgetApres(wind, listOf(fact)) } } }
                        }
                        error -> Text(apresCopy(lang, "error"), color = Muted, fontSize = 14.sp)
                        else -> CircularProgressIndicator(Modifier.size(24.dp))
                    }
                }
            }
        }
    }
}

/** Els grups (personal i per vent), amb ✕ per oblidar. Pura (ApresTest). */
@Composable
internal fun ApresContent(lang: String, groups: List<ApresGroup>, onForget: (wind: String, fact: String) -> Unit) {
    if (groups.isEmpty()) { Text(apresCopy(lang, "empty"), color = Muted, fontSize = 14.sp); return }
    for (g in groups) {
        Column(
            Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(SuperficieAlta).border(1.dp, Line, RoundedCornerShape(14.dp)).padding(14.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(if (g.wind.isEmpty()) apresCopy(lang, "personal") else g.name, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
            Text(if (g.wind.isEmpty()) apresCopy(lang, "personalSub") else apresCopy(lang, "windSub", g.name), color = Muted, fontSize = 12.sp)
            g.notes.forEachIndexed { i, n ->
                Row(Modifier.fillMaxWidth().clip(RoundedCornerShape(10.dp)).background(Ink.copy(alpha = 0.05f)).padding(start = 12.dp),
                    verticalAlignment = Alignment.CenterVertically) {
                    Text(n, color = Ink, fontSize = 14.sp, modifier = Modifier.weight(1f).padding(vertical = 8.dp))
                    Box(Modifier.size(48.dp).semantics { role = Role.Button; contentDescription = apresCopy(lang, "forget") + ": " + n }
                        .clickable { onForget(g.wind, n) }.testTag("apres-x-${g.wind}-$i"), contentAlignment = Alignment.Center) {
                        Text("✕", color = Muted, fontSize = 14.sp)
                    }
                }
            }
        }
    }
}
