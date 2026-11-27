package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowForward
import androidx.compose.material.icons.rounded.Check
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.time.LocalTime

/*
 * El resum del dia, a dalt d'«Avui» (com el Daily Brief de Duet): una targeta
 * per vent que ha fet o proposa alguna cosa, que es llisquen com les fitxes de
 * la comparativa. Els vents sense res no hi surten. Tot surt de la safata que
 * ja dona GET /api/winds/avui.
 */

/** [did]: la feina que ha acabat; [proposes]: la primera targeta oberta que encara demana alguna cosa. */
data class WindBrief(val wind: Wind, val did: AvuiCard?, val proposes: AvuiCard?) {
    /**
     * El botó de la targeta: l'acció del que proposa o, si no, la del que ha fet.
     * Una vigilància pausada (`pausa`) no: el xat no la reprèn, ho fa «Torna-hi» a la seva targeta.
     */
    val act: Pair<AvuiCard, AvuiAction>? get() =
        listOfNotNull(proposes, did).filter { it.source != "pausa" }.firstNotNullOfOrNull { c -> c.actions.firstOrNull()?.let { c to it } }
}

object AvuiBriefLogic {
    /** Feina delegada acabada o una rutina que ha corregut: el que el vent ha fet. */
    private fun worked(c: AvuiCard) = c.source.startsWith("feina:") || c.source == "job"

    /** Un per vent, en l'ordre de la safata (Ara toca primer) i després els que només han fet. */
    fun briefs(v: AvuiView, winds: List<Wind>): List<WindBrief> {
        val open = v.all.filter { it.state == "open" }
        val cards = open + v.done
        return cards.map { it.wind }.distinct().mapNotNull { id ->
            val mine = cards.filter { it.wind == id }
            val did = mine.firstOrNull(::worked)
            val proposes = open.firstOrNull { it.wind == id && it != did }
            if (did == null && proposes == null) return@mapNotNull null
            val wind = winds.firstOrNull { it.id == id } ?: Wind(id, mine.first().windName, "", "")
            WindBrief(wind, did, proposes)
        }
    }
}

@Composable
internal fun AvuiBrief(lang: String, view: AvuiView, winds: List<Wind>, onRun: (AvuiCard, AvuiAction) -> Unit) {
    val briefs = AvuiBriefLogic.briefs(view, winds)
    if (briefs.isEmpty()) return
    val motion = windMotionEnabled()
    val pager = rememberPagerState { briefs.size }
    // Els dibuixos: el vent del temps porta el cel d'avui; el que segueix un equip, el partit.
    val weatherId = weatherWind(winds)?.id
    val teams = GlanceLogic.follows(view).filter { it.kind == "team" }.map { it.wind }.toSet()
    Column(Modifier.padding(top = 14.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(tr(lang, "app.avui.brief.title"), color = Muted, style = GregalType.overline, modifier = Modifier.padding(start = 4.dp))
        HorizontalPager(pager, Modifier.fillMaxWidth().testTag("avui-brief"), pageSpacing = 12.dp, verticalAlignment = Alignment.Top) { page ->
            val b = briefs[page]
            val w = view.glance.weather
            BriefCard(lang, b, onRun) {
                if (b.wind.id == weatherId && w != null) Box(Modifier.testTag("brief-art-weather")) {
                    GlanceSky(WeatherJson.sky(w.code), GlanceLogic.night(w, LocalTime.now()), 52.dp, motion)
                } else if (b.wind.id in teams) MatchArt(Color(b.wind.argb), 52.dp, motion, Modifier.testTag("brief-art-match"))
            }
        }
        if (briefs.size > 1) PagerDots(pager, LocalPal.current.sea, motion, "avui-brief-dot")
    }
}

@Composable
private fun BriefCard(lang: String, b: WindBrief, onRun: (AvuiCard, AvuiAction) -> Unit, art: @Composable () -> Unit) {
    val accent = Color(b.wind.argb)
    val shape = GregalShape.lg
    Column(
        Modifier.fillMaxWidth().clip(shape)
            .background(Brush.linearGradient(listOf(accent.copy(alpha = .18f), SuperficieAlta.copy(alpha = .55f))))
            .border(1.dp, accent.copy(alpha = .32f), shape)
            .padding(16.dp).testTag("avui-brief-${b.wind.id}"),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Box(Modifier.size(40.dp).clip(RoundedCornerShape(12.dp)).background(Superficie), contentAlignment = Alignment.Center) {
                WindFace(size = 34, accent = accent, style = b.wind.faceStyle, showKit = false)
            }
            Text(b.wind.name, color = Ink, style = GregalType.heading, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
            art()
        }
        b.did?.let { c ->
            Row(verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Icon(Icons.Rounded.Check, null, tint = accent, modifier = Modifier.padding(top = 2.dp).size(15.dp))
                Column {
                    Text(tr(lang, "app.avui.brief.did"), color = Muted, style = GregalType.caption)
                    Text(c.title, color = Ink, style = GregalType.bodySmall, maxLines = 2, overflow = TextOverflow.Ellipsis)
                }
            }
        }
        b.proposes?.let { c ->
            Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(tr(lang, "app.avui.brief.proposes"), color = Muted, style = GregalType.caption)
                Text(c.title, color = Ink, style = GregalType.heading.copy(fontSize = 16.sp, lineHeight = 21.sp), maxLines = 3, overflow = TextOverflow.Ellipsis)
                if (c.why.isNotBlank()) Text(c.why, color = Muted, style = GregalType.bodySmall, maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
        }
        b.act?.let { (c, a) ->
            Row(
                Modifier.heightIn(min = 36.dp).clip(GregalShape.pill).background(Ink).clickable { onRun(c, a) }
                    .padding(horizontal = 14.dp).testTag("avui-brief-act"),
                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(5.dp),
            ) {
                Text(a.label, color = Superficie, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Icon(Icons.AutoMirrored.Rounded.ArrowForward, null, tint = Superficie, modifier = Modifier.size(14.dp))
            }
        }
    }
}
