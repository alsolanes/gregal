package xyz.solanes.gregal

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.spring
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.snapping.rememberSnapFlingBehavior
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.PagerState
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import org.json.JSONArray
import org.json.JSONObject

/*
 * Dues targetes noves, com a l'escriptori (md.js):
 *  - ```gregal-compare: 2-8 coses en fitxes que es llisquen, cada una amb la
 *    seva foto o logo, els mateixos trets alineats i el veredicte; si porten
 *    resum en punts (p. ex. programes electorals), una fitxa sencera per pàgina;
 *  - ```gregal-facts: 2-6 xifres clau grans, amb què volen dir.
 * Els vents les aprenen a companion.CardsGuide.
 */

/** [photos]: les fotos que el servidor ja ha resolt (màx. 2); [points]: el resum en punts. */
data class CompareItem(val name: String, val photos: List<String>, val facts: List<Pair<String, String>>, val pick: Boolean,
    val logo: String = "", val url: String = "", val points: List<String> = emptyList()) {
    val image: String get() = photos.firstOrNull().orEmpty()
}
data class CompareSpec(val title: String, val subtitle: String, val items: List<CompareItem>, val verdict: String,
    val sources: List<Pair<String, String>>, val actions: List<CardAction>) {
    /** Els trets en l'ordre en què surten, un cop cada un (màx. 8). */
    val keys: List<String> get() = items.flatMap { i -> i.facts.map { it.first } }.distinct().take(8)
    fun value(item: CompareItem, key: String): String = item.facts.firstOrNull { it.first == key }?.second.orEmpty()
    /** Amb resum en punts, una fitxa sencera per pàgina; si no, la fila compacta. */
    val paged: Boolean get() = items.any { it.points.isNotEmpty() }
}

data class FactTile(val value: String, val label: String, val note: String)
data class FactsSpec(val title: String, val facts: List<FactTile>, val sources: List<Pair<String, String>>, val actions: List<CardAction>)

private fun compareFacts(v: Any?): List<Pair<String, String>> = when (v) {
    is JSONObject -> v.keys().asSequence().map { it to v.optString(it).trim() }.filter { it.first.isNotBlank() }.toList()
    is JSONArray -> v.objects().mapNotNull { (it as? JSONObject)?.let { o -> o.optString("label").trim() to o.optString("value").trim() } }.filter { it.first.isNotBlank() }
    else -> emptyList()
}

/** Com md.js: com a mínim dues coses amb nom; si no, el bloc es queda com a codi. */
internal fun parseCompareSpec(raw: String): CompareSpec? {
    val o = runCatching { JSONObject(raw) }.getOrNull() ?: return null
    val title = o.optString("title").trim()
    val items = o.optJSONArray("items").objects().mapNotNull {
        val x = it as? JSONObject ?: return@mapNotNull null
        val name = x.optString("name").trim()
        if (name.isEmpty()) return@mapNotNull null
        val photos = (listOf(x.optString("image")) + x.optJSONArray("images").objects().map { it.toString() })
            .map(::cardImageUrl).filter { it.isNotEmpty() }.distinct().take(2)
        val points = x.optJSONArray("points").objects().map { it.toString().trim() }.filter { it.isNotEmpty() }.take(6)
        CompareItem(name, photos, compareFacts(x.opt("facts")), x.optBoolean("pick"), cardImageUrl(x.optString("logo")),
            x.optString("url").trim().takeIf { it.startsWith("https://") || it.startsWith("http://") }.orEmpty(), points)
    }.take(8)
    if (title.isEmpty() || items.size < 2) return null
    return CompareSpec(title, o.optString("subtitle").trim(), items, o.optString("verdict").trim(), o.sources(), o.cardActions())
}

internal fun parseFactsSpec(raw: String): FactsSpec? {
    val o = runCatching { JSONObject(raw) }.getOrNull() ?: return null
    val facts = o.optJSONArray("facts").objects().mapNotNull {
        val x = it as? JSONObject ?: return@mapNotNull null
        val v = x.optString("value").trim()
        val l = x.optString("label").trim()
        if (v.isEmpty() || l.isEmpty()) null else FactTile(v, l, x.optString("note").trim())
    }.take(6)
    if (facts.size < 2) return null
    return FactsSpec(o.optString("title").trim(), facts, o.sources(), o.cardActions())
}

@Composable
internal fun CompareCard(spec: CompareSpec, link: Color, lang: String) {
    val motion = windMotionEnabled()
    CardFrame("gregal-compare", "", spec.title, lang) {
        CardTitle(spec.title)
        if (spec.subtitle.isNotEmpty()) Text(spec.subtitle, color = Muted, fontSize = GregalSize.small)
        if (spec.paged) ComparePager(spec, link, lang, motion) else CompareRow(spec, link, lang, motion)
        if (spec.verdict.isNotEmpty()) Text(spec.verdict, color = Ink, fontSize = GregalSize.body, lineHeight = 20.sp,
            modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(LocalPal.current.surfaceHi).padding(12.dp).testTag("compare-verdict"))
        if (spec.sources.isNotEmpty()) Sources(spec.sources, lang)
    }
    CardActions(spec.actions, link)
}

