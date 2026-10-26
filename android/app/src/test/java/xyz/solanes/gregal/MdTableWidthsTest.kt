package xyz.solanes.gregal

import org.junit.Assert.assertTrue
import org.junit.Test

class MdTableWidthsTest {
    @Test fun `cap paraula no es parteix i el text llarg baixa de linia`() {
        val w = tableColumnWidths(
            listOf("Model", "Preu", "El bo"),
            listOf(listOf("[ASUS Zenbook 14 OLED UX3405](https://x)", "898,90 €", "OLED 14\" 400 cd/m², 75 Wh, HDMI 2.1 + Thunderbolt 4, molt bona bateria")),
        )
        assertTrue(w.all { it in 84..210 })
        assertTrue("la paraula més llarga hi cap", w[2] >= "Thunderbolt".length * 8)
        assertTrue("el preu no ocupa com un paràgraf", w[1] < w[2])
    }
}
