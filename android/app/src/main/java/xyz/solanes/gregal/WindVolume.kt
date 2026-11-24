package xyz.solanes.gregal

import android.content.Context
import android.hardware.Sensor
import android.hardware.SensorEvent
import android.hardware.SensorEventListener
import android.hardware.SensorManager
import android.os.PowerManager
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.RoundRect
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathOperation
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.clipPath
import androidx.compose.ui.graphics.drawscope.withTransform
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import kotlin.math.abs

/**
 * Volum dels vents: la silueta continua sent una sola forma neta, però ara té
 * cos — llum suau de dalt a l'esquerra, ombra pròpia a baix, una vora de llum
 * reflectida i una ombra de contacte a terra. Tot amb gradients de Canvas
 * (API 26+): ni shaders, ni imatges, ni biblioteques.
 *
 * Amb el mòbil a la mà, el sensor de gravetat fa una paral·laxi petita: els
 * trets es mouen amb la inclinació i la llum hi va en contra, i la bola sembla
 * una esfera. Un sol listener per a tota l'app, només amb l'activitat en
 * primer pla, sense estalvi de bateria i amb el moviment permès.
 */

/** Siluetes en coordenades de radi 1, centrades a l'origen. */
private val windBodies: Map<WindShape, Path> by lazy {
    fun circle(x: Float, y: Float, r: Float) = Path().apply { addOval(Rect(x - r, y - r, x + r, y + r)) }
    val cloud = listOf(circle(-.61f, -.05f, .67f), circle(.08f, -.19f, .86f), circle(.66f, .04f, .60f))
        .fold(Path().apply { addRoundRect(RoundRect(-1.19f, -.30f, 1.19f, .97f, CornerRadius(.58f))) }) { acc, p ->
            Path.combine(PathOperation.Union, acc, p)
        }
    // The gust is the ball with a curl on top (windCurl): the face keeps its room.
    mapOf(WindShape.BALL to circle(0f, 0f, 1f), WindShape.CLOUD to cloud, WindShape.SPIRAL to circle(0f, 0f, 1f))
}

/**
 * La silueta de Cares per a la forma `o` i la `s` (amb el rínxol): un còdol
 * de plastilina, de cap rodó i espatlles amples, assegut pla a terra. El
 * mateix camí que `pebble` a winds.js (110), en radis del cap (R 33, centre
 * 40, 40.5); el núvol continua sent [windBodies].
 */
private val windPebble by lazy {
    Path().apply {
        moveTo(0f, -.939f)
        cubicTo(.758f, -.939f, 1.045f, -.5f, 1.045f, .076f)
        cubicTo(1.045f, .652f, .667f, .939f, 0f, .939f)
        cubicTo(-.667f, .939f, -1.045f, .652f, -1.045f, .076f)
        cubicTo(-1.045f, -.5f, -.758f, -.939f, 0f, -.939f)
        close()
    }
}

/** Fins on arriba cada silueta des del centre de la vora de llum: la vora en cenyeix el contorn. */
private val rimReach = mapOf(WindShape.BALL to 1.32f, WindShape.CLOUD to 1.62f, WindShape.SPIRAL to 1.32f)

/** Tons del volum, derivats del color del vent (sense blancs ni negres purs). */
internal fun windLight(accent: Color) = lerp(accent, Color.White, .30f)
internal fun windDeep(accent: Color) = lerp(accent, Color(0xFF1B1530), .30f)

/**
 * El rínxol de la ratxa damunt de la bola: canvia el contorn, mai la cara.
 * El mateix camí que `wf-curl` a winds.js, en radis de la bola.
 */
private val windCurl by lazy {
    Path().apply {
        moveTo(-.418f, -.864f)
        cubicTo(-.227f, -.939f, -.094f, -1.055f, -.018f, -1.167f)
        cubicTo(.036f, -1.261f, .170f, -1.306f, .267f, -1.239f)
        cubicTo(.364f, -1.164f, .333f, -1.027f, .212f, -1.018f)
        cubicTo(.152f, -1.015f, .121f, -1.064f, .152f, -1.109f)
        cubicTo(.085f, -1.073f, .097f, -.958f, .209f, -.912f)
        lineTo(.418f, -.861f)
        close()
    }
}
private val curlRoot = Offset(-.085f, -.86f)

