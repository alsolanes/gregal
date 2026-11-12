package xyz.solanes.gregal

import java.util.Calendar
import java.util.Locale
import java.util.TimeZone

/*
 * Regles pures del fil de l'assistent (B1): on va cada missatge, com
 * s'agrupen en ratxes, quan surt l'hora i on comencen les novetats. Tot aquí
 * és testejable sense Compose; els composables només ho pinten.
 */

/** On es pinta una resposta de l'assistent. */
internal enum class ChatSurface {
    /** Bombolla amb fons suau (`surfaceHi`), radi 22 dp i ample màxim 84 %. */
    BUBBLE,
    /** Superfície de lectura: sense caixa i a tot l'ample. Per a taules,
     *  codi, gràfics i respostes llargues, que dins d'una bombolla queden
     *  estretes i feixugues. */
    READING,
}

/** A partir d'aquí una resposta es llegeix millor sense bombolla. */
internal const val READING_CHARS = 900

/**
 * La regla «bombolla o superfície de lectura». Una resposta amb una taula,
 * un bloc de codi o un gràfic, o de més de [READING_CHARS] caràcters, va a la
 * superfície de lectura; la resta, en bombolla. Les imatges soles no compten:
 * una foto dins una bombolla es veu bé. Pura: testejable.
 */
internal fun chatSurfaceFor(text: String, blocks: List<Rich> = richBlocksLive(text)): ChatSurface = when {
    text.trim().length > READING_CHARS -> ChatSurface.READING
    blocks.any { it is Rich.Table || it is Rich.Code || it is Rich.Chart } -> ChatSurface.READING
    else -> ChatSurface.BUBBLE
}

/** Clau estable d'un missatge a la llista: l'id del servidor; l'índex només
 *  quan el servidor és antic i no n'envia (B0, §4 del contracte). */
internal fun threadItemKey(id: String, index: Int): String =
    if (id.isNotBlank()) "m:$id" else "i:$index"

/** El que la llista necessita saber de cada missatge per agrupar-lo. */
internal data class ThreadEntry(val from: String, val ts: Long)

/** Com es pinta un missatge dins del fil. */
internal data class ThreadSlot(
    /** Primer de la seva ratxa: aire gran a sobre (14 dp) en lloc de 3 dp. */
    val firstOfRun: Boolean,
    /** Últim de la ratxa: porta la cua i l'hora. */
    val lastOfRun: Boolean,
    /** Si no és null, separador «Avui · 09:41» just a sobre (en millis). */
    val separatorAt: Long?,
)

/** Més d'aquest temps entre dos missatges posa un separador amb l'hora. */
internal const val THREAD_PAUSE_MS = 30L * 60 * 1000

/** L'hora del servidor arriba en segons Unix; la local, en millis. */
internal fun tsToMillis(ts: Long): Long = if (ts <= 0L) 0L else if (ts > 10_000_000_000L) ts else ts * 1000L

/**
 * Ratxes i separadors. Dos missatges seguits del mateix parlant formen una
 * ratxa (3 dp entre ells, cua i hora només a l'últim). Un separador amb
 * l'hora surt davant del primer missatge amb hora i quan han passat més de
 * 30 minuts des de l'anterior; aleshores també es trenca la ratxa. Els
 * missatges sense hora (servidors antics) no posen ni trenquen separadors.
 * Pura: testejable.
 */
internal fun threadSlots(entries: List<ThreadEntry>): List<ThreadSlot> {
    val sep = arrayOfNulls<Long>(entries.size)
    var lastTs = 0L
    for (i in entries.indices) {
        val t = tsToMillis(entries[i].ts)
        if (t > 0L) {
            if (lastTs == 0L || t - lastTs > THREAD_PAUSE_MS) sep[i] = t
            lastTs = t
        }
    }
    fun sameRun(a: Int, b: Int) = entries[a].from == entries[b].from && sep[b] == null
    return entries.indices.map { i ->
        ThreadSlot(
            firstOfRun = i == 0 || !sameRun(i - 1, i),
            lastOfRun = i == entries.lastIndex || !sameRun(i, i + 1),
            separatorAt = sep[i],
        )
    }
}

/**
 * L'etiqueta del separador: «Avui · 09:41», «Ahir · 21:10» o «12 de nov. ·
 * 09:41». Pura (rep l'hora d'ara i la zona): testejable.
 */
