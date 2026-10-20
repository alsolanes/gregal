package xyz.solanes.gregal

import androidx.compose.animation.core.*
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.dp
import kotlin.math.min

/**
 * WindFace — la cara viva d'un vent de Gregal: una bola del color triat que
 * respira, parpelleja i reacciona a l'estat.
 *
 * La cara és personal: ulls, boca i forma es trien al creador (WindStyle) i es
 * desen amb el vent. Tot és Canvas; res d'imatges.
 */

/** L'estat d'ànim del vent, pel qual canvien els ulls. */
enum class WindMood { IDLE, THINKING, HAPPY }

/** Decorative motion can be disabled for accessibility and deterministic UI tests. */
val LocalWindMotion = compositionLocalOf { true }

/**
 * WindStyle — les peces triables de la cara:
 *  - eyes: punts, badalls (ovals grossos), estrellats, mig tancats
 *  - mouth: somriure, oberta (parlant), neutra, llengua
 *  - shape: bola, núvol, espiral
 * Desat com a tres lletres al backend (p.ex. "pso" = punts/somriure/bola).
 */
data class WindStyle(val eyes: WindEyes = WindEyes.DOTS, val mouth: WindMouth = WindMouth.SMILE, val shape: WindShape = WindShape.BALL)

enum class WindEyes(val key: String) { DOTS("p"), WIDE("b"), STARRY("e"), SLEEPY("m") }
enum class WindMouth(val key: String) { SMILE("s"), OPEN("o"), NEUTRAL("n"), TONGUE("t") }
enum class WindShape(val key: String) { BALL("o"), CLOUD("n"), SPIRAL("s") }

/** Del codi de tres lletres del backend ("pso") a l'estil. */
fun windStyleOf(code: String?): WindStyle {
    val c = (code ?: "").ifBlank { "pso" }
    val eyes = WindEyes.entries.firstOrNull { it.key == c.getOrNull(0)?.toString() } ?: WindEyes.DOTS
    val mouth = WindMouth.entries.firstOrNull { it.key == c.getOrNull(1)?.toString() } ?: WindMouth.SMILE
    val shape = WindShape.entries.firstOrNull { it.key == c.getOrNull(2)?.toString() } ?: WindShape.BALL
    return WindStyle(eyes, mouth, shape)
}

/** De l'estil al codi de tres lletres que viatja al backend. */
fun WindStyle.encode(): String = eyes.key + mouth.key + shape.key

