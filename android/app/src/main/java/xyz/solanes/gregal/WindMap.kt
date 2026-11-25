package xyz.solanes.gregal

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.Paint
import androidx.compose.ui.graphics.PaintingStyle
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.clipPath
import androidx.compose.ui.graphics.drawscope.withTransform
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import kotlin.math.PI
import kotlin.math.cos
import kotlin.math.max
import kotlin.math.min
import kotlin.math.sin

// Mapa de vent (`mapa` in winds.js): a live weather map in a circle, streaks
// blowing away from where the wind comes from. mapStart, mapStep and the PRNG
// mirror winds.js number for number, so a wind gets the same picture on the
// web and here. Positions are 0..1 of the picture, angles compass radians.

internal data class MapParticle(val x: Double, val y: Double, val age: Int, val life: Double)
internal data class MapLine(val x0: Double, val y0: Double, val x1: Double, val y1: Double, val alpha: Double)
internal data class MapState(val phi: Double, val seed: Double, val t: Double, val r: Int, val ps: List<MapParticle>, val lines: List<MapLine>)

/** `mapRandom`: the next number in 0..1 and the next state. The Int bits are the web's unsigned 32-bit ones. */
internal fun mapRandom(state: Int): Pair<Double, Int> {
    val r = state + 0x6D2B79F5
    var z = (r xor (r ushr 15)) * (r or 1)
    z = z xor (z + (z xor (z ushr 7)) * (z or 61))
    return ((z xor (z ushr 14)).toLong() and 0xffffffffL) / 4294967296.0 to r
}

private fun mapSpawn(state: Int): Pair<MapParticle, Int> {
    val (x, r1) = mapRandom(state)
    val (y, r2) = mapRandom(r1)
    val (life, r3) = mapRandom(r2)
    return MapParticle(x, y, 0, 40 + life * 60) to r3
}

internal fun mapStart(style: WindStyle): MapState {
    var r = style.windId.hashCode()
    val seed = (r.toLong() and 0xffffffffL) % 1000 / 100.0
    val ps = List(90) { val (p, next) = mapSpawn(r); r = next; p }
    return MapState((style.bearing + 180) * PI / 180, seed, 0.0, r, ps, emptyList())
}

internal fun mapStep(st: MapState, busy: Boolean): MapState {
    val sp = if (busy) .013 else .0055
    val t = st.t + .016
    val lines = ArrayList<MapLine>(st.ps.size)
    var r = st.r
    val ps = st.ps.map { p ->
        val ang = st.phi + .6 * sin(p.x * 5.5 + st.seed + t * .4) * cos(p.y * 4.5 + t * .7)
        val x = p.x + sin(ang) * sp
        val y = p.y - cos(ang) * sp
        val age = p.age + 1
        lines += MapLine(p.x, p.y, x, y, sin(PI * p.age / p.life) * .5)
        if (age <= p.life && x > -.05 && x < 1.05 && y > -.05 && y < 1.05) MapParticle(x, y, age, p.life)
        else { val (next, after) = mapSpawn(r); r = after; next }
    }
    return MapState(st.phi, st.seed, t, r, ps, lines)
}

/** The picture the face shows: each step fades it towards [base] and adds its streaks, as `mapPaint`. */
internal class MapPicture(wind: WindStyle, val px: Int, accent: Color) {
    val image = ImageBitmap(px, px)
    private val canvas = androidx.compose.ui.graphics.Canvas(image)
    private var st = mapStart(wind)
    private val fade = Paint().apply { color = lerp(accent, Color(0xFF0C0A13), .4f) }
    private val streak = Paint().apply {
        color = lerp(accent, Color.White, .55f); style = PaintingStyle.Stroke; strokeCap = StrokeCap.Round; blendMode = BlendMode.Plus
        strokeWidth = max(1.2f, px / (if (px < 60) 28f else 70f))
    }

    init {
        // The still frame: 50 silent steps, then 20 painted ones over the plain ground.
        canvas.drawRect(0f, 0f, px.toFloat(), px.toFloat(), fade)
        repeat(50) { st = mapStep(st, false) }
        repeat(20) { paint(false) }
    }

    fun paint(busy: Boolean) {
        st = mapStep(st, busy)
        val s = px.toFloat()
        fade.alpha = if (busy) .16f else .1f
        canvas.drawRect(0f, 0f, s, s, fade)
        for (l in st.lines) {
            streak.alpha = l.alpha.toFloat()
            canvas.drawLine(Offset(l.x0.toFloat() * s, l.y0.toFloat() * s), Offset(l.x1.toFloat() * s, l.y1.toFloat() * s), streak)
        }
    }
}

/**
 * The map moves, one step a frame, while [motion] allows it (faster while the
 * wind works); otherwise it is the still frame, working or not.
 * Only a composed face steps, so a list scrolled away costs nothing.
 */
@Composable
internal fun MapFace(size: Int, mood: WindMood, accent: Color, style: WindStyle, motion: Boolean, modifier: Modifier) {
    val busy = mood in windWorkMoods
    val px = max(48, with(LocalDensity.current) { size.dp.roundToPx() })
    val picture = remember(style.windId, style.bearing, accent, px) { MapPicture(style, px, accent) }
    val tone = remember(accent) { lerp(accent, Color(0xFF1B1530), .8f) }
    // Seconds on the 12 s badge loop; reading it in the Canvas redraws each frame.
    var clock by remember { mutableFloatStateOf(0f) }
    if (motion) LaunchedEffect(picture, busy) {
        val start = withFrameNanos { it } - (clock * 1e9f).toLong()
        while (true) withFrameNanos { now ->
            picture.paint(busy)
            clock = (now - start) / 1e9f % 12f
        }
    }
    Canvas(modifier.size(size.dp)) {
        with(MapFigure) { draw(clock, motion, mood, picture.image, tone) }
    }
}

/** The map at one instant [t]: the live face and the still avatar of a notification draw it the same way. */
internal object MapFigure {
    fun DrawScope.draw(t: Float, motion: Boolean, mood: WindMood, image: ImageBitmap, tone: Color) {
        val k = min(this.size.width, this.size.height)
        val unit = k / 80f
        clipPath(Path().apply { addOval(androidx.compose.ui.geometry.Rect(0f, 0f, k, k)) }) {
            drawImage(image, dstSize = IntSize(k.toInt(), k.toInt()))
        }
        withTransform({ scale(unit, unit, Offset.Zero) }) {
            drawCircle(Brush.radialGradient(0f to Color.White.copy(alpha = .22f), 1f to Color.White.copy(alpha = 0f),
                center = Offset(25.6f, 20.8f), radius = 36.8f), 40f, Offset(40f, 40f))
            drawCircle(Color.White.copy(alpha = .08f), 39.5f, Offset(40f, 40f), style = Stroke(1f))
            drawBadge(mood, t, motion, 1f, tone)
        }
    }
}
