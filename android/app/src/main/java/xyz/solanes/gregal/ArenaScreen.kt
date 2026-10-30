package xyz.solanes.gregal

import android.annotation.SuppressLint
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
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
 * L'Arena (arena.go al servidor): la competició de prompts. Al mòbil: el teu
 * .md per a cada joc (editar-lo i desar-lo), partit de prova contra un bot,
 * el rànquing, els partits per mirar i la retransmissió en directe. Els
 * visors són les pàgines del servidor (/app/arena-viewer.html i
 * /app/arena-stream.html) dins d'un WebView, amb el token al fragment.
 */
data class ArenaGame(val id: String, val title: String, val starter: String)
data class ArenaRank(val display: String, val elo: Int, val record: String, val me: Boolean)
data class ArenaMatchRow(val id: String, val game: String, val title: String, val sub: String)
data class ArenaState(
    val me: String, val games: List<ArenaGame>, val mine: Map<String, Pair<String, String>>,
    val bots: Map<String, List<Pair<String, String>>>, val board: Map<String, List<ArenaRank>>,
    val matches: List<ArenaMatchRow>, val practiceLeft: Int,
)

internal fun parseArena(o: JSONObject): ArenaState {
    val me = o.optString("me")
    val g = o.optJSONArray("games") ?: JSONArray()
    val games = (0 until g.length()).map { g.getJSONObject(it) }.map { ArenaGame(it.optString("id"), it.optString("title"), it.optString("starter")) }
    val mineO = o.optJSONObject("mine") ?: JSONObject()
    val mine = mineO.keys().asSequence().associateWith { k -> mineO.getJSONObject(k).let { it.optString("md") to it.optString("display") } }
    val botsO = o.optJSONObject("bots") ?: JSONObject()
    val bots = botsO.keys().asSequence().associateWith { k ->
        val a = botsO.optJSONArray(k) ?: JSONArray()
        (0 until a.length()).map { a.getJSONObject(it) }.map { it.optString("owner") to it.optString("display") }
    }
    val boardO = o.optJSONObject("leaderboard") ?: JSONObject()
    val board = boardO.keys().asSequence().associateWith { k ->
        val a = boardO.optJSONArray(k) ?: JSONArray()
        (0 until a.length()).map { a.getJSONObject(it) }.map {
            ArenaRank(it.optString("display"), it.optDouble("elo").toInt(), "${it.optInt("won")}-${it.optInt("lost")}", it.optString("owner") == me)
        }
    }
    val m = o.optJSONArray("matches") ?: JSONArray()
    val matches = (0 until m.length()).map { m.getJSONObject(it) }.map {
        val names = it.optJSONArray("names") ?: JSONArray()
        val sc = it.optJSONArray("score") ?: JSONArray()
        ArenaMatchRow(it.optString("id"), it.optString("game"),
            "${names.optString(0)} ${sc.optInt(0)} - ${sc.optInt(1)} ${names.optString(1)}",
            it.optString("kind") + " · " + it.optString("status"))
    }
    return ArenaState(me, games, mine, bots, board, matches, o.optInt("practice_left"))
}

