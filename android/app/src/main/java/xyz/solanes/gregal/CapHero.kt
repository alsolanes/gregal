package xyz.solanes.gregal

import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Send
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * El perfil del Ventet: la criatura de companyia. Aquí se l'alimenta, s'hi
 * juga i se la vesteix; la conversa (VentetConversation) queda per parlar i
 * delegar. Les animacions són locals; l'estat (alegria, energia, gana,
 * nivell, roba) és al servidor.
 */
@OptIn(ExperimentalFoundationApi::class, ExperimentalMaterial3Api::class)
@Composable
fun VentetProfile(vm: GregalViewModel, lang: String, onClose: () -> Unit) {
    val ui by vm.cap.collectAsState()
    val st = ui.state
    val pet = st.pet
    val accent = Color(st.argb)
    var renaming by remember { mutableStateOf(false) }
    var game by remember { mutableStateOf<String?>(null) }
    var wardrobe by remember { mutableStateOf(false) }
    // Partícules que surten de la cara: cors en mimar-la, molles en menjar…
    val bursts = remember { mutableStateListOf<CapBurst>() }
    fun burst(emoji: String, n: Int = 5) {
        val now = System.nanoTime()
        repeat(n) { i -> bursts += CapBurst(now + i, emoji, (i - n / 2) * 14f + (-6..6).random(), i * 70L) }
    }
    val now by produceState(System.currentTimeMillis()) { while (true) { delay(1000); value = System.currentTimeMillis() } }
    val hour = remember(now / 600_000) { java.time.LocalTime.now().hour }
    val asleep = (pet.tired || hour >= 23 || hour < 7) && now - ui.sayAt > 6_000
    LaunchedEffect(ui.levelUpAt) { if (ui.levelUpAt > 0 && now - ui.levelUpAt < 5_000) burst("🎉", 9) }
    val mood = if (now - ui.sayAt < 4_000) WindMood.HAPPY else WindMood.IDLE
    val says = when {
        ui.say.isNotBlank() && now - ui.sayAt < 6_000 -> ui.say
        asleep -> tr(lang, "app.cap.sleepy")
        pet.hungry -> tr(lang, "app.cap.hungry")
        else -> tr(lang, "app.cap.profileHello")
    }
    ModalBottomSheet(onDismissRequest = onClose, containerColor = Superficie) {
        Column(
            Modifier.fillMaxWidth().padding(horizontal = 20.dp).padding(bottom = 28.dp).testTag("ventet-profile"),
            horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            CapCreature(st, mood, asleep, bursts, onTap = { vm.capPlay("pet"); burst("💛") }, onLongTap = { renaming = true })
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(st.name, color = Ink, fontWeight = FontWeight.Bold, fontSize = 22.sp,
                    modifier = Modifier.combinedClickable(onClick = { renaming = true }, onLongClick = { renaming = true }))
                Surface(color = accent.copy(alpha = .35f), shape = RoundedCornerShape(10.dp)) {
                    Text(tr(lang, "app.cap.level", pet.level), color = Ink, fontSize = 12.sp, fontWeight = FontWeight.Bold,
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp))
                }
            }
            Text(tr(lang, "app.cap.role"), color = Muted, fontSize = 14.sp)
            Surface(color = SuperficieAlta.copy(alpha = .6f), shape = RoundedCornerShape(18.dp)) {
                Text(says, color = Tinta, fontSize = 15.sp, lineHeight = 21.sp,
                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp).testTag("cap-says"))
            }
            CapStats(lang, pet, accent)
            CapActions(lang, pet, asleep,
                onFeed = { snack -> vm.capPlay("feed:$snack"); burst(when (snack) { "poma" -> "🍎"; "cafe" -> "☕"; else -> "🍪" }, 4) },
                onPet = { vm.capPlay("pet"); burst("💛") },
                onPlay = { game = it },
                onWardrobe = { wardrobe = true },
                onSleep = { vm.capPlay("sleep"); burst("💤", 3) })
            TextButton(onClick = { renaming = true }) { Text(tr(lang, "app.cap.renameAction"), color = Muted) }
        }
    }
    if (renaming) {
        var name by remember { mutableStateOf(st.name) }
        AlertDialog(
            onDismissRequest = { renaming = false },
            title = { Text(tr(lang, "app.cap.rename")) },
            text = { OutlinedTextField(name, { name = it.take(24) }, singleLine = true) },
            confirmButton = { TextButton(enabled = name.isNotBlank(), onClick = { vm.renameCap(name); renaming = false }) { Text(tr(lang, "app.cap.renameOk"), color = Sea) } },
            dismissButton = { TextButton(onClick = { renaming = false }) { Text("✕", color = Muted) } },
        )
    }
    when (game) {
        "nuvols" -> CapCloudGame(lang, st) { score -> game = null; if (score >= 0) { vm.capPlay("game:nuvols:$score"); burst("⭐", 6) } }
        "ppt" -> CapRpsGame(lang, st) { score -> game = null; if (score >= 0) { vm.capPlay("game:ppt:$score"); burst("✨", 6) } }
    }
    if (wardrobe) CapWardrobe(lang, st, ui.unlocked, onWear = { vm.capPlay("wear:$it") }) { wardrobe = false }
}

