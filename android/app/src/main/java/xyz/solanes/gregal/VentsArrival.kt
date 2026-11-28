package xyz.solanes.gregal

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.CustomAccessibilityAction
import androidx.compose.ui.semantics.customActions
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlin.math.abs

/** Una novetat que acaba d'arribar amb l'app oberta; [at] distingeix dues novetats seguides del mateix vent. */
data class WindArrival(val wind: String, val name: String, val text: String, val messageId: String, val at: Long)

/** La primera línia amb text, sense les marques de markdown: «# Butlletí» → «Butlletí». */
internal fun arrivalPreview(text: String): String =
    text.lineSequence().map { it.trim().trimStart('#', '*', '>', '-', ' ').trim() }.firstOrNull { it.isNotEmpty() }.orEmpty().take(120)

/**
 * El bàner de dalt: «Ponent t'ha escrit: …». Tocar-lo obre el xat al
 * missatge; lliscar-lo (amunt o de costat) el treu, i marxa sol als 8 s.
 */
@Composable
internal fun WindArrivalBanner(vm: GregalViewModel, lang: String, winds: List<Wind>, modifier: Modifier = Modifier) {
    val arrival by vm.windArrival.collectAsState()
    // L'últim que s'ha ensenyat: perquè l'animació de sortida encara tingui què pintar.
    var shown by remember { mutableStateOf<WindArrival?>(null) }
    arrival?.let { shown = it }
    LaunchedEffect(arrival) {
        val a = arrival ?: return@LaunchedEffect
        kotlinx.coroutines.delay(8_000)
        vm.dismissWindArrival(a)
    }
    AnimatedVisibility(
        arrival != null, modifier.fillMaxWidth(),
        enter = slideInVertically(tween(260)) { -it } + fadeIn(tween(200)),
        exit = slideOutVertically(tween(200)) { -it } + fadeOut(tween(160)),
    ) {
        val a = shown ?: return@AnimatedVisibility
        val wind = winds.firstOrNull { it.id == a.wind }
        val name = a.name.ifBlank { wind?.name ?: a.wind }
        val dismiss = tr(lang, "app.vents.arrival.dismiss")
        val limit = with(LocalDensity.current) { 32.dp.toPx() }
        var dx by remember(a) { mutableFloatStateOf(0f) }
        var dy by remember(a) { mutableFloatStateOf(0f) }
        Surface(
            shape = RoundedCornerShape(20.dp), color = SuperficieOpaca, shadowElevation = 8.dp,
            modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp)
                .graphicsLayer { translationX = dx; translationY = minOf(dy, 0f) }
                .pointerInput(a) {
                    detectDragGestures(
                        onDragEnd = {
                            if (dy < -limit || abs(dx) > limit * 3) vm.dismissWindArrival(a) else { dx = 0f; dy = 0f }
                        },
                        onDragCancel = { dx = 0f; dy = 0f },
                    ) { change, drag -> change.consume(); dx += drag.x; dy += drag.y }
                }
                .clickable { vm.openWindArrival(a) }
                .semantics { customActions = listOf(CustomAccessibilityAction(dismiss) { vm.dismissWindArrival(a); true }) }
                .testTag("wind-arrival"),
        ) {
            Row(Modifier.padding(horizontal = 12.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                NewsBounce(a) {
                    WindHalo(wind?.let { Color(it.argb) } ?: Sea, wind?.faceStyle ?: windStyleOf("pso"), size = 30,
                        mood = WindMood.HAPPY, online = false)
                }
                Text(tr(lang, "app.vents.arrival", name, arrivalPreview(a.text)), color = Ink, fontSize = GregalSize.body,
                    fontWeight = FontWeight.Medium, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
            }
        }
    }
}

/** Un salt curt i content quan [key] canvia a una novetat nova; quiet sense animacions. */
@Composable
internal fun NewsBounce(key: Any?, content: @Composable () -> Unit) {
    val lift = remember { Animatable(0f) }
    val motion = windMotionEnabled()
    val height = with(LocalDensity.current) { 10.dp.toPx() }
    LaunchedEffect(key) {
        if (key == null || !motion) return@LaunchedEffect
        repeat(2) {
            lift.animateTo(-height, tween(130))
            lift.animateTo(0f, spring(dampingRatio = Spring.DampingRatioMediumBouncy, stiffness = Spring.StiffnessMedium))
        }
    }
    Box(Modifier.graphicsLayer { translationY = lift.value }) { content() }
}

/** La insígnia de novetats d'una cara: quantes (1-9, «9+»), res si no n'hi ha. */
@Composable
internal fun NewsBadge(n: Int, modifier: Modifier = Modifier) {
    val text = unreadBadge(n)
    if (text.isEmpty()) return
    Box(modifier.background(Sea, CircleShape).padding(horizontal = 6.dp, vertical = 1.dp)) {
        Text(text, color = OnSea, fontSize = GregalSize.label, fontWeight = FontWeight.Bold)
    }
}
