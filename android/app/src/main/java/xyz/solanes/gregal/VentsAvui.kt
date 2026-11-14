package xyz.solanes.gregal

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowForward
import androidx.compose.material.icons.rounded.Bedtime
import androidx.compose.material.icons.rounded.Check
import androidx.compose.material.icons.rounded.Schedule
import androidx.compose.material.icons.rounded.ThumbDown
import androidx.compose.material.icons.rounded.ThumbDownOffAlt
import androidx.compose.material.icons.rounded.ThumbUp
import androidx.compose.material.icons.rounded.ThumbUpOffAlt
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/*
 * «Avui»: la safata dels vents, a l'estil de Duet i del Daily Brief de Gemini.
 * Tot el que un vent diu pel seu compte és una targeta amb una decisió (Fet,
 * Més tard, Ara no o una acció que obre la conversa). Tres seccions —Ara toca,
 * Per si de cas, Més endavant— i un final: «Això és tot per avui». Viu a la
 * seva pestanya dels Vents (Inici | Avui | Xats): la portada és la salutació i
 * el compositor, i una safata que s'acaba necessita la pantalla sencera.
 */

/** Lògica pura de la safata (es prova sense Compose). */
object AvuiLogic {
    private fun map(v: AvuiView, id: String, f: (AvuiCard) -> AvuiCard): AvuiView {
        fun List<AvuiCard>.m() = map { if (it.id == id) f(it) else it }
        return v.copy(now = v.now.m(), fyi = v.fyi.m(), ahead = v.ahead.m())
    }

    /** El canvi que es veu al moment, abans que respongui el servidor (el mateix que hi fa ell). */
    fun apply(v: AvuiView, id: String, action: String): AvuiView = when (action) {
        "done" -> {
            val c = v.all.firstOrNull { it.id == id }
            val next = map(v, id) { it.copy(state = "done") }
            if (c != null && v.done.none { it.id == id }) next.copy(done = listOf(c.copy(state = "done")) + next.done) else next
        }
        "dismiss" -> map(v, id) { it.copy(state = "dismissed") }
        // Més tard: el servidor l'amaga unes hores; aquí, fora de la vista.
        "later" -> map(v, id) { it.copy(state = "later") }
        "useful", "notuseful" -> map(v, id) {
            val fb = if (it.feedback == action) "" else action
            it.copy(feedback = fb, state = if (action == "notuseful" && fb.isNotEmpty()) "dismissed" else it.state)
        }
        "save" -> map(v, id) { it.copy(saved = !it.saved) }
        "reopen" -> map(v, id) { it.copy(state = "open") }.let { n -> n.copy(done = n.done.filterNot { it.id == id }) }
        else -> v
    }

    /** El servidor no ho ha acceptat: la targeta torna com era. */
    fun revert(current: AvuiView, before: AvuiView, id: String): AvuiView {
        val old = before.all.firstOrNull { it.id == id } ?: return current
        return map(current, id) { old }.copy(done = before.done)
    }

    private val diesCa = listOf("Dilluns", "Dimarts", "Dimecres", "Dijous", "Divendres", "Dissabte", "Diumenge")
    private val mesosCa = listOf("gener", "febrer", "març", "abril", "maig", "juny", "juliol", "agost", "setembre", "octubre", "novembre", "desembre")
    private val daysEn = listOf("Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday")
    private val monthsEn = listOf("January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December")

    /** «Dimarts 10 de novembre», «Dijous 2 d'octubre»; «Tuesday, November 10». Data buida o dolenta: avui. */
    fun dateLabel(lang: String, iso: String, today: java.time.LocalDate = java.time.LocalDate.now()): String {
        val d = runCatching { java.time.LocalDate.parse(iso) }.getOrNull() ?: today
        val dow = d.dayOfWeek.value - 1
        val m = d.monthValue - 1
        if (lang == "en") return "${daysEn[dow]}, ${monthsEn[m]} ${d.dayOfMonth}"
        val mes = mesosCa[m]
        val de = if (mes.first() in "aeiouàèéíòóú") "d’" else "de "
        return "${diesCa[dow]} ${d.dayOfMonth} $de$mes"
    }

