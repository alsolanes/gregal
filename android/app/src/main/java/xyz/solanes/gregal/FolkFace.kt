package xyz.solanes.gregal

import android.content.res.AssetManager
import android.graphics.BitmapFactory
import android.graphics.PorterDuff
import android.graphics.PorterDuffColorFilter
import android.graphics.PorterDuffXfermode
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.keyframes
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.drawscope.withTransform
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp

/**
 * Llegendes (folk): each wind is one of three felt characters rendered by
 * tools/characters/felt.py and copied into the APK assets (characters/folk/).
 * Three layers: the body without eyes, a mask where the wind's colour is
 * multiplied (the minairó's body, the dona d'aigua's hair, the tió's blanket),
 * and the eyes alone, which blink and now and then glance. As on the web
 * (winds.js folk()), the same wind is always the same character.
 */
internal val folkCast = listOf("minairo", "dona", "tio")

/** winds.js idHash(w) % 3: the unsigned 32-bit h * 31 + c over the id, which is String.hashCode's bits. */
internal fun folkCharacter(windId: String): String =
    folkCast[(Integer.toUnsignedLong(windId.hashCode()) % folkCast.size).toInt()]

/** Where each character's eyes sit, as a fraction of the square (the blink squeezes towards it). */
private val folkEyeY = mapOf("minairo" to .643f, "dona" to .576f, "tio" to .625f)

private object FolkAssets {
    private val raw = HashMap<String, ImageBitmap?>()

    // A tinted body is 256 KB at 256 px: plenty of winds fit.
    private val tinted = object : android.util.LruCache<Pair<String, Int>, ImageBitmap>(16 shl 20) {
        override fun sizeOf(key: Pair<String, Int>, value: ImageBitmap) = 4 * value.width * value.height
    }

    fun image(assets: AssetManager, file: String): ImageBitmap? = raw.getOrPut(file) {
        try {
            assets.open("characters/folk/$file").use { BitmapFactory.decodeStream(it) }?.asImageBitmap()
        } catch (e: java.io.IOException) {
            android.util.Log.w("FolkFace", "missing characters/folk/$file: ${e.message}")
            null
        }
    }

    /** The body with the wind's colour multiplied over its mask, once per character and colour. */
    fun body(assets: AssetManager, who: String, accent: Color): ImageBitmap? {
        val key = who to accent.toArgb()
        tinted.get(key)?.let { return it }
        val body = image(assets, "$who.webp")?.asAndroidBitmap() ?: return null
        val mask = image(assets, "$who.mask.webp")?.asAndroidBitmap() ?: return null
        val out = body.copy(android.graphics.Bitmap.Config.ARGB_8888, true)
        val canvas = android.graphics.Canvas(out)
        // The mask in the wind's colour, then multiplied onto the body where the mask is.
        val tint = android.graphics.Bitmap.createBitmap(mask.width, mask.height, android.graphics.Bitmap.Config.ARGB_8888)
        android.graphics.Canvas(tint).drawBitmap(mask, 0f, 0f, android.graphics.Paint().apply {
            colorFilter = PorterDuffColorFilter(accent.toArgb(), PorterDuff.Mode.SRC_IN)
        })
        val dst = android.graphics.Rect(0, 0, out.width, out.height)
        canvas.drawBitmap(tint, null, dst, android.graphics.Paint(android.graphics.Paint.FILTER_BITMAP_FLAG).apply {
            xfermode = PorterDuffXfermode(PorterDuff.Mode.MULTIPLY)
        })
        // MULTIPLY also multiplies alpha where the body is clear: keep the body's own outline.
        canvas.drawBitmap(body, 0f, 0f, android.graphics.Paint().apply { xfermode = PorterDuffXfermode(PorterDuff.Mode.DST_IN) })
        return out.asImageBitmap().also { tinted.put(key, it) }
    }
}

/** A felt face, or false when the character's files are not in the APK (the caller draws Cares). */
@Composable
internal fun FolkFace(size: Int, mood: WindMood, accent: Color, windId: String, motion: Boolean, modifier: Modifier): Boolean {
    val assets = LocalContext.current.assets
    val who = remember(windId) { folkCharacter(windId) }
    val body = remember(who, accent) { FolkAssets.body(assets, who, accent) } ?: return false
    val eyes = remember(who) { FolkAssets.image(assets, "$who.eyes.webp") } ?: return false
    // Only the eyes move: a blink every ~6.4 s and a small glance every ~13 s, each wind on its own phase.
    val phase = remember(windId) { (Integer.toUnsignedLong(windId.hashCode()) % 6400).toInt() }
    val blink: Float
    val glance: Float
    if (motion) {
        val loop = rememberInfiniteTransition(label = "folk")
        blink = loop.animateFloat(1f, 1f, infiniteRepeatable(keyframes {
            durationMillis = 6400
            1f at 5950
            .08f at 6110
            1f at 6270
        }, RepeatMode.Restart, androidx.compose.animation.core.StartOffset(phase)), label = "blink").value
        glance = loop.animateFloat(0f, 0f, infiniteRepeatable(keyframes {
            durationMillis = 13000
            0f at 4900 using LinearEasing
            1f at 5700
            1f at 7300
            0f at 8100
        }, RepeatMode.Restart, androidx.compose.animation.core.StartOffset(phase * 2)), label = "glance").value
    } else {
        blink = 1f
        glance = 0f
    }
    val side = if (Integer.toUnsignedLong(windId.hashCode()) % 2 == 0L) 1f else -1f
    Canvas(modifier.size(size.dp)) {
        val px = this.size.width.toInt()
        val whole = IntSize(px, px)
        drawImage(body, srcSize = IntSize(body.width, body.height), dstSize = whole, filterQuality = androidx.compose.ui.graphics.FilterQuality.High)
        // Thinking looks up and aside, a failed turn looks down; otherwise the idle glance.
        val (dx, dy) = when (mood) {
            WindMood.THINKING, WindMood.SEARCHING, WindMood.READING -> .015f to -.018f
            WindMood.FAILED -> 0f to .02f
            else -> .025f * glance * side to 0f
        }
        val squeeze = if (mood == WindMood.DONE || mood == WindMood.HAPPY) minOf(blink, .55f) else blink
        withTransform({
            translate(dx * px, dy * px)
            scale(1f, squeeze, Offset(px / 2f, px * (folkEyeY[who] ?: .62f)))
        }) {
            drawImage(eyes, srcSize = IntSize(eyes.width, eyes.height), dstOffset = IntOffset.Zero, dstSize = whole,
                filterQuality = androidx.compose.ui.graphics.FilterQuality.High)
        }
    }
    return true
}