/** Una partícula: emoji que surt de la cara, puja i s'esvaeix. */
data class CapBurst(val id: Long, val emoji: String, val dx: Float, val delayMs: Long)

/** La cara viva: respira, sura, salta, dorm, porta roba i treu partícules. */
@OptIn(ExperimentalFoundationApi::class)
@Composable
internal fun CapCreature(st: CapState, mood: WindMood, asleep: Boolean, bursts: MutableList<CapBurst>,
                        onTap: () -> Unit, onLongTap: () -> Unit) {
    val accent = Color(st.argb)
    val loop = rememberInfiniteTransition(label = "cap")
    val breath by loop.animateFloat(1f, 1.045f, infiniteRepeatable(tween(if (asleep) 2600 else 1600), RepeatMode.Reverse), label = "breath")
    val bob by loop.animateFloat(0f, if (asleep) 1.5f else 4f, infiniteRepeatable(tween(2200), RepeatMode.Reverse), label = "bob")
    val zz by loop.animateFloat(0f, 1f, infiniteRepeatable(tween(2400, easing = LinearEasing)), label = "zz")
    val hop = remember { Animatable(0f) }
    val scope = rememberCoroutineScope()
    val style = if (asleep) st.faceStyle.copy(eyes = WindEyes.SLEEPY) else st.faceStyle
    val outfit = capOutfitCatalog.firstOrNull { it.id == st.pet.outfit }
    Box(Modifier.size(width = 84.dp, height = 96.dp), contentAlignment = Alignment.BottomCenter) {
        Box(
            Modifier.graphicsLayer {
                translationY = -hop.value - bob
                scaleX = breath; scaleY = 2f - breath + (hop.value / 400f)
                rotationZ = hop.value / 3f
            }.clip(CircleShape)
                .combinedClickable(onClick = {
                    onTap()
                    // Un saltet quan la toques: és el mimat de la colla.
                    scope.launch {
                        hop.animateTo(26f, spring(dampingRatio = .45f, stiffness = Spring.StiffnessMedium))
                        hop.animateTo(0f, spring(dampingRatio = .35f, stiffness = Spring.StiffnessLow))
                    }
                }, onLongClick = onLongTap)
                .semantics { contentDescription = st.name }.testTag("cap-face"),
        ) {
            WindHalo(accent, style, size = 62, mood = mood, online = false, showKit = outfit == null)
            outfit?.let { o ->
                val (align, y) = when (o.spot) {
                    "eyes" -> Alignment.Center to (-4).dp
                    "neck" -> Alignment.BottomCenter to 2.dp
                    else -> Alignment.TopCenter to (-10).dp
                }
                Text(o.emoji, fontSize = if (o.spot == "top") 26.sp else 22.sp,
                    modifier = Modifier.align(align).offset(y = y).testTag("cap-outfit"))
            }
        }
        if (asleep) {
            Text("z", fontSize = (11 + 6 * zz).sp, color = Muted, fontWeight = FontWeight.Bold,
                modifier = Modifier.align(Alignment.TopEnd).offset(x = (4 * zz).dp, y = (-14 * zz).dp)
                    .graphicsLayer { alpha = 1f - zz })
        }
        bursts.toList().forEach { b ->
            key(b.id) {
                val p = remember { Animatable(0f) }
                LaunchedEffect(Unit) {
                    delay(b.delayMs)
                    p.animateTo(1f, tween(1100))
                    bursts.remove(b)
                }
                if (p.value > 0f) Text(b.emoji, fontSize = 16.sp,
                    modifier = Modifier.align(Alignment.Center)
                        .offset(x = (b.dx * (0.4f + p.value)).dp, y = (-20 - 60 * p.value).dp)
                        .graphicsLayer { alpha = 1f - p.value; scaleX = 0.7f + p.value * .6f; scaleY = scaleX })
            }
        }
    }
}

