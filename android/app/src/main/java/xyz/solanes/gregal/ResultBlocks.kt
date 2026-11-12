package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.json.JSONObject

/**
 * Targeta de resultat (direcció visual §4): quan Gregal torna opcions o una
 * comparació, envia un bloc ```gregal-result {json}``` i aquí es dibuixa com una
 * targeta —títol, fins a 5 files amb una de «recomanada», les fonts com a punts
 * i «Veure tots»— en lloc d'un mur de text o d'una taula tallada.
 *
 * {"title","subtitle","icon","rows":[{"lead","text","value","tag","pick":true,"url"}],
 *  "sources":[{"name","url"}],"more":{"label","url"}}
 */
data class ResultRow(val lead: String, val text: String, val value: String, val tag: String = "",
    val pick: Boolean = false, val url: String = "")
data class ResultSpec(val title: String, val subtitle: String = "", val icon: String = "",
    val rows: List<ResultRow> = emptyList(), val sources: List<Pair<String, String>> = emptyList(),
    val moreLabel: String = "", val moreUrl: String = "")

/** Pura: llegeix el JSON del bloc; null si no és una targeta vàlida (llavors surt com a codi). */
internal fun parseResultSpec(raw: String): ResultSpec? {
    val o = runCatching { JSONObject(raw) }.getOrNull() ?: return null
    val title = o.optString("title").trim()
    val rowsJ = o.optJSONArray("rows")
    val rows = (0 until (rowsJ?.length() ?: 0)).mapNotNull { i ->
        val r = rowsJ!!.optJSONObject(i) ?: return@mapNotNull null
        val row = ResultRow(r.optString("lead").trim(), r.optString("text").trim(), r.optString("value").trim(),
            r.optString("tag").trim(), r.optBoolean("pick"), r.optString("url").trim())
        row.takeIf { it.lead.isNotEmpty() || it.text.isNotEmpty() }
    }.take(5)
    if (title.isEmpty() || rows.isEmpty()) return null
    val srcJ = o.optJSONArray("sources")
    val sources = (0 until (srcJ?.length() ?: 0)).mapNotNull { i ->
        val s = srcJ!!.optJSONObject(i) ?: return@mapNotNull null
        val name = s.optString("name").trim()
        val url = s.optString("url").trim()
        if (name.isEmpty() && url.isEmpty()) null else name.ifEmpty { url.substringAfter("//").substringBefore('/') } to url
    }.take(6)
    val more = o.optJSONObject("more")
    return ResultSpec(title, o.optString("subtitle").trim(), o.optString("icon").trim().take(4), rows, sources,
        more?.optString("label")?.trim().orEmpty(), more?.optString("url")?.trim().orEmpty())
}

private val SourceDots = listOf(Color(0xFFC9152B), Color(0xFF2E7D32), Color(0xFF1565C0), Color(0xFFEF6C00), Color(0xFF6A1B9A), Color(0xFF00838F))

@Composable
internal fun GregalResultCard(spec: ResultSpec, accent: Color) {
    val uri = LocalUriHandler.current
    val shape = RoundedCornerShape(18.dp)
    val ok = Color(0xFF1E9E6A)
    // L'accent tal qual (p. ex. el groc de Gregal) no es llegeix sobre blanc: el de l'enllaç llegible.
    val link = readableLink(accent, LocalPal.current.dark)
    Column(Modifier.fillMaxWidth().padding(vertical = 6.dp).clip(shape).background(Superficie).border(1.dp, Line, shape)
        .testTag("gregal-result")) {
        Row(Modifier.padding(start = 14.dp, end = 14.dp, top = 12.dp, bottom = 8.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            if (spec.icon.isNotEmpty()) Box(Modifier.size(30.dp).clip(RoundedCornerShape(10.dp)).background(accent.copy(alpha = .14f)),
                contentAlignment = Alignment.Center) { Text(spec.icon, fontSize = 15.sp) }
            Column(Modifier.weight(1f)) {
                Text(spec.title, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, maxLines = 2, overflow = TextOverflow.Ellipsis)
                if (spec.subtitle.isNotEmpty()) Text(spec.subtitle, color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        spec.rows.forEach { r ->
            HorizontalDivider(color = Line)
            Row(Modifier.fillMaxWidth().then(if (r.pick) Modifier.background(ok.copy(alpha = .09f)) else Modifier)
                .then(if (r.url.isNotEmpty()) Modifier.clickable(role = Role.Button) { uri.openUri(r.url) } else Modifier)
                .heightIn(min = 44.dp).padding(horizontal = 14.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                Column(Modifier.weight(1f)) {
                    if (r.lead.isNotEmpty()) Text(r.lead, color = Ink, fontSize = 14.sp, fontWeight = FontWeight.Bold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    if (r.text.isNotEmpty()) Text(r.text, color = if (r.lead.isNotEmpty()) Muted else Ink, fontSize = 13.sp, lineHeight = 18.sp,
                        maxLines = 2, overflow = TextOverflow.Ellipsis)
                    if (r.tag.isNotEmpty()) Text(r.tag.uppercase(), color = if (r.pick) ok else Muted, fontSize = 10.5.sp, fontWeight = FontWeight.Bold)
                }
                if (r.value.isNotEmpty()) Text(r.value, color = Ink, fontSize = 13.5.sp, fontWeight = FontWeight.Bold)
            }
        }
        if (spec.sources.isNotEmpty() || spec.moreLabel.isNotEmpty()) {
            HorizontalDivider(color = Line)
            Row(Modifier.fillMaxWidth().heightIn(min = 40.dp).padding(horizontal = 14.dp, vertical = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                Row(horizontalArrangement = Arrangement.spacedBy((-6).dp)) {
                    spec.sources.forEachIndexed { i, _ ->
                        Box(Modifier.size(16.dp).clip(RoundedCornerShape(50)).background(Superficie).padding(2.dp)
                            .clip(RoundedCornerShape(50)).background(SourceDots[i % SourceDots.size]))
                    }
                }
                if (spec.sources.isNotEmpty()) Text("  " + resultSourceNames(spec.sources).joinToString(" · "), color = Muted, fontSize = 12.sp,
                    maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f)
                        .clickable(role = Role.Button) { spec.sources.firstOrNull()?.second?.takeIf { it.isNotEmpty() }?.let { uri.openUri(it) } })
                else Spacer(Modifier.weight(1f))
            }
            if (spec.moreLabel.isNotEmpty()) Text(spec.moreLabel + " ›", color = link, fontSize = 13.sp, fontWeight = FontWeight.SemiBold,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
                modifier = Modifier.fillMaxWidth().clickable(role = Role.Button) { if (spec.moreUrl.isNotEmpty()) uri.openUri(spec.moreUrl) }
                    .padding(start = 14.dp, end = 14.dp, bottom = 12.dp))
        }
    }
}

/** Pura: cada web una vegada (pel domini de l'enllaç; si no n'hi ha, pel nom). */
internal fun resultSourceNames(sources: List<Pair<String, String>>): List<String> =
    sources.map { (name, url) ->
        url.substringAfter("://", "").substringBefore('/').removePrefix("www.").substringBeforeLast('.').substringAfterLast('.')
            .takeIf { it.isNotBlank() } ?: name
    }.distinct()
