package xyz.solanes.gregal

import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.zIndex

/**
 * Peces espectaculars dels vents: halo amb resplendor, punts d'escriptura,
 * heroi de capçalera i píndoles. Tot amb la paleta Gregal + el color del vent.
 */

/** Cara amb halo: resplendor del seu color + ombra suau + punt en línia. */
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
        // Resplendor exterior del color del vent.
        Box(Modifier.size((size + 14).dp).background(accent.copy(alpha = 0.22f), CircleShape))
        Box(Modifier.size((size + 4).dp).background(accent.copy(alpha = 0.35f), CircleShape))
        Surface(
            shape = CircleShape,
            color = Superficie,
            shadowElevation = 6.dp,
            modifier = Modifier.size(size.dp),
        ) {
            Box(contentAlignment = Alignment.Center) {
                WindFace(size = size - 6, mood = mood, accent = accent, style = style, tappable = tappable)
            }
        }
        if (stacked.isNotEmpty()) {
            stacked.take(1).forEach {
                Box(Modifier.align(Alignment.BottomEnd).offset(x = 4.dp, y = 4.dp).zIndex(1f)) {
                    Surface(shape = CircleShape, color = Superficie, shadowElevation = 4.dp) {
                        WindFace(size = 26, accent = Color(it.argb), style = it.faceStyle)
                    }
                }
            }
            if (stacked.size > 1) {
                Box(
                    Modifier.align(Alignment.TopEnd).background(Ink, CircleShape)
                        .padding(horizontal = 5.dp, vertical = 1.dp).zIndex(1f)
                ) {
                    Text("+${stacked.size}", color = Superficie, fontSize = 9.sp, fontWeight = FontWeight.Bold)
                }
            }
        } else if (online) {
            // Insígnia de presència: punt verd amb anell clar, enganxat a la
            // vora de la cara (no un segon cercle flotant).
            Box(Modifier.align(Alignment.BottomEnd).offset(x = 3.dp, y = 3.dp).zIndex(1f)) {
                Surface(shape = CircleShape, color = Superficie, shadowElevation = 2.dp) {
                    Box(Modifier.padding(2.dp).size(8.dp).background(Green, CircleShape))
                }
            }
        }
    }
}

/** Tres punts que respiren quan el vent escriu. Estàtics als tests. */
@Composable
fun TypingDots(accent: Color = Sea) {
    if (!LocalWindMotion.current) {
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

/** Fons ambient d'un xat: dos respirs del color del vent. Estàtic als tests. */
@Composable
fun ChatAmbient(accent: Color) {
    Box(Modifier.fillMaxSize()) {
        if (!LocalWindMotion.current) {
            Box(Modifier.size(280.dp).offset(x = (-90).dp, y = (-60).dp).background(accent.copy(alpha = 0.10f), CircleShape))
            Box(Modifier.size(340.dp).align(Alignment.BottomEnd).offset(x = 110.dp, y = 120.dp).background(accent.copy(alpha = 0.07f), CircleShape))
            return@Box
        }
        val t = rememberInfiniteTransition(label = "ambient")
        val dx by t.animateFloat(0f, 36f, infiniteRepeatable(tween(12000, easing = FastOutSlowInEasing), RepeatMode.Reverse), label = "dx")
        val dy by t.animateFloat(0f, -28f, infiniteRepeatable(tween(17000, easing = FastOutSlowInEasing), RepeatMode.Reverse), label = "dy")
        Box(Modifier.size(280.dp).offset(x = (-90).dp + dx.dp, y = (-60).dp + dy.dp).background(accent.copy(alpha = 0.10f), CircleShape))
        Box(Modifier.size(340.dp).align(Alignment.BottomEnd).offset(x = (110).dp - dx.dp, y = 120.dp).background(accent.copy(alpha = 0.07f), CircleShape))
    }
}
/** Hero de Vents: marca viva + títol + cares apilades + mode. */
@Composable
fun VentsHero(lang: String, winds: List<Wind>, modeLabel: String) {
    Surface(
        shape = RoundedCornerShape(24.dp),
        shadowElevation = 8.dp,
        modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
    ) {
        Box(
            Modifier.background(
                Brush.linearGradient(
                    listOf(Sea.copy(alpha = 0.28f), Onada.copy(alpha = 0.18f), SuperficieAlta)
                )
            ).padding(16.dp)
        ) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                GregalMark(size = 56.dp, alive = LocalWindMotion.current)
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text(
                        tr(lang, "app.vents.title"),
                        fontSize = 24.sp, fontWeight = FontWeight.ExtraBold, color = Ink
                    )
                    Text(modeLabel, fontSize = 12.sp, color = Muted)
                    if (winds.isNotEmpty()) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            winds.take(4).forEachIndexed { i, w ->
                                Box(Modifier.offset(x = (-10 * i).dp).zIndex(4 - i.toFloat())) {
                                    Surface(shape = CircleShape, color = Superficie, shadowElevation = 3.dp) {
                                        WindFace(size = 28, accent = Color(w.argb), style = w.faceStyle)
                                    }
                                }
                            }
                            Text(
                                "  ${winds.size} · ${winds.take(3).joinToString(", ") { it.name }}",
                                fontSize = 12.sp, color = Muted, maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                        }
                    }
                }
            }
        }
    }
}

/** Pestanyes píndola: Converses / Els meus vents. */
@Composable
fun VentsPillTabs(lang: String, manage: Boolean, onManage: (Boolean) -> Unit) {
    Row(
        Modifier.padding(horizontal = 16.dp, vertical = 4.dp)
            .background(SuperficieAlta, RoundedCornerShape(999.dp))
            .padding(4.dp),
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        PillTab(
            label = tr(lang, "app.vents.conversations"), selected = !manage,
            modifier = Modifier.weight(1f), onClick = { onManage(false) },
        )
        PillTab(
            label = tr(lang, "app.vents.myWinds"), selected = manage,
            modifier = Modifier.weight(1f), onClick = { onManage(true) },
        )
    }
}

@Composable
private fun PillTab(label: String, selected: Boolean, modifier: Modifier = Modifier, onClick: () -> Unit) {
    Surface(
        shape = RoundedCornerShape(999.dp),
        color = if (selected) Sea else Color.Transparent,
        shadowElevation = if (selected) 4.dp else 0.dp,
        modifier = modifier.clip(RoundedCornerShape(999.dp)).clickable(onClick = onClick),
    ) {
        Box(contentAlignment = Alignment.Center, modifier = Modifier.padding(vertical = 8.dp)) {
            Text(
                label,
                fontSize = 13.sp,
                fontWeight = if (selected) FontWeight.Bold else FontWeight.Medium,
                color = if (selected) OnSea else Muted,
            )
        }
    }
}
