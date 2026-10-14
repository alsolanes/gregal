package xyz.solanes.gregal

import android.content.Context
import android.net.Uri
import android.provider.OpenableColumns
import android.util.Base64
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Refresh
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.animation.core.animateFloat
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.ByteArrayOutputStream
import java.io.InputStream

/**
 * Avisos (`vm.notice`) per a les pantalles que no són el xat. El consumeix
 * en arribar (clearNotice) perquè cap altra pantalla —ni el xat durant la
 * transició— el torni a ensenyar.
 */
@Composable
fun NoticeHost(vm: GregalViewModel) {
    val snack = remember { SnackbarHostState() }
    LaunchedEffect(vm) {
        vm.notice.collect { t ->
            if (t.isEmpty()) return@collect
            vm.clearNotice()
            snack.currentSnackbarData?.dismiss()
            launch { snack.showSnackbar(t, withDismissAction = true) }
        }
    }
    SnackbarHost(snack)
}

/** Botó d'actualitzar de la barra de dalt; mentre carrega, un indicador petit. */
@Composable
fun RefreshAction(lang: String, loading: Boolean, onClick: () -> Unit) {
    IconButton(onClick = onClick, enabled = !loading) {
        if (loading) {
            CircularProgressIndicator(Modifier.size(20.dp), color = Sea, strokeWidth = 2.dp)
        } else {
            Icon(Icons.Rounded.Refresh, tr(lang, "app.ui.refresh"), tint = Ink)
        }
    }
}

/** Primera càrrega en curs: millor això que un tauler buit que menteix. */
@Composable
fun LoadingBoard(lang: String) {
    Box(Modifier.fillMaxWidth().padding(vertical = 48.dp), contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            CircularProgressIndicator(color = Sea)
            Text(tr(lang, "app.ui.loading"), color = Muted, fontSize = 13.sp,
                modifier = Modifier.padding(top = 12.dp))
        }
    }
}

/** Resultat de llegir un fitxer del selector (SAF) com a base64. */
sealed interface UriRead {
    class Ok(val base64: String, val size: Long) : UriRead
    class TooLarge(val size: Long) : UriRead
    object Unreadable : UriRead
    class Failed(val message: String) : UriRead
}

/** Mida declarada pel proveïdor (OpenableColumns.SIZE o descriptor); -1 si no se sap. */
fun uriSize(ctx: Context, uri: Uri): Long {
    val cr = ctx.contentResolver
    try {
        cr.query(uri, arrayOf(OpenableColumns.SIZE), null, null, null)?.use { c ->
            val i = c.getColumnIndex(OpenableColumns.SIZE)
            if (c.moveToFirst() && i >= 0 && !c.isNull(i)) {
                val s = c.getLong(i)
                if (s >= 0) return s
            }
        }
    } catch (_: Exception) {
    }
    try {
        cr.openAssetFileDescriptor(uri, "r")?.use { fd ->
            if (fd.length >= 0) return fd.length
        }
    } catch (_: Exception) {
    }
    return -1
}

/** Llegeix com a molt `max` bytes; null si el flux en té més. */
internal fun readCapped(ins: InputStream, max: Long): ByteArray? {
    val out = ByteArrayOutputStream()
    val buf = ByteArray(64 * 1024)
    var total = 0L
    while (true) {
        val n = ins.read(buf)
        if (n < 0) break
        total += n
        if (total > max) return null
        out.write(buf, 0, n)
    }
    return out.toByteArray()
}

/**
 * Comprova la mida ABANS de llegir, i llegeix + codifica fora del fil
 * principal. Un fitxer gegant no ha de penjar la UI ni tombar l'app (OOM).
 */
suspend fun readUriBase64(ctx: Context, uri: Uri, maxBytes: Long): UriRead = withContext(Dispatchers.IO) {
    var size = -1L
    try {
        size = uriSize(ctx, uri)
        if (size > maxBytes) return@withContext UriRead.TooLarge(size)
        val ins = ctx.contentResolver.openInputStream(uri) ?: return@withContext UriRead.Unreadable
        // La mida declarada pot faltar o mentir: el límit també es vigila llegint.
        val raw = ins.use { readCapped(it, maxBytes) }
            ?: return@withContext UriRead.TooLarge(maxOf(size, maxBytes + 1))
        UriRead.Ok(Base64.encodeToString(raw, Base64.NO_WRAP), raw.size.toLong())
    } catch (_: OutOfMemoryError) {
        UriRead.TooLarge(size)
    } catch (e: Exception) {
        UriRead.Failed(e.message.orEmpty())
    }
}

