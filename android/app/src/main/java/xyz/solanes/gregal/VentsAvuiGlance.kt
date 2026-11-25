package xyz.solanes.gregal

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowForward
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.Close
import androidx.compose.material.icons.rounded.Event
import androidx.compose.material.icons.rounded.NightsStay
import androidx.compose.material.icons.rounded.Place
import androidx.compose.material.icons.rounded.Umbrella
import androidx.compose.material.icons.rounded.WbTwilight
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathOperation
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.time.Duration
import java.time.LocalDateTime
import java.time.LocalTime
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.temporal.ChronoUnit
import kotlinx.coroutines.delay
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/*
 * La ullada de dalt d'«Avui»: el temps d'on ets, el calendari d'avui i demà i
 * una targeta per cada cosa que segueixes (el proper partit, la propera cosa).
 * Ho llegeix el servidor (glance a GET /api/winds/avui); aquí es pinta viva:
 * les targetes entren amb molla, el cel es mou i els comptes enrere corren. Amb
 * el moviment expressiu apagat, tot quiet.
 */

data class AvuiWeather(
    val place: String, val nowC: Double, val minC: Double, val maxC: Double, val rainMm: Double,
    /** Les hores d'avui amb pluja, «HH». */
    val rainHours: List<String>, val code: Int, val sunrise: String, val sunset: String,
)

/** Una cita del calendari, en RFC 3339; [end] buit si no té durada. */
data class AvuiEvent(val start: String, val end: String, val title: String, val calendar: String)

/** Una cosa que segueixes; [title] buit: el vent encara no l'ha buscada. [at]: RFC 3339 o buit. */
data class AvuiFollow(
    val id: String, val kind: String, val name: String, val wind: String = "",
    val title: String = "", val at: String = "", val detail: String = "", val url: String = "",
)

data class AvuiGlance(val weather: AvuiWeather? = null, val events: List<AvuiEvent> = emptyList(), val follows: List<AvuiFollow> = emptyList())

@Serializable
internal class AvuiWeatherDto(
    val place: String = "", @SerialName("now_c") val nowC: Double = 0.0, @SerialName("min_c") val minC: Double = 0.0,
    @SerialName("max_c") val maxC: Double = 0.0, @SerialName("rain_mm") val rainMm: Double = 0.0,
    @SerialName("rain_hours") val rainHours: List<String> = emptyList(), val code: Int = 0,
    val sunrise: String = "", val sunset: String = "",
)

@Serializable internal class AvuiEventDto(val start: String = "", val end: String = "", val title: String = "", val calendar: String = "")

@Serializable
internal class AvuiFollowDto(
    val id: String = "", val kind: String = "", val name: String = "", val wind: String = "",
    val title: String = "", @SerialName("when") val at: String = "", val detail: String = "", val url: String = "",
)

@Serializable
internal class AvuiGlanceDto(
    val weather: AvuiWeatherDto? = null, val events: List<AvuiEventDto> = emptyList(), val follows: List<AvuiFollowDto> = emptyList(),
)

internal fun glanceFollow(j: AvuiFollowDto) = AvuiFollow(
    j.id, if (j.kind == "team") "team" else "other", j.name, j.wind, j.title, j.at, j.detail, j.url,
)

internal fun glance(o: AvuiGlanceDto?): AvuiGlance {
    if (o == null) return AvuiGlance()
    return AvuiGlance(
        weather = o.weather?.let { AvuiWeather(it.place, it.nowC, it.minC, it.maxC, it.rainMm, it.rainHours, it.code, it.sunrise, it.sunset) },
        events = o.events.filter { it.start.isNotBlank() && it.title.isNotBlank() }.map { AvuiEvent(it.start, it.end, it.title, it.calendar) },
        follows = o.follows.map(::glanceFollow).filter { it.id.isNotBlank() },
    )
}

/** Lògica pura de la ullada (es prova sense Compose). */
object GlanceLogic {
    private val diesCa = listOf("dilluns", "dimarts", "dimecres", "dijous", "divendres", "dissabte", "diumenge")
    private val mesosCa = listOf("gener", "febrer", "març", "abril", "maig", "juny", "juliol", "agost", "setembre", "octubre", "novembre", "desembre")
    private val daysEn = listOf("Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday")
    private val monthsEn = listOf("January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December")

