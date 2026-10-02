package xyz.solanes.gregal

import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import kotlin.math.abs
import kotlin.math.max
import kotlin.math.min

// Les tres animacions de la marea (docs/identitat.md) i la línia d'horitzó,
// reproduïdes en Compose. Regla única: es mou només mentre hi ha feina.
//
// Les funcions de text són pures i testables (`LogicTest.kt`): són un
// port literal de `internal/tui/anim.go` i `theme.go`, i han de donar el
// mateix que el TUI caràcter a caràcter. Si divergeixen, el mòbil deixa de
// ser el mateix producte.

/** La cresta, de cim a calma: `≋` al centre, després `≈`, `~`, `·`. */
val tideCrest = charArrayOf('≋', '≈', '~', '·')

/** Marge (en columnes) que espera l'onada abans de tornar a entrar. */
const val tideMargin = 6

/** Resolució de vuitens del mesurador de context. */
val gaugeCells = charArrayOf('▏', '▎', '▍', '▌', '▋', '▊', '▉', '█')

/** Línea d'horitzó: distància normalitzada al centre → glif. */
private val swellSteps = listOf(
    0.35f to '≋', 0.60f to '≈', 0.82f to '~', 0.95f to '·', 1.01f to ' ',
)

/**
 * L'onada que passa: una sola cresta travessa un buit d'esquerra a dreta i
 * deixa mar plana entre onada i onada. Mateixa fórmula que `tideLine`.
 */
fun tideLine(width: Int, frame: Int): String {
    if (width <= 0) return ""
    val period = width + 2 * tideMargin
    val center = frame % period - tideMargin
    val out = CharArray(width) { ' ' }
    for (i in 0 until width) {
        val d = abs(i - center)
        if (d < tideCrest.size) out[i] = tideCrest[d]
    }
    return String(out)
}

/**
 * La cresta que llisca: retorna els intervals de columnes que van amb
 * `escumaClara` sobre el text donat. Una cresta de 3 columnes (-1..+1) i una
 * pausa de 10 columnes al final del cicle perquè no faci estrobo.
 */
fun shimmerRanges(len: Int, frame: Int): List<IntRange> {
    if (len <= 0) return emptyList()
    val period = len + 10
    val center = frame % period
    val out = mutableListOf<IntRange>()
    var start = -1
    for (i in 0 until len) {
        val at = i - center in -1..1
        if (at && start < 0) start = i
        if (!at && start >= 0) { out += start..(i - 1); start = -1 }
    }
    if (start >= 0) out += start..(len - 1)
    return out
}

/**
 * El mesurador que batega: barra de context amb resolució de vuitens. Per
 * sobre del 80% l'última cel·la plena alterna `█`/`▓` (batec). Mateix
 * càlcul que `tideGauge`.
 */
fun tideGauge(pct: Int, width: Int, frame: Int): String {
    if (width <= 0) return ""
    val p = min(max(pct, 0), 100)
    val eighths = p * width * 8 / 100
    val full = eighths / 8
    val rest = eighths % 8
    val sb = StringBuilder()
    repeat(full) { sb.append('█') }
    if (full < width) {
        sb.append(if (rest > 0) gaugeCells[rest - 1] else '·')
        repeat(width - full - 1) { sb.append('·') }
    }
    val out = sb.toString().toCharArray()
    if (p >= 80 && full > 0 && frame % 2 == 1) out[full - 1] = '▓'
    return String(out)
}

/**
 * La línia d'horitzó: l'onada s'esmorteeix cap als extrems (cim `≋` al
 * centre, calma a les vores) en comptes d'una filera massissa. Port de
 * `waveLine`.
 */
fun waveLineChars(width: Int): String {
    if (width < 4) return ""
    val mid = (width - 1) / 2f
    val out = CharArray(width) { ' ' }
    for (i in 0 until width) {
        val t = if (mid > 0f) abs(i - mid) / mid else 1f
        for ((upto, ch) in swellSteps) {
            if (t < upto) { out[i] = ch; break }
        }
    }
    return String(out)
}

/** Durada del torn en curt, com al TUI: `7s`, `2m04s`, `1h05m`. */
fun elapsedShort(ms: Long): String {
    val s = (ms / 1000).coerceAtLeast(0)
    if (s < 60) return "${s}s"
    val m = s / 60
    if (m < 60) return "${m}m" + (s % 60).toString().padStart(2, '0') + "s"
    return "${m / 60}h" + (m % 60).toString().padStart(2, '0') + "m"
}

/** Percentatge de context ocupat (0 si el rol no declara finestra). */
fun contextPct(used: Int, window: Int): Int {
    if (window <= 0) return 0
    return min(100, used * 100 / window)
}

// --- Composables ---------------------------------------------------------

/**
 * L'onada que passa a la capçalera: només es mou mentre hi ha feina (regla
 * de la identitat). En repòs pinta la línia quieta.
 */
@Composable
fun TideLine(width: Int, working: Boolean, modifier: Modifier = Modifier) {
    if (width <= 0) return
    var frame by remember { mutableIntStateOf(0) }
    if (working) {
        LaunchedEffect(Unit) {
            while (true) { delay(130); frame++ }
        }
    }
    val run = remember(width, frame, working) {
        if (working) tideLine(width, frame) else waveLineChars(width)
    }
    Text(
        run, modifier,
        color = if (working) SeaClara else LocalPal.current.lineHi,
        fontFamily = FontFamily.Monospace, fontSize = 10.sp,
        maxLines = 1, softWrap = false,
    )
}

