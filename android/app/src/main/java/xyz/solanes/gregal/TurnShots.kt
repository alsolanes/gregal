package xyz.solanes.gregal

import android.graphics.BitmapFactory
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * «Ho he fet així»: les captures del que el vent ha fet a l'ordinador en un
 * torn, sota la resposta (com «torna amb captures» de Grok). Un toc les obre
 * grans, amb ‹ ›. La web fa el mateix a winds-workspace.js (shotStrip).
 */
data class MsgShot(val id: String, val title: String = "", val url: String = "", val shotUrl: String = "")

internal fun turnShotsCopy(lang: String, key: String, n: Int = 0): String {
    val en = lang == "en"
    return when (key) {
        "title" -> if (en) "How I did it" else "Ho he fet així"
        "count" -> if (en) (if (n == 1) "1 screenshot" else "$n screenshots") else (if (n == 1) "1 captura" else "$n captures")
        "shot" -> if (en) "Screenshot $n" else "Captura $n"
        "prev" -> if (en) "Previous" else "Anterior"
        "next" -> if (en) "Next" else "Següent"
        "close" -> if (en) "Close" else "Tanca"
        else -> key
    }
}

/** El domini d'una adreça, sense «www.» (el peu de cada miniatura). */
internal fun shotHost(url: String): String =
    runCatching { java.net.URI(url).host.orEmpty().removePrefix("www.") }.getOrDefault("")

@Composable
private fun rememberShot(conn: Conn, shot: MsgShot): State<ImageBitmap?> =
    produceState<ImageBitmap?>(null, shot.shotUrl) {
        value = try {
            withContext(Dispatchers.IO) {
                val bytes = conn.api().ordinadorShot(shot.shotUrl)
                BitmapFactory.decodeByteArray(bytes, 0, bytes.size)?.asImageBitmap()
            }
        } catch (e: Exception) {
            if (e is CancellationException) throw e
            null
        }
    }

@Composable
internal fun TurnShotsStrip(conn: Conn, lang: String, accent: Color, shots: List<MsgShot>) {
    var open by remember(shots) { mutableStateOf(-1) }
    Column(Modifier.padding(start = 46.dp, top = 6.dp).testTag("turn-shots"), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.Bottom) {
            Text(turnShotsCopy(lang, "title"), color = Ink, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
            Text(turnShotsCopy(lang, "count", shots.size), color = Muted, fontSize = 12.sp)
        }
        Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            shots.forEachIndexed { i, sh ->
                val bmp by rememberShot(conn, sh)
                val label = turnShotsCopy(lang, "shot", i + 1) + if (sh.title.isNotBlank()) ": " + sh.title else ""
                Column(
                    Modifier.width(120.dp).clip(GregalShape.sm)
                        .semantics { role = Role.Button; contentDescription = label }
                        .clickable { open = i }.testTag("turn-shot-$i"),
                    verticalArrangement = Arrangement.spacedBy(3.dp),
                ) {
                    Box(Modifier.fillMaxWidth().height(76.dp).clip(GregalShape.sm).background(SuperficieAlta).border(1.dp, Line, GregalShape.sm)) {
                        bmp?.let { Image(it, null, contentScale = ContentScale.Crop, alignment = Alignment.TopCenter, modifier = Modifier.fillMaxSize()) }
                    }
                    Text(shotHost(sh.url).ifBlank { sh.title }, color = Muted, fontSize = 11.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
        }
    }
    if (open >= 0) TurnShotsViewer(conn, lang, accent, shots, open) { open = -1 }
}

@Composable
internal fun TurnShotsViewer(conn: Conn, lang: String, accent: Color, shots: List<MsgShot>, start: Int, onClose: () -> Unit) {
    var i by remember { mutableStateOf(start.coerceIn(0, shots.lastIndex)) }
    Dialog(onDismissRequest = onClose, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Column(
            Modifier.fillMaxWidth().padding(12.dp).clip(GregalShape.lg).background(Superficie).padding(14.dp).testTag("turn-shots-viewer"),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            val sh = shots[i]
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text(turnShotsCopy(lang, "title"), color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                    Text(sh.title.ifBlank { sh.url }, color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                ShotNavButton("✕", turnShotsCopy(lang, "close"), enabled = true, onClick = onClose)
            }
            val bmp by rememberShot(conn, sh)
            Box(Modifier.fillMaxWidth().heightIn(min = 160.dp).clip(GregalShape.sm).background(SuperficieAlta), contentAlignment = Alignment.Center) {
                bmp?.let { Image(it, sh.title, contentScale = ContentScale.FillWidth, modifier = Modifier.fillMaxWidth()) }
            }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center, verticalAlignment = Alignment.CenterVertically) {
                ShotNavButton("‹", turnShotsCopy(lang, "prev"), enabled = i > 0) { i -= 1 }
                Text("${i + 1} / ${shots.size}", color = Muted, fontSize = 13.sp,
                    modifier = Modifier.padding(horizontal = 18.dp).testTag("turn-shots-pos"))
                ShotNavButton("›", turnShotsCopy(lang, "next"), enabled = i < shots.lastIndex) { i += 1 }
            }
        }
    }
}

@Composable
private fun ShotNavButton(label: String, description: String, enabled: Boolean, onClick: () -> Unit) {
    Box(
        Modifier.size(48.dp).clip(GregalShape.sm).border(1.dp, Line, GregalShape.sm)
            .semantics { role = Role.Button; contentDescription = description }
            .then(if (enabled) Modifier.clickable(onClick = onClick) else Modifier),
        contentAlignment = Alignment.Center,
    ) {
        Text(label, color = if (enabled) Ink else Muted.copy(alpha = .4f), fontSize = 18.sp)
    }
}
