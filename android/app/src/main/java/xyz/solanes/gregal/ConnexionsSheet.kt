package xyz.solanes.gregal

import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Link
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

/**
 * «Comptes connectats» (B5): el model únic de connexions del servidor
 * (/api/connexions). Compte, estat comprovat de debò, permisos efectius
 * (què pot fer l'eina ∩ què hi autoritza la persona), connectar un compte
 * nou d'un connector, tornar a iniciar sessió, desconnectar i treure.
 *
 * El consentiment s'obre al navegador del sistema; el servidor desa
 * l'autorització començada xifrada, així que tancar l'app durant el
 * consentiment no perd res: en tornar (o en obrir el full) l'estat es torna
 * a llegir del servidor. Cap secret arriba a l'app: només l'authRef opac.
 *
 * Textos propis (com ordinadorCopy de B3): AppStrings/Strings són de
 * l'integrador. La web té els mateixos textos a internal/web/app/connexions.js.
 */

internal fun connexioCopy(lang: String, key: String): String {
    val en = lang == "en"
    return when (key) {
        "title" -> if (en) "Connected accounts" else "Comptes connectats"
        "help" -> if (en) "The accounts your winds can use. The status is the last real check; having an account connected does not authorize every action: you choose what they may do."
            else "Els comptes que els teus vents poden fer servir. L'estat és el de l'última comprovació real; tenir un compte connectat no autoritza totes les accions: tu tries què hi poden fer."
        "open" -> if (en) "Connected accounts" else "Comptes connectats"
        "openHelp" -> if (en) "Connectors, calendars, mailboxes and sessions on the computer, with their real status." else "Connectors, calendaris, bústies i sessions a l'ordinador, amb el seu estat real."
        "empty" -> if (en) "No account is connected yet." else "Encara no hi ha cap compte connectat."
        "available" -> if (en) "Available" else "Disponible"
        "connecting" -> if (en) "Connecting…" else "Connectant…"
        "connected" -> if (en) "Connected" else "Connectat"
        "needs_session" -> if (en) "Sign in again" else "Cal tornar a iniciar sessió"
        "error" -> if (en) "Error" else "Error"
        "disconnected" -> if (en) "Disconnected" else "Desconnectat"
        "notChecked" -> if (en) "not checked yet" else "encara no comprovat"
        "checkedAt" -> if (en) "checked" else "comprovat"
        "svc.mcp" -> if (en) "Connector" else "Connector"
        "svc.calendar-ics" -> if (en) "Calendar (iCal)" else "Calendari (iCal)"
        "svc.mail-imap" -> if (en) "Email (IMAP)" else "Correu (IMAP)"
        "svc.browser" -> if (en) "Session on the computer's browser" else "Sessió al navegador de l'ordinador"
        "canRead" -> if (en) "can read" else "pot llegir"
        "canPrepare" -> if (en) "can prepare" else "pot preparar"
        "canSend" -> if (en) "can act with your confirmation" else "pot actuar amb la teva confirmació"
        "paused" -> if (en) "paused: cannot do anything" else "en pausa: no pot fer res"
        "check" -> if (en) "Check" else "Comprova"
        "reconnect" -> if (en) "Reconnect" else "Torna a connectar"
        "disconnect" -> if (en) "Disconnect" else "Desconnecta"
        "remove" -> if (en) "Remove" else "Treu"
        "allowSend" -> if (en) "Allow actions" else "Deixa actuar"
        "readOnly" -> if (en) "Read only" else "Només llegir"
        "connectOf" -> if (en) "Connect an account of " else "Connecta un compte de "
        "browserHelp" -> if (en) "For sites without a connector: ask a wind and sign in yourself on the computer's browser; the session stays here and the next task reuses it."
            else "Per a llocs sense connector: demana-ho a un vent i inicia sessió tu mateix al navegador de l'ordinador; la sessió queda aquí i la propera feina la reutilitza."
        "noBrowser" -> if (en) "Sites without a connector need a computer with a viewer; this server has none yet." else "Per a llocs sense connector cal un ordinador amb visor; aquest servidor encara no en té."
        "needsClientId" -> if (en) "This provider needs the administrator to register Gregal with it (client_id). Meanwhile, sign in on the computer's browser."
            else "Aquest proveïdor necessita que l'administrador hi registri Gregal (client_id). Mentrestant, inicia sessió al navegador de l'ordinador."
        "opening" -> if (en) "Finish in the browser and come back: the task carries on by itself." else "Acaba-ho al navegador i torna: la feina continua sola."
        "done" -> if (en) "Done." else "Fet."
        "failed" -> if (en) "Could not do it: " else "No s'ha pogut fer: "
        "close" -> if (en) "Close" else "Tanca"
        "waitTitle" -> if (en) "An account needs connecting" else "Cal connectar un compte"
        "waitSub" -> if (en) "The task is waiting" else "La feina espera"
        "waitOpen" -> if (en) "Open connected accounts" else "Obre els comptes connectats"
        "cancel" -> if (en) "Cancel" else "Cancel·la"
        else -> key
    }
}

