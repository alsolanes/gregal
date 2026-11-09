package xyz.solanes.gregal

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowForward
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.AddPhotoAlternate
import androidx.compose.material.icons.rounded.ArrowUpward
import androidx.compose.material.icons.rounded.Description
import androidx.compose.material.icons.rounded.Mic
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.drawWithCache
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.boundsInRoot
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch

/**
 * L'inici dels Vents, net com el de l'escriptori: la salutació gran, una
 * lluïssor suau al darrere i el compositor just a sota. El que és nou o en
 * marxa surt a sota, discret. El que s'escriu aquí va al Ventet, que respon o
 * ho passa a qui toca (un vent o una colla). Les converses són a «Xats».
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
        "attach" -> if (en) "Attach" else "Adjunta"
        "document" -> if (en) "Add a document" else "Afegeix un document"
        "documentHelp" -> if (en) "PDF, Word, Excel… as context" else "PDF, Word, Excel… com a context"
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
        Modifier.padding(start = 16.dp, top = 4.dp, bottom = 6.dp).clip(GregalShape.pill)
            .background(SuperficieAlta.copy(alpha = .55f)).padding(4.dp),
        horizontalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        listOf("home", "chats").forEach { key ->
            val on = tab == key
            // Cada pestanya fa 40 dp i la càpsula 48: abans eren 34 dp, justos per al dit.
            Row(
                Modifier.heightIn(min = 40.dp).clip(GregalShape.pill).background(if (on) Ink else Color.Transparent)
                    .clickable { onTab(key) }.semantics { selected = on; contentDescription = askCopy(lang, key) }
                    .padding(horizontal = 18.dp).testTag("ask-tab-$key"),
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

/**
 * La lluïssor de la portada, com a l'escriptori: dos degradats radials (l'accent
 * al 14% al centre i el lila al 8% una mica a la dreta) que s'esvaeixen. Es
 * calcula un cop per mida i es pinta estàtica: cap cost per fotograma.
 * [centerY] és on cau el centre, en fracció de l'alçada.
 */
internal fun Modifier.openingHalo(accent: Color, lila: Color, centerY: Float = .42f): Modifier = drawWithCache {
    val w = size.width
    val h = size.height
    val c1 = Offset(w * .5f, h * centerY)
    val rx1 = w * .80f
    val ry1 = minOf(h * .36f, w * .66f)
    val halo = Brush.radialGradient(0f to accent.copy(alpha = .14f), .75f to Color.Transparent, center = c1, radius = rx1)
    val c2 = Offset(w * .62f, c1.y + ry1 * .22f)
    val rx2 = w * .52f
    val ry2 = ry1 * .72f
    val lilaHalo = Brush.radialGradient(0f to lila.copy(alpha = .08f), .72f to Color.Transparent, center = c2, radius = rx2)
    onDrawBehind {
        // El·lipses: un cercle aixafat al voltant del seu centre.
        scale(1f, ry1 / rx1, pivot = c1) { drawCircle(halo, radius = rx1, center = c1) }
        scale(1f, ry2 / rx2, pivot = c2) { drawCircle(lilaHalo, radius = rx2, center = c2) }
    }
}

/** La salutació gran de la portada, amb el degradat de l'escriptori (accent → lila → aqua en fosc; tinta en clar). */
@Composable
internal fun OpeningGreeting(text: String, modifier: Modifier = Modifier) {
    val p = LocalPal.current
    val brush = remember(p) {
        if (p.dark) Brush.horizontalGradient(listOf(p.seaClara, p.lila, p.onada))
        else Brush.horizontalGradient(listOf(p.ink, p.ink, p.seaClara))
    }
    Text(
        text, style = GregalType.hero.copy(brush = brush), textAlign = TextAlign.Center,
        modifier = modifier.padding(horizontal = 28.dp).testTag("opening-greeting"),
    )
}

/**
 * L'anell d'enfocament dels compositors (el `--ring` de l'escriptori): una vora
 * de 3 dp de l'accent, per fora de la pastilla, només mentre s'escriu.
 */
internal fun Modifier.focusRing(focused: Boolean, color: Color, corner: Dp = 28.dp): Modifier =
    if (!focused) this else drawBehind {
        val grow = 3.dp.toPx()
        drawRoundRect(
            color = color.copy(alpha = .22f),
            topLeft = Offset(-grow / 2f, -grow / 2f),
            size = Size(size.width + grow, size.height + grow),
            cornerRadius = CornerRadius(corner.toPx() + grow / 2f),
            style = Stroke(width = grow),
        )
    }

/**
 * El compositor que baixa de la portada al seu lloc quan comença la conversa
 * (FLIP: on era → on és). La portada hi apunta [from] (la vora de baix del
 * compositor, en coordenades d'arrel) i el compositor de baix, en aparèixer,
 * surt d'allà i hi llisca.
 */
