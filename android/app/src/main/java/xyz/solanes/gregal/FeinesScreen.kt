package xyz.solanes.gregal

import android.annotation.SuppressLint
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Construction
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
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties

/**
 * Feines: el que els vents fan al servidor (o al mòbil) en una tasca llarga
 * —una web, una anàlisi, la feina d'un equip—, amb l'estat, qui hi treballa i
 * els fitxers que en surten. Abans només es veia dins del fil del grup i, si
 * tancaves l'app, en tornar ja no se'n veia el progrés.
 */
data class WindTask(
    val id: String,
    val task: String,
    val workspace: String,
    val thread: String,
    val created: Long,
    val updated: Long,
    val status: String, // running | done | failed | cancelled | interrupted
    val agents: List<String>,
    val files: List<String>,
    val result: String,
    val error: String,
) {
    val running get() = status == "running"

    /** Ruta per a /api/file: el fitxer dins de la carpeta de la feina. */
    fun pathOf(file: String): String =
        if (file.startsWith("/") || (file.length > 1 && file[1] == ':')) file
        else workspace.trimEnd('/', '\\') + "/" + file
}

/** Canvis d'estat entre dues lectures: les feines que han acabat. */
internal fun feinesAcabades(abans: List<WindTask>, ara: List<WindTask>): List<WindTask> {
    val enMarxa = abans.filter { it.running }.map { it.id }.toSet()
    return ara.filter { it.id in enMarxa && !it.running }
}

@Composable
internal fun FeinesButton(lang: String, running: Int, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        shape = MaterialTheme.shapes.medium,
        color = SuperficieAlta.copy(alpha = .30f),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 6.dp),
    ) {
        Row(Modifier.padding(horizontal = 14.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Rounded.Construction, null, tint = Sea, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(10.dp))
            Text(tr(lang, "app.feines.title"), color = Ink, fontWeight = FontWeight.SemiBold, fontSize = 14.sp, modifier = Modifier.weight(1f))
            if (running > 0) {
                CircularProgressIndicator(Modifier.size(14.dp), color = Sea, strokeWidth = 2.dp)
                Spacer(Modifier.width(8.dp))
                Text(tr(lang, "app.feines.running", running), color = Sea, fontSize = 13.sp)
                Spacer(Modifier.width(8.dp))
            }
            Text("›", color = Muted, fontSize = 18.sp)
        }
    }
}

@Composable
internal fun FeinesDialog(vm: GregalViewModel, lang: String, onOpenThread: (String) -> Unit, onDismiss: () -> Unit) {
    val tasks by vm.windTasks.collectAsState()
    var file by remember { mutableStateOf<Pair<WindTask, String>?>(null) }
    LaunchedEffect(Unit) { vm.loadWindTasks() }
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(shape = MaterialTheme.shapes.large, color = MaterialTheme.colorScheme.background) {
            Column(Modifier.fillMaxWidth().fillMaxHeight(0.85f).padding(top = 16.dp)) {
                Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(tr(lang, "app.feines.title"), color = Ink, fontWeight = FontWeight.Bold, fontSize = 18.sp, modifier = Modifier.weight(1f))
                    IconButton(onClick = onDismiss) { Text("✕", color = Muted, fontSize = 16.sp) }
                }
                val list = tasks
                when {
                    list == null -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        CircularProgressIndicator(color = Sea)
                    }
                    list.isEmpty() -> Column(Modifier.fillMaxSize().padding(28.dp), verticalArrangement = Arrangement.Center) {
                        Text(tr(lang, "app.feines.empty"), color = Ink, fontWeight = FontWeight.SemiBold)
                        Spacer(Modifier.height(6.dp))
                        Text(tr(lang, "app.feines.emptyHelp"), color = Muted, fontSize = 14.sp)
                    }
                    else -> LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = 20.dp)) {
                        items(list, key = { it.id }) { t ->
                            FeinaRow(lang, t, onFile = { f -> file = t to f }, onThread = {
                                onDismiss(); onOpenThread(t.thread)
                            })
                        }
                    }
                }
            }
        }
    }
    file?.let { (t, f) -> FeinaFileDialog(vm, lang, t, f) { file = null } }
}

