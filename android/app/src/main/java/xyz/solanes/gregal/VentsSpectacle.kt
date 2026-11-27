package xyz.solanes.gregal

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.ui.draw.drawWithCache
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.drawscope.rotate
import kotlin.math.PI
import kotlin.math.cos
import kotlin.math.sin
import kotlinx.coroutines.launch
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.zIndex

/** Shared minimal mascot wrappers and truthful activity indicators. */

/** Shared mascot wrapper; source-compatible name, no halo or invented presence. */
@Composable
fun WindHalo(
    accent: Color,
    style: WindStyle,
    size: Int = 48,
    mood: WindMood = WindMood.IDLE,
    online: Boolean = true,
    stacked: List<Wind> = emptyList(),
    tappable: Boolean = false,
    showKit: Boolean = false,
    clay: Boolean = false,
) {
    Box(contentAlignment = Alignment.Center, modifier = Modifier.size((size + 14).dp)) {
        // Same footprint as before, without rings or shadows around the silhouette.
        WindFace(size = size + 8, mood = mood, accent = accent, style = style, tappable = tappable, showKit = showKit, clay = clay)
        if (stacked.isNotEmpty()) {
            stacked.take(1).forEach {
                Box(Modifier.align(Alignment.BottomEnd).offset(x = 4.dp, y = 4.dp).zIndex(1f)) {
                    Surface(shape = CircleShape, color = Superficie) {
                        WindFace(size = 26, accent = Color(it.argb), style = it.faceStyle)
                    }
                }
            }
            if (stacked.size > 1) {
                // A l'esquerra: la cantonada dreta és per a la insígnia d'estat de la cara.
                Box(
                    Modifier.align(Alignment.TopStart).background(Ink, CircleShape)
                        .padding(horizontal = 5.dp, vertical = 1.dp).zIndex(1f)
                ) {
                    Text("+${stacked.size - 1}", color = Superficie, fontSize = 9.sp, fontWeight = FontWeight.Bold)
                }
            }
        }
        // `online` stays source-compatible; a decorative avatar is not proof of presence.
    }
}

/**
 * El gest d'un vent en acabar un torn: confeti del seu color quan ha anat bé
 * (DONE), una arronsada d'espatlles trista quan ha fallat (FAILED, si [shrugs]:
 * el vent de la cantonada del xat fa que no amb el cap, com al web). Només en
 * el canvi d'humor, mai per a una cara que ja es mostra feta, i res amb el
 * moviment apagat. Tot es llegeix al dibuixat i a la capa: cap fotograma
 * recompon la cara ni la llista on viu.
 */
