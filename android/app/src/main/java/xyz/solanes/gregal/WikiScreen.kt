package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.ClickableText
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.AutoStories
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties

/**
 * Wiki — les pàgines de resum de l'espai vents: el que s'ha parlat, en una
 * pàgina que es pot tornar a obrir des del mòbil, amb gràfics i imatges.
 * Les pàgines les escriu el backend (resum d'una conversa) o l'usuari;
 * viuen com a markdown a wind-wiki/ i es llegeixen amb el mateix renderer
 * que el xat (Md.kt): taules, ```chart i imatges incloses.
 */

/** La fila «Wiki» de la llista de vents: discreta, una sola, com Programats. */
@Composable
internal fun WikiButton(lang: String, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        shape = MaterialTheme.shapes.medium,
        color = SuperficieAlta.copy(alpha = .30f),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 6.dp),
    ) {
        Row(Modifier.padding(horizontal = 14.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Rounded.AutoStories, null, tint = Sea, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(10.dp))
            Text(tr(lang, "app.wiki.open"), color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp, modifier = Modifier.weight(1f))
            Text("›", color = Muted, fontSize = 18.sp)
        }
    }
}

@Composable
internal fun WikiDialog(vm: GregalViewModel, lang: String, onDismiss: () -> Unit) {
    val pages by vm.wikiPages.collectAsState()
    val busy by vm.wikiBusy.collectAsState()
    var open by remember { mutableStateOf<GregalApi.WikiPage?>(null) }
    var confirmDelete by remember { mutableStateOf<GregalApi.WikiPage?>(null) }
    LaunchedEffect(Unit) { vm.loadWiki() }
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(shape = MaterialTheme.shapes.large, color = MaterialTheme.colorScheme.background) {
            Column(Modifier.fillMaxWidth().fillMaxHeight(0.85f).padding(top = 16.dp)) {
                Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(tr(lang, "app.wiki.title"), color = Ink, fontWeight = FontWeight.Bold, fontSize = 18.sp, modifier = Modifier.weight(1f))
                    if (busy) CircularProgressIndicator(Modifier.size(16.dp), color = Sea, strokeWidth = 2.dp)
                    IconButton(onClick = onDismiss) { Text("✕", color = Muted, fontSize = 16.sp) }
                }
                when {
                    pages == null -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        CircularProgressIndicator(color = Sea)
                    }
                    pages!!.isEmpty() -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp), modifier = Modifier.padding(32.dp)) {
                            Text("📚", fontSize = 40.sp)
                            Text(tr(lang, "app.wiki.empty"), color = Ink, fontWeight = FontWeight.SemiBold)
                            Text(tr(lang, "app.wiki.emptyHelp"), color = Muted, fontSize = 13.sp)
                        }
                    }
                    else -> LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(start = 20.dp, end = 20.dp, top = 8.dp, bottom = 24.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        items(pages!!, key = { it.nom }) { p ->
                            Surface(
                                onClick = { open = p },
                                shape = MaterialTheme.shapes.medium,
                                color = SuperficieAlta.copy(alpha = .30f),
                                modifier = Modifier.fillMaxWidth(),
                            ) {
                                Column(Modifier.padding(horizontal = 14.dp, vertical = 12.dp)) {
                                    Text(p.titol.ifBlank { p.nom }, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
                                    if (p.descripcio.isNotBlank()) Text(p.descripcio, color = Muted, fontSize = 13.sp, maxLines = 2)
                                    if (p.updated.isNotBlank()) {
                                        Text(tr(lang, "app.wiki.updated") + " · " + programatHora(p.updated), color = Muted.copy(alpha = .8f), fontSize = 11.sp)
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
    // Lector d'una pàgina: markdown complet amb el renderer del xat.
    open?.let { p ->
        var full by remember(p.nom) { mutableStateOf<GregalApi.WikiPage?>(null) }
        LaunchedEffect(p.nom) {
            // el cos no ve a la llista: el carrega sol
            val source = vm.lecturaDePagina(p.nom)
            full = source
        }
        Dialog(onDismissRequest = { open = null }, properties = DialogProperties(usePlatformDefaultWidth = false)) {
            Surface(shape = MaterialTheme.shapes.large, color = MaterialTheme.colorScheme.background) {
                Column(Modifier.fillMaxWidth().fillMaxHeight(0.85f).padding(top = 16.dp)) {
                    Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text((full?.titol ?: p.titol).ifBlank { p.nom }, color = Ink, fontWeight = FontWeight.Bold, fontSize = 18.sp, modifier = Modifier.weight(1f))
                        IconButton(onClick = { confirmDelete = p }) { Text("🗑", color = Muted, fontSize = 16.sp) }
                        IconButton(onClick = { open = null }) { Text("✕", color = Muted, fontSize = 16.sp) }
                    }
                    val shown = full?.cos
                    if (shown == null) {
                        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { CircularProgressIndicator(color = Sea) }
                    } else {
                        Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = 20.dp)) {
                            MdPageBody(shown, lang)
                            Spacer(Modifier.height(24.dp))
                        }
                    }
                }
            }
        }
    }
    confirmDelete?.let { p ->
        AlertDialog(
            onDismissRequest = { confirmDelete = null },
            title = { Text(tr(lang, "app.wiki.delete")) },
            text = { Text(p.titol.ifBlank { p.nom }) },
            confirmButton = {
                TextButton(onClick = { vm.deleteWikiPage(p.nom); confirmDelete = null; open = null }) {
                    Text(tr(lang, "app.wiki.delete"), color = MaterialTheme.colorScheme.error)
                }
            },
            dismissButton = { TextButton(onClick = { confirmDelete = null }) { Text("✕", color = Muted) } },
        )
    }
}

/** El cos d'una pàgina amb el mateix renderer del xat: enllaços, taules, ```chart, imatges. */
@Composable
internal fun MdPageBody(src: String, lang: String) {
    val uri = LocalUriHandler.current
    val dark = LocalPal.current == DarkPal
    val accent = Sea
    val link = remember(accent, dark) { readableLink(accent, dark) }
    val codeBg = CodeDark
    val blocks = remember(src) { richBlocks(src) }
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        for (b in blocks) {
            when (b) {
                is Rich.Inline -> {
                    val rich = remember(b, link, codeBg) { renderSegs(b.segs, codeBg, link = link) }
                    ClickableText(
                        rich,
                        style = MaterialTheme.typography.bodyMedium.copy(fontSize = 15.sp, lineHeight = 22.sp, color = Tinta),
                        onClick = { off ->
                            rich.getStringAnnotations("URL", off, off).firstOrNull()?.let { uri.openUri(it.item) }
                        },
                    )
                }
                is Rich.Code -> Row(Modifier.horizontalScroll(rememberScrollState())) {
                    Text(
                        b.code,
                        fontSize = 12.sp,
                        fontFamily = FontFamily.Monospace,
                        softWrap = false,
                        color = Ink,
                        modifier = Modifier
                            .background(CodeDark, RoundedCornerShape(8.dp))
                            .padding(8.dp),
                    )
                }
                is Rich.Table -> MdTable(b.headers, b.rows, lang)
                is Rich.Chart -> ChartBlock(b.spec, accent, dark)
                is Rich.Image -> MdImage(b.url, b.alt, link, lang)
            }
        }
    }
}
