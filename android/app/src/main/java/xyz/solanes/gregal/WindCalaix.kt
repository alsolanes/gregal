package xyz.solanes.gregal

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.AcUnit
import androidx.compose.material.icons.rounded.Air
import androidx.compose.material.icons.rounded.Close
import androidx.compose.material.icons.rounded.Cloud
import androidx.compose.material.icons.rounded.Dehaze
import androidx.compose.material.icons.rounded.FilterDrama
import androidx.compose.material.icons.rounded.Grain
import androidx.compose.material.icons.rounded.NightsStay
import androidx.compose.material.icons.rounded.Place
import androidx.compose.material.icons.rounded.Star
import androidx.compose.material.icons.rounded.Thunderstorm
import androidx.compose.material.icons.rounded.Umbrella
import androidx.compose.material.icons.rounded.WbSunny
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
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
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.drawText
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.json.JSONArray
import org.json.JSONObject
import java.time.LocalDate
import java.time.LocalDateTime

/*
 * El calaix d'un vent (GET/POST /api/winds/calaix, winds_calaix.go): els
 * prestatges que omple ell sol per consultar-los a l'instant —la
 * classificació, el calendari de llançaments, el receptari, els llocs— i, per
 * al vent del temps, la previsió del lloc principal (Open-Meteo, via el
 * servidor). Lectura pura aquí dalt; les vistes, a sota.
 */

/** Un prestatge del calaix. [kind]: table | releases | recipes | places | list (la resta es pinta com a taula). */
data class CalaixShelf(
    val name: String, val title: String, val kind: String, val note: String, val updated: Long,
    val items: List<Map<String, Any?>>,
)

data class ReleaseMonth(val key: String, val rows: List<Map<String, Any?>>, val past: Boolean)

object CalaixJson {
    val KINDS = setOf("table", "releases", "recipes", "places", "list")

    /** JSON → valors de Kotlin (Map, List, String, Number, Boolean, null). */
    fun value(v: Any?): Any? = when (v) {
        null, JSONObject.NULL -> null
        is JSONObject -> v.keys().asSequence().associateWith { value(v.opt(it)) }
        is JSONArray -> (0 until v.length()).map { value(v.opt(it)) }
        else -> v
    }

    fun shelves(raw: String): List<CalaixShelf> {
        val a = JSONObject(raw).optJSONArray("shelves") ?: return emptyList()
        return (0 until a.length()).mapNotNull { i ->
            val s = a.optJSONObject(i) ?: return@mapNotNull null
            val items = s.optJSONArray("items") ?: JSONArray()
            CalaixShelf(
                name = s.optString("name"), title = s.optString("title").ifBlank { s.optString("name") },
                kind = s.optString("kind").let { if (it in KINDS) it else "table" },
                note = s.optString("note"), updated = s.optLong("updated"),
                items = (0 until items.length()).mapNotNull { j ->
                    @Suppress("UNCHECKED_CAST")
                    (value(items.optJSONObject(j)) as? Map<String, Any?>)
                },
            )
        }.filter { it.name.isNotBlank() }
    }

    /** La clau d'una fila, com al servidor (rowKey): id o el primer camp que la identifica, en minúscules. */
    fun rowKey(item: Map<String, Any?>): String {
        for (k in listOf("id", "key", "title", "name", "team", "equip", "artist", "titol", "nom")) {
            val v = item[k]?.let { cell(it) }?.trim().orEmpty()
            if (v.isNotEmpty()) return v.lowercase()
        }
        return JSONObject(item).toString()
    }

    /** El primer camp no buit d'entre [keys]. */
    fun str(item: Map<String, Any?>, vararg keys: String): String =
        keys.firstNotNullOfOrNull { k -> item[k]?.let { cell(it) }?.takeIf { it.isNotBlank() } }.orEmpty()

    fun strings(item: Map<String, Any?>, key: String): List<String> = when (val v = item[key]) {
        is List<*> -> v.mapNotNull { it?.let(::cell) }.filter { it.isNotBlank() }
        is String -> v.lines().map { it.trim().removePrefix("- ").trim() }.filter { it.isNotBlank() }
        else -> emptyList()
    }

    /** Un valor per pintar: 3.0 → «3», 2.50 → «2.5», llistes amb comes. */
    fun cell(v: Any?): String = when (v) {
        null -> ""
        is Double -> if (v == Math.floor(v) && !v.isInfinite()) v.toLong().toString() else v.toString()
        is Float -> cell(v.toDouble())
        is Number -> v.toString()
        is List<*> -> v.joinToString(", ") { cell(it) }
        is Map<*, *> -> v.values.joinToString(" · ") { cell(it) }
        else -> v.toString()
    }

