package xyz.solanes.gregal

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.ui.semantics.Role
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.ArrowBack
import androidx.compose.material.icons.rounded.Menu
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/*
 * Peces comunes de la interfície. Totes les pantalles parlen el mateix
 * llenguatge: capçalera transparent sobre el fons marí, etiquetes de secció
 * discretes, targetes opaques amb vora fina i radi moderat, files de
 * preferència amb icona i una sola acció principal per context.
 */

/** Radi de les targetes i files; les píndoles fan servir 999. */
val CardShape = RoundedCornerShape(20.dp)

/**
 * Capçalera de pantalla: menú o enrere, títol ferm amb subtítol opcional
 * (context: servidor, projecte, model) i accions a la dreta. Transparent: el
 * fons de la pantalla (`GregalBg`) hi passa per sota.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun GregalTopBar(
    title: String,
    lang: String,
    onMenu: (() -> Unit)? = null,
    onBack: (() -> Unit)? = null,
    subtitle: String? = null,
    actions: @Composable RowScope.() -> Unit = {},
) {
    TopAppBar(
        navigationIcon = {
            when {
                onBack != null -> IconButton(onClick = onBack) {
                    Icon(Icons.AutoMirrored.Rounded.ArrowBack, tr(lang, "app.vents.back"), tint = Ink)
                }
                onMenu != null -> IconButton(onClick = onMenu) {
                    Icon(Icons.Rounded.Menu, tr(lang, "app.nav.menu"), tint = Ink)
                }
            }
        },
        title = {
            Column {
                Text(
                    title, fontSize = 21.sp, fontWeight = FontWeight.Bold, color = Ink, letterSpacing = (-0.3).sp,
                    maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.semantics { heading() },
                )
                if (!subtitle.isNullOrBlank()) {
                    Text(subtitle, fontSize = 12.sp, color = Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
        },
        actions = actions,
        colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.Transparent, scrolledContainerColor = DeepTop),
    )
}

/** Etiqueta de secció: majúscules petites i espaiades, amb acció opcional a la dreta. */
@Composable
fun SectionLabel(text: String, modifier: Modifier = Modifier, action: (@Composable () -> Unit)? = null) {
    Row(
        modifier.fillMaxWidth().heightIn(min = 40.dp).padding(start = 20.dp, end = 12.dp, top = 14.dp, bottom = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text.uppercase(), fontSize = 11.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 1.4.sp, color = Faint,
            modifier = Modifier.weight(1f).semantics { heading() },
        )
        action?.invoke()
    }
}

/** Targeta opaca amb vora fina; clicable si cal (i amb pulsació llarga). */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun GCard(
    modifier: Modifier = Modifier,
    onClick: (() -> Unit)? = null,
    onLongClick: (() -> Unit)? = null,
    padding: PaddingValues = PaddingValues(16.dp),
    color: Color = Superficie,
    content: @Composable ColumnScope.() -> Unit,
) {
    val click = if (onClick != null || onLongClick != null)
        Modifier.combinedClickable(onClick = onClick ?: {}, onLongClick = onLongClick) else Modifier
    Surface(shape = CardShape, color = color, border = BorderStroke(1.dp, Line), modifier = modifier.clip(CardShape).then(click)) {
        Column(Modifier.padding(padding), content = content)
    }
}

/** Fila de preferència o d'opció: icona en una pastilla tintada, títol, detall i element final. */
@Composable
fun SettingsRow(
    icon: ImageVector,
    title: String,
    subtitle: String? = null,
    tint: Color = Sea,
    onClick: (() -> Unit)? = null,
    trailing: (@Composable () -> Unit)? = null,
) {
    Row(
        Modifier.fillMaxWidth().then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier)
            .heightIn(min = 56.dp).padding(horizontal = 16.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Box(Modifier.size(36.dp).background(tint.copy(alpha = 0.14f), RoundedCornerShape(11.dp)), contentAlignment = Alignment.Center) {
            Icon(icon, null, tint = tint, modifier = Modifier.size(20.dp))
        }
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(title, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = Ink)
            if (!subtitle.isNullOrBlank()) Text(subtitle, fontSize = 12.5.sp, color = Muted, lineHeight = 17.sp)
        }
        trailing?.invoke()
    }
}

