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
    val pet: CapPet = CapPet(),
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
                pet = o.optJSONObject("pet")?.let { CapPet.parse(it) } ?: CapPet(),
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
    /** Especialistes nous que proposa crear. */
    val create: List<CapNew> = emptyList(),
    /** Vents triats que encara no saben res: es proposa un curs accelerat. */
    val novices: List<String> = emptyList(),
    val pet: CapPet? = null,
    /** L'usuari ha dit «endavant» a la proposta d'abans: es fa sense tornar a preguntar. */
    val confirm: Boolean = false,
    /** La petició original de la proposta confirmada (la que va al vent o a la colla). */
    val message: String = "",
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
                create = o.optJSONArray("create")?.let { a -> (0 until a.length()).mapNotNull { a.optJSONObject(it)?.let(CapNew::parse) } }.orEmpty(),
                novices = o.optJSONArray("novices")?.let { a -> (0 until a.length()).map { a.optString(it) }.filter { it.isNotBlank() } }.orEmpty(),
                pet = o.optJSONObject("pet")?.let { CapPet.parse(it) },
                confirm = o.optBoolean("confirm", false),
                message = o.optString("message"),
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

/** Un especialista nou que la mascota proposa (es crea en dir «endavant»). */
data class CapNew(val name: String, val role: String, val persona: String = "", val kit: String = "",
                  val color: String = "", val style: String = "") {
    val argb: Long get() {
        val hex = color.removePrefix("#")
        return 0xFF000000L or (if (hex.matches(Regex("[0-9a-fA-F]{6}"))) hex else "C9B8F5").toLong(16)
    }
    fun json(): JSONObject = JSONObject().put("name", name).put("role", role).put("persona", persona)
        .put("kit", kit).put("color", color).put("style", style)
    companion object {
        fun parse(o: JSONObject) = CapNew(o.optString("name"), o.optString("role"), o.optString("persona"),
            o.optString("kit"), o.optString("color"), o.optString("style"))
    }
}

/** L'estat de criatura de companyia: alegria, energia, gana, nivell i roba. */
data class CapPet(
    val happy: Int = 70, val energy: Int = 80, val full: Int = 60,
    val level: Int = 1, val levelXp: Int = 0, val levelNeed: Int = 100,
    val outfit: String = "", val outfits: List<String> = emptyList(), val played: Int = 0,
) {
    val hungry: Boolean get() = full < 30
    val tired: Boolean get() = energy < 20
    companion object {
        fun parse(o: JSONObject): CapPet {
            val a = o.optJSONArray("outfits")
            return CapPet(o.optInt("happy", 70), o.optInt("energy", 80), o.optInt("full", 60),
                o.optInt("level", 1), o.optInt("level_xp"), o.optInt("level_need", 100),
                o.optString("outfit"), if (a == null) emptyList() else (0 until a.length()).map { a.optString(it) },
                o.optInt("played"))
        }
    }
}

/** La roba de la mascota: emoji, on va i com es guanya (ha de coincidir amb capOutfits del servidor). */
data class CapOutfit(val id: String, val emoji: String, val spot: String, val level: Int = 0, val month: Int = 0)

val capOutfitCatalog = listOf(
    CapOutfit("llacet", "🎀", "top", level = 2), CapOutfit("gorra", "🧢", "top", level = 3),
    CapOutfit("bufanda", "🧣", "neck", level = 4), CapOutfit("ulleres", "🕶️", "eyes", level = 5),
    CapOutfit("barret", "🎩", "top", level = 6), CapOutfit("auriculars", "🎧", "top", level = 7),
    CapOutfit("corona", "👑", "top", level = 8),
    CapOutfit("flors", "🌸", "top", month = 4), CapOutfit("carbassa", "🎃", "top", month = 10),
    CapOutfit("gorro", "🎅", "top", month = 12),
)

/** Resultat de «endavant»: on s'ha obert la conversa i qui estudia. */
data class CapGoResult(val wind: String, val rotllana: String, val studying: List<String>, val created: List<String>)
