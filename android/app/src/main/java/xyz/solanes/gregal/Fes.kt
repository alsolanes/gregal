package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties

/**
 * Fes (com la pestanya Do de Duet): plantilles per començar. Les completes
 * s'envien a Gregal d'un toc; les que tenen un buit ({x}) obren un camp a la
 * targeta; les del dia porten «Cada dia» (Gregal en fa una rutina). La web fa
 * el mateix a fesView (winds-workspace.js), amb els mateixos textos.
 */
data class FesTemplate(val icon: String, val title: String, val sub: String, val prompt: String, val blank: String = "", val daily: String = "")

internal fun fesTemplates(lang: String): List<Pair<String, List<FesTemplate>>> {
    val en = lang == "en"
    fun t(ca: String, e: String) = if (en) e else ca
    return listOf(
        t("El teu dia", "Your day") to listOf(
            FesTemplate("☀️", t("Llista del dia", "Today list"), t("Calendari, avisos i pendents, per ordre.", "Calendar, reminders and to-dos, in order."),
                t("Fes-me la llista d'avui: què tinc al calendari, quins avisos i què tinc pendent, per ordre d'hora i d'importància. Curt.",
                    "Make me today's list: what's on my calendar, which reminders and what's pending, ordered by time and importance. Keep it short."), daily = "08:00"),
            FesTemplate("📬", t("Triatge del correu", "Email triage"), t("El que necessita resposta, primer.", "What needs a reply, first."),
                t("Mira el meu correu d'avui i fes-ne el triatge: què necessita resposta meva, què és informatiu i què és brossa. Proposa'm respostes curtes per als importants, sense enviar res.",
                    "Look at today's email and triage it: what needs my reply, what is informational and what is junk. Draft short replies for the important ones, without sending anything."), daily = "09:00"),
            FesTemplate("🗓️", t("Prepara la setmana", "Plan the week"), t("Reunions, entregues i forats lliures.", "Meetings, deadlines and free slots."),
                t("Prepara-me la setmana: reunions, entregues i avisos, i on tinc forats lliures per concentrar-me.", "Plan my week: meetings, deadlines and reminders, and where I have free slots to focus.")),
        ),
        t("Busca i compara", "Find and compare") to listOf(
            FesTemplate("🏷️", t("Compara preus", "Compare prices"), t("Tres opcions amb preu i on.", "Three options with price and where."),
                t("Compara'm preus de {x}: tres opcions amb el preu, on comprar-les i quina em recomanes.", "Compare prices for {x}: three options with the price, where to buy them and which you recommend."),
                blank = t("Què vols comparar?", "What do you want to compare?")),
            FesTemplate("🍽️", t("Troba un restaurant", "Find a restaurant"), t("A prop, obert i amb taula.", "Nearby, open and with a table."),
                t("Troba'm tres restaurants per a {x}, amb horari, preu aproximat i com reservar.", "Find me three restaurants for {x}, with opening hours, rough price and how to book."),
                blank = t("On i per a quan?", "Where and when?")),
            FesTemplate("📰", t("Resumeix una web", "Summarise a page"), t("L'essencial en cinc línies.", "The gist in five lines."),
                t("Llegeix {x} i fes-me'n un resum de cinc línies amb el que importa.", "Read {x} and give me a five-line summary of what matters."),
                blank = t("Enganxa l'adreça", "Paste the address")),
        ),
        t("Escriu i prepara", "Write and prepare") to listOf(
            FesTemplate("✉️", t("Escriu un correu", "Write an email"), t("T'ensenya l'esborrany abans d'enviar.", "Shows you the draft before sending."),
                t("Escriu un correu per {x}. Ensenya'm l'esborrany i no l'enviïs.", "Write an email to {x}. Show me the draft and do not send it."),
                blank = t("A qui i per a què?", "To whom and what for?")),
            FesTemplate("🤝", t("Prepara una reunió", "Prepare a meeting"), t("Ordre del dia i què portar.", "Agenda and what to bring."),
                t("Prepara'm la reunió de {x}: ordre del dia, preguntes clau i què he de portar.", "Prepare my meeting about {x}: agenda, key questions and what I need to bring."),
                blank = t("De què va la reunió?", "What is the meeting about?")),
            FesTemplate("🧳", t("Planifica un viatge", "Plan a trip"), t("Transport, dormir i què fer.", "Transport, where to sleep and what to do."),
                t("Planifica'm un viatge a {x}: com hi vaig, on dormir i què fer, amb preus aproximats.", "Plan a trip to {x}: how to get there, where to stay and what to do, with rough prices."),
                blank = t("On i quins dies?", "Where and which days?")),
        ),
        t("Ordena i vigila", "Tidy and watch") to listOf(
            FesTemplate("🗂️", t("Endreça una carpeta", "Tidy a folder"), t("Et proposa l'ordre abans de moure res.", "Proposes an order before moving anything."),
                t("Mira la carpeta {x} i proposa'm com endreçar-la. No moguis res fins que t'ho digui.", "Look at the folder {x} and propose how to tidy it. Do not move anything until I say so."),
                blank = t("Quina carpeta?", "Which folder?")),
            FesTemplate("📉", t("Vigila un preu", "Watch a price"), t("T'avisa quan baixa.", "Tells you when it drops."),
                t("Vigila el preu de {x} i avisa'm si baixa.", "Watch the price of {x} and tell me if it drops."),
                blank = t("Què i fins a quin preu?", "What, and below which price?")),
            FesTemplate("⏰", t("Recorda-m'ho", "Remind me"), t("Un avís a l'hora que diguis.", "A reminder at the time you say."),
                t("Recorda'm {x}.", "Remind me {x}."), blank = t("Què i quan?", "What and when?")),
        ),
    )
}

