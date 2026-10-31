package xyz.solanes.gregal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.Folder
import androidx.compose.material3.Icon
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal fun ventsCopy(lang: String, key: String): String = when (key) {
    "all" -> if (lang == "en") "All" else "Tot"
    "unread" -> if (lang == "en") "Unread" else "No llegits"
    "groups" -> if (lang == "en") "Groups" else "Grups"
    "explore" -> if (lang == "en") "Explore" else "Explora"
    "exploreHelp" -> if (lang == "en") "Quiet tools, gathered in one place" else "Eines tranquil·les, en un sol lloc"
    "library" -> if (lang == "en") "Wind library" else "Biblioteca de vents"
    "addFavorite" -> if (lang == "en") "Add to favorites" else "Afegeix als favorits"
    "removeFavorite" -> if (lang == "en") "Remove from favorites" else "Treu dels favorits"
    else -> key
}

@Composable
internal fun VentsInboxFilters(lang: String, selected: String, onSelected: (String) -> Unit) {
    Row(
        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        listOf("all", "unread", "groups").forEach { key ->
            val active = selected == key
            Surface(
                shape = RoundedCornerShape(999.dp),
                color = if (active) Ink else SuperficieAlta.copy(alpha = .58f),
                modifier = Modifier.clip(RoundedCornerShape(999.dp)).clickable { onSelected(key) }
                    .semantics { this.selected = active; contentDescription = ventsCopy(lang, key) },
            ) {
                Text(
                    ventsCopy(lang, key), color = if (active) Superficie else Muted,
                    fontSize = 12.sp, modifier = Modifier.padding(horizontal = 13.dp, vertical = 8.dp),
                )
            }
        }
    }
}

@Composable
internal fun VentsExploreSheet(
    lang: String,
    showCorner: Boolean,
    showGames: Boolean,
    showArena: Boolean,
    onCorner: () -> Unit,
    onGames: () -> Unit,
    onArena: () -> Unit,
    onLibrary: () -> Unit,
) {
    Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(bottom = 2.dp)) {
            Icon(Icons.Rounded.AutoAwesome, null, tint = Sea, modifier = Modifier.size(18.dp))
            Text(ventsCopy(lang, "explore"), color = Ink, fontSize = 16.sp, modifier = Modifier.padding(start = 8.dp))
            Spacer(Modifier.weight(1f))
            Text(ventsCopy(lang, "exploreHelp"), color = Muted, fontSize = 11.sp)
        }
        if (showCorner) ExploreRow(lang, "Racó", "corner", onCorner)
        if (showGames) ExploreRow(lang, if (lang == "en") "Games" else "Partits", "games", onGames)
        if (showArena) ExploreRow(lang, "Arena", "arena", onArena)
        ExploreRow(lang, ventsCopy(lang, "library"), "library", onLibrary, folder = true)
    }
}

@Composable
internal fun EmptyInboxFilter(lang: String, filter: String) {
    Column(Modifier.fillMaxWidth().padding(horizontal = 28.dp, vertical = 44.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(ventsCopy(lang, filter), color = Ink, fontSize = 18.sp)
        Text(
            if (filter == "groups") {
                if (lang == "en") "Group conversations will appear here." else "Les converses de grup apareixeran aquí."
            } else {
                if (lang == "en") "You are caught up. New proactive updates will appear here." else "Estàs al dia. Els avisos nous apareixeran aquí."
            },
            color = Muted, fontSize = 14.sp,
        )
    }
}

@Composable
private fun ExploreRow(lang: String, title: String, key: String, onClick: () -> Unit, folder: Boolean = false) {
    Row(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).clickable(onClick = onClick)
            .background(SuperficieAlta.copy(alpha = .42f)).padding(horizontal = 12.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(if (folder) Icons.Rounded.Folder else Icons.Rounded.AutoAwesome, null, tint = Sea, modifier = Modifier.size(18.dp))
        Column(Modifier.padding(start = 10.dp)) {
            Text(title, color = Ink, fontSize = 14.sp)
            Text(if (folder) if (lang == "en") "Recover and reuse learned information" else "Recupera i reutilitza informació apresa" else "Plugin", color = Muted, fontSize = 11.sp)
        }
    }
}
