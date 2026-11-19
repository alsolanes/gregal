package xyz.solanes.gregal

import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.Typeface
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.ContentCopy
import androidx.compose.material.icons.rounded.Image
import androidx.compose.material.icons.rounded.Link
import androidx.compose.material.icons.rounded.PersonAdd
import androidx.compose.material.icons.rounded.Share
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.graphics.layer.drawLayer
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.FileProvider
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.io.File

/**
 * Compartir un missatge d'un vent (com share-msg.js a l'escriptori):
 * text per a WhatsApp/Telegram (les targetes passen a text llegible), la
 * bombolla com a imatge, enviar-ho a una altra persona de l'app (li arriba a
 * Desat) o un enllaç públic de només lectura (POST /api/winds/clip).
 */
object ShareText {
    private val CARD = Regex("```(gregal-[a-z]+|chart)[ \\t]*\\n([\\s\\S]*?)```")

    private fun s(v: Any?): String = if (v == null || v == JSONObject.NULL) "" else v.toString().trim()
    private fun join(xs: List<Any?>, sep: String) = xs.map { s(it) }.filter { it.isNotEmpty() }.joinToString(sep)
    private fun JSONObject.str(k: String) = s(opt(k))
    private fun JSONObject.arr(k: String): List<Any?> = optJSONArray(k)?.let { a -> (0 until a.length()).map { a.opt(it) } } ?: emptyList()
    private fun isUrl(u: String) = u.startsWith("http://") || u.startsWith("https://")

    /** Una targeta en línies de text; null si no s'entén (el bloc es queda). */
    fun cardText(lang: String, kind: String, json: String): String? {
        val o = runCatching { JSONObject(json) }.getOrNull() ?: return null
        val en = lang == "en"
        val out = mutableListOf<String>()
        fun head(t: String) { if (t.isNotEmpty()) out += "*$t*" }
        when (kind) {
            "gregal-result" -> {
                head(o.str("title")); if (o.str("subtitle").isNotEmpty()) out += o.str("subtitle")
                for (r in o.arr("rows").filterIsInstance<JSONObject>()) {
                    val what = join(listOf(r.opt("lead"), r.opt("text")), " — ")
                    out += "• " + what + (if (r.str("value").isNotEmpty()) ": " + r.str("value") else "") +
                        (if (r.optBoolean("pick")) " ✓" else "") + (if (isUrl(r.str("url"))) "\n  " + r.str("url") else "")
                }
            }
            "gregal-plan" -> {
                head(o.str("title")); if (o.str("subtitle").isNotEmpty()) out += o.str("subtitle")
                for (x in o.arr("items").filterIsInstance<JSONObject>()) {
                    out += (if (x.str("time").isNotEmpty()) x.str("time") + " · " else "• ") +
                        join(listOf(x.str("title").ifEmpty { x.str("text") }, x.opt("detail")), " — ")
                }
                val total = o.opt("total")
                if (total is JSONObject) out += join(listOf(total.opt("label"), total.opt("value")), ": ")
                else if (s(total).isNotEmpty()) out += "Total: " + s(total)
            }
            "gregal-route" -> {
                head(join(listOf(o.str("from").ifEmpty { o.str("from_stop") }, o.str("to").ifEmpty { o.str("to_stop") }), " → "))
                if (o.str("title").isNotEmpty()) out += o.str("title")
                out += join(listOf(join(listOf(o.opt("depart"), o.opt("arrive")), " → "), o.opt("service"), o.opt("duration"), o.opt("price")), " · ")
                for (a in o.arr("alternatives").filterIsInstance<JSONObject>()) out += "  · " + join(listOf(a.opt("time"), a.opt("service"), a.opt("price")), " · ")
            }
            "gregal-recipe" -> {
                head(o.str("title"))
                val n = o.str("servings").toIntOrNull() ?: 0
                val meta = join(listOf(o.opt("time"), o.opt("level"), if (n > 0) (if (en) "$n servings" else "$n racions") else ""), " · ")
                if (meta.isNotEmpty()) out += meta
                val ing = o.arr("ingredients")
                if (ing.isNotEmpty()) {
                    out += ""; out += "Ingredients:"
                    for (x in ing) out += "• " + (if (x is JSONObject) join(listOf(x.opt("qty"), x.opt("name")), " ") else s(x))
                }
                val steps = o.arr("steps")
                if (steps.isNotEmpty()) {
                    out += ""; out += if (en) "Steps:" else "Passos:"
                    steps.forEachIndexed { i, x ->
                        out += "${i + 1}. " + if (x is JSONObject) {
                            val min = x.str("minutes").toIntOrNull() ?: 0
                            x.str("text") + if (min > 0) " ($min min)" else ""
                        } else s(x)
                    }
                }
                if (o.str("tip").isNotEmpty()) { out += ""; out += o.str("tip") }
            }
            "chart", "gregal-plot" -> {
                head(o.str("title"))
                val labels = o.arr("labels")
                val series = o.arr("series").filterIsInstance<JSONObject>()
                labels.take(24).forEachIndexed { i, l ->
                    out += "• " + s(l) + ": " + series.joinToString(", ") { x ->
                        (if (series.size > 1) x.str("label") + " " else "") + s(x.optJSONArray("values")?.opt(i))
                    }
                }
            }
            else -> return null
        }
        if (out.isEmpty()) return null
        val src = o.arr("sources").filterIsInstance<JSONObject>().map { it.str("url") }.filter { isUrl(it) }.take(3)
        if (src.isNotEmpty()) { out += ""; out += src }
        return out.joinToString("\n")
    }

