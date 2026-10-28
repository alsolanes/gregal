package xyz.solanes.gregal

/** Plain-data projection shared by home/search; timestamps are backend/client facts, never synthesized. */
internal data class VentsInboxRow(
    val key: String,
    val title: String,
    val preview: String,
    val members: List<Wind>,
    val timestamp: Long,
    /** Hi ha missatges, encara que siguin antics i sense hora. */
    val hasMessages: Boolean = timestamp > 0,
)

private val MARKS = Regex("\\p{M}+")

internal fun ventsInbox(
    winds: List<Wind>, groups: List<Rotllana>,
    chats: Map<String, List<GregalViewModel.PetMsg>>,
    groupChats: Map<String, List<GregalViewModel.PetMsg>>,
    query: String = "",
    /** Últim missatge de cada conversa (key → resum): la portada no carrega tots els fils. */
    summaries: Map<String, ConvoSummary> = emptyMap(),
): List<VentsInboxRow> {
    fun searchable(text: String): String = java.text.Normalizer.normalize(text, java.text.Normalizer.Form.NFD)
        .replace(MARKS, "").lowercase(java.util.Locale.ROOT)
    val needle = searchable(query.trim())
    val rows = mutableListOf<Pair<VentsInboxRow, Boolean>>()
    fun add(key: String, title: String, fallback: String, members: List<Wind>, messages: List<GregalViewModel.PetMsg>) {
        val identityMatches = needle.isEmpty() || searchable(title + " " + fallback + " " + members.joinToString(" ") { it.name + " " + it.role }).contains(needle)
        val match = if (needle.isEmpty()) null else messages.lastOrNull { searchable(it.text).contains(needle) }
        if (!identityMatches && match == null) return
        // Sense el fil carregat (o amb un resum més nou), mana el resum del servidor.
        val sum = summaries[key]?.takeIf { it.count > 0 }
        val chatTs = messages.maxOfOrNull { it.ts } ?: 0L
        val useSum = sum != null && match == null && (messages.isEmpty() || sum.ts > chatTs)
        val preview = when {
            useSum -> (if (sum!!.speaker.isNotBlank() && key.startsWith("rot:")) sum.speaker + ": " else "") + sum.text
            else -> ((if (!identityMatches) match else messages.lastOrNull())?.text ?: fallback)
        }
        val has = messages.isNotEmpty() || sum != null
        rows += VentsInboxRow(key, title, stripMdInline(preview).replace(Regex("\\s+"), " ").trim(), members,
            maxOf(chatTs, sum?.ts ?: 0L), has) to has
    }
    winds.forEach { w -> add("wind:${w.id}", w.name, w.role, listOf(w), chats[w.id].orEmpty()) }
    groups.forEach { g ->
        val members = g.members.mapNotNull { id -> winds.firstOrNull { it.id == id } }
        add("rot:${g.id}", g.name, members.joinToString(", ") { it.name }, members, groupChats[g.id].orEmpty())
    }
    return rows.sortedWith(compareByDescending<Pair<VentsInboxRow, Boolean>> { it.first.timestamp }
        .thenByDescending { it.second }).map { it.first }
}

/**
 * L'hora d'una fila: avui l'hora, ahir «ahir», aquesta setmana el dia i,
 * si és més antic, la data. Només l'hora feia semblar més nou un xat d'ahir.
 */
internal fun inboxWhen(ms: Long, lang: String, now: Long = System.currentTimeMillis(),
                       zone: java.time.ZoneId = java.time.ZoneId.systemDefault()): String {
    val at = java.time.Instant.ofEpochMilli(ms).atZone(zone)
    val today = java.time.Instant.ofEpochMilli(now).atZone(zone).toLocalDate()
    val days = java.time.temporal.ChronoUnit.DAYS.between(at.toLocalDate(), today)
    val locale = java.util.Locale.forLanguageTag(if (lang == "en") "en" else "ca")
    fun f(p: String) = java.time.format.DateTimeFormatter.ofPattern(p, locale).format(at)
    return when {
        days <= 0L -> f("HH:mm")
        days == 1L -> tr(lang, "app.inbox.yesterday")
        days < 7L -> f("EEE")
        else -> f("d MMM")
    }
}

/** L'últim missatge d'una conversa (GET /api/winds/summaries); ts en mil·lisegons. */
data class ConvoSummary(val key: String, val from: String, val speaker: String, val text: String, val ts: Long, val count: Int)