    /** L'etiqueta d'edat d'una targeta oberta d'altres dies (buida si és d'avui). */
    fun ageLabel(lang: String, days: Int): String = when {
        days <= 0 -> ""
        days == 1 -> tr(lang, "app.avui.sinceYesterday")
        else -> tr(lang, "app.avui.sinceDays", days)
    }

    fun doneLabel(lang: String, n: Int): String =
        if (n == 1) tr(lang, "app.avui.doneOne") else tr(lang, "app.avui.doneMany", n)
}

/**
 * La safata sencera. [winds] dona la cara i el color de cada vent; les
 * decisions van a [onAct] (id, acció) i les accions de la targeta a [onRun].
 */
@Composable
internal fun AvuiTray(
    lang: String,
    view: AvuiView?,
    winds: List<Wind>,
    error: Boolean,
    onAct: (String, String) -> Unit,
    onRun: (AvuiCard, AvuiAction) -> Unit,
    onSettings: (AvuiSettings, String?) -> Unit,
    onRetry: () -> Unit,
    modifier: Modifier = Modifier,
    /** El temps del vent del temps (si en tens un amb lloc principal), sota la data. */
    weather: (@Composable () -> Unit)? = null,
    /** «A prop teu» (VentsLocal.kt), sota els ajustos. */
    local: (@Composable () -> Unit)? = null,
) {
    val p = LocalPal.current
    Column(
        modifier.fillMaxSize().verticalScroll(rememberScrollState())
            .padding(horizontal = 16.dp).padding(top = 8.dp, bottom = 32.dp).testTag("avui-tray"),
    ) {
        Text(
            AvuiLogic.dateLabel(lang, view?.date.orEmpty()), color = Ink,
            style = GregalType.title.copy(fontSize = 26.sp, lineHeight = 32.sp, letterSpacing = (-0.5).sp),
            modifier = Modifier.padding(start = 4.dp, top = 8.dp).testTag("avui-date"),
        )
        if (view == null) {
            Text(tr(lang, if (error) "app.avui.loadError" else "app.avui.loading"), color = Muted,
                style = GregalType.bodySmall, modifier = Modifier.padding(start = 4.dp, top = 6.dp))
            if (error) TextButton(onClick = onRetry) { Text(tr(lang, "app.avui.retry"), color = p.sea) }
            return@Column
        }
        if (view.held > 0) Row(
            Modifier.padding(start = 4.dp, top = 4.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            Icon(Icons.Rounded.Bedtime, null, tint = Muted, modifier = Modifier.size(14.dp))
            Text(tr(lang, "app.avui.held", view.held), color = Muted, style = GregalType.caption)
        }
        if (weather != null) Box(Modifier.padding(top = 14.dp)) { weather() }
        val open = view.openCount
        listOf("now" to view.now, "fyi" to view.fyi, "ahead" to view.ahead).forEach { (key, cards) ->
            AvuiSection(lang, key, cards, winds, onAct, onRun)
        }
        Spacer(Modifier.height(24.dp))
        if (open == 0 && view.done.isEmpty()) {
            Text(tr(lang, "app.avui.empty"), color = Muted, style = GregalType.body, textAlign = TextAlign.Center,
                modifier = Modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 20.dp).testTag("avui-empty"))
        } else AvuiEnd(lang, view.done.size)
        Spacer(Modifier.height(20.dp))
        AvuiSettingsCard(lang, view.settings, onSettings)
        if (local != null) { Spacer(Modifier.height(12.dp)); local() }
    }
}

