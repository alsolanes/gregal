package xyz.solanes.gregal

import android.webkit.JavascriptInterface
import kotlinx.serialization.Serializable
import kotlinx.serialization.Transient
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.contentOrNull
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
    val slide: JsonObject? = null,
)

/** Els layouts d'una diapositiva, en l'ordre de la llista de «Diapositiva nova» (deck-view.js). */
val slideLayouts = listOf("title", "section", "content", "two_columns", "comparison", "big_number", "image", "title_only", "blank", "closing")

/** Els camps del formulari d'un layout, com els fa servir ooxml.buildSlide; les notes hi són sempre. */
fun slideFields(layout: String): List<String> = when (layout) {
    "title", "section", "closing" -> listOf("title", "subtitle")
    "content" -> listOf("title", "body", "bullets")
    "two_columns", "comparison" -> listOf("title", "left_title", "left", "right_title", "right")
    "big_number" -> listOf("title", "number", "caption")
    "image" -> listOf("title", "image", "body")
    "title_only" -> listOf("title")
    else -> emptyList()
} + "notes"

fun slideText(s: JsonObject?, key: String): String = (s?.get(key) as? JsonPrimitive)?.contentOrNull.orEmpty()

fun slideBullets(s: JsonObject?): List<String> = (s?.get("bullets") as? JsonArray)?.mapNotNull { (it as? JsonPrimitive)?.contentOrNull }.orEmpty()

private fun slideForm(s: JsonObject): Map<String, String> = slideFields(slideText(s, "layout")).associateWith {
    if (it == "bullets") slideBullets(s).joinToString("\n") else slideText(s, it)
}

/** La diapositiva [s] amb els camps del formulari a sobre: els que no s'hi veuen (un gràfic, una taula) es queden.
 *  Els espais del principi d'un punt es queden: el constructor els llegeix com a nivell. */
