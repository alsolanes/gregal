package xyz.solanes.gregal

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Especificació d'un gràfic que el vent demana amb un bloc ```chart. */
data class ChartSpec(
    val type: String,
    val title: String,
    val labels: List<String>,
    val series: List<ChartSeries>,
)

data class ChartSeries(val name: String, val values: List<Float>)

/** Parseja l'JSON del bloc chart. Torna null si no és vàlid (es mostra com a codi). Pura: testejable. */
fun parseChartSpec(json: String): ChartSpec? {
    return try {
        val type = jsonStrField(json, "type")?.lowercase() ?: return null
        if (type != "bar" && type != "line" && type != "pie") return null
        val labels = jsonStrArray(json, "labels") ?: emptyList()
        val seriesRaw = jsonArrayRaw(json, "series") ?: return null
        val series = splitObjects(seriesRaw).map { obj ->
            val vals = jsonNumArray(obj, "values") ?: return null
            ChartSeries(jsonStrField(obj, "name") ?: "", vals)
        }
        if (series.isEmpty() || series.any { it.values.isEmpty() }) return null
        if (type == "pie" && (series.size != 1 || labels.size != series.first().values.size)) return null
        ChartSpec(type, jsonStrField(json, "title") ?: "", labels, series)
    } catch (_: Exception) {
        null
    }
}

/** Camp "clau":"text" de primer nivell (sense org.json: als tests és stub). */
private fun jsonStrField(src: String, key: String): String? {
    val re = Regex("\"" + Regex.escape(key) + "\"\\s*:\\s*\"((?:[^\"\\\\]|\\\\.)*)\"")
    return re.find(src)?.groups?.get(1)?.value
        ?.replace("\\\"", "\"")?.replace("\\\\", "\\")
}

/** Primer [...] d'una clau, amb el contingut cru (respecta comes dins de comes). */
private fun jsonArrayRaw(src: String, key: String): String? {
    val start = Regex("\"" + Regex.escape(key) + "\"\\s*:\\s*\\[").find(src) ?: return null
    var depth = 0
    var inStr = false
    var esc = false
    for (i in start.range.last + 1 until src.length) {
        val c = src[i]
        if (inStr) {
            if (esc) esc = false
            else if (c == '\\') esc = true
            else if (c == '"') inStr = false
        } else {
            if (c == '"') inStr = true
            else if (c == '[') depth++
            else if (c == ']') {
                if (depth == 0) return src.substring(start.range.last + 1, i)
                depth--
            }
        }
    }
    return null
}

/** Llista de cadenes ["a","b"] d'una clau. */
private fun jsonStrArray(src: String, key: String): List<String>? {
    val raw = jsonArrayRaw(src, key) ?: return null
    if (raw.isBlank()) return emptyList()
    val out = mutableListOf<String>()
    val re = Regex("\"((?:[^\"\\\\]|\\\\.)*)\"")
    re.findAll(raw).forEach { out += it.groups[1]!!.value.replace("\\\"", "\"").replace("\\\\", "\\") }
    return out
}

/** Llista de nombres [1, 2.5] dins d'un objecte de sèrie. */
private fun jsonNumArray(obj: String, key: String): List<Float>? {
    val raw = jsonArrayRaw(obj, key) ?: return null
    if (raw.isBlank()) return emptyList()
    return raw.split(",").map { it.trim().toFloat() }
}

/** Parteix [{...},{...}] en objectes (per profunditat de claus). */
private fun splitObjects(raw: String): List<String> {
    val out = mutableListOf<String>()
    var depth = 0
    var start = -1
    var inStr = false
    var esc = false
    for (i in raw.indices) {
        val c = raw[i]
        if (inStr) {
            if (esc) esc = false
            else if (c == '\\') esc = true
            else if (c == '"') inStr = false
        } else {
            if (c == '"') inStr = true
            else if (c == '{') {
                if (depth == 0) start = i
                depth++
            } else if (c == '}') {
                depth--
                if (depth == 0 && start >= 0) out += raw.substring(start, i + 1)
            }
        }
    }
    return out
}

