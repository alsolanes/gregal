package xyz.solanes.gregal

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.ArrowUpward
import androidx.compose.material.icons.rounded.Description
import androidx.compose.material.icons.rounded.FormatQuote
import androidx.compose.material.icons.rounded.Language
import androidx.compose.material.icons.rounded.Shield
import androidx.compose.material.icons.rounded.HelpOutline
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import kotlin.math.PI
import kotlin.math.sin

/**
 * Preguntes i aprovacions dels vents. Un vent que necessita una decisió (o
 * permís per instal·lar, baixar o esborrar) s'atura: el servidor ho publica i
 * aquí surt una targeta amb botons. Fins que no la contestes, el torn no
 * continua. Si has tancat l'app, la targeta és a /api/winds/pending i es
 * torna a trobar en obrir el xat.
 *
 * A més: un bloc \`\`\`choices al final del missatge del vent es pinta com a
 * botons de resposta ràpida (i no surt com a text).
 */

internal fun askUi(lang: String, key: String): String {
    val en = lang == "en"
    return when (key) {
        "allow" -> if (en) "Allow" else "Permet"
        "decline" -> if (en) "Decline" else "Rebutja"
        "other" -> if (en) "Something else…" else "Una altra cosa…"
        "send" -> if (en) "Send" else "Envia"
        "waiting" -> if (en) "Waiting for you" else "Espera la teva resposta"
        "install" -> if (en) "Wants to install software" else "Vol instal·lar programari"
        "download" -> if (en) "Wants to download a package or file" else "Vol baixar un paquet o un fitxer"
        "delete" -> if (en) "Wants to delete outside the project" else "Vol esborrar fora del projecte"
        "system" -> if (en) "Wants to change a system service" else "Vol canviar un servei del sistema"
        "approve" -> if (en) "Asks for your permission" else "Et demana permís"
        "no" -> "No"
        "allowed" -> if (en) "You allowed: " else "Has permès: "
        "declined" -> if (en) "You declined: " else "Has dit que no: "
        else -> key
    }
}

/** Les respostes ràpides del missatge: fins a 4 opcions curtes, una per línia. */
internal fun extractChoices(text: String): List<String> {
    val start = text.lastIndexOf("```choices")
    if (start < 0) return emptyList()
    val body = text.substring(start + "```choices".length).substringBefore("```")
    return body.lines()
        .map { it.trim().removePrefix("-").removePrefix("*").trim().replace(Regex("^\\d+[.)]\\s+"), "").trim() }
        .filter { it.isNotEmpty() && it.length <= 60 }
        .distinct().take(4)
}

/** El missatge sense el bloc de respostes ràpides (també si encara arriba en directe i no s'ha tancat). */
internal fun withoutChoices(text: String): String {
    val start = text.lastIndexOf("```choices")
    if (start < 0) return text
    val after = text.substring(start + "```choices".length)
    val close = after.indexOf("```")
    val tail = if (close >= 0) after.substring(close + 3) else ""
    return (text.substring(0, start).trimEnd() + (if (tail.isBlank()) "" else "\n" + tail.trim())).trimEnd()
}

