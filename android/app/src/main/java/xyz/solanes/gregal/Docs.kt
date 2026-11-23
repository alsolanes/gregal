package xyz.solanes.gregal

import kotlinx.serialization.Serializable
import kotlinx.serialization.Transient
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import java.net.URLEncoder

// Documents vius del projecte (/api/docs, internal/gdocs al servidor): la
// llista, un document, els events `block` del seu stream i l'edició d'un
// bloc. Com doc-view.js a la web; la pantalla és DocScreen.kt.
// scope "mine": un document de «Els meus documents», que totes les rutes
// han de demanar amb ?scope=mine; buit, un del projecte de la sessió.

@Serializable
data class DocRef(val id: String = "", val title: String = "", val kind: String = "doc", val scope: String = "")

private fun scoped(path: String, scope: String): String =
    if (scope.isEmpty()) path else path + (if ('?' in path) "&" else "?") + "scope=" + scope

/** La ruta del document (sub: "/stream", ...), amb el seu scope. */
fun docPath(d: DocRef, sub: String = ""): String = scoped("/api/docs/" + URLEncoder.encode(d.id, "UTF-8") + sub, d.scope)

/** «Document nou» o «Full de càlcul nou» ([kind] "sheet"), com doc-home.js: sempre a «Els meus documents». */
fun docCreateRequest(title: String, kind: String = "doc"): DocRequest =
    DocRequest("POST", "/api/docs?scope=mine", buildJsonObject { put("title", title); put("kind", kind) }.toString())

/** El nom de la secció «Aquest projecte»; null si no s'ha de mostrar (com a la web, només a Codi i amb projecte). */
fun docsProjectName(cwd: String, project: String, vents: Boolean): String? =
    if (vents || cwd.isBlank()) null else project.ifBlank { cwd }

/** `version` és la del document quan el bloc va canviar per última vegada.
 *  En un full de càlcul, el bloc és un full: `name`, l'entrada de cada
 *  cel·la (`cells`, A1 → número, text o fórmula) i el valor calculat pel servidor (`values`). */
@Serializable
data class DocBlock(
    val id: String = "",
    val md: String = "",
    val version: Int = 0,
    val name: String = "",
    val cells: Map<String, String> = emptyMap(),
    val values: Map<String, String> = emptyMap(),
)

/** La lletra d'una columna: 0 → A, 26 → AA. */
fun sheetColumn(i: Int): String = (if (i >= 26) sheetColumn(i / 26 - 1) else "") + ('A' + i % 26)

private val cellName = Regex("([A-Z]+)([0-9]+)")

/** Columnes i files de la graella: fins a l'última cel·la amb dades i unes quantes de buides per escriure-hi. */
fun sheetSize(b: DocBlock): Pair<Int, Int> {
    var cols = 0
    var rows = 0
    for (c in b.cells.keys) {
        val m = cellName.matchEntire(c) ?: continue
        cols = maxOf(cols, m.groupValues[1].fold(0) { n, ch -> n * 26 + (ch - 'A' + 1) })
        rows = maxOf(rows, m.groupValues[2].toInt())
    }
    return maxOf(cols + 1, 6) to maxOf(rows + 5, 20)
}

/**
 * L'editor obert: el bloc `id`, o un de nou després d'`after` si `id` és buit.
 * En un full, `id` és el full i `cell` la cel·la que s'edita.
 * `opened` és el text amb què s'ha obert (sense tocar no es desa) i `base`
 * la versió del bloc que la persona té; `conflict` és el bloc d'ara després
 * d'un 409; `gone`, que el stream l'ha esborrat mentre s'editava.
 */