    /** Les columnes d'una taula, en l'ordre en què surten (sense id/key). */
    fun columns(items: List<Map<String, Any?>>): List<String> {
        val out = LinkedHashSet<String>()
        items.forEach { it.keys.forEach { k -> if (k != "id" && k != "key") out += k } }
        return out.toList()
    }

    /** Els llançaments per mesos: els que vénen primer (del més proper), després els passats (del més recent). */
    fun releaseGroups(items: List<Map<String, Any?>>, today: LocalDate = LocalDate.now()): List<ReleaseMonth> {
        fun dateOf(r: Map<String, Any?>) = runCatching { LocalDate.parse(str(r, "date").take(10)) }.getOrNull()
        val dated = items.mapNotNull { r -> dateOf(r)?.let { it to r } }
        val undated = items.filter { dateOf(it) == null }
        val upcoming = dated.filter { !it.first.isBefore(today) }.sortedBy { it.first }
        val past = dated.filter { it.first.isBefore(today) }.sortedByDescending { it.first }
        fun group(list: List<Pair<LocalDate, Map<String, Any?>>>, isPast: Boolean) =
            list.groupBy { "%04d-%02d".format(it.first.year, it.first.monthValue) }
                .map { (k, v) -> ReleaseMonth(k, v.map { it.second }, isPast) }
        return group(upcoming, false) + (if (undated.isEmpty()) emptyList() else listOf(ReleaseMonth("", undated, false))) + group(past, true)
    }

    /** El lloc principal (o el primer). */
    fun primaryPlace(items: List<Map<String, Any?>>): Map<String, Any?>? =
        items.firstOrNull { it["primary"] == true } ?: items.firstOrNull()
}

/** Dates i mesos en català i anglès per al calaix i el temps. */
object CalaixFmt {
    private val mesosCa = listOf("gener", "febrer", "març", "abril", "maig", "juny", "juliol", "agost", "setembre", "octubre", "novembre", "desembre")
    private val monthsEn = listOf("January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December")
    private val diesCa = listOf("dl.", "dt.", "dc.", "dj.", "dv.", "ds.", "dg.")
    private val daysEn = listOf("Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun")

    /** «novembre 2026» / «November 2026». */
    fun month(lang: String, key: String): String {
        val y = key.take(4); val m = key.drop(5).toIntOrNull() ?: return key
        return (if (lang == "en") monthsEn else mesosCa)[m - 1] + " " + y
    }

    fun weekday(lang: String, d: LocalDate): String = (if (lang == "en") daysEn else diesCa)[d.dayOfWeek.value - 1]

    /** «12 de nov.» curt: el dia i el mes. */
    fun day(lang: String, iso: String): String {
        val d = runCatching { LocalDate.parse(iso.take(10)) }.getOrNull() ?: return iso
        val m = (if (lang == "en") monthsEn else mesosCa)[d.monthValue - 1]
        return if (lang == "en") "${m.take(3)} ${d.dayOfMonth}" else "${d.dayOfMonth} ${if (m.first() in "aeiou") "d’" else "de "}$m"
    }
}

// ---------------------------------------------------------------- el temps

data class WeatherPlace(val name: String, val lat: Double, val lon: Double, val where: String = "")
data class WeatherNow(val temp: Double, val feels: Double, val code: Int, val wind: Double, val isDay: Boolean, val time: String)
data class WeatherHour(val time: String, val temp: Double, val rainProb: Int, val rain: Double, val code: Int, val wind: Double, val gust: Double)
data class WeatherDay(
    val date: String, val code: Int, val max: Double, val min: Double, val rainProb: Int, val rainSum: Double,
    val gust: Double, val uv: Double, val sunrise: String, val sunset: String,
)
data class WeatherForecast(val place: WeatherPlace?, val now: WeatherNow?, val hours: List<WeatherHour>, val days: List<WeatherDay>)

/** El cel d'un codi WMO, en poques menes (les icones de la targeta). */
enum class Sky { CLEAR, PARTLY, CLOUDY, FOG, DRIZZLE, RAIN, SNOW, STORM }

object WeatherJson {
    fun sky(code: Int): Sky = when (code) {
        0 -> Sky.CLEAR
        1, 2 -> Sky.PARTLY
        3 -> Sky.CLOUDY
        45, 48 -> Sky.FOG
        in 51..57 -> Sky.DRIZZLE
        in 61..67, in 80..82 -> Sky.RAIN
        in 71..77, 85, 86 -> Sky.SNOW
        in 95..99 -> Sky.STORM
        else -> Sky.CLOUDY
    }

