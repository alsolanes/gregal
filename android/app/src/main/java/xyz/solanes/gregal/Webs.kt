package xyz.solanes.gregal

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Menu
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import org.json.JSONObject

/**
 * «Webs i apps»: tot el que els vents han construït i serveixen, en un lloc. Al
 * mòbil, amb el servidor al darrere, és el que més es fa: demanar una web o un
 * APK, i després veure'n l'enllaç, si segueix en marxa i poder-lo obrir, copiar,
 * reiniciar o treure. Té tres coses: pàgines publicades (/share/…), apps que
 * corren al servidor (/apps/…) i túnels de Cloudflare.
 */

internal data class WebItem(
    val kind: String,        // "site" | "app" | "tunnel"
    val id: String,
    val name: String,
    val url: String,         // absoluta, o "" si encara no n'hi ha
    val status: String,      // running | starting | restarting | crashed | stopped | published | open…
    val detail: String = "",
    val hasApk: Boolean = false,
)

internal data class WebsState(val items: List<WebItem> = emptyList(), val loaded: Boolean = false, val error: String = "")

private fun absUrl(base: String, u: String): String {
    val v = u.trim()
    return when {
        v.isEmpty() -> ""
        v.startsWith("http://") || v.startsWith("https://") -> v
        v.startsWith("/") -> base.trimEnd('/') + v
        else -> v
    }
}

/** Les tres respostes del servidor, plegades en una sola llista (cap d'elles és obligatòria). */
internal fun parseWebs(base: String, shares: JSONObject?, apps: JSONObject?, powers: JSONObject?): List<WebItem> {
    val out = mutableListOf<WebItem>()
    shares?.optJSONArray("shares")?.let { a ->
        for (i in 0 until a.length()) {
            val x = a.optJSONObject(i) ?: continue
            if (x.optString("kind") != "site") continue
            val id = x.optString("id")
            out += WebItem("site", id, x.optString("title").ifBlank { id.take(8) },
                absUrl(base, x.optString("url").ifBlank { "/share/$id/" }), "published",
                detail = x.optInt("files").let { if (it > 0) "$it" else "" })
        }
    }
    apps?.optJSONArray("apps")?.let { a ->
        for (i in 0 until a.length()) {
            val x = a.optJSONObject(i) ?: continue
            out += WebItem("app", x.optString("id"), x.optString("name").ifBlank { x.optString("id") },
                absUrl(base, x.optString("url").ifBlank { x.optString("path") }), x.optString("status").ifBlank { "stopped" },
                detail = x.optString("command"))
        }
    }
    powers?.optJSONArray("tunnels")?.let { a ->
        for (i in 0 until a.length()) {
            val x = a.optJSONObject(i) ?: continue
            val url = x.optString("url")
            out += WebItem("tunnel", x.optString("name").ifBlank { url }, x.optString("name").ifBlank { url.removePrefix("https://") },
                url, x.optString("status").ifBlank { "open" }, detail = x.optString("target"))
        }
    }
    return out
}

internal fun webCopy(lang: String, key: String): String {
    val en = lang == "en"
    return when (key) {
        "title" -> if (en) "Sites and apps" else "Webs i apps"
        "sites" -> if (en) "Published" else "Publicat"
        "apps" -> if (en) "Running on the server" else "En marxa al servidor"
        "tunnels" -> if (en) "Cloudflare tunnels" else "Túnels de Cloudflare"
        "open" -> if (en) "Open" else "Obre"
        "copy" -> if (en) "Copy link" else "Copia l’enllaç"
        "copied" -> if (en) "Link copied" else "Enllaç copiat"
        "unpublish" -> if (en) "Unpublish" else "Despublica"
        "restart" -> if (en) "Restart" else "Reinicia"
        "stop" -> if (en) "Stop" else "Atura"
        "start" -> if (en) "Start" else "Engega"
        "remove" -> if (en) "Remove" else "Treu"
        "refresh" -> if (en) "Refresh" else "Actualitza"
        "empty" -> if (en) "Nothing published yet. Ask a wind: “build me a page and publish it”, or “make me an app and send me the APK”."
        else "Encara no hi ha res publicat. Demana-ho a un vent: «fes-me una web i publica-la» o «fes-me una app i passa’m l’APK»."
        "loading" -> if (en) "Loading…" else "Carregant…"
        "running" -> if (en) "running" else "en marxa"
        "starting" -> if (en) "starting" else "arrencant"
        "restarting" -> if (en) "restarting" else "reiniciant"
        "crashed" -> if (en) "crashed" else "ha petat"
        "stopped" -> if (en) "stopped" else "aturada"
        "published" -> if (en) "published" else "publicada"
        "open" -> if (en) "open" else "oberta"
        else -> key
    }
}

private fun statusColor(status: String, ok: Color, bad: Color, mid: Color): Color = when (status) {
    "running", "published", "open" -> ok
    "crashed" -> bad
    else -> mid
}