    /** RFC 3339 (o hora local sense zona) a l'hora d'aquí; null si no ho és. */
    fun time(iso: String, zone: ZoneId): ZonedDateTime? =
        runCatching { OffsetDateTime.parse(iso).atZoneSameInstant(zone) }.getOrNull()
            ?: runCatching { LocalDateTime.parse(iso).atZone(zone) }.getOrNull()

    /** «dissabte a les 21:00», «demà a les 18:30»; «Saturday at 21:00». Buit si no hi ha hora. */
    fun whenLabel(lang: String, iso: String, now: ZonedDateTime): String {
        val t = time(iso, now.zone) ?: return ""
        val en = lang == "en"
        val hm = "%02d:%02d".format(t.hour, t.minute)
        val dow = t.dayOfWeek.value - 1
        val day = when (ChronoUnit.DAYS.between(now.toLocalDate(), t.toLocalDate())) {
            0L -> if (en) "today" else "avui"
            1L -> if (en) "tomorrow" else "demà"
            in 2L..6L -> if (en) daysEn[dow] else diesCa[dow]
            else -> {
                val m = t.monthValue - 1
                if (en) "${daysEn[dow]}, ${monthsEn[m]} ${t.dayOfMonth}"
                else "${diesCa[dow]} ${t.dayOfMonth} ${if (mesosCa[m].first() in "aeiouàèéíòóú") "d’" else "de "}${mesosCa[m]}"
            }
        }
        return if (en) "$day at $hm" else "$day ${if (t.hour == 1) "a la" else "a les"} $hm"
    }

    /** «d’aquí a 2 h», «d’aquí a 12 min», «d’aquí a 4 dies»; «Ara mateix» les dues hores després de començar. */
    fun countdown(lang: String, iso: String, now: ZonedDateTime): String {
        val t = time(iso, now.zone) ?: return ""
        val secs = Duration.between(now, t).seconds
        if (secs <= 0) return if (secs > -2 * 3600) tr(lang, "app.avui.glance.live") else ""
        val mins = maxOf(1L, secs / 60)
        val span = when {
            mins < 60 -> "$mins min"
            mins < 24 * 60 -> if (mins % 60 == 0L) "${mins / 60} h" else "${mins / 60} h ${mins % 60} min"
            else -> (mins / (24 * 60)).let { d -> tr(lang, if (d == 1L) "app.avui.glance.day" else "app.avui.glance.days", d) }
        }
        return tr(lang, "app.avui.glance.in", span)
    }

    /** De mitjanit a mitjanit (així arriben les de tot el dia). */
    fun allDay(e: AvuiEvent, zone: ZoneId): Boolean {
        val start = time(e.start, zone) ?: return false
        val end = time(e.end, zone) ?: return false
        return start.toLocalTime() == LocalTime.MIDNIGHT && end.toLocalTime() == LocalTime.MIDNIGHT && end.isAfter(start)
    }

    /** «18:00» o «Tot el dia». */
    fun eventTime(lang: String, e: AvuiEvent, zone: ZoneId): String =
        if (allDay(e, zone)) tr(lang, "app.avui.glance.allDay")
        else time(e.start, zone)?.let { "%02d:%02d".format(it.hour, it.minute) } ?: ""

    /** 0 avui, 1 demà. */
    fun eventDay(e: AvuiEvent, now: ZonedDateTime): Int =
        time(e.start, now.zone)?.let { ChronoUnit.DAYS.between(now.toLocalDate(), it.toLocalDate()).toInt() } ?: 0

    /** Ja ha passat (s'apaga una mica). Les de tot el dia, mai. */
    fun eventPast(e: AvuiEvent, now: ZonedDateTime): Boolean {
        if (allDay(e, now.zone)) return false
        val until = time(e.end, now.zone) ?: time(e.start, now.zone) ?: return false
        return until.isBefore(now)
    }

    /** Abans de la sortida del sol o després de la posta («HH:MM»). */
    fun night(w: AvuiWeather, t: LocalTime): Boolean {
        val rise = runCatching { LocalTime.parse(w.sunrise) }.getOrNull() ?: return false
        val set = runCatching { LocalTime.parse(w.sunset) }.getOrNull() ?: return false
        return t.isBefore(rise) || !t.isBefore(set)
    }

    /**
     * El que segueixes, en l'ordre dels ajustos i amb la darrera resposta de la
     * ullada; un de nou encara sense resposta surt amb el nom.
     */
    fun follows(v: AvuiView): List<AvuiFollow> {
        val g = v.glance.follows
        return v.settings.follows.map { s -> g.firstOrNull { it.id == s.id } ?: s } + g.filter { f -> v.settings.follows.none { it.id == f.id } }
    }
}

