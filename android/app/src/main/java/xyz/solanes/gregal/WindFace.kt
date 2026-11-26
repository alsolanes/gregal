package xyz.solanes.gregal

import androidx.compose.animation.core.*
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.RoundRect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.Paint
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.graphics.drawscope.CanvasDrawScope
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.LayoutDirection
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.clipPath
import androidx.compose.ui.graphics.drawscope.clipRect
import androidx.compose.ui.graphics.drawscope.withTransform
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.semantics.SemanticsPropertyKey
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import kotlin.math.PI
import kotlin.math.cos
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
 * (`badges` a internal/web/app/winds.js). WAITING és la marca del xat «t'espera»: el vent
 * té una pregunta per a tu.
 */
enum class WindMood { IDLE, THINKING, HAPPY, TALKING, SEARCHING, READING, WRITING, WORKING, DISCUSSING, WAITING, DONE, FAILED, NEWS }

private val windActivityMoods = mapOf(
    "read" to WindMood.READING, "read_file" to WindMood.READING, "ls" to WindMood.READING, "list" to WindMood.READING, "web_fetch" to WindMood.READING,
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
 * Desat com a tres lletres al backend (p.ex. "pso" = punts/somriure/bola),
 * i una quarta si la persona tria el color de la barretina del minairó.
 * `kit` és l'especialitat d'un vent de colla ("shopping", "frontend"…): li dona
 * peus i l'eina del seu ofici. No es desa amb l'estil; ve de Wind.specialty.
 */
data class WindStyle(val eyes: WindEyes = WindEyes.DOTS, val mouth: WindMouth = WindMouth.SMILE, val shape: WindShape = WindShape.BALL, val kit: String = "",
    /** The wind's own avatar pack (Wind.avatar) and the bearing it blows from (windDirection): they come from the wind, not the style code. */
    val pack: String = "", val bearing: Int = 0,
    /** windId: what seeds the Mapa de vent picture, so the same wind always gets the same one. */
    val windId: String = "",
    /** The barretina colour the person picked ([minairoBarretines]); empty picks one from the wind. */
    val barretina: String = "")

enum class WindEyes(val key: String) { DOTS("p"), WIDE("b"), STARRY("e"), SLEEPY("m") }
enum class WindMouth(val key: String) { SMILE("s"), OPEN("o"), NEUTRAL("n"), TONGUE("t") }
enum class WindShape(val key: String) { BALL("o"), CLOUD("n"), SPIRAL("s") }

/** Del codi de tres lletres del backend ("pso") a l'estil. */
fun windStyleOf(code: String?): WindStyle {
    val c = (code ?: "").ifBlank { "pso" }
    val eyes = WindEyes.entries.firstOrNull { it.key == c.getOrNull(0)?.toString() } ?: WindEyes.DOTS
    val mouth = WindMouth.entries.firstOrNull { it.key == c.getOrNull(1)?.toString() } ?: WindMouth.SMILE
    val shape = WindShape.entries.firstOrNull { it.key == c.getOrNull(2)?.toString() } ?: WindShape.BALL
    val barretina = c.getOrNull(3)?.toString()?.takeIf { letter -> minairoBarretines.any { it.first == letter } }.orEmpty()
    return WindStyle(eyes, mouth, shape, barretina = barretina)
}

/** De l'estil al codi que viatja al backend: tres lletres, i la de la barretina si n'hi ha. */
fun WindStyle.encode(): String = eyes.key + mouth.key + shape.key + barretina

/** The packs a wind can be drawn with; the account theme is one of them or "free" (task 045). */
internal val windPacks = listOf("cares", "esperit", "cap", "rosa", "mapa", "minairo", "pelut")

/** A pack's sample in a picker at [size]: a dial or a map fills its whole square, so it is drawn smaller to weigh as much as a figure. */
internal fun windPackSample(pack: String, size: Int): Int = if (pack == "rosa" || pack == "mapa") size * 5 / 6 else size

/** The account's avatar theme (`avatarTheme` of GET /api/winds); "free" draws each wind with its own pack. */
val LocalAvatarTheme = compositionLocalOf { "cares" }

internal fun avatarThemeOf(raw: String): String =
    org.json.JSONObject(raw).optString("avatarTheme").takeIf { it == "free" || it in windPacks } ?: "cares"

internal fun windPack(theme: String, style: WindStyle): String =
    if (theme != "free") theme else style.pack.takeIf { it in windPacks } ?: "cares"

/** "winds.avatarCares", "winds.avatarEsperit", "winds.avatarCap", "winds.avatarRosa", "winds.avatarMinairo", "winds.avatarPelut", "winds.avatarFree". */
internal fun avatarLabelKey(pack: String) = "winds.avatar" + pack.replaceFirstChar { it.uppercase() }

private val windRose = mapOf("tramuntana" to 0, "gregal" to 45, "llevant" to 90, "xaloc" to 135,
    "migjorn" to 180, "garbi" to 225, "ponent" to 270, "mestral" to 315)

/**
 * Compass bearing (0 = N) a wind blows from, as `direction` in winds.js: the
 * eight Catalan winds by name, case and accents aside; any other wind one of
 * the eight from its id (or, not saved yet, the slug its name will get).
 */
internal fun windDirection(id: String, name: String): Int {
    windRose[plainWindName(name)]?.let { return it }
    // winds.js keeps h * 31 + c as unsigned 32 bit: the low bits of String.hashCode.
    return (windId(id, name).hashCode() and 7) * 45
}

private fun plainWindName(name: String) = java.text.Normalizer.normalize(name, java.text.Normalizer.Form.NFD)
    .replace(Regex("[\u0300-\u036f]"), "").trim().lowercase()

/** `windId` in winds.js: the id, or for a wind not saved yet the slug its name will get. */
internal fun windId(id: String, name: String): String = id.ifEmpty { plainWindName(name).replace(Regex("[^a-z0-9]+"), "-").trim('-') }

/** The higher-contrast ink over the wind's colour, as the web picks it. */
internal fun windInk(accent: Color): Color {
    val dark = Color(0xFF202127)
    return if (contrastWith(accent, dark) >= contrastWith(accent, Color.White)) dark else Color.White
}

/** Aura del color del vent mentre treballa; a les llistes petites seria soroll. */
internal fun windShowsAura(mood: WindMood, size: Int): Boolean = size >= 64 && mood in windWorkMoods

internal val windWorkMoods = setOf(
    WindMood.THINKING, WindMood.SEARCHING, WindMood.READING,
    WindMood.WRITING, WindMood.WORKING, WindMood.DISCUSSING,
)

// Cares que bufen (`blow` in winds.js, same lines): while a Cares face works it
// blows downwind. Face units along +x from the rim, turned to the opposite of
// the wind's bearing; they reach 48 from the centre, the face box plus 10 %.
private val windBreathMoods = setOf(WindMood.THINKING, WindMood.SEARCHING, WindMood.READING, WindMood.WRITING, WindMood.WORKING)
private val windGusts = listOf("M35 -7C39 -9 43 -5 47 -7.5" to 3.2f, "M36 0C40 -2 44 2 48 0" to 3.2f, "M35 7C39 9 43 5 47 7.5" to 3.2f)
private val windPuffs = listOf("M36 -4C39 -5 42 -3 45 -4" to 5f, "M36 4C39 5 42 3 45 4" to 5f)

/** The three long lines, or the short, bolder pair a face under 28 dp can still show. */
internal fun windBreathLines(size: Int) = if (size < 28) windPuffs else windGusts
private val windBreathPaths by lazy { (windGusts + windPuffs).associate { (d, _) -> d to PathParser().parsePathString(d).toPath() } }

/**
 * The breath in face units around (0, 0). The long lines are dashes streaming
 * outward as [blow] runs 0..1; the gap is wider than the stroke so the round
 * caps don't close it. The short pair is solid, as in the mockup.
 */
private fun DrawScope.drawBreath(size: Int, accent: Color, blow: Float) {
    val brush = Brush.horizontalGradient(0f to lerp(accent, Color.White, .15f), 1f to accent.copy(alpha = 0f), startX = 34f, endX = 48f)
    val dash = if (size < 28) null else PathEffect.dashPathEffect(floatArrayOf(3f, 5f), (1f - blow) * 8f)
    for ((d, width) in windBreathLines(size)) drawPath(windBreathPaths.getValue(d), brush, style = Stroke(width, cap = StrokeCap.Round, pathEffect = dash))
}

/** [block] amb opacitat [alpha]: en passar d'un humor a l'altre, les marques es fonen en lloc de canviar de cop. */
private fun DrawScope.faded(alpha: Float, block: DrawScope.() -> Unit) {
    if (alpha <= .01f) return
    if (alpha >= .99f) return block()
    drawContext.canvas.saveLayer(Rect(-size.width, -size.height, 2f * size.width, 2f * size.height), Paint().apply { this.alpha = alpha })
    block()
    drawContext.canvas.restore()
}

/**
 * L'espurna de cada feina al voltant del cos (espai de 80, fora de la insígnia de
 * dalt a la dreta), del color del vent: pensant, bombolles que pugen; buscant, un
 * estel que li dona la volta; llegint, ratlles que passen; escrivint, gotes de
 * tinta; treballant (l'ordinador), guspires; discutint, ones cap a fora.
 */
internal fun DrawScope.drawWorkTrail(mood: WindMood, t: Float, accent: Color) {
    fun cycle(period: Float, shift: Float = 0f) = ((t + shift) % period + period) % period / period
    fun swell(f: Float) = sin(PI.toFloat() * f)
    when (mood) {
        WindMood.THINKING -> for (i in 0 until 3) {
            val f = cycle(2.4f, i * .8f)
            drawCircle(accent, 1.2f + 2f * f, Offset(13f - 4f * f, 30f - 24f * f), alpha = .75f * swell(f))
        }
        WindMood.SEARCHING -> for (i in 0 until 5) {
            val a = 2f * PI.toFloat() * (cycle(1.8f) - i * .035f)
            drawCircle(accent, 2.4f - i * .35f, Offset(40f + 38f * cos(a), 42f + 34f * sin(a)), alpha = .8f * (1f - i / 5f))
        }
        WindMood.READING -> for (i in 0 until 3) {
            val f = cycle(2.1f, i * .7f)
            val y = 70f - 36f * f
            drawLine(accent, Offset(1f, y), Offset(9f - 3f * (i % 2), y), strokeWidth = 1.8f, cap = StrokeCap.Round, alpha = .8f * swell(f))
        }
        WindMood.WRITING -> for (i in 0 until 3) {
            val f = cycle(1.2f, i * .4f)
            drawCircle(accent, 1.8f * (1f - .5f * f), Offset(63f + 3f * i, 64f + 14f * f * f), alpha = .85f * (1f - f))
        }
        WindMood.WORKING -> for (i in 0 until 5) {
            val f = cycle(.9f, i * .18f)
            val a = (200f + i * 34f) * PI.toFloat() / 180f
            val from = 34f + 10f * f
            drawLine(accent, Offset(40f + from * cos(a), 44f + from * sin(a)), Offset(40f + (from + 3.5f) * cos(a), 44f + (from + 3.5f) * sin(a)),
                strokeWidth = 2f, cap = StrokeCap.Round, alpha = .9f * (1f - f))
        }
        WindMood.DISCUSSING -> for (i in 0 until 2) {
            val f = cycle(1.6f, i * .8f)
            val r = 34f + 8f * f
            drawArc(accent, startAngle = 155f, sweepAngle = 50f, useCenter = false, topLeft = Offset(40f - r, 44f - r), size = Size(2f * r, 2f * r),
                alpha = .8f * (1f - f), style = Stroke(1.8f, cap = StrokeCap.Round))
        }
        else -> {}
    }
}

/**
 * Quan parpelleja (s dins el rellotge de 12 s): cada 2–4,5 s, a l'atzar però
 * sempre igual per al mateix vent. Un metrònom es nota; això sembla viu.
 */
internal fun windBlinkTimes(seed: Int): List<Float> {
    val rnd = kotlin.random.Random(seed)
    return generateSequence(.4f + rnd.nextFloat()) { it + 2f + rnd.nextFloat() * 2.5f }.takeWhile { it < 11.6f }.toList()
}

/** Dues mirades de reüll per volta: quan (s) i cap on, a l'atzar per vent. */
internal fun windGlances(seed: Int): List<Pair<Float, Offset>> {
    val rnd = kotlin.random.Random(seed xor 0x5eed)
    return listOf(1.5f, 7.5f).map { from ->
        from + rnd.nextFloat() * 3f to Offset(if (rnd.nextBoolean()) .75f else -.75f, if (rnd.nextInt(3) == 0) -.8f else .1f)
    }
}

/** On mira al temps [t]: davant, tret del segon llarg de cada mirada, que els ulls fan i desfan sense saltar. */
internal fun windGlanceAt(t: Float, glances: List<Pair<Float, Offset>>): Offset {
    val (from, to) = glances.firstOrNull { t - it.first in 0f..1.1f } ?: return Offset.Zero
    val d = t - from
    return to * smooth(min(d, 1.1f - d) / .16f)
}

/** Gir (graus) i batec (escala) de l'aura al temps `t` del rellotge de 12 s; sense moviment `t` és 0 i queda quieta. */
internal fun windAuraSway(t: Float): Pair<Float, Float> = t % 6f / 6f * 360f to 1f + sin(PI.toFloat() * t) * .06f

/** De quin vent (el seu color) és una cara: al xat d'un vent n'hi ha d'haver una de sola (109). */
internal val WindFaceKey = SemanticsPropertyKey<Color>("WindFace")

private fun smooth(x: Float): Float { val c = x.coerceIn(0f, 1f); return c * c * (3f - 2f * c) }

/**
 * A keyframe track like a CSS animation: [keys] are (fraction 0..1, value)
 * from 0, eased in and out between them, back to the first after the last.
 */
private fun track(f: Float, vararg keys: Pair<Float, Float>): Float {
    val i = keys.indexOfLast { it.first <= f }.coerceAtLeast(0)
    val (a, va) = keys[i]
    val (b, vb) = keys.getOrElse(i + 1) { 1f to keys[0].second }
    return if (b <= a) va else va + (vb - va) * smooth((f - a) / (b - a))
}

/**
 * Com està posat el vent, amb els números de `poses` a winds.js (110): el cos
 * es desplaça ([x], [y] en unitats de la caixa de 80, avall positiu),
 * s'inclina ([tilt], graus) i s'aixafa ([sx], [sy]); els ulls miren ([gx], [gy]);
 * la parpella de dalt baixa ([lid], 0..1 de l'ull) i es decanta ([brow], graus:
 * + decidit, − trist); la de baix puja en un arc content ([smile], 0..1).
 */
internal data class WindPose(
    val x: Float = 0f, val y: Float = 0f, val tilt: Float = 0f, val sx: Float = 1f, val sy: Float = 1f,
    val gx: Float = 0f, val gy: Float = 0f, val lid: Float = 0f, val brow: Float = 0f, val smile: Float = 0f,
)

internal fun windPoseBetween(a: WindPose, b: WindPose, f: Float): WindPose {
    fun mix(p: Float, q: Float) = p * (1f - f) + q * f
    return WindPose(mix(a.x, b.x), mix(a.y, b.y), mix(a.tilt, b.tilt), mix(a.sx, b.sx), mix(a.sy, b.sy),
        mix(a.gx, b.gx), mix(a.gy, b.gy), mix(a.lid, b.lid), mix(a.brow, b.brow), mix(a.smile, b.smile))
}

/**
 * La postura de cada estat amb la seva firma de moviment al temps [t] del
 * rellotge de 12 s, com `poses` i els bucles de winds.css: en repòs respira i
 * mira de reüll; pensant mira amunt i es decanta a poc a poc; buscant s'acosta
 * i escombra ràpid; llegint els ulls fan línies; escrivint bota una mica;
 * treballant s'aixafa i bota amb força, decidit; discutint es gronxa; esperant
 * s'inclina cap a tu; quan falla s'ensorra i sospira; acabat queda content;
 * amb novetats fa dos botets de «hola».
 */
internal fun windPose(mood: WindMood, t: Float, glances: List<Pair<Float, Offset>> = emptyList()): WindPose {
    fun at(period: Float) = t % period / period
    // wf-breathe: every calm face, and the figure under the other loops too.
    val breathe = track(at(4f), 0f to 0f, .5f to 1f)
    val calm = WindPose(sx = 1f - .01f * breathe, sy = 1f + .025f * breathe)
    val pose = when (mood) {
        WindMood.IDLE, WindMood.TALKING -> windGlanceAt(t, glances).let { g -> calm.copy(gx = g.x * 4f, gy = g.y * 1.2f, lid = .07f) }
        // Android's own «content» (profiles, games): idle with a little smile.
        WindMood.HAPPY -> windGlanceAt(t, glances).let { g -> calm.copy(gx = g.x * 4f, gy = g.y * 1.2f, lid = .07f, smile = .15f) }
        WindMood.THINKING -> {
            val f = at(6f)
            calm.copy(y = -.5f + track(f, 0f to 0f, .5f to -.6f), tilt = -5f + track(f, 0f to -2.5f, .5f to 2f),
                gx = 2.4f + track(f, 0f to 0f, .3f to -2.4f, .45f to -2.4f, .7f to .6f, .85f to .6f),
                gy = -3.2f + track(f, 0f to 0f, .3f to .4f, .45f to .4f, .7f to -.6f, .85f to -.6f))
        }
        WindMood.SEARCHING -> {
            val peer = track(at(2.4f), 0f to 0f, .5f to 1f)
            calm.copy(x = 1.5f + 1.2f * peer, tilt = 4f + 2f * peer, gy = -.6f, lid = .22f, brow = 8f,
                gx = track(at(1.2f), 0f to -3f, .45f to 3f, .55f to 3f))
        }
        WindMood.READING -> {
            val f = at(3f)
            calm.copy(x = .5f, y = 1f, tilt = 3f, lid = .32f,
                gx = track(f, 0f to -3f, .4f to 3f, .5f to -3f, .9f to 3f),
                gy = 2.2f + track(f, 0f to 1f, .4f to 1f, .5f to 3f, .9f to 3f))
        }
        WindMood.WRITING -> calm.copy(x = .5f, y = .5f + track(at(1.2f), 0f to 0f, .5f to -1.5f), tilt = 2f,
            gx = -.6f, gy = 2.6f, lid = .26f, brow = 5f, smile = .1f)
        WindMood.WORKING -> {
            // wf-busy: a hard squash-bob, its own scale instead of the breath.
            val f = at(.6f)
            WindPose(y = .8f + track(f, 0f to 0f, .4f to -3.2f, .8f to 0f),
                sx = 1.04f * track(f, 0f to 1.03f, .4f to .98f, .8f to 1.04f),
                sy = .96f * track(f, 0f to .97f, .4f to 1.03f, .8f to .95f),
                gy = .6f, lid = .38f, brow = 14f)
        }
        WindMood.DISCUSSING -> at(3f).let { f ->
            calm.copy(tilt = -3f + track(f, 0f to -4f, .5f to 4f), gx = track(f, 0f to -2.5f, .5f to 2.5f), lid = .08f, smile = .15f)
        }
        WindMood.WAITING -> {
            val lean = track(at(4f), 0f to 0f, .5f to 1f)
            calm.copy(x = -1.5f, y = -.8f * lean, tilt = -7f - 2.5f * lean, gx = -1.8f, gy = 1.6f, smile = .12f)
        }
        WindMood.DONE -> calm.copy(gy = -1f, smile = .5f)
        WindMood.FAILED -> {
            val f = at(6f)
            WindPose(y = 2.4f, tilt = 5f, sx = 1.05f * track(f, 0f to 1f, .4f to 1.02f, .55f to .99f),
                sy = .92f * track(f, 0f to 1f, .4f to .97f, .55f to 1.015f), gx = -.6f, gy = 2.6f, lid = .42f, brow = -16f)
        }
        WindMood.NEWS -> calm.copy(x = .5f, y = -.5f - 3f * hello(at(4f)), tilt = -4f, gx = 2f, gy = -1.6f, smile = .22f)
    }
    // Treballant ja bota fort (wf-busy); les altres feines no paren mai del tot:
    // suren i es gronxen a poc a poc, amb uns períodes que no casen amb cap altre
    // bucle (ni amb la cara de la capçalera).
    if (mood !in windWorkMoods || mood == WindMood.WORKING) return pose
    return pose.copy(y = pose.y + 1.1f * sin(2f * PI.toFloat() * t / 2.3f), tilt = pose.tilt + 1.6f * sin(2f * PI.toFloat() * t / 3.7f))
}

/**
 * Quant pesa cada humor mentre la cara passa d'un a l'altre ([f] 0..1 del pas en
 * marxa). Canviar a mig camí parteix dels pesos d'aquell moment: la cara no salta.
 */
internal class WindMoodBlend(mood: WindMood) {
    private val from = FloatArray(WindMood.entries.size).also { it[mood.ordinal] = 1f }
    var to = mood
        private set

    fun weight(m: WindMood, f: Float): Float = from[m.ordinal] * (1f - f) + if (m == to) f else 0f

    fun retarget(mood: WindMood, f: Float) {
        for (m in WindMood.entries) from[m.ordinal] = weight(m, f)
        to = mood
    }
}

/** The feet stepping while it works or writes (wf-step, .75 s, the right half a beat later); 0 otherwise. */
internal fun windStep(mood: WindMood, t: Float, right: Boolean): Float =
    if (mood != WindMood.WORKING && mood != WindMood.WRITING) 0f
    else track((t + if (right) .375f else 0f) % .75f / .75f, 0f to 0f, .25f to -2.4f, .5f to 0f)

/**
 * El salt d'acabar (f 0..1 en 1,1 s, wf-hop): s'ajup i s'eixampla, salta
 * estirat, toca a terra aixafat i s'assenta amb un rebot. y (avall positiu,
 * unitats de 80), escala x i escala y.
 */
internal fun windDoneHop(f: Float): Triple<Float, Float, Float> {
    val y = track(f, 0f to 0f, .16f to 0f, .4f to -9f, .6f to 0f, .72f to -1.5f, .86f to 0f, 1f to 0f)
    val sx = track(f, 0f to 1f, .16f to 1.12f, .4f to .92f, .6f to 1.1f, .72f to .97f, .86f to 1.02f, 1f to 1f)
    val sy = track(f, 0f to 1f, .16f to .84f, .4f to 1.1f, .6f to .88f, .72f to 1.03f, .86f to .98f, 1f to 1f)
    return Triple(y, sx, sy)
}

@Composable
fun WindFace(
    size: Int = 96,
    mood: WindMood = WindMood.IDLE,
    accent: Color = Color(0xFFFF9F00),
    style: WindStyle = WindStyle(),
    tappable: Boolean = false,
    /** Llista de vents: l'eina de l'ofici també en avatars petits, per triar sabent l'especialització. */
    showKit: Boolean = false,
    modifier: Modifier = Modifier,
    /** El toc, a més del bellugueig de [tappable]: el vent del xat obre el seu perfil. */
    onClick: (() -> Unit)? = null,
    /** A large face (the chat's, a profile, the home strip) is the clay wind (114) when its sheet is in the APK. */
    clay: Boolean = false,
    /** False for the chat's corner wind, which shakes its head on a failed turn instead (WindPresence), as on the web. */
    shrugs: Boolean = true,
) {
    val compact = size < 64
    val motionAllowed = windMotionEnabled()
    val motion = (!compact || tappable) && motionAllowed
    // Celebrar o arronsar-se és un sol gest en acabar el torn: també als avatars petits.
    val cued = modifier.semantics { set(WindFaceKey, accent) }.then(windTurnCue(mood, accent, shrugs))
    val packCued = if (onClick != null) cued.clickable(onClick = onClick) else cued
    val pack = windPack(LocalAvatarTheme.current, style)
    when (pack) {
        "esperit" -> return EsperitFace(size, mood, accent, style.bearing, motion, packCued)
        "cap" -> return CapFace(size, mood, accent, style.bearing, motion, packCued)
        "rosa" -> return RosaFace(size, mood, accent, style.bearing, motion, packCued)
        "mapa" -> return MapFace(size, mood, accent, style, motion, packCued)
    }
    // A pelut is its clay sheets at every size, without a trade kit (155); without them, the Cares face.
    val pelut = pack == "pelut"
    if (pelut && WindClayFace(size, mood, accent, accent, "pelut", motion, packCued)) return
    // The minairó is a Cares figure in costume: the same eyes, poses and springs, its own body.
    val minairo = pack == "minairo"
    // A face that wears its trade is clay too, the trade's icon on a badge (159); without the sheets, the Canvas figure with its kit. The minairó wears none and has its own sheets (119).
    val wearsKit = !minairo && (size >= 80 || showKit) && windKits[style.kit] != null
    if (clay && !pelut && WindClayFace(size, mood, accent, minairoLook(style, accent).cap,
            if (minairo) "minairo" else style.shape.key, motion, packCued, badge = if (wearsKit) windKitIcon(style.kit) else null)) return
    // La forma reacciona quan el dit toca la cara i torna amb molla en deixar-la.
    val touch = if (tappable && motion) remember { MutableInteractionSource() } else null
    val pressScale = if (touch != null) {
        val pressed by touch.collectIsPressedAsState()
        animateFloatAsState(
            targetValue = if (pressed) .91f else 1f,
            animationSpec = if (pressed) tween(75) else spring(dampingRatio = Spring.DampingRatioMediumBouncy),
            label = "wind-press",
        )
    } else null
    var punch by remember { mutableIntStateOf(0) }
    val cheer = if (touch != null) remember { Animatable(0f) } else null
    if (cheer != null) LaunchedEffect(punch) {
        if (punch > 0) {
            cheer.snapTo(1f)
            cheer.animateTo(0f, tween(500))
        }
    }
    // Un sol rellotge de 12 s mou tota la cara (els bucles el divideixen, com
    // al web). Cada vent hi té el seu desfasament: una graella de mascotes no
    // respira, parpelleja ni salta a l'uníson.
    val loop = if (motion) {
        rememberInfiniteTransition(label = "wind-loop").animateFloat(
            initialValue = 0f, targetValue = 12f,
            animationSpec = infiniteRepeatable(tween(12000, easing = LinearEasing)), label = "loop")
    } else null
    // Like the rest of Cares, the breath is shown but still in list faces and
    // when motion is off.
    val breathing = mood in windBreathMoods
    val blow = if (breathing && motion) {
        rememberInfiniteTransition(label = "wind-breath").animateFloat(
            initialValue = 0f, targetValue = 1f,
            animationSpec = infiniteRepeatable(tween(750, easing = LinearEasing)), label = "blow")
    } else null
    val seed = remember(accent, style) { ((accent.hashCode() * 31 + style.hashCode()) and 0xffff) / 65535f * 12f }
    val blinks = remember(seed) { windBlinkTimes(seed.toBits()) }
    val glances = remember(seed) { windGlances(seed.toBits()) }
    fun time() = loop?.let { (it.value + seed) % 12f } ?: 0f
    val doneHop = if (motion && mood == WindMood.DONE) {
        val hop = remember { Animatable(0f) }
        LaunchedEffect(hop) { hop.animateTo(1f, tween(1100, easing = LinearEasing)) }
        hop
    } else null
    // D'un estat a l'altre la postura, la insígnia i l'espurna passen en 300 ms
    // amb sortida suau, sense saltar ni reiniciar els bucles (el rellotge és el
    // mateix). Els pesos no són estat de Compose: els llegeix només el dibuix.
    val moodMix = remember { Animatable(1f) }
    val blend = remember { WindMoodBlend(mood) }
    LaunchedEffect(mood) {
        if (blend.to == mood) return@LaunchedEffect
        blend.retarget(mood, moodMix.value)
        moodMix.snapTo(0f)
        moodMix.animateTo(1f, tween(300, easing = FastOutSlowInEasing))
    }
    fun fade(m: WindMood) = if (!motion) (if (m == mood) 1f else 0f) else blend.weight(m, moodMix.value)
    fun pose(t: Float): WindPose {
        if (!motion) return windPose(mood, 0f)
        var mixed: WindPose? = null
        var total = 0f
        for (m in WindMood.entries) {
            val w = fade(m).takeIf { it > .001f } ?: continue
            total += w
            val p = windPose(m, t, glances)
            mixed = mixed?.let { windPoseBetween(it, p, w / total) } ?: p
        }
        return mixed ?: windPose(mood, t, glances)
    }
    // The large figure (profile, creator, a crew strip with tools) wears its kit
    // and is drawn a little smaller to make room for it, as `full` on the web;
    // the stored specialty stays intact when a small avatar does not show it.
    val full = size >= 80 || showKit
    // Paral·laxi de volum només a les cares grans (perfil, creador, xat buit):
    // a les llistes petites seria soroll i sensor per a res.
    val lean = rememberWindLean(motion && !compact)
    val bodyPaint = remember(style.shape, accent, minairo) { WindBodyPaint(if (minairo) WindShape.BALL else style.shape, accent) }
    val look = remember(style, accent) { minairoLook(style, accent) }
    // L'aura entra i surt amb un fos quan el vent comença o acaba la feina.
    val aura by animateFloatAsState(if (windShowsAura(mood, size)) 1f else 0f, tween(450), label = "wind-aura")
    val motionLayer = if (motion) Modifier.graphicsLayer {
        val p = pose(time())
        val (hop, hopX, hopY) = doneHop?.let { windDoneHop(it.value) } ?: Triple(0f, 1f, 1f)
        val scale = pressScale?.value ?: 1f
        val k = size.dp.toPx() / 80f
        transformOrigin = TransformOrigin(0.5f, 1f)
        translationX = p.x * k
        translationY = (p.y + hop) * k
        scaleX = (1f + (1f - scale) * .5f) * p.sx * hopX
        scaleY = scale * p.sy * hopY
        // En tocar-la, un bellugueig curt que s'apaga: «sí, sóc aquí».
        val c = cheer?.value ?: 0f
        rotationZ = sin((1f - c) * 3f * PI.toFloat()) * c * 9f + p.tilt
    } else Modifier
    val tap = if (touch != null) Modifier.clickable(interactionSource = touch, indication = null) { punch++; onClick?.invoke() }
        else if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier
    Canvas(cued.then(tap).then(motionLayer).size(size.dp)) {
        with(WindFigure) {
            draw(time(), size, mood, accent, style, minairo, full, motion, ::pose, blinks, aura, lean.value, cheer?.value, doneHop?.value, blow?.value, look, bodyPaint, ::fade)
        }
    }
}

/** The Cares figure (or the minairó) at one instant [t]: the live face and the still avatar of a notification draw it the same way. */
internal object WindFigure {
    private fun wave(time: Float, period: Float) = sin(2f * PI.toFloat() * (time % period) / period)

    fun DrawScope.draw(
        t: Float, size: Int, mood: WindMood, accent: Color, style: WindStyle, minairo: Boolean, full: Boolean, motion: Boolean,
        pose: (Float) -> WindPose, blinks: List<Float>, aura: Float, lean: Offset,
        cheer: Float?, doneHop: Float?, blow: Float?, look: MinairoLook, bodyPaint: WindBodyPaint,
        /** Quant es veu cada humor mentre passa d'un a l'altre. */
        fade: (WindMood) -> Float = { if (it == mood) 1f else 0f },
    ) {
        val kit = if (full && !minairo) windKits[style.kit] else null
        val ink = windInk(accent)
        // Eines i insígnies es dibuixen amb la tinta del mateix vent, no amb un lila fix.
        val tone = lerp(accent, Color(0xFF1B1530), .8f)
        val handEdge = lerp(accent, Color(0xFF1B1530), .45f)
        val breath = windBreathMoods.sumOf { fade(it).toDouble() }.toFloat()
        fun phase(period: Float) = t % period / period
        val talkOpen = motion && mood == WindMood.TALKING && phase(.4f) < .5f
        val twinkle = if (motion && (mood == WindMood.HAPPY || mood == WindMood.TALKING)) (wave(t, 2f) + 1f) / 2f else 0f
        // A blink closes and opens the lids in 160 ms instead of cutting to closed.
        val blink = if (!motion) 0f else blinks.maxOfOrNull { b -> (t - b).let { if (it in 0f..0.16f) sin(PI.toFloat() * it / .16f) else 0f } } ?: 0f
        val p = pose(t)
        val w = this.size.width
        val h = this.size.height
        val unit = min(w, h) / 80f
        // The web's 80 box: the head is a face of R 33 centred at (40, 40.5);
        // the large figure draws it at .9 from (4, 3), a small gust at .95 from
        // (2, 3.6) to fit its curl. [u] turns head units into pixels.
        val (headScale, headX, headY) = when {
            full -> Triple(.9f, 4f, 3f)
            style.shape == WindShape.SPIRAL -> Triple(.95f, 2f, 3.6f)
            else -> Triple(1f, 0f, 0f)
        }
        val u = unit * headScale
        val cx = (headX + 40f * headScale) * unit
        val cy = (headY + 40.5f * headScale) * unit
        val r = 33f * u
        fun hx(x: Float) = cx + (x - 40f) * u
        fun hy(y: Float) = cy + (y - 40.5f) * u
        // Una espurna per feina, del color del vent: només a les cares grans que es mouen.
        fun trails() {
            if (motion && size >= 64) withTransform({ scale(unit, unit, Offset.Zero) }) {
                for (m in windWorkMoods) faded(fade(m)) { drawWorkTrail(m, t, accent) }
            }
        }
        // La insígnia de cada humor que es veu, amb el seu pes.
        fun badges(skip: WindMood?) {
            for (m in WindMood.entries) if (m != skip) faded(fade(m)) { drawBadge(m, t, motion, doneHop ?: 1f, tone) }
        }

        // Inclinació del mòbil (−1..1), llegida aquí perquè només redibuixi.
        val (tx, ty) = lean
        val paint = if (tx == 0f && ty == 0f) bodyPaint else WindBodyPaint(if (minairo) WindShape.BALL else style.shape, accent, tx, ty)
        if (aura > 0f) {
            // Darrere el cos: un halo suau i un anell que gira, tots dos del color del vent.
            val (spin, pulse) = windAuraSway(t)
            val reach = min(w, h) / 2f * pulse.coerceAtMost(1f)
            drawCircle(Brush.radialGradient(
                0f to accent.copy(alpha = .38f * aura), .7f to accent.copy(alpha = .16f * aura), 1f to Color.Transparent,
                center = Offset(cx, cy), radius = reach), reach, Offset(cx, cy))
            val ring = r * 1.16f * pulse
            withTransform({ rotate(spin, Offset(cx, cy)) }) {
                drawCircle(Brush.sweepGradient(
                    listOf(Color.Transparent, accent.copy(alpha = .7f * aura), Color.Transparent), Offset(cx, cy)),
                    ring, Offset(cx, cy), style = Stroke(r * .07f, cap = StrokeCap.Round))
            }
        }
        if (minairo) {
            val cheered = (((cheer ?: 0f) - .15f) * 4f).coerceIn(0f, 1f)
            val eyePose = p.copy(lid = maxOf(blink, p.lid), smile = maxOf(p.smile, cheered * .5f))
            // The cap and the beard trail the figure: how far it moved in the last tenth of a second, the hop included.
            val hop = doneHop
            fun lift(at: Float) = if (hop == null) 0f else windDoneHop(at.coerceIn(0f, 1f)).first
            val moved = if (!motion) 0f else p.y - pose(t - .1f).y + lift(hop ?: 0f) - lift((hop ?: 0f) - .09f)
            withTransform({ scale(unit, unit, Offset.Zero) }) {
                drawMinairo(look, style.eyes, eyePose, mood, t, motion, moved, size, accent, ink, paint, tx, ty, talkOpen)
                // The badge goes to the corner the barretina does not fold over.
                withTransform({ translate(if (look.fold == 1) -54f else 0f, 0f) }) { badges(null) }
            }
            trails()
            return
        }
        drawWindBody(style.shape, paint, cx, cy, r, if (motion) .5f + wave(t, 6f) * 5.5f else 0f,
            ground = full, stepL = windStep(mood, t, false) / 33f, stepR = windStep(mood, t, true) / 33f)
        // A single small tool on large profiles, never a costume over the eyes.
        // El 'head' del kit (auriculars, gorres...) es dibuixa com al web, en
        // coordenades del cap; així cenyeix les seves vores i les ulleres
        // emmarquen els ulls. El 'prop' es porta davant, petit i a la dreta.
        fun accessories() {
            if (kit == null) return
            if (kit.head.isNotEmpty()) {
                withTransform({ scale(unit, unit, Offset.Zero); translate(headX, headY); scale(headScale, headScale, Offset.Zero) }) { drawKit(kit.head, tone) }
            }
            if (kit.prop.isNotEmpty() || kit.head.isEmpty()) {
                val shapes = kit.prop.ifEmpty { kit.head }
                withTransform({
                    scale(unit, unit, Offset.Zero)
                    translate(20f, 20f)
                    scale(.75f, .75f, Offset.Zero)
                }) {
                    drawKit(shapes, tone)
                    for (hand in kit.hands) {
                        drawOval(accent, Offset(hand.x - 4.6f, hand.y - 4f), Size(9.2f, 8f))
                        drawOval(handEdge, Offset(hand.x - 4.6f, hand.y - 4f), Size(9.2f, 8f), style = Stroke(1.4f))
                    }
                }
            }
        }

        // Soft cheeks: a blush that melts into the body on every wind colour, and on the large figure a touch of light on them.
        for (x in listOf(20.6f, 59.4f)) withTransform({ translate(hx(x), hy(48.8f)); scale(7f * u, 4.6f * u, Offset.Zero) }) {
            drawCircle(windBlush, 1f, Offset.Zero)
        }
        if (full) for (x in listOf(19.1f, 57.9f)) drawOval(Color.White, Offset(hx(x) - 2.6f * u, hy(46.8f) - 1.2f * u), Size(5.2f * u, 2.4f * u), alpha = .35f)

        // Els ulls porten l'expressió. Mouen tot l'ull cap on mira (també amb la
        // inclinació del mòbil); en tocar-la somriuen amb els ulls.
        val cheered = (((cheer ?: 0f) - .15f) * 4f).coerceIn(0f, 1f)
        val eyePose = p.copy(lid = maxOf(blink, p.lid), smile = maxOf(p.smile, cheered * .5f))
        val small = size < 28
        val gx = p.gx + tx * 2.4f
        val gy = p.gy + ty * 1.8f
        for (right in listOf(false, true)) {
            drawWindEye(style.eyes, eyePose, right, hx(if (right) 49.6f else 30.4f) + gx * u, hy(38.4f) + gy * u, u, small, full, ink, accent, paint, cx, cy, r)
        }

        // Boca: petita sota els ulls, que són els que parlen (els camins de winds.js).
        // Thin features vanish in a list avatar: the small face draws them bolder.
        val line = Stroke((if (full) 2.5f else 3.1f) * u, cap = StrokeCap.Round)
        fun smile() = drawPath(Path().apply { moveTo(hx(31.8f), hy(52.8f)); quadraticTo(hx(40f), hy(62.8f), hx(48.2f), hy(52.8f)) }, ink, style = line)
        if (mood == WindMood.FAILED) drawPath(Path().apply {
            // A worried wave instead of the mouth.
            moveTo(hx(32.5f), hy(55.4f)); quadraticTo(hx(36.25f), hy(52f), hx(40f), hy(55.4f)); quadraticTo(hx(43.75f), hy(58.8f), hx(47.5f), hy(55.4f))
        }, ink, style = line)
        else when (style.mouth) {
            WindMouth.SMILE -> smile()
            WindMouth.OPEN -> {
                // Boca oberta: llavor fosca amb una mica de llengua.
                drawOval(ink, Offset(hx(35.2f), hy(49.4f)), Size(9.6f * u, 10.4f * u))
                drawOval(WindTongue, Offset(hx(36.9f), hy(55.2f)), Size(6.2f * u, 4.4f * u))
            }
            WindMouth.NEUTRAL -> drawLine(ink, Offset(hx(34.6f), hy(54f)), Offset(hx(45.4f), hy(54f)), strokeWidth = line.width, cap = StrokeCap.Round)
            WindMouth.TONGUE -> {
                // La llengua penja del somriure, que es dibuixa a sobre.
                drawPath(Path().apply {
                    moveTo(hx(38.6f), hy(56.8f)); quadraticTo(hx(38.2f), hy(63.4f), hx(42f), hy(63.4f)); quadraticTo(hx(45.8f), hy(63.4f), hx(45.6f), hy(56f)); close()
                }, WindTongue)
                smile()
            }
        }
        // En parlar, la boca de l'estil s'obre i tanca: sembla que vocalitza.
        if (talkOpen) {
            drawOval(ink, Offset(hx(36.4f), hy(50.8f)), Size(7.2f * u, 7.2f * u))
            drawOval(WindTongue, Offset(hx(37.8f), hy(55f)), Size(4.4f * u, 3f * u))
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
        if (breath > 0f) withTransform({ scale(unit, unit, Offset.Zero); translate(40f, 40f); rotate(style.bearing + 90f, Offset.Zero) }) {
            faded(breath) { drawBreath(size, accent, blow ?: 0f) }
        }
        // In Cares the breath replaces the working cog; the other marks stay.
        withTransform({ scale(unit, unit, Offset.Zero) }) { badges(skip = WindMood.WORKING) }
        trails()
    }
}

/** A wind's still face as a bitmap, for a notification's large icon: drawn without a window, as a list avatar. */
internal fun windFaceBitmap(wind: Wind, theme: String, px: Int = 128): android.graphics.Bitmap {
    val accent = Color(wind.argb)
    val style = wind.faceStyle
    val pack = windPack(theme, style)
    val minairo = pack == "minairo"
    val image = ImageBitmap(px, px)
    val ball = WindBodyPaint(WindShape.BALL, accent)
    CanvasDrawScope().draw(Density(1f), LayoutDirection.Ltr, androidx.compose.ui.graphics.Canvas(image), Size(px.toFloat(), px.toFloat())) {
        when (pack) {
            "esperit" -> with(EsperitFigure) { draw(0f, false, WindMood.IDLE, accent, style.bearing, Offset.Zero, ball) }
            "cap" -> with(CapFigure) { draw(0f, false, 48, WindMood.IDLE, accent, style.bearing, Offset.Zero, ball) }
            "rosa" -> with(RosaFigure) { draw(0f, false, 48, WindMood.IDLE, accent, style.bearing, Offset.Zero, ball) }
            "mapa" -> with(MapFigure) { draw(0f, false, WindMood.IDLE, MapPicture(style, px, accent).image, lerp(accent, Color(0xFF1B1530), .8f)) }
            else -> with(WindFigure) {
                draw(0f, 48, WindMood.IDLE, accent, style, minairo, false, false, { windPose(WindMood.IDLE, 0f) }, emptyList(), 0f, Offset.Zero,
                    null, null, null, minairoLook(style, accent), WindBodyPaint(if (minairo) WindShape.BALL else style.shape, accent))
            }
        }
    }
    return image.asAndroidBitmap()
}

/** Eye size (rx, ry) and how much lower it sits, in head units, per eye style: tall, big, star shine, calm and low (`face` in winds.js). */
private fun windEyeSize(eyes: WindEyes) = when (eyes) {
    WindEyes.DOTS -> Triple(5.8f, 8f, 0f)
    WindEyes.WIDE -> Triple(7.2f, 8.6f, 0f)
    WindEyes.STARRY -> Triple(6.2f, 7.8f, 0f)
    WindEyes.SLEEPY -> Triple(6.8f, 5.8f, 1.6f)
}

/**
 * Un ull de Cares centrat a ([x], [y0]) en píxels, [u] píxels per unitat del cap.
 * Una el·lipse fosca i brillant amb la parpella de dalt (pintada amb el cos i un
 * plec del seu to fosc) que baixa fins a `lid` i es decanta amb `brow`, mirall a
 * l'ull dret, i la de baix, que puja amb `smile` fins a fer un arc content. Tot
 * retallat a l'ull: és l'ull qui canvia de forma. Sota 28 dp ([small]) és una
 * mica més gran i té un sol reflex gros; la figura gran ([full]) s'enfonsa una
 * mica a la cara.
 */
private fun DrawScope.drawWindEye(eyes: WindEyes, p: WindPose, right: Boolean, x: Float, y0: Float, u: Float, small: Boolean, full: Boolean,
    ink: Color, accent: Color, paint: WindBodyPaint, cx: Float, cy: Float, r: Float) {
    val (baseRx, baseRy, drop) = windEyeSize(eyes)
    val rx = baseRx * u * (if (small) 1.12f else 1f)
    val ry = baseRy * u * (if (small) 1.08f else 1f)
    val y = y0 + drop * u
    val lash = lerp(accent, Color(0xFF1B1530), .45f)
    val shine = if (ink == Color.White) accent else Color.White
    if (full) drawOval(Brush.radialGradient(0f to paint.core.copy(alpha = .45f), 1f to paint.core.copy(alpha = 0f),
        center = Offset(x, y - .6f * u - .1f * (ry + 1.4f * u)), radius = ry + 1.4f * u),
        Offset(x - rx - 1.2f * u, y - .6f * u - ry - 1.4f * u), Size(2f * (rx + 1.2f * u), 2f * (ry + 1.4f * u)))
    val eye = Path().apply { addOval(androidx.compose.ui.geometry.Rect(x - rx, y - ry, x + rx, y + ry)) }
    clipPath(eye) {
        drawOval(ink, Offset(x - rx, y - ry), Size(2f * rx, 2f * ry))
        if (!small) drawOval(Brush.radialGradient(0f to lerp(accent, Color.White, .2f).copy(alpha = .7f), 1f to accent.copy(alpha = 0f),
            center = Offset(x, y + .87f * ry), radius = .9f * rx),
            Offset(x - .9f * rx, y - .15f * ry), Size(1.8f * rx, 1.2f * ry))
        val gx = x + .3f * rx
        val gy = y - .38f * ry
        val gr = (if (small) .42f else .34f) * rx
        if (eyes == WindEyes.STARRY && !small) drawPath(Path().apply {
            moveTo(gx, gy - 2f * gr); quadraticTo(gx, gy, gx + 2f * gr, gy); quadraticTo(gx, gy, gx, gy + 2f * gr)
            quadraticTo(gx, gy, gx - 2f * gr, gy); quadraticTo(gx, gy, gx, gy - 2f * gr); close()
        }, shine)
        else drawCircle(shine, gr, Offset(gx, gy))
        if (!small) drawCircle(shine, .15f * rx, Offset(x - .32f * rx, y + .42f * ry), alpha = .7f)
        // The lids are the body's own paint (in its radius-1 frame) with a crease in its deep tone.
        val crease = 1.2f * u
        val h = 2f * ry + 2f * u
        val edge = y - ry + p.lid.coerceIn(0f, 1f) * h
        withTransform({ rotate(if (right) -p.brow else p.brow, Offset(x, edge)) }) {
            val lid = androidx.compose.ui.geometry.Rect(x - 1.6f * rx, edge - h, x + 1.6f * rx, edge)
            withTransform({ translate(cx, cy); scale(r, r, Offset.Zero) }) {
                drawRect(paint.volume, Offset((lid.left - cx) / r, (lid.top - cy) / r), Size(lid.width / r, lid.height / r))
            }
            drawLine(lash, Offset(lid.left, edge), Offset(lid.right, edge), strokeWidth = crease)
        }
        val lowY = y + 2.15f * ry + .5f * u - p.smile.coerceIn(0f, 1f) * 2.3f * ry
        val low = androidx.compose.ui.geometry.Rect(x - 1.7f * rx, lowY - 1.15f * ry, x + 1.7f * rx, lowY + 1.15f * ry)
        withTransform({ translate(cx, cy); scale(r, r, Offset.Zero) }) {
            drawOval(paint.volume, Offset((low.left - cx) / r, (low.top - cy) / r), Size(low.width / r, low.height / r))
        }
        drawOval(lash, low.topLeft, low.size, style = Stroke(crease))
    }
}

/**
 * A wind's own minairó (118, 120), the same every time and the same as on the
 * web (`minairo` in winds.js): the side its barretina folds to ([fold], 1 =
 * right: downwind, north and south by the seed), where its beard ends ([beard],
 * box units, from the mouth style), what it carries ([gear]: a boxwood needle
 * case, a sprig of St John's wort in the band, or a pebble), the barretina
 * colour ([cap]), and the brows, moustache and rare round glasses on top.
 */
internal data class MinairoLook(val fold: Int, val beard: Float, val gear: Int, val cap: Color,
    val brows: String, val moustache: String, val glasses: Boolean)

/** Folk barretina colours by the style letter that picks them, each worn under the black band (`barretines` in winds.js). */
internal val minairoBarretines = listOf("v" to Color(0xFFC9342C), "m" to Color(0xFF6E2F6B), "f" to Color(0xFF2E6B3F),
    "o" to Color(0xFFC8962E), "b" to Color(0xFF26406E), "g" to Color(0xFF7D1F33))

/** "winds.barretinaAuto" for the automatic pick, "winds.barretinaV", "winds.barretinaM"… for a palette letter. */
internal fun barretinaLabelKey(letter: String) = "winds.barretina" + letter.uppercase().ifEmpty { "Auto" }

private fun hueOf(c: Color): Double {
    val (r, g, b) = listOf(c.red, c.green, c.blue).map { it.toDouble() }
    val max = maxOf(r, g, b)
    val d = max - minOf(r, g, b)
    if (d == 0.0) return 0.0
    val h = when (max) { r -> (g - b) / d % 6; g -> (b - r) / d + 2; else -> (r - g) / d + 4 }
    return (h * 60 + 360) % 360
}

internal fun minairoLook(style: WindStyle, body: Color): MinairoLook {
    // The face seed of winds.js (its key is the id); the barretina walks from the id hash instead.
    val seed = style.windId.fold(0) { s, c -> (s * 31 + c.code) % 997 }
    val fold = if (style.bearing % 180 != 0) (if (style.bearing < 180) -1 else 1) else if (seed % 2 == 1) 1 else -1
    val beard = when (style.mouth) { WindMouth.NEUTRAL -> 63.5f; WindMouth.SMILE -> 67.5f; WindMouth.OPEN -> 71f; WindMouth.TONGUE -> 74.5f }
    val start = (Integer.toUnsignedLong(style.windId.hashCode()) % minairoBarretines.size).toInt()
    fun gap(c: Color) = Math.abs(hueOf(c) - hueOf(body)).let { minOf(it, 360 - it) }
    // The person's pick as it is; otherwise the next colour whose hue stands 30° from the body's.
    val cap = minairoBarretines.firstOrNull { it.first == style.barretina }?.second
        ?: minairoBarretines.indices.map { minairoBarretines[(start + it) % minairoBarretines.size].second }.first { gap(it) >= 30 }
    // Moduli prime to the 2 and 3 above, so ids one letter apart still differ.
    return MinairoLook(fold, beard, seed % 3, cap, listOf("straight", "arched", "bushy")[seed % 7 % 3],
        listOf("lobes", "walrus", "handlebar")[seed % 11 % 3], seed % 13 == 5)
}

// La cinta negra de la barretina: el que la fa catalana a primer cop d'ull.
private val MinairoBand = Color(0xFF17141A)
// The beard keeps its own colours on every wind.
private val MinairoBeard = Color(0xFFFFFDF7)
private val MinairoBeardShade = Color(0xFFE2DACD)
private val MinairoCrease = lerp(MinairoBeardShade, Color(0xFF1B1530), .25f)
private val MinairoNose = Color(0xFFE88875)

private fun svgPath(d: String) = PathParser().parsePathString(d).toPath()

// The barretina of winds.js, drawn for a body of rx 24 with its top at 25.5: the
// crown, the tube rising off it and folding over to the right, the shade on its
// hanging part, the crease where it folds, and the wide black band.
private val minairoCrown by lazy { svgPath("M17 32C17.5 22 27.5 16 40 16S62.5 22 63 32Z") }
private val minairoTip by lazy { svgPath("M30 18C28.5 8.5 33 1.5 42.5 1C52.5 .5 60.5 5 64 12.5C67 18.5 68 27 67 34.5C66.6 37.6 62.8 37.8 62.5 34.5" +
    "C62.2 29 60.5 23.5 57 20C54.5 17.5 51.5 16.5 49 16.5C43 16 36 16.5 30 18Z") }
private val minairoTipShade by lazy { svgPath("M64 12.5C67 18.5 68 27 67 34.5C66.6 37.6 62.8 37.8 62.5 34.5C62.2 29 60.5 23.5 57 20Z") }
private val minairoTipCrease by lazy { svgPath("M62.5 34.5C62.2 29 60.5 23.5 57 20C54.5 17.5 51.5 16.5 49 16.5") }
private val minairoBand by lazy { svgPath("M15.5 37.5C19 32 29 30.5 40 30.5S61 32 64.5 37.5L63.2 29C59 24.5 50 22.5 40 22.5S21 24.5 16.8 29Z") }
private val minairoBandShine by lazy { svgPath("M17.5 29.4C22 25.6 31 24 40 24S58 25.6 62.5 29.4") }
// White brows over the right eye, from its centre; the left one is its mirror.
private val minairoBrows by lazy { mapOf(
    "straight" to svgPath("M-5 -8.2L5 -7.4L5 -5.2L-5 -6Z"),
    "arched" to svgPath("M-5.6 -5.6Q0 -11.6 5.8 -5.8Q0 -8.6 -5.6 -5.6Z"),
    "bushy" to svgPath("M-6.4 -5C-5 -10.8 4 -11.6 7.2 -7.6C6 -5.4 2 -6.4 -1 -5.2C-3 -4.6 -5 -4.2 -6.4 -5Z")) }
// The moustache's right half from under the nose: cubic segments in (share of the half width, dy from the body's centre).
private val minairoLips = mapOf(
    "lobes" to listOf(listOf(.2f, -5.4f, .6f, -4.6f, .66f, .2f), listOf(.52f, 3.2f, .2f, 2.8f, 0f, .6f)),
    "walrus" to listOf(listOf(.24f, -5.6f, .72f, -4.4f, .74f, 3.8f), listOf(.5f, 4.2f, .2f, 2.4f, 0f, .6f)),
    "handlebar" to listOf(listOf(.2f, -5.2f, .55f, -4.4f, .74f, -1.4f), listOf(.84f, -.2f, 1f, -1.8f, .94f, -4.8f), listOf(.92f, 1.4f, .4f, 3.2f, 0f, .6f)))

/**
 * The minairó in the 80 box, as winds.js draws it: a Cares body of R 25 at
 * (40, 49) with its feet, a long soft beard narrower than the body with a
 * moustache over the lower face, a round nose, the Cares eyes at .84 with
 * white brows, and the barretina in [MinairoLook.cap] whose tube folds to
 * [MinairoLook.fold] over the black band. The cap's tip bobs and the beard
 * sways on their own clock and both trail the figure by [moved] (how far it
 * went down in the last tenth of a second); motion off leaves them at the
 * mood's rest. Under 28 dp only the silhouette is left.
 */
private fun DrawScope.drawMinairo(look: MinairoLook, eyes: WindEyes, p: WindPose, mood: WindMood, t: Float, motion: Boolean, moved: Float,
    size: Int, accent: Color, ink: Color, paint: WindBodyPaint, tx: Float, ty: Float, talkOpen: Boolean) {
    val s = look.fold.toFloat()
    val small = size < 28
    val full = size >= 80
    val gear = if (small) -1 else look.gear
    val rx = 25f
    val cy = 49f
    fun x(f: Float) = 40f + f * rx
    fun wave(period: Float) = if (motion) sin(2f * PI.toFloat() * (t % period) / period) else 0f
    // Accessories are drawn on the right and mirrored to the side away from the tip.
    fun away(block: DrawScope.() -> Unit) = withTransform({ scale(-s, 1f, Offset(40f, 0f)) }, block)

    if (gear == 2) away {
        drawOval(Color(0xFFA8A39B), Offset(x(1f) + 5f - 5.2f, cy + rx - 2f - 3.8f), Size(10.4f, 7.6f))
        drawOval(Color.White.copy(alpha = .45f), Offset(x(1f) + 3.6f - 2.2f, cy + rx - 3.6f - 1.1f), Size(4.4f, 2.2f))
    }
    drawWindBody(WindShape.BALL, paint, 40f, cy, rx, ground = full,
        stepL = windStep(mood, t, false) / rx, stepR = windStep(mood, t, true) / rx)

    // The beard hangs from under the nose and sways from there; it lags the hops by stretching.
    val busy = mood == WindMood.WORKING || mood == WindMood.WRITING
    val sway = wave(if (busy) .6f else 3.2f) * (if (busy) 3f else 2f) + (if (mood == WindMood.FAILED) 0f else p.tilt * -.3f)
    val stretch = (1f - moved * .05f).coerceIn(.85f, 1.15f)
    val l = if (small) 67f else look.beard
    // Its right half as cubic segments from under the nose to the tip, then back mirrored.
    val half = listOf(listOf(.28f, cy - .5f, .58f, cy - 1f, .82f, cy - 5.5f), listOf(.94f, cy + 1f, .84f, l - 12f, .58f, l - 5.5f),
        listOf(.47f, l - 2.5f, .34f, l - 4f, .25f, l - 2f), listOf(.16f, l, .07f, l, 0f, l + .8f))
    val beard = Path().apply {
        moveTo(40f, cy - 2.5f)
        for (c in half) cubicTo(x(c[0]), c[1], x(c[2]), c[3], x(c[4]), c[5])
        for (i in 3 downTo 0) {
            val c = half[i]
            val (ef, ey) = if (i > 0) half[i - 1][4] to half[i - 1][5] else 0f to cy - 2.5f
            cubicTo(x(-c[2]), c[3], x(-c[0]), c[1], x(-ef), ey)
        }
        close()
    }
    withTransform({ rotate(sway, Offset(40f, cy)); scale(1f, stretch, Offset(40f, cy)) }) {
        withTransform({ translate(1.2f, 1.8f) }) { drawPath(beard, paint.core, alpha = .35f) }
        drawPath(beard, Brush.verticalGradient(0f to MinairoBeard, .55f to MinairoBeard, 1f to MinairoBeardShade, startY = cy - 5.5f, endY = l + .8f))
        if (!small) drawPath(Path().apply {
            for (f in listOf(-.4f, 0f, .4f)) { moveTo(x(f), cy + 7f); quadraticTo(x(f * 1.15f), (cy + l) / 2f, x(f * .55f), l - 6f) }
        }, MinairoCrease, alpha = .6f, style = Stroke(1f, cap = StrokeCap.Round))
    }

    // The Cares eyes, at .84 on this smaller head.
    val gx = (p.gx + tx * 2.4f) * .8f
    val gy = (p.gy + ty * 1.8f) * .8f
    for (right in listOf(false, true)) {
        drawWindEye(eyes, p, right, x(if (right) .34f else -.34f) + gx, cy - 10f + gy, .84f, small, full, ink, accent, paint, 40f, cy, rx)
    }

    // A mouth only shows when it talks or cheers, under the moustache.
    val open = when (mood) {
        WindMood.TALKING -> if (talkOpen || !motion) 5.2f else 0f
        WindMood.DONE -> 4.4f
        else -> 0f
    }
    if (open > 0f) {
        drawOval(ink, Offset(36.6f, 54.6f), Size(6.8f, open))
        drawOval(WindTongue, Offset(38f, 54.6f + open * .55f), Size(4f, open * .4f))
    }
    // The moustache lifts with a smile and droops when it fails; a small face keeps the plain lobes.
    val curl = p.smile * 16f - (if (mood == WindMood.FAILED) 12f else 0f) + wave(4f) * 1.5f
    val lip = Path().apply {
        moveTo(0f, -1.8f)
        for (c in minairoLips.getValue(if (small) "lobes" else look.moustache)) cubicTo(c[0] * rx, c[1], c[2] * rx, c[3], c[4] * rx, c[5])
        close()
    }
    for (side in listOf(1f, -1f)) withTransform({ translate(40f, cy); rotate(curl * side, Offset.Zero); scale(side, 1f, Offset.Zero) }) {
        drawPath(lip, MinairoBeard)
        if (!small) drawPath(lip, MinairoCrease, style = Stroke(.8f))
    }
    val nose = if (small) 4.4f else 3.8f
    drawCircle(Brush.radialGradient(0f to Color(0xFFF8BCA8), .55f to MinairoNose, 1f to Color(0xFFB2544A),
        center = Offset(38.8f, cy - 4.1f), radius = nose * 1.5f), nose, Offset(40f, cy - 2.6f))
    if (!small) drawCircle(Color.White, .9f, Offset(38.7f, cy - 3.8f), alpha = .7f)

    // The barretina, fitted to this body from the web's rx 24 and top at 25.5.
    // The tip bobs on its own clock, lags the figure and flies up on the hop.
    val (period, amp) = when (mood) {
        WindMood.WORKING -> .6f to 7f
        WindMood.WRITING, WindMood.SEARCHING -> 1.2f to 4f
        WindMood.NEWS, WindMood.DISCUSSING -> 2f to 5f
        else -> 3f to 3f
    }
    val rest = when (mood) {
        WindMood.FAILED -> 14f
        WindMood.DONE, WindMood.HAPPY -> -6f
        WindMood.THINKING -> -3f
        else -> 0f
    }
    // Held to a swing the fold can make without the tube parting from the crown.
    val tip = (rest + wave(period) * amp - moved * 9f).coerceIn(-18f, 18f)
    val cap = look.cap
    fun capPaint(px: Float, py: Float, r: Float) = Brush.radialGradient(0f to lerp(cap, Color.White, .3f), .5f to cap,
        1f to lerp(cap, Color(0xFF1B1530), .35f), center = Offset(px, py), radius = r)
    withTransform({ translate(0f, cy - rx - 25.5f); scale(rx / 24f, 1f, Offset(40f, 0f)) }) {
        drawPath(minairoCrown, capPaint(30.8f, 19.2f, 44f))
        withTransform({ scale(s, 1f, Offset(40f, 0f)); rotate(tip, Offset(46f, 17f)) }) {
            drawPath(minairoTip, capPaint(40.4f, 8f, 38f))
            drawPath(minairoTipShade, MinairoBand, alpha = .32f)
            if (!small) drawPath(minairoTipCrease, MinairoBand, alpha = .6f, style = Stroke(1.2f, cap = StrokeCap.Round))
        }
        drawPath(minairoBand, MinairoBand)
        if (!small) drawPath(minairoBandShine, Color.White, alpha = .18f, style = Stroke(.9f, cap = StrokeCap.Round))
        // A sprig of St John's wort, the herb minairons are born from, tucked in the band.
        if (gear == 1) away {
            val leaf = Color(0xFF6C9A3E)
            val wort = Color(0xFFF4C431)
            drawPath(Path().apply { moveTo(58f, 30f); cubicTo(61f, 24f, 63f, 19f, 67f, 14.5f) }, leaf, style = Stroke(1.4f, cap = StrokeCap.Round))
            withTransform({ rotate(-40f, Offset(62.6f, 22.5f)) }) { drawOval(leaf, Offset(59.6f, 21.2f), Size(6f, 2.6f)) }
            withTransform({ rotate(25f, Offset(65.4f, 20.2f)) }) { drawOval(leaf, Offset(62.8f, 19.1f), Size(5.2f, 2.2f)) }
            for (a in 0 until 5) withTransform({ rotate(a * 72f, Offset(67f, 14.5f)) }) { drawOval(wort, Offset(65.5f, 9.5f), Size(3f, 4.8f)) }
            drawCircle(lerp(wort, cap, .4f), 1.3f, Offset(67f, 14.5f))
        }
    }

    // Glasses and white brows sit over the band's edge, so both read on it and on the body.
    val frame = Color(0xFF2B2530)
    for (side in listOf(-1f, 1f)) {
        val ex = x(.34f * side)
        if (!small && look.glasses) {
            drawCircle(Color.White.copy(alpha = .14f), 7.4f, Offset(ex, cy - 10f))
            drawCircle(frame, 7.4f, Offset(ex, cy - 10f), style = Stroke(1.4f))
        }
        // The brows follow the lids: down with them, and slanting with the mood.
        if (!small) withTransform({ translate(ex, cy - 10f + p.lid * 2.4f); rotate(side * -p.brow, Offset.Zero); scale(side, 1f, Offset.Zero) }) {
            val brow = minairoBrows.getValue(look.brows)
            drawPath(brow, MinairoBeard)
            drawPath(brow, MinairoCrease, style = Stroke(.8f, join = StrokeJoin.Round))
        }
    }
    if (!small && look.glasses) drawPath(Path().apply { moveTo(x(-.34f) + 7.4f, cy - 10f); quadraticTo(40f, cy - 12.5f, x(.34f) - 7.4f, cy - 10f) },
        frame, style = Stroke(1.4f))

    // The boxwood needle case minairons live in, standing by its feet as tall as half of it.
    if (gear == 0) away {
        val wood = Color(0xFFD9B26A)
        withTransform({ translate(x(1f) + 4.5f, cy + rx - 9f); rotate(8f, Offset.Zero) }) {
            drawRoundRect(wood, Offset(-3.4f, -9f), Size(6.8f, 18f), CornerRadius(3f))
            drawRoundRect(lerp(wood, Color(0xFF1B1530), .35f), Offset(-3.7f, -10f), Size(7.4f, 4f), CornerRadius(1.8f))
            drawLine(Color.White.copy(alpha = .45f), Offset(-1.5f, -4f), Offset(-1.5f, 6f), 1.2f, StrokeCap.Round)
            drawLine(lerp(wood, Color(0xFF1B1530), .2f), Offset(-3.4f, 3f), Offset(3.4f, 3f), .8f)
        }
    }
}

// Esperit de vent (`esperit` in winds.js): the tails in their own frame, the
// head's centre at the origin and pointing along +x, in the web's 100 box (R 28).
private val esperitTails by lazy {
    PathParser().parsePathString("M0 -28C25.2 -29.68 40.6 -20.16 53.2 -16.24C36.4 -11.76 23.8 -5.04 8.4 -.56Z" +
        "M0 28C22.4 28.84 34.16 21.84 44.24 17.92C30.8 14 19.6 8.4 7 3.36Z").toPath()
}
private val esperitStreak by lazy { PathParser().parsePathString("M28.56 1.12C36.4 -1.12 42 2.8 50.4 .56").toPath() }

/**
 * A head with two tails pointing to where the wind blows from, its eyes
 * looking where it goes. The idle sway follows [motion]; while the wind works
 * the tails move four times faster even without it, because that is the work, unless the person turned motion off.
 */
@Composable
private fun EsperitFace(size: Int, mood: WindMood, accent: Color, bearing: Int, motion: Boolean, modifier: Modifier) {
    val busy = mood in windWorkMoods
    // La feina mou la cara també als avatars petits, però mai amb el moviment apagat (com el web i el mapa).
    val loop = if (motion || busy && windMotionEnabled()) {
        rememberInfiniteTransition(label = "esperit").animateFloat(
            initialValue = 0f, targetValue = 12f,
            animationSpec = infiniteRepeatable(tween(12000, easing = LinearEasing)), label = "loop")
    } else null
    val seed = remember(accent, bearing) { ((accent.hashCode() * 31 + bearing) and 0xffff) / 65535f * 12f }
    // The head is a Cares ball: the same volume, and the same tilt on large faces only.
    val lean = rememberWindLean(motion && size >= 64)
    val bodyPaint = remember(accent) { WindBodyPaint(WindShape.BALL, accent) }
    Canvas(modifier.size(size.dp)) {
        with(EsperitFigure) { draw(loop?.let { (it.value + seed) % 12f } ?: 0f, loop != null, mood, accent, bearing, lean.value, bodyPaint) }
    }
}

/** An Esperit at one instant [t]: the live face and the still avatar of a notification draw it the same way. */
internal object EsperitFigure {
    fun DrawScope.draw(t: Float, moving: Boolean, mood: WindMood, accent: Color, bearing: Int, lean: Offset, bodyPaint: WindBodyPaint) {
        val busy = mood in windWorkMoods
        val ink = windInk(accent)
        val tone = lerp(accent, Color(0xFF1B1530), .8f)
        val (tx, ty) = lean
        val paint = if (tx == 0f && ty == 0f) bodyPaint else WindBodyPaint(WindShape.BALL, accent, tx, ty)
        val period = if (busy) .75f else 3f
        val turn = 2f * PI.toFloat() * (t % period) / period
        val sway = if (moving) sin(turn) * 5f else 0f
        val drift = if (moving) (1f - cos(turn)) / 2f else 0f
        val rad = Math.toRadians(bearing.toDouble())
        val ux = sin(rad).toFloat()
        val uy = -cos(rad).toFloat()
        val hx = 50f - 12f * ux
        val hy = 50f - 12f * uy
        val px = -1.6f * ux
        val py = -1.6f * uy
        val k = min(this.size.width, this.size.height) / 100f
        withTransform({ scale(k, k, Offset.Zero); translate(px * drift, (py - 1.4f) * drift) }) {
            withTransform({ translate(hx, hy); scale(28f, 28f, Offset.Zero) }) { drawWindShadow(paint) }
            withTransform({ translate(hx, hy); rotate(bearing - 90f + sway, Offset.Zero) }) {
                val tail = Brush.horizontalGradient(0f to accent, .55f to accent.copy(alpha = .55f), 1f to accent.copy(alpha = 0f),
                    startX = 11.2f, endX = 53.2f)
                drawPath(esperitTails, tail)
                drawPath(esperitStreak, tail, style = Stroke(3.2f, cap = StrokeCap.Round))
            }
            withTransform({ translate(hx, hy); scale(28f, 28f, Offset.Zero) }) { drawWindVolume(WindShape.BALL, paint) }
            for (side in listOf(-1f, 1f)) {
                drawOval(Color(0xFFE4577A), Offset(hx + 14f * side - 4.6f, hy + 3.2f), Size(9.2f, 5.6f), alpha = .32f)
                drawOval(ink, Offset(hx + 8f * side + px - 2.9f, hy - 5.7f + py), Size(5.8f, 7.4f))
                drawCircle(if (ink == Color.White) accent else Color.White, 1f, Offset(hx + 8f * side + 1f + px, hy - 3.4f + py))
            }
            val mouth = Path().apply { moveTo(hx - 5f, hy + 6f); quadraticTo(hx, hy + 10.5f, hx + 5f, hy + 6f) }
            drawPath(mouth, ink, style = Stroke(2.2f, cap = StrokeCap.Round))
        }
        val unit = min(this.size.width, this.size.height) / 80f
        withTransform({ scale(unit, unit, Offset.Zero) }) { drawBadge(mood, t, moving, 1f, tone) }
    }
}

// Cap de vent (`cap` in winds.js): the breath in its own frame, leaving the rim
// along +x, in the web's 100 box. Faces under 28 dp blow two short thick lines.
private val capGusts by lazy { listOf("M0-7C10-11 18-5 30-9", "M0 0C12-3 22 3 36 0", "M0 7C10 11 18 5 30 9").map { PathParser().parsePathString(it).toPath() } }
private val capPuffs by lazy { listOf("M0-5C6-7 12-4 18-6", "M0 6C6 8 12 5 18 7").map { PathParser().parsePathString(it).toPath() } }

/**
 * A head of the old maps, full cheeks and closed eyes, on the side the wind
 * comes from and blowing toward where it goes. The breath flows at idle with
 * [motion]; while the wind works it flows faster even without it.
 */
@Composable
private fun CapFace(size: Int, mood: WindMood, accent: Color, bearing: Int, motion: Boolean, modifier: Modifier) {
    val busy = mood in windWorkMoods
    // La feina mou la cara també als avatars petits, però mai amb el moviment apagat (com el web i el mapa).
    val loop = if (motion || busy && windMotionEnabled()) {
        rememberInfiniteTransition(label = "cap").animateFloat(
            initialValue = 0f, targetValue = 12f,
            animationSpec = infiniteRepeatable(tween(12000, easing = LinearEasing)), label = "loop")
    } else null
    val seed = remember(accent, bearing) { ((accent.hashCode() * 31 + bearing) and 0xffff) / 65535f * 12f }
    val lean = rememberWindLean(motion && size >= 64)
    val bodyPaint = remember(accent) { WindBodyPaint(WindShape.BALL, accent) }
    Canvas(modifier.size(size.dp)) {
        with(CapFigure) { draw(loop?.let { (it.value + seed) % 12f } ?: 0f, loop != null, size, mood, accent, bearing, lean.value, bodyPaint) }
    }
}

/** A Cap de vent at one instant [t]: the live face and the still avatar of a notification draw it the same way. */
internal object CapFigure {
    fun DrawScope.draw(t: Float, moving: Boolean, size: Int, mood: WindMood, accent: Color, bearing: Int, lean: Offset, bodyPaint: WindBodyPaint) {
        val busy = mood in windWorkMoods
        val ink = windInk(accent)
        val tone = lerp(accent, Color(0xFF1B1530), .8f)
        val small = size < 28
        val (tx, ty) = lean
        val paint = if (tx == 0f && ty == 0f) bodyPaint else WindBodyPaint(WindShape.BALL, accent, tx, ty)
        val period = if (busy) .7f else 1.8f
        val rad = Math.toRadians(bearing.toDouble())
        val ux = sin(rad).toFloat()
        val uy = -cos(rad).toFloat()
        val r = if (small) 28f else 25f
        val hx = 50f + 11f * ux
        val hy = 50f + 11f * uy
        val mx = hx - 9f * ux
        val my = hy + 7f - 3f * uy
        val k = min(this.size.width, this.size.height) / 100f
        // The web's face clips to its box, so the breath ends at the edge.
        clipRect {
            withTransform({ scale(k, k, Offset.Zero) }) {
                // The head is a ball 1.5 wider than tall, with the volume of Cares.
                withTransform({ translate(hx, hy); scale(r + 1.5f, r, Offset.Zero) }) {
                    drawWindShadow(paint)
                    drawWindVolume(WindShape.BALL, paint)
                }
                if (small) drawCircle(ink, 4f, Offset(mx, my)) else {
                    for (side in listOf(-1f, 1f)) drawCircle(Color(0xFFE4577A), 6.5f, Offset(hx + 13f * side, hy + 5f), alpha = .38f)
                    val eyes = Path().apply {
                        moveTo(hx - 12f, hy - 3f); relativeQuadraticBezierTo(4f, -4.5f, 8f, 0f)
                        moveTo(hx + 4f, hy - 3f); relativeQuadraticBezierTo(4f, -4.5f, 8f, 0f)
                    }
                    drawPath(eyes, ink, style = Stroke(2.3f, cap = StrokeCap.Round))
                    drawOval(ink, Offset(mx - 3.4f, my - 3.9f), Size(6.8f, 7.8f))
                }
                withTransform({ translate(hx - ux * (r + 3f), hy - uy * (r + 3f)); rotate(bearing + 90f, Offset.Zero) }) {
                    val breath = lerp(accent, Color.White, .25f)
                    val brush = Brush.horizontalGradient(0f to breath, 1f to breath.copy(alpha = 0f), startX = 0f, endX = if (small) 18f else 36f)
                    // Dashes 12/6 flowing outward (wf-flow: offset 18 to 0); the short pair is solid.
                    val dash = if (small) null else PathEffect.dashPathEffect(floatArrayOf(12f, 6f), 18f * (1f - t % period / period))
                    for (line in if (small) capPuffs else capGusts) drawPath(line, brush, style = Stroke(if (small) 7f else 3.6f, cap = StrokeCap.Round, pathEffect = dash))
                }
            }
        }
        val unit = min(this.size.width, this.size.height) / 80f
        withTransform({ scale(unit, unit, Offset.Zero) }) { drawBadge(mood, t, moving, 1f, tone) }
    }
}

/** One point of the rose (`point` in winds.js): two halves meeting along -y, the light one on the left. */
private fun DrawScope.rosePoint(len: Float, wd: Float, left: Color, right: Color, alpha: Float = 1f) {
    drawPath(Path().apply { moveTo(0f, 0f); lineTo(-wd, -wd); lineTo(0f, -len); close() }, left, alpha = alpha)
    drawPath(Path().apply { moveTo(0f, 0f); lineTo(wd, -wd); lineTo(0f, -len); close() }, right, alpha = alpha)
}

/**
 * Rosa de rellotge (`rosa` in winds.js): the wind rose as a dial on a dark tone
 * of the wind's colour, its long point in the wind's colour turned to where it
 * comes from. Under 28 dp only that point, wider; from 80 dp the minute ticks.
 * The point sways at idle with [motion] and swings while the wind works.
 */
@Composable
private fun RosaFace(size: Int, mood: WindMood, accent: Color, bearing: Int, motion: Boolean, modifier: Modifier) {
    val busy = mood in windWorkMoods
    // La feina mou la cara també als avatars petits, però mai amb el moviment apagat (com el web i el mapa).
    val loop = if (motion || busy && windMotionEnabled()) {
        rememberInfiniteTransition(label = "rosa").animateFloat(
            initialValue = 0f, targetValue = 12f,
            animationSpec = infiniteRepeatable(tween(12000, easing = LinearEasing)), label = "loop")
    } else null
    val seed = remember(accent, bearing) { ((accent.hashCode() * 31 + bearing) and 0xffff) / 65535f * 12f }
    val lean = rememberWindLean(motion && size >= 64)
    val bodyPaint = remember(accent) { WindBodyPaint(WindShape.BALL, accent) }
    Canvas(modifier.size(size.dp)) {
        with(RosaFigure) { draw(loop?.let { (it.value + seed) % 12f } ?: 0f, loop != null, size, mood, accent, bearing, lean.value, bodyPaint) }
    }
}

/** A Rosa de rellotge at one instant [t]: the live face and the still avatar of a notification draw it the same way. */
internal object RosaFigure {
    fun DrawScope.draw(t: Float, moving: Boolean, size: Int, mood: WindMood, accent: Color, bearing: Int, lean: Offset, bodyPaint: WindBodyPaint) {
        val busy = mood in windWorkMoods
        val tone = lerp(accent, Color(0xFF1B1530), .8f)
        val dark = lerp(accent, Color(0xFF0F0C18), .72f)
        val cream = Color(0xFFF3EAD8)
        val faint = lerp(cream, dark, .45f)
        val small = size < 28
        val face = lerp(accent, Color(0xFF1B1530), .3f)
        val (tx, ty) = lean
        val paint = if (tx == 0f && ty == 0f) bodyPaint else WindBodyPaint(WindShape.BALL, accent, tx, ty)
        val period = if (busy) 1.4f else 4f
        val swing = if (moving) -cos(2f * PI.toFloat() * (t % period) / period) * (if (busy) 10f else 5f) else 0f
        val k = min(this.size.width, this.size.height) / 100f
        withTransform({ scale(k, k, Offset.Zero); translate(50f, 50f) }) {
            // The dial keeps its dark face, lit like a Cares ball: a bezel of light and a glint on the glass.
            // It fills the box, so its contact shadow would fall outside: the needle's shadow on the dial gives the depth.
            withTransform({ scale(50f, 50f, Offset.Zero) }) {
                drawWindVolume(WindShape.BALL, paint, Brush.radialGradient(0f to face, 1f to dark, center = paint.light, radius = 1.4f))
            }
            if (!small) {
                drawCircle(cream, 44f, Offset.Zero, alpha = .3f, style = Stroke(1f))
                if (size >= 80) for (i in 0 until 32) withTransform({ rotate(i * 11.25f, Offset.Zero) }) {
                    drawLine(cream, Offset(0f, -43.5f), Offset(0f, if (i % 4 != 0) -41.5f else -40f), 1f, alpha = .4f)
                }
                for (a in 0 until 360 step 90) withTransform({ rotate(a.toFloat(), Offset.Zero) }) { rosePoint(29f, 5.5f, cream, faint, .55f) }
                for (a in 45 until 360 step 90) withTransform({ rotate(a.toFloat(), Offset.Zero) }) { rosePoint(20f, 4.5f, cream, faint, .32f) }
            }
            val shade = Color(0xFF0B0C14).copy(alpha = .5f)
            withTransform({ translate(2f + 2f * tx, 2.5f + 2f * ty); rotate(bearing + swing, Offset.Zero) }) {
                rosePoint(if (small) 40f else 43f, if (small) 12f else 8f, shade, shade)
            }
            withTransform({ rotate(bearing + swing, Offset.Zero) }) {
                rosePoint(if (small) 40f else 43f, if (small) 12f else 8f, lerp(accent, Color.White, .55f), accent)
            }
            drawCircle(cream, if (small) 7f else 4.5f, Offset.Zero)
            drawCircle(dark, if (small) 3f else 1.8f, Offset.Zero)
        }
        val unit = min(this.size.width, this.size.height) / 80f
        withTransform({ scale(unit, unit, Offset.Zero) }) { drawBadge(mood, t, moving, 1f, tone) }
    }
}

// --- Mascotes: l'eina de l'ofici de cada especialista de colla --------------
// Les mateixes formes que `kits` a internal/web/app/winds.js, amb la mateixa
// sintaxi de camins SVG i el mateix viewBox de 80×80. `head` va en coordenades
// de la cara (s'escala amb ella); `prop` es porta davant, amb `hands` a sobre.
// KitInk marca «la tinta»: en dibuixar es canvia pel to fosc del mateix vent.
private const val KitInk = 0xFF262040
private val WindTongue = Color(0xFFE8667E)
private val windBlush = Brush.radialGradient(
    0f to Color(0xFFE4577A).copy(alpha = .5f), 1f to Color(0xFFE4577A).copy(alpha = 0f), center = Offset.Zero, radius = 1f,
)

internal class KitShape(val d: String, val fill: Long? = null, val stroke: Long? = null, val width: Float = 0f, val fillAlpha: Float = 1f)
internal class MascotKit(val head: List<KitShape> = emptyList(), val prop: List<KitShape> = emptyList(), val hands: List<Offset> = emptyList())

private fun line(d: String, width: Float = 2.2f, color: Long = KitInk) = KitShape(d, stroke = color, width = width)
private fun solid(d: String, fill: Long, width: Float = 2.2f) = KitShape(d, fill = fill, stroke = KitInk, width = width)
private fun flat(d: String, fill: Long) = KitShape(d, fill = fill)
private fun oval(cx: Float, cy: Float, rx: Float, ry: Float) = "M${cx - rx} ${cy}a$rx $ry 0 1 0 ${2 * rx} 0a$rx $ry 0 1 0 ${-2 * rx} 0Z"
private fun circle(cx: Float, cy: Float, r: Float) = oval(cx, cy, r, r)
private fun rect(x: Float, y: Float, w: Float, h: Float, rx: Float) =
    "M${x + rx} ${y}h${w - 2 * rx}a$rx $rx 0 0 1 $rx ${rx}v${h - 2 * rx}a$rx $rx 0 0 1 ${-rx} ${rx}h${-(w - 2 * rx)}a$rx $rx 0 0 1 ${-rx} ${-rx}v${-(h - 2 * rx)}a$rx $rx 0 0 1 $rx ${-rx}Z"

private val movieFrame = rect(21f, 29f, 18f, 16f, 3.5f) + rect(41f, 29f, 18f, 16f, 3.5f) + "M39 35h2M21 34l-7-2.5M59 34l7-2.5"

internal val windKits: Map<String, MascotKit> = mapOf(
    "shopping" to MascotKit(hands = listOf(Offset(53f, 59f)), prop = listOf(
        line("M57 54v-3a6 6 0 0 1 12 0v3", 2.4f),
        solid("M52 54H74L72 75H54Z", 0xFFE97A86),
        line("M53.5 60H72.5", 2.4f, 0xD9FFF1EC))),
    "books" to MascotKit(hands = listOf(Offset(17f, 69.5f), Offset(63f, 69.5f)), prop = listOf(
        solid("M17 63.5Q28 58.5 40 63.5Q52 58.5 63 63.5V77.5Q52 73.5 40 78.5Q28 73.5 17 77.5Z", 0xFF5E86D6),
        solid("M20 62.5Q30 58.5 40 62.5V75.5Q30 71.5 20 74.5Z", 0xFFFFF8EE, 1.6f),
        solid("M60 62.5Q50 58.5 40 62.5V75.5Q50 71.5 60 74.5Z", 0xFFFFF8EE, 1.6f),
        line("M24 65.5q6-2 12 0M24 69.5q6-2 12 0M44 65.5q6-2 12 0M44 69.5q6-2 12 0", 1.3f, 0xFFA79FBC))),
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
            // Tall lenses: they frame the bold eyes instead of cutting them.
            KitShape(rect(22.6f, 27.8f, 15.6f, 21.2f, 7.4f), 0xFFFFFFFF, KitInk, 2.2f, .25f),
            KitShape(rect(41.8f, 27.8f, 15.6f, 21.2f, 7.4f), 0xFFFFFFFF, KitInk, 2.2f, .25f),
            line("M38.2 36.6q1.8-1.8 3.6 0M22.6 35.6l-7.6-2.6M57.4 35.6l7.6-2.6", 2f)),
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
        solid(rect(21f, 57f, 38f, 16.5f, 3f), 0xFF3A3F5C),
        line("M33 61l-4 4 4 4M47 61l4 4-4 4M42 60.2l-4 9.6", 2.2f, 0xFF8EE3B0),
        solid("M14 73.5H66L63 78H17Z", 0xFFC3C9D9))),
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
    "cooking" to MascotKit(hands = listOf(Offset(52f, 67f)),
        head = listOf(
            solid("M18 21C10 16 13 6 22 7C24 1 34-1 40 3C46-1 56 1 58 7C67 6 70 16 62 21Q40 12 18 21Z", 0xFFFFFFFF),
            solid("M16 20Q40 9 64 20V27Q40 16 16 27Z", 0xFFFFFFFF),
            line("M28 15.5v6M40 13v6M52 15.5v6", 1.6f, 0xFFC3C9D9)),
        prop = listOf(
            line("M59 61.5L51 67.5", 4.4f, 0xFF8A5A3B),
            solid(circle(66f, 56f, 9.5f), 0xFF3A3F5C),
            flat("M60.5 55.5c0-4 4-6 7-4.5 3-.5 5.5 2.5 4 5.5 1 3-2.5 5-5 4-3 1.5-6.5-1-6-5Z", 0xFFFFFFFF),
            flat(circle(66.5f, 56f, 2.7f), 0xFFF5B731))),
    "weather" to MascotKit(hands = listOf(Offset(65f, 60f)), prop = listOf(
        line("M65 45V66a3 3 0 0 1-6 0", 2.6f),
        line("M65 33v-2.5", 2.4f),
        solid("M52.5 45A12.5 12 0 0 1 77.5 45Q74.4 42.5 71.25 45Q68.1 42.5 65 45Q61.9 42.5 58.75 45Q55.6 42.5 52.5 45Z", 0xFF5E86D6),
        line("M65 33Q59.5 37 58.75 45M65 33Q70.5 37 71.25 45", 1.6f))),
    "travel" to MascotKit(hands = listOf(Offset(51f, 62f)), prop = listOf(
        line("M57.5 52v-3a3 3 0 0 1 3-3h3a3 3 0 0 1 3 3v3", 2.4f),
        solid(rect(50f, 52f, 24f, 20f, 3.5f), 0xFFF0A04B),
        line("M56 53v18M68 53v18", 2.4f, 0xFFA8612E),
        flat(circle(62f, 62f, 3f), 0xFFFFF8EE))),
    "fitness" to MascotKit(hands = listOf(Offset(63f, 64f)),
        head = listOf(
            solid("M11 24Q40 11 69 24L70 30.5Q40 18 10 30.5Z", 0xFF2FB58A, 2f),
            line("M10.6 27.3Q40 14.6 69.4 27.3", 1.8f, 0xFFFFFFFF),
            solid("M11 26.5L3.5 24L4.5 30.5ZM11 28.5L5.5 34L9.5 35.5Z", 0xFF2FB58A, 2f)),
        prop = listOf(
            line("M53 64H73", 3f),
            solid(rect(49f, 56f, 5f, 16f, 2f), 0xFFE5484D),
            solid(rect(72f, 56f, 5f, 16f, 2f), 0xFFE5484D),
            solid(rect(54f, 58.5f, 3.5f, 11f, 1.5f), 0xFFF28C6B, 1.6f),
            solid(rect(68.5f, 58.5f, 3.5f, 11f, 1.5f), 0xFFF28C6B, 1.6f))),
    "finance" to MascotKit(hands = listOf(Offset(51f, 67f)), prop = listOf(
        solid(rect(54f, 67f, 4.5f, 7f, 1.5f), 0xFFF4A3B5, 2f),
        solid(rect(64.5f, 67f, 4.5f, 7f, 1.5f), 0xFFF4A3B5, 2f),
        solid("M63.5 56L66 49.5L71 56Z", 0xFFF4A3B5, 2f),
        solid(oval(61f, 62.5f, 11.5f, 9f), 0xFFF4A3B5),
        solid(oval(72.5f, 63f, 3f, 3.6f), 0xFFF7C3CF, 2f),
        line("M71.6 62h.01M73.4 64h.01M67 59.5h.01", 2.2f),
        line("M56.5 54.5h6", 2f),
        solid(circle(59.5f, 48f, 4f), 0xFFF5C042, 2f))),
    "movies" to MascotKit(hands = listOf(Offset(51f, 66f)),
        head = listOf(
            // 3D glasses: tinted lenses over the eyes, a white card frame around them.
            KitShape(rect(21f, 29f, 18f, 16f, 3.5f), 0xFFE5484D, fillAlpha = .45f),
            KitShape(rect(41f, 29f, 18f, 16f, 3.5f), 0xFF3FB6D9, fillAlpha = .45f),
            line(movieFrame, 5.2f),
            line(movieFrame, 2.4f, 0xFFFFFFFF)),
        prop = listOf(
            flat(rect(50f, 56f, 24f, 16f, 2.5f), 0xFF3A3F5C),
            flat("M50 56h24v5H50Z", 0xFFFFFFFF),
            flat("M54 56h4l-3 5h-4ZM62 56h4l-3 5h-4ZM70 56h4l-3 5h-4Z", KitInk),
            line("M54 65.5h8M54 68.5h12", 1.6f, 0xFFA79FBC),
            line(rect(50f, 56f, 24f, 16f, 2.5f)),
            flat("M50 55L73.2 48.8L71.9 44L48.7 50.2Z", 0xFFFFFFFF),
            flat("M54.8 53.7L57.7 52.9L59.3 47.4L56.4 48.1ZM60.6 52.2L63.5 51.4L65.1 45.8L62.2 46.6ZM66.4 50.6L69.3 49.8L70.9 44.3L68 45Z", KitInk),
            line("M50 55L73.2 48.8L71.9 44L48.7 50.2Z"))),
)

