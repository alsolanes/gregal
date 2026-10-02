package xyz.solanes.gregal

import android.provider.OpenableColumns
import android.util.Base64
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Menu
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch

private val OFFICE_EXTS = setOf("docx", "xlsx", "pptx")

/** El nom triat és un document Office suportat (puja per extensió). */
fun isOfficeName(name: String): Boolean =
    name.substringAfterLast('.', "").lowercase() in OFFICE_EXTS

/** Frases ràpides per tipus, com el QUICK de la web: l'edició la fa
 *  l'agent parlant, sense camps manuals (fora set_cell/cerca-substitueix). */
private fun quickPrompts(kind: String, lang: String): List<Pair<String, String>> {
    val xlsx = kind.contains("sheet", true)
    val pptx = kind.contains("slide", true) || kind.contains("present", true)
    return when {
        xlsx -> listOf(
            tr(lang, "app.off.quick.explain") to tr(lang, "app.off.prompt.xlsxExplain"),
            tr(lang, "app.off.quick.findErrors") to tr(lang, "app.off.prompt.xlsxErrors"),
        )
        pptx -> listOf(
            tr(lang, "app.off.quick.resume") to tr(lang, "app.off.prompt.pptxResume"),
            tr(lang, "app.off.quick.fix") to tr(lang, "app.off.prompt.pptxFix"),
        )
        else -> listOf(
            tr(lang, "app.off.quick.resume") to tr(lang, "app.off.prompt.docResume"),
            tr(lang, "app.off.quick.fix") to tr(lang, "app.off.prompt.docFix"),
            tr(lang, "app.off.quick.translate") to tr(lang, "app.off.prompt.docTranslate"),
        )
    }
}

/** Pantalla Office: puja un .docx/.xlsx/.pptx, llegeix-lo i demana coses
 *  a l'agent sobre ell. Paritat amb la pàgina Office de la web. */
