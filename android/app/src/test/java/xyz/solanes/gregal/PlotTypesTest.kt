package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

class PlotTypesTest {
    private fun parse(type: String, values: String = "[1,2,3]", labels: String = "[\"A\",\"B\",\"C\"]", extra: String = "") =
        parsePlot("gregal-plot", """{"title":"Chart","type":"$type","labels":$labels,"series":[{"label":"S","values":$values}$extra]}""")

    @Test fun acceptsAllEightCanonicalTypes() {
        listOf("line", "bar", "scatter", "area", "horizontal-bar", "pie", "doughnut", "radar").forEach {
            assertNotNull(it, parse(it, labels = "[\"1\",\"2\",\"3\"]"))
        }
        assertNotNull(parse("area", "[-1,0,2]"))
        assertNotNull(parse("horizontal-bar", "[-1,0,2]"))
    }

    @Test fun circularChartsRequireOneNonnegativeNonzeroSeries() {
        listOf("pie", "doughnut").forEach {
            assertNull(parse(it, "[-1,0,2]"))
            assertNull(parse(it, "[0,0,0]"))
            assertNull(parse(it, extra = ",{\"label\":\"T\",\"values\":[1,2,3]}"))
            assertNotNull(parse(it, "[1.7976931348623157e308,1.7976931348623157e308,0]"))
        }
    }

    @Test fun radarBoundsCategoriesAndAllowsZeroButNotNegativeValues() {
        assertNotNull(parse("radar", "[0,0,0]"))
        assertNull(parse("radar", "[-1,0,2]"))
        assertNull(parse("radar", "[1,2]", "[\"A\",\"B\"]"))
        for (n in listOf(32, 33)) {
            val p = parse("radar", List(n) { "0" }.joinToString(",", "[", "]"), List(n) { "\"A\"" }.joinToString(",", "[", "]"))
            if (n == 32) assertNotNull(p) else assertNull(p)
        }
    }
}
