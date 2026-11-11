package xyz.solanes.gregal

import android.graphics.BitmapFactory
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import kotlin.math.min

/**
 * B13 — on actua l'agent a l'ordinador, al mòbil.
 *
 * En directe, el visor (WebView de /ordinador/visor/) ja dibuixa el cursor de
 * l'agent, el halo del clic, el camp on escriu, la traça i la franja: és la
 * mateixa pàgina que la web. Aquí hi ha la reproducció nativa de les últimes
 * accions d'una feina (GET /api/ordinador/accions), amb la captura i el punt
 * marcat, per quan no s'hi estava mirant. Les funcions pures (fitRect,
 * toScreen, fade) són les mateixes que les de la pàgina del visor.
 */

/** Color de l'agent: viu i diferent del cursor de la persona. */
internal val AgentColor = Color(0xFFFF2D95)

internal data class ComputerAction(
    val id: Long = 0,
    val type: String = "click",
    val x: Float = 0f,
    val y: Float = 0f,
    val toX: Float? = null,
    val toY: Float? = null,
    /** Caixa de l'objectiu [x, y, w, h] en fraccions del viewport, o null. */
    val rect: List<Float>? = null,
    val dir: String = "",
    val target: String = "",
    val text: String = "",
    val masked: Boolean = false,
    val viewW: Float = 0f,
    val viewH: Float = 0f,
    val screenW: Float = 0f,
    val screenH: Float = 0f,
    val left: Float = 0f,
    val top: Float = 0f,
    val step: String = "",
    val at: String = "",
    val run: Int = 0,
    val shotUrl: String = "",
)

internal fun parseComputerAction(o: JSONObject): ComputerAction {
    val v = o.optJSONObject("view") ?: JSONObject()
    val r = o.optJSONArray("rect")
    return ComputerAction(
        id = o.optLong("id"),
        type = o.optString("type").ifBlank { "click" },
        x = o.optDouble("x", 0.0).toFloat(),
        y = o.optDouble("y", 0.0).toFloat(),
        toX = if (o.has("toX")) o.optDouble("toX").toFloat() else null,
        toY = if (o.has("toY")) o.optDouble("toY").toFloat() else null,
        rect = if (r != null && r.length() == 4) List(4) { r.optDouble(it, 0.0).toFloat() } else null,
        dir = o.optString("dir"),
        target = o.optString("target"),
        text = o.optString("text"),
        masked = o.optBoolean("masked"),
        viewW = v.optDouble("w", 0.0).toFloat(),
        viewH = v.optDouble("h", 0.0).toFloat(),
        screenW = v.optDouble("screenW", 0.0).toFloat(),
        screenH = v.optDouble("screenH", 0.0).toFloat(),
        left = v.optDouble("left", 0.0).toFloat(),
        top = v.optDouble("top", 0.0).toFloat(),
        step = o.optString("step"),
        at = o.optString("at"),
        run = o.optInt("run"),
        shotUrl = o.optString("shotUrl"),
    )
}

internal fun parseComputerActions(o: JSONObject): List<ComputerAction> {
    val a = o.optJSONArray("actions") ?: return emptyList()
    return List(a.length()) { parseComputerAction(a.getJSONObject(it)) }
}

/** On queda una imatge iw×ih encabida (ContentScale.Fit) en una caixa bw×bh. */
internal data class FitBox(val x: Float, val y: Float, val w: Float, val h: Float) {
    /** Un punt en fraccions de la imatge, en píxels de la caixa. */
    fun map(fx: Float, fy: Float): Offset = Offset(x + fx * w, y + fy * h)
}

internal fun fitRect(bw: Float, bh: Float, iw: Float, ih: Float): FitBox {
    if (!(bw > 0f && bh > 0f && iw > 0f && ih > 0f)) return FitBox(0f, 0f, maxOf(bw, 0f), maxOf(bh, 0f))
    val s = min(bw / iw, bh / ih)
    val w = iw * s
    val h = ih * s
    return FitBox((bw - w) / 2f, (bh - h) / 2f, w, h)
}

