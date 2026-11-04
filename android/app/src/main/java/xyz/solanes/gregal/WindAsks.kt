package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.ArrowUpward
import androidx.compose.material.icons.rounded.Shield
import androidx.compose.material.icons.rounded.HelpOutline
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay

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
internal fun WindAskCards(vm: GregalViewModel, lang: String, accent: Color, busy: Boolean) {
    val asks by vm.windAsks.collectAsState()
    // En obrir el xat i mentre un vent treballa: potser n'hi ha una d'abans.
    LaunchedEffect(busy) {
        vm.loadWindAsks()
        while (busy) { delay(4_000); vm.loadWindAsks() }
    }
    if (asks.isEmpty()) return
    Column(Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 6.dp).testTag("wind-asks"),
        verticalArrangement = Arrangement.spacedBy(8.dp)) {
        asks.forEach { a ->
            if (a.type == "question") QuestionCard(lang, a, accent) { ans -> vm.answerWindAsk(a.key, answer = ans) }
            else ApproveCard(lang, a, accent, onAllow = { vm.answerWindAsk(a.key, approve = true) }, onDecline = { vm.answerWindAsk(a.key, approve = false) })
        }
    }
}

@Composable
private fun AskShell(accent: Color, tag: String, content: @Composable ColumnScope.() -> Unit) {
    Column(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(22.dp)).background(accent.copy(alpha = .10f).compositeOverSafe(SuperficieAlta.copy(alpha = .6f)))
            .border(1.dp, accent.copy(alpha = .45f), RoundedCornerShape(22.dp)).padding(16.dp).testTag(tag),
        verticalArrangement = Arrangement.spacedBy(10.dp), content = content,
    )
}

private fun Color.compositeOverSafe(bg: Color): Color = Color(
    red = red * alpha + bg.red * (1 - alpha), green = green * alpha + bg.green * (1 - alpha),
    blue = blue * alpha + bg.blue * (1 - alpha), alpha = maxOf(alpha, bg.alpha),
)

@Composable
private fun ApproveCard(lang: String, a: GregalApi.WindAsk, accent: Color, onAllow: () -> Unit, onDecline: () -> Unit) {
    AskShell(accent, "approve-card") {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Icon(Icons.Rounded.Shield, null, tint = accent, modifier = Modifier.size(22.dp))
            Column {
                Text(askUi(lang, a.kind.ifBlank { "approve" }).let { if (it == a.kind) askUi(lang, "approve") else it },
                    color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                Text(askUi(lang, "waiting"), color = Muted, fontSize = 11.sp)
            }
        }
        if (a.what.isNotBlank()) Text(
            a.what, color = Ink, fontSize = 12.5.sp, fontFamily = FontFamily.Monospace, maxLines = 4, overflow = TextOverflow.Ellipsis,
            modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(Superficie.copy(alpha = .7f)).padding(10.dp),
        )
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp), modifier = Modifier.fillMaxWidth()) {
            AskButton(askUi(lang, "decline"), primary = false, modifier = Modifier.weight(1f).testTag("ask-decline"), onClick = onDecline)
            AskButton(askUi(lang, "allow"), primary = true, accent = accent, modifier = Modifier.weight(1f).testTag("ask-allow"), onClick = onAllow)
        }
    }
}

@Composable
private fun QuestionCard(lang: String, a: GregalApi.WindAsk, accent: Color, onAnswer: (String) -> Unit) {
    var free by remember(a.key) { mutableStateOf("") }
    AskShell(accent, "question-card") {
        Row(verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Icon(Icons.Rounded.HelpOutline, null, tint = accent, modifier = Modifier.size(22.dp).padding(top = 1.dp))
            Text(a.query, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, lineHeight = 20.sp)
        }
        a.options.forEach { (label, desc) ->
            Column(
                Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(Superficie.copy(alpha = .7f))
                    .border(1.dp, Line, RoundedCornerShape(14.dp)).clickable { onAnswer(label) }.padding(horizontal = 14.dp, vertical = 11.dp).testTag("ask-option"),
            ) {
                Text(label, color = Ink, fontSize = 14.5.sp, fontWeight = FontWeight.Medium)
                if (desc.isNotBlank()) Text(desc, color = Muted, fontSize = 12.sp)
            }
        }
        Row(
            Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(Superficie.copy(alpha = .7f)).border(1.dp, Line, RoundedCornerShape(14.dp))
                .padding(start = 14.dp, end = 4.dp, top = 4.dp, bottom = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            BasicTextField(
                free, { free = it }, singleLine = true, textStyle = TextStyle(color = Ink, fontSize = 14.sp), cursorBrush = SolidColor(Ink),
                modifier = Modifier.weight(1f).testTag("ask-free"),
                decorationBox = { inner -> Box { if (free.isEmpty()) Text(askUi(lang, "other"), color = Muted, fontSize = 14.sp); inner() } },
            )
            Box(
                Modifier.size(36.dp).clip(RoundedCornerShape(50)).background(if (free.isNotBlank()) Ink else Ink.copy(alpha = .12f))
                    .clickable(enabled = free.isNotBlank()) { onAnswer(free.trim()) },
                contentAlignment = Alignment.Center,
            ) { Icon(Icons.Rounded.ArrowUpward, askUi(lang, "send"), tint = if (free.isNotBlank()) Superficie else Muted, modifier = Modifier.size(18.dp)) }
        }
    }
}

@Composable
private fun AskButton(label: String, primary: Boolean, modifier: Modifier = Modifier, accent: Color = Color.Unspecified, onClick: () -> Unit) {
    val bg = if (primary) (if (accent != Color.Unspecified) accent else Sea) else Color.Transparent
    Box(
        modifier.heightIn(min = 46.dp).clip(RoundedCornerShape(14.dp)).background(bg)
            .then(if (primary) Modifier else Modifier.border(1.dp, Line, RoundedCornerShape(14.dp))).clickable(onClick = onClick),
        contentAlignment = Alignment.Center,
    ) { Text(label, color = if (primary) OnSea else Ink, fontSize = 14.5.sp, fontWeight = FontWeight.SemiBold) }
}

/** Respostes ràpides: sota l'últim missatge del vent; un toc les envia com a resposta. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun ChoiceChips(options: List<String>, accent: Color, onPick: (String) -> Unit) {
    if (options.isEmpty()) return
    FlowRow(Modifier.fillMaxWidth().padding(top = 8.dp).testTag("choice-chips"),
        horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        options.forEach { o ->
            Text(o, color = Ink, fontSize = 13.5.sp, fontWeight = FontWeight.Medium,
                modifier = Modifier.clip(RoundedCornerShape(999.dp)).background(accent.copy(alpha = .14f))
                    .border(1.dp, accent.copy(alpha = .45f), RoundedCornerShape(999.dp)).clickable { onPick(o) }
                    .padding(horizontal = 14.dp, vertical = 9.dp).testTag("choice-chip"))
        }
    }
}