internal class ComposerFlip {
    var from: Float? = null
    var offset by mutableFloatStateOf(0f)
}

internal fun Modifier.landing(flip: ComposerFlip, scope: kotlinx.coroutines.CoroutineScope, enabled: Boolean): Modifier =
    onGloballyPositioned { c ->
        val start = flip.from ?: return@onGloballyPositioned
        flip.from = null
        val d = start - c.boundsInRoot().bottom
        if (!enabled || d >= 0f) return@onGloballyPositioned
        flip.offset = d
        scope.launch {
            androidx.compose.animation.core.animate(d, 0f, animationSpec = tween(320, easing = FastOutSlowInEasing)) { v, _ -> flip.offset = v }
        }
    }.graphicsLayer { translationY = flip.offset }

/**
 * La portada neta dels Vents: la salutació gran amb la lluïssor al darrere i el
 * compositor just a sota. Res més no hi competeix: el que és nou o en marxa
 * ([below]) surt a sota, discret, i s'amaga mentre escrius. En enviar, el
 * compositor baixa al seu lloc (a baix) i llavors s'obre la conversa.
 */
@Composable
internal fun VentsOpening(
    lang: String,
    name: String,
    text: String,
    onText: (String) -> Unit,
    busy: Boolean,
    onPhoto: (() -> Unit)?,
    onDoc: (() -> Unit)?,
    onSend: (String) -> Unit,
    modifier: Modifier = Modifier,
    /** El fil de la conversa: quan n'hi ha, la portada passa a ser el xat, amb la mateixa lluïssor. */
    chat: (@Composable () -> Unit)? = null,
    chatHeader: (@Composable () -> Unit)? = null,
    below: @Composable ColumnScope.() -> Unit = {},
) {
    val p = LocalPal.current
    val motion = LocalWindMotion.current
    val scope = rememberCoroutineScope()
    var focused by remember { mutableStateOf(false) }
    // 0: sota la salutació; 1: a baix de tot, on viu el compositor de la conversa.
    val drop = remember { Animatable(0f) }
    // On acaba l'àrea visible (sobre el teclat) i on acaba el compositor: la
    // distància es calcula en dibuixar, així no depèn de l'ordre dels callbacks.
    var composerBottom by remember { mutableFloatStateOf(0f) }
    var areaBottom by remember { mutableFloatStateOf(0f) }
    val gap = with(LocalDensity.current) { 10.dp.toPx() }
    fun send() {
        val t = text.trim()
        if (t.isEmpty() || busy || drop.isRunning) return
        scope.launch {
            // Amb conversa ja no hi ha res a baixar: el camp ja és al seu lloc.
            if (motion && chat == null) drop.animateTo(1f, tween(280, easing = FastOutSlowInEasing))
            onSend(t)
        }
    }
    Box(modifier.fillMaxSize().openingHalo(p.sea, p.lila).testTag("vents-opening")) {
        if (chat != null) {
            // Conversa a la portada: la lluïssor queda al darrere, el fil al mig
            // i el camp a baix, sense canviar de pantalla.
            Column(Modifier.fillMaxSize().imePadding().testTag("vents-home-chat")) {
                chatHeader?.invoke()
                Box(Modifier.weight(1f).fillMaxWidth()) { chat() }
                AskComposer(
                    lang, text, onText, busy, onPhoto = onPhoto, onDoc = onDoc, onSend = ::send,
                    onFocus = { focused = it },
                    modifier = Modifier.padding(start = 12.dp, end = 12.dp, bottom = 8.dp, top = 4.dp),
                )
            }
        } else {
            Column(
                Modifier.fillMaxSize().imePadding().onGloballyPositioned { areaBottom = it.boundsInRoot().bottom },
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                // Una mica per damunt del centre, com a l'escriptori.
                Spacer(Modifier.weight(0.8f))
                OpeningGreeting(
                    askGreeting(lang, java.util.Calendar.getInstance().get(java.util.Calendar.HOUR_OF_DAY), name),
                    Modifier.graphicsLayer { alpha = 1f - drop.value },
                )
                Spacer(Modifier.height(28.dp))
                AskComposer(
                    lang, text, onText, busy, onPhoto = onPhoto, onDoc = onDoc, onSend = ::send,
                    onFocus = { focused = it },
                    modifier = Modifier.padding(horizontal = 16.dp)
                        .onGloballyPositioned { composerBottom = it.boundsInRoot().bottom }
                        .graphicsLayer { translationY = drop.value * (areaBottom - gap - composerBottom).coerceAtLeast(0f) },
                )
                Spacer(Modifier.weight(1.2f))
                // Escrivint, la portada és la salutació i el camp; prou.
                if (!focused && text.isBlank() && drop.value == 0f) below()
            }
        }
    }
}

/**
 * La capçalera del xat a la portada: la cara petita del Ventet, el nom i què
 * està fent. Discreta, perquè la lluïssor i el fil ja diuen on ets.
 */