/** Un punt del viewport (fraccions) en fraccions de la pantalla de l'ordinador. */
internal fun toScreen(a: ComputerAction, fx: Float, fy: Float): Offset {
    val sw = if (a.screenW > 0f) a.screenW else a.viewW
    val sh = if (a.screenH > 0f) a.screenH else a.viewH
    if (sw <= 0f || sh <= 0f) return Offset(fx, fy)
    return Offset((a.left + fx * a.viewW) / sw, (a.top + fy * a.viewH) / sh)
}

/** Opacitat d'un ressalt que té ageMs i en dura durMs: ple al principi, s'apaga de pressa al final. */
internal fun fade(ageMs: Float, durMs: Float): Float {
    if (ageMs <= 0f) return 1f
    if (durMs <= 0f || ageMs >= durMs) return 0f
    val t = ageMs / durMs
    return 1f - t * t
}

/** Quant dura el ressalt de cada tipus d'acció (ms), com al visor web. */
internal fun actionDurationMs(type: String): Int = when (type) {
    "type" -> 2600
    "scroll" -> 800
    "drag" -> 1300
    else -> 900
}

private fun q(t: String) = "«" + t.take(60) + "»"

/** La frase planera de l'acció («Clica «Reserva»»), en català o anglès. */
internal fun describeComputerAction(lang: String, a: ComputerAction): String {
    val en = lang == "en"
    return when (a.type) {
        "type" -> when {
            a.text.isNotBlank() -> (if (en) "Types " else "Escriu ") + q(a.text) + (if (a.target.isNotBlank()) (if (en) " in " else " a ") + q(a.target) else "")
            else -> (if (en) "Types in " else "Escriu a ") + q(a.target)
        }
        "select" -> (if (en) "Picks " else "Tria ") + q(a.text) + (if (a.target.isNotBlank()) (if (en) " in " else " a ") + q(a.target) else "")
        "scroll" -> when (a.dir) {
            "up" -> if (en) "Scrolls up" else "Desplaça amunt"
            "left" -> if (en) "Scrolls left" else "Desplaça a l'esquerra"
            "right" -> if (en) "Scrolls right" else "Desplaça a la dreta"
            else -> if (en) "Scrolls down" else "Desplaça avall"
        }
        "drag" -> (if (en) "Drags " else "Arrossega ") + a.target
        else -> if (a.target.isNotBlank()) (if (en) "Clicks " else "Clica ") + q(a.target) else (if (en) "Clicks on the screen" else "Clica a la pantalla")
    }
}

internal fun accionsCopy(lang: String, key: String): String {
    val en = lang == "en"
    return when (key) {
        "last" -> if (en) "Last actions" else "Últimes accions"
        "none" -> if (en) "Gregal has not done anything on this computer yet." else "Gregal encara no ha fet res en aquest ordinador."
        "loading" -> if (en) "Loading…" else "Carregant…"
        "error" -> if (en) "Could not load the actions." else "No s'han pogut carregar les accions."
        "play" -> if (en) "Play" else "Reprodueix"
        "pause" -> if (en) "Pause" else "Pausa"
        "prev" -> if (en) "Previous action" else "Acció anterior"
        "next" -> if (en) "Next action" else "Acció següent"
        "close" -> if (en) "Close" else "Tanca"
        "step" -> if (en) "Step" else "Pas"
        "of" -> if (en) " of " else " de "
        "noShot" -> if (en) "No screenshot for this action." else "Aquesta acció no té captura."
        else -> key
    }
}

/** «3 de 8 · 17:20:31 · Pas 2/4 · Omple les dades». */
internal fun replayCaption(lang: String, a: ComputerAction, index: Int, total: Int, zone: ZoneId = ZoneId.systemDefault()): String {
    val time = runCatching { DateTimeFormatter.ofPattern("HH:mm:ss").withZone(zone).format(Instant.parse(a.at)) }.getOrDefault("")
    val step = if (a.step.isNotBlank()) accionsCopy(lang, "step") + " " + a.step else ""
    return listOf("${index + 1}${accionsCopy(lang, "of")}$total", time, step).filter { it.isNotBlank() }.joinToString(" · ")
}

/** GET /api/ordinador/accions: les últimes accions de l'agent (de la més vella a la més nova). */
internal fun GregalApi.ordinadorAccions(): List<ComputerAction> = parseComputerActions(get("/api/ordinador/accions"))

