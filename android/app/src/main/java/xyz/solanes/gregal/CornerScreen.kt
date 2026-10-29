package xyz.solanes.gregal

import android.annotation.SuppressLint
import android.webkit.JavascriptInterface
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

/**
 * El Racó dels vents (corner.go al servidor): una pàgina que fan ells sols.
 * Aquí es mostra en un WebView tancat (la pàgina porta una CSP que no deixa
 * connectar enlloc) amb un sol pont, GregalAndroid.send: «hello» → li passem
 * les dades dels vents; «ask» → la pregunta d'un visitant va a un vent i la
 * resposta torna a la pàgina. També: cridar-los o aturar-los, pausa, diari,
 * versions i comentaris per a la propera reunió.
 */
data class CornerEntry(val at: Long, val kind: String, val name: String, val text: String)
data class CornerVersion(val n: Int, val at: Long, val builder: String, val reviewer: String, val summary: String, val revertedFrom: Int)
data class CornerState(
    val paused: Boolean, val running: Boolean, val current: Int,
    val journal: List<CornerEntry>, val versions: List<CornerVersion>,
    val calls: Int, val maxCalls: Int, val sessions: Int, val maxSessions: Int,
)

internal fun parseCorner(o: JSONObject): CornerState {
    val j = o.optJSONArray("journal") ?: JSONArray()
    val v = o.optJSONArray("versions") ?: JSONArray()
    val b = o.optJSONObject("budget") ?: JSONObject()
    return CornerState(
        paused = o.optBoolean("paused"), running = o.optBoolean("running"), current = o.optInt("current"),
        journal = (0 until j.length()).map { j.getJSONObject(it) }.map { CornerEntry(it.optLong("at"), it.optString("kind"), it.optString("name"), it.optString("text")) },
        versions = (0 until v.length()).map { v.getJSONObject(it) }.map {
            CornerVersion(it.optInt("n"), it.optLong("at"), it.optString("builder_name"), it.optString("reviewer_name"), it.optString("summary"), it.optInt("reverted_from"))
        },
        calls = b.optInt("calls"), maxCalls = b.optInt("max_calls"), sessions = b.optInt("sessions"), maxSessions = b.optInt("max_sessions"),
    )
}

internal fun cornerIcon(kind: String) = when (kind) {
    "meeting" -> "🗣"; "rest" -> "☕"; "built" -> "🛠"; "review" -> "✅"; "published" -> "✨"
    "rejected" -> "↩"; "failed" -> "⚠"; "revert" -> "⏪"; "comment" -> "💬"; "talk" -> "👋"; "stopped" -> "⏹"
    else -> "·"
}

