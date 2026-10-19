package xyz.solanes.gregal

import android.graphics.Paint
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.unit.dp
import kotlin.math.abs
import kotlin.math.cos
import kotlin.math.sin

/** Canvas-only renderer: geometry and hit testing share the same circular bounds. */
internal fun DrawScope.drawNativePlot(spec: PlotSpec, view: PlotView, g: PlotGeometry,
                                     colors: List<Color>, pal: GregalPalette) {
    val pad = 10.dp.toPx()
    val w = (size.width - 2 * pad).coerceAtLeast(1f)
    val h = (size.height - 2 * pad).coerceAtLeast(1f)
    val diameter = minOf(w, h)
    val center = Offset(pad + w / 2, pad + h / 2)
    fun position(p: PlotPoint) = if (g.radial)
        center + Offset((p.x.toFloat() - 0.5f) * diameter, (p.y.toFloat() - 0.5f) * diameter)
        else Offset(pad + p.x.toFloat() * w, pad + p.y.toFloat() * h)
    val paint = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = pal.ink.toArgb(); textSize = 10.dp.toPx() }
    fun label(text: String, x: Float, y: Float) {
        drawContext.canvas.nativeCanvas.drawText(text.take(24), x, y, paint)
    }
    if (g.circular) {
        val hole = 0.55f
        val radius = diameter / 2
        g.slices.forEach { slice ->
            val selected = slice.index == view.selected
            val color = colors[slice.index % colors.size]
            if (spec.type == "doughnut") {
                // A stroked arc leaves a real transparent hole (no painted-over fake disk).
                val stroke = radius * (1 - hole)
                val arcRadius = radius - stroke / 2
                drawArc(color, slice.start.toFloat() - 90, slice.sweep.toFloat(), false,
                    center - Offset(arcRadius, arcRadius), Size(arcRadius * 2, arcRadius * 2), style = Stroke(stroke))
                if (selected) drawArc(pal.ink, slice.start.toFloat() - 90, slice.sweep.toFloat(), false,
                    center - Offset(radius, radius), Size(diameter, diameter), style = Stroke(3.dp.toPx()))
            } else {
                drawArc(color, slice.start.toFloat() - 90, slice.sweep.toFloat(), true,
                    center - Offset(radius, radius), Size(diameter, diameter))
                if (selected) drawArc(pal.ink, slice.start.toFloat() - 90, slice.sweep.toFloat(), true,
                    center - Offset(radius, radius), Size(diameter, diameter), style = Stroke(3.dp.toPx()))
            }
        }
        return
    }
    if (spec.type == "radar") {
        repeat(4) { ring ->
            val path = Path()
            spec.labels.indices.forEach { i ->
                val angle = i * 2 * Math.PI / spec.labels.size - Math.PI / 2
                val r = diameter / 2 * (ring + 1) / 4
                val p = center + Offset((cos(angle) * r).toFloat(), (sin(angle) * r).toFloat())
                if (i == 0) path.moveTo(p.x, p.y) else path.lineTo(p.x, p.y)
                if (ring == 3) {
                    drawLine(pal.lineHi, center, p, 1.dp.toPx())
                    label(spec.labels[i], (p.x - 8.dp.toPx()).coerceIn(pad, pad + w - 20.dp.toPx()),
                        (p.y + 10.dp.toPx()).coerceIn(pad + 10.dp.toPx(), pad + h))
                }
            }
            path.close()
            drawPath(path, pal.lineHi, style = Stroke(1.dp.toPx()))
        }
    } else {
        repeat(5) { i ->
            if (g.horizontal) {
                val x = pad + w * i / 4
                drawLine(pal.lineHi, Offset(x, pad), Offset(x, pad + h), 1.dp.toPx())
            } else {
                val y = pad + h * i / 4
                drawLine(pal.lineHi, Offset(pad, y), Offset(pad + w, y), 1.dp.toPx())
            }
        }
        if (spec.type in listOf("bar", "area", "horizontal-bar")) {
            if (g.horizontal) {
                val x = pad + (1 - g.zero).toFloat() * w
                drawLine(pal.muted, Offset(x, pad), Offset(x, pad + h), 1.dp.toPx())
            } else {
                val y = pad + g.zero.toFloat() * h
                drawLine(pal.muted, Offset(pad, y), Offset(pad + w, y), 1.dp.toPx())
            }
        }
    }
    g.visible.forEachIndexed { ordinal, series ->
        val points = g.points.filter { it.series == series }
        val color = colors[series % colors.size]
        if (spec.type in listOf("line", "area", "radar")) {
            val path = Path()
            points.forEachIndexed { i, point ->
                val p = position(point)
                if (i == 0) path.moveTo(p.x, p.y) else path.lineTo(p.x, p.y)
            }
            if (spec.type == "area" && points.isNotEmpty()) {
                val baseline = pad + g.zero.toFloat() * h
                path.lineTo(position(points.last()).x, baseline)
                path.lineTo(position(points.first()).x, baseline)
                path.close()
                drawPath(path, color.copy(alpha = 0.3f))
            } else if (spec.type == "radar") {
                path.close()
                drawPath(path, color.copy(alpha = 0.2f))
            }
            drawPath(path, color, style = Stroke(2.dp.toPx()))
        }
        points.forEach { point ->
            val p = position(point)
            when (spec.type) {
                "bar" -> {
                    val slot = w / g.range.count()
                    val barWidth = slot * 0.8f / g.visible.size.coerceAtLeast(1)
                    val zero = pad + g.zero.toFloat() * h
                    drawRect(color, Offset(p.x - slot * 0.4f + ordinal * barWidth, minOf(p.y, zero)),
                        Size(barWidth.coerceAtLeast(0.5f), abs(p.y - zero).coerceAtLeast(1f)))
                }
                "horizontal-bar" -> {
                    val slot = h / g.range.count()
                    val barHeight = slot * 0.8f / g.visible.size.coerceAtLeast(1)
                    val zero = pad + (1 - g.zero).toFloat() * w
                    drawRect(color, Offset(minOf(p.x, zero), p.y - slot * 0.4f + ordinal * barHeight),
                        Size(abs(p.x - zero).coerceAtLeast(1f), barHeight.coerceAtLeast(0.5f)))
                }
                else -> if (points.size <= 100 || spec.type == "scatter" || point.index == view.selected)
                    drawCircle(color, if (point.index == view.selected) 6.dp.toPx() else 3.dp.toPx(), p)
            }
            if (point.index == view.selected) drawCircle(pal.ink, 3.dp.toPx(), p)
        }
    }
    if (g.horizontal) {
        // Category labels live on Y; numeric endpoints are shown below the canvas.
        val stride = (g.range.count() / 12).coerceAtLeast(1)
        g.range.filterIndexed { i, _ -> i % stride == 0 }.forEach { i ->
            label(spec.labels[i], pad, pad + ((i - g.range.first + 0.5) / g.range.count() * h).toFloat())
        }
    }
}
