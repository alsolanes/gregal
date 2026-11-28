package xyz.solanes.gregal

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.gestures.detectTransformGestures
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.KeyboardArrowLeft
import androidx.compose.material.icons.automirrored.rounded.KeyboardArrowRight
import androidx.compose.material.icons.rounded.Close
import androidx.compose.material.icons.rounded.ContentCopy
import androidx.compose.material.icons.rounded.OpenInBrowser
import androidx.compose.material.icons.rounded.Share
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.DialogProperties
import kotlinx.coroutines.launch

/*
 * Visor a pantalla completa, com els artefactes de Claude o el canvas de
 * ChatGPT al mòbil: tot el que un vent ensenya al xat (fotos, webs, blocs
 * html/svg i gràfics) es toca i s'obre gran. Dins el visor es passa d'un a
 * l'altre de la mateixa conversa (lliscant o amb les fletxes), i es pot
 * compartir, copiar el codi o obrir al navegador.
 */

sealed interface ViewItem {
    data class Image(val url: String, val alt: String) : ViewItem
    data class Html(val kind: String, val code: String) : ViewItem
    data class Web(val spec: WebSpec) : ViewItem
    data class Chart(val spec: ChartSpec) : ViewItem
}

object ViewerLogic {
    /** El que es pot obrir gran d'una resposta, en ordre. */
    fun itemsOf(blocks: List<Rich>): List<ViewItem> = buildList {
        for (b in blocks) when (b) {
            is Rich.Image -> add(ViewItem.Image(b.url, b.alt))
            is Rich.Code -> WebPreviewLogic.kind(b.lang, b.code).takeIf { it.isNotEmpty() }?.let { add(ViewItem.Html(it, b.code)) }
            is Rich.Chart -> add(ViewItem.Chart(b.spec))
            is Rich.Card -> when (val c = b.card) {
                is RichCard.Web -> add(ViewItem.Web(c.spec))
                is RichCard.Gallery -> c.images.forEach { (alt, url) -> add(ViewItem.Image(url, alt)) }
                is RichCard.Compare -> c.spec.items.filter { it.image.isNotEmpty() }.forEach { add(ViewItem.Image(it.image, it.name)) }
                else -> {}
            }
            else -> {}
        }
    }

    /** Tota la conversa (les respostes acabades), sense repetits. */
    fun itemsOfTexts(texts: List<String>): List<ViewItem> = texts.flatMap { itemsOf(richBlocks(it)) }.distinct()

    /** On és l'element dins la llista; si no hi és, es mostra sol. */
    fun place(items: List<ViewItem>, item: ViewItem): Pair<List<ViewItem>, Int> {
        val i = items.indexOf(item)
        return if (i >= 0) items to i else listOf(item) to 0
    }

    fun title(item: ViewItem, lang: String): String = when (item) {
        is ViewItem.Image -> item.alt.ifBlank { if (lang == "en") "Image" else "Imatge" }
        is ViewItem.Html -> if (item.kind == "svg") "SVG" else (if (lang == "en") "Web page" else "Pàgina web")
        is ViewItem.Web -> item.spec.title
        is ViewItem.Chart -> item.spec.title.ifBlank { if (lang == "en") "Chart" else "Gràfic" }
    }

    /** L'adreça que es pot obrir fora o compartir (si n'hi ha). */
    fun link(item: ViewItem, base: String): String = when (item) {
        is ViewItem.Image -> item.url.takeIf { it.startsWith("http://") || it.startsWith("https://") }.orEmpty()
        is ViewItem.Web -> base.trimEnd('/') + item.spec.url
        else -> ""
    }

    /** Les pàgines amb WebView o amb zoom es queden el gest: es canvia amb les fletxes. */
    fun swipes(item: ViewItem): Boolean = item is ViewItem.Chart
}

/** L'estat del visor, un per app (MainActivity en dibuixa la capa). */
object Viewer {
    /** L'idioma de l'app (MainActivity el posa): per als botons sense paràmetre. */
    var lang by mutableStateOf("ca")
    var items by mutableStateOf<List<ViewItem>>(emptyList())
    var open by mutableStateOf<Pair<List<ViewItem>, Int>?>(null)
    fun show(item: ViewItem) { open = ViewerLogic.place(items, item) }
    fun close() { open = null }
}

/** Dins el visor no es tornen a posar botons d'«amplia». */
val LocalInViewer = staticCompositionLocalOf { false }

