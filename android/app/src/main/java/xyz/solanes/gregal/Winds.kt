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
    /** Especialitat dins una colla ("shopping", "frontend"…); objecte d'ofici per a la cara. */
    val specialty: String = "",
    /** L'objecte d'ofici que duu ("research", "tech"…): manen el specialty i la inferència del rol del servidor. */
    val kit: String = "",
    /** Colla a què pertany ("daily", "data", "software"); buit si és un vent propi. */
    val squad: String = "",
) {
    /** Color ARGB per a Compose. */
    val argb: Long get() = run {
        val hex = color.removePrefix("#")
        val v = if (hex.matches(Regex("[0-9a-fA-F]{6}"))) hex else "FF9F00"
        (0xFF000000L or v.toLong(16))
    }

    /** L'estil de cara per a WindFace; l'objecte d'ofici (kit) manen el specialty i la inferència del rol. */
    val faceStyle: WindStyle get() = windStyleOf(style).copy(kit = if (kit.isNotBlank()) kit else if (specialty.isNotBlank()) specialty else inferKit(role))

    companion object {
        fun parse(raw: String): List<Wind> {
            val o = JSONObject(raw)
            val a = o.optJSONArray("winds") ?: JSONArray()
            return (0 until a.length()).mapNotNull { i ->
                val w = a.optJSONObject(i) ?: return@mapNotNull null
                Wind(w.optString("id"), w.optString("name"), w.optString("color"), w.optString("role"), w.optString("style").ifBlank { "pso" }, w.optString("persona"), w.optString("specialty"), w.optString("kit"), w.optString("squad"))
            }
        }
    }
}

/**
 * Torn en directe d'un xat o d'una rotllana tal com l'observa l'app: l'última
 * fase del servidor, si ja arriba text i quantes veus han parlat. Només viu
 * mentre el torn és en marxa; la cara la tradueix amb windMoodOf.
 */
data class WindLive(val phase: String = "thinking", val writing: Boolean = false, val voices: Int = 1) {
    val mood: WindMood get() = windMoodOf(phase = phase, writing = writing, voices = voices)
}

/**
 * Veus proactives (cor, recordatoris) que encara no has obert. Un vent hi és
 * fins que n'obres el xat. La primera vegada només compta l'últim dia, perquè
 * un historial vell no desperti tota la colla.
 */
object WindNewsLogic {
    const val FIRST_RUN_WINDOW = 86_400L
    fun unseen(novetats: List<WindNovetat>, seen: Map<String, Long>, floor: Long): Set<String> =
        novetats.filter { it.wind.isNotBlank() && it.ts > maxOf(seen[it.wind] ?: 0L, floor) }.map { it.wind }.toSet()
}

/** Cossos de les peticions POST /api/winds i /api/winds/chat. */
object WindRequests {
    fun save(name: String, color: String, role: String, style: String = "pso", persona: String = "", id: String = ""): String =
        JSONObject().put("name", name).put("color", color).put("role", role).put("style", style)
            .put("persona", persona).apply { if (id.isNotEmpty()) put("id", id) }.toString()

    fun chat(windId: String, message: String): String =
        JSONObject().put("wind", windId).put("message", message).toString()

    /** Reprèn l'última resposta parcial del vent (sense text nou). */
    fun chatContinue(windId: String): String =
        JSONObject().put("wind", windId).put("message", "").put("continue", true).toString()
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

/**
 * L'objecte d'ofici que duu un vent propi, endevinat del seu rol — les
 * mateixes paraules clau que el servidor (winds_kit.go), en català i anglès.
 * Conservador: si res no encaixa, cap objecte. Així el preview del creador i
 * la llista mostren la pilota o la proveta abans que el servidor digui res.
 */
object WindKit {
    private val keywords = linkedMapOf(
        "security" to listOf("seguretat", "security", "hack", "vulnerab", "privacitat", "privacy"),
        "music" to listOf("música", "musica", "music", "canç", "song", "disc", "concert", "banda", "guitarra"),
        "sports" to listOf("esport", "sport", "futbol", "fútbol", "football", "basket", "bàsquet", "pilota", "lliga", "barça", "barca", "goal", "partit"),
        "books" to listOf("llibre", "libro", "book", "novel", "lectura", "reading", "literatura", "poesia"),
        "shopping" to listOf("compress", "compra", "shopping", "botiga", "preu", "price"),
        "organisation" to listOf("organitza", "organiza", "organi", "agenda", "calendar", "planific"),
        "research" to listOf("recerca", "research", "ciència", "ciencia", "science", "experiment", "lab", "provet", "descobr", "discover", "investig"),
        "tech" to listOf("tecnolog", "technolog", "tech", "novetat", "gadget", "digital", "internet", "informàt", "informàtica", "software", "xip", "chip"),
        "frontend" to listOf("codi", "code", "programa", "program", "desenvolup", "develop", "app", "web", "frontend"),
        "security" to listOf("seguretat", "security", "hack", "vulnerab", "privacitat", "privacy"),
        "ml" to listOf("machine learning", "aprenentatge", "learning", "intel·ligència artificial", "inteligència artificial", "intel·ligencia", "model de llenguatge", "llm"),
        "testing" to listOf("testeig", "testing", "qualitat de codi", "qa "),
        "delivery" to listOf("entrega", "delivery", "enviaments", "shipping"),
    )

    /** El kit de l'ofici del rol, o "" si cap encaixa. */
    fun of(role: String): String {
        val r = " " + role.trim().lowercase() + " "
        for ((kit, words) in keywords) for (w in words) if (w in r) return kit
        return ""
    }
}

/** L'objecte d'ofici d'un vent propi segons el seu rol (vegeu WindKit). */
internal fun inferKit(role: String): String = WindKit.of(role)

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

    /** Reprèn l'última resposta parcial de la rotllana (qui parla: wind). */
    fun chatContinue(rotllanaId: String, wind: String = ""): String =
        JSONObject().put("rotllana", rotllanaId).put("message", "").put("wind", wind).put("continue", true).toString()

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

/** Un vent que ja existeix i encaixa amb l'encàrrec (porta la seva memòria). */
data class CrewReuse(val id: String, val name: String, val role: String, val why: String)

/** Un especialista nou per a l'encàrrec, amb color, cara i ofici propis. */
data class CrewNew(val name: String, val role: String, val kit: String, val persona: String, val why: String, val color: String, val style: String) {
    val wind: Wind get() = Wind("", name, color, role, style, specialty = kit)
    fun json(): JSONObject = JSONObject().put("name", name).put("role", role).put("kit", kit)
        .put("persona", persona).put("why", why).put("color", color).put("style", style)
}

/** Proposta de colla per a un encàrrec (POST /api/winds/crew/propose). */
data class CrewProposal(val plan: String, val reuse: List<CrewReuse>, val create: List<CrewNew>) {
    companion object {
        fun parse(o: JSONObject): CrewProposal {
            val r = o.optJSONArray("reuse") ?: JSONArray()
            val c = o.optJSONArray("create") ?: JSONArray()
            return CrewProposal(
                o.optString("plan"),
                (0 until r.length()).mapNotNull { i -> r.optJSONObject(i)?.let { CrewReuse(it.optString("id"), it.optString("name"), it.optString("role"), it.optString("why")) } },
                (0 until c.length()).mapNotNull { i -> c.optJSONObject(i)?.let {
                    CrewNew(it.optString("name"), it.optString("role"), it.optString("kit"), it.optString("persona"), it.optString("why"), it.optString("color"), it.optString("style"))
                } },
            )
        }
    }
}
