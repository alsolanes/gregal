package xyz.solanes.gregal

import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowForward
import androidx.compose.material.icons.rounded.AddPhotoAlternate
import androidx.compose.material.icons.rounded.ArrowUpward
import androidx.compose.material.icons.rounded.Mic
import androidx.compose.material3.Icon
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * L'inici dels Vents a l'estil dels grans assistents (Grok, ChatGPT, dots):
 * primer es pregunta. Una salutació, la colla a un toc, «Avui» amb el que hi
 * ha en marxa i el que ve, i el camp d'escriptura gran a baix. El que
 * s'escriu aquí va al Ventet, que respon o ho passa a qui toca (un vent o
 * una colla). Les converses són a la pestanya «Xats».
 */

internal fun askCopy(lang: String, key: String): String {
    val en = lang == "en"
    return when (key) {
        "home" -> if (en) "Home" else "Inici"
        "chats" -> if (en) "Chats" else "Xats"
        "sub" -> if (en) "Ask, delegate or plan. Your crew takes care of the rest." else "Pregunta, encarrega o planifica. La colla s’encarrega de la resta."
        "hint" -> if (en) "Ask anything" else "Pregunta el que vulguis"
        "send" -> if (en) "Send" else "Envia"
        "photo" -> if (en) "Add a photo" else "Afegeix una foto"
        "today" -> if (en) "Today" else "Avui"
        "briefAt" -> if (en) "Morning briefing · %s" else "Resum del matí · %s"
        "pickUp" -> if (en) "Pick up" else "Repren"
        "news" -> if (en) "New message" else "Missatge nou"
        "work" -> if (en) "Work in progress" else "Feina en marxa"
        "reminder" -> if (en) "Reminder" else "Recordatori"
        "briefing" -> if (en) "Morning briefing" else "Resum del matí"
        "briefingOn" -> if (en) "Every day at %s" else "Cada dia a les %s"
        "briefingOff" -> if (en) "Calendar and email in two lines, every morning" else "El calendari i el correu en dues línies, cada matí"
        "briefingSetup" -> if (en) "Set it up" else "Configura’l"
        "withCrew" -> if (en) "With the crew" else "Amb la colla"
        "crew" -> if (en) "Ask a specialist" else "Pregunta a un especialista"
        else -> key
    }
}

internal fun askGreeting(lang: String, hour: Int, name: String): String {
    val en = lang == "en"
    val base = when {
        hour < 6 -> if (en) "Good night" else "Bona nit"
        hour < 13 -> if (en) "Good morning" else "Bon dia"
        hour < 20 -> if (en) "Good afternoon" else "Bona tarda"
        else -> if (en) "Good evening" else "Bon vespre"
    }
    val first = name.trim().split(Regex("\\s+")).firstOrNull().orEmpty()
    return if (first.isBlank()) "$base." else "$base, ${first.replaceFirstChar { it.uppercase() }}."
}

internal fun askSuggestions(lang: String): List<String> = if (lang == "en") listOf(
    "Plan my week", "What’s on my calendar today?", "Recommend a book", "Remind me tomorrow at 9 to call the doctor",
) else listOf(
    "Organitza’m la setmana", "Què tinc avui al calendari?", "Recomana’m un llibre", "Recorda’m demà a les 9 que truqui al metge",
)

/** Una targeta d'«Avui». */
internal data class TodayCard(
    val key: String, val kicker: String, val title: String, val sub: String,
    val wind: Wind? = null, val glyph: String = "", val soft: Boolean = false, val onClick: () -> Unit,
)

