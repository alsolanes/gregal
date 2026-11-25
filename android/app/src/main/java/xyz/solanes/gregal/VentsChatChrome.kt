package xyz.solanes.gregal

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.snap
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.animation.core.LinearEasing
import androidx.compose.foundation.lazy.LazyListLayoutInfo
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.runtime.snapshotFlow
import androidx.compose.runtime.withFrameMillis
import androidx.compose.ui.platform.LocalDensity
import kotlinx.coroutines.delay
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.isSpecified
import androidx.compose.ui.graphics.TransformOrigin
import kotlin.math.PI
import kotlin.math.sin
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.IconButton
import androidx.compose.ui.graphics.Brush
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.GraphicEq
import androidx.compose.material.icons.automirrored.rounded.ArrowBack
import androidx.compose.material.icons.automirrored.rounded.KeyboardArrowRight
import androidx.compose.material.icons.automirrored.rounded.Send
import androidx.compose.material.icons.rounded.Computer
import androidx.compose.material.icons.rounded.Menu
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.ArrowUpward
import androidx.compose.material.icons.rounded.Mic
import androidx.compose.material.icons.rounded.AutoStories
import androidx.compose.material.icons.rounded.Description
import androidx.compose.material.icons.rounded.Image
import androidx.compose.material.icons.rounded.FolderOpen
import androidx.compose.material.icons.rounded.Language
import androidx.compose.material.icons.rounded.Lock
import androidx.compose.material.icons.rounded.MoreHoriz
import androidx.compose.material.icons.rounded.Stop
import androidx.compose.material.icons.rounded.Forum
import androidx.compose.material.icons.rounded.Person
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.CompositingStrategy
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.compositeOver
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Same compact chrome for direct and group conversations. No invented online status. */
@Composable
internal fun VentsChatHeader(
    lang: String, title: String, subtitle: String, accent: Color, style: WindStyle,
    mood: WindMood, faces: List<Wind> = emptyList(), onBack: () -> Unit, onProfile: () -> Unit,
    onWiki: (() -> Unit)? = null,
    /** Sessions (fils) d'aquest vent: «…» → llista de converses. */
    onSessions: (() -> Unit)? = null,
    /** Quan hi ha on continuar (mòbil + servidor desat), el «…» obre menú en comptes d'anar directe al perfil. */
    onTransfer: (() -> Unit)? = null,
    /** Capçalera del fil personal: avatar de 44 dp centrat amb el nom a sota i
     *  botons circulars de 44 dp a banda i banda. */
    centered: Boolean = false,
    /** L'ordinador de l'assistent (dreta), quan n'hi ha. */
    onComputer: (() -> Unit)? = null,
    /** A la portada el botó de l'esquerra obre el menú (no torna enrere). */
    menu: Boolean = false,
    /** Portada de Gregal: només la cara, petita i flotant, sense nom ni fons (B14). */
    compact: Boolean = false,
    /** El vent del xat: el seu ofici va en pastilla al costat del nom. */
    wind: Wind? = null,
    /** Xat d'un vent: la cara no és aquí sinó un forat on hi vola la del xat (109), que és una de sola. */
    faceSlot: Modifier? = null,
    modifier: Modifier = Modifier,
) {
    if (centered) {
        CenteredChatHeader(lang, title, subtitle, accent, style, mood, faces, onBack, onProfile, onComputer,
            onWiki, onSessions, onTransfer, menu, compact)
        return
    }
    // Barra transparent: el fil passa per sota i només les pastilles (tornar,
    // cara + nom, «…») tenen fons. Així el mòbil guanya l'alçada d'una franja.
    Row(modifier.fillMaxWidth().padding(horizontal = 6.dp, vertical = 6.dp).testTag("chat-header"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        IconButton(onClick = onBack, modifier = Modifier.softBacking(SuperficieAlta)) {
            Icon(Icons.AutoMirrored.Rounded.ArrowBack, tr(lang, "app.vents.back"), tint = Ink)
        }
        Box(Modifier.weight(1f), contentAlignment = Alignment.CenterStart) {
            Row(Modifier.pillBacking(SuperficieAlta).clip(RoundedCornerShape(999.dp)).clickable(onClick = onProfile, role = Role.Button)
                .padding(start = 6.dp, end = 14.dp, top = 5.dp, bottom = 5.dp).testTag("chat-header-pill"),
                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                if (faceSlot != null) Box(faceSlot.height(44.dp).testTag("chat-header-face"))
                else WindHalo(accent, style, size = 30, mood = mood, stacked = faces.drop(1), online = false, showKit = faces.size <= 1)
                Column(verticalArrangement = Arrangement.spacedBy(0.dp)) {
                    // El nom es mesura primer: la pastilla es queda amb el que sobra.
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(title, color = Ink, fontSize = 15.sp, lineHeight = 18.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        if (wind != null) WindRolePill(wind)
                    }
                    if (subtitle.isNotBlank()) Text(subtitle, color = Muted, fontSize = 11.5.sp, lineHeight = 14.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
        }
        // Una sola acció a la dreta, amb noms, la mateixa per a tots els vents (Gregal inclòs) i en
        // l'ordre del web: Perfil, Fils, Desa a la wiki, l'ordinador. El perfil també és a la cara.
        if (onWiki != null || onSessions != null || onTransfer != null || onComputer != null) {
            var more by remember { mutableStateOf(false) }
            Box {
                IconButton(onClick = { more = true }, modifier = Modifier.softBacking(SuperficieAlta).testTag("chat-more")) {
                    Icon(Icons.Rounded.MoreHoriz, tr(lang, "app.vents.more"), tint = Ink)
                }
                DropdownMenu(expanded = more, onDismissRequest = { more = false }) {
                    DropdownMenuItem(leadingIcon = { Icon(Icons.Rounded.Person, null) },
                        text = { Text(tr(lang, "app.vents.menu.profile")) }, onClick = { more = false; onProfile() })
                    if (onSessions != null) DropdownMenuItem(
                        leadingIcon = { Icon(Icons.Rounded.Forum, null) },
                        text = { Text(tr(lang, "app.vents.menu.threads")) }, onClick = { more = false; onSessions() })
                    if (onWiki != null) DropdownMenuItem(
                        leadingIcon = { Icon(Icons.Rounded.AutoStories, null) },
                        text = { Text(tr(lang, "app.vents.menu.wiki")) }, onClick = { more = false; onWiki() })
                    if (onComputer != null) DropdownMenuItem(leadingIcon = { Icon(Icons.Rounded.Computer, null) },
                        text = { Text(tr(lang, "app.computer.name")) }, onClick = { more = false; onComputer() })
                    if (onTransfer != null) DropdownMenuItem(text = { Text(tr(lang, "app.vents.transferToServer")) }, onClick = { more = false; onTransfer() })
                }
            }
        }
    }
}

/** Alçada de la capçalera flotant del xat: el fil hi deixa aquest marge a dalt. */
internal val ChatHeaderInset = 62.dp

/** El fons de la conversa d'un vent: la superfície amb un vel suau del seu color. */
internal fun windWash(accent: Color, surface: Color): Color = accent.copy(alpha = 0.06f).compositeOver(surface)

/** Les bombolles del vent, tenyides del seu color com els punts d'escriure (16 %). */
internal fun windBubbleColor(accent: Color, background: Color): Color = accent.copy(alpha = 0.16f).compositeOver(background)

/** El nom del vent sobre les seves respostes: el seu color barrejat (40 %) amb la tinta, perquè es llegeixi en tots els temes. */
internal fun windNameColor(accent: Color, ink: Color): Color = androidx.compose.ui.graphics.lerp(ink, accent, 0.4f)

/**
 * Qui parla, al principi d'una ratxa de respostes: el nom petit amb el color del
 * vent i, en una conversa en grup ([face]), la seva cara al costat. Les respostes
 * són text corrent, sense bombolla; el color del vent és aquí i als accents.
 */
@Composable
internal fun WindByline(name: String, accent: Color, face: Wind? = null, onClick: (() -> Unit)? = null, modifier: Modifier = Modifier) {
    Row(modifier.padding(top = 2.dp)
        .then(if (onClick != null) Modifier.heightIn(min = 32.dp).clickable(onClick = onClick) else Modifier)
        .testTag("wind-byline"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        if (face != null) WindFace(size = 24, accent = Color(face.argb), style = face.faceStyle)
        Text(name, color = windNameColor(accent, Ink), style = GregalType.bodySmall, fontWeight = FontWeight.SemiBold, maxLines = 1,
            overflow = TextOverflow.Ellipsis)
    }
}

/**
 * L'estat del vent a la capçalera, la mateixa regla que el web: t'espera (té
 * una pregunta per a tu, encara que el torn segueixi), treballant, fet (el torn
 * ha acabat i encara no has escrit ni has sortit) o res. Torna la clau del text.
 */
internal fun windChatState(windId: String, busy: Set<String>, asks: List<GregalApi.WindAsk>, done: Boolean, failed: Boolean = false): String? = when {
    asks.any { it.wind == windId } -> "winds.state.waiting"
    windId in busy -> "winds.state.working"
    // Un torn que ha acabat amb «Torn interromput» no està fet.
    done && !failed -> "winds.state.done"
    else -> null
}

/** Mida (dp) del vent present a la cantonada del xat. */
internal const val WindPresenceSize = 72

/**
 * L'humor del vent de la cantonada segons [windChatState]: treballant sempre és
 * un humor de feina (el que diu el flux en directe, si ho és) perquè hi surti l'aura.
 */
internal fun windPresenceMood(state: String?, live: WindMood?): WindMood = when (state) {
    "winds.state.working" -> live?.takeIf { it in windWorkMoods } ?: WindMood.WORKING
    "winds.state.waiting" -> WindMood.WAITING
    "winds.state.done" -> WindMood.DONE
    else -> WindMood.IDLE
}

/** Quant (ms) es manté com a mínim un humor de feina: canviar d'eina cada pocs segons no reinicia la cara. */
internal const val WindMoodHoldMs = 1200L

/** L'humor que es veu: un de feina es queda [WindMoodHoldMs] abans de deixar pas al següent. El rellotge és qui el crida. */
internal class WindMoodHold(private var shown: WindMood, private var since: Long) {
    private var wanted = shown

    fun at(now: Long, want: WindMood): WindMood {
        wanted = want
        if (want != shown && (shown !in windWorkMoods || now - since >= WindMoodHoldMs)) { shown = want; since = now }
        return shown
    }

    /** Quant falta perquè surti l'humor que espera; 0 si no n'espera cap. */
    fun waitFor(now: Long): Long = if (wanted == shown) 0L else (since + WindMoodHoldMs - now).coerceAtLeast(0L)
}

@Composable
private fun rememberHeldMood(mood: WindMood): WindMood {
    val hold = remember { WindMoodHold(mood, 0L) }
    var shown by remember { mutableStateOf(mood) }
    LaunchedEffect(mood) {
        // El temps dels fotogrames: a les proves és el rellotge virtual.
        var now = withFrameMillis { it }
        shown = hold.at(now, mood)
        while (hold.waitFor(now) > 0L) {
            delay(hold.waitFor(now))
            now = withFrameMillis { it }
            shown = hold.at(now, mood)
        }
    }
    return shown
}

/**
 * El vent puja a la capçalera quan t'has allunyat clarament del final (200 dp) i
 * només torna a baix quan hi tornes a ser a prop (40 dp); entremig es queda on és.
 * Que el final s'allunyi sol (la resposta creix) no el mou: cal que t'hagis desplaçat.
 */
internal fun windFliesUp(up: Boolean, fromEndDp: Float, scrolled: Boolean): Boolean = when {
    fromEndDp <= 40f -> false
    scrolled && fromEndDp >= 200f -> true
    else -> up
}

/**
 * Quants px falten per arribar al final de la llista. Els elements de sota que no
 * es veuen no tenen mida encara: compten com el més petit dels visibles, per
 * quedar curt i que el vent no pugi abans d'hora.
 */
internal fun LazyListLayoutInfo.pxFromEnd(): Int {
    val last = visibleItemsInfo.lastOrNull() ?: return 0
    val unseen = (totalItemsCount - 1 - last.index) * visibleItemsInfo.minOf { it.size }
    return (last.offset + last.size - (viewportEndOffset - afterContentPadding) + unseen).coerceAtLeast(0)
}

/** [windFliesUp] seguint la llista del xat. */
@Composable
internal fun rememberWindUp(list: LazyListState): Boolean {
    val density = LocalDensity.current.density
    var up by remember(list) { mutableStateOf(false) }
    LaunchedEffect(list) {
        var at = -1 to -1
        snapshotFlow { Triple(list.firstVisibleItemIndex, list.firstVisibleItemScrollOffset, list.layoutInfo.pxFromEnd()) }
            .collect { (index, offset, px) ->
                val moved = (index to offset) != at
                at = index to offset
                up = windFliesUp(up, px / density, moved)
            }
    }
    return up
}

/** El que passa al xat i el vent nota amb el cos (128 al web). */
internal enum class WindEvent { SENT, APPROVED, DECLINED, REPLY, FAILED }

/** Com hi reacciona el vent: assenteix, s'hi acosta o fa que no. */
internal enum class WindReaction { NOD, LEAN, SHAKE }

internal fun windReaction(event: WindEvent): WindReaction = when (event) {
    WindEvent.SENT, WindEvent.APPROVED -> WindReaction.NOD
    WindEvent.REPLY -> WindReaction.LEAN
    WindEvent.FAILED, WindEvent.DECLINED -> WindReaction.SHAKE
}

/** Com ha acabat un torn, pel seu últim missatge; null si l'has aturat abans que respongués. */
internal fun windTurnEvent(msgs: List<GregalViewModel.PetMsg>): WindEvent? {
    val last = msgs.lastOrNull { it.kind != "memory" } ?: return null
    return when {
        last.kind == "error" -> WindEvent.FAILED
        last.from == "user" -> null
        else -> WindEvent.REPLY
    }
}

/** On pot seure el vent al seu xat (a l'arrel): la conversa, la cantonada i el forat de la capçalera. */
internal class WindPerch {
    var root by mutableStateOf(Offset.Unspecified)
    var corner by mutableStateOf(Offset.Unspecified)
    var header by mutableStateOf(Offset.Unspecified)
    /** La reacció en marxa, de 0 a 1 en 0,9 s; a 1 el vent és quiet. */
    val act = Animatable(1f)
    var reaction by mutableStateOf(WindReaction.NOD)
        private set

    suspend fun react(kind: WindReaction) {
        reaction = kind
        act.snapTo(0f)
        act.animateTo(1f, tween(900, easing = LinearEasing))
    }
}

/**
 * El vent del xat, un de sol (109): a l'últim missatge seu a la cantonada i, quan
 * puges pel fil ([up]), vola al costat del nom i s'encongeix fins a la mida de la
 * capçalera. Una sola cara moguda a la capa amb una molla que es pot girar a mig
 * vol: cap fotograma recompon la cara ni el xat. Sense moviment, salta.
 */
@Composable
internal fun BoxScope.WindPresence(perch: WindPerch, up: Boolean, mood: WindMood, accent: Color, style: WindStyle,
    label: String, hidden: Boolean, onClick: () -> Unit) {
    val motion = windMotionEnabled()
    val flight = animateFloatAsState(if (up) 1f else 0f,
        if (motion) spring(dampingRatio = .72f, stiffness = Spring.StiffnessMediumLow) else snap(), label = "wind-flight")
    WindFace(size = WindPresenceSize, mood = rememberHeldMood(mood), accent = accent, style = style, showKit = true, tappable = true,
        onClick = onClick, clay = true,
        modifier = Modifier.align(Alignment.TopStart).graphicsLayer {
            val root = perch.root
            val corner = perch.corner
            val header = perch.header
            if (hidden || !root.isSpecified || !corner.isSpecified || !header.isSpecified) { alpha = 0f; return@graphicsLayer }
            val f = flight.value
            // La cara de la capçalera fa 38 dp (WindHalo de 30) al mig del forat de 44.
            val to = header + Offset(3.dp.toPx(), 3.dp.toPx())
            // Un arc cap a fora i una mica de gir: vola, no llisca.
            val arc = sin(PI.toFloat() * f.coerceIn(0f, 1f))
            transformOrigin = TransformOrigin(0f, 0f)
            translationX = corner.x + (to.x - corner.x) * f - root.x + arc * 28.dp.toPx()
            translationY = corner.y + (to.y - corner.y) * f - root.y
            scaleX = 1f + (38f / WindPresenceSize - 1f) * f
            scaleY = scaleX
            rotationZ = -10f * arc
        }.graphicsLayer {
            // La reacció, a una capa pròpia que gira des dels peus; l'esclat de la resposta el fa la cara en passar a DONE.
            val n = perch.act.value
            if (!motion || n >= 1f) return@graphicsLayer
            transformOrigin = TransformOrigin(.5f, 1f)
            // Tres cops que s'apaguen: amunt i avall per assentir, de costat per fer que no (103).
            val swing = sin(n * 6f * PI.toFloat()) * (1f - n) * 7.dp.toPx()
            when (perch.reaction) {
                WindReaction.NOD -> translationY = swing
                // S'inclina cap al fil (a la seva dreta), una mica més gran, i torna.
                WindReaction.LEAN -> {
                    val lean = sin(PI.toFloat() * n)
                    translationX = 7.dp.toPx() * lean
                    translationY = -4.dp.toPx() * lean
                    rotationZ = 8f * lean
                    scaleX = 1f + .06f * lean
                    scaleY = scaleX
                }
                WindReaction.SHAKE -> translationX = swing
            }
        }.semantics { contentDescription = label }.testTag("wind-presence"))
}

/**
 * El fil s'esvaeix sota la capçalera flotant: sense això el text es llegia
 * entre els botons. És una màscara i no un degradat de color, perquè amb el
 * Vidre el fons no és un color pla.
 */
internal fun Modifier.fadeUnderChatHeader(): Modifier =
    graphicsLayer { compositingStrategy = CompositingStrategy.Offscreen }.drawWithContent {
        drawContent()
        val end = ChatHeaderInset.toPx()
        drawRect(
            // Buit fins a sota dels botons (54 dp) i el fil reapareix just després.
            Brush.verticalGradient(0f to Color.Transparent, 0.87f to Color.Transparent, 1f to Color.Black, startY = 0f, endY = end),
            size = Size(size.width, end), blendMode = BlendMode.DstIn,
        )
    }

/** Un botó rodó de 44 dp amb vora fina (el target tàctil arriba als 48). */
@Composable
internal fun ChatCircleButton(icon: androidx.compose.ui.graphics.vector.ImageVector, label: String,
    modifier: Modifier = Modifier, onClick: () -> Unit) {
    Box(
        modifier.minimumInteractiveComponentSize().size(GregalChat.control).clip(CircleShape)
            .border(1.dp, Line, CircleShape).clickable(onClickLabel = label, role = Role.Button, onClick = onClick)
            .semantics { contentDescription = label },
        contentAlignment = Alignment.Center,
    ) { Icon(icon, null, tint = Ink, modifier = Modifier.size(20.dp)) }
}

/**
 * La capçalera del fil personal: tornar a l'esquerra, l'avatar centrat amb el
 * nom (15 sp) a sota i, a la dreta, l'ordinador o el «…» si n'hi ha. Les dues
 * bandes fan el mateix ample perquè la cara quedi centrada de debò. El
 * subtítol només surt amb feina real («Preparant l'itinerari»).
 */
@Composable
private fun CenteredChatHeader(
    lang: String, title: String, subtitle: String, accent: Color, style: WindStyle, mood: WindMood,
    faces: List<Wind>, onBack: () -> Unit, onProfile: () -> Unit, onComputer: (() -> Unit)?,
    onWiki: (() -> Unit)?, onSessions: (() -> Unit)?, onTransfer: (() -> Unit)?,
    menu: Boolean = false,
    compact: Boolean = false,
) {
    Row(
        Modifier.fillMaxWidth().then(if (compact) Modifier else Modifier.background(Superficie))
            .padding(horizontal = 8.dp, vertical = if (compact) 2.dp else 6.dp).testTag("chat-header-centered"),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.width(52.dp), contentAlignment = Alignment.CenterStart) {
            if (menu) ChatCircleButton(Icons.Rounded.Menu, tr(lang, "app.nav.menu"), Modifier.testTag("home-menu"), onClick = onBack)
            else ChatCircleButton(Icons.AutoMirrored.Rounded.ArrowBack, tr(lang, "app.vents.back"), onClick = onBack)
        }
        Column(
            Modifier.weight(1f).clip(GregalShape.card).clickable(onClick = onProfile, role = Role.Button)
                .padding(vertical = 2.dp),
            horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            WindHalo(accent, style, size = if (compact) 38 else GregalChat.headerAvatar, mood = mood, stacked = faces.drop(1), online = false, showKit = faces.size <= 1)
            if (!compact) Text(title, color = Ink, fontSize = 15.sp, lineHeight = 20.sp, fontWeight = FontWeight.SemiBold,
                maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (subtitle.isNotBlank()) Text(subtitle, color = Muted, style = GregalType.chatMeta,
                maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.testTag("chat-header-status"))
        }
        Box(Modifier.width(52.dp), contentAlignment = Alignment.CenterEnd) {
            when {
                onComputer != null -> ChatCircleButton(Icons.Rounded.Computer, threadCopy(lang, "computer"), onClick = onComputer)
                onWiki != null || onSessions != null || onTransfer != null -> {
                    var more by remember { mutableStateOf(false) }
                    Box {
                        ChatCircleButton(Icons.Rounded.MoreHoriz, tr(lang, "app.vents.more"), Modifier.testTag("chat-more")) { more = true }
                        DropdownMenu(expanded = more, onDismissRequest = { more = false }) {
                            DropdownMenuItem(text = { Text(tr(lang, "app.vents.profile")) }, onClick = { more = false; onProfile() })
                            if (onSessions != null) DropdownMenuItem(text = { Text(tr(lang, "app.wind.sessions")) }, onClick = { more = false; onSessions() })
                            if (onWiki != null) DropdownMenuItem(text = { Text(tr(lang, "app.wiki.summarize")) }, onClick = { more = false; onWiki() })
                            if (onTransfer != null) DropdownMenuItem(text = { Text(tr(lang, "app.vents.transferToServer")) }, onClick = { more = false; onTransfer() })
                        }
                    }
                }
            }
        }
    }
}

/**
 * La franja «2 feines en marxa» just a sobre del compositor: 32 dp, discreta i
 * tocable (obre la feina). No bloqueja res: es pot continuar escrivint.
 */
@Composable
internal fun WorkStrip(lang: String, count: Int, onClick: () -> Unit) {
    val label = if (count == 1) threadCopy(lang, "workingOne") else tr(lang, "app.feines.runningLine", count)
    Row(
        Modifier.fillMaxWidth().heightIn(min = GregalChat.workStrip).padding(horizontal = 12.dp)
            .clip(GregalShape.pill).clickable(onClickLabel = tr(lang, "app.vents.openWork"), role = Role.Button, onClick = onClick)
            .padding(horizontal = 12.dp).testTag("work-strip"),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        CircularProgressIndicator(Modifier.size(12.dp), color = Sea, strokeWidth = 1.5.dp)
        Text(label, color = Ink, style = GregalType.chatMeta, fontWeight = FontWeight.Medium, modifier = Modifier.weight(1f))
        Icon(Icons.AutoMirrored.Rounded.KeyboardArrowRight, null, tint = Muted, modifier = Modifier.size(18.dp))
    }
}

/**
 * Shared text composer (estil Grok), amb dictat pel micròfon; @mention routing stays
 * in the owning conversation. Quan es passen les accions d'eines ([onAttachDoc],
 * [onPerm], [onWorkspace]), apareix el botó «+» amb funcionalitats i una
 * píndola compacta del projecte. Sense mode: els vents treballen sempre en
 * Actua i el que demanen permís ho decideix el nivell d'accés.
 */
@Composable
internal fun VentsComposer(
    lang: String, field: TextFieldValue, onChange: (TextFieldValue) -> Unit,
    hint: String, canSend: Boolean,
    /** El vent (o la colla) està treballant: el camp s'envolta de llum.
     *  Va abans de `onSend` perquè la lambda final segueixi funcionant. */
    busy: Boolean = false,
    accent: Color = Color.Unspecified,
    st: ServerState? = null,
    onAttachDoc: (() -> Unit)? = null,
    onAttachImage: (() -> Unit)? = null,
    onPerm: (() -> Unit)? = null,
    onWorkspace: (() -> Unit)? = null,
    /** Amb un torn en marxa, el botó d'enviar es converteix en «atura». */
    onStop: (() -> Unit)? = null,
    /** Obre la previsualització de la carpeta (la web que el vent ha fet). */
    onPreview: (() -> Unit)? = null,
    /** Feines en marxa: la franja de 32 dp sobre el camp, que les obre. */
    workCount: Int = 0,
    onWork: (() -> Unit)? = null,
    /** Conversa per veu (parla i et respon en veu alta), al «+». */
    onVoiceTalk: (() -> Unit)? = null,
    /** «A fons» (Tempesta, winds_tempesta.go): diversos vents alhora i una sola resposta. */
    tempesta: Boolean = false,
    onTempesta: (() -> Unit)? = null,
    onSend: () -> Unit,
) {
    val hasTools = onAttachDoc != null || onAttachImage != null || onPerm != null || onWorkspace != null || onPreview != null || onVoiceTalk != null
    var showPlus by remember { mutableStateOf(false) }
    val ringColor = if (accent != Color.Unspecified) accent else Sea
    Column(Modifier.fillMaxWidth().background(Superficie)) {
        if (workCount > 0 && onWork != null) WorkStrip(lang, workCount, onWork)
        if (st != null && onWorkspace != null && st.project.isNotBlank()) {
            Row(
                Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Surface(
                    onClick = onWorkspace,
                    shape = RoundedCornerShape(999.dp),
                    color = SuperficieAlta.copy(alpha = 0.45f),
                ) {
                    Row(
                        Modifier.padding(horizontal = 10.dp, vertical = 4.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(4.dp),
                    ) {
                        Icon(Icons.Rounded.FolderOpen, null, tint = Muted, modifier = Modifier.size(13.dp))
                        Text(
                            st.project + if (st.branch.isNotBlank()) " · " + st.branch else "",
                            color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
            }
        }
        // Una sola pastilla, com l'inici: «+», el camp i un botó rodó que és
        // el micròfon (camp buit), enviar (amb text) o aturar (vent treballant).
        val dictate = rememberDictation(lang) { said ->
            val t = appendDictated(field.text, said)
            onChange(TextFieldValue(t, androidx.compose.ui.text.TextRange(t.length)))
        }
        Row(
            Modifier.fillMaxWidth().padding(start = 12.dp, end = 12.dp, top = 8.dp, bottom = 10.dp)
                .workingRing(active = busy, corner = 28.dp, color = ringColor)
                .clip(RoundedCornerShape(28.dp)).background(SuperficieAlta)
                .border(1.dp, Line, RoundedCornerShape(28.dp))
                .padding(start = if (hasTools) 4.dp else 18.dp, end = 5.dp, top = 5.dp, bottom = 5.dp)
                .testTag("vents-composer-bar"),
            verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            if (hasTools) {
                Box {
                    IconButton(
                        onClick = { showPlus = true },
                        modifier = Modifier.size(48.dp).clip(CircleShape),
                    ) {
                        Icon(Icons.Rounded.Add, tr(lang, "app.mode.more"), tint = Ink, modifier = Modifier.size(20.dp))
                    }
                    DropdownMenu(expanded = showPlus, onDismissRequest = { showPlus = false }) {
                        if (onAttachDoc != null) {
                            DropdownMenuItem(
                                leadingIcon = { Icon(Icons.Rounded.Description, null, tint = Sea) },
                                text = {
                                    Column {
                                        Text(tr(lang, "app.composer.attachDoc"), color = Ink, fontSize = 14.sp)
                                        Text(tr(lang, "app.composer.attachDocHelp"), color = Muted, fontSize = 12.sp, maxLines = 2)
                                    }
                                },
                                onClick = { showPlus = false; onAttachDoc() },
                            )
                        }
                        if (onAttachImage != null) {
                            DropdownMenuItem(
                                leadingIcon = { Icon(Icons.Rounded.Image, null, tint = Sea) },
                                text = {
                                    Column {
                                        Text(tr(lang, "app.composer.attachImage"), color = Ink, fontSize = 14.sp)
                                        Text(tr(lang, "app.composer.attachImageHelp"), color = Muted, fontSize = 12.sp, maxLines = 2)
                                    }
                                },
                                onClick = { showPlus = false; onAttachImage() },
                            )
                        }
                        if (onWorkspace != null) {
                            DropdownMenuItem(
                                leadingIcon = { Icon(Icons.Rounded.FolderOpen, null, tint = Sea) },
                                text = {
                                    Column {
                                        Text(tr(lang, "app.carpeta.title"), color = Ink, fontSize = 14.sp)
                                        if (st != null && st.project.isNotBlank()) {
                                            Text(st.project + if (st.branch.isNotBlank()) " · " + st.branch else "", color = Muted, fontSize = 12.sp)
                                        }
                                    }
                                },
                                onClick = { showPlus = false; onWorkspace() },
                            )
                        }
                        if (onPreview != null) {
                            DropdownMenuItem(
                                leadingIcon = { Icon(Icons.Rounded.Language, null, tint = Sea) },
                                text = {
                                    Column {
                                        Text(tr(lang, "app.preview.open"), color = Ink, fontSize = 14.sp)
                                        Text(tr(lang, "app.preview.webs"), color = Muted, fontSize = 12.sp, maxLines = 1)
                                    }
                                },
                                onClick = { showPlus = false; onPreview() },
                            )
                        }
                        if (onVoiceTalk != null) {
                            DropdownMenuItem(
                                leadingIcon = { Icon(Icons.Rounded.GraphicEq, null, tint = Sea) },
                                text = { Text(voiceTalkCopy(lang, "talk"), color = Ink, fontSize = 14.sp) },
                                onClick = { showPlus = false; onVoiceTalk() },
                                modifier = Modifier.testTag("voice-talk"),
                            )
                        }
                        if (onPerm != null && st != null) {
                            HorizontalDivider(color = Line)
                            DropdownMenuItem(
                                leadingIcon = { Icon(Icons.Rounded.Lock, null, tint = Sand) },
                                text = { Text(tr(lang, "app.perm.title") + " · " + tr(lang, "app.perm." + st.access), color = Ink, fontSize = 14.sp) },
                                onClick = { showPlus = false; onPerm() },
                            )
                        }
                    }
                }
            }
            // Tan alt com el botó de la dreta (48 dp amb el target tàctil) i
            // centrat: a 40 dp i a dalt, la pista quedava més amunt que les icones.
            Box(Modifier.weight(1f).heightIn(min = 48.dp), contentAlignment = Alignment.CenterStart) {
                // La pista va dins del camp (decorationBox): així el camp i la pista són un sol node.
                BasicTextField(
                    value = field, onValueChange = onChange, maxLines = 5,
                    textStyle = GregalType.chatBody.copy(color = Ink),
                    cursorBrush = SolidColor(Ink),
                    keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences, autoCorrect = true, keyboardType = KeyboardType.Text),
                    modifier = Modifier.fillMaxWidth().testTag("vents-composer-field"),
                    decorationBox = { inner ->
                        Box(contentAlignment = Alignment.CenterStart) {
                            if (field.text.isEmpty()) Text(hint, color = Muted, style = GregalType.chatBody, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            inner()
                        }
                    },
                )
            }
            // «A fons»: només una icona dins la pastilla (no una fila sencera);
            // activada, s'il·lumina amb el color del vent.
            if (onTempesta != null) {
                val deepLabel = if (lang == "en") "Deep" else "A fons"
                // En tocar-lo, una línia que diu què fa i se'n va sola en
                // 2,5 s: no tapa res ni s'ha de tancar.
                var hintFor by remember { mutableStateOf<Boolean?>(null) }
                var hintSeq by remember { mutableStateOf(0) }
                LaunchedEffect(hintSeq) {
                    if (hintSeq > 0) { kotlinx.coroutines.delay(2500); hintFor = null }
                }
                Box {
                hintFor?.let { on -> TempestaHint(on, lang) }
                IconButton(onClick = { hintFor = !tempesta; hintSeq++; onTempesta() },
                    modifier = Modifier.size(48.dp).clip(CircleShape)
                        .background(if (tempesta) ringColor.copy(alpha = .18f) else Color.Transparent)
                        .testTag("tempesta-toggle")
                        .semantics { contentDescription = deepLabel; stateDescription = if (tempesta) "on" else "off" }) {
                    Icon(Icons.Rounded.AutoAwesome, null, tint = if (tempesta) Sea else Muted, modifier = Modifier.size(20.dp))
                }
                }
            }
            // Mentre el vent treballa: amb text, enviar (el missatge entra
            // com a steer, sense aturar el torn); sense text, «atura».
            when {
                busy && onStop != null && field.text.isBlank() -> IconButton(onClick = onStop,
                    modifier = Modifier.minimumInteractiveComponentSize().size(GregalChat.send).clip(CircleShape).background(SendBg)) {
                    Icon(Icons.Rounded.Stop, tr(lang, "app.vents.stop"), tint = SendFg, modifier = Modifier.size(20.dp))
                }
                field.text.isBlank() && dictate != null -> IconButton(onClick = dictate,
                    modifier = Modifier.minimumInteractiveComponentSize().size(GregalChat.send).clip(CircleShape).background(Ink.copy(alpha = .08f))) {
                    Icon(Icons.Rounded.Mic, voiceCopy(lang, "dictate"), tint = Ink, modifier = Modifier.size(21.dp))
                }
                else -> IconButton(onClick = onSend, enabled = canSend,
                    modifier = Modifier.minimumInteractiveComponentSize().size(GregalChat.send).clip(CircleShape).background(if (canSend) SendBg else SuperficieAlta)) {
                    // Avió de paper, com al tauler de marca (abans una fletxa amunt).
                    Icon(Icons.AutoMirrored.Rounded.Send, tr(lang, "app.vents.send"), tint = if (canSend) SendFg else Muted,
                        modifier = Modifier.size(20.dp).graphicsLayer { rotationZ = -30f; translationX = 1.5f })
                }
            }
        }
    }
}

/**
 * La capçalera de la portada de Gregal (direcció visual §2): cap barra ni vora.
 * Transparent: el fil hi passa per sota i només el menú, la cara i
 * l'ordinador porten un fons rodó semitransparent; la cara és de 34 dp
 * al centre; sota la cara, una paraula d'estat només mentre treballa.
 */
@Composable
internal fun GregalFloatingHeader(lang: String, accent: Color, style: WindStyle, mood: WindMood, working: Boolean,
    onMenu: () -> Unit, onProfile: () -> Unit, onComputer: () -> Unit, modifier: Modifier = Modifier) {
    Box(modifier.fillMaxWidth().height(if (working) 84.dp else 72.dp)

        .testTag("gregal-floating-header")) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 4.dp, vertical = 2.dp), verticalAlignment = Alignment.Top) {
            IconButton(onClick = onMenu, modifier = Modifier.softBacking(SuperficieAlta).testTag("home-menu")) {
                Icon(Icons.Rounded.Menu, tr(lang, "app.nav.menu"), tint = Ink)
            }
            Column(Modifier.weight(1f).padding(top = 6.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                // Sense clip rodó: el halo (feina, anells) sobrepassa els 34 dp i es tallava.
                Box(Modifier.softBacking(SuperficieAlta).clickable(onClick = onProfile, role = Role.Button).padding(4.dp)) {
                    WindHalo(accent, style, size = 34, mood = mood, online = false, showKit = false)
                }
                if (working) Text(threadCopy(lang, "working"), color = Muted, fontSize = 11.sp, lineHeight = 14.sp,
                    modifier = Modifier.padding(top = 2.dp).testTag("gregal-working"))
            }
            IconButton(onClick = onComputer, modifier = Modifier.softBacking(SuperficieAlta).testTag("home-computer")) {
                Icon(Icons.Rounded.Computer, threadCopy(lang, "computer"), tint = Ink)
            }
        }
    }
}

/** Com un botó flotant: un cercle del color de fons, semitransparent, darrere de
 *  l'avatar i de cada botó. La barra no és una franja; només ells es llegeixen
 *  sobre el fil que passa per sota. */
/** «A fons»: avís curt a sobre del botó, alineat a la seva dreta; no agafa el focus ni els tocs. */
@Composable
internal fun TempestaHint(on: Boolean, lang: String) {
    val text = when {
        lang == "en" && on -> "Deep: several winds work on it and give you one answer, slower"
        lang == "en" -> "Deep off"
        on -> "A fons: diversos vents hi treballen i et donen una sola resposta, més lenta"
        else -> "A fons desactivat"
    }
    val gap = with(androidx.compose.ui.platform.LocalDensity.current) { 8.dp.roundToPx() }
    val above = remember(gap) {
        object : androidx.compose.ui.window.PopupPositionProvider {
            override fun calculatePosition(
                anchorBounds: androidx.compose.ui.unit.IntRect,
                windowSize: androidx.compose.ui.unit.IntSize,
                layoutDirection: androidx.compose.ui.unit.LayoutDirection,
                popupContentSize: androidx.compose.ui.unit.IntSize,
            ) = androidx.compose.ui.unit.IntOffset(
                (anchorBounds.right - popupContentSize.width).coerceIn(0, (windowSize.width - popupContentSize.width).coerceAtLeast(0)),
                (anchorBounds.top - popupContentSize.height - gap).coerceAtLeast(0),
            )
        }
    }
    androidx.compose.ui.window.Popup(popupPositionProvider = above, properties = androidx.compose.ui.window.PopupProperties(focusable = false)) {
        Text(text, color = Ink, fontSize = 12.5.sp, lineHeight = 17.sp,
            modifier = Modifier.widthIn(max = 280.dp).clip(RoundedCornerShape(14.dp)).background(SuperficieAlta)
                .padding(horizontal = 12.dp, vertical = 7.dp).testTag("tempesta-hint")
                .semantics { liveRegion = LiveRegionMode.Polite })
    }
}

private fun Modifier.softBacking(bg: Color): Modifier = drawBehind { drawCircle(bg.copy(alpha = .9f), radius = size.minDimension / 2f) }

/** El mateix fons semitransparent en forma de pastilla (cara + nom). */
private fun Modifier.pillBacking(bg: Color): Modifier = drawBehind {
    drawRoundRect(bg.copy(alpha = .9f), cornerRadius = androidx.compose.ui.geometry.CornerRadius(size.height / 2f))
}