@Composable
private fun AvuiSection(
    lang: String, section: String, cards: List<AvuiCard>, winds: List<Wind>,
    onAct: (String, String) -> Unit, onRun: (AvuiCard, AvuiAction) -> Unit,
) {
    val motion = LocalWindMotion.current
    val any = cards.any { it.state == "open" }
    AnimatedVisibility(any, enter = if (motion) fadeIn(tween(180)) else fadeIn(tween(0)),
        exit = if (motion) fadeOut(tween(220)) + shrinkVertically(tween(260)) else fadeOut(tween(0))) {
        Row(Modifier.padding(start = 4.dp, top = 22.dp, bottom = 8.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            if (section == "now") Box(Modifier.size(6.dp).clip(CircleShape).background(Sea))
            Text(tr(lang, "app.avui.$section"), color = if (section == "now") Ink else Muted, style = GregalType.overline,
                modifier = Modifier.testTag("avui-section-$section"))
        }
    }
    cards.forEach { c ->
        key(c.id) {
            AnimatedVisibility(
                c.state == "open",
                enter = if (motion) fadeIn(tween(200)) + expandVertically(tween(240)) else fadeIn(tween(0)),
                exit = if (motion) fadeOut(tween(180)) + shrinkVertically(tween(280)) else fadeOut(tween(0)),
            ) {
                Box(Modifier.padding(bottom = 10.dp)) {
                    AvuiCardView(lang, c, winds.firstOrNull { it.id == c.wind }, prominent = section == "now", onAct, onRun)
                }
            }
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun AvuiCardView(
    lang: String, c: AvuiCard, wind: Wind?, prominent: Boolean,
    onAct: (String, String) -> Unit, onRun: (AvuiCard, AvuiAction) -> Unit,
) {
    val accent = wind?.let { Color(it.argb) } ?: Sea
    var expanded by remember(c.id) { mutableStateOf(false) }
    val shape = GregalShape.lg
    val bg = if (prominent) Modifier.background(Brush.linearGradient(listOf(accent.copy(alpha = .13f), SuperficieAlta.copy(alpha = .55f))))
    else Modifier.background(SuperficieAlta.copy(alpha = .5f))
    Column(
        Modifier.fillMaxWidth().clip(shape).then(bg)
            .border(1.dp, if (prominent) accent.copy(alpha = .30f) else Line, shape)
            .clickable(enabled = c.body.isNotBlank()) { expanded = !expanded }
            .padding(start = 14.dp, end = 14.dp, top = 14.dp, bottom = 10.dp)
            .testTag("avui-card-${c.id}"),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Row(verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Box(Modifier.size(36.dp).clip(RoundedCornerShape12).background(Superficie), contentAlignment = Alignment.Center) {
                WindFace(size = 30, accent = accent, style = wind?.faceStyle ?: WindStyle(), showKit = false)
            }
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text(c.windName.ifBlank { wind?.name.orEmpty() }, color = Muted, style = GregalType.caption,
                        maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
                    if (c.held) Icon(Icons.Rounded.Bedtime, tr(lang, "app.avui.heldMark"), tint = Muted.copy(alpha = .8f), modifier = Modifier.size(12.dp))
                    val age = AvuiLogic.ageLabel(lang, c.days)
                    if (age.isNotEmpty()) Text(age, color = if (LocalPal.current.dark) accent else Ink.copy(alpha = .78f), fontSize = 11.sp, fontWeight = FontWeight.Medium, maxLines = 1,
                        modifier = Modifier.clip(GregalShape.pill).background(accent.copy(alpha = .14f))
                            .padding(horizontal = 8.dp, vertical = 2.dp).testTag("avui-age"))
                }
                Text(c.title, color = Ink, style = GregalType.heading.copy(fontSize = 15.5.sp, lineHeight = 21.sp))
                if (c.why.isNotBlank()) Text(c.why, color = Muted, style = GregalType.bodySmall)
            }
        }
        if (expanded && c.body.isNotBlank()) {
            Text(renderMd(c.body, SuperficieAlta), color = Tinta, style = GregalType.bodySmall.copy(fontSize = 14.sp, lineHeight = 20.sp),
                modifier = Modifier.padding(start = 48.dp))
        }
        if (c.actions.isNotEmpty()) FlowRow(
            Modifier.padding(start = 48.dp), horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            c.actions.forEach { a ->
                Row(
                    Modifier.heightIn(min = 32.dp).clip(GregalShape.pill).border(1.dp, accent.copy(alpha = .45f), GregalShape.pill)
                        .clickable { onRun(c, a) }.padding(horizontal = 12.dp),
                    verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp),
                ) {
                    Text(a.label, color = Ink, fontSize = 12.5.sp, fontWeight = FontWeight.Medium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Icon(Icons.AutoMirrored.Rounded.ArrowForward, null, tint = accent, modifier = Modifier.size(13.dp))
                }
            }
        }
        Row(Modifier.fillMaxWidth().padding(start = 42.dp), verticalAlignment = Alignment.CenterVertically) {
            AvuiButton(tr(lang, "app.avui.done"), Icons.Rounded.Check, filled = true, tag = "avui-done") { onAct(c.id, "done") }
            AvuiButton(tr(lang, "app.avui.later"), Icons.Rounded.Schedule, tag = "avui-later") { onAct(c.id, "later") }
            AvuiButton(tr(lang, "app.avui.dismiss"), null, tag = "avui-dismiss") { onAct(c.id, "dismiss") }
            Spacer(Modifier.weight(1f))
            AvuiFeedback(if (c.feedback == "useful") Icons.Rounded.ThumbUp else Icons.Rounded.ThumbUpOffAlt,
                tr(lang, "app.avui.useful"), c.feedback == "useful", accent) { onAct(c.id, "useful") }
            AvuiFeedback(if (c.feedback == "notuseful") Icons.Rounded.ThumbDown else Icons.Rounded.ThumbDownOffAlt,
                tr(lang, "app.avui.notUseful"), c.feedback == "notuseful", accent) { onAct(c.id, "notuseful") }
        }
    }
}

private val RoundedCornerShape12 = androidx.compose.foundation.shape.RoundedCornerShape(12.dp)

/** Botó compacte: Fet és ple (tinta), Més tard i Ara no només text. */
@Composable
private fun AvuiButton(label: String, icon: ImageVector?, filled: Boolean = false, tag: String, onClick: () -> Unit) {
    Row(
        Modifier.heightIn(min = 34.dp).clip(GregalShape.pill)
            .background(if (filled) Ink else Color.Transparent)
            .clickable(onClick = onClick).padding(horizontal = if (filled) 12.dp else 9.dp).testTag(tag),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        if (icon != null) Icon(icon, null, tint = if (filled) Superficie else Muted, modifier = Modifier.size(15.dp))
        Text(label, color = if (filled) Superficie else Muted, fontSize = 12.5.sp, fontWeight = if (filled) FontWeight.SemiBold else FontWeight.Medium, maxLines = 1)
    }
}

@Composable
private fun AvuiFeedback(icon: ImageVector, label: String, on: Boolean, accent: Color, onClick: () -> Unit) {
    Box(
        Modifier.size(34.dp).clip(CircleShape).clickable(onClick = onClick).semantics { contentDescription = label },
        contentAlignment = Alignment.Center,
    ) { Icon(icon, null, tint = if (on) accent else Muted.copy(alpha = .75f), modifier = Modifier.size(16.dp)) }
}

/** El final: la safata s'acaba. */
@Composable
private fun AvuiEnd(lang: String, done: Int) {
    Column(Modifier.fillMaxWidth().testTag("avui-end"), horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Box(Modifier.width(36.dp).height(2.dp).clip(GregalShape.pill).background(LineHi))
        Spacer(Modifier.height(10.dp))
        Text(tr(lang, "app.avui.end"), color = Ink, style = GregalType.heading)
        if (done > 0) Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            Icon(Icons.Rounded.Check, null, tint = Green, modifier = Modifier.size(14.dp))
            Text(AvuiLogic.doneLabel(lang, done), color = Muted, style = GregalType.caption)
        }
    }
}

/** Ajusta el de demà, hores tranquil·les i mode tranquil. */
@Composable
private fun AvuiSettingsCard(lang: String, s: AvuiSettings, onSettings: (AvuiSettings, String?) -> Unit) {
    val p = LocalPal.current
    var tune by remember(s.tune) { mutableStateOf(s.tune) }
    var savedTune by remember { mutableStateOf(false) }
    var editHours by remember { mutableStateOf(false) }
    Column(
        Modifier.fillMaxWidth().clip(GregalShape.lg).background(SuperficieAlta.copy(alpha = .35f))
            .border(1.dp, Line, GregalShape.lg).padding(14.dp).testTag("avui-settings"),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Text(tr(lang, "app.avui.tune"), color = Ink, style = GregalType.bodySmall.copy(fontWeight = FontWeight.SemiBold))
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            OutlinedTextField(
                tune, { tune = it; savedTune = false }, maxLines = 3,
                placeholder = { Text(tr(lang, "app.avui.tuneHint"), fontSize = 13.sp, color = Muted) },
                textStyle = GregalType.bodySmall.copy(color = Ink, fontSize = 14.sp),
                shape = GregalShape.sm,
                colors = OutlinedTextFieldDefaults.colors(
                    focusedContainerColor = Superficie.copy(alpha = .6f), unfocusedContainerColor = Superficie.copy(alpha = .6f),
                    focusedBorderColor = p.sea, unfocusedBorderColor = Line, cursorColor = p.sea,
                ),
                modifier = Modifier.weight(1f).testTag("avui-tune"),
            )
            val dirty = tune.trim() != s.tune.trim()
            if (dirty) TextButton(onClick = { onSettings(s, tune.trim()); savedTune = true }) {
                Text(tr(lang, "app.avui.tuneSave"), color = p.sea, fontWeight = FontWeight.SemiBold)
            }
        }
        if (savedTune && tune.trim() == s.tune.trim() && tune.isNotBlank())
            Text(tr(lang, "app.avui.tuneSaved"), color = Muted, style = GregalType.caption)
        Box(Modifier.fillMaxWidth().height(1.dp).background(Line))
        Row(
            Modifier.fillMaxWidth().clip(GregalShape.sm).clickable { editHours = true }.padding(vertical = 4.dp).testTag("avui-quiet"),
            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Icon(Icons.Rounded.Bedtime, null, tint = Muted, modifier = Modifier.size(18.dp))
            Column(Modifier.weight(1f)) {
                Text(tr(lang, "app.avui.quiet"), color = Ink, style = GregalType.bodySmall.copy(fontWeight = FontWeight.Medium))
                Text(tr(lang, "app.avui.quietHelp", s.quietFrom, s.quietTo), color = Muted, style = GregalType.caption)
            }
            Text("${s.quietFrom} – ${s.quietTo}", color = Ink, fontSize = 13.sp, fontWeight = FontWeight.Medium,
                modifier = Modifier.clip(GregalShape.pill).background(Superficie.copy(alpha = .7f)).padding(horizontal = 10.dp, vertical = 4.dp))
        }
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Box(Modifier.size(18.dp), contentAlignment = Alignment.Center) { Box(Modifier.size(8.dp).clip(CircleShape).background(if (s.calm) p.sea else Muted.copy(alpha = .5f))) }
            Column(Modifier.weight(1f)) {
                Text(tr(lang, "app.avui.calm"), color = Ink, style = GregalType.bodySmall.copy(fontWeight = FontWeight.Medium))
                Text(tr(lang, "app.avui.calmHelp"), color = Muted, style = GregalType.caption)
            }
            Switch(
                s.calm, { onSettings(s.copy(level = if (it) "calm" else "normal"), null) },
                colors = SwitchDefaults.colors(checkedTrackColor = p.sea, checkedThumbColor = p.onSea),
                modifier = Modifier.testTag("avui-calm"),
            )
        }
    }
    if (editHours) QuietHoursDialog(lang, s, onDismiss = { editHours = false }) { from, to ->
        editHours = false
        onSettings(s.copy(quietFrom = from, quietTo = to), null)
    }
}

@Composable
private fun QuietHoursDialog(lang: String, s: AvuiSettings, onDismiss: () -> Unit, onSave: (String, String) -> Unit) {
    var from by remember { mutableStateOf(s.quietFrom) }
    var to by remember { mutableStateOf(s.quietTo) }
    val ok = AvuiJson.validTime(from) && AvuiJson.validTime(to)
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(tr(lang, "app.avui.quiet")) },
        text = {
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                OutlinedTextField(from, { from = it.take(5) }, singleLine = true, label = { Text(tr(lang, "app.avui.from")) },
                    isError = !AvuiJson.validTime(from), modifier = Modifier.weight(1f))
                OutlinedTextField(to, { to = it.take(5) }, singleLine = true, label = { Text(tr(lang, "app.avui.to")) },
                    isError = !AvuiJson.validTime(to), modifier = Modifier.weight(1f))
            }
        },
        confirmButton = { TextButton(enabled = ok, onClick = { onSave(from.trim(), to.trim()) }) { Text(tr(lang, "app.avui.change")) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text(tr(lang, "app.avui.cancel")) } },
    )
}
