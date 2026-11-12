package xyz.solanes.gregal

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext

/** Pura: el lloc que es mostra a la franja («renfe.com») a partir de l'adreça. */
internal fun workStripSite(url: String): String =
    url.substringAfter("://", url).substringBefore('/').removePrefix("www.").take(40)

/**
 * La franja de feina de la portada de Gregal (direcció visual §2.5): mentre
 * treballa amb el seu ordinador, a sobre del camp hi ha la miniatura en directe
 * (GET /api/ordinador/live), el lloc i «Mira-ho», que obre el full Ordinador.
 * Només consulta mentre és visible i Gregal treballa; si no té cap pàgina
 * oberta, no surt.
 */
@Composable
internal fun GregalWorkStrip(conn: Conn, lang: String, accent: Color, working: Boolean, onOpen: () -> Unit) {
    var shot by remember { mutableStateOf<OrdinadorLive?>(null) }
    LaunchedEffect(working, conn) {
        if (!working) { shot = null; return@LaunchedEffect }
        while (true) {
            try { shot = withContext(Dispatchers.IO) { conn.api().ordinadorLive() } }
            catch (e: Exception) { if (e is CancellationException) throw e; shot = null }
            delay(2_000)
        }
    }
    val s = shot?.takeIf { working && it.live } ?: return
    val bmp = remember(s.image) { s.image?.let { runCatching { android.graphics.BitmapFactory.decodeByteArray(it, 0, it.size)?.asImageBitmap() }.getOrNull() } }
    val en = lang == "en"
    val shape = RoundedCornerShape(16.dp)
    Row(Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 4.dp).clip(shape).background(Superficie)
        .border(1.dp, Line, shape).clickable(role = Role.Button, onClick = onOpen).padding(8.dp).testTag("gregal-work-strip"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        Box(Modifier.size(width = 64.dp, height = 42.dp).clip(RoundedCornerShape(9.dp)).background(SuperficieAlta).border(1.dp, Line, RoundedCornerShape(9.dp))) {
            if (bmp != null) Image(bmp, null, contentScale = ContentScale.Crop, alignment = Alignment.TopCenter, modifier = Modifier.fillMaxSize())
        }
        Column(Modifier.weight(1f)) {
            Text((if (en) "On " else "A ") + workStripSite(s.url), color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold,
                maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (s.title.isNotBlank()) Text(s.title, color = Muted, fontSize = 11.5.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        Text(if (en) "Watch" else "Mira-ho", color = accent, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(end = 6.dp))
    }
}
