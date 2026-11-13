package xyz.solanes.gregal

import android.annotation.SuppressLint
import android.webkit.CookieManager
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Computer
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.*
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.input.pointer.pointerInput
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
        "local" -> if (en) "Gregal's browser, on this computer" else "El navegador de Gregal, en aquest ordinador"
        "localSub" -> if (en) "You see it live. Tap the screen to use it yourself: Gregal waits meanwhile." else "El veus en directe. Toca la pantalla per fer-lo servir tu: mentrestant, Gregal s'espera."
        "localMine" -> if (en) "You have control · Gregal is waiting" else "Tens el control · Gregal s'espera"
        "giveShort" -> if (en) "Give it back" else "Torna-l'hi"
        "back" -> if (en) "Back" else "Enrere"
        "typeHint" -> if (en) "Type into the field you tapped…" else "Escriu al camp que has tocat…"
        "type" -> if (en) "Type" else "Escriu"
        "scrollUp" -> if (en) "Scroll up" else "Puja"
        "scrollDown" -> if (en) "Scroll down" else "Baixa"
        "localIdle" -> if (en) "Gregal has no page open right now. Ask it to look something up and you'll see it here." else "Ara mateix Gregal no té cap pàgina oberta. Demana-li que busqui alguna cosa i ho veuràs aquí."
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
        "noComputer" -> if (en) "No computer is set up for your account yet. Tasks that need it wait until there is one."
            else "El teu compte encara no té cap ordinador configurat. Les feines que en necessiten esperen fins que n'hi hagi."
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

