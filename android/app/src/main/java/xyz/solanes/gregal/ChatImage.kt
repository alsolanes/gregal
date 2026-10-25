package xyz.solanes.gregal

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.gestures.detectTransformGestures
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Close
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties

/** Proporció de la caixa d'una imatge al xat: la seva pròpia, però mai més
 *  alta que 4:5. Una imatge vertical es veu sencera dins la caixa (Fit) en
 *  comptes de retallar-se. Pura: testejable. */
fun chatImageBoxRatio(width: Int, height: Int): Float {
    if (width <= 0 || height <= 0) return 4f / 3f
    return (width.toFloat() / height).coerceIn(0.8f, 3f)
}

/** Peu d'una imatge: el text alternatiu i el domini d'on surt, perquè se
 *  sàpiga què és i de qui és. Pura: testejable. */
fun chatImageCaption(alt: String, url: String): String {
    val host = url.substringAfter("://", "").substringBefore('/').substringBefore('?').removePrefix("www.")
    val title = alt.trim()
    return when {
        title.isNotEmpty() && host.isNotEmpty() -> "$title · $host"
        title.isNotEmpty() -> title
        else -> host
    }
}

/** La imatge carregada dins la bombolla: sencera, emmarcada i amb peu. Un toc
 *  l'obre a pantalla completa. */
@Composable
internal fun ChatImageFrame(bmp: ImageBitmap, url: String, alt: String, lang: String) {
    var full by remember { mutableStateOf(false) }
    val description = tr(lang, "app.vents.imageContent", alt.ifBlank { url })
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Box(
            Modifier.fillMaxWidth()
                .aspectRatio(chatImageBoxRatio(bmp.width, bmp.height))
                .clip(RoundedCornerShape(12.dp))
                .background(Color.Black.copy(alpha = 0.25f))
                .clickable(onClickLabel = tr(lang, "app.vents.imageOpen")) { full = true },
            contentAlignment = Alignment.Center,
        ) {
            Image(bitmap = bmp, contentDescription = description, contentScale = ContentScale.Fit, modifier = Modifier.fillMaxSize())
        }
        val caption = chatImageCaption(alt, url)
        if (caption.isNotEmpty()) {
            Text(caption, fontSize = 11.sp, maxLines = 2, overflow = TextOverflow.Ellipsis,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f))
        }
    }
    if (full) ChatImageViewer(bmp, url, alt, lang) { full = false }
}

/** Pantalla completa: fons fosc, pessic per ampliar, doble toc per tornar a
 *  la mida, i un botó per obrir la imatge al seu lloc d'origen. */
@Composable
private fun ChatImageViewer(bmp: ImageBitmap, url: String, alt: String, lang: String, onClose: () -> Unit) {
    val uri = LocalUriHandler.current
    var scale by remember { mutableStateOf(1f) }
    var offset by remember { mutableStateOf(Offset.Zero) }
    Dialog(onDismissRequest = onClose, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Box(Modifier.fillMaxSize().background(Color.Black)) {
            Image(
                bitmap = bmp,
                contentDescription = tr(lang, "app.vents.imageContent", alt.ifBlank { url }),
                contentScale = ContentScale.Fit,
                modifier = Modifier.fillMaxSize()
                    .pointerInput(Unit) {
                        detectTransformGestures { _, pan, zoom, _ ->
                            scale = (scale * zoom).coerceIn(1f, 5f)
                            offset = if (scale == 1f) Offset.Zero else offset + pan
                        }
                    }
                    .pointerInput(Unit) {
                        detectTapGestures(onDoubleTap = { scale = 1f; offset = Offset.Zero })
                    }
                    .graphicsLayer(scaleX = scale, scaleY = scale, translationX = offset.x, translationY = offset.y),
            )
            IconButton(onClick = onClose, modifier = Modifier.align(Alignment.TopEnd).statusBarsPadding().padding(8.dp)) {
                Icon(Icons.Rounded.Close, contentDescription = tr(lang, "app.vents.imageClose"), tint = Color.White)
            }
            Column(
                Modifier.align(Alignment.BottomCenter).fillMaxWidth()
                    .background(Color.Black.copy(alpha = 0.55f)).navigationBarsPadding()
                    .padding(horizontal = 16.dp, vertical = 12.dp),
                verticalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                val caption = chatImageCaption(alt, url)
                if (caption.isNotEmpty()) Text(caption, color = Color.White, fontSize = 13.sp)
                TextButton(onClick = { try { uri.openUri(url) } catch (_: Exception) { } }, contentPadding = PaddingValues(0.dp)) {
                    Text(tr(lang, "app.vents.imageSource"), color = Color.White, fontSize = 13.sp)
                }
            }
        }
    }
}
