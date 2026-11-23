package xyz.solanes.gregal

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
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
    Scaffold(containerColor = Color.Transparent, topBar = {
        GregalTopBar(open?.title ?: tr(lang, "docs.home"), lang, onMenu = onMenu)
    }) { pad ->
        LazyColumn(Modifier.fillMaxSize().padding(pad).testTag("doc-screen"),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 110.dp)) {
            if (open != null && open.kind != "doc") {
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
            Text(when (d.kind) { "doc" -> tr(lang, "docs.kind.doc"); "deck" -> tr(lang, "docs.kind.deck"); else -> d.kind }, color = Muted, fontSize = 12.sp)
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
