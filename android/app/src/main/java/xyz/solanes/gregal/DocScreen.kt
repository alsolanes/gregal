package xyz.solanes.gregal

import android.annotation.SuppressLint
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.view.ViewGroup
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.imePadding
import androidx.compose.runtime.DisposableEffect
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Image
import androidx.compose.ui.layout.ContentScale
import androidx.compose.foundation.ScrollState
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.ArrowDropDown
import androidx.compose.material.icons.rounded.Check
import androidx.compose.material.icons.rounded.ChevronRight
import androidx.compose.material.icons.rounded.Description
import androidx.compose.material.icons.rounded.Download
import androidx.compose.material.icons.rounded.Slideshow
import androidx.compose.material.icons.rounded.TableChart
import androidx.compose.material3.IconButton
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.serialization.json.JsonObject

// Gregal Docs a pantalla completa (al mòbil no hi ha lloc per al panell
// lateral de la web). Sense document obert és l'inici (doc-home.js):
// «Document nou», «Els meus documents» i, a Codi amb projecte, «Aquest
// projecte». Un document de text s'obre amb l'editor de la web (DocWebPage);
// una presentació o un full, amb les diapositives o la graella, al dia
// mentre l'agent l'escriu, i a sota les mateixes llistes.

@Composable
fun DocScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val ref by vm.docOpen.collectAsState()
    val doc by vm.doc.collectAsState()
    val mine by vm.myDocs.collectAsState()
    val docs by vm.docs.collectAsState()
    val st by vm.state.collectAsState()
    val conn by vm.conn.collectAsState()
    val vents by vm.simpleVents.collectAsState()
    val project = docsProjectName(st.cwd, st.project, vents)
    // Torna a llegir les llistes en obrir un document: el que s'acaba de crear hi surt.
    LaunchedEffect(project != null, ref?.id) { vm.loadDocs(project != null) }
    LaunchedEffect(ref) { ref?.let { vm.followDoc(it) } }
    val open = doc?.takeIf { it.id == ref?.id }
    // Totes les files de la graella es desplacen alhora de costat.
    val sheetScroll = rememberScrollState()
    // Amb un document obert, enrere torna a l'inici de Docs, com la fletxa de dalt.
    BackHandler(ref != null) { vm.openDocsHome() }
    val kind = open?.kind ?: ref?.kind.orEmpty()
    Scaffold(containerColor = Color.Transparent, topBar = {
        if (ref == null) GregalTopBar(tr(lang, "docs.home"), lang, onMenu = onMenu)
        else GregalTopBar(open?.title ?: ref?.title.orEmpty(), lang, onBack = { vm.openDocsHome() }, subtitle = kindLabel(lang, kind)) {
            if (open != null) IconButton(onClick = {
                vm.downloadServerDoc(docExportPath(DocRef(open.id, open.title, open.kind, open.scope)), open.title + "." + docExtension(open.kind))
            }, modifier = Modifier.testTag("doc-download")) {
                Icon(Icons.Rounded.Download, tr(lang, downloadKey(open.kind)), tint = Ink)
            }
        }
    }) { pad ->
        if (open != null && open.inWebEditor) {
            Column(Modifier.fillMaxSize().padding(pad).imePadding().testTag("doc-screen")) {
                DocWebPage(open, conn.api(), { vm.conn.value.token }) { vm.idioma.value }
            }
            return@Scaffold
        }
        LazyColumn(Modifier.fillMaxSize().padding(pad).testTag("doc-screen"),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 110.dp)) {
            when {
                open != null && open.kind == "sheet" -> sheetView(vm, lang, open, sheetScroll)
                open != null && open.kind == "deck" -> deckView(vm, lang, open)
                ref != null -> item(key = "loading") { Text(tr(lang, "docs.loading"), color = Muted, modifier = Modifier.padding(vertical = 12.dp)) }
                else -> {
                    item(key = "new") { NewDocRow(vm, lang) }
                    docSection(vm, lang, "m", tr(lang, "docs.mine"), mine, ref)
                    if (project != null) docSection(vm, lang, "p", tr(lang, "docs.project", project), docs, ref)
                }
            }
        }
    }
}

