package xyz.solanes.gregal

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.json.JSONArray
import org.json.JSONObject

/**
 * Targetes riques del xat, com a l'escriptori (md.js): recepta (```gregal-recipe),
 * pla en línia de temps (```gregal-plan), checklists que es poden marcar i
 * galeria de fotos. Els vents les aprenen a companion.CardsGuide.
 */
sealed interface RichCard {
    data class Recipe(val spec: RecipeSpec) : RichCard
    data class Plan(val spec: PlanSpec) : RichCard
    /** Checklist del markdown («- [ ] …»): cada element, marcat o no, i el text. */
    data class Checks(val items: List<Pair<Boolean, List<Seg>>>) : RichCard
    /** Dues o més fotos seguides. */
    data class Gallery(val images: List<Pair<String, String>>) : RichCard
}

data class RecipeIngredient(val name: String, val qty: String)
data class RecipeStep(val text: String, val minutes: Int = 0)
data class RecipeSpec(
    val title: String, val image: String = "", val time: String = "", val level: String = "", val servings: Int = 0,
    val ingredients: List<RecipeIngredient> = emptyList(), val steps: List<RecipeStep> = emptyList(), val tip: String = "",
    val sources: List<Pair<String, String>> = emptyList(),
)
data class PlanItem(val time: String, val title: String, val detail: String = "")
data class PlanSpec(
    val title: String, val kicker: String = "", val subtitle: String = "", val image: String = "",
    val items: List<PlanItem> = emptyList(), val totalLabel: String = "", val totalValue: String = "",
    val sources: List<Pair<String, String>> = emptyList(),
)

/** Una imatge de targeta que es pot ensenyar: web (http/https) o d'un vent. */
internal fun cardImageUrl(u: String): String = u.trim().takeIf { it.startsWith("https://") || it.startsWith("http://") || isWindImageUrl(it) }.orEmpty()

private fun JSONObject.sources(): List<Pair<String, String>> {
    val a = optJSONArray("sources") ?: return emptyList()
    return (0 until a.length()).mapNotNull { i ->
        val s = a.optJSONObject(i) ?: return@mapNotNull null
        val name = s.optString("name").trim()
        val url = s.optString("url").trim().takeIf { it.startsWith("http") }.orEmpty()
        if (name.isEmpty() && url.isEmpty()) null else name.ifEmpty { url.substringAfter("//").substringBefore('/') } to url
    }.take(6)
}

private fun JSONArray?.objects(): List<Any> = if (this == null) emptyList() else (0 until length()).mapNotNull { opt(it) }

/** Pura: el JSON d'una recepta; null si no té títol i alguna cosa a dins. */
internal fun parseRecipeSpec(raw: String): RecipeSpec? {
    val o = runCatching { JSONObject(raw) }.getOrNull() ?: return null
    val title = o.optString("title").trim()
    val ing = o.optJSONArray("ingredients").objects().mapNotNull {
        when (it) {
            is String -> it.trim().takeIf { s -> s.isNotEmpty() }?.let { s -> RecipeIngredient(s, "") }
            is JSONObject -> it.optString("name").trim().takeIf { s -> s.isNotEmpty() }?.let { s -> RecipeIngredient(s, it.optString("qty").trim()) }
            else -> null
        }
    }.take(30)
    val steps = o.optJSONArray("steps").objects().mapNotNull {
        when (it) {
            is String -> it.trim().takeIf { s -> s.isNotEmpty() }?.let { s -> RecipeStep(s) }
            is JSONObject -> it.optString("text").trim().takeIf { s -> s.isNotEmpty() }?.let { s -> RecipeStep(s, it.optInt("minutes").coerceIn(0, 999)) }
            else -> null
        }
    }.take(20)
    if (title.isEmpty() || (ing.isEmpty() && steps.isEmpty())) return null
    return RecipeSpec(title, cardImageUrl(o.optString("image")), o.optString("time").trim(), o.optString("level").trim(),
        o.optInt("servings").coerceIn(0, 99), ing, steps, o.optString("tip").trim(), o.sources())
}