    private fun JSONObject?.d(k: String) = this?.optDouble(k, Double.NaN)?.takeIf { !it.isNaN() } ?: 0.0
    private fun JSONArray?.d(i: Int) = this?.optDouble(i, Double.NaN)?.takeIf { !it.isNaN() } ?: 0.0
    private fun JSONArray?.s(i: Int) = this?.optString(i).orEmpty()

    fun place(o: JSONObject?): WeatherPlace? {
        if (o == null) return null
        val name = o.optString("name")
        if (!o.has("lat") || !o.has("lon")) return null
        return WeatherPlace(name, o.optDouble("lat"), o.optDouble("lon"), o.optString("where"))
    }

    fun parse(raw: String): WeatherForecast {
        val o = JSONObject(raw)
        val place = place(o.optJSONObject("place"))
        val f = o.optJSONObject("forecast") ?: return WeatherForecast(place, null, emptyList(), emptyList())
        val c = f.optJSONObject("current")
        val now = c?.let {
            WeatherNow(it.d("temperature_2m"), it.d("apparent_temperature"), it.optInt("weather_code"), it.d("wind_speed_10m"),
                it.optInt("is_day", 1) == 1, it.optString("time"))
        }
        val h = f.optJSONObject("hourly")
        val ht = h?.optJSONArray("time") ?: JSONArray()
        val hours = (0 until ht.length()).map { i ->
            WeatherHour(ht.s(i), h?.optJSONArray("temperature_2m").d(i), h?.optJSONArray("precipitation_probability").d(i).toInt(),
                h?.optJSONArray("precipitation").d(i), h?.optJSONArray("weather_code").d(i).toInt(),
                h?.optJSONArray("wind_speed_10m").d(i), h?.optJSONArray("wind_gusts_10m").d(i))
        }
        val dd = f.optJSONObject("daily")
        val dt = dd?.optJSONArray("time") ?: JSONArray()
        val days = (0 until dt.length()).map { i ->
            WeatherDay(dt.s(i), dd?.optJSONArray("weather_code").d(i).toInt(), dd?.optJSONArray("temperature_2m_max").d(i),
                dd?.optJSONArray("temperature_2m_min").d(i), dd?.optJSONArray("precipitation_probability_max").d(i).toInt(),
                dd?.optJSONArray("precipitation_sum").d(i), dd?.optJSONArray("wind_gusts_10m_max").d(i),
                dd?.optJSONArray("uv_index_max").d(i), dd?.optJSONArray("sunrise").s(i), dd?.optJSONArray("sunset").s(i))
        }
        return WeatherForecast(place, now, hours, days)
    }

    fun places(raw: String): List<WeatherPlace> {
        val a = JSONObject(raw).optJSONArray("places") ?: return emptyList()
        return (0 until a.length()).mapNotNull { place(a.optJSONObject(it)) }.filter { it.name.isNotBlank() }
    }

    /** Les 24 hores que vénen, des de l'hora actual (la de «current», o la del rellotge). */
    fun next24(f: WeatherForecast, clock: LocalDateTime = LocalDateTime.now()): List<WeatherHour> {
        val from = runCatching { LocalDateTime.parse(f.now?.time.orEmpty()) }.getOrNull() ?: clock
        val hour = from.withMinute(0).withSecond(0).withNano(0)
        val i = f.hours.indexOfFirst { runCatching { !LocalDateTime.parse(it.time).isBefore(hour) }.getOrDefault(false) }
        return if (i < 0) f.hours.take(24) else f.hours.drop(i).take(24)
    }

    fun deg(t: Double): String = "${Math.round(t)}°"
}

// ---------------------------------------------------------------- API

fun GregalApi.getCalaix(wind: String): List<CalaixShelf> =
    CalaixJson.shelves(get("/api/winds/calaix?wind=" + java.net.URLEncoder.encode(wind, "UTF-8")).toString())

fun GregalApi.calaixAction(wind: String, shelf: String, action: String, key: String? = null, item: Map<String, Any?>? = null): List<CalaixShelf> {
    val body = JSONObject().put("wind", wind).put("shelf", shelf).put("action", action)
    if (key != null) body.put("key", key)
    if (item != null) body.put("item", JSONObject(item))
    return CalaixJson.shelves(post("/api/winds/calaix", body).toString())
}

