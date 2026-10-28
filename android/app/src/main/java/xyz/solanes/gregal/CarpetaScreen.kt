package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.ArrowUpward
import androidx.compose.material.icons.rounded.CheckCircle
import androidx.compose.material.icons.rounded.ChevronRight
import androidx.compose.material.icons.rounded.ErrorOutline
import androidx.compose.material.icons.rounded.FolderOpen
import androidx.compose.material.icons.rounded.Home
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * Carpeta de treball: on llegeixen, escriuen i executen ordres els vents.
 *
 * Fins ara la carpeta era la que tenia el servidor en arrencar i des del mòbil
 * no es podia canviar: qui volia treballar en un altre projecte havia d'anar a
 * l'ordinador. Aquí es tria entre els projectes recordats o navegant pel disc,
 * i el servidor valida sempre que la carpeta existeixi i sigui teva.
 *
 * El servidor no deixa canviar-la amb una feina en curs (els camins, els diffs
 * i els permisos pengen d'ella), i això es diu tal com és, sense un error cru.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CarpetaScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val idioma by vm.idioma.collectAsState()
    val ws by vm.workspaces.collectAsState()
    val dirs by vm.carpeta.collectAsState()
    val busy by vm.carpetaBusy.collectAsState()
    val error by vm.carpetaError.collectAsState()
    fun t(k: String, vararg args: Any?) = tr(idioma, k, *args)

    // En entrar s'actualitza: la carpeta pot haver canviat des de la web o
    // des d'un altre mòbil, i ensenyar-la vella seria pitjor que no ensenyar-la.
    LaunchedEffect(Unit) {
        vm.loadWorkspaces()
        vm.obrirCarpeta(null)
    }

    GregalBg {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = { GregalTopBar(t("app.carpeta.title"), idioma, onMenu = onMenu) },
            snackbarHost = { NoticeHost(vm) },
        ) { pad ->
            LazyColumn(
                Modifier.fillMaxSize().padding(pad),
                contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 110.dp),
                verticalArrangement = Arrangement.spacedBy(14.dp),
            ) {
                if (busy) {
                    item { LinearProgressIndicator(Modifier.fillMaxWidth(), color = Sea, trackColor = Line) }
                }

                // --- on es treballa ara ---
                item {
                    val actual = ws?.current.orEmpty()
                    val projecte = ws?.project?.ifBlank { actual.trimEnd('/').substringAfterLast('/') }.orEmpty()
                    GCard(Modifier.fillMaxWidth()) {
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                            Box(
                                Modifier.size(44.dp).background(Sea.copy(alpha = 0.14f), CircleShape),
                                contentAlignment = Alignment.Center,
                            ) { Text(inicialsProjecte(projecte), color = Sea, fontWeight = FontWeight.Bold, fontSize = 15.sp) }
                            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(3.dp)) {
                                Text(
                                    projecte.ifBlank { t("app.carpeta.none") },
                                    color = Ink, fontSize = 17.sp, fontWeight = FontWeight.SemiBold,
                                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                                )
                                Text(
                                    camiCurt(actual),
                                    color = Muted, fontSize = 12.5.sp, fontFamily = FontFamily.Monospace,
                                    maxLines = 2, overflow = TextOverflow.Ellipsis,
                                )
                            }
                            val branca = ws?.branch.orEmpty()
                            if (branca.isNotBlank()) Etiqueta(branca, Sea)
                        }
                        Spacer(Modifier.height(10.dp))
                        Text(t("app.carpeta.help"), color = Muted, fontSize = 12.5.sp, lineHeight = 17.sp)
                    }
                }

                // --- error amb reintent ---
                if (!error.isNullOrBlank()) {
                    item {
                        GCard(Modifier.fillMaxWidth()) {
                            Row(verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                                Icon(Icons.Rounded.ErrorOutline, null, tint = Red, modifier = Modifier.size(20.dp))
                                Column(Modifier.weight(1f)) {
                                    Text(missatgeCarpeta(error.orEmpty(), idioma), color = Ink, fontSize = 14.sp, lineHeight = 19.sp)
                                    TextButton(onClick = { vm.reintentarCarpeta() }) { Text(t("app.carpeta.retry"), color = Sea) }
                                }
                            }
                        }
                    }
                }

                // --- projectes recordats ---
                item { TitolSeccio(t("app.carpeta.projects")) }
                item {
                    val llista = ws?.list.orEmpty()
                    GCard(Modifier.fillMaxWidth(), padding = PaddingValues(vertical = 4.dp)) {
                        if (llista.isEmpty()) {
                            Text(
                                t("app.carpeta.noProjects"), color = Muted, fontSize = 13.sp,
                                modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp),
                            )
                        }
                        llista.forEach { p ->
                            val triada = p.path == ws?.current
                            FilaCarpeta(
                                nom = p.name.ifBlank { p.path.trimEnd('/').substringAfterLast('/') },
                                cami = camiCurt(p.path),
                                detall = when {
                                    triada -> t("app.carpeta.inUse")
                                    p.missing -> t("app.carpeta.missing")
                                    else -> null
                                },
                                detallColor = if (p.missing && !triada) Red else Sea,
                                icona = if (triada) Icons.Rounded.CheckCircle else Icons.Rounded.FolderOpen,
                                activa = triada,
                                enabled = !p.missing && !busy,
                            ) { vm.triarCarpeta(p.path) }
                        }
                    }
                }

                // --- navegador ---
                item { TitolSeccio(t("app.carpeta.browse")) }
                item {
                    val d = dirs
                    GCard(Modifier.fillMaxWidth(), padding = PaddingValues(vertical = 4.dp)) {
                        Row(
                            Modifier.fillMaxWidth().padding(start = 16.dp, end = 8.dp, top = 8.dp, bottom = 4.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Column(Modifier.weight(1f)) {
                                Text(
                                    d?.name?.ifBlank { d.path.substringAfterLast('/') } ?: "…",
                                    color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold,
                                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                                )
                                Text(
                                    camiCurt(d?.path.orEmpty(), maxim = 60),
                                    color = Muted, fontSize = 11.5.sp, fontFamily = FontFamily.Monospace,
                                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                                )
                            }
                            if (d != null && d.git) Etiqueta(t("app.carpeta.repo"), Onada)
                            IconButton(onClick = { vm.obrirCarpeta(d?.parent) }, enabled = !(d?.parent.isNullOrBlank()) && !busy) {
                                Icon(Icons.Rounded.ArrowUpward, t("app.carpeta.up"), tint = if (d?.parent.isNullOrBlank()) Line else Sea)
                            }
                            if (!d?.home.isNullOrBlank() && d?.home != d?.path) {
                                IconButton(onClick = { vm.obrirCarpeta(d?.home) }, enabled = !busy) {
                                    Icon(Icons.Rounded.Home, t("app.carpeta.home"), tint = Sea)
                                }
                            }
                        }
                        HorizontalDivider(Modifier.padding(horizontal = 16.dp), color = Line)
                        d?.dirs.orEmpty().forEach { sub ->
                            FilaCarpeta(
                                nom = sub.name,
                                cami = null,
                                detall = if (sub.git) t("app.carpeta.repo") else null,
                                detallColor = Onada,
                                icona = Icons.Rounded.FolderOpen,
                                activa = false,
                                enabled = !busy,
                            ) { vm.obrirCarpeta(sub.path) }
                        }
                        if (!d?.hidden.isNullOrZero()) {
                            Text(
                                t("app.carpeta.hidden", d?.hidden ?: 0),
                                color = Muted, fontSize = 11.5.sp,
                                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                            )
                        }
                        if (d?.truncated == true) {
                            Text(
                                t("app.carpeta.truncated"), color = Muted, fontSize = 11.5.sp,
                                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                            )
                        }
                    }
                }

                // --- triar la carpeta que s'està mirant ---
                item {
                    val triada = dirs?.path.orEmpty()
                    val actual = ws?.current.orEmpty()
                    val potTriar = triada.isNotBlank() && triada != actual && !busy
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Button(
                            onClick = { vm.triarCarpeta(triada) },
                            enabled = potTriar,
                            modifier = Modifier.fillMaxWidth().heightIn(min = 50.dp),
                            shape = RoundedCornerShape(16.dp),
                            colors = ButtonDefaults.buttonColors(containerColor = Sea, contentColor = Color.Black),
                        ) {
                            Text(
                                if (triada == actual) t("app.carpeta.alreadyHere") else t("app.carpeta.use"),
                                fontWeight = FontWeight.SemiBold,
                            )
                        }
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            var manual by remember { mutableStateOf("") }
                            OutlinedTextField(
                                value = manual,
                                onValueChange = { manual = it },
                                singleLine = true,
                                label = { Text(t("app.carpeta.typePath"), fontSize = 12.sp) },
                                textStyle = LocalTextStyle.current.copy(fontFamily = FontFamily.Monospace, fontSize = 13.sp),
                                modifier = Modifier.weight(1f),
                                shape = RoundedCornerShape(14.dp),
                            )
                            TextButton(
                                onClick = { vm.obrirCarpeta(manual.trim()) },
                                enabled = manual.isNotBlank() && !busy,
                            ) { Text(t("app.carpeta.go"), color = Sea) }
                        }
                    }
                }
            }
        }
    }
}

