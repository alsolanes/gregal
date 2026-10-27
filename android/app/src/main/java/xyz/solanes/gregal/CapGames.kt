package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import kotlinx.coroutines.delay
import kotlin.random.Random

/** Un núvol del joc: on surt, quan neix i si és d'or (val 3). */
internal data class CapCloud(val id: Int, val x: Float, val born: Long, val gold: Boolean, val life: Long)

/** Posició vertical d'un núvol (1 = a baix, 0 = a dalt); null si ja ha marxat. */
internal fun cloudY(c: CapCloud, now: Long): Float? {
    val t = (now - c.born).toFloat() / c.life
    return if (t < 0f || t > 1f) null else 1f - t
}

/**
 * «Atrapa els núvols»: 20 segons, els núvols pugen i s'han de tocar abans
 * que marxin; els d'or valen 3. La mascota anima des de dalt.
 */
@Composable
fun CapCloudGame(lang: String, st: CapState, onDone: (Int) -> Unit) {
    val total = 20_000L
    var start by remember { mutableStateOf(0L) }
    var now by remember { mutableStateOf(0L) }
    var score by remember { mutableStateOf(0) }
    val clouds = remember { mutableStateListOf<CapCloud>() }
    var nextId by remember { mutableStateOf(0) }
    var over by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) {
        start = System.currentTimeMillis()
        var lastSpawn = 0L
        while (true) {
            withFrameMillis { }
            now = System.currentTimeMillis()
            val left = total - (now - start)
            if (left <= 0) { over = true; break }
            // Cada cop més de pressa: de 750 ms a 420 ms entre núvols.
            val gap = 750 - (330 * (now - start) / total)
            if (now - lastSpawn > gap) {
                lastSpawn = now
                clouds += CapCloud(nextId++, Random.nextFloat() * .8f + .1f, now, Random.nextInt(8) == 0, Random.nextLong(2200, 3400))
            }
            clouds.removeAll { cloudY(it, now) == null }
        }
    }
    Dialog(onDismissRequest = { onDone(-1) }) {
        Surface(shape = RoundedCornerShape(24.dp), color = Superficie) {
            Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    WindFace(size = 40, accent = Color(st.argb), style = st.faceStyle, mood = if (over) WindMood.HAPPY else WindMood.TALKING)
                    Column(Modifier.weight(1f)) {
                        Text(tr(lang, "app.cap.game.nuvols"), color = Ink, fontWeight = FontWeight.Bold)
                        Text(if (over) tr(lang, "app.cap.game.over", score) else tr(lang, "app.cap.game.nuvolsHelp"), color = Muted, fontSize = 12.sp)
                    }
                    Text("$score", color = Sea, fontSize = 22.sp, fontWeight = FontWeight.ExtraBold, modifier = Modifier.testTag("game-score"))
                }
                val left = ((total - (now - start)).coerceAtLeast(0) / 1000).toInt()
                LinearProgressIndicator(progress = { if (start == 0L) 1f else (total - (now - start)).coerceAtLeast(0).toFloat() / total },
                    modifier = Modifier.fillMaxWidth(), color = Sea)
                BoxWithConstraints(Modifier.fillMaxWidth().height(320.dp).clip(RoundedCornerShape(18.dp))
                    .background(Color(0xFF8EC5FF).copy(alpha = .25f)).testTag("game-sky")) {
                    val w = maxWidth; val h = maxHeight
                    clouds.toList().forEach { c ->
                        val y = cloudY(c, now) ?: return@forEach
                        key(c.id) {
                            Text(if (c.gold) "⭐" else "☁️", fontSize = if (c.gold) 30.sp else 36.sp,
                                modifier = Modifier.offset(x = w * c.x - 22.dp, y = (h - 44.dp) * y)
                                    .clip(CircleShape)
                                    .clickable(enabled = !over) { score += if (c.gold) 3 else 1; clouds.remove(c) }
                                    .padding(4.dp))
                        }
                    }
                    if (over) Text("🏁", fontSize = 48.sp, modifier = Modifier.align(Alignment.Center))
                    else Text("${left}s", color = Muted, fontSize = 12.sp, modifier = Modifier.align(Alignment.TopEnd).padding(8.dp))
                }
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    if (over) Button(onClick = { onDone(score) }, colors = ButtonDefaults.buttonColors(containerColor = Sea, contentColor = OnSea),
                        modifier = Modifier.testTag("game-done")) { Text(tr(lang, "app.cap.game.done")) }
                    else TextButton(onClick = { onDone(-1) }) { Text(tr(lang, "app.cap.game.quit"), color = Muted) }
                }
            }
        }
    }
}

/** Qui guanya a pedra, paper, tisora: 1 tu, -1 ella, 0 empat. */
internal fun rpsWinner(you: Int, her: Int): Int = when ((you - her + 3) % 3) { 0 -> 0; 1 -> 1; else -> -1 }

