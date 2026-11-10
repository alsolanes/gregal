package xyz.solanes.gregal

import org.json.JSONArray
import org.json.JSONObject

/** Authenticated profile. Topic knowledge and personal memory are distinct stores. */
data class WindProfile(val wind: Wind, val knowledge: String, val personalMemory: String,
    /** L'ofici del vent (null si el seu ofici no té kit al servidor). */
    val trade: WindTrade? = null) {
    companion object {
        fun parse(raw: String): WindProfile {
            val obj = JSONObject(raw)
            val wind = obj.getJSONObject("wind")
            val parsed = Wind.parse(JSONObject().put("winds", JSONArray().put(wind)).toString()).first()
            return WindProfile(parsed, obj.optString("knowledge"), obj.optString("personal_memory"),
                WindTrade.parse(obj.optJSONObject("trade")))
        }
    }
}

/** Una vigilància que l'ofici ofereix: es converteix en una rutina del vent en activar-la. */
data class WindTradeWatch(
    val labelCa: String, val labelEn: String, val promptCa: String, val promptEn: String,
    /** «HH:MM» de la rutina diària (o setmanal). */
    val time: String, val weekly: Boolean, val intervalH: Double,
) {
    fun label(lang: String): String = if (lang == "en") labelEn.ifBlank { labelCa } else labelCa.ifBlank { labelEn }
    fun prompt(lang: String): String = if (lang == "en") promptEn.ifBlank { promptCa } else promptCa.ifBlank { promptEn }

    /** «cada 6 h», «un cop per setmana», «cada dia a les 08:00». */
    fun schedule(lang: String): String = when {
        intervalH > 0 -> tr(lang, "app.trade.everyHours", WindTrade.hours(intervalH))
        weekly -> tr(lang, "app.trade.weekly")
        else -> tr(lang, "app.trade.daily", time)
    }

    /** El cos de POST /api/winds/rutina, com el de l'escriptori. */
    fun rutina(windId: String, lang: String): JSONObject {
        val o = JSONObject().put("wind", windId).put("prompt", prompt(lang)).put("name", label(lang))
        return if (intervalH > 0) o.put("kind", "interval").put("interval_h", intervalH)
        else o.put("kind", "daily").put("time", time)
    }
}

/** L'ofici del vent al perfil: nom, nivell (1–10), què sap, d'on treu la informació i què pot vigilar. */
data class WindTrade(
    val kit: String, val nameCa: String, val nameEn: String,
    val sources: List<String>, val sites: List<String>,
    val level: Int, val knowledge: Int, val lessons: Int,
    val watches: List<WindTradeWatch>,
) {
    fun name(lang: String): String = if (lang == "en") nameEn.ifBlank { nameCa } else nameCa.ifBlank { nameEn }

    companion object {
        /** null si no hi ha ofici (el servidor envia null o no l'envia). */
        fun parse(o: JSONObject?): WindTrade? {
            if (o == null) return null
            fun strings(key: String): List<String> {
                val a = o.optJSONArray(key) ?: return emptyList()
                return (0 until a.length()).map { a.optString(it) }.filter { it.isNotBlank() }
            }
            val ws = o.optJSONArray("watches") ?: JSONArray()
            val watches = (0 until ws.length()).mapNotNull { i ->
                val w = ws.optJSONObject(i) ?: return@mapNotNull null
                WindTradeWatch(
                    w.optString("label_ca"), w.optString("label_en"), w.optString("prompt_ca"), w.optString("prompt_en"),
                    w.optString("time"), w.optBoolean("weekly", false), w.optDouble("interval_h", 0.0).takeIf { !it.isNaN() } ?: 0.0,
                ).takeIf { (it.labelCa.isNotBlank() || it.labelEn.isNotBlank()) && (it.promptCa.isNotBlank() || it.promptEn.isNotBlank()) }
            }
            return WindTrade(
                kit = o.optString("kit"), nameCa = o.optString("name_ca"), nameEn = o.optString("name_en"),
                sources = strings("sources"), sites = strings("sites"),
                level = o.optInt("level", 1).coerceIn(1, 10),
                knowledge = o.optInt("knowledge", 0).coerceAtLeast(0), lessons = o.optInt("lessons", 0).coerceAtLeast(0),
                watches = watches,
            )
        }

        /** 6.0 → «6», 1.5 → «1.5». */
        fun hours(h: Double): String = if (h == Math.floor(h)) h.toLong().toString() else h.toString()
    }
}