/** La cresta que llisca per sobre d'una etiqueta (TREBALLANT). */
@Composable
fun ShimmerText(text: String, modifier: Modifier = Modifier, size: Int = 14) {
    var frame by remember { mutableIntStateOf(0) }
    LaunchedEffect(text) {
        while (true) { delay(60); frame++ }
    }
    val ranges = remember(text, frame) { shimmerRanges(text.length, frame) }
    val hi = SeaClara
    val base = Muted
    Text(
        buildAnnotatedString {
            var i = 0
            for (r in ranges) {
                if (r.first > i) withStyle(SpanStyle(color = base)) { append(text.substring(i, r.first)) }
                withStyle(SpanStyle(color = hi, fontWeight = FontWeight.SemiBold)) {
                    append(text.substring(r.first, r.last + 1))
                }
                i = r.last + 1
            }
            if (i < text.length) withStyle(SpanStyle(color = base)) { append(text.substring(i)) }
        },
        modifier, fontFamily = FontFamily.Monospace, fontSize = size.sp,
        fontWeight = FontWeight.SemiBold, maxLines = 1,
    )
}

/**
 * El mesurador que batega: barra de context en glifs, amb els llindars del
 * TUI (≥50% arena, ≥80% vermell i batec). Si el rol no declara finestra
 * s'amaga: no s'inventa cap percentatge.
 */
@Composable
fun TideGaugeBar(used: Int, window: Int, cells: Int = 8, modifier: Modifier = Modifier) {
    // Sense finestra no s'inventa cap percentatge, i sense res consumit no
    // s'ensenya res: una filera de punts buida semblava una línia perduda.
    if (window <= 0 || used <= 0) return
    var frame by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) {
        while (true) { delay(120); frame++ }
    }
    val pct = contextPct(used, window)
    val run = remember(pct, frame) { tideGauge(pct, cells, frame) }
    val col = when {
        pct >= 80 -> Red
        pct >= 50 -> Sand
        else -> Sea
    }
    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        Text(
            run, color = col, fontFamily = FontFamily.Monospace, fontSize = 11.sp,
            maxLines = 1, softWrap = false,
        )
        // El número només quan ja és un avís: si no, els glifs ja són la
        // informació i el «43%» només era la mateixa dada dues vegades.
        if (pct >= 80) {
            Spacer(Modifier.width(5.dp))
            Text(
                "$pct%", color = col, fontFamily = FontFamily.Monospace, fontSize = 10.sp,
                fontWeight = FontWeight.Bold,
            )
        }
    }
}

/**
 * Superfície de vidre: translúcida sobre el fons amb vora `escuma` fina, com
 * les targetes de la web. El glassmorphism aquí és una vora + un vel, no un
 * blur (a Android el blur de fons costa i no afegeix res a aquesta densitat).
 */
@Composable
fun GlassCard(
    modifier: Modifier = Modifier,
    radius: Int = 14,
    hi: Boolean = false,
    content: @Composable () -> Unit,
) {
    val pal = LocalPal.current
    val shape = RoundedCornerShape(radius.dp)
    Box(
        modifier
            .background(
                Brush.verticalGradient(
                    listOf(
                        pal.surface.copy(alpha = if (hi) 0.94f else 0.78f),
                        pal.surfaceHi.copy(alpha = if (hi) 0.72f else 0.52f),
                    )
                ),
                shape,
            )
            .border(1.dp, if (hi) pal.lineHi else pal.line, shape),
    ) { content() }
}

/** Fons del mar: el degradat de fons més un vel d'escuma a la part alta. */
@Composable
fun SeaBackdrop(modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    val pal = LocalPal.current
    Box(
        modifier.background(
            Brush.verticalGradient(
                0f to pal.deepTop,
                0.55f to pal.deepTop,
                1f to pal.deepBottom,
            )
        )
    ) {
        Box(
            Modifier
                .height(180.dp)
                .background(
                    Brush.verticalGradient(
                        listOf(pal.sea.copy(alpha = 0.07f), Color.Transparent)
                    )
                )
        )
        content()
    }
}

/** Etiqueta d'estat amb la cresta lliscant i el rellotge del torn. */
@Composable
fun WorkingStrip(t0: Long, lang: String = "ca", modifier: Modifier = Modifier) {
    var now by remember { mutableIntStateOf(0) }
    LaunchedEffect(t0) {
        while (true) { delay(1000); now++ }
    }
    val ms = if (t0 > 0) System.currentTimeMillis() - t0 else 0L
    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        ShimmerText(tr(lang, "app.chat.workingCaps"))
        if (t0 > 0) {
            Spacer(Modifier.width(7.dp))
            Text("·", color = LocalPal.current.lineHi, fontSize = 12.sp)
            Spacer(Modifier.width(7.dp))
            Text(
                elapsedShort(ms), color = Muted,
                fontFamily = FontFamily.Monospace, fontSize = 12.sp,
            )
            // `now` només hi és perquè el recompose cada segon.
            if (now < 0) Spacer(Modifier.size(0.dp))
        }
    }
}

/** Color del text segons el to semàntic de la resposta. */
internal fun toColor(tone: String, pal: GregalPalette): Color = when (tone) {
    "ok" -> pal.green
    "err" -> pal.red
    else -> pal.muted
}