/** Les targetes dels vents que esperen: se'n tornen a buscar mentre el vent treballa. */
@Composable
internal fun WindAskCards(vm: GregalViewModel, lang: String, accent: Color, busy: Boolean, wind: Wind? = null,
    /** Xat de [wind]: la seva cara ja hi és, una de sola (109). No es repeteix a les targetes i és ella qui assenteix o fa que no. */
    onOwnAnswer: ((Boolean) -> Unit)? = null) {
    val asks by vm.windAsks.collectAsState()
    val winds by vm.winds.collectAsState()
    // Les aprovacions que ja has contestat queden plegades en una línia (sí o no).
    val answered = remember { mutableStateListOf<Pair<GregalApi.WindAsk, Boolean>>() }
    // En obrir el xat i mentre un vent treballa: potser n'hi ha una d'abans.
    // Només amb l'app a la vista: en segon pla el servidor ja avisa amb la notificació.
    LifecyclePoll(busy) {
        vm.loadWindAsks()
        while (busy) { delay(4_000); vm.loadWindAsks() }
    }
    if (asks.isEmpty() && answered.isEmpty()) return
    fun own(a: GregalApi.WindAsk) = onOwnAnswer != null && (winds.firstOrNull { it.id == a.wind } ?: wind)?.id == wind?.id
    fun faceOf(a: GregalApi.WindAsk) = if (own(a)) null else winds.firstOrNull { it.id == a.wind } ?: wind
    // Les targetes (menys les preguntes, com al web) són del color del vent que les demana, també a Feina i en un grup.
    fun colourOf(a: GregalApi.WindAsk) = (winds.firstOrNull { it.id == a.wind } ?: wind)?.let { Color(it.argb) } ?: accent
    fun answer(a: GregalApi.WindAsk, ok: Boolean) {
        answered.add(a to ok)
        if (own(a)) onOwnAnswer?.invoke(ok)
        vm.answerWindAsk(a.key, approve = ok)
    }
    Column(Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 6.dp).testTag("wind-asks"),
        verticalArrangement = Arrangement.spacedBy(8.dp)) {
        answered.forEach { (a, ok) -> AnsweredLine(lang, a, ok, faceOf(a)) }
        asks.forEach { a ->
            if (a.type == "handoff") HandoffCard(vm.conn.value, lang, a, colourOf(a),
                onDone = { vm.answerWindAsk(a.key, approve = true) }, onCancel = { vm.answerWindAsk(a.key, approve = false) })
            else if (a.type == "connection") ConnexioWaitCard(vm.conn.value, lang, a, colourOf(a),
                onCancel = { vm.answerWindAsk(a.key, approve = false) })
            else if (a.type == "secret") SecretCard(lang, a, colourOf(a),
                onSave = { v -> vm.answerWindAsk(a.key, approve = true, secret = v) }, onDecline = { vm.answerWindAsk(a.key, approve = false) })
            else if (a.type == "question") QuestionCard(lang, a, accent) { ans -> vm.answerWindAsk(a.key, answer = ans) }
            else ApproveCard(lang, a, colourOf(a), faceOf(a),
                onAllow = { answer(a, true) }, onDecline = { answer(a, false) })
        }
    }
}

@Composable
private fun AskShell(accent: Color, tag: String, edge: Boolean = false, content: @Composable ColumnScope.() -> Unit) {
    Column(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(22.dp)).background(accent.copy(alpha = .10f).compositeOverSafe(SuperficieAlta.copy(alpha = .6f)))
            .then(if (edge) Modifier.drawBehind { drawRect(accent, size = Size(3.dp.toPx(), size.height)) } else Modifier)
            .border(1.dp, accent.copy(alpha = .45f), RoundedCornerShape(22.dp)).padding(16.dp).testTag(tag),
        verticalArrangement = Arrangement.spacedBy(10.dp), content = content,
    )
}

private fun Color.compositeOverSafe(bg: Color): Color = Color(
    red = red * alpha + bg.red * (1 - alpha), green = green * alpha + bg.green * (1 - alpha),
    blue = blue * alpha + bg.blue * (1 - alpha), alpha = maxOf(alpha, bg.alpha),
)

/** Què es pregunta: la pregunta en pla, i on és i si es pot desfer. */
internal data class ApprovePlan(val question: String, val sub: String)

private fun askWhere(path: String, en: Boolean): Pair<String, String> {
    val full = path.replace('\\', '/')
    val parts = full.split('/').toMutableList()
    val file = parts.removeLastOrNull()?.ifBlank { null } ?: full
    val joined = parts.joinToString("/")
    val k = Regex("/gregal/projectes(/|$)", RegexOption.IGNORE_CASE).find(joined)?.range?.first ?: -1
    val dir = if (k >= 0) (if (en) "Projects" else "Projectes") + joined.substring(k + 17).let { if (it.isNotEmpty()) " › " + it.removePrefix("/") else "" }
        else parts.takeLast(2).joinToString("/")
    return file to dir
}

private fun askPath(s: String): String {
    val quoted = Regex("\"([^\"]+)\"|'([^']+)'").findAll(s).map { it.groupValues[1].ifEmpty { it.groupValues[2] } }.toList()
    return quoted.lastOrNull() ?: s.trim().split(Regex("\\s+")).lastOrNull().orEmpty()
}

