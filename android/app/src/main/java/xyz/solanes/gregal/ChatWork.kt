package xyz.solanes.gregal

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Check
import androidx.compose.material.icons.rounded.Checklist
import androidx.compose.material.icons.rounded.Close
import androidx.compose.material.icons.rounded.KeyboardArrowDown
import androidx.compose.material.icons.rounded.Lock
import androidx.compose.material.icons.rounded.LockOpen
import androidx.compose.material.icons.rounded.Mic
import androidx.compose.material.icons.rounded.Send
import androidx.compose.material.icons.rounded.Stop
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay

/** Rellotge que avança cada segon mentre `running` (només recompon la capçalera). */
@Composable
private fun elapsedSince(since: Long, running: Boolean): Long {
    val now by produceState(System.currentTimeMillis(), since, running) {
        while (running) {
            value = System.currentTimeMillis()
            delay(1000)
        }
    }
    return (if (running) now else System.currentTimeMillis()) - since
}

/** Tres barres d'onada que pugen i baixen: l'agent és viu. */
@Composable
fun WaveBars(color: Color, modifier: Modifier = Modifier) {
    val t = rememberInfiniteTransition(label = "onada")
    val phase by t.animateFloat(0f, 1f, infiniteRepeatable(tween(900, easing = LinearEasing)), label = "fase")
    Canvas(modifier.size(16.dp)) {
        val w = size.width / 5f
        for (i in 0 until 3) {
            val p = (phase + i * 0.22f) % 1f
            val h = size.height * (0.35f + 0.65f * kotlin.math.abs(kotlin.math.sin(p * Math.PI)).toFloat())
            drawRoundRect(
                color, topLeft = Offset(w * (i * 2), (size.height - h) / 2f),
                size = Size(w, h), cornerRadius = CornerRadius(w / 2f, w / 2f),
            )
        }
    }
}

/** Franja de llum que recorre la vora de dalt mentre l'agent treballa. */
@Composable
private fun SweepLine(colors: List<Color>) {
    val t = rememberInfiniteTransition(label = "escombrada")
    val x by t.animateFloat(-0.4f, 1.4f, infiniteRepeatable(tween(1600, easing = FastOutSlowInEasing)), label = "x")
    Canvas(Modifier.fillMaxWidth().height(2.dp)) {
        val w = size.width
        drawRect(Brush.horizontalGradient(
            listOf(Color.Transparent) + colors + listOf(Color.Transparent),
            startX = w * (x - 0.35f), endX = w * (x + 0.35f),
        ))
    }
}

/**
 * El treball d'un torn en una sola targeta plegable (com la web): en curs
 * mostra què fa ara i quanta estona fa; acabat, un resum d'una línia
 * («Ha treballat 32s · 5 eines») que es desplega en una línia de temps.
 */
