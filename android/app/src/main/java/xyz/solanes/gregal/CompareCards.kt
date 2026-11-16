package xyz.solanes.gregal

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.json.JSONArray
import org.json.JSONObject

/*
 * Dues targetes noves, com a l'escriptori (md.js):
 *  - ```gregal-compare: 2-3 coses una al costat de l'altra, cada una amb la
 *    seva foto, els mateixos trets en files i el veredicte;
 *  - ```gregal-facts: 2-6 xifres clau grans, amb què volen dir.
 * Els vents les aprenen a companion.CardsGuide.
 */

data class CompareItem(val name: String, val image: String, val facts: List<Pair<String, String>>, val pick: Boolean)
data class CompareSpec(val title: String, val subtitle: String, val items: List<CompareItem>, val verdict: String,
    val sources: List<Pair<String, String>>, val actions: List<CardAction>) {
    /** Els trets en l'ordre en què surten, un cop cada un (màx. 8). */
    val keys: List<String> get() = items.flatMap { i -> i.facts.map { it.first } }.distinct().take(8)
    fun value(item: CompareItem, key: String): String = item.facts.firstOrNull { it.first == key }?.second.orEmpty()
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
        if (name.isEmpty()) null else CompareItem(name, cardImageUrl(x.optString("image")), compareFacts(x.opt("facts")), x.optBoolean("pick"))
    }.take(3)
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
    CardFrame("gregal-compare", "", spec.title, lang) {
        CardTitle(spec.title)
        if (spec.subtitle.isNotEmpty()) Text(spec.subtitle, color = Muted, fontSize = 13.sp)
        // Fotos i noms en columnes iguals.
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            spec.items.forEach { item ->
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    if (item.image.isNotEmpty()) CompareThumb(item.image, item.name, item.pick, link)
                    Text(item.name, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, lineHeight = 19.sp)
                    if (item.pick) Text(if (lang == "en") "The pick" else "La tria", color = link, fontSize = 11.sp, fontWeight = FontWeight.SemiBold,
                        modifier = Modifier.clip(RoundedCornerShape(50)).background(LocalPal.current.surfaceHi).padding(horizontal = 8.dp, vertical = 2.dp))
                }
            }
        }
        // Cada tret: el nom a sobre (petit) i els valors en columnes.
        Column {
            spec.keys.forEach { key ->
                HorizontalDivider(color = Line)
                Column(Modifier.fillMaxWidth().padding(vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(3.dp)) {
                    Text(key, color = Muted, fontSize = 12.sp)
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        spec.items.forEach { item ->
                            Text(spec.value(item, key).ifEmpty { "—" }, color = Ink, fontSize = 14.sp, lineHeight = 19.sp, modifier = Modifier.weight(1f))
                        }
                    }
                }
            }
        }
        if (spec.verdict.isNotEmpty()) Text(spec.verdict, color = Ink, fontSize = 14.sp, lineHeight = 20.sp,
            modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(LocalPal.current.surfaceHi).padding(12.dp).testTag("compare-verdict"))
    }
    CardActions(spec.actions, link)
}

@Composable
private fun CompareThumb(url: String, name: String, pick: Boolean, link: Color) {
    val (bmp, failed) = rememberChatImage(url)
    if (failed) return
    val shape = RoundedCornerShape(12.dp)
    Box(Modifier.fillMaxWidth().aspectRatio(4f / 3f).clip(shape).background(CodeDark)
        .then(if (pick) Modifier.border(2.dp, link, shape) else Modifier)
        .then(if (bmp != null) Modifier.clickable(role = Role.Image) { Viewer.show(ViewItem.Image(url, name)) } else Modifier)
        .testTag("compare-photo")) {
        if (bmp != null) Image(bmp, name, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
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
                        Text(f.value, color = link, fontSize = 24.sp, lineHeight = 27.sp, fontWeight = FontWeight.Bold)
                        Text(f.label, color = Ink, fontSize = 13.sp, lineHeight = 17.sp)
                        if (f.note.isNotEmpty()) Text(f.note, color = Muted, fontSize = 12.sp, lineHeight = 16.sp)
                    }
                }
                if (row.size == 1) Spacer(Modifier.weight(1f))
            }
        }
    }
    CardActions(spec.actions, link)
}
