package xyz.solanes.gregal

/** Keep all the text, but only copy it for Compose at most once per interval. */
internal class WindStreamText(private val now: () -> Long = System::nanoTime) {
    private val text = StringBuilder()
    private var lastShown: Long? = null

    fun append(token: String): String? {
        if (token.isEmpty()) return null
        text.append(token)
        val time = now()
        if (lastShown?.let { time - it < 80_000_000L } == true) return null
        lastShown = time
        return text.toString()
    }

    // A tool or speaker boundary must retain even tokens between paints.
    fun take(): String = text.toString().also {
        text.clear()
        lastShown = null
    }
}
