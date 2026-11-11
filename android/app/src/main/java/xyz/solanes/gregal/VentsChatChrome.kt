package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowBack
import androidx.compose.material.icons.automirrored.rounded.Send
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
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.testTag
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
                if (subtitle.isNotBlank()) Text(subtitle, color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        // Una sola acció a la dreta, amb noms: dues icones soles (llibre,
        // bombolles) no deien què feien. El perfil també és a la cara.
        if (onWiki != null || onSessions != null || onTransfer != null) {
            var more by remember { mutableStateOf(false) }
            Box {
                IconButton(onClick = { more = true }, modifier = Modifier.testTag("chat-more")) {
                    Icon(Icons.Rounded.MoreHoriz, tr(lang, "app.vents.more"), tint = Ink)
                }
                DropdownMenu(expanded = more, onDismissRequest = { more = false }) {
                    DropdownMenuItem(text = { Text(tr(lang, "app.vents.profile")) }, onClick = { more = false; onProfile() })
                    if (onSessions != null) DropdownMenuItem(
                        leadingIcon = { Icon(Icons.Rounded.Forum, null) },
                        text = { Text(tr(lang, "app.wind.sessions")) }, onClick = { more = false; onSessions() })
                    if (onWiki != null) DropdownMenuItem(
                        leadingIcon = { Icon(Icons.Rounded.AutoStories, null) },
                        text = { Text(tr(lang, "app.wiki.summarize")) }, onClick = { more = false; onWiki() })
                    if (onTransfer != null) DropdownMenuItem(text = { Text(tr(lang, "app.vents.transferToServer")) }, onClick = { more = false; onTransfer() })
                }
            }
        }
    }
    // Sense línia de separació: la capçalera i el fil respiren junts, com l'inici.
    }
}


/**
 * Shared text composer (estil Grok), amb dictat pel micròfon; @mention routing stays
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
    onAttachImage: (() -> Unit)? = null,
    onMode: ((String) -> Unit)? = null,
    onPerm: (() -> Unit)? = null,
    onWorkspace: (() -> Unit)? = null,
    /** Amb un torn en marxa, el botó d'enviar es converteix en «atura». */
    onStop: (() -> Unit)? = null,
    /** Obre la previsualització de la carpeta (la web que el vent ha fet). */
    onPreview: (() -> Unit)? = null,
    /** La píndola de mode (Actua/Planifica/Pregunta) sobre el camp. Amb Vents
     *  net no surt: el mode continua al «+», sense un xip que no s'entén. */
    showModePill: Boolean = true,
    onSend: () -> Unit,
) {
    val hasTools = onAttachDoc != null || onAttachImage != null || onMode != null || onPerm != null || onWorkspace != null || onPreview != null
    var showPlus by remember { mutableStateOf(false) }
    val ringColor = if (accent != Color.Unspecified) accent else Sea
    Column(Modifier.fillMaxWidth().background(Superficie)) {
        if (st != null && ((onMode != null && showModePill) || (onWorkspace != null && st.project.isNotBlank()))) {
            Row(
                Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                if (onMode != null && showModePill) {
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
                            modeLabel,
                            color = if (isAct) Ink else Muted,
                            fontSize = 12.sp,
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
                                color = Muted, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis,
                            )
                        }
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
                .padding(start = if (hasTools) 4.dp else 18.dp, end = 5.dp, top = 5.dp, bottom = 5.dp),
            verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            if (hasTools) {
                Box {
                    IconButton(
                        onClick = { showPlus = true },
                        modifier = Modifier.size(40.dp).clip(CircleShape),
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
            Box(Modifier.weight(1f).heightIn(min = 40.dp).padding(vertical = 10.dp), contentAlignment = Alignment.CenterStart) {
                // La pista va dins del camp (decorationBox): així el camp i la pista són un sol node.
                BasicTextField(
                    value = field, onValueChange = onChange, maxLines = 5,
                    textStyle = TextStyle(color = Ink, fontSize = 15.sp, lineHeight = 21.sp),
                    cursorBrush = SolidColor(Ink),
                    keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences, autoCorrect = true, keyboardType = KeyboardType.Text),
                    modifier = Modifier.fillMaxWidth().testTag("vents-composer-field"),
                    decorationBox = { inner ->
                        Box(contentAlignment = Alignment.CenterStart) {
                            if (field.text.isEmpty()) Text(hint, color = Muted, fontSize = 15.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            inner()
                        }
                    },
                )
            }
            // Mentre el vent treballa: amb text, enviar (el missatge entra
            // com a steer, sense aturar el torn); sense text, «atura».
            when {
                busy && onStop != null && field.text.isBlank() -> IconButton(onClick = onStop,
                    modifier = Modifier.minimumInteractiveComponentSize().size(42.dp).clip(CircleShape).background(Sand.copy(alpha = 0.9f))) {
                    Icon(Icons.Rounded.Stop, tr(lang, "app.vents.stop"), tint = Superficie, modifier = Modifier.size(20.dp))
                }
                field.text.isBlank() && dictate != null -> IconButton(onClick = dictate,
                    modifier = Modifier.minimumInteractiveComponentSize().size(42.dp).clip(CircleShape).background(Ink.copy(alpha = .08f))) {
                    Icon(Icons.Rounded.Mic, voiceCopy(lang, "dictate"), tint = Ink, modifier = Modifier.size(21.dp))
                }
                else -> IconButton(onClick = onSend, enabled = canSend,
                    modifier = Modifier.minimumInteractiveComponentSize().size(42.dp).clip(CircleShape).background(if (canSend) Ink else Ink.copy(alpha = .12f))) {
                    Icon(Icons.Rounded.ArrowUpward, tr(lang, "app.vents.send"), tint = if (canSend) Superficie else Muted, modifier = Modifier.size(20.dp))
                }
            }
        }
    }
}
