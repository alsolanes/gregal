package xyz.solanes.gregal

/*
 * #retro: el xat tipus Slack/Teams amb els agents d'un torn acabat (la web:
 * internal/web/app/retro.js; el servidor: internal/web/retro.go). Cada agent
 * explica què ha fet amb «rebuts» (e1, e2…) que apunten als events reals, i
 * se'ls pot preguntar o rebatre. Aquí hi ha el model i el parser (purs, amb
 * MiniJson perquè els tests de JVM no tenen org.json).
 */

data class RetroAgent(val id: String, val name: String, val avatar: String, val model: String = "")

data class RetroReaction(val emoji: String, val by: List<String>) {
    val mine: Boolean get() = "user" in by
}

data class RetroMsg(
    val id: String,
    val thread: String,
    val author: String,
    val text: String,
    val receipts: List<String> = emptyList(),
    val reactions: List<RetroReaction> = emptyList(),
    val concede: Boolean = false,
    val fix: String = "",
    val at: String = "",
    /** Revisió de l'equip: finding (amb severity high|medium|low i title) o verdict. */
    val kind: String = "",
    val severity: String = "",
    val title: String = "",
    val fixRun: Long = 0,
) {
    val isFinding: Boolean get() = kind == "finding"
    val sev: String get() = if (severity in RETRO_SEVERITIES) severity else "low"
}

val RETRO_SEVERITIES = listOf("high", "medium", "low")

/** Troballes del canal principal per gravetat (la barra de revisió). */
fun retroFindingCounts(msgs: List<RetroMsg>): Map<String, Int> =
    msgs.filter { it.isFinding && it.thread.isEmpty() }.groupingBy { it.sev }.eachCount()

/** Missatges amb el filtre de la barra: "" tots, "all" les troballes, o una gravetat. */
fun retroFilter(msgs: List<RetroMsg>, filter: String): List<RetroMsg> =
    if (filter.isEmpty()) msgs
    else msgs.filter { it.isFinding && it.thread.isEmpty() && (filter == "all" || it.sev == filter) }

/** Un mood del #retro: to real del model, no decoració. */
data class RetroMood(val id: String)

/** Els 6 tons que funcionen de debò al backend (docs/api-contract.md). */
val RETRO_MOODS = listOf(
    RetroMood("neutral"),
    RetroMood("sarcastic"),
    RetroMood("playful"),
    RetroMood("sharp"),
    RetroMood("flirty"),
    RetroMood("serious"),
)

data class RetroChannel(
    val key: String,
    val title: String,
    val degraded: Boolean,
    val agents: List<RetroAgent>,
    val messages: List<RetroMsg>,
    val persona: String = "neutral",
) {
    fun agent(id: String): RetroAgent? = agents.firstOrNull { it.id == id }
}

/** Resposta de GET /api/retro: el canal (si ja existeix) i l'últim torn. */
data class RetroInfo(
    val channel: RetroChannel?,
    val key: String,
    val latestRun: Long,
    val latestRunning: Boolean,
    val emojis: List<String>,
)

val RETRO_DEFAULT_EMOJIS = listOf("👍", "🎉", "🔥", "🤔", "👀", "✅", "👎", "😂", "🙏")

@Suppress("UNCHECKED_CAST")
fun parseRetroChannel(o: Map<String, Any?>?): RetroChannel? {
    if (o == null) return null
    fun s(m: Map<String, Any?>, k: String) = m[k] as? String ?: ""
    val agents = (o["agents"] as? List<*>).orEmpty().mapNotNull { a ->
        (a as? Map<String, Any?>)?.let { RetroAgent(s(it, "id"), s(it, "name"), s(it, "avatar"), s(it, "model")) }
    }
    val msgs = (o["messages"] as? List<*>).orEmpty().mapNotNull { x ->
        val m = x as? Map<String, Any?> ?: return@mapNotNull null
        RetroMsg(
            id = s(m, "id"), thread = s(m, "thread_id"), author = s(m, "author"), text = s(m, "text"),
            receipts = (m["receipts"] as? List<*>).orEmpty().mapNotNull { it as? String },
            reactions = (m["reactions"] as? List<*>).orEmpty().mapNotNull { r ->
                (r as? Map<String, Any?>)?.let { RetroReaction(s(it, "emoji"), (it["by"] as? List<*>).orEmpty().mapNotNull { b -> b as? String }) }
            }.filter { it.by.isNotEmpty() },
            concede = m["concede"] == true,
            fix = s(m, "fix_suggestion"),
            at = s(m, "at"),
            kind = s(m, "kind"),
            severity = s(m, "severity"),
            title = s(m, "title"),
            fixRun = (m["fix_run"] as? Double)?.toLong() ?: 0L,
        )
    }
    val persona = (s(o, "persona").ifBlank { "neutral" }).takeIf { it in RETRO_MOODS.map { m -> m.id } } ?: "neutral"
    return RetroChannel(s(o, "key"), s(o, "title"), o["degraded"] == true, agents, msgs, persona)
}

@Suppress("UNCHECKED_CAST")
fun parseRetroInfo(json: String): RetroInfo {
    val o = MiniJson.obj(json) ?: return RetroInfo(null, "", 0, false, RETRO_DEFAULT_EMOJIS)
    val latest = o["latest"] as? Map<String, Any?> ?: emptyMap()
    return RetroInfo(
        channel = parseRetroChannel(o["channel"] as? Map<String, Any?>),
        key = o["key"] as? String ?: "",
        latestRun = (latest["run_id"] as? Double)?.toLong() ?: 0L,
        latestRunning = latest["running"] == true,
        emojis = (o["emojis"] as? List<*>)?.mapNotNull { it as? String }?.takeIf { it.isNotEmpty() } ?: RETRO_DEFAULT_EMOJIS,
    )
}

/** Cossos de les peticions del #retro que porten to (contracte a docs/api-contract.md). */
object RetroRequests {
    fun rewrite(key: String, persona: String): String =
        org.json.JSONObject().put("key", key).put("persona", persona).toString()
}

/** Canal dins d'una resposta de start/message/react ({"channel": …}). */
@Suppress("UNCHECKED_CAST")
fun parseRetroChannelResponse(json: String): RetroChannel? =
    parseRetroChannel(MiniJson.obj(json)?.get("channel") as? Map<String, Any?>)

/** Missatges en ordre de lectura: cada fil just sota el missatge que respon. */
fun retroOrdered(msgs: List<RetroMsg>): List<Pair<RetroMsg, Boolean>> {
    val byThread = msgs.filter { it.thread.isNotEmpty() }.groupBy { it.thread }
    val ids = msgs.map { it.id }.toSet()
    val out = mutableListOf<Pair<RetroMsg, Boolean>>()
    for (m in msgs) {
        if (m.thread.isNotEmpty() && m.thread in ids) continue
        out += m to false
        byThread[m.id]?.forEach { out += it to true }
    }
    return out
}

/** Hora curta d'un RFC3339 ("2026-10-14T10:37:12Z" → "10:37", en hora local). */
fun retroClock(at: String): String = try {
    val t = java.time.OffsetDateTime.parse(at).atZoneSameInstant(java.time.ZoneId.systemDefault())
    "%02d:%02d".format(t.hour, t.minute)
} catch (_: Exception) { "" }