private fun docExtension(kind: String) = when (kind) { "sheet" -> "xlsx"; "deck" -> "pptx"; else -> "docx" }
private fun downloadKey(kind: String) = when (kind) { "sheet" -> "docs.downloadXlsx"; "deck" -> "docs.downloadPptx"; else -> "docs.download" }
private fun kindLabel(lang: String, kind: String) = when (kind) {
    "deck" -> tr(lang, "docs.kind.deck"); "sheet" -> tr(lang, "docs.kind.sheet"); else -> tr(lang, "docs.kind.doc")
}

/** La icona de cada tipus, la mateixa a l'inici, a la llista i a la targeta del xat. */
private fun kindIcon(kind: String): ImageVector = when (kind) {
    "sheet" -> Icons.Rounded.TableChart; "deck" -> Icons.Rounded.Slideshow; else -> Icons.Rounded.Description
}

/** El to de cada tipus, viu en tots dos temes (blau, verd i taronja, com els gràfics). */
@Composable
private fun kindTint(kind: String): Color {
    val dark = LocalPal.current.dark
    return when (kind) {
        "sheet" -> if (dark) Color(0xFF7FD6C2) else Color(0xFF0E7C66)
        "deck" -> if (dark) Color(0xFFF2B48E) else Color(0xFFC4622D)
        else -> if (dark) Color(0xFF8A9BF0) else Color(0xFF2F6FD6)
    }
}

/** La icona del tipus dins d'un quadrat arrodonit del seu to. */
@Composable
internal fun KindBadge(kind: String, size: Dp = 40.dp) {
    val tint = kindTint(kind)
    Box(Modifier.size(size).background(tint.copy(alpha = 0.16f), RoundedCornerShape(size * 0.3f)), contentAlignment = Alignment.Center) {
        Icon(kindIcon(kind), null, tint = tint, modifier = Modifier.size(size * 0.55f))
    }
}

/** «Crea»: el títol (opcional; sense, «Document sense títol», com a doc-home.js) i una targeta per tipus. */
@Composable
private fun NewDocRow(vm: GregalViewModel, lang: String) {
    var title by remember { mutableStateOf("") }
    Text(tr(lang, "docs.create").uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 0.8.sp,
        modifier = Modifier.padding(top = 8.dp, bottom = 8.dp))
    OutlinedTextField(title, { title = it }, Modifier.fillMaxWidth().testTag("doc-new-title"), singleLine = true,
        shape = RoundedCornerShape(14.dp), placeholder = { Text(tr(lang, "docs.newTitle")) })
    Row(Modifier.fillMaxWidth().padding(top = 10.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        for ((kind, key, tag) in listOf(Triple("doc", "docs.new", "doc-new"), Triple("sheet", "docs.newSheet", "doc-new-sheet"),
            Triple("deck", "docs.newDeck", "doc-new-deck"))) {
            Surface(onClick = { vm.newDoc(title, kind) }, shape = RoundedCornerShape(16.dp), color = SuperficieOpaca,
                border = BorderStroke(1.dp, Line), modifier = Modifier.weight(1f).semantics { contentDescription = tr(lang, key) }.testTag(tag)) {
                Column(Modifier.padding(horizontal = 12.dp, vertical = 14.dp)) {
                    KindBadge(kind, 36.dp)
                    Text(kindLabel(lang, kind), color = Ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold, maxLines = 1,
                        overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = 12.dp))
                    Row(Modifier.padding(top = 2.dp), verticalAlignment = Alignment.CenterVertically) {
                        Icon(Icons.Rounded.Add, null, tint = kindTint(kind), modifier = Modifier.size(14.dp))
                        Text(tr(lang, "docs.createShort"), color = Muted, fontSize = 12.sp, modifier = Modifier.padding(start = 2.dp))
                    }
                }
            }
        }
    }
}