/** Pura: el JSON d'un pla; null si no té títol i passos. */
internal fun parsePlanSpec(raw: String): PlanSpec? {
    val o = runCatching { JSONObject(raw) }.getOrNull() ?: return null
    val title = o.optString("title").trim()
    val items = o.optJSONArray("items").objects().mapNotNull {
        val x = it as? JSONObject ?: return@mapNotNull null
        val t = x.optString("title").ifBlank { x.optString("text") }.trim()
        if (t.isEmpty()) null else PlanItem(x.optString("time").trim(), t, x.optString("detail").trim())
    }.take(12)
    if (title.isEmpty() || items.isEmpty()) return null
    val total = o.opt("total")
    val (tl, tv) = when (total) {
        is JSONObject -> total.optString("label").trim() to total.optString("value").trim()
        is String, is Number -> "" to total.toString().trim()
        else -> "" to ""
    }
    return PlanSpec(title, o.optString("kicker").trim(), o.optString("subtitle").trim(), cardImageUrl(o.optString("image")),
        items, tl, tv, o.sources())
}

private val fractions = mapOf('¼' to .25, '½' to .5, '¾' to .75, '⅓' to 1.0 / 3, '⅔' to 2.0 / 3)
private val qtyRe = Regex("(\\d+)/(\\d+)|(\\d+(?:[.,]\\d+)?)(?:\\s*([¼½¾⅓⅔]))?|([¼½¾⅓⅔])")

/** Pura, com scaleQty de md.js: escala el primer número de la quantitat. */
internal fun scaleQty(qty: String, from: Int, to: Int): String {
    if (from <= 0 || to <= 0 || from == to) return qty
    val m = qtyRe.find(qty) ?: return qty
    // «1/2» és mitja, no un 1: la fracció amb barra va primer.
    val num = m.groups[3]?.value
    val frac = (m.groups[4] ?: m.groups[5])?.value?.firstOrNull()
    val n = if (m.groups[1] != null) {
        val d = m.groups[2]!!.value.toDouble()
        if (d == 0.0) 0.0 else m.groups[1]!!.value.toDouble() / d
    } else (num?.replace(',', '.')?.toDoubleOrNull() ?: 0.0) + (frac?.let { fractions[it] } ?: 0.0)
    if (n == 0.0) return qty
    var v = n * to / from
    v = if (v >= 10) Math.round(v).toDouble() else Math.round(v * 4) / 4.0
    var s = if (v == Math.floor(v)) v.toLong().toString() else "%.2f".format(java.util.Locale.ROOT, v).trimEnd('0').trimEnd('.')
    if (num?.contains(',') == true) s = s.replace('.', ',')
    return qty.substring(0, m.range.first) + s + qty.substring(m.range.last + 1)
}

/** Clau estable d'una casella (títol de la targeta + element) per recordar-la. */
internal fun checkKey(context: String, item: String): String = "c" + (context + "\u0001" + item).hashCode().toUInt().toString(36)

@Composable
private fun rememberCheck(context: String, item: String): MutableState<Boolean> {
    val ctx = LocalContext.current
    val prefs = remember { ctx.getSharedPreferences("gregal_checks", android.content.Context.MODE_PRIVATE) }
    val key = checkKey(context, item)
    val state = remember(key) { mutableStateOf(prefs.getBoolean(key, false)) }
    return remember(key) {
        object : MutableState<Boolean> by state {
            override var value: Boolean
                get() = state.value
                set(v) { state.value = v; prefs.edit().apply { if (v) putBoolean(key, true) else remove(key) }.apply() }
        }
    }
}

@Composable
internal fun RichCardBlock(card: RichCard, accent: Color, lang: String) {
    val link = readableLink(accent, LocalPal.current.dark)
    when (card) {
        is RichCard.Recipe -> RecipeCard(card.spec, link, lang)
        is RichCard.Plan -> PlanCard(card.spec, link, lang)
        is RichCard.Checks -> ChecksBlock(card.items, link)
        is RichCard.Gallery -> GalleryBlock(card.images, lang)
    }
}