/** La fila de la portada de Vents que obre el Racó. */
@Composable
internal fun CornerRow(lang: String, onClick: () -> Unit) {
    Surface(onClick = onClick, shape = RoundedCornerShape(16.dp), color = SuperficieAlta.copy(alpha = .5f),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp)) {
        Row(Modifier.padding(horizontal = 14.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Text("✦", color = Sea, fontSize = 20.sp)
            Column(Modifier.weight(1f)) {
                Text(tr(lang, "app.corner.title"), color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
                Text(tr(lang, "app.corner.row"), color = Muted, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}

@SuppressLint("SetJavaScriptEnabled", "JavascriptInterface")
@Composable
internal fun CornerDialog(vm: GregalViewModel, lang: String, onDismiss: () -> Unit) {
    val conn by vm.conn.collectAsState()
    val scope = rememberCoroutineScope()
    var st by remember { mutableStateOf<CornerState?>(null) }
    var doc by remember { mutableStateOf<String?>(null) }
    var data by remember { mutableStateOf("{}") }
    var shownVersion by remember { mutableStateOf(-1) }
    var err by remember { mutableStateOf<String?>(null) }
    var journalOpen by remember { mutableStateOf(false) }
    var comment by remember { mutableStateOf("") }
    var webView by remember { mutableStateOf<WebView?>(null) }
    var reload by remember { mutableStateOf(0) }

    suspend fun refresh() {
        try {
            val api = conn.api()
            val s = withContext(Dispatchers.IO) { parseCorner(api.get("/api/corner")) }
            st = s
            if (s.current != shownVersion) {
                val page = withContext(Dispatchers.IO) { api.get("/api/corner/page") }
                doc = page.optString("doc").ifBlank { null }
                data = (page.optJSONObject("data") ?: JSONObject()).toString()
                shownVersion = s.current
            }
            err = null
        } catch (e: Exception) { err = e.message }
    }
    fun act(payload: JSONObject) = scope.launch {
        try { withContext(Dispatchers.IO) { conn.api().post("/api/corner", payload) } } catch (e: Exception) { err = e.message }
        delay(500); refresh()
    }
    LaunchedEffect(conn, reload) {
        while (true) {
            refresh()
            delay(if (st?.running == true) 6_000 else 30_000)
        }
    }

    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(color = MaterialTheme.colorScheme.background, modifier = Modifier.fillMaxSize()) {
            Column(Modifier.fillMaxSize().padding(top = 12.dp)) {
                Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text(tr(lang, "app.corner.title"), color = Ink, fontWeight = FontWeight.Bold, fontSize = 18.sp)
                        val s = st
                        val status = when {
                            s == null -> ""
                            s.running -> tr(lang, "app.corner.working")
                            s.paused -> tr(lang, if (s.current > 0 || s.journal.isNotEmpty()) "app.corner.paused" else "app.corner.off")
                            else -> s.versions.firstOrNull { it.n == s.current }?.let { v ->
                                if (v.revertedFrom > 0) tr(lang, "app.corner.reverted", v.n, v.revertedFrom)
                                else tr(lang, "app.corner.versionBy", v.n, v.builder, v.reviewer)
                            } ?: ""
                        }
                        if (status.isNotBlank()) Text(status, color = Muted, fontSize = 12.sp, maxLines = 2)
                    }
                    TextButton(onClick = { journalOpen = !journalOpen }) { Text(tr(lang, if (journalOpen) "app.corner.page" else "app.corner.journal")) }
                    IconButton(onClick = onDismiss) { Text("✕", color = Muted, fontSize = 16.sp) }
                }
                st?.let { s ->
                    Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp), verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        if (s.running) Button(onClick = { act(JSONObject().put("action", "stop")) },
                            colors = ButtonDefaults.buttonColors(containerColor = Color(0xFFC0392B))) { Text(tr(lang, "app.corner.stop")) }
                        else OutlinedButton(onClick = { act(JSONObject().put("action", "meet")) }) { Text(tr(lang, "app.corner.meet")) }
                        if (s.paused) Button(onClick = { act(JSONObject().put("action", "resume")) }) { Text(tr(lang, "app.corner.resume")) }
                        else OutlinedButton(onClick = { act(JSONObject().put("action", "pause")) }) { Text(tr(lang, "app.corner.pause")) }
                        Spacer(Modifier.weight(1f))
                        Text(tr(lang, "app.corner.spend", s.calls, s.maxCalls), color = Muted, fontSize = 11.sp)
                    }
                }
                err?.let { Text(it, color = Muted, fontSize = 12.sp, modifier = Modifier.padding(horizontal = 16.dp)) }
                Box(Modifier.weight(1f).fillMaxWidth()) {
                    if (journalOpen) CornerJournal(lang, st, onRevert = { n -> act(JSONObject().put("action", "revert").put("version", n)) })
                    else if (doc == null) Text(tr(lang, "app.corner.blank"), color = Muted, fontSize = 14.sp, lineHeight = 20.sp,
                        modifier = Modifier.align(Alignment.Center).padding(32.dp))
                    else key(shownVersion) {
                        AndroidView(modifier = Modifier.fillMaxSize(), factory = { ctx ->
                            WebView(ctx).apply {
                                settings.javaScriptEnabled = true
                                settings.allowFileAccess = false
                                settings.allowContentAccess = false
                                settings.domStorageEnabled = false
                                settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
                                webViewClient = object : WebViewClient() {
                                    // La pàgina no pot portar enlloc: cap navegació fora d'ella mateixa.
                                    override fun shouldOverrideUrlLoading(view: WebView?, request: android.webkit.WebResourceRequest?) = true
                                }
                                setBackgroundColor(android.graphics.Color.WHITE)
                                val wv = this
                                fun post(msg: JSONObject) = wv.post { wv.evaluateJavascript("window.postMessage(" + msg + ", '*')", null) }
                                addJavascriptInterface(object {
                                    @JavascriptInterface
                                    fun send(json: String) {
                                        val m = try { JSONObject(json) } catch (_: Exception) { return }
                                        when (m.optString("gregalCorner")) {
                                            "hello" -> post(JSONObject().put("gregalCorner", "init").put("data", JSONObject(data)))
                                            "ask" -> scope.launch {
                                                val reply = JSONObject().put("gregalCorner", "reply").put("id", m.optInt("id"))
                                                try {
                                                    val r = withContext(Dispatchers.IO) {
                                                        conn.api().post("/api/corner/ask", JSONObject().put("wind", m.optString("wind")).put("text", m.optString("text")))
                                                    }
                                                    reply.put("reply", r.optString("reply"))
                                                } catch (e: Exception) { reply.put("error", e.message ?: "error") }
                                                post(reply)
                                            }
                                        }
                                    }
                                }, "GregalAndroid")
                                loadDataWithBaseURL("about:blank", doc.orEmpty(), "text/html", "utf-8", null)
                                webView = this
                            }
                        })
                    }
                }
                Row(Modifier.fillMaxWidth().padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
                    OutlinedTextField(value = comment, onValueChange = { comment = it.take(1000) }, modifier = Modifier.weight(1f),
                        placeholder = { Text(tr(lang, "app.corner.commentHint"), fontSize = 13.sp) }, maxLines = 3)
                    TextButton(enabled = comment.isNotBlank(), onClick = {
                        val text = comment.trim(); comment = ""
                        act(JSONObject().put("action", "comment").put("text", text))
                    }) { Text(tr(lang, "app.corner.send")) }
                }
            }
        }
    }
}

