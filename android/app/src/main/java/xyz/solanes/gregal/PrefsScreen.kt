package xyz.solanes.gregal

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Menu
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.Checkbox
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Preferències: aparença i llengua. Paritat amb el panell prefs de la web
 *  (mateixes opcions de tema; densitat/mida les cobreix el sistema). */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PrefsScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val tema by vm.tema.collectAsState()
    val idioma by vm.idioma.collectAsState()
    val st by vm.state.collectAsState()
    val updateMsg by vm.updateMsg.collectAsState()
    val info by vm.update.collectAsState()
    val prog by vm.updateProg.collectAsState()
    val ctx = LocalContext.current
    val vc = remember { vm.versionCodeInstalada() }
    // Només s'ensenya la targeta si el manifest és més nou que el
    // versionCode instal·lat (si no, seria un botó que no fa res).
    val teNova = info?.takeIf { it.isAvailable }
    fun t(k: String, vararg args: Any?) = tr(idioma, k, *args)
    GregalBg {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                TopAppBar(
                    navigationIcon = { IconButton(onClick = onMenu) { Icon(Icons.Default.Menu, tr(idioma, "app.nav.menu"), tint = Ink) } },
                    title = { Text(t("prefs.title"), color = Ink) },
                    colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent))
            },
        ) { pad ->
            Column(Modifier.fillMaxSize().padding(pad).padding(16.dp).verticalScroll(rememberScrollState())) {
                Text(t("prefs.appearance"), color = Ink, fontSize = 16.sp)
                Spacer(Modifier.height(4.dp))
                Text(t("prefs.theme"), color = Muted, fontSize = 13.sp)
                Spacer(Modifier.height(4.dp))
                listOf("sistema" to "prefs.theme.system", "fosc" to "prefs.theme.dark", "clar" to "prefs.theme.light").forEach { (v, k) ->
                    Row(Modifier.fillMaxWidth().selectable(v == tema) { vm.setTema(v) }.padding(vertical = 8.dp),
                        verticalAlignment = Alignment.CenterVertically) {
                        RadioButton(selected = v == tema, onClick = { vm.setTema(v) })
                        Text(t(k), color = Ink, fontSize = 15.sp)
                    }
                }
                Text(t("prefs.theme.help"), color = Muted, fontSize = 12.sp)
                Spacer(Modifier.height(16.dp))
                Text(t("prefs.lang"), color = Ink, fontSize = 16.sp)
                Spacer(Modifier.height(4.dp))
                listOf("ca" to "Català", "en" to "English").forEach { (v, nom) ->
                    Row(Modifier.fillMaxWidth().selectable(v == idioma) { vm.setIdioma(v) }.padding(vertical = 8.dp),
                        verticalAlignment = Alignment.CenterVertically) {
                        RadioButton(selected = v == idioma, onClick = { vm.setIdioma(v) })
                        Text(nom, color = Ink, fontSize = 15.sp)
                    }
                }
                Spacer(Modifier.height(16.dp))
                Text(t("prefs.verify"), color = Ink, fontSize = 16.sp)
                Spacer(Modifier.height(4.dp))
                Row(Modifier.fillMaxWidth().selectable(st.verify == "auto") { vm.setVerify(st.verify != "auto") }.padding(vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically) {
                    Checkbox(checked = st.verify == "auto", onCheckedChange = { vm.setVerify(it) })
                    Text(t("prefs.verify.help"), color = Muted, fontSize = 13.sp, modifier = Modifier.weight(1f))
                }

                Spacer(Modifier.height(20.dp))
                Text(t("app.settings.version"), color = Ink, fontSize = 16.sp)
                Spacer(Modifier.height(4.dp))
                Text("Gregal Android ${vm.versionNameInstalada()} (vc${vm.versionCodeInstalada()})",
                    color = Muted, fontSize = 13.sp)
                Spacer(Modifier.height(6.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    OutlinedButton(onClick = { vm.checkUpdate() }) { Text(t("app.settings.checkUpdates")) }
                    Spacer(Modifier.width(10.dp))
                    Text(updateMsg, color = if (teNova != null) Sand else Muted, fontSize = 12.sp,
                        modifier = Modifier.weight(1f))
                }
                teNova?.let { r ->
                    Spacer(Modifier.height(8.dp))
                    GlassCard(Modifier.fillMaxWidth(), hi = true) {
                        Column(Modifier.padding(12.dp)) {
                            Text(t("app.update.install", r.latestVersion), color = Sea, fontSize = 15.sp)
                            if (r.changelog.isNotEmpty()) {
                                Spacer(Modifier.height(4.dp))
                                Text(r.changelog, color = Tinta, fontSize = 13.sp)
                            }
                            Spacer(Modifier.height(8.dp))
                            if (prog >= 0) {
                                // Baixant: barra + percentatge, sense botons.
                                LinearProgressIndicator(
                                    progress = { prog / 100f },
                                    modifier = Modifier.fillMaxWidth(),
                                    color = Sea, trackColor = Line,
                                )
                                Spacer(Modifier.height(4.dp))
                                Text(t("app.update.downloadProgress", prog), color = Muted, fontSize = 12.sp)
                            } else {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Button(onClick = { vm.baixaIInstalla() }) { Text(t("app.update.downloadInstall")) }
                                    Spacer(Modifier.width(8.dp))
                                    OutlinedButton(onClick = { vm.ignoraAquestaVersio() }) {
                                        Text(t("app.update.silence"))
                                    }
                                }
                                Spacer(Modifier.height(4.dp))
                                Text(
                                    t("app.update.shaHelp"),
                                    color = Faint, fontSize = 11.sp,
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}
