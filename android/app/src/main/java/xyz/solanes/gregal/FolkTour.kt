package xyz.solanes.gregal

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.slideInVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.shape.GenericShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.CheckCircle
import androidx.compose.material.icons.rounded.Event
import androidx.compose.material.icons.rounded.Language
import androidx.compose.material.icons.rounded.Lock
import androidx.compose.material.icons.rounded.Mail
import androidx.compose.material.icons.rounded.PhoneAndroid
import androidx.compose.material.icons.rounded.Dns
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.RoundRect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * What Gregal can do, told by the felt winds (after the 3D welcome): on each
 * page a character explains one idea in a comic bubble that types itself, and
 * a little window of the app does it live — a task turning into a published
 * website, the morning brief filling in, a job running while you are away,
 * everything staying at home. Swipe, Next or Skip; the last page hands over to
 * the server setup ([onDone]).
 */
internal sealed interface DemoLine {
    val delay: Long
}

/** What you type in the app. */
internal data class DemoYou(val text: String, override val delay: Long = 500) : DemoLine
/** A wind answering in the thread. */
internal data class DemoWind(val who: String, val text: String, override val delay: Long = 900) : DemoLine
/** A result card: icon, title and an optional line under it. */
internal data class DemoCard(val icon: ImageVector, val title: String, val sub: String = "", val good: Boolean = false, override val delay: Long = 900) : DemoLine
/** A job running: its label and a bar that fills. */
internal data class DemoWork(val text: String, override val delay: Long = 600) : DemoLine

internal data class FolkScene(val who: List<String>, val title: String, val say: String, val window: String, val lines: List<DemoLine>)

internal fun folkScenes(lang: String): List<FolkScene> = listOf(
    FolkScene(listOf("minairo"), "app.tour.work.title", "app.tour.work.say", "app.tour.work.window", listOf(
        DemoYou(tr(lang, "app.tour.work.you")),
        DemoWind("minairo", tr(lang, "app.tour.work.wind")),
        DemoWork(tr(lang, "app.tour.work.doing")),
        DemoCard(Icons.Rounded.Language, tr(lang, "app.tour.work.card"), tr(lang, "app.tour.work.cardSub"), good = true),
    )),
    FolkScene(listOf("dona"), "app.tour.day.title", "app.tour.day.say", "app.tour.day.window", listOf(
        DemoWind("dona", tr(lang, "app.tour.day.wind"), delay = 500),
        DemoCard(Icons.Rounded.Event, tr(lang, "app.tour.day.ev1"), tr(lang, "app.tour.day.ev1Sub"), delay = 700),
        DemoCard(Icons.Rounded.Event, tr(lang, "app.tour.day.ev2"), tr(lang, "app.tour.day.ev2Sub"), delay = 700),
        DemoCard(Icons.Rounded.Mail, tr(lang, "app.tour.day.mail"), tr(lang, "app.tour.day.mailSub"), delay = 700),
    )),
    FolkScene(listOf("tio"), "app.tour.keep.title", "app.tour.keep.say", "app.tour.keep.window", listOf(
        DemoYou(tr(lang, "app.tour.keep.you")),
        DemoWork(tr(lang, "app.tour.keep.doing")),
        DemoCard(Icons.Rounded.CheckCircle, tr(lang, "app.tour.keep.card"), tr(lang, "app.tour.keep.cardSub"), good = true, delay = 1600),
    )),
    FolkScene(listOf("minairo", "dona", "tio"), "app.tour.home.title", "app.tour.home.say", "app.tour.home.window", listOf(
        DemoCard(Icons.Rounded.PhoneAndroid, tr(lang, "app.tour.home.phone"), tr(lang, "app.tour.home.phoneSub"), delay = 500),
        DemoCard(Icons.Rounded.Dns, tr(lang, "app.tour.home.server"), tr(lang, "app.tour.home.serverSub"), delay = 700),
        DemoCard(Icons.Rounded.Lock, tr(lang, "app.tour.home.lock"), tr(lang, "app.tour.home.lockSub"), good = true, delay = 800),
    )),
)

