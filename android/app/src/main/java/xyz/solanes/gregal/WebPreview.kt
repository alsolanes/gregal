package xyz.solanes.gregal

import android.annotation.SuppressLint
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import org.json.JSONObject

/*
 * Vista prèvia al xat, com els artefactes de Claude o el canvas de Grok:
 *  - la web que el vent ha fet (```gregal-web, winds_webcard.go): l'adreça
 *    /p/<clau>/… del servidor, en un WebView;
 *  - els blocs ```html i ```svg: el document aïllat (sense xarxa), com
 *    md-preview.js a l'escriptori.
 * El WebView no té la sessió de l'app (ni galetes ni clau): una pàgina no pot
 * parlar amb l'API en nom teu.
 */

data class WebSpec(val title: String, val path: String, val url: String)

private val webUrlRe = Regex("""^/p/[a-f0-9]{8,}/[^\s"'<>]+$""")

/** Mateixa regla que md.js: només adreces /p/<clau>/ del servidor. */
fun parseWebSpec(json: String): WebSpec? = try {
    val o = JSONObject(json)
    val url = o.optString("url").trim()
    if (!webUrlRe.matches(url)) null
    else WebSpec(o.optString("title").trim().ifBlank { o.optString("path").trim().ifBlank { "web" } }, o.optString("path").trim(), url)
} catch (_: Exception) { null }

object WebPreviewLogic {
    private const val CSP = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; media-src data: blob:; connect-src 'none'; form-action 'none'; base-uri 'none'; frame-src 'none'"
    const val MAX = 200_000
    private val svgWhole = Regex("""<svg\b[\s\S]*</svg>\s*$""", RegexOption.IGNORE_CASE)
    private val anyTag = Regex("""<[a-z][\s\S]*>""", RegexOption.IGNORE_CASE)
    private val headTag = Regex("<head([^>]*)>", RegexOption.IGNORE_CASE)
    private val htmlTag = Regex("<html([^>]*)>", RegexOption.IGNORE_CASE)

    /** "html" | "svg" | "": mateixa regla que md-preview.js. */
    fun kind(lang: String, code: String): String {
        val l = lang.trim().lowercase()
        val c = code.trim()
        if (c.isEmpty() || c.length > MAX) return ""
        if (l == "svg" || (l == "xml" && c.startsWith("<svg", ignoreCase = true))) return if (svgWhole.containsMatchIn(c)) "svg" else ""
        if (l == "html" || l == "htm") return if (anyTag.containsMatchIn(c)) "html" else ""
        return ""
    }

    /** El document aïllat: la CSP va la primera dins del <head>. */
    fun srcdoc(kind: String, code: String): String {
        val meta = "<meta http-equiv=\"Content-Security-Policy\" content=\"$CSP\">"
        val c = code.trim()
        if (kind == "svg") {
            return "<!doctype html><html><head>$meta<meta charset=\"utf-8\"><style>html,body{margin:0;height:100%;display:grid;place-items:center;background:#fff}svg{max-width:100%;max-height:100%}</style></head><body>$c</body></html>"
        }
        return when {
            headTag.containsMatchIn(c) -> headTag.replaceFirst(c, "<head$1>" + meta.replace("$", "\\$"))
            htmlTag.containsMatchIn(c) -> htmlTag.replaceFirst(c, "<html$1><head>" + meta.replace("$", "\\$") + "</head>")
            else -> "<!doctype html><html><head>$meta<meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"></head><body>$c</body></html>"
        }
    }

    /** La base del servidor actual (local o remot), per a les adreces /p/. */
    fun serverBase(ctx: Context): String {
        val p = Prefs(ctx)
        return if (p.target == Prefs.TARGET_LOCAL) baseUrlOf("127.0.0.1", p.localPort.toString()) else baseUrlOf(p.host, p.port)
    }
}

