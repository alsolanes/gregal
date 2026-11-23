package xyz.solanes.gregal

import androidx.compose.foundation.BorderStroke
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
import androidx.compose.material.icons.rounded.Description
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
// projecte». Amb un document obert, els blocs amb el markdown del xat (o
// les diapositives d'una presentació, o la graella d'un full), al dia
// mentre l'agent l'escriu, i a sota les mateixes llistes.

@Composable
fun DocScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val ref by vm.docOpen.collectAsState()
    val doc by vm.doc.collectAsState()
    val mine by vm.myDocs.collectAsState()
    val docs by vm.docs.collectAsState()
    val st by vm.state.collectAsState()
    val vents by vm.simpleVents.collectAsState()
    val project = docsProjectName(st.cwd, st.project, vents)
    // Torna a llegir les llistes en obrir un document: el que s'acaba de crear hi surt.
    LaunchedEffect(project != null, ref?.id) { vm.loadDocs(project != null) }
    LaunchedEffect(ref) { ref?.let { vm.followDoc(it) } }
    val open = doc?.takeIf { it.id == ref?.id }
    // Totes les files de la graella es desplacen alhora de costat.
    val sheetScroll = rememberScrollState()
    Scaffold(containerColor = Color.Transparent, topBar = {
        GregalTopBar(open?.title ?: tr(lang, "docs.home"), lang, onMenu = onMenu)
    }) { pad ->
        LazyColumn(Modifier.fillMaxSize().padding(pad).testTag("doc-screen"),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 110.dp)) {
            if (open != null && open.kind == "sheet") {
                item(key = "download") {
                    OutlinedButton(onClick = { vm.downloadServerDoc(docExportPath(DocRef(open.id, open.title, open.kind, open.scope)), open.title + ".xlsx") }) {
                        Text(tr(lang, "docs.downloadXlsx"))
                    }
                }
                sheetView(vm, lang, open, sheetScroll)
            } else if (open != null && open.kind == "deck") {
                item(key = "download") {
                    OutlinedButton(onClick = { vm.downloadServerDoc(docExportPath(DocRef(open.id, open.title, open.kind, open.scope)), open.title + ".pptx") }) {
                        Text(tr(lang, "docs.downloadPptx"))
                    }
                }
                deckView(vm, lang, open)
            } else if (open != null) {
                item(key = "download") {
                    OutlinedButton(onClick = { vm.downloadServerDoc(docExportPath(DocRef(open.id, open.title, scope = open.scope)), open.title + ".docx") }) {
                        Text(tr(lang, "docs.download"))
                    }
                }
                if (open.blank) item(key = "empty") {
                    Text(tr(lang, "docs.write"), color = Muted, fontSize = 15.sp,
                        modifier = Modifier.fillMaxWidth().clickable { vm.writeEmptyDoc() }.padding(vertical = 12.dp).testTag("doc-empty"))
                }
                val e = open.edit
                // Un bloc nou al principi: el d'un document buit, o un que penjava del primer, esborrat.
                if (e != null && e.id.isEmpty() && e.after.isEmpty()) item(key = "new-top") { BlockEditor(vm, e, lang) }
                items(open.blocks, key = { "b:" + it.id }) { b ->
                    if (e?.id == b.id) BlockEditor(vm, e, lang)
                    else Column(Modifier.fillMaxWidth().clickable { vm.editDocBlock(b.id) }) { AssistantResponse(b.md, lang) }
                    if (e != null && e.id.isEmpty() && e.after == b.id) BlockEditor(vm, e, lang)
                }
            } else if (ref != null) {
                item(key = "loading") { Text(tr(lang, "docs.loading"), color = Muted, modifier = Modifier.padding(vertical = 12.dp)) }
            } else {
                item(key = "new") { NewDocRow(vm, lang) }
            }
            docSection(vm, lang, "m", tr(lang, "docs.mine"), mine, ref)
            if (project != null) docSection(vm, lang, "p", tr(lang, "docs.project", project), docs, ref)
        }
    }
}

