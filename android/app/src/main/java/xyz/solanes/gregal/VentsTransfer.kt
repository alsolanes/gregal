package xyz.solanes.gregal

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * Confirmació de «Continua al servidor»: COPIA, mai mou. L'historial i el
 * perfil hi van sempre; la memòria, els coneixements i les skills només si
 * es marquen. L'original d'aquí no es toca ni s'esborra.
 */
@Composable
internal fun TransferConfirmDialog(vm: GregalViewModel, lang: String, kind: String, id: String, onClose: () -> Unit) {
    val busy by vm.transferBusy.collectAsState()
    var memory by remember(kind, id) { mutableStateOf(false) }
    var knowledge by remember(kind, id) { mutableStateOf(false) }
    var skills by remember(kind, id) { mutableStateOf(false) }
    // Ja hi ha un vent amb aquest nom al servidor: per defecte, la conversa s'hi afegeix.
    var namesake by remember(kind, id) { mutableStateOf<Wind?>(null) }
    var join by remember(kind, id) { mutableStateOf(true) }
    LaunchedEffect(kind, id) { namesake = vm.serverNamesake(kind, id) }
    val server = vm.savedServerName().ifBlank { tr(lang, "app.where.server") }
    AlertDialog(
        onDismissRequest = { if (!busy) onClose() },
        title = { Text(tr(lang, "app.vents.transferToServer")) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                val same = namesake
                if (same == null) {
                    Text(tr(lang, "app.vents.transferConfirm", server), color = Tinta, fontSize = 14.sp, lineHeight = 20.sp)
                } else {
                    Text(tr(lang, "app.vents.transferNamesake", server, same.name), color = Tinta, fontSize = 14.sp, lineHeight = 20.sp)
                    TransferChoice(tr(lang, "app.vents.transferJoin", same.name), join, enabled = !busy) { join = true }
                    TransferChoice(tr(lang, "app.vents.transferNew"), !join, enabled = !busy) { join = false }
                }
                TransferTick(tr(lang, "app.vents.memoryTitle"), memory, enabled = !busy) { memory = it }
                TransferTick(tr(lang, "app.vents.topicMemory"), knowledge, enabled = !busy) { knowledge = it }
                TransferTick(tr(lang, "app.vents.skillsTitle"), skills, enabled = !busy) { skills = it }
                if (busy) Row(verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    CircularProgressIndicator(Modifier.size(16.dp), color = Sea, strokeWidth = 2.dp)
                    Text(tr(lang, "app.vents.transferWorking"), color = Muted, fontSize = 13.sp)
                }
            }
        },
        confirmButton = {
            TextButton(
                enabled = !busy,
                onClick = {
                    vm.transferConversationToServer(kind, id, memory, knowledge, skills,
                        into = namesake?.takeIf { join }?.id.orEmpty())
                    onClose()
                },
            ) { Text(tr(lang, "app.vents.transferToServer"), color = Sea) }
        },
        dismissButton = {
            TextButton(enabled = !busy, onClick = onClose) { Text(tr(lang, "app.vents.chatCancel"), color = Muted) }
        },
    )
}

@Composable
private fun TransferChoice(label: String, selected: Boolean, enabled: Boolean, onSelect: () -> Unit) {
    Row(Modifier.fillMaxWidth().clickable(enabled = enabled, onClick = onSelect),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        RadioButton(selected = selected, onClick = { if (enabled) onSelect() }, enabled = enabled)
        Text(label, color = Ink, fontSize = 14.sp, modifier = Modifier.weight(1f))
    }
}

@Composable
private fun TransferTick(label: String, checked: Boolean, enabled: Boolean, onChange: (Boolean) -> Unit) {
    Row(Modifier.fillMaxWidth().clickable(enabled = enabled, onClick = { onChange(!checked) }),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        Checkbox(checked = checked, onCheckedChange = { if (enabled) onChange(it) }, enabled = enabled)
        Text(label, color = Ink, fontSize = 14.sp, modifier = Modifier.weight(1f))
    }
}