internal fun threadSeparatorLabel(
    lang: String, atMillis: Long, nowMillis: Long = System.currentTimeMillis(),
    zone: TimeZone = TimeZone.getDefault(),
): String {
    val locale = if (lang == "en") Locale.ENGLISH else Locale.forLanguageTag("ca")
    fun cal(ms: Long) = Calendar.getInstance(zone, locale).apply { timeInMillis = ms }
    val at = cal(atMillis)
    val now = cal(nowMillis)
    fun dayIndex(c: Calendar) = c.get(Calendar.YEAR) * 400 + c.get(Calendar.DAY_OF_YEAR)
    val yesterday = cal(nowMillis).apply { add(Calendar.DAY_OF_YEAR, -1) }
    val hm = String.format(Locale.ROOT, "%02d:%02d", at.get(Calendar.HOUR_OF_DAY), at.get(Calendar.MINUTE))
    val day = when (dayIndex(at)) {
        dayIndex(now) -> threadCopy(lang, "today")
        dayIndex(yesterday) -> threadCopy(lang, "yesterday")
        else -> {
            val fmt = java.text.SimpleDateFormat("d MMM", locale).apply { timeZone = zone }
            val d = fmt.format(java.util.Date(atMillis))
            if (at.get(Calendar.YEAR) == now.get(Calendar.YEAR)) d else d + " " + at.get(Calendar.YEAR)
        }
    }
    return "$day · $hm"
}

/** L'hora curta d'un missatge (09:41), buida si no en porta. */
internal fun threadTime(ts: Long, zone: TimeZone = TimeZone.getDefault()): String {
    val ms = tsToMillis(ts)
    if (ms <= 0L) return ""
    val c = Calendar.getInstance(zone).apply { timeInMillis = ms }
    return String.format(Locale.ROOT, "%02d:%02d", c.get(Calendar.HOUR_OF_DAY), c.get(Calendar.MINUTE))
}

/**
 * On va el separador local «Una novetat per a tu»: davant del primer
 * missatge de l'assistent que ha arribat després de l'últim que vas veure
 * ([lastSeenId]). Sense marca (primera obertura) o si la marca ja no és al
 * fil, no se'n posa cap: millor no dir res que marcar tot el fil com a nou.
 * Un missatge teu després de la marca vol dir que ja hi eres: tampoc.
 * Pura: testejable.
 */
internal fun firstUnreadIndex(ids: List<String>, froms: List<String>, lastSeenId: String): Int? {
    if (lastSeenId.isBlank()) return null
    val seen = ids.lastIndexOf(lastSeenId)
    if (seen < 0) return null
    val next = seen + 1
    return if (next < ids.size && froms[next] != "user") next else null
}

/** Textos del fil nou. Paritat: la web encara no té aquests elements (veure l'informe de B1). */
internal fun threadCopy(lang: String, key: String): String {
    val en = lang == "en"
    return when (key) {
        "today" -> if (en) "Today" else "Avui"
        "yesterday" -> if (en) "Yesterday" else "Ahir"
        "unread" -> if (en) "Something new for you" else "Una novetat per a tu"
        "menu" -> if (en) "Menu" else "Menú"
        "computer" -> if (en) "Computer" else "Ordinador"
        "working" -> if (en) "working…" else "treballant…"
        "ready" -> if (en) "Ready on your computer" else "Preparat al teu ordinador"
        "seeWork" -> if (en) "See work" else "Veure feina"
        "open" -> if (en) "Open" else "Obre"
        "workingOne" -> if (en) "1 task running" else "1 feina en marxa"
        "suggest1" -> if (en) "Plan my week" else "Organitza'm la setmana"
        "suggest2" -> if (en) "Remind me tomorrow at 9 to call the doctor" else "Recorda'm demà a les 9 que truqui al metge"
        "suggest3" -> if (en) "Find me a quiet restaurant for Friday" else "Busca'm un restaurant tranquil per divendres"
        "intro" -> if (en) "I'm %s. Ask me something, give me a task or let me keep an eye on something for you: I'll come back here when it's done."
            else "Sóc %s. Pregunta'm el que vulguis, encarrega'm una feina o demana'm que vigili alguna cosa: tornaré aquí quan estigui fet."
        else -> key
    }
}
