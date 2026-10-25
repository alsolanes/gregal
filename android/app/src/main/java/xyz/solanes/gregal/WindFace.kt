package xyz.solanes.gregal

import androidx.compose.animation.core.*
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.withTransform
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.unit.dp
import kotlin.math.PI
import kotlin.math.min
import kotlin.math.sin

/** WCAG contrast ratio between two colors (1..21), pure math for unit tests. */
internal fun contrastWith(a: Color, b: Color): Double {
    fun channel(v: Float): Double = if (v <= 0.03928) (v / 12.92).toDouble() else Math.pow(((v + 0.055f) / 1.055f).toDouble(), 2.4)
    fun lum(c: Color): Double = 0.2126 * channel(c.red) + 0.7152 * channel(c.green) + 0.0722 * channel(c.blue)
    val (l1, l2) = listOf(lum(a), lum(b)).sortedDescending()
    return (l1 + 0.05) / (l2 + 0.05)
}

/**
 * WindFace — la cara viva d'un vent de Gregal: una bola del color triat que
 * respira, parpelleja i reacciona a l'estat.
 *
 * La cara és personal: ulls, boca i forma es trien al creador (WindStyle) i es
 * desen amb el vent. Tot és Canvas; res d'imatges.
 */

/**
 * L'estat d'ànim del vent. IDLE, HAPPY i TALKING són de la conversa; la resta
 * mostra feina observada (la fase del directe o un pas en marxa) o una veu
 * proactiva per obrir, amb una insígnia a dalt a la dreta com al web
 * (`badges` a internal/web/app/winds.js).
 */
enum class WindMood { IDLE, THINKING, HAPPY, TALKING, SEARCHING, READING, WRITING, WORKING, DISCUSSING, DONE, FAILED, NEWS }

private val windActivityMoods = mapOf(
    "read" to WindMood.READING, "ls" to WindMood.READING, "list" to WindMood.READING, "web_fetch" to WindMood.READING,
    "office_read" to WindMood.READING, "image" to WindMood.READING, "review" to WindMood.READING, "reviewing" to WindMood.READING,
    "glob" to WindMood.SEARCHING, "grep" to WindMood.SEARCHING, "find" to WindMood.SEARCHING, "search" to WindMood.SEARCHING,
    "web_search" to WindMood.SEARCHING, "web" to WindMood.SEARCHING, "browser" to WindMood.SEARCHING,
    "write" to WindMood.WRITING, "edit" to WindMood.WRITING, "patch" to WindMood.WRITING, "office_edit" to WindMood.WRITING,
    "office_create" to WindMood.WRITING, "draft" to WindMood.WRITING, "revision" to WindMood.WRITING, "coding" to WindMood.WRITING,
    "plan" to WindMood.THINKING, "analyze" to WindMood.THINKING, "question" to WindMood.THINKING, "discussion" to WindMood.DISCUSSING,
)

/**
 * Mateixa taula que `moodOf` a winds.js: només estats observats. `voices`
 * compta les veus d'un torn de rotllana; la segona i següents responen les
 * anteriors, i això és una discussió.
 */
fun windMoodOf(phase: String? = null, writing: Boolean = false, voices: Int = 1, state: String? = null, activity: String? = null): WindMood = when {
    voices > 1 -> WindMood.DISCUSSING
    writing -> WindMood.WRITING
    !phase.isNullOrEmpty() -> when (phase) {
        "searching" -> WindMood.SEARCHING
        "scheduling", "working" -> WindMood.WORKING
        else -> WindMood.THINKING
    }
    state == "done" || state == "completed" -> WindMood.DONE
    state == "failed" -> WindMood.FAILED
    state == "working" || state == "started" -> windActivityMoods[activity] ?: WindMood.WORKING
    else -> WindMood.IDLE
}

/** Decorative motion can be disabled for accessibility and deterministic UI tests. */
val LocalWindMotion = compositionLocalOf { true }

/** A single motion gate for faces and shared indicators, including Android reduce-motion. */
@Composable
internal fun windMotionEnabled(): Boolean {
    val context = androidx.compose.ui.platform.LocalContext.current
    return LocalWindMotion.current && android.provider.Settings.Global.getFloat(
        context.contentResolver, android.provider.Settings.Global.ANIMATOR_DURATION_SCALE, 1f) > 0f
}