@Composable
fun WorkGroupCard(work: WorkGroup, activity: Activity?, statusLine: String, lang: String, onRetro: (() -> Unit)? = null) {
    val running = work.running
    var open by remember { mutableStateOf(false) }
    // Un torn que falla s'obre sol: el que interessa és veure on.
    LaunchedEffect(work.failed, running) { if (!running && work.failed) open = true }
    val elapsed = elapsedSince(work.started, running)
    val total = if (running) elapsed else work.ended - work.started
    val stalled = running && activity != null && activity.key in setOf("model", "start") &&
        System.currentTimeMillis() - activity.since > 90_000
    val chevron by animateFloatAsState(if (open) 180f else 0f, spring(stiffness = 600f), label = "fletxa")
    val border = when {
        running -> Sea.copy(alpha = 0.35f)
        work.failed -> Red.copy(alpha = 0.45f)
        else -> Line
    }
    Surface(
        shape = RoundedCornerShape(16.dp),
        color = Superficie,
        border = BorderStroke(1.dp, border),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 2.dp, vertical = 6.dp),
    ) {
        Column(Modifier.animateContentSize(spring(stiffness = 500f))) {
            if (running) SweepLine(listOf(Sea, Onada, Lila))
            Row(
                Modifier.fillMaxWidth()
                    .clickable(role = Role.Button) { open = !open }
                    .padding(horizontal = 14.dp, vertical = 11.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(Modifier.size(26.dp), contentAlignment = Alignment.Center) {
                    when {
                        running -> WaveBars(if (stalled) Sand else Sea)
                        work.failed -> StatusDot(Red, "!")
                        work.stopped -> StatusDot(Muted, "■")
                        else -> StatusDot(Green, null)
                    }
                }
                Spacer(Modifier.width(10.dp))
                Column(Modifier.weight(1f)) {
                    val title = if (running) {
                        (activity?.text ?: tr(lang, "app.work.thinking")) +
                            if (total >= 3000) " · " + shortDuration(total) else ""
                    } else {
                        val parts = mutableListOf(tr(lang, if (work.failed || work.stopped) "app.work.failed" else "app.work.done", shortDuration(total)))
                        val tools = work.toolCount
                        if (tools == 1) parts += tr(lang, "app.work.tool1") else if (tools > 1) parts += tr(lang, "app.work.tools", tools)
                        else if (work.steps.size > 1) parts += tr(lang, "app.work.steps", work.steps.size)
                        if (work.failedCount > 0) parts += tr(lang, "app.work.errors", work.failedCount)
                        parts.joinToString(" · ")
                    }
                    AnimatedContent(
                        targetState = title.substringBefore(" · ").let { if (running) activity?.key ?: "" else "done" },
                        transitionSpec = {
                            (fadeIn(tween(180)) + slideInVertically { it / 2 }) togetherWith
                                (fadeOut(tween(120)) + slideOutVertically { -it / 2 })
                        },
                        label = "activitat",
                    ) { _ ->
                        Text(
                            title, color = if (running) Ink else Muted, fontSize = 14.sp,
                            fontWeight = if (running) FontWeight.SemiBold else FontWeight.Medium,
                            maxLines = 1, overflow = TextOverflow.Ellipsis,
                        )
                    }
                    val sub = when {
                        stalled -> tr(lang, "app.work.stalled")
                        running && work.steps.isNotEmpty() -> work.steps.last().let { s -> listOf(s.label, s.target).filter { it.isNotBlank() }.joinToString(" · ") }
                        running -> statusLine
                        else -> ""
                    }
                    if (sub.isNotBlank()) {
                        Text(sub, color = if (stalled) Sand else Faint, fontSize = 12.sp, maxLines = 1,
                            overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = 1.dp))
                    }
                }
                // El #retro d'aquest torn: el xat amb els agents sobre què han fet.
                if (!running && onRetro != null) {
                    Surface(
                        shape = RoundedCornerShape(50), color = Sea.copy(alpha = 0.13f),
                        modifier = Modifier.padding(end = 4.dp).clip(RoundedCornerShape(50)).clickable(onClick = onRetro),
                    ) {
                        Row(Modifier.padding(horizontal = 10.dp, vertical = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                            Text("#", color = Sea, fontWeight = FontWeight.Bold, fontSize = 13.sp)
                            Spacer(Modifier.width(3.dp))
                            Text(tr(lang, "app.retro.open"), color = Sea, fontWeight = FontWeight.SemiBold, fontSize = 12.5.sp)
                        }
                    }
                }
                if (work.steps.isNotEmpty()) {
                    Icon(Icons.Rounded.KeyboardArrowDown, null, tint = Muted,
                        modifier = Modifier.size(22.dp).rotate(chevron))
                }
            }
            if (open && work.steps.isNotEmpty()) {
                Column(Modifier.padding(start = 14.dp, end = 12.dp, bottom = 10.dp)) {
                    work.steps.forEachIndexed { i, s -> StepRow(s, last = i == work.steps.lastIndex, lang = lang) }
                }
            }
        }
    }
}

@Composable
private fun StatusDot(color: Color, mark: String?) {
    Box(Modifier.size(20.dp).background(color.copy(alpha = 0.16f), CircleShape), contentAlignment = Alignment.Center) {
        if (mark == null) Icon(Icons.Rounded.Check, null, tint = color, modifier = Modifier.size(13.dp))
        else Text(mark, color = color, fontSize = 12.sp, fontWeight = FontWeight.Bold)
    }
}

