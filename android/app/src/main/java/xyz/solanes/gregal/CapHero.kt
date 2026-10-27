package xyz.solanes.gregal

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.spring
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Send
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch

/**
 * La mascota a dalt de tot dels Vents: li dius què necessites i ella decideix
 * qui s'hi posa. La decisió es veu (amb les cares de qui ho farà) i es pot
 * canviar abans de dir «endavant».
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun CapHero(vm: GregalViewModel, lang: String, winds: List<Wind>) {
    val ui by vm.cap.collectAsState()
    val st = ui.state
    val accent = Color(st.argb)
    var input by rememberSaveable { mutableStateOf("") }
    var renaming by remember { mutableStateOf(false) }
    val hop = remember { Animatable(0f) }
    val scope = rememberCoroutineScope()
    val mood = when {
        ui.busy -> WindMood.THINKING
        ui.pending != null -> WindMood.HAPPY
        st.briefCount > 0 && ui.asked.isBlank() -> WindMood.NEWS
        else -> WindMood.IDLE
    }
    // El que diu ara: pensant, la decisió, la seva resposta, o el resum en arribar.
    val lastCap = st.history.lastOrNull { it.from == "cap" }
    val says = when {
        ui.busy -> tr(lang, "app.cap.thinking")
        ui.error -> tr(lang, "app.cap.error")
        ui.pending != null -> ui.pending!!.decision.reply
        ui.asked.isNotBlank() && lastCap != null -> lastCap.text
        st.brief.isNotBlank() -> st.brief
        else -> tr(lang, "app.cap.hello")
    }
    Surface(
        color = accent.copy(alpha = .13f), shape = RoundedCornerShape(26.dp),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp).testTag("cap-hero"),
    ) {
        Column(Modifier.padding(14.dp).animateContentSize(), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Row(verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                Box(
                    Modifier.graphicsLayer { translationY = -hop.value; rotationZ = hop.value / 3f }
                        .clip(CircleShape)
                        .combinedClickable(onClick = {
                            // Un saltet quan la toques: és el mimat de la colla.
                            scope.launch {
                                hop.animateTo(26f, spring(dampingRatio = .45f, stiffness = Spring.StiffnessMedium))
                                hop.animateTo(0f, spring(dampingRatio = .35f, stiffness = Spring.StiffnessLow))
                            }
                        }, onLongClick = { renaming = true })
                        .semantics { contentDescription = st.name },
                ) {
                    WindHalo(accent, st.faceStyle, size = 58, mood = mood, online = false, showKit = true)
                }
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(st.name, color = Ink, fontWeight = FontWeight.Bold, fontSize = 16.sp,
                            modifier = Modifier.combinedClickable(onClick = {}, onLongClick = { renaming = true }))
                        Text(tr(lang, "app.cap.role"), color = Muted, fontSize = 12.sp)
                    }
                    if (ui.asked.isNotBlank() && !ui.error) {
                        Text("«" + oneLineUi(ui.asked, 90) + "»", color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                    Surface(color = Superficie, shape = RoundedCornerShape(topStart = 4.dp, topEnd = 18.dp, bottomEnd = 18.dp, bottomStart = 18.dp)) {
                        Text(renderMd(says, SuperficieAlta), color = Tinta, fontSize = 14.sp, lineHeight = 19.sp,
                            maxLines = 12, overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.padding(horizontal = 12.dp, vertical = 9.dp).testTag("cap-says"))
                    }
                }
            }
            ui.pending?.let { CapDecisionRow(vm, lang, it, winds) }
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    input, { input = it }, singleLine = false, maxLines = 4, enabled = !ui.busy,
                    placeholder = { Text(tr(lang, "app.cap.hint", st.name), fontSize = 14.sp) },
                    shape = RoundedCornerShape(22.dp),
                    keyboardOptions = KeyboardOptions(imeAction = ImeAction.Send),
                    keyboardActions = KeyboardActions(onSend = { if (input.isNotBlank()) { vm.askCap(input); input = "" } }),
                    modifier = Modifier.weight(1f).testTag("cap-input"),
                    colors = OutlinedTextFieldDefaults.colors(
                        focusedContainerColor = Superficie, unfocusedContainerColor = Superficie,
                        focusedBorderColor = accent, unfocusedBorderColor = Color.Transparent,
                    ),
                )
                val canSend = input.isNotBlank() && !ui.busy
                Surface(color = if (canSend) Sea else SuperficieAlta, shape = CircleShape,
                    modifier = Modifier.size(44.dp).clip(CircleShape)
                        .combinedClickable(enabled = canSend, onClick = { vm.askCap(input); input = "" })
                        .semantics { contentDescription = tr(lang, "app.chat.send") }.testTag("cap-send")) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(Icons.Rounded.Send, null, tint = if (canSend) OnSea else Muted, modifier = Modifier.size(19.dp))
                    }
                }
            }
        }
    }
    if (renaming) {
        var name by remember { mutableStateOf(st.name) }
        AlertDialog(
            onDismissRequest = { renaming = false },
            title = { Text(tr(lang, "app.cap.rename")) },
            text = { OutlinedTextField(name, { name = it.take(24) }, singleLine = true) },
            confirmButton = { TextButton(enabled = name.isNotBlank(), onClick = { vm.renameCap(name); renaming = false }) { Text(tr(lang, "app.cap.renameOk"), color = Sea) } },
            dismissButton = { TextButton(onClick = { renaming = false }) { Text("✕", color = Muted) } },
        )
    }
}

/** Qui ho farà, «endavant» i els altres camins. */
@Composable
private fun CapDecisionRow(vm: GregalViewModel, lang: String, p: GregalViewModel.CapPending, winds: List<Wind>) {
    val d = p.decision
    val who = d.winds.mapNotNull { id -> winds.firstOrNull { it.id == id } }
    var other by remember { mutableStateOf(false) }
    Column(verticalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.testTag("cap-decision")) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            who.take(3).forEach { WindFace(size = 30, accent = Color(it.argb), style = it.faceStyle) }
            Text(
                when (d.action) {
                    "crew" -> tr(lang, "app.cap.toCrew")
                    "rotllana" -> tr(lang, "app.cap.toGroup", capRotllanaName(who.map { it.name }, lang))
                    else -> tr(lang, "app.cap.toWind", who.firstOrNull()?.name.orEmpty())
                },
                color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f),
                maxLines = 2, overflow = TextOverflow.Ellipsis,
            )
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            Button(onClick = { vm.capGo() }, colors = ButtonDefaults.buttonColors(containerColor = Sea, contentColor = OnSea),
                modifier = Modifier.testTag("cap-go")) { Text(tr(lang, "app.cap.go")) }
            Box {
                OutlinedButton(onClick = { other = true }, modifier = Modifier.testTag("cap-other")) { Text(tr(lang, "app.cap.other"), color = Ink) }
                DropdownMenu(expanded = other, onDismissRequest = { other = false }) {
                    DropdownMenuItem(text = { Text(tr(lang, "app.cap.self")) }, onClick = { other = false; vm.askCap(p.message, "self") })
                    winds.filter { w -> !(d.action == "wind" && w.id in d.winds) }.forEach { w ->
                        DropdownMenuItem(
                            leadingIcon = { WindFace(size = 24, accent = Color(w.argb), style = w.faceStyle) },
                            text = { Text(tr(lang, "app.cap.passTo", w.name)) },
                            onClick = { other = false; vm.askCap(p.message, "wind:" + w.id) },
                        )
                    }
                    if (winds.size >= 2 && d.action != "rotllana") {
                        DropdownMenuItem(text = { Text(tr(lang, "app.cap.group")) }, onClick = { other = false; vm.askCap(p.message, "rotllana") })
                    }
                    if (d.action != "crew") {
                        DropdownMenuItem(text = { Text(tr(lang, "app.cap.crew")) }, onClick = { other = false; vm.askCap(p.message, "crew") })
                    }
                }
            }
            TextButton(onClick = { vm.capDismiss() }) { Text(tr(lang, "app.cap.later"), color = Muted) }
        }
    }
}

private fun oneLineUi(s: String, max: Int): String {
    val t = s.split(Regex("\\s+")).filter { it.isNotEmpty() }.joinToString(" ")
    return if (t.length > max) t.take(max - 1) + "…" else t
}
