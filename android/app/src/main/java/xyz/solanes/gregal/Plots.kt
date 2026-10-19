package xyz.solanes.gregal

data class PlotSeries(val label: String, val values: List<Double>)
data class PlotSpec(val title: String, val type: String, val labels: List<String>, val series: List<PlotSeries>)

/** Invalid/unfinished plot blocks remain ordinary copyable code. */
fun parsePlot(language: String, code: String): PlotSpec? {
    if (language != "gregal-plot" || code.length > 4_000_000) return null
    return try {
        val root = PlotJson(code).read() as? Map<*, *> ?: return null
        val title = root["title"] as? String ?: return null
        val type = root["type"] as? String ?: return null
        val labels = (root["labels"] as? List<*>)?.map { it as? String ?: return null } ?: return null
        val series = (root["series"] as? List<*>)?.map {
            val s = it as? Map<*, *> ?: return null
            val label = s["label"] as? String ?: return null
            val values = (s["values"] as? List<*>)?.map { v -> v as? Double ?: return null } ?: return null
            PlotSeries(label, values)
        } ?: return null
        require(title.isNotBlank() && title.toByteArray(Charsets.UTF_8).size <= 200)
        require(type in listOf("line", "bar", "scatter", "area", "horizontal-bar", "pie", "doughnut", "radar"))
        require(labels.size in 1..2000 && labels.all { it.toByteArray(Charsets.UTF_8).size <= 200 })
        require(series.size in 1..20 && series.all {
            it.label.toByteArray(Charsets.UTF_8).size <= 200 && it.values.size == labels.size
        })
        if (type == "pie" || type == "doughnut") {
            require(series.size == 1 && series.single().values.all { it >= 0.0 } &&
                series.single().values.any { it > 0.0 })
        }
        if (type == "radar") require(labels.size in 3..32 && series.all { s -> s.values.all { it >= 0.0 } })
        if (type == "scatter") require(labels.all {
            JSON_NUMBER.matches(it.trim()) && it.trim().toDoubleOrNull()?.isFinite() == true
        })
        PlotSpec(title, type, labels, series)
    } catch (_: IllegalArgumentException) { null }
}

/** Small strict JSON reader: no Android runtime, scripts, I/O or coercion. */
private class PlotJson(private val src: String) {
    private var p = 0
    fun read(): Any? {
        val value = value(0)
        ws()
        require(p == src.length)
        return value
    }
    private fun ws() { while (p < src.length && src[p] in " \n\r\t") p++ }
    private fun take(c: Char): Boolean {
        ws()
        if (p < src.length && src[p] == c) { p++; return true }
        return false
    }
    private fun value(depth: Int): Any? {
        require(depth < 16)
        ws()
        require(p < src.length)
        return when (src[p]) {
            '"' -> string()
            '{' -> {
                p++
                val result = linkedMapOf<String, Any?>()
                if (!take('}')) {
                    do {
                        ws()
                        val key = string()
                        require(!result.containsKey(key) && take(':'))
                        result[key] = value(depth + 1)
                    } while (take(','))
                    require(take('}'))
                }
                result
            }
            '[' -> {
                p++
                val result = mutableListOf<Any?>()
                if (!take(']')) {
                    do { result += value(depth + 1) } while (take(','))
                    require(take(']'))
                }
                result
            }
            't' -> literal("true", true)
            'f' -> literal("false", false)
            'n' -> literal("null", null)
            else -> {
                val start = p
                while (p < src.length && src[p] in "-+0123456789.eE") p++
                val token = src.substring(start, p)
                require(JSON_NUMBER.matches(token))
                token.toDoubleOrNull()?.takeIf { it.isFinite() } ?: throw IllegalArgumentException()
            }
        }
    }
    private fun literal(word: String, result: Any?): Any? {
        require(src.startsWith(word, p)); p += word.length; return result
    }
    private fun string(): String {
        require(p < src.length && src[p++] == '"')
        val result = StringBuilder()
        while (p < src.length) {
            val c = src[p++]
            when {
                c == '"' -> return result.toString()
                c == '\\' -> {
                    require(p < src.length)
                    result.append(when (val escaped = src[p++]) {
                        '"', '\\', '/' -> escaped
                        'b' -> '\b'
                        'f' -> '\u000C'
                        'n' -> '\n'
                        'r' -> '\r'
                        't' -> '\t'
                        'u' -> {
                            require(p + 4 <= src.length)
                            val hex = src.substring(p, p + 4)
                            require(hex.all { it in "0123456789abcdefABCDEF" })
                            p += 4
                            hex.toInt(16).toChar()
                        }
                        else -> throw IllegalArgumentException()
                    })
                }
                else -> { require(c >= ' '); result.append(c) }
            }
        }
        throw IllegalArgumentException()
    }
}