fun GregalApi.getWeather(wind: String): WeatherForecast =
    WeatherJson.parse(get("/api/winds/weather?wind=" + java.net.URLEncoder.encode(wind, "UTF-8")).toString())

fun GregalApi.searchWeatherPlaces(q: String): List<WeatherPlace> =
    WeatherJson.places(get("/api/winds/weather/places?q=" + java.net.URLEncoder.encode(q, "UTF-8")).toString())

/** El vent del temps d'entre els teus (l'ofici «weather»). */
fun weatherWind(winds: List<Wind>): Wind? = winds.firstOrNull { it.kit == "weather" || it.specialty == "weather" }

/** Els membres d'una colla que ja tens (com a mínim un) però que encara no hi són tots. */
fun missingSquadMembers(squads: List<GregalApi.Squad>, winds: List<Wind>): Pair<GregalApi.Squad, List<GregalApi.SquadMember>>? {
    for (s in squads) {
        val mine = winds.filter { it.squad == s.id }
        if (mine.isEmpty()) continue
        val missing = s.members.filter { m -> mine.none { it.specialty == m.key } }
        if (missing.isNotEmpty()) return s to missing
    }
    return null
}

// ---------------------------------------------------------------- vistes: el temps

private fun skyIcon(sky: Sky, day: Boolean): ImageVector = when (sky) {
    Sky.CLEAR -> if (day) Icons.Rounded.WbSunny else Icons.Rounded.NightsStay
    Sky.PARTLY -> Icons.Rounded.FilterDrama
    Sky.CLOUDY -> Icons.Rounded.Cloud
    Sky.FOG -> Icons.Rounded.Dehaze
    Sky.DRIZZLE -> Icons.Rounded.Grain
    Sky.RAIN -> Icons.Rounded.Umbrella
    Sky.SNOW -> Icons.Rounded.AcUnit
    Sky.STORM -> Icons.Rounded.Thunderstorm
}

@Composable
private fun skyTint(sky: Sky, accent: Color): Color = when (sky) {
    Sky.CLEAR, Sky.PARTLY -> Sand
    Sky.RAIN, Sky.DRIZZLE, Sky.STORM -> Onada
    else -> accent
}

/**
 * La previsió del lloc principal: ara (temperatura gran, el cel, sensació i
 * vent), les 24 hores que vénen (corba de temperatura i barres de pluja) i, en
 * gran, els 7 dies. [compact]: la d'«Avui», sense els 7 dies.
 */
@Composable
internal fun WeatherCard(lang: String, f: WeatherForecast, accent: Color, compact: Boolean = false, onClick: (() -> Unit)? = null) {
    val now = f.now ?: return
    val sky = WeatherJson.sky(now.code)
    val shape = GregalShape.lg
    Column(
        Modifier.fillMaxWidth().clip(shape)
            .background(Brush.verticalGradient(listOf(accent.copy(alpha = .20f), SuperficieAlta.copy(alpha = .55f))))
            .border(1.dp, accent.copy(alpha = .30f), shape)
            .then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier)
            .padding(16.dp).testTag(if (compact) "weather-compact" else "weather-card"),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            Icon(Icons.Rounded.Place, null, tint = Muted, modifier = Modifier.size(14.dp))
            Text(f.place?.name.orEmpty(), color = Muted, style = GregalType.caption, maxLines = 1)
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
            Icon(skyIcon(sky, now.isDay), null, tint = skyTint(sky, accent), modifier = Modifier.size(if (compact) 40.dp else 52.dp))
            Text(WeatherJson.deg(now.temp), color = Ink, fontSize = if (compact) 40.sp else 54.sp, fontWeight = FontWeight.Light,
                letterSpacing = (-1.5).sp, modifier = Modifier.testTag("weather-temp"))
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(tr(lang, "app.weather.sky." + sky.name.lowercase()), color = Ink, style = GregalType.heading)
                Text(tr(lang, "app.weather.feels", WeatherJson.deg(now.feels)), color = Muted, style = GregalType.caption)
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                    Icon(Icons.Rounded.Air, null, tint = Muted, modifier = Modifier.size(13.dp))
                    Text(tr(lang, "app.weather.wind", Math.round(now.wind)), color = Muted, style = GregalType.caption)
                }
            }
        }
        val hours = remember(f) { WeatherJson.next24(f) }
        if (hours.size >= 2) WeatherChart(hours, accent, height = if (compact) 92 else 132)
        if (!compact && f.days.isNotEmpty()) {
            Box(Modifier.fillMaxWidth().height(1.dp).background(Line))
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                f.days.take(7).forEachIndexed { i, d ->
                    val date = runCatching { LocalDate.parse(d.date) }.getOrNull()
                    Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(3.dp),
                        modifier = Modifier.weight(1f)) {
                        Text(if (i == 0) tr(lang, "app.weather.today") else date?.let { CalaixFmt.weekday(lang, it) } ?: d.date,
                            color = if (i == 0) Ink else Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold, maxLines = 1)
                        val ds = WeatherJson.sky(d.code)
                        Icon(skyIcon(ds, true), null, tint = skyTint(ds, accent), modifier = Modifier.size(20.dp))
                        Text(WeatherJson.deg(d.max), color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                        Text(WeatherJson.deg(d.min), color = Muted, fontSize = 12.sp)
                        Text(if (d.rainProb > 0) "${d.rainProb}%" else " ", color = Onada, fontSize = 10.5.sp, fontWeight = FontWeight.Medium)
                    }
                }
            }
        }
    }
}

