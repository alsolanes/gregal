package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Send
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Checkbox
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** Manté al dia la llista i les invitacions mentre els Vents són oberts (cada 20 s). */
@Composable
internal fun SharedChatsSync(vm: GregalViewModel) {
    val conn by vm.conn.collectAsState()
    LifecyclePoll(conn) {
        if (conn.host.isBlank()) return@LifecyclePoll
        while (true) {
            SharedChatsStore.refresh(conn.api())
            delay(20_000)
        }
    }
}

/** Les rotllanes compartides a la llista de xats, amb la seva capçalera. */
internal fun LazyListScope.sharedChatRows(chats: List<SharedChatInfo>, busy: Set<String>, lang: String, onOpen: (String) -> Unit) {
    if (chats.isEmpty()) return
    item(key = "shared-hdr") { InboxHeader(tr(lang, "app.shared.section"), count = chats.size, lang = lang) }
    items(chats, key = { "shared:" + it.id }) { c ->
        val face = c.winds.firstOrNull()
        val preview = when {
            c.lastText.isNotBlank() -> (if (c.lastAuthor.isNotBlank()) c.lastAuthor + ": " else "") + c.lastText.lineSequence().first()
            else -> c.humans.joinToString(", ")
        }
        ConversationRow(c.name, preview, face?.let { Color(it.argb) } ?: Sea, face?.faceStyle ?: WindStyle(),
            busy = c.id in busy, faces = c.winds, timestamp = c.updated, onClick = { onOpen(c.id) }, lang = lang)
    }
}

/** La invitació salta en obrir l'app: Accepta o Rebutja. Tancar-la la deixa per a més tard. */
@Composable
internal fun SharedInvitePopup(vm: GregalViewModel, lang: String, onAccepted: (String) -> Unit) {
    val invites by SharedChatsStore.invites.collectAsState()
    val later = remember { mutableStateListOf<String>() }
    val scope = rememberCoroutineScope()
    var working by remember { mutableStateOf(false) }
    val inv = invites.firstOrNull { it.id !in later } ?: return
    fun answer(accept: Boolean) {
        working = true
        scope.launch {
            try {
                SharedChatsStore.answer(vm.conn.value.api(), inv.id, accept)
                if (accept) onAccepted(inv.id)
            } catch (e: Exception) {
                vm.notify(tr(lang, "app.shared.error", e.message.orEmpty().take(120)))
                later += inv.id
            } finally { working = false }
        }
    }
    AlertDialog(
        onDismissRequest = { if (!working) later += inv.id },
        title = { Text(tr(lang, "app.shared.inviteTitle")) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(tr(lang, "app.shared.inviteText", inv.owner, inv.name), color = Ink)
                val others = inv.people.map { it.name }.filter { it != SharedChatsStore.me.value }
                if (others.isNotEmpty()) Text(tr(lang, "app.shared.alsoInvited", others.joinToString(", ")), color = Muted, fontSize = 13.sp)
                if (inv.winds.isNotEmpty()) Text(tr(lang, "app.shared.windsInChat", inv.winds.joinToString(", ") { it.name }), color = Muted, fontSize = 13.sp)
            }
        },
        confirmButton = { TextButton(enabled = !working, onClick = { answer(true) }) { Text(tr(lang, "app.shared.accept"), color = Sea) } },
        dismissButton = { TextButton(enabled = !working, onClick = { answer(false) }) { Text(tr(lang, "app.shared.decline"), color = Muted) } },
    )
}