/** Els pinzells es reutilitzen mentre la inclinació i el color siguin els mateixos. */
internal class WindBodyPaint(shape: WindShape, accent: Color, tx: Float = 0f, ty: Float = 0f) {
    val deep = windDeep(accent)
    val foot = Offset(-tx * .08f, 1.13f)
    val light = Offset(-.38f - tx * .18f, -.50f - ty * .14f)
    // L'ombra de contacte porta el to del vent: a terra clar es veu de color, no grisa.
    private val ground = lerp(accent, Color(0xFF0B0C14), .72f)
    val shadow = Brush.radialGradient(
        0f to ground.copy(alpha = .5f), 1f to ground.copy(alpha = 0f), center = foot, radius = .8f,
    )
    val volume = Brush.radialGradient(
        0f to windLight(accent), .45f to accent, 1f to deep, center = light, radius = 1.9f,
    )
    val rim = Brush.radialGradient(
        0f to Color.Transparent, .80f to Color.Transparent,
        1f to lerp(accent, Color.White, .65f).copy(alpha = .40f),
        center = Offset(-.22f - tx * .10f, -.26f - ty * .10f), radius = rimReach.getValue(shape),
    )
    val shine = Brush.radialGradient(
        0f to Color.White.copy(alpha = .28f), 1f to Color.Transparent, center = light, radius = .5f,
    )
    // Plastilina de Cares, les capes de `face` a winds.js: un fosc que gira la
    // banda allunyada de la llum, la base que s'enfosqueix on toca els peus, una
    // vora de llum rebotada, un setí suau i una lluentor ampla i tova, no un punt.
    val core = lerp(accent, Color(0xFF1B1530), .6f)
    val shade = Brush.radialGradient(
        .4f to core.copy(alpha = 0f), .78f to core.copy(alpha = .5f), .93f to core.copy(alpha = .38f), 1f to core.copy(alpha = .18f),
        center = light, radius = 1.7f,
    )
    val occlusion = Brush.radialGradient(0f to core.copy(alpha = .75f), 1f to deep.copy(alpha = 0f), center = Offset.Zero, radius = 1f)
    private val glow = lerp(accent, Color.White, .5f)
    val caresRim = Brush.radialGradient(
        .8f to glow.copy(alpha = 0f), 1f to glow.copy(alpha = .55f),
        center = Offset(-.22f - tx * .10f, -.26f - ty * .10f), radius = rimReach.getValue(shape),
    )
    val sheen = Brush.radialGradient(0f to Color.White.copy(alpha = .3f), 1f to Color.Transparent, center = light, radius = .65f)
    val spec = Brush.radialGradient(0f to Color.White.copy(alpha = .7f), .45f to Color.White.copy(alpha = .4f), 1f to Color.Transparent,
        center = Offset.Zero, radius = 1f)
    val groundGlow = Brush.radialGradient(0f to ground.copy(alpha = .5f), 1f to ground.copy(alpha = 0f), center = Offset.Zero, radius = 1f)
}

/**
 * Un cos de Cares en radis del cap (R 33 de la caixa de 80, centre 40, 40.5):
 * peus (que trepitgen amb [stepL] i [stepR], en radis, cap amunt negatiu), el
 * rínxol de la ratxa, la silueta i la plastilina. L'ombra a terra només a la
 * figura gran ([ground]), com al web.
 */
internal fun DrawScope.drawWindBody(shape: WindShape, paint: WindBodyPaint, cx: Float, cy: Float, r: Float, sway: Float = 0f,
    ground: Boolean = true, stepL: Float = 0f, stepR: Float = 0f) {
    withTransform({ translate(cx, cy); scale(r, r, Offset.Zero) }) {
        if (ground) withTransform({ translate(0f, .967f); scale(.697f, .133f, Offset.Zero) }) { drawCircle(paint.groundGlow, 1f, Offset.Zero) }
        drawOval(paint.deep, topLeft = Offset(-.66f, .764f + stepL), size = Size(.43f, .224f))
        drawOval(paint.deep, topLeft = Offset(.23f, .764f + stepR), size = Size(.43f, .224f))
        // El rínxol es gronxa una mica amb l'aire (sway, en graus) des de l'arrel;
        // el seu camí va centrat a 40, no a 40.5.
        if (shape == WindShape.SPIRAL) withTransform({ translate(0f, -.015f); rotate(sway, curlRoot) }) { drawPath(windCurl, paint.volume) }
        // The cloud is drawn at R 30 around (40, 40), as on the web.
        withTransform({ if (shape == WindShape.CLOUD) { translate(0f, -.015f); scale(30f / 33f, 30f / 33f, Offset.Zero) } }) {
            val body = if (shape == WindShape.CLOUD) windBodies.getValue(shape) else windPebble
            drawPath(body, paint.volume)
            clipPath(body) {
                drawRect(paint.shade, topLeft = Offset(-1.6f, -1.6f), size = Size(3.2f, 3.2f))
                withTransform({ translate(0f, .906f); scale(.7f, .24f, Offset.Zero) }) { drawCircle(paint.occlusion, 1f, Offset.Zero) }
                drawRect(paint.caresRim, topLeft = Offset(-1.6f, -1.6f), size = Size(3.2f, 3.2f))
                drawCircle(paint.sheen, radius = .65f, center = paint.light)
            }
        }
        withTransform({ translate(-.36f, -.515f); rotate(-35f, Offset.Zero); scale(.27f, .16f, Offset.Zero) }) { drawCircle(paint.spec, 1f, Offset.Zero) }
    }
}

