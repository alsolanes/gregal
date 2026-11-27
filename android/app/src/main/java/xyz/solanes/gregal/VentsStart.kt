package xyz.solanes.gregal

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.snapping.SnapPosition
import androidx.compose.foundation.gestures.snapping.rememberSnapFlingBehavior
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.isSpecified
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.CompositingStrategy
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.boundsInRoot
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlin.math.PI
import kotlin.math.sin

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
    /** La cara tocada i on és (a l'arrel), perquè hi pugui volar fins a la capçalera del xat. */
    onWind: (Wind, Rect) -> Unit,
    onThread: (WindThread) -> Unit,
    modifier: Modifier = Modifier,
    aboveComposer: @Composable () -> Unit = {},
    news: Map<String, Int> = emptyMap(),
    bounce: WindArrival? = null,
    /** Els vents que treballen ara: porten un punt a la cara. */
    working: Set<String> = emptySet(),
    /** 168: els vents que t'esperen (una pregunta o un permís pendents): cara d'espera, amb la seva marca. */
    waiting: Set<String> = emptySet(),
) {
    VentsOpening(
        lang, name, text, onText, busy, onPhoto = null, onDoc = null, onSend = onSend,
        modifier = modifier, halo = false,
        top = { StartFaces(lang, winds, news, bounce, working, waiting, onWind) },
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

/** Un alè de [StartFaceBreathPeriod] s: l'escala de la cara [index] al segon [t], desfasada de la del costat. */
internal fun startFaceBreath(t: Float, index: Int): Float =
    1f + .018f * sin(2f * PI.toFloat() * (t / StartFaceBreathPeriod + index * .382f))

internal const val StartFaceBreathPeriod = 3.6f

@Composable
private fun StartFaces(lang: String, winds: List<Wind>, news: Map<String, Int>, bounce: WindArrival?, working: Set<String>,
    waiting: Set<String>, onWind: (Wind, Rect) -> Unit) {
    val clock = if (windMotionEnabled()) rememberInfiniteTransition(label = "start-breath").animateFloat(0f, StartFaceBreathPeriod,
        infiniteRepeatable(tween((StartFaceBreathPeriod * 1000).toInt(), easing = LinearEasing)), label = "t") else null
    val list = rememberLazyListState()
    LazyRow(
        Modifier.fillMaxWidth().testTag("start-faces")
            // Les vores s'esvaeixen només cap on queden cares: així se sap que la fila continua.
            .graphicsLayer { compositingStrategy = CompositingStrategy.Offscreen }.drawWithContent {
                drawContent()
                val edge = 28.dp.toPx() / size.width
                val left = if (list.canScrollBackward) Color.Transparent else Color.Black
                val right = if (list.canScrollForward) Color.Transparent else Color.Black
                drawRect(Brush.horizontalGradient(0f to left, edge to Color.Black, 1f - edge to Color.Black, 1f to right),
                    blendMode = BlendMode.DstIn)
            },
        state = list,
        flingBehavior = rememberSnapFlingBehavior(list, SnapPosition.Start),
        contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        itemsIndexed(winds, key = { _, w -> w.id }) { i, w ->
            var face by remember { mutableStateOf(Rect.Zero) }
            val busy = w.id in working
            val waits = w.id in waiting
            val state = if (waits) tr(lang, "winds.state.waiting") else if (busy) tr(lang, "winds.state.working") else ""
            Column(
                // Creix fins a 96 dp per la pastilla de l'ofici: a 60 dp només hi cabien dues lletres.
                Modifier.widthIn(min = 60.dp, max = 96.dp).clip(RoundedCornerShape(12.dp)).clickable(role = Role.Button) { onWind(w, face) }
                    .semantics { stateDescription = state }
                    .padding(vertical = 4.dp).testTag("start-wind-" + w.id),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                NewsBounce(bounce?.takeIf { it.wind == w.id }) {
                    Box(Modifier.onGloballyPositioned { face = it.boundsInRoot() }.graphicsLayer {
                        clock?.let { c -> startFaceBreath(c.value, i).let { scaleX = it; scaleY = it } }
                    }) {
                        WindHalo(Color(w.argb), w.faceStyle, size = 40, mood = if (waits) WindMood.WAITING else WindMood.IDLE, online = false, showKit = false, clay = true)
                        NewsBadge(news[w.id] ?: 0, Modifier.align(Alignment.TopEnd).testTag("start-wind-badge-" + w.id))
                        if (busy) Box(Modifier.align(Alignment.BottomEnd).padding(4.dp).size(12.dp)
                            .border(2.dp, Superficie, CircleShape).padding(2.dp).background(Color(w.argb), CircleShape)
                            .testTag("start-wind-busy-" + w.id))
                    }
                }
                Text(w.name, color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                WindRolePill(w)
            }
        }
    }
}

/**
 * La cara tocada a la portada vola de [from] (a l'arrel) al forat de la cara de
 * la capçalera del xat i s'hi encongeix; en arribar, [onLanded] i el xat torna
 * a pintar la seva. Sense moviment no vola: arriba de seguida.
 */
@Composable
internal fun BoxScope.FaceLanding(perch: WindPerch, from: Rect, accent: Color, style: WindStyle, onLanded: () -> Unit) {
    val motion = windMotionEnabled()
    val flight = remember { Animatable(0f) }
    val ready = perch.root.isSpecified && perch.header.isSpecified
    LaunchedEffect(ready) {
        if (motion && !ready) return@LaunchedEffect
        if (motion) flight.animateTo(1f, spring(dampingRatio = .8f, stiffness = Spring.StiffnessMediumLow))
        onLanded()
    }
    if (!motion) return
    // La de la portada fa 48 dp (WindHalo de 40); la de la capçalera, 38 al mig del forat de 44.
    WindFace(size = 48, accent = accent, style = style, clay = true, modifier = Modifier.align(Alignment.TopStart).graphicsLayer {
        val root = perch.root
        if (!root.isSpecified) { alpha = 0f; return@graphicsLayer }
        val start = from.center - root
        val end = if (perch.header.isSpecified) perch.header + Offset(22.dp.toPx(), 22.dp.toPx()) - root else start
        val f = flight.value
        val half = 24.dp.toPx()
        translationX = start.x + (end.x - start.x) * f - half
        translationY = start.y + (end.y - start.y) * f - half
        scaleX = 1f + (38f / 48f - 1f) * f
        scaleY = scaleX
    }.testTag("face-landing"))
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