/** La pantalla de xat diu què hi ha a la conversa (per passar d'un a l'altre). */
@Composable
fun ViewerItemsFrom(texts: List<String>) {
    val items = remember(texts) { ViewerLogic.itemsOfTexts(texts) }
    SideEffect { Viewer.items = items }
}

/** Botó petit per obrir en gran (sobre una targeta). */
@Composable
internal fun ExpandChip(lang: String, modifier: Modifier = Modifier, onClick: () -> Unit) {
    Text(if (lang == "en") "⤢ Expand" else "⤢ Amplia", color = Muted, fontSize = GregalSize.caption,
        modifier = modifier.clip(RoundedCornerShape(50)).clickable(onClick = onClick)
            .padding(horizontal = 10.dp, vertical = 4.dp).testTag("viewer-expand"))
}

@Composable
fun ViewerOverlay(lang: String) {
    SideEffect { Viewer.lang = lang }
    val open = Viewer.open ?: return
    val (items, start) = open
    val ctx = LocalContext.current
    val base = remember { WebPreviewLogic.serverBase(ctx) }
    val pager = rememberPagerState(initialPage = start) { items.size }
    val scope = rememberCoroutineScope()
    val item = items[pager.currentPage.coerceIn(0, items.lastIndex)]
    var showCode by remember(item) { mutableStateOf(false) }
    GregalDialog(onDismissRequest = { Viewer.close() }, properties = DialogProperties(usePlatformDefaultWidth = false, decorFitsSystemWindows = false)) {
        CompositionLocalProvider(LocalInViewer provides true) {
            Column(Modifier.fillMaxSize().background(DeepTop).testTag("viewer")) {
                Row(Modifier.fillMaxWidth().statusBarsPadding().padding(horizontal = 4.dp, vertical = 4.dp),
                    verticalAlignment = Alignment.CenterVertically) {
                    IconButton(onClick = { Viewer.close() }, modifier = Modifier.testTag("viewer-close")) {
                        Icon(Icons.Rounded.Close, contentDescription = if (lang == "en") "Close" else "Tanca", tint = Ink)
                    }
                    Column(Modifier.weight(1f)) {
                        Text(ViewerLogic.title(item, lang), color = Ink, fontSize = GregalSize.large, fontWeight = FontWeight.SemiBold,
                            maxLines = 1, overflow = TextOverflow.Ellipsis)
                        if (items.size > 1) Text("${pager.currentPage + 1} / ${items.size}", color = Muted, fontSize = GregalSize.caption,
                            modifier = Modifier.testTag("viewer-count"))
                    }
                    if (item is ViewItem.Html) {
                        TextButton(onClick = { showCode = !showCode }) {
                            Text(if (showCode) (if (lang == "en") "Preview" else "Vista") else (if (lang == "en") "Code" else "Codi"), color = Sea)
                        }
                        IconButton(onClick = { copy(ctx, item.code) }) {
                            Icon(Icons.Rounded.ContentCopy, contentDescription = if (lang == "en") "Copy code" else "Copia el codi", tint = Ink)
                        }
                    }
                    val link = ViewerLogic.link(item, base)
                    if (link.isNotEmpty()) {
                        IconButton(onClick = { runCatching { ctx.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(link)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) } }) {
                            Icon(Icons.Rounded.OpenInBrowser, contentDescription = if (lang == "en") "Open in browser" else "Obre al navegador", tint = Ink)
                        }
                    }
                    val shareText = link.ifEmpty { (item as? ViewItem.Html)?.code.orEmpty() }
                    if (shareText.isNotEmpty()) {
                        IconButton(onClick = { share(ctx, ViewerLogic.title(item, lang), shareText) }) {
                            Icon(Icons.Rounded.Share, contentDescription = if (lang == "en") "Share" else "Comparteix", tint = Ink)
                        }
                    }
                }
                Box(Modifier.weight(1f).fillMaxWidth()) {
                    HorizontalPager(pager, userScrollEnabled = ViewerLogic.swipes(item), modifier = Modifier.fillMaxSize(), key = { it }) { page ->
                        ViewerPage(items[page], lang, base, showCode && page == pager.currentPage)
                    }
                }
                if (items.size > 1) {
                    Row(Modifier.fillMaxWidth().navigationBarsPadding().padding(horizontal = 8.dp, vertical = 6.dp),
                        horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                        IconButton(onClick = { scope.launch { pager.animateScrollToPage(pager.currentPage - 1) } }, enabled = pager.currentPage > 0,
                            modifier = Modifier.testTag("viewer-prev")) {
                            Icon(Icons.AutoMirrored.Rounded.KeyboardArrowLeft, contentDescription = if (lang == "en") "Previous" else "Anterior", tint = Ink)
                        }
                        Text(if (lang == "en") "Everything shown in this chat" else "Tot el que s'ha ensenyat en aquesta conversa",
                            color = Muted, fontSize = GregalSize.caption)
                        IconButton(onClick = { scope.launch { pager.animateScrollToPage(pager.currentPage + 1) } }, enabled = pager.currentPage < items.lastIndex,
                            modifier = Modifier.testTag("viewer-next")) {
                            Icon(Icons.AutoMirrored.Rounded.KeyboardArrowRight, contentDescription = if (lang == "en") "Next" else "Següent", tint = Ink)
                        }
                    }
                } else Spacer(Modifier.navigationBarsPadding())
            }
        }
    }
}

