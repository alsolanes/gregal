package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.ArrowBack
import androidx.compose.material.icons.rounded.Send
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * PetChatSheet — xat amb una mascota.
 *
 * Full-screen dialog: avatar + nom a dalt, bombolles a sota, input al final.
 * Les bombolles de la mascota porten el seu color d'accent suau; les de
 * l'usuari, el blau del sistema.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PetChatSheet(vm: GregalViewModel, pet: Pet, onBack: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val chat by vm.petChat.collectAsState()
    val busy by vm.petChatBusy.collectAsState()
    val msgs = chat[pet.id].orEmpty()
    val accent = PetAvatars.accent(pet.id)

    Scaffold(
        containerColor = Color.Transparent,
        topBar = {
            TopAppBar(
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Rounded.ArrowBack, tr(lang, "app.nav.menu"), tint = Ink)
                    }
                },
                title = {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        PetAvatar(pet.id, size = 28)
                        Spacer(Modifier.width(10.dp))
                        Column {
                            Text(pet.name, fontSize = 16.sp, fontWeight = FontWeight.Bold, color = Ink)
                            Text(pet.role, fontSize = 12.sp, color = Muted)
                        }
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent),
            )
        },
    ) { pad ->
        Column(
            Modifier
                .fillMaxSize()
                .padding(pad)
                .imePadding(),
        ) {
            // Historial
            Column(
                Modifier
                    .weight(1f)
                    .fillMaxWidth()
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = 16.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                if (msgs.isEmpty()) {
                    Text(
                        tr(lang, "app.company.chatEmpty"),
                        fontSize = 14.sp,
                        color = Muted,
                        textAlign = androidx.compose.ui.text.style.TextAlign.Center,
                        modifier = Modifier.fillMaxWidth().padding(vertical = 32.dp),
                    )
                }
                for (m in msgs) {
                    PetBubble(m, accent)
                }
                if (busy.contains(pet.id)) {
                    Text("…", fontSize = 16.sp, color = Muted)
                }
                Spacer(Modifier.height(8.dp))
            }
            // Input
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 12.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                var text by remember { mutableStateOf("") }
                OutlinedTextField(
                    value = text,
                    onValueChange = { text = it },
                    placeholder = { Text(tr(lang, "app.company.chatHint"), fontSize = 14.sp) },
                    shape = RoundedCornerShape(22.dp),
                    modifier = Modifier.weight(1f),
                    maxLines = 3,
                )
                FilledIconButton(
                    onClick = {
                        vm.sendPetMessage(pet.id, text)
                        text = ""
                    },
                    enabled = text.isNotBlank() && !busy.contains(pet.id),
                    shape = CircleShape,
                ) {
                    Icon(Icons.Rounded.Send, null, tint = Color.White)
                }
            }
        }
    }
}

/** Una bombolla: esquerra (mascota, accent suau) o dreta (usuari, mar). */
@Composable
private fun PetBubble(m: GregalViewModel.PetMsg, accent: Long) {
    val mine = m.from == "user"
    Row(
        Modifier.fillMaxWidth(),
        horizontalArrangement = if (mine) Arrangement.End else Arrangement.Start,
    ) {
        Surface(
            shape = RoundedCornerShape(
                topStart = 14.dp, topEnd = 14.dp,
                bottomStart = if (mine) 14.dp else 3.dp,
                bottomEnd = if (mine) 3.dp else 14.dp,
            ),
            color = if (mine) Sea.copy(alpha = 0.14f) else Color(accent.toInt()).copy(alpha = 0.16f),
            modifier = Modifier.widthIn(max = 300.dp),
        ) {
            Text(
                m.text,
                fontSize = 14.sp,
                color = if (mine) Tinta else Color(accent.toInt()),
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
            )
        }
    }
}
