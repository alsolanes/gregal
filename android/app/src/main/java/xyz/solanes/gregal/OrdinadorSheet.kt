package xyz.solanes.gregal

import android.annotation.SuppressLint
import android.webkit.CookieManager
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Computer
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject

/**
 * «Ordinador» (B3): veure l'ordinador personal, prendre'n o retornar-ne el
 * control i, en un handoff, iniciar-hi sessió i dir «Ja està».
 *
 * El visor és una pàgina del servidor (/ordinador/visor/) que s'obre amb un
 * testimoni d'un sol ús i de 60 s: el WebView el bescanvia per una galeta de
 * sessió curta. Mai no hi viatja el token de l'app ni cap contrasenya: el que
 * la persona escriu va del WebView al navegador de l'ordinador i prou.
 * Sense el control, el visor és de només lectura.
 *
 * Textos propis (com askUi/threadCopy de B1): AppStrings/Strings són de
 * l'integrador. La web té els mateixos textos a winds-workspace.js (t()).
 */

internal fun ordinadorCopy(lang: String, key: String): String {
    val en = lang == "en"
    return when (key) {
        "title" -> if (en) "Computer" else "Ordinador"
        "starting" -> if (en) "Starting your computer…" else "Engegant el teu ordinador…"
        "startingSub" -> if (en) "It was saved to disk; it usually takes under 30 s." else "Estava desat a disc; sol trigar menys de 30 s."
        "running" -> if (en) "Gregal is working" else "Gregal hi treballa"
        "runningSub" -> if (en) "You are watching. Take control to use it yourself." else "Ho estàs veient. Pren el control per fer-lo servir tu."
        "idle" -> if (en) "Your computer" else "El teu ordinador"
        "idleSub" -> if (en) "Nobody is using it. Take control to use it yourself." else "Ningú no l'està fent servir. Pren el control per fer-lo servir tu."
        "user" -> if (en) "You have control" else "Tens el control"
        "userSub" -> if (en) "Gregal is paused until you give control back." else "Gregal s'espera fins que el tornis."
        "stopped" -> if (en) "Computer saved" else "Ordinador desat"
        "stoppedSub" -> if (en) "Opening the viewer starts it." else "En obrir el visor s'engega."
        "unreachable" -> if (en) "The computer does not answer" else "L'ordinador no respon"
        "unreachableSub" -> if (en) "Network or computer down. Gregal keeps the task waiting." else "Xarxa o ordinador caigut. Gregal manté la feina en espera."
        "noViewer" -> if (en) "This server has no remote viewer." else "Aquest servidor no té visor remot."
        "error" -> if (en) "Could not open the computer." else "No s'ha pogut obrir l'ordinador."
        "take" -> if (en) "Take control" else "Pren el control"
        "give" -> if (en) "Give control back" else "Torna el control"
        "done" -> if (en) "Done" else "Ja està"
        "cancel" -> if (en) "Cancel" else "Cancel·la"
        "close" -> if (en) "Close" else "Tanca"
        "screen" -> if (en) "Computer screen" else "Pantalla de l'ordinador"
        "handoffTitle" -> if (en) "Your turn on the computer" else "Et toca a tu a l'ordinador"
        "handoffHint" -> if (en) "Gregal never types passwords or codes: you do it on the computer and the task carries on."
            else "Gregal no escriu mai contrasenyes ni codis: ho fas tu a l'ordinador i l'encàrrec continua."
        "open" -> if (en) "Open the computer" else "Obre l'ordinador"
        else -> key
    }
}

/** L'estat del visor tal com el torna el servidor (/api/ordinador/viewer|control). */
internal data class OrdinadorView(
    val health: String = "",
    val controlOwner: String = "none",
    val leaseExpiry: String = "",
    val viewer: Boolean = true,
    val handoff: String = "",
    val url: String = "",
)

internal fun parseOrdinadorView(o: JSONObject): OrdinadorView = OrdinadorView(
    health = o.optString("health"),
    controlOwner = o.optString("controlOwner").ifBlank { "none" },
    leaseExpiry = if (o.isNull("leaseExpiry")) "" else o.optString("leaseExpiry"),
    viewer = o.optBoolean("viewer", true),
    handoff = o.optString("handoff"),
    url = o.optString("url"),
)

/** Clau de text de l'estat: starting, user, running (l'agent hi treballa), idle, stopped, unreachable. */
internal fun ordinadorStateKey(v: OrdinadorView): String = when (v.health) {
    "running" -> when (v.controlOwner) { "user" -> "user"; "agent" -> "running"; else -> "idle" }
    "starting", "stopped", "unreachable" -> v.health
    else -> "unreachable"
}

