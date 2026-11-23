package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

// La persona al peu del calaix, com al peu de la barra lateral de la web
// (user-avatar.js): les claus del perfil que comparteixen.
internal const val USER_NAME_KEY = "gregal_user_name"
internal const val USER_COLOR_KEY = "gregal_user_color"
// Gregal, el vent que dona nom a l'app, quan encara no se n'ha triat cap.
internal const val USER_DEFAULT_COLOR = "#C1B4DE"

/** Els colors de l'avatar: els vuit dels vents, els mateixos de la web. */
internal val userColors get() = WindSuggestions.all

/** El nom desat; si no, el del compte; sense comptes, «Tu» / «You». */
internal fun userDisplayName(saved: String, account: String, local: Boolean, lang: String): String {
    val own = saved.trim()
    if (own.isNotEmpty()) return own
    if (!local && account.isNotBlank()) return account
    return if (lang == "en") "You" else "Tu"
}

/** Primera lletra del primer i de l'últim mot: «Maria del Mar Puig» → «MP». */
internal fun userInitials(name: String): String {
    val words = name.trim().split(Regex("\\s+")).filter { it.isNotEmpty() }
    if (words.isEmpty()) return ""
    fun first(w: String) = String(Character.toChars(w.codePointAt(0))).uppercase()
    return if (words.size == 1) first(words[0]) else first(words.first()) + first(words.last())
}

/** La fila del peu del calaix: les inicials sobre el teu color i el teu nom. */
@Composable
internal fun UserAvatarRow(vm: GregalViewModel, lang: String, onClick: () -> Unit) {
    val saved by vm.userName.collectAsState()
    val color by vm.userColor.collectAsState()
    val me by vm.me.collectAsState()
    val name = userDisplayName(saved, me.user, me.local, lang)
    val fill = hexColor(color, Color(0xFFC1B4DE))
    Row(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).clickable(onClick = onClick)
            .heightIn(min = 52.dp).padding(horizontal = 12.dp).testTag("user-avatar"),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // El nom ja es llegeix al costat: les inicials només es veuen.
        Box(Modifier.size(30.dp).background(fill, CircleShape).clearAndSetSemantics {}, contentAlignment = Alignment.Center) {
            Text(userInitials(name), color = windInk(fill), fontSize = 12.sp, fontWeight = FontWeight.SemiBold, maxLines = 1)
        }
        Spacer(Modifier.width(12.dp))
        Text(name, color = Tinta, fontSize = 15.sp, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
    }
}

/** «Com et dius» i el color de l'avatar, a Preferències › General. */
@Composable
internal fun UserProfileFields(vm: GregalViewModel, lang: String) {
    val savedName by vm.userName.collectAsState()
    val savedColor by vm.userColor.collectAsState()
    val me by vm.me.collectAsState()
    var name by remember(savedName) { mutableStateOf(savedName) }
    var color by remember(savedColor) { mutableStateOf(savedColor) }
    var done by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        OutlinedTextField(name, { name = it.take(80); done = false }, singleLine = true,
            label = { Text(tr(lang, "app.prefs.userName")) },
            placeholder = { Text(userDisplayName("", me.user, me.local, lang)) },
            supportingText = { Text(tr(lang, "app.prefs.userNameHelp")) },
            modifier = Modifier.fillMaxWidth())
        Text(tr(lang, "app.prefs.userColor"), color = Ink, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
        Row(Modifier.fillMaxWidth()) {
            userColors.forEach { s ->
                val selected = s.color == color
                Box(
                    Modifier.weight(1f).height(44.dp)
                        .selectable(selected, role = Role.RadioButton) { color = s.color; done = false }
                        .semantics { contentDescription = s.name },
                    contentAlignment = Alignment.Center,
                ) {
                    Box(Modifier.size(28.dp).background(hexColor(s.color, Color.Gray), CircleShape)
                        .border(if (selected) 2.dp else 0.dp, if (selected) Ink else Color.Transparent, CircleShape))
                }
            }
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = { vm.saveUserProfile(name, color); done = true }) { Text(tr(lang, "app.prefs.userSave")) }
            if (done) Text(tr(lang, "app.prefs.userSaved"), color = Sea, fontSize = 12.5.sp)
        }
    }
}
