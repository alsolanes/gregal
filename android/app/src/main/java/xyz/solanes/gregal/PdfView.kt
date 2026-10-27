package xyz.solanes.gregal

import android.graphics.Bitmap
import android.graphics.Color as AColor
import android.graphics.pdf.PdfRenderer
import android.os.ParcelFileDescriptor
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File

/** Pàgines que es pinten a la vista d'un PDF (la resta, a la pestanya Text
 *  o demanant-les a l'agent): prou per reconèixer-lo sense gastar memòria. */
const val PDF_PREVIEW_PAGES = 6

/** Enllaç a un document que serveix Gregal (el que crea un vent o un adjunt
 *  editat): es baixa amb l'autenticació de l'app, no al navegador. Pura. */
fun isServerDocLink(url: String): Boolean =
    url.startsWith("/api/winds/doc?") || url.startsWith("/api/office/download?")

/** Nom amb què es desa un document de Gregal: el «name» de l'enllaç, o el
 *  text de l'enllaç si no en du. Pura. */
fun serverDocName(url: String, fallback: String = "document"): String {
    val q = url.substringAfter('?', "")
    val name = q.split('&').firstOrNull { it.startsWith("name=") }?.substringAfter('=')
        ?.let { runCatching { java.net.URLDecoder.decode(it, "UTF-8") }.getOrNull() }
    return name?.substringAfterLast('/')?.takeIf { it.isNotBlank() } ?: fallback
}

/** El document pujat és un PDF (pel tipus del servidor o pel nom). Pura. */
fun isPdfDoc(kind: String, name: String): Boolean =
    kind.equals("pdf", true) || name.endsWith(".pdf", true)

/** Amplada en píxels per pintar una pàgina de w×h punts dins maxWidth,
 *  mantenint la proporció i sense passar-se de 2× la mida original. Pura. */
fun pdfPageBitmapSize(w: Int, h: Int, maxWidth: Int): Pair<Int, Int> {
    if (w <= 0 || h <= 0 || maxWidth <= 0) return 1 to 1
    val width = minOf(maxWidth, w * 2)
    return width to maxOf(1, (h.toLong() * width / w).toInt())
}

/** Vista d'un PDF amb el PdfRenderer d'Android: no cal cap rasteritzador al
 *  servidor. Els bytes els baixa l'app (Bearer) i es pinten fora del fil
 *  principal. */
@Composable
fun PdfPages(vm: GregalViewModel, doc: OfficeDoc, modifier: Modifier = Modifier) {
    val ctx = LocalContext.current
    val lang by vm.idioma.collectAsState()
    var pages by remember(doc.id, doc.text) { mutableStateOf<List<ImageBitmap>>(emptyList()) }
    var total by remember(doc.id) { mutableStateOf(0) }
    var error by remember(doc.id, lang) { mutableStateOf("") }

    LaunchedEffect(doc.id, doc.text) {
        try {
            val bytes = vm.officeDownloadBytes()
            if (bytes == null) { error = tr(lang, "app.off.downloadError"); return@LaunchedEffect }
            val maxWidth = ctx.resources.displayMetrics.widthPixels
            val (bmps, n) = withContext(Dispatchers.IO) {
                val f = File(ctx.cacheDir, "gregal-pdf-${doc.id}.pdf")
                f.writeBytes(bytes)
                try {
                    ParcelFileDescriptor.open(f, ParcelFileDescriptor.MODE_READ_ONLY).use { fd ->
                        PdfRenderer(fd).use { r ->
                            val out = mutableListOf<ImageBitmap>()
                            for (i in 0 until minOf(r.pageCount, PDF_PREVIEW_PAGES)) {
                                r.openPage(i).use { p ->
                                    val (bw, bh) = pdfPageBitmapSize(p.width, p.height, maxWidth)
                                    val bmp = Bitmap.createBitmap(bw, bh, Bitmap.Config.ARGB_8888)
                                    bmp.eraseColor(AColor.WHITE)
                                    p.render(bmp, null, null, PdfRenderer.Page.RENDER_MODE_FOR_DISPLAY)
                                    out.add(bmp.asImageBitmap())
                                }
                            }
                            out to r.pageCount
                        }
                    }
                } finally {
                    f.delete()
                }
            }
            pages = bmps
            total = n
            error = ""
        } catch (e: Exception) {
            error = tr(lang, "app.off.renderErrorPrefix") + e.message.orEmpty() + tr(lang, "app.off.renderErrorSuffix")
        }
    }

    Card(modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = CodeDark)) {
        Column(Modifier.padding(8.dp).heightIn(max = 520.dp).verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(8.dp)) {
            when {
                error.isNotBlank() -> Text(error, color = Ink, fontSize = 13.sp, modifier = Modifier.padding(4.dp))
                pages.isEmpty() -> Text(tr(lang, "app.off.renderLoading"), color = Muted, fontSize = 13.sp, modifier = Modifier.padding(4.dp))
                else -> {
                    pages.forEach { bmp ->
                        Image(bmp, contentDescription = null, contentScale = ContentScale.FillWidth,
                            modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(6.dp)).background(Color.White))
                    }
                    if (total > pages.size) {
                        Text(tr(lang, "app.off.pdfMorePages", pages.size, total), color = Muted, fontSize = 12.sp)
                    }
                }
            }
        }
    }
}