/** L'adreça absoluta del visor (el servidor la dona relativa). */
internal fun viewerAbsoluteUrl(base: String, rel: String, lang: String): String {
    if (rel.isBlank()) return ""
    val b = base.trimEnd('/')
    val abs = if (rel.startsWith("http://") || rel.startsWith("https://")) rel else b + (if (rel.startsWith("/")) rel else "/$rel")
    return abs + (if (abs.contains('?')) "&" else "?") + "lang=" + (if (lang == "en") "en" else "ca")
}

/** POST /api/ordinador/viewer: testimoni curt del visor (i el control, si cal). */
internal fun GregalApi.openOrdinadorViewer(control: Boolean, handoff: String = ""): OrdinadorView =
    parseOrdinadorView(post("/api/ordinador/viewer", JSONObject().put("control", control).put("handoff", handoff)))

/** /api/ordinador/control: llegir (action buida), take o release. */
internal fun GregalApi.ordinadorControl(action: String = ""): OrdinadorView =
    parseOrdinadorView(if (action.isBlank()) get("/api/ordinador/control") else post("/api/ordinador/control", JSONObject().put("action", action)))

/**
 * El full «Ordinador». handoffKey: la decisió de handoff que l'ha obert (pren
 * el control en obrir-se i ensenya «Ja està» / «Cancel·la»).
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun OrdinadorSheet(
    conn: Conn, lang: String, accent: Color = Color.Unspecified, handoffKey: String = "",
    onDone: (() -> Unit)? = null, onCancel: (() -> Unit)? = null, onClose: () -> Unit,
) {
    val scope = rememberCoroutineScope()
    var view by remember { mutableStateOf<OrdinadorView?>(null) }
    var url by remember { mutableStateOf("") }
    var error by remember { mutableStateOf("") }
    val acc = if (accent != Color.Unspecified) accent else Sea

    LaunchedEffect(handoffKey) {
        try {
            val v = withContext(Dispatchers.IO) { conn.api().openOrdinadorViewer(control = handoffKey.isNotBlank(), handoff = handoffKey) }
            view = v
            url = viewerAbsoluteUrl(conn.api().base, v.url, lang)
        } catch (e: Exception) {
            if (e is CancellationException) throw e
            error = if (e.message.orEmpty().contains("503")) ordinadorCopy(lang, "noViewer") else ordinadorCopy(lang, "error")
        }
    }
    // Estat: mentre el full és a la vista (l'app en primer pla).
    LifecyclePoll(url) {
        while (url.isNotBlank()) {
            delay(5_000)
            try { view = withContext(Dispatchers.IO) { conn.api().ordinadorControl() } }
            catch (e: Exception) { if (e is CancellationException) throw e }
        }
    }
    fun control(action: String) = scope.launch {
        try { view = withContext(Dispatchers.IO) { conn.api().ordinadorControl(action) } }
        catch (e: Exception) { if (e is CancellationException) throw e }
    }
    // En tancar el full, el control torna a l'agent (si no, caduca sol).
    DisposableEffect(Unit) {
        onDispose {
            if (view?.controlOwner == "user") Thread {
                try { conn.api().ordinadorControl("release") } catch (_: Exception) {}
            }.start()
        }
    }

    ModalBottomSheet(onDismissRequest = onClose, containerColor = Superficie,
        sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)) {
        Column(Modifier.fillMaxWidth().fillMaxHeight(.92f).padding(horizontal = 16.dp).padding(bottom = 16.dp).testTag("ordinador-sheet"),
            verticalArrangement = Arrangement.spacedBy(10.dp)) {
            val v = view
            val key = v?.let { ordinadorStateKey(it) } ?: "starting"
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp),
                modifier = Modifier.fillMaxWidth().semantics { liveRegion = LiveRegionMode.Polite }) {
                Icon(Icons.Rounded.Computer, null, tint = acc, modifier = Modifier.size(24.dp))
                Column(Modifier.weight(1f)) {
                    Text(if (error.isNotBlank()) ordinadorCopy(lang, "title") else ordinadorCopy(lang, key),
                        color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.testTag("ordinador-state"))
                    Text(error.ifBlank { ordinadorCopy(lang, key + "Sub") }, color = Muted, fontSize = 12.sp)
                }
                if (v != null && v.health == "running" && error.isBlank()) {
                    val mine = v.controlOwner == "user"
                    OrdButton(ordinadorCopy(lang, if (mine) "give" else "take"), primary = !mine, accent = acc,
                        modifier = Modifier.testTag("ordinador-control")) { control(if (mine) "release" else "take") }
                }
            }
            if (handoffKey.isNotBlank()) {
                Text(ordinadorCopy(lang, "handoffHint"), color = Muted, fontSize = 12.sp)
            }
            Box(Modifier.weight(1f).fillMaxWidth().clip(GregalShape.lg).background(SuperficieAlta).border(1.dp, Line, GregalShape.lg)) {
                if (url.isNotBlank()) ViewerWebView(url, ordinadorCopy(lang, "screen"))
            }
            if (handoffKey.isNotBlank()) {
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp), modifier = Modifier.fillMaxWidth()) {
                    OrdButton(ordinadorCopy(lang, "cancel"), primary = false, modifier = Modifier.weight(1f).testTag("ordinador-cancel")) {
                        onCancel?.invoke(); onClose()
                    }
                    OrdButton(ordinadorCopy(lang, "done"), primary = true, accent = acc, modifier = Modifier.weight(1f).testTag("ordinador-done")) {
                        onDone?.invoke(); onClose()
                    }
                }
            }
        }
    }
}

@SuppressLint("SetJavaScriptEnabled")
@Composable
private fun ViewerWebView(url: String, description: String) {
    key(url) {
        AndroidView(modifier = Modifier.fillMaxSize().semantics { contentDescription = description }.testTag("ordinador-webview"), factory = { ctx ->
            WebView(ctx).apply {
                settings.javaScriptEnabled = true // el noVNC és JavaScript
                settings.domStorageEnabled = true
                settings.allowFileAccess = false
                settings.allowContentAccess = false
                settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
                CookieManager.getInstance().setAcceptCookie(true)
                webViewClient = object : WebViewClient() {
                    // Només el visor: cap enllaç no porta enlloc més.
                    override fun shouldOverrideUrlLoading(view: WebView?, request: android.webkit.WebResourceRequest?): Boolean {
                        val target = request?.url?.toString().orEmpty()
                        return !target.contains("/ordinador/visor/")
                    }
                }
                loadUrl(url)
            }
        }, onRelease = { it.destroy() })
    }
}

@Composable
private fun OrdButton(label: String, primary: Boolean, modifier: Modifier = Modifier, accent: Color = Color.Unspecified, onClick: () -> Unit) {
    val bg = if (primary) (if (accent != Color.Unspecified) accent else Sea) else Color.Transparent
    Box(
        modifier.heightIn(min = 48.dp).clip(GregalShape.sm).background(bg)
            .then(if (primary) Modifier else Modifier.border(1.dp, Line, GregalShape.sm))
            .semantics { role = Role.Button }.clickable(onClick = onClick).padding(horizontal = 14.dp),
        contentAlignment = Alignment.Center,
    ) { Text(label, color = if (primary) OnSea else Ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold) }
}

/** Targeta de handoff al fil: la feina espera que la persona faci una cosa a l'ordinador. */
@Composable
internal fun HandoffCard(conn: Conn, lang: String, a: GregalApi.WindAsk, accent: Color, onDone: () -> Unit, onCancel: () -> Unit) {
    var open by remember(a.key) { mutableStateOf(false) }
    Column(
        Modifier.fillMaxWidth().clip(GregalShape.lg).background(Superficie).border(1.dp, accent.copy(alpha = .45f), GregalShape.lg)
            .padding(16.dp).testTag("handoff-card"),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Icon(Icons.Rounded.Computer, null, tint = accent, modifier = Modifier.size(22.dp))
            Column {
                Text(ordinadorCopy(lang, "handoffTitle"), color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                Text(askUi(lang, "waiting"), color = Muted, fontSize = 11.sp)
            }
        }
        if (a.what.isNotBlank()) Text(a.what, color = Ink, fontSize = 14.sp, lineHeight = 20.sp)
        Text(ordinadorCopy(lang, "handoffHint"), color = Muted, fontSize = 12.sp)
        OrdButton(ordinadorCopy(lang, "open"), primary = true, accent = accent, modifier = Modifier.fillMaxWidth().testTag("handoff-open")) { open = true }
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp), modifier = Modifier.fillMaxWidth()) {
            OrdButton(ordinadorCopy(lang, "cancel"), primary = false, modifier = Modifier.weight(1f).testTag("handoff-cancel"), onClick = onCancel)
            OrdButton(ordinadorCopy(lang, "done"), primary = false, modifier = Modifier.weight(1f).testTag("handoff-done"), onClick = onDone)
        }
    }
    if (open) OrdinadorSheet(conn, lang, accent, handoffKey = a.key, onDone = onDone, onCancel = onCancel, onClose = { open = false })
}