/** El que surt a «Avui», en ordre: avisos nous, feina en marxa, converses, recordatoris i el resum del matí. */
internal fun todayCards(
    lang: String,
    rows: List<VentsInboxRow>,
    news: Set<String>,
    running: List<WindTask>,
    reminders: List<ProgramatReminder>,
    conn: GregalApi.Connections?,
    onOpen: (String) -> Unit,
    onWork: () -> Unit,
    onReminders: () -> Unit,
    onBriefing: () -> Unit,
): List<TodayCard> {
    val out = mutableListOf<TodayCard>()
    val withNews = rows.filter { it.key.startsWith("wind:") && it.key.substringAfter(':') in news }
    withNews.take(2).forEach { r ->
        out += TodayCard(r.key, askCopy(lang, "news"), r.title, r.preview, wind = r.members.firstOrNull(), onClick = { onOpen(r.key) })
    }
    running.take(2).forEach { t ->
        out += TodayCard("task:" + t.id, askCopy(lang, "work"), t.task, t.agents.joinToString(" · "), glyph = "◷", onClick = onWork)
    }
    rows.filter { it.hasMessages && it !in withNews }.take(3 - minOf(withNews.size, 2)).forEach { r ->
        out += TodayCard(r.key, askCopy(lang, "pickUp"), r.title, r.preview.ifBlank { if (r.members.size > 1) askCopy(lang, "withCrew") else "" },
            wind = r.members.firstOrNull(), onClick = { onOpen(r.key) })
    }
    reminders.take(2).forEach { r ->
        out += TodayCard("rem:" + r.id, askCopy(lang, "reminder") + " · " + r.at.take(16).replace('T', ' '), r.text, r.windName,
            glyph = "⏰", soft = true, onClick = onReminders)
    }
    if (conn != null) out += TodayCard("briefing", askCopy(lang, "briefing"),
        if (conn.briefingOn) askCopy(lang, "briefingOn").format(conn.briefingTime) else askCopy(lang, "briefingSetup"),
        if (conn.briefingOn) conn.briefingWind else askCopy(lang, "briefingOff"),
        glyph = "☀", soft = true, onClick = onBriefing)
    return out
}