private val nameStyle = TextStyle(fontSize = GregalSize.body, lineHeight = 19.sp, fontWeight = FontWeight.SemiBold)
private val valueStyle = TextStyle(fontSize = GregalSize.body, lineHeight = 19.sp)
private val itemShape = RoundedCornerShape(16.dp)

/** Fitxes en fila que s'aturen a l'inici de cada una; la següent treu el cap. */
@Composable
private fun CompareRow(spec: CompareSpec, link: Color, lang: String, motion: Boolean) {
    BoxWithConstraints(Modifier.fillMaxWidth()) {
        val gap = 10.dp
        val width = if (spec.items.size == 2) (maxWidth - gap) / 2 else minOf(maxWidth * 0.72f, 280.dp)
        // Els trets queden alineats d'una fitxa a l'altra: cada un fa les línies del valor més llarg.
        val measurer = rememberTextMeasurer()
        val inner = with(LocalDensity.current) { (width - 24.dp).roundToPx() }
        val lines = remember(spec, inner) {
            fun most(texts: List<String>, style: TextStyle) =
                texts.maxOf { measurer.measure(it, style, constraints = Constraints(maxWidth = inner)).lineCount }
            spec.keys.map { k -> most(spec.items.map { spec.value(it, k).ifEmpty { "—" } }, valueStyle) } to most(spec.items.map { it.name }, nameStyle)
        }
        val state = rememberLazyListState()
        LazyRow(Modifier.fillMaxWidth().testTag("compare-row"), state = state, flingBehavior = rememberSnapFlingBehavior(state),
            horizontalArrangement = Arrangement.spacedBy(gap)) {
            itemsIndexed(spec.items) { i, item ->
                Column(Modifier.width(width).springIn(i, motion).clip(itemShape).background(LocalPal.current.surfaceHi)
                    .border(if (item.pick) 2.dp else 1.dp, if (item.pick) link else Line, itemShape)
                    .padding(12.dp).testTag("compare-item"), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Box {
                        CompareVisual(item.name, item.image.ifEmpty { item.logo }, item.image.isEmpty(), link,
                            Modifier.fillMaxWidth().aspectRatio(4f / 3f))
                        if (item.pick) PickBadge(link, lang, Modifier.align(Alignment.TopStart).padding(6.dp))
                    }
                    Text(item.name, color = Ink, style = nameStyle, minLines = lines.second)
                    spec.keys.forEachIndexed { k, key ->
                        Text(key, color = Muted, fontSize = GregalSize.caption)
                        Text(spec.value(item, key).ifEmpty { "—" }, color = Ink, style = valueStyle, minLines = lines.first[k])
                    }
                }
            }
        }
    }
}

/** Una fitxa sencera per pàgina (p. ex. un partit i el seu programa): es llisca, i els punts diuen on ets. */
@Composable
private fun ComparePager(spec: CompareSpec, link: Color, lang: String, motion: Boolean) {
    val pager = rememberPagerState { spec.items.size }
    val uri = LocalUriHandler.current
    HorizontalPager(pager, Modifier.fillMaxWidth().springIn(0, motion).testTag("compare-pager"), pageSpacing = 12.dp,
        verticalAlignment = Alignment.Top) { page ->
        val item = spec.items[page]
        Column(Modifier.fillMaxWidth().clip(itemShape).background(LocalPal.current.surfaceHi)
            .border(if (item.pick) 2.dp else 1.dp, if (item.pick) link else Line, itemShape)
            .padding(14.dp).testTag("compare-page"), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            val visuals = (item.photos.map { it to false } + (item.logo to true)).filter { it.first.isNotEmpty() }.distinctBy { it.first }.take(2)
            if (visuals.isEmpty()) CompareVisual(item.name, "", false, link, Modifier.fillMaxWidth().height(120.dp))
            else Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                visuals.forEach { (u, logo) ->
                    CompareVisual(item.name, u, logo, link, Modifier.weight(1f).aspectRatio(if (visuals.size == 1) 16f / 9f else 1f))
                }
            }
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(item.name, color = Ink, fontSize = GregalSize.title, lineHeight = 23.sp, fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f, fill = false))
                if (item.pick) PickBadge(link, lang)
            }
            item.points.forEach { p ->
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text("•", color = link, fontSize = GregalSize.body, lineHeight = 21.sp, fontWeight = FontWeight.Bold)
                    Text(p, color = Ink, fontSize = GregalSize.body, lineHeight = 21.sp)
                }
            }
            spec.keys.forEach { key ->
                val v = spec.value(item, key)
                if (v.isNotEmpty()) Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text(key, color = Muted, fontSize = GregalSize.small, lineHeight = 19.sp, modifier = Modifier.weight(.4f))
                    Text(v, color = Ink, fontSize = GregalSize.body, lineHeight = 19.sp, modifier = Modifier.weight(.6f))
                }
            }
            if (item.url.isNotEmpty()) Text(item.url.substringAfter("//").substringBefore('/') + " ↗", color = link, fontSize = GregalSize.small,
                fontWeight = FontWeight.SemiBold, modifier = Modifier.clip(RoundedCornerShape(8.dp))
                    .clickable(role = Role.Button) { uri.openUri(item.url) }.padding(vertical = 6.dp).testTag("compare-link"))
        }
    }
    PagerDots(pager, link, motion, "compare-dot")
}