@Composable
private fun FeinaRow(lang: String, t: WindTask, onFile: (String) -> Unit, onThread: () -> Unit) {
    val (label, color) = when (t.status) {
        "running" -> tr(lang, "app.feines.st.running") to Sea
        "done" -> tr(lang, "app.feines.st.done") to Color(0xFF6FCF97)
        "interrupted" -> tr(lang, "app.feines.st.interrupted") to Sand
        "cancelled" -> tr(lang, "app.feines.st.cancelled") to Muted
        else -> tr(lang, "app.feines.st.failed") to Color(0xFFE57373)
    }
    Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 10.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            if (t.running) {
                CircularProgressIndicator(Modifier.size(12.dp), color = color, strokeWidth = 2.dp)
                Spacer(Modifier.width(6.dp))
            }
            Text(label, color = color, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.width(8.dp))
            Text(programatHora(java.time.Instant.ofEpochMilli(t.updated).toString(), lang = lang),
                color = Muted, fontSize = 12.sp, maxLines = 1)
        }
        Spacer(Modifier.height(3.dp))
        Text(t.task, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, maxLines = 3, overflow = TextOverflow.Ellipsis)
        if (t.agents.isNotEmpty()) {
            Text(t.agents.joinToString(" · "), color = Muted, fontSize = 12.sp, maxLines = 1)
        }
        val text = t.error.ifBlank { t.result }
        if (text.isNotBlank()) {
            Spacer(Modifier.height(4.dp))
            Text(stripMdInline(text).replace(Regex("\\s+"), " ").trim(), color = Muted, fontSize = 13.sp,
                maxLines = 3, overflow = TextOverflow.Ellipsis, lineHeight = 18.sp)
        }
        if (t.files.isNotEmpty() || t.thread.isNotBlank()) {
            Spacer(Modifier.height(6.dp))
            Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                t.files.take(8).forEach { f ->
                    AssistChip(onClick = { onFile(f) }, label = { Text(f.substringAfterLast('/'), fontSize = 12.sp, maxLines = 1) })
                }
                if (t.thread.isNotBlank()) {
                    AssistChip(onClick = onThread, label = { Text(tr(lang, "app.feines.thread"), fontSize = 12.sp) })
                }
            }
        }
        HorizontalDivider(Modifier.padding(top = 10.dp), color = Line.copy(alpha = .5f))
    }
}

/**
 * Obre un fitxer de la feina: una web es mostra com a web (el que ha fet el
 * vent, sense accés a l'app ni als fitxers del mòbil); la resta, en text.
 */
@SuppressLint("SetJavaScriptEnabled")
@Composable
private fun FeinaFileDialog(vm: GregalViewModel, lang: String, t: WindTask, file: String, onDismiss: () -> Unit) {
    var content by remember { mutableStateOf<String?>(null) }
    var err by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(t.id, file) {
        vm.readTaskFile(t.pathOf(file)) { text, error -> content = text; err = error }
    }
    val html = file.endsWith(".html", true) || file.endsWith(".htm", true)
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(shape = MaterialTheme.shapes.large, color = MaterialTheme.colorScheme.background) {
            Column(Modifier.fillMaxWidth().fillMaxHeight(0.92f).padding(top = 12.dp)) {
                Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(file.substringAfterLast('/'), color = Ink, fontWeight = FontWeight.Bold, fontSize = 16.sp,
                        maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                    IconButton(onClick = onDismiss) { Text("✕", color = Muted, fontSize = 16.sp) }
                }
                val c = content
                when {
                    err != null -> Text(err.orEmpty(), color = Muted, modifier = Modifier.padding(20.dp))
                    c == null -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { CircularProgressIndicator(color = Sea) }
                    html -> AndroidView(
                        modifier = Modifier.fillMaxSize(),
                        factory = { ctx ->
                            WebView(ctx).apply {
                                settings.javaScriptEnabled = true
                                settings.allowFileAccess = false
                                settings.allowContentAccess = false
                                settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
                                webViewClient = WebViewClient()
                                // Mentre es pinta, blanc com una pàgina (no negre).
                                setBackgroundColor(android.graphics.Color.WHITE)
                                loadDataWithBaseURL("about:blank", c, "text/html", "utf-8", null)
                            }
                        },
                    )
                    else -> Text(c, color = Ink, fontFamily = FontFamily.Monospace, fontSize = 12.sp,
                        modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState())
                            .horizontalScroll(rememberScrollState()).padding(16.dp))
                }
            }
        }
    }
}