/** Una fila de carpeta: icona, nom, camí o detall, i fletxa si es pot obrir. */
@Composable
private fun FilaCarpeta(
    nom: String,
    cami: String?,
    detall: String?,
    detallColor: Color,
    icona: androidx.compose.ui.graphics.vector.ImageVector,
    activa: Boolean,
    enabled: Boolean,
    onClick: () -> Unit,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .clickable(enabled = enabled, onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Icon(icona, null, tint = if (activa) Sea else Muted, modifier = Modifier.size(20.dp))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(
                nom, color = if (enabled) Ink else Muted, fontSize = 14.5.sp,
                fontWeight = if (activa) FontWeight.SemiBold else FontWeight.Normal,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
            if (!cami.isNullOrBlank()) {
                Text(cami, color = Muted, fontSize = 11.5.sp, fontFamily = FontFamily.Monospace, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        if (!detall.isNullOrBlank()) Etiqueta(detall, detallColor)
        Icon(Icons.Rounded.ChevronRight, null, tint = Line, modifier = Modifier.size(18.dp))
    }
}

/** Pastilla petita per a «En ús», «Ja no hi és», la branca o «repo». */
@Composable
private fun Etiqueta(text: String, color: Color) {
    Text(
        text,
        color = color,
        fontSize = 10.5.sp,
        fontWeight = FontWeight.SemiBold,
        maxLines = 1,
        modifier = Modifier
            .background(color.copy(alpha = 0.14f), RoundedCornerShape(8.dp))
            .padding(horizontal = 8.dp, vertical = 3.dp),
    )
}

@Composable
private fun TitolSeccio(text: String) {
    Text(text.uppercase(), color = Muted, fontSize = 11.5.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 0.6.sp)
}

private fun Int?.isNullOrZero(): Boolean = this == null || this == 0

/**
 * El servidor contesta en anglès quan hi ha una feina en curs (el text surt del
 * nucli de l'agent); aquí es diu què cal fer en lloc de deixar-lo cru.
 */
internal fun missatgeCarpeta(error: String, idioma: String): String {
    val cru = error.removePrefix("HTTP 400: ").trim()
    return when {
        cru.contains("stop the current run", ignoreCase = true) -> tr(idioma, "app.carpeta.stopRun")
        cru.contains("no existeix", ignoreCase = true) -> tr(idioma, "app.carpeta.noExists")
        cru.isBlank() -> tr(idioma, "app.carpeta.generic")
        else -> cru
    }
}
