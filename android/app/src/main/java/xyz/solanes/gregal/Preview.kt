package xyz.solanes.gregal

import android.annotation.SuppressLint
import android.content.Intent
import android.net.Uri
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import org.json.JSONObject

/**
 * Què ha demanat l'eina preview de l'agent (internal/agent/preview.go):
 * html (prototip autònom), markdown (document) o url (servidor de
 * desenvolupament a localhost de la màquina del servidor).
 */
data class PreviewReq(val kind: String, val content: String, val title: String = "")

/**
 * Llegeix la sortida de l'eina preview (o els seus arguments, que són el
 * mateix JSON). Null si no és una petició vàlida: el registre durable retalla
 * les sortides llargues i llavors el JSON queda trencat.
 */
fun parsePreviewRequest(json: String?): PreviewReq? {
    if (json.isNullOrBlank()) return null
    val o = try { JSONObject(json) } catch (_: Exception) { return null }
    val kind = o.optString("kind")
    val content = o.optString("content")
    if (kind !in setOf("html", "markdown", "url") || content.isBlank() || content.length > 200_000) return null
    return PreviewReq(kind, content, o.optString("title").trim().take(200))
}

/**
 * L'adreça que el mòbil pot obrir per a una preview url. L'agent només
 * en dona de localhost, que vol dir la màquina del servidor: des d'un mòbil
 * connectat a un servidor remot, localhost és el mateix mòbil. Es canvia per
 * l'adreça del servidor (el port i el camí es queden). Amb el servidor al
 * mateix mòbil, localhost ja és bo.
 */
fun previewUrlFor(raw: String, serverHost: String, local: Boolean): String? {
    val u = try { java.net.URI(raw.trim()) } catch (_: Exception) { return null }
    if (u.scheme != "http" && u.scheme != "https" || u.rawUserInfo != null) return null
    val h = u.host?.lowercase() ?: return null
    if (h !in setOf("localhost", "127.0.0.1", "[::1]")) return null
    if (local) return u.toString()
    val server = serverHost.trim().substringAfter("://").substringBefore('/').let {
        if (it.startsWith("[")) it.substringBefore(']') + "]" else it.substringBefore(':')
    }
    if (server.isEmpty()) return u.toString()
    return u.scheme + "://" + server + (if (u.port > 0) ":${u.port}" else "") +
        (u.rawPath ?: "") + (u.rawQuery?.let { "?$it" } ?: "") + (u.rawFragment?.let { "#$it" } ?: "")
}

/** Una web publicada al servidor (POST /api/shares/site). */
data class PublishedSite(val id: String, val link: String, val public: Boolean)

/** Estat del botó Publica de la preview oberta. */
data class PublishState(val site: PublishedSite? = null, val busy: Boolean = false,
                        val error: String = "", val dialog: Boolean = false)

/**
 * L'enllaç que es comparteix: el públic del servidor (GREGAL_PUBLIC_URL)
 * si en té; si no, el camí sobre l'adreça amb què el mòbil hi arriba.
 */
fun parsePublished(json: String, base: String): PublishedSite? {
    val o = try { JSONObject(json) } catch (_: Exception) { return null }
    val id = o.optString("id")
    val path = o.optString("path")
    if (id.isBlank() || !path.startsWith("/share/")) return null
    val url = o.optString("url")
    return PublishedSite(id, url.ifBlank { base.trimEnd('/') + path }, o.optBoolean("public") && url.isNotBlank())
}

/**
 * On funciona l'enllaç: "" a tot arreu (domini públic), "phone" si el
 * servidor és el mateix mòbil, "network" si és una IP o nom de casa.
 */
fun publishedReach(site: PublishedSite, conn: Conn): String = when {
    site.public -> ""
    conn.local -> "phone"
    isPublicDomain(conn.host) -> ""
    else -> "network"
}

/** Publica (o republica amb el mateix id) la preview. Bloqueja: crida-la fora del fil principal. */
fun publishPreviewSite(conn: Conn, req: PreviewReq, id: String): PublishedSite {
    val body = JSONObject().put("kind", req.kind).put("content", req.content).put("title", req.title)
    if (id.isNotBlank()) body.put("id", id)
    val o = conn.api().post("/api/shares/site", body)
    return parsePublished(o.toString(), baseUrlOf(conn.host, conn.port)) ?: throw IllegalStateException("bad response")
}

