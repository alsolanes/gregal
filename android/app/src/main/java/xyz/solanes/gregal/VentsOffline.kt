package xyz.solanes.gregal

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.net.ConnectException
import java.net.NoRouteToHostException
import java.net.SocketTimeoutException
import java.net.UnknownHostException

/*
 * El servidor mana; el mòbil és la reserva (fase 1).
 *
 * Amb un servidor desat, els vents i les converses viuen allà. Aquí en
 * guardem una còpia per servidor (origen) perquè, si el servidor no respon,
 * l'app s'obri igualment amb el que hi havia:
 *  - la llista de vents i els últims missatges de cada xat;
 *  - una cua d'enviament: el que escrius sense connexió s'envia sol, en
 *    ordre, quan el servidor torna;
 *  - els propers recordatoris del servidor, perquè si a l'hora el servidor
 *    no respon l'avís surti igualment del mòbil (i no dues vegades quan torni).
 *
 * Res d'això respon sense el servidor: no hi ha cap model al mòbil en
 * aquesta fase. Tot és per origen: un altre servidor no veu res d'aquest.
 */

/** Un missatge escrit sense connexió, pendent d'enviar. */
data class OutboxItem(val wind: String, val text: String, val ts: Long)

/** Un recordatori del servidor que el mòbil també vigila. */
data class MirroredReminder(val id: String, val wind: String, val name: String, val at: Long, val text: String)

/** El servidor no s'ha pogut ni tocar: res no ha arribat a l'altra banda. */
fun isUnreachable(e: Throwable?): Boolean {
    var c = e
    var depth = 0
    while (c != null && depth++ < 6) {
        when (c) {
            is ConnectException, is NoRouteToHostException, is UnknownHostException -> return true
            // Només el temps d'espera de connexió: un torn llarg que ja corria no és «sense connexió».
            is SocketTimeoutException -> if ((c.message ?: "").contains("connect", true)) return true
        }
        val m = c.message.orEmpty()
        // L'emulador (i alguns NAT) accepten la connexió i la tanquen de seguida
        // quan al darrere no hi ha ningú: cap resposta, ni una capçalera.
        if (c is java.io.EOFException || m.contains("unexpected end of stream", true) ||
            m.contains("Connection reset", true) || m.contains("Connection closed by peer", true)) return true
        if (m.contains("failed to connect", true) || m.contains("Unable to resolve host", true) ||
            m.contains("ECONNREFUSED") || m.contains("ENETUNREACH") || m.contains("EHOSTUNREACH")) return true
        // Un proxy al davant que diu que el servidor no hi és.
        if (m.startsWith("HTTP 502") || m.startsWith("HTTP 503") || m.startsWith("HTTP 504")) return true
        c = c.cause
    }
    return false
}

/** Lògica pura (sense Android) per als tests de la JVM. */
object OfflineLogic {
    const val CHAT_KEEP = 60
    /** Un recordatori que no s'ha pogut confirmar en 12 h ja no s'avisa. */
    const val STALE_MS = 12 * 3600_000L
    /** Els ids avisats es recorden 3 dies (prou per deduplicar el que el servidor dispari tard). */
    const val SHOWN_KEEP_MS = 3 * 24 * 3600_000L

    fun encodeChat(msgs: List<GregalViewModel.PetMsg>): String {
        val a = JSONArray()
        for (m in msgs.filter { !it.streaming && it.kind != "queued" && it.text.isNotBlank() }.takeLast(CHAT_KEEP)) {
            a.put(JSONObject().put("from", m.from).put("text", m.text).put("speaker_id", m.speakerId)
                .put("speaker_name", m.speakerName).put("ts", m.ts).put("kind", m.kind).put("parcial", m.parcial))
        }
        return a.toString()
    }

    fun decodeChat(raw: String?): List<GregalViewModel.PetMsg>? {
        if (raw.isNullOrBlank()) return null
        return try {
            val a = JSONArray(raw)
            (0 until a.length()).mapNotNull { i ->
                val o = a.optJSONObject(i) ?: return@mapNotNull null
                GregalViewModel.PetMsg(o.optString("from"), o.optString("text"), o.optString("speaker_id"),
                    o.optString("speaker_name"), parcial = o.optBoolean("parcial"), ts = o.optLong("ts"), kind = o.optString("kind"))
            }
        } catch (_: Exception) { null }
    }