/** Pura i provada (ApprovePlanTest): la mateixa lògica que askPlan a la web. */
internal fun approvePlan(lang: String, kind: String, what: String, name: String = ""): ApprovePlan {
    val en = lang == "en"
    fun t(ca: String, e: String) = if (en) e else ca
    val w = what.trim()
    Regex("^(write|edit|patch)\\s+(.+)$").find(w)?.let { m ->
        val (file, dir) = askWhere(m.groupValues[2], en)
        val where = if (dir.isNotEmpty()) t("És a $dir.", "It is in $dir.") else ""
        return if (m.groupValues[1] == "write") ApprovePlan(t("Vols que escrigui $file?", "Shall I write $file?"), where)
        else ApprovePlan(t("Vols que modifiqui $file?", "Shall I edit $file?"), where)
    }
    return when (kind) {
        "delete" -> {
            val (file, dir) = askWhere(askPath(w), en)
            ApprovePlan(t("Vols que esborri $file?", "Shall I delete $file?"),
                t((if (dir.isNotEmpty()) "És a $dir, fora" else "És fora") + " del projecte. No es podrà desfer.",
                    (if (dir.isNotEmpty()) "It is in $dir, outside" else "It is outside") + " the project. It cannot be undone."))
        }
        "install" -> ApprovePlan(t("Vols que instal·li programari?", "Shall I install software?"), t("Programari nou a l'ordinador.", "New software on the computer."))
        "download" -> ApprovePlan(t("Vols que baixi un paquet o un fitxer?", "Shall I download a package or file?"), t("De fora, a l'ordinador.", "From outside, onto the computer."))
        "system" -> ApprovePlan(t("Vols que canviï un servei del sistema?", "Shall I change a system service?"), t("Afecta tot l'ordinador, no només el projecte.", "It affects the whole computer, not just the project."))
        "publish_public" -> ApprovePlan(t("Vols que publiqui la web?", "Shall I publish the site?"), t("Serà pública: qualsevol que tingui l'enllaç la veurà.", "It will be public: anyone with the link can see it."))
        // Una regla teva («Pregunta'm abans»): la targeta diu quina a la línia del perquè.
        "rule" -> ApprovePlan(when (name) {
            "mail" -> t("Vols que enviï aquest correu?", "Shall I send this email?")
            "browser" -> t("Vols que faci aquest pas al navegador?", "Shall I take this step in the browser?")
            "bash", "bash_background" -> t("Vols que executi aquesta ordre?", "Shall I run this command?")
            "publish" -> t("Vols que publiqui la web?", "Shall I publish the site?")
            "calendar" -> t("Vols que canviï el calendari?", "Shall I change the calendar?")
            "remind" -> t("Vols que programi l'avís?", "Shall I set the reminder?")
            else -> t("Vols que ho faci?", "Shall I do it?")
        }, "")
        // «other» i els tipus que no coneixem: el títol genèric. (askUi("other") és el
        // camp lliure de les preguntes, «Una altra cosa…»: sortia com a títol.)
        else -> ApprovePlan(askUi(lang, "approve"), "")
    }
}

/**
 * El que es veurà abans de dir que sí: una pàgina (url), un document (doc) o el
 * text que farà servir (text). Només amb el que /api/winds/pending envia (what,
 * detail, name): el servidor no hi posa captures ni imports. Null per a les ordres,
 * que van plegades a «Veure l'ordre».
 */
internal data class AskPreview(val kind: String, val title: String, val body: String)

internal fun askPreview(lang: String, a: GregalApi.WindAsk): AskPreview? {
    val src = a.detail.ifBlank { a.what }.trim()
    Regex("https?://[^\\s\"'<>]+").find(src)?.let { m ->
        val host = runCatching { java.net.URI(m.value).host }.getOrNull()?.removePrefix("www.") ?: m.value
        return AskPreview("url", host, m.value)
    }
    Regex("^(write|edit|patch)\\s+(.+)$").find(a.what.trim())?.let { m ->
        val (file, dir) = askWhere(m.groupValues[2], lang == "en")
        return AskPreview("doc", file, dir)
    }
    if (a.name in setOf("bash", "bash_background") || a.kind in setOf("install", "download", "delete", "system")) return null
    return if (src.isNotEmpty() && src != a.name) AskPreview("text", "", src) else null
}

@Composable
private fun AskPreviewBox(p: AskPreview, accent: Color) {
    Row(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(Superficie.copy(alpha = .7f))
            .border(1.dp, Line, RoundedCornerShape(14.dp)).padding(12.dp).testTag("ask-preview-" + p.kind),
        horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.Top,
    ) {
        val icon = when (p.kind) { "url" -> Icons.Rounded.Language; "doc" -> Icons.Rounded.Description; else -> Icons.Rounded.FormatQuote }
        Icon(icon, null, tint = accent, modifier = Modifier.size(18.dp))
        Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
            if (p.title.isNotEmpty()) Text(p.title, color = Ink, fontSize = GregalSize.body, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (p.body.isNotEmpty()) Text(p.body, color = if (p.title.isEmpty()) Ink else Muted, fontSize = GregalSize.small, lineHeight = 18.sp,
                maxLines = if (p.kind == "text") 8 else 2, overflow = TextOverflow.Ellipsis)
        }
    }
}