private val cardShape = RoundedCornerShape(20.dp)

@Composable
private fun CardFrame(tag: String, image: String, title: String, lang: String, content: @Composable ColumnScope.() -> Unit) {
    Column(Modifier.fillMaxWidth().padding(vertical = 6.dp).clip(cardShape).background(Superficie).border(1.dp, Line, cardShape).testTag(tag)) {
        if (image.isNotEmpty()) CardHero(image, title, lang)
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp), content = content)
    }
}

/** La foto de dalt de la targeta, a tota l'amplada; un toc l'obre gran. Si
 *  no es pot baixar, no hi és (millor res que un forat). */
@Composable
private fun CardHero(url: String, alt: String, lang: String) {
    val (bmp, failed) = rememberChatImage(url)
    var full by remember { mutableStateOf(false) }
    if (failed) return
    Box(Modifier.fillMaxWidth().aspectRatio(16f / 9f).background(CodeDark)
        .then(if (bmp != null) Modifier.clickable(role = Role.Image) { full = true } else Modifier)
        .testTag("card-hero")) {
        if (bmp != null) Image(bmp, alt, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
    }
    if (full && bmp != null) ChatImageViewer(bmp, url, alt, lang) { full = false }
}

/** La foto de la targeta d'opcions (ResultBlocks.kt). */
@Composable
internal fun ResultHero(url: String, alt: String, lang: String) = CardHero(url, alt, lang)

@Composable
private fun CardTitle(text: String) =
    Text(text, color = Ink, fontSize = 21.sp, lineHeight = 25.sp, fontWeight = FontWeight.Bold)

@Composable
private fun SubHead(text: String, end: @Composable () -> Unit = {}) {
    HorizontalDivider(color = Line)
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Text(text, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
        end()
    }
}

@Composable
private fun Tick(on: Boolean, link: Color, modifier: Modifier = Modifier) {
    Box(modifier.size(20.dp).clip(RoundedCornerShape(6.dp))
        .background(if (on) link else Color.Transparent)
        .border(1.5.dp, if (on) link else Line, RoundedCornerShape(6.dp)),
        contentAlignment = Alignment.Center) {
        if (on) Text("✓", color = Superficie, fontSize = 13.sp, fontWeight = FontWeight.Bold)
    }
}

@Composable
private fun RecipeCard(spec: RecipeSpec, link: Color, lang: String) {
    var serves by remember(spec) { mutableIntStateOf(spec.servings) }
    CardFrame("gregal-recipe", spec.image, spec.title, lang) {
        CardTitle(spec.title)
        val meta = listOfNotNull(
            spec.time.takeIf { it.isNotEmpty() }?.let { "⏱ $it" },
            spec.level.takeIf { it.isNotEmpty() }?.let { "👩‍🍳 $it" },
            if (spec.servings > 0) "🍽 " + tr(lang, "app.card.serves").replace("{n}", serves.toString()) else null,
        )
        if (meta.isNotEmpty()) Text(meta.joinToString("   "), color = Muted, fontSize = 13.sp)
        if (spec.ingredients.isNotEmpty()) {
            SubHead(tr(lang, "app.card.ingredients")) {
                if (spec.servings > 0) Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    StepButton("−", link, tr(lang, "app.card.less")) { if (serves > 1) serves-- }
                    Text(serves.toString(), color = Ink, fontWeight = FontWeight.Bold, modifier = Modifier.testTag("recipe-serves"))
                    StepButton("+", link, tr(lang, "app.card.more")) { if (serves < 99) serves++ }
                }
            }
            Column {
                spec.ingredients.forEachIndexed { i, ing ->
                    if (i > 0) HorizontalDivider(color = Line)
                    var on by rememberCheck(spec.title, ing.name)
                    Row(Modifier.fillMaxWidth().clickable(role = Role.Checkbox) { on = !on }
                        .semantics { stateDescription = if (on) "✓" else "" }
                        .heightIn(min = 44.dp).padding(vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        Tick(on, link)
                        Text(ing.name, color = if (on) Muted else Ink, fontSize = 14.sp,
                            textDecoration = if (on) TextDecoration.LineThrough else null, modifier = Modifier.weight(1f))
                        if (ing.qty.isNotEmpty()) Text(scaleQty(ing.qty, spec.servings, serves), color = Muted, fontSize = 14.sp,
                            modifier = Modifier.testTag("recipe-qty"))
                    }
                }
            }
        }
        if (spec.steps.isNotEmpty()) {
            if (spec.ingredients.isNotEmpty()) SubHead(tr(lang, "app.card.steps"))
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                spec.steps.forEachIndexed { i, st ->
                    Row(verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        Box(Modifier.size(28.dp).clip(CircleShape).background(link.copy(alpha = .16f)), contentAlignment = Alignment.Center) {
                            Text((i + 1).toString(), color = link, fontSize = 13.sp, fontWeight = FontWeight.Bold)
                        }
                        Text(st.text, color = Ink, fontSize = 14.sp, lineHeight = 20.sp, modifier = Modifier.weight(1f).padding(top = 3.dp))
                        if (st.minutes > 0) Text("${st.minutes} min ⏱", color = link, fontSize = 12.sp, fontWeight = FontWeight.SemiBold,
                            modifier = Modifier.clip(RoundedCornerShape(50)).background(link.copy(alpha = .14f)).padding(horizontal = 9.dp, vertical = 3.dp))
                    }
                }
            }
        }
        if (spec.tip.isNotEmpty()) Tip(spec.tip, link)
        if (spec.sources.isNotEmpty()) Sources(spec.sources, lang)
    }
}

