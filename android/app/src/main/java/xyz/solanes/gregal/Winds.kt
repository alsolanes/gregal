package xyz.solanes.gregal

import org.json.JSONArray
import org.json.JSONObject

/**
 * Vents de Gregal: companions personals amb nom de vent, color, cara i veu.
 *
 * El catàleg ve del backend (GET /api/winds); aquí hi ha el parseig
 * (testejat per WindModeTest) i els noms de vents suggerits per al creador.
 */

/** Un vent del catàleg: id, nom, color (#RRGGBB), rol de la UI i cara. */
data class Wind(
    val id: String,
    val name: String,
    val color: String,
    val role: String,
    val style: String = "pso",
    val persona: String = "",
) {
    /** Color ARGB per a Compose. */
    val argb: Long get() = run {
        val hex = color.removePrefix("#")
        val v = if (hex.matches(Regex("[0-9a-fA-F]{6}"))) hex else "FF9F00"
        (0xFF000000L or v.toLong(16))
    }

    /** L'estil de cara per a WindFace. */
    val faceStyle: WindStyle get() = windStyleOf(style)

    companion object {
        fun parse(raw: String): List<Wind> {
            val o = JSONObject(raw)
            val a = o.optJSONArray("winds") ?: JSONArray()
            return (0 until a.length()).mapNotNull { i ->
                val w = a.optJSONObject(i) ?: return@mapNotNull null
                Wind(w.optString("id"), w.optString("name"), w.optString("color"), w.optString("role"), w.optString("style").ifBlank { "pso" }, w.optString("persona"))
            }
        }
    }
}

/** Cossos de les peticions POST /api/winds i /api/winds/chat. */
object WindRequests {
    fun save(name: String, color: String, role: String, style: String = "pso", persona: String = "", id: String = ""): String =
        JSONObject().put("name", name).put("color", color).put("role", role).put("style", style)
            .put("persona", persona).apply { if (id.isNotEmpty()) put("id", id) }.toString()

    fun chat(windId: String, message: String): String =
        JSONObject().put("wind", windId).put("message", message).toString()
}

/** Plantilles de feina (estil Redde): el tema i la manera de ser ja vénen
 *  fets; l'usuari hi posa el nom i la carona. Textos a AppStrings
 *  (app.vents.tpl.<id>.*), paritat ca/en. */
object WindTemplates {
    data class Template(val id: String, val color: String, val style: String)

    val all = listOf(
        Template("recerca", "#A8CDE2", "pso"),
        Template("escriptura", "#E8AEC2", "pto"),
        Template("codi", "#BFCFA4", "esn"),
        Template("casa", "#EFC69A", "bsn"),
        Template("plans", "#EACF87", "msn"),
    )
}

/** Noms de vents suggerits al creador, amb color i cara proposades (paleta PICO-8). */
object WindSuggestions {
    data class Suggestion(val name: String, val color: String, val role: String, val style: String)

    val all = listOf(
        Suggestion("Marinada", "#EFC69A", "calma i benestar", "bsn"),
        Suggestion("Xaloc", "#EFA68F", "música i descobertes", "eto"),
        Suggestion("Tramuntana", "#A8CDE2", "idees i curiositat", "pso"),
        Suggestion("Mestral", "#BFCFA4", "aprendre plegats", "esn"),
        Suggestion("Llebeig", "#E8AEC2", "històries i cultura", "pto"),
        Suggestion("Gregal", "#C1B4DE", "el dia a dia", "mso"),
        Suggestion("Ponent", "#EACF87", "llibres i tardes", "msn"),
        Suggestion("Migjorn", "#9ED3C5", "natura i escapades", "bss"),
    )
}

/** Una rotllana: un xat de grup amb els vents que l'usuari triï. */
data class Rotllana(
    val id: String,
    val name: String,
    val members: List<String>,
)

/** Cossos de les peticions de rotllanes. */
object RotllanaRequests {
    fun save(name: String, members: List<String>): String =
        JSONObject().put("name", name).put("members", JSONArray(members)).toString()

    fun chat(rotllanaId: String, message: String, wind: String = ""): String =
        JSONObject().put("rotllana", rotllanaId).put("message", message).put("wind", wind).toString()

    fun parse(raw: String): List<Rotllana> {
        val o = JSONObject(raw)
        val a = o.optJSONArray("rotllanes") ?: JSONArray()
        return (0 until a.length()).mapNotNull { i ->
            val r = a.optJSONObject(i) ?: return@mapNotNull null
            val m = r.optJSONArray("members") ?: JSONArray()
            Rotllana(r.optString("id"), r.optString("name"), (0 until m.length()).map { m.optString(it) })
        }
    }
}
