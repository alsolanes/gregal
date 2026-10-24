package xyz.solanes.gregal

import androidx.compose.ui.graphics.Color
import org.junit.Assert.*
import org.junit.Test

/** Pure math checks for eye-ink contrast decisions on the new solid palette. */
class EyeInkContrastTest {
    private fun bestInk(accent: Color): Color =
        if (contrastWith(accent, Color(0xFF202127)) >= contrastWith(accent, Color.White)) Color(0xFF202127) else Color.White

    @Test fun picksTheHigherContrastInkForEachSolidPaletteColor() {
        val solid = listOf(
            "#20BE92" to Color(0xFF20BE92), "#1A9BEE" to Color(0xFF1A9BEE),
            "#27292F" to Color(0xFF27292F), "#FF9F24" to Color(0xFFFF9F24),
            "#EB4EB4" to Color(0xFFEB4EB4), "#EF5965" to Color(0xFFEF5965),
            "#8C6CE8" to Color(0xFF8C6CE8), "#EACF87" to Color(0xFFEACF87),
        )
        solid.forEach { (hex, accent) ->
            val ink = bestInk(accent)
            val ratio = contrastWith(accent, ink)
            assertTrue("$hex needs readable eyes, got $ratio", ratio >= 3.0)
            assertEquals("$hex picks the darker-contrast winner",
                contrastWith(accent, Color(0xFF202127)) >= contrastWith(accent, Color.White),
                ink == Color(0xFF202127))
        }
    }

    @Test fun paleSandsChooseDarkInkAndDeepInksChooseWhite() {
        assertEquals(Color(0xFF202127), bestInk(Color(0xFFEACF87)))
        assertEquals(Color(0xFF202127), bestInk(Color(0xFFFF9F24)))
        assertEquals(Color.White, bestInk(Color(0xFF27292F)))
        assertEquals(Color(0xFF202127), bestInk(Color(0xFF1A9BEE)))
    }
}
