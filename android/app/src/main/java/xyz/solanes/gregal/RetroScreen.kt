package xyz.solanes.gregal

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.slideInVertically
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowBack
import androidx.compose.material.icons.rounded.AddReaction
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.BugReport
import androidx.compose.material.icons.rounded.Build
import androidx.compose.material.icons.rounded.Code
import androidx.compose.material.icons.rounded.Lightbulb
import androidx.compose.material.icons.rounded.Palette
import androidx.compose.material.icons.rounded.Shield
import androidx.compose.material.icons.rounded.Person
import androidx.compose.material.icons.rounded.RateReview
import androidx.compose.material.icons.rounded.Search
import androidx.compose.material.icons.rounded.Send
import androidx.compose.material.icons.rounded.Tag
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Color i icona de cada agent del #retro (com els avatars de la web). */
private data class Who(val name: String, val color: Color, val icon: ImageVector)

@Composable
private fun who(ch: RetroChannel?, id: String, lang: String): Who {
    if (id == "user") return Who(tr(lang, "app.retro.you"), Sand, Icons.Rounded.Person)
    val a = ch?.agent(id)
    val name = a?.name?.ifBlank { null } ?: id.replaceFirstChar { it.uppercase() }
    return when (a?.avatar ?: id) {
        "researcher" -> Who(name, Onada, Icons.Rounded.Search)
        "builder" -> Who(name, Green, Icons.Rounded.Build)
        "reviewer" -> Who(name, Lila, Icons.Rounded.RateReview)
        // Especialistes: revisen des del seu angle (funcionalitat, codi, seguretat).
        "design" -> Who(name, Lila, Icons.Rounded.Palette)
        "product" -> Who(name, Sand, Icons.Rounded.Lightbulb)
        "code" -> Who(name, Green, Icons.Rounded.Code)
        "security" -> Who(name, Red, Icons.Rounded.Shield)
        "qa" -> Who(name, Sea, Icons.Rounded.BugReport)
        else -> Who(name, Sea, Icons.Rounded.AutoAwesome)
    }
}

