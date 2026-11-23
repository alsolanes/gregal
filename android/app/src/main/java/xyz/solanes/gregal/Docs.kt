package xyz.solanes.gregal

import kotlinx.serialization.Serializable
import java.net.URLEncoder

// Documents vius del projecte (/api/docs, internal/gdocs al servidor): la
// llista, un document i els events `block` del seu stream. Com doc-view.js
// a la web; la pantalla és DocScreen.kt. Pur, perquè l'editor (063) hi pugui
// afegir canvis sense tocar la pantalla.

@Serializable
data class DocRef(val id: String = "", val title: String = "")

@Serializable
data class DocBlock(val id: String = "", val md: String = "")

/** Un event `block`: `insert` posa el bloc després d'`after` (buit = al principi). */
@Serializable
data class DocChange(val op: String = "", val block: DocBlock = DocBlock(), val after: String = "", val version: Int = 0)

/** El document llegit, amb els events que han arribat després aplicats. */
@Serializable
data class LiveDoc(
    val id: String = "",
    val title: String = "",
    val version: Int = 0,
    val blocks: List<DocBlock> = emptyList(),
    /** La versió del document llegit: els events fins aquí ja hi són. */
    val base: Int = version,
) {
    fun apply(c: DocChange): LiveDoc {
        // Un insert de diversos blocs envia un event per bloc amb la mateixa
        // versió: per això es compara amb la base i no amb l'última versió.
        if (c.version <= base) return this
        val i = blocks.indexOfFirst { it.id == c.block.id }
        val next = when (c.op) {
            // Després d'un bloc que no hi és: al final, com a la web.
            "insert" -> blocks.toMutableList().apply {
                add(if (c.after.isEmpty()) 0 else blocks.indexOfFirst { it.id == c.after }.let { if (it < 0) size else it + 1 }, c.block)
            }
            "replace" -> if (i < 0) blocks else blocks.toMutableList().apply { set(i, c.block) }
            "delete" -> if (i < 0) blocks else blocks.filterIndexed { k, _ -> k != i }
            else -> blocks
        }
        return copy(blocks = next, version = c.version)
    }
}

/** L'enllaç «Descarrega en Word»: es baixa com els altres documents de
 *  Gregal (isServerDocLink), amb el títol com a nom del fitxer. */
fun docExportPath(d: DocRef): String =
    "/api/docs/" + URLEncoder.encode(d.id, "UTF-8") + "/export?format=docx&name=" + URLEncoder.encode(d.title + ".docx", "UTF-8")

@Serializable
private class DocListDto(val items: List<DocRef> = emptyList())

object DocsJson {
    /** El payload de `doc_changed`; null si no du id. */
    fun changed(raw: String): DocRef? =
        runCatching { GregalJson.decodeFromString(DocRef.serializer(), raw) }.getOrNull()?.takeIf { it.id.isNotBlank() }

    fun list(raw: String): List<DocRef> = GregalJson.decodeFromString(DocListDto.serializer(), raw).items

    fun doc(raw: String): LiveDoc = GregalJson.decodeFromString(LiveDoc.serializer(), raw)

    fun change(raw: String): DocChange? =
        runCatching { GregalJson.decodeFromString(DocChange.serializer(), raw) }.getOrNull()
}