@Composable
internal fun windTurnCue(mood: WindMood, accent: Color, shrugs: Boolean): Modifier {
    if (!windMotionEnabled()) return Modifier
    // 1 = acabat: res a dibuixar.
    val burst = remember { Animatable(1f) }
    val shrug = remember { Animatable(1f) }
    var last by remember { mutableStateOf(mood) }
    // Fora de l'efecte: si l'humor torna a canviar a mig gest, el gest acaba igualment i no queda glaçat.
    val scope = rememberCoroutineScope()
    LaunchedEffect(mood) {
        val before = last
        last = mood
        if (before == mood) return@LaunchedEffect
        when (mood) {
            WindMood.DONE -> scope.launch { burst.snapTo(0f); burst.animateTo(1f, tween(950, easing = LinearEasing)) }
            WindMood.FAILED -> if (shrugs) scope.launch { shrug.snapTo(0f); shrug.animateTo(1f, tween(900, easing = LinearEasing)) }
            else -> {}
        }
    }
    val tones = remember(accent) { listOf(accent, windLight(accent), windDeep(accent)) }
    return Modifier.graphicsLayer {
        val f = shrug.value
        if (f >= 1f) return@graphicsLayer
        // Puja les espatlles un moment i es deixa caure, decantat, abans de refer-se.
        val up = if (f < .22f) sin(PI.toFloat() * f / .22f) else 0f
        val down = if (f >= .22f) sin(PI.toFloat() * (f - .22f) / .78f) else 0f
        transformOrigin = TransformOrigin(.5f, 1f)
        translationY = (-.05f * up + .06f * down) * size.height
        scaleY = 1f + .05f * up - .06f * down
        scaleX = 1f + .04f * down
        rotationZ = -8f * down
    }.drawWithContent {
        drawContent()
        val f = burst.value
        if (f >= 1f) return@drawWithContent
        val side = size.minDimension
        val c = center
        val out = 1f - (1f - f) * (1f - f) * (1f - f)
        val fade = if (f < .55f) 1f else (1f - f) / .45f
        for (i in 0 until 14) {
            // Desordre fix per partícula: el mateix esclat cada cop, però no una roda.
            val j = sin(i * 12.9898f) * 43758.547f % 1f
            val a = (i * 360f / 14f + j * 18f) * PI.toFloat() / 180f
            val reach = side * (.5f + (.3f + .14f * kotlin.math.abs(j)) * out)
            val p = Offset(c.x + cos(a) * reach, c.y + sin(a) * reach + side * .16f * f * f)
            val tone = tones[i % 3].copy(alpha = fade)
            val bit = side * .05f * (1f - .4f * f)
            when (i % 3) {
                // Una espurna de quatre puntes.
                0 -> {
                    drawLine(tone, Offset(p.x - bit, p.y), Offset(p.x + bit, p.y), bit * .45f, StrokeCap.Round)
                    drawLine(tone, Offset(p.x, p.y - bit), Offset(p.x, p.y + bit), bit * .45f, StrokeCap.Round)
                }
                // Un paperet que gira mentre vola.
                1 -> rotate(f * 540f * (if (j > 0) 1f else -1f) + i * 40f, p) {
                    drawRoundRect(tone, Offset(p.x - bit * .9f, p.y - bit * .4f), Size(bit * 1.8f, bit * .8f), CornerRadius(bit * .3f))
                }
                else -> drawCircle(tone, bit * .5f, p)
            }
        }
    }
}

/** Tres punts que respiren quan el vent escriu. Estàtics als tests. */
@Composable
fun TypingDots(accent: Color = Sea) {
    if (!windMotionEnabled()) {
        Text("•••", color = accent, fontSize = 16.sp, fontWeight = FontWeight.Bold)
        return
    }
    val t = rememberInfiniteTransition(label = "typing")
    Row(horizontalArrangement = Arrangement.spacedBy(4.dp), verticalAlignment = Alignment.CenterVertically) {
        for (i in 0 until 3) {
            val a by t.animateFloat(
                initialValue = 0.25f, targetValue = 1f,
                animationSpec = infiniteRepeatable(
                    tween(900, delayMillis = i * 180, easing = FastOutSlowInEasing), RepeatMode.Reverse
                ), label = "dot$i"
            )
            // L'alfa es llegeix a la fase de dibuixat (drawWithCache): l'animació
            // no recomposa el Row ni la resta de la fila 60 cops per segon.
            Box(Modifier.size(7.dp).drawWithCache {
                val color = accent.copy(alpha = 0.35f + 0.65f * a)
                onDrawBehind { drawCircle(color) }
            })
        }
    }
}

/**
 * Cursor que pampallugueja al final del text en directe: l'efecte «està
 * escrivint» dels xats de tota la vida. Estàtic (visible) sense animacions.
 */
@Composable
fun TypingCursor(accent: Color = Sea) {
    if (!windMotionEnabled()) {
        Text("▍", color = accent, fontSize = 15.sp)
        return
    }
    val t = rememberInfiniteTransition(label = "cursor")
    val a by t.animateFloat(
        initialValue = 0.15f, targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(650, easing = FastOutSlowInEasing), RepeatMode.Reverse),
        label = "blink"
    )
    // Fase de dibuixat: el parpelleig no recomposa la bombolla del missatge.
    Text("▍", color = accent, fontSize = 15.sp,
        modifier = Modifier.graphicsLayer { alpha = 0.15f + 0.85f * a })
}
