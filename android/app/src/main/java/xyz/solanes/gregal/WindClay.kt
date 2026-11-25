package xyz.solanes.gregal

import android.content.res.AssetManager
import android.graphics.BitmapFactory
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.platform.LocalContext
import androidx.compose.animation.core.withInfiniteAnimationFrameNanos
import androidx.compose.ui.semantics.SemanticsPropertyKey
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import org.json.JSONObject
import kotlin.math.roundToInt

/**
 * Els vents de plastilina (112): fulls de fotogrames renderitzats amb Blender
 * (tools/characters/), que el build copia als assets de l'APK. Una cara gran
 * en reprodueix el de l'humor, tenyit del color del vent a través de la
 * màscara; si el full no hi és, la cara de Canvas de sempre.
 */

/** The assets folder with `manifest.json` and the sheets; tests point it at one that does not exist. */
internal val LocalWindClay = staticCompositionLocalOf { "characters" }

/** Which sheet a clay face plays, `<silhouette>/<mood>`. */
internal val WindClayKey = SemanticsPropertyKey<String>("WindClay")

/** [cap]: the minairó's barretina, tinted with its own colour (`caps` in the manifest). [still]: the frame shown without motion (142: one with the prop in view). */
internal class WindClaySheet(val name: String, val sheet: ImageBitmap, val mask: ImageBitmap, val cap: ImageBitmap?,
    val size: Int, val maskSize: Int, val frames: Int, val loop: Boolean, val fps: Int, val still: Int)

/** Android's own moods have no sheet: content is idle, talking is the talking turn of discussing. */
internal fun windClayMood(mood: WindMood): String = when (mood) {
    WindMood.HAPPY -> "idle"
    WindMood.TALKING -> "discussing"
    else -> mood.name.lowercase()
}

internal object WindClay {
    private val manifests = HashMap<String, JSONObject?>()
    // Decoded once per sheet; a sheet of 24 frames is 1.5 MB of pixels, so the least used go.
    private val sheets = object : android.util.LruCache<String, WindClaySheet>(24 shl 20) {
        override fun sizeOf(key: String, value: WindClaySheet) =
            4 * (value.sheet.width * value.sheet.height + 2 * value.mask.width * value.mask.height)
    }

    private fun manifest(assets: AssetManager, dir: String): JSONObject? = manifests.getOrPut(dir) {
        try {
            JSONObject(assets.open("$dir/manifest.json").bufferedReader().use { it.readText() })
        } catch (e: java.io.IOException) {
            android.util.Log.w("WindClay", "no clay winds in assets/$dir: ${e.message}")
            null
        }
    }

    /** The sheet of the silhouette that stands for [family] (a shape's key, or the minairó) in [mood], or null when the manifest or the files are not there. */
    fun sheet(assets: AssetManager, dir: String, family: String, mood: WindMood): WindClaySheet? {
        val m = manifest(assets, dir) ?: return null
        val shapes = m.optJSONObject("silhouettes") ?: return null
        val silhouette = shapes.keys().asSequence().firstOrNull { shapes.optString(it) == family } ?: return null
        val moodName = windClayMood(mood)
        val spec = m.optJSONObject("moods")?.optJSONObject(moodName) ?: return null
        val name = "$silhouette/$moodName"
        sheets.get("$dir/$name")?.let { return it }
        fun decode(file: String) = try {
            assets.open("$dir/$file").use { BitmapFactory.decodeStream(it) }?.asImageBitmap()
        } catch (e: java.io.IOException) {
            android.util.Log.w("WindClay", "missing $dir/$file: ${e.message}")
            null
        }
        val sheet = decode("$name.webp") ?: return null
        val mask = decode("$name.mask.webp") ?: return null
        val capped = m.optJSONArray("caps")?.let { caps -> (0 until caps.length()).any { caps.optString(it) == silhouette } } == true
        val cap = if (capped) decode("$name.cap.webp") ?: return null else null
        return WindClaySheet(name, sheet, mask, cap, m.optInt("size", 128), m.optInt("maskSize", 64),
            spec.optInt("frames", 1), spec.optBoolean("loop", true), m.optInt("fps", 12), spec.optInt("still", 0))
            .also { sheets.put("$dir/$name", it) }
    }

    // A tinted sheet is as big as the sheet: a few winds on screen in a few moods.
    private val tints = object : android.util.LruCache<Triple<WindClaySheet, Color, Color>, ImageBitmap>(16 shl 20) {
        override fun sizeOf(key: Triple<WindClaySheet, Color, Color>, value: ImageBitmap) = 4 * value.width * value.height
    }

    fun tinted(s: WindClaySheet, accent: Color, barretina: Color): ImageBitmap {
        val key = Triple(s, accent, if (s.cap != null) barretina else Color.Unspecified)
        return tints.get(key) ?: tintClay(s, accent, barretina).also { tints.put(key, it) }
    }
}