    /** El missatge tal com s'ha d'enganxar a un xat de fora (mateixa regla que share-msg.js). */
    fun plain(md: String, lang: String): String {
        var t = md.replace("\r", "")
        t = CARD.replace(t) { m -> cardText(lang, m.groupValues[1], m.groupValues[2].trim())?.let { it + "\n" } ?: m.value }
        t = Regex("!\\[[^\\]\\n]*\\]\\((https?://[^\\s)]+)\\)").replace(t) { it.groupValues[1] }
        t = Regex("!\\[[^\\]\\n]*\\]\\([^)\\n]*\\)").replace(t, "")
        t = Regex("\\[([^\\]\\n]+)\\]\\(/api/[^)\\s]*\\)").replace(t) { it.groupValues[1] }
        t = Regex("\\[([^\\]\\n]+)\\]\\((https?://[^\\s)]+)\\)").replace(t) { m ->
            val (txt, u) = m.destructured; if (txt == u) u else "$txt ($u)"
        }
        t = Regex("(?m)^#{1,6}\\s+(.+)$").replace(t) { "*" + it.groupValues[1] + "*" }
        t = Regex("\\*\\*([^*\\n]+)\\*\\*").replace(t) { "*" + it.groupValues[1] + "*" }
        t = Regex("__([^_\\n]+)__").replace(t) { "*" + it.groupValues[1] + "*" }
        t = Regex("(?m)^[ \\t]*[-*][ \\t]+").replace(t, "• ")
        t = Regex("`([^`\\n]+)`").replace(t) { it.groupValues[1] }
        return Regex("\\n{3,}").replace(t, "\n\n").trim()
    }

    /** Textos del full de compartir. */
    fun word(lang: String, key: String): String {
        val ca = mapOf(
            "title" to "Comparteix", "copy" to "Copia el text", "copied" to "Copiat",
            "text" to "Comparteix com a text", "textSub" to "WhatsApp, Telegram… les targetes, com a text",
            "image" to "Comparteix com a imatge", "imageSub" to "La bombolla tal com la veus",
            "person" to "Envia a algú de Gregal", "personSub" to "Li arriba a Desat",
            "link" to "Crea un enllaç públic", "linkSub" to "Només lectura, sense compte",
            "send" to "Envia", "sent" to "Enviat a %s", "noAccounts" to "Per enviar a algú, el servidor ha de tenir comptes.",
            "nobody" to "Encara no hi ha ningú més en aquest servidor.", "making" to "Creant l'enllaç…",
            "private" to "Ull: és una adreça de casa; de fora no s'obrirà.", "shareLink" to "Comparteix l'enllaç",
            "from" to "De %s", "remove" to "Treu-ho", "chooser" to "Comparteix amb…", "imageFail" to "No s'ha pogut fer la imatge.",
        )
        val en = mapOf(
            "title" to "Share", "copy" to "Copy text", "copied" to "Copied",
            "text" to "Share as text", "textSub" to "WhatsApp, Telegram… cards as plain text",
            "image" to "Share as image", "imageSub" to "The bubble as you see it",
            "person" to "Send to someone on Gregal", "personSub" to "It lands in their Saved",
            "link" to "Create a public link", "linkSub" to "Read-only, no account needed",
            "send" to "Send", "sent" to "Sent to %s", "noAccounts" to "To send to someone, the server needs accounts.",
            "nobody" to "There's nobody else on this server yet.", "making" to "Creating the link…",
            "private" to "Heads up: it's a home address; it won't open from outside.", "shareLink" to "Share the link",
            "from" to "From %s", "remove" to "Remove", "chooser" to "Share with…", "imageFail" to "Couldn't make the image.",
        )
        return (if (lang == "en") en else ca)[key] ?: key
    }
}

