package xyz.solanes.gregal

/** Text humà per al diàleg d'aprovacions (pur, sense android.jar: testeable a JVM). */

fun toolTitle(name: String, lang: String = "ca"): String {
    val key = when (name) {
        "read" -> "app.tool.title.read"
        "bash" -> "app.tool.title.bash"
        "write" -> "app.tool.title.write"
        "edit" -> "app.tool.title.edit"
        "grep" -> "app.tool.title.search"
        "glob" -> "app.tool.title.list"
        "web_search" -> "app.tool.title.webSearch"
        "web_fetch" -> "app.tool.title.webFetch"
        else -> if (name.startsWith("mcp_")) "app.tool.title.external" else return name
    }
    return tr(lang, key)
}

private val fieldRe = Regex("\"([^\"]+)\"\\s*:\\s*(\"(?:[^\"\\\\]|\\\\.)*\"|-?[0-9.]+|true|false|null)")

private fun jsonFields(src: String): Map<String, String> {
    val out = mutableMapOf<String, String>()
    for (m in fieldRe.findAll(src)) {
        var v = m.groups[2]!!.value
        if (v.startsWith("\"")) {
            v = v.substring(1, v.length - 1)
                .replace("\\n", "\n").replace("\\t", "\t").replace("\\\"", "\"").replace("\\\\", "\\")
        }
        out[m.groups[1]!!.value] = v
    }
    return out
}

fun toolSummary(name: String, argsJson: String, lang: String = "ca"): String {
    val f = try { jsonFields(argsJson) } catch (_: Exception) { emptyMap() }
    val key = when (name) {
        "read", "write", "edit" -> "path"
        "bash" -> "command"
        "grep" -> "pattern"
        "glob" -> "pattern"
        "web_search" -> "query"
        "web_fetch" -> "url"
        else -> ""
    }
    val v = (f[key] ?: "").take(140)
    val verbKey = when (name) {
        "read", "web_fetch" -> "app.tool.verb.read"
        "bash" -> "app.tool.verb.run"
        "write" -> "app.tool.verb.write"
        "edit" -> "app.tool.verb.edit"
        "grep", "glob", "web_search" -> "app.tool.verb.search"
        else -> "app.tool.verb.use"
    }
    val verb = tr(lang, verbKey)
    if (v.isEmpty()) return "$verb: ${toolTitle(name, lang).lowercase()}."
    return "$verb: $v"
}

/** Arguments una clau per línia (sense org.json, que no existeix als tests JVM). */
fun prettyArgs(argsJson: String, lang: String = "ca"): String {
    val t = argsJson.trim()
    if (t.isEmpty() || t == "{}") return tr(lang, "app.tool.noArgs")
    if (!t.startsWith("{") || !t.endsWith("}")) return t.take(800)
    val parts = mutableListOf<String>()
    var depth = 0
    var inStr = false
    var esc = false
    var cur = StringBuilder()
    for (c in t.substring(1, t.length - 1)) {
        if (inStr) {
            cur.append(c)
            if (esc) esc = false else if (c == '\\') esc = true else if (c == '"') inStr = false
            continue
        }
        when (c) {
            '"' -> { inStr = true; cur.append(c) }
            '{', '[' -> { depth++; cur.append(c) }
            '}', ']' -> { depth--; cur.append(c) }
            ',' -> if (depth == 0) {
                parts += cur.toString().trim()
                cur = StringBuilder()
            } else cur.append(c)
            else -> cur.append(c)
        }
    }
    if (cur.toString().isNotBlank()) parts += cur.toString().trim()
    if (parts.isEmpty()) return t.take(800)
    return parts.joinToString("\n") { p ->
        val t2 = p.trim()
        val m = Regex("^\"([^\"]+)\"\\s*:\\s*(.+)$").find(t2) ?: return@joinToString t2
        var v = m.groups[2]!!.value.trim()
        if (v.startsWith("\"") && v.endsWith("\"") && v.length >= 2) {
            v = v.substring(1, v.length - 1)
        }
        "${m.groups[1]!!.value}: $v"
    }.take(800)
}