    fun encodeOutbox(items: List<OutboxItem>): String = JSONArray().apply {
        for (i in items) put(JSONObject().put("wind", i.wind).put("text", i.text).put("ts", i.ts))
    }.toString()

    fun decodeOutbox(raw: String?): List<OutboxItem> = try {
        val a = JSONArray(raw ?: "[]")
        (0 until a.length()).mapNotNull { i ->
            val o = a.optJSONObject(i) ?: return@mapNotNull null
            val text = o.optString("text")
            if (text.isBlank() || o.optString("wind").isBlank()) null else OutboxItem(o.optString("wind"), text, o.optLong("ts"))
        }.sortedBy { it.ts }
    } catch (_: Exception) { emptyList() }

    /** Afegeix a la cua conservant l'ordre d'escriptura (un reintent torna al seu lloc). */
    fun enqueue(items: List<OutboxItem>, item: OutboxItem): List<OutboxItem> =
        (items.filterNot { it.wind == item.wind && it.ts == item.ts } + item).sortedBy { it.ts }

    /** El següent a enviar de cada vent lliure: un per vent, el més antic. */
    fun nextToSend(items: List<OutboxItem>, busy: Set<String>): List<OutboxItem> =
        items.sortedBy { it.ts }.groupBy { it.wind }.filterKeys { it !in busy }.map { it.value.first() }

    /** L'historial que es mostra: el del servidor (o la còpia) i, al final, el que espera a sortir. */
    fun withQueued(history: List<GregalViewModel.PetMsg>, outbox: List<OutboxItem>, wind: String): List<GregalViewModel.PetMsg> =
        history.filter { it.kind != "queued" } +
            outbox.filter { it.wind == wind }.sortedBy { it.ts }.map { GregalViewModel.PetMsg("user", it.text, ts = it.ts, kind = "queued") }

    fun encodeReminders(items: List<MirroredReminder>): String = JSONArray().apply {
        for (r in items) put(JSONObject().put("id", r.id).put("wind", r.wind).put("name", r.name).put("at", r.at).put("text", r.text))
    }.toString()

    fun decodeReminders(raw: String?): List<MirroredReminder> = try {
        val a = JSONArray(raw ?: "[]")
        (0 until a.length()).mapNotNull { i ->
            val o = a.optJSONObject(i) ?: return@mapNotNull null
            if (o.optString("id").isBlank() || o.optLong("at") <= 0) null
            else MirroredReminder(o.optString("id"), o.optString("wind"), o.optString("name"), o.optLong("at"), o.optString("text"))
        }
    } catch (_: Exception) { emptyList() }

    /** Avisos programats de la llista d'automatitzacions (només els d'un sol tret amb hora). */
    fun remindersOf(items: List<WindAutomation>): List<MirroredReminder> = items
        .filter { it.enabled && it.kind in setOf("reminder", "followup", "task") && it.repeat == "once" }
        .mapNotNull { a ->
            JobSchedule.parseInstant(a.nextAt)?.let { MirroredReminder(a.id, a.wind, a.windName, it, a.title) }
        }

    /**
     * Fusiona una llista nova amb la desada. Amb `wind` buit la llista és
     * sencera i substitueix; amb un vent, només en substitueix els seus.
     */
    fun mergeReminders(cur: List<MirroredReminder>, fresh: List<MirroredReminder>, wind: String?): List<MirroredReminder> {
        val kept = when (wind) {
            null -> cur.filterNot { c -> fresh.any { it.id == c.id } }
            "" -> emptyList()
            else -> cur.filter { it.wind != wind }
        }
        return (kept + fresh).distinctBy { it.id }.sortedBy { it.at }.takeLast(64)
    }

    /** Sense servidor: els que ja toquen, no gaire vells i encara no avisats. */
    fun dueOffline(items: List<MirroredReminder>, shown: Map<String, Long>, now: Long): List<MirroredReminder> =
        items.filter { it.at <= now && it.at > now - STALE_MS && it.id !in shown }.sortedBy { it.at }

    /** Amb servidor: el que ja ha passat ho ha fet el servidor; fora de la llista. */
    fun pruneOnline(items: List<MirroredReminder>, now: Long): List<MirroredReminder> = items.filter { it.at > now }

    /** Les veus del servidor d'un recordatori que el mòbil ja ha avisat no tornen a sonar. */
    fun notYetShown(novetats: List<WindNovetat>, shown: Map<String, Long>): List<WindNovetat> =
        novetats.filter { it.job.isBlank() || it.job !in shown }

