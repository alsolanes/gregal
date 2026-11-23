package xyz.solanes.gregal

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.ScrollState
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

// Gregal Docs a pantalla completa (al mòbil no hi ha lloc per al panell
// lateral de la web). Sense document obert és l'inici (doc-home.js):
// «Document nou», «Els meus documents» i, a Codi amb projecte, «Aquest
// projecte». Amb un document obert, els blocs amb el markdown del xat, al
// dia mentre l'agent l'escriu, i a sota les mateixes llistes.

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
            } else if (open != null && open.kind != "doc") {
                // Les presentacions s'obriran i s'editaran a 074.
                item(key = "deck") { Text(tr(lang, "docs.deckLater"), color = Muted, modifier = Modifier.padding(vertical = 12.dp)) }
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
    OutlinedButton(onClick = { vm.newDoc(title, "sheet") }, modifier = Modifier.testTag("doc-new-sheet")) { Text(tr(lang, "docs.newSheet")) }
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