/** Entra pujant, amb molla, una mica després de la d'abans. Sense moviment, quieta. */
@Composable
private fun Modifier.springIn(index: Int, motion: Boolean): Modifier {
    val p = remember { Animatable(if (motion) 0f else 1f) }
    if (motion) LaunchedEffect(p) {
        delay(index * 80L)
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

/** Una fase 0→1 que es repeteix; sense moviment, sempre 0 (el dibuix queda quiet). */
@Composable
private fun phase(motion: Boolean, millis: Int, label: String): State<Float> =
    if (motion) rememberInfiniteTransition(label = label).animateFloat(0f, 1f, infiniteRepeatable(tween(millis, easing = LinearEasing)), label = label)
    else remember { mutableFloatStateOf(0f) }

/**
 * La ullada sencera, a dalt de la safata. [windWeather]: el temps del vent del
 * temps, si n'hi ha, per quan la ullada encara no en porta.
 */
@Composable
internal fun AvuiGlanceSection(
    lang: String, view: AvuiView, winds: List<Wind>,
    onFollow: (String, String) -> Unit, onUnfollow: (String) -> Unit, onPlace: (String) -> Unit,
    windWeather: (@Composable () -> Unit)?,
) {
    val motion = windMotionEnabled()
    // Els comptes enrere corren; sense moviment es queden on eren en obrir.
    val now by produceState(ZonedDateTime.now(), motion) {
        while (motion) { delay(10_000); value = ZonedDateTime.now() }
    }
    var placeOpen by remember { mutableStateOf(false) }
    var followOpen by remember { mutableStateOf(false) }
    val g = view.glance
    Column(Modifier.padding(top = 14.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        val w = g.weather
        if (placeOpen) GlanceEditor(
            lang, tr(lang, "app.avui.glance.placeTitle"), tr(lang, "app.avui.glance.placeHint"), view.settings.place,
            tr(lang, "app.avui.glance.save"), "glance-place", kinds = false, onDismiss = { placeOpen = false },
        ) { _, place -> placeOpen = false; onPlace(place) }
        if (w != null) Box(Modifier.springIn(0, motion)) { GlanceWeather(lang, w, now, motion) { placeOpen = true } }
        else {
            PlaceInvite(lang, view.settings.place, Modifier.springIn(0, motion)) { placeOpen = true }
            windWeather?.invoke()
        }
        if (g.events.isNotEmpty()) Box(Modifier.springIn(1, motion)) { GlanceEvents(lang, g.events, now) }
        GlanceFollows(lang, GlanceLogic.follows(view), winds, now, motion, onUnfollow) { followOpen = true }
        if (followOpen) GlanceEditor(
            lang, tr(lang, "app.avui.glance.followTitle"), tr(lang, "app.avui.glance.followHint"), "",
            tr(lang, "app.avui.glance.followDo"), "glance-follow", kinds = true, onDismiss = { followOpen = false },
        ) { kind, name -> followOpen = false; onFollow(kind, name) }
    }
}

@Composable
private fun skyTone(sky: Sky, night: Boolean): Color = when {
    night && (sky == Sky.CLEAR || sky == Sky.PARTLY) -> Lila
    sky == Sky.CLEAR || sky == Sky.PARTLY -> Sand
    sky == Sky.RAIN || sky == Sky.DRIZZLE || sky == Sky.STORM -> Onada
    else -> Sea
}

@Composable
private fun GlanceWeather(lang: String, w: AvuiWeather, now: ZonedDateTime, motion: Boolean, onPlace: () -> Unit) {
    val sky = WeatherJson.sky(w.code)
    val night = GlanceLogic.night(w, now.toLocalTime())
    val tone = skyTone(sky, night)
    val shape = GregalShape.lg
    Column(
        Modifier.fillMaxWidth().clip(shape)
            .background(Brush.linearGradient(listOf(tone.copy(alpha = .26f), SuperficieAlta.copy(alpha = .55f))))
            .border(1.dp, tone.copy(alpha = .32f), shape)
            .padding(start = 16.dp, end = 16.dp, top = 12.dp, bottom = 14.dp).testTag("glance-weather"),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Row(
                Modifier.clip(GregalShape.pill).clickable(onClick = onPlace).padding(end = 8.dp, top = 4.dp, bottom = 4.dp)
                    .semantics { contentDescription = tr(lang, "app.avui.glance.placeChange") }.testTag("glance-place"),
                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                Icon(Icons.Rounded.Place, null, tint = Muted, modifier = Modifier.size(14.dp))
                Text(w.place, color = Muted, style = GregalType.caption, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            Spacer(Modifier.weight(1f))
            SunTime(Icons.Rounded.WbTwilight, tr(lang, "app.avui.glance.sunrise"), w.sunrise)
            Spacer(Modifier.width(10.dp))
            SunTime(Icons.Rounded.NightsStay, tr(lang, "app.avui.glance.sunset"), w.sunset)
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
            GlanceSky(sky, night, 64.dp, motion)
            Text(WeatherJson.deg(w.nowC), color = Ink, fontSize = 46.sp, fontWeight = FontWeight.Light, letterSpacing = (-1.5).sp)
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(tr(lang, "app.weather.sky." + sky.name.lowercase()), color = Ink, style = GregalType.heading)
                Text("↑ ${WeatherJson.deg(w.maxC)}  ↓ ${WeatherJson.deg(w.minC)}", color = Muted, style = GregalType.bodySmall)
            }
        }
        if (w.rainHours.isEmpty()) Text(tr(lang, "app.avui.glance.dry"), color = Muted, style = GregalType.caption)
        else Row(
            Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(5.dp),
        ) {
            Icon(Icons.Rounded.Umbrella, tr(lang, "app.avui.glance.rainHours"), tint = Onada, modifier = Modifier.size(15.dp))
            w.rainHours.forEach { h ->
                Text(tr(lang, "app.avui.glance.hour", h), color = Ink, fontSize = 12.sp, fontWeight = FontWeight.Medium,
                    modifier = Modifier.clip(GregalShape.pill).background(Onada.copy(alpha = .22f)).padding(horizontal = 8.dp, vertical = 3.dp))
            }
            val mm = Math.round(w.rainMm)
            if (mm >= 1) Text("$mm mm", color = Muted, style = GregalType.caption, modifier = Modifier.padding(start = 4.dp))
        }
    }
}

@Composable
private fun SunTime(icon: androidx.compose.ui.graphics.vector.ImageVector, label: String, time: String) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(3.dp)) {
        Icon(icon, label, tint = Muted, modifier = Modifier.size(13.dp))
        Text(time, color = Muted, style = GregalType.caption)
    }
}

/** El cel que es mou: el sol gira a poc a poc, els núvols suren, la pluja i la neu cauen, el llamp parpelleja. */
@Composable
internal fun GlanceSky(sky: Sky, night: Boolean, size: Dp, motion: Boolean) {
    val sun = Sand
    val rain = Onada
    val dark = LocalPal.current.dark
    val cloud = if (dark) Color(0xFFE3E7F1) else Color(0xFFA9B2C3)
    val moon = Color(0xFFF2E6B5)
    val fall = phase(motion, 1300, "sky-fall")
    val turn = phase(motion, 26_000, "sky-turn")
    val drift = phase(motion, 5200, "sky-drift")
    Canvas(Modifier.size(size).testTag("glance-sky")) {
        val s = this.size.minDimension
        val dx = kotlin.math.sin(drift.value * 2 * Math.PI).toFloat() * s * .03f
        fun sunOrMoon(c: Offset, r: Float) = if (night) moon(c, r, moon) else sun(c, r, turn.value * 360f, sun)
        when (sky) {
            Sky.CLEAR -> {
                sunOrMoon(Offset(s * .5f, s * .5f), s * .2f)
                if (night) drawCircle(moon.copy(alpha = .35f + .65f * fall.value), s * .025f, Offset(s * .8f, s * .22f))
            }
            Sky.PARTLY -> {
                sunOrMoon(Offset(s * .62f, s * .36f), s * .15f)
                cloud(Offset(s * .44f + dx, s * .58f), s * .74f, cloud)
            }
            Sky.CLOUDY -> {
                cloud(Offset(s * .62f - dx, s * .38f), s * .52f, cloud.copy(alpha = .55f))
                cloud(Offset(s * .44f + dx, s * .58f), s * .8f, cloud)
            }
            Sky.FOG -> listOf(.36f, .52f, .68f).forEachIndexed { i, y ->
                val off = kotlin.math.sin((drift.value + i * .3f) * 2 * Math.PI).toFloat() * s * .06f
                drawLine(cloud, Offset(s * .16f + off, s * y), Offset(s * .84f + off, s * y), s * .07f, StrokeCap.Round)
            }
            Sky.DRIZZLE, Sky.RAIN -> {
                val n = if (sky == Sky.RAIN) 4 else 3
                val len = if (sky == Sky.RAIN) .1f else .05f
                repeat(n) { i ->
                    val p = (fall.value + i * .31f) % 1f
                    val x = s * (.3f + .14f * i)
                    val y = s * (.6f + .28f * p)
                    drawLine(rain.copy(alpha = 1f - p * .8f), Offset(x, y), Offset(x - s * .025f, y + s * len), s * .045f, StrokeCap.Round)
                }
                cloud(Offset(s * .5f + dx, s * .4f), s * .8f, cloud)
            }
            Sky.SNOW -> {
                repeat(4) { i ->
                    val p = (fall.value * .5f + i * .27f) % 1f
                    val sway = kotlin.math.sin((p + i) * 2 * Math.PI).toFloat() * s * .03f
                    drawCircle(cloud.copy(alpha = 1f - p * .7f), s * .035f, Offset(s * (.28f + .15f * i) + sway, s * (.62f + .3f * p)))
                }
                cloud(Offset(s * .5f + dx, s * .4f), s * .8f, cloud)
            }
            Sky.STORM -> {
                val flash = if (fall.value < .08f || fall.value in .16f..0.22f) 1f else .45f
                val bolt = Path().apply {
                    moveTo(s * .52f, s * .5f); lineTo(s * .4f, s * .72f); lineTo(s * .5f, s * .72f)
                    lineTo(s * .44f, s * .92f); lineTo(s * .62f, s * .64f); lineTo(s * .52f, s * .64f); close()
                }
                drawPath(bolt, sun.copy(alpha = flash))
                cloud(Offset(s * .5f + dx, s * .4f), s * .8f, cloud)
            }
        }
    }
}

private fun DrawScope.sun(c: Offset, r: Float, degrees: Float, color: Color) {
    rotate(degrees, c) {
        repeat(8) { i ->
            val a = Math.toRadians(i * 45.0)
            val (cos, sin) = kotlin.math.cos(a).toFloat() to kotlin.math.sin(a).toFloat()
            drawLine(color, Offset(c.x + cos * r * 1.4f, c.y + sin * r * 1.4f), Offset(c.x + cos * r * 1.8f, c.y + sin * r * 1.8f), r * .22f, StrokeCap.Round)
        }
    }
    drawCircle(color, r, c)
}

private fun DrawScope.moon(c: Offset, r: Float, color: Color) {
    val full = Path().apply { addOval(androidx.compose.ui.geometry.Rect(c, r)) }
    val bite = Path().apply { addOval(androidx.compose.ui.geometry.Rect(Offset(c.x + r * .55f, c.y - r * .35f), r * .9f)) }
    drawPath(Path().apply { op(full, bite, PathOperation.Difference) }, color)
}

/** Un núvol de [w] d'ample centrat a [c]: tres boles i una base arrodonida. */
private fun DrawScope.cloud(c: Offset, w: Float, color: Color) {
    drawCircle(color, w * .19f, Offset(c.x - w * .2f, c.y + w * .02f))
    drawCircle(color, w * .26f, Offset(c.x + w * .04f, c.y - w * .06f))
    drawCircle(color, w * .17f, Offset(c.x + w * .27f, c.y + w * .05f))
    drawRoundRect(color, Offset(c.x - w * .4f, c.y), Size(w * .84f, w * .22f), CornerRadius(w * .11f))
}

/** Un partit en petit: el camp amb les ratlles del color del vent i la pilota que bota. */
@Composable
internal fun MatchArt(accent: Color, size: Dp, motion: Boolean, modifier: Modifier = Modifier) {
    val bounce = phase(motion, 1200, "match-bounce")
    val ball = if (LocalPal.current.dark) Color.White else Color(0xFFFDFBF6)
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension
        val line = androidx.compose.ui.graphics.drawscope.Stroke(s * .045f)
        drawRoundRect(accent.copy(alpha = .16f), Offset(s * .08f, s * .24f), Size(s * .84f, s * .56f), CornerRadius(s * .08f))
        drawRoundRect(accent.copy(alpha = .7f), Offset(s * .08f, s * .24f), Size(s * .84f, s * .56f), CornerRadius(s * .08f), style = line)
        drawLine(accent.copy(alpha = .7f), Offset(s * .5f, s * .24f), Offset(s * .5f, s * .8f), s * .045f)
        drawCircle(accent.copy(alpha = .7f), s * .11f, Offset(s * .5f, s * .52f), style = line)
        val up = kotlin.math.abs(kotlin.math.sin(bounce.value * Math.PI)).toFloat() * s * .12f
        val c = Offset(s * .7f, s * .56f - up)
        drawCircle(ball, s * .12f, c)
        drawCircle(accent, s * .12f, c, style = androidx.compose.ui.graphics.drawscope.Stroke(s * .03f))
        drawCircle(accent, s * .045f, c)
    }
}

