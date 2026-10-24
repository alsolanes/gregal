package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Menu
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material.icons.rounded.TravelExplore
import androidx.compose.material.icons.rounded.Construction
import androidx.compose.material.icons.rounded.ChatBubbleOutline
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * WorkScreen: el mode feina, separat dels vents.
 *
 * Aquí es delega feina real al motor d'agent del projecte actiu:
 * investigar (només mirar) o implementar (amb aprovació pas a pas).
 * Els vents proposen des del seu xat; aquí es fa.
 */
@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
fun WorkScreen(vm: GregalViewModel, openDrawer: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val sending by vm.sending.collectAsState()

    Scaffold(
        containerColor = androidx.compose.ui.graphics.Color.Transparent,
        topBar = {
            GregalTopBar(tr(lang, "app.work.title"), lang, onMenu = openDrawer)
        },
        snackbarHost = { NoticeHost(vm) },
    ) { pad ->
        Column(
            Modifier
                .fillMaxSize()
                .padding(pad)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            var workRequest by remember { mutableStateOf("") }
            GCard(Modifier.fillMaxWidth()) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Box(Modifier.size(40.dp).background(Sand.copy(alpha = 0.16f), RoundedCornerShape(13.dp)), contentAlignment = Alignment.Center) {
                        Icon(Icons.Rounded.Construction, null, tint = Sand, modifier = Modifier.size(22.dp))
                    }
                    Text(tr(lang, "app.company.workTitle"), fontSize = 17.sp, fontWeight = FontWeight.Bold, color = Ink)
                }
                Spacer(Modifier.height(14.dp))
                OutlinedTextField(
                    value = workRequest,
                    onValueChange = { workRequest = it },
                    placeholder = { Text(tr(lang, "app.company.workHint"), color = Faint) },
                    modifier = Modifier.fillMaxWidth(),
                    minLines = 3,
                    maxLines = 6,
                    shape = RoundedCornerShape(16.dp),
                    colors = OutlinedTextFieldDefaults.colors(
                        focusedContainerColor = DeepTop, unfocusedContainerColor = DeepTop,
                        unfocusedBorderColor = Line, focusedBorderColor = Sea.copy(alpha = 0.6f),
                    ),
                )
                Spacer(Modifier.height(12.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    FilledTonalButton(
                        onClick = { vm.startCompanyWork(workRequest, makeChanges = false); workRequest = "" },
                        enabled = workRequest.isNotBlank() && !sending,
                        shape = RoundedCornerShape(999.dp),
                        modifier = Modifier.weight(1f).height(48.dp),
                    ) {
                        Icon(Icons.Rounded.TravelExplore, null, modifier = Modifier.size(18.dp))
                        Spacer(Modifier.width(6.dp))
                        Text(tr(lang, "app.company.inspect"), fontWeight = FontWeight.SemiBold)
                    }
                    Button(
                        onClick = { vm.startCompanyWork(workRequest, makeChanges = true); workRequest = "" },
                        enabled = workRequest.isNotBlank() && !sending,
                        colors = ButtonDefaults.buttonColors(containerColor = Sea, contentColor = OnSea),
                        shape = RoundedCornerShape(999.dp),
                        modifier = Modifier.weight(1f).height(48.dp),
                    ) {
                        Icon(Icons.Rounded.Construction, null, modifier = Modifier.size(18.dp))
                        Spacer(Modifier.width(6.dp))
                        Text(tr(lang, "app.company.implement"), fontWeight = FontWeight.SemiBold)
                    }
                }
            }
            // Com funciona: tres passos clars en lloc d'un paràgraf dens.
            Text(tr(lang, "app.work.how").uppercase(), fontSize = 11.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 1.4.sp,
                color = Faint, modifier = Modifier.padding(start = 4.dp, top = 8.dp))
            GCard(Modifier.fillMaxWidth(), padding = PaddingValues(vertical = 4.dp)) {
                SettingsRow(Icons.Rounded.TravelExplore, tr(lang, "app.company.inspect"), tr(lang, "app.work.howInspect"), tint = Onada)
                SettingsRow(Icons.Rounded.Construction, tr(lang, "app.company.implement"), tr(lang, "app.work.howImplement"), tint = Sea)
                SettingsRow(Icons.Rounded.ChatBubbleOutline, tr(lang, "app.work.howResultTitle"), tr(lang, "app.work.runningHint"), tint = Lila)
            }
            Spacer(Modifier.height(24.dp))
        }
    }
}