/** L'ombra de contacte a terra, en coordenades de radi 1: també la fan els caps d'Esperit i Cap. */
internal fun DrawScope.drawWindShadow(paint: WindBodyPaint) {
    withTransform({ scale(1f, .2f, paint.foot) }) {
        drawCircle(paint.shadow, radius = .8f, center = paint.foot)
    }
}

/**
 * El cos amb llum, vora i brillantor, en coordenades de radi 1. [fill] és el
 * gradient del cos; la Rosa hi posa la seva esfera fosca.
 */
internal fun DrawScope.drawWindVolume(shape: WindShape, paint: WindBodyPaint, fill: Brush = paint.volume) {
    val body = windBodies.getValue(shape)
    drawPath(body, fill)
    clipPath(body) {
        drawRect(paint.rim, topLeft = Offset(-1.6f, -1.6f), size = Size(3.2f, 3.2f))
        drawCircle(paint.shine, radius = .5f, center = paint.light)
    }
}

/** Un sol sensor de gravetat compartit per totes les cares visibles. */
internal object WindLean {
    val value = mutableStateOf(Offset.Zero)
    private var users = 0
    private var manager: SensorManager? = null
    private val fast = FloatArray(3)
    private val slow = FloatArray(3)
    private var primed = false

    private val listener = object : SensorEventListener {
        override fun onSensorChanged(event: SensorEvent) {
            val v = event.values
            if (!primed) { for (i in 0..2) { fast[i] = v[i]; slow[i] = v[i] }; primed = true }
            // La línia base s'adapta a poc a poc: respon als moviments i torna al
            // centre quan el mòbil queda quiet en qualsevol angle.
            for (i in 0..2) { fast[i] += (v[i] - fast[i]) * .25f; slow[i] += (v[i] - slow[i]) * .015f }
            val next = Offset(((slow[0] - fast[0]) / 3f).coerceIn(-1f, 1f), ((slow[1] - fast[1]) / 3f).coerceIn(-1f, 1f))
            val now = value.value
            if (abs(next.x - now.x) > .01f || abs(next.y - now.y) > .01f) value.value = next
        }
        override fun onAccuracyChanged(sensor: Sensor?, accuracy: Int) {}
    }

    fun acquire(context: Context) {
        if (users++ > 0) return
        val m = context.getSystemService(SensorManager::class.java) ?: return
        val sensor = m.getDefaultSensor(Sensor.TYPE_GRAVITY) ?: m.getDefaultSensor(Sensor.TYPE_ACCELEROMETER) ?: return
        manager = m
        m.registerListener(listener, sensor, SensorManager.SENSOR_DELAY_UI)
    }

    fun release() {
        if (users == 0 || --users > 0) return
        manager?.unregisterListener(listener)
        manager = null
        primed = false
        value.value = Offset.Zero
    }
}

private val still: State<Offset> = mutableStateOf(Offset.Zero)

/** La inclinació per a una cara; quieta si no cal (petita, moviment reduït, estalvi). */
@Composable
internal fun rememberWindLean(enabled: Boolean): State<Offset> {
    if (!enabled) return still
    val context = LocalContext.current.applicationContext
    val owner = LocalLifecycleOwner.current
    val saver = remember { context.getSystemService(PowerManager::class.java)?.isPowerSaveMode == true }
    val on = !saver
    DisposableEffect(on, owner) {
        var held = false
        fun hold(want: Boolean) {
            if (want && !held) WindLean.acquire(context) else if (!want && held) WindLean.release()
            held = want
        }
        val observer = LifecycleEventObserver { _, _ -> hold(on && owner.lifecycle.currentState.isAtLeast(Lifecycle.State.RESUMED)) }
        owner.lifecycle.addObserver(observer)
        onDispose { owner.lifecycle.removeObserver(observer); hold(false) }
    }
    return if (on) WindLean.value else still
}