/**
 * WindStyle — les peces triables de la cara:
 *  - eyes: punts, badalls (ovals grossos), estrellats, mig tancats
 *  - mouth: somriure, oberta (parlant), neutra, llengua
 *  - shape: bola, núvol, espiral
 * Desat com a tres lletres al backend (p.ex. "pso" = punts/somriure/bola).
 * `kit` és l'especialitat d'un vent de colla ("shopping", "frontend"…): li dona
 * peus i l'eina del seu ofici. No es desa amb l'estil; ve de Wind.specialty.
 */
data class WindStyle(val eyes: WindEyes = WindEyes.DOTS, val mouth: WindMouth = WindMouth.SMILE, val shape: WindShape = WindShape.BALL, val kit: String = "")

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
    tappable: Boolean = false,
    /** Llista de vents: l'eina de l'ofici també en avatars petits, per triar sabent l'especialització. */
    showKit: Boolean = false,
) {
    val motion = windMotionEnabled()
    // Petó en prémer: la cara s'aixafa, torna amb molla i somriu amb els ulls.
    var punch by remember { mutableIntStateOf(0) }
    val punchScale = remember { Animatable(1f) }
    val cheer = remember { Animatable(0f) }
    LaunchedEffect(punch) {
        if (punch > 0 && motion) {
            cheer.snapTo(1f)
            punchScale.animateTo(0.84f, spring(stiffness = Spring.StiffnessMedium))
            punchScale.animateTo(1f, spring(dampingRatio = Spring.DampingRatioMediumBouncy, stiffness = Spring.StiffnessLow))
            cheer.animateTo(0f, tween(500))
        }
    }
    // Un sol rellotge de 12 s mou tota la cara (els bucles el divideixen, com
    // al web). Cada vent hi té el seu desfasament: una graella de mascotes no
    // respira, parpelleja ni salta a l'uníson.
    val loop: Float = if (motion) {
        val value by rememberInfiniteTransition(label = "wind-loop").animateFloat(
            initialValue = 0f, targetValue = 12f,
            animationSpec = infiniteRepeatable(tween(12000, easing = LinearEasing)), label = "loop")
        value
    } else 0f
    val seed = remember(accent, style) { ((accent.hashCode() * 31 + style.hashCode()) and 0xffff) / 65535f * 12f }
    val t = if (motion) (loop + seed) % 12f else 0f
    fun wave(period: Float) = sin(2f * PI.toFloat() * (t % period) / period)
    fun phase(period: Float) = t % period / period
    // No ambient bouncing, even on the happy creator/profile preview.
    val tilt = if (mood == WindMood.THINKING) wave(6f) else 0f
    // Boca que s'obre i tanca en parlar (unes 2,5 síl·labes per segon).
    val talkOpen = motion && mood == WindMood.TALKING && phase(.4f) < .5f
    // A hint of delight only while happy or talking.
    val twinkle = if (motion && (mood == WindMood.HAPPY || mood == WindMood.TALKING)) (wave(2f) + 1f) / 2f else 0f
    // Parpelleig curt cada 6 s; contents, de tant en tant fan els ulls feliços.
    val blinking = motion && phase(6f) in .955f..0.975f
    val squint = motion && mood == WindMood.HAPPY && phase(6f) in .30f..0.44f
    // Enllestit: un sol salt i la insígnia que apareix, cada cop que hi arriba.
    val doneHop = remember { Animatable(1f) }
    LaunchedEffect(mood, motion) {
        if (mood == WindMood.DONE && motion) { doneHop.snapTo(0f); doneHop.animateTo(1f, tween(800)) } else doneHop.snapTo(1f)
    }
    val lift = when (mood) {
        WindMood.DONE -> -sin(PI.toFloat() * doneHop.value) * 7f
        else -> 0f
    } / 80f
    val moodTilt = if (mood == WindMood.DISCUSSING) wave(3f) * 4f else 0f
    // On mira: cap a la insígnia, escombrant, llegint línies o la feina.
    val look: Pair<Float, Float> = when (mood) {
        WindMood.THINKING, WindMood.NEWS -> .8f to -.9f
        WindMood.SEARCHING -> wave(2.4f) to 0f
        WindMood.READING -> {
            val f = phase(3f)
            ((f % .5f / .4f).coerceAtMost(1f) * 2f - 1f) to (if (f < .5f) .3f else .9f)
        }
        WindMood.WRITING -> 0f to 1f
        WindMood.DISCUSSING -> wave(3f) to 0f
        else -> 0f to 0f
    }
    // Rest is nearly still: breathing and an occasional blink, not a wandering gaze.
    val breath = wave(4f)
    // The stored specialty stays intact; small avatars show only their identity…
    // unless the caller opts in (the crew strip): picking a wind by its trade tool.
    val kit = if (size >= 80 || showKit) windKits[style.kit] else null
    Box(Modifier.graphicsLayer {
        transformOrigin = TransformOrigin(0.5f, 1f)
        translationY = lift * size.dp.toPx()
        scaleX = punchScale.value * (1f - breath * 0.002f)
        scaleY = punchScale.value * (1f + breath * 0.004f)
        rotationZ = tilt * 5f + moodTilt
    }.then(if (tappable) Modifier.clickable { punch++ } else Modifier)) {
    Canvas(Modifier.size(size.dp)) {
        val w = this.size.width
        val h = this.size.height
        val cx = w / 2f
        // Keep the same silhouette/face placement with or without a specialty.
        val cy = h / 2f
        val r = min(w, h) * 0.37f
        val ink = if (contrastWith(accent, Color(0xFF202127)) >= contrastWith(accent, Color.White)) Color(0xFF202127) else Color.White
        val unit = min(w, h) / 80f

        when (style.shape) {
            WindShape.CLOUD -> {
                drawRoundRect(accent, Offset(cx - r * 1.19f, cy - r * .30f), Size(r * 2.38f, r * 1.27f),
                    cornerRadius = androidx.compose.ui.geometry.CornerRadius(r * .58f))
                drawCircle(accent, r * .67f, Offset(cx - r * .61f, cy - r * .05f))
                drawCircle(accent, r * .86f, Offset(cx + r * .08f, cy - r * .19f))
                drawCircle(accent, r * .60f, Offset(cx + r * .66f, cy + r * .04f))
            }
            WindShape.SPIRAL -> {
                // A curled gust with an open notch: recognizable from its outline,
                // not a circle with a faint spiral etched onto the forehead.
                val curl = Path().apply {
                    moveTo(-.95f, -.16f)
                    cubicTo(-1.10f, -.91f, -.14f, -1.27f, .43f, -.96f)
                    cubicTo(.93f, -.68f, .66f, -.20f, .19f, -.33f)
                    cubicTo(.55f, -.38f, .51f, -.73f, .17f, -.73f)
                    cubicTo(-.23f, -.73f, -.32f, -.43f, -.14f, -.23f)
                    cubicTo(.05f, -.09f, .53f, -.25f, .82f, .04f)
                    cubicTo(1.23f, .45f, .73f, 1.03f, .02f, 1.03f)
                    cubicTo(-.68f, 1.03f, -1.11f, .48f, -.95f, -.16f)
                    close()
                }
                withTransform({ translate(cx, cy); scale(r, r, Offset.Zero) }) { drawPath(curl, accent) }
            }
            WindShape.BALL -> drawCircle(accent, r, Offset(cx, cy))
        }
        // A single small tool on large profiles, never a costume over the eyes.
        // El 'head' del kit (auriculars, gorres...) es dibuixa com al web: en
        // coordenades de la cara sencera, cenyint a les seves vores. El 'prop'
        // es porta davant, petit i a la dreta.
        fun accessories() {
            if (kit == null) return
            if (kit.head.isNotEmpty()) {
                withTransform({ scale(unit, unit, Offset.Zero) }) { drawKit(kit.head) }
            }
            if (kit.prop.isNotEmpty() || kit.head.isEmpty()) {
                val shapes = kit.prop.ifEmpty { kit.head }
                withTransform({
                    scale(unit, unit, Offset.Zero)
                    translate(38f, 43f)
                    scale(.43f, .43f, Offset.Zero)
                }) { drawKit(shapes) }
            }
        }

        val eyeY = cy - r * 0.05f + look.second * r * 0.07f
        val eyeDx = r * 0.29f
        val eyeW = r * 0.135f
        val gazeShift = look.first * r * 0.07f

        // Two bold eye marks; no pupils, reflections, cheeks or glossy shading.
        if (mood == WindMood.DONE || blinking || squint || cheer.value > .15f) {
            for (side in listOf(-1f, 1f)) {
                drawArc(ink, 190f, 160f, false,
                    Offset(cx + side * eyeDx - eyeW, eyeY - eyeW * .55f),
                    Size(eyeW * 2f, eyeW * 1.1f),
                    style = Stroke(r * .085f, cap = StrokeCap.Round))
            }
        } else for (side in listOf(-1f, 1f)) {
            val ex = cx + side * eyeDx + gazeShift
            when (style.eyes) {
                WindEyes.DOTS, WindEyes.WIDE -> {
                    val wide = if (style.eyes == WindEyes.WIDE) 1.3f else 1f
                    drawRoundRect(ink, Offset(ex - eyeW * wide, eyeY - r * .27f),
                        Size(eyeW * 2f * wide, r * .54f),
                        cornerRadius = androidx.compose.ui.geometry.CornerRadius(eyeW * wide))
                }
                WindEyes.STARRY -> {
                    val rad = r * .23f
                    val star = Path().apply {
                        moveTo(ex, eyeY - rad)
                        lineTo(ex + rad * .3f, eyeY - rad * .3f)
                        lineTo(ex + rad, eyeY)
                        lineTo(ex + rad * .3f, eyeY + rad * .3f)
                        lineTo(ex, eyeY + rad)
                        lineTo(ex - rad * .3f, eyeY + rad * .3f)
                        lineTo(ex - rad, eyeY)
                        lineTo(ex - rad * .3f, eyeY - rad * .3f)
                        close()
                    }
                    drawPath(star, ink)
                }
                WindEyes.SLEEPY -> drawLine(ink, Offset(ex - eyeW, eyeY),
                    Offset(ex + eyeW, eyeY), r * .09f, StrokeCap.Round)
            }
        }

        // Boca: segons l'estil. La base és un arc cap AVALL (somriure).
        // Cal revisar-ho: boca plana, sense somriure que ho contradigui.
        when (if (mood == WindMood.FAILED) WindMouth.NEUTRAL else style.mouth) {
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
        // En parlar, la boca de l'estil s'obre i tanca: sembla que vocalitza.
        if (talkOpen) {
            drawOval(
                color = ink,
                topLeft = Offset(cx - r * 0.11f, cy + r * 0.16f),
                size = Size(r * 0.22f, r * 0.20f),
            )
            drawOval(
                color = Color(0xFFE86A6A),
                topLeft = Offset(cx - r * 0.07f, cy + r * 0.26f),
                size = Size(r * 0.14f, r * 0.10f),
            )
        }
        accessories()
        // Espurnes quan està content o explica coses.
        if (twinkle > 0f) {
            fun sparkle(sx: Float, sy: Float, rad: Float, alpha: Float) {
                val a = accent.copy(alpha = alpha.coerceIn(0f, 1f))
                drawLine(a, Offset(sx - rad, sy), Offset(sx + rad, sy), strokeWidth = r * 0.03f, cap = StrokeCap.Round)
                drawLine(a, Offset(sx, sy - rad), Offset(sx, sy + rad), strokeWidth = r * 0.03f, cap = StrokeCap.Round)
            }
            sparkle(cx + r * 1.05f, cy - r * 0.75f, r * 0.13f, twinkle)
            sparkle(cx - r * 1.10f, cy - r * 0.45f, r * 0.10f, 1f - twinkle)
        }
        withTransform({ scale(unit, unit, Offset.Zero) }) { drawBadge(mood, t, motion, doneHop.value) }
    }
    }
}

// --- Mascotes: l'eina de l'ofici de cada especialista de colla --------------
// Les mateixes formes que `kits` a internal/web/app/winds.js, amb la mateixa
// sintaxi de camins SVG i el mateix viewBox de 80×80. `head` va en coordenades
// de la cara (s'escala amb ella); `prop` es porta davant, amb `hands` a sobre.
private const val KitInk = 0xFF262040

internal class KitShape(val d: String, val fill: Long? = null, val stroke: Long? = null, val width: Float = 0f, val fillAlpha: Float = 1f)
internal class MascotKit(val head: List<KitShape> = emptyList(), val prop: List<KitShape> = emptyList(), val hands: List<Offset> = emptyList())

private fun line(d: String, width: Float = 2.2f, color: Long = KitInk) = KitShape(d, stroke = color, width = width)
private fun solid(d: String, fill: Long, width: Float = 2.2f) = KitShape(d, fill = fill, stroke = KitInk, width = width)
private fun flat(d: String, fill: Long) = KitShape(d, fill = fill)
private fun oval(cx: Float, cy: Float, rx: Float, ry: Float) = "M${cx - rx} ${cy}a$rx $ry 0 1 0 ${2 * rx} 0a$rx $ry 0 1 0 ${-2 * rx} 0Z"
private fun circle(cx: Float, cy: Float, r: Float) = oval(cx, cy, r, r)
private fun rect(x: Float, y: Float, w: Float, h: Float, rx: Float) =
    "M${x + rx} ${y}h${w - 2 * rx}a$rx $rx 0 0 1 $rx ${rx}v${h - 2 * rx}a$rx $rx 0 0 1 ${-rx} ${rx}h${-(w - 2 * rx)}a$rx $rx 0 0 1 ${-rx} ${-rx}v${-(h - 2 * rx)}a$rx $rx 0 0 1 $rx ${-rx}Z"

internal val windKits: Map<String, MascotKit> = mapOf(
    "shopping" to MascotKit(hands = listOf(Offset(53f, 59f)), prop = listOf(
        line("M57 54v-3a6 6 0 0 1 12 0v3", 2.4f),
        solid("M52 54H74L72 75H54Z", 0xFFE97A86),
        line("M53.5 60H72.5", 2.4f, 0xD9FFF1EC))),
    "books" to MascotKit(hands = listOf(Offset(17f, 67f), Offset(63f, 67f)), prop = listOf(
        solid("M17 61Q28 56 40 61Q52 56 63 61V75Q52 71 40 76Q28 71 17 75Z", 0xFF5E86D6),
        solid("M20 60Q30 56 40 60V73Q30 69 20 72Z", 0xFFFFF8EE, 1.6f),
        solid("M60 60Q50 56 40 60V73Q50 69 60 72Z", 0xFFFFF8EE, 1.6f),
        line("M24 63q6-2 12 0M24 67q6-2 12 0M44 63q6-2 12 0M44 67q6-2 12 0", 1.3f, 0xFFA79FBC))),
    "sports" to MascotKit(hands = listOf(Offset(53f, 64f)),
        head = listOf(solid("M11.5 24Q40 11 68.5 24L70 31Q40 19 10 31Z", 0xFFE5484D, 2f)),
        prop = listOf(
            solid(circle(63f, 62f, 11f), 0xFFFFFFFF),
            flat("M63 57.5l4.3 3.1-1.6 5h-5.4l-1.6-5Z", KitInk),
            line("M63 57.5V51.5M67.3 60.6l5.4-1.8M65.7 65.6l3.3 4.6M60.3 65.6l-3.3 4.6M58.7 60.6l-5.4-1.8", 1.5f))),
    "music" to MascotKit(head = listOf(
        line("M8.5 42A31.5 31.5 0 0 1 71.5 42", 5f, 0xFF585D7E),
        solid(rect(1f, 33f, 11f, 21f, 5f), 0xFFF28C6B),
        solid(rect(68f, 33f, 11f, 21f, 5f), 0xFFF28C6B))),
    "organisation" to MascotKit(hands = listOf(Offset(50f, 65f)), prop = listOf(
        solid(rect(50f, 49f, 24f, 24f, 4f), 0xFFFFFFFF),
        solid("M50 57V53a4 4 0 0 1 4-4h16a4 4 0 0 1 4 4v4Z", 0xFFE5484D),
        line("M57 46.5v5M67 46.5v5", 2.4f),
        line("M56 65l3.5 3.5 7-7", 2.8f, 0xFF2F9E6A))),
    "quality" to MascotKit(prop = listOf(
        line("M66 66L74 74", 6f, 0xFF7A5236),
        solid(circle(59f, 59f, 9.5f), 0xFFD7EEFA, 3f),
        line("M53.5 57a6 6 0 0 1 5-5", 2f, 0xFFFFFFFF))),
    "regression" to MascotKit(hands = listOf(Offset(48f, 63f)), prop = listOf(
        solid(rect(47f, 47f, 28f, 26f, 3.5f), 0xFFFFFFFF),
        line("M52 52V68H71", 1.6f),
        line("M54 66L70 53", 2.2f, 0xFFE5484D),
        line("M57 63h.01M60.5 64.5h.01M63 59.5h.01M66.5 58h.01M68 54.5h.01", 3.6f, 0xFF4C7BD9))),
    "ml" to MascotKit(hands = listOf(Offset(50f, 63f)),
        head = listOf(line("M41 8q0-4 5-6", 2.6f, 0xFF7A7F9C), solid(circle(48f, 2.5f, 3.6f), 0xFF8EE3B0, 2f)),
        prop = listOf(
            line("M55 49v-3M61 49v-3M67 49v-3M55 74v-3M61 74v-3M67 74v-3M72 54h3M72 60h3M72 66h3", 2f),
            solid(rect(50f, 49f, 22f, 22f, 4f), 0xFF5B6BB5),
            flat(rect(56f, 55f, 10f, 10f, 2f), 0xFFA9F0C5))),
    "timeseries" to MascotKit(hands = listOf(Offset(52f, 64f)), prop = listOf(
        solid(circle(63f, 61f, 11.5f), 0xFFFFFFFF),
        line("M55 64.5l3-3.5 3 2.5 4-6 3 3.5 2.5-2", 2f, 0xFF3FA39A),
        line("M63 61V53.5M63 61l4.5 3", 2f))),
    "research" to MascotKit(hands = listOf(Offset(52f, 64f)), prop = listOf(
        solid("M50 46h7v9l5 13a3 3 0 0 1-2.8 4H47.8A3 3 0 0 1 45 68l5-13Z", 0xFFD7EEFA, 2.2f),
        solid("M48 61h14l1.5 7a1.5 1.5 0 0 1-1.4 2h-14a1.5 1.5 0 0 1-1.4-2Z", 0xFF5E86D6),
        line("M51 64h3M57 64h3", 1.6f, 0xFFFFF8EE),
        solid(oval(53.5f, 69f, 1.6f, 2.2f), 0xFFFFF8EE),
        line("M52 46v-2.5a2 2 0 0 1 3 0V46", 1.6f, 0xFFA79FBC))),
    "tech" to MascotKit(hands = listOf(Offset(52f, 63f)), prop = listOf(
        solid(rect(50f, 50f, 23f, 23f, 4f), 0xFF3A3F5C, 2.2f),
        line("M56 56h11M56 60h11M56 64h6", 2f, 0xFF8EE3B0),
        line("M61.5 73v4M56 73v4M67 73v4", 1.8f, 0xFFA79FBC))),
    "review" to MascotKit(hands = listOf(Offset(52f, 63f)),
        head = listOf(
            KitShape(circle(29f, 36f, 8.5f), 0xFFFFFFFF, KitInk, 2.2f, .25f),
            KitShape(circle(51f, 36f, 8.5f), 0xFFFFFFFF, KitInk, 2.2f, .25f),
            line("M37.5 35q2.5-2 5 0M20.5 34.5l-6-2M59.5 34.5l6-2", 2f)),
        prop = listOf(
            solid(circle(63f, 61f, 11f), 0xFFF2C14E),
            solid("M63 61V50A11 11 0 0 1 73.5 64.5Z", 0xFF5E86D6, 1.6f),
            solid("M63 61L73.5 64.5A11 11 0 0 1 59 71.3Z", 0xFFE97A86, 1.6f))),
    "ux" to MascotKit(hands = listOf(Offset(52f, 65f)),
        head = listOf(line("M40 3.5l1.5-4", 2.8f, 0xFF6B4FB8), solid("M7 24C8 9 29 1 50 5C63 8 68 15 63 18C47 13 25 15 9 28Z", 0xFF8E6CD9)),
        prop = listOf(
            solid("M51 62c0-9 9-14 17-12 7 2 9 8 6 12-2 3-6 1-7 4-1 4 3 6 0 8-4 3-16 0-16-12Z", 0xFFF6E3C5),
            flat(circle(57.5f, 58f, 2.4f), 0xFFE5484D), flat(circle(63.5f, 55f, 2.4f), 0xFF5E86D6),
            flat(circle(69f, 58.5f, 2.2f), 0xFFF2C14E), flat(circle(57.5f, 65f, 2.2f), 0xFF2F9E6A))),
    "frontend" to MascotKit(prop = listOf(
        solid(rect(21f, 53f, 38f, 19f, 3f), 0xFF3A3F5C),
        line("M33 58.5l-4 4 4 4M47 58.5l4 4-4 4M42 57.5l-4 10", 2.2f, 0xFF8EE3B0),
        solid("M14 72H66L63 77H17Z", 0xFFC3C9D9))),
    "backend" to MascotKit(hands = listOf(Offset(51f, 63f)), prop = listOf(
        solid("M51 52V70A11 4 0 0 0 73 70V52", 0xFF7C8CE0),
        line("M51 58.5A11 4 0 0 0 73 58.5M51 64.5A11 4 0 0 0 73 64.5", 1.6f),
        solid(oval(62f, 52f, 11f, 4f), 0xFFB4BDF4))),
    "testing" to MascotKit(head = listOf(
        line("M35 6l1-4M39 8.5l4-2", 1.6f),
        flat(circle(37f, 9.5f, 3.8f), KitInk),
        solid(circle(30f, 15f, 8f), 0xFFE5484D, 2f),
        line("M36 10L24 20", 1.5f),
        line("M26.5 13h.01M33 18h.01M27.5 19.5h.01M31 11.5h.01", 3.2f))),
    "security" to MascotKit(hands = listOf(Offset(50f, 62f)), prop = listOf(
        solid("M62 47L74 51.5V60C74 67.5 69 72.5 62 75C55 72.5 50 67.5 50 60V51.5Z", 0xFF4F8FD9),
        flat(circle(62f, 58.5f, 3.2f), 0xFFFFFFFF),
        flat("M60.6 60.5h2.8l1 6.5h-4.8Z", 0xFFFFFFFF))),
    "delivery" to MascotKit(
        head = listOf(
            solid("M11 25C11 9 24 2 40 2S69 9 69 25Z", 0xFFF5C042),
            line("M40 3V24", 4f, 0xFFFFE29A),
            solid(rect(4f, 23f, 72f, 6f, 3f), 0xFFF5C042)),
        prop = listOf(
            line("M55 73L66 62", 8f),
            line("M55 73L66 62", 4.4f, 0xFFA7AFC2),
            solid("M71 50.5a7.5 7.5 0 1 0 5 7.5l-4.5 1.2-2.6-2.6 1-4.6Z", 0xFFA7AFC2))),
)

private val kitPaths = HashMap<String, Path>()

private fun DrawScope.drawKit(shapes: List<KitShape>) {
    for (shape in shapes) {
        val path = kitPaths.getOrPut(shape.d) { PathParser().parsePathString(shape.d).toPath() }
        shape.fill?.let { drawPath(path, Color(it), alpha = shape.fillAlpha) }
        shape.stroke?.let { drawPath(path, Color(it), style = Stroke(shape.width, cap = StrokeCap.Round, join = StrokeJoin.Round)) }
    }
}

/** El salt de «tinc una cosa per dir-te»: quiet, dos botets i quiet (cicle de 4 s). */
private fun hello(f: Float): Float = when {
    f < .70f -> 0f
    f < .78f -> (f - .70f) / .08f
    f < .86f -> 1f - (f - .78f) / .08f
    f < .92f -> (f - .86f) / .06f * .5f
    else -> .5f - (f - .92f) / .08f * .5f
}

// --- Insígnies d'estat: les mateixes formes que `badges` i `tools` a winds.js ---
private val bubble = solid(circle(67f, 12f, 10f), 0xFFFFFFFF, 2f)
private val badgeShapes: Map<WindMood, List<KitShape>> = mapOf(
    WindMood.THINKING to listOf(solid(circle(55f, 22f, 1.8f), 0xFFFFFFFF, 1.4f), bubble),
    WindMood.SEARCHING to listOf(bubble),
    WindMood.READING to listOf(bubble, solid("M60 9q3.5-2 7 0v8q-3.5-2-7 0Z", 0xFFFFF8EE, 1.5f), solid("M67 9q3.5-2 7 0v8q-3.5-2-7 0Z", 0xFFFFF8EE, 1.5f)),
    WindMood.WRITING to listOf(bubble, line("M61 17.5h7", 1.5f, 0xFFA79FBC)),
    WindMood.WORKING to listOf(bubble),
    WindMood.FAILED to listOf(solid(circle(67f, 12f, 9f), 0xFFF5B45D, 2f), line("M67 7.5v5.5M67 16.5h.01", 2.6f)),
    WindMood.NEWS to listOf(solid("M58 3h17a3.5 3.5 0 0 1 3.5 3.5v9a3.5 3.5 0 0 1-3.5 3.5h-10l-4 4v-4h-3a3.5 3.5 0 0 1-3.5-3.5v-9A3.5 3.5 0 0 1 58 3Z", 0xFFFFFFFF, 2f)),
)
private val magnifier = listOf(solid(circle(65.5f, 10.5f, 4f), 0xFFD7EEFA, 1.8f), line("M68.5 13.5l3.5 3.5", 2.2f))
private val flipPage = listOf(solid("M67 9q3.5-2 7 0v8q-3.5-2-7 0Z", 0xFFFFF8EE, 1.5f))
private val pencil = listOf(solid("M62.5 15.5l1-3.5 7.5-7.5 2.5 2.5-7.5 7.5Z", 0xFFF2C14E, 1.5f), flat("M62.5 15.5l1-3.5 2.5 2.5Z", KitInk))
private val gear = listOf(
    solid("M65.5 4.5h3l.5 2.6 2.2 1.3 2.5-1 1.5 2.6-2 1.7v2.6l2 1.7-1.5 2.6-2.5-1-2.2 1.3-.5 2.6h-3l-.5-2.6-2.2-1.3-2.5 1-1.5-2.6 2-1.7v-2.6l-2-1.7 1.5-2.6 2.5 1 2.2-1.3Z", 0xFFA7AFC2, 1.4f),
    solid(circle(67f, 12f, 2.4f), 0xFFFFFFFF, 1.4f))
private val doneMark = listOf(solid(circle(67f, 12f, 9.5f), 0xFF3FAE73, 2f), line("M62.5 12.2l3 3 6-6", 2.6f, 0xFFFFFFFF))
private val sayB = listOf(solid("M66 1.5h9a3 3 0 0 1 3 3v6a3 3 0 0 1-3 3h-1v3.5l-3.5-3.5H66a3 3 0 0 1-3-3v-6a3 3 0 0 1 3-3Z", 0xFFD9CCF5, 1.8f))
private val sayA = listOf(solid("M56 8h11a3 3 0 0 1 3 3v6a3 3 0 0 1-3 3h-6.5L57 23.5V20h-1a3 3 0 0 1-3-3v-6a3 3 0 0 1 3-3Z", 0xFFFFFFFF, 1.8f))
private val sayDots = listOf(line("M58 14h.01M61.5 14h.01M65 14h.01", 2.2f))
private val newsStar = listOf(solid("M66.5 4.5l1.7 4 4 1.7-4 1.7-1.7 4-1.7-4-4-1.7 4-1.7Z", 0xFFF2B53A, 1.3f))

/** Insígnia de l'estat a dalt a la dreta (espai de 80×80). Quieta sense moviment. */
private fun DrawScope.drawBadge(mood: WindMood, loop: Float, motion: Boolean, done: Float) {
    fun wave(period: Float) = sin(2f * PI.toFloat() * (loop % period) / period)
    badgeShapes[mood]?.let { drawKit(it) }
    when (mood) {
        WindMood.THINKING -> for (i in 0 until 3) {
            val f = ((loop - i * .2f) % 1.5f + 1.5f) % 1.5f / 1.5f
            val alpha = if (!motion) .85f else if (f < .3f) .3f + f / .3f * .7f else if (f < .6f) 1f - (f - .3f) / .3f * .7f else .3f
            drawCircle(Color(KitInk), 1.9f, Offset(62f + i * 5f, 12f), alpha = alpha)
        }
        WindMood.SEARCHING -> {
            val f = loop % 2.4f / 2.4f * 4f
            val corner = listOf(Offset(-1.5f, -1f), Offset(1.5f, -1f), Offset(1.5f, 1.5f), Offset(-1.5f, 1.5f))
            val a = corner[f.toInt() % 4]; val b = corner[(f.toInt() + 1) % 4]; val t = f % 1f
            withTransform({ translate(a.x + (b.x - a.x) * t, a.y + (b.y - a.y) * t) }) { drawKit(magnifier) }
        }
        WindMood.READING -> {
            val f = loop % 3f / 3f
            val flip = when { f < .4f -> 1f; f < .55f -> 1f - (f - .4f) / .15f * 2f; else -> -1f }
            withTransform({ scale(flip, 1f, Offset(67f, 13f)) }) { drawKit(flipPage) }
        }
        WindMood.WRITING -> withTransform({ translate(wave(.6f) * 1.5f, 0f); rotate(wave(.6f) * 6f, Offset(67f, 10f)) }) { drawKit(pencil) }
        WindMood.WORKING -> withTransform({ rotate(loop % 3f / 3f * 360f, Offset(67f, 12f)) }) { drawKit(gear) }
        WindMood.DISCUSSING -> {
            fun say(f: Float) = if (f < .22f) 1f + f / .22f * .14f else if (f < .5f) 1.14f - (f - .22f) / .28f * .14f else 1f
            val b = say((loop + 1f) % 2f / 2f); val a = say(loop % 2f / 2f)
            withTransform({ scale(b, b, Offset(70.5f, 9f)) }) { drawKit(sayB) }
            withTransform({ scale(a, a, Offset(61.5f, 15f)) }) { drawKit(sayA) }
            drawKit(sayDots)
        }
        WindMood.DONE -> {
            val pop = (done * 1.6f).coerceAtMost(1f)
            withTransform({ scale(pop, pop, Offset(67f, 12f)) }) { drawKit(doneMark) }
        }
        WindMood.NEWS -> {
            val t = (wave(2f) + 1f) / 2f
            withTransform({ rotate(t * 20f, Offset(66.5f, 10.2f)); scale(.85f + t * .25f, .85f + t * .25f, Offset(66.5f, 10.2f)) }) { drawKit(newsStar) }
        }
        else -> {}
    }
}
