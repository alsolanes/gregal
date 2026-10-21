package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

/** La cara i el nom només al primer missatge de cada ratxa del mateix
 *  parlant: si no, cada bombolla repetia cara + insígnia. */
class RunKeyTest {
    private val xaloc = Wind("x", "Xaloc", "#EFA68F", "música")
    private val marinada = Wind("m", "Marinada", "#EFC69A", "calma")
    private val members = listOf(xaloc, marinada)

    private fun pet(text: String, speakerId: String = "") =
        resolveWindMessage(GregalViewModel.PetMsg("pet", text, speakerId), members)
    private fun user(text: String) =
        resolveWindMessage(GregalViewModel.PetMsg("user", text), members)

    @Test fun `mateix vent amb speakerId agrupa`() {
        assertEquals(runKeyOf(pet("hola", "x")), runKeyOf(pet("adeu", "x")))
    }
    @Test fun `vents diferents separen`() {
        assertNotEquals(runKeyOf(pet("hola", "x")), runKeyOf(pet("hola", "m")))
    }
    @Test fun `legacy pel prefix del nom agrupa`() {
        assertEquals(runKeyOf(pet("Xaloc: hola")), runKeyOf(pet("Xaloc: adeu")))
    }
    @Test fun `legacy diferent separa`() {
        assertNotEquals(runKeyOf(pet("Xaloc: hola")), runKeyOf(pet("Marinada: hola")))
    }
    @Test fun `usuari trenca la ratxa del vent`() {
        assertNotEquals(runKeyOf(user("ei")), runKeyOf(pet("hola", "x")))
    }
    @Test fun `dos usuaris seguits agrupen`() {
        assertEquals(runKeyOf(user("a")), runKeyOf(user("b")))
    }
}
