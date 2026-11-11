package xyz.solanes.gregal

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.KeyboardArrowRight
import androidx.compose.material.icons.rounded.Description
import androidx.compose.material.icons.rounded.Forum
import androidx.compose.material.icons.rounded.Image
import androidx.compose.material.icons.rounded.InsertDriveFile
import androidx.compose.material.icons.rounded.Language
import androidx.compose.material.icons.rounded.TableChart
import androidx.compose.material3.Icon
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** El tipus d'un resultat lliurat al fil: decideix la icona de 40 dp. */
internal enum class ResultKind { DOCUMENT, SHEET, IMAGE, WEB, CHAT, FILE }

/**
 * El tipus a partir del nom del fitxer o de l'adreça del resultat. Pura:
 * testejable. Sense extensió coneguda és un fitxer genèric.
 */
internal fun resultKindOf(ref: String): ResultKind {
    val r = ref.trim().lowercase()
    if (r.startsWith("wind:") || r.startsWith("rot:")) return ResultKind.CHAT
    if (r.startsWith("http://") || r.startsWith("https://")) return ResultKind.WEB
    return when (r.substringAfterLast('.', "")) {
        "pdf", "doc", "docx", "md", "txt", "odt", "rtf" -> ResultKind.DOCUMENT
        "xls", "xlsx", "csv", "ods", "tsv" -> ResultKind.SHEET
        "png", "jpg", "jpeg", "webp", "gif", "heic", "svg" -> ResultKind.IMAGE
        "html", "htm" -> ResultKind.WEB
        else -> ResultKind.FILE
    }
}

private fun ResultKind.icon(): ImageVector = when (this) {
    ResultKind.DOCUMENT -> Icons.Rounded.Description
    ResultKind.SHEET -> Icons.Rounded.TableChart
    ResultKind.IMAGE -> Icons.Rounded.Image
    ResultKind.WEB -> Icons.Rounded.Language
    ResultKind.CHAT -> Icons.Rounded.Forum
    ResultKind.FILE -> Icons.Rounded.InsertDriveFile
}

/** La icona de tipus: un quadrat de 40 dp amb la icona al mig. */
@Composable
internal fun ResultTypeIcon(kind: ResultKind) {
    Box(Modifier.size(40.dp).clip(GregalShape.xs).background(SuperficieAlta), contentAlignment = Alignment.Center) {
        Icon(kind.icon(), null, tint = Ink, modifier = Modifier.size(22.dp))
    }
}

/**
 * Un resultat lliurat dins del fil: icona de tipus, títol, subtítol i xevró,
 * en una targeta de 15 dp amb vora fina. Tota la targeta obre el resultat
 * (target tàctil sencer). A sota, opcionalment, una línia petita de procedència
 * («✓ Preparat al teu ordinador · Veure feina») amb la seva pròpia acció.
 *
 * No és una caixa decorativa: només s'ha de fer servir per a una cosa real
 * que es pot obrir (un fitxer, una conversa, una web).
 */
@Composable
internal fun ResultCard(
    title: String,
    subtitle: String,
    modifier: Modifier = Modifier,
    kind: ResultKind = ResultKind.FILE,
    enabled: Boolean = true,
    /** Substitueix la icona de tipus (p. ex. la cara del vent que se n'ocupa). */
    leading: (@Composable () -> Unit)? = null,
    /** La línia de procedència, sense el ✓ (s'hi posa sol). */
    footer: String = "",
    footerAction: String = "",
    onFooter: (() -> Unit)? = null,
    onOpen: () -> Unit,
) {
    Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Surface(
            color = Superficie, shape = GregalShape.card, border = BorderStroke(1.dp, Line),
            modifier = Modifier.fillMaxWidth().clip(GregalShape.card)
                .clickable(enabled = enabled, onClick = onOpen)
                .semantics { role = Role.Button }
                .testTag("result-card"),
        ) {
            Row(
                Modifier.heightIn(min = 64.dp).padding(horizontal = 12.dp, vertical = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                if (leading != null) Box(Modifier.size(40.dp), contentAlignment = Alignment.Center) { leading() }
                else ResultTypeIcon(kind)
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(title, color = Ink, fontSize = 15.sp, lineHeight = 20.sp, fontWeight = FontWeight.SemiBold,
                        maxLines = 2, overflow = TextOverflow.Ellipsis)
                    if (subtitle.isNotBlank()) Text(subtitle, color = Muted, style = GregalType.caption,
                        maxLines = 2, overflow = TextOverflow.Ellipsis)
                }
                if (enabled) Icon(Icons.AutoMirrored.Rounded.KeyboardArrowRight, null, tint = Muted)
            }
        }
        if (footer.isNotBlank()) Row(
            Modifier.padding(start = 4.dp), verticalAlignment = Alignment.CenterVertically,
        ) {
            // Amb lletra gran, la procedència es parteix i l'acció conserva el seu lloc.
            Text("✓ $footer", color = Muted, style = GregalType.chatMeta, modifier = Modifier.weight(1f, fill = false))
            if (onFooter != null && footerAction.isNotBlank()) {
                Text(" · ", color = Muted, style = GregalType.chatMeta)
                // Target tàctil de 48 dp d'alçada encara que el text sigui petit.
                Text(footerAction, color = Ink, style = GregalType.chatMeta, fontWeight = FontWeight.SemiBold,
                    modifier = Modifier.testTag("result-card-footer").heightIn(min = 48.dp).clip(GregalShape.xs)
                        .clickable(onClick = onFooter).wrapContentHeight().padding(horizontal = 6.dp))
            }
        }
    }
}