@Composable
fun WindFace(
    size: Int = 96,
    mood: WindMood = WindMood.IDLE,
    accent: Color = Color(0xFFFF9F00),
    style: WindStyle = WindStyle(),
) {
    val context = androidx.compose.ui.platform.LocalContext.current
    val motion = LocalWindMotion.current && android.provider.Settings.Global.getFloat(
        context.contentResolver, android.provider.Settings.Global.ANIMATOR_DURATION_SCALE, 1f) > 0f
    val breathe = if (motion) {
        val value by rememberInfiniteTransition(label = "wind-breathe").animateFloat(
            initialValue = 1f, targetValue = 1.04f,
            animationSpec = infiniteRepeatable(tween(1600, easing = LinearEasing), RepeatMode.Reverse), label = "breathing")
        value
    } else 1f
    val blink = if (motion) {
        val value by rememberInfiniteTransition(label = "wind-blink").animateFloat(
            initialValue = 1f, targetValue = 1f,
            animationSpec = infiniteRepeatable(keyframes {
                durationMillis = 4200; 1f at 0; 1f at 3900; 0f at 3960; 0f at 4050; 1f at 4120
            }), label = "eyelids")
        value
    } else 1f
    val blinking = blink < .5f
    val gaze = if (motion) {
        val value by rememberInfiniteTransition(label = "wind-gaze").animateFloat(
            initialValue = -1f, targetValue = 1f,
            animationSpec = infiniteRepeatable(tween(3400, easing = FastOutSlowInEasing), RepeatMode.Reverse), label = "gaze")
        value
    } else 0f
    val thinkingPhase = if (motion) {
        val value by rememberInfiniteTransition(label = "wind-think").animateFloat(
            initialValue = 0f, targetValue = 3f,
            animationSpec = infiniteRepeatable(tween(1200, easing = LinearEasing)), label = "thinking")
        value
    } else 0f

    Canvas(Modifier.size(size.dp)) {
        val w = this.size.width
        val h = this.size.height
        val cx = w / 2f
        val cy = h / 2f
        val r = min(w, h) * 0.37f * breathe
        val ink = Color(0xFF463C46)

        // Keep the complete silhouette inside the canvas, including a breathing cloud.
        drawOval(ink.copy(alpha = .06f), Offset(cx - r * .82f, cy + r * .88f), Size(r * 1.64f, r * .22f))
        when (style.shape) {
            WindShape.CLOUD -> {
                drawRoundRect(accent, Offset(cx - r * 1.19f, cy - r * .30f), Size(r * 2.38f, r * 1.27f),
                    cornerRadius = androidx.compose.ui.geometry.CornerRadius(r * .58f))
                drawCircle(accent, r * .67f, Offset(cx - r * .61f, cy - r * .05f))
                drawCircle(accent, r * .86f, Offset(cx + r * .08f, cy - r * .19f))
                drawCircle(accent, r * .60f, Offset(cx + r * .66f, cy + r * .04f))
            }
            WindShape.SPIRAL -> {
                drawCircle(accent, r, Offset(cx, cy))
                drawArc(ink.copy(alpha = .14f), 210f, 230f, false,
                    Offset(cx - r * .93f, cy - r * 1.02f), Size(r * 1.18f, r * .84f),
                    style = Stroke(r * .055f, cap = StrokeCap.Round))
                drawCircle(accent, r * .32f, Offset(cx - r * .30f, cy - r * .80f))
            }
            WindShape.BALL -> drawCircle(accent, r, Offset(cx, cy))
        }
        drawOval(Color.White.copy(alpha = .25f), Offset(cx - r * .57f, cy - r * .78f), Size(r * .47f, r * .22f))
        // Blush is warm and subtle, not a second outline around the face.
        for (side in listOf(-1f, 1f)) {
            drawOval(Color(0xFFD8747E).copy(alpha = .31f), Offset(cx + side * r * .61f - r * .17f, cy + r * .20f), Size(r * .34f, r * .18f))
        }

        if (mood == WindMood.THINKING) {
            val phase = thinkingPhase.toInt() % 3
            for (i in 0 until 3) {
                val dy = if (i == phase) -r * 0.10f else 0f
                drawCircle(color = ink, radius = r * 0.09f, center = Offset(cx + (i - 1) * r * 0.28f, cy + dy))
            }
            return@Canvas
        }

        val eyeY = cy - r * 0.05f
        val eyeDx = r * 0.38f
        val eyeW = r * 0.12f
        val gazeShift = gaze * r * 0.06f

        // Ulls: segons l'estil; HAPPY tanca (arc feliç) i el parpelleig també.
        if (mood == WindMood.HAPPY || blinking) {
            for (side in listOf(-1f, 1f)) {
                drawArc(
                    color = ink,
                    startAngle = 190f,
                    sweepAngle = 160f,
                    useCenter = false,
                    topLeft = Offset(cx + side * eyeDx - eyeW, eyeY - eyeW),
                    size = Size(eyeW * 2, eyeW * 2),
                    style = Stroke(width = r * 0.05f, cap = StrokeCap.Round),
                )
            }
        } else {
            for (side in listOf(-1f, 1f)) {
                val ex = cx + side * eyeDx + gazeShift
                when (style.eyes) {
                    WindEyes.DOTS -> {
                        drawOval(
                            color = ink,
                            topLeft = Offset(ex - eyeW, eyeY - eyeW * 1.2f),
                            size = Size(eyeW * 2, eyeW * 2.4f),
                        )
                        drawCircle(
                            color = Color.White.copy(alpha = 0.85f), radius = eyeW * 0.32f,
                            center = Offset(ex + eyeW * 0.35f, eyeY - eyeW * 0.35f),
                        )
                    }
                    WindEyes.WIDE -> {
                        // Badalls grossos: blanc darrere, nineta gran.
                        drawOval(
                            color = Color.White.copy(alpha = 0.92f),
                            topLeft = Offset(ex - eyeW * 1.25f, eyeY - eyeW * 1.45f),
                            size = Size(eyeW * 2.5f, eyeW * 2.9f),
                        )
                        drawCircle(color = ink, radius = eyeW * 0.85f, center = Offset(ex, eyeY))
                        drawCircle(
                            color = Color.White.copy(alpha = 0.9f), radius = eyeW * 0.28f,
                            center = Offset(ex + eyeW * 0.30f, eyeY - eyeW * 0.30f),
                        )
                    }
                    WindEyes.STARRY -> {
                        // Estrella de quatre puntes: dues rombes creuades.
                        fun star(cxS: Float, cyS: Float, rad: Float) {
                            val pts = listOf(
                                Offset(cxS, cyS - rad),
                                Offset(cxS + rad * 0.28f, cyS - rad * 0.28f),
                                Offset(cxS + rad, cyS),
                                Offset(cxS + rad * 0.28f, cyS + rad * 0.28f),
                                Offset(cxS, cyS + rad),
                                Offset(cxS - rad * 0.28f, cyS + rad * 0.28f),
                                Offset(cxS - rad, cyS),
                                Offset(cxS - rad * 0.28f, cyS - rad * 0.28f),
                            )
                            for (i in 0 until pts.size) {
                                drawLine(
                                    color = ink,
                                    start = pts[i],
                                    end = pts[(i + 1) % pts.size],
                                    strokeWidth = r * 0.028f,
                                    cap = StrokeCap.Round,
                                )
                            }
                        }
                        star(ex, eyeY, eyeW * 1.5f)
                        drawCircle(color = Color.White.copy(alpha = 0.8f), radius = eyeW * 0.22f, center = Offset(ex, eyeY))
                    }
                    WindEyes.SLEEPY -> {
                        // Mig tancats: tapa superior + escletxa.
                        drawOval(
                            color = ink,
                            topLeft = Offset(ex - eyeW, eyeY - eyeW * 0.4f),
                            size = Size(eyeW * 2, eyeW * 1.5f),
                        )
                        drawOval(
                            color = accent,
                            topLeft = Offset(ex - eyeW * 1.2f, eyeY - eyeW * 1.6f),
                            size = Size(eyeW * 2.4f, eyeW * 1.5f),
                        )
                    }
                }
            }
        }

        // Boca: segons l'estil. La base és un arc cap AVALL (somriure).
        when (style.mouth) {
            WindMouth.SMILE -> drawArc(
                color = ink,
                // Compose mesura els arcs des de la dreta, en sentit horari:
                // startAngle=20° + sweep 140° dibuixa la meitat INFERIOR de la
                // circumferència → la boca corba cap avall als extrems = somriure.
                startAngle = 20f,
                sweepAngle = 140f,
                useCenter = false,
                topLeft = Offset(cx - r * 0.30f, cy + r * 0.10f),
                size = Size(r * 0.60f, r * 0.55f),
                style = Stroke(width = r * 0.045f, cap = StrokeCap.Round),
            )
            WindMouth.OPEN -> {
                // Boca oberta: llavor fosca amb una mica de llengua.
                drawOval(
                    color = ink,
                    topLeft = Offset(cx - r * 0.14f, cy + r * 0.16f),
                    size = Size(r * 0.28f, r * 0.26f),
                )
                drawOval(
                    color = Color(0xFFE86A6A),
                    topLeft = Offset(cx - r * 0.10f, cy + r * 0.28f),
                    size = Size(r * 0.20f, r * 0.14f),
                )
            }
            WindMouth.NEUTRAL -> drawLine(
                color = ink,
                start = Offset(cx - r * 0.16f, cy + r * 0.28f),
                end = Offset(cx + r * 0.16f, cy + r * 0.28f),
                strokeWidth = r * 0.045f,
                cap = StrokeCap.Round,
            )
            WindMouth.TONGUE -> {
                drawArc(
                    color = ink,
                    startAngle = 20f,
                    sweepAngle = 140f,
                    useCenter = false,
                    topLeft = Offset(cx - r * 0.30f, cy + r * 0.10f),
                    size = Size(r * 0.60f, r * 0.55f),
                    style = Stroke(width = r * 0.045f, cap = StrokeCap.Round),
                )
                drawOval(
                    color = Color(0xFFE86A6A),
                    topLeft = Offset(cx - r * 0.08f, cy + r * 0.30f),
                    size = Size(r * 0.16f, r * 0.22f),
                )
            }
        }
    }
}
