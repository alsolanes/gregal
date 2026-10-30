package xyz.solanes.gregal

import android.annotation.SuppressLint
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
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
 * Partits de futbol americà entre els vents (football.go al servidor). Les
 * dues franquícies amb el seu rècord, jugar-ne un o aturar-lo, la llista de
 * partits i el visor: la mateixa pàgina que a l'escriptori
 * (/app/football-viewer.html) dins d'un WebView, amb el token al fragment de
 * l'adreça perquè no viatgi mai al servidor ni quedi als logs.
 */
data class FbTeam(val name: String, val record: String, val color: String, val color2: String, val stars: String)
data class FbGame(val id: String, val status: String, val home: String, val away: String, val score: Pair<Int, Int>, val started: Long)
data class FbState(val teams: List<FbTeam>, val games: List<FbGame>, val today: Int, val max: Int, val error: String)

internal fun parseFootball(o: JSONObject): FbState {
    val t = o.optJSONArray("teams") ?: JSONArray()
    val g = o.optJSONArray("games") ?: JSONArray()
    return FbState(
        teams = (0 until t.length()).map { t.getJSONObject(it) }.map { x ->
            val ps = x.optJSONArray("players") ?: JSONArray()
            val stars = (0 until ps.length()).map { ps.getJSONObject(it) }.filter { it.optString("wind").isNotBlank() }
                .joinToString(", ") { it.optString("name") + " (" + it.optString("pos") + ")" }
            FbTeam(x.optString("name"), x.optString("record"), x.optString("color"), x.optString("color2"), stars)
        },
        games = (0 until g.length()).map { g.getJSONObject(it) }.map { x ->
            val teams = x.optJSONArray("teams") ?: JSONArray()
            val sc = x.optJSONArray("score") ?: JSONArray()
            FbGame(x.optString("id"), x.optString("status"),
                teams.optJSONObject(0)?.optString("abbr").orEmpty(), teams.optJSONObject(1)?.optString("abbr").orEmpty(),
                sc.optInt(0) to sc.optInt(1), x.optLong("started"))
        },
        today = o.optInt("today"), max = o.optInt("max_per_day", 3), error = o.optString("error"),
    )
}

internal fun hexColor(h: String, fallback: Color): Color = try { Color(android.graphics.Color.parseColor(h)) } catch (_: Exception) { fallback }

