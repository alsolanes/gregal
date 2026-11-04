package xyz.solanes.gregal

/**
 * Nom llegible d'un model: «deepseek-v4.1-flash» → «DeepSeek V4.1 Flash». La
 * mateixa lògica que la web (model-name.js); l'identificador complet es
 * continua veient al menú i als detalls.
 */
private val BRANDS = mapOf(
    "deepseek" to "DeepSeek", "gpt" to "GPT", "claude" to "Claude", "qwen" to "Qwen", "llama" to "Llama",
    "gemini" to "Gemini", "mistral" to "Mistral", "glm" to "GLM", "kimi" to "Kimi", "grok" to "Grok",
    "gemma" to "Gemma", "phi" to "Phi", "minimax" to "MiniMax", "codestral" to "Codestral",
    "devstral" to "Devstral", "command" to "Command", "sonar" to "Sonar",
)
private val ATTACH = setOf("qwen")
private val IGNORE = setOf("latest")
private val RE_VERSION = Regex("^[0-9]+([.][0-9]+)*$")
private val RE_SIZE = Regex("^[0-9]+(x[0-9]+)?[bkm]$")
private val RE_ALNUM_SIZE = Regex("^[a-z][0-9]+[bkm]$")
private val RE_DATE = Regex("-(20[0-9]{6}|[0-9]{4})$")
private val RE_BRAND_NUM = Regex("^([a-z]+)([0-9][[0-9].]*)(.*)$")

private fun modelWord(t: String): String = when {
    RE_VERSION.matches(t) -> t
    t.startsWith("v") && t.length > 1 && t[1].isDigit() -> "V" + t.drop(1)
    RE_SIZE.matches(t) -> t.dropLast(1) + t.last().uppercaseChar()
    t.firstOrNull()?.isDigit() == true -> t
    t.length > 1 && t[0] == 'o' && t[1].isDigit() -> t
    RE_ALNUM_SIZE.matches(t) -> t.uppercase()
    else -> t.replaceFirstChar { it.uppercaseChar() }
}

fun prettyModelName(id: String?): String {
    var s = id.orEmpty().trim()
    if (s.isEmpty()) return ""
    val original = s
    s = s.substringAfterLast('/').lowercase()
    var tag = ""
    val colon = s.indexOf(':')
    if (colon >= 0) { tag = s.substring(colon + 1); s = s.substring(0, colon) }
    s = s.replace(RE_DATE, "")
    val toks = s.split('-', '_').filter { it.isNotEmpty() }.toMutableList()
    val m = RE_BRAND_NUM.find(s)
    if (m != null && BRANDS.containsKey(m.groupValues[1]) && toks.isNotEmpty() && Regex("^[a-z]+[0-9]").containsMatchIn(toks[0]) && m.groupValues[1] !in ATTACH) {
        toks[0] = m.groupValues[1]
        toks.add(1, m.groupValues[2] + m.groupValues[3].takeIf { it.isNotEmpty() && !it.startsWith("-") }.orEmpty())
    }
    val merged = mutableListOf<String>()
    for (t in toks) {
        val prev = merged.lastOrNull()
        if (t.length <= 2 && t.all { it.isDigit() } && prev != null && RE_VERSION.matches(prev)) merged[merged.size - 1] = "$prev.$t"
        else merged += t
    }
    if (tag.isNotEmpty() && tag !in IGNORE) merged += tag
    val words = merged.filter { it !in IGNORE }.map { BRANDS[it] ?: modelWord(it) }.toMutableList()
    if (words.size > 1 && words[0] == "GPT" && words[1].firstOrNull()?.isDigit() == true) {
        val v = words[1]; words.removeAt(1); words[0] = "GPT-$v"
    }
    return words.joinToString(" ").ifEmpty { original }
}