@Composable
private fun StepButton(label: String, link: Color, description: String, onClick: () -> Unit) {
    Box(Modifier.size(36.dp).clip(RoundedCornerShape(10.dp)).background(link.copy(alpha = .16f))
        .clickable(role = Role.Button, onClick = onClick).semantics { contentDescription = description },
        contentAlignment = Alignment.Center) {
        Text(label, color = link, fontSize = 18.sp, fontWeight = FontWeight.Bold)
    }
}

@Composable
private fun Tip(text: String, link: Color) {
    Row(Modifier.height(IntrinsicSize.Min)) {
        Box(Modifier.width(3.dp).fillMaxHeight().background(link))
        Text(renderSegs(parseInline(text), CodeDark, link = link).text, color = Ink, fontSize = 14.sp, fontStyle = FontStyle.Italic,
            modifier = Modifier.padding(start = 12.dp))
    }
}

@Composable
private fun Sources(sources: List<Pair<String, String>>, lang: String) {
    val uri = LocalUriHandler.current
    HorizontalDivider(color = Line)
    Row(verticalAlignment = Alignment.CenterVertically) {
        Row(horizontalArrangement = Arrangement.spacedBy((-6).dp)) {
            sources.forEachIndexed { i, (_, url) ->
                Box(Modifier.size(16.dp).clip(CircleShape).background(Superficie).padding(2.dp).clip(CircleShape)
                    .background(listOf(Color(0xFFC9152B), Color(0xFF2E7D32), Color(0xFF1565C0), Color(0xFFEF6C00), Color(0xFF6A1B9A), Color(0xFF00838F))[i % 6])
                    .then(if (url.isNotEmpty()) Modifier.clickable(role = Role.Button) { uri.openUri(url) } else Modifier))
            }
        }
        Text("  " + resultFootText(lang, resultSourceNames(sources).size, 0L), color = Muted, fontSize = 12.sp)
    }
}