@Composable
internal fun HomeThreadHeader(lang: String, name: String, accent: Color, style: WindStyle, working: Boolean) {
    Row(
        Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 6.dp, bottom = 2.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        WindHalo(accent, style, size = 30, mood = if (working) WindMood.THINKING else WindMood.IDLE, online = false)
        Column {
            Text(name, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
            Text(tr(lang, if (working) "app.cap.thinking" else "app.cap.role"), color = Muted, fontSize = 12.sp)
        }
    }
}

/** El compositor de l'inici: una pastilla gran amb el botó d'enviar rodó, l'ombra suau i l'anell en escriure. */
@Composable
internal fun AskComposer(lang: String, text: String, onText: (String) -> Unit, busy: Boolean,
    /** Afegir una foto o un document (PDF, Word, Excel…): es pugen i en queda la referència al camp. */
    onPhoto: (() -> Unit)? = null,
    onDoc: (() -> Unit)? = null,
    onFocus: (Boolean) -> Unit = {},
    modifier: Modifier = Modifier,
    onSend: () -> Unit) {
    val can = text.isNotBlank() && !busy
    val dictate = rememberDictation(lang) { onText(appendDictated(text, it)) }
    var focused by remember { mutableStateOf(false) }
    val shape = GregalShape.xl
    Row(
        modifier.fillMaxWidth().workingRing(busy, corner = 28.dp)
            .focusRing(focused, Sea)
            .shadow(if (focused) 10.dp else 4.dp, shape, clip = false)
            .clip(shape)
            .background(SuperficieAlta).border(1.dp, if (focused) Sea.copy(alpha = .55f) else LineHi, shape)
            .padding(start = if (onPhoto != null) 6.dp else 20.dp, end = 6.dp, top = 6.dp, bottom = 6.dp).testTag("ask-bar"),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (onPhoto != null || onDoc != null) {
            var menu by remember { mutableStateOf(false) }
            Box {
                Box(
                    Modifier.size(44.dp).clip(CircleShape).clickable { menu = true }.semantics { contentDescription = askCopy(lang, "attach") }.testTag("ask-attach"),
                    contentAlignment = Alignment.Center,
                ) { Icon(Icons.Rounded.Add, null, tint = Muted, modifier = Modifier.size(24.dp)) }
                DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                    if (onPhoto != null) DropdownMenuItem(
                        leadingIcon = { Icon(Icons.Rounded.AddPhotoAlternate, null, tint = Sea) },
                        text = { Text(askCopy(lang, "photo")) }, onClick = { menu = false; onPhoto() })
                    if (onDoc != null) DropdownMenuItem(
                        leadingIcon = { Icon(Icons.Rounded.Description, null, tint = Sea) },
                        text = { Column { Text(askCopy(lang, "document")); Text(askCopy(lang, "documentHelp"), color = Muted, style = GregalType.caption) } },
                        onClick = { menu = false; onDoc() })
                }
            }
            Spacer(Modifier.width(4.dp))
        }
        Box(Modifier.weight(1f).padding(vertical = 10.dp)) {
            BasicTextField(
                text, onText, textStyle = TextStyle(color = Ink, fontSize = 16.sp, lineHeight = 22.sp),
                cursorBrush = SolidColor(Sea), maxLines = 5,
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Default),
                modifier = Modifier.fillMaxWidth().testTag("ask-input")
                    .onFocusChanged { focused = it.isFocused; onFocus(it.isFocused) },
                decorationBox = { inner ->
                    Box(contentAlignment = Alignment.CenterStart) {
                        if (text.isEmpty()) Text(askCopy(lang, "hint"), color = Muted, fontSize = 16.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        inner()
                    }
                },
            )
        }
        // Camp buit: el micròfon (dictar); amb text: enviar.
        val mic = !busy && text.isBlank() && dictate != null
        Box(
            Modifier.size(48.dp).clip(CircleShape).background(if (can) Ink else Ink.copy(alpha = if (mic) .08f else .18f))
                .clickable(enabled = can || mic, onClick = { if (mic) dictate?.invoke() else onSend() })
                .semantics { contentDescription = if (mic) voiceCopy(lang, "dictate") else askCopy(lang, "send") }
                .testTag("ask-send"),
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

@Composable
internal fun TodaySection(lang: String, cards: List<TodayCard>, brief: GregalViewModel.TodayBrief? = null, briefWind: Wind? = null, onBrief: () -> Unit = {}) {
    if (cards.isEmpty() && brief == null) return
    Column(Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 28.dp).testTag("ask-today"),
        verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(askCopy(lang, "today").uppercase(), color = Muted, style = GregalType.overline,
            modifier = Modifier.padding(start = 4.dp, bottom = 2.dp))
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

/** L'estat del camp de l'inici, perquè sobrevisqui a girar la pantalla. */
@Composable
internal fun rememberAskText(): MutableState<String> = rememberSaveable { mutableStateOf("") }