/** Quin text d'error toca segons la resposta del servidor (pura: testejable). */
internal fun ordinadorErrorKey(message: String): String = when {
    message.startsWith("HTTP 404") -> "noComputer"
    message.startsWith("HTTP 503") -> "noViewer"
    else -> "error"
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

/** Mode local: una captura actual del navegador de Gregal en aquest servidor. */
internal data class OrdinadorLive(
    val live: Boolean, val url: String = "", val title: String = "", val image: ByteArray? = null,
    /** "user" quan la persona té el control (Gregal s'espera). */
    val controlOwner: String = "none",
)

internal fun parseOrdinadorLive(o: JSONObject): OrdinadorLive {
    val img = o.optString("image").substringAfter("base64,", "").takeIf { it.isNotBlank() }
        ?.let { runCatching { android.util.Base64.decode(it, android.util.Base64.DEFAULT) }.getOrNull() }
    return OrdinadorLive(o.optBoolean("live") && img != null, o.optString("url"), o.optString("title"), img,
        o.optString("controlOwner").ifBlank { "none" })
}

/** GET /api/ordinador/live (mode local, sense VM ni noVNC). */
internal fun GregalApi.ordinadorLive(): OrdinadorLive = parseOrdinadorLive(get("/api/ordinador/live"))

/**
 * POST /api/ordinador/live: la persona el fa servir des del mòbil (tap amb x/y
 * en fraccions de la captura, type, key, scroll, back). Pren el control i torna
 * la captura de després.
 */
internal fun GregalApi.ordinadorLiveInput(body: JSONObject): OrdinadorLive = parseOrdinadorLive(post("/api/ordinador/live", body))

/**
 * On cau un toc dins de la captura, en fraccions (0–1), quan la imatge es
 * dibuixa encaixada (Fit) i alineada a dalt al centre d'una caixa. Fora de la
 * imatge, null. Pura: testejable.
 */
internal fun liveTapFraction(tapX: Float, tapY: Float, boxW: Float, boxH: Float, imgW: Float, imgH: Float): Pair<Float, Float>? {
    if (boxW <= 0f || boxH <= 0f || imgW <= 0f || imgH <= 0f) return null
    val scale = minOf(boxW / imgW, boxH / imgH)
    val w = imgW * scale
    val h = imgH * scale
    val left = (boxW - w) / 2f
    val fx = (tapX - left) / w
    val fy = tapY / h
    if (fx < 0f || fx > 1f || fy < 0f || fy > 1f) return null
    return fx to fy
}

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
    var replay by remember { mutableStateOf(false) }
    // Sense ordinador remot (ni noVNC): l'ordinador és el navegador d'aquest servidor, en directe.
    var local by remember { mutableStateOf(false) }
    var shot by remember { mutableStateOf<OrdinadorLive?>(null) }
    val acc = if (accent != Color.Unspecified) accent else Sea

    LaunchedEffect(handoffKey) {
        try {
            val v = withContext(Dispatchers.IO) { conn.api().openOrdinadorViewer(control = handoffKey.isNotBlank(), handoff = handoffKey) }
            view = v
            url = viewerAbsoluteUrl(conn.api().base, v.url, lang)
        } catch (e: Exception) {
            if (e is CancellationException) throw e
            if (e.message.orEmpty().contains("no_viewer") || e.message.orEmpty().contains("no remote viewer")) local = true
            else error = ordinadorErrorKey(e.message.orEmpty()).let { ordinadorCopy(lang, it) }
        }
    }
    var acting by remember { mutableStateOf(false) }
    LifecyclePoll(local) {
        while (local) {
            if (!acting) try { shot = withContext(Dispatchers.IO) { conn.api().ordinadorLive() } }
            catch (e: Exception) { if (e is CancellationException) throw e }
            delay(1_000)
        }
    }
    // Mode local: el que la persona fa des del mòbil (pren el control).
    fun act(body: JSONObject) {
        if (acting) return
        acting = true
        scope.launch {
            try { shot = withContext(Dispatchers.IO) { conn.api().ordinadorLiveInput(body) } }
            catch (e: Exception) { if (e is CancellationException) throw e }
            finally { acting = false }
        }
    }
    val localHeld = local && shot?.controlOwner == "user"
    fun giveBackLocal() = scope.launch {
        shot = shot?.copy(controlOwner = "none")
        try { withContext(Dispatchers.IO) { conn.api().ordinadorControl("release") } }
        catch (e: Exception) { if (e is CancellationException) throw e }
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
            if (view?.controlOwner == "user" || shot?.controlOwner == "user") Thread {
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
                    Text(if (local) ordinadorCopy(lang, "local") else if (error.isNotBlank()) ordinadorCopy(lang, "title") else ordinadorCopy(lang, key),
                        color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.testTag("ordinador-state"))
                    Text(if (localHeld) ordinadorCopy(lang, "localMine") else if (local) ordinadorCopy(lang, "localSub") else error.ifBlank { ordinadorCopy(lang, key + "Sub") },
                        color = if (localHeld) acc else Muted, fontSize = 12.sp, modifier = Modifier.testTag("ordinador-sub"))
                }
                if (localHeld) OrdButton(ordinadorCopy(lang, "giveShort"), primary = false,
                    modifier = Modifier.testTag("ordinador-local-give")) { giveBackLocal() }
                if (v != null && v.health == "running" && error.isBlank()) {
                    val mine = v.controlOwner == "user"
                    OrdButton(ordinadorCopy(lang, if (mine) "give" else "take"), primary = !mine, accent = acc,
                        modifier = Modifier.testTag("ordinador-control")) { control(if (mine) "release" else "take") }
                }
            }
            if (handoffKey.isNotBlank()) {
                Text(ordinadorCopy(lang, "handoffHint"), color = Muted, fontSize = 12.sp)
            } else if (!replay && (error.isBlank() || local)) {
                // B13: el que ha fet l'agent quan no s'hi mirava, amb les captures marcades.
                OrdButton(accionsCopy(lang, "last"), primary = false, modifier = Modifier.testTag("ordinador-last")) { replay = true }
            }
            Box(Modifier.weight(1f).fillMaxWidth().clip(GregalShape.lg).background(SuperficieAlta).border(1.dp, Line, GregalShape.lg)) {
                // El visor en directe ja dibuixa on actua l'agent (pàgina del servidor).
                if (url.isNotBlank()) ViewerWebView(url, ordinadorCopy(lang, "screen"))
                if (local) LocalLiveView(lang, shot, onTap = { x, y -> act(JSONObject().put("kind", "tap").put("x", x.toDouble()).put("y", y.toDouble())) })
                if (replay) Box(Modifier.fillMaxSize().background(SuperficieAlta).padding(8.dp)) {
                    OrdinadorReplay(conn, lang, acc, onClose = { replay = false })
                }
            }
            if (local && shot?.live == true && !replay) LocalLiveControls(lang, acc) { act(it) }
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

/** Mode local: la pestanya on treballa Gregal, en directe (una captura per segon). */
@Composable
internal fun LocalLiveView(lang: String, shot: OrdinadorLive?, onTap: (Float, Float) -> Unit = { _, _ -> }) {
    val bmp = remember(shot?.image) {
        shot?.image?.let { runCatching { android.graphics.BitmapFactory.decodeByteArray(it, 0, it.size)?.asImageBitmap() }.getOrNull() }
    }
    Column(Modifier.fillMaxSize().padding(10.dp).testTag("ordinador-local"), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        if (shot?.live == true && bmp != null) {
            Text(shot.title.ifBlank { shot.url }, color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, maxLines = 1)
            if (shot.title.isNotBlank()) Text(shot.url, color = Muted, fontSize = 11.sp, maxLines = 1)
            androidx.compose.foundation.Image(bmp, ordinadorCopy(lang, "screen"),
                modifier = Modifier.fillMaxWidth().weight(1f).clip(GregalShape.sm).testTag("ordinador-local-shot")
                    .pointerInput(bmp) {
                        detectTapGestures { o ->
                            liveTapFraction(o.x, o.y, size.width.toFloat(), size.height.toFloat(), bmp.width.toFloat(), bmp.height.toFloat())
                                ?.let { (x, y) -> onTap(x, y) }
                        }
                    },
                contentScale = androidx.compose.ui.layout.ContentScale.Fit, alignment = Alignment.TopCenter)
        } else {
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text(ordinadorCopy(lang, "localIdle"), color = Muted, fontSize = 14.sp, lineHeight = 20.sp,
                    textAlign = androidx.compose.ui.text.style.TextAlign.Center, modifier = Modifier.padding(24.dp))
            }
        }
    }
}