@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun OfficeScreen(vm: GregalViewModel, onMenu: () -> Unit, onAskAgent: () -> Unit) {
    val doc by vm.office.collectAsState()
    val lang by vm.idioma.collectAsState()
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var notice by remember { mutableStateOf("") }
    // Edició directa del document (abans en un diàleg amb icona pròpia).
    var full by remember { mutableStateOf("") }
    var cela by remember { mutableStateOf("") }
    var valor by remember { mutableStateOf("") }
    var cerca by remember { mutableStateOf("") }
    var substitut by remember { mutableStateOf("") }
    var vista by remember { mutableStateOf(true) }
    var question by remember { mutableStateOf("") }

    val pick = rememberLauncherForActivityResult(ActivityResultContracts.GetContent()) { uri ->
        if (uri == null) return@rememberLauncherForActivityResult
        try {
            var name = "document"
            ctx.contentResolver.query(uri, null, null, null, null)?.use { c ->
                val i = c.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                if (c.moveToFirst() && i >= 0) name = c.getString(i) ?: name
            }
            if (!isOfficeName(name)) {
                notice = tr(lang, "app.off.fileTypeError", name)
                return@rememberLauncherForActivityResult
            }
            val bytes = ctx.contentResolver.openInputStream(uri)?.use { it.readBytes() }
                ?: run { notice = tr(lang, "app.off.fileReadError"); return@rememberLauncherForActivityResult }
            if (bytes.size > 25 * 1024 * 1024) {
                notice = tr(lang, "app.off.fileTooLarge", 25, bytes.size / 1024 / 1024)
                return@rememberLauncherForActivityResult
            }
            notice = ""
            vista = true
            vm.officeUpload(name, Base64.encodeToString(bytes, Base64.NO_WRAP))
        } catch (e: Exception) {
            notice = tr(lang, "app.off.fileOpenError", e.message.orEmpty())
        }
    }

    fun ask(text: String) {
        val q = text.ifBlank { return }
        vm.send(tr(lang, "app.off.askContext", doc.name, doc.id, q))
        question = ""
        onAskAgent()
    }

    GregalBg {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                TopAppBar(
                    navigationIcon = { IconButton(onClick = onMenu) { Icon(Icons.Default.Menu, tr(lang, "app.nav.menu"), tint = Ink) } },
                    title = { Text(tr(lang, "app.off.title"), color = Ink) },
                    actions = {
                        if (doc.id.isNotBlank()) IconButton(onClick = { vm.officeClear(); notice = "" }) {
                            Icon(Icons.Default.Refresh, tr(lang, "app.off.closeDoc"), tint = Ink)
                        }
                    },
                    colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent))
            },
        ) { pad ->
            Column(Modifier.fillMaxSize().padding(pad).padding(12.dp).verticalScroll(rememberScrollState())) {
                if (doc.id.isBlank()) {
                    EmptyBoard("❏", tr(lang, "app.off.empty"), tr(lang, "app.off.emptyHelp"))
                    Spacer(Modifier.height(12.dp))
                    Button(onClick = { pick.launch("*/*") }, modifier = Modifier.fillMaxWidth()) {
                        Text(tr(lang, "app.off.pick"))
                    }
                } else {
                    Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = PillDark)) {
                        Column(Modifier.padding(14.dp)) {
                            Text(doc.name.ifBlank { tr(lang, "app.off.document") }, color = Ink, fontSize = 16.sp)
                            if (doc.kind.isNotBlank()) Text(doc.kind, color = Muted, fontSize = 13.sp)
                            if (doc.result.isNotBlank()) Text(doc.result, color = Sea, fontSize = 13.sp)
                            if (doc.error.isNotBlank()) Text(doc.error, color = Sand, fontSize = 13.sp)
                        }
                    }
                    Spacer(Modifier.height(10.dp))
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        FilterChip(vista, { vista = true }, { Text(tr(lang, "app.off.view")) })
                        FilterChip(!vista, { vista = false }, { Text(tr(lang, "app.off.text")) })
                    }
                    Spacer(Modifier.height(8.dp))
                    if (vista) {
                        OfficeRender(vm, doc, Modifier.fillMaxWidth())
                    } else if (doc.text.isNotBlank()) {
                        Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = CodeDark)) {
                            Text(doc.text, color = Ink, fontSize = 13.sp, fontFamily = FontFamily.Monospace,
                                modifier = Modifier.padding(12.dp).heightIn(max = 320.dp).verticalScroll(rememberScrollState()))
                        }
                    }
                    Spacer(Modifier.height(10.dp))
                    FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        quickPrompts(doc.kind.ifBlank { doc.name }, lang).forEach { (label, prompt) ->
                            OutlinedButton(onClick = { ask(prompt) }) { Text(label) }
                        }
                    }
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(question, { question = it }, Modifier.fillMaxWidth(),
                        label = { Text(tr(lang, "app.off.ask")) })
                    Spacer(Modifier.height(8.dp))
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Button(onClick = { ask(question.ifBlank { tr(lang, "app.off.prompt.docResume") }) },
                            modifier = Modifier.weight(1f)) { Text(tr(lang, "btn.send")) }
                        OutlinedButton(onClick = {
                            scope.launch {
                                val bytes = vm.officeDownloadBytes()
                                notice = if (bytes != null) saveToDownloads(ctx, doc.name, bytes, lang)
                                else tr(lang, "app.off.downloadError")
                            }
                        }, modifier = Modifier.weight(1f)) { Text(tr(lang, "app.off.download")) }
                    }
                    Spacer(Modifier.height(14.dp))
                    Text(tr(lang, "app.off.edit"), color = Ink, fontSize = 15.sp)
                    Spacer(Modifier.height(6.dp))
                    if (doc.kind == "xlsx") {
                        OutlinedTextField(full, { full = it }, Modifier.fillMaxWidth(),
                            label = { Text(tr(lang, "app.off.sheet")) }, singleLine = true)
                        Spacer(Modifier.height(6.dp))
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            OutlinedTextField(cela, { cela = it }, Modifier.weight(1f),
                                label = { Text(tr(lang, "app.off.cell")) }, singleLine = true)
                            OutlinedTextField(valor, { valor = it }, Modifier.weight(1f),
                                label = { Text(tr(lang, "app.off.value")) }, singleLine = true)
                        }
                        Spacer(Modifier.height(6.dp))
                        OutlinedButton(onClick = {
                            notice = ""
                            vm.officeEdit("set_cell", full, cela, valor)
                        }) { Text(tr(lang, "app.off.setValue")) }
                    } else {
                        OutlinedTextField(cerca, { cerca = it }, Modifier.fillMaxWidth(),
                            label = { Text(tr(lang, "app.off.findText")) }, singleLine = true)
                        Spacer(Modifier.height(6.dp))
                        OutlinedTextField(substitut, { substitut = it }, Modifier.fillMaxWidth(),
                            label = { Text(tr(lang, "app.off.replaceWith")) }, singleLine = true)
                        Spacer(Modifier.height(6.dp))
                        OutlinedButton(onClick = {
                            notice = ""
                            vm.officeEdit("replace", find = cerca, replace = substitut)
                        }) { Text(tr(lang, "app.off.replace")) }
                    }
                    Spacer(Modifier.height(8.dp))
                    OutlinedButton(onClick = { pick.launch("*/*") }, modifier = Modifier.fillMaxWidth()) {
                        Text(tr(lang, "app.off.change"))
                    }
                }
                if (notice.isNotBlank()) {
                    Spacer(Modifier.height(8.dp))
                    Text(notice, color = Sand, fontSize = 13.sp)
                }
            }
        }
    }
}