/** Alegria, energia i panxa, amb barres petites. */
@Composable
internal fun CapStats(lang: String, pet: CapPet, accent: Color) {
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically,
        modifier = Modifier.testTag("cap-stats")) {
        CapBar("💛", pet.happy, accent, tr(lang, "app.cap.statHappy"), Modifier.weight(1f))
        CapBar("⚡", pet.energy, accent, tr(lang, "app.cap.statEnergy"), Modifier.weight(1f))
        CapBar("🍪", pet.full, accent, tr(lang, "app.cap.statFull"), Modifier.weight(1f))
    }
}

@Composable
private fun CapBar(icon: String, value: Int, accent: Color, label: String, modifier: Modifier) {
    Row(modifier.semantics { contentDescription = "$label $value%" }, verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(3.dp)) {
        Text(icon, fontSize = 11.sp)
        Box(Modifier.weight(1f).height(6.dp).clip(RoundedCornerShape(3.dp)).background(SuperficieAlta)) {
            Box(Modifier.fillMaxHeight().fillMaxWidth(value.coerceIn(0, 100) / 100f).clip(RoundedCornerShape(3.dp))
                .background(if (value < 30) Color(0xFFE57373) else accent))
        }
    }
}

/** Menjar, mimar, jugar, vestir, dormir: xips petits. */
@Composable
internal fun CapActions(lang: String, pet: CapPet, asleep: Boolean, onFeed: (String) -> Unit, onPet: () -> Unit,
                       onPlay: (String) -> Unit, onWardrobe: () -> Unit, onSleep: () -> Unit) {
    var feedMenu by remember { mutableStateOf(false) }
    var playMenu by remember { mutableStateOf(false) }
    Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).testTag("cap-actions"),
        horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        Box {
            CapChip("🍪", tr(lang, "app.cap.feed"), highlight = pet.hungry) { feedMenu = true }
            DropdownMenu(feedMenu, { feedMenu = false }) {
                listOf("galeta" to "🍪", "poma" to "🍎", "cafe" to "☕").forEach { (id, e) ->
                    DropdownMenuItem(text = { Text("$e  " + tr(lang, "app.cap.snack.$id")) }, onClick = { feedMenu = false; onFeed(id) })
                }
            }
        }
        CapChip("🤲", tr(lang, "app.cap.pet"), onClick = onPet)
        Box {
            CapChip("🎮", tr(lang, "app.cap.play"), enabled = !pet.tired) { playMenu = true }
            DropdownMenu(playMenu, { playMenu = false }) {
                DropdownMenuItem(text = { Text("☁️  " + tr(lang, "app.cap.game.nuvols")) }, onClick = { playMenu = false; onPlay("nuvols") })
                DropdownMenuItem(text = { Text("✊  " + tr(lang, "app.cap.game.ppt")) }, onClick = { playMenu = false; onPlay("ppt") })
            }
        }
        CapChip("👕", tr(lang, "app.cap.wardrobe"), onClick = onWardrobe)
        if (pet.energy < 60 || asleep) CapChip("😴", tr(lang, "app.cap.sleep"), highlight = pet.tired, onClick = onSleep)
    }
}