internal data class ConnexioItem(
    val id: String,
    val service: String,
    val provider: String = "",
    val accountLabel: String = "",
    val status: String = "",
    val capabilities: List<String> = emptyList(),
    val allowed: List<String> = emptyList(),
    val effective: List<String> = emptyList(),
    val lastCheckedAt: String = "",
    val error: String = "",
)

internal data class ConnexioCatalogue(val service: String, val provider: String = "", val how: String = "", val available: Boolean = false)

internal data class ConnexionsState(
    val items: List<ConnexioItem> = emptyList(),
    val catalogue: List<ConnexioCatalogue> = emptyList(),
    val loaded: Boolean = false,
)

private fun JSONArray?.strings(): List<String> {
    if (this == null) return emptyList()
    return (0 until length()).map { optString(it) }.filter { it.isNotBlank() }
}

/** GET /api/connexions → estat (pur: testejable sense xarxa). */
internal fun parseConnexions(o: JSONObject): ConnexionsState {
    val items = mutableListOf<ConnexioItem>()
    o.optJSONArray("connexions")?.let { a ->
        for (i in 0 until a.length()) {
            val x = a.optJSONObject(i) ?: continue
            val id = x.optString("id")
            if (id.isBlank()) continue
            items += ConnexioItem(
                id = id, service = x.optString("service"), provider = x.optString("provider"),
                accountLabel = x.optString("accountLabel"), status = x.optString("status"),
                capabilities = x.optJSONArray("capabilities").strings(), allowed = x.optJSONArray("allowed").strings(),
                effective = x.optJSONArray("effective").strings(), lastCheckedAt = x.optString("lastCheckedAt"),
                error = x.optString("error"),
            )
        }
    }
    val cat = mutableListOf<ConnexioCatalogue>()
    o.optJSONArray("catalogue")?.let { a ->
        for (i in 0 until a.length()) {
            val x = a.optJSONObject(i) ?: continue
            cat += ConnexioCatalogue(x.optString("service"), x.optString("provider"), x.optString("how"), x.optBoolean("available"))
        }
    }
    return ConnexionsState(items, cat, loaded = true)
}

internal fun connexioServiceText(lang: String, c: ConnexioItem): String {
    val base = connexioCopy(lang, "svc." + c.service).let { if (it == "svc." + c.service) c.service else it }
    return if (c.provider.isNotBlank() && (c.service == "mcp" || c.service == "browser")) "$base · ${c.provider}" else base
}

/** El permís que val de debò, en paraules (el mateix que permsText de la web). */
internal fun connexioPermsText(lang: String, c: ConnexioItem): String {
    val parts = buildList {
        if ("read" in c.effective) add(connexioCopy(lang, "canRead"))
        if ("prepare" in c.effective) add(connexioCopy(lang, "canPrepare"))
        if ("send" in c.effective) add(connexioCopy(lang, "canSend"))
    }
    if (parts.isEmpty()) return connexioCopy(lang, "paused")
    return parts.joinToString(", ").replaceFirstChar { it.uppercase() }
}

/** El que es pot fer amb una connexió (pur: les regles de la pantalla). */
internal fun connexioActions(c: ConnexioItem): List<String> = buildList {
    if (c.service == "mcp" && c.status in setOf("needs_session", "disconnected", "error")) add("reconnect")
    if (c.status != "disconnected") {
        add("check")
        if ("send" in c.capabilities) add(if ("send" in c.allowed) "readOnly" else "allowSend")
        add("disconnect")
    }
    add("remove")
}

/** Esquema propi de l'app per tornar del consentiment (fork-safe: l'ID de paquet). */
internal fun connexioReturnTo(ctx: Context): String = ctx.packageName + "://connexions"