/** El missatge que envia una plantilla (amb el buit omplert, o com a rutina diària). */
internal fun fesMessage(lang: String, tpl: FesTemplate, filled: String = "", everyDay: Boolean = false): String {
    val body = tpl.prompt.replace("{x}", filled.trim())
    if (!everyDay || tpl.daily.isEmpty()) return body
    return if (lang == "en") "Every day at ${tpl.daily}: $body Make it a routine and don't do it now."
    else "Cada dia a les ${tpl.daily}: $body Fes-ne una rutina i no ho facis ara."
}

@Composable
internal fun FesDialog(lang: String, onSend: (String) -> Unit, onDismiss: () -> Unit) {
    val en = lang == "en"
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(shape = MaterialTheme.shapes.large, color = MaterialTheme.colorScheme.background) {
            Column(Modifier.fillMaxWidth().fillMaxHeight(0.9f).padding(top = 16.dp).testTag("fes-dialog")) {
                Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(if (en) "Do" else "Fes", color = Ink, fontWeight = FontWeight.Bold, fontSize = 18.sp, modifier = Modifier.weight(1f))
                    IconButton(onClick = onDismiss, modifier = Modifier.semantics { contentDescription = if (en) "Close" else "Tanca" }) { Text("✕", color = Muted, fontSize = 16.sp) }
                }
                Column(Modifier.verticalScroll(rememberScrollState()).padding(horizontal = 16.dp, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    Text(if (en) "Templates to get going: one tap and Gregal gets on it. The daily ones can run every day."
                        else "Plantilles per començar: un toc i Gregal s'hi posa. Les del dia es poden fer cada dia.",
                        color = Muted, fontSize = 13.sp, lineHeight = 18.sp, modifier = Modifier.padding(horizontal = 4.dp))
                    FesContent(lang) { onSend(it); onDismiss() }
                }
            }
        }
    }
}

/** La llista de plantilles; onSend rep el missatge final. Pura (FesTest). */
@Composable
internal fun FesContent(lang: String, onSend: (String) -> Unit) {
    val en = lang == "en"
    var open by remember { mutableStateOf("") }
    for ((group, items) in fesTemplates(lang)) {
        Text(group.uppercase(), color = Muted, fontSize = 12.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 8.dp, start = 4.dp))
        for (tpl in items) {
            var draft by remember(tpl.title) { mutableStateOf("") }
            val expanded = open == tpl.title
            Column(
                Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(SuperficieAlta).border(1.dp, Line, RoundedCornerShape(14.dp))
                    .testTag("fes-" + tpl.title),
            ) {
                Row(
                    Modifier.fillMaxWidth().clickable {
                        if (tpl.blank.isEmpty()) onSend(fesMessage(lang, tpl)) else open = if (expanded) "" else tpl.title
                    }.padding(14.dp),
                    verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    Text(tpl.icon, fontSize = 22.sp)
                    Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                        Text(tpl.title, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
                        Text(tpl.sub, color = Muted, fontSize = 13.sp)
                    }
                    if (tpl.daily.isNotEmpty()) {
                        Text(if (en) "Every day" else "Cada dia", color = Muted, fontSize = 12.sp,
                            modifier = Modifier.clip(RoundedCornerShape(999.dp)).border(1.dp, Line, RoundedCornerShape(999.dp))
                                .clickable { onSend(fesMessage(lang, tpl, everyDay = true)) }
                                .padding(horizontal = 10.dp, vertical = 8.dp).testTag("fes-daily-" + tpl.title))
                    }
                }
                if (expanded) {
                    val go = { if (draft.isNotBlank()) onSend(fesMessage(lang, tpl, draft)) }
                    Row(Modifier.fillMaxWidth().padding(start = 12.dp, end = 12.dp, bottom = 12.dp), verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedTextField(value = draft, onValueChange = { draft = it.take(300) }, singleLine = true,
                            placeholder = { Text(tpl.blank, fontSize = 14.sp) },
                            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Send), keyboardActions = KeyboardActions(onSend = { go() }),
                            modifier = Modifier.weight(1f).testTag("fes-input"))
                        Button(onClick = go, enabled = draft.isNotBlank(), modifier = Modifier.testTag("fes-go")) { Text(if (en) "Do it" else "Fes-ho") }
                    }
                }
            }
        }
    }
}
