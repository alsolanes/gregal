package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
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
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** El selector Codi | Vents de dalt del calaix: l'únic lloc on es canvia de mode.
 *  Dues meitats iguals, com un control segmentat. [ventsDot] posa un punt a
 *  «Vents» mentre algun vent treballa o té novetats. */
@Composable
internal fun ModeSwitcher(lang: String, current: AppArea, ventsDot: Boolean, onSelect: (AppArea) -> Unit, modifier: Modifier = Modifier) {
    Row(
        modifier.clip(RoundedCornerShape(50)).background(SuperficieAlta).padding(3.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        ModeChoice(lang, AppArea.Vents, current == AppArea.Vents, ventsDot, Modifier.weight(1f)) { onSelect(AppArea.Vents) }
        ModeChoice(lang, AppArea.Code, current == AppArea.Code, false, Modifier.weight(1f)) { onSelect(AppArea.Code) }
    }
}

@Composable
private fun ModeChoice(lang: String, area: AppArea, selected: Boolean, dot: Boolean, modifier: Modifier, onClick: () -> Unit) {
    val label = tr(lang, if (area == AppArea.Vents) "app.mode.vents" else "app.mode.code")
    Row(
        modifier.clip(RoundedCornerShape(50))
            .background(if (selected) SeaSoft(0.22f) else androidx.compose.ui.graphics.Color.Transparent)
            .clickable(role = Role.Tab, onClick = onClick)
            .semantics { this.selected = selected }
            .heightIn(min = 44.dp).padding(horizontal = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(6.dp, Alignment.CenterHorizontally),
    ) {
        Icon(if (area == AppArea.Vents) Icons.Rounded.Groups else Icons.Rounded.Code, null,
            tint = if (selected) Sea else Muted, modifier = Modifier.padding(vertical = 2.dp))
        Text(label, color = if (selected) Ink else Muted, fontSize = GregalSize.body,
            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal)
        if (dot) {
            val desc = tr(lang, "app.mode.ventsAttention")
            Box(Modifier.size(7.dp).clip(CircleShape).background(Sea).semantics { contentDescription = desc })
        }
    }
}