/** Pedra, paper, tisora a tres rondes contra la mascota. */
@Composable
fun CapRpsGame(lang: String, st: CapState, onDone: (Int) -> Unit) {
    val hands = listOf("✊", "✋", "✌️") // pedra, paper, tisora (en aquest ordre: cadascuna guanya l'anterior)
    var round by remember { mutableStateOf(0) }
    var you by remember { mutableStateOf(0) }
    var her by remember { mutableStateOf(0) }
    var last by remember { mutableStateOf<Pair<Int, Int>?>(null) }
    var thinking by remember { mutableStateOf(false) }
    val over = round >= 3
    LaunchedEffect(thinking) { if (thinking) { delay(700); thinking = false } }
    Dialog(onDismissRequest = { onDone(-1) }) {
        Surface(shape = RoundedCornerShape(24.dp), color = Superficie) {
            Column(Modifier.padding(18.dp), verticalArrangement = Arrangement.spacedBy(12.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                Text(tr(lang, "app.cap.game.ppt"), color = Ink, fontWeight = FontWeight.Bold)
                Text(tr(lang, "app.cap.game.pptScore", you, her), color = Muted, fontSize = 13.sp, modifier = Modifier.testTag("rps-score"))
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(18.dp)) {
                    Text(last?.let { hands[it.first] } ?: "❔", fontSize = 44.sp)
                    Text("vs", color = Muted)
                    Box(contentAlignment = Alignment.Center) {
                        WindFace(size = 54, accent = Color(st.argb), style = st.faceStyle,
                            mood = when { thinking -> WindMood.THINKING; last == null -> WindMood.IDLE; rpsWinner(last!!.first, last!!.second) < 0 -> WindMood.HAPPY; else -> WindMood.TALKING })
                        if (!thinking) last?.let { Text(hands[it.second], fontSize = 26.sp, modifier = Modifier.align(Alignment.BottomEnd)) }
                    }
                }
                val result = last?.let { rpsWinner(it.first, it.second) }
                Text(
                    when {
                        over && you > her -> tr(lang, "app.cap.game.youWin")
                        over && you < her -> tr(lang, "app.cap.game.sheWins", st.name)
                        over -> tr(lang, "app.cap.game.tie")
                        thinking -> "…"
                        result == 1 -> tr(lang, "app.cap.game.roundYou")
                        result == -1 -> tr(lang, "app.cap.game.roundHer")
                        result == 0 -> tr(lang, "app.cap.game.roundTie")
                        else -> tr(lang, "app.cap.game.pptHelp")
                    }, color = Tinta, fontSize = 14.sp, textAlign = TextAlign.Center,
                )
                if (!over) Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    hands.forEachIndexed { i, h ->
                        Surface(onClick = {
                            if (thinking) return@Surface
                            val pick = Random.nextInt(3)
                            last = i to pick
                            when (rpsWinner(i, pick)) { 1 -> you++; -1 -> her++ }
                            round++
                            thinking = true
                        }, shape = CircleShape, color = SuperficieAlta,
                            modifier = Modifier.size(62.dp).semantics { contentDescription = h }.testTag("rps-$i")) {
                            Box(contentAlignment = Alignment.Center) { Text(h, fontSize = 30.sp) }
                        }
                    }
                }
                if (over) Button(onClick = { onDone(you * 4 + (3 - you - her)) }, colors = ButtonDefaults.buttonColors(containerColor = Sea, contentColor = OnSea),
                    modifier = Modifier.testTag("game-done")) { Text(tr(lang, "app.cap.game.done")) }
                else TextButton(onClick = { onDone(-1) }) { Text(tr(lang, "app.cap.game.quit"), color = Muted) }
            }
        }
    }
}

/** L'armari: la roba guanyada es pot posar; la resta diu com es guanya. */
@Composable
fun CapWardrobe(lang: String, st: CapState, fresh: List<String>, onWear: (String) -> Unit, onClose: () -> Unit) {
    val pet = st.pet
    Dialog(onDismissRequest = onClose) {
        Surface(shape = RoundedCornerShape(24.dp), color = Superficie) {
            Column(Modifier.padding(18.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Box {
                        WindFace(size = 48, accent = Color(st.argb), style = st.faceStyle, mood = WindMood.HAPPY)
                        capOutfitCatalog.firstOrNull { it.id == pet.outfit }?.let {
                            Text(it.emoji, fontSize = 20.sp, modifier = Modifier.align(if (it.spot == "top") Alignment.TopCenter else Alignment.BottomCenter))
                        }
                    }
                    Column(Modifier.weight(1f)) {
                        Text(tr(lang, "app.cap.wardrobe"), color = Ink, fontWeight = FontWeight.Bold)
                        Text(tr(lang, "app.cap.wardrobeHelp", pet.level, pet.levelXp, pet.levelNeed), color = Muted, fontSize = 12.sp)
                    }
                }
                capOutfitCatalog.chunked(4).forEach { row ->
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        row.forEach { o ->
                            val owned = o.id in pet.outfits
                            val worn = o.id == pet.outfit
                            Surface(onClick = { if (owned) onWear(if (worn) "" else o.id) }, enabled = owned,
                                shape = RoundedCornerShape(14.dp),
                                color = if (worn) Sea.copy(alpha = .3f) else SuperficieAlta.copy(alpha = if (owned) .8f else .35f),
                                modifier = Modifier.size(64.dp).testTag("outfit-${o.id}")) {
                                Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
                                    Text(if (owned) o.emoji else "🔒", fontSize = 24.sp)
                                    Text(
                                        when {
                                            o.id in fresh -> tr(lang, "app.cap.new")
                                            owned -> ""
                                            o.month > 0 -> java.time.Month.of(o.month).getDisplayName(java.time.format.TextStyle.SHORT, java.util.Locale.forLanguageTag(if (lang == "en") "en" else "ca"))
                                            else -> tr(lang, "app.cap.level", o.level)
                                        }, color = if (o.id in fresh) Sea else Muted, fontSize = 10.sp, maxLines = 1)
                                }
                            }
                        }
                    }
                }
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                    TextButton(onClick = onClose) { Text(tr(lang, "app.cap.game.done"), color = Sea) }
                }
            }
        }
    }
}
