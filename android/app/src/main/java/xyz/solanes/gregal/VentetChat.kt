package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowForward
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * La conversa amb el Ventet: el mateix xat que els vents (capçalera, fil i
 * camp d'escriptura fix a baix). Les delegacions són part del fil: la
 * proposta amb les cares de qui ho farà, «endavant» o una altra opció, i
 * després l'enllaç a la conversa que n'ha sortit. Menjar, jugar i vestir-lo
 * són al seu perfil (tocant la capçalera).
 */
@Composable
internal fun VentetConversation(vm: GregalViewModel, lang: String, winds: List<Wind>, onBack: () -> Unit) {
    val ui by vm.cap.collectAsState()
    val rotllanes by vm.rotllanes.collectAsState()
    val st = ui.state
    val accent = Color(st.argb)
    val history = st.history
    var field by rememberSaveable(stateSaver = TextFieldValue.Saver) { mutableStateOf(TextFieldValue("")) }
    var profile by remember { mutableStateOf(false) }
    val listState = rememberLazyListState()
    LaunchedEffect(Unit) { vm.loadCap() }
    // El missatge que no ha arribat torna al camp: no es perd res.
    LaunchedEffect(ui.failedText) {
        val t = ui.failedText
        if (t.isNotBlank() && field.text.isBlank()) field = TextFieldValue(t, TextRange(t.length))
    }
    // Sempre a baix de tot quan arriba res nou (com els altres xats).
    LaunchedEffect(history.size, ui.busy, ui.going, ui.error) {
        val last = listState.layoutInfo.totalItemsCount - 1
        if (last >= 0) listState.scrollToItem(last)
    }
    val mood = when {
        ui.busy || ui.going -> WindMood.THINKING
        ui.pending != null -> WindMood.HAPPY
        st.briefCount > 0 -> WindMood.NEWS
        else -> WindMood.IDLE
    }
    Box(Modifier.fillMaxSize().background(Superficie).testTag("ventet-chat")) {
        Column(Modifier.fillMaxSize()) {
            VentsChatHeader(lang, st.name,
                if (ui.busy) tr(lang, "app.cap.thinking") else tr(lang, "app.cap.role"),
                accent, st.faceStyle, mood, onBack = onBack, onProfile = { profile = true })
            LazyColumn(Modifier.weight(1f).fillMaxWidth(), state = listState,
                contentPadding = PaddingValues(horizontal = 12.dp, vertical = 10.dp)) {
                if (history.isEmpty()) item {
                    if (ui.loadError) CapLoadError(lang) { vm.loadCap() }
                    else ChatEmptyState(
                        face = { WindHalo(accent, st.faceStyle, size = 88, mood = WindMood.HAPPY, tappable = true) },
                        title = st.name,
                        hint = tr(lang, "app.cap.hello"),
                    )
                }
                items(history.size, key = { it }) { i ->
                    val m = history[i]
                    val gap = if (i > 0 && history[i - 1].from != m.from) 16.dp else 4.dp
                    Column(Modifier.padding(top = gap)) {
                        when {
                            m.action == "link" -> CapLinkCard(lang, m, accent, winds, rotllanes) { vm.openVentsChat(m.link) }
                            else -> {
                                WindBubble(GregalViewModel.PetMsg(if (m.from == "user") "user" else "pet", m.text, ts = m.ts), accent, lang = lang)
                                if (m.delegates) {
                                    val pending = ui.pending
                                    if (i == history.lastIndex && pending != null) {
                                        Surface(color = accent.copy(alpha = .10f), shape = RoundedCornerShape(18.dp),
                                            modifier = Modifier.fillMaxWidth().padding(top = 6.dp)) {
                                            Box(Modifier.padding(12.dp)) { CapDecisionRow(vm, lang, pending, winds, ui.course, ui.going) }
                                        }
                                    } else CapDecisionDone(lang, m, winds)
                                }
                            }
                        }
                    }
                }
                // Mentrestant: el que ha passat des de l'última visita, al final del fil.
                if (st.briefCount > 0 && st.brief.isNotBlank()) item { CapNote(st.brief) }
                if (ui.busy || ui.going) item {
                    Row(Modifier.fillMaxWidth().padding(vertical = 8.dp), horizontalArrangement = Arrangement.Center,
                        verticalAlignment = Alignment.CenterVertically) {
                        TypingDots(accent)
                        Text("  " + tr(lang, if (ui.going) "app.cap.going" else "app.cap.thinking"), fontSize = 13.sp, color = Muted)
                    }
                }
                if (ui.error && !ui.busy) item {
                    Column(Modifier.fillMaxWidth().padding(vertical = 8.dp).testTag("cap-send-error"),
                        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text(tr(lang, "app.cap.error"), color = Red, fontSize = 14.sp, textAlign = TextAlign.Center)
                        TextButton(onClick = { field = TextFieldValue(""); vm.capRetry() }) { Text(tr(lang, "app.cap.retry"), color = Sea) }
                    }
                }
            }
            VentsComposer(lang, field, { field = it }, tr(lang, "app.cap.hint", st.name),
                canSend = field.text.isNotBlank() && !ui.busy,
                busy = ui.busy || ui.going) {
                vm.askCap(field.text); field = TextFieldValue("")
            }
        }
    }
    if (profile) VentetProfile(vm, lang) { profile = false }
}

/** No s'ha pogut carregar el Ventet: es diu i es pot tornar a provar. */
@Composable
private fun CapLoadError(lang: String, onRetry: () -> Unit) {
    Column(Modifier.fillMaxWidth().padding(32.dp).testTag("cap-load-error"),
        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(tr(lang, "app.cap.loadError"), color = Ink, fontSize = 15.sp, textAlign = TextAlign.Center)
        Button(onClick = onRetry, colors = ButtonDefaults.buttonColors(containerColor = Sea, contentColor = OnSea)) {
            Text(tr(lang, "app.cap.retry"))
        }
    }
}

/** Una proposta ja resolta (o deixada): qui la feia, en petit. */
@Composable
private fun CapDecisionDone(lang: String, m: CapMsg, winds: List<Wind>) {
    val who = m.winds.mapNotNull { id -> winds.firstOrNull { it.id == id } }
    val names = who.map { it.name } + m.create.map { it.name }
    // Propostes antigues (abans es desava menys) o vents que ja no hi són: res a dir.
    if (names.isEmpty() && m.action != "crew") return
    Row(Modifier.padding(start = 6.dp, top = 4.dp), verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        who.take(3).forEach { WindFace(size = 20, accent = Color(it.argb), style = it.faceStyle) }
        Text(when (m.action) {
            "crew" -> tr(lang, "app.cap.toCrew")
            "rotllana" -> tr(lang, "app.cap.toGroup", capRotllanaName(names, lang))
            else -> tr(lang, "app.cap.toWind", names.firstOrNull().orEmpty())
        }, color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

/** On ha anat a parar una delegació: toca-la i s'obre aquella conversa. */
@Composable
private fun CapLinkCard(lang: String, m: CapMsg, accent: Color, winds: List<Wind>, rotllanes: List<Rotllana>, onOpen: () -> Unit) {
    val id = m.link.substringAfter(':')
    val wind = if (m.link.startsWith("wind:")) winds.firstOrNull { it.id == id } else null
    val rot = if (m.link.startsWith("rot:")) rotllanes.firstOrNull { it.id == id } else null
    val faces = wind?.let { listOf(it) } ?: rot?.members?.mapNotNull { mid -> winds.firstOrNull { it.id == mid } }.orEmpty()
    val exists = wind != null || rot != null
    Surface(color = SuperficieAlta.copy(alpha = .5f), shape = RoundedCornerShape(16.dp),
        modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp))
            .clickable(enabled = exists, onClick = onOpen).testTag("cap-link")) {
        Row(Modifier.padding(horizontal = 12.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            faces.firstOrNull()?.let { WindHalo(Color(it.argb), it.faceStyle, size = 30, stacked = faces.drop(1), online = false) }
                ?: WindHalo(accent, windStyleOf("pso"), size = 30, online = false)
            Column(Modifier.weight(1f)) {
                Text(m.text, color = Ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold, maxLines = 2, overflow = TextOverflow.Ellipsis)
                Text(tr(lang, if (exists) "app.cap.openChat" else "app.cap.chatGone"), color = Muted, fontSize = 12.sp)
            }
            if (exists) Icon(Icons.AutoMirrored.Rounded.ArrowForward, null, tint = Muted)
        }
    }
}

/** Una nota del sistema al fil («mentrestant…»). */
@Composable
private fun CapNote(text: String) {
    Box(Modifier.fillMaxWidth().padding(vertical = 10.dp), contentAlignment = Alignment.Center) {
        Surface(color = SuperficieAlta.copy(alpha = .5f), shape = RoundedCornerShape(12.dp)) {
            Text(text, color = Muted, fontSize = 13.sp, textAlign = TextAlign.Center,
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp))
        }
    }
}
