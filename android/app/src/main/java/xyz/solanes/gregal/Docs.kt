package xyz.solanes.gregal

import kotlinx.serialization.Serializable
import kotlinx.serialization.Transient
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import java.net.URLEncoder

// Documents vius del projecte (/api/docs, internal/gdocs al servidor): la
// llista, un document, els events `block` del seu stream i l'edició d'un
// bloc. Com doc-view.js a la web; la pantalla és DocScreen.kt.

@Serializable
data class DocRef(val id: String = "", val title: String = "")

/** `version` és la del document quan el bloc va canviar per última vegada. */
@Serializable
data class DocBlock(val id: String = "", val md: String = "", val version: Int = 0)

/**
 * L'editor obert: el bloc `id`, o un de nou després d'`after` si `id` és buit.
 * `opened` és el text amb què s'ha obert (sense tocar no es desa) i `base`
 * la versió del bloc que la persona té; `conflict` és el bloc d'ara després
 * d'un 409.
 */
data class DocEdit(
    val id: String,
    val after: String = "",
    val opened: String = "",
    val text: String = opened,
    val base: Int = 0,
    val conflict: DocBlock? = null,
    val saving: Boolean = false,
)

data class DocRequest(val method: String, val path: String, val body: String? = null)

/** Què ha quedat després de desar: `at` és el bloc sota el qual aniria un de nou
 *  (null si no n'hi ha: esborrat, buit, conflicte o error) i `error` el que cal dir. */
data class DocSaved(val doc: LiveDoc, val at: String? = null, val error: String? = null)

@Serializable
private class BlockAnswer(val version: Int = 0, val block: DocBlock? = null, val blocks: List<DocBlock> = emptyList(), val error: String = "")

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
    @Transient val edit: DocEdit? = null,
) {
    fun apply(c: DocChange): LiveDoc {
        // Un insert de diversos blocs envia un event per bloc amb la mateixa
        // versió: per això es compara amb la base i no amb l'última versió.
        if (c.version <= base) return this
        val i = blocks.indexOfFirst { it.id == c.block.id }
        val next = when (c.op) {
            // Ja hi és: la resposta del POST l'ha posat abans que arribés pel stream.
            "insert" -> if (i >= 0) blocks else blocks.toMutableList().apply {
                // Després d'un bloc que no hi és: al final, com a la web.
                add(if (c.after.isEmpty()) 0 else blocks.indexOfFirst { it.id == c.after }.let { if (it < 0) size else it + 1 }, c.block)
            }
            // El text de la persona és a l'editor: un replace només canvia el que se'n sap.
            "replace" -> if (i < 0) blocks else blocks.toMutableList().apply { set(i, c.block) }
            // El bloc que s'edita es queda, perquè l'editor no desaparegui amb el text.
            "delete" -> if (i < 0 || edit?.id == c.block.id) blocks else blocks.filterIndexed { k, _ -> k != i }
            else -> blocks
        }
        return copy(blocks = next, version = c.version)
    }

    fun open(id: String): LiveDoc {
        val b = blocks.firstOrNull { it.id == id } ?: return this
        return copy(edit = DocEdit(id, opened = b.md.trim(), text = b.md, base = b.version))
    }

    fun openNew(after: String): LiveDoc = copy(edit = DocEdit("", after = after))

    fun typed(text: String): LiveDoc = copy(edit = edit?.copy(text = text))

    /** El que cal enviar per desar l'editor; null si no cal enviar res. */
    fun saveRequest(): DocRequest? {
        val e = edit ?: return null
        val md = e.text.trim()
        val path = "/api/docs/" + URLEncoder.encode(id, "UTF-8") + "/blocks"
        if (e.id.isEmpty()) return if (md.isEmpty()) null else DocRequest("POST", path, buildJsonObject { put("after", e.after); put("md", md) }.toString())
        // Sense tocar no es desa: el text obert pot ser més vell que el de l'agent.
        if (e.conflict == null && md == e.opened) return null
        val block = path + "/" + URLEncoder.encode(e.id, "UTF-8")
        if (md.isEmpty()) return DocRequest("DELETE", block + "?base_version=" + e.base)
        return DocRequest("PUT", block, buildJsonObject { put("md", md); put("base_version", e.base) }.toString())
    }

    /** Tanca l'editor quan saveRequest no té res per enviar. */
    fun skipped(): DocSaved = DocSaved(copy(edit = null), edit?.id?.ifEmpty { null })

    /** Aplica la resposta d'una ruta de blocs a l'editor obert. */
    fun saved(code: Int, body: String): DocSaved {
        val e = edit ?: return DocSaved(this)
        val a = runCatching { GregalJson.decodeFromString(BlockAnswer.serializer(), body) }.getOrNull() ?: BlockAnswer()
        val b = a.block
        val closed = copy(edit = null)
        return when {
            // Es veu el text d'ara i es queda el de la persona; tornar a desar trepitja aquesta versió.
            code == 409 && b != null ->
                DocSaved(copy(blocks = blocks.map { if (it.id == b.id) b else it }, edit = e.copy(base = b.version, conflict = b, saving = false)))
            code != 200 -> DocSaved(copy(edit = e.copy(saving = false)), error = a.error.ifBlank { "HTTP $code" })
            e.id.isEmpty() -> {
                var d = closed
                var at = e.after
                for (n in a.blocks) {
                    d = d.apply(DocChange("insert", n, at, a.version))
                    at = n.id
                }
                DocSaved(d, at)
            }
            b != null -> DocSaved(closed.copy(blocks = blocks.map { if (it.id == b.id) b else it }), e.id)
            else -> DocSaved(closed.copy(blocks = blocks.filter { it.id != e.id }))
        }
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