/** La corba de les 24 hores: la temperatura amb un degradat suau a sota i la probabilitat de pluja en barres. */
@Composable
private fun WeatherChart(hours: List<WeatherHour>, accent: Color, height: Int) {
    val measurer = rememberTextMeasurer()
    val ink = Ink; val muted = Muted; val rainColor = Onada
    val label = TextStyle(color = muted, fontSize = 10.sp)
    val tempLabel = TextStyle(color = ink, fontSize = 10.5.sp, fontWeight = FontWeight.SemiBold)
    Canvas(Modifier.fillMaxWidth().height(height.dp).testTag("weather-chart")) {
        val bottomLabels = 14.dp.toPx()
        val top = 14.dp.toPx()
        val rainH = size.height * .22f
        val chartBottom = size.height - bottomLabels - rainH - 4.dp.toPx()
        val w = size.width
        val n = hours.size
        val step = w / (n - 1).coerceAtLeast(1)
        val tMin = hours.minOf { it.temp }; val tMax = hours.maxOf { it.temp }
        val span = (tMax - tMin).coerceAtLeast(2.0)
        fun y(t: Double) = (chartBottom - ((t - tMin) / span * (chartBottom - top))).toFloat()
        // Barres de pluja, a baix.
        val barW = (step * .55f).coerceAtLeast(2f)
        hours.forEachIndexed { i, h ->
            if (h.rainProb <= 0) return@forEachIndexed
            val bh = rainH * (h.rainProb / 100f)
            val x = i * step
            drawRoundRect(rainColor.copy(alpha = .25f + .45f * h.rainProb / 100f),
                topLeft = Offset(x - barW / 2, size.height - bottomLabels - bh), size = Size(barW, bh),
                cornerRadius = CornerRadius(barW / 2))
        }
        // Corba suau (punts mitjos) i el degradat a sota.
        val line = Path()
        hours.forEachIndexed { i, h ->
            val x = i * step; val yy = y(h.temp)
            if (i == 0) line.moveTo(x, yy) else {
                val px = (i - 1) * step; val py = y(hours[i - 1].temp)
                line.cubicTo((px + x) / 2, py, (px + x) / 2, yy, x, yy)
            }
        }
        val fill = Path().apply { addPath(line); lineTo((n - 1) * step, chartBottom); lineTo(0f, chartBottom); close() }
        drawPath(fill, Brush.verticalGradient(listOf(accent.copy(alpha = .32f), accent.copy(alpha = 0f)), startY = top, endY = chartBottom))
        drawPath(line, accent, style = Stroke(width = 2.5.dp.toPx(), cap = StrokeCap.Round, join = StrokeJoin.Round))
        // Màxima i mínima marcades.
        val iMax = hours.indices.maxBy { hours[it].temp }; val iMin = hours.indices.minBy { hours[it].temp }
        listOf(iMax, iMin).distinct().forEach { i ->
            val x = i * step; val yy = y(hours[i].temp)
            drawCircle(accent, 3.5.dp.toPx(), Offset(x, yy))
            val t = measurer.measure(WeatherJson.deg(hours[i].temp), tempLabel)
            val tx = (x - t.size.width / 2).coerceIn(0f, w - t.size.width)
            val ty = if (i == iMax) (yy - t.size.height - 3.dp.toPx()).coerceAtLeast(0f) else (yy + 4.dp.toPx()).coerceAtMost(chartBottom - t.size.height)
            drawText(t, topLeft = Offset(tx, ty))
        }
        // Hores cada 3.
        hours.forEachIndexed { i, h ->
            if (i % 3 != 0) return@forEachIndexed
            val hh = h.time.substringAfter('T').take(2)
            val t = measurer.measure(hh, label)
            val tx = (i * step - t.size.width / 2).coerceIn(0f, w - t.size.width)
            drawText(t, topLeft = Offset(tx, size.height - t.size.height))
        }
    }
}