/**
 * #retro al mòbil: el xat tipus Slack/Teams on els agents expliquen què han
 * fet al torn (amb rebuts que apunten als events reals), i on els pots
 * preguntar, rebatre (@nom) i reaccionar amb emojis.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RetroScreen(vm: GregalViewModel, onMenu: () -> Unit, onBack: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val ch by vm.retro.collectAsState()
    val state by vm.retroState.collectAsState()
    val emojis by vm.retroEmojis.collectAsState()
    val haptic = LocalHapticFeedback.current
    var input by remember { mutableStateOf("") }
    val list = rememberLazyListState()
    var filter by remember { mutableStateOf("") }
    val counts = remember(ch) { retroFindingCounts(ch?.messages.orEmpty()) }
    val rows = remember(ch, filter) { retroOrdered(retroFilter(ch?.messages.orEmpty(), filter)) }
    val replying = state == "replying"
    LaunchedEffect(rows.size, replying) { if (rows.isNotEmpty()) list.animateScrollToItem(rows.size + if (replying) 1 else 0) }

    Scaffold(
        containerColor = Color.Transparent,
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Rounded.ArrowBack, tr(lang, "btn.close"), tint = Ink) } },
                title = {
                    Column {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text("#", color = Sea, fontWeight = FontWeight.Bold, fontSize = 20.sp)
                            Spacer(Modifier.width(4.dp))
                            Text("retro", color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 18.sp)
                        }
                        ch?.title?.takeIf { it.isNotBlank() }?.let {
                            Text(it, color = Faint, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        }
                    }
                },
                actions = {
                    // Qui hi ha al canal: els avatars dels agents.
                    Row(horizontalArrangement = Arrangement.spacedBy(4.dp), modifier = Modifier.padding(end = 12.dp)) {
                        ch?.agents.orEmpty().take(5).forEach { a -> Avatar(who(ch, a.id, lang), 24) }
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent),
            )
        },
        snackbarHost = { NoticeHost(vm) },
        bottomBar = {
            if (ch != null) {
                Surface(
                    shape = RoundedCornerShape(24.dp), color = PillDark, border = BorderStroke(1.dp, Line),
                    modifier = Modifier.imePadding().fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp)
                        .workingRing(active = state == "rewriting", corner = 24.dp),
                ) {
                    Row(Modifier.padding(start = 16.dp, end = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                        BasicTextField(
                            input, { input = it },
                            modifier = Modifier.weight(1f).padding(vertical = 14.dp),
                            textStyle = MaterialTheme.typography.bodyLarge.copy(color = Ink),
                            cursorBrush = SolidColor(Sea), maxLines = 5,
                            decorationBox = { inner ->
                                if (input.isEmpty()) Text(tr(lang, "app.retro.placeholder"), color = Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                inner()
                            },
                        )
                        IconButton(
                            onClick = { haptic.performHapticFeedback(HapticFeedbackType.LongPress); vm.sendRetro(input); input = "" },
                            enabled = input.isNotBlank() && !replying, modifier = Modifier.size(48.dp),
                        ) {
                            Surface(color = if (input.isNotBlank() && !replying) Sea else CodeDark, shape = CircleShape, modifier = Modifier.size(36.dp)) {
                                Box(contentAlignment = Alignment.Center) {
                                    Icon(Icons.Rounded.Send, tr(lang, "app.chat.send"), tint = if (input.isNotBlank()) OnSea else Muted, modifier = Modifier.size(17.dp))
                                }
                            }
                        }
                    }
                }
            }
        },
    ) { pad ->
        Box(Modifier.fillMaxSize().padding(pad)) {
            when {
                ch == null && (state == "loading" || state == "starting") -> RetroWaiting(
                    if (state == "starting") tr(lang, "app.retro.starting") else tr(lang, "app.ui.loading"))
                ch == null -> Column(Modifier.fillMaxSize(), verticalArrangement = Arrangement.Center, horizontalAlignment = Alignment.CenterHorizontally) {
                    EmptyBoard(Icons.Rounded.Tag, tr(lang, "app.retro.title"), state.ifBlank { tr(lang, "app.retro.none") })
                    androidx.compose.material3.OutlinedButton(onClick = { vm.openRetro() }) { Text(tr(lang, "app.chat.retry")) }
                }
                else -> LazyColumn(Modifier.fillMaxSize(), state = list) {
                    item {
                        Text(tr(lang, "app.retro.lead"), color = Faint, fontSize = 12.5.sp, lineHeight = 18.sp,
                            modifier = Modifier.padding(horizontal = 20.dp, vertical = 8.dp))
                        if (ch!!.degraded) {
                            Text(tr(lang, "app.retro.degraded"), color = Sand, fontSize = 12.5.sp,
                                modifier = Modifier.padding(horizontal = 20.dp, vertical = 4.dp))
                        }
                        if (counts.isNotEmpty()) ReviewBar(counts, filter, lang) { filter = if (filter == it) "" else it }
                        MoodBar(ch?.persona ?: "neutral", busy = state == "rewriting", lang = lang) { vm.setRetroMood(it) }
                    }
                    items(rows, key = { it.first.id }) { (m, inThread) ->
                        val prev = rows.getOrNull(rows.indexOfFirst { it.first.id == m.id } - 1)?.first
                        RetroMessageRow(ch, m, inThread, grouped = prev != null && prev.author == m.author && !inThread,
                            emojis = emojis, lang = lang, onReact = { e -> haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove); vm.reactRetro(m.id, e) },
                            onFix = { haptic.performHapticFeedback(HapticFeedbackType.LongPress); vm.fixRetro(m.id) })
                    }
                    if (replying || state == "rewriting") item { Typing(lang) }
                    if (state.isNotBlank() && state !in setOf("loading", "starting", "replying", "rewriting")) item {
                        Text(state, color = Red, fontSize = 13.sp, modifier = Modifier.padding(horizontal = 20.dp, vertical = 8.dp))
                    }
                    item { Spacer(Modifier.height(8.dp)) }
                }
            }
        }
    }
}

/** Color de cada gravetat (com la web: vermell, ambre, escuma). */
@Composable
private fun sevColor(sev: String): Color = when (sev) {
    "high" -> Red
    "medium" -> Sand
    else -> Sea
}