/** Reproducció: captura marcada de cada acció, amb anterior/següent i reproducció automàtica. */
@Composable
internal fun OrdinadorReplay(conn: Conn, lang: String, accent: Color, onClose: () -> Unit) {
    var list by remember { mutableStateOf<List<ComputerAction>?>(null) }
    var error by remember { mutableStateOf(false) }
    var i by remember { mutableIntStateOf(0) }
    var playing by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) {
        try {
            val l = withContext(Dispatchers.IO) { conn.api().ordinadorAccions() }
            list = l
            playing = l.size > 1
        } catch (e: Exception) {
            if (e is CancellationException) throw e
            error = true
            list = emptyList()
        }
    }
    val items = list.orEmpty()
    LaunchedEffect(playing, i, items.size) {
        if (!playing) return@LaunchedEffect
        delay(1600)
        if (i < items.lastIndex) i++ else playing = false
    }
    Column(Modifier.fillMaxSize().testTag("ordinador-replay")) {
        val a = items.getOrNull(i)
        Box(Modifier.weight(1f).fillMaxWidth()) {
            when {
                list == null -> Text(accionsCopy(lang, "loading"), color = Muted, fontSize = 13.sp, modifier = Modifier.align(Alignment.Center))
                a == null -> Text(accionsCopy(lang, if (error) "error" else "none"), color = Muted, fontSize = 13.sp,
                    modifier = Modifier.align(Alignment.Center).padding(24.dp).testTag("ordinador-replay-empty"))
                else -> ReplayFrame(conn, a)
            }
        }
        Row(Modifier.fillMaxWidth().padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            if (items.size > 1) {
                ReplayButton("◀", accionsCopy(lang, "prev")) { playing = false; if (i > 0) i-- }
                ReplayButton(accionsCopy(lang, if (playing) "pause" else "play"), null) {
                    if (!playing && i >= items.lastIndex) i = 0
                    playing = !playing
                }
                ReplayButton("▶", accionsCopy(lang, "next")) { playing = false; if (i < items.lastIndex) i++ }
            }
            Column(Modifier.weight(1f).semantics { liveRegion = LiveRegionMode.Polite }) {
                if (a != null) {
                    Text(describeComputerAction(lang, a), color = Ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold,
                        maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.testTag("ordinador-replay-text"))
                    Text(replayCaption(lang, a, i, items.size), color = Muted, fontSize = 11.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
            ReplayButton(accionsCopy(lang, "close"), null, accent, onClose)
        }
    }
}

@Composable
private fun ReplayButton(label: String, description: String?, accent: Color = Color.Unspecified, onClick: () -> Unit) {
    Box(
        Modifier.heightIn(min = 48.dp).widthIn(min = 48.dp).clip(GregalShape.sm)
            .then(if (accent != Color.Unspecified) Modifier.background(accent) else Modifier.border(1.dp, Line, GregalShape.sm))
            .semantics(mergeDescendants = true) {
                role = Role.Button
                if (description != null) contentDescription = description
            }
            .clickable(onClick = onClick).padding(horizontal = 12.dp),
        contentAlignment = Alignment.Center,
    ) { Text(label, color = if (accent != Color.Unspecified) OnSea else Ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold) }
}

@Composable
private fun ReplayFrame(conn: Conn, a: ComputerAction) {
    var bmp by remember(a.shotUrl) { mutableStateOf<ImageBitmap?>(null) }
    LaunchedEffect(a.shotUrl) {
        if (a.shotUrl.isBlank()) return@LaunchedEffect
        bmp = try {
            withContext(Dispatchers.IO) {
                val bytes = conn.api().ordinadorShot(a.shotUrl)
                BitmapFactory.decodeByteArray(bytes, 0, bytes.size)?.asImageBitmap()
            }
        } catch (e: Exception) {
            if (e is CancellationException) throw e
            null
        }
    }
    val dur = actionDurationMs(a.type).toFloat()
    val age = remember(a.id, a.at) { Animatable(0f) }
    LaunchedEffect(a.id, a.at) { age.animateTo(dur, tween(dur.toInt(), easing = LinearEasing)) }
    Box(Modifier.fillMaxSize().clip(GregalShape.sm).background(SuperficieAlta)) {
        bmp?.let { Image(it, null, contentScale = ContentScale.Fit, modifier = Modifier.fillMaxSize()) }
        Canvas(Modifier.fillMaxSize().testTag("ordinador-replay-mark")) {
            val iw = bmp?.width?.toFloat() ?: a.viewW
            val ih = bmp?.height?.toFloat() ?: a.viewH
            val box = fitRect(size.width, size.height, iw, ih)
            drawComputerAction(a, box, age.value, dur)
        }
    }
}

/** Pinta el ressalt d'una acció sobre una captura del viewport (fraccions → caixa). */
internal fun DrawScope.drawComputerAction(a: ComputerAction, box: FitBox, ageMs: Float, durMs: Float) {
    val done = ageMs >= durMs
    val alpha = if (done) 1f else fade(ageMs, durMs).coerceAtLeast(.35f)
    val t = if (durMs > 0f) (ageMs / durMs).coerceIn(0f, 1f) else 1f
    val p = box.map(a.x, a.y)
    a.rect?.let { r ->
        val p1 = box.map(r[0], r[1])
        val p2 = box.map(r[0] + r[2], r[1] + r[3])
        val tl = Offset(p1.x - 3f, p1.y - 3f)
        val sz = Size(p2.x - p1.x + 6f, p2.y - p1.y + 6f)
        if (a.type == "type" || a.type == "select") drawRect(AgentColor.copy(alpha = .16f * alpha), tl, sz)
        drawRect(AgentColor.copy(alpha = alpha * if (a.type == "click") .6f else 1f), tl, sz, style = Stroke(width = 3.dp.toPx()))
    }
    when (a.type) {
        "click", "select" -> {
            val r = if (done) 18.dp.toPx() else (8f + 26f * t).dp.toPx()
            drawCircle(AgentColor.copy(alpha = alpha), r, p, style = Stroke(width = 4.dp.toPx()))
            drawCircle(AgentColor.copy(alpha = .35f * alpha), 7.dp.toPx(), p)
        }
        "scroll" -> {
            val d = when (a.dir) { "up" -> Offset(0f, -1f); "left" -> Offset(-1f, 0f); "right" -> Offset(1f, 0f); else -> Offset(0f, 1f) }
            val len = 64.dp.toPx() * if (done) 1f else min(1f, t * 1.6f)
            drawLine(AgentColor.copy(alpha = alpha), p, p + d * len, strokeWidth = 5.dp.toPx(), cap = StrokeCap.Round)
        }
        "drag" -> if (a.toX != null && a.toY != null) {
            val q2 = box.map(a.toX, a.toY)
            val k = if (done) 1f else min(1f, t * 1.4f)
            drawLine(AgentColor.copy(alpha = alpha), p, p + (q2 - p) * k, strokeWidth = 4.dp.toPx(), cap = StrokeCap.Round,
                pathEffect = PathEffect.dashPathEffect(floatArrayOf(10.dp.toPx(), 6.dp.toPx())))
            drawCircle(AgentColor.copy(alpha = alpha), 6.dp.toPx(), p)
        }
    }
    val end = if (a.type == "drag" && a.toX != null && a.toY != null) box.map(a.toX, a.toY) else p
    drawAgentCursor(end)
}

/** El cursor de l'agent: fletxa magenta amb vora blanca. */
private fun DrawScope.drawAgentCursor(at: Offset) {
    val s = 1.dp.toPx()
    val path = Path().apply {
        moveTo(at.x, at.y)
        lineTo(at.x, at.y + 22 * s); lineTo(at.x + 6 * s, at.y + 16.5f * s); lineTo(at.x + 10.5f * s, at.y + 26 * s)
        lineTo(at.x + 14 * s, at.y + 24.5f * s); lineTo(at.x + 9.6f * s, at.y + 15.2f * s); lineTo(at.x + 17 * s, at.y + 15.2f * s)
        close()
    }
    drawPath(path, AgentColor)
    drawPath(path, Color.White, style = Stroke(width = 2 * s))
}