// ---------------------------------------------------------------- vistes: el calaix

/**
 * Tots els prestatges del calaix, cadascun amb la seva vista. [onAction]:
 * (prestatge, acció, clau, fila) — remove, primary, add.
 */
@Composable
internal fun CalaixSection(
    lang: String, windName: String, shelves: List<CalaixShelf>, accent: Color,
    onAction: (shelf: String, action: String, key: String?, item: Map<String, Any?>?) -> Unit,
    onSearchPlaces: (suspend (String) -> List<WeatherPlace>)? = null,
) {
    Column(Modifier.fillMaxWidth().testTag("calaix"), verticalArrangement = Arrangement.spacedBy(18.dp)) {
        Text(tr(lang, "app.calaix.help", windName), color = Muted, style = GregalType.bodySmall)
        if (shelves.isEmpty()) Text(tr(lang, "app.calaix.empty", windName), color = Muted, style = GregalType.body)
        shelves.forEach { sh ->
            Column(verticalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.testTag("shelf-${sh.name}")) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(sh.title.uppercase(), color = Muted, style = GregalType.overline, modifier = Modifier.weight(1f))
                    if (sh.items.isNotEmpty()) Text("${sh.items.size}", color = Faint, style = GregalType.caption)
                }
                if (sh.note.isNotBlank()) Text(sh.note, color = Muted, style = GregalType.caption)
                val remove: (Map<String, Any?>) -> Unit = { onAction(sh.name, "remove", CalaixJson.rowKey(it), null) }
                when {
                    sh.items.isEmpty() && sh.kind != "places" ->
                        Text(tr(lang, "app.calaix.emptyShelf.${sh.kind}", windName), color = Muted, style = GregalType.bodySmall,
                            modifier = Modifier.clip(GregalShape.md).background(SuperficieAlta.copy(alpha = .35f)).fillMaxWidth().padding(12.dp))
                    sh.kind == "releases" -> ReleasesView(lang, sh.items, accent, remove)
                    sh.kind == "recipes" -> RecipesView(lang, sh.items, accent, remove)
                    sh.kind == "places" -> PlacesView(lang, windName, sh, accent, onAction, onSearchPlaces)
                    sh.kind == "list" -> ListView(sh.items, accent, remove)
                    else -> TableView(sh.items, remove)
                }
            }
        }
    }
}

@Composable
private fun RemoveX(onClick: () -> Unit) {
    Box(Modifier.size(28.dp).clip(CircleShape).clickable(onClick = onClick).semantics { contentDescription = "×" },
        contentAlignment = Alignment.Center) { Icon(Icons.Rounded.Close, null, tint = Faint, modifier = Modifier.size(14.dp)) }
}

@Composable
private fun TableView(items: List<Map<String, Any?>>, remove: (Map<String, Any?>) -> Unit) {
    val cols = remember(items) { CalaixJson.columns(items) }
    Column(Modifier.fillMaxWidth().clip(GregalShape.md).background(SuperficieAlta.copy(alpha = .35f)).border(1.dp, Line, GregalShape.md)
        .horizontalScroll(rememberScrollState()).padding(vertical = 4.dp)) {
        Row(Modifier.padding(horizontal = 10.dp, vertical = 6.dp)) {
            cols.forEach { c -> Text(c, color = Muted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold, maxLines = 1,
                modifier = Modifier.width(if (cols.indexOf(c) == 0) 130.dp else 70.dp)) }
        }
        items.forEach { r ->
            Row(Modifier.padding(start = 10.dp), verticalAlignment = Alignment.CenterVertically) {
                cols.forEachIndexed { i, c ->
                    Text(CalaixJson.cell(r[c]), color = if (i == 0) Ink else Tinta, fontSize = 13.sp,
                        fontWeight = if (i == 0) FontWeight.Medium else FontWeight.Normal, maxLines = 1, overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.width(if (i == 0) 130.dp else 70.dp))
                }
                RemoveX { remove(r) }
            }
        }
    }
}

