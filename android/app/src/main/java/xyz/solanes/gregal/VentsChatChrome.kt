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
import androidx.compose.material.icons.rounded.AutoStories
import androidx.compose.material.icons.rounded.MoreHoriz
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
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
) {
    Row(Modifier.fillMaxWidth().heightIn(min = 64.dp).background(Superficie).padding(horizontal = 4.dp),
        verticalAlignment = Alignment.CenterVertically) {
        IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Rounded.ArrowBack, tr(lang, "app.vents.back"), tint = Ink) }
        Row(Modifier.weight(1f).clip(RoundedCornerShape(18.dp)).clickable(onClick = onProfile).padding(vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            WindHalo(accent, style, size = 34, mood = mood, stacked = faces.drop(1), online = false)
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(title, color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (subtitle.isNotBlank()) Text(subtitle, color = Muted, fontSize = 11.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        if (onWiki != null) IconButton(onClick = onWiki) { Icon(Icons.Rounded.AutoStories, tr(lang, "app.wiki.summarize"), tint = Ink) }
        IconButton(onClick = onProfile) { Icon(Icons.Rounded.MoreHoriz, tr(lang, "app.vents.profile"), tint = Ink) }
    }
}

/** Shared text composer. Voice stays out; @mention routing stays in the owning conversation. */
@Composable
internal fun VentsComposer(
    lang: String, field: TextFieldValue, onChange: (TextFieldValue) -> Unit,
    hint: String, canSend: Boolean, onSend: () -> Unit,
) {
    Row(Modifier.fillMaxWidth().background(Superficie).padding(horizontal = 16.dp, vertical = 10.dp),
        verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        OutlinedTextField(
            value = field, onValueChange = onChange,
            placeholder = { Text(hint, fontSize = 14.sp, maxLines = 1, overflow = TextOverflow.Ellipsis) },
            shape = RoundedCornerShape(26.dp), modifier = Modifier.weight(1f), maxLines = 4,
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences, autoCorrect = true, keyboardType = KeyboardType.Text),
            colors = OutlinedTextFieldDefaults.colors(
                focusedBorderColor = LineHi, unfocusedBorderColor = Color.Transparent,
                focusedContainerColor = SuperficieAlta.copy(alpha = .45f), unfocusedContainerColor = SuperficieAlta.copy(alpha = .45f),
                cursorColor = Ink,
            ),
        )
        IconButton(onClick = onSend, enabled = canSend,
            modifier = Modifier.padding(bottom = 4.dp).size(48.dp).clip(CircleShape)
                .background(if (canSend) Ink else SuperficieAlta)) {
            Icon(Icons.AutoMirrored.Rounded.Send, tr(lang, "app.vents.send"), tint = if (canSend) Superficie else Muted, modifier = Modifier.size(20.dp))
        }
    }
}