@Composable
private fun CapChip(icon: String, label: String, highlight: Boolean = false, enabled: Boolean = true, onClick: () -> Unit) {
    Surface(onClick = onClick, enabled = enabled, shape = RoundedCornerShape(16.dp),
        color = if (highlight) Sea.copy(alpha = .25f) else Superficie.copy(alpha = .7f)) {
        Row(Modifier.padding(horizontal = 10.dp, vertical = 6.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(icon, fontSize = 13.sp)
            Text(label, color = if (enabled) Ink else Muted, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
        }
    }
}

/**
 * Qui ho farà (especialistes que ja hi són i els nous), el curs accelerat
 * per als que no en saben, «endavant» i els altres camins. Si no cal
 * decidir res, va sola al cap d'uns segons.
 */
@Composable
internal fun CapDecisionRow(vm: GregalViewModel, lang: String, p: GregalViewModel.CapPending, winds: List<Wind>, course: Boolean, going: Boolean) {
    val d = p.decision
    val who = d.winds.mapNotNull { id -> winds.firstOrNull { it.id == id } }
    var other by remember { mutableStateOf(false) }
    val needsChoice = d.create.isNotEmpty() || d.novices.isNotEmpty() || d.action == "crew"
    // Compte enrere: sense res a decidir, la mascota ja obre el xat.
    val countdown = remember(p) { Animatable(0f) }
    LaunchedEffect(p, other, needsChoice, going) {
        if (needsChoice || other || going) { countdown.snapTo(0f); return@LaunchedEffect }
        countdown.animateTo(1f, tween(4000, easing = LinearEasing))
        vm.capGo()
    }
    Column(verticalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.testTag("cap-decision")) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            who.take(3).forEach { WindFace(size = 30, accent = Color(it.argb), style = it.faceStyle) }
            d.create.forEach { n ->
                Box {
                    WindFace(size = 30, accent = Color(n.argb), style = windStyleOf(n.style))
                    Text("✨", fontSize = 10.sp, modifier = Modifier.align(Alignment.TopEnd))
                }
            }
            val names = who.map { it.name } + d.create.map { it.name }
            Text(
                when (d.action) {
                    "crew" -> tr(lang, "app.cap.toCrew")
                    "rotllana" -> tr(lang, "app.cap.toGroup", capRotllanaName(names, lang))
                    else -> tr(lang, "app.cap.toWind", names.firstOrNull().orEmpty())
                },
                color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f),
                maxLines = 2, overflow = TextOverflow.Ellipsis,
            )
        }
        if (d.create.isNotEmpty()) {
            Text(tr(lang, "app.cap.newSpecialists", d.create.joinToString(", ") { it.name + " (" + it.role + ")" }),
                color = Muted, fontSize = 12.sp, lineHeight = 16.sp)
        }
        val learners = d.novices.mapNotNull { id -> winds.firstOrNull { it.id == id }?.name } + d.create.map { it.name }
        if (learners.isNotEmpty()) {
            Surface(onClick = { vm.capToggleCourse() }, shape = RoundedCornerShape(14.dp),
                color = if (course) Sea.copy(alpha = .22f) else Superficie.copy(alpha = .7f),
                modifier = Modifier.fillMaxWidth().testTag("cap-course")) {
                Row(Modifier.padding(horizontal = 10.dp, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Checkbox(checked = course, onCheckedChange = { vm.capToggleCourse() }, modifier = Modifier.size(20.dp))
                    Column(Modifier.weight(1f)) {
                        Text(tr(lang, "app.cap.course", capRotllanaName(learners, lang)), color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                        Text(tr(lang, "app.cap.courseHelp"), color = Muted, fontSize = 11.sp, lineHeight = 14.sp)
                    }
                }
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            Button(onClick = { vm.capGo() }, enabled = !going, colors = ButtonDefaults.buttonColors(containerColor = Sea, contentColor = OnSea),
                modifier = Modifier.testTag("cap-go")) {
                if (!needsChoice && countdown.value > 0f && !going) {
                    CircularProgressIndicator(progress = { countdown.value }, modifier = Modifier.size(14.dp), color = OnSea, strokeWidth = 2.dp)
                    Spacer(Modifier.width(6.dp))
                }
                if (going) { CircularProgressIndicator(Modifier.size(14.dp), color = OnSea, strokeWidth = 2.dp); Spacer(Modifier.width(6.dp)) }
                Text(tr(lang, "app.cap.go"))
            }
            Box {
                OutlinedButton(onClick = { other = true }, enabled = !going, modifier = Modifier.testTag("cap-other")) { Text(tr(lang, "app.cap.other"), color = Ink) }
                DropdownMenu(expanded = other, onDismissRequest = { other = false }) {
                    DropdownMenuItem(text = { Text(tr(lang, "app.cap.self")) }, onClick = { other = false; vm.askCap(p.message, "self") })
                    winds.filter { w -> !(d.action == "wind" && w.id in d.winds) }.forEach { w ->
                        DropdownMenuItem(
                            leadingIcon = { WindFace(size = 24, accent = Color(w.argb), style = w.faceStyle) },
                            text = { Text(tr(lang, "app.cap.passTo", w.name)) },
                            onClick = { other = false; vm.askCap(p.message, "wind:" + w.id) },
                        )
                    }
                    if (winds.size >= 2 && d.action != "rotllana") {
                        DropdownMenuItem(text = { Text(tr(lang, "app.cap.group")) }, onClick = { other = false; vm.askCap(p.message, "rotllana") })
                    }
                    if (d.action != "crew") {
                        DropdownMenuItem(text = { Text(tr(lang, "app.cap.crew")) }, onClick = { other = false; vm.askCap(p.message, "crew") })
                    }
                }
            }
            TextButton(onClick = { vm.capDismiss() }, enabled = !going) { Text(tr(lang, "app.cap.later"), color = Muted) }
        }
    }
}

internal fun oneLineUi(s: String, max: Int): String {
    val t = s.split(Regex("\\s+")).filter { it.isNotEmpty() }.joinToString(" ")
    return if (t.length > max) t.take(max - 1) + "…" else t
}