/** Text de l'error d'un «Connecta». */
internal fun connexioConnectError(lang: String, message: String): String =
    if (message.contains("needs_client_id")) connexioCopy(lang, "needsClientId") else connexioCopy(lang, "failed") + message.take(160)

internal fun GregalApi.connexions(): ConnexionsState = parseConnexions(get("/api/connexions"))

/** POST /api/connexions/connect → l'URL del consentiment. */
internal fun GregalApi.connectConnexio(body: JSONObject): String = post("/api/connexions/connect", body).optString("authUrl")

private fun openInBrowser(ctx: Context, url: String) {
    try { ctx.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) } catch (_: Exception) {}
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun ConnexionsSheet(conn: Conn, lang: String, onClose: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var state by remember { mutableStateOf(ConnexionsState()) }
    var msg by remember { mutableStateOf("") }
    suspend fun reload() {
        try { state = withContext(Dispatchers.IO) { conn.api().connexions() } }
        catch (e: Exception) { if (e is CancellationException) throw e; msg = connexioCopy(lang, "failed") + e.message.orEmpty().take(160) }
    }
    // En tornar del navegador (ON_RESUME) i mentre algun compte connecta.
    LifecyclePoll(Unit) {
        reload()
        while (state.items.any { it.status == "connecting" }) { delay(3_000); reload() }
    }
    fun act(path: String, body: JSONObject) = scope.launch {
        msg = "…"
        try {
            val r = withContext(Dispatchers.IO) { conn.api().post(path, body) }
            msg = r.optString("note").ifBlank { connexioCopy(lang, "done") }
            reload()
        } catch (e: Exception) { if (e is CancellationException) throw e; msg = connexioCopy(lang, "failed") + e.message.orEmpty().take(160) }
    }
    fun connect(body: JSONObject) = scope.launch {
        msg = "…"
        try {
            val url = withContext(Dispatchers.IO) { conn.api().connectConnexio(body.put("returnTo", connexioReturnTo(ctx))) }
            if (url.isNotBlank()) { openInBrowser(ctx, url); msg = connexioCopy(lang, "opening") }
            reload()
        } catch (e: Exception) { if (e is CancellationException) throw e; msg = connexioConnectError(lang, e.message.orEmpty()) }
    }

    ModalBottomSheet(onDismissRequest = onClose, containerColor = Superficie,
        sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)) {
        Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).padding(horizontal = 16.dp).padding(bottom = 24.dp).testTag("connexions-sheet"),
            verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                Icon(Icons.Rounded.Link, null, tint = Sea, modifier = Modifier.size(22.dp))
                Text(connexioCopy(lang, "title"), color = Ink, fontSize = 16.sp, fontWeight = FontWeight.SemiBold)
            }
            Text(connexioCopy(lang, "help"), color = Muted, fontSize = 12.5.sp, lineHeight = 17.sp)
            if (state.loaded && state.items.isEmpty()) Text(connexioCopy(lang, "empty"), color = Muted, fontSize = 13.sp)
            state.items.forEach { c ->
                ConnexioRow(lang, c) { action ->
                    val id = JSONObject().put("id", c.id)
                    when (action) {
                        "reconnect" -> connect(id)
                        "check" -> act("/api/connexions/check", id)
                        "disconnect" -> act("/api/connexions/disconnect", id)
                        "remove" -> act("/api/connexions/remove", id)
                        "allowSend" -> act("/api/connexions/update", id.put("allowed", JSONArray((c.allowed + "send").distinct())))
                        "readOnly" -> act("/api/connexions/update", id.put("allowed", JSONArray(c.allowed.filter { it != "send" })))
                    }
                }
            }
            state.catalogue.filter { it.service == "mcp" && it.provider.isNotBlank() }.forEach { k ->
                ConnButton(connexioCopy(lang, "connectOf") + k.provider, primary = true, modifier = Modifier.fillMaxWidth().testTag("connexio-connect-" + k.provider)) {
                    connect(JSONObject().put("service", "mcp").put("provider", k.provider))
                }
            }
            state.catalogue.firstOrNull { it.service == "browser" }?.let { b ->
                Text(connexioCopy(lang, if (b.available) "browserHelp" else "noBrowser"), color = Muted, fontSize = 12.sp, lineHeight = 16.sp)
            }
            if (msg.isNotBlank()) Text(msg, color = Muted, fontSize = 12.5.sp, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }.testTag("connexions-msg"))
            ConnButton(connexioCopy(lang, "close"), primary = false, modifier = Modifier.fillMaxWidth(), onClick = onClose)
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun ConnexioRow(lang: String, c: ConnexioItem, onAction: (String) -> Unit) {
    val bad = c.status == "needs_session" || c.status == "error"
    Column(
        Modifier.fillMaxWidth().clip(GregalShape.lg).background(SuperficieAlta).border(1.dp, if (bad) Sea.copy(alpha = .6f) else Line, GregalShape.lg)
            .padding(12.dp).testTag("connexio-" + c.id),
        verticalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        Text(c.accountLabel.ifBlank { c.id }, color = Ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
        val checked = if (c.lastCheckedAt.isBlank()) connexioCopy(lang, "notChecked") else connexioCopy(lang, "checkedAt") + " " + c.lastCheckedAt.replace('T', ' ').removeSuffix("Z").take(16)
        Text(connexioServiceText(lang, c) + " · " + connexioCopy(lang, c.status) + " · " + checked, color = Muted, fontSize = 12.sp,
            modifier = Modifier.testTag("connexio-status-" + c.id))
        if (c.status != "disconnected") Text(connexioPermsText(lang, c), color = Muted, fontSize = 12.sp)
        if (c.error.isNotBlank()) Text(c.error, color = Ink, fontSize = 12.sp)
        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            connexioActions(c).forEach { a ->
                ConnButton(connexioCopy(lang, a), primary = a == "reconnect", modifier = Modifier.testTag("connexio-$a-" + c.id)) { onAction(a) }
            }
        }
    }
}