/** Mode local: enrere, escriure al camp tocat, Enter i desplaçar. */
@Composable
internal fun LocalLiveControls(lang: String, accent: Color, onAct: (JSONObject) -> Unit) {
    var text by remember { mutableStateOf("") }
    fun send() {
        if (text.isEmpty()) onAct(JSONObject().put("kind", "key").put("key", "Enter"))
        else { onAct(JSONObject().put("kind", "type").put("text", text)); text = "" }
    }
    Column(Modifier.fillMaxWidth().testTag("ordinador-local-controls"), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            androidx.compose.foundation.text.BasicTextField(
                value = text, onValueChange = { text = it }, singleLine = true,
                textStyle = androidx.compose.ui.text.TextStyle(color = Ink, fontSize = 15.sp),
                cursorBrush = androidx.compose.ui.graphics.SolidColor(accent),
                keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(imeAction = androidx.compose.ui.text.input.ImeAction.Send),
                keyboardActions = androidx.compose.foundation.text.KeyboardActions(onSend = { send() }),
                modifier = Modifier.weight(1f).heightIn(min = 48.dp).clip(GregalShape.sm).background(SuperficieAlta)
                    .border(1.dp, Line, GregalShape.sm).testTag("ordinador-local-text"),
                decorationBox = { inner ->
                    Box(Modifier.padding(horizontal = 12.dp), contentAlignment = Alignment.CenterStart) {
                        if (text.isEmpty()) Text(ordinadorCopy(lang, "typeHint"), color = Muted, fontSize = 14.sp, maxLines = 1)
                        inner()
                    }
                },
            )
            OrdButton(if (text.isEmpty()) "↵" else ordinadorCopy(lang, "type"), primary = true, accent = accent,
                modifier = Modifier.testTag("ordinador-local-send")) { send() }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
            OrdButton("‹ " + ordinadorCopy(lang, "back"), primary = false, modifier = Modifier.weight(1f).testTag("ordinador-local-back")) {
                onAct(JSONObject().put("kind", "back"))
            }
            OrdButton("↑", primary = false, modifier = Modifier.weight(1f).semantics { contentDescription = ordinadorCopy(lang, "scrollUp") }) {
                onAct(JSONObject().put("kind", "scroll").put("dir", "up"))
            }
            OrdButton("↓", primary = false, modifier = Modifier.weight(1f).semantics { contentDescription = ordinadorCopy(lang, "scrollDown") }) {
                onAct(JSONObject().put("kind", "scroll").put("dir", "down"))
            }
        }
    }
}
