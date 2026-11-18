package xyz.solanes.gregal

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue

/**
 * Quants caràcters s'han de mostrar després de [dtNanos] si n'hi ha [backlog]
 * per revelar. El ritme creix amb la cua (s'esgota en uns 0,25 s) però mai baixa
 * d'un mínim: el text raja de forma contínua en lloc d'arribar a salts cada 80 ms.
 */
internal fun pacedStep(backlog: Float, dtNanos: Long): Float {
    if (backlog <= 0f) return 0f
    val perSecond = maxOf(60f, backlog * 4f)
    return minOf(backlog, perSecond * dtNanos / 1_000_000_000f)
}

/** Una paraula més llarga que això (un enllaç, codi) es revela igualment lletra a lletra. */
private const val LONG_WORD = 24

/**
 * On es pot tallar el text en directe sense deixar una paraula a mitges:
 * revelar-la lletra a lletra feia que, en no cabre, saltés a la línia de sota a
 * mig escriure (el tremolor del final de línia). [n] és el que tocaria mostrar;
 * si cau dins d'una paraula (o al final d'un tros que encara no l'ha acabada),
 * es recula fins al seu inici. Pura: testejable.
 */
internal fun wordSafeCut(text: String, n: Int): Int {
    if (n <= 0) return 0
    val end = n.coerceAtMost(text.length)
    // Dins d'una paraula: el caràcter següent encara és lletra (o el tros acaba sense espai).
    val midWord = if (end < text.length) !text[end].isWhitespace() && !text[end - 1].isWhitespace()
        else !text[end - 1].isWhitespace()
    if (!midWord) return end
    var i = end
    while (i > 0 && !text[i - 1].isWhitespace()) i--
    return if (end - i > LONG_WORD) end else i
}

/** El text que es veu: persegueix [target] fotograma a fotograma mentre [enabled]. */
@Composable
internal fun rememberPacedText(target: String, enabled: Boolean): String {
    var shown by remember { mutableIntStateOf(if (enabled) 0 else target.length) }
    val latest by rememberUpdatedState(target)
    LaunchedEffect(enabled) {
        if (!enabled) return@LaunchedEffect
        var exact = shown.toFloat()
        var last = 0L
        while (true) {
            androidx.compose.runtime.withFrameNanos { now ->
                val t = latest
                if (exact > t.length) exact = t.length.toFloat()
                if (last != 0L) exact += pacedStep(t.length - exact, now - last)
                last = now
                var n = wordSafeCut(t, exact.toInt())
                // No partir un parell de substitució (emoji) pel mig.
                if (n in 1 until t.length && Character.isHighSurrogate(t[n - 1])) n--
                if (n != shown) shown = n
            }
        }
    }
    return if (enabled) target.take(shown.coerceAtMost(target.length)) else target
}