private fun slideWith(s: JsonObject, form: Map<String, String>): JsonObject = JsonObject(s.toMutableMap().apply {
    for ((k, v) in form) {
        val lines = v.split("\n").map { it.trimEnd() }.filter { it.isNotBlank() }
        when {
            k == "bullets" && lines.isNotEmpty() -> put(k, JsonArray(lines.map { JsonPrimitive(it) }))
            k != "bullets" && v.isNotBlank() -> put(k, JsonPrimitive(v.trim()))
            else -> remove(k)
        }
    }
})

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
 * En una presentació, `slide` és la diapositiva tal com s'ha obert i `fields` el formulari del seu layout.
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
    val slide: JsonObject? = null,
    val fields: Map<String, String> = emptyMap(),
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
    /** En una presentació, l'última diapositiva oberta: la nova va després. */
    @Transient val current: String = "",
) {
    /** Sense blocs ni editor: es veu un bloc buit on escriure, com a la web (no en una presentació). */
    val blank: Boolean get() = kind == "doc" && blocks.isEmpty() && edit == null

    /** Un document de text s'edita amb l'editor de la web; les presentacions i els fulls, amb les seves pantalles. */
    val inWebEditor: Boolean get() = kind == "doc"

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
        if (kind == "deck") {
            val s = b.slide ?: JsonObject(emptyMap())
            return copy(current = id, edit = DocEdit(id, base = b.version, slide = s, fields = slideForm(s)))
        }
        return copy(edit = DocEdit(id, opened = b.md.trim(), text = b.md, base = b.version))
    }

    fun typedField(key: String, value: String): LiveDoc = copy(edit = edit?.copy(fields = edit.fields + (key to value)))

    /** La diapositiva oberta per últim, si encara hi és; si no, l'última (o el principi d'una presentació buida). */
    val slideAfter: String get() = blocks.firstOrNull { it.id == current }?.id ?: blocks.lastOrNull()?.id.orEmpty()

    /** «Diapositiva nova»: una de buida del [layout] després d'[after]. */
    fun addSlideRequest(after: String, layout: String): DocRequest =
        DocRequest("POST", scoped(blocksPath(), scope), buildJsonObject {
            put("after", after); put("slide", buildJsonObject { put("layout", layout) })
        }.toString())

    /** La resposta de «Diapositiva nova»: s'insereix després d'[after] i se n'obre el formulari. */
    fun slideAdded(after: String, code: Int, body: String): DocSaved {
        val a = runCatching { GregalJson.decodeFromString(BlockAnswer.serializer(), body) }.getOrNull() ?: BlockAnswer()
        if (code != 200) return DocSaved(this, error = a.error.ifBlank { "HTTP $code" })
        var d = this
        var at = after
        for (n in a.blocks) {
            d = d.apply(DocChange("insert", n, at, a.version))
            at = n.id
        }
        return DocSaved(d.open(at), at)
    }

    /** «Esborra la diapositiva»: amb la versió que la persona té, com en desar. */
    fun deleteRequest(): DocRequest? {
        val e = edit ?: return null
        return DocRequest("DELETE", scoped(blocksPath() + "/" + URLEncoder.encode(e.id, "UTF-8") + "?base_version=" + e.base, scope))
    }

    private fun blocksPath() = "/api/docs/" + URLEncoder.encode(id, "UTF-8") + "/blocks"

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
        val path = blocksPath()
        if (e.slide != null) {
            // Després d'un 409, els camps de la persona van sobre la diapositiva d'ara.
            val s = slideWith(e.conflict?.slide ?: e.slide, e.fields)
            if (e.conflict == null && s == slideWith(e.slide, slideForm(e.slide))) return null
            return DocRequest("PUT", scoped(path + "/" + URLEncoder.encode(e.id, "UTF-8"), scope),
                buildJsonObject { put("slide", s); put("base_version", e.base) }.toString())
        }
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
            // Un DELETE respon sense bloc.
            e.slide != null && b == null -> DocSaved(closedWithout(e.id))
            e.slide == null && e.cell.isEmpty() && e.text.isBlank() -> DocSaved(closedWithout(e.id))
            else -> DocSaved(closed.copy(blocks = b?.let { n -> blocks.map { if (it.id == n.id) n else it } } ?: blocks), e.id)
        }
    }
}

/** L'enllaç «Descarrega en Word» (en Excel un full de càlcul, en PowerPoint una presentació): es baixa com
 *  els altres documents de Gregal (isServerDocLink), amb el títol com a nom del fitxer. */
fun docExportPath(d: DocRef): String = when (d.kind) { "sheet" -> "xlsx"; "deck" -> "pptx"; else -> "docx" }.let { f ->
    docPath(d, "/export?format=$f&name=" + URLEncoder.encode(d.title + "." + f, "UTF-8"))
}

/** La pàgina del document de la web (/app/doc/{id}, doc-page.js), amb la sessió de l'app i el scope: l'editor, la barra i la pàgina de la web. */
fun docPageUrl(base: String, d: DocRef, session: String): String =
    base + scoped("/app/doc/" + URLEncoder.encode(d.id, "UTF-8").replace("+", "%20") + "?session=" + URLEncoder.encode(session, "UTF-8"), d.scope)

/** La clau per carregar la pàgina: en una capçalera, mai a la URL. */
fun docPageHeaders(token: String): Map<String, String> = if (token.isBlank()) emptyMap() else mapOf("Authorization" to "Bearer $token")

/** Les crides de la pàgina: doc-page.js llegeix la clau d'aquest pont (window.GregalAndroid), com a l'escriptori de window.gregalDesktop. */
class DocPageBridge(private val token: String) {
    @JavascriptInterface
    fun token(): String = token
}

/** Només la pàgina mateixa es carrega al WebView (la pàgina, amb el pont i la clau, no navega enlloc més). */
fun docPageKeeps(url: String, page: String): Boolean = url == page

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
