package xyz.solanes.gregal

import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * La portada d'inici de Vents: salutació i compositor com a la portada de
 * sempre, les cares dels vents a dalt i les converses recents. Escriure-hi
 * comença una conversa nova; abans la portada era un sol fil amb Gregal que
 * no s'acabava mai i els altres vents només eren al calaix.
 */
@Composable
internal fun VentsStartPage(
    lang: String,
    name: String,
    winds: List<Wind>,
    threads: List<WindThread>,
    text: String,
    onText: (String) -> Unit,
    busy: Boolean,
    onSend: (String) -> Unit,
    onWind: (Wind) -> Unit,
    onThread: (WindThread) -> Unit,
    modifier: Modifier = Modifier,
    aboveComposer: @Composable () -> Unit = {},
    news: Map<String, Int> = emptyMap(),
    bounce: WindArrival? = null,
) {
    VentsOpening(
        lang, name, text, onText, busy, onPhoto = null, onDoc = null, onSend = onSend,
        modifier = modifier, halo = false,
        top = { StartFaces(winds, news, bounce, onWind) },
        below = {
            if (threads.isNotEmpty()) StartRecents(lang) {
                threads.forEach { t ->
                    val wind = winds.firstOrNull { it.id == t.wind }
                    RecentRow(t.title.ifBlank { wind?.name ?: t.wind },
                        listOfNotNull(wind?.name, threadWhen(t.at, lang).ifBlank { null }).joinToString(" · "),
                        onClick = { onThread(t) }) {
                        if (wind != null) WindHalo(Color(wind.argb), wind.faceStyle, size = 26, online = false, showKit = false)
                    }
                }
            }
        },
        aboveComposer = aboveComposer,
    )
}

@Composable
private fun StartFaces(winds: List<Wind>, news: Map<String, Int>, bounce: WindArrival?, onWind: (Wind) -> Unit) {
    Row(
        Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).padding(horizontal = 12.dp, vertical = 6.dp),
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        winds.forEach { w ->
            Column(
                // Creix fins a 96 dp per la pastilla de l'ofici: a 60 dp només hi cabien dues lletres.
                Modifier.widthIn(min = 60.dp, max = 96.dp).clip(RoundedCornerShape(12.dp)).clickable(role = Role.Button) { onWind(w) }
                    .padding(vertical = 4.dp).testTag("start-wind-" + w.id),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                NewsBounce(bounce?.takeIf { it.wind == w.id }) {
                    Box {
                        WindHalo(Color(w.argb), w.faceStyle, size = 40, online = false, showKit = false)
                        NewsBadge(news[w.id] ?: 0, Modifier.align(Alignment.TopEnd).testTag("start-wind-badge-" + w.id))
                    }
                }
                Text(w.name, color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                WindRolePill(w)
            }
        }
    }
}

/** Les converses recents d'una portada, la de Vents i la de Codi: el títol i, a sota, les fileres. */
@Composable
internal fun StartRecents(lang: String, rows: @Composable ColumnScope.() -> Unit) {
    Column(Modifier.fillMaxWidth().testTag("start-recents"), verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(tr(lang, "app.vents.start.recent"), color = Muted, fontSize = 13.sp, fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(start = 4.dp, bottom = 4.dp))
        rows()
    }
}

@Composable
internal fun RecentRow(title: String, detail: String, onClick: () -> Unit, lead: @Composable () -> Unit = {}) {
    Row(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).clickable(role = Role.Button, onClick = onClick)
            .heightIn(min = 48.dp).padding(horizontal = 4.dp, vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        lead()
        Column(Modifier.weight(1f)) {
            Text(title, color = Ink, fontSize = 15.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(detail, color = Muted, fontSize = 12.sp, maxLines = 1)
        }
    }
}

/** «Avui 10:24», «Ahir» o «18-11» per a la marca RFC 3339 d'una conversa (pura: testejable). */
internal fun threadWhen(at: String, lang: String, nowMs: Long = System.currentTimeMillis()): String {
    val ms = runCatching { java.time.OffsetDateTime.parse(at).toInstant().toEpochMilli() }.getOrNull() ?: return ""
    val day = dayLabel(ms, nowMs, lang)
    return if (dayKey(ms) == dayKey(nowMs)) day + " " + hourMinute(ms) else day
}

/** Les recents de la més nova a la més vella per l'instant, no pel text: «09:00+01:00» és abans que «08:30Z». */
internal fun newestFirst(threads: List<WindThread>): List<WindThread> =
    threads.sortedByDescending { runCatching { java.time.OffsetDateTime.parse(it.at).toInstant().toEpochMilli() }.getOrDefault(0L) }
