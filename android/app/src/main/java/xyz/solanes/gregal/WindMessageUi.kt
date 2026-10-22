package xyz.solanes.gregal

internal data class ResolvedWindMessage(val message: GregalViewModel.PetMsg, val wind: Wind?, val name: String)

/** Only strip a legacy speaker prefix when it identifies a real member; URLs and ordinary colons survive. */
internal fun resolveWindMessage(message: GregalViewModel.PetMsg, members: List<Wind>): ResolvedWindMessage {
    if (message.from == "user") return ResolvedWindMessage(message, null, "")
    val explicit = members.firstOrNull { it.id == message.speakerId }
        ?: members.firstOrNull { message.speakerName.isNotBlank() && it.name.equals(message.speakerName, ignoreCase = true) }
    val legacyName = message.text.substringBefore(':', "").trim()
    val legacy = members.firstOrNull { it.name.equals(legacyName, ignoreCase = true) }
    val wind = explicit ?: legacy
    val text = if (legacy != null && (explicit == null || explicit.id == legacy.id)) message.text.substringAfter(':').trimStart() else message.text
    return ResolvedWindMessage(message.copy(text = text), wind, wind?.name ?: message.speakerName)
}

/** Clau de ratxa: dos missatges consecutius amb la mateixa clau són del
 *  mateix parlant i comparteixen cara+nom (només el primer els mostra).
 *  Pura: testejable. */
internal fun runKeyOf(r: ResolvedWindMessage): String =
    (if (r.message.from == "user") "user" else "w") + "|" + (r.wind?.id ?: r.name)

/** L'@menció que s'està escrivint: posició de l'@, fragment i candidates. */
internal data class MentionQuery(val at: Int, val frag: String, val cands: List<Wind>)

/** Troba l'@menció activa (cursor al final del token) per al desplegable.
 *  Pura: testejable. */
internal fun activeMention(text: String, cursor: Int, winds: List<Wind>): MentionQuery? {
    if (cursor <= 0 || cursor > text.length) return null
    val at = text.lastIndexOf('@', cursor - 1)
    if (at < 0) return null
    if (at > 0 && !text[at - 1].isWhitespace() && text[at - 1] != '\n') return null
    val frag = text.substring(at + 1, cursor)
    if (frag.any { it.isWhitespace() } || frag.length > 24) return null
    val cands = winds.filter { it.name.startsWith(frag, ignoreCase = true) }
    if (cands.isEmpty()) return null
    return MentionQuery(at, frag, cands)
}

/** Resol l'@Nom d'un missatge enviat al vent corresponent (primer que
 *  coincideixi, insensible a majúscules). La menció es queda visible
 *  al text: fa de testimoni del traspàs. Pura: testejable. */
internal fun resolveMention(text: String, winds: List<Wind>): Wind? {
    val re = Regex("@([\\p{L}][\\p{L}\\-']*)")
    for (m in re.findAll(text)) {
        winds.firstOrNull { it.name.equals(m.groupValues[1], ignoreCase = true) }?.let { return it }
    }
    return null
}
