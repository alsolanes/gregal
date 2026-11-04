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
    mapOf(WindShape.BALL to circle(0f, 0f, 1f), WindShape.CLOUD to cloud, WindShape.SPIRAL to curl)
}

/** Fins on arriba cada silueta des del centre de la vora de llum: la vora en cenyeix el contorn. */
private val rimReach = mapOf(WindShape.BALL to 1.32f, WindShape.CLOUD to 1.62f, WindShape.SPIRAL to 1.50f)

/** Tons del volum, derivats del color del vent (sense blancs ni negres purs). */
internal fun windLight(accent: Color) = lerp(accent, Color.White, .30f)
internal fun windDeep(accent: Color) = lerp(accent, Color(0xFF1B1530), .30f)

/** Cos del vent amb volum; `tx`/`ty` són la inclinació (−1..1). */
internal fun DrawScope.drawWindBody(shape: WindShape, accent: Color, cx: Float, cy: Float, r: Float, tx: Float, ty: Float) {
    val body = windBodies.getValue(shape)
    // La llum és del món: quan els trets van cap a un costat, la llum hi va en contra.
    val light = Offset(-.38f - tx * .18f, -.50f - ty * .14f)
    withTransform({ translate(cx, cy); scale(r, r, Offset.Zero) }) {
        // Ombra de contacte: el vent sura just damunt d'un terra.
        val foot = Offset(-tx * .08f, 1.13f)
        withTransform({ scale(1f, .2f, foot) }) {
            drawCircle(Brush.radialGradient(0f to Color.Black.copy(alpha = .22f), 1f to Color.Transparent, center = foot, radius = .8f),
                radius = .8f, center = foot)
        }
        // Little feet anchor the character without changing the saved style.
        drawOval(windDeep(accent), topLeft = Offset(-.66f, .78f), size = Size(.43f, .22f))
        drawOval(windDeep(accent), topLeft = Offset(.23f, .78f), size = Size(.43f, .22f))
        drawPath(body, Brush.radialGradient(0f to windLight(accent), .45f to accent, 1f to windDeep(accent), center = light, radius = 1.9f))
        clipPath(body) {
            // Vora de llum reflectida, a baix a la dreta: dona el gir de la forma.
            drawRect(Brush.radialGradient(0f to Color.Transparent, .80f to Color.Transparent,
                1f to lerp(accent, Color.White, .65f).copy(alpha = .40f),
                center = Offset(-.22f - tx * .10f, -.26f - ty * .10f), radius = rimReach.getValue(shape)),
                topLeft = Offset(-1.6f, -1.6f), size = Size(3.2f, 3.2f))
            // Reflex setinat, gran i difús: volum, no plàstic brillant.
            drawCircle(Brush.radialGradient(0f to Color.White.copy(alpha = .28f), 1f to Color.Transparent, center = light, radius = .5f),
                radius = .5f, center = light)
        }
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
    val context = LocalContext.current.applicationContext
    val owner = LocalLifecycleOwner.current
    val saver = remember { context.getSystemService(PowerManager::class.java)?.isPowerSaveMode == true }
    val on = enabled && !saver
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
