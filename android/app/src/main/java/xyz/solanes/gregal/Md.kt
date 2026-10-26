package xyz.solanes.gregal

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.sp

/** Segment de text ja classificat (pur, testeable). */
sealed interface Seg {
    data class Text(val t: String) : Seg
    data class Bold(val t: String) : Seg
    data class Italic(val t: String) : Seg
    data class Code(val t: String) : Seg
    data class Block(val lang: String, val t: String) : Seg
    data class Heading(val level: Int, val inner: List<Seg>) : Seg
    data class Link(val label: String, val url: String) : Seg
    data class Image(val alt: String, val url: String) : Seg
    data class Table(val headers: List<String>, val rows: List<List<String>>) : Seg
    data class Chart(val spec: ChartSpec) : Seg
}

/** Bloc ric per emetre composables (text corregut, codi, taula o imatge). */
sealed interface Rich {
    data class Inline(val segs: List<Seg>) : Rich
    data class Code(val lang: String, val code: String) : Rich
    data class Table(val headers: List<String>, val rows: List<List<String>>) : Rich
    data class Chart(val spec: ChartSpec) : Rich
    data class Image(val alt: String, val url: String) : Rich
}

/** Parteix markdown lleuger en segments (tots els detalls, testeable). */
fun splitMd(src: String): List<Seg> {
    val out = mutableListOf<Seg>()
    var rest = src
    while (true) {
        val b = rest.indexOf("```")
        if (b < 0) {
            out += splitBlockLines(rest)
            break
        }
        if (b > 0) out += splitBlockLines(rest.substring(0, b))
        val end = rest.indexOf("```", b + 3)
        if (end < 0) {
            out += listOf(Seg.Block("", rest.substring(b + 3).trim('\n')))
            break
        }
        val inner = rest.substring(b + 3, end).trim('\n')
        val nl = inner.indexOf('\n')
        if (nl > 0 && !inner.substring(0, nl).contains(' ') && inner.substring(0, nl).length <= 20) {
            val lang = inner.substring(0, nl)
            val body = inner.substring(nl + 1)
            // Bloc chart amb JSON vàlid: gràfic; si no, codi com sempre.
            // ```gregal-plot (el del xat principal i l'escriptori) és el mateix gràfic.
            if (lang == "chart" || lang == "gregal-plot") {
                val spec = parseChartSpec(body.trim())
                if (spec != null) out += listOf(Seg.Chart(spec))
                else out += listOf(Seg.Block(lang, body))
            } else {
                out += listOf(Seg.Block(lang, body))
            }
        } else {
            out += listOf(Seg.Block("", inner))
        }
        rest = rest.substring(end + 3)
    }
    return out
}

private val headingRe = Regex("^(#{1,6})\\s+(.*)$")
private val hrRe = Regex("^\\s*(---+|\\*\\*\\*+|___+)\\s*$")
private val quoteRe = Regex("^\\s*>\\s?(.*)$")
private val bulletRe = Regex("^\\s*(?:[-*•]|\\d+[.)])\\s+(.*)$")
private val taskRe = Regex("^\\s*(?:[-*•]|\\d+[.)])\\s+\\[([ xX])\\]\\s+(.*)$")
private val orderedRe = Regex("^\\s*(\\d+[.)])\\s+(.*)$")
private val tableLineRe = Regex("^\\s*\\|.*\\|\\s*$")
private val tableSepCellRe = Regex("^:?-{1,}:?$")

private fun parseTableRow(line: String): List<String> {
    val t = line.trim()
    return t.removePrefix("|").removeSuffix("|").split("|").map { it.trim() }
}

