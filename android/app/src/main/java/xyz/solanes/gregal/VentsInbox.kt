package xyz.solanes.gregal

/** Plain-data projection shared by home/search; timestamps are backend/client facts, never synthesized. */
internal data class VentsInboxRow(
    val key: String,
    val title: String,
    val preview: String,
    val members: List<Wind>,
    val timestamp: Long,
)

private val MARKS = Regex("\\p{M}+")

internal fun ventsInbox(
    winds: List<Wind>, groups: List<Rotllana>,
    chats: Map<String, List<GregalViewModel.PetMsg>>,
    groupChats: Map<String, List<GregalViewModel.PetMsg>>,
    query: String = "",
): List<VentsInboxRow> {
    fun searchable(text: String): String = java.text.Normalizer.normalize(text, java.text.Normalizer.Form.NFD)
        .replace(MARKS, "").lowercase(java.util.Locale.ROOT)
    val needle = searchable(query.trim())
    val rows = mutableListOf<Pair<VentsInboxRow, Boolean>>()
    fun add(key: String, title: String, fallback: String, members: List<Wind>, messages: List<GregalViewModel.PetMsg>) {
        val identityMatches = needle.isEmpty() || searchable(title + " " + fallback + " " + members.joinToString(" ") { it.name + " " + it.role }).contains(needle)
        val match = if (needle.isEmpty()) null else messages.lastOrNull { searchable(it.text).contains(needle) }
        if (!identityMatches && match == null) return
        val preview = (if (!identityMatches) match else messages.lastOrNull())?.text ?: fallback
        rows += VentsInboxRow(key, title, stripMdInline(preview).replace(Regex("\\s+"), " ").trim(), members,
            messages.maxOfOrNull { it.ts } ?: 0L) to messages.isNotEmpty()
    }
    winds.forEach { w -> add("wind:${w.id}", w.name, w.role, listOf(w), chats[w.id].orEmpty()) }
    groups.forEach { g ->
        val members = g.members.mapNotNull { id -> winds.firstOrNull { it.id == id } }
        add("rot:${g.id}", g.name, members.joinToString(", ") { it.name }, members, groupChats[g.id].orEmpty())
    }
    return rows.sortedWith(compareByDescending<Pair<VentsInboxRow, Boolean>> { it.first.timestamp }
        .thenByDescending { it.second }).map { it.first }
}