/** «Revisió de l'equip»: troballes per gravetat; cada xip filtra. */
@Composable
private fun ReviewBar(counts: Map<String, Int>, filter: String, lang: String, onPick: (String) -> Unit) {
    Surface(
        shape = RoundedCornerShape(14.dp), color = PillDark, border = BorderStroke(1.dp, Line),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 14.dp, vertical = 6.dp),
    ) {
        Column(Modifier.padding(horizontal = 12.dp, vertical = 10.dp)) {
            Text(tr(lang, "app.retro.review").uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.Bold, letterSpacing = 0.6.sp)
            Row(
                Modifier.padding(top = 8.dp).horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                SevChip(tr(lang, "app.retro.findings"), counts.values.sum(), null, filter == "all") { onPick("all") }
                RETRO_SEVERITIES.forEach { s ->
                    val n = counts[s] ?: 0
                    if (n > 0) SevChip(tr(lang, "app.retro.sev.$s"), n, sevColor(s), filter == s) { onPick(s) }
                }
            }
        }
    }
}

/** To del #retro: 6 moods reals. Canviar-ne reescriu els missatges d'agent. */
@Composable
private fun MoodBar(current: String, busy: Boolean, lang: String, onPick: (String) -> Unit) {
    Surface(
        shape = RoundedCornerShape(14.dp), color = PillDark, border = BorderStroke(1.dp, Line),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 14.dp, vertical = 6.dp),
    ) {
        Column(Modifier.padding(horizontal = 12.dp, vertical = 10.dp)) {
            Text(tr(lang, "app.retro.mood").uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.Bold, letterSpacing = 0.6.sp)
            if (busy) {
                Text(tr(lang, "app.retro.rewriting"), color = Sea, fontSize = 12.sp, modifier = Modifier.padding(top = 4.dp))
            }
            Row(
                Modifier.padding(top = 8.dp).horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                RETRO_MOODS.forEach { m ->
                    MoodChip(tr(lang, "app.retro.moods." + m.id), on = current == m.id, enabled = !busy) { onPick(m.id) }
                }
            }
        }
    }
}

@Composable
private fun MoodChip(label: String, on: Boolean, enabled: Boolean, onClick: () -> Unit) {
    Surface(
        shape = RoundedCornerShape(50), color = if (on) Sea.copy(alpha = 0.16f) else CodeDark,
        border = BorderStroke(1.dp, if (on) Sea.copy(alpha = 0.6f) else Line),
        modifier = Modifier.clip(RoundedCornerShape(50)).clickable(onClick = onClick, enabled = enabled),
    ) {
        Text(label, color = if (on) Sea else Muted, fontSize = 12.5.sp, fontWeight = if (on) FontWeight.Bold else FontWeight.Normal,
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp))
    }
}