@Composable
internal fun ArenaRow(lang: String, onClick: () -> Unit) {
    Surface(onClick = onClick, shape = RoundedCornerShape(16.dp), color = SuperficieAlta.copy(alpha = .5f),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp)) {
        Row(Modifier.padding(horizontal = 14.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Text("🏆", fontSize = 20.sp)
            Column(Modifier.weight(1f)) {
                Text(tr(lang, "app.arena.title"), color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
                Text(tr(lang, "app.arena.row"), color = Muted, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}

@Composable
internal fun ArenaDialog(vm: GregalViewModel, lang: String, onDismiss: () -> Unit) {
    val conn by vm.conn.collectAsState()
    val scope = rememberCoroutineScope()
    var st by remember { mutableStateOf<ArenaState?>(null) }
    var game by remember { mutableStateOf("nfl") }
    var md by remember { mutableStateOf("") }
    var display by remember { mutableStateOf("") }
    var loadedFor by remember { mutableStateOf("") }
    var msg by remember { mutableStateOf<String?>(null) }
    var page by remember { mutableStateOf<String?>(null) } // pàgina del visor oberta

    suspend fun refresh() {
        try {
            val s = withContext(Dispatchers.IO) { parseArena(conn.api().get("/api/arena")) }
            st = s
            if (loadedFor != game) {
                md = s.mine[game]?.first.orEmpty(); display = s.mine[game]?.second ?: s.me; loadedFor = game
            }
        } catch (e: Exception) { msg = e.message }
    }
    LaunchedEffect(conn, game) { refresh() }
    LaunchedEffect(conn) { while (true) { delay(20_000); refresh() } }
    fun act(body: JSONObject, then: (JSONObject) -> Unit = {}) = scope.launch {
        try { val r = withContext(Dispatchers.IO) { conn.api().post("/api/arena", body) }; then(r); msg = null } catch (e: Exception) { msg = e.message }
        refresh()
    }

    page?.let { p -> ArenaWeb(conn, lang, p) { page = null }; return }

    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(color = MaterialTheme.colorScheme.background, modifier = Modifier.fillMaxSize()) {
            Column(Modifier.fillMaxSize().padding(top = 12.dp)) {
                Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(tr(lang, "app.arena.title"), color = Ink, fontWeight = FontWeight.Bold, fontSize = 18.sp, modifier = Modifier.weight(1f))
                    TextButton(onClick = { page = "stream" }) { Text(tr(lang, "app.arena.stream")) }
                    IconButton(onClick = onDismiss) { Text("✕", color = Muted, fontSize = 16.sp) }
                }
                val s = st
                Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).padding(horizontal = 12.dp), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    s?.games?.forEach { g -> FilterChip(selected = g.id == game, onClick = { game = g.id }, label = { Text(g.title) }) }
                }
                msg?.let { Text(it, color = Muted, fontSize = 12.sp, modifier = Modifier.padding(horizontal = 16.dp)) }
                LazyColumn(Modifier.weight(1f).fillMaxWidth(), contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    item {
                        Text(tr(lang, "app.arena.mine"), color = Ink, fontWeight = FontWeight.SemiBold)
                        OutlinedTextField(value = display, onValueChange = { display = it.take(40) }, label = { Text(tr(lang, "app.arena.display")) }, singleLine = true, modifier = Modifier.fillMaxWidth())
                        OutlinedTextField(value = md, onValueChange = { md = it.take(12000) }, modifier = Modifier.fillMaxWidth().heightIn(min = 220.dp),
                            textStyle = LocalTextStyle.current.copy(fontFamily = FontFamily.Monospace, fontSize = 12.sp), placeholder = { Text("# …") })
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                            TextButton(onClick = { s?.games?.firstOrNull { it.id == game }?.let { md = it.starter } }) { Text(tr(lang, "app.arena.starter")) }
                            Spacer(Modifier.weight(1f))
                            Button(onClick = { act(JSONObject().put("action", "save").put("game", game).put("md", md).put("display", display)) { msg = tr(lang, "app.arena.saved") } }) { Text(tr(lang, "app.arena.save")) }
                        }
                    }
                    if (s != null) item {
                        val bots = s.bots[game].orEmpty()
                        if (bots.isNotEmpty()) {
                            Text(tr(lang, "app.arena.practice", s.practiceLeft), color = Muted, fontSize = 12.sp)
                            Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                                bots.forEach { (owner, name) ->
                                    OutlinedButton(onClick = { act(JSONObject().put("action", "practice").put("game", game).put("opponent", owner)) { r -> r.optString("id").takeIf { it.isNotBlank() }?.let { page = "match:$it" } } }) { Text(name) }
                                }
                            }
                        }
                    }
                    if (s != null) item { Text(tr(lang, "app.arena.ranking").uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold) }
                    if (s != null) items(s.board[game].orEmpty()) { r ->
                        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                            Text(r.display, color = if (r.me) Sea else Ink, fontWeight = if (r.me) FontWeight.Bold else FontWeight.Normal, modifier = Modifier.weight(1f))
                            Text(r.record, color = Muted, fontSize = 12.sp, modifier = Modifier.padding(end = 10.dp))
                            Text(r.elo.toString(), color = Ink, fontWeight = FontWeight.Bold)
                        }
                    }
                    if (s != null) item { Text(tr(lang, "app.arena.matches").uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold) }
                    if (s != null) items(s.matches.filter { it.game == game }, key = { it.id }) { m ->
                        Surface(onClick = { page = "match:" + m.id }, shape = RoundedCornerShape(12.dp), color = SuperficieAlta.copy(alpha = .45f), modifier = Modifier.fillMaxWidth()) {
                            Column(Modifier.padding(12.dp)) {
                                Text(m.title, color = Ink, fontWeight = FontWeight.SemiBold)
                                Text(m.sub, color = Muted, fontSize = 12.sp)
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
private fun ArenaWeb(conn: Conn, lang: String, page: String, onClose: () -> Unit) {
    Dialog(onDismissRequest = onClose, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Box(Modifier.fillMaxSize().background(Color(0xFF070A14))) {
            AndroidView(modifier = Modifier.fillMaxSize(), factory = { ctx ->
                WebView(ctx).apply {
                    settings.javaScriptEnabled = true
                    settings.domStorageEnabled = false
                    settings.allowFileAccess = false
                    settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
                    webViewClient = object : WebViewClient() {
                        override fun shouldOverrideUrlLoading(view: WebView?, request: android.webkit.WebResourceRequest?) = true
                    }
                    setBackgroundColor(android.graphics.Color.rgb(7, 10, 20))
                    val token = if (conn.token.isNotBlank()) "&token=" + android.net.Uri.encode(conn.token) else ""
                    val url = if (page.startsWith("match:")) "/app/arena-viewer.html#id=" + android.net.Uri.encode(page.removePrefix("match:")) + "&lang=" + lang + token
                    else "/app/arena-stream.html#lang=$lang$token"
                    loadUrl(baseUrlOf(conn.host, conn.port) + url)
                }
            })
            IconButton(onClick = onClose, modifier = Modifier.align(Alignment.TopStart).padding(4.dp)) { Text("✕", color = Color.White, fontSize = 18.sp) }
        }
    }
}
