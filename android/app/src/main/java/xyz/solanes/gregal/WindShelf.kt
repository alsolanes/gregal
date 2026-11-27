package xyz.solanes.gregal

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.slideInVertically
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.time.OffsetDateTime
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

// El prestatge d'un vent (GET /api/winds/outputs): el que ha fet al seu xat i
// es pot tornar a obrir. Pestanya del perfil, com a la web (vents-shelf.js).

/** Una peça del prestatge: kind és doc, deck, sheet, site, link o file. */
data class WindOutput(val at: String, val kind: String, val title: String, val url: String)

fun parseWindOutputs(raw: String): List<WindOutput> {
    val arr = JSONObject(raw).optJSONArray("items") ?: JSONArray()
    return (0 until arr.length()).mapNotNull { i ->
        val x = arr.optJSONObject(i) ?: return@mapNotNull null
        WindOutput(x.optString("at"), x.optString("kind"), x.optString("title"), x.optString("url"))
    }
}

fun GregalApi.getWindOutputs(wind: String): List<WindOutput> =
    parseWindOutputs(get("/api/winds/outputs?wind=" + java.net.URLEncoder.encode(wind, "UTF-8")).toString())

private val docExport = Regex("^/api/docs/([^/?]+)/export\\?")

/** El document viu d'una peça (s'obre a la pantalla de Docs), o null si és un enllaç o un fitxer. */
fun windOutputDoc(o: WindOutput): DocRef? =
    docExport.find(o.url)?.let { DocRef(it.groupValues[1], o.title, o.kind, "mine") }

private val shelfIcon = mapOf("doc" to "📄", "deck" to "📽️", "sheet" to "📊", "site" to "🌐", "link" to "🔗", "file" to "📎")

/** Les targetes del prestatge; onShare només per a webs i enllaços, que s'obren sense la clau de l'app. */
@Composable
fun WindShelfList(lang: String, items: List<WindOutput>, onOpen: (WindOutput) -> Unit, onShare: (WindOutput) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        if (items.isEmpty()) {
            Surface(color = Ink.copy(alpha = 0.04f), shape = MaterialTheme.shapes.medium) {
                Text(tr(lang, "shelf.empty"), color = Tinta, fontSize = 14.sp, lineHeight = 20.sp, modifier = Modifier.padding(16.dp))
            }
            return@Column
        }
        val motion = LocalWindMotion.current
        items.forEachIndexed { i, o ->
            key(o.kind + o.url) {
                // Les targetes entren una darrere l'altra (sense moviment si la persona no en vol).
                var shown by remember { mutableStateOf(!motion) }
                LaunchedEffect(Unit) { kotlinx.coroutines.delay(minOf(i, 8) * 40L); shown = true }
                AnimatedVisibility(shown, enter = fadeIn() + slideInVertically { it / 4 }) {
                    Surface(color = Ink.copy(alpha = 0.04f), shape = MaterialTheme.shapes.medium,
                        modifier = Modifier.fillMaxWidth().clickable { onOpen(o) }) {
                        Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            Text(shelfIcon[o.kind] ?: "•", fontSize = 18.sp)
                            Column(Modifier.weight(1f)) {
                                Text(o.title, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
                                Text(runCatching { savedDay(lang, OffsetDateTime.parse(o.at).toEpochSecond()) + " " + windActivityTime(o.at) }.getOrDefault(""), color = Muted, fontSize = 12.sp, maxLines = 1)
                            }
                            if (o.kind == "site" || o.kind == "link") TextButton(onClick = { onShare(o) }) { Text(tr(lang, "shelf.share")) }
                        }
                    }
                }
            }
        }
    }
}

/** La pestanya del perfil: carrega el prestatge i obre cada peça on toca. */
@Composable
internal fun WindShelfSection(vm: GregalViewModel, lang: String, wind: String, onLeave: () -> Unit) {
    var items by remember(wind) { mutableStateOf<List<WindOutput>?>(null) }
    var error by remember(wind) { mutableStateOf<String?>(null) }
    val conn = vm.conn.value
    val uri = LocalUriHandler.current
    val ctx = LocalContext.current
    LaunchedEffect(wind) {
        try { items = withContext(Dispatchers.IO) { conn.api().getWindOutputs(wind) } }
        catch (e: Exception) { error = e.message.orEmpty() }
    }
    val base = baseUrlOf(conn.host, conn.port).trimEnd('/')
    val full = { o: WindOutput -> if (o.url.startsWith("/") && o.kind != "file") base + o.url else o.url }
    when {
        error != null -> Text(tr(lang, "shelf.loadError", error), color = MaterialTheme.colorScheme.error)
        items == null -> LinearProgressIndicator(Modifier.fillMaxWidth())
        else -> WindShelfList(lang, items.orEmpty(),
            onOpen = { o ->
                val doc = windOutputDoc(o)
                if (doc != null) { onLeave(); vm.openDoc(doc) }
                // Els fitxers els baixa l'app amb la seva clau (MainActivity, isServerDocLink).
                else runCatching { uri.openUri(full(o)) }.onFailure { vm.notify(it.message.orEmpty()) }
            },
            onShare = { o -> shareTextIntent(ctx, full(o), tr(lang, "shelf.share")) })
    }
}
