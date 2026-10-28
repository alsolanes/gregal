package xyz.solanes.gregal

import androidx.compose.foundation.layout.imePadding

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
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.ui.draw.clip
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.toggleable
import androidx.compose.ui.semantics.Role
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.Toys
import androidx.compose.material.icons.rounded.Dns
import androidx.compose.material.icons.rounded.SystemUpdate
import androidx.compose.material.icons.rounded.Verified
import androidx.compose.material.icons.rounded.Lock
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.TextButton
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.material.icons.rounded.Menu
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
    val temaApp by vm.temaApp.collectAsState()
    val idioma by vm.idioma.collectAsState()
    val idiomaChoice by vm.idiomaChoice.collectAsState()
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
                GregalTopBar(t("prefs.title"), idioma, onMenu = onMenu)
            },
            snackbarHost = { NoticeHost(vm) },
        ) { pad ->
            Column(Modifier.fillMaxSize().padding(pad).imePadding().verticalScroll(rememberScrollState()).padding(start = 16.dp, end = 16.dp, bottom = 104.dp)) {
                // On corre Gregal i amb quin model: abans no es veia enlloc
                // fora del calaix, i la clau només es podia posar des del
                // desplegable del model.
                val conn by vm.conn.collectAsState()
                PrefsSection(t("app.prefs.server"))
                GCard(Modifier.fillMaxWidth(), padding = PaddingValues(vertical = 4.dp)) {
                    SettingsRow(
                        Icons.Rounded.Dns, t("app.prefs.server"),
                        if (conn.local) t("app.server.thisDevice") else t("app.prefs.serverRemote", "${conn.host}:${conn.port}"),
                        tint = Onada,
                        trailing = { TextButton(onClick = { vm.switchServer() }) { Text(t("app.server.switch"), color = Sea) } },
                    )
                    HorizontalDivider(Modifier.padding(start = 66.dp), color = Line)
                    SettingsRow(
                        Icons.Rounded.AutoAwesome, t("app.prefs.model"),
                        st.provider.ifBlank { "—" } + if (st.model.isNotBlank()) " · " + st.model else "",
                    )
                }
                Spacer(Modifier.height(12.dp))
                ProviderSetup(vm, current = st.provider, change = true)

                PrefsSection(t("prefs.appearance"))
                GCard(Modifier.fillMaxWidth()) {
                    Text(t("prefs.theme"), color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                    Spacer(Modifier.height(10.dp))
                    ChoicePills(listOf("sistema" to t("prefs.theme.system"), "fosc" to t("prefs.theme.dark"), "clar" to t("prefs.theme.light")), tema, { vm.setTema(it) })
                    Spacer(Modifier.height(8.dp))
                    Text(t("app.prefs.themeHelp"), color = Muted, fontSize = 12.5.sp, lineHeight = 17.sp)
                    Spacer(Modifier.height(18.dp))
                    Text(t("prefs.appTheme"), color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                    Spacer(Modifier.height(8.dp))
                    Text(t("prefs.appThemeHelp"), color = Muted, fontSize = 12.5.sp, lineHeight = 17.sp)
                    Spacer(Modifier.height(10.dp))
                    // El mostrari ensenya la paleta de debò: dues bombolles i
                    // l'accent, amb el clar/fosc que tens triat ara mateix.
                    val foscAra = esFosc(tema, isSystemInDarkTheme())
                    Row(
                        Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                    ) {
                        for (tem in GregalTemes) {
                            TemaMostra(
                                tem = tem, fosc = foscAra, triat = temaApp == tem.id,
                                nom = t("prefs.tema." + tem.id),
                            ) { vm.setTemaApp(tem.id) }
                        }
                    }
                    Spacer(Modifier.height(18.dp))
                    Text(t("prefs.lang"), color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                    Spacer(Modifier.height(10.dp))
                    ChoicePills(listOf("system" to t("app.prefs.languageSystem"), "ca" to "Català", "en" to "English"), idiomaChoice, { vm.setIdioma(it) })
                }

                PrefsSection(t("prefs.reviewer"))
                GCard(Modifier.fillMaxWidth(), padding = PaddingValues(vertical = 4.dp)) {
                    Row(
                        Modifier.fillMaxWidth().toggleable(st.autoReview, role = Role.Switch) { vm.setReviewer(it) }
                            .padding(horizontal = 16.dp, vertical = 12.dp),
                        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp),
                    ) {
                        Box(Modifier.size(36.dp).background(Sand.copy(alpha = 0.14f), RoundedCornerShape(11.dp)), contentAlignment = Alignment.Center) {
                            Icon(Icons.Rounded.Lock, null, tint = Sand, modifier = Modifier.size(20.dp))
                        }
                        Text(t("prefs.reviewer.help"), color = Tinta, fontSize = 14.sp, lineHeight = 19.sp, modifier = Modifier.weight(1f))
                        Switch(checked = st.autoReview, onCheckedChange = null,
                            colors = SwitchDefaults.colors(checkedTrackColor = Sea, checkedThumbColor = OnSea))
                    }
                }

                PrefsSection(t("prefs.verify"))
                GCard(Modifier.fillMaxWidth(), padding = PaddingValues(vertical = 4.dp)) {
                    Row(
                        Modifier.fillMaxWidth().toggleable(st.verify == "auto", role = Role.Switch) { vm.setVerify(it) }
                            .padding(horizontal = 16.dp, vertical = 12.dp),
                        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp),
                    ) {
                        Box(Modifier.size(36.dp).background(Green.copy(alpha = 0.14f), RoundedCornerShape(11.dp)), contentAlignment = Alignment.Center) {
                            Icon(Icons.Rounded.Verified, null, tint = Green, modifier = Modifier.size(20.dp))
                        }
                        Text(t("prefs.verify.help"), color = Tinta, fontSize = 14.sp, lineHeight = 19.sp, modifier = Modifier.weight(1f))
                        Switch(checked = st.verify == "auto", onCheckedChange = null,
                            colors = SwitchDefaults.colors(checkedTrackColor = Sea, checkedThumbColor = OnSea))
                    }
                }

                PrefsSection(t("app.settings.version"))
                GCard(Modifier.fillMaxWidth(), padding = PaddingValues(vertical = 4.dp)) {
                    SettingsRow(
                        Icons.Rounded.SystemUpdate, "Gregal Android ${vm.versionNameInstalada()}",
                        updateMsg.ifBlank { "vc${vm.versionCodeInstalada()}" }, tint = Lila,
                        trailing = { TextButton(onClick = { vm.checkUpdate() }) { Text(t("app.settings.checkUpdates"), color = Sea) } },
                    )
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

/** Títol de secció de preferències: el mateix estil discret de tota l'app. */
@Composable
private fun PrefsSection(text: String) {
    Text(
        text.uppercase(), fontSize = 11.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 1.4.sp, color = Faint,
        modifier = Modifier.padding(start = 4.dp, top = 22.dp, bottom = 8.dp),
    )
}

/** Un quadre del selector de temes: mini conversa amb la paleta de debò. */
@Composable
private fun TemaMostra(tem: GregalTema, fosc: Boolean, triat: Boolean, nom: String, onPick: () -> Unit) {
    val p = tem.perMode(fosc)
    Column(
        Modifier.width(96.dp)
            .clip(RoundedCornerShape(14.dp))
            .background(p.surface)
            .border(if (triat) 2.dp else 1.dp, if (triat) p.sea else p.lineHi, RoundedCornerShape(14.dp))
            .clickable(onClick = onPick)
            .padding(9.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box(Modifier.fillMaxWidth().height(52.dp).clip(RoundedCornerShape(10.dp)).background(p.deepTop)) {
            Column(Modifier.fillMaxSize().padding(6.dp), verticalArrangement = Arrangement.spacedBy(5.dp)) {
                Box(Modifier.width(38.dp).height(10.dp).clip(RoundedCornerShape(6.dp)).background(p.surfaceHi))
                Box(
                    Modifier.width(26.dp).height(10.dp).clip(RoundedCornerShape(6.dp))
                        .background(p.sea).align(Alignment.End),
                )
                Box(Modifier.width(30.dp).height(6.dp).clip(RoundedCornerShape(4.dp)).background(p.surfaceHi))
            }
        }
        Spacer(Modifier.height(7.dp))
        Text(
            nom, color = if (triat) p.sea else p.text, fontSize = 12.sp, maxLines = 1,
            fontWeight = if (triat) FontWeight.SemiBold else FontWeight.Normal,
        )
    }
}