@Composable
internal fun FolkTour(lang: String, onDone: () -> Unit, modifier: Modifier = Modifier) {
    val scenes = remember(lang) { folkScenes(lang) }
    val pager = rememberPagerState { scenes.size }
    val scope = rememberCoroutineScope()
    val motion = windMotionEnabled()
    Column(modifier.fillMaxWidth().testTag("folk-tour"), horizontalAlignment = Alignment.CenterHorizontally) {
        HorizontalPager(pager, Modifier.fillMaxWidth(), pageSpacing = 16.dp, verticalAlignment = Alignment.Top) { i ->
            FolkScenePage(lang, scenes[i], playing = pager.currentPage == i, motion = motion, modifier = Modifier.testTag("folk-tour-$i"))
        }
        Spacer(Modifier.height(12.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            scenes.indices.forEach { i ->
                val on = i == pager.currentPage
                Box(Modifier.size(width = if (on) 18.dp else 7.dp, height = 7.dp).background(if (on) Sea else Line, RoundedCornerShape(50)))
            }
        }
        Spacer(Modifier.height(12.dp))
        val last = pager.currentPage == scenes.lastIndex
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
            TextButton(onClick = onDone, modifier = Modifier.testTag("folk-tour-skip")) {
                Text(tr(lang, "app.intro.skip"), color = Muted, fontSize = GregalSize.small)
            }
            Button(onClick = { if (last) onDone() else scope.launch { pager.animateScrollToPage(pager.currentPage + 1) } },
                shape = RoundedCornerShape(14.dp), modifier = Modifier.testTag("folk-tour-next")) {
                Text(tr(lang, if (last) "app.tour.start" else "app.tour.next"), fontSize = GregalSize.body)
            }
        }
    }
}

/** One scene: the title, the little app window doing it, and the character saying it. */
@Composable
private fun FolkScenePage(lang: String, scene: FolkScene, playing: Boolean, motion: Boolean, modifier: Modifier = Modifier) {
    // How many lines of the window are showing; the bubble starts typing once the window has begun.
    var shown by remember { mutableIntStateOf(if (motion) 0 else scene.lines.size) }
    LaunchedEffect(playing) {
        if (!playing || !motion) return@LaunchedEffect
        shown = 0
        for ((n, line) in scene.lines.withIndex()) {
            delay(line.delay)
            shown = n + 1
        }
    }
    Column(modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
        Text(tr(lang, scene.title), color = Ink, style = GregalType.title, textAlign = TextAlign.Center)
        Spacer(Modifier.height(12.dp))
        DemoWindow(tr(lang, scene.window), scene.lines, shown, motion)
        Spacer(Modifier.height(14.dp))
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.Bottom) {
            Row(horizontalArrangement = Arrangement.spacedBy((-16).dp), verticalAlignment = Alignment.Bottom) {
                scene.who.forEach { FolkPortrait(it, if (scene.who.size == 1) 96 else 64) }
            }
            Spacer(Modifier.width(6.dp))
            ComicBubble(tr(lang, scene.say), playing = playing && motion, modifier = Modifier.weight(1f).padding(bottom = 30.dp))
        }
    }
}

/** A speech bubble with a tail towards the speaker (bottom left), typing its text. */
@Composable
private fun ComicBubble(text: String, playing: Boolean, modifier: Modifier = Modifier) {
    var chars by remember(text) { mutableIntStateOf(if (playing) 0 else text.length) }
    LaunchedEffect(text, playing) {
        if (!playing) { chars = text.length; return@LaunchedEffect }
        chars = 0
        delay(350)
        while (chars < text.length) { delay(22); chars++ }
    }
    val fill = Color.White
    val tail = 12.dp
    val shape = remember {
        GenericShape { size, _ ->
            val t = 12f
            addRoundRect(RoundRect(t * 1.6f, 0f, size.width, size.height, CornerRadius(42f, 42f)))
            moveTo(t * 1.6f + 6f, size.height - 34f)
            lineTo(0f, size.height - 4f)
            lineTo(t * 1.6f + 34f, size.height - 14f)
            close()
        }
    }
    Box(modifier.background(fill, shape).padding(start = tail + 10.dp, end = 14.dp, top = 10.dp, bottom = 12.dp).testTag("folk-bubble")) {
        // The full text keeps the bubble's size; the typed part is drawn on top of an invisible copy.
        Text(text, color = Color.Transparent, style = GregalType.bodySmall)
        Text(text.take(chars), color = Color(0xFF1B1F2E), style = GregalType.bodySmall, fontWeight = FontWeight.Medium)
    }
}