data class PlotView(
    val hidden: Set<Int> = emptySet(),
    val selected: Int? = null,
    val start: Int = 0,
    val window: Int = Int.MAX_VALUE,
) : java.io.Serializable {
    fun range(count: Int): IntRange {
        if (count <= 0) return IntRange.EMPTY
        val size = window.coerceIn(1, count)
        val first = start.coerceIn(0, count - size)
        return first until first + size
    }
    fun zoom(count: Int, inward: Boolean): PlotView {
        if (count <= 0) return this
        val current = range(count)
        val size = if (inward) ((current.count() + 1) / 2).coerceAtLeast(1)
            else (current.count() * 2).coerceAtMost(count)
        val anchor = selected?.takeIf { it in current } ?: (current.first + current.count() / 2)
        return copy(start = (anchor - size / 2).coerceIn(0, count - size), window = size)
    }
    fun select(index: Int, count: Int): PlotView {
        if (count <= 0) return this
        val picked = index.coerceIn(0, count - 1)
        val range = range(count)
        return copy(selected = picked, start = if (picked in range) range.first
            else (picked - range.count() / 2).coerceIn(0, count - range.count()))
    }
    fun reset(): PlotView = PlotView()
    fun toggle(series: Int): PlotView = copy(hidden = if (series in hidden) hidden - series else hidden + series)
}

data class PlotPoint(val series: Int, val index: Int, val x: Double, val y: Double)

data class PlotSlice(val index: Int, val start: Double, val sweep: Double)

class PlotGeometry(private val spec: PlotSpec, view: PlotView) {
    val circular = spec.type == "pie" || spec.type == "doughnut"
    val radial = circular || spec.type == "radar"
    val horizontal = spec.type == "horizontal-bar"
    val range = if (radial) spec.labels.indices else view.range(spec.labels.size)
    // Circular legends toggle categories, not the single series.
    val categories = range.filter { !circular || it !in view.hidden }
    val visible = spec.series.indices.filter { circular || it !in view.hidden }
    private val values = visible.flatMap { s -> categories.map { spec.series[s].values[it] } }
    private val includesZero = spec.type in listOf("bar", "horizontal-bar", "area", "radar")
    val yAxis = PlotAxis(
        (values.minOrNull() ?: 0.0).let { if (includesZero) minOf(0.0, it) else it },
        (values.maxOrNull() ?: 0.0).let { if (includesZero) maxOf(0.0, it) else it },
    )
    val zero = 1.0 - yAxis.fraction(0.0)
    private val scatter = spec.type == "scatter"
    private val xs = if (scatter) range.map { spec.labels[it].trim().toDouble() } else emptyList()
    private val xAxis = PlotAxis(xs.minOrNull() ?: 0.0, xs.maxOrNull() ?: 0.0)
    val slices: List<PlotSlice> = if (circular) {
        val max = values.maxOrNull() ?: 0.0
        // Divide before summing: multiple finite MAX_VALUEs must not overflow.
        val weights = categories.map { if (max > 0.0) spec.series[0].values[it] / max else 0.0 }
        val sum = weights.sum()
        var start = 0.0
        categories.mapIndexedNotNull { i, category ->
            val sweep = if (sum > 0.0) weights[i] / sum * 360.0 else 0.0
            PlotSlice(category, start, sweep).also { start += sweep }.takeIf { sweep > 0.0 }
        }
    } else emptyList()
    val points = if (circular) emptyList() else visible.flatMap { s -> range.map { i ->
        val category = (i - range.first + 0.5) / range.count()
        val value = spec.series[s].values[i]
        when {
            spec.type == "radar" -> {
                val radius = if (yAxis.max > 0.0) value / yAxis.max * 0.5 else 0.0
                val angle = i * 2.0 * kotlin.math.PI / spec.labels.size - kotlin.math.PI / 2
                PlotPoint(s, i, 0.5 + radius * kotlin.math.cos(angle), 0.5 + radius * kotlin.math.sin(angle))
            }
            horizontal -> PlotPoint(s, i, yAxis.fraction(value), category)
            else -> PlotPoint(s, i, if (scatter) xAxis.fraction(spec.labels[i].trim().toDouble()) else category,
                1.0 - yAxis.fraction(value))
        }
    } }
    fun nearest(x: Double, y: Double, width: Double, height: Double): Int? {
        val diameter = minOf(width, height)
        if (radial) {
            val dx = (x - 0.5) * width
            val dy = (y - 0.5) * height
            val radius = kotlin.math.hypot(dx, dy) / (diameter / 2)
            if (radius > 1.0 || (spec.type == "doughnut" && radius < 0.55)) return null
            if (circular) {
                val angle = (Math.toDegrees(kotlin.math.atan2(dy, dx)) + 450.0) % 360.0
                return slices.firstOrNull { angle >= it.start && angle < it.start + it.sweep }?.index
            }
        }
        return points.minByOrNull {
            val dx = if (radial) (it.x - 0.5) * diameter - (x - 0.5) * width else (it.x - x) * width
            val dy = if (radial) (it.y - 0.5) * diameter - (y - 0.5) * height
                else if (horizontal || scatter) (it.y - y) * height else 0.0
            (if (horizontal) 0.0 else dx * dx) + dy * dy
        }?.index
    }
}

data class PlotAxis(val min: Double, val max: Double) {
    fun fraction(value: Double): Double {
        if (min == max) return 0.5
        // Normalize before subtracting: MAX_VALUE - (-MAX_VALUE) would overflow.
        val scale = maxOf(kotlin.math.abs(min), kotlin.math.abs(max))
        val low = min / scale
        val high = max / scale
        return ((value / scale - low) / (high - low)).coerceIn(0.0, 1.0)
    }
}

private val JSON_NUMBER = Regex("-?(0|[1-9][0-9]*)(\\.[0-9]+)?([eE][+-]?[0-9]+)?")