/** El pas en què és el vent quan s'atura a preguntar, amb les mateixes paraules que el directe. */
private fun askStep(lang: String, a: GregalApi.WindAsk): String =
    (if (a.name.isNotBlank()) windProgressLabel(lang, WindLive(phase = "tool", tool = a.name)) + " · " else "") + askUi(lang, "waiting")

/** Contestada: una línia, i la cara del vent assenteix (sí) o fa que no amb el cap (no). */
@Composable
private fun AnsweredLine(lang: String, a: GregalApi.WindAsk, ok: Boolean, face: Wind?) {
    val what = remember(a.key, lang) { askPreview(lang, a)?.title?.ifEmpty { null } ?: a.what.ifBlank { a.name } }
    Row(Modifier.fillMaxWidth().padding(horizontal = 6.dp).semantics(mergeDescendants = true) {}.testTag("ask-answered"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        if (face != null) {
            val motion = LocalWindMotion.current
            val t = remember(a.key) { Animatable(0f) }
            LaunchedEffect(a.key) { if (motion) t.animateTo(1f, tween(900)) }
            WindFace(size = 22, mood = if (ok) WindMood.HAPPY else WindMood.IDLE, accent = Color(face.argb), style = face.faceStyle,
                modifier = Modifier.graphicsLayer {
                    // Tres cops que s'apaguen: amunt i avall per al sí, de costat per al no.
                    val swing = sin(t.value * 6f * PI.toFloat()) * (1f - t.value) * 3.dp.toPx()
                    if (ok) translationY = swing else translationX = swing
                }.testTag(if (ok) "ask-face-nod" else "ask-face-shake"))
        }
        Text(askUi(lang, if (ok) "allowed" else "declined") + what, color = Muted, fontSize = GregalSize.small, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

/**
 * Targeta de permís com a la maqueta: una pregunta en pla («Vols que esborri x?»),
 * on és i si es pot desfer, què es veurà (pàgina, document o text) o l'ordre
 * plegada, i Permet / No. La cara del vent seu a sobre de la targeta amb el pas
 * en què s'ha aturat.
 */
@Composable
private fun ApproveCard(lang: String, a: GregalApi.WindAsk, accent: Color, face: Wind?, onAllow: () -> Unit, onDecline: () -> Unit) {
    val plan = remember(a.key, lang) { approvePlan(lang, a.kind, a.what, a.name) }
    val preview = remember(a.key, lang) { askPreview(lang, a) }
    var showCmd by remember(a.key) { mutableStateOf(false) }
    val full = a.detail.ifBlank { a.what }
    Box(Modifier.fillMaxWidth().padding(top = if (face != null) 16.dp else 0.dp)) {
        AskShell(accent, "approve-card", edge = true) {
            Text(askStep(lang, a), color = Muted, fontSize = GregalSize.caption, maxLines = 1, overflow = TextOverflow.Ellipsis,
                modifier = Modifier.padding(start = if (face != null) 42.dp else 0.dp).testTag("ask-step"))
            Row(verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                // La cara del vent ja és a la targeta; l'escut, per a un vent que no coneixem (o el del xat, que és a la cantonada).
                if (face == null) Icon(Icons.Rounded.Shield, null, tint = accent, modifier = Modifier.size(22.dp).testTag("ask-shield"))
                Column {
                    Text(plan.question, color = Ink, fontSize = GregalSize.body, fontWeight = FontWeight.SemiBold)
                    if (plan.sub.isNotEmpty()) Text(plan.sub, color = Muted, fontSize = GregalSize.small, lineHeight = 18.sp)
                    if (a.reason.isNotBlank()) Text(a.reason, color = Muted, fontSize = GregalSize.small, lineHeight = 18.sp, maxLines = 1,
                        overflow = TextOverflow.Ellipsis, modifier = Modifier.testTag("ask-why"))
                }
            }
            if (preview != null) AskPreviewBox(preview, accent)
            else if (full.isNotBlank()) {
                Text((if (showCmd) "▾ " else "▸ ") + if (lang == "en") "See the command" else "Veure l'ordre", color = Muted, fontSize = GregalSize.caption,
                    modifier = Modifier.clickable { showCmd = !showCmd }.padding(vertical = 2.dp).testTag("ask-command-toggle"))
                if (showCmd) Text(
                    full, color = Ink, fontSize = GregalSize.caption, fontFamily = FontFamily.Monospace,
                    modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(Superficie.copy(alpha = .7f)).padding(10.dp),
                )
            }
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp), modifier = Modifier.fillMaxWidth()) {
                AskButton(askUi(lang, "allow"), primary = true, accent = accent, modifier = Modifier.weight(1f).testTag("ask-allow"), onClick = onAllow)
                AskButton(askUi(lang, "no"), primary = false, modifier = Modifier.weight(1f).testTag("ask-decline"), onClick = onDecline)
            }
        }
        if (face != null) WindFace(size = 36, accent = Color(face.argb), style = face.faceStyle,
            modifier = Modifier.offset(x = 14.dp, y = (-16).dp).testTag("ask-face"))
    }
}

@Composable
private fun QuestionCard(lang: String, a: GregalApi.WindAsk, accent: Color, onAnswer: (String) -> Unit) {
    var free by remember(a.key) { mutableStateOf("") }
    AskShell(accent, "question-card") {
        Row(verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Icon(Icons.Rounded.HelpOutline, null, tint = accent, modifier = Modifier.size(22.dp).padding(top = 1.dp))
            Text(a.query, color = Ink, fontSize = GregalSize.body, fontWeight = FontWeight.SemiBold, lineHeight = 20.sp)
        }
        a.options.forEach { (label, desc) ->
            Column(
                Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(Superficie.copy(alpha = .7f))
                    .border(1.dp, Line, RoundedCornerShape(14.dp)).clickable { onAnswer(label) }.padding(horizontal = 14.dp, vertical = 11.dp).testTag("ask-option"),
            ) {
                Text(label, color = Ink, fontSize = GregalSize.body, fontWeight = FontWeight.Medium)
                if (desc.isNotBlank()) Text(desc, color = Muted, fontSize = GregalSize.caption)
            }
        }
        Row(
            Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(Superficie.copy(alpha = .7f)).border(1.dp, Line, RoundedCornerShape(14.dp))
                .padding(start = 14.dp, end = 4.dp, top = 4.dp, bottom = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            BasicTextField(
                free, { free = it }, singleLine = true, textStyle = TextStyle(color = Ink, fontSize = GregalSize.body), cursorBrush = SolidColor(Ink),
                modifier = Modifier.weight(1f).testTag("ask-free"),
                decorationBox = { inner -> Box { if (free.isEmpty()) Text(askUi(lang, "other"), color = Muted, fontSize = GregalSize.body); inner() } },
            )
            Box(
                Modifier.size(36.dp).clip(RoundedCornerShape(50)).background(if (free.isNotBlank()) Ink else Ink.copy(alpha = .12f))
                    .clickable(enabled = free.isNotBlank()) { onAnswer(free.trim()) },
                contentAlignment = Alignment.Center,
            ) { Icon(Icons.Rounded.ArrowUpward, askUi(lang, "send"), tint = if (free.isNotBlank()) Superficie else Muted, modifier = Modifier.size(18.dp)) }
        }
    }
}

/**
 * Permet en el color del vent (com allowPaint al web): de les dues tintes del tema
 * ([ink] i [bg]), la que deixa més color del vent; el fons es barreja cap a l'altra
 * només fins que el text arriba a 4.5:1. Torna el fons i la tinta del text.
 */
internal fun allowPaint(color: Color, ink: Color, bg: Color): Pair<Color, Color> {
    var best: Triple<Color, Color, Int>? = null
    for ((text, other) in listOf(ink to bg, bg to ink)) for (p in 100 downTo 0 step 2) {
        val f = p / 100f
        val fill = if (p == 100) color else Color(other.red + (color.red - other.red) * f, other.green + (color.green - other.green) * f,
            other.blue + (color.blue - other.blue) * f)
        if (contrastWith(fill, text) >= 4.5) {
            if (best == null || p > best.third) best = Triple(fill, text, p)
            break
        }
    }
    return best!!.let { it.first to it.second }
}

@Composable
private fun AskButton(label: String, primary: Boolean, modifier: Modifier = Modifier, accent: Color = Color.Unspecified, onClick: () -> Unit) {
    val pal = LocalPal.current
    val (bg, fg) = if (primary) remember(accent, pal) { allowPaint(accent, pal.ink, pal.deepTop) } else Color.Transparent to Ink
    Box(
        modifier.heightIn(min = 46.dp).clip(RoundedCornerShape(14.dp)).background(bg)
            .then(if (primary) Modifier else Modifier.border(1.dp, Line, RoundedCornerShape(14.dp))).clickable(onClick = onClick),
        contentAlignment = Alignment.Center,
    ) { Text(label, color = fg, fontSize = GregalSize.body, fontWeight = FontWeight.SemiBold) }
}

/** Respostes ràpides: sota l'últim missatge del vent; un toc les envia com a resposta. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun ChoiceChips(options: List<String>, accent: Color, onPick: (String) -> Unit) {
    if (options.isEmpty()) return
    FlowRow(Modifier.fillMaxWidth().padding(top = 8.dp).testTag("choice-chips"),
        horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        options.forEach { o ->
            Text(o, color = Ink, fontSize = GregalSize.small, fontWeight = FontWeight.Medium,
                modifier = Modifier.clip(RoundedCornerShape(999.dp)).background(accent.copy(alpha = .14f))
                    .border(1.dp, accent.copy(alpha = .45f), RoundedCornerShape(999.dp)).clickable { onPick(o) }
                    .padding(horizontal = 14.dp, vertical = 9.dp).testTag("choice-chip"))
        }
    }
}

internal fun secretCopy(lang: String, key: String): String {
    val en = lang == "en"
    return when (key) {
        "need" -> if (en) "I need: " else "Necessito: "
        "hint" -> if (en) "Paste it here" else "Enganxa-la aquí"
        "show" -> if (en) "Show" else "Mostra"
        "hide" -> if (en) "Hide" else "Amaga"
        "save" -> if (en) "Save it" else "Desa-la"
        "no" -> if (en) "Not now" else "Ara no"
        "note" -> if (en) "🔒 Stored encrypted on your server. Gregal never sees it or keeps it in the thread: it only uses it when needed."
            else "🔒 Va xifrada al teu servidor. Gregal no la veu mai ni la desa al fil: només la fa servir quan cal."
        else -> key
    }
}

/**
 * Formulari segur (com Grok): un camp amagat per a una contrasenya o una clau.
 * El valor va directe al magatzem xifrat del servidor (internal/web/winds_secret.go);
 * ni el fil ni el model el veuen. La web fa el mateix a paintAsks (type «secret»).
 */
@Composable
internal fun SecretCard(lang: String, a: GregalApi.WindAsk, accent: Color, onSave: (String) -> Unit, onDecline: () -> Unit) {
    var value by remember(a.key) { mutableStateOf("") }
    var visible by remember(a.key) { mutableStateOf(false) }
    AskShell(accent, "secret-card") {
        Row(verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Icon(Icons.Rounded.Shield, null, tint = accent, modifier = Modifier.size(20.dp))
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(secretCopy(lang, "need") + a.query.ifBlank { a.name }, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = GregalSize.body)
                if (a.reason.isNotBlank()) Text(a.reason, color = Muted, fontSize = GregalSize.small)
            }
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            androidx.compose.material3.OutlinedTextField(
                value = value, onValueChange = { value = it.take(8192) }, singleLine = true,
                placeholder = { Text(secretCopy(lang, "hint"), fontSize = GregalSize.body) },
                visualTransformation = if (visible) androidx.compose.ui.text.input.VisualTransformation.None else androidx.compose.ui.text.input.PasswordVisualTransformation(),
                keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(
                    keyboardType = androidx.compose.ui.text.input.KeyboardType.Password, autoCorrectEnabled = false),
                modifier = Modifier.weight(1f).testTag("secret-input"),
            )
            androidx.compose.material3.TextButton(onClick = { visible = !visible }, modifier = Modifier.testTag("secret-show")) {
                Text(secretCopy(lang, if (visible) "hide" else "show"), color = Ink)
            }
        }
        Text(secretCopy(lang, "note"), color = Muted, fontSize = GregalSize.caption, lineHeight = 16.sp)
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            androidx.compose.material3.Button(onClick = { val v = value; if (v.isNotBlank()) { value = ""; onSave(v) } },
                enabled = value.isNotBlank(), modifier = Modifier.testTag("secret-save")) { Text(secretCopy(lang, "save")) }
            androidx.compose.material3.TextButton(onClick = onDecline) { Text(secretCopy(lang, "no"), color = Muted) }
        }
    }
}