private fun copyToClipboard(ctx: Context, text: String) {
    val cm = ctx.getSystemService(Context.CLIPBOARD_SERVICE) as? ClipboardManager ?: return
    cm.setPrimaryClip(ClipData.newPlainText("Gregal", text))
}

private fun openUrl(ctx: Context, url: String) {
    try { ctx.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) } catch (_: Exception) { }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun WebsScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val st by vm.webs.collectAsState()
    val ctx = LocalContext.current
    // Es refresca en entrar i cada 10 s mentre es mira: una app que arrenca o peta es nota.
    // En segon pla no pica: només mentre la pantalla és visible.
    LifecyclePoll(Unit) { while (true) { vm.loadWebs(); delay(10_000) } }
    Scaffold(containerColor = Superficie, topBar = {
        TopAppBar(
            title = { Text(webCopy(lang, "title"), fontSize = 20.sp, fontWeight = FontWeight.SemiBold, color = Ink) },
            navigationIcon = { IconButton(onClick = onMenu) { Icon(Icons.Rounded.Menu, tr(lang, "app.nav.menu"), tint = Ink) } },
            actions = { IconButton(onClick = { vm.loadWebs() }) { Icon(Icons.Rounded.Refresh, webCopy(lang, "refresh"), tint = Ink) } },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = Superficie),
        )
    }) { pad ->
        Column(Modifier.fillMaxSize().padding(pad)) {
            if (!st.loaded) Text(webCopy(lang, "loading"), color = Muted, modifier = Modifier.padding(20.dp))
            else if (st.items.isEmpty()) Text(webCopy(lang, "empty"), color = Muted, fontSize = 14.sp, lineHeight = 20.sp,
                modifier = Modifier.padding(24.dp).testTag("webs-empty"))
            else LazyColumn(Modifier.fillMaxSize().testTag("webs-list"), contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 110.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp)) {
                for ((kind, title) in listOf("site" to "sites", "app" to "apps", "tunnel" to "tunnels")) {
                    val group = st.items.filter { it.kind == kind }
                    if (group.isEmpty()) continue
                    item(key = "h-$kind") {
                        Text(webCopy(lang, title).uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold,
                            letterSpacing = 0.9.sp, modifier = Modifier.padding(start = 4.dp, top = 14.dp))
                    }
                    items(group, key = { it.kind + ":" + it.id }) { w -> WebRow(lang, w, ctx, vm) }
                }
            }
        }
    }
}

@Composable
private fun WebRow(lang: String, w: WebItem, ctx: Context, vm: GregalViewModel) {
    val ok = Green; val bad = Red; val mid = Sand
    Column(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(20.dp)).background(SuperficieAlta.copy(alpha = .5f))
            .border(1.dp, Line, RoundedCornerShape(20.dp)).padding(14.dp).testTag("web-" + w.kind),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Box(Modifier.size(9.dp).clip(CircleShape).background(statusColor(w.status, ok, bad, mid)))
            Text(w.name, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f))
            Text(webCopy(lang, w.status), color = Muted, fontSize = 12.sp)
        }
        if (w.url.isNotBlank()) Text(w.url, color = Sea, fontSize = 12.5.sp, maxLines = 2, overflow = TextOverflow.Ellipsis,
            modifier = Modifier.clickable { openUrl(ctx, w.url) })
        if (w.detail.isNotBlank() && w.kind != "site") Text(w.detail, color = Muted, fontSize = 11.5.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
        Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            if (w.url.isNotBlank()) {
                WebAction(webCopy(lang, "open")) { openUrl(ctx, w.url) }
                WebAction(webCopy(lang, "copy")) { copyToClipboard(ctx, w.url); vm.notify(webCopy(lang, "copied")) }
            }
            when (w.kind) {
                "site" -> WebAction(webCopy(lang, "unpublish")) { vm.webAction(w, "unpublish") }
                "app" -> {
                    if (w.status == "running" || w.status == "starting" || w.status == "restarting") {
                        WebAction(webCopy(lang, "restart")) { vm.webAction(w, "restart") }
                        WebAction(webCopy(lang, "stop")) { vm.webAction(w, "stop") }
                    } else WebAction(webCopy(lang, "restart")) { vm.webAction(w, "restart") }
                    WebAction(webCopy(lang, "remove")) { vm.webAction(w, "remove") }
                }
            }
        }
    }
}

@Composable
private fun WebAction(label: String, onClick: () -> Unit) {
    Text(label, color = Ink, fontSize = 12.5.sp, fontWeight = FontWeight.Medium,
        modifier = Modifier.clip(RoundedCornerShape(10.dp)).background(Ink.copy(alpha = .07f)).clickable(onClick = onClick)
            .padding(horizontal = 12.dp, vertical = 7.dp))
}