/** Una presentació: una diapositiva 16:9 per bloc (la que s'edita, amb el seu formulari) i «Diapositiva nova». */
private fun LazyListScope.deckView(vm: GregalViewModel, lang: String, d: LiveDoc) {
    val e = d.edit
    if (d.blocks.isEmpty()) item(key = "deck-empty") { Text(tr(lang, "docs.deckEmpty"), color = Muted, modifier = Modifier.padding(vertical = 12.dp)) }
    itemsIndexed(d.blocks, key = { _, b -> "s:" + b.id }) { i, b ->
        Column(Modifier.padding(vertical = 8.dp)) {
            // El número de la diapositiva i el seu layout, a sobre, com al panell de la web.
            Row(Modifier.padding(bottom = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.size(22.dp).background(kindTint("deck").copy(alpha = 0.18f), CircleShape), contentAlignment = Alignment.Center) {
                    Text("${i + 1}", color = kindTint("deck"), fontSize = 11.sp, fontWeight = FontWeight.Bold)
                }
                val layout = slideText(b.slide, "layout")
                Text(tr(lang, "docs.layout.$layout").takeIf { layout in slideLayouts } ?: layout, color = Muted, fontSize = 12.sp,
                    modifier = Modifier.padding(start = 8.dp))
            }
            if (e?.id == b.id) SlideForm(vm, e, lang)
            else Box(Modifier.clip(RoundedCornerShape(12.dp)).clickable { vm.editDocBlock(b.id) }) { SlideCard(b.slide) }
        }
    }
    item(key = "deck-add") { NewSlideRow(vm, lang) }
}