/** Què es comparteix: el text del missatge, qui el va dir i, si es pot, la foto de la bombolla. */
data class ShareTarget(val text: String, val from: String, val capture: (suspend () -> ImageBitmap?)? = null)

/** Resultat de l'enllaç públic. */
data class ClipLink(val link: String, val private: Boolean)

object ShareApi {
    fun people(api: GregalApi): List<String> {
        val a = api.get("/api/winds/shared/people").optJSONArray("people") ?: JSONArray()
        return (0 until a.length()).map { a.optString(it) }.filter { it.isNotBlank() }
    }
    fun send(api: GregalApi, to: List<String>, text: String, from: String): List<String> {
        val r = api.post("/api/winds/send", JSONObject().put("to", JSONArray(to)).put("text", text).put("from", from))
        val a = r.optJSONArray("sent") ?: JSONArray()
        return (0 until a.length()).map { a.optString(it) }
    }
    fun clip(api: GregalApi, text: String, from: String): ClipLink {
        val r = api.post("/api/winds/clip", JSONObject().put("text", text).put("from", from))
        return ClipLink(r.optString("link"), r.optBoolean("private"))
    }
    fun forget(api: GregalApi, id: String) { api.post("/api/winds/inbox", JSONObject().put("id", id).put("action", "forget")) }
    fun markRead(api: GregalApi) { runCatching { api.post("/api/winds/inbox", JSONObject().put("id", "*").put("action", "read")) } }
}

/** El full d'Android amb el text (WhatsApp, Telegram, correu…). */
fun shareTextIntent(ctx: Context, text: String, chooser: String) {
    val send = Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, text)
    ctx.startActivity(Intent.createChooser(send, chooser).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
}

/**
 * La bombolla com a PNG per compartir: fons del tema, marges i una signatura
 * discreta («gregal · Xaloc») a sota. Va a la caché (el FileProvider només
 * exposa la caché) i se n'esborren les velles.
 */
fun shareImageFile(ctx: Context, shot: ImageBitmap, bg: Color, ink: Color, accent: Color, from: String): File {
    val src = shot.asAndroidBitmap().copy(Bitmap.Config.ARGB_8888, false)
    val d = ctx.resources.displayMetrics.density
    val pad = (20 * d).toInt(); val foot = (34 * d).toInt()
    val out = Bitmap.createBitmap(src.width + pad * 2, src.height + pad * 2 + foot, Bitmap.Config.ARGB_8888)
    val c = Canvas(out)
    c.drawColor(bg.toArgb())
    c.drawBitmap(src, pad.toFloat(), pad.toFloat(), null)
    val brand = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = ink.toArgb(); textSize = 15 * d; typeface = Typeface.create(Typeface.DEFAULT, Typeface.BOLD) }
    val y = (src.height + pad * 2 + foot * 0.45f)
    c.drawText("gregal", pad.toFloat(), y, brand)
    val dot = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = accent.toArgb() }
    val w = brand.measureText("gregal")
    c.drawCircle(pad + w + 6 * d, y - 4 * d, 3 * d, dot)
    if (from.isNotBlank()) {
        val meta = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = ink.copy(alpha = 0.6f).toArgb(); textSize = 13 * d }
        c.drawText(from, pad + w + 16 * d, y, meta)
    }
    val dir = File(ctx.cacheDir, "share").apply { mkdirs() }
    dir.listFiles()?.filter { System.currentTimeMillis() - it.lastModified() > 3_600_000L }?.forEach { it.delete() }
    val f = File(dir, "gregal-${System.currentTimeMillis()}.png")
    f.outputStream().use { out.compress(Bitmap.CompressFormat.PNG, 100, it) }
    return f
}