/** Màxim global (per escalar eixos). Pura: testejable. */
fun chartMax(spec: ChartSpec): Float = spec.series.maxOf { s -> s.values.maxOrNull() ?: 0f }.takeIf { it > 0 } ?: 1f

/** Paleta derivada de l'accent del vent: gira el to per sèrie. Pura: testejable (sense android.graphics). */
fun chartPalette(accent: Color, n: Int): List<Color> {
    if (n <= 1) return listOf(accent)
    val (h, s, v) = rgbToHsv(accent.red, accent.green, accent.blue)
    return List(n) { i ->
        val (r, g, b) = hsvToRgb((h + i * (360f / n)) % 360f, s, v)
        Color(r, g, b, accent.alpha)
    }
}

private fun rgbToHsv(r: Float, g: Float, b: Float): Triple<Float, Float, Float> {
    val mx = maxOf(r, g, b)
    val mn = minOf(r, g, b)
    val d = mx - mn
    val h = when {
        d == 0f -> 0f
        mx == r -> 60f * (((g - b) / d) % 6)
        mx == g -> 60f * ((b - r) / d + 2)
        else -> 60f * ((r - g) / d + 4)
    }
    return Triple((h + 360f) % 360f, if (mx == 0f) 0f else d / mx, mx)
}

private fun hsvToRgb(h: Float, s: Float, v: Float): Triple<Float, Float, Float> {
    val c = v * s
    val x = c * (1 - kotlin.math.abs((h / 60f) % 2 - 1))
    val m = v - c
    val (r, g, b) = when {
        h < 60 -> Triple(c, x, 0f)
        h < 120 -> Triple(x, c, 0f)
        h < 180 -> Triple(0f, c, x)
        h < 240 -> Triple(0f, x, c)
        h < 300 -> Triple(x, 0f, c)
        else -> Triple(c, 0f, x)
    }
    return Triple(r + m, g + m, b + m)
}

/** Bloc de gràfic: barres, línies o formatget, amb títol i llegenda. */
@Composable
fun ChartBlock(spec: ChartSpec, accent: Color, dark: Boolean) {
    val ink = if (dark) Color(0xFFF2EDE3) else Ink
    val grid = ink.copy(alpha = 0.14f)
    val palette = remember(accent, spec.series.size) { chartPalette(accent, spec.series.size.coerceAtLeast(1)) }
    Column(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp))
            .background(if (dark) Color(0xFF23201A) else Color(0xFFF6F1E7))
            .padding(12.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        if (spec.title.isNotBlank()) Text(spec.title, fontSize = 14.sp, fontWeight = androidx.compose.ui.text.font.FontWeight.Bold, color = ink)
        Canvas(Modifier.fillMaxWidth().height(190.dp)) {
            when (spec.type) {
                "pie" -> drawPie(this, spec, palette)
                "line" -> drawLines(this, spec, palette, grid, ink)
                else -> drawBars(this, spec, palette, grid, ink)
            }
        }
        ChartLegend(spec, palette, ink)
    }
}

private fun DrawScope.label(text: String, x: Float, y: Float, color: Color) {
    drawContext.canvas.nativeCanvas.drawText(
        text, x, y,
        android.graphics.Paint().apply {
            this.color = color.toArgb()
            textSize = 26f
            textAlign = android.graphics.Paint.Align.CENTER
        },
    )
}

private fun drawBars(scope: DrawScope, spec: ChartSpec, palette: List<Color>, grid: Color, ink: Color) {
    val max = chartMax(spec)
    val n = spec.series.first().values.size
    val groups = (0 until n)
    val gw = scope.size.width / n.coerceAtLeast(1)
    val bw = (gw / (spec.series.size + 1)).coerceAtMost(56f)
    scope.drawLine(grid, Offset(0f, 0f), Offset(scope.size.width, 0f), 2f)
    scope.drawLine(grid, Offset(0f, scope.size.height - 34f), Offset(scope.size.width, scope.size.height - 34f), 2f)
    groups.forEach { gi ->
        spec.series.forEachIndexed { si, s ->
            val v = s.values.getOrNull(gi) ?: 0f
            val h = (v / max) * (scope.size.height - 60f)
            val x = gi * gw + (gw - bw * spec.series.size) / 2 + si * bw
            scope.drawRoundRect(palette[si % palette.size], Offset(x, scope.size.height - 34f - h), Size(bw * 0.86f, h), CornerRadius(8f, 8f))
        }
        if (gi < spec.labels.size) scope.label(spec.labels[gi].take(8), gi * gw + gw / 2, scope.size.height - 8f, ink)
    }
}