/** Un pas de la línia de temps: punt d'estat, acció, objectiu i detall plegable. */
@Composable
private fun StepRow(s: WorkStep, last: Boolean, lang: String) {
    var showDetail by remember(s.id) { mutableStateOf(false) }
    val color = when {
        s.state == StepState.Running -> Sea
        s.state == StepState.Failed -> Red
        s.kind == StepKind.User -> Sand
        s.kind == StepKind.Warn -> Sand
        s.kind == StepKind.Think || s.kind == StepKind.Note -> Lila
        s.kind == StepKind.Delegate -> Onada
        s.kind == StepKind.Tool -> Green
        else -> Muted
    }
    val lineColor = Line
    Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min)) {
        // Columna de la línia de temps.
        Box(Modifier.width(22.dp).fillMaxHeight()) {
            if (!last) Canvas(Modifier.fillMaxHeight().width(22.dp)) {
                drawLine(lineColor, Offset(size.width / 2, 18.dp.toPx()), Offset(size.width / 2, size.height), 1.5.dp.toPx())
            }
            Box(Modifier.padding(top = 5.dp).size(22.dp, 12.dp), contentAlignment = Alignment.Center) {
                if (s.state == StepState.Running) PulsingDot(color) else
                    Box(Modifier.size(8.dp).background(color, CircleShape))
            }
        }
        Column(
            Modifier.weight(1f)
                .clip(RoundedCornerShape(8.dp))
                .then(if (s.detail.isNotBlank()) Modifier.clickable { showDetail = !showDetail } else Modifier)
                .padding(start = 6.dp, end = 4.dp, top = 2.dp, bottom = 8.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(s.label, color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold,
                    maxLines = if (s.kind == StepKind.Note || s.kind == StepKind.Info) 2 else 1,
                    overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
                if (s.meta.isNotBlank()) {
                    Spacer(Modifier.width(6.dp))
                    Text(s.meta, color = Faint, fontSize = 11.sp, maxLines = 1)
                }
            }
            if (s.target.isNotBlank()) {
                Text(s.target, color = Muted, fontSize = 12.sp,
                    fontFamily = if (s.kind == StepKind.Tool) FontFamily.Monospace else FontFamily.Default,
                    maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
            if (showDetail) {
                Surface(shape = RoundedCornerShape(8.dp), color = CodeDark,
                    modifier = Modifier.fillMaxWidth().padding(top = 6.dp)) {
                    Text(
                        s.detail.take(2500), color = Tinta, fontSize = 11.5.sp, lineHeight = 16.sp,
                        fontFamily = FontFamily.Monospace, maxLines = 24, overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.heightIn(max = 360.dp).horizontalScroll(rememberScrollState()).padding(10.dp),
                    )
                }
            }
        }
    }
}

@Composable
private fun PulsingDot(color: Color) {
    val t = rememberInfiniteTransition(label = "pols")
    val r by t.animateFloat(0.6f, 1.25f, infiniteRepeatable(tween(800), RepeatMode.Reverse), label = "r")
    Canvas(Modifier.size(12.dp)) {
        drawCircle(color.copy(alpha = 0.25f), radius = size.minDimension / 2 * r)
        drawCircle(color, radius = size.minDimension / 4)
    }
}

/** La resposta mentre s'escriu: el mateix markdown i un cursor que respira. */
@Composable
fun LiveAnswer(text: String, lang: String) {
    Column(Modifier.fillMaxWidth()) {
        AssistantResponse(text, lang)
        val t = rememberInfiniteTransition(label = "cursor")
        val a by t.animateFloat(1f, 0.15f, infiniteRepeatable(tween(520), RepeatMode.Reverse), label = "a")
        Box(Modifier.padding(start = 8.dp, bottom = 6.dp).size(9.dp, 16.dp)
            .background(Sea.copy(alpha = a), RoundedCornerShape(2.dp)))
    }
}

// --- Composer: Pregunta/Planifica/Actua, modes avançats i permisos ---

/** Selector Pregunta | Planifica | Actua amb la píndola que llisca (com la web i el TUI). */
@Composable
fun ModeToggle(uiMode: String, enabled: Boolean, lang: String, onChange: (String) -> Unit) {
    Surface(shape = RoundedCornerShape(50), color = CodeDark, modifier = Modifier.height(34.dp)) {
        Row(Modifier.padding(3.dp), verticalAlignment = Alignment.CenterVertically) {
            listOf("chat" to "app.mode.ask", "plan" to "app.mode.plan", "act" to "app.mode.act").forEach { (v, key) ->
                val label = tr(lang, key)
                val sel = v == uiMode
                val bg by androidx.compose.animation.animateColorAsState(if (sel) Sea else Color.Transparent, tween(180), label = "bg")
                val fg by androidx.compose.animation.animateColorAsState(if (sel) OnSea else Muted, tween(180), label = "fg")
                Box(
                    Modifier.fillMaxHeight().clip(RoundedCornerShape(50)).background(bg)
                        .clickable(enabled = enabled, role = Role.Tab) { if (!sel) onChange(v) }
                        .padding(horizontal = 12.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(label, color = fg, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, maxLines = 1)
                }
            }
        }
    }
}

/** Xip damunt del composer per a un mode especial o uns permisos no habituals. */
@Composable
fun ComposerChip(text: String, color: Color, icon: (@Composable () -> Unit)?, onClick: () -> Unit, onClose: (() -> Unit)?) {
    Surface(
        shape = RoundedCornerShape(50), color = color.copy(alpha = 0.13f),
        border = BorderStroke(1.dp, color.copy(alpha = 0.35f)),
        modifier = Modifier.clip(RoundedCornerShape(50)).clickable(onClick = onClick),
    ) {
        Row(Modifier.padding(start = 10.dp, end = if (onClose != null) 2.dp else 12.dp), verticalAlignment = Alignment.CenterVertically) {
            if (icon != null) { icon(); Spacer(Modifier.width(6.dp)) }
            Text(text, color = color, fontSize = 12.5.sp, fontWeight = FontWeight.SemiBold,
                maxLines = 1, modifier = Modifier.padding(vertical = 7.dp))
            if (onClose != null) {
                IconButton(onClick = onClose, modifier = Modifier.size(32.dp)) {
                    Icon(Icons.Rounded.Close, null, tint = color, modifier = Modifier.size(15.dp))
                }
            }
        }
    }
}

@Composable
fun PermIcon(open: Boolean, color: Color) =
    Icon(if (open) Icons.Rounded.LockOpen else Icons.Rounded.Lock, null, tint = color, modifier = Modifier.size(15.dp))

/**
 * Un sol botó a la dreta: atura si treballa, envia si hi ha text, dicta si
 * està buit. Canvia de forma amb una animació curta.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun SendStopButton(busy: Boolean, canSend: Boolean, voice: Boolean, lang: String,
                   onSend: () -> Unit, onStop: () -> Unit, onVoice: () -> Unit, onInterrupt: (() -> Unit)? = null) {
    val mode = when { busy && !canSend -> 2; canSend -> 1; voice -> 0; else -> 1 }
    Box(
        Modifier.size(48.dp).clip(CircleShape).combinedClickable(
            onClick = { when (mode) { 2 -> onStop(); 1 -> if (canSend) onSend(); else -> onVoice() } },
            // Mentre treballa: mantenir premut interromp i li diu ara.
            onLongClick = if (busy && canSend && onInterrupt != null) onInterrupt else null,
        ).semantics { contentDescription = tr(lang, if (mode == 2) "app.chat.stop" else "app.chat.send") },
        contentAlignment = Alignment.Center,
    ) {
        AnimatedContent(targetState = mode, transitionSpec = {
            (fadeIn(tween(140)) + androidx.compose.animation.scaleIn(initialScale = 0.7f)) togetherWith
                (fadeOut(tween(100)) + androidx.compose.animation.scaleOut(targetScale = 0.7f))
        }, label = "enviar") { m ->
            when (m) {
                0 -> Icon(Icons.Rounded.Mic, tr(lang, "app.chat.dictate"), tint = Muted, modifier = Modifier.size(23.dp))
                2 -> Surface(color = Ink, shape = CircleShape, modifier = Modifier.size(36.dp)) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(Icons.Rounded.Stop, tr(lang, "app.chat.stop"), tint = Superficie, modifier = Modifier.size(18.dp))
                    }
                }
                else -> Surface(color = if (canSend) (if (busy) Sand else Sea) else CodeDark, shape = CircleShape, modifier = Modifier.size(36.dp)) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(Icons.Rounded.Send, null, tint = if (canSend) OnSea else Muted, modifier = Modifier.size(17.dp))
                    }
                }
            }
        }
    }
}

/** Menú «+»: adjuntar, comparar models, modes avançats i permisos. */
@Composable
fun ComposerPlusMenu(
    expanded: Boolean, onClose: () -> Unit, lang: String, st: ServerState,
    canUseInput: Boolean, canAttach: Boolean,
    onAttach: () -> Unit, onCompare: () -> Unit,
    onMode: (String) -> Unit, onPerm: () -> Unit,
    onAttachDoc: () -> Unit = {},
) {
    androidx.compose.material3.DropdownMenu(expanded = expanded, onDismissRequest = onClose) {
        @Composable
        fun item(label: String, help: String = "", enabled: Boolean = true, checked: Boolean = false, onClick: () -> Unit) {
            androidx.compose.material3.DropdownMenuItem(
                enabled = enabled,
                text = {
                    Column {
                        Text(label, color = if (enabled) Ink else Faint, fontSize = 14.sp)
                        if (help.isNotEmpty()) Text(help, color = Faint, fontSize = 11.5.sp, maxLines = 2)
                    }
                },
                trailingIcon = if (checked) ({ Icon(Icons.Rounded.Check, null, tint = Sea) }) else null,
                onClick = { onClick(); onClose() },
            )
        }
        item(tr(lang, "app.composer.attach"), enabled = canAttach, onClick = onAttach)
        item(tr(lang, "app.composer.attachDoc"), tr(lang, "app.composer.attachDocHelp"), onClick = onAttachDoc)
        item(tr(lang, "app.composer.compare"), enabled = canUseInput, onClick = onCompare)
        androidx.compose.material3.HorizontalDivider(color = Line)
        Text(tr(lang, "app.mode.more"), Modifier.padding(horizontal = 14.dp, vertical = 6.dp),
            color = Muted, fontSize = 11.5.sp, fontWeight = FontWeight.Bold)
        item(tr(lang, "app.mode.goal"), tr(lang, "app.mode.goalHelp"), checked = st.mode == "goal") { onMode("goal") }
        item(tr(lang, "app.mode.debug"), tr(lang, "app.mode.debugHelp"), checked = st.mode == "debug") { onMode("debug") }
        if (st.mode in setOf("goal", "debug")) item(tr(lang, "app.mode.exit")) { onMode("act") }
        androidx.compose.material3.HorizontalDivider(color = Line)
        item(tr(lang, "app.perm.title") + " · " + tr(lang, "app.perm." + st.access), onClick = onPerm)
    }
}

/** Permís d'Actua (com la web i el TUI): demana, edicions o autonomia total. */
@Composable
fun PermissionDialog(st: ServerState, lang: String, onPick: (String) -> Unit, onClose: () -> Unit) {
    val cur = st.access
    androidx.compose.material3.AlertDialog(
        onDismissRequest = onClose,
        title = { Text(tr(lang, "app.perm.title")) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                // A Pregunta i Planifica no es canvia res: triar-ne un passa a Actua.
                if (uiModeOf(st.mode) != "act") {
                    Text(tr(lang, "app.perm.appliesInAct"), color = Muted, fontSize = 12.sp)
                }
                ACCESSES.forEach { id ->
                    val l = "app.perm.$id"
                    val h = "app.perm.${id}Help"
                    val sel = id == cur
                    Surface(
                        shape = RoundedCornerShape(14.dp),
                        color = if (sel) Sea.copy(alpha = 0.12f) else CodeDark,
                        border = BorderStroke(1.dp, if (sel) Sea.copy(alpha = 0.5f) else Color.Transparent),
                        modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp))
                            .clickable(role = Role.RadioButton) { onPick(id) },
                    ) {
                        Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
                            PermIcon(id == "full", if (id == "full") Sand else if (sel) Sea else Muted)
                            Spacer(Modifier.width(10.dp))
                            Column(Modifier.weight(1f)) {
                                Text(tr(lang, l), color = Ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
                                Text(tr(lang, h), color = Muted, fontSize = 12.sp)
                            }
                            if (sel) Icon(Icons.Rounded.Check, null, tint = Sea, modifier = Modifier.size(18.dp))
                        }
                    }
                }
            }
        },
        confirmButton = { androidx.compose.material3.TextButton(onClick = onClose) { Text(tr(lang, "btn.close")) } },
    )
}