private fun splitBlockLines(s: String): List<Seg> {
    if (s.isEmpty()) return emptyList()
    val out = mutableListOf<Seg>()
    val lines = s.split('\n')
    var i = 0
    // Primer bloc de codi ja separat: aquí només queden línies normals.
    while (i < lines.size) {
        val line = lines[i]
        if (tableLineRe.matches(line)) {
            val rows = mutableListOf<List<String>>()
            while (i < lines.size && tableLineRe.matches(lines[i])) {
                rows += parseTableRow(lines[i])
                i++
            }
            val data = rows.filterNot { r -> r.isNotEmpty() && r.all { tableSepCellRe.matches(it) } }
            if (data.size >= 2) {
                if (out.isNotEmpty()) out += Seg.Text("\n")
                out += Seg.Table(data.first(), data.drop(1))
                out += Seg.Text("\n")
            } else {
                data.forEach { r -> out += parseInline(r.joinToString(" | ")) }
                out += Seg.Text("\n")
            }
            continue
        }
        if (i > 0) out += listOf(Seg.Text("\n"))
        val h = headingRe.find(line)
        if (h != null) {
            val level = h.groups[1]!!.value.length
            val body = h.groups[2]!!.value.ifEmpty { " " }
            out += listOf(Seg.Heading(level, parseInline(body)))
            i++
            continue
        }
        if (hrRe.matches(line)) {
            out += listOf(Seg.Text("────────"))
            i++
            continue
        }
        val q = quoteRe.find(line)
        if (q != null) {
            out += listOf(Seg.Text("│ "))
            out += parseInline(q.groups[1]!!.value)
            i++
            continue
        }
        val task = taskRe.find(line)
        if (task != null) {
            val done = task.groups[1]!!.value.trim().lowercase() == "x"
            out += listOf(Seg.Text(if (done) "☑ " else "☐ "))
            out += parseInline(task.groups[2]!!.value)
            i++
            continue
        }
        val b = bulletRe.find(line)
        if (b != null) {
            val ord = orderedRe.find(line)
            out += listOf(Seg.Text(if (ord != null) ord.groups[1]!!.value + " " else "• "))
            out += parseInline(b.groups[1]!!.value)
            i++
            continue
        }
        out += parseInline(line)
        i++
    }
    return out
}

/** Inline públic per reutilitzar (cel·les de taula). */
fun parseInline(s: String): List<Seg> {
    if (s.isEmpty()) return emptyList()
    val out = mutableListOf<Seg>()
    val buf = StringBuilder()
    var i = 0
    val n = s.length
    fun flush() {
        if (buf.isNotEmpty()) {
            out += Seg.Text(buf.toString())
            buf.clear()
        }
    }
    while (i < n) {
        if (s.startsWith("**", i)) {
            val end = s.indexOf("**", i + 2)
            if (end > i + 2) {
                flush()
                val inner = s.substring(i + 2, end)
                // Els models escriuen sovint **`nom()`**: és codi, no negreta
                // amb accents greus literals.
                out += if (inner.length > 2 && inner.startsWith('`') && inner.endsWith('`') && inner.count { it == '`' } == 2)
                    Seg.Code(inner.substring(1, inner.length - 1))
                else Seg.Bold(inner.replace("`", ""))
                i = end + 2
                continue
            }
        }
        if (s[i] == '*' && (i + 1 >= n || s[i + 1] != '*')) {
            // Cursiva simple *text*: els models l'usen molt per a títols
            // d'obres. No forma part de **negreta** ni de vinyeta (aquesta
            // ja s'ha resolt per línia). Cal tanca amb el següent * sol.
            var end = -1
            var j = i + 1
            while (j < n) {
                val k = s.indexOf('*', j)
                if (k < 0) break
                if (k + 1 < n && s[k + 1] == '*') { j = k + 2; continue }
                end = k
                break
            }
            if (end > i + 1) {
                val inner = s.substring(i + 1, end)
                val prevOk = i == 0 || s[i - 1].isWhitespace() || s[i - 1] in "([\"'¿¡:;"
                val nextOk = end + 1 >= n || s[end + 1].isWhitespace() || s[end + 1] in ".,;:!?)\"'»—–-"
                if (prevOk && nextOk && inner.isNotBlank() && !inner.startsWith(' ') && !inner.endsWith(' ')) {
                    flush()
                    out += Seg.Italic(inner)
                    i = end + 1
                    continue
                }
            }
        }
        if (s[i] == '`') {
            val end = s.indexOf('`', i + 1)
            if (end > i + 1) {
                flush()
                out += Seg.Code(s.substring(i + 1, end))
                i = end + 1
                continue
            }
        }
        val lb = if (s[i] == '!' && i + 1 < n && s[i + 1] == '[') i + 1 else i
        if (s[lb] == '[') {
            val rb = s.indexOf(']', lb + 1)
            if (rb > lb + 1 && rb + 1 < n && s[rb + 1] == '(') {
                val rp = s.indexOf(')', rb + 2)
                if (rp > rb + 2) {
                    val url = s.substring(rb + 2, rp)
                    if (!url.any { it == ' ' || it == '\n' || it == '\t' }) {
                        flush()
                        val label = s.substring(lb + 1, rb).ifEmpty { url }
                        // ![alt](url) és imatge; [txt](url) és enllaç.
                        out += if (lb == i + 1) Seg.Image(label, url) else Seg.Link(label, url)
                        i = rp + 1
                        continue
                    }
                }
            }
        }
        buf.append(s[i])
        i++
    }
    flush()
    return out
}