@Composable
internal fun GamesRow(lang: String, onClick: () -> Unit) {
    Surface(onClick = onClick, shape = RoundedCornerShape(16.dp), color = SuperficieAlta.copy(alpha = .5f),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp)) {
        Row(Modifier.padding(horizontal = 14.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Text("🏈", fontSize = 20.sp)
            Column(Modifier.weight(1f)) {
                Text(tr("en", "app.games.title"), color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
                Text(tr("en", "app.games.row"), color = Muted, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}

@Composable
internal fun GamesDialog(vm: GregalViewModel, lang: String, onDismiss: () -> Unit) {
    val conn by vm.conn.collectAsState()
    val scope = rememberCoroutineScope()
    var st by remember { mutableStateOf<FbState?>(null) }
    var err by remember { mutableStateOf<String?>(null) }
    var watching by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }

    suspend fun refresh() {
        try { st = withContext(Dispatchers.IO) { parseFootball(conn.api().get("/api/football")) }; err = null } catch (e: Exception) { err = e.message }
    }
    LaunchedEffect(conn) { while (true) { refresh(); delay(15_000) } }

    watching?.let { id -> GameViewer(conn, lang, id) { watching = null }; return }

    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(color = MaterialTheme.colorScheme.background, modifier = Modifier.fillMaxSize()) {
            Column(Modifier.fillMaxSize().padding(top = 12.dp)) {
                Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(tr("en", "app.games.title"), color = Ink, fontWeight = FontWeight.Bold, fontSize = 18.sp, modifier = Modifier.weight(1f))
                    IconButton(onClick = onDismiss) { Text("✕", color = Muted, fontSize = 16.sp) }
                }
                val s = st
                err?.let { Text(it, color = Muted, fontSize = 12.sp, modifier = Modifier.padding(horizontal = 16.dp)) }
                if (s != null && s.error.isNotBlank()) Text(s.error, color = Muted, fontSize = 13.sp, modifier = Modifier.padding(16.dp))
                LazyColumn(Modifier.weight(1f).fillMaxWidth(), contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    if (s != null) items(s.teams) { t ->
                        val c = hexColor(t.color, Sea)
                        Column(Modifier.fillMaxWidth().background(Brush.linearGradient(listOf(c, c.copy(alpha = .6f))), RoundedCornerShape(14.dp)).padding(14.dp)) {
                            Text(t.name, color = Color.White, fontWeight = FontWeight.Bold, fontSize = 16.sp)
                            Text(t.record, color = hexColor(t.color2, Color.White), fontWeight = FontWeight.Bold, fontSize = 13.sp)
                            if (t.stars.isNotBlank()) Text(t.stars, color = Color.White.copy(alpha = .85f), fontSize = 12.sp, lineHeight = 16.sp)
                        }
                    }
                    if (s != null) item {
                        val running = s.games.firstOrNull { it.status == "live" || it.status == "pregame" }
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                            if (running != null) Button(enabled = !busy, onClick = {
                                busy = true
                                scope.launch {
                                    try { withContext(Dispatchers.IO) { conn.api().post("/api/football/match?id=" + running.id, JSONObject().put("action", "stop")) } } catch (e: Exception) { err = e.message }
                                    busy = false; refresh()
                                }
                            }, colors = ButtonDefaults.buttonColors(containerColor = Color(0xFFC0392B))) { Text(tr("en", "app.games.stop")) }
                            else Button(enabled = !busy && s.today < s.max && s.error.isBlank(), onClick = {
                                busy = true
                                scope.launch {
                                    try {
                                        val r = withContext(Dispatchers.IO) { conn.api().post("/api/football", JSONObject().put("action", "play")) }
                                        r.optString("id").takeIf { it.isNotBlank() }?.let { watching = it }
                                    } catch (e: Exception) { err = e.message }
                                    busy = false; refresh()
                                }
                            }) { Text(tr("en", "app.games.play")) }
                            Text(tr("en", "app.games.today", s.today, s.max), color = Muted, fontSize = 12.sp)
                        }
                    }
                    if (s != null && s.games.isNotEmpty()) item {
                        Text(tr("en", "app.games.list").uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold)
                    }
                    if (s != null) items(s.games, key = { it.id }) { g ->
                        Surface(onClick = { watching = g.id }, shape = RoundedCornerShape(12.dp), color = SuperficieAlta.copy(alpha = .45f), modifier = Modifier.fillMaxWidth()) {
                            Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
                                Text("${g.home} ${g.score.first} - ${g.score.second} ${g.away}", color = Ink, fontWeight = FontWeight.Bold, fontSize = 15.sp, modifier = Modifier.weight(1f))
                                Text(tr("en", "app.games.status." + g.status), color = if (g.status == "live") Sea else Muted, fontSize = 12.sp)
                            }
                        }
                    }
                }
            }
        }
    }
}

@SuppressLint("SetJavaScriptEnabled")
@Composable
private fun GameViewer(conn: Conn, lang: String, id: String, onClose: () -> Unit) {
    Dialog(onDismissRequest = onClose, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Box(Modifier.fillMaxSize().background(Color(0xFF0B1020))) {
            AndroidView(modifier = Modifier.fillMaxSize(), factory = { ctx ->
                WebView(ctx).apply {
                    settings.javaScriptEnabled = true
                    settings.domStorageEnabled = false
                    settings.allowFileAccess = false
                    settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
                    webViewClient = object : WebViewClient() {
                        override fun shouldOverrideUrlLoading(view: WebView?, request: android.webkit.WebResourceRequest?) = true
                    }
                    setBackgroundColor(android.graphics.Color.rgb(11, 16, 32))
                    val hash = "id=" + android.net.Uri.encode(id) + "&lang=" + lang + (if (conn.token.isNotBlank()) "&token=" + android.net.Uri.encode(conn.token) else "")
                    loadUrl(baseUrlOf(conn.host, conn.port) + "/app/football-viewer.html#" + hash)
                }
            })
            IconButton(onClick = onClose, modifier = Modifier.align(Alignment.TopStart).padding(4.dp)) { Text("✕", color = Color.White, fontSize = 18.sp) }
        }
    }
}