/** Inici | Xats: dues pastilles a la capçalera. */
@Composable
internal fun AskTabs(lang: String, tab: String, unread: Int, onTab: (String) -> Unit) {
    Row(
        Modifier.padding(start = 16.dp, top = 4.dp, bottom = 6.dp).clip(RoundedCornerShape(999.dp))
            .background(SuperficieAlta.copy(alpha = .55f)).padding(3.dp),
        horizontalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        listOf("home", "chats").forEach { key ->
            val on = tab == key
            Row(
                Modifier.clip(RoundedCornerShape(999.dp)).background(if (on) Ink else Color.Transparent)
                    .clickable { onTab(key) }.semantics { selected = on; contentDescription = askCopy(lang, key) }
                    .padding(horizontal = 16.dp, vertical = 7.dp).testTag("ask-tab-$key"),
                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                Text(askCopy(lang, key), color = if (on) Superficie else Muted, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                if (key == "chats" && unread > 0) Surface(shape = RoundedCornerShape(999.dp), color = Sea) {
                    Text(unreadBadge(unread), color = OnSea, fontSize = 10.sp, fontWeight = FontWeight.Bold,
                        modifier = Modifier.padding(horizontal = 6.dp, vertical = 1.dp))
                }
            }
        }
    }
}

/** La part de dalt de l'inici: la cara que respira, la salutació i la colla. */
@Composable
internal fun AskHero(lang: String, name: String, lead: Wind?, winds: List<Wind>, busy: Set<String>, news: Set<String>, onTalk: (Wind) -> Unit) {
    val glow = rememberInfiniteTransition(label = "inici")
    val breathe by glow.animateFloat(0.55f, 1f, infiniteRepeatable(tween(2600, easing = FastOutSlowInEasing), RepeatMode.Reverse), label = "respira")
    val accent = lead?.let { Color(it.argb) } ?: Sea
    Column(Modifier.fillMaxWidth().padding(top = 36.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Box(
            Modifier.size(112.dp).drawBehind {
                drawCircle(Brush.radialGradient(listOf(accent.copy(alpha = .38f * breathe), Color.Transparent),
                    center = Offset(size.width / 2, size.height / 2), radius = size.minDimension / 2))
            },
            contentAlignment = Alignment.Center,
        ) {
            if (lead != null) WindHalo(accent, lead.faceStyle, size = 68, mood = WindMood.HAPPY, online = false, showKit = true)
        }
        Text(askGreeting(lang, java.util.Calendar.getInstance().get(java.util.Calendar.HOUR_OF_DAY), name),
            color = Ink, fontSize = 30.sp, fontWeight = FontWeight.Bold, textAlign = TextAlign.Center, lineHeight = 34.sp,
            modifier = Modifier.padding(horizontal = 24.dp))
        Text(askCopy(lang, "sub"), color = Muted, fontSize = 14.sp, textAlign = TextAlign.Center, lineHeight = 19.sp,
            modifier = Modifier.padding(horizontal = 32.dp, vertical = 8.dp))
        if (winds.isNotEmpty()) LazyRow(
            Modifier.fillMaxWidth().padding(top = 10.dp).testTag("ask-crew"),
            contentPadding = PaddingValues(horizontal = 16.dp), horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            items(winds, key = { it.id }) { w ->
                val mood = when { w.id in busy -> WindMood.THINKING; w.id in news -> WindMood.NEWS; else -> WindMood.IDLE }
                Row(
                    Modifier.clip(RoundedCornerShape(999.dp)).background(SuperficieAlta.copy(alpha = .5f))
                        .border(1.dp, Color(w.argb).copy(alpha = if (w.id in news) .7f else .18f), RoundedCornerShape(999.dp))
                        .clickable { onTalk(w) }.semantics { contentDescription = w.name }
                        .padding(start = 4.dp, end = 14.dp, top = 4.dp, bottom = 4.dp),
                    verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    WindHalo(Color(w.argb), w.faceStyle, size = 30, mood = mood, online = false, showKit = false)
                    Text(w.name, color = Ink, fontSize = 13.sp, fontWeight = FontWeight.Medium, maxLines = 1)
                }
            }
        }
    }
}

@Composable
internal fun AskSuggestions(lang: String, onPick: (String) -> Unit) {
    LazyRow(
        Modifier.fillMaxWidth().padding(top = 14.dp),
        contentPadding = PaddingValues(horizontal = 16.dp), horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        items(askSuggestions(lang)) { s ->
            Text(s, color = Muted, fontSize = 13.sp, maxLines = 1,
                modifier = Modifier.clip(RoundedCornerShape(12.dp)).border(1.dp, Line, RoundedCornerShape(12.dp))
                    .clickable { onPick(s) }.padding(horizontal = 12.dp, vertical = 8.dp))
        }
    }
}

@Composable
internal fun TodaySection(lang: String, cards: List<TodayCard>, brief: GregalViewModel.TodayBrief? = null, briefWind: Wind? = null, onBrief: () -> Unit = {}) {
    if (cards.isEmpty() && brief == null) return
    Column(Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 28.dp).testTag("ask-today"),
        verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(askCopy(lang, "today").uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold,
            letterSpacing = 0.9.sp, modifier = Modifier.padding(start = 4.dp, bottom = 2.dp))
        if (brief != null) TodayBriefView(lang, brief, briefWind, onBrief)
        cards.filter { brief == null || it.key != "briefing" }.forEach { c -> TodayCardView(c) }
    }
}

/** El resum del matí, escrit pel vent, a dalt de tot: com el «què tens avui» dels grans assistents. */
@Composable
private fun TodayBriefView(lang: String, b: GregalViewModel.TodayBrief, wind: Wind?, onClick: () -> Unit) {
    val accent = wind?.let { Color(it.argb) } ?: Sea
    Column(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(22.dp))
            .background(Brush.linearGradient(listOf(accent.copy(alpha = .16f), SuperficieAlta.copy(alpha = .5f))))
            .border(1.dp, accent.copy(alpha = .35f), RoundedCornerShape(22.dp))
            .clickable(onClick = onClick).padding(16.dp).testTag("ask-brief"),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            if (wind != null) WindHalo(accent, wind.faceStyle, size = 26, mood = WindMood.HAPPY, online = false, showKit = false)
            else Text("☀", fontSize = 18.sp)
            Column {
                Text(askCopy(lang, "briefAt").format(b.at), color = Muted, fontSize = 11.sp)
                if (b.windName.isNotBlank()) Text(b.windName, color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
            }
        }
        Text(speakableText(b.text), color = Ink, fontSize = 14.5.sp, lineHeight = 20.sp, maxLines = 6, overflow = TextOverflow.Ellipsis)
    }
}