@Composable
private fun SevChip(label: String, n: Int, color: Color?, on: Boolean, onClick: () -> Unit) {
    Surface(
        shape = RoundedCornerShape(50), color = if (on) Sea.copy(alpha = 0.16f) else CodeDark,
        border = BorderStroke(1.dp, if (on) Sea.copy(alpha = 0.6f) else Line),
        modifier = Modifier.clip(RoundedCornerShape(50)).clickable(onClick = onClick),
    ) {
        Row(Modifier.padding(horizontal = 12.dp, vertical = 6.dp), verticalAlignment = Alignment.CenterVertically) {
            if (color != null) {
                Box(Modifier.size(8.dp).background(color, CircleShape))
                Spacer(Modifier.width(6.dp))
            }
            Text(label, color = if (on) Ink else Tinta, fontSize = 13.sp)
            Spacer(Modifier.width(6.dp))
            Text(n.toString(), color = Muted, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
        }
    }
}

@Composable
private fun Avatar(w: Who, size: Int) {
    Box(
        Modifier.size(size.dp).background(w.color.copy(alpha = 0.18f), RoundedCornerShape((size * 0.32f).dp)),
        contentAlignment = Alignment.Center,
    ) { Icon(w.icon, null, tint = w.color, modifier = Modifier.size((size * 0.58f).dp)) }
}

@Composable
private fun RetroMessageRow(
    ch: RetroChannel?, m: RetroMsg, inThread: Boolean, grouped: Boolean,
    emojis: List<String>, lang: String, onReact: (String) -> Unit, onFix: () -> Unit,
) {
    val w = who(ch, m.author, lang)
    var picker by remember { mutableStateOf(false) }
    val visible = remember(m.id) { androidx.compose.animation.core.MutableTransitionState(m.id.startsWith("local-")).apply { targetState = true } }
    AnimatedVisibility(visible, enter = fadeIn() + slideInVertically { it / 3 }) {
        Row(Modifier.fillMaxWidth().padding(start = if (inThread) 52.dp else 14.dp, end = 14.dp, top = if (grouped) 2.dp else 12.dp)) {
            if (grouped) Spacer(Modifier.width(38.dp)) else Avatar(w, 38)
            Spacer(Modifier.width(10.dp))
            Column(Modifier.weight(1f)) {
                if (!grouped) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(w.name, color = Ink, fontWeight = FontWeight.Bold, fontSize = 14.sp)
                        if (m.concede) {
                            Spacer(Modifier.width(6.dp))
                            Text(tr(lang, "app.retro.conceded"), color = Sand, fontSize = 11.sp, fontWeight = FontWeight.SemiBold)
                        }
                        Spacer(Modifier.width(6.dp))
                        Text(retroClock(m.at), color = Faint, fontSize = 11.5.sp)
                    }
                }
                if (m.isFinding) FindingCard(m, lang, onFix)
                else Text(renderMd(m.text, CodeDark), color = if (m.kind == "verdict") Muted else Tinta, fontSize = 15.sp, lineHeight = 22.sp,
                    fontStyle = if (m.kind == "verdict") FontStyle.Italic else FontStyle.Normal,
                    modifier = Modifier.padding(top = 2.dp))
                if (m.fix.isNotBlank() && !m.isFinding) {
                    Surface(shape = RoundedCornerShape(10.dp), color = Green.copy(alpha = 0.10f), modifier = Modifier.padding(top = 6.dp)) {
                        Text("💡 " + m.fix, color = Tinta, fontSize = 13.sp, modifier = Modifier.padding(horizontal = 10.dp, vertical = 7.dp))
                    }
                }
                // Rebuts (les proves: events reals del torn), reaccions i el
                // botó per reaccionar, tot en una sola línia.
                Row(
                    Modifier.padding(top = 6.dp).horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically,
                ) {
                    m.receipts.forEach { r ->
                        Surface(shape = RoundedCornerShape(6.dp), color = CodeDark) {
                            Text(r, color = Muted, fontSize = 11.sp, fontFamily = FontFamily.Monospace,
                                modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp))
                        }
                    }
                    m.reactions.forEach { r ->
                        Surface(
                            shape = RoundedCornerShape(50), color = if (r.mine) Sea.copy(alpha = 0.16f) else CodeDark,
                            border = BorderStroke(1.dp, if (r.mine) Sea.copy(alpha = 0.5f) else Color.Transparent),
                            modifier = Modifier.clip(RoundedCornerShape(50)).clickable { onReact(r.emoji) },
                        ) {
                            Text("${r.emoji} ${r.by.size}", fontSize = 12.5.sp, color = if (r.mine) Sea else Muted,
                                modifier = Modifier.padding(horizontal = 9.dp, vertical = 3.dp))
                        }
                    }
                    if (!m.id.startsWith("local-")) Box {
                        IconButton(onClick = { picker = true }, modifier = Modifier.size(30.dp)) {
                            Icon(Icons.Rounded.AddReaction, tr(lang, "app.retro.react"), tint = Faint, modifier = Modifier.size(17.dp))
                        }
                        DropdownMenu(expanded = picker, onDismissRequest = { picker = false }) {
                            Row(Modifier.padding(horizontal = 8.dp)) {
                                emojis.forEach { e ->
                                    Text(e, fontSize = 22.sp, textAlign = TextAlign.Center,
                                        modifier = Modifier.clip(CircleShape).clickable { picker = false; onReact(e) }.padding(8.dp))
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

/** Una troballa: gravetat, títol, text i l'arranjament amb «Arregla-ho». */
@Composable
private fun FindingCard(m: RetroMsg, lang: String, onFix: () -> Unit) {
    val c = sevColor(m.sev)
    Surface(
        shape = RoundedCornerShape(12.dp), color = PillDark, border = BorderStroke(1.dp, Line),
        modifier = Modifier.fillMaxWidth().padding(top = 4.dp),
    ) {
        Row(Modifier.height(androidx.compose.foundation.layout.IntrinsicSize.Min)) {
            Box(Modifier.width(3.dp).fillMaxHeight().background(c))
            Column(Modifier.padding(horizontal = 12.dp, vertical = 10.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Surface(shape = RoundedCornerShape(50), color = c.copy(alpha = 0.16f)) {
                        Text(tr(lang, "app.retro.sev.${m.sev}").uppercase(), color = c, fontSize = 10.5.sp, fontWeight = FontWeight.Bold,
                            letterSpacing = 0.5.sp, modifier = Modifier.padding(horizontal = 7.dp, vertical = 2.dp))
                    }
                    if (m.title.isNotBlank()) {
                        Spacer(Modifier.width(8.dp))
                        Text(m.title, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.5.sp)
                    }
                }
                if (m.text.isNotBlank()) Text(renderMd(m.text, CodeDark), color = Tinta, fontSize = 14.5.sp, lineHeight = 21.sp,
                    modifier = Modifier.padding(top = 6.dp))
                if (m.fix.isNotBlank()) {
                    Box(Modifier.padding(top = 8.dp).fillMaxWidth().height(1.dp).background(Line))
                    Row(Modifier.padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text(tr(lang, "app.retro.fix").uppercase(), color = c, fontSize = 10.5.sp, fontWeight = FontWeight.Bold, letterSpacing = 0.5.sp)
                            Text(m.fix, color = Tinta, fontSize = 13.sp, lineHeight = 18.sp)
                        }
                        Spacer(Modifier.width(8.dp))
                        if (m.fixRun > 0) Text("#${m.fixRun}", color = Muted, fontSize = 12.sp)
                        else androidx.compose.material3.FilledTonalButton(onClick = onFix) { Text(tr(lang, "app.retro.fixIt"), fontSize = 13.sp) }
                    }
                }
            }
        }
    }
}

/** «L'equip escriu…» amb tres punts que salten. */
@Composable
private fun Typing(lang: String) {
    val t = rememberInfiniteTransition(label = "escriu")
    Row(Modifier.padding(horizontal = 62.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
        for (i in 0 until 3) {
            val a by t.animateFloat(0.25f, 1f, infiniteRepeatable(tween(500, delayMillis = i * 150), RepeatMode.Reverse), label = "p$i")
            Box(Modifier.padding(end = 4.dp).size(7.dp).background(Muted.copy(alpha = a), CircleShape))
        }
        Spacer(Modifier.width(6.dp))
        Text(tr(lang, "app.retro.typing"), color = Faint, fontSize = 12.5.sp)
    }
}

@Composable
private fun RetroWaiting(text: String) {
    Column(Modifier.fillMaxSize(), verticalArrangement = Arrangement.Center, horizontalAlignment = Alignment.CenterHorizontally) {
        GregalMark(56.dp, alive = true)
        Spacer(Modifier.height(16.dp))
        Text(text, color = Muted, fontSize = 14.sp, textAlign = TextAlign.Center, modifier = Modifier.padding(horizontal = 32.dp))
    }
}