/**
 * [s] tinted, every frame: white where the body is not, the wind's colour
 * through the mask and the [barretina] colour through the cap mask, the clay
 * multiplied in and all of it cut to the clay's alpha. It is done once per
 * sheet and colour in a software canvas: tinting each frame in a GPU layer
 * left a dark square round the wind on phones.
 */
private fun tintClay(s: WindClaySheet, accent: Color, barretina: Color): ImageBitmap {
    val clay = s.sheet.asAndroidBitmap()
    val out = android.graphics.Bitmap.createBitmap(clay.width, clay.height, android.graphics.Bitmap.Config.ARGB_8888)
    val canvas = android.graphics.Canvas(out)
    val whole = android.graphics.Rect(0, 0, clay.width, clay.height)
    canvas.drawColor(android.graphics.Color.WHITE)
    fun mask(m: ImageBitmap, tint: Color) = canvas.drawBitmap(m.asAndroidBitmap(), null, whole, android.graphics.Paint(android.graphics.Paint.FILTER_BITMAP_FLAG).apply {
        colorFilter = android.graphics.PorterDuffColorFilter(tint.toArgb(), android.graphics.PorterDuff.Mode.SRC_IN)
    })
    mask(s.mask, accent)
    s.cap?.let { mask(it, barretina) }
    fun blend(mode: android.graphics.PorterDuff.Mode) = canvas.drawBitmap(clay, 0f, 0f, android.graphics.Paint().apply {
        xfermode = android.graphics.PorterDuffXfermode(mode)
    })
    blend(android.graphics.PorterDuff.Mode.MULTIPLY)
    blend(android.graphics.PorterDuff.Mode.DST_IN)
    return out.asImageBitmap()
}

private fun DrawScope.drawClay(s: WindClaySheet, frame: Int, accent: Color, barretina: Color, alpha: Float) {
    if (alpha <= 0f) return
    drawImage(WindClay.tinted(s, accent, barretina), IntOffset(frame * s.size, 0), IntSize(s.size, s.size),
        dstSize = IntSize(size.width.roundToInt(), size.height.roundToInt()), alpha = alpha)
}

/**
 * The clay wind of [shape] in [mood]; false (and nothing drawn) when its
 * sheet is missing, so the caller draws the Canvas face. The frame number is
 * a state read only while drawing: the sheet plays at its 12 fps without
 * recomposing anything, stops while the screen is not started, and with
 * [motion] off holds the sheet's still frame. A new mood crossfades over the last
 * frame of the old one. [barretina] tints the minairó's barretina.
 */
@Composable
internal fun WindClayFace(size: Int, mood: WindMood, accent: Color, barretina: Color, family: String, motion: Boolean, modifier: Modifier): Boolean {
    val assets = LocalContext.current.assets
    val dir = LocalWindClay.current
    val sheet = remember(dir, family, mood) { WindClay.sheet(assets, dir, family, mood) } ?: return false
    val shown = remember { object { var sheet: WindClaySheet? = null; var frame = 0 } }
    // Built in composition so the first frame of the new mood already fades in over the old one.
    val change = remember(sheet) {
        val from = shown.sheet?.takeIf { motion && it !== sheet }?.let { it to shown.frame }
        shown.sheet = sheet
        from to Animatable(if (from == null) 1f else 0f)
    }
    val (from, fade) = change
    if (from != null) LaunchedEffect(change) { fade.animateTo(1f, tween(220)) }
    val frame = remember(sheet) { mutableIntStateOf(if (motion) 0 else sheet.still) }
    var started by remember { mutableStateOf(true) }
    val owner = androidx.compose.ui.platform.LocalLifecycleOwner.current
    DisposableEffect(owner) {
        val obs = LifecycleEventObserver { _, _ -> started = owner.lifecycle.currentState.isAtLeast(Lifecycle.State.STARTED) }
        owner.lifecycle.addObserver(obs)
        onDispose { owner.lifecycle.removeObserver(obs) }
    }
    if (motion && started) LaunchedEffect(sheet) {
        // Back from the background it carries on from the frame it was on.
        val nanos = 1_000_000_000L / sheet.fps
        // A looping sheet never ends: like an infinite transition, tests and idling do not wait for it.
        suspend fun <R> tick(f: (Long) -> R) = if (sheet.loop) withInfiniteAnimationFrameNanos(f) else withFrameNanos(f)
        val start = tick { it } - frame.intValue * nanos
        while (true) {
            val n = tick { ((it - start) / nanos).toInt() }
            frame.intValue = if (sheet.loop) n % sheet.frames else minOf(n, sheet.frames - 1)
            if (!sheet.loop && n >= sheet.frames - 1) break
        }
    }
    Canvas(modifier.semantics { set(WindClayKey, sheet.name) }.size(size.dp)) {
        val f = frame.intValue
        shown.frame = f
        val mix = fade.value
        if (from != null && mix < 1f) drawClay(from.first, from.second, accent, barretina, 1f - mix)
        drawClay(sheet, f, accent, barretina, mix)
    }
    return true
}
