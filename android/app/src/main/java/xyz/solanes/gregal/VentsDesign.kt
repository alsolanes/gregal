package xyz.solanes.gregal

import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.AutoAwesome
import androidx.compose.material.icons.rounded.AutoStories
import androidx.compose.material.icons.rounded.Construction
import androidx.compose.material.icons.rounded.EditNote
import androidx.compose.material.icons.rounded.Folder
import androidx.compose.material.icons.rounded.FolderOpen
import androidx.compose.material.icons.rounded.Groups
import androidx.compose.material.icons.rounded.Schedule
import androidx.compose.material.icons.rounded.SportsSoccer
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
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
internal fun VentsInboxFilters(lang: String, selected: String, unread: Int = 0, onSelected: (String) -> Unit) {
    Row(
        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        listOf("all", "unread", "groups").forEach { key ->
            val active = selected == key
            // Quants missatges esperen, al mateix filtre: si no n'hi ha cap, la
            // pastilla queda com sempre. FilterChip M3 com la resta de filtres.
            val badge = if (key == "unread") unreadBadge(unread) else ""
            FilterChip(
                selected = active,
                onClick = { onSelected(key) },
                label = {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(ventsCopy(lang, key), fontSize = 12.sp)
                        if (badge.isNotEmpty()) Surface(shape = RoundedCornerShape(999.dp), color = Sea) {
                            Text(
                                badge, color = OnSea, fontSize = 11.sp, fontWeight = FontWeight.Bold,
                                modifier = Modifier.padding(horizontal = 6.dp, vertical = 1.dp),
                            )
                        }
                    }
                },
            )
        }
    }
}

/**
 * L'esquelet d'una conversa mentre el fil arriba del servidor: bombolles
 * alternes que respiren amb un accent suau. Substitueix el «comença la
 * conversa» que sortia un instant i marxava — obrir un xat amb historial no
 * hauria de semblar un xat buit.
 */
@Composable
internal fun ChatSkeleton(accent: Color, modifier: Modifier = Modifier) {
    val respiracio = rememberInfiniteTransition(label = "esquelet")
    val glow by respiracio.animateFloat(
        initialValue = 0.35f, targetValue = 0.9f,
        animationSpec = infiniteRepeatable(tween(850), RepeatMode.Reverse), label = "glow",
    )
    Column(
        modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        // Amplades i costats com una conversa de debò: la del vent a l'esquerra,
        // les teves a la dreta.
        listOf(
            Triple(0.56f, false, 34.dp),
            Triple(0.74f, true, 34.dp),
            Triple(0.42f, false, 34.dp),
            Triple(0.64f, true, 34.dp),
        ).forEach { (amplada, meu, alt) ->
            Box(
                Modifier.align(if (meu) Alignment.End else Alignment.Start)
                    .fillMaxWidth(amplada).height(alt)
                    .clip(RoundedCornerShape(18.dp))
                    .background(if (meu) SuperficieAlta.copy(alpha = 0.55f + 0.25f * glow)
                    else accent.copy(alpha = 0.10f + 0.18f * glow)),
            )
        }
    }
}

@Composable
internal fun VentsExploreSheet(
    lang: String,
    showCorner: Boolean,
    showGames: Boolean,
    onCorner: () -> Unit,
    onGames: () -> Unit,
    onLibrary: () -> Unit,
    onProgramats: (() -> Unit)? = null,
    onActivity: (() -> Unit)? = null,
    onSaved: (() -> Unit)? = null,
    onWiki: (() -> Unit)? = null,
    onFeines: (() -> Unit)? = null,
    onCrew: (() -> Unit)? = null,
    onWorkspace: (() -> Unit)? = null,
) {
    val en = lang == "en"
    Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(bottom = 2.dp)) {
            Icon(Icons.Rounded.AutoAwesome, null, tint = Sea, modifier = Modifier.size(18.dp))
            Text(ventsCopy(lang, "explore"), color = Ink, fontSize = 16.sp, modifier = Modifier.padding(start = 8.dp))
            Spacer(Modifier.weight(1f))
            Text(ventsCopy(lang, "exploreHelp"), color = Muted, fontSize = 12.sp)
        }
        if (onActivity != null) ExploreRow(lang, tr(lang, "act.title"), tr(lang, "explore.activity"), Icons.Rounded.Schedule, onActivity)
        if (onSaved != null) ExploreRow(lang, tr(lang, "saved.title"), tr(lang, "explore.saved"), Icons.Rounded.Folder, onSaved)
        if (onCrew != null) ExploreRow(lang, tr(lang, "app.crew.title"), if (en) "Summon specialists to discuss, build and review" else "Convoca especialistes que en parlin, ho facin i ho revisin", Icons.Rounded.Groups, onCrew)
        if (onFeines != null) ExploreRow(lang, tr(lang, "app.feines.title"), if (en) "Long tasks running on the server and their files" else "Tasques llargues en marxa i els fitxers resultants", Icons.Rounded.Construction, onFeines)
        if (onProgramats != null) ExploreRow(lang, tr(lang, "app.programats.open"), if (en) "Reminders, watches and routines" else "Recordatoris, cors en guàrdia i rutines", Icons.Rounded.Schedule, onProgramats)
        if (onWiki != null) ExploreRow(lang, tr(lang, "app.wiki.open"), if (en) "Distilled pages from your conversations" else "Pàgines resumides de les teves converses", Icons.Rounded.AutoStories, onWiki)
        if (onWorkspace != null) ExploreRow(lang, tr(lang, "app.carpeta.title"), if (en) "Active project folder where winds read and write" else "Carpeta del projecte on llegeixen i escriuen els vents", Icons.Rounded.FolderOpen, onWorkspace)
        if (showCorner) ExploreRow(lang, "Racó", if (en) "The page the winds write on their own" else "La pàgina que escriuen els vents sols", Icons.Rounded.EditNote, onCorner)
        if (showGames) ExploreRow(lang, if (en) "Games" else "Partits", if (en) "Football league between winds" else "La lliga de futbol entre els vents", Icons.Rounded.SportsSoccer, onGames)
        ExploreRow(lang, ventsCopy(lang, "library"), if (en) "Recover and reuse learned information" else "Recupera i reutilitza informació apresa", Icons.Rounded.Folder, onLibrary)
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
private fun ExploreRow(@Suppress("UNUSED_PARAMETER") lang: String, title: String, subtitle: String, icon: ImageVector, onClick: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).clickable(onClick = onClick)
            .background(SuperficieAlta.copy(alpha = .42f)).padding(horizontal = 12.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icon, null, tint = Sea, modifier = Modifier.size(18.dp))
        Column(Modifier.padding(start = 10.dp)) {
            Text(title, color = Ink, fontSize = 14.sp)
            Text(subtitle, color = Muted, fontSize = 12.sp)
        }
    }
}