@Composable
private fun ConnButton(label: String, primary: Boolean, modifier: Modifier = Modifier, onClick: () -> Unit) {
    Box(
        modifier.heightIn(min = 48.dp).clip(GregalShape.sm).background(if (primary) Sea else Color.Transparent)
            .then(if (primary) Modifier else Modifier.border(1.dp, Line, GregalShape.sm))
            .semantics { role = Role.Button }.clickable(onClick = onClick).padding(horizontal = 14.dp),
        contentAlignment = Alignment.Center,
    ) { Text(label, color = if (primary) OnSea else Ink, fontSize = 13.5.sp, fontWeight = FontWeight.SemiBold) }
}

/** Fila de Preferències que obre el full de comptes connectats. */
@Composable
internal fun ConnexionsEntry(conn: Conn, lang: String) {
    var open by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxWidth().clip(GregalShape.lg).clickable { open = true }.padding(vertical = 8.dp).testTag("connexions-entry"),
        verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(connexioCopy(lang, "open"), color = Ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
        Text(connexioCopy(lang, "openHelp"), color = Muted, fontSize = 12.sp, lineHeight = 16.sp)
    }
    if (open) ConnexionsSheet(conn, lang) { open = false }
}

/** Targeta al fil: la feina espera que la persona connecti un compte (waiting_connection). */
@Composable
internal fun ConnexioWaitCard(conn: Conn, lang: String, a: GregalApi.WindAsk, accent: Color, onCancel: () -> Unit) {
    var open by remember(a.key) { mutableStateOf(false) }
    Column(
        Modifier.fillMaxWidth().clip(GregalShape.lg).background(Superficie).border(1.dp, accent.copy(alpha = .45f), GregalShape.lg)
            .padding(16.dp).testTag("connexio-wait-card"),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Icon(Icons.Rounded.Link, null, tint = accent, modifier = Modifier.size(22.dp))
            Column {
                Text(connexioCopy(lang, "waitTitle"), color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                Text(connexioCopy(lang, "waitSub"), color = Muted, fontSize = 11.sp)
            }
        }
        if (a.what.isNotBlank()) Text(a.what, color = Ink, fontSize = 14.sp, lineHeight = 20.sp)
        if (a.reason.isNotBlank()) Text(a.reason, color = Muted, fontSize = 12.sp)
        ConnButton(connexioCopy(lang, "waitOpen"), primary = true, modifier = Modifier.fillMaxWidth().testTag("connexio-wait-open")) { open = true }
        ConnButton(connexioCopy(lang, "cancel"), primary = false, modifier = Modifier.fillMaxWidth().testTag("connexio-wait-cancel"), onClick = onCancel)
    }
    if (open) ConnexionsSheet(conn, lang) { open = false }
}
