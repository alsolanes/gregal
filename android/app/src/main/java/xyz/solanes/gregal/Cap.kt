package xyz.solanes.gregal

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.builtins.ListSerializer

/**
 * La mascota, cap de gabinet de la colla (/api/winds/cap). Viu al servidor
 * (o al del mòbil): el nom, la cara i el fil curt amb l'usuari.
 */
data class CapMsg(
    val from: String, val text: String, val action: String = "", val winds: List<String> = emptyList(),
    /** L'encàrrec d'una colla proposada. */
    val task: String = "",
    /** La proposta sencera: especialistes nous i vents que encara no en saben. */
    val create: List<CapNew> = emptyList(), val novices: List<String> = emptyList(),
    /** On ha anat a parar una delegació ("wind:<id>" o "rot:<id>"); action = "link". */
    val link: String = "",
    /** Segons Unix (0 = sense hora). */
    val ts: Long = 0L,
    /** Id estable del missatge al servidor (buit en servidors antics). */
    val id: String = "",
) {
    /** És una proposta de delegació (un vent, una rotllana o una colla). */
    val delegates: Boolean get() = from == "cap" && (action == "wind" || action == "rotllana" || action == "crew")
}

/**
 * La delegació que encara espera «endavant»: l'última proposta del fil si no
 * hi ha res després (ni la resposta de l'usuari ni l'enllaç a la conversa) i
 * l'usuari no l'ha deixada per a més tard. Torna la petició original i la
 * decisió. Així una proposta es pot acceptar també després de tornar a obrir l'app.
 */
internal fun capPendingOf(history: List<CapMsg>, dismissedTs: Long = 0L): Pair<String, CapDecision>? {
    val last = history.lastOrNull() ?: return null
    if (!last.delegates) return null
    if (last.ts != 0L && last.ts <= dismissedTs) return null
    val asked = history.getOrNull(history.size - 2)?.takeIf { it.from == "user" }?.text ?: return null
    return asked to CapDecision(last.action, last.text, last.winds, last.task, create = last.create, novices = last.novices)
}

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
        /** GET /api/winds/cap (tots els camps opcionals). */
        fun parse(raw: String): CapState {
            val o = GregalJson.decodeFromString(CapStateDto.serializer(), raw)
            val b = o.brief
            return CapState(
                name = o.name.ifBlank { "Ventet" },
                color = o.color.ifBlank { "#FFC857" },
                style = o.style.ifBlank { "pto" },
                history = o.history.map { it.toMsg() },
                brief = b?.text.orEmpty(),
                briefCount = if (b == null) 0 else b.done + b.failed + b.news,
                pet = o.pet ?: CapPet(),
            )
        }

        /** El fil de la mascota (una llista JSON de missatges). */
        fun parseHistory(raw: String): List<CapMsg> =
            GregalJson.decodeFromString(ListSerializer(CapMsgDto.serializer()), raw).map { it.toMsg() }
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
        /** La resposta de POST /api/winds/cap (o el done del flux). */
        fun parse(raw: String): CapDecision {
            val o = GregalJson.decodeFromString(CapDecisionDto.serializer(), raw)
            return CapDecision(
                action = o.action.ifBlank { "self" },
                reply = o.reply,
                winds = o.winds.filter { it.isNotBlank() },
                task = o.task,
                rotllana = o.rotllana,
                history = o.history.map { it.toMsg() },
                create = o.create,
                novices = o.novices.filter { it.isNotBlank() },
                pet = o.pet,
                confirm = o.confirm,
                message = o.message,
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

/** Un especialista nou que la mascota proposa (es crea en dir «endavant»). Viatja tal qual en el JSON. */
@Serializable
data class CapNew(val name: String = "", val role: String = "", val persona: String = "", val kit: String = "",
                  val color: String = "", val style: String = "") {
    val argb: Long get() {
        val hex = color.removePrefix("#")
        return 0xFF000000L or (if (hex.matches(Regex("[0-9a-fA-F]{6}"))) hex else "C9B8F5").toLong(16)
    }
}

/** L'estat de criatura de companyia: alegria, energia, gana, nivell i roba. */
@Serializable
data class CapPet(
    val happy: Int = 70, val energy: Int = 80, val full: Int = 60,
    val level: Int = 1, @SerialName("level_xp") val levelXp: Int = 0, @SerialName("level_need") val levelNeed: Int = 100,
    val outfit: String = "", val outfits: List<String> = emptyList(), val played: Int = 0,
) {
    val hungry: Boolean get() = full < 30
    val tired: Boolean get() = energy < 20
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

// --- Lectura (kotlinx.serialization): tots els camps opcionals ---

@Serializable
internal class CapMsgDto(
    val from: String = "", val text: String = "", val action: String = "",
    val winds: List<String> = emptyList(), val task: String = "",
    val create: List<CapNew> = emptyList(), val novices: List<String> = emptyList(),
    val link: String = "", val ts: Long = 0L, val id: String = "",
) {
    fun toMsg() = CapMsg(from, text, action, winds, task = task, create = create,
        novices = novices.filter { it.isNotBlank() }, link = link, ts = ts, id = id)
}

@Serializable
internal class CapBriefDto(val done: Int = 0, val failed: Int = 0, val news: Int = 0, val text: String = "")

@Serializable
internal class CapStateDto(
    val name: String = "", val color: String = "", val style: String = "",
    val history: List<CapMsgDto> = emptyList(), val brief: CapBriefDto? = null, val pet: CapPet? = null,
)

@Serializable
internal class CapDecisionDto(
    val action: String = "", val reply: String = "", val winds: List<String> = emptyList(),
    val task: String = "", val rotllana: String = "", val history: List<CapMsgDto> = emptyList(),
    val create: List<CapNew> = emptyList(), val novices: List<String> = emptyList(),
    val pet: CapPet? = null, val confirm: Boolean = false, val message: String = "",
)
