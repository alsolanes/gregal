package xyz.solanes.gregal

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.AutoStories
import androidx.compose.material.icons.rounded.Construction
import androidx.compose.material.icons.rounded.Schedule
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Programats, wiki i feines en una fila de xips: les converses queden a dalt. */
@Composable
internal fun VentsShortcuts(
    lang: String, running: Int, hasTasks: Boolean,
    onProgramats: () -> Unit, onWiki: () -> Unit, onFeines: () -> Unit,
) {
    Row(
        Modifier.fillMaxWidth().horizontalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 6.dp).testTag("vents-shortcuts"),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        ShortcutChip(Icons.Rounded.Schedule, tr(lang, "app.programats.open"), onClick = onProgramats)
        ShortcutChip(Icons.Rounded.AutoStories, tr(lang, "app.wiki.open"), onClick = onWiki)
        if (hasTasks) ShortcutChip(Icons.Rounded.Construction, tr(lang, "app.feines.title"), busy = running > 0,
            badge = if (running > 0) running.toString() else null, onClick = onFeines)
    }
}

@Composable
private fun ShortcutChip(icon: ImageVector, label: String, busy: Boolean = false, badge: String? = null, onClick: () -> Unit) {
    Surface(onClick = onClick, shape = RoundedCornerShape(18.dp), color = SuperficieAlta.copy(alpha = .45f)) {
        Row(Modifier.padding(horizontal = 12.dp, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            if (busy) CircularProgressIndicator(Modifier.size(14.dp), color = Sea, strokeWidth = 2.dp)
            else Icon(icon, null, tint = Sea, modifier = Modifier.size(16.dp))
            Text(label, color = Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, maxLines = 1)
            badge?.let { Text(it, color = Sea, fontSize = 12.sp, fontWeight = FontWeight.Bold) }
        }
    }
}