@Composable
private fun ReleasesView(lang: String, items: List<Map<String, Any?>>, accent: Color, remove: (Map<String, Any?>) -> Unit) {
    val groups = remember(items) { CalaixJson.releaseGroups(items) }
    val uri = LocalUriHandler.current
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        groups.forEach { g ->
            Text(if (g.key.isEmpty()) tr(lang, "app.calaix.noDate") else CalaixFmt.month(lang, g.key),
                color = if (g.past) Faint else Ink, style = GregalType.bodySmall.copy(fontWeight = FontWeight.SemiBold),
                modifier = Modifier.padding(top = 4.dp))
            g.rows.forEach { r ->
                val date = runCatching { LocalDate.parse(CalaixJson.str(r, "date").take(10)) }.getOrNull()
                val link = CalaixJson.str(r, "link", "url")
                val alpha = if (g.past) .5f else 1f
                Row(Modifier.fillMaxWidth().clip(GregalShape.md).background(SuperficieAlta.copy(alpha = .35f * alpha + .1f))
                    .clickable(enabled = link.isNotBlank()) { runCatching { uri.openUri(link) } }.padding(8.dp),
                    verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Column(Modifier.width(44.dp).clip(GregalShape.sm).background(accent.copy(alpha = .16f * alpha + .04f)).padding(vertical = 5.dp),
                        horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(date?.dayOfMonth?.toString() ?: "–", color = Ink.copy(alpha = alpha), fontSize = 16.sp, fontWeight = FontWeight.SemiBold)
                        Text(date?.let { CalaixFmt.weekday(lang, it) }.orEmpty(), color = Muted.copy(alpha = alpha), fontSize = 10.sp)
                    }
                    Column(Modifier.weight(1f)) {
                        Text(CalaixJson.str(r, "title", "name"), color = Ink.copy(alpha = alpha), fontSize = 14.sp, fontWeight = FontWeight.SemiBold,
                            maxLines = 2, overflow = TextOverflow.Ellipsis)
                        val sub = listOf(CalaixJson.str(r, "artist", "author"), CalaixJson.str(r, "type"), CalaixJson.str(r, "publisher"))
                            .filter { it.isNotBlank() }.joinToString(" · ")
                        if (sub.isNotBlank()) Text(sub, color = Muted.copy(alpha = alpha), style = GregalType.caption, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                    RemoveX { remove(r) }
                }
            }
        }
    }
}

@Composable
private fun RecipesView(lang: String, items: List<Map<String, Any?>>, accent: Color, remove: (Map<String, Any?>) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        items.forEach { r ->
            var open by remember(CalaixJson.rowKey(r)) { mutableStateOf(false) }
            Column(Modifier.fillMaxWidth().clip(GregalShape.lg).background(SuperficieAlta.copy(alpha = .45f)).border(1.dp, Line, GregalShape.lg)
                .clickable { open = !open }.padding(start = 14.dp, end = 6.dp, top = 10.dp, bottom = 10.dp).testTag("recipe"),
                verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text(CalaixJson.str(r, "title", "name"), color = Ink, style = GregalType.heading)
                        val meta = listOfNotNull(
                            CalaixJson.str(r, "servings").takeIf { it.isNotBlank() }?.let { tr(lang, "app.calaix.servings", it) },
                            CalaixJson.str(r, "time").takeIf { it.isNotBlank() },
                        ).joinToString(" · ")
                        if (meta.isNotBlank()) Text(meta, color = Muted, style = GregalType.caption)
                    }
                    RemoveX { remove(r) }
                }
                if (open) {
                    val ing = CalaixJson.strings(r, "ingredients")
                    if (ing.isNotEmpty()) {
                        Text(tr(lang, "app.calaix.ingredients").uppercase(), color = Muted, style = GregalType.overline, modifier = Modifier.padding(top = 4.dp))
                        ing.forEach { Text("•  $it", color = Tinta, style = GregalType.bodySmall) }
                    }
                    val steps = CalaixJson.strings(r, "steps")
                    if (steps.isNotEmpty()) {
                        Text(tr(lang, "app.calaix.steps").uppercase(), color = Muted, style = GregalType.overline, modifier = Modifier.padding(top = 4.dp))
                        steps.forEachIndexed { i, s ->
                            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                Text("${i + 1}", color = accent, fontSize = 13.sp, fontWeight = FontWeight.Bold, modifier = Modifier.width(16.dp))
                                Text(s, color = Tinta, style = GregalType.bodySmall, modifier = Modifier.weight(1f))
                            }
                        }
                    }
                    val tips = CalaixJson.str(r, "tips")
                    if (tips.isNotBlank()) Text("💡 $tips", color = Tinta, style = GregalType.bodySmall)
                    val foot = listOfNotNull(
                        CalaixJson.str(r, "source").takeIf { it.isNotBlank() },
                        CalaixJson.str(r, "reviewed").takeIf { it.isNotBlank() }?.let { tr(lang, "app.calaix.reviewed", CalaixFmt.day(lang, it)) },
                    ).joinToString(" · ")
                    if (foot.isNotBlank()) Text(foot, color = Faint, style = GregalType.caption)
                }
            }
        }
    }
}