private val kitPaths = HashMap<String, Path>()

private fun DrawScope.drawKit(shapes: List<KitShape>, tone: Color) {
    fun tint(color: Long) = if (color == KitInk) tone else Color(color)
    for (shape in shapes) {
        val path = kitPaths.getOrPut(shape.d) { PathParser().parsePathString(shape.d).toPath() }
        shape.fill?.let { drawPath(path, tint(it), alpha = shape.fillAlpha) }
        shape.stroke?.let { drawPath(path, tint(it), style = Stroke(shape.width, cap = StrokeCap.Round, join = StrokeJoin.Round)) }
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
internal fun DrawScope.drawBadge(mood: WindMood, loop: Float, motion: Boolean, done: Float, tone: Color) {
    fun wave(period: Float) = sin(2f * PI.toFloat() * (loop % period) / period)
    fun drawKit(shapes: List<KitShape>) = drawKit(shapes, tone)
    badgeShapes[mood]?.let { drawKit(it) }
    when (mood) {
        WindMood.THINKING -> for (i in 0 until 3) {
            val f = ((loop - i * .2f) % 1.5f + 1.5f) % 1.5f / 1.5f
            val alpha = if (!motion) .85f else if (f < .3f) .3f + f / .3f * .7f else if (f < .6f) 1f - (f - .3f) / .3f * .7f else .3f
            drawCircle(tone, 1.9f, Offset(62f + i * 5f, 12f), alpha = alpha)
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