@Composable
private fun TodayCardView(c: TodayCard) {
    val accent = c.wind?.let { Color(it.argb) } ?: Sea
    Row(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(20.dp))
            .background(if (c.soft) accent.copy(alpha = .08f).compositeOverSafe(SuperficieAlta.copy(alpha = .35f)) else SuperficieAlta.copy(alpha = .5f))
            .border(1.dp, Line, RoundedCornerShape(20.dp))
            .clickable(onClick = c.onClick).padding(horizontal = 14.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Box(Modifier.size(40.dp).clip(RoundedCornerShape(13.dp)).background(Superficie), contentAlignment = Alignment.Center) {
            if (c.wind != null) WindHalo(Color(c.wind.argb), c.wind.faceStyle, size = 34, mood = WindMood.IDLE, online = false, showKit = false)
            else Text(c.glyph, fontSize = 18.sp, color = Ink)
        }
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(1.dp)) {
            Text(c.kicker, color = Muted, fontSize = 11.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(c.title, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, maxLines = 2, overflow = TextOverflow.Ellipsis, lineHeight = 19.sp)
            if (c.sub.isNotBlank()) Text(c.sub, color = Muted, fontSize = 12.5.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        Icon(Icons.AutoMirrored.Rounded.ArrowForward, null, tint = Muted.copy(alpha = .7f), modifier = Modifier.size(18.dp))
    }
}

private fun Color.compositeOverSafe(bg: Color): Color = Color(
    red = red * alpha + bg.red * (1 - alpha), green = green * alpha + bg.green * (1 - alpha),
    blue = blue * alpha + bg.blue * (1 - alpha), alpha = maxOf(alpha, bg.alpha),
)

/** El camp gran de l'inici, fix a baix: una pastilla amb el botó d'enviar rodó. */
@Composable
internal fun AskBar(lang: String, text: String, onText: (String) -> Unit, busy: Boolean,
    /** Afegir una foto (puja i en posa la referència al camp). */
    onPhoto: (() -> Unit)? = null,
    onSend: () -> Unit) {
    val can = text.isNotBlank() && !busy
    val dictate = rememberDictation(lang) { onText(appendDictated(text, it)) }
    Box(Modifier.fillMaxWidth().background(Brush.verticalGradient(listOf(Color.Transparent, Superficie, Superficie)))
        .navigationBarsPadding().imePadding().padding(start = 12.dp, end = 12.dp, top = 10.dp, bottom = 12.dp)) {
        Row(
            Modifier.fillMaxWidth().workingRing(busy, corner = 28.dp).clip(RoundedCornerShape(28.dp))
                .background(SuperficieAlta).border(1.dp, Line, RoundedCornerShape(28.dp))
                .padding(start = if (onPhoto != null) 6.dp else 20.dp, end = 6.dp, top = 6.dp, bottom = 6.dp).testTag("ask-bar"),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (onPhoto != null) Box(
                Modifier.size(40.dp).clip(CircleShape).clickable(onClick = onPhoto).semantics { contentDescription = askCopy(lang, "photo") },
                contentAlignment = Alignment.Center,
            ) { Icon(Icons.Rounded.AddPhotoAlternate, null, tint = Muted, modifier = Modifier.size(22.dp)) }
            if (onPhoto != null) Spacer(Modifier.width(6.dp))
            Box(Modifier.weight(1f).padding(vertical = 10.dp)) {
                BasicTextField(
                    text, onText, textStyle = TextStyle(color = Ink, fontSize = 16.sp, lineHeight = 22.sp),
                    cursorBrush = SolidColor(Sea), maxLines = 5,
                    keyboardOptions = KeyboardOptions(imeAction = ImeAction.Default),
                    modifier = Modifier.fillMaxWidth().testTag("ask-input"),
                    decorationBox = { inner ->
                        Box(contentAlignment = Alignment.CenterStart) {
                            if (text.isEmpty()) Text(askCopy(lang, "hint"), color = Muted, fontSize = 16.sp)
                            inner()
                        }
                    },
                )
            }
            // Camp buit: el micròfon (dictar); amb text: enviar.
            val mic = !busy && text.isBlank() && dictate != null
            Box(
                Modifier.size(44.dp).clip(CircleShape).background(if (can) Ink else Ink.copy(alpha = if (mic) .08f else .18f))
                    .clickable(enabled = can || mic, onClick = { if (mic) dictate?.invoke() else onSend() })
                    .semantics { contentDescription = if (mic) voiceCopy(lang, "dictate") else askCopy(lang, "send") },
                contentAlignment = Alignment.Center,
            ) {
                when {
                    busy -> TypingDots(Superficie)
                    mic -> Icon(Icons.Rounded.Mic, null, tint = Ink, modifier = Modifier.size(22.dp))
                    else -> Icon(Icons.Rounded.ArrowUpward, null, tint = if (can) Superficie else Muted)
                }
            }
        }
    }
}

/** L'estat del camp de l'inici, perquè sobrevisqui a girar la pantalla. */
@Composable
internal fun rememberAskText(): MutableState<String> = rememberSaveable { mutableStateOf("") }
