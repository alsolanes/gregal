package xyz.solanes.gregal

/**
 * Un torn que ha perdut el flux (has sortit de l'app mentre el vent
 * treballava): el servidor el continua, el desa i t'avisa quan acaba. El
 * client no el dona per fallat; va mirant el fil fins que hi és la resposta.
 */
object DetachedTurn {
    /** Com a molt, el que pot durar un torn al servidor (winds: 15 min) i marge. */
    const val WAIT_MS = 20 * 60_000L
    const val POLL_MS = 3_000L
    /** Preguntes seguides sense xarxa abans de rendir-se (~3 min). */
    const val MAX_MISSES = 60

    /**
     * Hi ha la resposta d'aquest torn al fil del servidor? Després del
     * missatge enviat ([sent]; buit a «Continua») hi ha d'haver una resposta
     * del vent que el mòbil no tenia: per id i text, perquè «Continua» allarga
     * el mateix missatge. No es fa servir l'hora: el rellotge del mòbil i el
     * del servidor poden no coincidir.
     */
    fun answered(hist: List<GregalViewModel.PetMsg>, known: List<GregalViewModel.PetMsg>, sent: String): Boolean {
        val start = if (sent.isNotBlank()) hist.indexOfLast { it.from == "user" && it.text.trim() == sent.trim() } else -1
        if (sent.isNotBlank() && start < 0) return false
        val seen = known.filter { it.from == "pet" }.map { it.id to it.text.trim() }.toSet()
        return hist.drop(start + 1).any { it.from == "pet" && it.text.isNotBlank() && (it.id to it.text.trim()) !in seen }
    }
}
