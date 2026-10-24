package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

/** L'objecte d'ofici del vent propi, endevinat del rol (paritat amb winds_kit.go). */
class WindKitTest {
    @Test fun infereixPerOfici() {
        assertEquals("music", inferKit("Música i descobertes"))
        assertEquals("sports", inferKit("Futbol i Barça"))
        assertEquals("tech", inferKit("Tecnologia i novetats digitals"))
        assertEquals("research", inferKit("Recerca i experiments"))
        assertEquals("books", inferKit("Llibres i tardes"))
        assertEquals("ml", inferKit("Machine learning"))
        assertEquals("security", inferKit("Seguretat informàtica"))
        assertEquals("music", inferKit("Music and songs"))
    }

    @Test fun conservadorSiNoEncaixa() {
        assertEquals("", inferKit("calma i benestar"))
        assertEquals("", inferKit(""))
    }

    @Test fun faceStylePrioritzaKitDelServidor() {
        // Kit explícit del servidor mana; després specialty; al final la inferència del rol.
        val delServidor = Wind("a", "X", "#111111", "música", kit = "tech")
        assertEquals("tech", delServidor.faceStyle.kit)
        val deColla = Wind("b", "Y", "#222222", "qualsevol", specialty = "review")
        assertEquals("review", deColla.faceStyle.kit)
        val propi = Wind("c", "Z", "#333333", "Futbol i Barça")
        assertEquals("sports", propi.faceStyle.kit)
    }
}
