package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowBack
import androidx.compose.material.icons.automirrored.rounded.Send
import androidx.compose.material.icons.rounded.Add
import androidx.compose.material.icons.rounded.AutoStories
import androidx.compose.material.icons.rounded.Description
import androidx.compose.material.icons.rounded.FolderOpen
import androidx.compose.material.icons.rounded.Language
import androidx.compose.material.icons.rounded.Lock
import androidx.compose.material.icons.rounded.MoreHoriz
import androidx.compose.material.icons.rounded.Stop
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
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
    /** Quan hi ha on continuar (mòbil + servidor desat), el «…» obre menú en comptes d'anar directe al perfil. */
    onTransfer: (() -> Unit)? = null,
) {
    Column(Modifier.fillMaxWidth().background(Superficie)) {
    Row(Modifier.fillMaxWidth().heightIn(min = 62.dp).padding(horizontal = 4.dp),
        verticalAlignment = Alignment.CenterVertically) {
        IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Rounded.ArrowBack, tr(lang, "app.vents.back"), tint = Ink) }
        Row(Modifier.weight(1f).clip(RoundedCornerShape(18.dp)).clickable(onClick = onProfile).padding(vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            WindHalo(accent, style, size = 34, mood = mood, stacked = faces.drop(1), online = false, showKit = faces.size <= 1)
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(title, color = Ink, fontSize = 16.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (subtitle.isNotBlank()) Text(subtitle, color = Muted, fontSize = 11.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        if (onWiki != null) IconButton(onClick = onWiki) { Icon(Icons.Rounded.AutoStories, tr(lang, "app.wiki.summarize"), tint = Ink) }
        if (onTransfer != null) {
            var more by remember { mutableStateOf(false) }
            Box {
                IconButton(onClick = { more = true }) { Icon(Icons.Rounded.MoreHoriz, tr(lang, "app.vents.profile"), tint = Ink) }
                DropdownMenu(expanded = more, onDismissRequest = { more = false }) {
                    DropdownMenuItem(text = { Text(tr(lang, "app.vents.profile")) }, onClick = { more = false; onProfile() })
                    DropdownMenuItem(text = { Text(tr(lang, "app.vents.transferToServer")) }, onClick = { more = false; onTransfer() })
                }
            }
        } else {
            IconButton(onClick = onProfile) { Icon(Icons.Rounded.MoreHoriz, tr(lang, "app.vents.profile"), tint = Ink) }
        }
    }
    androidx.compose.material3.HorizontalDivider(color = Line.copy(alpha = .55f), thickness = 1.dp)
    }
}

/** One quiet, persistent status row. Timers measure observation, not completion. */
@Composable
internal fun VentsWorkProgress(lang: String, live: WindLive, modifier: Modifier = Modifier) {
    val now = androidx.compose.runtime.produceState(System.currentTimeMillis(), live.startedAt) {
        while (true) { value = System.currentTimeMillis(); kotlinx.coroutines.delay(1000) }
    }.value
    Column(modifier.fillMaxWidth().background(SuperficieAlta.copy(alpha = .35f)).padding(horizontal = 18.dp, vertical = 10.dp)) {
        Text((if (live.speaker.isNotBlank()) live.speaker + " · " else "") + windProgressLabel(lang, live), color = Ink, fontSize = 12.sp, fontWeight = FontWeight.Medium)
        Text(windProgressTiming(lang, live, now), color = Muted, fontSize = 11.sp)
    }
}

/**
 * Shared text composer (estil Grok). Voice stays out; @mention routing stays
 * in the owning conversation. Quan es passen les accions d'eines ([onAttachDoc],
 * [onMode], [onPerm], [onWorkspace]), apareix el botó «+» amb funcionalitats i
 * una píndola compacta de mode/projecte perquè el vent tingui sempre els permisos
 * que toquen.
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
    onMode: ((String) -> Unit)? = null,
    onPerm: (() -> Unit)? = null,
    onWorkspace: (() -> Unit)? = null,
    /** Amb un torn en marxa, el botó d'enviar es converteix en «atura». */
    onStop: (() -> Unit)? = null,
    /** Obre la previsualització de la carpeta (la web que el vent ha fet). */
    onPreview: (() -> Unit)? = null,
    onSend: () -> Unit,
) {
    val hasTools = onAttachDoc != null || onMode != null || onPerm != null || onWorkspace != null || onPreview != null
    var showPlus by remember { mutableStateOf(false) }
    val ringColor = if (accent != Color.Unspecified) accent else Sea
    Column(Modifier.fillMaxWidth().background(Superficie)) {
        if (st != null && (onMode != null || onWorkspace != null)) {
            Row(
                Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                if (onMode != null) {
                    val uiMode = uiModeOf(st.mode)
                    val isAct = uiMode == "act"
                    val modeLabel = tr(lang, when (uiMode) {
                        "plan" -> "app.mode.plan"
                        "act" -> "app.mode.act"
                        else -> "app.mode.ask"
                    })
                    Surface(
                        onClick = { onMode(if (isAct) "chat" else "act") },
                        shape = RoundedCornerShape(999.dp),
                        color = (if (isAct) ringColor else SuperficieAlta).copy(alpha = if (isAct) 0.16f else 0.55f),
                    ) {
                        Text(
                            (if (isAct) "⚡ " else "💬 ") + modeLabel,
                            color = if (isAct) Ink else Muted,
                            fontSize = 11.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            modifier = Modifier.padding(horizontal = 10.dp, vertical = 4.dp),
                        )
                    }
                }
                if (onWorkspace != null && st.project.isNotBlank()) {
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
                                color = Muted, fontSize = 11.5.sp, maxLines = 1, overflow = TextOverflow.Ellipsis,
                            )
                        }
                    }
                }
            }
        }
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 14.dp, vertical = 10.dp),
            verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            if (hasTools) {
                Box {
                    IconButton(
                        onClick = { showPlus = true },
                        modifier = Modifier.padding(bottom = 3.dp).size(42.dp).clip(CircleShape)
                            .background(SuperficieAlta.copy(alpha = .55f)),
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
                                        Text(tr(lang, "app.composer.attachDocHelp"), color = Muted, fontSize = 11.5.sp, maxLines = 2)
                                    }
                                },
                                onClick = { showPlus = false; onAttachDoc() },
                            )
                        }
                        if (onWorkspace != null) {
                            DropdownMenuItem(
                                leadingIcon = { Icon(Icons.Rounded.FolderOpen, null, tint = Sea) },
                                text = {
                                    Column {
                                        Text(tr(lang, "app.carpeta.title"), color = Ink, fontSize = 14.sp)
                                        if (st != null && st.project.isNotBlank()) {
                                            Text(st.project + if (st.branch.isNotBlank()) " · " + st.branch else "", color = Muted, fontSize = 11.5.sp)
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
                                        Text(tr(lang, "app.preview.webs"), color = Muted, fontSize = 11.5.sp, maxLines = 1)
                                    }
                                },
                                onClick = { showPlus = false; onPreview() },
                            )
                        }
                        if (onMode != null && st != null) {
                            HorizontalDivider(color = Line)
                            val curUi = uiModeOf(st.mode)
                            listOf("act" to "app.mode.act", "plan" to "app.mode.plan", "chat" to "app.mode.ask").forEach { (m, k) ->
                                val sel = curUi == m
                                DropdownMenuItem(
                                    text = { Text((if (sel) "✓ " else "") + tr(lang, k), color = if (sel) Sea else Ink, fontWeight = if (sel) FontWeight.SemiBold else FontWeight.Normal) },
                                    onClick = { showPlus = false; onMode(m) },
                                )
                            }
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
            OutlinedTextField(
                value = field, onValueChange = onChange,
                placeholder = { Text(hint, fontSize = 14.sp, maxLines = 1, overflow = TextOverflow.Ellipsis) },
                shape = RoundedCornerShape(26.dp), maxLines = 4,
                modifier = Modifier.weight(1f).workingRing(active = busy, corner = 26.dp, color = ringColor)
                    .padding(vertical = 1.dp),
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences, autoCorrect = true, keyboardType = KeyboardType.Text),
                colors = OutlinedTextFieldDefaults.colors(
                    focusedBorderColor = LineHi, unfocusedBorderColor = Color.Transparent,
                    focusedContainerColor = SuperficieAlta.copy(alpha = .45f), unfocusedContainerColor = SuperficieAlta.copy(alpha = .45f),
                    cursorColor = Ink,
                ),
            )
            // Mentre el vent treballa, el botó d'enviar es converteix en
            // «atura»: un toc i el torn para de seguida (abans només es podia
            // esperar el límit dur). Sense onStop, el botó és el de sempre.
            if (busy && onStop != null) {
                IconButton(onClick = onStop,
                    modifier = Modifier.padding(bottom = 3.dp).size(48.dp).clip(CircleShape)
                        .background(Sand.copy(alpha = 0.9f))) {
                    Icon(Icons.Rounded.Stop, tr(lang, "app.vents.stop"), tint = Superficie, modifier = Modifier.size(20.dp))
                }
            } else {
                IconButton(onClick = onSend, enabled = canSend,
                    modifier = Modifier.padding(bottom = 3.dp).size(48.dp).clip(CircleShape)
                        .background(if (canSend) Ink else SuperficieAlta)) {
                    Icon(Icons.AutoMirrored.Rounded.Send, tr(lang, "app.vents.send"), tint = if (canSend) Superficie else Muted, modifier = Modifier.size(20.dp))
                }
            }
        }
    }
}
