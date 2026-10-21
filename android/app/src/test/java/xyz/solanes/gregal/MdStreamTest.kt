package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

/** Mentre el vent escriu (streaming) el text arriba amb marques crues
 *  (**negreta**, [etiqueta](url), `codi`, > cites, - vinyetes): stripMdInline
 *  les neteja perquè el bombolla no mostri sintaxi a mitges. */
class MdStreamTest {
    @Test fun `treu negretes i cursives`() {
        assertEquals("hola món", stripMdInline("hola **món**"))
    }
    @Test fun `enllaç mostra només l'etiqueta`() {
        assertEquals("mira calendario", stripMdInline("mira [calendario](https://x.cat/c)"))
    }
    @Test fun `treu codi inline i blocs`() {
        assertEquals("fes servir pip", stripMdInline("fes servir `pip`"))
        assertEquals("bloc", stripMdInline("```py\nbloc\n```"))
    }
    @Test fun `treu marques de cita i vinyeta`() {
        assertEquals("cita", stripMdInline("> cita"))
        assertEquals("punt", stripMdInline("- punt"))
        assertEquals("segon", stripMdInline("2. segon"))
    }
    @Test fun `text pla intacte i buit buit`() {
        assertEquals("Hola, com va?", stripMdInline("Hola, com va?"))
        assertEquals("", stripMdInline(""))
    }
}
