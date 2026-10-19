package xyz.solanes.gregal

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.semantics.*
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties

/** Data-only native plot: state is shared by the inline and enlarged view. */
@Composable
fun PlotCard(spec: PlotSpec, lang: String = "ca") {
    var view by rememberSaveable(spec) { mutableStateOf(PlotView()) }
    var enlarged by rememberSaveable(spec) { mutableStateOf(false) }
    PlotContent(spec, lang, view, { view = it }, false, { enlarged = true })
    if (enlarged) {
        Dialog(onDismissRequest = { enlarged = false }, properties = DialogProperties(usePlatformDefaultWidth = false)) {
            Surface(Modifier.fillMaxWidth().padding(12.dp), shape = RoundedCornerShape(18.dp), color = LocalPal.current.surface) {
                Column(Modifier.heightIn(max = 680.dp).verticalScroll(rememberScrollState())) {
                    PlotContent(spec, lang, view, { view = it }, true, { enlarged = false })
                }
            }
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun PlotContent(spec: PlotSpec, lang: String, view: PlotView, change: (PlotView) -> Unit,
                        enlarged: Boolean, expandOrClose: () -> Unit) {
    val pal = LocalPal.current
    val colors = listOf(pal.sea, pal.sand, pal.green, pal.lila, pal.red, pal.onada)
    // Selection does not rebuild the geometry or restart a running drag gesture.
    val geometry = remember(spec, view.hidden, view.start, view.window) { PlotGeometry(spec, view) }
    val latestView by rememberUpdatedState(view)
    val latestChange by rememberUpdatedState(change)
    val n = spec.labels.size
    val previous = tr(lang, "app.plot.previous")
    val next = tr(lang, "app.plot.next")
    Card(Modifier.fillMaxWidth().padding(vertical = 6.dp), shape = RoundedCornerShape(14.dp),
        colors = CardDefaults.cardColors(containerColor = pal.surfaceHi)) {
        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(spec.title, color = pal.ink, fontWeight = FontWeight.SemiBold, fontSize = 16.sp)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                TextButton(onClick = { change(view.zoom(n, true)) }, enabled = !geometry.radial && geometry.range.count() > 1) {
                    Text("+", modifier = Modifier.semantics { contentDescription = trPlotZoomIn(lang) })
                }
                TextButton(onClick = { change(view.zoom(n, false)) }, enabled = !geometry.radial && geometry.range.count() < n) {
                    Text("−", modifier = Modifier.semantics { contentDescription = trPlotZoomOut(lang) })
                }
                TextButton(onClick = { change(view.reset()) }) { Text(tr(lang, "app.plot.reset")) }
                TextButton(onClick = expandOrClose) { Text(tr(lang, if (enlarged) "btn.close" else "app.plot.expand")) }
            }
            Text(tr(lang, "app.plot.help"), color = pal.muted, fontSize = 12.sp)
            if (!geometry.radial && !geometry.horizontal) Text(geometry.yAxis.max.toString(), color = pal.muted, fontSize = 11.sp, maxLines = 1)
            val description = "${spec.title} · ${spec.type} · ${geometry.range.first + 1}–${geometry.range.last + 1} / $n"
            Canvas(Modifier.fillMaxWidth().height(if (enlarged) 320.dp else 200.dp)
                .semantics {
                    contentDescription = description
                    customActions = listOf(
                        CustomAccessibilityAction(previous) { change(view.select((view.selected ?: 1) - 1, n)); true },
                        CustomAccessibilityAction(next) { change(view.select((view.selected ?: -1) + 1, n)); true },
                    )
                }
                .pointerInput(geometry) {
                    detectTapGestures { position ->
                        val pad = 10.dp.toPx()
                        val w = (size.width - 2 * pad).coerceAtLeast(1f)
                        val h = (size.height - 2 * pad).coerceAtLeast(1f)
                        geometry.nearest(((position.x - pad) / w).toDouble(), ((position.y - pad) / h).toDouble(), w.toDouble(), h.toDouble())
                            ?.let { latestChange(latestView.select(it, n)) }
                    }
                }
                .pointerInput(geometry) {
                    detectHorizontalDragGestures(onDragStart = { position ->
                        val pad = 10.dp.toPx()
                        geometry.nearest(((position.x - pad) / (size.width - 2 * pad).coerceAtLeast(1f)).toDouble(),
                            ((position.y - pad) / (size.height - 2 * pad).coerceAtLeast(1f)).toDouble(), (size.width - 2 * pad).coerceAtLeast(1f).toDouble(), (size.height - 2 * pad).coerceAtLeast(1f).toDouble())
                            ?.let { latestChange(latestView.select(it, n)) }
                    }) { event, _ ->
                        event.consume()
                        val pad = 10.dp.toPx()
                        val w = (size.width - 2 * pad).coerceAtLeast(1f)
                        val h = (size.height - 2 * pad).coerceAtLeast(1f)
                        geometry.nearest(((event.position.x - pad) / w).toDouble(), ((event.position.y - pad) / h).toDouble(), w.toDouble(), h.toDouble())
                            ?.let { latestChange(latestView.select(it, n)) }
                    }
                }) {
                drawNativePlot(spec, view, geometry, colors, pal)
            }
            if (!geometry.radial && !geometry.horizontal) Text(geometry.yAxis.min.toString(), color = pal.muted, fontSize = 11.sp, maxLines = 1)
            if (!geometry.radial) Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Text(if (geometry.horizontal) geometry.yAxis.min.toString() else spec.labels[geometry.range.first], color = pal.muted, fontSize = 11.sp, maxLines = 1,
                    overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                Text(if (geometry.horizontal) geometry.yAxis.max.toString() else spec.labels[geometry.range.last], color = pal.muted, fontSize = 11.sp, maxLines = 1,
                    overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
            }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                TextButton(onClick = { change(view.select((view.selected ?: 1) - 1, n)) }, enabled = view.selected != 0) { Text(previous) }
                TextButton(onClick = { change(view.select((view.selected ?: -1) + 1, n)) }, enabled = view.selected != n - 1) { Text(next) }
            }
            view.selected?.let { index ->
                Column(Modifier.semantics { liveRegion = LiveRegionMode.Polite }) {
                    Text("${spec.labels[index]} · ${index + 1}/$n", color = pal.ink, fontWeight = FontWeight.Medium)
                    geometry.visible.filter { !geometry.circular || index !in view.hidden }.forEach { series ->
                        Text("${spec.series[series].label}: ${spec.series[series].values[index]}", color = pal.ink, fontSize = 13.sp)
                    }
                }
            }
            Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                (if (geometry.circular) spec.labels else spec.series.map { it.label }).forEachIndexed { index, label ->
                    FilterChip(selected = index !in view.hidden, onClick = { change(view.toggle(index)) }, label = {
                        Text("${index + 1} · $label", color = colors[index % colors.size])
                    })
                }
            }
            if (if (geometry.circular) geometry.slices.isEmpty() else geometry.visible.isEmpty()) Text(tr(lang, "app.plot.hidden"), color = pal.muted, fontSize = 12.sp)
        }
    }
}

// Semantics lambdas are not composable; resolve the label before assigning it.
private fun trPlotZoomIn(lang: String) = tr(lang, "app.plot.zoomIn")
private fun trPlotZoomOut(lang: String) = tr(lang, "app.plot.zoomOut")
