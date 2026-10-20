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
            TopAppBar(
                navigationIcon = { IconButton(onClick = openDrawer) { Icon(Icons.Rounded.Menu, tr(lang, "app.nav.menu"), tint = Ink) } },
                title = { Text(tr(lang, "app.work.title"), color = Ink) },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = androidx.compose.ui.graphics.Color.Transparent),
            )
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
            Card(
                shape = RoundedCornerShape(16.dp),
                colors = CardDefaults.cardColors(containerColor = Superficie),
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    Text(tr(lang, "app.company.workTitle"), fontSize = 16.sp, fontWeight = androidx.compose.ui.text.font.FontWeight.Bold, color = Ink)
                    Text(tr(lang, "app.company.workHelp"), fontSize = 13.sp, color = Muted)
                    OutlinedTextField(
                        value = workRequest,
                        onValueChange = { workRequest = it },
                        placeholder = { Text(tr(lang, "app.company.workHint")) },
                        modifier = Modifier.fillMaxWidth(),
                        minLines = 3,
                        maxLines = 6,
                    )
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedButton(
                            onClick = { vm.startCompanyWork(workRequest, makeChanges = false); workRequest = "" },
                            enabled = workRequest.isNotBlank() && !sending,
                            shape = RoundedCornerShape(12.dp),
                            modifier = Modifier.weight(1f).height(48.dp),
                        ) {
                            Icon(Icons.Rounded.TravelExplore, null, tint = Sea, modifier = Modifier.size(16.dp))
                            Spacer(Modifier.width(6.dp))
                            Text(tr(lang, "app.company.inspect"), fontSize = 13.sp)
                        }
                        Button(
                            onClick = { vm.startCompanyWork(workRequest, makeChanges = true); workRequest = "" },
                            enabled = workRequest.isNotBlank() && !sending,
                            colors = ButtonDefaults.buttonColors(containerColor = Sea, contentColor = OnSea),
                            shape = RoundedCornerShape(12.dp),
                            modifier = Modifier.weight(1f).height(48.dp),
                        ) {
                            Icon(Icons.Rounded.Construction, null, modifier = Modifier.size(16.dp))
                            Spacer(Modifier.width(6.dp))
                            Text(tr(lang, "app.company.implement"), fontSize = 13.sp)
                        }
                    }
                }
            }
            // La feina en marxa es veu al xat del projecte (runs de l'agent).
            Text(tr(lang, "app.work.runningHint"), fontSize = 13.sp, color = Muted)
            Spacer(Modifier.height(24.dp))
        }
    }
}
