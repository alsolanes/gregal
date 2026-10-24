package xyz.solanes.gregal

import android.content.SharedPreferences
import java.security.MessageDigest
import org.json.JSONArray
import org.json.JSONObject

/** Device-observed receipts only; legacy GETs never establish a date.
 * Only hashes and observed timestamps reach this private preferences file.
 */
internal class VentsRecency(private val prefs: SharedPreferences) {
    private fun hash(parts: List<String>): String {
        val digest = MessageDigest.getInstance("SHA-256")
        parts.forEach { part ->
            val bytes = part.toByteArray(Charsets.UTF_8)
            digest.update("${bytes.size}:".toByteArray(Charsets.UTF_8))
            digest.update(bytes)
        }
        return digest.digest().joinToString("") { "%02x".format(it) }
    }

    private fun key(conn: Conn, kind: String, id: String): String {
        // The embedded backend's port AND token are regenerated on restart.
        val scope = if (conn.local) listOf("local", conn.user)
            else listOf("remote", conn.host, conn.port, conn.token, conn.user)
        return hash(scope + listOf(kind, id))
    }

    private fun fingerprint(history: List<GregalViewModel.PetMsg>) = hash(
        history.flatMap { listOf(it.from, it.text, it.speakerId, it.speakerName) }
    )

    fun restore(conn: Conn, kind: String, id: String, history: List<GregalViewModel.PetMsg>): List<GregalViewModel.PetMsg> {
        if (history.isEmpty() || history.any { it.streaming }) return history
        return try {
            val receipt = JSONObject(prefs.getString(key(conn, kind, id), null) ?: return history)
            if (receipt.optString("hash") != fingerprint(history)) return history
            val times = receipt.getJSONArray("times")
            if (times.length() != history.size) return history
            history.mapIndexed { index, msg -> if (msg.ts > 0) msg else msg.copy(ts = times.optLong(index).coerceAtLeast(0)) }
        } catch (_: Exception) { history }
    }

    fun completed(conn: Conn, kind: String, id: String, history: List<GregalViewModel.PetMsg>, at: Long,
                  known: List<GregalViewModel.PetMsg>? = null): List<GregalViewModel.PetMsg> {
        if (at <= 0 || history.isEmpty() || history.any { it.streaming } ||
            history.last().from != "pet" || history.last().text.isBlank()) return history
        // A silent continuation did not create a new message; keep the old receipt.
        if (known != null && fingerprint(known) == fingerprint(history)) return restore(conn, kind, id, history)
        val stamped = history.mapIndexed { index, msg ->
            val prior = known?.getOrNull(index)
            val ts = when {
                index == history.lastIndex -> at
                msg.ts > 0 -> msg.ts
                prior != null && prior.from == msg.from && prior.text == msg.text && prior.speakerId == msg.speakerId -> prior.ts
                else -> 0L
            }
            msg.copy(ts = ts)
        }
        try {
            val receipt = JSONObject().put("hash", fingerprint(stamped))
                .put("times", JSONArray(stamped.map { it.ts }))
            prefs.edit().putString(key(conn, kind, id), receipt.toString()).apply()
        } catch (_: Exception) { /* Receipt failure must never retry an already successful turn. */ }
        return stamped
    }

    fun clear(conn: Conn, kind: String, id: String) {
        try { prefs.edit().remove(key(conn, kind, id)).apply() } catch (_: Exception) { }
    }
}