    fun pruneShown(shown: Map<String, Long>, now: Long): Map<String, Long> =
        shown.filterValues { it > now - SHOWN_KEEP_MS }

    fun encodeShown(shown: Map<String, Long>): String = JSONObject().apply { shown.forEach { (k, v) -> put(k, v) } }.toString()

    fun decodeShown(raw: String?): Map<String, Long> = try {
        val o = JSONObject(raw ?: "{}")
        o.keys().asSequence().associateWith { o.optLong(it) }
    } catch (_: Exception) { emptyMap() }
}

/** La còpia d'un servidor (origen) al mòbil: fitxers petits al directori privat de l'app. */
class VentsOfflineStore(private val dir: File) {
    private fun file(name: String) = File(dir, name)

    private fun read(name: String): String? = synchronized(LOCK) {
        try { file(name).takeIf { it.isFile }?.readText() } catch (_: Exception) { null }
    }

    private fun write(name: String, text: String) = synchronized(LOCK) {
        try {
            dir.mkdirs()
            val tmp = file("$name.tmp")
            tmp.writeText(text)
            if (!tmp.renameTo(file(name))) { file(name).delete(); tmp.renameTo(file(name)) }
        } catch (_: Exception) {}
    }

    private fun safe(id: String) = id.replace(Regex("[^A-Za-z0-9_-]"), "_").take(80)

    fun saveWinds(raw: String) = write("winds.json", raw)
    fun windsRaw(): String? = read("winds.json")
    /** Quan es va desar la llista (ms), 0 si no n'hi ha. */
    fun savedAt(): Long = synchronized(LOCK) { file("winds.json").takeIf { it.isFile }?.lastModified() ?: 0L }

    fun saveChat(wind: String, msgs: List<GregalViewModel.PetMsg>) = write("chat-${safe(wind)}.json", OfflineLogic.encodeChat(msgs))
    fun chat(wind: String): List<GregalViewModel.PetMsg>? = OfflineLogic.decodeChat(read("chat-${safe(wind)}.json"))

    fun outbox(): List<OutboxItem> = OfflineLogic.decodeOutbox(read("outbox.json"))
    fun saveOutbox(items: List<OutboxItem>) = write("outbox.json", OfflineLogic.encodeOutbox(items))

    fun reminders(): List<MirroredReminder> = OfflineLogic.decodeReminders(read("reminders.json"))
    fun saveReminders(items: List<MirroredReminder>) = write("reminders.json", OfflineLogic.encodeReminders(items))

    fun shown(): Map<String, Long> = OfflineLogic.decodeShown(read("shown.json"))
    fun saveShown(m: Map<String, Long>) = write("shown.json", OfflineLogic.encodeShown(m))

    companion object { private val LOCK = Any() }
}

object VentsOffline {
    fun store(ctx: Context, originId: String): VentsOfflineStore =
        VentsOfflineStore(File(ctx.filesDir, "vents-offline/" + originId.replace(Regex("[^A-Za-z0-9_-]"), "_")))

    /** La còpia del servidor desat (la que vigila el WindWatch), o null si no n'hi ha. */
    fun savedServer(ctx: Context): VentsOfflineStore? =
        NotificationOrigins.remote(Prefs(ctx))?.let { store(ctx, it.id) }
}

/**
 * Per què ha fallat un torn, en paraules de persona. Abans tot error deia
 * «Torn interromput… revisa la feina desada», també quan faltava la clau del
 * proveïdor i no hi havia res a revisar. Buit = el missatge genèric.
 */
fun turnFailureText(lang: String, error: String): String {
    val e = error.lowercase()
    val en = lang == "en"
    return when {
        "http 401" in e || "api key" in e || "unauthorized" in e || "invalid_api_key" in e ->
            if (en) "The model provider rejected the request: the API key is missing or wrong. Tap the model at the top and pick the provider to fix the key."
            else "El proveïdor del model no ha acceptat la petició: falta la clau d'API o no és bona. Toca el model a dalt i tria el proveïdor per arreglar-la."
        "http 402" in e || "insufficient" in e || "quota" in e || "credit" in e ->
            if (en) "The model provider says there is no credit or quota left on this key."
            else "El proveïdor del model diu que aquesta clau no té saldo o quota."
        "http 429" in e || "rate limit" in e ->
            if (en) "The model provider is limiting requests right now. Try again in a minute."
            else "El proveïdor del model està limitant les peticions ara mateix. Torna-ho a provar d'aquí a un minut."
        else -> ""
    }
}