fun shareImageIntent(ctx: Context, file: File, chooser: String) {
    val uri = FileProvider.getUriForFile(ctx, "${ctx.packageName}.fileprovider", file)
    val send = Intent(Intent.ACTION_SEND).setType("image/png").putExtra(Intent.EXTRA_STREAM, uri)
        .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
    ctx.startActivity(Intent.createChooser(send, chooser).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_GRANT_READ_URI_PERMISSION))
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ShareSheet(vm: GregalViewModel, lang: String, target: ShareTarget, onDismiss: () -> Unit) {
    ModalBottomSheet(onDismissRequest = onDismiss, containerColor = SuperficieOpaca) {
        ShareSheetBody(vm, lang, target, onDismiss)
    }
}

/** El contingut del full (a part, perquè es pugui provar sense el full modal). */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ShareSheetBody(vm: GregalViewModel, lang: String, target: ShareTarget, onDismiss: () -> Unit) {
    val ctx = LocalContext.current
    val clipboard = LocalClipboardManager.current
    val scope = rememberCoroutineScope()
    val pal = LocalPal.current
    val imgBg = pal.deepBottom; val imgInk = Ink; val imgAccent = Sea
    val w = { k: String -> ShareText.word(lang, k) }
    val plain = remember(target.text, lang) { ShareText.plain(target.text, lang) }
    var status by remember { mutableStateOf("") }
    var bad by remember { mutableStateOf(false) }
    var people by remember { mutableStateOf<List<String>?>(null) }
    val chosen = remember { mutableStateListOf<String>() }
    var link by remember { mutableStateOf<ClipLink?>(null) }
    var busy by remember { mutableStateOf(false) }
    fun say(msg: String, isBad: Boolean = false) { status = msg; bad = isBad }
    fun run(block: suspend () -> Unit) {
        if (busy) return
        busy = true
        scope.launch { try { block() } catch (e: Exception) { say(e.message.orEmpty().ifBlank { "Error" }, true) } finally { busy = false } }
    }
    Box {
        Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp).padding(bottom = 24.dp).testTag("share-sheet"),
            verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(w("title"), color = Ink, fontWeight = FontWeight.Bold, fontSize = 18.sp, modifier = Modifier.padding(start = 8.dp, bottom = 8.dp))
            ShareOption(Icons.Rounded.Share, w("text"), w("textSub"), "share-text") {
                shareTextIntent(ctx, plain, w("chooser")); onDismiss()
            }
            if (target.capture != null) ShareOption(Icons.Rounded.Image, w("image"), w("imageSub"), "share-image") {
                run {
                    val shot = target.capture.invoke()
                    if (shot == null) { say(w("imageFail"), true); return@run }
                    val f = withContext(Dispatchers.IO) { shareImageFile(ctx, shot, imgBg, imgInk, imgAccent, target.from) }
                    shareImageIntent(ctx, f, w("chooser")); onDismiss()
                }
            }
            ShareOption(Icons.Rounded.ContentCopy, w("copy"), null, "share-copy") {
                clipboard.setText(AnnotatedString(plain)); say(w("copied"))
            }
            HorizontalDivider(Modifier.padding(vertical = 6.dp), color = Ink.copy(alpha = 0.08f))
            ShareOption(Icons.Rounded.PersonAdd, w("person"), w("personSub"), "share-person") {
                run {
                    val got = try { withContext(Dispatchers.IO) { ShareApi.people(vm.conn.value.api()) } }
                        catch (_: Exception) { say(w("noAccounts"), true); return@run }
                    if (got.isEmpty()) say(w("nobody"), true) else { people = got; say("") }
                }
            }
            people?.let { list ->
                FlowRow(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp), horizontalArrangement = Arrangement.spacedBy(6.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    for (n in list) FilterChip(selected = n in chosen, onClick = { if (n in chosen) chosen.remove(n) else chosen.add(n) },
                        label = { Text(n) }, modifier = Modifier.testTag("share-pick-$n"))
                    Button(enabled = chosen.isNotEmpty() && !busy, modifier = Modifier.testTag("share-send"), onClick = {
                        run {
                            val sent = withContext(Dispatchers.IO) { ShareApi.send(vm.conn.value.api(), chosen.toList(), target.text, target.from) }
                            people = null; chosen.clear(); say(w("sent").format(sent.joinToString(", ")))
                        }
                    }) { Text(w("send")) }
                }
            }
            ShareOption(Icons.Rounded.Link, w("link"), w("linkSub"), "share-link") {
                run {
                    say(w("making"))
                    val l = withContext(Dispatchers.IO) { ShareApi.clip(vm.conn.value.api(), target.text, target.from) }
                    link = l
                    clipboard.setText(AnnotatedString(l.link))
                    say(l.link + if (l.private) "\n" + w("private") else "", l.private)
                }
            }
            link?.let { l ->
                TextButton(onClick = { shareTextIntent(ctx, l.link, w("chooser")); onDismiss() }, modifier = Modifier.padding(start = 4.dp).testTag("share-link-send")) {
                    Text(w("shareLink"), color = Sea, fontWeight = FontWeight.SemiBold)
                }
            }
            if (status.isNotBlank()) Text(status, color = if (bad) Sand else Muted, fontSize = 13.sp,
                modifier = Modifier.padding(horizontal = 8.dp, vertical = 6.dp).testTag("share-status"))
        }
    }
}

