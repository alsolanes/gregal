package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

/** Un sol format de gràfic: el ```chart dels vents i el ```gregal-plot de l'escriptori. */
class ChartFormatsTest {
    @Test fun `gregal-plot de l'escriptori es dibuixa`() {
        val src = "Vendes:\n```gregal-plot\n{\"title\":\"Vendes\",\"type\":\"area\",\"labels\":[\"G\",\"F\"],\"series\":[{\"label\":\"2026\",\"values\":[3,5]}]}\n```"
        val chart = splitMd(src).filterIsInstance<Seg.Chart>().single().spec
        assertEquals("line", chart.type)
        assertEquals("2026", chart.series.single().name)
    }
    @Test fun `doughnut i barra horitzontal tenen equivalent`() {
        assertEquals("pie", parseChartSpec("{\"type\":\"doughnut\",\"labels\":[\"A\",\"B\"],\"series\":[{\"name\":\"S\",\"values\":[1,3]}]}")?.type)
        assertEquals("bar", parseChartSpec("{\"type\":\"horizontal-bar\",\"labels\":[\"A\"],\"series\":[{\"name\":\"S\",\"values\":[1]}]}")?.type)
        assertNull(parseChartSpec("{\"type\":\"radar\",\"labels\":[\"A\"],\"series\":[{\"name\":\"S\",\"values\":[1]}]}"))
    }
    @Test fun `etiquetes que caben o porten punts suspensius`() {
        assertEquals("FC Barcelona", fitChartLabel("FC Barcelona", 300f))
        assertEquals("Reial Ma…", fitChartLabel("Reial Madrid", 126f))
        assertEquals("2.5", fmtNum(2.5f))
    }
}
