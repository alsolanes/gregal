package xyz.solanes.gregal

import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.zIndex

/** Shared minimal mascot wrappers and truthful activity indicators. */

/** Shared mascot wrapper; source-compatible name, no halo or invented presence. */
@Composable
fun WindHalo(
    accent: Color,
    style: WindStyle,
    size: Int = 48,
    mood: WindMood = WindMood.IDLE,
    online: Boolean = true,
    stacked: List<Wind> = emptyList(),
    tappable: Boolean = false,
) {
    Box(contentAlignment = Alignment.Center, modifier = Modifier.size((size + 14).dp)) {
        // Same footprint as before, without rings or shadows around the silhouette.
        WindFace(size = size + 8, mood = mood, accent = accent, style = style, tappable = tappable)
        if (stacked.isNotEmpty()) {
            stacked.take(1).forEach {
                Box(Modifier.align(Alignment.BottomEnd).offset(x = 4.dp, y = 4.dp).zIndex(1f)) {
                    Surface(shape = CircleShape, color = Superficie) {
                        WindFace(size = 26, accent = Color(it.argb), style = it.faceStyle)
                    }
                }
            }
            if (stacked.size > 1) {
                // A l'esquerra: la cantonada dreta és per a la insígnia d'estat de la cara.
                Box(
                    Modifier.align(Alignment.TopStart).background(Ink, CircleShape)
                        .padding(horizontal = 5.dp, vertical = 1.dp).zIndex(1f)
                ) {
                    Text("+${stacked.size - 1}", color = Superficie, fontSize = 9.sp, fontWeight = FontWeight.Bold)
                }
            }
        }
        // `online` stays source-compatible; a decorative avatar is not proof of presence.
    }
}

/** Tres punts que respiren quan el vent escriu. Estàtics als tests. */
@Composable
fun TypingDots(accent: Color = Sea) {
    if (!windMotionEnabled()) {
        Text("•••", color = accent, fontSize = 16.sp, fontWeight = FontWeight.Bold)
        return
    }
    val t = rememberInfiniteTransition(label = "typing")
    Row(horizontalArrangement = Arrangement.spacedBy(4.dp), verticalAlignment = Alignment.CenterVertically) {
        for (i in 0 until 3) {
            val a by t.animateFloat(
                initialValue = 0.25f, targetValue = 1f,
                animationSpec = infiniteRepeatable(
                    tween(900, delayMillis = i * 180, easing = FastOutSlowInEasing), RepeatMode.Reverse
                ), label = "dot$i"
            )
            Box(Modifier.size(7.dp).background(accent.copy(alpha = 0.35f + 0.65f * a), CircleShape))
        }
    }
}