/** Píndola d'estat amb punt de color: connectat, en pausa, error… mai només color: sempre amb text. */
@Composable
fun StatusPill(text: String, color: Color, modifier: Modifier = Modifier) {
    Row(
        modifier.background(color.copy(alpha = 0.12f), RoundedCornerShape(999.dp)).padding(horizontal = 10.dp, vertical = 5.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.size(7.dp).background(color, CircleShape))
        Spacer(Modifier.width(6.dp))
        Text(text, fontSize = 12.sp, fontWeight = FontWeight.Medium, color = Tinta, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

/**
 * Il·lustració dels estats buits: una bombolla amb degradat mariner, un
 * anell fi i la icona al centre. Més acollidora que una icona sola.
 */
@Composable
fun EmptyArt(icon: ImageVector, tint: Color = Sea, box: Dp = 96.dp, glyph: Dp = 40.dp) {
    Box(
        Modifier.size(box)
            .background(Brush.linearGradient(listOf(tint.copy(alpha = 0.22f), Onada.copy(alpha = 0.10f))), CircleShape)
            .border(1.dp, tint.copy(alpha = 0.22f), CircleShape),
        contentAlignment = Alignment.Center,
    ) {
        Box(Modifier.size(box * 0.62f).background(Superficie.copy(alpha = 0.55f), CircleShape), contentAlignment = Alignment.Center) {
            Icon(icon, null, tint = tint, modifier = Modifier.size(glyph))
        }
    }
}

/** Botó principal en píndola, l'única acció destacada d'un context. */
@Composable
fun PrimaryPill(label: String, onClick: () -> Unit, modifier: Modifier = Modifier, icon: ImageVector? = null, enabled: Boolean = true) {
    Button(
        onClick = onClick, enabled = enabled, modifier = modifier.heightIn(min = 48.dp),
        shape = RoundedCornerShape(999.dp),
        colors = ButtonDefaults.buttonColors(containerColor = Sea, contentColor = OnSea),
        contentPadding = PaddingValues(horizontal = 22.dp, vertical = 12.dp),
    ) {
        if (icon != null) { Icon(icon, null, modifier = Modifier.size(18.dp)); Spacer(Modifier.width(8.dp)) }
        Text(label, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
    }
}

/** Estat buit complet: il·lustració, títol, explicació i, si n'hi ha, l'acció. */
@Composable
fun EmptyState(
    icon: ImageVector,
    title: String,
    body: String,
    modifier: Modifier = Modifier,
    actionLabel: String? = null,
    onAction: (() -> Unit)? = null,
) {
    Column(
        modifier.fillMaxWidth().padding(horizontal = 32.dp, vertical = 48.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        EmptyArt(icon)
        Spacer(Modifier.height(20.dp))
        Text(title, color = Ink, fontSize = 19.sp, fontWeight = FontWeight.SemiBold, textAlign = TextAlign.Center)
        if (body.isNotBlank()) {
            Spacer(Modifier.height(8.dp))
            Text(body, color = Muted, fontSize = 14.sp, lineHeight = 20.sp, textAlign = TextAlign.Center)
        }
        if (actionLabel != null && onAction != null) {
            Spacer(Modifier.height(22.dp))
            PrimaryPill(actionLabel, onAction)
        }
    }
}

/**
 * Selector segmentat en píndola (tema, idioma…): l'opció triada en l'accent,
 * la resta discretes. Cada opció és un botó de ràdio per a l'accessibilitat.
 */
@Composable
fun ChoicePills(options: List<Pair<String, String>>, selected: String, onSelect: (String) -> Unit, modifier: Modifier = Modifier) {
    Row(
        modifier.fillMaxWidth().background(SuperficieAlta, RoundedCornerShape(999.dp)).padding(4.dp).selectableGroup(),
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        options.forEach { (value, label) ->
            val on = value == selected
            Box(
                Modifier.weight(1f).clip(RoundedCornerShape(999.dp))
                    .background(if (on) Sea else Color.Transparent)
                    .selectable(on, role = Role.RadioButton) { onSelect(value) }
                    .heightIn(min = 40.dp).padding(horizontal = 8.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    label, fontSize = 14.sp, fontWeight = if (on) FontWeight.SemiBold else FontWeight.Medium,
                    color = if (on) OnSea else Muted, maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
}