@SuppressLint("SetJavaScriptEnabled")
@Composable
private fun PreviewWebView(modifier: Modifier, url: String? = null, html: String? = null, allowStorage: Boolean = false) {
    AndroidView(
        modifier = modifier,
        factory = { ctx ->
            WebView(ctx).apply {
                settings.javaScriptEnabled = true
                settings.domStorageEnabled = allowStorage
                settings.allowFileAccess = false
                settings.allowContentAccess = false
                // La targeta és més estreta que la pantalla: la pàgina s'encongeix
                // fins a cabre-hi sencera en lloc de quedar tallada a la dreta.
                settings.useWideViewPort = true
                settings.loadWithOverviewMode = true
                setBackgroundColor(android.graphics.Color.WHITE)
                webViewClient = object : WebViewClient() {
                    // Un enllaç cap a fora s'obre al navegador, no dins la targeta.
                    override fun shouldOverrideUrlLoading(view: WebView, req: WebResourceRequest): Boolean {
                        val target = req.url.toString()
                        if (url != null && target.startsWith(url.substringBeforeLast('/'))) return false
                        runCatching { ctx.startActivity(Intent(Intent.ACTION_VIEW, req.url).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
                        return true
                    }

                    // El document d'un bloc html no surt a la xarxa (la CSP ja ho diu; això ho garanteix).
                    override fun shouldInterceptRequest(view: WebView, req: WebResourceRequest): WebResourceResponse? =
                        if (html != null && req.url.scheme in setOf("http", "https")) WebResourceResponse("text/plain", "utf-8", null) else null
                }
                if (url != null) loadUrl(url) else loadDataWithBaseURL(null, html.orEmpty(), "text/html", "utf-8", null)
            }
        },
    )
}

/** La web del vent: capçalera amb el nom i «Obre», i la pàgina funcionant. */
@Composable
internal fun WebCardBlock(spec: WebSpec, link: Color, lang: String) {
    val ctx = LocalContext.current
    val full = remember(spec.url) { WebPreviewLogic.serverBase(ctx).trimEnd('/') + spec.url }
    val shape = RoundedCornerShape(16.dp)
    Column(Modifier.fillMaxWidth().padding(vertical = 6.dp).clip(shape).background(Superficie).border(1.dp, Line, shape).testTag("gregal-web")) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 14.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Box(Modifier.size(30.dp).clip(RoundedCornerShape(9.dp)).background(SuperficieAlta), contentAlignment = Alignment.Center) {
                Text("◧", color = link, fontSize = 15.sp)
            }
            Column(Modifier.weight(1f)) {
                Text(spec.title, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (spec.path.isNotBlank()) Text(spec.path, color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            Text(if (lang == "en") "Open ↗" else "Obre ↗", color = link, fontSize = 13.sp, fontWeight = FontWeight.SemiBold,
                modifier = Modifier.clip(RoundedCornerShape(50)).background(SuperficieAlta)
                    .clickable { runCatching { ctx.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(full)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) } }
                    .padding(horizontal = 12.dp, vertical = 6.dp).testTag("gregal-web-open"))
        }
        PreviewWebView(Modifier.fillMaxWidth().height(420.dp), url = full, allowStorage = true)
    }
}

/** Bloc ```html / ```svg: «Vista» (funcionant, aïllat) o «Codi». */
@Composable
internal fun CodePreviewBlock(kind: String, code: String, lang: String) {
    var showCode by remember(code) { mutableStateOf(false) }
    val doc = remember(kind, code) { WebPreviewLogic.srcdoc(kind, code) }
    val shape = RoundedCornerShape(12.dp)
    Column(Modifier.fillMaxWidth().clip(shape).background(CodeDark).border(1.dp, Line, shape).testTag("md-code-preview")) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 6.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(kind, color = Muted, fontSize = 12.sp, modifier = Modifier.padding(horizontal = 6.dp))
            listOf(false to (if (lang == "en") "Preview" else "Vista"), true to (if (lang == "en") "Code" else "Codi")).forEach { (isCode, label) ->
                val on = showCode == isCode
                Text(label, color = if (on) Sea else Muted, fontSize = 12.sp, fontWeight = if (on) FontWeight.SemiBold else FontWeight.Normal,
                    modifier = Modifier.clip(RoundedCornerShape(50)).background(if (on) SuperficieAlta else Color.Transparent)
                        .clickable { showCode = isCode }.padding(horizontal = 10.dp, vertical = 4.dp))
            }
        }
        if (showCode) {
            Text(code, color = Ink, fontSize = 12.sp, fontFamily = FontFamily.Monospace, modifier = Modifier.padding(10.dp))
        } else {
            PreviewWebView(Modifier.fillMaxWidth().height(if (kind == "svg") 260.dp else 360.dp), html = doc)
        }
    }
}