/** «Document nou»: el títol (o «Document sense títol») i el botó, com a doc-home.js. */
@Composable
private fun NewDocRow(vm: GregalViewModel, lang: String) {
    var title by remember { mutableStateOf("") }
    Row(Modifier.fillMaxWidth().padding(vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
        OutlinedTextField(title, { title = it }, Modifier.weight(1f).testTag("doc-new-title"), singleLine = true,
            placeholder = { Text(tr(lang, "docs.newTitle")) })
        Button(onClick = { vm.newDoc(title) }, modifier = Modifier.padding(start = 8.dp).testTag("doc-new")) { Text(tr(lang, "docs.new")) }
    }
    Row {
        OutlinedButton(onClick = { vm.newDoc(title, "sheet") }, modifier = Modifier.testTag("doc-new-sheet")) { Text(tr(lang, "docs.newSheet")) }
        OutlinedButton(onClick = { vm.newDoc(title, "deck") }, modifier = Modifier.padding(start = 8.dp).testTag("doc-new-deck")) { Text(tr(lang, "docs.newDeck")) }
    }
}

/** Una presentació: una diapositiva 16:9 per bloc (la que s'edita, amb el seu formulari) i «Diapositiva nova». */
private fun LazyListScope.deckView(vm: GregalViewModel, lang: String, d: LiveDoc) {
    val e = d.edit
    if (d.blocks.isEmpty()) item(key = "deck-empty") { Text(tr(lang, "docs.deckEmpty"), color = Muted, modifier = Modifier.padding(vertical = 12.dp)) }
    items(d.blocks, key = { "s:" + it.id }) { b ->
        if (e?.id == b.id) SlideForm(vm, e, lang)
        else Box(Modifier.padding(vertical = 6.dp).clickable { vm.editDocBlock(b.id) }) { SlideCard(b.slide) }
    }
    item(key = "deck-add") { NewSlideRow(vm, lang) }
}

/** El layout de la diapositiva nova, d'una llista, i el botó. */
@Composable
private fun NewSlideRow(vm: GregalViewModel, lang: String) {
    var layout by remember { mutableStateOf("content") }
    var menu by remember { mutableStateOf(false) }
    Row(Modifier.padding(vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
        Box {
            OutlinedButton(onClick = { menu = true }, modifier = Modifier.semantics { contentDescription = tr(lang, "docs.slideLayout") }) {
                Text(tr(lang, "docs.layout.$layout"))
            }
            DropdownMenu(menu, { menu = false }) {
                for (k in slideLayouts) DropdownMenuItem(text = { Text(tr(lang, "docs.layout.$k")) }, onClick = { layout = k; menu = false })
            }
        }
        Button(onClick = { vm.addSlide(layout) }, modifier = Modifier.padding(start = 8.dp).testTag("deck-add")) { Text(tr(lang, "docs.newSlide")) }
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
    Column(Modifier.fillMaxWidth().aspectRatio(16f / 9f).shadow(2.dp, RoundedCornerShape(4.dp)).background(bg, RoundedCornerShape(4.dp))
        .clip(RoundedCornerShape(4.dp)).padding(horizontal = 22.dp, vertical = 16.dp).testTag("slide"),
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
            // Una ruta del projecte no es pot carregar aquí: se'n diu el nom.
            "image" -> Row {
                Column(Modifier.weight(1f).padding(end = 10.dp)) { SlideBody(s) }
                Text(slideText(s, "image"), color = Muted, fontSize = 11.sp, modifier = Modifier.weight(1f))
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
        val layout = slideText(e.slide, "layout")
        Text(tr(lang, "docs.layout.$layout").takeIf { layout in slideLayouts } ?: layout, color = Muted, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
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
    val s = d.shownSheet() ?: return
    val e = d.edit?.takeIf { it.cell.isNotEmpty() }
    item(key = "sheet-tabs") {
        Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).padding(vertical = 8.dp)) {
            for (b in d.blocks) {
                val on = b.id == s.id
                Surface(onClick = { vm.showSheet(b.id) }, shape = RoundedCornerShape(8.dp), color = if (on) SuperficieOpaca else Color.Transparent,
                    border = BorderStroke(1.dp, if (on) Sea else Line), modifier = Modifier.padding(end = 6.dp).testTag("sheet-tab")) {
                    Text(b.name, color = if (on) Sea else Ink, fontSize = 13.sp, modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp))
                }
            }
        }
    }
    if (e != null) item(key = "cell-editor") { CellEditor(vm, d, e, lang) }
    val (cols, rows) = sheetSize(s)
    item(key = "sheet-head") {
        Row {
            SheetHead("", RowHeadWidth)
            Row(Modifier.horizontalScroll(hs)) { repeat(cols) { SheetHead(sheetColumn(it), CellWidth) } }
        }
    }
    items((1..rows).toList(), key = { "r:$it" }) { r ->
        Row {
            SheetHead("$r", RowHeadWidth)
            Row(Modifier.horizontalScroll(hs)) {
                repeat(cols) { c ->
                    val name = sheetColumn(c) + r
                    val on = e?.id == s.id && e.cell == name
                    Text(s.values[name] ?: s.cells[name].orEmpty(), color = Ink, fontSize = 13.sp, maxLines = 1, overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.size(CellWidth, CellHeight).border(if (on) 2.dp else 0.5.dp, if (on) Sea else Line)
                            .clickable { vm.editCell(s.id, name) }.padding(horizontal = 6.dp, vertical = 9.dp).testTag("cell-$name"))
                }
            }
        }
    }
}

/** La lletra d'una columna o el número d'una fila. */
@Composable
private fun SheetHead(text: String, width: Dp) {
    Box(Modifier.size(width, CellHeight).border(0.5.dp, Line), contentAlignment = Alignment.Center) {
        Text(text, color = Muted, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
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
        Row(verticalAlignment = Alignment.CenterVertically) {
            OutlinedTextField(e.text, vm::typeDocBlock, Modifier.weight(1f).focusRequester(focus), enabled = !e.saving, singleLine = true,
                label = { Text(sheet + "!" + e.cell) })
            Button(onClick = { vm.saveDocBlock(false) }, enabled = !e.saving, modifier = Modifier.padding(start = 8.dp)) { Text(tr(lang, "docs.save")) }
        }
    }
}

private fun LazyListScope.docSection(vm: GregalViewModel, lang: String, key: String, title: String, items: List<DocRef>, open: DocRef?) {
    item(key = "h:$key") {
        Text(title.uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(top = 20.dp, bottom = 6.dp))
        if (items.isEmpty()) Text(tr(lang, "docs.none"), color = Muted, fontSize = 14.sp)
    }
    items(items, key = { "$key:" + it.id }) { d ->
        Column(Modifier.fillMaxWidth().clickable { vm.openDoc(d) }.padding(vertical = 8.dp)) {
            Text(d.title, color = if (d.id == open?.id && d.scope == open.scope) Sea else Ink, fontSize = 15.sp)
            Text(when (d.kind) { "doc" -> tr(lang, "docs.kind.doc"); "deck" -> tr(lang, "docs.kind.deck"); "sheet" -> tr(lang, "docs.kind.sheet"); else -> d.kind }, color = Muted, fontSize = 12.sp)
        }
    }
}

/** L'editor d'un bloc: el markdown al seu lloc; després d'un 409, a sobre el text d'ara. */
@Composable
private fun BlockEditor(vm: GregalViewModel, e: DocEdit, lang: String) {
    Column(Modifier.fillMaxWidth().padding(vertical = 6.dp).testTag("doc-block-editor")) {
        e.conflict?.let { b ->
            Text(tr(lang, "docs.conflict"), color = Sea, fontWeight = FontWeight.SemiBold, fontSize = 13.sp)
            AssistantResponse(b.md, lang)
        }
        // Un bloc nou (també el d'un document buit) s'obre amb el cursor a dins, com a la web.
        val focus = remember { FocusRequester() }
        if (e.id.isEmpty()) LaunchedEffect(Unit) { focus.requestFocus() }
        OutlinedTextField(e.text, vm::typeDocBlock, Modifier.fillMaxWidth().focusRequester(focus), enabled = !e.saving)
        Row(Modifier.padding(top = 6.dp)) {
            Button(onClick = { vm.saveDocBlock(false) }, enabled = !e.saving) { Text(tr(lang, "docs.save")) }
            OutlinedButton(onClick = { vm.saveDocBlock(true) }, enabled = !e.saving, modifier = Modifier.padding(start = 8.dp)) {
                Text(tr(lang, "docs.addBlock"))
            }
        }
    }
}

/** La targeta del xat quan l'agent escriu un document: l'obre a DocScreen. */
@Composable
fun DocCard(d: DocRef, lang: String, onOpen: () -> Unit) {
    Surface(onClick = onOpen, shape = RoundedCornerShape(14.dp), color = SuperficieOpaca, border = BorderStroke(1.dp, Line),
        modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp).testTag("doc-card")) {
        Row(Modifier.padding(horizontal = 14.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Rounded.Description, null, tint = Sea)
            Column(Modifier.padding(start = 12.dp)) {
                Text(d.title, color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
                Text(tr(lang, "docs.card"), color = Muted, fontSize = 12.sp)
            }
        }
    }
}