@Composable
private fun PlaceInvite(lang: String, place: String, modifier: Modifier, onPlace: () -> Unit) {
    val p = LocalPal.current
    Row(
        modifier.fillMaxWidth().clip(GregalShape.lg).background(SuperficieAlta.copy(alpha = .45f))
            .border(1.dp, Line, GregalShape.lg).padding(14.dp).testTag("glance-place-invite"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        GlanceSky(Sky.PARTLY, false, 40.dp, false)
        Text(
            if (place.isBlank()) tr(lang, "app.avui.glance.placeAsk") else tr(lang, "app.avui.glance.placeMissing", place),
            color = Ink, style = GregalType.bodySmall, modifier = Modifier.weight(1f),
        )
        Text(
            tr(lang, if (place.isBlank()) "app.avui.glance.placeSet" else "app.avui.glance.placeChange"),
            color = p.onSea, fontSize = 12.5.sp, fontWeight = FontWeight.SemiBold, maxLines = 1,
            modifier = Modifier.clip(GregalShape.pill).background(p.sea).clickable(onClick = onPlace)
                .padding(horizontal = 12.dp, vertical = 8.dp).testTag("glance-place"),
        )
    }
}

@Composable
private fun GlanceEvents(lang: String, events: List<AvuiEvent>, now: ZonedDateTime) {
    val p = LocalPal.current
    val sorted = events.sortedBy { GlanceLogic.eventDay(it, now) }
    // La propera d'avui porta el compte enrere.
    val next = sorted.firstOrNull { GlanceLogic.eventDay(it, now) == 0 && !GlanceLogic.allDay(it, now.zone) &&!GlanceLogic.eventPast(it, now) &&
        GlanceLogic.time(it.start, now.zone)?.isAfter(now) == true }
    Column(
        Modifier.fillMaxWidth().clip(GregalShape.lg).background(SuperficieAlta.copy(alpha = .5f))
            .border(1.dp, Line, GregalShape.lg).padding(start = 14.dp, end = 14.dp, top = 12.dp, bottom = 12.dp).testTag("glance-events"),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            Icon(Icons.Rounded.Event, null, tint = p.sea, modifier = Modifier.size(14.dp))
            Text(tr(lang, "app.avui.glance.calendar"), color = Muted, style = GregalType.overline)
        }
        var lastDay = 0
        sorted.forEach { e ->
            val day = GlanceLogic.eventDay(e, now)
            if (day > 0 && lastDay == 0) Text(tr(lang, "app.avui.glance.tomorrow"), color = Muted, style = GregalType.caption,
                fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 2.dp))
            lastDay = day
            val past = GlanceLogic.eventPast(e, now)
            val isNext = e == next
            Row(
                Modifier.fillMaxWidth().graphicsLayer { alpha = if (past) .5f else 1f },
                verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                Text(GlanceLogic.eventTime(lang, e, now.zone), color = if (isNext) p.sea else Ink, fontSize = 13.sp,
                    fontWeight = FontWeight.SemiBold, maxLines = 1, modifier = Modifier.width(64.dp))
                Box(Modifier.width(3.dp).height(34.dp).clip(GregalShape.pill).background(if (isNext) p.sea else LineHi))
                Column(Modifier.weight(1f)) {
                    Text(e.title, color = Ink, style = GregalType.bodySmall.copy(fontWeight = FontWeight.Medium), maxLines = 2, overflow = TextOverflow.Ellipsis)
                    val sub = listOfNotNull(e.calendar.ifBlank { null }, if (isNext) GlanceLogic.countdown(lang, e.start, now) else null)
                    if (sub.isNotEmpty()) Text(sub.joinToString(" · "), color = Muted, style = GregalType.caption, maxLines = 1)
                }
            }
        }
    }
}

