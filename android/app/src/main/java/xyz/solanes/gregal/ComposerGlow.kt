package xyz.solanes.gregal

import androidx.compose.animation.core.LinearEasing
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
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathMeasure
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/**
 * La llum que dona voltes a la vora de la zona d'escriure mentre el vent
 * treballa: una cua d'estela que recorre el perímetre de la pastilla.
 *
 * Es dibuixa **per fora** del camp, així no tapa mai ni el text ni els botons,
 * i només apareix quan hi ha feina. El recorregut va per la vora de debò
 * (`PathMeasure` sobre el rectangle arrodonit): amb un `sweepGradient` girat la
 * llum no seguia la forma i sortia com una ratlla recta que travessava el camp.
 *
 * @param active si s'ha de veure (l'activitat de l'agent)
 * @param corner radi de la pastilla, perquè la llum segueixi la seva forma
 * @param outset com surt cap enfora; prou perquè el gruix quedi tot visible
 * @param volta segons que triga a fer una volta sencera
 */
@Composable
fun Modifier.workingRing(
    active: Boolean,
    corner: Dp = 27.dp,
    outset: Dp = 1.5.dp,
    volta: Int = 1800,
): Modifier {
    if (!active) return this
    val accent = Sea
    val transicio = rememberInfiniteTransition(label = "anell-composer")
    val progrés by transicio.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(volta, easing = LinearEasing)),
        label = "progrés",
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
        val mesura = PathMeasure().apply { setPath(cami, false) }
        val llarg = mesura.length
        if (llarg <= 0f) return@drawBehind
        // La vora apagada de fons: sense ella la cua semblaria un error.
        drawPath(cami, accent.copy(alpha = 0.22f), style = Stroke(1.dp.toPx()))
        // La cua: tirades cap enrere amb cada cop menys opacitat, que és el
        // que fa l'efecte d'estela. Se solapen una mica perquè no surtin
        // puntejades a les vores rectes, i al davant hi ha el cap de llum.
        val trams = 8
        val ample = llarg * 0.20f
        val pas = ample / trams
        for (i in 0 until trams) {
            val cap = progrés * llarg
            val inici = cap - (i + 1) * pas - pas * 0.35f
            val final_ = cap - i * pas
            val a = 1f - i.toFloat() / trams
            val tros = Path()
            val s = ((inici % llarg) + llarg) % llarg
            val e = ((final_ % llarg) + llarg) % llarg
            if (s <= e) {
                mesura.getSegment(s, e, tros, true)
            } else {
                mesura.getSegment(s, llarg, tros, true)
                val segona = Path()
                mesura.getSegment(0f, e, segona, true)
                tros.addPath(segona)
            }
            val alfa = Math.pow(a.toDouble(), 1.3).toFloat()  // cua més llarga visible
            // Halo: la mateixa tirada, més ampla i molt fluixa, que fa que la
            // llum «tacui» la vora en lloc de ser un filet dibuixat.
            drawPath(tros, accent.copy(alpha = alfa * 0.30f), style = Stroke(7.dp.toPx(), cap = StrokeCap.Round))
            drawPath(tros, accent.copy(alpha = alfa), style = Stroke(1.8.dp.toPx() + a * 1.6.dp.toPx(), cap = StrokeCap.Round))
        }
        // El cap: una llum més forta allà on va la punta de l'estela.
        val punta = mesura.getPosition(((progrés * llarg) % llarg))
        drawCircle(accent.copy(alpha = 0.35f), radius = 7.dp.toPx(), center = punta)
        drawCircle(accent, radius = 3.dp.toPx(), center = punta)
    }
}
