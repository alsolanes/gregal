package xyz.solanes.gregal

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Qui ha passat un encàrrec a un altre vent (`by` del fil, task 160): Gregal si el demana Gregal o el Codi. */
data class MsgBy(val id: String, val name: String)

/** El servidor desa l'encàrrec com «Encàrrec de A: …»; la capçalera ja diu el nom, el cos no el repeteix. */
internal fun handoffBody(text: String, name: String): String {
    for (p in listOf("Encàrrec de $name: ", "Task from $name: ")) if (text.startsWith(p)) return text.removePrefix(p)
    return text
}

/** Un encàrrec d'un altre vent: la seva cara i «Encàrrec de A», no una bombolla teva (handoffRow a la web). */
@Composable
internal fun HandoffRow(m: GregalViewModel.PetMsg, asker: Wind?, lang: String) {
    val by = m.by ?: return
    val w = asker ?: Wind(by.id, by.name, "", "")
    val accent = Color(w.argb)
    Column(Modifier.padding(top = 16.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp),
            modifier = Modifier.padding(start = 6.dp, bottom = 2.dp)) {
            WindFace(size = 20, accent = accent, style = w.faceStyle)
            Text(tr(lang, "app.vents.handoffFrom", by.name), color = Muted, fontSize = 12.sp, modifier = Modifier.testTag("handoff-from"))
        }
        WindBubble(m.copy(from = "pet", text = handoffBody(m.text, by.name), by = null), accent, lang = lang)
    }
}