/** Els punts sota un pàger: diuen on ets i, tocats, hi porten (també el resum d'«Avui»). */
@Composable
internal fun PagerDots(pager: PagerState, color: Color, motion: Boolean, tag: String) {
    val scope = rememberCoroutineScope()
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center) {
        (0 until pager.pageCount).forEach { i ->
            val on = pager.currentPage == i
            Box(Modifier.size(28.dp).clip(CircleShape)
                .clickable(role = Role.Tab) { scope.launch { if (motion) pager.animateScrollToPage(i) else pager.scrollToPage(i) } }
                .semantics { selected = on; contentDescription = "${i + 1} / ${pager.pageCount}" }.testTag(tag),
                contentAlignment = Alignment.Center) {
                Box(Modifier.size(if (on) 9.dp else 7.dp).clip(CircleShape).background(if (on) color else Line))
            }
        }
    }
}

@Composable
private fun PickBadge(link: Color, lang: String, modifier: Modifier = Modifier) =
    Text(if (lang == "en") "The pick" else "La tria", color = link, fontSize = GregalSize.label, fontWeight = FontWeight.SemiBold,
        modifier = modifier.clip(RoundedCornerShape(50)).background(LocalPal.current.surfaceHi).padding(horizontal = 8.dp, vertical = 2.dp).testTag("compare-pick"))

/** La foto (retallada) o el logo (sencer, sobre blanc); si no n'hi ha o no baixa, la inicial. Un toc obre la foto gran. */
@Composable
private fun CompareVisual(name: String, url: String, logo: Boolean, link: Color, modifier: Modifier) {
    val (bmp, failed) = if (url.isEmpty()) null to true else rememberChatImage(url)
    Box(modifier.clip(RoundedCornerShape(12.dp))
        .background(if (bmp != null && logo) Color.White else if (failed) link.copy(alpha = .16f) else Superficie)
        .then(if (bmp != null && !logo) Modifier.clickable(role = Role.Image) { Viewer.show(ViewItem.Image(url, name)) } else Modifier)
        .testTag(if (bmp != null) "compare-photo" else "compare-visual"), contentAlignment = Alignment.Center) {
        if (bmp != null) Image(bmp, name, contentScale = if (logo) ContentScale.Fit else ContentScale.Crop,
            modifier = if (logo) Modifier.fillMaxSize().padding(14.dp) else Modifier.fillMaxSize())
        else if (failed) Text(name.trim().take(1).uppercase(), color = link, fontSize = 40.sp, fontWeight = FontWeight.Bold)
    }
}

/** Entra pujant, amb molla, una mica després de l'anterior. Sense moviment, quieta. */
@Composable
private fun Modifier.springIn(index: Int, motion: Boolean): Modifier {
    val p = remember { Animatable(if (motion) 0f else 1f) }
    if (motion) LaunchedEffect(p) {
        delay(index.coerceAtMost(4) * 80L)
        p.animateTo(1f, spring(dampingRatio = .6f, stiffness = 320f))
    }
    return graphicsLayer {
        val v = p.value
        translationY = (1f - v) * 26.dp.toPx()
        alpha = v.coerceIn(0f, 1f)
        scaleX = .96f + .04f * v
        scaleY = .96f + .04f * v
    }
}

@Composable
internal fun FactsCard(spec: FactsSpec, link: Color, lang: String) {
    CardFrame("gregal-facts", "", spec.title, lang) {
        if (spec.title.isNotEmpty()) CardTitle(spec.title)
        spec.facts.chunked(2).forEach { row ->
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                row.forEach { f ->
                    Column(Modifier.weight(1f).clip(RoundedCornerShape(14.dp)).background(LocalPal.current.surfaceHi).padding(horizontal = 14.dp, vertical = 12.dp)
                        .testTag("fact-tile"), verticalArrangement = Arrangement.spacedBy(3.dp)) {
                        Text(f.value, color = link, fontSize = GregalSize.display, lineHeight = 27.sp, fontWeight = FontWeight.Bold)
                        Text(f.label, color = Ink, fontSize = GregalSize.small, lineHeight = 17.sp)
                        if (f.note.isNotEmpty()) Text(f.note, color = Muted, fontSize = GregalSize.caption, lineHeight = 16.sp)
                    }
                }
                if (row.size == 1) Spacer(Modifier.weight(1f))
            }
        }
    }
    CardActions(spec.actions, link)
}