/** A little window of the app: a title bar with dots and the lines appearing one by one. */
@Composable
private fun DemoWindow(title: String, lines: List<DemoLine>, shown: Int, motion: Boolean) {
    Column(Modifier.fillMaxWidth().widthIn(max = 420.dp).heightIn(min = 236.dp)
        .background(Pagina, RoundedCornerShape(18.dp)).border(1.dp, Line, RoundedCornerShape(18.dp)).testTag("folk-window")) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 9.dp), verticalAlignment = Alignment.CenterVertically) {
            listOf(Color(0xFFFF6B6B), Color(0xFFFFC857), Color(0xFF59D98E)).forEach {
                Box(Modifier.size(8.dp).background(it, RoundedCornerShape(50)))
                Spacer(Modifier.width(5.dp))
            }
            Spacer(Modifier.width(6.dp))
            Text(title, color = Muted, style = GregalType.caption)
        }
        Box(Modifier.fillMaxWidth().height(1.dp).background(Line))
        Column(Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            lines.forEachIndexed { n, line ->
                AnimatedVisibility(n < shown, enter = if (motion) fadeIn(tween(280)) + slideInVertically(tween(280)) { it / 3 } else fadeIn(tween(0))) {
                    DemoRow(line, active = n == shown - 1, motion = motion)
                }
            }
        }
    }
}

@Composable
private fun DemoRow(line: DemoLine, active: Boolean, motion: Boolean) {
    when (line) {
        is DemoYou -> Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
            Surface(shape = RoundedCornerShape(16.dp, 16.dp, 4.dp, 16.dp), color = Sea) {
                Text(line.text, color = Color.White, style = GregalType.bodySmall, modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp))
            }
        }
        is DemoWind -> Row(verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            FolkPortrait(line.who, 26)
            Surface(shape = RoundedCornerShape(4.dp, 16.dp, 16.dp, 16.dp), color = SeaSoft(0.14f)) {
                Text(line.text, color = Ink, style = GregalType.bodySmall, modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp))
            }
        }
        is DemoWork -> Column(Modifier.fillMaxWidth()) {
            Text(line.text, color = Muted, style = GregalType.caption)
            Spacer(Modifier.height(5.dp))
            // The bar fills once, from empty, while the job runs.
            var started by remember { mutableIntStateOf(0) }
            LaunchedEffect(Unit) { started = 1 }
            val fill by animateFloatAsState(if (started == 1 || !motion) 1f else 0f, tween(if (motion) 1400 else 0), label = "work")
            LinearProgressIndicator(progress = { fill }, modifier = Modifier.fillMaxWidth().height(6.dp), color = Sea, trackColor = Line)
        }
        is DemoCard -> Surface(shape = RoundedCornerShape(14.dp), color = if (line.good) Color(0x2259D98E) else SeaSoft(0.08f),
            modifier = Modifier.fillMaxWidth().border(1.dp, if (line.good) Color(0x6659D98E) else Line, RoundedCornerShape(14.dp))) {
            Row(Modifier.padding(horizontal = 12.dp, vertical = 9.dp), verticalAlignment = Alignment.CenterVertically) {
                Icon(line.icon, null, tint = if (line.good) Color(0xFF59D98E) else Sea, modifier = Modifier.size(20.dp))
                Spacer(Modifier.width(10.dp))
                Column {
                    Text(line.title, color = Ink, style = GregalType.bodySmall, fontWeight = FontWeight.SemiBold)
                    if (line.sub.isNotBlank()) Text(line.sub, color = Muted, style = GregalType.caption)
                }
            }
        }
    }
}