@Composable
private fun PlanCard(spec: PlanSpec, link: Color, lang: String) {
    CardFrame("gregal-plan", spec.image, spec.title, lang) {
        if (spec.kicker.isNotEmpty()) Text(spec.kicker.uppercase(), color = link, fontSize = 11.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.3.sp)
        CardTitle(spec.title)
        if (spec.subtitle.isNotEmpty()) Text(spec.subtitle, color = Muted, fontSize = 13.sp)
        Column {
            spec.items.forEachIndexed { i, it ->
                Row(Modifier.height(IntrinsicSize.Min)) {
                    // El punt i la línia que el lliga amb el següent.
                    Box(Modifier.width(10.dp).fillMaxHeight()) {
                        Box(Modifier.padding(top = 6.dp).size(10.dp).clip(CircleShape).background(link))
                        if (i < spec.items.lastIndex) Box(Modifier.padding(start = 4.5.dp, top = 16.dp).width(1.dp).fillMaxHeight().background(link.copy(alpha = .5f)))
                    }
                    Spacer(Modifier.width(12.dp))
                    if (it.time.isNotEmpty()) Text(it.time, color = link, fontWeight = FontWeight.Bold, fontSize = 15.sp, modifier = Modifier.widthIn(min = 52.dp))
                    if (it.time.isNotEmpty()) Spacer(Modifier.width(12.dp))
                    Column(Modifier.weight(1f).padding(bottom = if (i < spec.items.lastIndex) 14.dp else 0.dp)) {
                        Text(it.title, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                        if (it.detail.isNotEmpty()) Text(it.detail, color = Muted, fontSize = 13.sp)
                    }
                }
            }
        }
        if (spec.totalValue.isNotEmpty()) {
            HorizontalDivider(color = Line)
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(spec.totalLabel.ifEmpty { tr(lang, "app.card.total") }, color = Ink, fontSize = 14.sp, modifier = Modifier.weight(1f))
                Text(spec.totalValue, color = Ink, fontSize = 18.sp, fontWeight = FontWeight.Bold)
            }
        }
        if (spec.sources.isNotEmpty()) Sources(spec.sources, lang)
    }
}

@Composable
private fun ChecksBlock(items: List<Pair<Boolean, List<Seg>>>, link: Color) {
    val codeBg = CodeDark
    val context = items.joinToString("|") { renderSegs(it.second, codeBg).text }.take(80)
    Column(Modifier.fillMaxWidth().testTag("md-checks")) {
        items.forEach { (done, segs) ->
            val text = renderSegs(segs, codeBg, link = link)
            val remembered = rememberCheck(context, text.text)
            val on = done || remembered.value
            Row(Modifier.fillMaxWidth().clickable(role = Role.Checkbox, enabled = !done) { remembered.value = !remembered.value }
                .semantics { stateDescription = if (on) "✓" else "" }
                .heightIn(min = 40.dp).padding(vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                Tick(on, link)
                Text(text, color = if (on) Muted else Ink, fontSize = 15.sp, lineHeight = 21.sp,
                    textDecoration = if (on) TextDecoration.LineThrough else null)
            }
        }
    }
}

/** Dues fotos, l'una al costat de l'altra; tres, una gran i dues petites. */
@Composable
private fun GalleryBlock(images: List<Pair<String, String>>, lang: String) {
    val shown = images.take(3)
    Row(Modifier.fillMaxWidth().height(if (shown.size == 3) 220.dp else 160.dp).testTag("md-gallery"),
        horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        GalleryTile(shown[0], lang, Modifier.weight(if (shown.size == 3) 2f else 1f).fillMaxHeight())
        if (shown.size == 2) GalleryTile(shown[1], lang, Modifier.weight(1f).fillMaxHeight())
        if (shown.size == 3) Column(Modifier.weight(1f).fillMaxHeight(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            GalleryTile(shown[1], lang, Modifier.weight(1f).fillMaxWidth())
            GalleryTile(shown[2], lang, Modifier.weight(1f).fillMaxWidth())
        }
    }
}

@Composable
private fun GalleryTile(image: Pair<String, String>, lang: String, modifier: Modifier) {
    val (alt, url) = image
    val (bmp, _) = rememberChatImage(url)
    var full by remember { mutableStateOf(false) }
    Box(modifier.clip(RoundedCornerShape(14.dp)).background(CodeDark)
        .then(if (bmp != null) Modifier.clickable(role = Role.Image) { full = true } else Modifier)) {
        if (bmp != null) Image(bmp, alt, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
    }
    if (full && bmp != null) ChatImageViewer(bmp, url, alt, lang) { full = false }
}