/** Crear un xat compartit: nom, els meus vents que hi parlen i les persones del servidor. */
@Composable
internal fun SharedChatCreator(vm: GregalViewModel, lang: String, winds: List<Wind>, onClose: () -> Unit, onOpened: (String) -> Unit) {
    var name by rememberSaveable { mutableStateOf("") }
    val pickedWinds = remember { mutableStateListOf<String>() }
    val pickedPeople = remember { mutableStateListOf<String>() }
    var people by remember { mutableStateOf<List<String>?>(null) }
    var error by remember { mutableStateOf("") }
    var working by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    LaunchedEffect(Unit) {
        people = try { kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) { vm.conn.value.api().sharedPeople() } }
        catch (e: Exception) { error = tr(lang, "app.shared.noAccounts"); emptyList() }
    }
    val ok = name.isNotBlank() && pickedWinds.isNotEmpty() && pickedPeople.isNotEmpty() && !working
    AlertDialog(
        onDismissRequest = { if (!working) onClose() },
        title = { Text(tr(lang, "app.shared.new")) },
        text = {
            Column(Modifier.heightIn(max = 520.dp).verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text(tr(lang, "app.shared.help"), color = Muted, fontSize = 13.sp)
                OutlinedTextField(name, { name = it }, singleLine = true, label = { Text(tr(lang, "app.shared.name")) }, modifier = Modifier.fillMaxWidth())
                Text(tr(lang, "app.shared.people"), fontWeight = FontWeight.SemiBold, color = Ink, modifier = Modifier.padding(top = 8.dp))
                when {
                    people == null -> Text("…", color = Muted)
                    people!!.isEmpty() -> Text(error.ifBlank { tr(lang, "app.shared.noPeople") }, color = Muted, fontSize = 13.sp)
                    else -> people!!.forEach { p ->
                        PickRow(p, p in pickedPeople) { if (p in pickedPeople) pickedPeople.remove(p) else pickedPeople.add(p) }
                    }
                }
                Text(tr(lang, "app.shared.winds"), fontWeight = FontWeight.SemiBold, color = Ink, modifier = Modifier.padding(top = 8.dp))
                if (winds.isEmpty()) Text(tr(lang, "app.shared.noWinds"), color = Muted, fontSize = 13.sp)
                winds.forEach { w ->
                    PickRow(w.name, w.id in pickedWinds, Color(w.argb)) { if (w.id in pickedWinds) pickedWinds.remove(w.id) else pickedWinds.add(w.id) }
                }
                Text(tr(lang, "app.shared.privacy"), color = Muted, fontSize = 12.sp, modifier = Modifier.padding(top = 8.dp))
                if (error.isNotBlank() && people?.isNotEmpty() == true) Text(error, color = Color(0xFFB3261E), fontSize = 13.sp)
            }
        },
        confirmButton = {
            TextButton(enabled = ok, onClick = {
                working = true; error = ""
                scope.launch {
                    try {
                        val chat = SharedChatsStore.save(vm.conn.value.api(), name, pickedWinds.toList(), pickedPeople.toList())
                        onClose(); onOpened(chat.id)
                    } catch (e: Exception) {
                        error = tr(lang, "app.shared.error", e.message.orEmpty().substringAfter(": ").take(160))
                    } finally { working = false }
                }
            }) { Text(tr(lang, "app.shared.create"), color = if (ok) Sea else Muted) }
        },
        dismissButton = { TextButton(enabled = !working, onClick = onClose) { Text(tr(lang, "app.vents.back"), color = Muted) } },
    )
}

@Composable
private fun PickRow(label: String, checked: Boolean, dot: Color? = null, onToggle: () -> Unit) {
    Row(Modifier.fillMaxWidth().clickable(onClick = onToggle), verticalAlignment = Alignment.CenterVertically) {
        Checkbox(checked, { onToggle() })
        if (dot != null) Box(Modifier.padding(end = 8.dp).background(dot, RoundedCornerShape(50)).padding(5.dp))
        Text(label, color = Ink)
    }
}

