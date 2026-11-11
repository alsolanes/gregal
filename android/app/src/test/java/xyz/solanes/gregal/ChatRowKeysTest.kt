package xyz.solanes.gregal

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Les files del xat es mouen de lloc quan el servidor torna a omplir el fil:
 * si la clau és la posició, la llista sencera es recompon i l'scroll salta.
 * Les claus han de ser úniques i dependre del missatge, no del seu lloc.
 */
class ChatRowKeysTest {

    private data class Row(val ts: Long, val who: String)

    private fun keysOf(vararg rows: Pair<Long, String>) =
        chatRowKeys(rows.map { (ts, who) -> Row(ts, who) }) { "s${it.ts}|${it.who}" }

    @Test
    fun `cada fila te la seva clau`() {
        val keys = keysOf(1L to "anna", 2L to "vent", 3L to "anna")
        assertEquals(listOf("s1|anna", "s2|vent", "s3|anna"), keys)
    }

    @Test
    fun `el mateix missatge conserva la clau quan n_arriben de nous`() {
        val abans = keysOf(1L to "anna", 2L to "vent")
        val despres = keysOf(1L to "anna", 2L to "vent", 3L to "vent")
        assertEquals(abans, despres.take(2))
    }

    @Test
    fun `dues files del mateix segon es desenmpalen`() {
        val keys = keysOf(5L to "anna", 5L to "anna", 5L to "anna")
        assertEquals(listOf("s5|anna", "s5|anna#2", "s5|anna#3"), keys)
        assertEquals(3, keys.toSet().size)
    }

    @Test
    fun `un identificador que ja porta sufix no xoca amb un de desenmpalat`() {
        // «bob» al segon 5 repetit dona «s5|bob#2», que és justament l'id de l'altra fila.
        val keys = keysOf(5L to "bob", 5L to "bob", 5L to "bob#2")
        assertEquals(listOf("s5|bob", "s5|bob#2", "s5|bob#2#2"), keys)
    }

    @Test
    fun `una llista buida no dona cap clau`() {
        assertEquals(emptyList<String>(), chatRowKeys(emptyList<Row>()) { "x" })
    }
}