@Composable
private fun ViewerPage(item: ViewItem, lang: String, base: String, showCode: Boolean) {
    when (item) {
        is ViewItem.Image -> ZoomableChatImage(item.url, item.alt, lang)
        is ViewItem.Web -> PreviewWebView(Modifier.fillMaxSize(), url = base.trimEnd('/') + item.spec.url, allowStorage = true)
        is ViewItem.Html -> if (showCode) {
            Box(Modifier.fillMaxSize().background(CodeDark).verticalScroll(rememberScrollState()).horizontalScroll(rememberScrollState())) {
                Text(item.code, color = Ink, fontSize = GregalSize.caption, fontFamily = FontFamily.Monospace, softWrap = false, modifier = Modifier.padding(14.dp))
            }
        } else {
            val doc = remember(item) { WebPreviewLogic.srcdoc(item.kind, item.code, fill = true) }
            PreviewWebView(Modifier.fillMaxSize(), html = doc)
        }
        is ViewItem.Chart -> Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp),
            verticalArrangement = Arrangement.Center) {
            ChartBlock(item.spec, Sea, LocalPal.current.dark, height = 340.dp)
        }
    }
}

/** Foto a tota la pantalla: pessic per ampliar, doble toc per tornar. */
@Composable
private fun ZoomableChatImage(url: String, alt: String, lang: String) {
    val (bmp, failed) = rememberChatImage(url)
    var scale by remember(url) { mutableStateOf(1f) }
    var offset by remember(url) { mutableStateOf(Offset.Zero) }
    Box(Modifier.fillMaxSize().background(Color.Black), contentAlignment = Alignment.Center) {
        when {
            bmp != null -> Image(
                bitmap = bmp,
                contentDescription = tr(lang, "app.vents.imageContent", alt.ifBlank { url }),
                contentScale = ContentScale.Fit,
                modifier = Modifier.fillMaxSize()
                    .pointerInput(url) {
                        detectTransformGestures { _, pan, zoom, _ ->
                            scale = (scale * zoom).coerceIn(1f, 5f)
                            offset = if (scale == 1f) Offset.Zero else offset + pan
                        }
                    }
                    .pointerInput(url) { detectTapGestures(onDoubleTap = { scale = 1f; offset = Offset.Zero }) }
                    .graphicsLayer(scaleX = scale, scaleY = scale, translationX = offset.x, translationY = offset.y),
            )
            failed -> Text(alt.ifBlank { url }, color = Color.White)
            else -> CircularProgressIndicator(Modifier.size(28.dp), color = Sea, strokeWidth = 2.dp)
        }
        val caption = chatImageCaption(alt, url)
        if (caption.isNotEmpty()) Text(caption, color = Color.White, fontSize = GregalSize.small,
            modifier = Modifier.align(Alignment.BottomCenter).fillMaxWidth().background(Color.Black.copy(alpha = 0.55f)).padding(12.dp))
    }
}

private fun copy(ctx: Context, text: String) {
    (ctx.getSystemService(Context.CLIPBOARD_SERVICE) as? ClipboardManager)?.setPrimaryClip(ClipData.newPlainText("code", text))
}

private fun share(ctx: Context, title: String, text: String) {
    val send = Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_SUBJECT, title).putExtra(Intent.EXTRA_TEXT, text)
    runCatching { ctx.startActivity(Intent.createChooser(send, title).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
}