/**
 * Estirar per refrescar sense dependències (material3 1.2 no en porta i la
 * de material no és a la memòria cau): gest amb nestedScroll i un indicador
 * dibuixat damunt la llista. Es posa al modificador d'una LazyColumn.
 */
fun Modifier.pullToRefresh(refreshing: Boolean, onRefresh: () -> Unit): Modifier = composed {
    val density = androidx.compose.ui.platform.LocalDensity.current
    val threshold = with(density) { 76.dp.toPx() }
    val haptic = androidx.compose.ui.platform.LocalHapticFeedback.current
    var pull by remember { androidx.compose.runtime.mutableFloatStateOf(0f) }
    val color = Sea
    val bg = Superficie
    val line = LineHi
    val shown by androidx.compose.animation.core.animateFloatAsState(
        if (refreshing) threshold * 0.8f else pull, label = "estira")
    val spin by androidx.compose.animation.core.rememberInfiniteTransition(label = "gira").animateFloat(
        0f, 360f, androidx.compose.animation.core.infiniteRepeatable(
            androidx.compose.animation.core.tween(900, easing = androidx.compose.animation.core.LinearEasing)), label = "angle")
    val latest = androidx.compose.runtime.rememberUpdatedState(onRefresh)
    val busy = androidx.compose.runtime.rememberUpdatedState(refreshing)
    val conn = remember {
        object : androidx.compose.ui.input.nestedscroll.NestedScrollConnection {
            override fun onPreScroll(available: androidx.compose.ui.geometry.Offset,
                                     source: androidx.compose.ui.input.nestedscroll.NestedScrollSource): androidx.compose.ui.geometry.Offset {
                // Tornar a pujar el dit primer desfà l'estirada.
                if (available.y < 0 && pull > 0f) {
                    val c = maxOf(available.y, -pull)
                    pull += c
                    return androidx.compose.ui.geometry.Offset(0f, c)
                }
                return androidx.compose.ui.geometry.Offset.Zero
            }
            override fun onPostScroll(consumed: androidx.compose.ui.geometry.Offset, available: androidx.compose.ui.geometry.Offset,
                                      source: androidx.compose.ui.input.nestedscroll.NestedScrollSource): androidx.compose.ui.geometry.Offset {
                if (source == androidx.compose.ui.input.nestedscroll.NestedScrollSource.Drag && available.y > 0f && !busy.value) {
                    val before = pull
                    pull = (pull + available.y * 0.5f).coerceAtMost(threshold * 1.5f)
                    if (before < threshold && pull >= threshold) {
                        haptic.performHapticFeedback(androidx.compose.ui.hapticfeedback.HapticFeedbackType.TextHandleMove)
                    }
                    return androidx.compose.ui.geometry.Offset(0f, available.y)
                }
                return androidx.compose.ui.geometry.Offset.Zero
            }
            override suspend fun onPreFling(available: androidx.compose.ui.unit.Velocity): androidx.compose.ui.unit.Velocity {
                val armed = pull >= threshold
                val had = pull > 0f
                pull = 0f
                if (armed && !busy.value) latest.value()
                return if (had) available else androidx.compose.ui.unit.Velocity.Zero
            }
        }
    }
    this.nestedScroll(conn).drawWithContent {
        drawContent()
        if (shown < 2f) return@drawWithContent
        val p = if (refreshing) 1f else (shown / threshold).coerceIn(0f, 1f)
        val r = 17.dp.toPx()
        val c = androidx.compose.ui.geometry.Offset(size.width / 2f, -r + shown.coerceAtMost(threshold * 1.2f))
        drawCircle(bg, r, c)
        drawCircle(line, r, c, style = androidx.compose.ui.graphics.drawscope.Stroke(1.dp.toPx()))
        val a = r * 0.55f
        drawArc(
            color.copy(alpha = 0.35f + 0.65f * p),
            startAngle = if (refreshing) spin else -90f + 120f * p,
            sweepAngle = if (refreshing) 270f else 300f * p,
            useCenter = false,
            topLeft = androidx.compose.ui.geometry.Offset(c.x - a, c.y - a),
            size = androidx.compose.ui.geometry.Size(a * 2, a * 2),
            style = androidx.compose.ui.graphics.drawscope.Stroke(2.4.dp.toPx(), cap = androidx.compose.ui.graphics.StrokeCap.Round),
        )
    }
}