data class DocEdit(
    val id: String,
    val after: String = "",
    val opened: String = "",
    val text: String = opened,
    val base: Int = 0,
    val conflict: DocBlock? = null,
    val saving: Boolean = false,
    val gone: Boolean = false,
    val cell: String = "",
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
    val kind: String = "doc",
    val version: Int = 0,
    val blocks: List<DocBlock> = emptyList(),
    /** La versió del document llegit: els events fins aquí ja hi són. */
    val base: Int = version,
    @Transient val edit: DocEdit? = null,
    /** El de la llista d'on s'ha obert: el servidor no el torna. */
    @Transient val scope: String = "",
    /** En un full de càlcul, la pestanya que es mira (buit = la primera). */
    @Transient val sheet: String = "",
) {
    /** Sense blocs ni editor: es veu un bloc buit on escriure, com a la web (no en una presentació). */
    val blank: Boolean get() = kind == "doc" && blocks.isEmpty() && edit == null

    fun apply(c: DocChange): LiveDoc {
        // Un insert de diversos blocs envia un event per bloc amb la mateixa
        // versió: per això es compara amb la base i no amb l'última versió.
        if (c.version <= base) return this
        val i = blocks.indexOfFirst { it.id == c.block.id }
        var e = edit
        val next = when (c.op) {
            // Ja hi és: la resposta del POST l'ha posat abans que arribés pel stream.
            "insert" -> if (i >= 0) blocks else blocks.toMutableList().apply {
                // Després d'un bloc que no hi és: al final, com a la web.
                add(if (c.after.isEmpty()) 0 else blocks.indexOfFirst { it.id == c.after }.let { if (it < 0) size else it + 1 }, c.block)
            }
            // El text de la persona és a l'editor: un replace només canvia el que se'n sap.
            "replace" -> if (i < 0) blocks else blocks.toMutableList().apply { set(i, c.block) }
            "delete" -> when {
                i < 0 -> blocks
                // El bloc que s'edita es queda fins que l'editor es tanca, perquè no desaparegui amb el text.
                edit?.id == c.block.id -> { e = edit.copy(gone = true); blocks }
                else -> {
                    // Un bloc nou penjava d'aquest: passa al de sobre (o al principi).
                    if (edit != null && edit.id.isEmpty() && edit.after == c.block.id) e = edit.copy(after = blocks.getOrNull(i - 1)?.id.orEmpty())
                    blocks.filterIndexed { k, _ -> k != i }
                }
            }
            else -> blocks
        }
        return copy(blocks = next, version = maxOf(version, c.version), edit = e)
    }

    fun open(id: String): LiveDoc {
        val b = blocks.firstOrNull { it.id == id } ?: return this
        return copy(edit = DocEdit(id, opened = b.md.trim(), text = b.md, base = b.version))
    }

    fun openNew(after: String): LiveDoc = copy(edit = DocEdit("", after = after))

    /** El full de la pestanya triada; si l'han esborrat, el primer. */
    fun shownSheet(): DocBlock? = blocks.firstOrNull { it.id == sheet } ?: blocks.firstOrNull()

    fun showSheet(id: String): LiveDoc = copy(sheet = id)

    /** Edita la cel·la [cell] del full [sheet] amb la seva entrada (la fórmula, no el valor). */
    fun openCell(sheet: String, cell: String): LiveDoc {
        val b = blocks.firstOrNull { it.id == sheet } ?: return this
        val raw = b.cells[cell].orEmpty()
        return copy(edit = DocEdit(sheet, opened = raw.trim(), text = raw, base = b.version, cell = cell))
    }

    fun typed(text: String): LiveDoc = copy(edit = edit?.copy(text = text))

    /** El que cal enviar per desar l'editor; null si no cal enviar res. */
    fun saveRequest(): DocRequest? {
        val e = edit ?: return null
        val md = e.text.trim()
        val path = "/api/docs/" + URLEncoder.encode(id, "UTF-8") + "/blocks"
        if (e.id.isEmpty()) return if (md.isEmpty()) null else DocRequest("POST", scoped(path, scope), buildJsonObject { put("after", e.after); put("md", md) }.toString())
        // Sense tocar no es desa: el text obert pot ser més vell que el de l'agent.
        if (e.conflict == null && md == e.opened) return null
        val block = path + "/" + URLEncoder.encode(e.id, "UTF-8")
        // Una cel·la buida s'esborra amb una entrada buida; el full es queda.
        if (e.cell.isNotEmpty()) return DocRequest("PUT", scoped(block, scope),
            buildJsonObject { put("cells", buildJsonObject { put(e.cell, md) }); put("base_version", e.base) }.toString())
        if (md.isEmpty()) return DocRequest("DELETE", scoped(block + "?base_version=" + e.base, scope))
        return DocRequest("PUT", scoped(block, scope), buildJsonObject { put("md", md); put("base_version", e.base) }.toString())
    }

    /** Tanca l'editor quan saveRequest no té res per enviar. */
    fun skipped(): DocSaved = when {
        edit?.gone == true -> DocSaved(closedWithout(edit.id))
        else -> DocSaved(copy(edit = null), edit?.id?.ifEmpty { null })
    }

    private fun closedWithout(id: String) = copy(edit = null, blocks = blocks.filter { it.id != id })

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
            // L'agent l'ha esborrat: no hi ha res on desar el text.
            code == 404 && e.gone -> DocSaved(closedWithout(e.id), error = a.error.ifBlank { "HTTP $code" })
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
            e.cell.isEmpty() && e.text.isBlank() -> DocSaved(closedWithout(e.id))
            else -> DocSaved(closed.copy(blocks = b?.let { n -> blocks.map { if (it.id == n.id) n else it } } ?: blocks), e.id)
        }
    }
}

/** L'enllaç «Descarrega en Word» o, d'un full de càlcul, «Descarrega en Excel»: es baixa com
 *  els altres documents de Gregal (isServerDocLink), amb el títol com a nom del fitxer. */
fun docExportPath(d: DocRef): String = (if (d.kind == "sheet") "xlsx" else "docx").let { f ->
    docPath(d, "/export?format=$f&name=" + URLEncoder.encode(d.title + "." + f, "UTF-8"))
}

@Serializable
private class DocListDto(val items: List<DocRef> = emptyList())

object DocsJson {
    /** El payload de `doc_changed`; null si no du id. */
    fun changed(raw: String): DocRef? =
        runCatching { GregalJson.decodeFromString(DocRef.serializer(), raw) }.getOrNull()?.takeIf { it.id.isNotBlank() }

    /** Els documents d'una llista; cadascun s'obrirà amb el scope que s'ha demanat. */
    fun list(raw: String, scope: String = ""): List<DocRef> =
        GregalJson.decodeFromString(DocListDto.serializer(), raw).items.map { it.copy(scope = scope) }

    fun doc(raw: String): LiveDoc = GregalJson.decodeFromString(LiveDoc.serializer(), raw)

    fun change(raw: String): DocChange? =
        runCatching { GregalJson.decodeFromString(DocChange.serializer(), raw) }.getOrNull()
}
