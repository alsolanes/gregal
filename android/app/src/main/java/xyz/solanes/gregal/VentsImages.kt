package xyz.solanes.gregal

/** Imatges dels vents: l'eina image del servidor les desa i el vent les posa a
 *  la resposta com ![títol](/api/winds/image?id=…). La ruta vol la clau, com
 *  els documents, així que no es poden carregar com una URL qualsevol. */
private val windImagePath = Regex("""^/api/winds/image\?id=[a-f0-9]{24}\.(jpg|png|webp)$""")

internal fun isWindImageUrl(url: String): Boolean = windImagePath.matches(url.trim())

/** Bytes d'una imatge d'un vent, baixada del servidor amb la clau. */
internal fun windImageBytes(ctx: android.content.Context, url: String): ByteArray? = try {
    if (!isWindImageUrl(url)) null else {
        val p = ctx.getSharedPreferences("gregal", android.content.Context.MODE_PRIVATE)
        val token = p.getString("token", "").orEmpty()
        val host = p.getString("host", "").orEmpty()
        val port = p.getString("port", "8097").orEmpty()
        if (token.isBlank() || host.isBlank()) null
        else GregalApi(baseUrlOf(host, port), token).windImage(url.trim())
    }
} catch (_: Exception) { null }
