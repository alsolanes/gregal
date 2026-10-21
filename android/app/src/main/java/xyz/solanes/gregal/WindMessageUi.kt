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
