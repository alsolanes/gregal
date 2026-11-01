package xyz.solanes.gregal

import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.RoundRect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathMeasure
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/**
 * Halo de llum de la zona d'escriure mentre el vent treballa (estil Grok /
 * Apple Intelligence):
 *
 * - Base sempre sencera que respira (doble corona exterior + vora viva),
 *   perquè el contorn mai quedi buit ni sembli trencat.
 * - Dos feixos bessons oposats (a 180°) que orbiten seguint el perímetre real
 *   amb [PathMeasure] i segments solapats (~40% de solapament) des de [accent]
 *   cap a [Sand], amb capçal lluminós suau.
 *
 * Es dibuixa **per fora** del camp ([drawBehind]), així no tapa mai ni el text
 * ni els botons, i només apareix quan hi ha feina ([active] = true).
 *
 * @param active si s'ha de veure (l'activitat del vent o de l'agent)
 * @param corner radi de la pastilla, perquè la llum segueixi la seva forma
 * @param outset com surt cap enfora; prou perquè el gruix quedi tot visible
 * @param volta ms d'una òrbita sencera
 * @param color color principal del halo (per defecte [Sea], o el color del vent)
 */
@Composable
fun Modifier.workingRing(
    active: Boolean,
    corner: Dp = 27.dp,
    outset: Dp = 2.dp,
    volta: Int = 2600,
    color: Color = Color.Unspecified,
): Modifier {
    if (!active) return this
    val primary = if (color != Color.Unspecified) color else Sea
    val secondary = lerp(primary, Sand, 0.55f)
    val transicio = rememberInfiniteTransition(label = "anell-composer")
    val fase by transicio.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(volta, easing = LinearEasing)),
        label = "orbita",
    )
    val respira by transicio.animateFloat(
        initialValue = 0.38f,
        targetValue = 0.78f,
        animationSpec = infiniteRepeatable(tween(volta / 2, easing = FastOutSlowInEasing), RepeatMode.Reverse),
        label = "respira",
    )
    return drawBehind {
        val fora = outset.toPx()
        val radi = corner.toPx() + fora
        val cami = Path().apply {
            addRoundRect(
                RoundRect(
                    rect = Rect(Offset(-fora, -fora), Size(size.width + fora * 2, size.height + fora * 2)),
                    cornerRadius = CornerRadius(radi),
                ),
            )
        }
        // 1. Corona exterior ampla i difusa que respira.
        drawPath(cami, primary.copy(alpha = 0.09f * respira), style = Stroke(12.dp.toPx()))
        drawPath(cami, secondary.copy(alpha = 0.14f * respira), style = Stroke(6.5.dp.toPx()))
        // 2. Anell base complet i continu (mai queda cap cantonada fosca).
        drawPath(cami, primary.copy(alpha = 0.34f + 0.18f * respira), style = Stroke(1.6.dp.toPx()))

        // 3. Dos feixos bessons a 180° seguint el contorn real amb PathMeasure.
        val mesura = PathMeasure()
        mesura.setPath(cami, true)
        val total = mesura.length
        if (total <= 1f) return@drawBehind

        val passos = 24
        val llargadaCua = total * 0.32f
        val pas = llargadaCua / passos
        val gruixMax = 3.2.dp.toPx()
        val gruixHalo = 7.5.dp.toPx()

        for (bessons in 0..1) {
            val capFrac = (fase + bessons * 0.5f) % 1f
            val posCap = capFrac * total
            for (i in 0 until passos) {
                val t = 1f - (i.toFloat() / passos) // 1 al cap, >0 a la cua
                // Solapament ~45% perquè els trams rectes i corbs es fonguin sense punts.
                val fi = (posCap - i * pas + total * 2) % total
                val inici = (fi - pas * 1.45f + total * 2) % total
                val tram = Path()
                if (inici < fi) {
                    mesura.getSegment(inici, fi, tram, true)
                } else {
                    mesura.getSegment(inici, total, tram, true)
                    mesura.getSegment(0f, fi, tram, true)
                }
                val to = lerp(secondary, primary, t)
                val alfa = (t * t) * 0.78f
                drawPath(tram, to.copy(alpha = alfa * 0.28f), style = Stroke(gruixHalo * (0.4f + 0.6f * t), cap = StrokeCap.Round))
                drawPath(tram, to.copy(alpha = alfa), style = Stroke(gruixMax * (0.35f + 0.65f * t), cap = StrokeCap.Round))
            }
            // Capçal brillant al davant de cada feix.
            val puntCap = mesura.getPosition(posCap)
            drawCircle(primary.copy(alpha = 0.30f), radius = 6.dp.toPx(), center = puntCap)
            drawCircle(lerp(primary, Color.White, 0.45f).copy(alpha = 0.92f), radius = 2.4.dp.toPx(), center = puntCap)
        }
    }
}
