package xyz.solanes.gregal

import org.json.JSONArray
import org.json.JSONObject

/**
 * Mascotes del company: agents especialitzats amb veu pròpia.
 *
 * El catàleg ve del backend (GET /api/companion/pets); aquí només hi ha el
 * parseig (testejat per PetModeTest) i els avatars pixel-art 16x16 en
 * matrius — es dibuixen amb Canvas, sense recursos d'imatge.
 */

/** Una mascota del catàleg: id, nom i rol de la UI. */
data class Pet(
    val id: String,
    val name: String,
    val role: String,
) {
    companion object {
        fun parse(raw: String): List<Pet> {
            val o = JSONObject(raw)
            val a = o.optJSONArray("pets") ?: JSONArray()
            return (0 until a.length()).map { i ->
                val p = a.optJSONObject(i) ?: JSONObject()
                Pet(p.optString("id"), p.optString("name"), p.optString("role"))
            }
        }
    }
}

/** Un check-in d'una mascota: qui ha parlat i què ha dit. */
data class PetCheckin(
    val pet: String,
    val text: String,
    val at: String,
) {
    companion object {
        fun parse(raw: String): PetCheckin {
            val o = JSONObject(raw)
            return PetCheckin(o.optString("pet"), o.optString("checkin"), o.optString("at"))
        }
    }
}

/** Cos del POST /api/companion/checkin amb mascota. */
object PetRequests {
    fun checkin(petId: String): String =
        JSONObject().put("pet", petId).toString()

    fun chat(petId: String, message: String): String =
        JSONObject().put("pet", petId).put("message", message).toString()
}

/**
 * Avatars pixel-art 16x16, paleta PICO-8 (mates matrius que el generador).
 * Cada matriu és 16 files de 16 caràcters; el caràcter índex la paleta.
 */
object PetAvatars {
    // Paleta PICO-8: índex -> ARGB
    private val PAL = intArrayOf(
        0xFF000000.toInt(), // K negre
        0xFF1D2B53.toInt(), // D blau fosc
        0xFF7E2553.toInt(), // P porpra fosc
        0xFFBE1256.toInt(), // R vermell
        0xFFFF4444.toInt(), // G vermell clar
        0xFFFF9F00.toInt(), // O taronja
        0xFFFFEC39.toInt(), // Y groc
        0xFFA3E24E.toInt(), // C verd clar
        0xFF5FAD56.toInt(), // V verd fosc
        0xFF29ADFF.toInt(), // B blau clar
        0xFF83769C.toInt(), // L blau-gris
        0xFFF0F0EE.toInt(), // S blanc trencat
        0xFFFFFFFF.toInt(), // W blanc
        0xFF9BADB7.toInt(), // N gris clar
        0xFF5E6D9C.toInt(), // M blau mig
        0xFF262040.toInt(), // F fosc
    )

    private val CH = mapOf(
        'K' to 0, 'D' to 1, 'P' to 2, 'R' to 3, 'G' to 4, 'O' to 5,
        'Y' to 6, 'C' to 7, 'V' to 8, 'B' to 9, 'L' to 10, 'S' to 11,
        'W' to 12, 'N' to 13, 'M' to 14, 'F' to 15,
    )

    // Mel: cap amb auriculars, disc groc
    val MEL = listOf(
        "FFFFFFFFFFFFFFFF",
        "FFFFFFFFFFFFFFFF",
        "FFFOOOOOOOOOOFFF",
        "FFOOOOOOOOOOOOFF",
        "FFOOWWWOOWWWOOFF",
        "FYYOKWWOOWWKOYYF",
        "FYYOWWWOOWWWOYYF",
        "FYYOOOOOOOOOOYYF",
        "FYYOOOOOOOOOOOFF",
        "FYYOOPPPPPPOOOFF",
        "FFOOOOOOOOOOOOFF",
        "FFFOOOOOOOOOOFFF",
        "FFFFFFFFFFFFFFFF",
        "FFFFFFDYYDFFFFFF",
        "FFFFFFDDDDFFFFFF",
        "FFFFFFFFFFFFFFFF",
    )

    // Llumen: fanal amb llumeta càlida
    val LLUMEN = listOf(
        "FFFFFFFFFFFFFFFF",
        "FFFFFNNNNNNFFFFF",
        "FFFFNNNNNNNNFFFF",
        "FFFFNYYYYYYNFFFF",
        "FFFFNYOOOOYNFFFF",
        "FFFFNYOWWOYNFFFF",
        "FFFFNYYWWYYNFFFF",
        "FFFFNYYYYYYNFFFF",
        "FFFFNYYYYYYNFFFF",
        "FFFFNYYYYYYNFFFF",
        "FFFFNNNNNNNNFFFF",
        "FFFFFFNNNNFFFFFF",
        "FFFFFNNNNNNFFFFF",
        "FFFFNNNNNNNNFFFF",
        "FFFFFFFFFFFFFFFF",
        "FFFFFFFFFFFFFFFF",
    )

    // Pixel: robot amb pantalla de cara
    val PIXEL = listOf(
        "FFFFFFFRRFFFFFFF",
        "FFFFFNNNNNNFFFFF",
        "FFFFSSSSSSSSFFFF",
        "FFFFSFFFFFFSFFFF",
        "FFFFSFBFFBFSFFFF",
        "FFFFSFBFFBFSFFFF",
        "FFFFSFBFFBFSFFFF",
        "FFFFSFFFFFFSFFFF",
        "FFFFSSBBBBSSFFFF",
        "FFFFSSSSSSSSFFFF",
        "FFFFSSSSSSSSFFFF",
        "FFFFFNNNNNNFFFFF",
        "FFFFNNFFFFNNFFFF",
        "FFFFNNFFFFNNFFFF",
        "FFFFFFFFFFFFFFFF",
        "FFFFFFFFFFFFFFFF",
    )

    // Mica: gateta amb orelles
    val MICA = listOf(
        "FFFFFFFFFFFFFFFF",
        "FFNPNFFFFFFNPNFF",
        "FFNPNFFFFFFNPNFF",
        "FFSSSSSSSSSSSSFF",
        "FFSSSSSSSSSSSSFF",
        "FFSSKWSSSSWKSSFF",
        "FFSSKKSSSSKKSSFF",
        "FNSSSSSSSSSSSSNF",
        "FNSSSSSPPSSSSSNF",
        "FFSSSSSSSSSSSSFF",
        "FFSSSSSSSSSSSSFF",
        "FFFSSSSSSSSSSFFF",
        "FFFFFSSSSSSFFFFF",
        "FFFFFSSSSSSFFFFF",
        "FFFFFFSSSSFFFFFF",
        "FFFFFFFFFFFFFFFF",
    )

    /** Torna la matriu de l'avatar d'una mascota (fallback: robot). */
    fun grid(id: String): List<String> = when (id) {
        "mel" -> MEL
        "llumen" -> LLUMEN
        "pixel" -> PIXEL
        "mica" -> MICA
        else -> PIXEL
    }

    /** Converteix la matriu a pixels: llista de files, cada pixel ARGB. */
    fun pixels(grid: List<String>): List<List<Int>> =
        grid.map { r -> r.map { c -> PAL[CH.getValue(c)] } }

    /** Color accent de cada mascota (per botons i detalls de la UI). */
    fun accent(id: String): Long = when (id) {
        "mel" -> 0xFFFF9F00 // taronja DJ
        "llumen" -> 0xFFFFEC39 // groc fanal
        "pixel" -> 0xFF29ADFF // blau robot
        "mica" -> 0xFFA3E24E // verd gateta
        else -> 0xFF29ADFF
    }
}