@Composable
private fun ShareOption(icon: ImageVector, title: String, sub: String?, tag: String, onClick: () -> Unit) {
    Row(Modifier.fillMaxWidth().clip12().clickable(onClick = onClick).padding(horizontal = 8.dp, vertical = 11.dp).testTag(tag),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
        Icon(icon, contentDescription = null, tint = Ink.copy(alpha = 0.8f), modifier = Modifier.size(22.dp))
        Column(Modifier.weight(1f)) {
            Text(title, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.Medium)
            if (sub != null) Text(sub, color = Muted, fontSize = 12.sp)
        }
    }
}

private fun Modifier.clip12() = this.then(Modifier.clip(RoundedCornerShape(12.dp)))

/** El que algú de l'app t'ha enviat, a Desat: el missatge pintat com al xat. */
@Composable
fun ReceivedCard(vm: GregalViewModel, lang: String, s: WindSaved, onRemoved: () -> Unit) {
    var share by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    Surface(color = Ink.copy(alpha = 0.04f), shape = MaterialTheme.shapes.medium,
        border = if (s.unread) BorderStroke(1.dp, Sea.copy(alpha = 0.6f)) else null,
        modifier = Modifier.fillMaxWidth().testTag("saved-received")) {
        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("📨", fontSize = 18.sp)
                Column(Modifier.weight(1f)) {
                    Text(s.title, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp)
                    Text(listOfNotNull(ShareText.word(lang, "from").format(s.from), s.windName.ifBlank { null },
                        if (s.ts > 0) savedDay(lang, s.ts) else null).joinToString(" · "), color = Muted, fontSize = 12.sp)
                }
            }
            WindBubble(GregalViewModel.PetMsg(from = s.windName.ifBlank { "gregal" }, text = s.text), Sea, lang = lang, showMeta = false,
                onCopy = { share = true })
            Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                TextButton(onClick = { share = true }) { Text(ShareText.word(lang, "title"), color = Sea) }
                TextButton(onClick = {
                    scope.launch {
                        runCatching { withContext(Dispatchers.IO) { ShareApi.forget(vm.conn.value.api(), s.id) } }
                        onRemoved()
                    }
                }) { Text(ShareText.word(lang, "remove"), color = Muted) }
            }
        }
    }
    if (share) ShareSheet(vm, lang, ShareTarget(s.text, s.windName), onDismiss = { share = false })
}

/** Captura d'una bombolla: el modifier la grava en una capa i `shot()` en fa la foto. */
class BubbleCapture(val modifier: Modifier, val shot: suspend () -> ImageBitmap?)

@Composable
fun rememberBubbleCapture(): BubbleCapture {
    val layer = androidx.compose.ui.graphics.rememberGraphicsLayer()
    return remember(layer) {
        BubbleCapture(
            Modifier.drawWithContent {
                layer.record { this@drawWithContent.drawContent() }
                drawLayer(layer)
            },
        ) { runCatching { layer.toImageBitmap() }.getOrNull() }
    }
}