private fun drawLines(scope: DrawScope, spec: ChartSpec, palette: List<Color>, grid: Color, ink: Color) {
    val max = chartMax(spec)
    val n = spec.series.first().values.size
    val h = scope.size.height - 60f
    scope.drawLine(grid, Offset(0f, h), Offset(scope.size.width, h), 2f)
    fun px(i: Int) = if (n <= 1) scope.size.width / 2 else i * scope.size.width / (n - 1)
    spec.series.forEachIndexed { si, s ->
        val pts = s.values.mapIndexed { i, v -> Offset(px(i), h - (v / max) * (h - 20f)) }
        if (pts.size >= 2) {
            val path = Path().apply {
                moveTo(pts.first().x, pts.first().y)
                pts.drop(1).forEach { lineTo(it.x, it.y) }
            }
            scope.drawPath(path, palette[si % palette.size], style = Stroke(6f))
        }
        pts.forEach { scope.drawCircle(palette[si % palette.size], 9f, it) }
    }
    if (n > 1) (0 until n).forEach { i ->
        if (i < spec.labels.size) scope.label(spec.labels[i].take(8), px(i).coerceIn(30f, scope.size.width - 30f), scope.size.height - 8f, ink)
    }
}

private fun drawPie(scope: DrawScope, spec: ChartSpec, palette: List<Color>) {
    val vals = spec.series.first().values
    val total = vals.sum().takeIf { it > 0 } ?: 1f
    val r = minOf(scope.size.width, scope.size.height - 30f) / 2
    val c = Offset(scope.size.width / 2, (scope.size.height - 30f) / 2)
    var start = -90f
    vals.forEachIndexed { i, v ->
        val sweep = v / total * 360f
        scope.drawArc(palette[i % palette.size], start, sweep, true, c - Offset(r, r), Size(r * 2, r * 2))
        val mid = Math.toRadians((start + sweep / 2).toDouble())
        val pct = (v / total * 100).toInt()
        if (pct >= 8) scope.label(
            "$pct%", (c.x + kotlin.math.cos(mid) * r * 0.62).toFloat(),
            (c.y + kotlin.math.sin(mid) * r * 0.62 + 9).toFloat(), Color.White,
        )
        start += sweep
    }
}

@Composable
private fun ChartLegend(spec: ChartSpec, palette: List<Color>, ink: Color) {
    if (spec.type == "pie") {
        Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
            spec.labels.forEachIndexed { i, l ->
                ChartLegendRow(palette[i % palette.size], l, spec.series.first().values.getOrNull(i), ink)
            }
        }
    } else {
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
            spec.series.forEachIndexed { i, s ->
                ChartLegendRow(palette[i % palette.size], s.name.ifBlank { "Sèrie ${i + 1}" }, null, ink)
            }
        }
    }
}

@Composable
private fun ChartLegendRow(color: Color, name: String, value: Float?, ink: Color) {
    Row(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(10.dp).clip(RoundedCornerShape(3.dp)).background(color))
        Text(name, fontSize = 12.sp, color = ink)
        if (value != null) Text(fmtNum(value), fontSize = 12.sp, fontWeight = androidx.compose.ui.text.font.FontWeight.Bold, color = ink)
    }
}

/** Formata un nombre: enters sense decimals. Pura: testejable. */
internal fun fmtNum(v: Float): String = if (v == v.toInt().toFloat()) v.toInt().toString() else String.format("%.1f", v)
