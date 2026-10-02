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
    data class Code(val t: String) : Seg
    data class Block(val lang: String, val t: String) : Seg
    data class Heading(val level: Int, val inner: List<Seg>) : Seg
    data class Link(val label: String, val url: String) : Seg
    data class Table(val headers: List<String>, val rows: List<List<String>>) : Seg
}

/** Bloc ric per emetre composables (text corregut, codi o taula). */
sealed interface Rich {
    data class Inline(val segs: List<Seg>) : Rich
    data class Code(val lang: String, val code: String) : Rich
    data class Table(val headers: List<String>, val rows: List<List<String>>) : Rich
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
            out += listOf(Seg.Block(inner.substring(0, nl), inner.substring(nl + 1)))
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
                out += Seg.Bold(s.substring(i + 2, end))
                i = end + 2
                continue
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
                        out += Seg.Link(label, url)
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

private val mdLinkRe = Regex("\\[([^\\]]+)\\]\\(([^)\\s]+)\\)")

/**
 * Títol d'una sola línia sense marques markdown: treu encapçalaments, cites,
 * vinyetes, negretes, codi i enllaços. Per targetes (objectius, sessions).
 */
fun stripMdForTitle(src: String): String {
    val line = src.lineSequence().map { it.trim() }.firstOrNull { it.isNotEmpty() } ?: return ""
    var t = line
    t = headingRe.replace(t) { it.groups[2]!!.value }
    t = quoteRe.replace(t) { it.groups[1]!!.value }
    t = bulletRe.replace(t) { it.groups[1]!!.value }
    t = mdLinkRe.replace(t) { it.groups[1]!!.value }
    t = t.replace("**", "").replace("__", "").replace("`", "")
    return t.trim().take(140)
}

/** Agrupa segments en blocs rics per emetre composables. */
fun groupRich(segs: List<Seg>): List<Rich> {
    val out = mutableListOf<Rich>()
    var run = mutableListOf<Seg>()
    fun flush() {
        if (run.isNotEmpty()) {
            out += Rich.Inline(run)
            run = mutableListOf()
        }
    }
    for (seg in segs) {
        when (seg) {
            is Seg.Block -> {
                flush()
                out += Rich.Code(seg.lang, seg.t)
            }
            is Seg.Table -> {
                flush()
                out += Rich.Table(seg.headers, seg.rows)
            }
            else -> run += seg
        }
    }
    flush()
    return out
}

/** Render d'una llista de segments (nucli compartit). El color d'enllaç
 *  entra per paràmetre: la funció és pura (no composable) i Sea és temàtic. */
fun renderSegs(segs: List<Seg>, codeBg: Color, activity: Boolean = false, link: Color = Color(0xFF0E8C74)): AnnotatedString {
    return buildAnnotatedString {
        for (seg in segs) {
            when (seg) {
                is Seg.Text -> if (activity) {
                    withStyle(SpanStyle(fontStyle = androidx.compose.ui.text.font.FontStyle.Italic)) { append(seg.t) }
                } else append(seg.t)
                is Seg.Bold -> withStyle(SpanStyle(fontWeight = FontWeight.Bold)) { append(seg.t) }
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
                is Seg.Table -> {
                    append("\n")
                    val head = seg.headers.joinToString(" | ")
                    withStyle(SpanStyle(fontWeight = FontWeight.Bold)) { append(head) }
                    append("\n")
                    seg.rows.take(12).forEach { r -> append(r.joinToString(" | ") + "\n") }
                }
            }
        }
    }
}

/** Render clàssic des de text (negreta, codi, títols, enllaços, taules). */
fun renderMd(src: String, codeBg: Color, activity: Boolean = false): AnnotatedString {
    return renderSegs(splitMd(src), codeBg, activity)
}
