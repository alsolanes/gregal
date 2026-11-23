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
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Description
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

// Un document viu a pantalla completa (al mòbil no hi ha lloc per al panell
// lateral de la web): els blocs amb el markdown del xat, al dia mentre
// l'agent l'escriu, i a sota els altres documents del projecte.

@Composable
fun DocScreen(vm: GregalViewModel, onMenu: () -> Unit) {
    val lang by vm.idioma.collectAsState()
    val id by vm.docId.collectAsState()
    val doc by vm.doc.collectAsState()
    val docs by vm.docs.collectAsState()
    LaunchedEffect(Unit) { vm.loadDocs() }
    LaunchedEffect(id) { if (id.isNotEmpty()) vm.followDoc(id) }
    val open = doc?.takeIf { it.id == id }
    Scaffold(containerColor = Color.Transparent, topBar = {
        GregalTopBar(open?.title ?: tr(lang, "docs.title"), lang, onMenu = onMenu)
    }) { pad ->
        LazyColumn(Modifier.fillMaxSize().padding(pad).testTag("doc-screen"),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 110.dp)) {
            if (open != null) {
                item(key = "download") {
                    OutlinedButton(onClick = { vm.downloadServerDoc(docExportPath(DocRef(open.id, open.title)), open.title + ".docx") }) {
                        Text(tr(lang, "docs.download"))
                    }
                }
                items(open.blocks, key = { "b:" + it.id }) { b -> AssistantResponse(b.md, lang) }
            } else if (id.isNotEmpty()) {
                item(key = "loading") { Text(tr(lang, "docs.loading"), color = Muted, modifier = Modifier.padding(vertical = 12.dp)) }
            }
            item(key = "list") {
                Text(tr(lang, "docs.list").uppercase(), color = Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold,
                    modifier = Modifier.padding(top = 20.dp, bottom = 6.dp))
                if (docs.isEmpty()) Text(tr(lang, "docs.empty"), color = Muted, fontSize = 14.sp)
            }
            items(docs, key = { "d:" + it.id }) { d ->
                Text(d.title, color = if (d.id == id) Sea else Ink, fontSize = 15.sp,
                    modifier = Modifier.fillMaxWidth().clickable { vm.openDoc(d.id) }.padding(vertical = 10.dp))
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
