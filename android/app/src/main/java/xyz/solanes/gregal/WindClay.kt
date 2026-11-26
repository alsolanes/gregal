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
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.VectorPainter
import androidx.compose.ui.graphics.vector.rememberVectorPainter
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
    // Decoded once per sheet; at 256 px (157) a sheet of 24 frames and its mask are 8 MB of
    // pixels, so this keeps a few winds in a few moods and the least used go.
    private val sheets = object : android.util.LruCache<String, WindClaySheet>(48 shl 20) {
        override fun sizeOf(key: String, value: WindClaySheet) =
            4 * (value.sheet.width * value.sheet.height + value.mask.width * value.mask.height * (if (value.cap != null) 2 else 1))
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

    private data class Tint(val sheet: WindClaySheet, val accent: Color, val barretina: Color, val still: Boolean)

    // A tinted sheet is as big as the sheet, up to 6 MB at 256 px: a few winds playing in a
    // few moods. A still face tints only its still frame, 256 KB, so lists of them fit too.
    private val tints = object : android.util.LruCache<Tint, ImageBitmap>(32 shl 20) {
        override fun sizeOf(key: Tint, value: ImageBitmap) = 4 * value.width * value.height
    }

    /** [s] tinted, the whole sheet, or with [still] only its still frame. */
    fun tinted(s: WindClaySheet, accent: Color, barretina: Color, still: Boolean): ImageBitmap {
        val key = Tint(s, accent, if (s.cap != null) barretina else Color.Unspecified, still)
        return tints.get(key) ?: tintClay(s, accent, barretina, still).also { tints.put(key, it) }
    }
}

/**
 * [s] tinted, every frame or with [still] only the still one: white where the body is not, the wind's colour
 * through the mask and the [barretina] colour through the cap mask, the clay
 * multiplied in and all of it cut to the clay's alpha. It is done once per
 * sheet and colour in a software canvas: tinting each frame in a GPU layer
 * left a dark square round the wind on phones.
 */
private fun tintClay(s: WindClaySheet, accent: Color, barretina: Color, still: Boolean): ImageBitmap {
    val clay = s.sheet.asAndroidBitmap()
    val (first, frames) = if (still) s.still to 1 else 0 to s.frames
    val out = android.graphics.Bitmap.createBitmap(frames * s.size, s.size, android.graphics.Bitmap.Config.ARGB_8888)
    val canvas = android.graphics.Canvas(out)
    val whole = android.graphics.Rect(0, 0, out.width, out.height)
    canvas.drawColor(android.graphics.Color.WHITE)
    fun mask(m: ImageBitmap, tint: Color) = canvas.drawBitmap(m.asAndroidBitmap(),
        android.graphics.Rect(first * s.maskSize, 0, (first + frames) * s.maskSize, s.maskSize), whole,
        android.graphics.Paint(android.graphics.Paint.FILTER_BITMAP_FLAG).apply {
            colorFilter = android.graphics.PorterDuffColorFilter(tint.toArgb(), android.graphics.PorterDuff.Mode.SRC_IN)
        })
    mask(s.mask, accent)
    s.cap?.let { mask(it, barretina) }
    val frame = android.graphics.Rect(first * s.size, 0, (first + frames) * s.size, s.size)
    fun blend(mode: android.graphics.PorterDuff.Mode) = canvas.drawBitmap(clay, frame, whole, android.graphics.Paint().apply {
        xfermode = android.graphics.PorterDuffXfermode(mode)
    })
    blend(android.graphics.PorterDuff.Mode.MULTIPLY)
    blend(android.graphics.PorterDuff.Mode.DST_IN)
    return out.asImageBitmap()
}

/** With [still] it draws the still frame from a tint of that frame alone. */
private fun DrawScope.drawClay(s: WindClaySheet, frame: Int, accent: Color, barretina: Color, alpha: Float, still: Boolean) {
    if (alpha <= 0f) return
    drawImage(WindClay.tinted(s, accent, barretina, still), IntOffset(if (still) 0 else frame * s.size, 0), IntSize(s.size, s.size),
        dstSize = IntSize(size.width.roundToInt(), size.height.roundToInt()), alpha = alpha)
}

/** 159: a wind's trade on its clay figure, the trade chip's icon on a white disc at the bottom-right corner, beside its feet (as on the web). */
private fun DrawScope.drawBadge(icon: VectorPainter, accent: Color) {
    val r = size.width * .15f
    val centre = Offset(size.width * .82f, size.height * .8f)
    drawCircle(Color.White, r, centre)
    drawCircle(accent, r, centre, style = Stroke(size.width * .012f))
    val side = r * 1.25f
    translate(centre.x - side / 2, centre.y - side / 2) {
        with(icon) { draw(Size(side, side), colorFilter = ColorFilter.tint(Color(0xFF262040))) }
    }
}

private val waitBubble = PathParser().parsePathString("M4 1h8a3 3 0 0 1 3 3v5a3 3 0 0 1-3 3H7l-3 3v-3a3 3 0 0 1-3-3V4a3 3 0 0 1 3-3Z").toPath()
private val waitAsk = PathParser().parsePathString("M6.2 5a1.9 1.9 0 1 1 2.6 1.8c-.6.3-.8.7-.8 1.3M8 10.2h.01").toPath()

/** 168: a wind waiting for you says so on the figure, a «?» bubble at the top-right corner, white with the wind's colour like [drawBadge], which keeps the bottom (as on the web). */
private fun DrawScope.drawWaiting(accent: Color) {
    val unit = size.width * .26f / 16f
    val edge = size.width * .012f / unit
    translate(size.width * .7f, size.height * .04f) {
        scale(unit, unit, Offset.Zero) {
            drawPath(waitBubble, Color.White)
            drawPath(waitBubble, accent, style = Stroke(edge, join = StrokeJoin.Round))
            drawPath(waitAsk, Color(0xFF262040), style = Stroke(1.6f, cap = StrokeCap.Round, join = StrokeJoin.Round))
        }
    }
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
internal fun WindClayFace(size: Int, mood: WindMood, accent: Color, barretina: Color, family: String, motion: Boolean, modifier: Modifier,
    badge: ImageVector? = null): Boolean {
    val assets = LocalContext.current.assets
    val dir = LocalWindClay.current
    val sheet = remember(dir, family, mood) { WindClay.sheet(assets, dir, family, mood) } ?: return false
    val icon = badge?.let { rememberVectorPainter(it) }
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
        if (from != null && mix < 1f) drawClay(from.first, from.second, accent, barretina, 1f - mix, false)
        drawClay(sheet, f, accent, barretina, mix, !motion)
        if (icon != null) drawBadge(icon, accent)
        if (mood == WindMood.WAITING) drawWaiting(accent)
    }
    return true
}
