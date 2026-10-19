package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

class PlotNewGeometryTest {
    private fun spec(type: String, values: List<Double> = listOf(1.0, 1.0, 2.0)) =
        PlotSpec("T", type, listOf("A", "B", "C"), listOf(PlotSeries("S", values)))

    @Test fun pieHitTestingUsesAnglesAndRealCircularBounds() {
        val g = PlotGeometry(spec("pie", listOf(Double.MAX_VALUE, Double.MAX_VALUE, 0.0)), PlotView())
        assertEquals(0, g.nearest(0.6, 0.5, 400.0, 200.0))
        assertEquals(1, g.nearest(0.4, 0.5, 400.0, 200.0))
        assertNull(g.nearest(0.9, 0.5, 400.0, 200.0))
        assertNull(g.nearest(0.5, -0.1, 400.0, 200.0))
        val hidden = PlotGeometry(spec("pie"), PlotView(hidden = setOf(0)))
        assertEquals(1, hidden.nearest(0.6, 0.2, 400.0, 200.0))
        assertNull(PlotGeometry(spec("pie"), PlotView(hidden = setOf(0, 1, 2))).nearest(0.6, 0.5, 400.0, 200.0))
    }

    @Test fun doughnutHoleDoesNotSelectASlice() {
        val g = PlotGeometry(spec("doughnut"), PlotView(start = 1, window = 1))
        assertNull(g.nearest(0.5, 0.5, 400.0, 200.0))
        assertEquals(0, g.nearest(0.65, 0.3, 400.0, 200.0))
        assertEquals(3, g.range.count())
    }

    @Test fun radarNormalizesFromZeroAndZeroOnlyCollapsesToCenter() {
        val zero = PlotGeometry(spec("radar", listOf(0.0, 0.0, 0.0)), PlotView(window = 1))
        assertEquals(3, zero.range.count())
        assertTrue(zero.points.all { it.x == 0.5 && it.y == 0.5 })
        val g = PlotGeometry(spec("radar", listOf(Double.MAX_VALUE, 0.0, Double.MAX_VALUE)), PlotView())
        assertEquals(0.5, g.points[0].x, 0.000001)
        assertEquals(0.0, g.points[0].y, 0.000001)
        assertEquals(0, g.nearest(0.5, 0.0, 400.0, 200.0))
        assertNull(g.nearest(0.9, 0.0, 400.0, 200.0))
        assertTrue(PlotGeometry(spec("radar"), PlotView(hidden = setOf(0))).points.isEmpty())
    }

    @Test fun horizontalBarsUseNumericXAndCategoryYAndAreaIncludesZero() {
        val g = PlotGeometry(spec("horizontal-bar", listOf(-2.0, 0.0, 2.0)), PlotView())
        assertEquals(0.0, g.points[0].x, 0.000001)
        assertEquals(1.0 / 6, g.points[0].y, 0.000001)
        assertEquals(2, g.nearest(0.1, 0.9, 400.0, 200.0))
        val area = PlotGeometry(spec("area", listOf(2.0, 3.0, 4.0)), PlotView())
        assertEquals(0.0, area.yAxis.min, 0.0)
        assertEquals(1.0, area.zero, 0.0)
    }
}