@Composable
private fun GlanceFollows(
    lang: String, follows: List<AvuiFollow>, winds: List<Wind>, now: ZonedDateTime, motion: Boolean,
    onUnfollow: (String) -> Unit, onAdd: () -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(tr(lang, "app.avui.glance.follows"), color = Muted, style = GregalType.overline, modifier = Modifier.padding(start = 4.dp, top = 4.dp))
        if (follows.isEmpty()) {
            Row(
                Modifier.fillMaxWidth().springIn(2, motion).clip(GregalShape.lg).background(SuperficieAlta.copy(alpha = .45f))
                    .border(1.dp, Line, GregalShape.lg).padding(14.dp).testTag("glance-follow-empty"),
                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                Text(tr(lang, "app.avui.glance.followEmpty"), color = Ink, style = GregalType.bodySmall, modifier = Modifier.weight(1f))
                AddPill(lang, onAdd)
            }
            return@Column
        }
        Row(
            Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).height(IntrinsicSize.Max),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            follows.forEachIndexed { i, f ->
                key(f.id) {
                    val accent = winds.firstOrNull { it.id == f.wind }?.let { Color(it.argb) } ?: LocalPal.current.sea
                    FollowCard(lang, f, accent, now, motion, Modifier.springIn(2 + i, motion)) { onUnfollow(f.id) }
                }
            }
            Column(
                Modifier.width(112.dp).fillMaxHeight().springIn(2 + follows.size, motion).clip(GregalShape.lg)
                    .border(1.dp, LineHi, GregalShape.lg).clickable(onClick = onAdd).padding(12.dp).testTag("glance-add"),
                horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center,
            ) {
                Box(Modifier.size(36.dp).clip(CircleShape).background(SuperficieAlta), contentAlignment = Alignment.Center) {
                    Icon(Icons.Rounded.Add, null, tint = Ink, modifier = Modifier.size(20.dp))
                }
                Spacer(Modifier.height(8.dp))
                Text(tr(lang, "app.avui.glance.follow"), color = Ink, style = GregalType.bodySmall.copy(fontWeight = FontWeight.Medium))
            }
        }
    }
}

