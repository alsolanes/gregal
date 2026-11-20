package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Code
import androidx.compose.material.icons.rounded.Groups
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Compact, labelled area selector used at the top of the navigation drawer. */
@Composable
internal fun ModeSwitcher(lang: String, current: AppArea, onSelect: (AppArea) -> Unit, modifier: Modifier = Modifier) {
    Row(
        modifier.clip(RoundedCornerShape(50)).background(SuperficieAlta).padding(3.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        ModeChoice(lang, AppArea.Vents, current == AppArea.Vents) { onSelect(AppArea.Vents) }
        ModeChoice(lang, AppArea.Code, current == AppArea.Code) { onSelect(AppArea.Code) }
    }
}

@Composable
private fun ModeChoice(lang: String, area: AppArea, selected: Boolean, onClick: () -> Unit) {
    val label = when (area) {
        AppArea.Vents -> if (lang == "en") "Winds" else "Vents"
        AppArea.Code -> if (lang == "en") "Code" else "Codi"
    }
    Row(
        Modifier.clip(RoundedCornerShape(50))
            .background(if (selected) SeaSoft(0.2f) else androidx.compose.ui.graphics.Color.Transparent)
            .clickable(role = Role.Tab, onClick = onClick)
            .semantics { this.selected = selected }
            .heightIn(min = 44.dp).padding(horizontal = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(5.dp),
    ) {
        Icon(if (area == AppArea.Vents) Icons.Rounded.Groups else Icons.Rounded.Code, null,
            tint = if (selected) Sea else Muted, modifier = Modifier.padding(vertical = 2.dp))
        Text(label, color = if (selected) Ink else Muted, fontSize = 13.sp)
    }
}

/** One-tap opposite-area shortcut for constrained app-bar slots. */
@Composable
internal fun ModeSwitchIcon(lang: String, current: AppArea, onSelect: (AppArea) -> Unit, modifier: Modifier = Modifier) {
    val target = if (current == AppArea.Vents) AppArea.Code else AppArea.Vents
    val label = when (target) {
        AppArea.Vents -> if (lang == "en") "Switch to Winds" else "Canvia a Vents"
        AppArea.Code -> if (lang == "en") "Switch to Code" else "Canvia a Codi"
    }
    androidx.compose.material3.IconButton(onClick = { onSelect(target) }, modifier = modifier) {
        Icon(if (target == AppArea.Vents) Icons.Rounded.Groups else Icons.Rounded.Code, label, tint = Sea)
    }
}