@OptIn(ExperimentalLayoutApi::class, ExperimentalFoundationApi::class)
@Composable
private fun ListView(items: List<Map<String, Any?>>, accent: Color, remove: (Map<String, Any?>) -> Unit) {
    FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        items.forEach { r ->
            Row(Modifier.clip(GregalShape.pill).background(accent.copy(alpha = .12f)).padding(start = 12.dp),
                verticalAlignment = Alignment.CenterVertically) {
                Text(CalaixJson.str(r, "name", "title", "nom", "titol").ifBlank { CalaixJson.cell(r.values.firstOrNull()) },
                    color = Ink, fontSize = 13.sp, maxLines = 1)
                RemoveX { remove(r) }
            }
        }
    }
}

@Composable
private fun PlacesView(
    lang: String, windName: String, sh: CalaixShelf, accent: Color,
    onAction: (String, String, String?, Map<String, Any?>?) -> Unit,
    onSearchPlaces: (suspend (String) -> List<WeatherPlace>)?,
) {
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        if (sh.items.isEmpty()) Text(tr(lang, "app.calaix.emptyShelf.places", windName), color = Muted, style = GregalType.bodySmall)
        sh.items.forEach { r ->
            val primary = r["primary"] == true
            Row(Modifier.fillMaxWidth().clip(GregalShape.md).background(if (primary) accent.copy(alpha = .12f) else SuperficieAlta.copy(alpha = .35f))
                .padding(start = 12.dp, top = 4.dp, bottom = 4.dp), verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Icon(if (primary) Icons.Rounded.Star else Icons.Rounded.Place, null, tint = if (primary) accent else Muted, modifier = Modifier.size(18.dp))
                Text(CalaixJson.str(r, "name"), color = Ink, fontSize = 14.sp, fontWeight = FontWeight.Medium, modifier = Modifier.weight(1f))
                if (primary) Text(tr(lang, "app.calaix.primary"), color = accent, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
                else TextButton(onClick = { onAction(sh.name, "primary", CalaixJson.rowKey(r), null) }) {
                    Text(tr(lang, "app.calaix.makePrimary"), fontSize = 12.sp, color = accent)
                }
                RemoveX { onAction(sh.name, "remove", CalaixJson.rowKey(r), null) }
            }
        }
        if (onSearchPlaces != null) PlaceSearch(lang, accent, onSearchPlaces) { p ->
            onAction(sh.name, "add", p.name.lowercase(), mapOf("name" to p.name, "lat" to p.lat, "lon" to p.lon, "primary" to true))
        }
    }
}

@Composable
private fun PlaceSearch(lang: String, accent: Color, search: suspend (String) -> List<WeatherPlace>, onPick: (WeatherPlace) -> Unit) {
    var q by remember { mutableStateOf("") }
    var results by remember { mutableStateOf<List<WeatherPlace>>(emptyList()) }
    LaunchedEffect(q) {
        if (q.trim().length < 2) { results = emptyList(); return@LaunchedEffect }
        kotlinx.coroutines.delay(350)
        results = try { search(q.trim()) } catch (e: Exception) {
            if (e is kotlinx.coroutines.CancellationException) throw e
            emptyList()
        }
    }
    OutlinedTextField(q, { q = it }, singleLine = true, modifier = Modifier.fillMaxWidth().testTag("place-search"),
        placeholder = { Text(tr(lang, "app.calaix.addPlace"), fontSize = 13.sp, color = Muted) },
        leadingIcon = { Icon(Icons.Rounded.Place, null, tint = Muted) }, shape = GregalShape.sm,
        colors = OutlinedTextFieldDefaults.colors(focusedBorderColor = accent, unfocusedBorderColor = Line, cursorColor = accent))
    results.forEach { p ->
        Row(Modifier.fillMaxWidth().clip(GregalShape.sm).clickable { onPick(p); q = ""; results = emptyList() }.padding(10.dp),
            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Column(Modifier.weight(1f)) {
                Text(p.name, color = Ink, fontSize = 14.sp)
                if (p.where.isNotBlank()) Text(p.where, color = Muted, style = GregalType.caption)
            }
            Text(tr(lang, "app.calaix.follow"), color = accent, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
        }
    }
}
