package xyz.solanes.gregal

import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.keyframes
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Typography
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/*
 * Identitat visual: la marca (la ratxa de vent amb ulls de ventet.svg, la
 * mateixa de l'escriptori i de la icona) i una tipografia més ajustada que la
 * de Material per defecte, que té un espaiat de lletra pensat per a botons.
 */

private const val MARK_D = "M810,100C950,360 900,640 640,795C470,900 240,915 70,880C300,870 490,795 565,705" +
    "C460,765 340,800 225,810C490,665 535,575 225,510C510,425 770,340 810,100Z"

/**
 * La marca Gregal. `alive`: sura suaument i parpelleja (estats buits i
 * benvinguda); si no, és estàtica (capçaleres).
 */
@Composable
fun GregalMark(size: Dp, modifier: Modifier = Modifier, alive: Boolean = false) {
    val path = remember { PathParser().parsePathString(MARK_D).toPath() }
    val top = SeaClara
    val mid = Sea
    val eye = DeepTop
    var float = 0f
    var blink = 1f
    if (alive) {
        val t = rememberInfiniteTransition(label = "marca")
        val f by t.animateFloat(-1f, 1f, infiniteRepeatable(tween(2600, easing = FastOutSlowInEasing), RepeatMode.Reverse), label = "sura")
        val b by t.animateFloat(1f, 1f, infiniteRepeatable(keyframes {
            durationMillis = 4200
            1f at 3800; 0.1f at 3900; 1f at 4000
        }), label = "parpella")
        float = f; blink = b
    }
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension / 1024f
        translate(top = float * this.size.height * 0.025f) {
            scale(s, s, pivot = Offset.Zero) {
                drawPath(path, Brush.linearGradient(listOf(top, mid, mid.copy(alpha = 0.92f)), Offset(810f, 100f), Offset(70f, 880f)))
                // Ulls: el·lipses girades 18° (com el mestre SVG).
                rotate(18f, Offset(675f, 530f)) {
                    drawOval(eye, topLeft = Offset(646f, 530f - 53f * blink), size = androidx.compose.ui.geometry.Size(58f, 106f * blink))
                }
                rotate(18f, Offset(792f, 518f)) {
                    drawOval(eye, topLeft = Offset(765f, 518f - 51f * blink), size = androidx.compose.ui.geometry.Size(54f, 102f * blink))
                }
            }
        }
    }
}

/** Icona dins d'un cercle tintat suau: estats buits i capçaleres de secció. */
@Composable
fun TintedIcon(icon: ImageVector, tint: Color, box: Dp = 64.dp, glyph: Dp = 30.dp) {
    Box(Modifier.size(box).background(tint.copy(alpha = 0.13f), CircleShape), contentAlignment = Alignment.Center) {
        Icon(icon, null, tint = tint, modifier = Modifier.size(glyph))
    }
}

/**
 * Tipografia Gregal: la de Material sense l'espaiat de lletra ample del cos
 * (0.5 sp feia el text del xat airejat i poc dens) i títols més ferms.
 */
fun gregalTypography(): Typography {
    val base = Typography()
    fun TextStyle.tight(ls: Double = 0.0) = copy(letterSpacing = ls.sp)
    return base.copy(
        displaySmall = base.displaySmall.copy(fontWeight = FontWeight.SemiBold, letterSpacing = (-0.5).sp),
        headlineMedium = base.headlineMedium.copy(fontWeight = FontWeight.SemiBold, letterSpacing = (-0.4).sp),
        headlineSmall = base.headlineSmall.copy(fontWeight = FontWeight.SemiBold, letterSpacing = (-0.3).sp),
        titleLarge = base.titleLarge.copy(fontWeight = FontWeight.SemiBold, letterSpacing = (-0.2).sp),
        titleMedium = base.titleMedium.copy(fontWeight = FontWeight.SemiBold).tight(),
        titleSmall = base.titleSmall.tight(),
        bodyLarge = base.bodyLarge.tight(),
        bodyMedium = base.bodyMedium.tight(),
        bodySmall = base.bodySmall.tight(0.1),
        labelLarge = base.labelLarge.copy(fontWeight = FontWeight.SemiBold).tight(0.1),
        labelMedium = base.labelMedium.tight(0.2),
        labelSmall = base.labelSmall.tight(0.3),
    )
}