/** El xat compartit: qui ha escrit cada missatge (persona o vent) i el camp per escriure. */
@Composable
internal fun SharedChatConversation(vm: GregalViewModel, lang: String, chat: SharedChatInfo, onBack: () -> Unit) {
    val histories by SharedChatsStore.history.collectAsState()
    val busySet by SharedChatsStore.busy.collectAsState()
    val msgs = histories[chat.id].orEmpty()
    val busy = chat.id in busySet
    val scope = rememberCoroutineScope()
    var field by rememberSaveable(chat.id) { mutableStateOf("") }
    var info by remember(chat.id) { mutableStateOf(false) }
    val listState = rememberLazyListState()
    val accent = chat.winds.firstOrNull()?.let { Color(it.argb) } ?: Sea
    // El fil es rellegeix cada pocs segons: seguir-lo només si HI ÉS, al final.
    // Abans un LaunchedEffect(size) hi tornava sempre i et saltava a baix
    // mentre pujaves a rellegir.
    val follow = rememberChatFollow(listState, chat.id)
    val keys = remember(msgs) { chatRowKeys(msgs) { m -> "s${m.ts}|${m.speakerId}|${m.author}" } }
    // Les altres persones escriuen des dels seus mòbils: el fil es torna a llegir sovint,
    // però només mentre es mira (en segon pla no té sentit llegir-lo).
    LifecyclePoll(chat.id) {
        while (true) {
            try { SharedChatsStore.load(vm.conn.value.api(), chat.id) } catch (_: Exception) {}
            delay(if (chat.id in SharedChatsStore.busy.value) 2_000 else 4_000)
        }
    }
    val follow = rememberChatFollow(listState, chat.id)
    androidx.activity.compose.BackHandler(onBack = onBack)
    Column(Modifier.fillMaxSize().background(Superficie).imePadding()) {
        VentsChatHeader(lang, chat.name,
            if (busy) tr(lang, "app.company.groupThinking") else (chat.humans + chat.winds.map { it.name }).joinToString(", "),
            accent, chat.winds.firstOrNull()?.faceStyle ?: WindStyle(),
            if (busy) WindMood.THINKING else WindMood.IDLE, chat.winds,
            onBack = onBack, onProfile = { info = true })
        Box(Modifier.weight(1f)) {
        LazyColumn(Modifier.fillMaxSize(), state = listState,
            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 10.dp)) {
            if (msgs.isEmpty()) item {
                Text(tr(lang, "app.shared.empty", chat.humans.filter { it != SharedChatsStore.me.value }.joinToString(", ")), color = Muted, fontSize = 14.sp,
                    modifier = Modifier.fillMaxWidth().padding(24.dp))
            }
            items(msgs.size, key = { idx -> keys[idx] }) { idx ->
                val m = msgs[idx]
                val wind = chat.winds.firstOrNull { it.id == m.speakerId }
                val prev = msgs.getOrNull(idx - 1)
                val firstOfRun = prev == null || prev.speakerId != m.speakerId
                Column(Modifier.padding(top = if (firstOfRun && idx > 0) 14.dp else 4.dp)) {
                    WindBubble(m.asPetMsg(), if (m.mine) accent else wind?.let { Color(it.argb) } ?: Muted, lang = lang,
                        speaker = if (firstOfRun) wind else null,
                        speakerName = if (firstOfRun && !m.mine) m.speakerName else "")
                }
            }
            if (busy) item {
                Row(Modifier.fillMaxWidth().padding(vertical = 6.dp), horizontalArrangement = Arrangement.Center, verticalAlignment = Alignment.CenterVertically) {
                    TypingDots(accent)
                    Text("  " + tr(lang, "app.company.groupThinking"), fontSize = 12.sp, color = accent)
                }
            }
        }
        if (msgs.isNotEmpty() && !follow.following) LatestPill(lang) {
            follow.requestFollow(); scope.launch { listState.scrollToLatest() }
        }
        }
        Row(Modifier.fillMaxWidth().padding(horizontal = 10.dp, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            OutlinedTextField(field, { field = it }, modifier = Modifier.weight(1f), maxLines = 5,
                placeholder = { Text(tr(lang, "app.shared.hint"), fontSize = 14.sp) }, shape = RoundedCornerShape(22.dp))
            IconButton(enabled = field.isNotBlank() && !busy, onClick = {
                val text = field.trim(); field = ""
                follow.requestFollow()
                scope.launch {
                    try { SharedChatsStore.send(vm.conn.value.api(), chat.id, text) }
                    catch (e: Exception) { vm.notify(tr(lang, "app.shared.error", e.message.orEmpty().take(120))) }
                }
            }) { Icon(Icons.Rounded.Send, tr(lang, "app.shared.send"), tint = if (field.isNotBlank() && !busy) accent else Muted) }
        }
    }
    if (info) SharedChatInfoDialog(vm, lang, chat, onClose = { info = false }, onLeft = onBack)
}

/** Qui hi ha al xat (i qui encara no ha respost) i sortir-ne; el propietari l'esborra per a tothom. */
@Composable
private fun SharedChatInfoDialog(vm: GregalViewModel, lang: String, chat: SharedChatInfo, onClose: () -> Unit, onLeft: () -> Unit) {
    var confirm by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    AlertDialog(
        onDismissRequest = onClose,
        title = { Text(chat.name) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text(tr(lang, "app.shared.people"), fontWeight = FontWeight.SemiBold, color = Ink)
                Text("• " + chat.owner + " — " + tr(lang, "app.shared.status.owner"), color = Ink, fontSize = 14.sp)
                chat.people.forEach { p ->
                    Text("• " + p.name + " — " + tr(lang, "app.shared.status." + p.status.ifBlank { "invited" }), color = Ink, fontSize = 14.sp)
                }
                Text(tr(lang, "app.shared.winds"), fontWeight = FontWeight.SemiBold, color = Ink, modifier = Modifier.padding(top = 6.dp))
                Text(chat.winds.joinToString(", ") { it.name }, color = Ink, fontSize = 14.sp)
                if (confirm) Text(tr(lang, if (chat.mine) "app.shared.deleteConfirm" else "app.shared.leaveConfirm"), color = Color(0xFFB3261E), fontSize = 13.sp)
            }
        },
        confirmButton = {
            TextButton(onClick = {
                if (!confirm) { confirm = true; return@TextButton }
                scope.launch {
                    try { SharedChatsStore.leave(vm.conn.value.api(), chat.id); onClose(); onLeft() }
                    catch (e: Exception) { vm.notify(tr(lang, "app.shared.error", e.message.orEmpty().take(120))) }
                }
            }) { Text(tr(lang, if (chat.mine) "app.shared.delete" else "app.shared.leave"), color = Color(0xFFB3261E)) }
        },
        dismissButton = { TextButton(onClick = onClose) { Text(tr(lang, "app.library.close"), color = Muted) } },
    )
}