/** El layout de la diapositiva nova, d'una llista, i el botó. */
@Composable
private fun NewSlideRow(vm: GregalViewModel, lang: String) {
    var layout by remember { mutableStateOf("content") }
    var menu by remember { mutableStateOf(false) }
    Surface(shape = RoundedCornerShape(14.dp), color = Color.Transparent, border = BorderStroke(1.dp, LineHi),
        modifier = Modifier.fillMaxWidth().padding(top = 10.dp)) {
        Row(Modifier.padding(10.dp), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.weight(1f)) {
                Surface(onClick = { menu = true }, shape = RoundedCornerShape(10.dp), color = SuperficieAlta,
                    modifier = Modifier.semantics { contentDescription = tr(lang, "docs.slideLayout") }) {
                    Row(Modifier.padding(start = 12.dp, end = 6.dp, top = 8.dp, bottom = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text(tr(lang, "docs.layout.$layout"), color = Ink, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        Icon(Icons.Rounded.ArrowDropDown, null, tint = Muted)
                    }
                }
                DropdownMenu(menu, { menu = false }) {
                    for (k in slideLayouts) DropdownMenuItem(text = { Text(tr(lang, "docs.layout.$k")) }, onClick = { layout = k; menu = false })
                }
            }
            Button(onClick = { vm.addSlide(layout) }, modifier = Modifier.padding(start = 8.dp).testTag("deck-add")) {
                Icon(Icons.Rounded.Add, null, modifier = Modifier.size(18.dp))
                Text(tr(lang, "docs.newSlide"), modifier = Modifier.padding(start = 4.dp))
            }
        }
    }
}

// Els colors del tema «default» del pptx (com docs-panel.js): una diapositiva és blanca en tots dos temes, com a PowerPoint.
private val SlideInk = Color(0xFF222222)
private val SlideHead = Color(0xFF1F2A44)
private val SlideSub = Color(0xFF1F4E79)
private val SlideAccent = Color(0xFF2E86AB)

/** Una diapositiva dibuixada a partir dels seus camps: prou per reconèixer-ne el layout; la referència és el pptx. */
@Composable
private fun SlideCard(s: JsonObject?) {
    val layout = slideText(s, "layout")
    val cover = layout in setOf("title", "section", "closing")
    val bg = when (layout) { "section" -> SlideHead; "title", "closing" -> SlideSub; else -> Color.White }
    val ink = if (cover) Color(0xFFEAF1F8) else SlideInk
    val title = slideText(s, "title")
    Column(Modifier.fillMaxWidth().aspectRatio(16f / 9f).shadow(6.dp, RoundedCornerShape(12.dp)).background(bg, RoundedCornerShape(12.dp))
        .clip(RoundedCornerShape(12.dp)).padding(horizontal = 22.dp, vertical = 16.dp).testTag("slide"),
        verticalArrangement = if (cover) Arrangement.Center else Arrangement.Top) {
        if (cover) {
            Text(title, color = Color.White, fontSize = 22.sp, fontWeight = FontWeight.SemiBold, lineHeight = 26.sp)
            Text(slideText(s, "subtitle").ifEmpty { slideText(s, "body") }, color = ink, fontSize = 13.sp)
            return@Column
        }
        if (title.isNotEmpty()) {
            Text(title, color = SlideHead, fontSize = 17.sp, fontWeight = FontWeight.SemiBold, maxLines = 2, overflow = TextOverflow.Ellipsis)
            Box(Modifier.padding(top = 4.dp, bottom = 8.dp).fillMaxWidth().height(2.dp).background(SlideAccent))
        }
        when (layout) {
            "two_columns", "comparison" -> Row {
                for ((h, x) in listOf("left_title" to "left", "right_title" to "right")) Column(Modifier.weight(1f).padding(end = 10.dp)) {
                    if (slideText(s, h).isNotEmpty()) Text(slideText(s, h), color = SlideSub, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
                    Text(slideText(s, x), color = ink, fontSize = 11.sp)
                }
            }
            "big_number" -> {
                Text(slideText(s, "number"), color = SlideAccent, fontSize = 40.sp, fontWeight = FontWeight.Bold)
                Text(slideText(s, "caption"), color = ink, fontSize = 12.sp)
            }
            // Una adreça web es baixa com les imatges del xat; una ruta del projecte no es pot carregar aquí: se'n diu el nom.
            "image" -> Row {
                Column(Modifier.weight(1f).padding(end = 10.dp)) { SlideBody(s) }
                val bmp = slideImageUrl(s)?.let { rememberChatImage(it).first }
                if (bmp != null) Image(bmp, slideText(s, "title"), Modifier.weight(1f), contentScale = ContentScale.Fit)
                else Text(slideText(s, "image"), color = Muted, fontSize = 11.sp, modifier = Modifier.weight(1f))
            }
            else -> SlideBody(s)
        }
        (s?.get("chart") as? JsonObject)?.let { c ->
            Text(slideText(c, "title").ifEmpty { slideText(c, "type") }, color = Color(0xFF6B7280), fontSize = 11.sp, fontStyle = FontStyle.Italic)
        }
    }
}

/** El text i els punts (planers: el sagnat només és el nivell al pptx). */
@Composable
private fun SlideBody(s: JsonObject?) {
    val body = slideText(s, "body")
    if (body.isNotEmpty()) Text(body, color = SlideInk, fontSize = 12.sp)
    for (b in slideBullets(s)) Text("• " + b.trim(), color = SlideInk, fontSize = 12.sp)
}

/** El formulari del layout; després d'un 409, l'avís i la diapositiva d'ara a sobre, amb els camps de la persona. */
@Composable
private fun SlideForm(vm: GregalViewModel, e: DocEdit, lang: String) {
    Column(Modifier.fillMaxWidth().padding(vertical = 6.dp).testTag("slide-form")) {
        e.conflict?.let { b ->
            Text(tr(lang, "docs.slideConflict"), color = Sea, fontWeight = FontWeight.SemiBold, fontSize = 13.sp)
            Box(Modifier.padding(vertical = 6.dp)) { SlideCard(b.slide) }
        }
        val layout = e.layout.ifEmpty { slideText(e.slide, "layout") }
        var menu by remember { mutableStateOf(false) }
        Box {
            OutlinedButton(onClick = { menu = true }, enabled = !e.saving,
                modifier = Modifier.semantics { contentDescription = tr(lang, "docs.layout") }.testTag("slide-layout")) {
                Text(tr(lang, "docs.layout.$layout").takeIf { layout in slideLayouts } ?: layout)
            }
            DropdownMenu(menu, { menu = false }) {
                for (k in slideLayouts) DropdownMenuItem(text = { Text(tr(lang, "docs.layout.$k")) }, onClick = { vm.chooseSlideLayout(k); menu = false })
            }
        }
        for ((k, v) in e.fields) {
            val long = k in setOf("body", "bullets", "left", "right", "notes")
            OutlinedTextField(v, { vm.typeSlideField(k, it) }, Modifier.fillMaxWidth().padding(top = 4.dp), enabled = !e.saving,
                singleLine = !long, minLines = if (long) 2 else 1, label = { Text(tr(lang, "docs.field.$k")) })
        }
        Row(Modifier.padding(top = 6.dp)) {
            Button(onClick = { vm.saveDocBlock(false) }, enabled = !e.saving) { Text(tr(lang, "docs.save")) }
            OutlinedButton(onClick = { vm.cancelDocEdit() }, enabled = !e.saving, modifier = Modifier.padding(start = 8.dp)) { Text(tr(lang, "docs.cancel")) }
        }
        OutlinedButton(onClick = { vm.deleteSlide() }, enabled = !e.saving, modifier = Modifier.padding(top = 6.dp)) { Text(tr(lang, "docs.deleteSlide")) }
    }
}

private val CellWidth = 96.dp
private val CellHeight = 36.dp
private val RowHeadWidth = 40.dp

/** Un full de càlcul: les pestanyes, l'editor de la cel·la triada i la graella amb els valors calculats. */
private fun LazyListScope.sheetView(vm: GregalViewModel, lang: String, d: LiveDoc, hs: ScrollState) {
    val s = d.shownSheet()
    val e = d.edit?.takeIf { it.cell.isNotEmpty() }
    item(key = "sheet-tabs") {
        val tint = kindTint("sheet")
        Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).padding(vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
            for (b in d.blocks) {
                val on = b.id == s?.id
                Surface(onClick = { vm.showSheet(b.id) }, shape = RoundedCornerShape(50), color = if (on) tint.copy(alpha = 0.18f) else Color.Transparent,
                    modifier = Modifier.padding(end = 6.dp).testTag("sheet-tab")) {
                    Text(b.name, color = if (on) tint else Muted, fontSize = 13.sp, fontWeight = if (on) FontWeight.SemiBold else FontWeight.Normal,
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 7.dp))
                }
            }
            Surface(onClick = { vm.addSheet() }, shape = RoundedCornerShape(50), color = Color.Transparent, border = BorderStroke(1.dp, Line),
                modifier = Modifier.testTag("sheet-add")) {
                Row(Modifier.padding(start = 10.dp, end = 14.dp, top = 6.dp, bottom = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                    Icon(Icons.Rounded.Add, null, tint = Muted, modifier = Modifier.size(16.dp))
                    Text(tr(lang, "docs.addSheet"), color = Muted, fontSize = 13.sp, modifier = Modifier.padding(start = 4.dp))
                }
            }
        }
    }
    if (s == null) {
        item(key = "sheet-empty") { Text(tr(lang, "docs.sheetEmpty"), color = Muted, modifier = Modifier.padding(vertical = 12.dp)) }
        return
    }
    if (e != null) item(key = "cell-editor") { CellEditor(vm, d, e, lang) }
    val (cols, rows) = sheetSize(s)
    // La columna i la fila de la cel·la triada es ressalten a les capçaleres.
    val sel = e?.takeIf { it.id == s.id }?.cell.orEmpty()
    val selCol = sel.takeWhile { it.isLetter() }
    val selRow = sel.dropWhile { it.isLetter() }
    item(key = "sheet-head") {
        Row {
            SheetHead("", RowHeadWidth, false)
            Row(Modifier.horizontalScroll(hs)) { repeat(cols) { SheetHead(sheetColumn(it), CellWidth, sheetColumn(it) == selCol) } }
        }
    }
    items((1..rows).toList(), key = { "r:$it" }) { r ->
        val tint = kindTint("sheet")
        Row {
            SheetHead("$r", RowHeadWidth, "$r" == selRow)
            Row(Modifier.horizontalScroll(hs)) {
                repeat(cols) { c ->
                    val name = sheetColumn(c) + r
                    val on = sel == name
                    val shown = s.values[name] ?: s.cells[name].orEmpty()
                    // Els números a la dreta, com a qualsevol full de càlcul.
                    Text(shown, color = Ink, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis,
                        textAlign = if (shown.toDoubleOrNull() != null) TextAlign.End else TextAlign.Start,
                        modifier = Modifier.size(CellWidth, CellHeight).background(if (on) tint.copy(alpha = 0.10f) else Color.Transparent)
                            .border(if (on) 2.dp else 0.5.dp, if (on) tint else Line)
                            .clickable { vm.editCell(s.id, name) }.padding(horizontal = 8.dp, vertical = 9.dp).testTag("cell-$name"))
                }
            }
        }
    }
}

/** La lletra d'una columna o el número d'una fila. */
@Composable
private fun SheetHead(text: String, width: Dp, on: Boolean) {
    val tint = kindTint("sheet")
    Box(Modifier.size(width, CellHeight).background(if (on) tint.copy(alpha = 0.22f) else SuperficieAlta).border(0.5.dp, Line),
        contentAlignment = Alignment.Center) {
        Text(text, color = if (on) tint else Muted, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
    }
}

/** L'entrada de la cel·la (la fórmula, no el valor); després d'un 409, l'avís i el que hi ha ara. */
@Composable
private fun CellEditor(vm: GregalViewModel, d: LiveDoc, e: DocEdit, lang: String) {
    val sheet = d.blocks.firstOrNull { it.id == e.id }?.name.orEmpty()
    Column(Modifier.fillMaxWidth().padding(vertical = 6.dp).testTag("doc-cell-editor")) {
        e.conflict?.let { b ->
            Text(tr(lang, "docs.sheetConflict"), color = Sea, fontWeight = FontWeight.SemiBold, fontSize = 13.sp)
            b.cells[e.cell]?.let { Text(e.cell + " = " + it, color = Ink, fontSize = 13.sp) }
        }
        val focus = remember { FocusRequester() }
        LaunchedEffect(e.id, e.cell) { focus.requestFocus() }
        val tint = kindTint("sheet")
        // La barra de fórmules: la cel·la, «fx», l'entrada (la fórmula, no el valor) i desar.
        Surface(shape = RoundedCornerShape(14.dp), color = SuperficieOpaca, border = BorderStroke(1.dp, LineHi)) {
            Row(Modifier.padding(start = 8.dp, end = 2.dp), verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.background(tint.copy(alpha = 0.16f), RoundedCornerShape(8.dp)).padding(horizontal = 8.dp, vertical = 5.dp)
                    .semantics { contentDescription = sheet + "!" + e.cell }) {
                    Text(e.cell, color = tint, fontSize = 13.sp, fontWeight = FontWeight.Bold, fontFamily = FontFamily.Monospace)
                }
                Text("fx", color = Muted, fontSize = 15.sp, fontStyle = FontStyle.Italic, fontFamily = FontFamily.Serif,
                    modifier = Modifier.padding(start = 10.dp))
                TextField(e.text, vm::typeDocBlock, Modifier.weight(1f).focusRequester(focus), enabled = !e.saving, singleLine = true,
                    textStyle = TextStyle(fontFamily = FontFamily.Monospace, fontSize = 15.sp, color = Ink),
                    colors = TextFieldDefaults.colors(focusedContainerColor = Color.Transparent, unfocusedContainerColor = Color.Transparent,
                        disabledContainerColor = Color.Transparent, focusedIndicatorColor = Color.Transparent,
                        unfocusedIndicatorColor = Color.Transparent, disabledIndicatorColor = Color.Transparent))
                IconButton(onClick = { vm.saveDocBlock(false) }, enabled = !e.saving, modifier = Modifier.testTag("cell-save")) {
                    Icon(Icons.Rounded.Check, tr(lang, "docs.save"), tint = tint)
                }
            }
        }
    }
}

private fun LazyListScope.docSection(vm: GregalViewModel, lang: String, key: String, title: String, items: List<DocRef>, open: DocRef?) {
    item(key = "h:$key") {
        Text(title.uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 0.8.sp,
            modifier = Modifier.padding(top = 24.dp, bottom = 8.dp))
        if (items.isEmpty()) Text(tr(lang, "docs.none"), color = Muted, fontSize = 14.sp)
    }
    items(items, key = { "$key:" + it.id }) { d ->
        val on = d.id == open?.id && d.scope == open.scope
        Surface(onClick = { vm.openDoc(d) }, shape = RoundedCornerShape(14.dp), color = SuperficieOpaca,
            border = BorderStroke(1.dp, if (on) Sea else Line), modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp)) {
            Row(Modifier.padding(horizontal = 12.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
                KindBadge(d.kind)
                Column(Modifier.weight(1f).padding(start = 12.dp)) {
                    Text(d.title, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.Medium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Text(kindLabel(lang, d.kind), color = Muted, fontSize = 12.sp)
                }
                Icon(Icons.Rounded.ChevronRight, null, tint = Muted)
            }
        }
    }
}

/**
 * Un document de text: la pàgina del document de la web (doc-page.js) en un WebView, amb el mateix editor,
 * la mateixa barra i la mateixa pàgina. La clau va a la capçalera de la càrrega i al pont GregalAndroid,
 * mai a la URL; per això la pàgina no navega enlloc més i els enllaços s'obren fora.
 */
@Composable
private fun ColumnScope.DocWebPage(d: LiveDoc, api: GregalApi, key: () -> String, lang: () -> String) {
    val page = docPageUrl(api.base, DocRef(d.id, d.title, d.kind, d.scope), sessioApp)
    val ctx = LocalContext.current
    var web by remember { mutableStateOf<WebView?>(null) }
    DisposableEffect(Unit) { onDispose { web?.stopLoading(); web?.destroy() } }
    AndroidView(modifier = Modifier.fillMaxWidth().weight(1f), factory = { c ->
        docWebView(c, page, key, lang) { url -> runCatching { ContextCompat.startActivity(ctx, Intent(Intent.ACTION_VIEW, Uri.parse(url)), null) } }
            .also { web = it }
    }, update = { view ->
        if (view.tag != page) {
            view.tag = page
            view.loadUrl(page, docPageHeaders(api.token))
        }
    })
}

/** El WebView de l'editor de text. MATCH_PARENT: AndroidView li dona WRAP_CONTENT si no en
 *  porta, i aleshores el WebView mesura la pàgina pel contingut: amb html,body{height:100%}
 *  la pàgina feia 0 px d'alt i el document es veia en negre. */
@SuppressLint("SetJavaScriptEnabled", "JavascriptInterface")
internal fun docWebView(c: Context, page: String, key: () -> String, lang: () -> String, openOutside: (String) -> Unit): WebView =
    WebView(c).apply {
        layoutParams = ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT)
        settings.javaScriptEnabled = true
        // doc-page.js hi llegeix el tema de l'app.
        settings.domStorageEnabled = true
        settings.allowFileAccess = false
        settings.allowContentAccess = false
        webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView?, request: WebResourceRequest?): Boolean {
                val url = request?.url?.toString().orEmpty()
                if (docPageKeeps(url, page)) return false
                openOutside(url)
                return true
            }
        }
        addJavascriptInterface(DocPageBridge(key, lang), "GregalAndroid")
    }

/** La targeta del xat quan l'agent escriu un document: l'obre a DocScreen. */
@Composable
fun DocCard(d: DocRef, lang: String, onOpen: () -> Unit) {
    Surface(onClick = onOpen, shape = RoundedCornerShape(14.dp), color = SuperficieOpaca, border = BorderStroke(1.dp, Line),
        modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp).testTag("doc-card")) {
        Row(Modifier.padding(horizontal = 14.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            KindBadge(d.kind)
            Column(Modifier.padding(start = 12.dp)) {
                Text(d.title, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
                Text(tr(lang, "docs.card"), color = Muted, fontSize = 12.sp)
            }
        }
    }
}