private val mdLinkRe = Regex("!?\\[([^\\]]+)\\]\\(([^\\)\\s]+)\\)")
private val mdEmRe = Regex("\\*([^\\s*][^*]*?)\\*")

/**
 * Títol d'una sola línia sense marques markdown: treu encapçalaments, cites,
 * vinyetes, negretes, cursives, codi i enllaços. Per targetes (objectius, sessions).
 */
fun stripMdForTitle(src: String, max: Int = 140): String {
    val line = src.lineSequence().map { it.trim() }.firstOrNull { it.isNotEmpty() } ?: return ""
    return stripMdInline(line).take(max)
}

/** Neteja marques markdown d'un text sencer (totes les línies) sense
 *  retallar-lo: per mostrar el text mentre el vent encara escriu
 *  (streaming), quan el markdown parcial es veuria amb sintaxi crua.
 *  Pura: testejable (MdStreamTest). */
fun stripMdInline(src: String): String {
    if (src.isEmpty()) return ""
    // Línies de tanca de bloc de codi (``` o ```py): fora senceres,
    // que si no hi quedaria l'etiqueta del llenguatge penjada.
    return src.lineSequence().map { raw ->
        var t = raw.trim()
        if (t.startsWith("```")) return@map ""
        t = headingRe.replace(t) { it.groups[2]!!.value }
        t = taskRe.replace(t) { it.groups[2]!!.value }
        t = quoteRe.replace(t) { it.groups[1]!!.value }
        t = bulletRe.replace(t) { it.groups[1]!!.value }
        t = mdLinkRe.replace(t) { it.groups[1]!!.value }
        t = t.replace("**", "").replace("__", "").replace("```", "").replace("`", "")
        // Cursiva simple *títol*: després de treure ** (si no, el regex
        // mossegaria la negreta). L'estel solitari es deixa tal qual.
        t = mdEmRe.replace(t) { it.groups[1]!!.value }
        t.trim()
    }.joinToString("\n").trim()
}

/** Agrupa segments en blocs rics per emetre composables. */
fun groupRich(segs: List<Seg>): List<Rich> {
    val out = mutableListOf<Rich>()
    var run = mutableListOf<Seg>()
    // Els salts de línia que envolten un bloc (imatge, taula, codi) ja els fa
    // l'espaiat entre blocs: si es deixessin, cada bloc tindria una franja buida.
    fun flush() {
        while ((run.firstOrNull() as? Seg.Text)?.t?.isBlank() == true) run.removeAt(0)
        while ((run.lastOrNull() as? Seg.Text)?.t?.isBlank() == true) run.removeAt(run.size - 1)
        (run.firstOrNull() as? Seg.Text)?.let { run[0] = Seg.Text(it.t.trimStart()) }
        (run.lastOrNull() as? Seg.Text)?.let { run[run.size - 1] = Seg.Text(it.t.trimEnd()) }
        if (run.isNotEmpty()) {
            out += Rich.Inline(run)
        }
        run = mutableListOf()
    }
    for (seg in segs) {
        when (seg) {
            is Seg.Block -> {
                flush()
                out += Rich.Code(seg.lang, seg.t)
            }
            is Seg.Image -> {
                flush()
                out += Rich.Image(seg.alt, seg.url)
            }
            is Seg.Table -> {
                flush()
                out += Rich.Table(seg.headers, seg.rows)
            }
            is Seg.Chart -> {
                flush()
                out += Rich.Chart(seg.spec)
            }
            else -> run += seg
        }
    }
    flush()
    return out
}

/** L'accent tal qual no sempre es llegeix: en clar, un groc pàl·lid sobre
 *  blanc és invisible (s'enfosqueix); en fosc, un verd gairebé negre sobre
 *  bombolla fosca tampoc (s'aclareix cap al blanc). Pura: testejable. */