/** El diàleg amb l'enllaç: copia, comparteix, actualitza o despublica. */
@Composable
fun PublishDialog(st: PublishState, conn: Conn, lang: String, onRepublish: () -> Unit, onUnpublish: () -> Unit, onClose: () -> Unit) {
    val ctx = LocalContext.current
    val clipboard = androidx.compose.ui.platform.LocalClipboardManager.current
    var copied by remember(st.site) { mutableStateOf(false) }
    AlertDialog(
        onDismissRequest = onClose,
        title = { Text(tr(lang, if (st.site == null) "app.publish.title" else "app.publish.done")) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                when {
                    st.busy -> Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                        Text(tr(lang, "app.publish.busy"), color = Muted)
                    }
                    st.error.isNotBlank() -> Text(tr(lang, "app.publish.error", st.error), color = Muted)
                }
                st.site?.let { site ->
                    SelectionContainerText(site.link)
                    when (publishedReach(site, conn)) {
                        "phone" -> Text(tr(lang, "app.publish.onlyPhone"), color = Muted, fontSize = 13.sp, lineHeight = 18.sp)
                        "network" -> Text(tr(lang, "app.publish.onlyNetwork"), color = Muted, fontSize = 13.sp, lineHeight = 18.sp)
                    }
                    Text(tr(lang, "app.publish.snapshot"), color = Muted, fontSize = 12.sp, lineHeight = 17.sp)
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedButton(onClick = { clipboard.setText(androidx.compose.ui.text.AnnotatedString(site.link)); copied = true }) {
                            Text(tr(lang, if (copied) "app.publish.copied" else "app.publish.copy"))
                        }
                        Button(onClick = {
                            val send = Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, site.link)
                            try { ctx.startActivity(Intent.createChooser(send, null).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) } catch (_: Exception) {}
                        }) { Text(tr(lang, "app.publish.share")) }
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        TextButton(enabled = !st.busy, onClick = onRepublish) { Text(tr(lang, "app.publish.update")) }
                        TextButton(enabled = !st.busy, onClick = onUnpublish) { Text(tr(lang, "app.publish.unpublish")) }
                    }
                }
                if (st.site == null && !st.busy) TextButton(onClick = onRepublish) { Text(tr(lang, "app.publish.retry")) }
            }
        },
        confirmButton = { TextButton(onClick = onClose) { Text(tr(lang, "app.publish.close")) } },
    )
}

@Composable
private fun SelectionContainerText(link: String) {
    androidx.compose.foundation.text.selection.SelectionContainer {
        Text(link, color = Sea, fontSize = 14.sp, fontWeight = FontWeight.Medium)
    }
}

/**
 * La preview a pantalla sencera. html es pinta tancat (sense xarxa ni
 * navegació, com el marc aïllat de la web); markdown amb el renderer de
 * l'app; url carregant la pàgina a través de l'adreça del servidor, amb
 * «Obre al navegador» per si no s'hi arriba.
 */
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun PreviewSheet(req: PreviewReq, conn: Conn, lang: String, published: PublishedSite? = null,
                 onPublish: (() -> Unit)? = null, onClose: () -> Unit) {
    val ctx = LocalContext.current
    val url = remember(req, conn) { if (req.kind == "url") previewUrlFor(req.content, conn.host, conn.local) else null }
    var failed by remember(req) { mutableStateOf(req.kind == "url" && url == null) }
    fun openExternal() {
        url ?: return
        try { ctx.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) } catch (_: Exception) {}
    }
    Dialog(onDismissRequest = onClose, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Column(Modifier.fillMaxSize().background(Superficie)) {
            Row(Modifier.fillMaxWidth().statusBarsPadding().padding(start = 4.dp, end = 8.dp, top = 4.dp, bottom = 4.dp),
                verticalAlignment = Alignment.CenterVertically) {
                IconButton(onClick = onClose) { Text("✕", color = Ink, fontSize = 18.sp) }
                Column(Modifier.weight(1f)) {
                    Text(req.title.ifBlank { tr(lang, "app.preview.title") }, color = Ink, fontSize = 16.sp,
                        fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Text(url ?: tr(lang, if (req.kind == "markdown") "app.preview.markdown" else "app.preview.html"),
                        color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                if (url != null) TextButton(onClick = { openExternal() }) { Text(tr(lang, "app.preview.openBrowser")) }
                // Una adreça de localhost no es pot copiar: és un servidor viu, no una pàgina.
                else if (onPublish != null) TextButton(onClick = onPublish) {
                    Text(tr(lang, if (published == null) "app.publish.action" else "app.publish.link"))
                }
            }
            Box(Modifier.weight(1f).fillMaxWidth()) {
                when {
                    req.kind == "markdown" -> Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp)) {
                        MdPageBody(req.content, lang)
                    }
                    failed -> Column(Modifier.align(Alignment.Center).padding(32.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(tr(lang, "app.preview.unreachable", url ?: req.content), color = Muted, fontSize = 14.sp, lineHeight = 20.sp)
                        if (url != null) {
                            Spacer(Modifier.height(12.dp))
                            Button(onClick = { openExternal() }) { Text(tr(lang, "app.preview.openBrowser")) }
                        }
                    }
                    else -> key(req) {
                        AndroidView(modifier = Modifier.fillMaxSize(), factory = { c ->
                            WebView(c).apply {
                                settings.javaScriptEnabled = true
                                settings.allowFileAccess = false
                                settings.allowContentAccess = false
                                settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
                                settings.useWideViewPort = true
                                settings.loadWithOverviewMode = true
                                setBackgroundColor(android.graphics.Color.WHITE)
                                if (url == null) {
                                    // Com el marc aïllat de la web: el prototip no surt a la xarxa.
                                    settings.blockNetworkLoads = true
                                    settings.domStorageEnabled = false
                                    webViewClient = object : WebViewClient() {
                                        override fun shouldOverrideUrlLoading(view: WebView?, request: WebResourceRequest?) = true
                                    }
                                    loadDataWithBaseURL("about:blank", req.content, "text/html", "utf-8", null)
                                } else {
                                    settings.domStorageEnabled = true
                                    webViewClient = object : WebViewClient() {
                                        override fun onReceivedError(view: WebView?, request: WebResourceRequest?, error: WebResourceError?) {
                                            if (request?.isForMainFrame == true) failed = true
                                        }
                                    }
                                    loadUrl(url)
                                }
                            }
                        }, onRelease = { it.destroy() })
                    }
                }
            }
        }
    }
}