@Composable
private fun CornerJournal(lang: String, st: CornerState?, onRevert: (Int) -> Unit) {
    val s = st ?: return
    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp)) {
        if (s.versions.isNotEmpty()) item {
            Text(tr(lang, "app.corner.versions").uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(vertical = 6.dp))
        }
        items(s.versions, key = { "v" + it.n }) { v ->
            Row(Modifier.fillMaxWidth().padding(vertical = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text("v${v.n} · " + v.summary, color = Ink, fontSize = 13.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
                    if (v.builder.isNotBlank()) Text(v.builder + " · " + v.reviewer, color = Muted, fontSize = 11.sp)
                }
                if (v.n != s.current) TextButton(onClick = { onRevert(v.n) }) { Text(tr(lang, "app.corner.back")) }
            }
        }
        item {
            Text(tr(lang, "app.corner.journal").uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(top = 12.dp, bottom = 6.dp))
        }
        if (s.journal.isEmpty()) item { Text(tr(lang, "app.corner.nothing"), color = Muted, fontSize = 13.sp) }
        items(s.journal) { e ->
            Row(Modifier.fillMaxWidth().padding(vertical = 4.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(cornerIcon(e.kind), fontSize = 14.sp)
                Text((if (e.name.isNotBlank()) e.name + ": " else "") + e.text, color = Ink, fontSize = 13.sp, lineHeight = 18.sp)
            }
        }
    }
}
