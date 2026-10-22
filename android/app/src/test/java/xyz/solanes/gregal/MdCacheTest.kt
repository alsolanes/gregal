package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

/** La cau de blocs: el mateix text no es parseja dues vegades. */
class MdCacheTest {
    @Test fun `mateix text mateixa instancia`() {
        val a = richBlocks("Hola **mon** `codi`")
        val b = richBlocks("Hola **mon** `codi`")
        assertTrue(a === b)
    }
    @Test fun `textos diferents no xoquen`() {
        val a = richBlocks("primer text únic 1")
        val b = richBlocks("segon text únic 2")
        assertFalse(a === b)
    }
    @Test fun `la cau no creix sense fre`() {
        repeat(300) { richBlocks("text de farciment per a la cau $it") }
        assertTrue(richBlocksCached() <= 256)
    }
}
