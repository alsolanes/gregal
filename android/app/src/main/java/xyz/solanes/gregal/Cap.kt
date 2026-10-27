package xyz.solanes.gregal

import org.json.JSONArray
import org.json.JSONObject

/**
 * La mascota, cap de gabinet de la colla (/api/winds/cap). Viu al servidor
 * (o al del mòbil): el nom, la cara i el fil curt amb l'usuari.
 */
data class CapMsg(val from: String, val text: String, val action: String = "", val winds: List<String> = emptyList())

data class CapState(
    val name: String = "Ventet",
    val color: String = "#FFC857",
    val style: String = "pto",
    val history: List<CapMsg> = emptyList(),
    val brief: String = "",
    val briefCount: Int = 0,
) {
    val argb: Long get() {
        val hex = color.removePrefix("#")
        return 0xFF000000L or (if (hex.matches(Regex("[0-9a-fA-F]{6}"))) hex else "FFC857").toLong(16)
    }
    /** La cara amb la carpeta del cap de gabinet. */
    val faceStyle: WindStyle get() = windStyleOf(style).copy(kit = "organisation")

    companion object {
        fun parse(o: JSONObject): CapState {
            val b = o.optJSONObject("brief")
            val count = if (b == null) 0 else b.optInt("done") + b.optInt("failed") + b.optInt("news")
            return CapState(
                name = o.optString("name").ifBlank { "Ventet" },
                color = o.optString("color").ifBlank { "#FFC857" },
                style = o.optString("style").ifBlank { "pto" },
                history = parseHistory(o.optJSONArray("history")),
                brief = b?.optString("text").orEmpty(),
                briefCount = count,
            )
        }

        fun parseHistory(a: JSONArray?): List<CapMsg> {
            if (a == null) return emptyList()
            return (0 until a.length()).mapNotNull { i ->
                val m = a.optJSONObject(i) ?: return@mapNotNull null
                val w = m.optJSONArray("winds")
                CapMsg(m.optString("from"), m.optString("text"), m.optString("action"),
                    if (w == null) emptyList() else (0 until w.length()).map { w.optString(it) })
            }
        }
    }
}

/** El que la mascota ha decidit: self, wind, rotllana o crew. */
data class CapDecision(
    val action: String,
    val reply: String,
    val winds: List<String> = emptyList(),
    val task: String = "",
    val rotllana: String = "",
    val history: List<CapMsg> = emptyList(),
) {
    /** Hi ha res a fer? «self» ja és la resposta. */
    val delegates: Boolean get() = action == "wind" || action == "rotllana" || action == "crew"

    companion object {
        fun parse(o: JSONObject): CapDecision {
            val w = o.optJSONArray("winds")
            return CapDecision(
                action = o.optString("action").ifBlank { "self" },
                reply = o.optString("reply"),
                winds = if (w == null) emptyList() else (0 until w.length()).map { w.optString(it) }.filter { it.isNotBlank() },
                task = o.optString("task"),
                rotllana = o.optString("rotllana"),
                history = CapState.parseHistory(o.optJSONArray("history")),
            )
        }
    }
}

/** Nom d'una rotllana nova feta per la mascota: «Marinada i Garbí». */
internal fun capRotllanaName(names: List<String>, lang: String): String = when (names.size) {
    0 -> ""
    1 -> names[0]
    else -> names.dropLast(1).joinToString(", ") + (if (lang == "ca") " i " else " & ") + names.last()
}
