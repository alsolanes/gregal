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
                var n = exact.toInt()
                // No partir un parell de substitució (emoji) pel mig.
                if (n in 1 until t.length && Character.isHighSurrogate(t[n - 1])) n--
                if (n != shown) shown = n
            }
        }
    }
    return if (enabled) target.take(shown.coerceAtMost(target.length)) else target
}