@Composable
private fun AddPill(lang: String, onAdd: () -> Unit) {
    val p = LocalPal.current
    Row(
        Modifier.clip(GregalShape.pill).background(p.sea).clickable(onClick = onAdd).padding(horizontal = 12.dp, vertical = 8.dp).testTag("glance-add"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        Icon(Icons.Rounded.Add, null, tint = p.onSea, modifier = Modifier.size(15.dp))
        Text(tr(lang, "app.avui.glance.follow"), color = p.onSea, fontSize = 12.5.sp, fontWeight = FontWeight.SemiBold, maxLines = 1)
    }
}

@Composable
private fun FollowCard(lang: String, f: AvuiFollow, accent: Color, now: ZonedDateTime, motion: Boolean, modifier: Modifier, onRemove: () -> Unit) {
    val shape = GregalShape.lg
    val uri = LocalUriHandler.current
    val countdown = GlanceLogic.countdown(lang, f.at, now)
    val live = countdown == tr(lang, "app.avui.glance.live")
    Column(
        modifier.width(236.dp).fillMaxHeight().clip(shape)
            .background(Brush.linearGradient(listOf(accent.copy(alpha = .18f), SuperficieAlta.copy(alpha = .5f))))
            .border(1.dp, accent.copy(alpha = .3f), shape)
            .padding(start = 14.dp, end = 6.dp, top = 8.dp, bottom = 12.dp).testTag("glance-follow-${f.id}"),
        verticalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            if (f.kind == "team") MatchArt(accent, 30.dp, motion, Modifier.testTag("glance-match-art"))
            else Box(Modifier.size(26.dp).clip(CircleShape).background(accent.copy(alpha = .2f)), contentAlignment = Alignment.Center) {
                Icon(Icons.Rounded.AutoAwesome, null, tint = accent, modifier = Modifier.size(15.dp))
            }
            Text(f.name, color = Muted, style = GregalType.caption, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
            Box(
                Modifier.size(36.dp).clip(CircleShape).clickable(onClick = onRemove)
                    .semantics { contentDescription = tr(lang, "app.avui.glance.unfollow", f.name) },
                contentAlignment = Alignment.Center,
            ) { Icon(Icons.Rounded.Close, null, tint = Muted.copy(alpha = .8f), modifier = Modifier.size(15.dp)) }
        }
        Column(Modifier.padding(end = 8.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            if (f.title.isBlank()) {
                val breathe = phase(motion, 1800, "follow-pending")
                Text(tr(lang, "app.avui.glance.pending"), color = Muted, style = GregalType.bodySmall,
                    modifier = Modifier.graphicsLayer { alpha = .55f + .45f * kotlin.math.abs(1f - 2f * breathe.value) })
                return@Column
            }
            Text(f.title, color = Ink, style = GregalType.heading.copy(fontSize = 16.sp, lineHeight = 21.sp), maxLines = 2, overflow = TextOverflow.Ellipsis)
            val whenText = GlanceLogic.whenLabel(lang, f.at, now)
            if (whenText.isNotEmpty()) Text(whenText, color = Ink.copy(alpha = .82f), style = GregalType.bodySmall)
            if (countdown.isNotEmpty()) Row(
                Modifier.clip(GregalShape.pill).background(accent.copy(alpha = .16f)).padding(horizontal = 8.dp, vertical = 3.dp),
                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(5.dp),
            ) {
                val pulse = phase(motion && live, 1400, "follow-live")
                Box(Modifier.size(6.dp).graphicsLayer { alpha = 1f - .6f * pulse.value; scaleX = 1f + .5f * pulse.value; scaleY = 1f + .5f * pulse.value }
                    .clip(CircleShape).background(if (live) Red else accent))
                Text(countdown, color = Ink, fontSize = 11.5.sp, fontWeight = FontWeight.SemiBold, maxLines = 1)
            }
            if (f.detail.isNotBlank()) Text(f.detail, color = Muted, style = GregalType.caption, maxLines = 2, overflow = TextOverflow.Ellipsis)
            if (f.url.isNotBlank()) Row(
                Modifier.clip(GregalShape.pill).clickable { uri.openUri(f.url) }.padding(vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(3.dp),
            ) {
                Text(tr(lang, "app.avui.glance.open"), color = accent, fontSize = 12.5.sp, fontWeight = FontWeight.SemiBold)
                Icon(Icons.AutoMirrored.Rounded.ArrowForward, null, tint = accent, modifier = Modifier.size(13.dp))
            }
        }
    }
}

/**
 * Un camp en línia per dir on ets o què vols seguir (amb [kinds]: un equip o
 * una altra cosa). Al mòbil s'obre al lloc on has tocat, no en un diàleg.
 */
@Composable
private fun GlanceEditor(
    lang: String, title: String, hint: String, initial: String, ok: String, tag: String, kinds: Boolean,
    onDismiss: () -> Unit, onSave: (String, String) -> Unit,
) {
    val p = LocalPal.current
    var text by remember { mutableStateOf(initial) }
    var kind by remember { mutableStateOf("team") }
    Column(
        Modifier.fillMaxWidth().clip(GregalShape.lg).background(SuperficieAlta.copy(alpha = .6f))
            .border(1.dp, p.sea.copy(alpha = .45f), GregalShape.lg).padding(14.dp).testTag("$tag-editor"),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Text(title, color = Ink, style = GregalType.bodySmall.copy(fontWeight = FontWeight.SemiBold))
        OutlinedTextField(
            text, { text = it }, singleLine = true,
            placeholder = { Text(hint, fontSize = 13.sp, color = Muted) },
            textStyle = GregalType.bodySmall.copy(color = Ink, fontSize = 14.sp), shape = GregalShape.sm,
            colors = OutlinedTextFieldDefaults.colors(
                focusedContainerColor = Superficie.copy(alpha = .6f), unfocusedContainerColor = Superficie.copy(alpha = .6f),
                focusedBorderColor = p.sea, unfocusedBorderColor = Line, cursorColor = p.sea,
            ),
            modifier = Modifier.fillMaxWidth().testTag("$tag-name"),
        )
        if (kinds) Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            listOf("team" to "app.avui.glance.team", "other" to "app.avui.glance.other").forEach { (k, label) ->
                val on = kind == k
                Text(tr(lang, label), color = if (on) p.onSea else Ink, fontSize = 13.sp, fontWeight = FontWeight.Medium,
                    modifier = Modifier.clip(GregalShape.pill).background(if (on) p.sea else Color.Transparent)
                        .border(1.dp, if (on) p.sea else Line, GregalShape.pill).clickable { kind = k }
                        .padding(horizontal = 12.dp, vertical = 8.dp))
            }
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
            TextButton(onClick = onDismiss) { Text(tr(lang, "app.avui.cancel"), color = Muted) }
            TextButton(enabled = text.isNotBlank(), onClick = { onSave(kind, text.trim()) }, modifier = Modifier.testTag("$tag-ok")) {
                Text(ok, color = if (text.isNotBlank()) p.sea else Muted, fontWeight = FontWeight.SemiBold)
            }
        }
    }
}