fun readableLink(accent: Color, dark: Boolean): Color {
    val lum = 0.2126f * accent.red + 0.7152f * accent.green + 0.0722f * accent.blue
    if (!dark && lum > 0.45f) return accent.copy(red = accent.red * 0.45f, green = accent.green * 0.45f, blue = accent.blue * 0.45f)
    if (dark && lum < 0.35f) {
        val k = 0.45f
        return accent.copy(red = 1 - (1 - accent.red) * k, green = 1 - (1 - accent.green) * k, blue = 1 - (1 - accent.blue) * k)
    }
    return accent
}

/** Render d'una llista de segments (nucli compartit). El color d'enllaç
 *  entra per paràmetre: la funció és pura (no composable) i Sea és temàtic. */
fun renderSegs(segs: List<Seg>, codeBg: Color, activity: Boolean = false, link: Color = LightPal.sea): AnnotatedString {
    return buildAnnotatedString {
        for (seg in segs) {
            when (seg) {
                is Seg.Text -> if (activity) {
                    withStyle(SpanStyle(fontStyle = androidx.compose.ui.text.font.FontStyle.Italic)) { append(seg.t) }
                } else append(seg.t)
                is Seg.Bold -> withStyle(SpanStyle(fontWeight = FontWeight.Bold)) { append(seg.t) }
                is Seg.Italic -> withStyle(SpanStyle(fontStyle = FontStyle.Italic)) { append(seg.t) }
                is Seg.Code -> withStyle(SpanStyle(fontFamily = FontFamily.Monospace, background = codeBg)) {
                    append(seg.t)
                }
                is Seg.Block -> {
                    append("\n")
                    withStyle(SpanStyle(fontFamily = FontFamily.Monospace, background = codeBg)) {
                        append(seg.t.trimEnd())
                    }
                    append("\n")
                }
                is Seg.Heading -> {
                    val size = when (seg.level) {
                        1 -> 20.sp
                        2 -> 18.sp
                        else -> 16.sp
                    }
                    withStyle(SpanStyle(fontWeight = FontWeight.Bold, fontSize = size)) {
                        append(renderSegs(seg.inner, codeBg, activity, link).text)
                    }
                    append("\n")
                }
                is Seg.Link -> {
                    pushStringAnnotation("URL", seg.url)
                    withStyle(SpanStyle(color = link, textDecoration = TextDecoration.Underline)) {
                        append(seg.label)
                    }
                    pop()
                }
                // Les imatges són bloc propi (Rich.Image); si n'arriba una
                // aquí (títol, cel·la), text pla amb l'alt.
                is Seg.Image -> append(seg.alt)
                is Seg.Table -> {
                    append("\n")
                    val head = seg.headers.joinToString(" | ")
                    withStyle(SpanStyle(fontWeight = FontWeight.Bold)) { append(head) }
                    append("\n")
                    seg.rows.take(12).forEach { r -> append(r.joinToString(" | ") + "\n") }
                }
                // El gràfic és bloc propi (Rich.Chart); en text pla, resum.
                is Seg.Chart -> {
                    append("\n")
                    withStyle(SpanStyle(fontWeight = FontWeight.Bold)) {
                        append("📊 " + seg.spec.title.ifBlank { seg.spec.type })
                    }
                    append("\n")
                    seg.spec.series.forEach { s ->
                        append((if (s.name.isNotBlank()) s.name + ": " else "") + s.values.joinToString(", ", transform = ::fmtNum) + "\n")
                    }
                }
            }
        }
    }
}

/** Render clàssic des de text (negreta, codi, títols, enllaços, taules). */
fun renderMd(src: String, codeBg: Color, activity: Boolean = false): AnnotatedString {
    return renderSegs(splitMd(src), codeBg, activity)
}

/**
 * Blocs d'un missatge amb cau global (LRU, 256 entrades): parsejar el mateix
 * text a cada recomposició (i a cada token del streaming, i en tornar a la
 * pantalla) era el que feia l'scroll a salts. El parse és determinista, així
 * que compartir-lo entre bombolles i pantalles és segur. Pura: testejable.
 */
private val mdBlocksCache = object : LinkedHashMap<String, List<Rich>>(128, 0.75f, true) {
    override fun removeEldestEntry(eldest: MutableMap.MutableEntry<String, List<Rich>>?): Boolean = size > 256
}

fun richBlocks(src: String): List<Rich> = synchronized(mdBlocksCache) {
    mdBlocksCache[src] ?: groupRich(splitMd(src)).also { mdBlocksCache[src] = it }
}

/** Quantes entrades hi ha a la cau (només tests). */
internal fun richBlocksCached(): Int = synchronized(mdBlocksCache) { mdBlocksCache.size }
