package xyz.solanes.gregal

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.spring
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
import androidx.compose.ui.graphics.graphicsLayer
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

/** Pura: el document que el vent ha escrit en aquest torn (l'últim després del teu missatge). */
internal fun turnDoc(msgs: List<GregalViewModel.PetMsg>): DocRef? =
    msgs.takeLastWhile { it.from != "user" }.lastOrNull { it.kind == "doc" }?.doc

/**
 * La feina del vent a sobre del camp (direcció visual §2.5): mentre treballa amb
 * l'ordinador, la miniatura en directe (GET /api/ordinador/live) i «Mira-ho», que
 * obre el full Ordinador; si escriu un document, el document i «Obre». Només
 * consulta mentre és visible i el vent treballa. Quan el torn acaba la targeta
 * marxa, tret que l'hagis obert durant el torn: llavors es queda.
 */
@Composable
internal fun GregalWorkStrip(conn: Conn, lang: String, accent: Color, working: Boolean, doc: DocRef?, onOpen: () -> Unit, onDoc: (DocRef) -> Unit) {
    var opened by remember { mutableStateOf(false) }
    LaunchedEffect(working) { if (working) opened = false }
    val active = working || opened
    var shot by remember { mutableStateOf<OrdinadorLive?>(null) }
    LaunchedEffect(active, conn) {
        if (!active) { shot = null; return@LaunchedEffect }
        while (true) {
            try { shot = withContext(Dispatchers.IO) { conn.api().ordinadorLive() } }
            catch (e: Exception) { if (e is CancellationException) throw e; shot = null }
            delay(2_000)
        }
    }
    if (!active) return
    WindWorkCard(lang, accent, shot, doc, onComputer = { opened = true; onOpen() }, onDoc = { opened = true; onDoc(it) })
}

/** La targeta: l'ordinador si hi ha una pàgina oberta, el document si n'escriu un; sense res, no surt. */
@Composable
internal fun WindWorkCard(lang: String, accent: Color, shot: OrdinadorLive?, doc: DocRef?, onComputer: () -> Unit, onDoc: (DocRef) -> Unit) {
    val s = shot?.takeIf { it.live }
    if (s == null && doc == null) return
    // Entra pujant des del camp, amb molla; la capa la mou sense tornar-la a mesurar.
    val rise = if (windMotionEnabled()) remember { Animatable(0f) } else null
    if (rise != null) LaunchedEffect(rise) { rise.animateTo(1f, spring(dampingRatio = .75f, stiffness = 500f)) }
    val shape = RoundedCornerShape(16.dp)
    Column(Modifier.fillMaxWidth().graphicsLayer {
        val p = rise?.value ?: 1f
        translationY = (1f - p) * 28.dp.toPx()
        alpha = p.coerceIn(0f, 1f)
    }.padding(horizontal = 12.dp, vertical = 4.dp).clip(shape).background(Superficie)
        .border(1.dp, Line, shape).testTag("wind-work-card")) {
        if (s != null) {
            val bmp = remember(s.image) { s.image?.let { runCatching { android.graphics.BitmapFactory.decodeByteArray(it, 0, it.size)?.asImageBitmap() }.getOrNull() } }
            WorkRow(tr(lang, "vents.work.on", workStripSite(s.url)), s.title, tr(lang, "vents.work.watch"), accent, onComputer) {
                Box(Modifier.size(width = 64.dp, height = 42.dp).clip(RoundedCornerShape(9.dp)).background(SuperficieAlta).border(1.dp, Line, RoundedCornerShape(9.dp))) {
                    if (bmp != null) Image(bmp, null, contentScale = ContentScale.Crop, alignment = Alignment.TopCenter, modifier = Modifier.fillMaxSize())
                }
            }
        }
        if (doc != null) WorkRow(doc.title, tr(lang, "vents.work.writing"), tr(lang, "vents.work.open"), accent, { onDoc(doc) }) {
            KindBadge(doc.kind, 42.dp)
        }
    }
}

@Composable
private fun WorkRow(title: String, sub: String, action: String, accent: Color, onClick: () -> Unit, lead: @Composable () -> Unit) {
    Row(Modifier.fillMaxWidth().clickable(role = Role.Button, onClick = onClick).padding(8.dp),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        lead()
        Column(Modifier.weight(1f)) {
            Text(title, color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (sub.isNotBlank()) Text(sub, color = Muted, fontSize = 11.5.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        Text(action, color = accent, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(end = 6.dp))
    }
}
